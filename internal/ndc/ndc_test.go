package ndc

import (
	"testing"
)

// AC-001: Accept product NDC in 5-4 format
// AC-002: Accept product NDC in 4-4 format
// AC-003: Accept product NDC in 5-3 format
// AC-004: Strip package segment from 3-segment NDCs
// AC-005: Reject dashless input
// AC-013: Return error for invalid format

func TestParse_ValidFormats(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		labeler string
		product string
		format  string
	}{
		// AC-001: 5-4 format
		{name: "AC-001 5-4 format", input: "00069-3150", labeler: "00069", product: "3150", format: "5-4"},
		{name: "AC-001 5-4 another", input: "12345-6789", labeler: "12345", product: "6789", format: "5-4"},

		// AC-002: 4-4 format
		{name: "AC-002 4-4 format", input: "0069-3150", labeler: "0069", product: "3150", format: "4-4"},
		{name: "AC-002 4-4 another", input: "1234-5678", labeler: "1234", product: "5678", format: "4-4"},

		// AC-003: 5-3 format
		{name: "AC-003 5-3 format", input: "00069-315", labeler: "00069", product: "315", format: "5-3"},
		{name: "AC-003 5-3 another", input: "12345-678", labeler: "12345", product: "678", format: "5-3"},

		// AC-004: 3-segment NDC — strip package
		{name: "AC-004 strip package 5-4-2", input: "00069-3150-83", labeler: "00069", product: "3150", format: "5-4"},
		{name: "AC-004 strip package 4-4-2", input: "0069-3150-83", labeler: "0069", product: "3150", format: "4-4"},
		{name: "AC-004 strip package 5-3-2", input: "00069-315-83", labeler: "00069", product: "315", format: "5-3"},
		{name: "AC-004 strip package 5-4-1", input: "00069-3150-3", labeler: "00069", product: "3150", format: "5-4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.input, err)
			}
			if got.Labeler != tt.labeler {
				t.Errorf("Parse(%q).Labeler = %q, want %q", tt.input, got.Labeler, tt.labeler)
			}
			if got.Product != tt.product {
				t.Errorf("Parse(%q).Product = %q, want %q", tt.input, got.Product, tt.product)
			}
			if got.Format != tt.format {
				t.Errorf("Parse(%q).Format = %q, want %q", tt.input, got.Format, tt.format)
			}
		})
	}
}

func TestParse_InvalidFormats(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		// AC-005: dashless rejected
		{name: "AC-005 dashless 9 digits", input: "000693150"},
		{name: "AC-005 dashless 8 digits", input: "00693150"},

		// AC-013: invalid format
		{name: "AC-013 non-numeric labeler", input: "ABCDE-1234"},
		{name: "AC-013 non-numeric product", input: "12345-ABCD"},
		{name: "AC-013 non-numeric both", input: "ABCDE-ABCD"},
		{name: "AC-013 labeler too long", input: "123456-1234"},
		{name: "AC-013 labeler too short", input: "123-1234"},
		{name: "AC-013 product too long", input: "12345-12345"},
		{name: "AC-013 product too short", input: "12345-12"},
		{name: "AC-013 empty input", input: ""},
		{name: "AC-013 just a dash", input: "-"},
		{name: "AC-013 too many segments", input: "00069-3150-83-99"},

		// Edge cases
		{name: "edge whitespace only", input: "   "},
		{name: "edge no digits", input: "abc-def"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)
			if err == nil {
				t.Errorf("Parse(%q) expected error, got nil", tt.input)
			}
		})
	}
}

func TestParse_WhitespaceStripping(t *testing.T) {
	got, err := Parse("  00069-3150  ")
	if err != nil {
		t.Fatalf("Parse with whitespace: unexpected error: %v", err)
	}
	if got.Labeler != "00069" || got.Product != "3150" {
		t.Errorf("Parse with whitespace: got labeler=%q product=%q, want 00069/3150", got.Labeler, got.Product)
	}
}

// AC-006, AC-007, AC-008: Fallback normalization
func TestFallbackNDC(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		fallback string
	}{
		// AC-007: 4-4 pads labeler to 5-4
		{name: "AC-007 4-4 fallback to 5-4", input: "0069-3150", fallback: "00069-3150"},
		{name: "AC-007 4-4 another", input: "1234-5678", fallback: "01234-5678"},

		// AC-008: 5-3 pads product to 5-4
		{name: "AC-008 5-3 fallback to 5-4", input: "00069-315", fallback: "00069-0315"},
		{name: "AC-008 5-3 another", input: "12345-678", fallback: "12345-0678"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.input, err)
			}
			got := parsed.FallbackNDC()
			if got != tt.fallback {
				t.Errorf("FallbackNDC() for %q = %q, want %q", tt.input, got, tt.fallback)
			}
		})
	}
}

func TestFallbackNDC_5_4_ReturnsEmpty(t *testing.T) {
	parsed, err := Parse("00069-3150")
	if err != nil {
		t.Fatalf("Parse unexpected error: %v", err)
	}
	got := parsed.FallbackNDC()
	if got != "" {
		t.Errorf("FallbackNDC() for 5-4 should return empty, got %q", got)
	}
}

// AC-017: ProductNDC string representation
func TestProductNDC_String(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"00069-3150", "00069-3150"},
		{"0069-3150", "0069-3150"},
		{"00069-315", "00069-315"},
		{"00069-3150-83", "00069-3150"}, // package stripped
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			parsed, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.input, err)
			}
			got := parsed.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

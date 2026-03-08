package ndc

import (
	"fmt"
	"strings"
	"unicode"
)

// ProductNDC represents a parsed product NDC (labeler-product, 2 segments).
type ProductNDC struct {
	Raw     string // Original input
	Labeler string // Labeler segment (4-5 digits)
	Product string // Product segment (3-4 digits)
	Format  string // Detected format: "5-4", "4-4", or "5-3"
}

// Parse validates and parses an NDC string into a ProductNDC.
// Accepts 2-segment (labeler-product) or 3-segment (labeler-product-package) NDCs.
// The package segment is stripped if present. Dash is required.
func Parse(input string) (ProductNDC, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ProductNDC{}, fmt.Errorf("NDC must not be empty")
	}

	parts := strings.Split(trimmed, "-")

	switch len(parts) {
	case 1:
		// No dash — rejected
		return ProductNDC{}, fmt.Errorf("NDC must contain a dash separating labeler and product segments")
	case 2:
		// 2-segment: labeler-product
		return parseSegments(trimmed, parts[0], parts[1])
	case 3:
		// 3-segment: labeler-product-package — strip package
		return parseSegments(trimmed, parts[0], parts[1])
	default:
		return ProductNDC{}, fmt.Errorf("NDC contains too many segments (expected 2 or 3, got %d)", len(parts))
	}
}

func parseSegments(raw, labeler, product string) (ProductNDC, error) {
	if !isDigits(labeler) || !isDigits(product) {
		return ProductNDC{}, fmt.Errorf("NDC segments must contain only digits")
	}

	ll := len(labeler)
	pl := len(product)

	var format string
	switch {
	case ll == 5 && pl == 4:
		format = "5-4"
	case ll == 4 && pl == 4:
		format = "4-4"
	case ll == 5 && pl == 3:
		format = "5-3"
	default:
		return ProductNDC{}, fmt.Errorf(
			"invalid NDC segment lengths: labeler=%d product=%d (valid: 5-4, 4-4, 5-3)", ll, pl,
		)
	}

	return ProductNDC{
		Raw:     raw,
		Labeler: labeler,
		Product: product,
		Format:  format,
	}, nil
}

// String returns the product NDC as labeler-product.
func (n ProductNDC) String() string {
	return n.Labeler + "-" + n.Product
}

// FallbackNDC returns the padded 5-4 form for fallback lookup.
// Returns empty string if already in 5-4 format (no fallback needed).
func (n ProductNDC) FallbackNDC() string {
	switch n.Format {
	case "4-4":
		return "0" + n.Labeler + "-" + n.Product
	case "5-3":
		return n.Labeler + "-0" + n.Product
	default:
		return ""
	}
}

func isDigits(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

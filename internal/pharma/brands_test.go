package pharma

import (
	"reflect"
	"testing"
)

// AC-006: Brand names deduplicated case-insensitively and normalized to title case
func TestDeduplicateBrandNames_Basic(t *testing.T) {
	input := []string{"Zocor", "ZOCOR", "zocor"}
	got := DeduplicateBrandNames(input)
	want := []string{"Zocor"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDeduplicateBrandNames_MultipleBrands(t *testing.T) {
	input := []string{"COUMADIN", "Coumadin", "JANTOVEN", "Jantoven"}
	got := DeduplicateBrandNames(input)
	want := []string{"Coumadin", "Jantoven"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDeduplicateBrandNames_AllCaps(t *testing.T) {
	input := []string{"LIPITOR"}
	got := DeduplicateBrandNames(input)
	want := []string{"Lipitor"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDeduplicateBrandNames_Empty(t *testing.T) {
	got := DeduplicateBrandNames(nil)
	if got == nil {
		t.Error("should return empty slice, not nil")
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestDeduplicateBrandNames_EmptyStrings(t *testing.T) {
	input := []string{"", "", "Zocor"}
	got := DeduplicateBrandNames(input)
	want := []string{"Zocor"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

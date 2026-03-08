package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Helper to build a cash-drugs style response with data as flat array
func cashDrugsResponse(results []map[string]any) map[string]any {
	return map[string]any{
		"data": results,
		"meta": map[string]any{
			"slug":       "fda-ndc",
			"source_url": "https://api.fda.gov/drug/ndc.json",
			"fetched_at": "2026-03-08T00:00:00Z",
			"page_count": 1,
			"stale":      false,
		},
	}
}

// AC-006: Query cash-drugs fda-ndc endpoint
func TestHTTPDrugClient_LookupByNDC_HappyPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/cache/fda-ndc" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		ndc := r.URL.Query().Get("NDC")
		if ndc != "58151-158" {
			t.Errorf("unexpected NDC param: %s", ndc)
		}

		resp := cashDrugsResponse([]map[string]any{
			{
				"product_ndc":  "58151-158",
				"brand_name":   "Lipitor",
				"generic_name": "atorvastatin calcium",
				"pharm_class":  []string{"HMG-CoA Reductase Inhibitor [EPC]", "Hydroxymethylglutaryl-CoA Reductase Inhibitors [MoA]"},
			},
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	c := NewHTTPDrugClient(upstream.URL)
	result, err := c.LookupByNDC(context.Background(), "58151-158")
	if err != nil {
		t.Fatalf("LookupByNDC() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("LookupByNDC() returned nil result")
	}
	if result.BrandName != "Lipitor" {
		t.Errorf("BrandName = %q, want %q", result.BrandName, "Lipitor")
	}
	if result.GenericName != "atorvastatin calcium" {
		t.Errorf("GenericName = %q, want %q", result.GenericName, "atorvastatin calcium")
	}
	if len(result.PharmClass) != 2 {
		t.Errorf("PharmClass = %v, want 2 entries", result.PharmClass)
	}
}

// AC-014: Return not found when upstream 404
func TestHTTPDrugClient_LookupByNDC_NotFound(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not_found"})
	}))
	defer upstream.Close()

	c := NewHTTPDrugClient(upstream.URL)
	result, err := c.LookupByNDC(context.Background(), "99999-9999")
	if err != nil {
		t.Fatalf("LookupByNDC() unexpected error for 404: %v", err)
	}
	if result != nil {
		t.Errorf("LookupByNDC() should return nil for 404, got %+v", result)
	}
}

// AC-015: Upstream returns 502
func TestHTTPDrugClient_LookupByNDC_UpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "upstream unavailable", "slug": "fda-ndc"})
	}))
	defer upstream.Close()

	c := NewHTTPDrugClient(upstream.URL)
	_, err := c.LookupByNDC(context.Background(), "00069-3150")
	if err == nil {
		t.Error("LookupByNDC() expected error for 502, got nil")
	}
}

// AC-015: cash-drugs unreachable
func TestHTTPDrugClient_LookupByNDC_Unreachable(t *testing.T) {
	c := NewHTTPDrugClient("http://localhost:1")
	_, err := c.LookupByNDC(context.Background(), "00069-3150")
	if err == nil {
		t.Error("LookupByNDC() expected error for unreachable, got nil")
	}
}

// AC-016: Partial data — no pharm class
func TestHTTPDrugClient_LookupByNDC_PartialData(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := cashDrugsResponse([]map[string]any{
			{
				"product_ndc":  "58151-158",
				"brand_name":   "Lipitor",
				"generic_name": "atorvastatin calcium",
				// no pharm_class
			},
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	c := NewHTTPDrugClient(upstream.URL)
	result, err := c.LookupByNDC(context.Background(), "58151-158")
	if err != nil {
		t.Fatalf("LookupByNDC() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("LookupByNDC() returned nil for partial data")
	}
	if result.BrandName != "Lipitor" {
		t.Errorf("BrandName = %q, want Lipitor", result.BrandName)
	}
}

// Malformed JSON response from upstream
func TestHTTPDrugClient_LookupByNDC_MalformedJSON(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer upstream.Close()

	c := NewHTTPDrugClient(upstream.URL)
	_, err := c.LookupByNDC(context.Background(), "00069-3150")
	if err == nil {
		t.Error("LookupByNDC() expected error for malformed JSON, got nil")
	}
}

// Empty data array returns nil
func TestHTTPDrugClient_LookupByNDC_EmptyResults(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := cashDrugsResponse([]map[string]any{})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	c := NewHTTPDrugClient(upstream.URL)
	result, err := c.LookupByNDC(context.Background(), "99999-9999")
	if err != nil {
		t.Fatalf("LookupByNDC() unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("LookupByNDC() should return nil for empty data, got %+v", result)
	}
}

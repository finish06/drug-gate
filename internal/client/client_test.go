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

// LookupByGenericName happy path
func TestHTTPDrugClient_LookupByGenericName_HappyPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("GENERIC_NAME") != "simvastatin" {
			t.Errorf("unexpected GENERIC_NAME param: %s", r.URL.Query().Get("GENERIC_NAME"))
		}
		resp := cashDrugsResponse([]map[string]any{
			{
				"product_ndc":  "00069-3150",
				"brand_name":   "Zocor",
				"generic_name": "simvastatin",
				"pharm_class":  []string{"HMG-CoA Reductase Inhibitor [EPC]"},
			},
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	c := NewHTTPDrugClient(upstream.URL)
	results, err := c.LookupByGenericName(context.Background(), "simvastatin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].GenericName != "simvastatin" {
		t.Errorf("GenericName = %q, want simvastatin", results[0].GenericName)
	}
}

// LookupByBrandName happy path
func TestHTTPDrugClient_LookupByBrandName_HappyPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("BRAND_NAME") != "Lipitor" {
			t.Errorf("unexpected BRAND_NAME param: %s", r.URL.Query().Get("BRAND_NAME"))
		}
		resp := cashDrugsResponse([]map[string]any{
			{
				"product_ndc":  "00069-3150",
				"brand_name":   "Lipitor",
				"generic_name": "atorvastatin calcium",
				"pharm_class":  []string{"HMG-CoA Reductase Inhibitor [EPC]"},
			},
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	c := NewHTTPDrugClient(upstream.URL)
	results, err := c.LookupByBrandName(context.Background(), "Lipitor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].BrandName != "Lipitor" {
		t.Errorf("BrandName = %q, want Lipitor", results[0].BrandName)
	}
}

// LookupByGenericName returns nil for empty results
func TestHTTPDrugClient_LookupByGenericName_NotFound(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer upstream.Close()

	c := NewHTTPDrugClient(upstream.URL)
	results, err := c.LookupByGenericName(context.Background(), "notreal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for 404, got %+v", results)
	}
}

// LookupByGenericName upstream error
func TestHTTPDrugClient_LookupByGenericName_UpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer upstream.Close()

	c := NewHTTPDrugClient(upstream.URL)
	_, err := c.LookupByGenericName(context.Background(), "simvastatin")
	if err == nil {
		t.Error("expected error for 502, got nil")
	}
}

// LookupByPharmClass happy path
func TestHTTPDrugClient_LookupByPharmClass_HappyPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("PHARM_CLASS") == "" {
			t.Error("missing PHARM_CLASS param")
		}
		resp := cashDrugsResponse([]map[string]any{
			{"product_ndc": "00069-3150", "brand_name": "Zocor", "generic_name": "simvastatin"},
			{"product_ndc": "00069-3151", "brand_name": "Lipitor", "generic_name": "atorvastatin calcium"},
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	c := NewHTTPDrugClient(upstream.URL)
	results, err := c.LookupByPharmClass(context.Background(), "HMG-CoA Reductase Inhibitor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
}

// FetchDrugNames happy path
func TestHTTPDrugClient_FetchDrugNames_HappyPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/cache/drugnames" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := map[string]any{
			"data": []map[string]any{
				{"name_type": "G", "drug_name": "simvastatin"},
				{"name_type": "B", "drug_name": "Zocor"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	c := NewHTTPDrugClient(upstream.URL)
	names, err := c.FetchDrugNames(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("got %d names, want 2", len(names))
	}
	if names[0].NameType != "G" || names[0].DrugName != "simvastatin" {
		t.Errorf("names[0] = %+v, want G/simvastatin", names[0])
	}
}

// FetchDrugNames upstream error
func TestHTTPDrugClient_FetchDrugNames_UpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer upstream.Close()

	c := NewHTTPDrugClient(upstream.URL)
	_, err := c.FetchDrugNames(context.Background())
	if err == nil {
		t.Error("expected error for 502, got nil")
	}
}

// FetchDrugClasses happy path
func TestHTTPDrugClient_FetchDrugClasses_HappyPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/cache/drugclasses" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := map[string]any{
			"data": []map[string]any{
				{"class_name": "HMG-CoA Reductase Inhibitor", "class_type": "EPC"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	c := NewHTTPDrugClient(upstream.URL)
	classes, err := c.FetchDrugClasses(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(classes) != 1 {
		t.Fatalf("got %d classes, want 1", len(classes))
	}
	if classes[0].ClassName != "HMG-CoA Reductase Inhibitor" {
		t.Errorf("ClassName = %q, want HMG-CoA Reductase Inhibitor", classes[0].ClassName)
	}
}

// Unreachable upstream for name-based lookup
func TestHTTPDrugClient_LookupByGenericName_Unreachable(t *testing.T) {
	c := NewHTTPDrugClient("http://localhost:1")
	_, err := c.LookupByGenericName(context.Background(), "simvastatin")
	if err == nil {
		t.Error("expected error for unreachable, got nil")
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

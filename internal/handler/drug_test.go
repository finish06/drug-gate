package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/go-chi/chi/v5"
)

// mockDrugClient implements client.DrugClient for testing
type mockDrugClient struct {
	results map[string]*client.DrugResult // keyed by NDC
	err     error
}

func (m *mockDrugClient) LookupByNDC(ctx context.Context, ndc string) (*client.DrugResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	result, ok := m.results[ndc]
	if !ok {
		return nil, nil
	}
	return result, nil
}

// callCountMockClient allows per-call behavior control
type callCountMockClient struct {
	onCall func(ndc string) (*client.DrugResult, error)
}

func (m *callCountMockClient) LookupByNDC(ctx context.Context, ndc string) (*client.DrugResult, error) {
	return m.onCall(ndc)
}

func newTestRouter(h *DrugHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/v1/drugs/ndc/{ndc}", h.HandleNDCLookup)
	return r
}

func doRequest(router http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// TC-001: Happy path 5-4
func TestHandleNDCLookup_AC001_HappyPath_5_4(t *testing.T) {
	mock := &mockDrugClient{
		results: map[string]*client.DrugResult{
			"00069-3150": {
				ProductNDC:  "00069-3150",
				BrandName:   "Lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass:  []string{"HMG-CoA Reductase Inhibitor"},
			},
		},
	}
	h := NewDrugHandler(mock)
	router := newTestRouter(h)
	rr := doRequest(router, "/v1/drugs/ndc/00069-3150")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", rr.Code, rr.Body.String())
	}

	var resp model.DrugDetailResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.NDC != "00069-3150" {
		t.Errorf("NDC = %q, want %q", resp.NDC, "00069-3150")
	}
	if resp.Name != "Lipitor" {
		t.Errorf("Name = %q, want %q", resp.Name, "Lipitor")
	}
	if resp.GenericName != "atorvastatin calcium" {
		t.Errorf("GenericName = %q, want %q", resp.GenericName, "atorvastatin calcium")
	}
	if len(resp.Classes) != 1 || resp.Classes[0] != "HMG-CoA Reductase Inhibitor" {
		t.Errorf("Classes = %v, want [HMG-CoA Reductase Inhibitor]", resp.Classes)
	}
}

// TC-002: 4-4 exact match
func TestHandleNDCLookup_AC002_4_4_ExactMatch(t *testing.T) {
	mock := &mockDrugClient{
		results: map[string]*client.DrugResult{
			"0069-3150": {
				ProductNDC:  "0069-3150",
				BrandName:   "Lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass:  []string{"HMG-CoA Reductase Inhibitor"},
			},
		},
	}
	h := NewDrugHandler(mock)
	router := newTestRouter(h)
	rr := doRequest(router, "/v1/drugs/ndc/0069-3150")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

// TC-003: 4-4 fallback to 5-4
func TestHandleNDCLookup_AC007_4_4_FallbackTo5_4(t *testing.T) {
	mock := &mockDrugClient{
		results: map[string]*client.DrugResult{
			// No exact match for "0069-3150", but padded "00069-3150" exists
			"00069-3150": {
				ProductNDC:  "00069-3150",
				BrandName:   "Lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass:  []string{"HMG-CoA Reductase Inhibitor"},
			},
		},
	}
	h := NewDrugHandler(mock)
	router := newTestRouter(h)
	rr := doRequest(router, "/v1/drugs/ndc/0069-3150")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (fallback). body: %s", rr.Code, rr.Body.String())
	}

	var resp model.DrugDetailResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.NDC != "00069-3150" {
		t.Errorf("NDC = %q, want %q (padded fallback)", resp.NDC, "00069-3150")
	}
}

// TC-004: 5-3 exact match
func TestHandleNDCLookup_AC003_5_3_ExactMatch(t *testing.T) {
	mock := &mockDrugClient{
		results: map[string]*client.DrugResult{
			"00069-315": {
				ProductNDC:  "00069-315",
				BrandName:   "Lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass:  []string{"HMG-CoA Reductase Inhibitor"},
			},
		},
	}
	h := NewDrugHandler(mock)
	router := newTestRouter(h)
	rr := doRequest(router, "/v1/drugs/ndc/00069-315")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

// TC-005: 5-3 fallback to 5-4
func TestHandleNDCLookup_AC008_5_3_FallbackTo5_4(t *testing.T) {
	mock := &mockDrugClient{
		results: map[string]*client.DrugResult{
			"00069-0315": {
				ProductNDC:  "00069-0315",
				BrandName:   "Lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass:  []string{"HMG-CoA Reductase Inhibitor"},
			},
		},
	}
	h := NewDrugHandler(mock)
	router := newTestRouter(h)
	rr := doRequest(router, "/v1/drugs/ndc/00069-315")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (fallback). body: %s", rr.Code, rr.Body.String())
	}

	var resp model.DrugDetailResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.NDC != "00069-0315" {
		t.Errorf("NDC = %q, want %q (padded fallback)", resp.NDC, "00069-0315")
	}
}

// TC-006: 3-segment NDC — package stripped
func TestHandleNDCLookup_AC004_StripPackage(t *testing.T) {
	mock := &mockDrugClient{
		results: map[string]*client.DrugResult{
			"00069-3150": {
				ProductNDC:  "00069-3150",
				BrandName:   "Lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass:  []string{"HMG-CoA Reductase Inhibitor"},
			},
		},
	}
	h := NewDrugHandler(mock)
	router := newTestRouter(h)
	rr := doRequest(router, "/v1/drugs/ndc/00069-3150-83")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", rr.Code, rr.Body.String())
	}

	var resp model.DrugDetailResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.NDC != "00069-3150" {
		t.Errorf("NDC = %q, want %q", resp.NDC, "00069-3150")
	}
}

// TC-007: Dashless rejected
func TestHandleNDCLookup_AC005_DashlessRejected(t *testing.T) {
	h := NewDrugHandler(&mockDrugClient{})
	router := newTestRouter(h)
	rr := doRequest(router, "/v1/drugs/ndc/000693150")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}

	var resp model.ErrorResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Error != "invalid_ndc" {
		t.Errorf("Error = %q, want %q", resp.Error, "invalid_ndc")
	}
}

// TC-008: Non-numeric rejected
func TestHandleNDCLookup_AC013_NonNumeric(t *testing.T) {
	h := NewDrugHandler(&mockDrugClient{})
	router := newTestRouter(h)
	rr := doRequest(router, "/v1/drugs/ndc/ABCDE-1234")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

// TC-009: Wrong segment lengths
func TestHandleNDCLookup_AC013_WrongSegments(t *testing.T) {
	h := NewDrugHandler(&mockDrugClient{})
	router := newTestRouter(h)
	rr := doRequest(router, "/v1/drugs/ndc/123456-12345")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

// TC-010: Not found after fallback
func TestHandleNDCLookup_AC014_NotFound(t *testing.T) {
	mock := &mockDrugClient{results: map[string]*client.DrugResult{}}
	h := NewDrugHandler(mock)
	router := newTestRouter(h)
	rr := doRequest(router, "/v1/drugs/ndc/99999-9999")

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404. body: %s", rr.Code, rr.Body.String())
	}

	var resp model.ErrorResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Error != "not_found" {
		t.Errorf("Error = %q, want %q", resp.Error, "not_found")
	}
}

// TC-011: Upstream unavailable
func TestHandleNDCLookup_AC015_UpstreamError(t *testing.T) {
	mock := &mockDrugClient{err: client.ErrUpstream}
	h := NewDrugHandler(mock)
	router := newTestRouter(h)
	rr := doRequest(router, "/v1/drugs/ndc/00069-3150")

	if rr.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502. body: %s", rr.Code, rr.Body.String())
	}

	var resp model.ErrorResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Error != "upstream_error" {
		t.Errorf("Error = %q, want %q", resp.Error, "upstream_error")
	}
}

// TC-012: Partial data — no classes
func TestHandleNDCLookup_AC016_PartialData(t *testing.T) {
	mock := &mockDrugClient{
		results: map[string]*client.DrugResult{
			"00069-3150": {
				ProductNDC:  "00069-3150",
				BrandName:   "Lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass:  nil,
			},
		},
	}
	h := NewDrugHandler(mock)
	router := newTestRouter(h)
	rr := doRequest(router, "/v1/drugs/ndc/00069-3150")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.DrugDetailResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Classes == nil {
		t.Error("Classes should be empty slice, not nil")
	}
	if len(resp.Classes) != 0 {
		t.Errorf("Classes = %v, want empty", resp.Classes)
	}
}

// Internal error (non-ErrUpstream) on exact lookup
func TestHandleNDCLookup_InternalError_ExactLookup(t *testing.T) {
	mock := &mockDrugClient{err: errors.New("something unexpected")}
	h := NewDrugHandler(mock)
	router := newTestRouter(h)
	rr := doRequest(router, "/v1/drugs/ndc/00069-3150")

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

// Upstream error during fallback lookup
func TestHandleNDCLookup_UpstreamError_DuringFallback(t *testing.T) {
	callCount := 0
	mock := &callCountMockClient{
		onCall: func(ndc string) (*client.DrugResult, error) {
			callCount++
			if callCount == 1 {
				return nil, nil // exact match not found
			}
			return nil, client.ErrUpstream // fallback fails
		},
	}
	h := NewDrugHandler(mock)
	router := newTestRouter(h)
	rr := doRequest(router, "/v1/drugs/ndc/0069-3150") // 4-4 triggers fallback

	if rr.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rr.Code)
	}
}

// Internal error during fallback lookup
func TestHandleNDCLookup_InternalError_DuringFallback(t *testing.T) {
	callCount := 0
	mock := &callCountMockClient{
		onCall: func(ndc string) (*client.DrugResult, error) {
			callCount++
			if callCount == 1 {
				return nil, nil // exact match not found
			}
			return nil, errors.New("unexpected fallback error")
		},
	}
	h := NewDrugHandler(mock)
	router := newTestRouter(h)
	rr := doRequest(router, "/v1/drugs/ndc/0069-3150")

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

// AC-012: Consistent response shape
func TestHandleNDCLookup_AC012_ConsistentShape(t *testing.T) {
	mock := &mockDrugClient{
		results: map[string]*client.DrugResult{
			"00069-3150": {
				ProductNDC:  "00069-3150",
				BrandName:   "Lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass:  []string{"HMG-CoA Reductase Inhibitor"},
			},
		},
	}
	h := NewDrugHandler(mock)
	router := newTestRouter(h)
	rr := doRequest(router, "/v1/drugs/ndc/00069-3150")

	var raw map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	// Verify all expected fields are present
	for _, field := range []string{"ndc", "name", "generic_name", "classes"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("response missing field %q", field)
		}
	}
}

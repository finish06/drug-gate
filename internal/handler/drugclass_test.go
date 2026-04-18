package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/go-chi/chi/v5"
)

// mockDrugClassClient implements DrugClassClient for testing.
type mockDrugClassClient struct {
	genericResults []client.DrugResult
	genericErr     error
	brandResults   []client.DrugResult
	brandErr       error
}

func (m *mockDrugClassClient) LookupByGenericName(ctx context.Context, name string) ([]client.DrugResult, error) {
	if m.genericErr != nil {
		return nil, m.genericErr
	}
	return m.genericResults, nil
}

func (m *mockDrugClassClient) LookupByBrandName(ctx context.Context, name string) ([]client.DrugResult, error) {
	if m.brandErr != nil {
		return nil, m.brandErr
	}
	return m.brandResults, nil
}

func newDrugClassTestRouter(h *DrugClassHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/v1/drugs/class", h.HandleDrugClassLookup)
	return r
}

func doDrugClassRequest(router http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// AC-001: Valid generic name returns 200 with drug class info
func TestHandleDrugClassLookup_AC001_ValidGenericName(t *testing.T) {
	mock := &mockDrugClassClient{
		genericResults: []client.DrugResult{
			{
				ProductNDC:  "00069-3150",
				BrandName:   "Lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass:  []string{"HMG-CoA Reductase Inhibitor [EPC]"},
			},
		},
	}
	h := NewDrugClassHandler(mock)
	router := newDrugClassTestRouter(h)
	rr := doDrugClassRequest(router, "/v1/drugs/class?name=atorvastatin")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", rr.Code, rr.Body.String())
	}

	var resp model.DrugClassResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(resp.Classes) == 0 {
		t.Error("expected at least one class in response")
	}
}

// AC-002: If GENERIC_NAME returns no results, retry with BRAND_NAME
func TestHandleDrugClassLookup_AC002_FallbackToBrandName(t *testing.T) {
	mock := &mockDrugClassClient{
		genericResults: nil, // no results for generic lookup
		brandResults: []client.DrugResult{
			{
				ProductNDC:  "00069-3150",
				BrandName:   "Lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass:  []string{"HMG-CoA Reductase Inhibitor [EPC]"},
			},
		},
	}
	h := NewDrugClassHandler(mock)
	router := newDrugClassTestRouter(h)
	rr := doDrugClassRequest(router, "/v1/drugs/class?name=lipitor")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (brand fallback). body: %s", rr.Code, rr.Body.String())
	}

	var resp model.DrugClassResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.GenericName != "atorvastatin calcium" {
		t.Errorf("GenericName = %q, want %q", resp.GenericName, "atorvastatin calcium")
	}
}

// AC-003: Response includes query_name
func TestHandleDrugClassLookup_AC003_QueryNameInResponse(t *testing.T) {
	mock := &mockDrugClassClient{
		genericResults: []client.DrugResult{
			{
				ProductNDC:  "00069-3150",
				BrandName:   "Lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass:  []string{"HMG-CoA Reductase Inhibitor [EPC]"},
			},
		},
	}
	h := NewDrugClassHandler(mock)
	router := newDrugClassTestRouter(h)
	rr := doDrugClassRequest(router, "/v1/drugs/class?name=atorvastatin")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.DrugClassResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.QueryName != "atorvastatin" {
		t.Errorf("QueryName = %q, want %q", resp.QueryName, "atorvastatin")
	}
}

// AC-004: Response includes generic_name
func TestHandleDrugClassLookup_AC004_GenericNameInResponse(t *testing.T) {
	mock := &mockDrugClassClient{
		genericResults: []client.DrugResult{
			{
				ProductNDC:  "00069-3150",
				BrandName:   "Lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass:  []string{"HMG-CoA Reductase Inhibitor [EPC]"},
			},
		},
	}
	h := NewDrugClassHandler(mock)
	router := newDrugClassTestRouter(h)
	rr := doDrugClassRequest(router, "/v1/drugs/class?name=atorvastatin")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.DrugClassResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.GenericName != "atorvastatin calcium" {
		t.Errorf("GenericName = %q, want %q", resp.GenericName, "atorvastatin calcium")
	}
}

// AC-005: Response includes brand_names as deduplicated array
func TestHandleDrugClassLookup_AC005_BrandNamesDeduplicated(t *testing.T) {
	mock := &mockDrugClassClient{
		genericResults: []client.DrugResult{
			{
				ProductNDC:  "00069-3150",
				BrandName:   "Lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass:  []string{"HMG-CoA Reductase Inhibitor [EPC]"},
			},
			{
				ProductNDC:  "00069-3151",
				BrandName:   "Lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass:  []string{"HMG-CoA Reductase Inhibitor [EPC]"},
			},
			{
				ProductNDC:  "00591-0001",
				BrandName:   "Atorvastatin Calcium",
				GenericName: "atorvastatin calcium",
				PharmClass:  []string{"HMG-CoA Reductase Inhibitor [EPC]"},
			},
		},
	}
	h := NewDrugClassHandler(mock)
	router := newDrugClassTestRouter(h)
	rr := doDrugClassRequest(router, "/v1/drugs/class?name=atorvastatin")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.DrugClassResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(resp.BrandNames) != 2 {
		t.Errorf("BrandNames count = %d, want 2 (deduplicated). got: %v", len(resp.BrandNames), resp.BrandNames)
	}
}

// AC-006: Brand names deduplicated case-insensitively and title-cased
func TestHandleDrugClassLookup_AC006_BrandNamesTitleCased(t *testing.T) {
	mock := &mockDrugClassClient{
		genericResults: []client.DrugResult{
			{
				ProductNDC:  "00069-3150",
				BrandName:   "LIPITOR",
				GenericName: "atorvastatin calcium",
				PharmClass:  []string{"HMG-CoA Reductase Inhibitor [EPC]"},
			},
			{
				ProductNDC:  "00069-3151",
				BrandName:   "lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass:  []string{"HMG-CoA Reductase Inhibitor [EPC]"},
			},
			{
				ProductNDC:  "00591-0001",
				BrandName:   "Lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass:  []string{"HMG-CoA Reductase Inhibitor [EPC]"},
			},
		},
	}
	h := NewDrugClassHandler(mock)
	router := newDrugClassTestRouter(h)
	rr := doDrugClassRequest(router, "/v1/drugs/class?name=atorvastatin")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.DrugClassResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	// All three are "lipitor" case-insensitively, so should deduplicate to 1
	if len(resp.BrandNames) != 1 {
		t.Fatalf("BrandNames count = %d, want 1 (case-insensitive dedup). got: %v", len(resp.BrandNames), resp.BrandNames)
	}
	if resp.BrandNames[0] != "Lipitor" {
		t.Errorf("BrandNames[0] = %q, want %q (title-cased)", resp.BrandNames[0], "Lipitor")
	}
}

// AC-007: Response includes classes array with name and type
func TestHandleDrugClassLookup_AC007_ClassesArrayWithNameAndType(t *testing.T) {
	mock := &mockDrugClassClient{
		genericResults: []client.DrugResult{
			{
				ProductNDC:  "00069-3150",
				BrandName:   "Lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass: []string{
					"HMG-CoA Reductase Inhibitor [EPC]",
					"Hydroxymethylglutaryl-CoA Reductase Inhibitors [MoA]",
				},
			},
		},
	}
	h := NewDrugClassHandler(mock)
	router := newDrugClassTestRouter(h)
	rr := doDrugClassRequest(router, "/v1/drugs/class?name=atorvastatin")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.DrugClassResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(resp.Classes) != 2 {
		t.Fatalf("Classes count = %d, want 2. got: %v", len(resp.Classes), resp.Classes)
	}
	// Verify each class has Name and Type
	for i, c := range resp.Classes {
		if c.Name == "" {
			t.Errorf("Classes[%d].Name is empty", i)
		}
		if c.Type == "" {
			t.Errorf("Classes[%d].Type is empty", i)
		}
	}
}

// AC-008: Class type parsed from bracket suffix
func TestHandleDrugClassLookup_AC008_ClassTypeParsedFromBracket(t *testing.T) {
	mock := &mockDrugClassClient{
		genericResults: []client.DrugResult{
			{
				ProductNDC:  "00069-3150",
				BrandName:   "Lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass: []string{
					"HMG-CoA Reductase Inhibitor [EPC]",
					"Hydroxymethylglutaryl-CoA Reductase Inhibitors [MoA]",
				},
			},
		},
	}
	h := NewDrugClassHandler(mock)
	router := newDrugClassTestRouter(h)
	rr := doDrugClassRequest(router, "/v1/drugs/class?name=atorvastatin")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.DrugClassResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(resp.Classes) < 2 {
		t.Fatalf("Classes count = %d, want >= 2", len(resp.Classes))
	}

	// First class: "HMG-CoA Reductase Inhibitor [EPC]"
	if resp.Classes[0].Name != "HMG-CoA Reductase Inhibitor" {
		t.Errorf("Classes[0].Name = %q, want %q", resp.Classes[0].Name, "HMG-CoA Reductase Inhibitor")
	}
	if resp.Classes[0].Type != "EPC" {
		t.Errorf("Classes[0].Type = %q, want %q", resp.Classes[0].Type, "EPC")
	}

	// Second class: "Hydroxymethylglutaryl-CoA Reductase Inhibitors [MoA]"
	if resp.Classes[1].Name != "Hydroxymethylglutaryl-CoA Reductase Inhibitors" {
		t.Errorf("Classes[1].Name = %q, want %q", resp.Classes[1].Name, "Hydroxymethylglutaryl-CoA Reductase Inhibitors")
	}
	if resp.Classes[1].Type != "MoA" {
		t.Errorf("Classes[1].Type = %q, want %q", resp.Classes[1].Type, "MoA")
	}
}

// AC-009: Missing name param returns 400
func TestHandleDrugClassLookup_AC009_MissingNameParam(t *testing.T) {
	mock := &mockDrugClassClient{}
	h := NewDrugClassHandler(mock)
	router := newDrugClassTestRouter(h)
	rr := doDrugClassRequest(router, "/v1/drugs/class")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400. body: %s", rr.Code, rr.Body.String())
	}

	var resp model.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Error == "" {
		t.Error("expected non-empty error field in response")
	}
}

// AC-010: Empty name param returns 400
func TestHandleDrugClassLookup_AC010_EmptyNameParam(t *testing.T) {
	mock := &mockDrugClassClient{}
	h := NewDrugClassHandler(mock)
	router := newDrugClassTestRouter(h)
	rr := doDrugClassRequest(router, "/v1/drugs/class?name=")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400. body: %s", rr.Code, rr.Body.String())
	}

	var resp model.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Error == "" {
		t.Error("expected non-empty error field in response")
	}
}

// AC-011: Drug not found returns 404
func TestHandleDrugClassLookup_AC011_DrugNotFound(t *testing.T) {
	mock := &mockDrugClassClient{
		genericResults: nil, // no generic results
		brandResults:   nil, // no brand results either
	}
	h := NewDrugClassHandler(mock)
	router := newDrugClassTestRouter(h)
	rr := doDrugClassRequest(router, "/v1/drugs/class?name=nonexistentdrug")

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404. body: %s", rr.Code, rr.Body.String())
	}

	var resp model.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Error != "not_found" {
		t.Errorf("Error = %q, want %q", resp.Error, "not_found")
	}
}

// AC-012: Upstream error returns 502
func TestHandleDrugClassLookup_AC012_UpstreamError(t *testing.T) {
	mock := &mockDrugClassClient{
		genericErr: client.ErrUpstream,
	}
	h := NewDrugClassHandler(mock)
	router := newDrugClassTestRouter(h)
	rr := doDrugClassRequest(router, "/v1/drugs/class?name=atorvastatin")

	if rr.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502. body: %s", rr.Code, rr.Body.String())
	}

	var resp model.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Error != "upstream_error" {
		t.Errorf("Error = %q, want %q", resp.Error, "upstream_error")
	}
}

// AC-012 variant: Upstream error on brand fallback returns 502
func TestHandleDrugClassLookup_AC012_UpstreamErrorOnBrandFallback(t *testing.T) {
	mock := &mockDrugClassClient{
		genericResults: nil,                // no generic results, triggers fallback
		brandErr:       client.ErrUpstream, // brand lookup fails
	}
	h := NewDrugClassHandler(mock)
	router := newDrugClassTestRouter(h)
	rr := doDrugClassRequest(router, "/v1/drugs/class?name=lipitor")

	if rr.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502. body: %s", rr.Code, rr.Body.String())
	}
}

// AC-015: Empty classes array when no pharm_class
func TestHandleDrugClassLookup_AC015_EmptyClassesWhenNoPharmClass(t *testing.T) {
	mock := &mockDrugClassClient{
		genericResults: []client.DrugResult{
			{
				ProductNDC:  "00069-3150",
				BrandName:   "Lipitor",
				GenericName: "atorvastatin calcium",
				PharmClass:  nil,
			},
		},
	}
	h := NewDrugClassHandler(mock)
	router := newDrugClassTestRouter(h)
	rr := doDrugClassRequest(router, "/v1/drugs/class?name=atorvastatin")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.DrugClassResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Classes == nil {
		t.Error("Classes should be empty slice, not nil")
	}
	if len(resp.Classes) != 0 {
		t.Errorf("Classes count = %d, want 0. got: %v", len(resp.Classes), resp.Classes)
	}
}

// AC-016: Empty brand_names when no brand name
func TestHandleDrugClassLookup_AC016_EmptyBrandNamesWhenNoBrandName(t *testing.T) {
	mock := &mockDrugClassClient{
		genericResults: []client.DrugResult{
			{
				ProductNDC:  "00069-3150",
				BrandName:   "",
				GenericName: "atorvastatin calcium",
				PharmClass:  []string{"HMG-CoA Reductase Inhibitor [EPC]"},
			},
		},
	}
	h := NewDrugClassHandler(mock)
	router := newDrugClassTestRouter(h)
	rr := doDrugClassRequest(router, "/v1/drugs/class?name=atorvastatin")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.DrugClassResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.BrandNames == nil {
		t.Error("BrandNames should be empty slice, not nil")
	}
	if len(resp.BrandNames) != 0 {
		t.Errorf("BrandNames count = %d, want 0. got: %v", len(resp.BrandNames), resp.BrandNames)
	}
}

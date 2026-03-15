package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/go-chi/chi/v5"
)

// testDrugsInClass returns a fixed set of drugs belonging to a class.
func testDrugsInClass() []model.DrugInClassEntry {
	return []model.DrugInClassEntry{
		{GenericName: "atorvastatin calcium", BrandName: "Lipitor"},
		{GenericName: "rosuvastatin calcium", BrandName: "Crestor"},
		{GenericName: "simvastatin", BrandName: "Zocor"},
		{GenericName: "pravastatin sodium", BrandName: "Pravachol"},
		{GenericName: "lovastatin", BrandName: "Mevacor"},
		{GenericName: "fluvastatin sodium", BrandName: "Lescol"},
	}
}

func newDrugsByClassRouter(h *DrugsByClassHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/v1/drugs/classes/drugs", h.HandleDrugsByClass)
	return r
}

func doDrugsByClassRequest(router http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// AC-009: Returns drugs belonging to a class.
func TestHandleDrugsByClass_AC009_ReturnsDrugsInClass(t *testing.T) {
	svc := &mockDataService{drugsByClass: testDrugsInClass()}
	h := NewDrugsByClassHandler(svc)
	router := newDrugsByClassRouter(h)
	rr := doDrugsByClassRequest(router, "/v1/drugs/classes/drugs?class=HMG-CoA+Reductase+Inhibitors")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", rr.Code, rr.Body.String())
	}

	var resp model.PaginatedResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("data is not an array: %T", resp.Data)
	}
	if len(data) == 0 {
		t.Error("expected non-empty data array")
	}
	if resp.Pagination.Total != len(testDrugsInClass()) {
		t.Errorf("pagination.total = %d, want %d", resp.Pagination.Total, len(testDrugsInClass()))
	}
}

// AC-010: Missing class param returns 400.
func TestHandleDrugsByClass_AC010_MissingClassParam(t *testing.T) {
	svc := &mockDataService{drugsByClass: testDrugsInClass()}
	h := NewDrugsByClassHandler(svc)
	router := newDrugsByClassRouter(h)
	rr := doDrugsByClassRequest(router, "/v1/drugs/classes/drugs")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400. body: %s", rr.Code, rr.Body.String())
	}

	var resp model.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Error == "" {
		t.Error("expected non-empty error code")
	}
}

// AC-010 continued: empty class param also returns 400.
func TestHandleDrugsByClass_AC010_EmptyClassParam(t *testing.T) {
	svc := &mockDataService{drugsByClass: testDrugsInClass()}
	h := NewDrugsByClassHandler(svc)
	router := newDrugsByClassRouter(h)
	rr := doDrugsByClassRequest(router, "/v1/drugs/classes/drugs?class=")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400. body: %s", rr.Code, rr.Body.String())
	}
}

// AC-011: Each entry has generic_name and brand_name fields.
func TestHandleDrugsByClass_AC011_EntryHasGenericAndBrandName(t *testing.T) {
	svc := &mockDataService{drugsByClass: testDrugsInClass()}
	h := NewDrugsByClassHandler(svc)
	router := newDrugsByClassRouter(h)
	rr := doDrugsByClassRequest(router, "/v1/drugs/classes/drugs?class=HMG-CoA+Reductase+Inhibitors")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.PaginatedResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	data := resp.Data.([]interface{})

	for i, item := range data {
		entry := item.(map[string]interface{})
		if _, ok := entry["generic_name"]; !ok {
			t.Errorf("entry %d missing 'generic_name' field", i)
		}
		if _, ok := entry["brand_name"]; !ok {
			t.Errorf("entry %d missing 'brand_name' field", i)
		}
	}
}

// AC-012: Default page=1, limit=100, max limit=500.
func TestHandleDrugsByClass_AC012_DefaultPagination(t *testing.T) {
	svc := &mockDataService{drugsByClass: testDrugsInClass()}
	h := NewDrugsByClassHandler(svc)
	router := newDrugsByClassRouter(h)
	rr := doDrugsByClassRequest(router, "/v1/drugs/classes/drugs?class=HMG-CoA+Reductase+Inhibitors")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.PaginatedResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)

	if resp.Pagination.Page != 1 {
		t.Errorf("pagination.page = %d, want 1", resp.Pagination.Page)
	}
	if resp.Pagination.Limit != 100 {
		t.Errorf("pagination.limit = %d, want 100", resp.Pagination.Limit)
	}
}

func TestHandleDrugsByClass_AC012_LimitClampedToMax500(t *testing.T) {
	svc := &mockDataService{drugsByClass: testDrugsInClass()}
	h := NewDrugsByClassHandler(svc)
	router := newDrugsByClassRouter(h)
	rr := doDrugsByClassRequest(router, "/v1/drugs/classes/drugs?class=HMG-CoA+Reductase+Inhibitors&limit=1000")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.PaginatedResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)

	if resp.Pagination.Limit != 500 {
		t.Errorf("pagination.limit = %d, want 500 (clamped)", resp.Pagination.Limit)
	}
}

// AC-017: Pagination metadata present.
func TestHandleDrugsByClass_AC017_PaginationMetadata(t *testing.T) {
	svc := &mockDataService{drugsByClass: testDrugsInClass()}
	h := NewDrugsByClassHandler(svc)
	router := newDrugsByClassRouter(h)
	rr := doDrugsByClassRequest(router, "/v1/drugs/classes/drugs?class=Statins&limit=2")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var raw map[string]json.RawMessage
	_ = json.Unmarshal(rr.Body.Bytes(), &raw)

	if _, ok := raw["pagination"]; !ok {
		t.Fatal("response missing 'pagination' field")
	}
	if _, ok := raw["data"]; !ok {
		t.Fatal("response missing 'data' field")
	}

	var pagination model.Pagination
	_ = json.Unmarshal(raw["pagination"], &pagination)

	if pagination.Total != len(testDrugsInClass()) {
		t.Errorf("total = %d, want %d", pagination.Total, len(testDrugsInClass()))
	}
	// 6 items / limit 2 = 3 total pages
	if pagination.TotalPages != 3 {
		t.Errorf("total_pages = %d, want 3", pagination.TotalPages)
	}
}

// AC-020: Upstream error returns 502.
func TestHandleDrugsByClass_AC020_UpstreamErrorReturns502(t *testing.T) {
	svc := &mockDataService{err: client.ErrUpstream}
	h := NewDrugsByClassHandler(svc)
	router := newDrugsByClassRouter(h)
	rr := doDrugsByClassRequest(router, "/v1/drugs/classes/drugs?class=Statins")

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502. body: %s", rr.Code, rr.Body.String())
	}

	var resp model.ErrorResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Error != "upstream_error" {
		t.Errorf("Error = %q, want %q", resp.Error, "upstream_error")
	}
}

// AC-022: Unknown class returns empty data (not 404).
func TestHandleDrugsByClass_AC022_UnknownClassReturnsEmptyData(t *testing.T) {
	// Service returns empty slice for unknown class (not an error)
	svc := &mockDataService{drugsByClass: []model.DrugInClassEntry{}}
	h := NewDrugsByClassHandler(svc)
	router := newDrugsByClassRouter(h)
	rr := doDrugsByClassRequest(router, "/v1/drugs/classes/drugs?class=NonExistentClass")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (empty data, not 404). body: %s", rr.Code, rr.Body.String())
	}

	var resp model.PaginatedResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)

	data := resp.Data.([]interface{})
	if len(data) != 0 {
		t.Errorf("expected empty data for unknown class, got %d items", len(data))
	}
	if resp.Pagination.Total != 0 {
		t.Errorf("total = %d, want 0", resp.Pagination.Total)
	}
}

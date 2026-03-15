package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/finish06/drug-gate/internal/model"
	"github.com/go-chi/chi/v5"
)

// testDrugClasses returns a fixed set of drug class entries for testing.
func testDrugClasses() []model.DrugClassEntry {
	return []model.DrugClassEntry{
		{Name: "HMG-CoA Reductase Inhibitors", Type: "epc"},
		{Name: "Proton Pump Inhibitors", Type: "epc"},
		{Name: "Selective Serotonin Reuptake Inhibitors", Type: "epc"},
		{Name: "Cyclooxygenase Inhibitors", Type: "moa"},
		{Name: "Dopamine Agonists", Type: "moa"},
	}
}

func newDrugClassesRouter(h *DrugClassesHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/v1/drugs/classes", h.HandleDrugClasses)
	return r
}

func doDrugClassesRequest(router http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// AC-005: Returns paginated list of drug classes.
func TestHandleDrugClasses_AC005_ReturnsPaginatedList(t *testing.T) {
	svc := &mockDataService{drugClasses: testDrugClasses()}
	h := NewDrugClassesHandler(svc)
	router := newDrugClassesRouter(h)
	rr := doDrugClassesRequest(router, "/v1/drugs/classes")

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
	// Default filter is EPC — 3 out of 5 test entries are EPC
	if resp.Pagination.Total != 3 {
		t.Errorf("pagination.total = %d, want 3 (default EPC filter)", resp.Pagination.Total)
	}
}

// AC-006: type param filters by class type (default: epc).
func TestHandleDrugClasses_AC006_DefaultTypeFilterEPC(t *testing.T) {
	svc := &mockDataService{drugClasses: testDrugClasses()}
	h := NewDrugClassesHandler(svc)
	router := newDrugClassesRouter(h)

	// No type param — default should be epc
	rr := doDrugClassesRequest(router, "/v1/drugs/classes")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.PaginatedResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	data := resp.Data.([]interface{})

	for _, item := range data {
		entry := item.(map[string]interface{})
		if entry["type"] != "epc" {
			t.Errorf("expected default type=epc, got %v", entry["type"])
		}
	}

	// testDrugClasses has 3 EPC entries
	if len(data) != 3 {
		t.Errorf("expected 3 epc entries with default filter, got %d", len(data))
	}
}

func TestHandleDrugClasses_AC006_TypeFilterMoA(t *testing.T) {
	svc := &mockDataService{drugClasses: testDrugClasses()}
	h := NewDrugClassesHandler(svc)
	router := newDrugClassesRouter(h)
	rr := doDrugClassesRequest(router, "/v1/drugs/classes?type=moa")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.PaginatedResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	data := resp.Data.([]interface{})

	for _, item := range data {
		entry := item.(map[string]interface{})
		if entry["type"] != "moa" {
			t.Errorf("expected type=moa, got %v", entry["type"])
		}
	}

	// testDrugClasses has 2 MoA entries
	if len(data) != 2 {
		t.Errorf("expected 2 moa entries, got %d", len(data))
	}
}

// AC-007: Each entry has name and type fields.
func TestHandleDrugClasses_AC007_EntryHasNameAndType(t *testing.T) {
	svc := &mockDataService{drugClasses: testDrugClasses()}
	h := NewDrugClassesHandler(svc)
	router := newDrugClassesRouter(h)

	// Use type=moa to also verify non-default types have the right shape
	rr := doDrugClassesRequest(router, "/v1/drugs/classes?type=moa")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.PaginatedResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	data := resp.Data.([]interface{})

	for i, item := range data {
		entry := item.(map[string]interface{})
		if _, ok := entry["name"]; !ok {
			t.Errorf("entry %d missing 'name' field", i)
		}
		if _, ok := entry["type"]; !ok {
			t.Errorf("entry %d missing 'type' field", i)
		}
	}
}

// AC-008: Default page=1, limit=50, max limit=100.
func TestHandleDrugClasses_AC008_DefaultPagination(t *testing.T) {
	svc := &mockDataService{drugClasses: testDrugClasses()}
	h := NewDrugClassesHandler(svc)
	router := newDrugClassesRouter(h)

	// Request all types to get all entries for pagination check
	rr := doDrugClassesRequest(router, "/v1/drugs/classes?type=moa")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.PaginatedResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)

	if resp.Pagination.Page != 1 {
		t.Errorf("pagination.page = %d, want 1", resp.Pagination.Page)
	}
	if resp.Pagination.Limit != 50 {
		t.Errorf("pagination.limit = %d, want 50", resp.Pagination.Limit)
	}
}

func TestHandleDrugClasses_AC008_LimitClampedToMax(t *testing.T) {
	svc := &mockDataService{drugClasses: testDrugClasses()}
	h := NewDrugClassesHandler(svc)
	router := newDrugClassesRouter(h)
	rr := doDrugClassesRequest(router, "/v1/drugs/classes?limit=200")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.PaginatedResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)

	if resp.Pagination.Limit != 100 {
		t.Errorf("pagination.limit = %d, want 100 (clamped)", resp.Pagination.Limit)
	}
}

// AC-017: Pagination metadata present.
func TestHandleDrugClasses_AC017_PaginationMetadata(t *testing.T) {
	svc := &mockDataService{drugClasses: testDrugClasses()}
	h := NewDrugClassesHandler(svc)
	router := newDrugClassesRouter(h)
	rr := doDrugClassesRequest(router, "/v1/drugs/classes?limit=2")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var raw map[string]json.RawMessage
	_ = json.Unmarshal(rr.Body.Bytes(), &raw)

	if _, ok := raw["pagination"]; !ok {
		t.Fatal("response missing 'pagination' field")
	}

	var pagination model.Pagination
	_ = json.Unmarshal(raw["pagination"], &pagination)

	if pagination.TotalPages < 1 {
		t.Errorf("total_pages = %d, want >= 1", pagination.TotalPages)
	}
	if pagination.Total < 1 {
		t.Errorf("total = %d, want >= 1", pagination.Total)
	}
}

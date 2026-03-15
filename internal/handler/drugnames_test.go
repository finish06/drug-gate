package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/finish06/drug-gate/internal/model"
	"github.com/go-chi/chi/v5"
)

// mockDataService implements DataService for testing drug listing handlers.
type mockDataService struct {
	drugNames    []model.DrugNameEntry
	drugClasses  []model.DrugClassEntry
	drugsByClass []model.DrugInClassEntry
	err          error
}

func (m *mockDataService) GetDrugNames(ctx context.Context) ([]model.DrugNameEntry, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.drugNames, nil
}

func (m *mockDataService) GetDrugClasses(ctx context.Context) ([]model.DrugClassEntry, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.drugClasses, nil
}

func (m *mockDataService) GetDrugsByClass(ctx context.Context, className string) ([]model.DrugInClassEntry, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.drugsByClass, nil
}

// testDrugNames returns a fixed set of drug name entries for testing.
func testDrugNames() []model.DrugNameEntry {
	return []model.DrugNameEntry{
		{Name: "Lipitor", Type: "brand"},
		{Name: "atorvastatin calcium", Type: "generic"},
		{Name: "Metformin", Type: "generic"},
		{Name: "Januvia", Type: "brand"},
		{Name: "sitagliptin", Type: "generic"},
		{Name: "Advil", Type: "brand"},
		{Name: "ibuprofen", Type: "generic"},
		{Name: "Tylenol", Type: "brand"},
	}
}

func newDrugNamesRouter(h *DrugNamesHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/v1/drugs/names", h.HandleDrugNames)
	return r
}

func doDrugNamesRequest(router http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// AC-001: Returns paginated list of drug names (200 with data + pagination).
func TestHandleDrugNames_AC001_ReturnsPaginatedList(t *testing.T) {
	svc := &mockDataService{drugNames: testDrugNames()}
	h := NewDrugNamesHandler(svc)
	router := newDrugNamesRouter(h)
	rr := doDrugNamesRequest(router, "/v1/drugs/names")

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
	if resp.Pagination.Total != len(testDrugNames()) {
		t.Errorf("pagination.total = %d, want %d", resp.Pagination.Total, len(testDrugNames()))
	}
}

// AC-002: q param filters case-insensitively (substring match).
func TestHandleDrugNames_AC002_QueryFiltersCaseInsensitive(t *testing.T) {
	svc := &mockDataService{drugNames: testDrugNames()}
	h := NewDrugNamesHandler(svc)
	router := newDrugNamesRouter(h)

	// Search for "lipitor" should match "Lipitor" (case-insensitive)
	rr := doDrugNamesRequest(router, "/v1/drugs/names?q=lipitor")

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
	if len(data) != 1 {
		t.Errorf("expected 1 result for q=lipitor, got %d", len(data))
	}

	// Verify the match is Lipitor
	entry := data[0].(map[string]interface{})
	if entry["name"] != "Lipitor" {
		t.Errorf("expected Lipitor, got %v", entry["name"])
	}
}

// AC-002 continued: uppercase query should also match.
func TestHandleDrugNames_AC002_QueryFiltersCaseInsensitiveUppercase(t *testing.T) {
	svc := &mockDataService{drugNames: testDrugNames()}
	h := NewDrugNamesHandler(svc)
	router := newDrugNamesRouter(h)

	rr := doDrugNamesRequest(router, "/v1/drugs/names?q=METFORMIN")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.PaginatedResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	data := resp.Data.([]interface{})
	if len(data) != 1 {
		t.Errorf("expected 1 result for q=METFORMIN, got %d", len(data))
	}
}

// AC-003: Each entry has name and type fields.
func TestHandleDrugNames_AC003_EntryHasNameAndType(t *testing.T) {
	svc := &mockDataService{drugNames: testDrugNames()}
	h := NewDrugNamesHandler(svc)
	router := newDrugNamesRouter(h)
	rr := doDrugNamesRequest(router, "/v1/drugs/names")

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

// AC-004: Default page=1, limit=50.
func TestHandleDrugNames_AC004_DefaultPagination(t *testing.T) {
	svc := &mockDataService{drugNames: testDrugNames()}
	h := NewDrugNamesHandler(svc)
	router := newDrugNamesRouter(h)
	rr := doDrugNamesRequest(router, "/v1/drugs/names")

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

// AC-004: Explicit page and limit params.
func TestHandleDrugNames_AC004_ExplicitPageAndLimit(t *testing.T) {
	svc := &mockDataService{drugNames: testDrugNames()}
	h := NewDrugNamesHandler(svc)
	router := newDrugNamesRouter(h)
	rr := doDrugNamesRequest(router, "/v1/drugs/names?page=2&limit=3")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.PaginatedResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)

	if resp.Pagination.Page != 2 {
		t.Errorf("pagination.page = %d, want 2", resp.Pagination.Page)
	}
	if resp.Pagination.Limit != 3 {
		t.Errorf("pagination.limit = %d, want 3", resp.Pagination.Limit)
	}

	data := resp.Data.([]interface{})
	// With 8 items, limit=3, page=2 should have 3 items (items 4-6)
	if len(data) != 3 {
		t.Errorf("expected 3 items on page 2 with limit 3, got %d", len(data))
	}
}

// AC-017: Response includes pagination metadata.
func TestHandleDrugNames_AC017_PaginationMetadata(t *testing.T) {
	svc := &mockDataService{drugNames: testDrugNames()}
	h := NewDrugNamesHandler(svc)
	router := newDrugNamesRouter(h)
	rr := doDrugNamesRequest(router, "/v1/drugs/names?limit=3")

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

	if pagination.Total != len(testDrugNames()) {
		t.Errorf("total = %d, want %d", pagination.Total, len(testDrugNames()))
	}
	// 8 items / limit 3 = 3 total pages (ceil)
	if pagination.TotalPages != 3 {
		t.Errorf("total_pages = %d, want 3", pagination.TotalPages)
	}
}

// AC-018: Limit above max clamped to 100.
func TestHandleDrugNames_AC018_LimitClampedToMax(t *testing.T) {
	svc := &mockDataService{drugNames: testDrugNames()}
	h := NewDrugNamesHandler(svc)
	router := newDrugNamesRouter(h)
	rr := doDrugNamesRequest(router, "/v1/drugs/names?limit=500")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.PaginatedResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)

	if resp.Pagination.Limit != 100 {
		t.Errorf("pagination.limit = %d, want 100 (clamped)", resp.Pagination.Limit)
	}
}

// AC-019: Page beyond total returns empty data with correct metadata.
func TestHandleDrugNames_AC019_PageBeyondTotal(t *testing.T) {
	svc := &mockDataService{drugNames: testDrugNames()}
	h := NewDrugNamesHandler(svc)
	router := newDrugNamesRouter(h)
	rr := doDrugNamesRequest(router, "/v1/drugs/names?page=999")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (empty page, not error)", rr.Code)
	}

	var resp model.PaginatedResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)

	data := resp.Data.([]interface{})
	if len(data) != 0 {
		t.Errorf("expected empty data for page beyond total, got %d items", len(data))
	}
	if resp.Pagination.Total != len(testDrugNames()) {
		t.Errorf("total = %d, want %d", resp.Pagination.Total, len(testDrugNames()))
	}
	if resp.Pagination.Page != 999 {
		t.Errorf("page = %d, want 999", resp.Pagination.Page)
	}
}

// AC-021: type param filters by generic/brand/all.
func TestHandleDrugNames_AC021_TypeFilterGeneric(t *testing.T) {
	svc := &mockDataService{drugNames: testDrugNames()}
	h := NewDrugNamesHandler(svc)
	router := newDrugNamesRouter(h)
	rr := doDrugNamesRequest(router, "/v1/drugs/names?type=generic")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.PaginatedResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	data := resp.Data.([]interface{})

	for _, item := range data {
		entry := item.(map[string]interface{})
		if entry["type"] != "generic" {
			t.Errorf("expected type=generic, got %v", entry["type"])
		}
	}

	// testDrugNames has 4 generic entries
	if len(data) != 4 {
		t.Errorf("expected 4 generic entries, got %d", len(data))
	}
}

func TestHandleDrugNames_AC021_TypeFilterBrand(t *testing.T) {
	svc := &mockDataService{drugNames: testDrugNames()}
	h := NewDrugNamesHandler(svc)
	router := newDrugNamesRouter(h)
	rr := doDrugNamesRequest(router, "/v1/drugs/names?type=brand")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.PaginatedResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	data := resp.Data.([]interface{})

	for _, item := range data {
		entry := item.(map[string]interface{})
		if entry["type"] != "brand" {
			t.Errorf("expected type=brand, got %v", entry["type"])
		}
	}

	// testDrugNames has 4 brand entries
	if len(data) != 4 {
		t.Errorf("expected 4 brand entries, got %d", len(data))
	}
}

func TestHandleDrugNames_AC021_TypeFilterAll(t *testing.T) {
	svc := &mockDataService{drugNames: testDrugNames()}
	h := NewDrugNamesHandler(svc)
	router := newDrugNamesRouter(h)

	// type=all or no type param should return everything
	rr := doDrugNamesRequest(router, "/v1/drugs/names?type=all")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp model.PaginatedResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	data := resp.Data.([]interface{})

	if len(data) != len(testDrugNames()) {
		t.Errorf("expected %d entries for type=all, got %d", len(testDrugNames()), len(data))
	}
}

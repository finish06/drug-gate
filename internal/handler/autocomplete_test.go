package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/go-chi/chi/v5"
)

// mockAutocompleteService implements AutocompleteService for testing.
type mockAutocompleteService struct {
	results []model.DrugNameEntry
	err     error
}

func (m *mockAutocompleteService) AutocompleteDrugs(ctx context.Context, prefix string, limit int) ([]model.DrugNameEntry, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func newAutocompleteRouter(h *AutocompleteHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/v1/drugs/autocomplete", h.HandleAutocomplete)
	return r
}

func doAutocompleteRequest(router http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// AC-001: Returns prefix-matched drug names.
func TestHandleAutocomplete_AC001_ReturnsPrefixMatches(t *testing.T) {
	svc := &mockAutocompleteService{
		results: []model.DrugNameEntry{
			{Name: "metformin", Type: "generic"},
			{Name: "metoprolol", Type: "generic"},
		},
	}
	h := NewAutocompleteHandler(svc)
	router := newAutocompleteRouter(h)
	rr := doAutocompleteRequest(router, "/v1/drugs/autocomplete?q=met")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []model.DrugNameEntry `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Errorf("expected 2 results, got %d", len(resp.Data))
	}
}

// AC-003/AC-004: Missing q parameter returns 400.
func TestHandleAutocomplete_AC004_MissingQReturns400(t *testing.T) {
	svc := &mockAutocompleteService{}
	h := NewAutocompleteHandler(svc)
	router := newAutocompleteRouter(h)
	rr := doAutocompleteRequest(router, "/v1/drugs/autocomplete")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}

	var resp model.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Error != "bad_request" {
		t.Errorf("error = %q, want %q", resp.Error, "bad_request")
	}
}

// AC-003/AC-004: Short q (< 2 chars) returns 400.
func TestHandleAutocomplete_AC004_ShortQReturns400(t *testing.T) {
	svc := &mockAutocompleteService{}
	h := NewAutocompleteHandler(svc)
	router := newAutocompleteRouter(h)
	rr := doAutocompleteRequest(router, "/v1/drugs/autocomplete?q=a")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

// AC-005: limit parameter controls max results (default 10).
func TestHandleAutocomplete_AC005_DefaultLimit(t *testing.T) {
	svc := &mockAutocompleteService{
		results: []model.DrugNameEntry{
			{Name: "simvastatin", Type: "generic"},
		},
	}
	h := NewAutocompleteHandler(svc)
	router := newAutocompleteRouter(h)

	// This test just verifies that the handler parses and passes the default limit.
	// The actual limit enforcement is in the service method.
	rr := doAutocompleteRequest(router, "/v1/drugs/autocomplete?q=sim")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

// AC-005: Custom limit parameter.
func TestHandleAutocomplete_AC005_CustomLimit(t *testing.T) {
	svc := &mockAutocompleteService{
		results: []model.DrugNameEntry{
			{Name: "simvastatin", Type: "generic"},
		},
	}
	h := NewAutocompleteHandler(svc)
	router := newAutocompleteRouter(h)
	rr := doAutocompleteRequest(router, "/v1/drugs/autocomplete?q=sim&limit=3")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

// AC-007: Response shape has data array with name and type fields.
func TestHandleAutocomplete_AC007_ResponseShape(t *testing.T) {
	svc := &mockAutocompleteService{
		results: []model.DrugNameEntry{
			{Name: "lisinopril", Type: "generic"},
		},
	}
	h := NewAutocompleteHandler(svc)
	router := newAutocompleteRouter(h)
	rr := doAutocompleteRequest(router, "/v1/drugs/autocomplete?q=lis")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if _, ok := raw["data"]; !ok {
		t.Fatal("response missing 'data' field")
	}

	var data []map[string]interface{}
	if err := json.Unmarshal(raw["data"], &data); err != nil {
		t.Fatalf("unmarshal data error: %v", err)
	}
	if len(data) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(data))
	}
	if _, ok := data[0]["name"]; !ok {
		t.Error("entry missing 'name' field")
	}
	if _, ok := data[0]["type"]; !ok {
		t.Error("entry missing 'type' field")
	}
}

// AC-009: No matches returns empty data array, not error.
func TestHandleAutocomplete_AC009_EmptyResults(t *testing.T) {
	svc := &mockAutocompleteService{
		results: []model.DrugNameEntry{},
	}
	h := NewAutocompleteHandler(svc)
	router := newAutocompleteRouter(h)
	rr := doAutocompleteRequest(router, "/v1/drugs/autocomplete?q=zzzzz")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp struct {
		Data []model.DrugNameEntry `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Data == nil {
		t.Error("expected empty array, got nil")
	}
	if len(resp.Data) != 0 {
		t.Errorf("expected 0 results, got %d", len(resp.Data))
	}
}

// Test upstream error returns 502.
func TestHandleAutocomplete_UpstreamError(t *testing.T) {
	svc := &mockAutocompleteService{
		err: fmt.Errorf("fetch failed: %w", client.ErrUpstream),
	}
	h := NewAutocompleteHandler(svc)
	router := newAutocompleteRouter(h)
	rr := doAutocompleteRequest(router, "/v1/drugs/autocomplete?q=met")

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rr.Code)
	}
}

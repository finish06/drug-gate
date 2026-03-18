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

// mockSPLService implements SPLDataService for testing.
type mockSPLService struct {
	searchEntries []model.SPLEntry
	searchTotal   int
	searchErr     error
	detail        *model.SPLDetail
	detailErr     error
	interactions  *model.SPLDetail
	interErr      error
	resolvedName  string
	resolveErr    error
}

func (m *mockSPLService) SearchSPLs(_ context.Context, _ string, _, _ int) ([]model.SPLEntry, int, error) {
	return m.searchEntries, m.searchTotal, m.searchErr
}

func (m *mockSPLService) GetSPLDetail(_ context.Context, _ string) (*model.SPLDetail, error) {
	return m.detail, m.detailErr
}

func (m *mockSPLService) GetInteractionsForDrug(_ context.Context, _ string) (*model.SPLDetail, error) {
	return m.interactions, m.interErr
}

func (m *mockSPLService) ResolveDrugNameFromNDC(_ context.Context, _ string) (string, error) {
	return m.resolvedName, m.resolveErr
}

func TestSPLHandler_SearchSPLs_Success(t *testing.T) {
	svc := &mockSPLService{
		searchEntries: []model.SPLEntry{
			{Title: "LIPITOR [PFIZER]", SetID: "abc-123", PublishedDate: "May 02, 2024", SPLVersion: 42},
		},
		searchTotal: 1,
	}
	h := NewSPLHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/spls?name=lipitor", nil)
	w := httptest.NewRecorder()
	h.HandleSearchSPLs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp model.PaginatedResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.Pagination.Total != 1 {
		t.Errorf("total = %d, want 1", resp.Pagination.Total)
	}
}

func TestSPLHandler_SearchSPLs_MissingName(t *testing.T) {
	h := NewSPLHandler(&mockSPLService{})

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/spls", nil)
	w := httptest.NewRecorder()
	h.HandleSearchSPLs(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestSPLHandler_SearchSPLs_UpstreamError(t *testing.T) {
	svc := &mockSPLService{
		searchErr: client.ErrUpstream,
	}
	h := NewSPLHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/spls?name=lipitor", nil)
	w := httptest.NewRecorder()
	h.HandleSearchSPLs(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", w.Code)
	}
}

func TestSPLHandler_Detail_Success(t *testing.T) {
	svc := &mockSPLService{
		detail: &model.SPLDetail{
			Title: "LIPITOR [PFIZER]",
			SetID: "abc-123",
			Interactions: []model.InteractionSection{
				{Title: "7 DRUG INTERACTIONS", Text: "Summary text."},
			},
		},
	}
	h := NewSPLHandler(svc)

	r := chi.NewRouter()
	r.Get("/v1/drugs/spls/{setid}", h.HandleSPLDetail)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/spls/abc-123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var detail model.SPLDetail
	_ = json.NewDecoder(w.Body).Decode(&detail)
	if detail.SetID != "abc-123" {
		t.Errorf("SetID = %q, want %q", detail.SetID, "abc-123")
	}
	if len(detail.Interactions) != 1 {
		t.Errorf("interactions = %d, want 1", len(detail.Interactions))
	}
}

func TestSPLHandler_Detail_NotFound(t *testing.T) {
	svc := &mockSPLService{detail: nil}
	h := NewSPLHandler(svc)

	r := chi.NewRouter()
	r.Get("/v1/drugs/spls/{setid}", h.HandleSPLDetail)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/spls/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestSPLHandler_Detail_UpstreamError(t *testing.T) {
	svc := &mockSPLService{detailErr: client.ErrUpstream}
	h := NewSPLHandler(svc)

	r := chi.NewRouter()
	r.Get("/v1/drugs/spls/{setid}", h.HandleSPLDetail)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/spls/abc-123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", w.Code)
	}
}

func TestSPLHandler_DrugInfo_ByName(t *testing.T) {
	svc := &mockSPLService{
		interactions: &model.SPLDetail{
			Title: "WARFARIN [REMEDYREPACK]",
			SetID: "war-123",
			Interactions: []model.InteractionSection{
				{Title: "7 DRUG INTERACTIONS", Text: "Bleeding risk."},
			},
		},
	}
	h := NewSPLHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/info?name=warfarin", nil)
	w := httptest.NewRecorder()
	h.HandleDrugInfo(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp model.DrugInfoResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.DrugName != "warfarin" {
		t.Errorf("DrugName = %q, want %q", resp.DrugName, "warfarin")
	}
	if resp.InputType != "name" {
		t.Errorf("InputType = %q, want %q", resp.InputType, "name")
	}
	if resp.SPL == nil {
		t.Fatal("SPL should not be nil")
	}
	if len(resp.Interactions) != 1 {
		t.Errorf("interactions = %d, want 1", len(resp.Interactions))
	}
}

func TestSPLHandler_DrugInfo_ByNDC(t *testing.T) {
	svc := &mockSPLService{
		resolvedName: "atorvastatin calcium",
		interactions: &model.SPLDetail{
			Title: "LIPITOR [PFIZER]",
			SetID: "abc-123",
			Interactions: []model.InteractionSection{
				{Title: "7 DRUG INTERACTIONS", Text: "CYP3A4 interactions."},
			},
		},
	}
	h := NewSPLHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/info?ndc=0071-0155-23", nil)
	w := httptest.NewRecorder()
	h.HandleDrugInfo(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp model.DrugInfoResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.DrugName != "atorvastatin calcium" {
		t.Errorf("DrugName = %q, want %q", resp.DrugName, "atorvastatin calcium")
	}
	if resp.InputType != "ndc" {
		t.Errorf("InputType = %q, want %q", resp.InputType, "ndc")
	}
}

func TestSPLHandler_DrugInfo_MissingParams(t *testing.T) {
	h := NewSPLHandler(&mockSPLService{})

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/info", nil)
	w := httptest.NewRecorder()
	h.HandleDrugInfo(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestSPLHandler_DrugInfo_NDCNotFound(t *testing.T) {
	svc := &mockSPLService{
		resolveErr: errors.New("no drug found for NDC 9999-9999-99"),
	}
	h := NewSPLHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/info?ndc=9999-9999-99", nil)
	w := httptest.NewRecorder()
	h.HandleDrugInfo(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestSPLHandler_DrugInfo_NoSPL(t *testing.T) {
	svc := &mockSPLService{
		interactions: nil,
	}
	h := NewSPLHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/info?name=obscuredrug", nil)
	w := httptest.NewRecorder()
	h.HandleDrugInfo(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp model.DrugInfoResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.SPL != nil {
		t.Errorf("SPL should be nil for drug with no SPL")
	}
	if len(resp.Interactions) != 0 {
		t.Errorf("interactions should be empty, got %d", len(resp.Interactions))
	}
}

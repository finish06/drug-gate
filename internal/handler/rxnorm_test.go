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

type mockRxNormService struct {
	searchResult  *model.RxNormSearchResult
	searchErr     error
	ndcResult     *model.RxNormNDCResponse
	ndcErr        error
	genericResult *model.RxNormGenericResponse
	genericErr    error
	relatedResult *model.RxNormRelatedResponse
	relatedErr    error
	profileResult *model.RxNormProfile
	profileErr    error
}

func (m *mockRxNormService) Search(_ context.Context, _ string) (*model.RxNormSearchResult, error) {
	return m.searchResult, m.searchErr
}
func (m *mockRxNormService) GetNDCs(_ context.Context, _ string) (*model.RxNormNDCResponse, error) {
	return m.ndcResult, m.ndcErr
}
func (m *mockRxNormService) GetGenerics(_ context.Context, _ string) (*model.RxNormGenericResponse, error) {
	return m.genericResult, m.genericErr
}
func (m *mockRxNormService) GetRelated(_ context.Context, _ string) (*model.RxNormRelatedResponse, error) {
	return m.relatedResult, m.relatedErr
}
func (m *mockRxNormService) GetProfile(_ context.Context, _ string) (*model.RxNormProfile, error) {
	return m.profileResult, m.profileErr
}

func rxnormRouter(h *RxNormHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/v1/drugs/rxnorm/search", h.HandleSearch)
	r.Get("/v1/drugs/rxnorm/profile", h.HandleProfile)
	r.Get("/v1/drugs/rxnorm/{rxcui}/ndcs", h.HandleNDCs)
	r.Get("/v1/drugs/rxnorm/{rxcui}/generics", h.HandleGenerics)
	r.Get("/v1/drugs/rxnorm/{rxcui}/related", h.HandleRelated)
	return r
}

func TestRxNormHandler_Search_HappyPath(t *testing.T) {
	svc := &mockRxNormService{
		searchResult: &model.RxNormSearchResult{
			Query:       "lipitor",
			Candidates:  []model.RxNormCandidate{{RxCUI: "153165", Name: "atorvastatin calcium", Score: 100}},
			Suggestions: []string{},
		},
	}
	h := NewRxNormHandler(svc)
	r := rxnormRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/rxnorm/search?name=lipitor", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var result model.RxNormSearchResult
	_ = json.NewDecoder(w.Body).Decode(&result)
	if result.Query != "lipitor" {
		t.Errorf("Query = %q, want %q", result.Query, "lipitor")
	}
	if len(result.Candidates) != 1 {
		t.Errorf("got %d candidates, want 1", len(result.Candidates))
	}
}

func TestRxNormHandler_Search_MissingName(t *testing.T) {
	h := NewRxNormHandler(&mockRxNormService{})
	r := rxnormRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/rxnorm/search", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestRxNormHandler_Search_NotFound(t *testing.T) {
	svc := &mockRxNormService{
		searchResult: &model.RxNormSearchResult{
			Query:       "notadrug",
			Candidates:  []model.RxNormCandidate{},
			Suggestions: []string{},
		},
	}
	h := NewRxNormHandler(svc)
	r := rxnormRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/rxnorm/search?name=notadrug", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestRxNormHandler_Search_WithSuggestions(t *testing.T) {
	svc := &mockRxNormService{
		searchResult: &model.RxNormSearchResult{
			Query:       "liiptor",
			Candidates:  []model.RxNormCandidate{},
			Suggestions: []string{"lipitor"},
		},
	}
	h := NewRxNormHandler(svc)
	r := rxnormRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/rxnorm/search?name=liiptor", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Has suggestions, so should return 200 not 404
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (has suggestions)", w.Code)
	}
}

func TestRxNormHandler_Search_UpstreamError(t *testing.T) {
	svc := &mockRxNormService{searchErr: client.ErrUpstream}
	h := NewRxNormHandler(svc)
	r := rxnormRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/rxnorm/search?name=test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

func TestRxNormHandler_NDCs_HappyPath(t *testing.T) {
	svc := &mockRxNormService{
		ndcResult: &model.RxNormNDCResponse{RxCUI: "153165", NDCs: []string{"0071-0155-23"}},
	}
	h := NewRxNormHandler(svc)
	r := rxnormRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/rxnorm/153165/ndcs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var result model.RxNormNDCResponse
	_ = json.NewDecoder(w.Body).Decode(&result)
	if len(result.NDCs) != 1 {
		t.Errorf("got %d NDCs, want 1", len(result.NDCs))
	}
}

func TestRxNormHandler_Generics_HappyPath(t *testing.T) {
	svc := &mockRxNormService{
		genericResult: &model.RxNormGenericResponse{
			RxCUI:    "153165",
			Generics: []model.RxNormConcept{{RxCUI: "83367", Name: "atorvastatin"}},
		},
	}
	h := NewRxNormHandler(svc)
	r := rxnormRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/rxnorm/153165/generics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRxNormHandler_Related_HappyPath(t *testing.T) {
	svc := &mockRxNormService{
		relatedResult: &model.RxNormRelatedResponse{
			RxCUI:         "153165",
			Ingredients:   []model.RxNormConcept{{RxCUI: "83367", Name: "atorvastatin"}},
			BrandNames:    []model.RxNormConcept{},
			DoseForms:     []model.RxNormConcept{},
			ClinicalDrugs: []model.RxNormConcept{},
			BrandedDrugs:  []model.RxNormConcept{},
		},
	}
	h := NewRxNormHandler(svc)
	r := rxnormRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/rxnorm/153165/related", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRxNormHandler_Profile_HappyPath(t *testing.T) {
	svc := &mockRxNormService{
		profileResult: &model.RxNormProfile{
			Query: "lipitor",
			RxCUI: "153165",
			Name:  "atorvastatin calcium",
		},
	}
	h := NewRxNormHandler(svc)
	r := rxnormRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/rxnorm/profile?name=lipitor", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var result model.RxNormProfile
	_ = json.NewDecoder(w.Body).Decode(&result)
	if result.RxCUI != "153165" {
		t.Errorf("RxCUI = %q, want %q", result.RxCUI, "153165")
	}
}

func TestRxNormHandler_NDCs_UpstreamError(t *testing.T) {
	svc := &mockRxNormService{ndcErr: client.ErrUpstream}
	h := NewRxNormHandler(svc)
	r := rxnormRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/rxnorm/153165/ndcs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

func TestRxNormHandler_NDCs_EmptyResult(t *testing.T) {
	svc := &mockRxNormService{
		ndcResult: &model.RxNormNDCResponse{RxCUI: "999999", NDCs: []string{}},
	}
	h := NewRxNormHandler(svc)
	r := rxnormRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/rxnorm/999999/ndcs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (empty result, not 404)", w.Code)
	}
}

func TestRxNormHandler_Generics_UpstreamError(t *testing.T) {
	svc := &mockRxNormService{genericErr: client.ErrUpstream}
	h := NewRxNormHandler(svc)
	r := rxnormRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/rxnorm/153165/generics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

func TestRxNormHandler_Generics_EmptyResult(t *testing.T) {
	svc := &mockRxNormService{
		genericResult: &model.RxNormGenericResponse{RxCUI: "999999", Generics: []model.RxNormConcept{}},
	}
	h := NewRxNormHandler(svc)
	r := rxnormRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/rxnorm/999999/generics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (empty result, not 404)", w.Code)
	}
}

func TestRxNormHandler_Related_UpstreamError(t *testing.T) {
	svc := &mockRxNormService{relatedErr: client.ErrUpstream}
	h := NewRxNormHandler(svc)
	r := rxnormRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/rxnorm/153165/related", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

func TestRxNormHandler_Related_NotFound(t *testing.T) {
	svc := &mockRxNormService{
		relatedResult: &model.RxNormRelatedResponse{
			RxCUI:         "999999",
			Ingredients:   []model.RxNormConcept{},
			BrandNames:    []model.RxNormConcept{},
			DoseForms:     []model.RxNormConcept{},
			ClinicalDrugs: []model.RxNormConcept{},
			BrandedDrugs:  []model.RxNormConcept{},
		},
	}
	h := NewRxNormHandler(svc)
	r := rxnormRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/rxnorm/999999/related", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestRxNormHandler_Profile_UpstreamError(t *testing.T) {
	svc := &mockRxNormService{profileErr: client.ErrUpstream}
	h := NewRxNormHandler(svc)
	r := rxnormRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/rxnorm/profile?name=test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

func TestRxNormHandler_Profile_MissingName(t *testing.T) {
	h := NewRxNormHandler(&mockRxNormService{})
	r := rxnormRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/rxnorm/profile", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestRxNormHandler_Profile_NotFound(t *testing.T) {
	svc := &mockRxNormService{profileResult: nil}
	h := NewRxNormHandler(svc)
	r := rxnormRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/rxnorm/profile?name=notadrug", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

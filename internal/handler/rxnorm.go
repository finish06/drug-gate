package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/go-chi/chi/v5"
)

// RxNormDataService defines the interface the RxNorm handlers need.
type RxNormDataService interface {
	Search(ctx context.Context, name string) (*model.RxNormSearchResult, error)
	GetNDCs(ctx context.Context, rxcui string) (*model.RxNormNDCResponse, error)
	GetGenerics(ctx context.Context, rxcui string) (*model.RxNormGenericResponse, error)
	GetRelated(ctx context.Context, rxcui string) (*model.RxNormRelatedResponse, error)
	GetProfile(ctx context.Context, name string) (*model.RxNormProfile, error)
}

// RxNormHandler handles all RxNorm endpoints.
type RxNormHandler struct {
	svc RxNormDataService
}

// NewRxNormHandler creates a handler with the given RxNorm service.
func NewRxNormHandler(svc RxNormDataService) *RxNormHandler {
	return &RxNormHandler{svc: svc}
}

// HandleSearch handles GET /v1/drugs/rxnorm/search?name={name}.
//
// @Summary      Search drugs by name (RxNorm)
// @Description  Performs an approximate-match search via the RxNorm API and returns up to 5 candidates ranked by score, each with an RxCUI. When no exact matches are found, spelling suggestions are included to help correct the query. Use this as the entry point for RxNorm workflows before calling NDC, generic, or related endpoints.
// @Tags         rxnorm
// @Produce      json
// @Param        name  query  string  true  "Drug name to search for"  example(lisinopril)
// @Success      200  {object}  model.RxNormSearchResult
// @Failure      400  {object}  model.ErrorResponse  "Missing name parameter"
// @Failure      401  {object}  model.ErrorResponse  "Missing or invalid API key"
// @Failure      404  {object}  model.ErrorResponse  "No drugs found"
// @Failure      429  {object}  model.ErrorResponse  "Rate limit exceeded"
// @Failure      502  {object}  model.ErrorResponse  "Upstream service unavailable"
// @Security     ApiKeyAuth
// @Router       /v1/drugs/rxnorm/search [get]
func (h *RxNormHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "name query parameter is required")
		return
	}

	result, err := h.svc.Search(r.Context(), name)
	if err != nil {
		if errors.Is(err, client.ErrUpstream) {
			writeError(w, http.StatusBadGateway, "upstream_error", "Unable to reach drug data service")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Unexpected error")
		return
	}

	if len(result.Candidates) == 0 && len(result.Suggestions) == 0 {
		writeError(w, http.StatusNotFound, "not_found", "No drugs found for name '"+name+"'")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// HandleNDCs handles GET /v1/drugs/rxnorm/{rxcui}/ndcs.
//
// @Summary      Get NDCs for an RxCUI
// @Description  Returns all National Drug Code (NDC) identifiers associated with the given RxNorm concept. Use this to map an RxCUI obtained from the search endpoint to specific packaged products. Results are cached with a long TTL since NDC-to-RxCUI mappings change infrequently.
// @Tags         rxnorm
// @Produce      json
// @Param        rxcui  path  string  true  "RxNorm concept unique identifier"  example(314076)
// @Success      200  {object}  model.RxNormNDCResponse
// @Failure      401  {object}  model.ErrorResponse  "Missing or invalid API key"
// @Failure      404  {object}  model.ErrorResponse  "RxCUI not found"
// @Failure      429  {object}  model.ErrorResponse  "Rate limit exceeded"
// @Failure      502  {object}  model.ErrorResponse  "Upstream service unavailable"
// @Security     ApiKeyAuth
// @Router       /v1/drugs/rxnorm/{rxcui}/ndcs [get]
func (h *RxNormHandler) HandleNDCs(w http.ResponseWriter, r *http.Request) {
	rxcui := strings.TrimSpace(chi.URLParam(r, "rxcui"))
	if rxcui == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "rxcui path parameter is required")
		return
	}

	result, err := h.svc.GetNDCs(r.Context(), rxcui)
	if err != nil {
		if errors.Is(err, client.ErrUpstream) {
			writeError(w, http.StatusBadGateway, "upstream_error", "Unable to reach drug data service")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Unexpected error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// HandleGenerics handles GET /v1/drugs/rxnorm/{rxcui}/generics.
//
// @Summary      Get generic products for an RxCUI
// @Description  Returns generic product information for the given RxNorm concept identifier, including ingredient names and dose form groups. Use this endpoint to find generic equivalents of a branded drug after resolving its RxCUI via the search endpoint.
// @Tags         rxnorm
// @Produce      json
// @Param        rxcui  path  string  true  "RxNorm concept unique identifier"  example(314076)
// @Success      200  {object}  model.RxNormGenericResponse
// @Failure      401  {object}  model.ErrorResponse  "Missing or invalid API key"
// @Failure      404  {object}  model.ErrorResponse  "RxCUI not found"
// @Failure      429  {object}  model.ErrorResponse  "Rate limit exceeded"
// @Failure      502  {object}  model.ErrorResponse  "Upstream service unavailable"
// @Security     ApiKeyAuth
// @Router       /v1/drugs/rxnorm/{rxcui}/generics [get]
func (h *RxNormHandler) HandleGenerics(w http.ResponseWriter, r *http.Request) {
	rxcui := strings.TrimSpace(chi.URLParam(r, "rxcui"))
	if rxcui == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "rxcui path parameter is required")
		return
	}

	result, err := h.svc.GetGenerics(r.Context(), rxcui)
	if err != nil {
		if errors.Is(err, client.ErrUpstream) {
			writeError(w, http.StatusBadGateway, "upstream_error", "Unable to reach drug data service")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Unexpected error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// HandleRelated handles GET /v1/drugs/rxnorm/{rxcui}/related.
//
// @Summary      Get related concepts for an RxCUI
// @Description  Returns all related RxNorm concepts grouped by relationship type: ingredients, brand names, dose forms, clinical drugs, and branded drugs. Returns 404 only when all groups are empty, indicating an unknown RxCUI. Individual empty groups within a valid response are normal.
// @Tags         rxnorm
// @Produce      json
// @Param        rxcui  path  string  true  "RxNorm concept unique identifier"  example(314076)
// @Success      200  {object}  model.RxNormRelatedResponse
// @Failure      401  {object}  model.ErrorResponse  "Missing or invalid API key"
// @Failure      404  {object}  model.ErrorResponse  "RxCUI not found or has no related concepts"
// @Failure      429  {object}  model.ErrorResponse  "Rate limit exceeded"
// @Failure      502  {object}  model.ErrorResponse  "Upstream service unavailable"
// @Security     ApiKeyAuth
// @Router       /v1/drugs/rxnorm/{rxcui}/related [get]
func (h *RxNormHandler) HandleRelated(w http.ResponseWriter, r *http.Request) {
	rxcui := strings.TrimSpace(chi.URLParam(r, "rxcui"))
	if rxcui == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "rxcui path parameter is required")
		return
	}

	result, err := h.svc.GetRelated(r.Context(), rxcui)
	if err != nil {
		if errors.Is(err, client.ErrUpstream) {
			writeError(w, http.StatusBadGateway, "upstream_error", "Unable to reach drug data service")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Unexpected error")
		return
	}

	// 404 only when ALL groups are empty (unknown RxCUI). Individual empty groups are valid.
	if len(result.Ingredients) == 0 && len(result.BrandNames) == 0 && len(result.DoseForms) == 0 &&
		len(result.ClinicalDrugs) == 0 && len(result.BrandedDrugs) == 0 {
		writeError(w, http.StatusNotFound, "not_found", "No data found for RxCUI '"+rxcui+"'")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// HandleProfile handles GET /v1/drugs/rxnorm/profile?name={name}.
//
// @Summary      Get unified drug profile
// @Description  Resolves a drug name via RxNorm approximate match, then assembles NDCs, generic equivalents, and all related concepts into a single unified response. This is a convenience endpoint that combines the results of search, NDCs, generics, and related into one call. Use this when you need a complete drug profile from a single request.
// @Tags         rxnorm
// @Produce      json
// @Param        name  query  string  true  "Drug name (generic or brand)"  example(metformin)
// @Success      200  {object}  model.RxNormProfile
// @Failure      400  {object}  model.ErrorResponse  "Missing name parameter"
// @Failure      401  {object}  model.ErrorResponse  "Missing or invalid API key"
// @Failure      404  {object}  model.ErrorResponse  "Drug not found"
// @Failure      429  {object}  model.ErrorResponse  "Rate limit exceeded"
// @Failure      502  {object}  model.ErrorResponse  "Upstream service unavailable"
// @Security     ApiKeyAuth
// @Router       /v1/drugs/rxnorm/profile [get]
func (h *RxNormHandler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "name query parameter is required")
		return
	}

	profile, err := h.svc.GetProfile(r.Context(), name)
	if err != nil {
		if errors.Is(err, client.ErrUpstream) {
			writeError(w, http.StatusBadGateway, "upstream_error", "Unable to reach drug data service")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Unexpected error")
		return
	}

	if profile == nil {
		writeError(w, http.StatusNotFound, "not_found", "No drugs found for name '"+name+"'")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(profile)
}

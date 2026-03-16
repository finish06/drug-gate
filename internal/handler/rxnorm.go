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
// @Description  Approximate match search via RxNorm. Returns up to 5 candidates ranked by score. Includes spelling suggestions when no matches are found.
// @Tags         rxnorm
// @Produce      json
// @Param        name  query  string  true  "Drug name to search for"
// @Success      200  {object}  model.RxNormSearchResult
// @Failure      400  {object}  model.ErrorResponse  "Missing name parameter"
// @Failure      404  {object}  model.ErrorResponse  "No drugs found"
// @Failure      502  {object}  model.ErrorResponse  "Upstream service error"
// @Router       /v1/drugs/rxnorm/search [get]
func (h *RxNormHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "name query parameter is required")
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
// @Description  Returns NDC codes associated with the given RxNorm concept identifier.
// @Tags         rxnorm
// @Produce      json
// @Param        rxcui  path  string  true  "RxNorm concept unique identifier"
// @Success      200  {object}  model.RxNormNDCResponse
// @Failure      404  {object}  model.ErrorResponse  "RxCUI not found"
// @Failure      502  {object}  model.ErrorResponse  "Upstream service error"
// @Router       /v1/drugs/rxnorm/{rxcui}/ndcs [get]
func (h *RxNormHandler) HandleNDCs(w http.ResponseWriter, r *http.Request) {
	rxcui := chi.URLParam(r, "rxcui")

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
// @Description  Returns generic product information for the given RxNorm concept identifier.
// @Tags         rxnorm
// @Produce      json
// @Param        rxcui  path  string  true  "RxNorm concept unique identifier"
// @Success      200  {object}  model.RxNormGenericResponse
// @Failure      404  {object}  model.ErrorResponse  "RxCUI not found"
// @Failure      502  {object}  model.ErrorResponse  "Upstream service error"
// @Router       /v1/drugs/rxnorm/{rxcui}/generics [get]
func (h *RxNormHandler) HandleGenerics(w http.ResponseWriter, r *http.Request) {
	rxcui := chi.URLParam(r, "rxcui")

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
// @Description  Returns all related concepts grouped by type (ingredients, brand names, dose forms, clinical drugs, branded drugs).
// @Tags         rxnorm
// @Produce      json
// @Param        rxcui  path  string  true  "RxNorm concept unique identifier"
// @Success      200  {object}  model.RxNormRelatedResponse
// @Failure      404  {object}  model.ErrorResponse  "RxCUI not found"
// @Failure      502  {object}  model.ErrorResponse  "Upstream service error"
// @Router       /v1/drugs/rxnorm/{rxcui}/related [get]
func (h *RxNormHandler) HandleRelated(w http.ResponseWriter, r *http.Request) {
	rxcui := chi.URLParam(r, "rxcui")

	result, err := h.svc.GetRelated(r.Context(), rxcui)
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

// HandleProfile handles GET /v1/drugs/rxnorm/profile?name={name}.
//
// @Summary      Get unified drug profile
// @Description  Resolves a drug name via approximate match, then assembles NDCs, generic equivalents, and related concepts into a single response.
// @Tags         rxnorm
// @Produce      json
// @Param        name  query  string  true  "Drug name (generic or brand)"
// @Success      200  {object}  model.RxNormProfile
// @Failure      400  {object}  model.ErrorResponse  "Missing name parameter"
// @Failure      404  {object}  model.ErrorResponse  "Drug not found"
// @Failure      502  {object}  model.ErrorResponse  "Upstream service error"
// @Router       /v1/drugs/rxnorm/profile [get]
func (h *RxNormHandler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "name query parameter is required")
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

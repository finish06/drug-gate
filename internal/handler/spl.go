package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/go-chi/chi/v5"
)

// SPLDataService defines the interface for SPL operations used by the handler.
type SPLDataService interface {
	SearchSPLs(ctx context.Context, drugName string, limit, offset int) ([]model.SPLEntry, int, error)
	GetSPLDetail(ctx context.Context, setID string) (*model.SPLDetail, error)
	GetInteractionsForDrug(ctx context.Context, drugName string) (*model.SPLDetail, error)
	ResolveDrugNameFromNDC(ctx context.Context, ndc string) (string, error)
	CheckInteractions(ctx context.Context, drugs []model.DrugIdentifier) (*model.InteractionCheckResponse, error)
}

// SPLHandler handles SPL-related requests.
type SPLHandler struct {
	svc SPLDataService
}

// NewSPLHandler creates a handler with the given SPL service.
func NewSPLHandler(svc SPLDataService) *SPLHandler {
	return &SPLHandler{svc: svc}
}

// HandleSearchSPLs handles GET /v1/drugs/spls?name={name}.
//
// @Summary      Search SPL documents
// @Description  Searches Structured Product Labels (SPLs) by drug name and returns paginated metadata from DailyMed. Each result includes the SPL title, set ID, published date, and version. Use the set ID from results to fetch full SPL detail with parsed interaction sections.
// @Tags         spl
// @Produce      json
// @Param        name   query  string  true   "Drug name to search"  example(metformin)
// @Param        page   query  int     false  "Page number (default: 1)"  example(1)
// @Param        limit  query  int     false  "Results per page (default: 20, max: 100)"  example(20)
// @Success      200  {object}  model.PaginatedResponse
// @Failure      400  {object}  model.ErrorResponse  "Missing name parameter"
// @Failure      401  {object}  model.ErrorResponse  "Missing or invalid API key"
// @Failure      429  {object}  model.ErrorResponse  "Rate limit exceeded"
// @Failure      502  {object}  model.ErrorResponse  "Upstream service unavailable"
// @Security     ApiKeyAuth
// @Router       /v1/drugs/spls [get]
func (h *SPLHandler) HandleSearchSPLs(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "Query parameter 'name' is required")
		return
	}

	p := parsePagination(r, 20, 100)
	offset := (p.Page - 1) * p.Limit

	entries, total, err := h.svc.SearchSPLs(r.Context(), name, p.Limit, offset)
	if err != nil {
		if errors.Is(err, client.ErrUpstream) {
			writeError(w, http.StatusBadGateway, "upstream_error", "Unable to reach drug data service")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Unexpected error")
		return
	}

	totalPages := total / p.Limit
	if total%p.Limit != 0 {
		totalPages++
	}

	resp := model.PaginatedResponse{
		Data: entries,
		Pagination: model.Pagination{
			Page:       p.Page,
			Limit:      p.Limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleSPLDetail handles GET /v1/drugs/spls/{setid}.
//
// @Summary      Get SPL detail with interactions
// @Description  Retrieves full SPL metadata and parsed safety sections from the SPL XML document, including Drug Interactions (Section 7), Contraindications, Warnings, and Adverse Reactions. Use this endpoint when you have a set ID from the SPL search results and need the complete label content.
// @Tags         spl
// @Produce      json
// @Param        setid  path  string  true  "SPL set ID (UUID format)"  example(2c6ca939-8494-4b3b-8930-81c6c09b0125)
// @Success      200  {object}  model.SPLDetail
// @Failure      401  {object}  model.ErrorResponse  "Missing or invalid API key"
// @Failure      404  {object}  model.ErrorResponse  "SPL not found"
// @Failure      429  {object}  model.ErrorResponse  "Rate limit exceeded"
// @Failure      502  {object}  model.ErrorResponse  "Upstream service unavailable"
// @Security     ApiKeyAuth
// @Router       /v1/drugs/spls/{setid} [get]
func (h *SPLHandler) HandleSPLDetail(w http.ResponseWriter, r *http.Request) {
	setID := chi.URLParam(r, "setid")
	if setID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "Set ID is required")
		return
	}

	detail, err := h.svc.GetSPLDetail(r.Context(), setID)
	if err != nil {
		if errors.Is(err, client.ErrUpstream) {
			writeError(w, http.StatusBadGateway, "upstream_error", "Unable to reach drug data service")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Unexpected error")
		return
	}

	if detail == nil {
		writeError(w, http.StatusNotFound, "not_found", "SPL not found for the given set ID")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(detail)
}

// HandleDrugInfo handles GET /v1/drugs/info?name={name} or ?ndc={ndc}.
//
// @Summary      Drug info card with interactions
// @Description  Look up a single drug by name or NDC and return a consolidated info card with SPL metadata, drug interactions, contraindications, warnings, and adverse reactions. When an NDC is provided, it is normalized and resolved to a drug name internally. Provide either name or ndc; if both are given, ndc takes precedence.
// @Tags         spl
// @Produce      json
// @Param        name  query  string  false  "Drug name"  example(warfarin)
// @Param        ndc   query  string  false  "NDC code (any format)"  example(00069-3150)
// @Success      200  {object}  model.DrugInfoResponse
// @Failure      400  {object}  model.ErrorResponse  "Missing name or ndc"
// @Failure      401  {object}  model.ErrorResponse  "Missing or invalid API key"
// @Failure      404  {object}  model.ErrorResponse  "NDC not found or no SPL data"
// @Failure      429  {object}  model.ErrorResponse  "Rate limit exceeded"
// @Failure      502  {object}  model.ErrorResponse  "Upstream service unavailable"
// @Security     ApiKeyAuth
// @Router       /v1/drugs/info [get]
func (h *SPLHandler) HandleDrugInfo(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	ndcParam := strings.TrimSpace(r.URL.Query().Get("ndc"))

	if name == "" && ndcParam == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "Either 'name' or 'ndc' query parameter is required")
		return
	}

	var (
		drugName  string
		inputType string
		inputVal  string
	)

	// NDC takes precedence if both provided
	if ndcParam != "" {
		inputType = "ndc"
		inputVal = ndcParam
		resolved, err := h.svc.ResolveDrugNameFromNDC(r.Context(), ndcParam)
		if err != nil {
			if errors.Is(err, client.ErrUpstream) {
				writeError(w, http.StatusBadGateway, "upstream_error", "Unable to reach drug data service")
				return
			}
			writeError(w, http.StatusNotFound, "not_found", "No drug found for the given NDC")
			return
		}
		drugName = resolved
	} else {
		inputType = "name"
		inputVal = name
		drugName = name
	}

	detail, err := h.svc.GetInteractionsForDrug(r.Context(), drugName)
	if err != nil {
		if errors.Is(err, client.ErrUpstream) {
			writeError(w, http.StatusBadGateway, "upstream_error", "Unable to reach drug data service")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Unexpected error")
		return
	}

	resp := model.DrugInfoResponse{
		DrugName:   drugName,
		InputType:  inputType,
		InputValue: inputVal,
	}

	if detail != nil {
		resp.SPL = &model.SPLSource{
			Title:         detail.Title,
			SetID:         detail.SetID,
			PublishedDate: detail.PublishedDate,
			SPLVersion:    detail.SPLVersion,
		}
		resp.Interactions = detail.Interactions
		resp.Contraindications = detail.Contraindications
		resp.Warnings = detail.Warnings
		resp.AdverseReactions = detail.AdverseReactions
	}

	// Ensure all section fields are empty slices, never null in JSON
	if resp.Interactions == nil {
		resp.Interactions = []model.InteractionSection{}
	}
	if resp.Contraindications == nil {
		resp.Contraindications = []model.InteractionSection{}
	}
	if resp.Warnings == nil {
		resp.Warnings = []model.InteractionSection{}
	}
	if resp.AdverseReactions == nil {
		resp.AdverseReactions = []model.InteractionSection{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleCheckInteractions handles POST /v1/drugs/interactions.
//
// @Summary      Check drug interactions
// @Description  Submit 2 to 10 drug identifiers (by name or NDC) and receive cross-referenced interaction warnings from FDA SPL labels. Each drug's Section 7 (Drug Interactions) is searched for mentions of the other submitted drugs. NDC identifiers are resolved to drug names automatically. Use this endpoint to check a patient's medication list for potential interactions.
// @Tags         spl
// @Accept       json
// @Produce      json
// @Param        body  body  model.InteractionCheckRequest  true  "Drug identifiers to check"
// @Success      200  {object}  model.InteractionCheckResponse
// @Failure      400  {object}  model.ErrorResponse  "Too few/many drugs or invalid input"
// @Failure      401  {object}  model.ErrorResponse  "Missing or invalid API key"
// @Failure      429  {object}  model.ErrorResponse  "Rate limit exceeded"
// @Failure      502  {object}  model.ErrorResponse  "Upstream service unavailable"
// @Security     ApiKeyAuth
// @Router       /v1/drugs/interactions [post]
func (h *SPLHandler) HandleCheckInteractions(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 1MB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req model.InteractionCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Request body must be valid JSON with a 'drugs' array")
		return
	}

	if len(req.Drugs) < 2 {
		writeError(w, http.StatusBadRequest, "bad_request", "At least 2 drugs are required")
		return
	}
	if len(req.Drugs) > 10 {
		writeError(w, http.StatusBadRequest, "bad_request", "Maximum 10 drugs per request")
		return
	}

	// Validate each drug has at least name or ndc
	for i, d := range req.Drugs {
		if d.Name == "" && d.NDC == "" {
			writeError(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("Drug at index %d must have 'name' or 'ndc'", i))
			return
		}
	}

	resp, err := h.svc.CheckInteractions(r.Context(), req.Drugs)
	if err != nil {
		if errors.Is(err, client.ErrUpstream) {
			writeError(w, http.StatusBadGateway, "upstream_error", "Unable to reach drug data service")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Unexpected error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

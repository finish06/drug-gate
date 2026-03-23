package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/model"
)

// AutocompleteService provides drug autocomplete results.
type AutocompleteService interface {
	AutocompleteDrugs(ctx context.Context, prefix string, limit int) ([]model.DrugNameEntry, error)
}

// AutocompleteHandler handles drug autocomplete requests.
type AutocompleteHandler struct {
	svc AutocompleteService
}

// NewAutocompleteHandler creates a handler with the given autocomplete service.
func NewAutocompleteHandler(svc AutocompleteService) *AutocompleteHandler {
	return &AutocompleteHandler{svc: svc}
}

// HandleAutocomplete handles GET /v1/drugs/autocomplete.
//
// @Summary      Drug name autocomplete
// @Description  Returns drug names matching the given prefix. Fast typeahead endpoint for building search UIs.
// @Tags         drugs
// @Produce      json
// @Param        q      query  string  true   "Prefix to match (min 2 chars)"
// @Param        limit  query  int     false  "Max results (default: 10, max: 50)"
// @Success      200  {object}  map[string][]model.DrugNameEntry
// @Failure      400  {object}  model.ErrorResponse  "Missing or invalid q parameter"
// @Failure      502  {object}  model.ErrorResponse  "Upstream service error"
// @Security     ApiKeyAuth
// @Router       /v1/drugs/autocomplete [get]
func (h *AutocompleteHandler) HandleAutocomplete(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 {
		writeError(w, http.StatusBadRequest, "bad_request", "q parameter is required and must be at least 2 characters")
		return
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 50 {
		limit = 50
	}

	results, err := h.svc.AutocompleteDrugs(r.Context(), q, limit)
	if err != nil {
		if errors.Is(err, client.ErrUpstream) {
			writeError(w, http.StatusBadGateway, "upstream_error", "Unable to reach drug data service")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Unexpected error")
		return
	}

	resp := map[string]interface{}{
		"data": results,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}


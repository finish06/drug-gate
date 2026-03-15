package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/model"
)

// DataService provides drug data for listing endpoints.
type DataService interface {
	GetDrugNames(ctx context.Context) ([]model.DrugNameEntry, error)
	GetDrugClasses(ctx context.Context) ([]model.DrugClassEntry, error)
	GetDrugsByClass(ctx context.Context, className string) ([]model.DrugInClassEntry, error)
}

// DrugNamesHandler handles drug names listing requests.
type DrugNamesHandler struct {
	svc DataService
}

// NewDrugNamesHandler creates a handler with the given data service.
func NewDrugNamesHandler(svc DataService) *DrugNamesHandler {
	return &DrugNamesHandler{svc: svc}
}

// HandleDrugNames handles GET /v1/drugs/names.
//
// @Summary      List drug names
// @Description  Returns a paginated list of drug names from the DailyMed dataset. Supports case-insensitive substring search and type filtering (generic/brand).
// @Tags         drugs
// @Produce      json
// @Param        q      query  string  false  "Search substring filter (case-insensitive)"
// @Param        type   query  string  false  "Filter by type: generic, brand, or all"  Enums(generic, brand, all)
// @Param        page   query  int     false  "Page number (default: 1)"
// @Param        limit  query  int     false  "Results per page (default: 50, max: 100)"
// @Success      200  {object}  model.PaginatedResponse
// @Failure      502  {object}  model.ErrorResponse  "Upstream service error"
// @Router       /v1/drugs/names [get]
func (h *DrugNamesHandler) HandleDrugNames(w http.ResponseWriter, r *http.Request) {
	names, err := h.svc.GetDrugNames(r.Context())
	if err != nil {
		if errors.Is(err, client.ErrUpstream) {
			writeError(w, http.StatusBadGateway, "upstream_error", "Unable to reach drug data service")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Unexpected error")
		return
	}

	// Apply type filter
	typeFilter := strings.ToLower(r.URL.Query().Get("type"))
	if typeFilter != "" && typeFilter != "all" {
		filtered := make([]model.DrugNameEntry, 0)
		for _, n := range names {
			if n.Type == typeFilter {
				filtered = append(filtered, n)
			}
		}
		names = filtered
	}

	// Apply search filter
	q := strings.ToLower(r.URL.Query().Get("q"))
	if q != "" {
		filtered := make([]model.DrugNameEntry, 0)
		for _, n := range names {
			if strings.Contains(strings.ToLower(n.Name), q) {
				filtered = append(filtered, n)
			}
		}
		names = filtered
	}

	// Paginate
	p := parsePagination(r, 50, 100)
	page, pagination := paginateSlice(names, p)

	resp := model.PaginatedResponse{
		Data:       page,
		Pagination: pagination,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

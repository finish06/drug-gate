package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/model"
)

// DrugClassesHandler handles drug classes listing requests.
type DrugClassesHandler struct {
	svc DataService
}

// NewDrugClassesHandler creates a handler with the given data service.
func NewDrugClassesHandler(svc DataService) *DrugClassesHandler {
	return &DrugClassesHandler{svc: svc}
}

// HandleDrugClasses handles GET /v1/drugs/classes.
//
// @Summary      List drug classes
// @Description  Returns a paginated list of pharmacological drug classes from DailyMed. Defaults to EPC (Established Pharmacologic Class) type. Supports filtering by class type: epc (pharmacologic class), moa (mechanism of action), pe (physiologic effect), cs (chemical structure), or all. Use this endpoint to discover available drug classes for browsing or filtering.
// @Tags         drugs
// @Produce      json
// @Param        type   query  string  false  "Filter by class type (default: epc)"  Enums(epc, moa, pe, cs, all)
// @Param        page   query  int     false  "Page number (default: 1)"  example(1)
// @Param        limit  query  int     false  "Results per page (default: 50, max: 100)"  example(50)
// @Success      200  {object}  model.PaginatedResponse
// @Failure      401  {object}  model.ErrorResponse  "Missing or invalid API key"
// @Failure      429  {object}  model.ErrorResponse  "Rate limit exceeded"
// @Failure      502  {object}  model.ErrorResponse  "Upstream service unavailable"
// @Security     ApiKeyAuth
// @Router       /v1/drugs/classes [get]
func (h *DrugClassesHandler) HandleDrugClasses(w http.ResponseWriter, r *http.Request) {
	classes, err := h.svc.GetDrugClasses(r.Context())
	if err != nil {
		if errors.Is(err, client.ErrUpstream) {
			writeError(w, http.StatusBadGateway, "upstream_error", "Unable to reach drug data service")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Unexpected error")
		return
	}

	// Apply type filter (default: epc)
	typeFilter := strings.ToLower(r.URL.Query().Get("type"))
	if typeFilter == "" {
		typeFilter = "epc"
	}
	if typeFilter != "all" {
		filtered := make([]model.DrugClassEntry, 0, len(classes)/4)
		for _, c := range classes {
			if c.Type == typeFilter {
				filtered = append(filtered, c)
			}
		}
		classes = filtered
	}

	// Paginate
	p := parsePagination(r, 50, 100)
	page, pagination := paginateSlice(classes, p)

	resp := model.PaginatedResponse{
		Data:       page,
		Pagination: pagination,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

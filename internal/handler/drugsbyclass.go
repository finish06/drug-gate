package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/model"
)

// DrugsByClassHandler handles drugs-by-class listing requests.
type DrugsByClassHandler struct {
	svc DataService
}

// NewDrugsByClassHandler creates a handler with the given data service.
func NewDrugsByClassHandler(svc DataService) *DrugsByClassHandler {
	return &DrugsByClassHandler{svc: svc}
}

// HandleDrugsByClass handles GET /v1/drugs/classes/drugs.
//
// @Summary      List drugs in a class
// @Description  Returns a paginated list of drugs belonging to a specific pharmacological class, resolved via FDA NDC data. Returns empty data (not 404) for unknown classes.
// @Tags         drugs
// @Produce      json
// @Param        class  query  string  true   "Pharmacological class name (e.g. HMG-CoA Reductase Inhibitor)"
// @Param        page   query  int     false  "Page number (default: 1)"
// @Param        limit  query  int     false  "Results per page (default: 100, max: 500)"
// @Success      200  {object}  model.PaginatedResponse
// @Failure      400  {object}  model.ErrorResponse  "Missing or empty class parameter"
// @Failure      502  {object}  model.ErrorResponse  "Upstream service error"
// @Security     ApiKeyAuth
// @Router       /v1/drugs/classes/drugs [get]
func (h *DrugsByClassHandler) HandleDrugsByClass(w http.ResponseWriter, r *http.Request) {
	className := strings.TrimSpace(r.URL.Query().Get("class"))
	if className == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "class query parameter is required")
		return
	}

	drugs, err := h.svc.GetDrugsByClass(r.Context(), className)
	if err != nil {
		if errors.Is(err, client.ErrUpstream) {
			writeError(w, http.StatusBadGateway, "upstream_error", "Unable to reach drug data service")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Unexpected error")
		return
	}

	if drugs == nil {
		drugs = []model.DrugInClassEntry{}
	}

	// Paginate
	p := parsePagination(r, 100, 500)
	page, pagination := paginateSlice(drugs, p)

	resp := model.PaginatedResponse{
		Data:       page,
		Pagination: pagination,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/finish06/drug-gate/internal/pharma"
)

// DrugClassClient defines the interface for looking up drugs by name.
type DrugClassClient interface {
	LookupByGenericName(ctx context.Context, name string) ([]client.DrugResult, error)
	LookupByBrandName(ctx context.Context, name string) ([]client.DrugResult, error)
}

// DrugClassHandler handles drug class lookup requests.
type DrugClassHandler struct {
	client DrugClassClient
}

// NewDrugClassHandler creates a handler with the given drug class client.
func NewDrugClassHandler(c DrugClassClient) *DrugClassHandler {
	return &DrugClassHandler{client: c}
}

// HandleDrugClassLookup handles GET /v1/drugs/class?name={drug_name}.
//
// @Summary      Look up drug class by name
// @Description  Looks up a drug by generic or brand name and returns its pharmacological classes (EPC, MoA, PE, CS), deduplicated brand names, and generic name. Tries generic name first and falls back to brand name search. Use this endpoint to discover what class a drug belongs to.
// @Tags         drugs
// @Produce      json
// @Param        name  query  string  true  "Drug name (generic or brand)"  example(simvastatin)
// @Success      200  {object}  model.DrugClassResponse
// @Failure      400  {object}  model.ErrorResponse  "Missing or empty name parameter"
// @Failure      401  {object}  model.ErrorResponse  "Missing or invalid API key"
// @Failure      404  {object}  model.ErrorResponse  "Drug not found"
// @Failure      429  {object}  model.ErrorResponse  "Rate limit exceeded"
// @Failure      502  {object}  model.ErrorResponse  "Upstream service unavailable"
// @Security     ApiKeyAuth
// @Router       /v1/drugs/class [get]
func (h *DrugClassHandler) HandleDrugClassLookup(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "name query parameter is required")
		return
	}

	// Try generic name first
	results, err := h.client.LookupByGenericName(r.Context(), name)
	if err != nil {
		if errors.Is(err, client.ErrUpstream) {
			writeError(w, http.StatusBadGateway, "upstream_error", "Unable to reach drug data service")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Unexpected error")
		return
	}

	// If no generic results, fallback to brand name
	if len(results) == 0 {
		results, err = h.client.LookupByBrandName(r.Context(), name)
		if err != nil {
			if errors.Is(err, client.ErrUpstream) {
				writeError(w, http.StatusBadGateway, "upstream_error", "Unable to reach drug data service")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "Unexpected error")
			return
		}
	}

	if len(results) == 0 {
		writeError(w, http.StatusNotFound, "not_found", "No drug found for name '"+name+"'")
		return
	}

	// Build response from aggregated results
	genericName := results[0].GenericName

	// Collect and deduplicate brand names
	var brandNamesList []string
	for _, r := range results {
		if r.BrandName != "" {
			brandNamesList = append(brandNamesList, r.BrandName)
		}
	}
	brandNames := pharma.DeduplicateBrandNames(brandNamesList)

	// Parse pharm classes from first result (consistent across products of same generic)
	var classes []model.DrugClass
	if len(results[0].PharmClass) > 0 {
		parsed := pharma.ParsePharmClasses(results[0].PharmClass)
		classes = make([]model.DrugClass, len(parsed))
		for i, pc := range parsed {
			classes[i] = model.DrugClass{Name: pc.Name, Type: pc.Type}
		}
	}
	if classes == nil {
		classes = []model.DrugClass{}
	}

	resp := model.DrugClassResponse{
		QueryName:   name,
		GenericName: genericName,
		BrandNames:  brandNames,
		Classes:     classes,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

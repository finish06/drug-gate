package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/finish06/drug-gate/internal/ndc"
	"github.com/go-chi/chi/v5"
)

// DrugHandler handles drug lookup requests.
type DrugHandler struct {
	client client.DrugClient
}

// NewDrugHandler creates a handler with the given drug client.
func NewDrugHandler(c client.DrugClient) *DrugHandler {
	return &DrugHandler{client: c}
}

// HandleNDCLookup handles GET /v1/drugs/ndc/{ndc}.
func (h *DrugHandler) HandleNDCLookup(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "ndc")

	parsed, err := ndc.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_ndc", err.Error())
		return
	}

	// Try exact match first
	result, err := h.client.LookupByNDC(r.Context(), parsed.String())
	if err != nil {
		if errors.Is(err, client.ErrUpstream) {
			writeError(w, http.StatusBadGateway, "upstream_error", "Unable to reach drug data service")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Unexpected error")
		return
	}

	// If not found and fallback is available, try padded 5-4 form
	if result == nil {
		fallback := parsed.FallbackNDC()
		if fallback != "" {
			result, err = h.client.LookupByNDC(r.Context(), fallback)
			if err != nil {
				if errors.Is(err, client.ErrUpstream) {
					writeError(w, http.StatusBadGateway, "upstream_error", "Unable to reach drug data service")
					return
				}
				writeError(w, http.StatusInternalServerError, "internal_error", "Unexpected error")
				return
			}
		}
	}

	if result == nil {
		writeError(w, http.StatusNotFound, "not_found", "No drug found for NDC "+parsed.String())
		return
	}

	classes := result.PharmClass
	if classes == nil {
		classes = []string{}
	}

	resp := model.DrugDetailResponse{
		NDC:         result.ProductNDC,
		Name:        result.BrandName,
		GenericName: result.GenericName,
		Classes:     classes,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func writeError(w http.ResponseWriter, status int, errCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(model.ErrorResponse{
		Error:   errCode,
		Message: message,
	})
}

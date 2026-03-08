package handler

import (
	"encoding/json"
	"net/http"

	"github.com/finish06/drug-gate/internal/version"
)

// HealthCheck returns service health status.
//
// @Summary      Health check
// @Description  Returns service health status and build version.
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]string  "status and version"
// @Router       /health [get]
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"version": version.Version,
	})
}

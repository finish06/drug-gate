package handler

import (
	"encoding/json"
	"net/http"

	"github.com/finish06/drug-gate/internal/version"
)

// HealthCheck returns service health status.
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"version": version.Version,
	})
}

package handler

import (
	"encoding/json"
	"net/http"
)

// Livez handles GET /livez — a liveness probe.
//
// Unlike /health (which checks Redis and the upstream and returns 503 when a
// critical dependency is down), /livez reports only that the process is up and
// serving. It has no dependencies, so it never returns a non-200 for an external
// outage — using it as the Kubernetes liveness probe prevents a Redis outage
// from crash-looping pods. Readiness stays on /health.
//
// @Summary      Liveness check
// @Description  Dependency-free liveness probe. Returns 200 as long as the process is up and able to serve requests; it does NOT check Redis or upstream. Use as the Kubernetes liveness probe (readiness should use /health).
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]string  "alive"
// @Router       /livez [get]
func Livez(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

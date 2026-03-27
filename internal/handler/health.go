package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/version"
	"github.com/redis/go-redis/v9"
)

// CircuitChecker can report circuit breaker state.
type CircuitChecker interface {
	IsOpen() bool
}

// HealthResponse is the structured health check response.
type HealthResponse struct {
	Status  string            `json:"status"`
	Version string            `json:"version"`
	Deps    map[string]string `json:"dependencies,omitempty"`
}

// HealthHandler provides health checks with dependency verification.
type HealthHandler struct {
	rdb         *redis.Client
	upstreamURL string
	breaker     *client.CircuitBreaker
}

// NewHealthHandler creates a health handler with dependency checks.
func NewHealthHandler(rdb *redis.Client, upstreamURL string, breaker ...*client.CircuitBreaker) *HealthHandler {
	h := &HealthHandler{rdb: rdb, upstreamURL: upstreamURL}
	if len(breaker) > 0 {
		h.breaker = breaker[0]
	}
	return h
}

// Handle returns service health status with dependency checks.
//
// @Summary      Health check
// @Description  Returns service health status, build version, and dependency health for Redis, the upstream cash-drugs API, and the circuit breaker. Returns 200 when all dependencies are healthy and 503 when any dependency is degraded. Use this endpoint for load balancer health probes and monitoring dashboards.
// @Tags         system
// @Produce      json
// @Success      200  {object}  HealthResponse  "All dependencies healthy"
// @Success      503  {object}  HealthResponse  "One or more dependencies unhealthy"
// @Router       /health [get]
func (h *HealthHandler) Handle(w http.ResponseWriter, r *http.Request) {
	deps := make(map[string]string)
	healthy := true

	// Check Redis
	if h.rdb != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := h.rdb.Ping(ctx).Err(); err != nil {
			deps["redis"] = "unhealthy"
			healthy = false
		} else {
			deps["redis"] = "ok"
		}
	}

	// Check upstream
	if h.upstreamURL != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, h.upstreamURL+"/health", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			deps["upstream"] = "unhealthy"
			healthy = false
		} else {
			deps["upstream"] = "ok"
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
	}

	// Check circuit breaker
	if h.breaker != nil && h.breaker.IsOpen() {
		deps["circuit_breaker"] = "open"
		healthy = false
	} else if h.breaker != nil {
		deps["circuit_breaker"] = "closed"
	}

	status := "ok"
	httpStatus := http.StatusOK
	if !healthy {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(HealthResponse{
		Status:  status,
		Version: version.Version,
		Deps:    deps,
	})
}

// HealthCheck is the legacy health check function (no dependency verification).
// Kept for backward compatibility — use NewHealthHandler for full checks.
//
// @Summary      Health check (simple)
// @Description  Returns a minimal health response with service status and build version. This legacy endpoint does not check dependencies. Prefer the full health check endpoint for production monitoring.
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]string  "status and version"
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"version": version.Version,
	})
}

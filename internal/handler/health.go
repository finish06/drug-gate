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

// HealthResponse is the standard health response shape shared across services
// in the stack (rx-dag, cash-drugs, drug-gate, drugs-quiz BFF).
type HealthResponse struct {
	Status       string           `json:"status"`
	Version      string           `json:"version"`
	Uptime       string           `json:"uptime"`
	StartTime    time.Time        `json:"start_time"`
	Dependencies []DependencyInfo `json:"dependencies"`
}

// DependencyInfo is a single dependency check result.
type DependencyInfo struct {
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	LatencyMs float64 `json:"latency_ms"`
	Error     string  `json:"error,omitempty"`
}

// HealthHandler provides health checks with dependency verification.
type HealthHandler struct {
	rdb         *redis.Client
	upstreamURL string
	startTime   time.Time
	breaker     *client.CircuitBreaker
}

// NewHealthHandler creates a health handler with dependency checks. startTime
// should be captured once at application boot and is reused by every probe
// to compute uptime.
func NewHealthHandler(rdb *redis.Client, upstreamURL string, startTime time.Time, breaker ...*client.CircuitBreaker) *HealthHandler {
	h := &HealthHandler{
		rdb:         rdb,
		upstreamURL: upstreamURL,
		startTime:   startTime,
	}
	if len(breaker) > 0 {
		h.breaker = breaker[0]
	}
	return h
}

// Handle returns service health status with dependency checks.
//
// @Summary      Health check
// @Description  Returns service health, build version, uptime, process start time, and per-dependency status for Redis, the upstream cash-drugs API, and the circuit breaker. Redis is a critical dependency — its failure returns HTTP 503 and status=error. Upstream or breaker issues are reported as status=degraded with HTTP 200 so load balancers keep routing traffic. Follows the cross-service health endpoint standard.
// @Tags         system
// @Produce      json
// @Success      200  {object}  HealthResponse  "ok or degraded"
// @Success      503  {object}  HealthResponse  "critical dependency down"
// @Router       /health [get]
func (h *HealthHandler) Handle(w http.ResponseWriter, _ *http.Request) {
	deps := make([]DependencyInfo, 0, 3)
	criticalFailure := false
	nonCriticalFailure := false

	if h.rdb != nil {
		d := checkRedis(h.rdb)
		if d.Status != "connected" {
			criticalFailure = true
		}
		deps = append(deps, d)
	}

	if h.upstreamURL != "" {
		d := checkUpstream(h.upstreamURL)
		if d.Status != "connected" {
			nonCriticalFailure = true
		}
		deps = append(deps, d)
	}

	if h.breaker != nil {
		d := checkBreaker(h.breaker)
		if d.Status == "open" {
			nonCriticalFailure = true
		}
		deps = append(deps, d)
	}

	status := "ok"
	httpStatus := http.StatusOK
	switch {
	case criticalFailure:
		status = "error"
		httpStatus = http.StatusServiceUnavailable
	case nonCriticalFailure:
		status = "degraded"
	}

	resp := HealthResponse{
		Status:       status,
		Version:      version.Version,
		Uptime:       time.Since(h.startTime).String(),
		StartTime:    h.startTime,
		Dependencies: deps,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(resp)
}

const depCheckTimeout = 2 * time.Second

func checkRedis(rdb *redis.Client) DependencyInfo {
	d := DependencyInfo{Name: "redis"}
	ctx, cancel := context.WithTimeout(context.Background(), depCheckTimeout)
	defer cancel()

	start := time.Now()
	err := rdb.Ping(ctx).Err()
	d.LatencyMs = msSince(start)

	if err != nil {
		d.Status = "disconnected"
		d.Error = err.Error()
		return d
	}
	d.Status = "connected"
	return d
}

func checkUpstream(url string) DependencyInfo {
	d := DependencyInfo{Name: "cash-drugs-upstream"}
	ctx, cancel := context.WithTimeout(context.Background(), depCheckTimeout)
	defer cancel()

	c := &http.Client{Timeout: depCheckTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url+"/health", nil)
	if err != nil {
		d.Status = "disconnected"
		d.Error = err.Error()
		return d
	}

	start := time.Now()
	resp, err := c.Do(req)
	d.LatencyMs = msSince(start)

	if err != nil {
		d.Status = "disconnected"
		d.Error = err.Error()
		return d
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		d.Status = "disconnected"
		d.Error = "upstream returned " + resp.Status
		return d
	}
	d.Status = "connected"
	return d
}

func checkBreaker(cb *client.CircuitBreaker) DependencyInfo {
	d := DependencyInfo{Name: "circuit_breaker"}
	if cb.IsOpen() {
		d.Status = "open"
		return d
	}
	d.Status = "closed"
	return d
}

// msSince returns elapsed time since t in milliseconds as a float (sub-ms precision).
func msSince(t time.Time) float64 {
	return float64(time.Since(t).Microseconds()) / 1000.0
}

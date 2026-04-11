package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/version"
	"github.com/redis/go-redis/v9"
)

var errForceOpen = errors.New("force open")

// findDep returns the dependency entry with the given name, or nil if absent.
func findDep(t *testing.T, deps []DependencyInfo, name string) *DependencyInfo {
	t.Helper()
	for i := range deps {
		if deps[i].Name == name {
			return &deps[i]
		}
	}
	return nil
}

// AC-001, AC-002, AC-005, AC-006, AC-008, AC-011, AC-012:
// Healthy Redis + healthy upstream + closed breaker → status=ok, 200, all deps present.
func TestHealth_AC001_OK_AllDepsHealthy(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	breaker := client.NewCircuitBreaker(10, time.Second)
	start := time.Now().UTC().Add(-5 * time.Second)
	h := NewHealthHandler(rdb, upstream.URL, start, breaker)

	rr := httptest.NewRecorder()
	h.Handle(rr, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("status = %q, want ok", resp.Status)
	}
	if resp.Version == "" {
		t.Error("version should not be empty")
	}
	if len(resp.Dependencies) != 3 {
		t.Fatalf("dependencies len = %d, want 3", len(resp.Dependencies))
	}
	for _, name := range []string{"redis", "cash-drugs-upstream", "circuit_breaker"} {
		d := findDep(t, resp.Dependencies, name)
		if d == nil {
			t.Errorf("missing dependency %q", name)
			continue
		}
		if d.LatencyMs < 0 {
			t.Errorf("%s latency_ms = %v, want >= 0", name, d.LatencyMs)
		}
	}
}

// AC-003: uptime is a parseable Go duration string.
func TestHealth_AC003_UptimeFormat(t *testing.T) {
	start := time.Now().UTC().Add(-time.Minute)
	h := NewHealthHandler(nil, "", start)

	rr := httptest.NewRecorder()
	h.Handle(rr, httptest.NewRequest(http.MethodGet, "/health", nil))

	var resp HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Uptime == "" {
		t.Fatal("uptime empty")
	}
	d, err := time.ParseDuration(resp.Uptime)
	if err != nil {
		t.Fatalf("uptime %q not parseable: %v", resp.Uptime, err)
	}
	if d < 50*time.Second {
		t.Errorf("uptime = %s, want >= 50s", d)
	}
}

// AC-004, AC-014: start_time is stable across calls and reflects the constructor param.
func TestHealth_AC004_StartTimeStable(t *testing.T) {
	start := time.Date(2026, 4, 11, 14, 0, 0, 0, time.UTC)
	h := NewHealthHandler(nil, "", start)

	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		h.Handle(rr, httptest.NewRequest(http.MethodGet, "/health", nil))

		var resp HealthResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !resp.StartTime.Equal(start) {
			t.Errorf("call %d: start_time = %s, want %s", i, resp.StartTime, start)
		}
	}
}

// AC-007, AC-010, AC-011: Redis down is a critical failure → status=error, HTTP 503,
// and the redis dependency entry has a populated error field.
func TestHealth_AC010_Error_RedisDown(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mr.Close()

	h := NewHealthHandler(rdb, "", time.Now().UTC())
	rr := httptest.NewRecorder()
	h.Handle(rr, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rr.Code)
	}

	var resp HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("status = %q, want error", resp.Status)
	}
	d := findDep(t, resp.Dependencies, "redis")
	if d == nil {
		t.Fatal("missing redis dep")
	}
	if d.Status == "connected" {
		t.Errorf("redis status = %q, want not connected", d.Status)
	}
	if d.Error == "" {
		t.Error("redis error field should be populated")
	}
}

// AC-007, AC-009, AC-011: Upstream unreachable is non-critical → status=degraded, HTTP 200,
// upstream dep entry has error populated.
func TestHealth_AC009_Degraded_UpstreamDown(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// Closed server → connection refused.
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	downstream.Close()

	h := NewHealthHandler(rdb, downstream.URL, time.Now().UTC())
	rr := httptest.NewRecorder()
	h.Handle(rr, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}

	var resp HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Status != "degraded" {
		t.Errorf("status = %q, want degraded", resp.Status)
	}
	d := findDep(t, resp.Dependencies, "cash-drugs-upstream")
	if d == nil {
		t.Fatal("missing upstream dep")
	}
	if d.Error == "" {
		t.Error("upstream error field should be populated when unreachable")
	}
}

// AC-009: Open breaker is non-critical → status=degraded, HTTP 200, breaker.status=open.
func TestHealth_AC009_Degraded_BreakerOpen(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	breaker := client.NewCircuitBreaker(1, time.Hour)
	// Force breaker open: one failing Execute with maxFailures=1 opens the circuit.
	_ = breaker.Execute(func() error { return errForceOpen })
	if !breaker.IsOpen() {
		t.Fatalf("breaker should be open after exceeding threshold")
	}

	h := NewHealthHandler(rdb, upstream.URL, time.Now().UTC(), breaker)
	rr := httptest.NewRecorder()
	h.Handle(rr, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}

	var resp HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Status != "degraded" {
		t.Errorf("status = %q, want degraded", resp.Status)
	}
	d := findDep(t, resp.Dependencies, "circuit_breaker")
	if d == nil {
		t.Fatal("missing circuit_breaker dep")
	}
	if d.Status != "open" {
		t.Errorf("circuit_breaker status = %q, want open", d.Status)
	}
}

// AC-002: Health reports the actual version value.
func TestHealth_AC002_ReportsVersion(t *testing.T) {
	original := version.Version
	version.Version = "v9.9.9"
	defer func() { version.Version = original }()

	h := NewHealthHandler(nil, "", time.Now().UTC())
	rr := httptest.NewRecorder()
	h.Handle(rr, httptest.NewRequest(http.MethodGet, "/health", nil))

	var resp HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Version != "v9.9.9" {
		t.Errorf("version = %q, want v9.9.9", resp.Version)
	}
}

// AC-011: Handler sets application/json content type.
func TestHealth_ContentType(t *testing.T) {
	h := NewHealthHandler(nil, "", time.Now().UTC())
	rr := httptest.NewRecorder()
	h.Handle(rr, httptest.NewRequest(http.MethodGet, "/health", nil))

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}
}

// AC-008: No dependencies configured (nil Redis, empty upstream, no breaker) → ok + 200.
func TestHealth_NoDepsConfigured(t *testing.T) {
	h := NewHealthHandler(nil, "", time.Now().UTC())
	rr := httptest.NewRecorder()
	h.Handle(rr, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	var resp HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("status = %q, want ok", resp.Status)
	}
}

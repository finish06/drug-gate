package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/finish06/drug-gate/internal/version"
	"github.com/redis/go-redis/v9"
)

func TestHealthCheck(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	HealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want %q", resp["status"], "ok")
	}
}

// AC-003: Health response includes version field
func TestHealthCheck_AC003_IncludesVersion(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	HealthCheck(rr, req)

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	v, ok := resp["version"]
	if !ok {
		t.Fatal("health response missing 'version' field")
	}
	if v == "" {
		t.Error("version field should not be empty")
	}
}

// AC-003: Health returns correct Content-Type
func TestHealthCheck_ContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	HealthCheck(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

// AC-002/AC-003: Health reports the actual version value
func TestHealthCheck_ReportsSetVersion(t *testing.T) {
	original := version.Version
	version.Version = "v1.2.3"
	defer func() { version.Version = original }()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	HealthCheck(rr, req)

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["version"] != "v1.2.3" {
		t.Errorf("version = %q, want %q", resp["version"], "v1.2.3")
	}
}

// OBS-1: Health handler with healthy Redis returns 200 + dependencies
func TestHealthHandler_OBS1_HealthyRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	h := NewHealthHandler(rdb, "")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	h.Handle(rr, req)

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
	if resp.Deps["redis"] != "ok" {
		t.Errorf("redis dep = %q, want ok", resp.Deps["redis"])
	}
}

// OBS-1: Health handler with down Redis returns 503 + degraded
func TestHealthHandler_OBS1_UnhealthyRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mr.Close() // kill Redis

	h := NewHealthHandler(rdb, "")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	h.Handle(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rr.Code)
	}

	var resp HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Status != "degraded" {
		t.Errorf("status = %q, want degraded", resp.Status)
	}
	if resp.Deps["redis"] != "unhealthy" {
		t.Errorf("redis dep = %q, want unhealthy", resp.Deps["redis"])
	}
}

// OBS-1: Health handler with healthy upstream
func TestHealthHandler_OBS1_HealthyUpstream(t *testing.T) {
	// Mock upstream
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	h := NewHealthHandler(nil, upstream.URL)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	h.Handle(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}

	var resp HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Deps["upstream"] != "ok" {
		t.Errorf("upstream dep = %q, want ok", resp.Deps["upstream"])
	}
}

// OBS-1: Health handler with no deps configured returns 200
func TestHealthHandler_OBS1_NoDeps(t *testing.T) {
	h := NewHealthHandler(nil, "")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	h.Handle(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

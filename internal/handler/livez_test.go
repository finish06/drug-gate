package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// AC-006: Liveness must be a lightweight, dependency-free check (NOT the
// dependency-aware /health). /livez returns 200 with a small JSON body and
// never touches Redis or any upstream — so a Redis outage can never make it
// fail and crash-loop the pod.
func TestLivez_AC006_AlwaysOK(t *testing.T) {
	rr := httptest.NewRecorder()
	Livez(rr, httptest.NewRequest(http.MethodGet, "/livez", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status field = %q, want %q", resp["status"], "ok")
	}
}

// AC-006: The handler is a dependency-free package function (no Redis/upstream
// wiring), so it returns 200 deterministically on repeated calls regardless of
// any external state.
func TestLivez_AC006_DependencyFreeAndStable(t *testing.T) {
	for i := 0; i < 3; i++ {
		rr := httptest.NewRecorder()
		Livez(rr, httptest.NewRequest(http.MethodGet, "/livez", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("call %d: status = %d, want 200", i, rr.Code)
		}
	}
}

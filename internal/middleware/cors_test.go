package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/finish06/drug-gate/internal/apikey"
)

// injectAPIKey sets an APIKey into the request context.
func injectAPIKey(r *http.Request, key *apikey.APIKey) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), APIKeyContextKey, key))
}

// dummyHandler is the inner handler that records whether it was called.
func dummyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

// AC-004: Origin-locked key + allowed origin → Access-Control-Allow-Origin set
func TestPerKeyCORS_AC004_AllowedOrigin(t *testing.T) {
	key := &apikey.APIKey{
		Key:     "test-key-1",
		AppName: "test-app",
		Origins: []string{"https://example.com", "https://other.com"},
		Active:  true,
	}

	handler := PerKeyCORS(dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req = injectAPIKey(req, key)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	got := rr.Header().Get("Access-Control-Allow-Origin")
	if got != "https://example.com" {
		t.Errorf("AC-004: expected Access-Control-Allow-Origin %q, got %q", "https://example.com", got)
	}

	if rr.Code != http.StatusOK {
		t.Errorf("AC-004: expected status 200, got %d", rr.Code)
	}
}

// AC-004: Second origin in list also matches
func TestPerKeyCORS_AC004_AllowedOriginSecondEntry(t *testing.T) {
	key := &apikey.APIKey{
		Key:     "test-key-2",
		AppName: "test-app",
		Origins: []string{"https://example.com", "https://other.com"},
		Active:  true,
	}

	handler := PerKeyCORS(dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "https://other.com")
	req = injectAPIKey(req, key)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	got := rr.Header().Get("Access-Control-Allow-Origin")
	if got != "https://other.com" {
		t.Errorf("AC-004: expected Access-Control-Allow-Origin %q, got %q", "https://other.com", got)
	}
}

// AC-005: Origin-locked key + wrong origin → no CORS header
func TestPerKeyCORS_AC005_WrongOrigin(t *testing.T) {
	key := &apikey.APIKey{
		Key:     "test-key-3",
		AppName: "test-app",
		Origins: []string{"https://allowed.com"},
		Active:  true,
	}

	handler := PerKeyCORS(dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	req = injectAPIKey(req, key)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	got := rr.Header().Get("Access-Control-Allow-Origin")
	if got != "" {
		t.Errorf("AC-005: expected no Access-Control-Allow-Origin header, got %q", got)
	}

	if rr.Code != http.StatusOK {
		t.Errorf("AC-005: expected status 200 (request proceeds), got %d", rr.Code)
	}
}

// SEC-3: Empty origins → deny cross-origin (no implicit wildcard)
func TestPerKeyCORS_SEC3_EmptyOriginsDeny(t *testing.T) {
	key := &apikey.APIKey{
		Key:     "test-key-4",
		AppName: "test-app",
		Origins: []string{},
		Active:  true,
	}

	handler := PerKeyCORS(dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "https://anywhere.com")
	req = injectAPIKey(req, key)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	got := rr.Header().Get("Access-Control-Allow-Origin")
	if got != "" {
		t.Errorf("SEC-3: empty origins should deny CORS, got %q", got)
	}
}

// SEC-3: Nil origins → deny cross-origin
func TestPerKeyCORS_SEC3_NilOriginsDeny(t *testing.T) {
	key := &apikey.APIKey{
		Key:     "test-key-5",
		AppName: "test-app",
		Origins: nil,
		Active:  true,
	}

	handler := PerKeyCORS(dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "https://anywhere.com")
	req = injectAPIKey(req, key)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	got := rr.Header().Get("Access-Control-Allow-Origin")
	if got != "" {
		t.Errorf("SEC-3: nil origins should deny CORS, got %q", got)
	}
}

// SEC-3: Explicit "*" in origins → allow all
func TestPerKeyCORS_SEC3_ExplicitWildcard(t *testing.T) {
	key := &apikey.APIKey{
		Key:     "test-key-wildcard",
		AppName: "test-app",
		Origins: []string{"*"},
		Active:  true,
	}

	handler := PerKeyCORS(dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "https://anywhere.com")
	req = injectAPIKey(req, key)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	got := rr.Header().Get("Access-Control-Allow-Origin")
	if got != "*" {
		t.Errorf("SEC-3: explicit '*' should allow all, got %q", got)
	}
}

// AC-019: OPTIONS preflight returns 204 with CORS headers
func TestPerKeyCORS_AC019_PreflightOptions(t *testing.T) {
	key := &apikey.APIKey{
		Key:     "test-key-6",
		AppName: "test-app",
		Origins: []string{"https://example.com"},
		Active:  true,
	}

	handler := PerKeyCORS(dummyHandler())

	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req = injectAPIKey(req, key)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("AC-019: expected status 204 for preflight, got %d", rr.Code)
	}

	allowOrigin := rr.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "https://example.com" {
		t.Errorf("AC-019: expected Access-Control-Allow-Origin %q, got %q", "https://example.com", allowOrigin)
	}

	allowMethods := rr.Header().Get("Access-Control-Allow-Methods")
	if allowMethods == "" {
		t.Error("AC-019: expected Access-Control-Allow-Methods header to be set")
	}

	allowHeaders := rr.Header().Get("Access-Control-Allow-Headers")
	if allowHeaders == "" {
		t.Error("AC-019: expected Access-Control-Allow-Headers header to be set")
	}
}

// SEC-3: Preflight with empty origins → deny (no implicit wildcard)
func TestPerKeyCORS_SEC3_PreflightEmptyOriginsDeny(t *testing.T) {
	key := &apikey.APIKey{
		Key:     "test-key-7",
		AppName: "test-app",
		Origins: []string{},
		Active:  true,
	}

	handler := PerKeyCORS(dummyHandler())

	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	req.Header.Set("Origin", "https://anywhere.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req = injectAPIKey(req, key)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status 204 for preflight, got %d", rr.Code)
	}

	allowOrigin := rr.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "" {
		t.Errorf("SEC-3: empty origins should deny CORS on preflight, got %q", allowOrigin)
	}
}

// AC-019+AC-005: Preflight with wrong origin → no CORS headers
func TestPerKeyCORS_AC019_PreflightWrongOrigin(t *testing.T) {
	key := &apikey.APIKey{
		Key:     "test-key-8",
		AppName: "test-app",
		Origins: []string{"https://allowed.com"},
		Active:  true,
	}

	handler := PerKeyCORS(dummyHandler())

	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req = injectAPIKey(req, key)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	allowOrigin := rr.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "" {
		t.Errorf("AC-019+AC-005: expected no Access-Control-Allow-Origin for wrong origin on preflight, got %q", allowOrigin)
	}
}

// Defensive: No APIKey in context → pass through without CORS headers
func TestPerKeyCORS_NoAPIKeyInContext(t *testing.T) {
	handler := PerKeyCORS(dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "https://example.com")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	allowOrigin := rr.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "" {
		t.Errorf("expected no CORS headers without APIKey in context, got Access-Control-Allow-Origin %q", allowOrigin)
	}

	if rr.Code != http.StatusOK {
		t.Errorf("expected request to proceed with status 200, got %d", rr.Code)
	}
}

// Non-browser request (no Origin header) → passes through
func TestPerKeyCORS_NoOriginHeader(t *testing.T) {
	key := &apikey.APIKey{
		Key:     "test-key-9",
		AppName: "test-app",
		Origins: []string{"https://example.com"},
		Active:  true,
	}

	handler := PerKeyCORS(dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req = injectAPIKey(req, key)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for request without Origin header, got %d", rr.Code)
	}
}

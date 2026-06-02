package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

// newV1Chain composes the production /v1 middleware ordering from
// cmd/server/main.go: CORSPreflight → APIKeyAuth → PerKeyCORS → handler.
// Tests use this to exercise the real request flow a browser triggers, where
// the preflight carries no X-API-Key (the browser strips it).
func newV1Chain(store apikey.Store, inner http.Handler) http.Handler {
	return CORSPreflight(APIKeyAuth(store)(PerKeyCORS(inner)))
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

// AC-021: A CORS preflight carries no X-API-Key (the browser strips it), so it
// must be answered BEFORE auth. Through the production chain, a keyless preflight
// returns 204 with the requesting origin reflected — never a 401.
func TestCORSPreflight_AC021_KeylessPreflightBypassesAuth(t *testing.T) {
	store := &mockAPIKeyStore{keys: map[string]*apikey.APIKey{}}
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	chain := newV1Chain(store, inner)

	req := httptest.NewRequest(http.MethodOptions, "/v1/drugs/ndc/00069-3150", nil)
	req.Header.Set("Origin", "https://myapp.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "x-api-key")
	// No X-API-Key header — browsers strip custom headers from preflights.

	rr := httptest.NewRecorder()
	chain.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("AC-021: expected 204 for keyless preflight, got %d", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://myapp.com" {
		t.Errorf("AC-021: expected reflected Access-Control-Allow-Origin, got %q", got)
	}
	if called {
		t.Error("AC-021: inner handler must NOT run for a preflight")
	}
}

// AC-021: The preflight must advertise X-API-Key in Allow-Headers, or the browser
// will refuse to send it on the real request.
func TestCORSPreflight_AC021_AdvertisesAPIKeyHeader(t *testing.T) {
	store := &mockAPIKeyStore{keys: map[string]*apikey.APIKey{}}
	chain := newV1Chain(store, dummyHandler())

	req := httptest.NewRequest(http.MethodOptions, "/v1/drugs/ndc/00069-3150", nil)
	req.Header.Set("Origin", "https://myapp.com")
	req.Header.Set("Access-Control-Request-Method", "GET")

	rr := httptest.NewRecorder()
	chain.ServeHTTP(rr, req)

	allowHeaders := rr.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(allowHeaders, "X-API-Key") {
		t.Errorf("AC-021: Allow-Headers must include X-API-Key, got %q", allowHeaders)
	}
	if rr.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("AC-021: expected Access-Control-Allow-Methods to be set")
	}
}

// AC-004 (real request): per-key origin enforcement happens on the actual request,
// which DOES carry the key. Allowed origin → Access-Control-Allow-Origin set.
func TestCORSPreflight_AC004_RealRequestAllowedOrigin(t *testing.T) {
	key := &apikey.APIKey{
		Key:       "real-allowed",
		AppName:   "test-app",
		Origins:   []string{"https://myapp.com"},
		RateLimit: 100,
		Active:    true,
	}
	store := &mockAPIKeyStore{keys: map[string]*apikey.APIKey{"real-allowed": key}}
	chain := newV1Chain(store, dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/ndc/00069-3150", nil)
	req.Header.Set("Origin", "https://myapp.com")
	req.Header.Set("X-API-Key", "real-allowed")

	rr := httptest.NewRecorder()
	chain.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("AC-004: expected 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://myapp.com" {
		t.Errorf("AC-004: expected Access-Control-Allow-Origin %q, got %q", "https://myapp.com", got)
	}
}

// AC-005 (real request): a permissive preflight does NOT weaken origin locking.
// The real request from a disallowed origin still gets no Access-Control-Allow-Origin,
// so the browser blocks the response read.
func TestCORSPreflight_AC005_RealRequestDisallowedOrigin(t *testing.T) {
	key := &apikey.APIKey{
		Key:       "real-locked",
		AppName:   "test-app",
		Origins:   []string{"https://good.com"},
		RateLimit: 100,
		Active:    true,
	}
	store := &mockAPIKeyStore{keys: map[string]*apikey.APIKey{"real-locked": key}}
	chain := newV1Chain(store, dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/ndc/00069-3150", nil)
	req.Header.Set("Origin", "https://evil.com")
	req.Header.Set("X-API-Key", "real-locked")

	rr := httptest.NewRecorder()
	chain.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("AC-005: expected the request to proceed (200), got %d", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("AC-005: disallowed origin must get no Access-Control-Allow-Origin, got %q", got)
	}
}

// An OPTIONS request that is NOT a CORS preflight (no Access-Control-Request-Method)
// must not bypass auth — it falls through to the auth layer.
func TestCORSPreflight_NonPreflightOptionsDoesNotBypassAuth(t *testing.T) {
	store := &mockAPIKeyStore{keys: map[string]*apikey.APIKey{}}
	chain := newV1Chain(store, dummyHandler())

	req := httptest.NewRequest(http.MethodOptions, "/v1/drugs/ndc/00069-3150", nil)
	req.Header.Set("Origin", "https://myapp.com")
	// No Access-Control-Request-Method → not a preflight.

	rr := httptest.NewRecorder()
	chain.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for non-preflight OPTIONS without a key, got %d", rr.Code)
	}
}

// A non-OPTIONS request passes straight through CORSPreflight to the next handler.
func TestCORSPreflight_GetPassesThrough(t *testing.T) {
	called := false
	handler := CORSPreflight(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/ndc/00069-3150", nil)
	req.Header.Set("Origin", "https://myapp.com")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("expected GET to pass through CORSPreflight to the next handler")
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

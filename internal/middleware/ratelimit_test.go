package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/finish06/drug-gate/internal/apikey"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/finish06/drug-gate/internal/ratelimit"
)

// mockLimiter implements ratelimit.Limiter for middleware tests.
type mockLimiter struct {
	result *ratelimit.Result
	err    error
}

func (m *mockLimiter) Allow(ctx context.Context, key string, limit int) (*ratelimit.Result, error) {
	return m.result, m.err
}

// contextWithAPIKey injects an APIKey into the request context using the
// same context key the auth middleware would set.
func contextWithAPIKey(ctx context.Context, ak *apikey.APIKey) context.Context {
	return context.WithValue(ctx, APIKeyContextKey, ak)
}

// okHandler is a simple handler that returns 200 OK with a body, used as
// the inner handler behind the rate limit middleware.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
})

// AC-007: When the limiter allows the request, the middleware passes through
// to the next handler and returns 200.
func TestRateLimitMiddleware_AC007_Allowed(t *testing.T) {
	resetAt := time.Now().Add(1 * time.Minute)
	lim := &mockLimiter{
		result: &ratelimit.Result{
			Allowed:   true,
			Remaining: 9,
			ResetAt:   resetAt,
		},
	}

	handler := RateLimit(lim)(okHandler)

	ak := &apikey.APIKey{
		Key:       "pk_testkey123",
		AppName:   "test-app",
		RateLimit: 10,
		Active:    true,
		CreatedAt: time.Now(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/drugs", nil)
	req = req.WithContext(contextWithAPIKey(req.Context(), ak))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got %q", rr.Body.String())
	}
}

// AC-007: When the limiter denies the request, the middleware returns 429
// Too Many Requests.
func TestRateLimitMiddleware_AC007_Denied(t *testing.T) {
	resetAt := time.Now().Add(30 * time.Second)
	lim := &mockLimiter{
		result: &ratelimit.Result{
			Allowed:    false,
			Remaining:  0,
			ResetAt:    resetAt,
			RetryAfter: 30 * time.Second,
		},
	}

	handler := RateLimit(lim)(okHandler)

	ak := &apikey.APIKey{
		Key:       "pk_testkey123",
		AppName:   "test-app",
		RateLimit: 10,
		Active:    true,
		CreatedAt: time.Now(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/drugs", nil)
	req = req.WithContext(contextWithAPIKey(req.Context(), ak))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", rr.Code)
	}

	// Verify the response body is a valid ErrorResponse.
	var errResp model.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error == "" {
		t.Error("expected non-empty Error field in response")
	}
	if errResp.Message == "" {
		t.Error("expected non-empty Message field in response")
	}
}

// AC-008: When rate limited (429), the Retry-After header is present with a
// value > 0 so the client knows how long to wait.
func TestRateLimitMiddleware_AC008_RetryAfterHeader(t *testing.T) {
	retryAfter := 45 * time.Second
	resetAt := time.Now().Add(retryAfter)
	lim := &mockLimiter{
		result: &ratelimit.Result{
			Allowed:    false,
			Remaining:  0,
			ResetAt:    resetAt,
			RetryAfter: retryAfter,
		},
	}

	handler := RateLimit(lim)(okHandler)

	ak := &apikey.APIKey{
		Key:       "pk_testkey123",
		AppName:   "test-app",
		RateLimit: 10,
		Active:    true,
		CreatedAt: time.Now(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/drugs", nil)
	req = req.WithContext(contextWithAPIKey(req.Context(), ak))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", rr.Code)
	}

	retryHeader := rr.Header().Get("Retry-After")
	if retryHeader == "" {
		t.Fatal("expected Retry-After header to be present on 429 response")
	}

	retrySeconds, err := strconv.Atoi(retryHeader)
	if err != nil {
		t.Fatalf("Retry-After header is not a valid integer: %q", retryHeader)
	}
	if retrySeconds <= 0 {
		t.Errorf("expected Retry-After > 0, got %d", retrySeconds)
	}
}

// AC-009: When the request is allowed (200), the response includes
// X-RateLimit-Remaining and X-RateLimit-Reset headers.
func TestRateLimitMiddleware_AC009_RateLimitHeaders(t *testing.T) {
	resetAt := time.Now().Add(1 * time.Minute)
	lim := &mockLimiter{
		result: &ratelimit.Result{
			Allowed:   true,
			Remaining: 7,
			ResetAt:   resetAt,
		},
	}

	handler := RateLimit(lim)(okHandler)

	ak := &apikey.APIKey{
		Key:       "pk_testkey123",
		AppName:   "test-app",
		RateLimit: 10,
		Active:    true,
		CreatedAt: time.Now(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/drugs", nil)
	req = req.WithContext(contextWithAPIKey(req.Context(), ak))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	// Verify X-RateLimit-Remaining header.
	remainingHeader := rr.Header().Get("X-RateLimit-Remaining")
	if remainingHeader == "" {
		t.Fatal("expected X-RateLimit-Remaining header to be present")
	}
	remaining, err := strconv.Atoi(remainingHeader)
	if err != nil {
		t.Fatalf("X-RateLimit-Remaining is not a valid integer: %q", remainingHeader)
	}
	if remaining != 7 {
		t.Errorf("expected X-RateLimit-Remaining=7, got %d", remaining)
	}

	// Verify X-RateLimit-Reset header.
	resetHeader := rr.Header().Get("X-RateLimit-Reset")
	if resetHeader == "" {
		t.Fatal("expected X-RateLimit-Reset header to be present")
	}
	resetEpoch, err := strconv.ParseInt(resetHeader, 10, 64)
	if err != nil {
		t.Fatalf("X-RateLimit-Reset is not a valid integer: %q", resetHeader)
	}
	if resetEpoch <= 0 {
		t.Errorf("expected X-RateLimit-Reset > 0, got %d", resetEpoch)
	}
}

// Verify that rate limit headers are also set on denied (429) responses.
func TestRateLimitMiddleware_AC009_HeadersOnDenied(t *testing.T) {
	resetAt := time.Now().Add(30 * time.Second)
	lim := &mockLimiter{
		result: &ratelimit.Result{
			Allowed:    false,
			Remaining:  0,
			ResetAt:    resetAt,
			RetryAfter: 30 * time.Second,
		},
	}

	handler := RateLimit(lim)(okHandler)

	ak := &apikey.APIKey{
		Key:       "pk_testkey123",
		AppName:   "test-app",
		RateLimit: 10,
		Active:    true,
		CreatedAt: time.Now(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/drugs", nil)
	req = req.WithContext(contextWithAPIKey(req.Context(), ak))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", rr.Code)
	}

	remainingHeader := rr.Header().Get("X-RateLimit-Remaining")
	if remainingHeader == "" {
		t.Error("expected X-RateLimit-Remaining header on 429 response")
	}

	resetHeader := rr.Header().Get("X-RateLimit-Reset")
	if resetHeader == "" {
		t.Error("expected X-RateLimit-Reset header on 429 response")
	}
}

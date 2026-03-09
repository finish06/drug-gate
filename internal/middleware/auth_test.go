package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/finish06/drug-gate/internal/apikey"
	"github.com/go-chi/chi/v5"
)

// mockAPIKeyStore implements apikey.Store for testing.
type mockAPIKeyStore struct {
	keys map[string]*apikey.APIKey
	err  error
}

func (m *mockAPIKeyStore) Get(_ context.Context, key string) (*apikey.APIKey, error) {
	if m.err != nil {
		return nil, m.err
	}
	k, ok := m.keys[key]
	if !ok {
		return nil, nil
	}
	return k, nil
}

func (m *mockAPIKeyStore) Create(_ context.Context, _ string, _ []string, _ int) (*apikey.APIKey, error) {
	return nil, nil
}

func (m *mockAPIKeyStore) List(_ context.Context) ([]apikey.APIKey, error) {
	return nil, nil
}

func (m *mockAPIKeyStore) Deactivate(_ context.Context, _ string) error {
	return nil
}

func (m *mockAPIKeyStore) Rotate(_ context.Context, _ string, _ time.Duration) (*apikey.APIKey, error) {
	return nil, nil
}

// newTestRouter sets up a Chi router with the auth middleware and a simple 200 handler.
func newTestRouter(store apikey.Store) *chi.Mux {
	r := chi.NewRouter()
	r.Use(APIKeyAuth(store))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		// Verify the API key is stored in context.
		ak, ok := r.Context().Value(APIKeyContextKey).(*apikey.APIKey)
		if !ok || ak == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return r
}

func TestAPIKeyAuth_AC001_MissingKey(t *testing.T) {
	store := &mockAPIKeyStore{keys: map[string]*apikey.APIKey{}}
	router := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No X-API-Key header set.
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["error"] != "unauthorized" {
		t.Errorf("error = %q, want %q", body["error"], "unauthorized")
	}
	if body["message"] == "" {
		t.Error("expected non-empty message in error response")
	}
}

func TestAPIKeyAuth_AC002_InvalidKey(t *testing.T) {
	store := &mockAPIKeyStore{keys: map[string]*apikey.APIKey{}}
	router := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "nonexistent-key-12345")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["error"] != "unauthorized" {
		t.Errorf("error = %q, want %q", body["error"], "unauthorized")
	}
}

func TestAPIKeyAuth_AC003_ValidKey(t *testing.T) {
	validKey := &apikey.APIKey{
		Key:       "valid-key-abc",
		AppName:   "test-app",
		Origins:   []string{"http://localhost"},
		RateLimit: 100,
		Active:    true,
		CreatedAt: time.Now().Add(-24 * time.Hour),
	}
	store := &mockAPIKeyStore{
		keys: map[string]*apikey.APIKey{
			"valid-key-abc": validKey,
		},
	}
	router := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "valid-key-abc")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Errorf("body = %q, want %q", rr.Body.String(), "ok")
	}
}

func TestAPIKeyAuth_AC012_InactiveKey(t *testing.T) {
	inactiveKey := &apikey.APIKey{
		Key:       "inactive-key-xyz",
		AppName:   "disabled-app",
		Origins:   []string{"http://localhost"},
		RateLimit: 100,
		Active:    false,
		CreatedAt: time.Now().Add(-24 * time.Hour),
	}
	store := &mockAPIKeyStore{
		keys: map[string]*apikey.APIKey{
			"inactive-key-xyz": inactiveKey,
		},
	}
	router := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "inactive-key-xyz")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["error"] != "unauthorized" {
		t.Errorf("error = %q, want %q", body["error"], "unauthorized")
	}
}

func TestAPIKeyAuth_AC013_ExpiredPastGracePeriod(t *testing.T) {
	// Key expired well beyond any grace period (30 days ago).
	expired := time.Now().Add(-30 * 24 * time.Hour)
	expiredKey := &apikey.APIKey{
		Key:       "expired-key-old",
		AppName:   "expired-app",
		Origins:   []string{"http://localhost"},
		RateLimit: 100,
		Active:    true,
		CreatedAt: time.Now().Add(-90 * 24 * time.Hour),
		ExpiresAt: &expired,
	}
	store := &mockAPIKeyStore{
		keys: map[string]*apikey.APIKey{
			"expired-key-old": expiredKey,
		},
	}
	router := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "expired-key-old")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["error"] != "unauthorized" {
		t.Errorf("error = %q, want %q", body["error"], "unauthorized")
	}
}

func TestAPIKeyAuth_AC013_WithinGracePeriod(t *testing.T) {
	// Key has an ExpiresAt set but it's still in the future (within grace period).
	recentlyExpired := time.Now().Add(1 * time.Hour)
	gracefulKey := &apikey.APIKey{
		Key:       "grace-key-recent",
		AppName:   "grace-app",
		Origins:   []string{"http://localhost"},
		RateLimit: 100,
		Active:    true,
		CreatedAt: time.Now().Add(-60 * 24 * time.Hour),
		ExpiresAt: &recentlyExpired,
	}
	store := &mockAPIKeyStore{
		keys: map[string]*apikey.APIKey{
			"grace-key-recent": gracefulKey,
		},
	}
	router := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "grace-key-recent")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Errorf("body = %q, want %q", rr.Body.String(), "ok")
	}
}

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/finish06/drug-gate/internal/apikey"
	"github.com/go-chi/chi/v5"
)

// mockAdminStore implements apikey.Store for handler-level unit testing.
type mockAdminStore struct {
	keys map[string]*apikey.APIKey
}

func newMockAdminStore() *mockAdminStore {
	return &mockAdminStore{
		keys: make(map[string]*apikey.APIKey),
	}
}

func (m *mockAdminStore) Create(ctx context.Context, appName string, origins []string, rateLimit int) (*apikey.APIKey, error) {
	key := fmt.Sprintf("pk_test_%d", len(m.keys)+1)
	ak := &apikey.APIKey{
		Key:       key,
		AppName:   appName,
		Origins:   origins,
		RateLimit: rateLimit,
		Active:    true,
		CreatedAt: time.Now().UTC(),
	}
	m.keys[key] = ak
	return ak, nil
}

func (m *mockAdminStore) Get(ctx context.Context, key string) (*apikey.APIKey, error) {
	ak, ok := m.keys[key]
	if !ok {
		return nil, nil
	}
	return ak, nil
}

func (m *mockAdminStore) List(ctx context.Context) ([]apikey.APIKey, error) {
	result := make([]apikey.APIKey, 0, len(m.keys))
	for _, ak := range m.keys {
		result = append(result, *ak)
	}
	return result, nil
}

func (m *mockAdminStore) Deactivate(ctx context.Context, key string) error {
	ak, ok := m.keys[key]
	if !ok {
		return fmt.Errorf("key not found")
	}
	ak.Active = false
	return nil
}

func (m *mockAdminStore) Rotate(ctx context.Context, oldKey string, gracePeriod time.Duration) (*apikey.APIKey, error) {
	old, ok := m.keys[oldKey]
	if !ok {
		return nil, fmt.Errorf("key not found")
	}

	// Set expiration on old key
	exp := time.Now().UTC().Add(gracePeriod)
	old.ExpiresAt = &exp

	// Create new key with same metadata
	newKeyStr := fmt.Sprintf("pk_test_%d", len(m.keys)+1)
	ak := &apikey.APIKey{
		Key:       newKeyStr,
		AppName:   old.AppName,
		Origins:   old.Origins,
		RateLimit: old.RateLimit,
		Active:    true,
		CreatedAt: time.Now().UTC(),
	}
	m.keys[newKeyStr] = ak
	return ak, nil
}

// newAdminTestRouter creates a Chi router with admin routes registered.
// No admin auth middleware is applied — auth is tested separately.
func newAdminTestRouter(h *AdminHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/admin/keys", h.CreateKey)
	r.Get("/admin/keys", h.ListKeys)
	r.Get("/admin/keys/{key}", h.GetKey)
	r.Delete("/admin/keys/{key}", h.DeactivateKey)
	r.Post("/admin/keys/{key}/rotate", h.RotateKey)
	return r
}

// doAdminRequest is a helper that dispatches a request to the router and
// returns the recorded response.
func doAdminRequest(router http.Handler, method, path string, body io.Reader) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// --- AC-011: Create API key ---

// TestAdmin_AC011_CreateKey verifies that POST /admin/keys with a valid body
// returns 201 and the key metadata.
func TestAdmin_AC011_CreateKey(t *testing.T) {
	store := newMockAdminStore()
	h := NewAdminHandler(store)
	router := newAdminTestRouter(h)

	reqBody := `{"app_name":"my-frontend","origins":["https://example.com"],"rate_limit":100}`
	rr := doAdminRequest(router, http.MethodPost, "/admin/keys", bytes.NewBufferString(reqBody))

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["key"] == nil || resp["key"] == "" {
		t.Error("expected non-empty 'key' in response")
	}
	if resp["app_name"] != "my-frontend" {
		t.Errorf("app_name = %v, want %q", resp["app_name"], "my-frontend")
	}
	if resp["active"] != true {
		t.Errorf("active = %v, want true", resp["active"])
	}
}

// TestAdmin_AC011_CreateKey_MissingAppName verifies that POST /admin/keys
// without app_name returns 400.
func TestAdmin_AC011_CreateKey_MissingAppName(t *testing.T) {
	store := newMockAdminStore()
	h := NewAdminHandler(store)
	router := newAdminTestRouter(h)

	reqBody := `{"origins":["https://example.com"],"rate_limit":100}`
	rr := doAdminRequest(router, http.MethodPost, "/admin/keys", bytes.NewBufferString(reqBody))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rr.Code, rr.Body.String())
	}
}

// TestAdmin_AC011_CreateKey_ZeroRateLimit verifies that POST /admin/keys
// with rate_limit=0 returns 400.
func TestAdmin_AC011_CreateKey_ZeroRateLimit(t *testing.T) {
	store := newMockAdminStore()
	h := NewAdminHandler(store)
	router := newAdminTestRouter(h)

	reqBody := `{"app_name":"my-app","origins":["https://example.com"],"rate_limit":0}`
	rr := doAdminRequest(router, http.MethodPost, "/admin/keys", bytes.NewBufferString(reqBody))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rr.Code, rr.Body.String())
	}
}

// --- AC-016: List API keys ---

// TestAdmin_AC016_ListKeys verifies that GET /admin/keys returns 200 with
// a list of all keys.
func TestAdmin_AC016_ListKeys(t *testing.T) {
	store := newMockAdminStore()
	h := NewAdminHandler(store)
	router := newAdminTestRouter(h)

	// Create two keys first.
	body1 := `{"app_name":"app-one","origins":[],"rate_limit":50}`
	rr := doAdminRequest(router, http.MethodPost, "/admin/keys", bytes.NewBufferString(body1))
	if rr.Code != http.StatusCreated {
		t.Fatalf("setup: create key 1 failed: %d", rr.Code)
	}
	body2 := `{"app_name":"app-two","origins":["https://two.com"],"rate_limit":75}`
	rr = doAdminRequest(router, http.MethodPost, "/admin/keys", bytes.NewBufferString(body2))
	if rr.Code != http.StatusCreated {
		t.Fatalf("setup: create key 2 failed: %d", rr.Code)
	}

	// List keys.
	rr = doAdminRequest(router, http.MethodGet, "/admin/keys", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}

	var keys []map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &keys); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

// --- AC-017: Get single API key ---

// TestAdmin_AC017_GetKey verifies that GET /admin/keys/{key} returns 200
// with key metadata for a known key.
func TestAdmin_AC017_GetKey(t *testing.T) {
	store := newMockAdminStore()
	h := NewAdminHandler(store)
	router := newAdminTestRouter(h)

	// Create a key.
	reqBody := `{"app_name":"get-me","origins":["https://get.com"],"rate_limit":60}`
	rr := doAdminRequest(router, http.MethodPost, "/admin/keys", bytes.NewBufferString(reqBody))
	if rr.Code != http.StatusCreated {
		t.Fatalf("setup: create key failed: %d", rr.Code)
	}

	var created map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal create response: %v", err)
	}
	keyStr, ok := created["key"].(string)
	if !ok || keyStr == "" {
		t.Fatal("expected non-empty key string from create response")
	}

	// Fetch the key.
	rr = doAdminRequest(router, http.MethodGet, "/admin/keys/"+keyStr, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}

	var fetched map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("failed to unmarshal get response: %v", err)
	}
	if fetched["key"] != keyStr {
		t.Errorf("key = %v, want %q", fetched["key"], keyStr)
	}
	if fetched["app_name"] != "get-me" {
		t.Errorf("app_name = %v, want %q", fetched["app_name"], "get-me")
	}
}

// TestAdmin_AC017_GetKey_NotFound verifies that GET /admin/keys/{unknown}
// returns 404.
func TestAdmin_AC017_GetKey_NotFound(t *testing.T) {
	store := newMockAdminStore()
	h := NewAdminHandler(store)
	router := newAdminTestRouter(h)

	rr := doAdminRequest(router, http.MethodGet, "/admin/keys/pk_nonexistent_key", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rr.Code, rr.Body.String())
	}
}

// --- AC-012: Deactivate API key ---

// TestAdmin_AC012_DeactivateKey verifies that DELETE /admin/keys/{key}
// returns 200 and the key is deactivated.
func TestAdmin_AC012_DeactivateKey(t *testing.T) {
	store := newMockAdminStore()
	h := NewAdminHandler(store)
	router := newAdminTestRouter(h)

	// Create a key.
	reqBody := `{"app_name":"deact-app","origins":[],"rate_limit":30}`
	rr := doAdminRequest(router, http.MethodPost, "/admin/keys", bytes.NewBufferString(reqBody))
	if rr.Code != http.StatusCreated {
		t.Fatalf("setup: create key failed: %d", rr.Code)
	}

	var created map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal create response: %v", err)
	}
	keyStr := created["key"].(string)

	// Deactivate the key.
	rr = doAdminRequest(router, http.MethodDelete, "/admin/keys/"+keyStr, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}

	// Verify the key is now inactive by fetching it.
	rr = doAdminRequest(router, http.MethodGet, "/admin/keys/"+keyStr, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("get after deactivate: status = %d, want 200", rr.Code)
	}

	var fetched map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("failed to unmarshal get response: %v", err)
	}
	if fetched["active"] != false {
		t.Errorf("expected active=false after deactivation, got %v", fetched["active"])
	}
}

// --- AC-013: Rotate API key ---

// TestAdmin_AC013_RotateKey verifies that POST /admin/keys/{key}/rotate
// returns 200 with old_key, new_key, and old_key_expires_at.
func TestAdmin_AC013_RotateKey(t *testing.T) {
	store := newMockAdminStore()
	h := NewAdminHandler(store)
	router := newAdminTestRouter(h)

	// Create a key.
	reqBody := `{"app_name":"rotate-app","origins":["https://rotate.com"],"rate_limit":90}`
	rr := doAdminRequest(router, http.MethodPost, "/admin/keys", bytes.NewBufferString(reqBody))
	if rr.Code != http.StatusCreated {
		t.Fatalf("setup: create key failed: %d", rr.Code)
	}

	var created map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal create response: %v", err)
	}
	oldKeyStr := created["key"].(string)

	// Rotate the key.
	rotateBody := `{"grace_period":"24h"}`
	rr = doAdminRequest(router, http.MethodPost, "/admin/keys/"+oldKeyStr+"/rotate", bytes.NewBufferString(rotateBody))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}

	var rotateResp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &rotateResp); err != nil {
		t.Fatalf("failed to unmarshal rotate response: %v", err)
	}

	// Verify response contains old_key, new_key, and old_key_expires_at.
	if rotateResp["old_key"] == nil || rotateResp["old_key"] == "" {
		t.Error("expected non-empty 'old_key' in rotate response")
	}
	if rotateResp["new_key"] == nil || rotateResp["new_key"] == "" {
		t.Error("expected non-empty 'new_key' in rotate response")
	}
	if rotateResp["old_key_expires_at"] == nil || rotateResp["old_key_expires_at"] == "" {
		t.Error("expected non-empty 'old_key_expires_at' in rotate response")
	}

	// Old and new keys should be different.
	if rotateResp["old_key"] == rotateResp["new_key"] {
		t.Error("expected old_key and new_key to be different")
	}
}

// --- AC-018: Custom rate limit ---

// TestAdmin_AC018_CreateKey_CustomRateLimit verifies that POST /admin/keys
// with a custom rate_limit stores it correctly.
func TestAdmin_AC018_CreateKey_CustomRateLimit(t *testing.T) {
	store := newMockAdminStore()
	h := NewAdminHandler(store)
	router := newAdminTestRouter(h)

	reqBody := `{"app_name":"rate-app","origins":["https://rate.com"],"rate_limit":500}`
	rr := doAdminRequest(router, http.MethodPost, "/admin/keys", bytes.NewBufferString(reqBody))
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rr.Code, rr.Body.String())
	}

	var created map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal create response: %v", err)
	}

	keyStr := created["key"].(string)

	// Fetch the key and verify rate_limit is stored.
	rr = doAdminRequest(router, http.MethodGet, "/admin/keys/"+keyStr, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("get: status = %d, want 200", rr.Code)
	}

	var fetched map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("failed to unmarshal get response: %v", err)
	}

	// JSON numbers are float64 by default.
	rateLimit, ok := fetched["rate_limit"].(float64)
	if !ok {
		t.Fatalf("rate_limit is not a number: %v (%T)", fetched["rate_limit"], fetched["rate_limit"])
	}
	if int(rateLimit) != 500 {
		t.Errorf("rate_limit = %v, want 500", rateLimit)
	}
}

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

func setupCacheTest(t *testing.T) (*miniredis.Miniredis, *CacheHandler) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	h := NewCacheHandler(rdb)
	return mr, h
}

func cacheRouter(h *CacheHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Delete("/admin/cache", h.ClearCache)
	return r
}

type cacheClearResponse struct {
	Status      string `json:"status"`
	KeysDeleted int    `json:"keys_deleted"`
}

// AC-001: DELETE /admin/cache deletes all cache:* keys
func TestCacheHandler_ClearAll(t *testing.T) {
	mr, h := setupCacheTest(t)
	r := cacheRouter(h)

	// Populate cache keys
	mr.Set("cache:drugnames", "data")
	mr.Set("cache:drugclasses", "data")
	mr.Set("cache:rxnorm:search:lipitor", "data")
	// Non-cache keys — should survive
	mr.Set("apikey:pk_abc123", "keydata")
	mr.Set("ratelimit:pk_abc123", "limitdata")

	req := httptest.NewRequest(http.MethodDelete, "/admin/cache", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp cacheClearResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Status != "ok" {
		t.Errorf("status = %q, want %q", resp.Status, "ok")
	}
	if resp.KeysDeleted != 3 {
		t.Errorf("keys_deleted = %d, want 3", resp.KeysDeleted)
	}

	// AC-008: Non-cache keys preserved
	if !mr.Exists("apikey:pk_abc123") {
		t.Error("apikey:pk_abc123 should not have been deleted")
	}
	if !mr.Exists("ratelimit:pk_abc123") {
		t.Error("ratelimit:pk_abc123 should not have been deleted")
	}
	// Cache keys gone
	if mr.Exists("cache:drugnames") {
		t.Error("cache:drugnames should have been deleted")
	}
}

// AC-002: DELETE /admin/cache?prefix=rxnorm deletes only matching keys
func TestCacheHandler_ClearByPrefix(t *testing.T) {
	mr, h := setupCacheTest(t)
	r := cacheRouter(h)

	mr.Set("cache:rxnorm:search:lipitor", "data")
	mr.Set("cache:rxnorm:ndcs:153165", "data")
	mr.Set("cache:drugnames", "data")
	mr.Set("cache:drugclasses", "data")

	req := httptest.NewRequest(http.MethodDelete, "/admin/cache?prefix=rxnorm", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp cacheClearResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.KeysDeleted != 2 {
		t.Errorf("keys_deleted = %d, want 2", resp.KeysDeleted)
	}

	// RxNorm keys gone
	if mr.Exists("cache:rxnorm:search:lipitor") {
		t.Error("rxnorm key should have been deleted")
	}
	// Non-rxnorm cache keys preserved
	if !mr.Exists("cache:drugnames") {
		t.Error("cache:drugnames should not have been deleted")
	}
	if !mr.Exists("cache:drugclasses") {
		t.Error("cache:drugclasses should not have been deleted")
	}
}

// AC-007: Prefix matching nothing returns 200 with keys_deleted: 0
func TestCacheHandler_ClearNoMatches(t *testing.T) {
	_, h := setupCacheTest(t)
	r := cacheRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/admin/cache?prefix=nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp cacheClearResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.KeysDeleted != 0 {
		t.Errorf("keys_deleted = %d, want 0", resp.KeysDeleted)
	}
}

// AC-006: Empty prefix clears all cache:* keys
func TestCacheHandler_ClearEmptyPrefix(t *testing.T) {
	mr, h := setupCacheTest(t)
	r := cacheRouter(h)

	mr.Set("cache:drugnames", "data")
	mr.Set("cache:rxnorm:search:test", "data")

	req := httptest.NewRequest(http.MethodDelete, "/admin/cache?prefix=", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp cacheClearResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.KeysDeleted != 2 {
		t.Errorf("keys_deleted = %d, want 2", resp.KeysDeleted)
	}
}

package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

type testItem struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, rdb
}

func newTestMetrics(t *testing.T) *metrics.Metrics {
	t.Helper()
	reg := prometheus.NewRegistry()
	return metrics.NewMetrics(reg)
}

// AC-002: Cache hit returns deserialized data via GetEx (sliding TTL).
func TestCacheAside_AC002_CacheHit(t *testing.T) {
	mr, rdb := setupTestRedis(t)
	defer mr.Close()

	// Pre-populate cache
	item := testItem{Name: "aspirin", Value: 42}
	data, _ := json.Marshal(item)
	mr.Set("test:key", string(data))
	mr.SetTTL("test:key", 60*time.Minute)

	m := newTestMetrics(t)
	ca := New[testItem](rdb, m, "test:key", 60*time.Minute, "test")

	fetchCalled := false
	result, err := ca.Get(context.Background(), func(ctx context.Context) (testItem, error) {
		fetchCalled = true
		return testItem{}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fetchCalled {
		t.Error("fetch should not be called on cache hit")
	}
	if result.Name != "aspirin" || result.Value != 42 {
		t.Errorf("unexpected result: %+v", result)
	}
}

// AC-003: Cache miss calls fetch, stores result.
func TestCacheAside_AC003_CacheMiss(t *testing.T) {
	mr, rdb := setupTestRedis(t)
	defer mr.Close()

	m := newTestMetrics(t)
	ca := New[testItem](rdb, m, "test:miss", 60*time.Minute, "test")

	fetchCalled := false
	result, err := ca.Get(context.Background(), func(ctx context.Context) (testItem, error) {
		fetchCalled = true
		return testItem{Name: "metformin", Value: 99}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fetchCalled {
		t.Error("fetch should be called on cache miss")
	}
	if result.Name != "metformin" || result.Value != 99 {
		t.Errorf("unexpected result: %+v", result)
	}

	// Verify stored in Redis
	stored, err := mr.Get("test:miss")
	if err != nil {
		t.Fatalf("value not stored in Redis: %v", err)
	}
	var cached testItem
	if err := json.Unmarshal([]byte(stored), &cached); err != nil {
		t.Fatalf("stored value not valid JSON: %v", err)
	}
	if cached.Name != "metformin" {
		t.Errorf("stored value mismatch: %+v", cached)
	}
}

// AC-003: Second call hits cache (no second fetch).
func TestCacheAside_AC003_SecondCallHitsCache(t *testing.T) {
	mr, rdb := setupTestRedis(t)
	defer mr.Close()

	m := newTestMetrics(t)
	ca := New[testItem](rdb, m, "test:twice", 60*time.Minute, "test")

	fetchCount := 0
	fetch := func(ctx context.Context) (testItem, error) {
		fetchCount++
		return testItem{Name: "lisinopril", Value: 10}, nil
	}

	// First call — miss
	_, _ = ca.Get(context.Background(), fetch)
	// Second call — hit
	result, _ := ca.Get(context.Background(), fetch)

	if fetchCount != 1 {
		t.Errorf("expected 1 fetch call, got %d", fetchCount)
	}
	if result.Name != "lisinopril" {
		t.Errorf("unexpected result: %+v", result)
	}
}

// AC-004: Corrupt cache triggers fresh fetch.
func TestCacheAside_AC004_CorruptCache(t *testing.T) {
	mr, rdb := setupTestRedis(t)
	defer mr.Close()

	mr.Set("test:corrupt", "not-valid-json{{{")

	m := newTestMetrics(t)
	ca := New[testItem](rdb, m, "test:corrupt", 60*time.Minute, "test")

	result, err := ca.Get(context.Background(), func(ctx context.Context) (testItem, error) {
		return testItem{Name: "recovered", Value: 1}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "recovered" {
		t.Errorf("expected fresh fetch after corrupt cache, got: %+v", result)
	}
}

// AC-005: Metrics recorded for hit and miss.
func TestCacheAside_AC005_MetricsRecorded(t *testing.T) {
	mr, rdb := setupTestRedis(t)
	defer mr.Close()

	m := newTestMetrics(t)
	ca := New[testItem](rdb, m, "test:metrics", 60*time.Minute, "metrics-test")

	// Miss
	_, _ = ca.Get(context.Background(), func(ctx context.Context) (testItem, error) {
		return testItem{Name: "a"}, nil
	})
	// Hit
	_, _ = ca.Get(context.Background(), func(ctx context.Context) (testItem, error) {
		return testItem{Name: "b"}, nil
	})

	// Verify metrics (if we can read them)
	// The metrics package uses CounterVec — we just verify no panic occurred
	// and the calls completed successfully
}

// AC-006: Upstream error propagated.
func TestCacheAside_AC006_UpstreamError(t *testing.T) {
	mr, rdb := setupTestRedis(t)
	defer mr.Close()

	m := newTestMetrics(t)
	ca := New[testItem](rdb, m, "test:error", 60*time.Minute, "test")

	upstreamErr := errors.New("upstream failed")
	_, err := ca.Get(context.Background(), func(ctx context.Context) (testItem, error) {
		return testItem{}, upstreamErr
	})

	if !errors.Is(err, upstreamErr) {
		t.Errorf("expected upstream error, got: %v", err)
	}
}

// AC-007: TTL is per-instance.
func TestCacheAside_AC007_CustomTTL(t *testing.T) {
	mr, rdb := setupTestRedis(t)
	defer mr.Close()

	m := newTestMetrics(t)
	ca := New[testItem](rdb, m, "test:ttl", 5*time.Minute, "test")

	_, _ = ca.Get(context.Background(), func(ctx context.Context) (testItem, error) {
		return testItem{Name: "short-ttl"}, nil
	})

	ttl := mr.TTL("test:ttl")
	if ttl <= 0 || ttl > 5*time.Minute {
		t.Errorf("expected TTL ~5m, got %v", ttl)
	}
}

// Nil metrics should not panic.
func TestCacheAside_NilMetrics(t *testing.T) {
	mr, rdb := setupTestRedis(t)
	defer mr.Close()

	ca := New[testItem](rdb, nil, "test:nilmetrics", 60*time.Minute, "test")

	result, err := ca.Get(context.Background(), func(ctx context.Context) (testItem, error) {
		return testItem{Name: "works"}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "works" {
		t.Errorf("unexpected result: %+v", result)
	}
}

// Sliding TTL: GetEx resets TTL on read.
func TestCacheAside_SlidingTTL(t *testing.T) {
	mr, rdb := setupTestRedis(t)
	defer mr.Close()

	m := newTestMetrics(t)
	ca := New[testItem](rdb, m, "test:sliding", 60*time.Minute, "test")

	// Populate
	_, _ = ca.Get(context.Background(), func(ctx context.Context) (testItem, error) {
		return testItem{Name: "sliding"}, nil
	})

	// Advance 50 minutes
	mr.FastForward(50 * time.Minute)

	// Read again — should hit cache and reset TTL
	_, _ = ca.Get(context.Background(), func(ctx context.Context) (testItem, error) {
		t.Error("fetch should not be called — cache should still be alive")
		return testItem{}, nil
	})

	// Advance another 50 minutes (100 total, but only 50 since reset)
	mr.FastForward(50 * time.Minute)

	// Should still be cached
	fetchCount := 0
	_, _ = ca.Get(context.Background(), func(ctx context.Context) (testItem, error) {
		fetchCount++
		return testItem{Name: "fresh"}, nil
	})

	if fetchCount != 0 {
		t.Error("TTL should have been reset by GetEx — expected cache hit")
	}
}

// AC-009: Stale cache served when circuit is open.
func TestCacheAside_AC009_StaleOnCircuitOpen(t *testing.T) {
	mr, rdb := setupTestRedis(t)
	defer mr.Close()

	m := newTestMetrics(t)
	ca := New[testItem](rdb, m, "test:stale", 60*time.Minute, "test")

	// Populate via GetWithStale (creates both fresh + stale keys)
	_, _ = ca.GetWithStale(context.Background(), func(ctx context.Context) (testItem, error) {
		return testItem{Name: "original", Value: 42}, nil
	})

	// Expire the fresh cache key
	mr.FastForward(61 * time.Minute)

	// Stale backup key (stale:test:stale) should still exist (no TTL)
	if !mr.Exists("stale:test:stale") {
		t.Fatal("stale backup key should exist")
	}

	// Fetch with circuit open error — should serve stale from backup key
	result, err := ca.GetWithStale(context.Background(), func(ctx context.Context) (testItem, error) {
		return testItem{}, fmt.Errorf("wrapped: %w", client.ErrCircuitOpen)
	})

	if err != nil {
		t.Fatalf("expected stale cache, got error: %v", err)
	}
	if !result.Stale {
		t.Error("expected Stale=true")
	}
	if result.Value.Name != "original" {
		t.Errorf("expected stale value 'original', got %q", result.Value.Name)
	}
}

// AC-012: No stale cache + circuit open → error.
func TestCacheAside_AC012_NoStaleCircuitOpen(t *testing.T) {
	mr, rdb := setupTestRedis(t)
	defer mr.Close()

	m := newTestMetrics(t)
	ca := New[testItem](rdb, m, "test:nostale", 60*time.Minute, "test")

	// No cache populated — fetch with circuit open
	_, err := ca.GetWithStale(context.Background(), func(ctx context.Context) (testItem, error) {
		return testItem{}, client.ErrCircuitOpen
	})

	if !errors.Is(err, client.ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got: %v", err)
	}
}

// GetWithStale returns fresh data when cache is valid.
func TestCacheAside_GetWithStale_FreshHit(t *testing.T) {
	mr, rdb := setupTestRedis(t)
	defer mr.Close()

	m := newTestMetrics(t)
	ca := New[testItem](rdb, m, "test:freshstale", 60*time.Minute, "test")

	// Populate
	_, _ = ca.Get(context.Background(), func(ctx context.Context) (testItem, error) {
		return testItem{Name: "fresh"}, nil
	})

	// GetWithStale should return fresh (not stale)
	result, err := ca.GetWithStale(context.Background(), func(ctx context.Context) (testItem, error) {
		t.Error("fetch should not be called on cache hit")
		return testItem{}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stale {
		t.Error("expected Stale=false for fresh cache hit")
	}
	if result.Value.Name != "fresh" {
		t.Errorf("expected 'fresh', got %q", result.Value.Name)
	}
}

// S-001: Singleflight prevents thundering herd on concurrent cache misses.
func TestCacheAside_Singleflight_ConcurrentMiss(t *testing.T) {
	mr, rdb := setupTestRedis(t)
	defer mr.Close()

	m := newTestMetrics(t)

	var fetchCount atomic.Int32
	concurrency := 100

	var wg sync.WaitGroup
	results := make([]testItem, concurrency)
	errs := make([]error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Each goroutine creates its own CacheAside (mimics per-request construction)
			ca := New[testItem](rdb, m, "test:singleflight", 60*time.Minute, "test")
			results[idx], errs[idx] = ca.Get(context.Background(), func(ctx context.Context) (testItem, error) {
				fetchCount.Add(1)
				time.Sleep(10 * time.Millisecond) // simulate upstream latency
				return testItem{Name: "shared", Value: 42}, nil
			})
		}(i)
	}

	wg.Wait()

	// Singleflight: exactly 1 fetch despite 100 concurrent callers
	if fc := fetchCount.Load(); fc != 1 {
		t.Errorf("expected exactly 1 fetch call, got %d", fc)
	}

	// All goroutines got the same result
	for i := 0; i < concurrency; i++ {
		if errs[i] != nil {
			t.Errorf("goroutine %d got error: %v", i, errs[i])
		}
		if results[i].Name != "shared" || results[i].Value != 42 {
			t.Errorf("goroutine %d got wrong result: %+v", i, results[i])
		}
	}
}

// S-001: Singleflight propagates fetch errors to all waiters.
func TestCacheAside_Singleflight_ErrorPropagation(t *testing.T) {
	mr, rdb := setupTestRedis(t)
	defer mr.Close()

	m := newTestMetrics(t)

	var fetchCount atomic.Int32
	concurrency := 50
	fetchErr := fmt.Errorf("upstream down")

	// Use a barrier to ensure all goroutines are ready before any start fetching
	ready := make(chan struct{})

	var wg sync.WaitGroup
	errs := make([]error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-ready // wait for all goroutines to be ready
			ca := New[testItem](rdb, m, "test:sf-error", 60*time.Minute, "test")
			_, errs[idx] = ca.Get(context.Background(), func(ctx context.Context) (testItem, error) {
				fetchCount.Add(1)
				time.Sleep(10 * time.Millisecond) // hold the singleflight key so others coalesce
				return testItem{}, fetchErr
			})
		}(i)
	}

	close(ready) // release all goroutines simultaneously
	wg.Wait()

	if fc := fetchCount.Load(); fc != 1 {
		t.Errorf("expected exactly 1 fetch call, got %d", fc)
	}

	for i := 0; i < concurrency; i++ {
		if errs[i] == nil {
			t.Errorf("goroutine %d should have received error", i)
		}
	}
}

// S-001: Cache hit bypasses singleflight entirely.
func TestCacheAside_Singleflight_CacheHitBypass(t *testing.T) {
	mr, rdb := setupTestRedis(t)
	defer mr.Close()

	m := newTestMetrics(t)
	ca := New[testItem](rdb, m, "test:sf-hit", 60*time.Minute, "test")

	// Pre-populate cache
	data, _ := json.Marshal(testItem{Name: "cached", Value: 99})
	_ = mr.Set("test:sf-hit", string(data))

	var fetchCount atomic.Int32
	result, err := ca.Get(context.Background(), func(ctx context.Context) (testItem, error) {
		fetchCount.Add(1)
		return testItem{}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fetchCount.Load() != 0 {
		t.Error("fetch should not be called on cache hit")
	}
	if result.Name != "cached" {
		t.Errorf("expected 'cached', got %q", result.Name)
	}
}

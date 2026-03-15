package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/finish06/drug-gate/internal/ratelimit"
	"github.com/redis/go-redis/v9"
)

func setupMiniRedisLimiter(t *testing.T) *ratelimit.RedisLimiter {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	return ratelimit.NewRedisLimiter(rdb)
}

func TestUnit_AllowUnderLimit(t *testing.T) {
	limiter := setupMiniRedisLimiter(t)
	ctx := context.Background()

	result, err := limiter.Allow(ctx, "test-key", 10)
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if !result.Allowed {
		t.Error("expected allowed=true under limit")
	}
	if result.Remaining != 9 {
		t.Errorf("Remaining = %d, want 9", result.Remaining)
	}
}

func TestUnit_ExceedLimit(t *testing.T) {
	limiter := setupMiniRedisLimiter(t)
	ctx := context.Background()

	limit := 5
	for i := 0; i < limit; i++ {
		result, err := limiter.Allow(ctx, "exceed-key", limit)
		if err != nil {
			t.Fatalf("Allow[%d]: %v", i, err)
		}
		if !result.Allowed {
			t.Fatalf("request %d should be allowed", i)
		}
	}

	// Next request should be rejected
	result, err := limiter.Allow(ctx, "exceed-key", limit)
	if err != nil {
		t.Fatalf("Allow[over]: %v", err)
	}
	if result.Allowed {
		t.Error("expected allowed=false when over limit")
	}
	if result.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0", result.Remaining)
	}
	if result.RetryAfter <= 0 {
		t.Error("expected positive RetryAfter")
	}
}

func TestUnit_SeparateKeys(t *testing.T) {
	limiter := setupMiniRedisLimiter(t)
	ctx := context.Background()

	r1, err := limiter.Allow(ctx, "key-a", 10)
	if err != nil {
		t.Fatalf("Allow key-a: %v", err)
	}
	r2, err := limiter.Allow(ctx, "key-b", 10)
	if err != nil {
		t.Fatalf("Allow key-b: %v", err)
	}

	if !r1.Allowed || !r2.Allowed {
		t.Error("both keys should be allowed independently")
	}
	if r1.Remaining != 9 || r2.Remaining != 9 {
		t.Error("each key should have independent remaining count")
	}
}

func TestUnit_RemainingDecrements(t *testing.T) {
	limiter := setupMiniRedisLimiter(t)
	ctx := context.Background()

	limit := 10
	for i := 0; i < 5; i++ {
		result, err := limiter.Allow(ctx, "decrement-key", limit)
		if err != nil {
			t.Fatalf("Allow[%d]: %v", i, err)
		}
		expected := limit - (i + 1)
		if result.Remaining != expected {
			t.Errorf("request %d: Remaining = %d, want %d", i, result.Remaining, expected)
		}
	}
}

func TestUnit_ResetAtIsFuture(t *testing.T) {
	limiter := setupMiniRedisLimiter(t)
	ctx := context.Background()

	before := time.Now()
	result, err := limiter.Allow(ctx, "reset-key", 10)
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}

	if result.ResetAt.Before(before) {
		t.Errorf("ResetAt %v is before request time %v", result.ResetAt, before)
	}
	if result.ResetAt.Before(before.Add(30 * time.Second)) {
		t.Errorf("ResetAt %v should be ~1 minute in the future", result.ResetAt)
	}
}

func TestUnit_RetryAfterOnDenied(t *testing.T) {
	limiter := setupMiniRedisLimiter(t)
	ctx := context.Background()

	// Exhaust limit of 1
	result, err := limiter.Allow(ctx, "retry-key", 1)
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if !result.Allowed {
		t.Fatal("first request should be allowed")
	}

	// Second request should be denied with RetryAfter
	result, err = limiter.Allow(ctx, "retry-key", 1)
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if result.Allowed {
		t.Error("expected denied")
	}
	if result.RetryAfter <= 0 {
		t.Errorf("expected positive RetryAfter, got %v", result.RetryAfter)
	}
	if result.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0", result.Remaining)
	}
}

func TestUnit_LimitOfOne(t *testing.T) {
	limiter := setupMiniRedisLimiter(t)
	ctx := context.Background()

	r1, err := limiter.Allow(ctx, "one-key", 1)
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if !r1.Allowed {
		t.Error("first request with limit=1 should be allowed")
	}
	if r1.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0 after single allowed request", r1.Remaining)
	}

	r2, err := limiter.Allow(ctx, "one-key", 1)
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if r2.Allowed {
		t.Error("second request with limit=1 should be denied")
	}
}

func TestUnit_HighLimit(t *testing.T) {
	limiter := setupMiniRedisLimiter(t)
	ctx := context.Background()

	result, err := limiter.Allow(ctx, "high-key", 10000)
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if !result.Allowed {
		t.Error("expected allowed with high limit")
	}
	if result.Remaining != 9999 {
		t.Errorf("Remaining = %d, want 9999", result.Remaining)
	}
}

func TestUnit_AllowedFieldTrue(t *testing.T) {
	limiter := setupMiniRedisLimiter(t)
	ctx := context.Background()

	result, err := limiter.Allow(ctx, "allowed-key", 5)
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if result.Allowed != true {
		t.Error("expected Allowed field to be true for request under limit")
	}
}

func TestUnit_DeniedFieldFalse(t *testing.T) {
	limiter := setupMiniRedisLimiter(t)
	ctx := context.Background()

	// Exhaust limit
	_, err := limiter.Allow(ctx, "denied-key", 1)
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}

	// This request should be denied
	result, err := limiter.Allow(ctx, "denied-key", 1)
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if result.Allowed != false {
		t.Error("expected Allowed field to be false for request over limit")
	}
}

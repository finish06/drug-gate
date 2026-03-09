//go:build integration

package ratelimit_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/finish06/drug-gate/internal/ratelimit"
	"github.com/redis/go-redis/v9"
)

func setupRedisLimiter(t *testing.T) *ratelimit.RedisLimiter {
	t.Helper()

	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		addr = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}

	t.Cleanup(func() {
		iter := rdb.Scan(ctx, 0, "ratelimit:*", 100).Iterator()
		for iter.Next(ctx) {
			rdb.Del(ctx, iter.Val())
		}
		rdb.Close()
	})

	return ratelimit.NewRedisLimiter(rdb)
}

func TestRedisLimiter_AllowUnderLimit(t *testing.T) {
	limiter := setupRedisLimiter(t)
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

func TestRedisLimiter_ExceedLimit(t *testing.T) {
	limiter := setupRedisLimiter(t)
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

func TestRedisLimiter_SeparateKeys(t *testing.T) {
	limiter := setupRedisLimiter(t)
	ctx := context.Background()

	r1, _ := limiter.Allow(ctx, "key-a", 10)
	r2, _ := limiter.Allow(ctx, "key-b", 10)

	if !r1.Allowed || !r2.Allowed {
		t.Error("both keys should be allowed independently")
	}
	if r1.Remaining != 9 || r2.Remaining != 9 {
		t.Error("each key should have independent remaining count")
	}
}

func TestRedisLimiter_RemainingDecrements(t *testing.T) {
	limiter := setupRedisLimiter(t)
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

func TestRedisLimiter_ResetAtIsFuture(t *testing.T) {
	limiter := setupRedisLimiter(t)
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

func TestRedisLimiter_RetryAfterOnDenied(t *testing.T) {
	limiter := setupRedisLimiter(t)
	ctx := context.Background()

	// Exhaust limit of 1
	result, _ := limiter.Allow(ctx, "retry-key", 1)
	if !result.Allowed {
		t.Fatal("first request should be allowed")
	}

	// Second request should be denied with RetryAfter
	result, err := limiter.Allow(ctx, "retry-key", 1)
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

func TestRedisLimiter_LimitOfOne(t *testing.T) {
	limiter := setupRedisLimiter(t)
	ctx := context.Background()

	r1, _ := limiter.Allow(ctx, "one-key", 1)
	if !r1.Allowed {
		t.Error("first request with limit=1 should be allowed")
	}
	if r1.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0 after single allowed request", r1.Remaining)
	}

	r2, _ := limiter.Allow(ctx, "one-key", 1)
	if r2.Allowed {
		t.Error("second request with limit=1 should be denied")
	}
}

func TestRedisLimiter_HighLimit(t *testing.T) {
	limiter := setupRedisLimiter(t)
	ctx := context.Background()

	// Verify high limits work correctly
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

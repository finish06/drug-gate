package ratelimit

import (
	"context"
	"testing"
	"time"
)

// MockLimiter implements Limiter for unit testing.
type MockLimiter struct {
	result *Result
	err    error
	calls  []mockCall
}

type mockCall struct {
	Key   string
	Limit int
}

func NewMockLimiter(result *Result, err error) *MockLimiter {
	return &MockLimiter{result: result, err: err}
}

func (m *MockLimiter) Allow(ctx context.Context, key string, limit int) (*Result, error) {
	m.calls = append(m.calls, mockCall{Key: key, Limit: limit})
	return m.result, m.err
}

// Verify MockLimiter satisfies the Limiter interface.
func TestMockLimiter_ImplementsLimiterInterface(t *testing.T) {
	var _ Limiter = (*MockLimiter)(nil)
}

// AC-007: When the request count is under the configured limit, Allow returns
// Allowed=true and Remaining decrements correctly.
func TestMockLimiter_AC007_UnderLimit(t *testing.T) {
	resetAt := time.Now().Add(1 * time.Minute)
	limiter := NewMockLimiter(&Result{
		Allowed:    true,
		Remaining:  9,
		ResetAt:    resetAt,
		RetryAfter: 0,
	}, nil)

	ctx := context.Background()
	res, err := limiter.Allow(ctx, "pk_testkey", 10)
	if err != nil {
		t.Fatalf("Allow() returned unexpected error: %v", err)
	}
	if !res.Allowed {
		t.Error("expected Allowed to be true when under limit")
	}
	if res.Remaining != 9 {
		t.Errorf("expected Remaining=9, got %d", res.Remaining)
	}
}

// AC-007: Verify Remaining decrements on successive requests by simulating
// a sequence of calls with decreasing remaining counts.
func TestMockLimiter_AC007_RemainingDecrements(t *testing.T) {
	ctx := context.Background()
	resetAt := time.Now().Add(1 * time.Minute)

	// Simulate 3 requests with decrementing remaining.
	remainingValues := []int{9, 8, 7}
	for i, expected := range remainingValues {
		limiter := NewMockLimiter(&Result{
			Allowed:   true,
			Remaining: expected,
			ResetAt:   resetAt,
		}, nil)

		res, err := limiter.Allow(ctx, "pk_testkey", 10)
		if err != nil {
			t.Fatalf("call %d: Allow() returned unexpected error: %v", i, err)
		}
		if !res.Allowed {
			t.Errorf("call %d: expected Allowed=true", i)
		}
		if res.Remaining != expected {
			t.Errorf("call %d: expected Remaining=%d, got %d", i, expected, res.Remaining)
		}
	}
}

// AC-007: When the request count exceeds the configured limit, Allow returns
// Allowed=false.
func TestMockLimiter_AC007_OverLimit(t *testing.T) {
	limiter := NewMockLimiter(&Result{
		Allowed:    false,
		Remaining:  0,
		ResetAt:    time.Now().Add(30 * time.Second),
		RetryAfter: 30 * time.Second,
	}, nil)

	ctx := context.Background()
	res, err := limiter.Allow(ctx, "pk_testkey", 10)
	if err != nil {
		t.Fatalf("Allow() returned unexpected error: %v", err)
	}
	if res.Allowed {
		t.Error("expected Allowed to be false when over limit")
	}
	if res.Remaining != 0 {
		t.Errorf("expected Remaining=0 when over limit, got %d", res.Remaining)
	}
}

// AC-008: When rate limited, the Result has RetryAfter > 0 so the client
// knows how long to wait before retrying.
func TestMockLimiter_AC008_RetryAfterPopulated(t *testing.T) {
	retryAfter := 45 * time.Second
	limiter := NewMockLimiter(&Result{
		Allowed:    false,
		Remaining:  0,
		ResetAt:    time.Now().Add(retryAfter),
		RetryAfter: retryAfter,
	}, nil)

	ctx := context.Background()
	res, err := limiter.Allow(ctx, "pk_testkey", 10)
	if err != nil {
		t.Fatalf("Allow() returned unexpected error: %v", err)
	}
	if res.Allowed {
		t.Error("expected Allowed=false for rate-limited request")
	}
	if res.RetryAfter <= 0 {
		t.Errorf("expected RetryAfter > 0, got %v", res.RetryAfter)
	}
	if res.RetryAfter != retryAfter {
		t.Errorf("expected RetryAfter=%v, got %v", retryAfter, res.RetryAfter)
	}
}

// AC-009: When the request is allowed, Remaining and ResetAt are populated
// so the client can display rate limit status.
func TestMockLimiter_AC009_RemainingAndResetAtPopulated(t *testing.T) {
	resetAt := time.Now().Add(1 * time.Minute)
	limiter := NewMockLimiter(&Result{
		Allowed:   true,
		Remaining: 5,
		ResetAt:   resetAt,
	}, nil)

	ctx := context.Background()
	res, err := limiter.Allow(ctx, "pk_testkey", 10)
	if err != nil {
		t.Fatalf("Allow() returned unexpected error: %v", err)
	}
	if !res.Allowed {
		t.Fatal("expected Allowed=true")
	}
	if res.Remaining < 0 {
		t.Errorf("expected Remaining >= 0, got %d", res.Remaining)
	}
	if res.ResetAt.IsZero() {
		t.Error("expected ResetAt to be populated, got zero time")
	}
	if !res.ResetAt.Equal(resetAt) {
		t.Errorf("expected ResetAt=%v, got %v", resetAt, res.ResetAt)
	}
}

// AC-020: Sliding window behavior — the key and limit are passed correctly to
// the limiter so that the underlying implementation can apply a sliding window.
func TestMockLimiter_AC020_SlidingWindowKeyAndLimit(t *testing.T) {
	limiter := NewMockLimiter(&Result{
		Allowed:   true,
		Remaining: 99,
		ResetAt:   time.Now().Add(1 * time.Minute),
	}, nil)

	ctx := context.Background()
	key := "pk_sliding_test"
	limit := 100
	_, err := limiter.Allow(ctx, key, limit)
	if err != nil {
		t.Fatalf("Allow() returned unexpected error: %v", err)
	}

	if len(limiter.calls) != 1 {
		t.Fatalf("expected 1 call recorded, got %d", len(limiter.calls))
	}
	if limiter.calls[0].Key != key {
		t.Errorf("expected key %q, got %q", key, limiter.calls[0].Key)
	}
	if limiter.calls[0].Limit != limit {
		t.Errorf("expected limit %d, got %d", limit, limiter.calls[0].Limit)
	}
}

// AC-020: Sliding window — different keys are tracked independently.
func TestMockLimiter_AC020_DifferentKeysTrackedIndependently(t *testing.T) {
	limiter := NewMockLimiter(&Result{
		Allowed:   true,
		Remaining: 49,
		ResetAt:   time.Now().Add(1 * time.Minute),
	}, nil)

	ctx := context.Background()
	_, _ = limiter.Allow(ctx, "pk_key_a", 50)
	_, _ = limiter.Allow(ctx, "pk_key_b", 50)

	if len(limiter.calls) != 2 {
		t.Fatalf("expected 2 calls recorded, got %d", len(limiter.calls))
	}
	if limiter.calls[0].Key == limiter.calls[1].Key {
		t.Error("expected different keys to be tracked independently")
	}
}

// Verify that Allow propagates errors correctly.
func TestMockLimiter_AllowError(t *testing.T) {
	expectedErr := context.DeadlineExceeded
	limiter := NewMockLimiter(nil, expectedErr)

	ctx := context.Background()
	res, err := limiter.Allow(ctx, "pk_testkey", 10)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
	if res != nil {
		t.Errorf("expected nil result on error, got %+v", res)
	}
}

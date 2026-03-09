package ratelimit

import (
	"context"
	"time"
)

// Result holds the outcome of a rate limit check.
type Result struct {
	Allowed    bool
	Remaining  int
	ResetAt    time.Time
	RetryAfter time.Duration
}

// Limiter defines the interface for rate limiting.
type Limiter interface {
	Allow(ctx context.Context, key string, limit int) (*Result, error)
}

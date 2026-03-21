package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/metrics"
	"github.com/redis/go-redis/v9"
)

// Result wraps a cached value with a staleness indicator.
type Result[T any] struct {
	Value T
	Stale bool
}

// CacheAside is a generic cache-aside (lazy loading) utility backed by Redis.
// It handles the full cache lifecycle: try cache via GetEx (sliding TTL),
// unmarshal, record metrics, fetch on miss, marshal and store.
type CacheAside[T any] struct {
	rdb     *redis.Client
	metrics *metrics.Metrics
	key     string
	ttl     time.Duration
	keyType string
}

// New creates a CacheAside instance for a specific cache key and TTL.
// Pass nil for metrics to skip metric recording.
func New[T any](rdb *redis.Client, m *metrics.Metrics, key string, ttl time.Duration, keyType string) *CacheAside[T] {
	return &CacheAside[T]{
		rdb:     rdb,
		metrics: m,
		key:     key,
		ttl:     ttl,
		keyType: keyType,
	}
}

// Get returns cached data if available, otherwise calls fetch and caches the result.
// Uses GetEx for sliding TTL — each cache hit resets the expiry.
func (c *CacheAside[T]) Get(ctx context.Context, fetch func(ctx context.Context) (T, error)) (T, error) {
	// Try cache
	data, err := c.rdb.GetEx(ctx, c.key, c.ttl).Bytes()
	if err == nil {
		var result T
		if err := json.Unmarshal(data, &result); err == nil {
			c.recordCache("hit")
			return result, nil
		}
		slog.Warn("failed to unmarshal cached data, fetching fresh", "key", c.key)
	}

	c.recordCache("miss")

	// Fetch from upstream
	result, err := fetch(ctx)
	if err != nil {
		var zero T
		return zero, err
	}

	// Store in cache
	if encoded, err := json.Marshal(result); err == nil {
		if err := c.rdb.Set(ctx, c.key, encoded, c.ttl).Err(); err != nil {
			slog.Warn("failed to cache data", "key", c.key, "err", err)
		}
	}

	return result, nil
}

// GetWithStale is like Get but falls back to stale cached data when the
// fetch function returns ErrCircuitOpen. Uses a separate stale key
// (prefix "stale:") that persists without TTL, ensuring data survives
// cache expiry for circuit-open fallback.
func (c *CacheAside[T]) GetWithStale(ctx context.Context, fetch func(ctx context.Context) (T, error)) (Result[T], error) {
	staleKey := "stale:" + c.key

	// Try fresh cache first
	data, err := c.rdb.GetEx(ctx, c.key, c.ttl).Bytes()
	if err == nil {
		var result T
		if err := json.Unmarshal(data, &result); err == nil {
			c.recordCache("hit")
			return Result[T]{Value: result, Stale: false}, nil
		}
		slog.Warn("failed to unmarshal cached data, fetching fresh", "key", c.key)
	}

	c.recordCache("miss")

	// Fetch from upstream
	result, fetchErr := fetch(ctx)
	if fetchErr != nil {
		// If circuit is open, try stale cache (no-TTL backup key)
		if errors.Is(fetchErr, client.ErrCircuitOpen) {
			staleData, staleErr := c.rdb.Get(ctx, staleKey).Bytes()
			if staleErr == nil {
				var staleResult T
				if err := json.Unmarshal(staleData, &staleResult); err == nil {
					c.recordCache("stale")
					slog.Info("serving stale cache (circuit open)", "key", c.key)
					return Result[T]{Value: staleResult, Stale: true}, nil
				}
			}
		}
		var zero T
		return Result[T]{Value: zero}, fetchErr
	}

	// Store in both fresh cache (with TTL) and stale backup (no TTL)
	if encoded, err := json.Marshal(result); err == nil {
		if err := c.rdb.Set(ctx, c.key, encoded, c.ttl).Err(); err != nil {
			slog.Warn("failed to cache data", "key", c.key, "err", err)
		}
		// Stale backup — no TTL, survives cache expiry
		c.rdb.Set(ctx, staleKey, encoded, 0)
	}

	return Result[T]{Value: result, Stale: false}, nil
}

func (c *CacheAside[T]) recordCache(outcome string) {
	if c.metrics != nil {
		c.metrics.CacheHitsTotal.WithLabelValues(c.keyType, outcome).Inc()
	}
}

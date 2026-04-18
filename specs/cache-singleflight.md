# Spec: Cache Singleflight (Stampede Prevention)

**Version:** 1.0
**Created:** 2026-03-27
**PRD Reference:** docs/prd.md (Engineering Backlog S-001)
**Status:** Complete
**Milestone:** Backlog — Critical

## 1. Overview

Add `golang.org/x/sync/singleflight` to the generic `CacheAside[T].Get` method to prevent thundering herd on cache TTL expiry. When a cache key expires, only one goroutine should fetch from upstream — all concurrent callers for the same key wait and share the result.

### User Story

As a **service operator**, I want **cache miss fetches to be deduplicated across concurrent requests**, so that **a single TTL expiry doesn't generate hundreds of identical upstream calls that could overwhelm cash-drugs**.

## 2. Problem

`CacheAside[T].Get` (in `internal/cache/aside.go`) is called by every cached service method. When a Redis key expires:

1. All concurrent requests see a cache miss
2. Each independently calls the `fetch` function (upstream HTTP call)
3. Each stores the same result back to Redis

For `cache:drugnames` (7.4MB payload), at 1K concurrent requests this means 1,000 simultaneous 7.4MB HTTP fetches from cash-drugs — effectively a self-inflicted DDoS on the upstream.

The autocomplete index (`internal/service/drugdata.go:141-174`) already has stampede prevention via `TryLock`. But the underlying `CacheAside` layer that powers **all 11 cached service methods** does not.

## 3. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | `CacheAside[T].Get` uses `singleflight.Group` to coalesce concurrent fetches for the same cache key | Must |
| AC-002 | Only 1 upstream call is made when N concurrent requests hit a cold/expired cache key | Must |
| AC-003 | The N-1 waiting callers receive the same result as the fetcher | Must |
| AC-004 | If the fetch fails, the error is returned to all waiting callers (no silent swallowing) | Must |
| AC-005 | `GetWithStale` (if wired up) also uses singleflight for its fetch path | Should |
| AC-006 | Singleflight key matches the Redis cache key (one group per `CacheAside` instance) | Must |
| AC-007 | No performance regression on cache hits (singleflight only activates on miss) | Must |
| AC-008 | Unit test demonstrates that N concurrent cache misses produce exactly 1 fetch call | Must |

## 4. User Test Cases

### TC-001: Concurrent cache miss produces single upstream call
1. Set up `CacheAside` with a mock fetch that counts invocations
2. Launch 100 goroutines calling `Get` simultaneously with an empty cache
3. Assert fetch was called exactly 1 time
4. Assert all 100 goroutines received the same result

### TC-002: Cache hit bypasses singleflight entirely
1. Pre-populate Redis with cached data
2. Call `Get` — should return cached data without invoking fetch
3. Verify no singleflight overhead on hot path

### TC-003: Fetch error propagates to all waiters
1. Set up `CacheAside` with a fetch that returns an error
2. Launch 50 goroutines calling `Get` simultaneously
3. Assert all 50 receive the error
4. Assert fetch was called exactly 1 time

## 5. Technical Approach

```go
import "golang.org/x/sync/singleflight"

type CacheAside[T any] struct {
    rdb     *redis.Client
    metrics *metrics.Metrics
    key     string
    ttl     time.Duration
    keyType string
    group   singleflight.Group
}

func (c *CacheAside[T]) Get(ctx context.Context, fetch func(context.Context) (T, error)) (T, error) {
    // 1. Try cache hit (unchanged — no singleflight overhead)
    cached, err := c.rdb.Get(ctx, c.key).Result()
    if err == nil {
        // ... unmarshal and return (existing code)
    }

    // 2. Cache miss — coalesce concurrent fetches
    val, err, _ := c.group.Do(c.key, func() (any, error) {
        result, err := fetch(ctx)
        if err != nil {
            return result, err
        }
        // Store in Redis (existing code)
        data, _ := json.Marshal(result)
        c.rdb.Set(ctx, c.key, data, c.ttl)
        return result, nil
    })
    if err != nil {
        var zero T
        return zero, err
    }
    return val.(T), nil
}
```

## 6. Affected Files

| File | Change |
|------|--------|
| `internal/cache/aside.go` | Add `singleflight.Group` field, wrap fetch in `group.Do` |
| `internal/cache/aside_test.go` | Add concurrent miss test (TC-001, TC-002, TC-003) |
| `go.mod` | Add `golang.org/x/sync` dependency |

## 7. Edge Cases

- **Context cancellation:** If the fetching goroutine's context is cancelled, singleflight returns the error to all waiters. This is correct — they should retry on next request.
- **Stale key constructor:** `CacheAside` is created per-call in service methods (e.g., `cache.New[T](...)`). The `singleflight.Group` must be shared across calls for the same key. Consider making it a package-level map or passing it via constructor.
- **Different contexts:** Concurrent callers may have different request contexts. Singleflight uses the first caller's context for the fetch. If that context has a shorter timeout, the fetch may fail prematurely for all waiters. Mitigate by using `context.WithoutCancel` or a detached context for the fetch.

## 8. Dependencies

- `golang.org/x/sync/singleflight` — standard Go extended library, no external risk

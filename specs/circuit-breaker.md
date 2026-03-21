# Spec: Circuit Breaker + Upstream Resilience

**Version:** 0.1.0
**Created:** 2026-03-21
**PRD Reference:** docs/prd.md — M9: Upstream Resilience + Production Deploy
**Status:** Complete

## 1. Overview

Add a circuit breaker to the cash-drugs HTTP client that prevents cascading failures when the upstream is unhealthy. When the circuit is open, serve stale cached data instead of returning 502. Also parallelize the interaction checker and limit upstream response sizes.

### User Story

As an **operator**, I want **drug-gate to gracefully degrade when cash-drugs is down**, so that **frontend applications continue serving cached data instead of showing errors**.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | Circuit breaker starts in Closed state (requests pass through) | Must |
| AC-002 | Circuit opens after 10 consecutive upstream failures | Must |
| AC-003 | Open circuit rejects requests with `ErrCircuitOpen` | Must |
| AC-004 | Circuit transitions to Half-Open after 30s cooldown | Must |
| AC-005 | Half-Open: success → Closed (reset failure counter) | Must |
| AC-006 | Half-Open: failure → Open (reset cooldown timer) | Must |
| AC-007 | Success resets consecutive failure counter (non-consecutive failures don't trip) | Must |
| AC-008 | Thread-safe for concurrent access | Must |
| AC-009 | When circuit is open, CacheAside serves expired cached data | Must |
| AC-010 | Stale responses include `X-Cache-Stale: true` header | Must |
| AC-011 | Health endpoint reports "degraded" when circuit is open | Must |
| AC-012 | When no cached data exists and circuit is open, return 503 | Must |
| AC-013 | Interaction checker resolves drugs in parallel via errgroup | Must |
| AC-014 | Parallel concurrency capped at 5 | Must |
| AC-015 | MaxBytesReader limits upstream response body to 5MB | Must |
| AC-016 | Circuit breaker state exposed via Prometheus metric | Should |

## 3. Circuit Breaker States

```
         success
    ┌─────────────┐
    │             │
    ▼             │
 CLOSED ──────► OPEN ──── 30s cooldown ────► HALF-OPEN
    ▲         (10 fails)                        │
    │                                           │
    └───────── success ◄────────────────────────┘
                                    failure → back to OPEN
```

## 4. Implementation Notes

### Circuit Breaker (`internal/client/breaker.go`)
- `CircuitBreaker` struct with Closed/Open/HalfOpen states
- `Execute(fn func() error) error` — wraps any function with circuit protection
- Thread-safe via `sync.Mutex`
- Configurable: `maxFailures`, `cooldownDuration`

### Stale-Cache (`internal/cache/aside.go`)
- Extend `CacheAside[T].Get()` to accept `allowStale bool`
- When fetch fails with `ErrCircuitOpen` and `allowStale` is true, try `rdb.Get()` (no TTL check)
- Return a `StaleResult[T]` or use a context/header signal

### MaxBytesReader
- Wrap `resp.Body` with `io.LimitReader(resp.Body, 5<<20)` in client methods
- Return `ErrUpstream` if response exceeds limit

### errgroup for interactions
- Replace sequential loop with `errgroup.Group` + `SetLimit(5)`
- Collect results into `sync.Mutex`-protected slice

## 5. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| First request after startup | Circuit is Closed, request passes |
| 9 failures then 1 success | Counter resets, circuit stays Closed |
| Circuit open + no cached data | Return 503 with clear error message |
| Half-open probe fails | Re-open circuit, reset cooldown |
| Concurrent goroutines hitting breaker | Mutex protects state transitions |
| MaxBytesReader triggers on SPL XML | 5MB is 25x typical SPL size, unlikely |

## 6. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-21 | 0.1.0 | calebdunn | Initial spec |

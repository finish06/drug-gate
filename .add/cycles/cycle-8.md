# Cycle 8 — Upstream Resilience

**Milestone:** M9 — Upstream Resilience + Production Deploy
**Maturity:** Beta
**Status:** PLANNED
**Started:** TBD
**Completed:** TBD
**Duration Budget:** 7 hours (away mode)

## Work Items

| Feature | Current Pos | Target Pos | Assigned | Est. Effort | Validation |
|---------|-------------|-----------|----------|-------------|------------|
| Circuit Breaker | SHAPED | VERIFIED | Agent | ~3h | Spec + breaker + tests (open/closed/half-open) |
| Stale-Cache Serving | SHAPED | VERIFIED | Agent | ~1.5h | X-Cache-Stale header + degraded health |
| Parallel Interaction Checker | SHAPED | VERIFIED | Agent | ~1h | errgroup with cap 5, tests |
| MaxBytesReader | SHAPED | VERIFIED | Agent | ~30m | 5MB limit on all upstream calls |
| Documentation | — | DONE | Agent | ~1h | Spec, Swagger, changelog, learnings, PR |

## Dependencies & Serialization

```
Phase 1: Spec
    ↓
Phase 2: Circuit Breaker (foundational — wraps HTTP client)
    ↓
Phase 3: Stale-Cache Serving (depends on circuit breaker state)
    ↓
Phase 4: MaxBytesReader (independent, applied to client)
    ↓
Phase 5: Parallel Interaction Checker (uses circuit-aware client)
    ↓
Phase 6: Finalize (tests, Swagger, docs, PR)
```

## Execution Plan

### Phase 1: Spec (~30m)

Write `specs/circuit-breaker.md` covering:
- Circuit breaker states: closed → open (after 10 failures) → half-open (after 30s) → closed (on success)
- Stale-cache: CacheAside[T] serves expired data when circuit is open
- X-Cache-Stale: true header on stale responses
- Health endpoint reports "degraded" when circuit is open
- errgroup with bounded concurrency (5) for interaction checker
- MaxBytesReader (5MB) on all upstream HTTP responses

### Phase 2: Circuit Breaker (~3h)

**RED:**
1. Test circuit starts closed (requests pass through)
2. Test circuit opens after 10 consecutive failures
3. Test circuit rejects requests when open (returns error)
4. Test circuit transitions to half-open after 30s cooldown
5. Test half-open: success → closes circuit
6. Test half-open: failure → re-opens circuit
7. Test non-consecutive failures don't trip (success resets counter)
8. Test concurrent access is safe

**GREEN:**
1. Create `internal/client/breaker.go`:
   - `CircuitBreaker` struct with states: Closed, Open, HalfOpen
   - `Allow() bool` — checks if request should proceed
   - `RecordSuccess()` / `RecordFailure()` — update state
   - Thread-safe via sync.Mutex
2. Wrap the HTTP client methods with circuit breaker checks
3. Return a specific `ErrCircuitOpen` when circuit is open

### Phase 3: Stale-Cache Serving (~1.5h)

**RED:**
1. Test: when circuit is open, CacheAside serves expired cached data
2. Test: stale response includes X-Cache-Stale: true header
3. Test: health endpoint returns "degraded" when circuit is open
4. Test: when no cached data exists and circuit is open, return 503

**GREEN:**
1. Extend CacheAside[T] to accept a `fallbackToStale` flag
2. When fetch fails with ErrCircuitOpen, try Redis GET (without TTL check)
3. If stale data found, return it + signal staleness
4. Wire X-Cache-Stale header in middleware or handler layer
5. Update health handler to check circuit breaker state

### Phase 4: MaxBytesReader (~30m)

1. Add `io.LimitReader` or `http.MaxBytesReader` wrapper in HTTP client
2. Apply 5MB limit to all response body reads
3. Test: responses > 5MB return error

### Phase 5: Parallel Interaction Checker (~1h)

**RED:**
1. Test: interaction checker resolves drugs in parallel
2. Test: concurrency capped at 5 (6th drug waits)
3. Test: one drug failure doesn't cancel others
4. Test: context cancellation stops all goroutines

**GREEN:**
1. Replace sequential loop in CheckInteractions with errgroup + semaphore
2. Use `golang.org/x/sync/errgroup` with `SetLimit(5)`
3. Collect results into thread-safe slice

### Phase 6: Finalize (~1h)

1. Update PRD: circuit breaker threshold 5 → 10
2. Run full test suite, verify coverage > 80%
3. Update Swagger if needed
4. Write learning checkpoint
5. Update changelog
6. Create PR

## Validation Criteria

- [ ] Circuit breaker opens after 10 consecutive failures
- [ ] Circuit auto-recovers via half-open after 30s
- [ ] Stale-cache served when circuit open, with X-Cache-Stale header
- [ ] Health reports degraded when circuit open
- [ ] Interaction checker runs parallel with cap 5
- [ ] Upstream responses capped at 5MB
- [ ] Coverage stays above 80%
- [ ] No regressions
- [ ] PR created

## Agent Autonomy (Away Mode)

**Autonomous:** Write spec, TDD, commit per phase, push, create PR, update PRD.
**Boundaries:** Do NOT merge. Do NOT deploy. If circuit breaker design has multiple valid approaches, pick the simplest and document alternatives.

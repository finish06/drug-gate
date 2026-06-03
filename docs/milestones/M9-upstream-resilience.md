# M9 — Upstream Resilience

**Goal:** Eliminate single points of failure in the cash-drugs upstream path.

**Status:** DONE
**Target Maturity:** beta
**Appetite:** 2 weeks
**Started:** 2026-03-21
**Shipped:** v0.8.0

> Production-deploy automation was originally bundled into this milestone. It has
> been split out to **M9.5 — Production Deploy** (see `docs/milestones/M9.5-production-deploy.md`).

## Success Criteria

- [x] Circuit breaker trips after 10 consecutive upstream failures, serves stale cache
- [x] Circuit auto-recovers via half-open probe after 30s cooldown
- [x] Stale-cache responses include X-Cache-Stale header, health reports degraded
- [x] Multi-drug interaction checker runs parallel upstream calls via errgroup (cap 5)
- [x] MaxBytesReader limits upstream response size to 5MB
- [x] 80%+ test coverage on new code

## Hill Chart

| Feature | Position | Notes |
|---------|----------|-------|
| Circuit Breaker | DONE | Wraps cash-drugs HTTP client |
| Stale-Cache Serving | DONE | Returns expired cached data when circuit open |
| Parallel Interaction Checker | DONE | errgroup with concurrency cap of 5 |
| MaxBytesReader | DONE | 5MB limit on upstream responses |

## Features

| Feature | Spec | Current Position | Target |
|---------|------|-----------------|--------|
| Circuit Breaker | specs/circuit-breaker.md | DONE | VERIFIED |
| Stale-Cache Serving | specs/circuit-breaker.md | DONE | VERIFIED |
| Parallel Interaction Checker | specs/circuit-breaker.md | DONE | VERIFIED |
| MaxBytesReader | specs/circuit-breaker.md | DONE | VERIFIED |

## Dependencies

- Circuit breaker wraps the existing HTTP client — foundational
- Stale-cache depends on circuit breaker state (open → serve stale)
- errgroup depends on circuit breaker (respects circuit state)
- MaxBytesReader is independent (can be added to client in any order)

## Risks

| Risk | Mitigation |
|------|-----------|
| Circuit breaker too aggressive — trips on transient errors | 10 consecutive failures threshold, 30s cooldown |
| Stale cache data too old — misleading clinical info | X-Cache-Stale header lets clients decide, short stale window |
| errgroup goroutine leak | Context cancellation + bounded concurrency (5) |
| MaxBytesReader breaks large SPL XMLs | 5MB limit is 25x typical SPL size (~200KB) |

## Cycles

| Cycle | Features | Status | Notes |
|-------|----------|--------|-------|
| cycle-8 | Circuit Breaker + Stale-Cache + errgroup + MaxBytesReader | DONE | 7h away session; shipped v0.8.0 |

## Retrospective

Resilience work landed cleanly in cycle-8 (see learnings L-019, L-020). The
`io.NopCloser` connection-reuse bug (L-019) was the main catch. Production-deploy
automation was deferred and is now tracked separately as M9.5.

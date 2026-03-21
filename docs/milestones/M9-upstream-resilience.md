# M9 — Upstream Resilience + Production Deploy

**Goal:** Eliminate single points of failure and establish production-grade deployment.

**Status:** IN_PROGRESS
**Target Maturity:** GA candidate
**Appetite:** 2 weeks
**Started:** 2026-03-21

## Success Criteria

- [ ] Circuit breaker trips after 10 consecutive upstream failures, serves stale cache
- [ ] Circuit auto-recovers via half-open probe after 30s cooldown
- [ ] Stale-cache responses include X-Cache-Stale header, health reports degraded
- [ ] Multi-drug interaction checker runs parallel upstream calls via errgroup (cap 5)
- [ ] MaxBytesReader limits upstream response size to 5MB
- [ ] Production deploy pinned to version tags, triggered by GH Actions
- [ ] Health gate verifies deployment before promoting
- [ ] One-command rollback documented and tested
- [ ] 80%+ test coverage on new code

## Hill Chart

| Feature | Position | Notes |
|---------|----------|-------|
| Circuit Breaker | SHAPED | Wraps cash-drugs HTTP client |
| Stale-Cache Serving | SHAPED | Return expired cached data when circuit open |
| Parallel Interaction Checker | SHAPED | errgroup with concurrency cap of 5 |
| MaxBytesReader | SHAPED | 5MB limit on upstream responses |
| Production Deploy Automation | SHAPED | Deferred to cycle-9 |

## Features

| Feature | Spec | Current Position | Target |
|---------|------|-----------------|--------|
| Circuit Breaker | specs/circuit-breaker.md | SHAPED | VERIFIED |
| Stale-Cache Serving | specs/circuit-breaker.md | SHAPED | VERIFIED |
| Parallel Interaction Checker | specs/circuit-breaker.md | SHAPED | VERIFIED |
| MaxBytesReader | specs/circuit-breaker.md | SHAPED | VERIFIED |
| Production Deploy Automation | specs/deploy-automation.md | SHAPED | VERIFIED |

## Dependencies

- Circuit breaker wraps the existing HTTP client — foundational
- Stale-cache depends on circuit breaker state (open → serve stale)
- errgroup depends on circuit breaker (respects circuit state)
- MaxBytesReader is independent (can be added to client in any order)
- Deploy automation is independent (deferred to next cycle)

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
| cycle-8 | Circuit Breaker + Stale-Cache + errgroup + MaxBytesReader | PLANNED | 7h away session |

# M7 — Operational Hardening

**Goal:** Stabilize operations with Redis persistence, request tracing, alerting, and ship the highest-value quick-win feature (drug autocomplete).

**Status:** IN_PROGRESS
**Target Maturity:** Beta
**Appetite:** 2 weeks
**Started:** 2026-03-20

## Success Criteria

- [ ] Redis AOF enabled, nightly snapshot cron running, restore procedure tested
- [ ] X-Request-ID present in all responses and correlated in logs
- [ ] Prometheus alert rules firing correctly for error rate > 5%, p95 latency > 500ms, Redis unreachable
- [ ] Autocomplete endpoint returns results in < 50ms for cached data
- [ ] 80%+ test coverage on new code

## Hill Chart

| Feature | Position | Notes |
|---------|----------|-------|
| Request ID Correlation | SHAPED | Middleware + slog integration |
| Drug Autocomplete | SHAPED | Reuses existing drug names cache |
| Redis Persistence | SHAPED | docker-compose + staging config + prod docs |
| Prometheus Alert Rules | SHAPED | Rules file + ops documentation |

## Features

| Feature | Spec | Current Position | Target |
|---------|------|-----------------|--------|
| Request ID Correlation | specs/request-id.md | SHAPED | VERIFIED |
| Drug Autocomplete | specs/drug-autocomplete.md | SHAPED | VERIFIED |
| Redis Persistence + Backup | specs/redis-persistence.md | SHAPED | VERIFIED |
| Prometheus Alert Rules | specs/prometheus-alerts.md | SHAPED | VERIFIED |

## Dependencies

- Request ID middleware is foundational (all other features benefit from correlated logs)
- Autocomplete reuses existing drug names cache (GetDrugNames from M3)
- Redis Persistence is independent infrastructure work
- Alert Rules depend on existing Prometheus metrics (already defined in internal/metrics/)

## Risks

| Risk | Mitigation |
|------|-----------|
| AOF persistence adds disk I/O to Redis | Use `appendfsync everysec` (default), benchmark locally first |
| Autocomplete latency on cold cache | First request triggers cache load (~2s), subsequent requests sub-50ms |
| Alert rule thresholds may be too sensitive | Start conservative, tune based on staging observation |

## Cycles

| Cycle | Features | Status | Notes |
|-------|----------|--------|-------|
| cycle-3 | Request ID + Autocomplete + Redis Persistence + Alert Rules | PLANNED | 2-3 day away session |

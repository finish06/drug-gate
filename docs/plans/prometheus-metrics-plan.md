# Implementation Plan: Prometheus Metrics

**Spec Version:** 0.1.0
**Created:** 2026-03-14
**Team Size:** Solo (agent-driven)
**Estimated Duration:** 2-3 days

## Overview

Add Prometheus metrics instrumentation to drug-gate: HTTP request counters/histograms, Redis cache counters, auth/rate-limit rejection counters, Redis health background collector, and container system metrics via procfs. Follow cash-drugs patterns exactly.

## Implementation Phases

### Phase 1: Metrics Core + HTTP Middleware

Foundation: `Metrics` struct, `NewMetrics()`, HTTP instrumentation middleware, `/metrics` endpoint.

| Task ID | Description | ACs | Effort | Dependencies |
|---------|-------------|-----|--------|--------------|
| TASK-001 | `go get github.com/prometheus/client_golang` | AC-028 | 5min | — |
| TASK-002 | Create `internal/metrics/metrics.go` — `Metrics` struct with all collectors, `NewMetrics(reg)` | AC-003, AC-004, AC-006, AC-008, AC-009, AC-010, AC-011, AC-014, AC-015–AC-023, AC-028 | 1h | TASK-001 |
| TASK-003 | Create `internal/middleware/metrics.go` — Chi middleware that records `HTTPRequestsTotal` and `HTTPRequestDuration` using `chi.RouteContext().RoutePattern()` for route label | AC-003, AC-004, AC-005, AC-030 | 45min | TASK-002 |
| TASK-004 | Wire into `cmd/server/main.go`: create `Metrics`, register middleware on router, add `r.Handle("/metrics", promhttp.Handler())` as public route | AC-001, AC-002, AC-013, AC-030 | 30min | TASK-002, TASK-003 |
| TASK-005 | Unit tests for metrics middleware — verify counter/histogram labels, status code capture, route pattern usage | AC-003, AC-004, AC-005 | 1h | TASK-003 |

### Phase 2: Auth, Rate Limit, and Cache Instrumentation

Add metric recording to existing middleware and service code.

| Task ID | Description | ACs | Effort | Dependencies |
|---------|-------------|-----|--------|--------------|
| TASK-006 | Modify `internal/middleware/auth.go` — accept `*Metrics`, record `AuthRejectionsTotal` with reason label (missing, invalid, inactive) | AC-009 | 30min | TASK-002 |
| TASK-007 | Modify `internal/middleware/ratelimit.go` — accept `*Metrics`, record `RateLimitRejectionsTotal` with api_key label on 429 | AC-008 | 30min | TASK-002 |
| TASK-008 | Modify `internal/service/drugdata.go` — accept `*Metrics`, record `CacheHitsTotal` with key_type and outcome labels in `GetDrugNames`, `GetDrugClasses`, `GetDrugsByClass` | AC-006, AC-007 | 45min | TASK-002 |
| TASK-009 | Update `cmd/server/main.go` — pass `*Metrics` to modified middleware and service constructors | — | 15min | TASK-006, TASK-007, TASK-008 |
| TASK-010 | Update existing tests — fix middleware/service constructor signatures, verify metric recording | AC-029 | 1h | TASK-006, TASK-007, TASK-008 |

### Phase 3: Redis Health Collector

Background goroutine that pings Redis every 30s.

| Task ID | Description | ACs | Effort | Dependencies |
|---------|-------------|-----|--------|--------------|
| TASK-011 | Create `internal/metrics/redis_collector.go` — `RedisCollector` with `Start()`/`Stop()` lifecycle, 30s ping interval, sets `RedisUp` and `RedisPingDuration` | AC-010, AC-011, AC-012 | 45min | TASK-002 |
| TASK-012 | Unit tests for `RedisCollector` using miniredis — verify healthy/unhealthy states, ping duration recording, lifecycle | AC-010, AC-011, AC-012 | 1h | TASK-011 |
| TASK-013 | Wire `RedisCollector` into `main.go` — `Start()` after Redis client init, `Stop()` in shutdown | AC-012 | 15min | TASK-011 |

### Phase 4: Container System Metrics

Linux-only procfs/cgroup metrics with background collector.

| Task ID | Description | ACs | Effort | Dependencies |
|---------|-------------|-----|--------|--------------|
| TASK-014 | Create `internal/metrics/system.go` — `SystemSource` interface (`CPUUsage`, `MemoryInfo`, `DiskUsage`, `NetworkStats`), data structs (`MemInfo`, `DiskInfo`, `NetStat`) | AC-015–AC-023 | 30min | TASK-002 |
| TASK-015 | Create `internal/metrics/system_procfs.go` with `//go:build linux` — `ProcfsSource` implementation reading `/proc/self/stat`, `/proc/self/status`, `/proc/net/dev`, `/sys/fs/cgroup/memory.max`, `syscall.Statfs`, `syscall.Getrusage` | AC-015–AC-023, AC-026 | 2h | TASK-014 |
| TASK-016 | Create `internal/metrics/system_collector.go` — `SystemCollector` with `Start()`/`Stop()` lifecycle, configurable interval, panic recovery | AC-024, AC-025, AC-027 | 45min | TASK-014 |
| TASK-017 | Unit tests for `SystemCollector` using mock `SystemSource` — verify all gauge updates, panic recovery, lifecycle | AC-024, AC-027 | 1h | TASK-016 |
| TASK-018 | Unit tests for procfs parsing using file-based fixtures (not live `/proc`) | AC-026 | 1.5h | TASK-015 |
| TASK-019 | Wire `SystemCollector` into `main.go` — parse `SYSTEM_METRICS_INTERVAL` env var, `Start()`/`Stop()` with Linux guard via `runtime.GOOS` | AC-024, AC-025 | 20min | TASK-016 |

### Phase 5: Verification

Full suite validation and cleanup.

| Task ID | Description | ACs | Effort | Dependencies |
|---------|-------------|-----|--------|--------------|
| TASK-020 | Run full test suite — verify no regressions from metrics additions | AC-029 | 15min | All above |
| TASK-021 | Run `go vet ./...` and verify no issues | AC-029 | 5min | TASK-020 |
| TASK-022 | Build Docker image, verify `/metrics` endpoint responds with Prometheus format | AC-001, AC-002 | 15min | TASK-020 |
| TASK-023 | Regenerate swagger docs (`swag init`) | — | 5min | TASK-004 |

## Effort Summary

| Phase | Tasks | Estimated Hours |
|-------|-------|-----------------|
| Phase 1: Core + HTTP | 5 | 3.5h |
| Phase 2: Auth/Rate/Cache | 5 | 3h |
| Phase 3: Redis Collector | 3 | 2h |
| Phase 4: System Metrics | 6 | 6h |
| Phase 5: Verification | 4 | 0.5h |
| **Total** | **23** | **15h** |

## File Changes

### New Files

| File | Purpose |
|------|---------|
| `internal/metrics/metrics.go` | `Metrics` struct, `NewMetrics()` — all Prometheus collectors |
| `internal/metrics/metrics_test.go` | Unit tests for metric registration |
| `internal/metrics/redis_collector.go` | Background Redis health collector |
| `internal/metrics/redis_collector_test.go` | Redis collector tests (miniredis) |
| `internal/metrics/system.go` | `SystemSource` interface, data structs |
| `internal/metrics/system_collector.go` | Background system metrics collector |
| `internal/metrics/system_collector_test.go` | System collector tests (mock source) |
| `internal/metrics/system_procfs.go` | `//go:build linux` — procfs implementation |
| `internal/metrics/system_procfs_test.go` | `//go:build linux` — fixture-based tests |
| `internal/middleware/metrics.go` | HTTP request instrumentation middleware |
| `internal/middleware/metrics_test.go` | Middleware tests |

### Modified Files

| File | Change |
|------|--------|
| `cmd/server/main.go` | Init `Metrics`, wire middleware, add `/metrics` route, start/stop collectors |
| `internal/middleware/auth.go` | Accept optional `*Metrics`, record auth rejections |
| `internal/middleware/ratelimit.go` | Accept optional `*Metrics`, record rate limit rejections |
| `internal/service/drugdata.go` | Accept optional `*Metrics`, record cache hit/miss |
| `go.mod` / `go.sum` | Add `prometheus/client_golang` dependency |

### Existing Test Updates

| File | Change |
|------|--------|
| `internal/middleware/*_test.go` | Update constructor calls for new `*Metrics` parameter |
| `internal/service/drugdata_test.go` | Update constructor calls for new `*Metrics` parameter |
| `internal/handler/*_test.go` | May need mock service signature updates |

## Dependency Graph

```
TASK-001 (go get)
    └─→ TASK-002 (Metrics struct)
            ├─→ TASK-003 (HTTP middleware) ─→ TASK-005 (tests)
            ├─→ TASK-006 (auth instrumentation) ─┐
            ├─→ TASK-007 (ratelimit instrumentation) ─┤
            ├─→ TASK-008 (cache instrumentation) ──┤
            │                                      └─→ TASK-009 (wire) ─→ TASK-010 (fix tests)
            ├─→ TASK-011 (Redis collector) ─→ TASK-012 (tests) ─→ TASK-013 (wire)
            └─→ TASK-014 (SystemSource interface)
                    ├─→ TASK-015 (procfs impl) ─→ TASK-018 (fixture tests)
                    └─→ TASK-016 (SystemCollector) ─→ TASK-017 (tests) ─→ TASK-019 (wire)

TASK-004 (main.go wiring) depends on TASK-002, TASK-003
All ─→ TASK-020 ─→ TASK-021 ─→ TASK-022 ─→ TASK-023
```

## Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Middleware signature changes break existing tests | High | Medium | Phase 2 includes TASK-010 for test updates; use optional `*Metrics` parameter (nil-safe) |
| Procfs parsing edge cases on different Linux distros | Medium | Low | Use fixture-based tests with known file content |
| cgroup v1 vs v2 differences | Medium | Low | Check v2 first, fall back to v1, default to -1 |
| Chi `RoutePattern()` returns empty for unmatched routes | Low | Low | Fall back to request path if pattern empty |

## Testing Strategy

- **Unit tests** (Phase 1-4): Mock interfaces, verify metric values via `prometheus.Registry` test helpers
- **Integration tests** (Phase 3): miniredis for Redis collector
- **Fixture tests** (Phase 4): File-based procfs fixtures for Linux-specific parsing
- **Regression** (Phase 5): Full `go test ./...` to verify no breakage
- **Manual verification** (Phase 5): Docker build + curl `/metrics`

## Spec Traceability

| AC | Tasks |
|----|-------|
| AC-001 | TASK-004, TASK-022 |
| AC-002 | TASK-004 |
| AC-003 | TASK-002, TASK-003, TASK-005 |
| AC-004 | TASK-002, TASK-003, TASK-005 |
| AC-005 | TASK-003, TASK-005 |
| AC-006 | TASK-002, TASK-008 |
| AC-007 | TASK-008 |
| AC-008 | TASK-002, TASK-007 |
| AC-009 | TASK-002, TASK-006 |
| AC-010 | TASK-002, TASK-011, TASK-012 |
| AC-011 | TASK-002, TASK-011, TASK-012 |
| AC-012 | TASK-011, TASK-013 |
| AC-013 | TASK-004 |
| AC-014 | TASK-002 |
| AC-015–AC-023 | TASK-002, TASK-014, TASK-015, TASK-016, TASK-017, TASK-018 |
| AC-024 | TASK-016, TASK-019 |
| AC-025 | TASK-019 |
| AC-026 | TASK-015, TASK-018 |
| AC-027 | TASK-016, TASK-017 |
| AC-028 | TASK-002 |
| AC-029 | TASK-010, TASK-020, TASK-021 |
| AC-030 | TASK-003, TASK-004 |

# Spec: Prometheus Metrics

**Version:** 0.1.0
**Created:** 2026-03-14
**PRD Reference:** docs/prd.md
**Status:** Draft

## 1. Overview

Expose a `/metrics` endpoint in Prometheus exposition format providing full operational observability for drug-gate. Instrument HTTP handlers, Redis cache, rate limiting, authentication, and Redis health with labeled counters, histograms, and gauges. Add container-level system metrics (CPU, memory, disk, network) via procfs. Follow the same patterns established in cash-drugs: single `Metrics` struct, `promhttp.Handler()`, namespace prefix `druggate_`, and background collectors with `Start()`/`Stop()` lifecycle.

### User Story

As an **operator**, I want a Prometheus-compatible `/metrics` endpoint on drug-gate, so that I can monitor request performance, cache efficiency, auth/rate-limit behavior, Redis health, and container resource usage in Grafana without custom log parsing.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | `GET /metrics` returns valid Prometheus exposition format (`text/plain; version=0.0.4`) | Must |
| AC-002 | `/metrics` endpoint does not require API key authentication (public, like `/health`) | Must |
| AC-003 | HTTP request counter `druggate_http_requests_total` with labels `route`, `method`, `status_code` | Must |
| AC-004 | HTTP request duration histogram `druggate_http_request_duration_seconds` with labels `route`, `method` using `prometheus.DefBuckets` | Must |
| AC-005 | `route` label uses Chi's `RouteContext().RoutePattern()` to avoid high cardinality from path params (e.g. `/v1/drugs/ndc/{ndc}` not `/v1/drugs/ndc/00069-3150`) | Must |
| AC-006 | Redis cache outcome counter `druggate_cache_hits_total` with labels `key_type`, `outcome` (hit, miss) | Must |
| AC-007 | `key_type` label values: `drugnames`, `drugclasses`, `drugsbyclass` (matching cache key prefixes) | Must |
| AC-008 | Rate limit rejection counter `druggate_ratelimit_rejections_total` with label `api_key` | Must |
| AC-009 | Auth rejection counter `druggate_auth_rejections_total` with label `reason` (missing, invalid, inactive) | Must |
| AC-010 | Redis health gauge `druggate_redis_up` (1 = healthy, 0 = unhealthy) | Must |
| AC-011 | Redis ping latency gauge `druggate_redis_ping_duration_seconds` | Must |
| AC-012 | Redis health collector runs as background goroutine every 30s with `Start()`/`Stop()` lifecycle, not on every `/metrics` scrape | Must |
| AC-013 | Go runtime metrics (goroutines, memory, GC) included via default Prometheus collectors | Must |
| AC-014 | All metric names use the `druggate_` namespace prefix | Must |
| AC-015 | Container CPU usage gauge `druggate_container_cpu_usage_seconds_total` from `/proc/self/stat` or `syscall.Getrusage` | Must |
| AC-016 | Container CPU core count gauge `druggate_container_cpu_cores_available` from `runtime.NumCPU()` | Must |
| AC-017 | Container memory RSS gauge `druggate_container_memory_rss_bytes` from `/proc/self/status` | Must |
| AC-018 | Container memory VMS gauge `druggate_container_memory_vms_bytes` | Should |
| AC-019 | Container memory limit gauge `druggate_container_memory_limit_bytes` from cgroup (`-1` if unlimited) | Should |
| AC-020 | Container memory usage ratio gauge `druggate_container_memory_usage_ratio` (RSS / limit, omitted if limit unavailable) | Should |
| AC-021 | Container disk gauges `druggate_container_disk_total_bytes`, `druggate_container_disk_free_bytes`, `druggate_container_disk_used_bytes` for root volume via `syscall.Statfs` | Must |
| AC-022 | Container network I/O gauges `druggate_container_network_receive_bytes_total`, `druggate_container_network_transmit_bytes_total` with label `interface` from `/proc/net/dev` | Must |
| AC-023 | Container network packet gauges `druggate_container_network_receive_packets_total`, `druggate_container_network_transmit_packets_total` with label `interface` | Should |
| AC-024 | Container system metrics collected via background goroutine on configurable interval (default: 15s), not on every scrape | Must |
| AC-025 | Collection interval configurable via `SYSTEM_METRICS_INTERVAL` env var (default: `15s`) | Should |
| AC-026 | Procfs/cgroup code restricted to Linux via `//go:build linux` build tags — unit tests use file-based fixtures | Must |
| AC-027 | System collector follows `Start()`/`Stop()` lifecycle consistent with Redis health collector | Must |
| AC-028 | Single `Metrics` struct in `internal/metrics/metrics.go` holds all collectors, registered via `NewMetrics(reg prometheus.Registerer)` | Must |
| AC-029 | Adding the metrics package does not break any existing tests or functionality | Must |
| AC-030 | Metrics middleware is inserted into the Chi middleware chain to instrument all API requests | Must |

## 3. User Test Cases

### TC-001: Metrics endpoint returns Prometheus format

**Precondition:** Service is running
**Steps:**
1. `curl http://localhost:8081/metrics`
2. Inspect response headers and body
**Expected Result:** Content-Type is `text/plain; version=0.0.4; charset=utf-8`. Body contains `# HELP` and `# TYPE` lines. Body contains `druggate_http_requests_total` metric.
**Screenshot Checkpoint:** N/A
**Maps to:** TBD

### TC-002: Metrics endpoint is unauthenticated

**Precondition:** Service is running, no API key configured for test client
**Steps:**
1. `curl http://localhost:8081/metrics` (no `X-API-Key` header)
**Expected Result:** `200 OK` response with Prometheus metrics. Not `401`.
**Maps to:** TBD

### TC-003: HTTP request counter increments

**Precondition:** Service running with valid API key
**Steps:**
1. `GET /v1/drugs/names` with valid API key
2. `GET /v1/drugs/class?name=nonexistent` with valid API key (404)
3. Check `/metrics`
**Expected Result:** `druggate_http_requests_total{route="/v1/drugs/names",method="GET",status_code="200"}` >= 1 and `druggate_http_requests_total{route="/v1/drugs/class",method="GET",status_code="404"}` >= 1. Duration histogram has observations.
**Maps to:** TBD

### TC-004: Cache hit/miss counters track Redis caching

**Precondition:** Service running, Redis cache empty
**Steps:**
1. `GET /v1/drugs/names` (cache miss — fetches from upstream)
2. `GET /v1/drugs/names` again (cache hit)
3. Check `/metrics`
**Expected Result:** `druggate_cache_hits_total{key_type="drugnames",outcome="miss"}` >= 1 and `druggate_cache_hits_total{key_type="drugnames",outcome="hit"}` >= 1
**Maps to:** TBD

### TC-005: Rate limit rejection counter increments

**Precondition:** Service running, API key with rate limit of 5/min
**Steps:**
1. Send 6 requests in quick succession with the same API key
2. Check `/metrics`
**Expected Result:** `druggate_ratelimit_rejections_total{api_key="pk_..."}` >= 1
**Maps to:** TBD

### TC-006: Auth rejection counter tracks reasons

**Precondition:** Service running
**Steps:**
1. Request with no `X-API-Key` header
2. Request with invalid API key `pk_bogus`
3. Check `/metrics`
**Expected Result:** `druggate_auth_rejections_total{reason="missing"}` >= 1 and `druggate_auth_rejections_total{reason="invalid"}` >= 1
**Maps to:** TBD

### TC-007: Redis health metrics present

**Precondition:** Service running with Redis connected
**Steps:**
1. Check `/metrics`
**Expected Result:** `druggate_redis_up` is 1. `druggate_redis_ping_duration_seconds` has a positive value.
**Maps to:** TBD

### TC-008: Redis health shows down when Redis unavailable

**Precondition:** Service running, Redis stopped after startup
**Steps:**
1. Stop Redis container
2. Wait 30s (collector interval)
3. Check `/metrics`
**Expected Result:** `druggate_redis_up` is 0.
**Maps to:** TBD

### TC-009: Container CPU metrics reported

**Precondition:** Service running in Docker container (Linux)
**Steps:**
1. `curl http://localhost:8081/metrics | grep druggate_container_cpu`
**Expected Result:** `druggate_container_cpu_usage_seconds_total` > 0 and `druggate_container_cpu_cores_available` matches container CPU allocation.
**Maps to:** TBD

### TC-010: Container memory metrics reported

**Precondition:** Service running in Docker container
**Steps:**
1. `curl http://localhost:8081/metrics | grep druggate_container_memory`
**Expected Result:** `druggate_container_memory_rss_bytes` > 0. If cgroup limit set, `druggate_container_memory_limit_bytes` shows the limit.
**Maps to:** TBD

### TC-011: Container disk metrics reported

**Precondition:** Service running
**Steps:**
1. `curl http://localhost:8081/metrics | grep druggate_container_disk`
**Expected Result:** `druggate_container_disk_total_bytes`, `druggate_container_disk_free_bytes`, `druggate_container_disk_used_bytes` with sensible values (total > used > 0).
**Maps to:** TBD

### TC-012: Container network metrics with interface labels

**Precondition:** Service running in Docker container
**Steps:**
1. `curl http://localhost:8081/metrics | grep druggate_container_network`
**Expected Result:** `druggate_container_network_receive_bytes_total{interface="eth0"}` and `druggate_container_network_transmit_bytes_total{interface="eth0"}` present.
**Maps to:** TBD

## 4. Data Model

No persistent data. All metrics are in-memory collectors registered with Prometheus.

### Metrics Struct (in-memory)

| Field | Type | Labels | Description |
|-------|------|--------|-------------|
| `HTTPRequestsTotal` | CounterVec | route, method, status_code | Total HTTP requests |
| `HTTPRequestDuration` | HistogramVec | route, method | HTTP request latency in seconds |
| `CacheHitsTotal` | CounterVec | key_type, outcome | Redis cache outcomes (hit/miss) |
| `RateLimitRejectionsTotal` | CounterVec | api_key | Rate limit 429 rejections |
| `AuthRejectionsTotal` | CounterVec | reason | Auth failures by reason |
| `RedisUp` | Gauge | — | Redis health (1/0) |
| `RedisPingDuration` | Gauge | — | Last Redis ping latency in seconds |
| `ContainerCPUUsage` | Gauge | — | Cumulative CPU seconds |
| `ContainerCPUCores` | Gauge | — | Available CPU cores |
| `ContainerMemoryRSS` | Gauge | — | Resident set size bytes |
| `ContainerMemoryVMS` | Gauge | — | Virtual memory size bytes |
| `ContainerMemoryLimit` | Gauge | — | Cgroup memory limit bytes |
| `ContainerMemoryUsageRatio` | Gauge | — | RSS / limit ratio |
| `ContainerDiskTotal` | Gauge | — | Total disk bytes |
| `ContainerDiskFree` | Gauge | — | Free disk bytes |
| `ContainerDiskUsed` | Gauge | — | Used disk bytes |
| `ContainerNetworkReceiveBytes` | GaugeVec | interface | Bytes received per interface |
| `ContainerNetworkTransmitBytes` | GaugeVec | interface | Bytes transmitted per interface |
| `ContainerNetworkReceivePackets` | GaugeVec | interface | Packets received per interface |
| `ContainerNetworkTransmitPackets` | GaugeVec | interface | Packets transmitted per interface |

### Relationships

- `Metrics` struct is created once in `main.go` and passed to middleware, handlers, and background collectors
- Redis health collector holds a `*redis.Client` and the `Metrics` pointer
- System collector holds a `SystemSource` interface and the `Metrics` pointer
- Metrics middleware wraps the Chi router and records per-request counters/histograms

## 5. API Contract

### GET /metrics

**Description:** Prometheus metrics endpoint. Returns all application, Redis, and container metrics in Prometheus exposition format.

**Response (200):**
```
Content-Type: text/plain; version=0.0.4; charset=utf-8

# HELP druggate_http_requests_total Total HTTP requests.
# TYPE druggate_http_requests_total counter
druggate_http_requests_total{route="/v1/drugs/names",method="GET",status_code="200"} 42

# HELP druggate_http_request_duration_seconds HTTP request latency in seconds.
# TYPE druggate_http_request_duration_seconds histogram
druggate_http_request_duration_seconds_bucket{route="/v1/drugs/names",method="GET",le="0.005"} 38
...

# HELP druggate_redis_up Redis health status (1 = healthy, 0 = unhealthy).
# TYPE druggate_redis_up gauge
druggate_redis_up 1
```

**Authentication:** None. This endpoint bypasses API key middleware.

**No changes to existing endpoints.** The `/metrics` endpoint is additive.

## 6. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Redis down at startup | `druggate_redis_up` = 0, ping duration not recorded |
| Redis recovers after being down | Next collector tick sets `druggate_redis_up` = 1 |
| `/metrics` called before any API requests | Metrics exist with zero values |
| High cardinality from API keys in rate limit label | Bounded by number of provisioned keys (typically < 100) |
| Inactive API key used | `druggate_auth_rejections_total{reason="inactive"}` increments |
| Concurrent metric writes | Prometheus client library is thread-safe |
| cgroup memory limit reads `max` (no limit) | `memory_limit_bytes` = -1, `usage_ratio` omitted |
| cgroup v1 vs v2 | Check v2 path first (`/sys/fs/cgroup/memory.max`), fall back to v1 (`/sys/fs/cgroup/memory/memory.limit_in_bytes`) |
| Container has no network interfaces | Network metrics omitted |
| Loopback interface (`lo`) | Included with `interface="lo"` label |
| System collector goroutine panics | Recover, log error, continue on next interval |
| Running on macOS (local dev) | Container metrics skipped (Linux build tags), app metrics still work |

## 7. Dependencies

- `github.com/prometheus/client_golang` — Prometheus Go client library
- Existing `internal/middleware/` — auth and rate limit middleware need metric recording hooks
- Existing `internal/service/drugdata.go` — cache hit/miss recording
- Existing `*redis.Client` — for health ping
- `syscall` / `/proc` / `/sys/fs/cgroup` — container system metrics (Linux only)
- Follows patterns from cash-drugs `internal/metrics/` package

## 8. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-14 | 0.1.0 | calebdunn | Initial spec from /add:spec interview |

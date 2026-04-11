# Spec: Health and Version Endpoint Standard Compliance

**Version:** 0.1.0
**Created:** 2026-04-11
**PRD Reference:** docs/prd.md — Operational Hardening
**Status:** Draft

## 1. Overview

Bring `/health` and `/version` into compliance with the cross-service "Health and Version Endpoints" standard (applies to rx-dag, cash-drugs, drug-gate, drugs-quiz BFF). The rx-dag implementation (`dag-rx` repo: `internal/api/health.go`, `internal/api/version.go`) is the reference.

Today `/health` returns `status`, `version`, and a `map[string]string` of dependencies. It is missing `uptime`, `start_time`, a structured dependency array with latency, and the `error` status tier. `/version` is missing `os`, `arch`, and `build_time`.

### User Story

As an **operator monitoring the drug-gate service**, I want **`/health` and `/version` to follow the same shape as every other service in the stack**, so that **dashboards, alerting rules, and runbooks work uniformly across services and I can tell at a glance when a build was cut, how long a service has been up, and which dependency is slow**.

## 2. Acceptance Criteria

### `/health`

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | Response includes top-level `status` field with value `ok`, `degraded`, or `error` | Must |
| AC-002 | Response includes top-level `version` string (from `internal/version.Version`) | Must |
| AC-003 | Response includes top-level `uptime` string (Go `time.Duration.String()` format, e.g. `4h32m10s`) | Must |
| AC-004 | Response includes top-level `start_time` field (UTC, RFC 3339) representing process start | Must |
| AC-005 | Response includes `dependencies` array (not a map) where each entry has `name`, `status`, `latency_ms`, and optional `error` | Must |
| AC-006 | Each dependency check records wall-clock latency in milliseconds as a float (`latency_ms`) | Must |
| AC-007 | Failed dependency checks include a human-readable `error` string on the dependency entry | Must |
| AC-008 | `status = "ok"` when every dependency is healthy | Must |
| AC-009 | `status = "degraded"` when a non-critical dependency is unhealthy OR the upstream circuit breaker is open (gateway still functional via cache) | Must |
| AC-010 | `status = "error"` when a critical dependency is down. Redis is the only critical dependency for drug-gate (loss of Redis means no rate limiting, no API keys, no cache) | Must |
| AC-011 | HTTP status is `200` when top-level `status` is `ok` or `degraded`, and `503` when `status` is `error` | Must |
| AC-012 | Dependencies checked: `redis` (critical), `cash-drugs-upstream` (non-critical), `circuit_breaker` (non-critical, reports open/closed as status) | Must |
| AC-013 | Each dependency check uses a 2-second timeout and the check's own context (not the request context) so a slow request cannot starve concurrent health probes | Should |
| AC-014 | Process start time is captured exactly once at application boot and reused by every health check | Must |
| AC-015 | The existing Swagger annotation is updated to describe the new response shape | Must |

### `/version`

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-016 | Response includes `version`, `git_commit`, `git_branch`, `go_version` (already present — must remain) | Must |
| AC-017 | Response includes `os` field sourced from `runtime.GOOS` | Must |
| AC-018 | Response includes `arch` field sourced from `runtime.GOARCH` | Must |
| AC-019 | Response includes `build_time` field (UTC, RFC 3339) injected via ldflags at build time; defaults to `"unknown"` for local builds without ldflags | Must |
| AC-020 | `internal/version` package exports a new `BuildTime` variable defaulting to `"unknown"` | Must |
| AC-021 | `Dockerfile` accepts a `BUILD_TIME` build arg and passes it through `-X ...version.BuildTime=${BUILD_TIME}` | Must |
| AC-022 | `Makefile` `build` target injects `BuildTime` using `$(shell date -u +%Y-%m-%dT%H:%M:%SZ)` | Must |
| AC-023 | `.github/workflows/ci.yml` passes `BUILD_TIME` as a build-arg in both beta and release docker build steps | Must |
| AC-024 | The existing Swagger annotation is updated to describe the new response shape | Must |

### Back-compat

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-025 | The legacy `HealthCheck` function in `internal/handler/health.go` is deleted (unused in routes — confirm via grep before removing) | Must |
| AC-026 | `HealthResponse` struct is redefined to the new shape; all references compile | Must |

## 3. User Test Cases

### TC-001: Healthy `/health` response shape

**Steps:**
1. Start drug-gate with Redis and cash-drugs both reachable
2. Wait 5 seconds
3. `curl http://localhost:8081/health | jq`

**Expected Result:**
```json
{
  "status": "ok",
  "version": "dev",
  "uptime": "5.1s",
  "start_time": "2026-04-11T14:00:00Z",
  "dependencies": [
    { "name": "redis", "status": "connected", "latency_ms": 0.8 },
    { "name": "cash-drugs-upstream", "status": "connected", "latency_ms": 12.4 },
    { "name": "circuit_breaker", "status": "closed", "latency_ms": 0 }
  ]
}
```
HTTP status: `200`.

**Maps to:** AC-001, AC-002, AC-003, AC-004, AC-005, AC-006, AC-008, AC-011, AC-012

### TC-002: Redis down → `error` status + 503

**Steps:**
1. Start drug-gate, then stop Redis (`docker stop redis`)
2. `curl -i http://localhost:8081/health`

**Expected Result:** HTTP `503`, top-level `status = "error"`, `redis` dependency entry has `status != "connected"` and a populated `error` field.

**Maps to:** AC-007, AC-010, AC-011

### TC-003: Circuit breaker open → `degraded` + 200

**Steps:**
1. Force the upstream circuit breaker open (integration test hook, or simulate N consecutive upstream failures)
2. `curl -i http://localhost:8081/health`

**Expected Result:** HTTP `200`, `status = "degraded"`, `circuit_breaker` dependency shows `status = "open"`.

**Maps to:** AC-009, AC-011

### TC-004: Upstream cash-drugs unreachable → `degraded`

**Steps:**
1. Point `CASHDRUGS_URL` at an unreachable host
2. `curl -i http://localhost:8081/health`

**Expected Result:** HTTP `200`, `status = "degraded"`, `cash-drugs-upstream` entry has non-connected status and an `error` field.

**Maps to:** AC-007, AC-009

### TC-005: `/version` response shape

**Steps:**
1. `curl http://localhost:8081/version | jq`

**Expected Result:**
```json
{
  "version": "v0.7.3",
  "git_commit": "a1b2c3d",
  "git_branch": "main",
  "go_version": "go1.26.0",
  "os": "linux",
  "arch": "amd64",
  "build_time": "2026-04-11T12:00:00Z"
}
```

**Maps to:** AC-016, AC-017, AC-018, AC-019

### TC-006: Local build fills safe defaults

**Steps:**
1. `make build` (no `VERSION` override)
2. Run binary, hit `/version`

**Expected Result:** `version = "dev"`, `build_time` is a real UTC timestamp (from `$(shell date -u ...)`) — not `"unknown"`. `git_commit`, `git_branch` default to `"unknown"` since the Makefile doesn't inject them.

**Maps to:** AC-019, AC-020, AC-022

### TC-007: Uptime monotonically increases

**Steps:**
1. Hit `/health` twice, 10 seconds apart
2. Compare `uptime` values

**Expected Result:** Second value is strictly greater than the first. `start_time` is identical across both calls.

**Maps to:** AC-003, AC-004, AC-014

## 4. Data Model

### HealthResponse (new shape)

```go
type HealthResponse struct {
    Status       string           `json:"status"`                 // ok | degraded | error
    Version      string           `json:"version"`
    Uptime       string           `json:"uptime"`                 // time.Duration.String()
    StartTime    time.Time        `json:"start_time"`             // UTC, RFC 3339
    Dependencies []DependencyInfo `json:"dependencies"`
}

type DependencyInfo struct {
    Name      string  `json:"name"`
    Status    string  `json:"status"`                // connected | disconnected | open | closed | degraded
    LatencyMs float64 `json:"latency_ms"`
    Error     string  `json:"error,omitempty"`
}
```

### VersionResponse (new — replaces ad-hoc map)

```go
type VersionResponse struct {
    Version   string `json:"version"`
    GitCommit string `json:"git_commit"`
    GitBranch string `json:"git_branch"`
    GoVersion string `json:"go_version"`
    OS        string `json:"os"`
    Arch      string `json:"arch"`
    BuildTime string `json:"build_time"`
}
```

### `internal/version` additions

```go
var (
    Version   = "dev"
    GitCommit = "unknown"
    GitBranch = "unknown"
    BuildTime = "unknown"  // NEW
)
```

## 5. API Contract

### GET `/health`

Returns `200 OK` when `status` is `ok` or `degraded`, `503 Service Unavailable` when `status` is `error`. Body is always `HealthResponse` JSON. No request body or parameters.

### GET `/version`

Always returns `200 OK` with `VersionResponse` JSON. No request body or parameters.

## 6. Implementation Notes

- Capture `startTime := time.Now().UTC()` in `cmd/server/main.go` before wiring handlers, pass it to `NewHealthHandler`.
- `HealthHandler` struct gains a `startTime time.Time` field.
- Dependency checks run **sequentially** (matches current code, simpler, health is low-QPS). If latency becomes a concern, parallelize in a follow-up — do not preoptimize.
- Each dep check wraps the work in `t := time.Now(); ...; latency := float64(time.Since(t).Microseconds())/1000` for sub-millisecond precision.
- Critical vs non-critical classification lives in the handler code, not config — Redis is critical by design for this service.
- `status` precedence: if any critical dep is unhealthy → `error`. Else if any non-critical is unhealthy OR breaker is open → `degraded`. Else → `ok`.
- Circuit breaker `status` string is `"open"` or `"closed"` (matches existing values) — but it contributes to top-level `degraded` when open.
- Upstream check uses the existing pattern (`GET {upstreamURL}/health`) — do not change the wire behavior.
- `VersionInfo` handler switches from `map[string]string` to the typed `VersionResponse` struct. `runtime.GOOS`, `runtime.GOARCH`, `runtime.Version()` are pulled inline.
- Delete `HealthCheck` (legacy) after confirming via `grep -rn HealthCheck cmd/ internal/` that nothing references it.

### Build metadata wiring

**Dockerfile:**
```dockerfile
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 go build -ldflags="-s -w \
  -X github.com/finish06/drug-gate/internal/version.Version=${VERSION} \
  -X github.com/finish06/drug-gate/internal/version.GitCommit=${GIT_COMMIT} \
  -X github.com/finish06/drug-gate/internal/version.GitBranch=${GIT_BRANCH} \
  -X github.com/finish06/drug-gate/internal/version.BuildTime=${BUILD_TIME}" \
  -o /server ./cmd/server
```

**Makefile:**
```makefile
VERSION    ?= dev
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

build:
	go build -ldflags="\
	  -X github.com/finish06/drug-gate/internal/version.Version=$(VERSION) \
	  -X github.com/finish06/drug-gate/internal/version.BuildTime=$(BUILD_TIME)" \
	  -o bin/server ./cmd/server
```

**CI (`.github/workflows/ci.yml`):** Both beta and release `docker/build-push-action` steps add:
```yaml
BUILD_TIME=${{ github.event.head_commit.timestamp }}
```
(Falls back to `unknown` for manual dispatches — acceptable.)

## 7. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Process started <1s ago, health hit immediately | `uptime` string still valid (e.g. `"87ms"` or `"1.2s"`) — just use `time.Since(startTime).String()` |
| Redis ping succeeds but returns slowly (>1s) | Dep reports `connected`, `latency_ms` reflects reality. Only timeout → error. |
| Upstream returns `200` with garbage body | Considered `connected` — drug-gate's `/health` only checks reachability, not content |
| Multiple health probes hit concurrently | Each builds its own `DependencyInfo` slice; no shared mutable state |
| `BuildTime` ldflag contains a space | `date -u +%Y-%m-%dT%H:%M:%SZ` never produces spaces, but quote the ldflag in Makefile to be safe |
| Clock skew at boot | `start_time` reflects whatever the container clock said at boot — document, do not compensate |
| Go `runtime.GOOS` disagrees with Docker image OS | Not possible in practice — binary is static-linked, GOOS reflects compile target |

## 8. Dependencies

- **Modifies:** `internal/handler/health.go`, `internal/handler/version.go`, `internal/handler/health_test.go`, `internal/handler/version_test.go`, `internal/version/version.go`, `cmd/server/main.go`, `Dockerfile`, `Makefile`, `.github/workflows/ci.yml`
- **Touches at the edges:** `docs/sequence-diagram.md` (health flow already represented — verify still accurate)
- **No new external dependencies**
- **Cross-service coordination:** this spec assumes rx-dag already implements the standard. Confirm by reading `dag-rx` repo `internal/api/health.go` and `internal/api/version.go` before finalizing the response shape — if rx-dag uses slightly different field casing, match it exactly.

## 9. Screenshot Checkpoints

N/A — backend-only, no UI.

## 10. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-04-11 | 0.1.0 | calebdunn | Initial spec |

# Plan: Health and Version Endpoint Standard Compliance

**Spec:** specs/health-version-standard.md
**Created:** 2026-04-11
**Maturity:** beta (strict TDD, full gates)

## Task Breakdown

### Phase 1 — Foundation (version package + data model)

1. **Add `BuildTime` to `internal/version/version.go`**
   - Add `BuildTime = "unknown"` alongside existing vars
   - Update `version_test.go` to assert default
   - **Covers:** AC-020

2. **Define `VersionResponse` and new `HealthResponse` / `DependencyInfo` structs** in `internal/handler/health.go` and `version.go`
   - Keep exported names (`HealthResponse` already referenced in Swagger)
   - **Covers:** AC-005, AC-026

### Phase 2 — `/version` endpoint (RED → GREEN → REFACTOR)

3. **RED:** Write `TestVersionInfo_IncludesOSArchBuildTime` in `version_test.go`
   - Parses response JSON, asserts `os`, `arch`, `build_time`, `go_version` present and non-empty
   - Asserts `os == runtime.GOOS`, `arch == runtime.GOARCH`
   - **Covers:** AC-017, AC-018, AC-019

4. **GREEN:** Update `VersionInfo` handler to emit `VersionResponse` with `runtime.GOOS`, `runtime.GOARCH`, `version.BuildTime`
   - Update Swagger annotation to reference `VersionResponse`
   - **Covers:** AC-016, AC-017, AC-018, AC-019, AC-024

5. **Wire ldflags:**
   - `Dockerfile`: add `ARG BUILD_TIME=unknown` + `-X ...BuildTime=${BUILD_TIME}`
   - `Makefile`: add `BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)` + inject into ldflags
   - `.github/workflows/ci.yml`: add `BUILD_TIME=${{ github.event.head_commit.timestamp }}` to both beta and release build-args
   - **Covers:** AC-021, AC-022, AC-023

### Phase 3 — `/health` endpoint (RED → GREEN → REFACTOR)

6. **RED:** Rewrite `health_test.go` tests:
   - `TestHealth_OK_AllDepsHealthy` — mock Redis client (already in place), stub upstream 200, closed breaker → assert full response shape, `status == "ok"`, 200
   - `TestHealth_Degraded_UpstreamDown` — upstream timeout → `status == "degraded"`, 200, `cash-drugs-upstream` entry has `error` field
   - `TestHealth_Degraded_BreakerOpen` — breaker open → `status == "degraded"`, 200
   - `TestHealth_Error_RedisDown` — Redis ping fails → `status == "error"`, 503, `redis` entry has `error` field
   - `TestHealth_UptimeFormat` — `uptime` parses as `time.Duration`
   - `TestHealth_StartTimeStable` — two consecutive calls return identical `start_time`
   - `TestHealth_DependencyLatencyPopulated` — every dep has `latency_ms >= 0`
   - **Covers:** AC-001–AC-014

7. **GREEN:** Rewrite `HealthHandler`:
   - Add `startTime time.Time` field
   - `NewHealthHandler` gains a `startTime` param (passed from `main.go`)
   - `Handle` builds `[]DependencyInfo` in order: redis → cash-drugs-upstream → circuit_breaker
   - Each check: start timer, perform check, record latency, populate `status` and `error`
   - Compute top-level status: critical failure → `error`/503; any non-critical failure → `degraded`/200; else `ok`/200
   - Use `time.Since(h.startTime).String()` for uptime, `h.startTime.UTC()` for start_time
   - Update Swagger annotation
   - **Covers:** AC-001–AC-013, AC-015

8. **Wire in `cmd/server/main.go`:**
   - Capture `startTime := time.Now().UTC()` at top of `main()`
   - Pass to `handler.NewHealthHandler(rdb, upstreamURL, startTime, breaker)`
   - **Covers:** AC-014

9. **REFACTOR:** Extract per-dep check helpers if the handler exceeds ~80 lines. Otherwise leave inline — clarity over abstraction at this size.

### Phase 4 — Cleanup

10. **Delete `HealthCheck` legacy function** from `internal/handler/health.go`
    - Grep first: `grep -rn "HealthCheck\b" cmd/ internal/`
    - If unreferenced, delete function + its Swagger block
    - **Covers:** AC-025

11. **Regenerate Swagger docs**
    - `make swagger`
    - Verify `docs/swagger.json` shows new `HealthResponse` / `VersionResponse` schemas

12. **Update sequence diagram if needed**
    - `docs/sequence-diagram.md` — confirm health probe flow still matches (should be a no-op visually)

### Phase 5 — Verification

13. **Run full test suite:** `make test-unit && make test-coverage`
14. **Lint + vet:** `make lint && make vet`
15. **Build locally:** `make build && ./bin/server` — hit `/health` and `/version`, eyeball response shape
16. **Integration sanity:** `docker-compose up`, hit both endpoints via the container
17. **Cross-reference rx-dag:** read `dag-rx` repo's `internal/api/health.go` and `internal/api/version.go`, diff field naming. Adjust drug-gate if rx-dag uses different casing or ordering.

## File Changes

| File | Change | Phase |
|------|--------|-------|
| `internal/version/version.go` | Add `BuildTime` var | 1 |
| `internal/version/version_test.go` | Add `BuildTime` default test | 1 |
| `internal/handler/health.go` | Rewrite `HealthResponse`, `HealthHandler`, delete `HealthCheck` | 1, 3, 4 |
| `internal/handler/health_test.go` | Rewrite test suite for new shape | 3 |
| `internal/handler/version.go` | Switch to `VersionResponse` struct, add os/arch/build_time | 2 |
| `internal/handler/version_test.go` | Add os/arch/build_time assertions | 2 |
| `cmd/server/main.go` | Capture `startTime`, pass to `NewHealthHandler` | 3 |
| `Dockerfile` | Add `BUILD_TIME` ARG + ldflag | 2 |
| `Makefile` | Add `BUILD_TIME` var + ldflag | 2 |
| `.github/workflows/ci.yml` | Pass `BUILD_TIME` build-arg (both beta + release) | 2 |
| `docs/swagger.json`, `docs/swagger.yaml`, `docs/docs.go` | Regenerated via `make swagger` | 4 |
| `docs/sequence-diagram.md` | Verify only (likely no edit) | 4 |

## Test Strategy

- **Unit:** covers AC-001 through AC-020 via `health_test.go` and `version_test.go`. Existing test harness already wires a mock Redis client — extend rather than rewrite that scaffolding.
- **Integration:** one integration test (`make test-integration`) that runs against real Redis, verifies `/health` reports `redis: connected` with real latency. Covers AC-006 end-to-end.
- **Manual smoke:** Phase 5 local binary test. No new E2E needed — `/health` is already hit by `tests/k6/staging.js` smoke flow and will catch response-shape regressions via k6 baseline comparison.
- **No fixtures needed** — everything is deterministic except latency values (assert `>= 0`, not exact).

## Dependencies

- **Before starting:** confirm rx-dag's current `/health` and `/version` response shape (spec claims it's the reference; verify field names match exactly)
- **Blocks:** any monitoring dashboard or alert rule that parses `/health` expecting the new shape
- **Not blocking:** cash-drugs, drugs-quiz BFF — they can migrate independently on the same standard

## Spec Traceability

| AC | Phase | Task | Test |
|----|-------|------|------|
| AC-001 | 3 | 6, 7 | `TestHealth_OK_AllDepsHealthy`, `TestHealth_Error_RedisDown` |
| AC-002 | 3 | 6, 7 | `TestHealth_OK_AllDepsHealthy` |
| AC-003 | 3 | 6, 7 | `TestHealth_UptimeFormat` |
| AC-004 | 3 | 6, 7 | `TestHealth_StartTimeStable` |
| AC-005 | 1, 3 | 2, 7 | `TestHealth_OK_AllDepsHealthy` |
| AC-006 | 3 | 6, 7 | `TestHealth_DependencyLatencyPopulated` + integration test |
| AC-007 | 3 | 6, 7 | `TestHealth_Error_RedisDown`, `TestHealth_Degraded_UpstreamDown` |
| AC-008 | 3 | 6, 7 | `TestHealth_OK_AllDepsHealthy` |
| AC-009 | 3 | 6, 7 | `TestHealth_Degraded_UpstreamDown`, `TestHealth_Degraded_BreakerOpen` |
| AC-010 | 3 | 6, 7 | `TestHealth_Error_RedisDown` |
| AC-011 | 3 | 6, 7 | All health tests assert HTTP status |
| AC-012 | 3 | 6, 7 | `TestHealth_OK_AllDepsHealthy` asserts all 3 dep names |
| AC-013 | 3 | 7 | Covered by timeout construction in handler |
| AC-014 | 3 | 6, 8 | `TestHealth_StartTimeStable` |
| AC-015 | 3 | 7 | Manual: `make swagger` diff |
| AC-016 | 2 | 4 | `TestVersionInfo_IncludesOSArchBuildTime` |
| AC-017 | 2 | 3, 4 | `TestVersionInfo_IncludesOSArchBuildTime` |
| AC-018 | 2 | 3, 4 | `TestVersionInfo_IncludesOSArchBuildTime` |
| AC-019 | 2 | 3, 4, 5 | `TestVersionInfo_IncludesOSArchBuildTime` + manual `make build` check |
| AC-020 | 1 | 1 | `TestVersion_BuildTimeDefault` |
| AC-021 | 2 | 5 | Manual: docker build → inspect binary |
| AC-022 | 2 | 5 | Manual: `make build` → `/version` shows real timestamp |
| AC-023 | 2 | 5 | Manual: CI run → deployed `/version` shows commit timestamp |
| AC-024 | 2 | 4 | Manual: `make swagger` diff |
| AC-025 | 4 | 10 | `grep HealthCheck` returns zero |
| AC-026 | 1, 3 | 2, 7 | Full suite compiles + passes |

## Risks

1. **rx-dag field-name divergence** — if rx-dag used slightly different JSON keys (`build_timestamp` vs `build_time`), dashboards will break. Mitigation: Phase 5 task 17 cross-checks before merge.
2. **CI `head_commit.timestamp` format** — GitHub returns ISO 8601 but with timezone offset, not `Z`. If the standard requires `Z`, normalize in the workflow step.
3. **Swagger regeneration noise** — `make swagger` may rewrite unrelated parts of `docs.go`. Keep the regen commit separate from logic changes for review clarity.
4. **Downstream clients parsing `dependencies` as a map** — the shape change from `map[string]string` to `[]DependencyInfo` is a breaking change. At beta maturity with no external consumers of `/health` beyond monitoring, acceptable. Flag in commit message and CHANGELOG.

## Commits (expected sequence)

1. `test: add failing tests for version endpoint os/arch/build_time (RED)`
2. `feat: version endpoint exposes os, arch, build_time (GREEN)`
3. `chore: inject BuildTime ldflag via Dockerfile, Makefile, CI`
4. `test: add failing tests for new health response shape (RED)`
5. `feat: health endpoint emits uptime, start_time, structured dependencies (GREEN)`
6. `refactor: extract dependency check helpers if needed`
7. `chore: delete legacy HealthCheck function`
8. `docs: regenerate swagger for new health/version schemas`

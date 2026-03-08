# Implementation Plan: NDC Lookup

**Spec Version:** 0.1.0
**Created:** 2026-03-07
**Team Size:** Small team (2-4), autonomous agent execution
**Estimated Duration:** 2-3 days (solo dev + agent)

## Overview

Build the foundational drug-gate service: Go module scaffolding, Chi router, NDC normalization library, cash-drugs HTTP client, and the `GET /v1/drugs/ndc/{ndc}` endpoint. This is M1 — the first working vertical slice of drug-gate.

## Objectives

- Accept NDC codes in any valid format and normalize to 11-digit canonical form
- Query cash-drugs `fda-ndc` endpoint and transform the response
- Return clean, frontend-friendly JSON with drug name, generic name, and therapeutic classes
- Establish project scaffolding (go.mod, Chi router, structured logging, health check)
- Achieve 80% test coverage with mocked cash-drugs client for unit tests

## Success Criteria

- [ ] All 16 acceptance criteria implemented and tested
- [ ] Code coverage >= 80%
- [ ] `go vet ./...` passes
- [ ] All 10 user test cases covered by automated tests
- [ ] Health check endpoint functional
- [ ] Structured logging (slog) on all requests

## Acceptance Criteria Analysis

### AC-001 through AC-005: NDC Format Acceptance
- **Complexity:** Medium (5 format variations, normalization logic)
- **Tasks:** NDC parser/normalizer package with comprehensive unit tests
- **Risk:** 10-digit dashless NDCs are ambiguous — need a strategy decision

### AC-006: Normalization to 11-digit
- **Complexity:** Medium (zero-padding rules per segment pattern)
- **Tasks:** Core of the NDC package — the normalize function
- **Testing:** Table-driven tests covering all format combinations

### AC-007: Query cash-drugs
- **Complexity:** Simple (HTTP GET with query param)
- **Tasks:** HTTP client package with interface for mocking
- **Risk:** Depends on `fda-ndc` endpoint existing in cash-drugs

### AC-008 through AC-011: Response shaping
- **Complexity:** Simple (extract fields, build response struct)
- **Tasks:** Response model, transformation function
- **Testing:** Unit tests with sample upstream JSON

### AC-012 through AC-014: Error handling
- **Complexity:** Simple (standard HTTP error responses)
- **Tasks:** Error types, handler error mapping
- **Testing:** Unit tests per error scenario

### AC-015 through AC-016: Partial data and display format
- **Complexity:** Simple
- **Tasks:** Nil-safe field mapping, display formatter

## Implementation Phases

### Phase 0: Project Scaffolding

Set up the Go module, directory structure, and foundational infrastructure before any feature code.

| Task ID | Description | AC | Effort | Dependencies |
|---------|-------------|-----|--------|--------------|
| TASK-001 | Initialize Go module (`go mod init github.com/finish06/drug-gate`) | — | 15min | — |
| TASK-002 | Create directory structure: `cmd/server/`, `internal/{handler,middleware,client,ndc,model}/` | — | 15min | TASK-001 |
| TASK-003 | Add Chi dependency, create main.go with Chi router, slog logger, graceful shutdown | — | 1h | TASK-002 |
| TASK-004 | Add health check endpoint (`GET /health`) | — | 30min | TASK-003 |
| TASK-005 | Create Dockerfile (multi-stage Alpine, same pattern as cash-drugs) | — | 30min | TASK-003 |
| TASK-006 | Create docker-compose.yml (drug-gate on :8081, Redis on :6379) | — | 30min | TASK-005 |
| TASK-007 | Create Makefile (test-unit, test-coverage, build, docker targets) | — | 30min | TASK-003 |

**Phase Duration:** ~3.5 hours
**Blockers:** None

### Phase 1: NDC Normalization Package

Pure logic, zero external dependencies. Fully testable in isolation.

| Task ID | Description | AC | Effort | Dependencies |
|---------|-------------|-----|--------|--------------|
| TASK-008 | Define NDC struct (raw, digits, normalized, display fields) | AC-016 | 30min | TASK-002 |
| TASK-009 | Implement `Parse(input string) (NDC, error)` — strip dashes/whitespace, validate digit count | AC-001–005, AC-012 | 1h | TASK-008 |
| TASK-010 | Implement `Normalize()` — detect segment pattern from dashes, zero-pad to 11-digit | AC-006 | 1.5h | TASK-009 |
| TASK-011 | Implement dashless 10-digit strategy (try all 3 normalizations) | AC-005, AC-006 | 1h | TASK-010 |
| TASK-012 | Implement `Display()` — format as 5-4-2 with dashes | AC-016 | 15min | TASK-010 |
| TASK-013 | Write table-driven unit tests for Parse (valid formats, invalid inputs, edge cases) | AC-001–005, AC-012 | 1.5h | TASK-009 |
| TASK-014 | Write table-driven unit tests for Normalize (all format conversions) | AC-006 | 1h | TASK-010 |
| TASK-015 | Write unit tests for dashless ambiguity handling | AC-005 | 30min | TASK-011 |

**Phase Duration:** ~6.25 hours
**Blockers:** None — pure logic
**Parallelizable:** Tests (TASK-013–015) can be written alongside implementation if pair-developing

### Phase 2: cash-drugs Client

HTTP client to query the upstream `fda-ndc` endpoint. Interface-based for easy mocking.

| Task ID | Description | AC | Effort | Dependencies |
|---------|-------------|-----|--------|--------------|
| TASK-016 | Define `DrugClient` interface: `LookupByNDC(ctx, ndc string) (*DrugResult, error)` | AC-007 | 30min | TASK-002 |
| TASK-017 | Define `DrugResult` struct (raw upstream response fields) | AC-008–010 | 30min | TASK-016 |
| TASK-018 | Implement `HTTPDrugClient` — HTTP GET to `{baseURL}/api/cache/fda-ndc?NDC={ndc}` | AC-007 | 1h | TASK-016 |
| TASK-019 | Add timeout, error handling (connection refused → 502, 404 passthrough) | AC-013, AC-014 | 1h | TASK-018 |
| TASK-020 | Write mock client for unit tests | — | 30min | TASK-016 |
| TASK-021 | Write unit tests for HTTPDrugClient (mocked HTTP server) | AC-007, AC-013, AC-014 | 1.5h | TASK-018, TASK-019 |

**Phase Duration:** ~5 hours
**Blockers:** None — uses httptest for mock server
**Note:** Real integration tests depend on cash-drugs `fda-ndc` endpoint (external dependency)

### Phase 3: Handler and Response Shaping

Wire NDC normalization + client together in the HTTP handler. Transform upstream response to frontend shape.

| Task ID | Description | AC | Effort | Dependencies |
|---------|-------------|-----|--------|--------------|
| TASK-022 | Define response models: `DrugDetailResponse`, `ErrorResponse` | AC-011 | 30min | TASK-002 |
| TASK-023 | Implement `DrugHandler` struct with `DrugClient` interface dependency | — | 30min | TASK-016, TASK-022 |
| TASK-024 | Implement `HandleNDCLookup` — parse NDC, normalize, call client, shape response | AC-001–011, AC-015, AC-016 | 2h | TASK-010, TASK-018, TASK-022 |
| TASK-025 | Wire handler into Chi router with `/v1/drugs/ndc/{ndc}` route | — | 30min | TASK-003, TASK-024 |
| TASK-026 | Add request logging middleware (slog, request ID) | — | 1h | TASK-003 |
| TASK-027 | Write handler unit tests with mock client (happy path, all error scenarios) | AC-001–016 | 2h | TASK-024, TASK-020 |
| TASK-028 | Write handler tests for partial upstream data | AC-015 | 30min | TASK-024, TASK-020 |

**Phase Duration:** ~7 hours
**Blockers:** Phase 1 (NDC package) and Phase 2 (client interface) must be complete

### Phase 4: Integration and Polish

| Task ID | Description | AC | Effort | Dependencies |
|---------|-------------|-----|--------|--------------|
| TASK-029 | Configuration: upstream base URL from env var (`CASHDRUGS_URL`) | — | 30min | TASK-018 |
| TASK-030 | Add `.env.example` with all env vars | — | 15min | TASK-029 |
| TASK-031 | Run `go vet ./...`, fix any issues | — | 30min | All code tasks |
| TASK-032 | Run coverage report, fill gaps to reach 80% | — | 1h | All test tasks |
| TASK-033 | Update CLAUDE.md with actual project structure and commands | — | 30min | All tasks |
| TASK-034 | Create GitHub Actions CI workflow (vet + test + coverage + build) | — | 1h | TASK-007 |

**Phase Duration:** ~3.75 hours
**Blockers:** All implementation and test phases complete

## Effort Summary

| Phase | Tasks | Estimated Hours |
|-------|-------|-----------------|
| Phase 0: Scaffolding | 7 | 3.5h |
| Phase 1: NDC Package | 8 | 6.25h |
| Phase 2: Client | 6 | 5h |
| Phase 3: Handler | 7 | 7h |
| Phase 4: Polish | 6 | 3.75h |
| **Total** | **34** | **25.5h** |
| **With 15% contingency** | | **~29h** |

## Dependencies

### External Dependencies
| Dependency | Blocks | Status | Mitigation |
|------------|--------|--------|------------|
| cash-drugs `fda-ndc` endpoint | Integration/E2E tests only | Not yet created | Mock client for unit tests; build this endpoint in cash-drugs in parallel |

### Internal Dependencies (task ordering)
```
TASK-001 → TASK-002 → TASK-003 → TASK-004/005/006/007 (parallel)
                    ↘ TASK-008 → TASK-009 → TASK-010 → TASK-011/012 (parallel)
                                          ↘ TASK-013/014/015 (tests, parallel)
                    ↘ TASK-016 → TASK-017 → TASK-018 → TASK-019
                                          ↘ TASK-020/021 (tests, parallel)
TASK-010 + TASK-018 → TASK-022 → TASK-023 → TASK-024 → TASK-025/026/027/028 (parallel)
All → TASK-029 → TASK-030/031/032/033/034 (parallel)
```

## Parallelization Strategy

Phase 1 (NDC) and Phase 2 (Client) are **fully independent** and can execute in parallel:

```
Day 1:
  Stream A: TASK-001 → TASK-002 → TASK-003 → TASK-004/005/006/007
  Stream B: (blocked until TASK-002)

Day 1-2:
  Stream A: TASK-008 → TASK-009 → TASK-010 → TASK-011/012 → TASK-013/014/015
  Stream B: TASK-016 → TASK-017 → TASK-018 → TASK-019 → TASK-020/021

Day 2-3:
  Stream A: TASK-022 → TASK-023 → TASK-024 → TASK-025/026
  Stream B: TASK-027/028 → TASK-029 → TASK-030/031/032/033/034
```

**Optimal with 2 streams: ~2 days**
**Sequential (solo): ~3 days**

## Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| 10-digit dashless NDC ambiguity causes false matches | Medium | Medium | Try all 3 normalizations, return first match; document limitation |
| cash-drugs `fda-ndc` response shape unknown | Medium | Low | Build client interface now, adapt parsing when endpoint exists |
| cash-drugs `fda-ndc` doesn't include therapeutic class | Low | Medium | May need to cross-reference with `drugclasses` endpoint |
| Chi v5 API differences from docs | Low | Low | Pin version, read release notes |

## Testing Strategy

1. **Unit Tests (Phase 1-3):** Table-driven tests for NDC parsing/normalization, mocked HTTP client for upstream calls, handler tests with mock client injection
2. **Integration Tests (future):** Against running cash-drugs with `fda-ndc` endpoint — blocked on upstream dependency
3. **Coverage target:** 80% (standard quality mode)
4. **Quality gates:** `go vet` (blocking), test coverage report

## Deliverables

### Code
```
cmd/server/main.go                    — Entrypoint, Chi router, graceful shutdown
internal/ndc/ndc.go                   — NDC parsing, normalization, display formatting
internal/client/client.go             — DrugClient interface + HTTPDrugClient
internal/model/response.go            — DrugDetailResponse, ErrorResponse structs
internal/handler/drug.go              — DrugHandler with HandleNDCLookup
internal/middleware/logging.go        — Request logging with slog
```

### Tests
```
internal/ndc/ndc_test.go              — Table-driven NDC parsing/normalization tests
internal/client/client_test.go        — HTTP client tests with httptest
internal/handler/drug_test.go         — Handler tests with mock client
```

### Infrastructure
```
Dockerfile                            — Multi-stage Alpine build
docker-compose.yml                    — drug-gate + Redis
Makefile                              — Build, test, coverage targets
.github/workflows/ci.yml             — GitHub Actions pipeline
.env.example                          — Environment variables
```

## Open Decisions

1. **Dashless 10-digit strategy:** Try all 3 normalizations (recommended) or require dashes? — Flagged in spec, needs decision before TASK-011.
2. **cash-drugs `fda-ndc` response shape:** Unknown until endpoint is built. Client interface is designed to adapt.

## Plan History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-07 | 0.1.0 | calebdunn | Initial plan from /add:plan |

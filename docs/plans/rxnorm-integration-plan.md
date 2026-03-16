# Implementation Plan: RxNorm Integration

**Spec:** specs/rxnorm-integration.md v0.1.0
**Created:** 2026-03-16
**Team Size:** Solo (1 agent)
**Estimated Duration:** 3-4 TDD cycles

## Overview

Add 5 RxNorm endpoints to drug-gate: search (approximate match), NDCs by RxCUI, generics by RxCUI, related concepts by RxCUI, and a unified profile endpoint. Follows existing architecture: client → service (Redis cache) → handler. The upstream cash-drugs already has the RxNorm proxy endpoints configured.

## Objectives

- Expose RxNorm drug resolution, NDC cross-reference, and related concepts through drug-gate
- Provide a convenience "profile" endpoint that orchestrates multiple upstream calls
- Cache results in Redis with appropriate TTLs (24h for search, 7d for RxCUI lookups)
- Maintain consistency with M3 patterns (error handling, caching, pagination)

## Acceptance Criteria Analysis

### AC-001–003: Search endpoint
- **Complexity:** Medium — new client methods for approximate-match + spelling-suggestions, response transformation, top-5 cap
- **Tasks:** Client methods, raw types, service caching, handler, tests
- **Risk:** Upstream response shape for approximate-match is nested (`approximateGroup.candidate`) — need careful JSON mapping

### AC-004: NDCs endpoint
- **Complexity:** Simple — single upstream call, flat response
- **Tasks:** Client method, service caching, handler, tests

### AC-005: Generics endpoint
- **Complexity:** Simple — single upstream call, flat response
- **Tasks:** Client method, service caching, handler, tests

### AC-006–008: Related endpoint
- **Complexity:** Medium — upstream returns concept groups by TTY code, need to filter and group into 5 categories
- **Tasks:** Client method, TTY mapping logic, service caching, handler, tests
- **Risk:** Upstream `allRelatedGroup.conceptGroup` has variable structure per TTY

### AC-009–011: Profile endpoint
- **Complexity:** High — orchestrates search + NDCs + generics + related, assembles combined response
- **Tasks:** Service orchestration method, handler, caching of assembled result, tests
- **Dependencies:** All granular endpoints must work first

### AC-012–017: Validation, errors, auth, rate limiting
- **Complexity:** Simple — reuses existing middleware and patterns from M3
- **Tasks:** Validation in handlers, error responses consistent with existing pattern

### AC-018–023: Caching
- **Complexity:** Medium — two TTL tiers (24h and 7d), sliding TTL, key schema
- **Tasks:** Cache constants, service methods with configurable TTL

## Implementation Phases

### Phase 1: Client Layer — RxNorm upstream methods (RED → GREEN)

New `RxNormClient` interface and HTTP client methods. Follows the existing `DrugClient` pattern but as a separate interface since RxNorm is a different domain.

| Task ID | Description | AC | Effort | Dependencies |
|---------|-------------|-----|--------|-------------|
| TASK-001 | Define `RxNormClient` interface with 5 methods | All | 30m | — |
| TASK-002 | Define raw response types (`RxNormCandidateRaw`, `RxNormNDCRaw`, etc.) | All | 30m | — |
| TASK-003 | Implement `SearchApproximate(ctx, name)` — calls `rxnorm-approximate-match` | AC-001 | 1h | TASK-001, TASK-002 |
| TASK-004 | Implement `FetchSpellingSuggestions(ctx, name)` — calls `rxnorm-spelling-suggestions` | AC-002 | 30m | TASK-001, TASK-002 |
| TASK-005 | Implement `FetchNDCs(ctx, rxcui)` — calls `rxnorm-ndcs` | AC-004 | 30m | TASK-001, TASK-002 |
| TASK-006 | Implement `FetchGenericProduct(ctx, rxcui)` — calls `rxnorm-generic-product` | AC-005 | 30m | TASK-001, TASK-002 |
| TASK-007 | Implement `FetchAllRelated(ctx, rxcui)` — calls `rxnorm-all-related` | AC-006 | 1h | TASK-001, TASK-002 |
| TASK-008 | Add JSON deserialization tests for all raw types (upstream format verification) | All | 1h | TASK-002 |
| TASK-009 | Add client HTTP tests (happy path + error cases) via httptest | All | 2h | TASK-003–007 |

**Phase effort:** ~7.5h
**Files created:** `internal/client/rxnorm.go`, `internal/client/rxnorm_test.go`

### Phase 2: Model Layer — Response types

| Task ID | Description | AC | Effort | Dependencies |
|---------|-------------|-----|--------|-------------|
| TASK-010 | Add RxNorm response models to `internal/model/` | AC-003, AC-007–008, AC-011 | 30m | — |

Models: `RxNormSearchResult`, `RxNormCandidate`, `RxNormNDCResponse`, `RxNormGenericResponse`, `RxNormRelatedResponse`, `RxNormConcept`, `RxNormProfile`

**Phase effort:** 30m
**Files modified:** `internal/model/response.go` (or new `internal/model/rxnorm.go`)

### Phase 3: Service Layer — Caching + transformation (RED → GREEN)

New `RxNormService` following the `DrugDataService` pattern. Handles caching with two TTL tiers.

| Task ID | Description | AC | Effort | Dependencies |
|---------|-------------|-----|--------|-------------|
| TASK-011 | Create `RxNormService` struct with Redis + client deps | All | 30m | TASK-001, TASK-010 |
| TASK-012 | Implement `Search(ctx, name)` — approximate match + top 5 + fallback to suggestions | AC-001–003, AC-018 | 1.5h | TASK-003, TASK-004, TASK-011 |
| TASK-013 | Implement `GetNDCs(ctx, rxcui)` — fetch + cache 7d | AC-004, AC-019 | 30m | TASK-005, TASK-011 |
| TASK-014 | Implement `GetGenerics(ctx, rxcui)` — fetch + cache 7d | AC-005, AC-019 | 30m | TASK-006, TASK-011 |
| TASK-015 | Implement `GetRelated(ctx, rxcui)` — fetch + TTY grouping + cache 7d | AC-006–008, AC-019 | 1.5h | TASK-007, TASK-011 |
| TASK-016 | Implement `GetProfile(ctx, name)` — orchestrate search → NDCs + generics + related, cache 24h | AC-009–011, AC-020 | 2h | TASK-012–015 |
| TASK-017 | Service unit tests with miniredis mock | All | 3h | TASK-012–016 |

**Phase effort:** ~9.5h
**Files created:** `internal/service/rxnorm.go`, `internal/service/rxnorm_test.go`

### Phase 4: Handler Layer — HTTP endpoints (RED → GREEN)

New handlers following the existing pattern (DrugNamesHandler, DrugClassesHandler, etc.).

| Task ID | Description | AC | Effort | Dependencies |
|---------|-------------|-----|--------|-------------|
| TASK-018 | Create `RxNormSearchHandler` — `GET /v1/drugs/rxnorm/search` | AC-001–003, AC-012–013 | 1h | TASK-012 |
| TASK-019 | Create `RxNormNDCHandler` — `GET /v1/drugs/rxnorm/{rxcui}/ndcs` | AC-004, AC-014 | 30m | TASK-013 |
| TASK-020 | Create `RxNormGenericsHandler` — `GET /v1/drugs/rxnorm/{rxcui}/generics` | AC-005, AC-014 | 30m | TASK-014 |
| TASK-021 | Create `RxNormRelatedHandler` — `GET /v1/drugs/rxnorm/{rxcui}/related` | AC-006–008, AC-014 | 30m | TASK-015 |
| TASK-022 | Create `RxNormProfileHandler` — `GET /v1/drugs/rxnorm/profile` | AC-009–011, AC-012–013 | 1h | TASK-016 |
| TASK-023 | Add Swagger annotations to all 5 handlers | All | 30m | TASK-018–022 |
| TASK-024 | Handler unit tests (mock service, all paths) | All | 2.5h | TASK-018–022 |

**Phase effort:** ~6.5h
**Files created:** `internal/handler/rxnorm.go`, `internal/handler/rxnorm_test.go`

### Phase 5: Wire-up + Integration (RED → GREEN)

| Task ID | Description | AC | Effort | Dependencies |
|---------|-------------|-----|--------|-------------|
| TASK-025 | Register routes in `cmd/server/main.go` under `/v1/drugs/rxnorm/` | AC-016–017 | 30m | TASK-018–022 |
| TASK-026 | Update E2E config.yaml with RxNorm slugs | All | 15m | — |
| TASK-027 | Add E2E tests for all 5 endpoints | All | 2h | TASK-025, TASK-026 |
| TASK-028 | Add integration tests (real Redis) for RxNorm service caching | AC-018–021 | 2h | TASK-012–016 |
| TASK-029 | Regenerate Swagger docs (`swag init`) | All | 15m | TASK-023 |

**Phase effort:** ~5h
**Files modified:** `cmd/server/main.go`, `tests/e2e/config.yaml`, `tests/e2e/e2e_test.go`
**Files created:** `internal/service/rxnorm_integration_test.go`

### Phase 6: Documentation + Polish

| Task ID | Description | AC | Effort | Dependencies |
|---------|-------------|-----|--------|-------------|
| TASK-030 | Update `docs/sequence-diagram.md` with RxNorm flows | — | 1h | TASK-025 |
| TASK-031 | Update `CLAUDE.md` with new endpoints and key slugs | — | 15m | TASK-025 |
| TASK-032 | Update `README.md` with RxNorm endpoints | — | 15m | TASK-025 |
| TASK-033 | Update CHANGELOG.md | — | 15m | All |
| TASK-034 | Run `/add:verify` — full quality gates | — | 30m | All |

**Phase effort:** ~2.25h

## Effort Summary

| Phase | Description | Estimated Hours |
|-------|-------------|-----------------|
| Phase 1 | Client layer | 7.5h |
| Phase 2 | Model layer | 0.5h |
| Phase 3 | Service layer | 9.5h |
| Phase 4 | Handler layer | 6.5h |
| Phase 5 | Wire-up + integration | 5h |
| Phase 6 | Documentation + polish | 2.25h |
| **Total** | | **31.25h** |
| **With 15% contingency** | | **~36h** |

## TDD Cycle Mapping

The plan maps to 3-4 TDD cycles:

| Cycle | Phases | Focus |
|-------|--------|-------|
| **Cycle 1** | Phase 1 + 2 | Client + models — RED: write client tests with expected upstream JSON. GREEN: implement client methods |
| **Cycle 2** | Phase 3 | Service — RED: write service tests with mock client. GREEN: implement caching + transformation |
| **Cycle 3** | Phase 4 + 5 | Handlers + wire-up — RED: write handler + E2E tests. GREEN: implement handlers, register routes |
| **Cycle 4** | Phase 6 | Polish — docs, verify, coverage check |

## Dependencies

### External
- **cash-drugs RxNorm endpoints** — must be configured and reachable (already confirmed in E2E config review)
- **RxNorm API** — cash-drugs proxies to `rxnav.nlm.nih.gov`, which has no auth but may be slow

### Internal
- **Existing middleware** — auth, CORS, rate limiting (no changes needed, just route registration)
- **Redis** — same instance, new key namespace (`cache:rxnorm:*`)

## Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Upstream RxNorm response shapes differ from documented | Medium | High | TASK-008 deserialization tests verify against real JSON samples early |
| RxNorm API slow (2-5s for some queries) | Medium | Medium | Redis caching with long TTLs (7d) absorbs latency after first request |
| `allRelatedGroup` structure varies by drug | Low | Medium | Handle missing concept groups gracefully (empty arrays) |
| Profile endpoint too slow (4 upstream calls) | Medium | Medium | Cache assembled profile (24h TTL), individual results also cached |
| E2E tests flaky due to upstream timeouts | Medium | Low | Same pattern as M3: accept 502 as valid in E2E for uncached queries |

## File Changes Summary

### New Files
| File | Purpose |
|------|---------|
| `internal/client/rxnorm.go` | RxNorm client interface + HTTP implementation |
| `internal/client/rxnorm_test.go` | Client unit tests + deserialization tests |
| `internal/model/rxnorm.go` | RxNorm response models |
| `internal/service/rxnorm.go` | RxNorm service with Redis caching |
| `internal/service/rxnorm_test.go` | Service unit tests (miniredis) |
| `internal/service/rxnorm_integration_test.go` | Service integration tests (real Redis) |
| `internal/handler/rxnorm.go` | 5 HTTP handlers + Swagger annotations |
| `internal/handler/rxnorm_test.go` | Handler unit tests |

### Modified Files
| File | Change |
|------|--------|
| `cmd/server/main.go` | Register RxNorm routes under `/v1/drugs/rxnorm/` |
| `tests/e2e/config.yaml` | Add RxNorm slugs |
| `tests/e2e/e2e_test.go` | Add E2E tests |
| `docs/sequence-diagram.md` | Add RxNorm flow diagrams |
| `CLAUDE.md` | Add endpoints + slugs |
| `README.md` | Add endpoints |
| `CHANGELOG.md` | Add entry |

## Success Criteria

- [ ] All 23 acceptance criteria implemented and tested
- [ ] Code coverage >= 80% on new packages
- [ ] All quality gates passing (lint, vet, tests)
- [ ] E2E tests pass against real upstream
- [ ] Swagger docs include all 5 new endpoints
- [ ] Sequence diagrams updated
- [ ] CLAUDE.md and README.md updated

## Next Steps

1. Review and approve this plan
2. Run `/add:tdd-cycle specs/rxnorm-integration.md` to begin Cycle 1

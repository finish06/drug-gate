# Implementation Plan: SPL Document Browser

**Spec:** specs/spl-browser.md v1.0
**Created:** 2026-03-17
**Team Size:** Solo (1 agent)
**Estimated Duration:** 3-4 TDD cycles

## Overview

Add SPL (Structured Product Label) browsing to drug-gate: search SPLs by drug name with pagination, and retrieve SPL detail with parsed Section 7 (Drug Interactions) from XML. Follows existing architecture: client → service (Redis cache) → handler. A new `internal/spl/` package handles XML parsing. The upstream cash-drugs already has the `spls-by-name`, `spl-detail`, and `spl-xml` endpoints configured.

## Objectives

- Expose SPL metadata search by drug name through `GET /v1/drugs/spls`
- Expose SPL detail with parsed interaction sections through `GET /v1/drugs/spls/{setid}`
- Parse Section 7 (Drug Interactions) from SPL XML into structured title+text pairs
- Cache parsed interaction data in Redis with 60-minute sliding TTL
- Use offset-based pagination (limit/offset) per spec, adding offset support to the handler pagination utilities

## Acceptance Criteria Analysis

### AC-001–002: SPL search endpoint
- **Complexity:** Simple — single upstream call (`spls-by-name`), JSON mapping, pagination
- **Tasks:** Client method, response types, service caching, handler, pagination with offset
- **Note:** Upstream returns `meta.results_count` for total — use for pagination metadata

### AC-003–005: SPL detail with Section 7 parsing
- **Complexity:** High — requires two upstream calls (spl-detail + spl-xml), XML parsing, text extraction
- **Tasks:** Client methods (detail + XML fetch), XML parser package, service orchestration, handler
- **Risk:** SPL XML structure varies; Section 7 detection must be robust

### AC-006: Redis caching with 60-minute sliding TTL
- **Complexity:** Simple — same pattern as DrugDataService, 60-minute TTL matches existing `cacheTTL`
- **Tasks:** Cache in service layer for both search results and parsed interactions

### AC-007–009: Error handling (empty results, upstream down, 404)
- **Complexity:** Simple — follows existing patterns (empty data array, 502, 404)
- **Tasks:** Handled in client (ErrUpstream, ErrNotFound) and handler error mapping

### AC-010–011: Auth + rate limiting
- **Complexity:** None — existing middleware handles this via route registration
- **Tasks:** Register routes inside the `/v1` group (already has APIKeyAuth + RateLimit)

### AC-012: Offset-based pagination
- **Complexity:** Medium — existing pagination is page-based; need offset-based variant
- **Tasks:** Add `parseOffsetPagination` and `paginateSliceOffset` helpers, or convert offset to page internally

### AC-013: Case-insensitive name search
- **Complexity:** Simple — lowercase the name before passing to upstream (upstream handles matching)

### AC-014: Graceful handling of missing Section 7
- **Complexity:** Simple — XML parser returns empty slice when Section 7 not found

## Implementation Phases

### Phase 1: Models + XML Parser (RED → GREEN)

Define response types and the XML parsing package. The parser is the riskiest component so we build and test it first.

| Task ID | Description | AC | Effort | Dependencies |
|---------|-------------|-----|--------|-------------|
| TASK-001 | Define SPL models (`SPLEntry`, `SPLDetail`, `InteractionSection`) in `internal/model/spl.go` | AC-002, AC-005 | 30m | — |
| TASK-002 | Create `internal/spl/parser.go` with `ParseInteractions(xmlData []byte) ([]InteractionSection, error)` | AC-004, AC-005, AC-014 | 2h | TASK-001 |
| TASK-003 | Write parser unit tests with sample SPL XML (Section 7 present, missing, malformed) | AC-005, AC-014 | 2h | TASK-002 |

**Phase effort:** ~4.5h
**Files created:** `internal/model/spl.go`, `internal/spl/parser.go`, `internal/spl/parser_test.go`

### Phase 2: Client Layer — SPL upstream methods (RED → GREEN)

New `SPLClient` interface and HTTP client in a separate file, following the `RxNormClient` pattern.

| Task ID | Description | AC | Effort | Dependencies |
|---------|-------------|-----|--------|-------------|
| TASK-004 | Define `SPLClient` interface with 3 methods: `SearchByName`, `FetchDetail`, `FetchXML` | All | 30m | — |
| TASK-005 | Define raw upstream types (`SPLEntryRaw`, `SPLMetaRaw`) for JSON mapping | AC-001, AC-002 | 30m | — |
| TASK-006 | Implement `HTTPSPLClient.SearchByName(ctx, name string) ([]SPLEntryRaw, *SPLMetaRaw, error)` — calls `spls-by-name?DRUGNAME={name}` | AC-001, AC-013 | 1h | TASK-004, TASK-005 |
| TASK-007 | Implement `HTTPSPLClient.FetchDetail(ctx, setid string) (*SPLEntryRaw, error)` — calls `spl-detail?SETID={setid}` | AC-003 | 45m | TASK-004, TASK-005 |
| TASK-008 | Implement `HTTPSPLClient.FetchXML(ctx, setid string) ([]byte, error)` — calls `spl-xml?SETID={setid}`, returns raw XML bytes | AC-004 | 45m | TASK-004 |
| TASK-009 | Add client HTTP tests (happy path + 404 + upstream error) via httptest | All | 2h | TASK-006–008 |

**Phase effort:** ~5.5h
**Files created:** `internal/client/spl.go`, `internal/client/spl_test.go`

### Phase 3: Service Layer — Caching + orchestration (RED → GREEN)

New `SPLService` following the `DrugDataService` pattern. Orchestrates client calls and caches results.

| Task ID | Description | AC | Effort | Dependencies |
|---------|-------------|-----|--------|-------------|
| TASK-010 | Create `SPLService` struct with `SPLClient`, Redis, metrics deps | All | 30m | TASK-004, TASK-001 |
| TASK-011 | Implement `SearchSPLs(ctx, name string) ([]model.SPLEntry, int, error)` — fetch from cache or upstream, return entries + total count | AC-001, AC-002, AC-006, AC-007, AC-013 | 1.5h | TASK-006, TASK-010 |
| TASK-012 | Implement `GetSPLDetail(ctx, setid string) (*model.SPLDetail, error)` — fetch detail + XML, parse interactions, cache result | AC-003, AC-004, AC-005, AC-006, AC-009, AC-014 | 2h | TASK-007, TASK-008, TASK-002, TASK-010 |
| TASK-013 | Service unit tests with miniredis mock (cache hit, miss, upstream errors, empty results, missing Section 7) | All | 2.5h | TASK-011, TASK-012 |

**Phase effort:** ~6.5h
**Files created:** `internal/service/spl.go`, `internal/service/spl_test.go`

### Phase 4: Handler Layer + Pagination (RED → GREEN)

New handler and offset-based pagination support. The spec uses `limit`/`offset` rather than the existing `page`/`limit` pattern, so we add an offset-based pagination helper.

| Task ID | Description | AC | Effort | Dependencies |
|---------|-------------|-----|--------|-------------|
| TASK-014 | Add `parseOffsetPagination(r, defaultLimit, maxLimit)` and `paginateSliceByOffset[T]` helpers to `internal/handler/pagination.go` | AC-012 | 1h | — |
| TASK-015 | Add offset pagination model (`OffsetPagination` with total/limit/offset) to `internal/model/spl.go` | AC-012 | 15m | TASK-001 |
| TASK-016 | Create `SPLHandler` struct with `SPLService` dependency | All | 15m | TASK-010 |
| TASK-017 | Implement `HandleSPLSearch` — `GET /v1/drugs/spls?name={name}&limit=20&offset=0` | AC-001, AC-002, AC-007, AC-008, AC-010, AC-011, AC-012, AC-013 | 1.5h | TASK-011, TASK-014, TASK-016 |
| TASK-018 | Implement `HandleSPLDetail` — `GET /v1/drugs/spls/{setid}` | AC-003, AC-005, AC-008, AC-009, AC-010, AC-011 | 1h | TASK-012, TASK-016 |
| TASK-019 | Add Swagger annotations to both handlers | All | 30m | TASK-017, TASK-018 |
| TASK-020 | Handler unit tests (mock service, all paths: success, empty, 404, 502, missing name param) | All | 2h | TASK-017, TASK-018 |

**Phase effort:** ~6.5h
**Files created:** `internal/handler/spl.go`, `internal/handler/spl_test.go`
**Files modified:** `internal/handler/pagination.go`, `internal/model/spl.go`

### Phase 5: Wire-up + Integration (RED → GREEN)

| Task ID | Description | AC | Effort | Dependencies |
|---------|-------------|-----|--------|-------------|
| TASK-021 | Register routes in `cmd/server/main.go`: `GET /v1/drugs/spls` and `GET /v1/drugs/spls/{setid}` | AC-010, AC-011 | 30m | TASK-017, TASK-018 |
| TASK-022 | Add integration tests (real Redis) for SPLService caching | AC-006 | 1.5h | TASK-011, TASK-012 |
| TASK-023 | Add E2E tests for both endpoints (search lipitor, search nonexistent, detail with interactions) | AC-001–009 | 2h | TASK-021 |
| TASK-024 | Regenerate Swagger docs (`make swagger`) | All | 15m | TASK-019 |

**Phase effort:** ~4.25h
**Files modified:** `cmd/server/main.go`
**Files created:** `internal/service/spl_integration_test.go`, `tests/e2e/spl_test.go`

### Phase 6: Documentation + Polish

| Task ID | Description | AC | Effort | Dependencies |
|---------|-------------|-----|--------|-------------|
| TASK-025 | Update `docs/sequence-diagram.md` with SPL search and detail flows | — | 1h | TASK-021 |
| TASK-026 | Update `CLAUDE.md` with new endpoints and upstream slugs (`spl-detail`, `spl-xml`) | — | 15m | TASK-021 |
| TASK-027 | Update CHANGELOG.md | — | 15m | All |
| TASK-028 | Run `make lint && make vet && make test-unit && make test-coverage` — full quality gates | — | 30m | All |

**Phase effort:** ~2h

## Effort Summary

| Phase | Description | Estimated Hours |
|-------|-------------|-----------------|
| Phase 1 | Models + XML parser | 4.5h |
| Phase 2 | Client layer | 5.5h |
| Phase 3 | Service layer | 6.5h |
| Phase 4 | Handler layer + pagination | 6.5h |
| Phase 5 | Wire-up + integration | 4.25h |
| Phase 6 | Documentation + polish | 2h |
| **Total** | | **29.25h** |
| **With 15% contingency** | | **~34h** |

## TDD Cycle Mapping

| Cycle | Phases | Focus |
|-------|--------|-------|
| **Cycle 1** | Phase 1 | XML parser — RED: write parser tests with sample XML. GREEN: implement parser. This is the riskiest component. |
| **Cycle 2** | Phase 2 + 3 | Client + service — RED: write client tests + service tests with mocks. GREEN: implement client + service with caching. |
| **Cycle 3** | Phase 4 + 5 | Handlers + wire-up — RED: write handler + E2E tests. GREEN: implement handlers, register routes. |
| **Cycle 4** | Phase 6 | Polish — docs, verify, coverage check. |

## Dependencies

### External
- **cash-drugs SPL endpoints** — `spls-by-name`, `spl-detail`, `spl-xml` must be configured and reachable
- **DailyMed SPL XML format** — Section 7 structure must be consistent enough for parsing

### Internal
- **Existing middleware** — auth, CORS, rate limiting (no changes needed, just route registration)
- **Redis** — same instance, new key namespace (`cache:spl:*`)
- **`client.ErrUpstream`** — reuse existing sentinel error for upstream failures

## Design Decisions

### 1. Separate SPLClient interface (not on DrugClient)
SPL browsing is a different domain from NDC/drug-name lookups. Following the `RxNormClient` precedent, `SPLClient` gets its own interface in `internal/client/spl.go` with its own `HTTPSPLClient` implementation. This keeps interfaces small and testable.

### 2. XML parser in dedicated package (`internal/spl/`)
XML parsing is complex enough to warrant isolation. `internal/spl/parser.go` exports `ParseInteractions(xmlData []byte) ([]model.InteractionSection, error)`. This makes it independently testable with sample XML fixtures and keeps the service layer clean.

### 3. Offset-based pagination for SPL endpoints
The spec requires `limit`/`offset` parameters. The existing codebase uses `page`/`limit`. Rather than changing existing endpoints, we add offset-based helpers alongside the existing page-based ones in `pagination.go`. The SPL response uses a separate `OffsetPagination` struct with `total`, `limit`, `offset` fields (matching the spec's response shape).

### 4. Two-call pattern for SPL detail
`GetSPLDetail` makes two upstream calls: `spl-detail` (metadata) + `spl-xml` (raw XML for parsing). The assembled result (metadata + parsed interactions) is cached as one unit with 60-minute sliding TTL. This avoids re-parsing XML on every request.

### 5. FetchXML returns raw bytes
The `SPLClient.FetchXML` method returns `[]byte` rather than a parsed structure. This keeps the client layer thin (just HTTP transport) and delegates all XML interpretation to the `spl` parser package.

## Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| SPL XML structure varies across manufacturers | Medium | High | Parser tests use multiple real XML samples; graceful fallback to empty interactions |
| Section 7 detection by title string is fragile | Medium | Medium | Search for multiple patterns (`7 DRUG INTERACTIONS`, `DRUG INTERACTIONS`); fallback to empty array |
| Large XML payloads (200KB+) slow down parsing | Low | Medium | Parse only Section 7, discard rest; cache parsed result so XML is fetched once |
| `spl-detail` and `spl-xml` are separate calls (latency) | Medium | Medium | Cache assembled detail; both calls are to same upstream host (fast LAN) |
| E2E tests flaky due to upstream timeouts | Medium | Low | Same pattern as RxNorm E2E: accept 502 as valid for uncached queries |
| `spl-xml` endpoint returns non-XML or empty body | Low | Medium | Parser returns empty interactions + logs warning; handler still returns metadata |

## Spec Traceability

| AC | Tasks |
|----|-------|
| AC-001 | TASK-005, TASK-006, TASK-011, TASK-017, TASK-023 |
| AC-002 | TASK-001, TASK-005, TASK-011, TASK-017, TASK-023 |
| AC-003 | TASK-001, TASK-007, TASK-012, TASK-018, TASK-023 |
| AC-004 | TASK-002, TASK-008, TASK-012, TASK-023 |
| AC-005 | TASK-001, TASK-002, TASK-003, TASK-012, TASK-018, TASK-023 |
| AC-006 | TASK-011, TASK-012, TASK-013, TASK-022 |
| AC-007 | TASK-011, TASK-017, TASK-020, TASK-023 |
| AC-008 | TASK-006, TASK-007, TASK-017, TASK-018, TASK-020 |
| AC-009 | TASK-007, TASK-012, TASK-018, TASK-020, TASK-023 |
| AC-010 | TASK-021 (route registration inside auth middleware group) |
| AC-011 | TASK-021 (route registration inside rate-limit middleware group) |
| AC-012 | TASK-014, TASK-015, TASK-017 |
| AC-013 | TASK-006, TASK-011, TASK-017 |
| AC-014 | TASK-002, TASK-003, TASK-012 |

## File Changes Summary

### New Files
| File | Purpose |
|------|---------|
| `internal/model/spl.go` | SPL response models (`SPLEntry`, `SPLDetail`, `InteractionSection`, `OffsetPagination`) |
| `internal/spl/parser.go` | XML Section 7 parser — `ParseInteractions(xmlData []byte)` |
| `internal/spl/parser_test.go` | Parser unit tests with sample XML fixtures |
| `internal/client/spl.go` | `SPLClient` interface + `HTTPSPLClient` implementation |
| `internal/client/spl_test.go` | Client HTTP tests (httptest) |
| `internal/service/spl.go` | `SPLService` with Redis caching |
| `internal/service/spl_test.go` | Service unit tests (miniredis) |
| `internal/service/spl_integration_test.go` | Service integration tests (real Redis) |
| `internal/handler/spl.go` | `SPLHandler` with search + detail handlers |
| `internal/handler/spl_test.go` | Handler unit tests |
| `tests/e2e/spl_test.go` | E2E tests against live upstream |

### Modified Files
| File | Change |
|------|--------|
| `cmd/server/main.go` | Register SPL routes: `GET /v1/drugs/spls` and `GET /v1/drugs/spls/{setid}` |
| `internal/handler/pagination.go` | Add `parseOffsetPagination` and `paginateSliceByOffset` helpers |
| `docs/sequence-diagram.md` | Add SPL search + detail flow diagrams |
| `CLAUDE.md` | Add endpoints + upstream slugs (`spl-detail`, `spl-xml`) |
| `CHANGELOG.md` | Add M6 entry |

## Success Criteria

- [ ] All 14 acceptance criteria implemented and tested
- [ ] Code coverage >= 80% on new packages (`internal/spl/`, `internal/client/spl.go`, `internal/service/spl.go`, `internal/handler/spl.go`)
- [ ] All quality gates passing (lint, vet, tests)
- [ ] XML parser handles real SPL XML from DailyMed
- [ ] E2E tests pass against real upstream
- [ ] Swagger docs include both new endpoints
- [ ] Sequence diagrams updated

## Next Steps

1. Review and approve this plan
2. Run `/add:tdd-cycle specs/spl-browser.md` to begin Cycle 1 (XML parser)

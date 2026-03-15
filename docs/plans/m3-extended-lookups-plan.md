# Implementation Plan: M3 Extended Lookups

**Specs:** `specs/drug-class-lookup.md`, `specs/drug-data-listings.md`
**Created:** 2026-03-09
**Status:** In Progress

## Task Breakdown

### Phase 1: Shared Infrastructure

| # | Task | Files | ACs |
|---|------|-------|-----|
| 1.1 | Add `PharmClass` parsed type and parser utility | `internal/pharma/parse.go`, `internal/pharma/parse_test.go` | DCL AC-008 |
| 1.2 | Add `DeduplicateBrandNames` utility | `internal/pharma/brands.go`, `internal/pharma/brands_test.go` | DCL AC-005, AC-006 |
| 1.3 | Extend `DrugClient` interface with `LookupByName`, `FetchDrugNames`, `FetchDrugClasses`, `LookupByPharmClass` | `internal/client/client.go` | DCL AC-001, DDL AC-001, AC-005, AC-009 |
| 1.4 | Implement HTTP methods on `HTTPDrugClient` | `internal/client/client.go`, `internal/client/client_test.go` | DCL AC-002, AC-012, DDL AC-020 |
| 1.5 | Add new response models | `internal/model/response.go` | DCL AC-003-007, DDL AC-003, AC-007, AC-011, AC-017 |

### Phase 2: Drug Class Lookup (spec 1)

| # | Task | Files | ACs |
|---|------|-------|-----|
| 2.1 | RED: Write failing tests for drug class handler | `internal/handler/drugclass_test.go` | DCL AC-001 through AC-016 |
| 2.2 | GREEN: Implement `HandleDrugClassLookup` handler | `internal/handler/drugclass.go` | DCL AC-001 through AC-016 |
| 2.3 | Wire route `GET /v1/drugs/class` | `cmd/server/main.go` | DCL AC-013, AC-014 |

### Phase 3: Drug Data Listings (spec 2)

| # | Task | Files | ACs |
|---|------|-------|-----|
| 3.1 | Add Redis cache service with lazy-load + sliding TTL | `internal/cache/cache.go`, `internal/cache/cache_test.go` | DDL AC-015, AC-016 |
| 3.2 | RED: Write failing tests for names handler | `internal/handler/drugnames_test.go` | DDL AC-001 through AC-004, AC-017-019, AC-021 |
| 3.3 | GREEN: Implement `HandleDrugNames` handler | `internal/handler/drugnames.go` | DDL AC-001 through AC-004 |
| 3.4 | RED: Write failing tests for classes handler | `internal/handler/drugclasses_test.go` | DDL AC-005 through AC-008, AC-017 |
| 3.5 | GREEN: Implement `HandleDrugClasses` handler | `internal/handler/drugclasses.go` | DDL AC-005 through AC-008 |
| 3.6 | RED: Write failing tests for drugs-by-class handler | `internal/handler/drugsbyclass_test.go` | DDL AC-009 through AC-012, AC-017, AC-022 |
| 3.7 | GREEN: Implement `HandleDrugsByClass` handler | `internal/handler/drugsbyclass.go` | DDL AC-009 through AC-012 |
| 3.8 | Wire routes for all 3 listing endpoints | `cmd/server/main.go` | DDL AC-013, AC-014 |

### Phase 4: Verify

| # | Task | Files | ACs |
|---|------|-------|-----|
| 4.1 | Run full test suite, verify coverage | — | All |
| 4.2 | Run lint + vet | — | All |

## Test Strategy

- **Unit tests:** Mock client interface for all handler tests; mock Redis for cache tests
- **Integration tests:** Redis cache operations (tagged `//go:build integration`)
- **E2E tests:** Against live cash-drugs (future, not blocking M3)

## Dependencies

- M2 auth/rate limit middleware (done)
- cash-drugs endpoints: `fda-ndc`, `drugnames`, `drugclasses` (available)

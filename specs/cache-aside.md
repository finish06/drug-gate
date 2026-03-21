# Spec: Generic CacheAside[T]

**Version:** 0.1.0
**Created:** 2026-03-20
**PRD Reference:** docs/prd.md — M8: Cache Architecture + Clinical Data
**Status:** Approved

## 1. Overview

Replace the per-endpoint cache boilerplate across all 3 service files with a typed generic `CacheAside[T]` utility. Eliminates ~165 lines of duplicated cache fetch/store/expire logic while preserving the existing sliding TTL behavior via `GetEx`.

### User Story

As a **developer**, I want **a single cache utility that handles all Redis read/write patterns**, so that **adding new cached endpoints requires zero boilerplate and cache behavior is consistent across the entire API**.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | `CacheAside[T]` generic type exists in `internal/cache/` package | Must |
| AC-002 | `Get` method: tries Redis cache via `GetEx` (sliding TTL), returns cached data on hit | Must |
| AC-003 | `Get` method: on cache miss, calls fetch function, stores result in Redis, returns data | Must |
| AC-004 | `Get` method: on unmarshal failure, treats as cache miss and fetches fresh | Must |
| AC-005 | `Get` method: records cache hit/miss metrics via provided Metrics reference | Must |
| AC-006 | `Get` method: propagates upstream errors from fetch function | Must |
| AC-007 | TTL is configurable per CacheAside instance | Must |
| AC-008 | All 11 cached methods across DrugDataService, RxNormService, SPLService migrated | Must |
| AC-009 | All existing service tests pass without modification (behavior preserved) | Must |
| AC-010 | Net reduction of ~150+ lines of code | Should |
| AC-011 | `recordCache` helper methods removed from all 3 services | Should |

## 3. API Contract

### CacheAside[T] Constructor

```go
func New[T any](rdb *redis.Client, m *metrics.Metrics, key string, ttl time.Duration, keyType string) *CacheAside[T]
```

### Get Method

```go
func (c *CacheAside[T]) Get(ctx context.Context, fetch func(ctx context.Context) (T, error)) (T, error)
```

## 4. Migration Map

| Service | Method | Cache Key | TTL | Metrics Key |
|---------|--------|-----------|-----|-------------|
| DrugDataService | GetDrugNames | cache:drugnames | 60m | drugnames |
| DrugDataService | GetDrugClasses | cache:drugclasses | 60m | drugclasses |
| DrugDataService | GetDrugsByClass | cache:drugsbyclass:{class} | 60m | drugsbyclass |
| RxNormService | Search | cache:rxnorm:search:{name} | 24h | rxnorm-search |
| RxNormService | GetNDCs | cache:rxnorm:ndcs:{rxcui} | 7d | rxnorm-ndcs |
| RxNormService | GetGenerics | cache:rxnorm:generic:{rxcui} | 7d | rxnorm-generic |
| RxNormService | GetRelated | cache:rxnorm:related:{rxcui} | 7d | rxnorm-related |
| RxNormService | GetProfile | cache:rxnorm:profile:{name} | 24h | rxnorm-profile |
| SPLService | SearchSPLs | cache:spls:name:{name} | 60m | spls-by-name |
| SPLService | GetSPLDetail | cache:spl:detail:{setid} | 60m | spl-detail |
| SPLService | GetInteractionsForDrug | cache:spl:interactions:{name} | 60m | spl-interactions |

## 5. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Nil metrics pointer | Skip metric recording (no panic) |
| Fetch returns nil value | Cache the nil/zero value (prevent repeated upstream calls) |
| Redis unavailable | Cache miss on every call (fetch from upstream, warn in logs) |
| Corrupt cache data | Unmarshal fails → treat as miss → fetch fresh |

## 6. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-20 | 0.1.0 | calebdunn | Initial spec |

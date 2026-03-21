# M8 — Cache Architecture + Clinical Data

**Goal:** Clean up technical debt that unblocks faster development, double the clinical data coverage by parsing additional SPL sections.

**Status:** IN_PROGRESS
**Target Maturity:** Beta
**Appetite:** 2 weeks
**Started:** 2026-03-20

## Success Criteria

- [x] CacheAside[T] generic used by all cached endpoints (drug names, classes, NDC, RxNorm, SPL)
- [x] Net reduction of ~300 lines of cache boilerplate
- [x] TTL configurable per environment via config/env vars
- [x] SPL detail endpoint returns sections 4, 5, 6, and 7
- [x] Drug info card includes contraindications, warnings, and adverse reactions
- [x] 80%+ test coverage on new code

## Hill Chart

| Feature | Position | Notes |
|---------|----------|-------|
| Generic CacheAside[T] | VERIFIED | 9 tests, 211 lines eliminated, all 11 methods migrated |
| Expanded SPL Sections | VERIFIED | 6 tests, sections 4-6 parsing, model + handler updated |

## Features

| Feature | Spec | Current Position | Target |
|---------|------|-----------------|--------|
| Generic CacheAside[T] | specs/cache-aside.md | VERIFIED | VERIFIED |
| Expanded SPL Sections | specs/spl-expanded-sections.md | VERIFIED | VERIFIED |

## Dependencies

- CacheAside[T] is foundational — implemented first, then SPL endpoints migrated to use it
- Expanded SPL sections depends on existing SPL XML parser from M6
- Both features touch internal/service/ — executed sequentially

## Risks

| Risk | Mitigation |
|------|-----------|
| CacheAside[T] refactor breaks existing cache behavior | All existing tests pass without modification |
| SPL XML structure varies across sections 4-6 | Regex handles numbered and unnumbered formats, missing sections return empty |
| TTL configuration adds environment complexity | Deferred — current sliding TTL preserved, env config in future cycle |

## Cycles

| Cycle | Features | Status | Notes |
|-------|----------|--------|-------|
| cycle-4 | CacheAside[T] + Expanded SPL Sections | COMPLETE | 211 lines eliminated, 15 new tests, 81.1% coverage. PR #16 merged. |

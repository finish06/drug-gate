# M8 — Cache Architecture + Clinical Data

**Goal:** Clean up technical debt that unblocks faster development, double the clinical data coverage by parsing additional SPL sections.

**Status:** IN_PROGRESS
**Target Maturity:** Beta
**Appetite:** 2 weeks
**Started:** 2026-03-20

## Success Criteria

- [ ] CacheAside[T] generic used by all cached endpoints (drug names, classes, NDC, RxNorm, SPL)
- [ ] Net reduction of ~300 lines of cache boilerplate
- [ ] TTL configurable per environment via config/env vars
- [ ] SPL detail endpoint returns sections 4, 5, 6, and 7
- [ ] Drug info card includes contraindications, warnings, and adverse reactions
- [ ] 80%+ test coverage on new code

## Hill Chart

| Feature | Position | Notes |
|---------|----------|-------|
| Generic CacheAside[T] | SHAPED | Replaces per-endpoint cache boilerplate across all services |
| Expanded SPL Sections | SHAPED | Parse sections 4 (Contraindications), 5 (Warnings), 6 (Adverse Reactions) |

## Features

| Feature | Spec | Current Position | Target |
|---------|------|-----------------|--------|
| Generic CacheAside[T] | specs/cache-aside.md | SHAPED | VERIFIED |
| Expanded SPL Sections | specs/spl-expanded-sections.md | SHAPED | VERIFIED |

## Dependencies

- CacheAside[T] is foundational — should be implemented first, then SPL endpoints migrated to use it
- Expanded SPL sections depends on existing SPL XML parser (internal/spl/) from M6
- Both features touch internal/service/ — serialized execution recommended

## Risks

| Risk | Mitigation |
|------|-----------|
| CacheAside[T] refactor breaks existing cache behavior | Comprehensive tests before migration, compare behavior before/after |
| SPL XML structure varies across sections 4-6 | Probe live data first, handle missing sections gracefully |
| TTL configuration adds environment complexity | Use sensible defaults, env var override is optional |

## Cycles

| Cycle | Features | Status | Notes |
|-------|----------|--------|-------|
| cycle-4 | CacheAside[T] + Expanded SPL Sections | PLANNED | TBD |

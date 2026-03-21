# Cycle 4 — Cache Architecture + Clinical Data

**Milestone:** M8 — Cache Architecture + Clinical Data
**Maturity:** Beta
**Status:** PLANNED
**Started:** TBD
**Completed:** TBD
**Duration Budget:** 24 hours (away mode)

## Work Items

| Feature | Current Pos | Target Pos | Assigned | Est. Effort | Validation |
|---------|-------------|-----------|----------|-------------|------------|
| Generic CacheAside[T] | SHAPED | VERIFIED | Agent | ~6 hours | Spec + generic + migrate all 11 methods + tests |
| Expanded SPL Sections | SHAPED | VERIFIED | Agent | ~4 hours | Spec + parser + models + tests + probe live data |
| Documentation | — | DONE | Agent | ~1 hour | Changelog, Swagger, milestone, learnings |

## Dependencies & Serialization

```
Phase 1: Specs (both features)
    ↓
Phase 2: CacheAside[T] generic (foundational — all services depend on it)
    ↓
Phase 3: Migrate all 11 cache methods to CacheAside[T]
    ↓
Phase 4: Expanded SPL Sections (uses CacheAside[T] for new cached data)
    ↓
Phase 5: Finalize (tests, Swagger, docs, PR)
```

## Parallel Strategy

Single-threaded execution. CacheAside[T] must land before SPL sections.

## Execution Plan

### Phase 1: Specs (~1h)

1. `specs/cache-aside.md` — Generic CacheAside[T] utility
   - Type signature, TTL strategy (preserve sliding TTL via GetEx)
   - Migration plan for all 11 methods across 3 services
   - Metrics integration (recordCache hit/miss)
   - Error handling (unmarshal failure → fetch fresh)

2. `specs/spl-expanded-sections.md` — Sections 4-6 parsing
   - Extend existing regex parser for sections 4, 5, 6
   - New model fields on SPLDetail
   - Drug info card includes new sections
   - Raw text output (structured subsections deferred)

### Phase 2: CacheAside[T] Generic (~3h)

**RED:**
1. Write tests for `CacheAside[T]` utility:
   - Cache hit returns deserialized data
   - Cache miss calls fetch function, stores result
   - Sliding TTL reset on cache hit (GetEx behavior)
   - Unmarshal failure triggers fresh fetch
   - Upstream error propagated
   - Metrics recorded (hit/miss with key type)
   - Nil/empty results cached (don't re-fetch on every call)

**GREEN:**
1. Create `internal/cache/aside.go`:
   ```go
   type FetchFunc[T any] func(ctx context.Context) (T, error)

   type CacheAside[T any] struct {
       rdb     *redis.Client
       metrics *metrics.Metrics
       key     string
       ttl     time.Duration
       keyType string  // for metrics labels
   }

   func (c *CacheAside[T]) Get(ctx context.Context, fetch FetchFunc[T]) (T, error)
   ```
2. Implement Get: try cache (GetEx) → unmarshal → hit metric → return
   OR miss metric → fetch() → marshal → Set → return

### Phase 3: Migrate All Services (~3h)

Migrate in order, running tests after each:

1. **DrugDataService** (3 methods: GetDrugNames, GetDrugClasses, GetDrugsByClass)
   - Replace cache boilerplate with CacheAside[T].Get()
   - Keep transform logic inline (fetch function includes transform)
   - Verify all existing drugdata_test.go tests still pass

2. **RxNormService** (5 methods: Search, Profile, NDCs, Generics, Related)
   - Same pattern — replace boilerplate, keep transforms
   - Verify rxnorm_test.go still passes
   - Note: RxNorm uses two TTLs (24h for search/profile, 7d for RxCUI-based)

3. **SPLService** (3 methods: SearchSPLs, GetSPLDetail, GetDrugInfo)
   - Same pattern
   - Verify spl_test.go still passes

4. Count lines eliminated — target ~165 lines removed

### Phase 4: Expanded SPL Sections (~4h)

**Probe live data first:**
1. Pick random 10% from top 200 drugs (use drugnames cache)
2. Fetch SPL XML for ~20 drugs via cash-drugs
3. Check which have sections 4, 5, 6 — note structural variations

**RED:**
1. Write parser tests for sections 4, 5, 6:
   - Section 4 (Contraindications) extracted
   - Section 5 (Warnings and Precautions) extracted
   - Section 6 (Adverse Reactions) extracted
   - Missing sections return empty (graceful)
   - Multiple subsections (5.1, 5.2) captured

2. Write model tests:
   - SPLDetail includes new section fields
   - Drug info response includes new sections

**GREEN:**
1. Extend `ParseInteractions` → `ParseSections` (or add new function):
   - Regex patterns for sections 4, 5, 6 (same approach as Section 7)
   - Handle numbered (e.g., "4 CONTRAINDICATIONS") and unnumbered formats
2. Add new fields to SPLDetail model:
   - `Contraindications []InteractionSection`
   - `Warnings []InteractionSection`
   - `AdverseReactions []InteractionSection`
3. Update SPLService.GetSPLDetail to populate new fields
4. Update drug info response to include new sections

### Phase 5: Finalize (~1h)

1. Run full test suite — verify no regressions
2. Run `make lint` and `make vet`
3. Verify coverage stays above 80%
4. Update Swagger annotations
5. Run `make swagger`
6. Run `make k6-smoke` against staging (after deploy)
7. Update M8 milestone hill chart
8. Write learning checkpoint (L-014+)
9. Update CHANGELOG [Unreleased]
10. Create PR

## Validation Criteria

### Per-Item Validation
- **CacheAside[T]:** All 11 existing cache methods migrated, all existing tests pass, ~165 lines eliminated
- **SPL Sections:** Sections 4-6 parsed for probe drugs, graceful on missing sections, drug info card updated
- **Documentation:** Swagger regenerated, CHANGELOG updated, milestone updated, learnings recorded

### Cycle Success Criteria
- [ ] CacheAside[T] generic implemented and tested
- [ ] All 11 cache methods migrated (DrugDataService, RxNormService, SPLService)
- [ ] Net line reduction ~150+ lines
- [ ] SPL sections 4, 5, 6 parsed and returned
- [ ] Drug info card includes new sections
- [ ] Coverage stays above 80%
- [ ] No regressions in existing test suite
- [ ] k6 smoke test passes on staging
- [ ] Learning checkpoint written
- [ ] Code committed, pushed, PR created

## Agent Autonomy (Away Mode)

**Level:** High autonomy (beta maturity, 24h away session)

**Autonomous actions:**
- Write specs, execute TDD phases, commit after each phase
- Push regularly, create PR when done
- Fix lint/type errors
- Probe live cash-drugs for SPL section data
- Run k6 smoke test after staging deploys

**Boundaries:**
- Do NOT merge to main
- Do NOT deploy
- If CacheAside[T] migration breaks tests, fix before moving to SPL sections
- If a service has unusual cache behavior, preserve it and document the difference
- Write learning checkpoints after each phase (L-013 directive: documentation is first-class)

## Notes

- RxNorm uses two different TTLs (24h search/profile, 7d RxCUI lookups) — CacheAside[T] must accept TTL per instance
- AutocompleteDrugs method doesn't cache directly (reuses GetDrugNames) — no migration needed
- The `recordCache` helper on each service can be removed once CacheAside handles metrics
- SPL sections may reuse `InteractionSection` model type (it's just Title+Text)
- openapi-enrichment.md spec is still Approved (not part of this milestone)

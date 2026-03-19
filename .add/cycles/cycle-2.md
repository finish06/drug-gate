# Cycle 2 — Background Indexer, E2E Tests, Docs

**Milestone:** M6 — SPL Interactions
**Maturity:** Beta
**Status:** PLANNED
**Started:** 2026-03-18
**Duration Budget:** 24 hours (away mode)

## Work Items

| Feature | Current Pos | Target Pos | Assigned | Est. Effort | Validation |
|---------|-------------|-----------|----------|-------------|------------|
| Background Indexer | SHAPED | VERIFIED | Agent | ~4 hours | Goroutine starts on boot, caches parsed interactions, unit tests |
| E2E Tests | SHAPED | VERIFIED | Agent | ~4 hours | SPL search, detail, drug info, interaction checker against live cash-drugs |
| XML Parser Edge Cases | IN_PROGRESS | VERIFIED | Agent | ~2 hours | Test drugs with unusual Section 7 (no subsections, empty, very long) |
| Cross-Reference Accuracy | IN_PROGRESS | VERIFIED | Agent | ~2 hours | E2E tests for known drug pairs (warfarin+aspirin, warfarin+fluconazole) |
| Swagger/OpenAPI Docs | SHAPED | VERIFIED | Agent | ~1 hour | Swagger annotations on 4 new endpoints |
| PRD Roadmap Update | SHAPED | DONE | Agent | ~30 min | M4.5 → M6 rename, mark status |

## Dependencies & Serialization

```
XML Parser Edge Cases (independent — can start immediately)
    ↓
E2E Tests (uses parser, validates full flow)
    ↓
Cross-Reference Accuracy (E2E subset, validates real drug pairs)

Background Indexer (independent — parallel to above)

Swagger Docs (independent — parallel)
PRD Update (independent — parallel)
```

## Parallel Strategy

### File Reservations
- **E2E + Edge Cases:** tests/e2e/spl_test.go, internal/spl/parser_test.go
- **Indexer:** internal/spl/indexer.go, internal/spl/indexer_test.go, cmd/server/main.go (startup wiring)
- **Docs:** internal/handler/spl.go (swagger comments only), docs/prd.md

### Merge Sequence
Single feature branch — all work serialized within one agent.

## Execution Plan

### Phase 1: XML Parser Edge Cases (~2h)
1. Probe live cash-drugs for drugs with varying Section 7 structures
2. Test drugs with: no Section 7, single section only, deeply nested subsections, very long text
3. Add parser tests for discovered edge cases
4. Fix parser if any edge cases fail

### Phase 2: Background Indexer (~4h)
1. Create `internal/spl/indexer.go` — background goroutine
2. On startup: read drug names from Redis cache, iterate top N, fetch+parse+cache interactions
3. Periodic refresh (configurable interval, default 24h)
4. Graceful shutdown (context cancellation)
5. Unit tests with miniredis + mock SPL client
6. Wire into cmd/server/main.go startup

### Phase 3: E2E Tests (~4h)
1. Create `tests/e2e/spl_test.go`
2. Test SPL search: known drugs (lipitor, warfarin, metformin)
3. Test SPL detail: fetch real setid, verify Section 7 parsed
4. Test drug info: by name + by NDC
5. Test interaction checker: warfarin+aspirin (known interaction), metformin+lisinopril (no interaction)
6. All tests use 30s timeout, graceful 502/timeout handling
7. Cross-reference accuracy: verify warfarin+aspirin finds "aspirin" in Section 7.3

### Phase 4: Swagger + PRD (~1.5h)
1. Add Swagger annotations to HandleSearchSPLs, HandleSPLDetail, HandleDrugInfo, HandleCheckInteractions
2. Regenerate swagger docs
3. Update docs/prd.md: M4.5 → M6, mark features, update roadmap table

## Validation Criteria

### Per-Item Validation
- **Indexer:** Starts on boot, indexes at least 10 drugs in test, unit tests pass
- **E2E:** All SPL E2E tests pass against live cash-drugs (or gracefully skip on timeout)
- **Parser Edge Cases:** At least 3 new edge case tests added
- **Cross-Ref Accuracy:** warfarin+aspirin interaction found in E2E
- **Swagger:** All 4 endpoints documented, `make swagger` succeeds
- **PRD:** Roadmap table updated, M6 reflected

### Cycle Success Criteria
- [ ] Background indexer implemented and tested
- [ ] E2E tests passing (or gracefully skipping on upstream timeout)
- [ ] XML parser handles edge cases
- [ ] Cross-reference accuracy validated via E2E
- [ ] Swagger docs updated
- [ ] PRD roadmap updated
- [ ] Coverage stays above 80%
- [ ] No regressions
- [ ] Code committed, pushed, PR created

## Agent Autonomy (Away Mode)

**Level:** High autonomy (beta maturity, 24h away session)

**Autonomous actions:**
- Execute all phases sequentially
- Commit after each phase with conventional commits
- Push regularly
- Create PR when done
- Fix lint/type errors
- Probe live cash-drugs for E2E data

**Boundaries:**
- Do NOT merge to main
- Do NOT deploy
- If cash-drugs is down, skip E2E and log blocker
- If XML parsing edge case reveals a design issue, fix it and document the decision

## Notes

- `spls-by-class` still deferred (unreliable)
- E2E tests should follow same pattern as existing tests/e2e/ (TestMain setup, environment vars)
- Indexer should not block server startup — run in background goroutine
- Indexer should be optional (skip if Redis is empty or drugnames not cached yet)

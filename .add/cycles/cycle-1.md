# Cycle 1 — SPL Foundation + Drug Info Card

**Milestone:** M6 — SPL Interactions
**Maturity:** Beta
**Status:** PLANNED
**Started:** 2026-03-17
**Duration Budget:** Overnight (~8-12 hours)

## Work Items

| Feature | Current Pos | Target Pos | Assigned | Est. Effort | Validation |
|---------|-------------|-----------|----------|-------------|------------|
| SPL Document Browser | SHAPED | VERIFIED | Agent | ~4 hours | AC-001 through AC-014 passing |
| Drug Info Card | SHAPED | IN_PROGRESS | Agent | ~3 hours | AC-001 through AC-010 passing |

## Dependencies & Serialization

```
SPL Document Browser (foundation)
    ↓ (Drug Info Card reuses client, XML parsing, models)
Drug Info Card
```

Interaction Checker deferred to cycle-2 (depends on both above being VERIFIED).

## Execution Plan

### Phase 1: SPL Browser (foundation)

1. **Plan:** Write `docs/plans/spl-browser-plan.md`
2. **Client methods:** Add `FetchSPLsByName`, `FetchSPLDetail`, `FetchSPLXML` to cash-drugs client
3. **XML parser:** Create `internal/spl/` package for Section 7 extraction
4. **Models:** Add `SPLEntry`, `SPLDetail`, `InteractionSection` to `internal/model/`
5. **Service:** Create `SPLService` in `internal/service/` with Redis caching
6. **Handler:** Add `SPLHandler` with routes `GET /v1/drugs/spls` and `GET /v1/drugs/spls/{setid}`
7. **TDD:** RED → GREEN → REFACTOR for each component
8. **Verify:** Run full test suite + lint

### Phase 2: Drug Info Card

1. **Plan:** Write `docs/plans/spl-drug-info-plan.md`
2. **Handler:** Add `GET /v1/drugs/info` route
3. **NDC resolution:** Wire NDC → drug name → SPL lookup flow
4. **TDD:** RED → GREEN → REFACTOR
5. **Verify:** Run full test suite + lint

### Phase 3 (if time permits): Background Indexer

1. Start background indexer skeleton (goroutine, Redis iteration)
2. Index top 100 drugs from drugnames cache
3. Unit tests for indexer logic

## Validation Criteria

### Per-Item Validation
- **SPL Browser:** All 14 ACs tested and passing, client methods tested against mock upstream, XML parser handles warfarin Section 7 correctly
- **Drug Info Card:** ACs 1-10 tested and passing, NDC resolution flow works end-to-end

### Cycle Success Criteria
- [ ] SPL Browser reaches VERIFIED (all ACs, tests pass, lint clean)
- [ ] Drug Info Card reaches IN_PROGRESS or VERIFIED
- [ ] Coverage stays above 80%
- [ ] No regressions in existing tests
- [ ] Code committed and pushed to feature branch

## Agent Autonomy (Away Mode)

**Level:** High autonomy (beta maturity, overnight away session)

**Autonomous actions:**
- Write implementation plans
- Execute TDD cycles (RED → GREEN → REFACTOR → VERIFY)
- Commit to feature branch with conventional commits
- Push to remote
- Create PR when feature is ready
- Fix lint/type errors

**Boundaries:**
- Do NOT merge to main
- Do NOT deploy to staging/production
- If spec is ambiguous, make reasonable choice and document in commit message
- If blocked on upstream (cash-drugs down), log blocker and move to next task
- Log decisions in `.add/handoff.md`

## Notes

- `spls-by-class` endpoint is unreliable (timeouts) — not used in this cycle
- SPL XML is ~200KB — parse only Section 7, cache parsed result (not raw XML)
- Section 7 structure varies per drug — regex-based extraction, not full XML parser
- Warfarin is the best test case (rich Section 7 with subsections 7.1-7.5)

# M6 — SPL Interactions

**Goal:** Expose drug interaction data from FDA Structured Product Labels (SPL) via three complementary APIs: a document browser, drug info cards with interaction sections, and a multi-drug interaction checker with backend cross-referencing.

**Status:** DONE
**Target Maturity:** Beta
**Appetite:** 1 week
**Started:** 2026-03-17

## Success Criteria

- [x] SPL document search by drug name returns metadata (title, setid, published_date, spl_version)
- [x] SPL detail endpoint returns parsed Section 7 (Drug Interactions) text from XML
- [x] Drug info card endpoint returns SPL metadata + structured interaction sections
- [x] Multi-drug interaction checker accepts 2+ drug names/NDCs and returns cross-referenced warnings
- [ ] Background indexer pre-fetches and caches parsed interaction data from popular drugs
- [x] All endpoints authenticated, rate-limited, and cached in Redis
- [x] 80%+ test coverage on new code

## Hill Chart

| Feature | Position | Notes |
|---------|----------|-------|
| SPL Document Browser | VERIFIED | Merged in PR #12 — 22 tests |
| Drug Info Card | VERIFIED | Merged in PR #12 — 11 tests |
| Drug Interaction Checker | VERIFIED | Merged in PR #12 — 17 tests |
| Background Indexer | VERIFIED | Delivered in cycle-2, PR #13 |
| E2E Tests | VERIFIED | Delivered in cycle-2, PR #13 |

## Features

| Feature | Spec | Current Position | Target |
|---------|------|-----------------|--------|
| SPL Document Browser | specs/spl-browser.md | VERIFIED | VERIFIED |
| Drug Info Card with Interactions | specs/spl-drug-info.md | VERIFIED | VERIFIED |
| Drug Interaction Checker | specs/spl-interaction-checker.md | VERIFIED | VERIFIED |
| Background Indexer | specs/spl-interaction-checker.md (section) | SHAPED | VERIFIED |
| E2E Tests | — | SHAPED | VERIFIED |

## Dependencies

- SPL Browser is foundational (client methods, models, caching)
- Drug Info Card depends on Browser (XML parsing, Section 7 extraction)
- Interaction Checker depends on both (cross-referencing logic)
- Background Indexer depends on all 3 (uses existing service methods)
- E2E tests depend on all endpoints being wired

## Risks

| Risk | Mitigation |
|------|-----------|
| SPL XML parsing is complex (semi-structured, varies per drug) | Start with regex extraction of Section 7, iterate on edge cases |
| `spls-by-class` endpoint unreliable (timeouts) | Use `spls-by-name` as primary lookup; defer class-based search |
| Large XML documents (200KB+) | Cache parsed results in Redis, not raw XML |
| Cross-referencing accuracy | Start with text-based matching, not clinical NLP |

## Cycles

| Cycle | Features | Status | Notes |
|-------|----------|--------|-------|
| cycle-1 | SPL Browser + Drug Info Card + Interaction Checker | COMPLETE | All 3 features delivered in overnight away session. PR #12 merged. |
| cycle-2 | Background Indexer + E2E Tests + Docs | COMPLETE | Delivered in PR #13, v0.6.1 tagged |

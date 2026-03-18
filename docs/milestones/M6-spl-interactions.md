# M6 — SPL Interactions

**Goal:** Expose drug interaction data from FDA Structured Product Labels (SPL) via three complementary APIs: a document browser, drug info cards with interaction sections, and a multi-drug interaction checker with backend cross-referencing.

**Status:** IN_PROGRESS
**Target Maturity:** Beta
**Appetite:** 1 week
**Started:** 2026-03-17

## Success Criteria

- [ ] SPL document search by drug name returns metadata (title, setid, published_date, spl_version)
- [ ] SPL detail endpoint returns parsed Section 7 (Drug Interactions) text from XML
- [ ] Drug info card endpoint returns SPL metadata + structured interaction sections
- [ ] Multi-drug interaction checker accepts 2+ drug names/NDCs and returns cross-referenced warnings
- [ ] Background indexer pre-fetches and caches parsed interaction data from popular drugs
- [ ] All endpoints authenticated, rate-limited, and cached in Redis
- [ ] 80%+ test coverage on new code

## Hill Chart

| Feature | Position | Notes |
|---------|----------|-------|
| SPL Document Browser | SHAPED | Spec needed, upstream endpoints discovered |
| Drug Info Card | SHAPED | Spec needed, depends on XML parsing |
| Drug Interaction Checker | SHAPED | Spec needed, depends on browser + card |

## Features

| Feature | Spec | Current Position | Target |
|---------|------|-----------------|--------|
| SPL Document Browser | specs/spl-browser.md | SHAPED | VERIFIED |
| Drug Info Card with Interactions | specs/spl-drug-info.md | SHAPED | VERIFIED |
| Drug Interaction Checker | specs/spl-interaction-checker.md | SHAPED | VERIFIED |

## Dependencies

- SPL Browser is foundational (client methods, models, caching)
- Drug Info Card depends on Browser (XML parsing, Section 7 extraction)
- Interaction Checker depends on both (cross-referencing logic)

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
| cycle-1 | SPL Browser + Drug Info Card (specs, plans, TDD) | PLANNED | Overnight away session |

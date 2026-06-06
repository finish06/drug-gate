# M12 — Drug Market Timeline

**Goal:** A discovery view of *what drugs entered the market when* — newest-first,
grouped by period and marketing category — so users can see what types of drugs are
coming to market and when.

**Status:** IN_PROGRESS
**Target Maturity:** beta
**Appetite:** ~1 week
**Started:** 2026-06-06

## Feature

| Feature | Spec | Current Position | Target |
|---------|------|-----------------|--------|
| Drug Market Timeline | specs/drug-market-timeline.md | SHAPED → (spec in progress) | VERIFIED |

## Spike Findings (2026-06-06)

- **Source = openFDA** (`/drug/ndc.json`), proxied by cash-drugs. Fields present:
  `marketing_start_date` (YYYYMMDD), `marketing_category` (NDA/ANDA/BLA/OTC MONOGRAPH/
  NDA AUTHORIZED GENERIC/…), `application_number`, brand/generic names.
- openFDA supports **date-range `search` + `count` aggregation server-side** → a
  timeline can be a query+transform layer; **no SPL-style indexer required**.
- **cash-drugs `fda-ndc` is lookup-only** (search params: name/ndc/class — no date or
  `count`); drug-gate currently drops the marketing fields. → **central decision:**
  (A) drug-gate → openFDA directly (cached + circuit-broken) vs (B) extend cash-drugs.
- "New to market" is noise-dominated: 2025 = 6,207 OTC monograph + 3,964 ANDA vs only
  398 NDA + 214 BLA → **category filtering is essential**; default finished/prescription.

## Success Criteria (refined by spec)

- [ ] `GET /v1/drugs/market-timeline` returns counts by period + `by_category`, date-range filtered
- [ ] Category + quality filtering (NDA/BLA vs OTC/ANDA; finished/prescription)
- [ ] Upstream decision (A vs B) implemented with caching + circuit breaking
- [ ] 80%+ test coverage on new code

## Open Questions (for the spec)

- A (direct openFDA) vs B (extend cash-drugs) for the data path
- Endpoint shape + grouping (year/month), and a companion "new since" list
- "New to market" threshold (last N months) and default category/quality filters
- Refresh/caching cadence (openFDA NDC updates ~daily)

## Cycles

| Cycle | Features | Status | Notes |
|-------|----------|--------|-------|
| _next_ | Drug Market Timeline | PLANNED | Spec in progress (`/add:spec drug-market-timeline`) |

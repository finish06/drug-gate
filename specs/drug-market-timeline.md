# Spec: Drug Market Timeline

**Slug:** drug-market-timeline
**Milestone:** M12 — Drug Market Timeline
**Status:** Draft
**Maturity:** beta

## 1. Feature Description

A discovery view of **what drugs entered the market when**. Two endpoints over the
FDA NDC `marketing_start_date` + `marketing_category` data: an **aggregate timeline**
(`GET /v1/drugs/market-timeline`) — counts grouped by period (year/month) with a
per-category breakdown — and a **drill-down list** (`GET /v1/drugs/new`) — the actual
newest-to-market drugs. By default both show **novel** launches (NDA / BLA / NDA
authorized generic, finished prescription drugs); callers can widen to generics/OTC
or everything via a `category` filter.

**Data path (decided):** drug-gate **proxies cash-drugs** (consistent with every other
slug). This requires a **new cash-drugs capability** for the `fda-ndc` slug
(date-range filter + `count` aggregation + category/finished filters — see §6).
**drug-gate implementation is blocked until cash-drugs ships that contract.**

The underlying source is openFDA (`/drug/ndc.json`), which supports date-range
`search` + `count` server-side — so **no local index is needed** (unlike the SPL
indexer); the timeline is a proxy + transform + cache layer.

### User Story

As a user of drug-gate, I want to see which drugs are new to the market, which have
been on the market a long time, and which are brand-only vs genericized — grouped by
when they entered the market — so that I understand what types of drugs are coming to
market and when.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | `GET /v1/drugs/market-timeline` returns periods, each with a total `count` and a `by_category` breakdown, over a date range | Must |
| AC-002 | `from`/`to` accept `YYYY` or `YYYY-MM`; `group_by` is `year` (default) or `month` | Must |
| AC-003 | Default scope counts only **novel** categories — `NDA`, `BLA`, `NDA AUTHORIZED GENERIC` — and finished prescription products; OTC monograph / ANDA / bulk / homeopathic are excluded unless requested | Must |
| AC-004 | `category` filter narrows/widens: e.g. `?category=ANDA` (repeatable or comma-separated); `?category=all` returns every category with the full breakdown | Must |
| AC-005 | `GET /v1/drugs/new` returns the actual drugs newest-to-market with `name` (brand), `generic_name`, `category`, `first_marketed`, `ndc`, `application_number` | Must |
| AC-006 | `GET /v1/drugs/new?since=YYYY-MM` filters to drugs first marketed on/after that month; default = the last 12 months when omitted. Same category default/filter as the timeline | Must |
| AC-007 | `/v1/drugs/new` is paginated (existing `PaginatedResponse`) and ordered newest-first by `marketing_start_date` | Must |
| AC-008 | Both endpoints source data by proxying cash-drugs' `fda-ndc` date-range + `count` capability; responses are cached (CacheAside sliding TTL, default ~24h scaled from `CACHE_TTL`) with singleflight coalescing | Must |
| AC-009 | Both endpoints are under `/v1` → require a valid API key and are subject to per-key rate limiting (existing middleware) | Must |
| AC-010 | Invalid `from`/`to`/`group_by`/`category`/`since` → `400` with `ErrorResponse` | Must |
| AC-011 | cash-drugs unreachable/erroring → `502` `ErrorResponse` (existing pattern); circuit breaker applies; stale cache served when available | Must |
| AC-012 | A query matching no drugs → `200` with empty `periods`/`data` (not an error) | Should |
| AC-013 | Upstream `marketing_start_date` (`YYYYMMDD`) is normalized to `YYYY-MM-DD` in responses | Should |

## 3. User Test Cases

### TC-001: Aggregate timeline by year (happy path)
**Precondition:** cash-drugs `fda-ndc` date-range+count capability live; valid API key.
**Steps:** `GET /v1/drugs/market-timeline?from=2022&to=2025&group_by=year` with `X-API-Key`.
**Expected Result:** `200`; periods `2022..2025`, each with `count` + `by_category` (e.g. `{NDA: …, BLA: …}`), novel-only by default.
**Maps to:** TBD

### TC-002: Newest-to-market drill-down, filtered
**Steps:** `GET /v1/drugs/new?since=2025-01&category=NDA`.
**Expected Result:** `200`; paginated list newest-first, each `{name, generic_name, category:"NDA", first_marketed:"2025-…", ndc, application_number}`.
**Maps to:** TBD

### TC-003: Default curates out the noise
**Steps:** `GET /v1/drugs/new?since=2025-01` (no category).
**Expected Result:** Only NDA / BLA / NDA AUTHORIZED GENERIC finished prescription drugs — OTC monograph and ANDA generics excluded.
**Maps to:** TBD

### TC-004: Opt into everything
**Steps:** `GET /v1/drugs/market-timeline?from=2025&to=2025&category=all`.
**Expected Result:** `by_category` shows the full distribution (OTC monograph, ANDA, NDA, BLA, bulk, …).
**Maps to:** TBD

### TC-005: Invalid input
**Steps:** `GET /v1/drugs/market-timeline?from=banana`.
**Expected Result:** `400` `ErrorResponse` (no upstream call).
**Maps to:** TBD

### TC-006: Upstream down
**Precondition:** cash-drugs unreachable.
**Steps:** `GET /v1/drugs/market-timeline?from=2025&to=2025`.
**Expected Result:** `502` `ErrorResponse` (or stale cache if available); circuit breaker trips per existing behavior.
**Maps to:** TBD

### TC-007: No matches
**Steps:** `GET /v1/drugs/new?since=2099-01`.
**Expected Result:** `200` with empty `data` and zero `pagination.total`.
**Maps to:** TBD

## 4. Data Model

```
MarketTimelineResponse
  from        string   "2022" | "2022-01"
  to          string
  group_by    string   "year" | "month"
  total       int      sum of period counts
  periods     []MarketTimelinePeriod

MarketTimelinePeriod
  period      string   "2025" (year) | "2025-06" (month)
  count       int
  by_category map[string]int   { "NDA": 398, "BLA": 214, ... }

NewDrugEntry            (data[] of a PaginatedResponse)
  name            string   brand name
  generic_name    string
  category        string   marketing_category (NDA/BLA/ANDA/OTC MONOGRAPH/…)
  first_marketed  string   YYYY-MM-DD (normalized from upstream YYYYMMDD)
  ndc             string   product_ndc
  application_number string (e.g. NDA021071) when present
```

## 5. API Contract

### GET /v1/drugs/market-timeline
Query: `from` (req), `to` (req), `group_by` = `year`(default)|`month`, `category`
(repeatable/CSV; default novel set; `all` = every category).
→ `200 MarketTimelineResponse` · `400` invalid input · `401` no/invalid key · `429`
rate limited · `502` upstream down.

### GET /v1/drugs/new
Query: `since` = `YYYY-MM` (default: last 12 months), `category` (as above),
`limit`/`offset` (pagination).
→ `200 PaginatedResponse{ data: []NewDrugEntry }` newest-first · `400` · `401` · `429` · `502`.

## 6. Upstream Contract (cash-drugs — prerequisite, owned outside this repo)

drug-gate needs the `fda-ndc` slug (which proxies openFDA `/drug/ndc.json`) to expose:

- **Date-range filter** on `marketing_start_date` (e.g. `marketing_start_date:[YYYYMMDD TO YYYYMMDD]`).
- **Aggregation**: `count=marketing_category` within the active filter (powers `by_category`).
- **Filters**: `marketing_category`, `finished`, `product_type` (for the novel/prescription default).
- **Fields** on list results: `brand_name`, `generic_name`, `marketing_category`,
  `marketing_start_date`, `product_ndc`, `application_number`, `finished`, `product_type`.
- **Pagination** (existing offset style) for the drug list.

All of the above are native openFDA capabilities; the cash-drugs work is to surface
them through the slug. **This spec's implementation is blocked until that lands.**

## 7. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| `from` > `to` | `400` |
| `group_by=month` over a many-year range | Allowed; large `periods[]` (consider a sane max range → `400` beyond, e.g. 20 years) |
| `marketing_start_date` in the future / re-listing | Included as-is (it reflects the listed market date); documented, not corrected |
| Category value unknown | `400` (unknown category) unless `all` |
| openFDA/cash-drugs partial/malformed data | Skip the bad record; never 500 — log server-side |
| Very large result for `/v1/drugs/new` | Pagination caps page size (existing `PaginatedResponse` limits) |
| `since` omitted | Defaults to last 12 months |

## 8. Dependencies

- **cash-drugs `fda-ndc` date-range + count capability** (§6) — **blocking prerequisite**
- `internal/cache` CacheAside (sliding TTL, singleflight) + `client.CircuitBreaker` — existing
- `internal/middleware` APIKeyAuth + RateLimit — existing (under `/v1`)
- `PaginatedResponse` / pagination helpers — existing
- openFDA NDC fields (via cash-drugs): `marketing_start_date`, `marketing_category`,
  `application_number`, `finished`, `product_type`

## 9. Out of Scope

- The cash-drugs-side implementation of §6 (separate repo/service)
- Pre-approval **pipeline** / forecasting (unapproved drugs) — not in NDC data; "new"
  here means *recently entered the market* by `marketing_start_date`
- Drugs@FDA approval-date sourcing (distinct from NDC market date)
- Per-drug lifecycle classification on existing lookups (a possible later feature)
- Any UI (API only)

## 10. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-06-06 | 0.1.0 | calebdunn | Initial spec from /add:spec interview. Market-timeline + newest-to-market drill-down over FDA NDC marketing data; novel-category default; proxies a new cash-drugs date-range/count capability (blocking prerequisite); no local index (openFDA aggregates server-side). |

# Spec: Drug Autocomplete / Typeahead

**Version:** 0.1.0
**Created:** 2026-03-20
**PRD Reference:** docs/prd.md — M7: Operational Hardening
**Status:** Complete

## 1. Overview

A fast prefix-matching autocomplete endpoint for drug names. Reuses the existing Redis-cached drug names from M3 (`GetDrugNames`) and filters in-memory for sub-50ms response times on cached data.

### User Story

As a **frontend developer**, I want **a fast autocomplete endpoint that returns drug name suggestions as the user types**, so that **I can build typeahead search UIs without loading the full 104K drug name list client-side**.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | `GET /v1/drugs/autocomplete?q={prefix}` returns drug names matching the prefix | Must |
| AC-002 | Matching is case-insensitive prefix match on the drug name | Must |
| AC-003 | `q` parameter is required and must be at least 2 characters | Must |
| AC-004 | Returns 400 with error if `q` is missing or less than 2 characters | Must |
| AC-005 | `limit` parameter controls max results (default: 10, max: 50) | Must |
| AC-006 | Results are sorted alphabetically | Must |
| AC-007 | Response shape: `{"data": [{"name": "...", "type": "generic|brand"}]}` | Must |
| AC-008 | Response time < 50ms when drug names are cached in Redis | Should |
| AC-009 | Returns empty `data` array (not error) when no matches found | Must |
| AC-010 | Endpoint is authenticated (requires valid API key like all /v1 routes) | Must |

## 3. User Test Cases

### TC-001: Basic prefix search

**Steps:**
1. Send `GET /v1/drugs/autocomplete?q=met`
2. Check response
**Expected Result:** Returns drug names starting with "met" (e.g., metformin, metoprolol, methotrexate). Max 10 results, sorted alphabetically.
**Maps to:** AC-001, AC-002, AC-005, AC-006

### TC-002: Case insensitivity

**Steps:**
1. Send `GET /v1/drugs/autocomplete?q=MET`
2. Compare with `GET /v1/drugs/autocomplete?q=met`
**Expected Result:** Same results regardless of case
**Maps to:** AC-002

### TC-003: Custom limit

**Steps:**
1. Send `GET /v1/drugs/autocomplete?q=sim&limit=3`
**Expected Result:** Returns at most 3 results
**Maps to:** AC-005

### TC-004: Missing q parameter

**Steps:**
1. Send `GET /v1/drugs/autocomplete`
**Expected Result:** 400 with error message indicating q is required
**Maps to:** AC-003, AC-004

### TC-005: Short q parameter

**Steps:**
1. Send `GET /v1/drugs/autocomplete?q=a`
**Expected Result:** 400 with error message indicating minimum 2 characters
**Maps to:** AC-003, AC-004

### TC-006: No matches

**Steps:**
1. Send `GET /v1/drugs/autocomplete?q=zzzzz`
**Expected Result:** 200 with `{"data": []}`
**Maps to:** AC-009

## 4. Data Model

### Response Shape

Reuses existing `model.DrugNameEntry`:

```json
{
  "data": [
    {"name": "metformin", "type": "generic"},
    {"name": "metoprolol", "type": "generic"},
    {"name": "Metformin HCl", "type": "brand"}
  ]
}
```

**No pagination.** This endpoint intentionally omits the `pagination` wrapper used by list endpoints (`/v1/drugs/names`, `/v1/drugs/classes`). Autocomplete returns a flat `{"data": [...]}` response capped by the `limit` parameter. This is a design decision — autocomplete is a typeahead helper, not a browseable list. Clients should not expect `pagination.total` or `pagination.total_pages` fields.

## 5. API Contract

### GET /v1/drugs/autocomplete

**Description:** Returns drug names matching the given prefix. Fast typeahead endpoint for building search UIs.

**Query Parameters:**

| Param | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| q | string | Yes | — | Prefix to match (min 2 chars) |
| limit | int | No | 10 | Max results to return (max 50) |

**Response (200):**
```json
{
  "data": [
    {"name": "metformin", "type": "generic"},
    {"name": "metoprolol", "type": "generic"}
  ]
}
```

**Response (400):**
```json
{
  "error": "bad_request",
  "message": "q parameter is required and must be at least 2 characters"
}
```

## 6. Implementation Notes

- New file: `internal/handler/autocomplete.go`
- New service method: `DrugDataService.AutocompleteDrugs(ctx, prefix, limit) ([]DrugNameEntry, error)`
- Reuses `GetDrugNames()` — no new upstream calls or cache keys
- Prefix match: `strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix))`
- Sort: `sort.Slice` alphabetically on Name field
- Cap at limit after filtering and sorting

## 7. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Cold cache (first request after Redis eviction) | First request loads from upstream (~2s), subsequent requests sub-50ms |
| Limit exceeds 50 | Clamp to 50 |
| Limit is 0 or negative | Use default (10) |
| Unicode prefix | Match as-is with case folding via strings.ToLower |
| Drug names with special characters | Matched literally |

## 8. Dependencies

- `internal/service/drugdata.go` — `GetDrugNames()` (existing, from M3)
- `internal/model/response.go` — `DrugNameEntry` (existing)
- `cmd/server/main.go` — wire new route

## 9. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-20 | 0.1.0 | calebdunn | Initial spec |

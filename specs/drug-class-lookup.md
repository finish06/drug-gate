# Spec: Drug Class Lookup

**Version:** 0.1.0
**Created:** 2026-03-09
**PRD Reference:** docs/prd.md (M3: Extended Lookups)
**Status:** Complete

## 1. Overview

Look up a drug by name and return its therapeutic/pharmacological class(es). The endpoint queries the existing cash-drugs `fda-ndc` endpoint using the `GENERIC_NAME` search parameter first, falling back to `BRAND_NAME` if no results are found. The response includes the queried name, resolved generic name, all associated brand names (deduplicated, title-cased), and drug classes with their classification type.

### User Story

As a **frontend developer**, I want to **submit a drug name and get back its therapeutic class(es)**, so that **I can categorize and display drug classification information without knowing NDC codes**.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | `GET /v1/drugs/class?name={drug_name}` returns 200 with drug class info for a valid generic name | Must |
| AC-002 | If `GENERIC_NAME` search returns no results, retry with `BRAND_NAME` | Must |
| AC-003 | Response includes `query_name` (the original input) | Must |
| AC-004 | Response includes `generic_name` from upstream data | Must |
| AC-005 | Response includes `brand_names` as a deduplicated array of all brand names found | Must |
| AC-006 | Brand names are deduplicated case-insensitively and normalized to title case | Must |
| AC-007 | Response includes `classes` array with objects containing `name` and `type` fields | Must |
| AC-008 | Class `type` is parsed from the FDA bracket suffix (e.g., `[EPC]`, `[MoA]`, `[PE]`, `[CS]`) | Must |
| AC-009 | Missing `name` query parameter returns 400 with error response | Must |
| AC-010 | Empty `name` query parameter returns 400 with error response | Must |
| AC-011 | Drug name not found via generic or brand name returns 404 | Must |
| AC-012 | Upstream cash-drugs error returns 502 | Must |
| AC-013 | Endpoint is protected by existing auth middleware (API key required) | Must |
| AC-014 | Endpoint is rate-limited via existing rate limit middleware | Must |
| AC-015 | Response includes empty `classes` array when upstream returns no `pharm_class` data | Should |
| AC-016 | Response includes empty `brand_names` array when no brand name is present | Should |

## 3. User Test Cases

### TC-001: Happy path — generic name lookup

**Precondition:** cash-drugs is running, `fda-ndc` endpoint returns data for `GENERIC_NAME=simvastatin`
**Steps:**
1. Send `GET /v1/drugs/class?name=simvastatin` with valid API key
2. Observe response
**Expected Result:** 200 OK with `query_name: "simvastatin"`, `generic_name: "simvastatin"`, `brand_names: ["Zocor", ...]`, `classes` populated with EPC/MoA entries
**Maps to:** AC-001, AC-003, AC-004, AC-005, AC-007

### TC-002: Brand name fallback

**Precondition:** cash-drugs returns no results for `GENERIC_NAME=Lipitor` but returns results for `BRAND_NAME=Lipitor`
**Steps:**
1. Send `GET /v1/drugs/class?name=Lipitor` with valid API key
2. Observe response
**Expected Result:** 200 OK with `query_name: "Lipitor"`, `generic_name: "atorvastatin calcium"`, `brand_names: ["Lipitor"]`, `classes` populated
**Maps to:** AC-002, AC-003, AC-004

### TC-003: Multiple brand names

**Precondition:** cash-drugs returns multiple products for `GENERIC_NAME=warfarin` with brand names "COUMADIN", "Coumadin", "JANTOVEN", "Jantoven"
**Steps:**
1. Send `GET /v1/drugs/class?name=warfarin` with valid API key
2. Observe response
**Expected Result:** 200 OK with `brand_names: ["Coumadin", "Jantoven"]` (deduplicated, title-cased)
**Maps to:** AC-005, AC-006

### TC-004: Missing name parameter

**Steps:**
1. Send `GET /v1/drugs/class` with valid API key (no `name` param)
2. Observe response
**Expected Result:** 400 with `{"error": "validation_error", "message": "name query parameter is required"}`
**Maps to:** AC-009

### TC-005: Drug not found

**Steps:**
1. Send `GET /v1/drugs/class?name=notarealdrug` with valid API key
2. Observe response
**Expected Result:** 404 with `{"error": "not_found", "message": "No drug found for name 'notarealdrug'"}`
**Maps to:** AC-011

### TC-006: No API key

**Steps:**
1. Send `GET /v1/drugs/class?name=simvastatin` without API key
2. Observe response
**Expected Result:** 401 Unauthorized
**Maps to:** AC-013

### TC-007: Partial data — no pharm_class

**Precondition:** cash-drugs returns drug data but `pharm_class` is empty/missing
**Steps:**
1. Send `GET /v1/drugs/class?name=someDrug` with valid API key
2. Observe response
**Expected Result:** 200 OK with `classes: []`
**Maps to:** AC-015

## 4. Data Model

### DrugClassResponse (response)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| query_name | string | Yes | The drug name as submitted by the client |
| generic_name | string | Yes | Resolved generic/active ingredient name from upstream |
| brand_names | []string | Yes | Deduplicated brand names (title case), empty array if none |
| classes | []DrugClass | Yes | Therapeutic classifications, empty array if unknown |

### DrugClass

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | Yes | Class name (e.g., "HMG-CoA Reductase Inhibitor") |
| type | string | Yes | Classification type: "EPC", "MoA", "PE", "CS" |

### Parsing `pharm_class` from upstream

The FDA `pharm_class` field contains strings like:
- `"HMG-CoA Reductase Inhibitor [EPC]"`
- `"Hydroxymethylglutaryl-CoA Reductase Inhibitors [MoA]"`

Parse by splitting on the last `[` to extract `name` and `type`.

### Brand name deduplication

1. Collect all `brand_name` values from upstream results
2. Lowercase each for comparison
3. Keep first occurrence of each unique lowercase value
4. Normalize to title case (e.g., "COUMADIN" → "Coumadin")

## 5. API Contract

### GET /v1/drugs/class?name={drug_name}

**Description:** Look up drug class(es) by drug name. Tries generic name first, then brand name.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| name | string | Yes | Drug name (generic or brand) |

**Response (200):**
```json
{
  "query_name": "simvastatin",
  "generic_name": "simvastatin",
  "brand_names": ["Zocor"],
  "classes": [
    {"name": "HMG-CoA Reductase Inhibitor", "type": "EPC"},
    {"name": "Hydroxymethylglutaryl-CoA Reductase Inhibitors", "type": "MoA"}
  ]
}
```

**Error Responses:**

- `400` — Missing or empty name parameter. Body: `{"error": "validation_error", "message": "name query parameter is required"}`
- `401` — Missing or invalid API key (handled by auth middleware)
- `404` — No drug found. Body: `{"error": "not_found", "message": "No drug found for name 'xyz'"}`
- `429` — Rate limited (handled by rate limit middleware)
- `502` — Upstream error. Body: `{"error": "upstream_error", "message": "Unable to reach drug data service"}`

## 6. Upstream Integration

### cash-drugs endpoint

Uses the existing `fda-ndc` slug with name-based search params:

**Generic name search:**
```
GET /api/cache/fda-ndc?GENERIC_NAME={name}
```

**Brand name fallback:**
```
GET /api/cache/fda-ndc?BRAND_NAME={name}
```

Both return the same response shape:
```json
{
  "data": [
    {
      "product_ndc": "...",
      "brand_name": "Zocor",
      "generic_name": "simvastatin",
      "pharm_class": [
        "HMG-CoA Reductase Inhibitor [EPC]",
        "Hydroxymethylglutaryl-CoA Reductase Inhibitors [MoA]"
      ]
    }
  ]
}
```

**No changes needed to cash-drugs config** — the `fda-ndc` endpoint already supports `GENERIC_NAME` and `BRAND_NAME` search parameters.

## 7. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Name with extra whitespace | Trim before querying |
| Name with mixed case | Pass as-is to upstream (FDA search is case-insensitive) |
| Generic search returns results, skip brand fallback | Return immediately after generic match |
| Both generic and brand return no results | Return 404 |
| Upstream returns multiple products (same generic, different manufacturers) | Aggregate: use first `generic_name`, collect all unique `brand_names`, use first product's `pharm_class` (classes are consistent across products of the same generic) |
| `pharm_class` with unknown bracket type | Parse bracket content as-is for `type` field |
| `pharm_class` with no bracket | Use full string as `name`, set `type` to empty string |
| Cash-drugs unreachable on generic search | Return 502 immediately (don't attempt brand fallback) |
| Cash-drugs returns 404 on generic search | This means no results — proceed to brand fallback |

## 8. Dependencies

- **cash-drugs `fda-ndc` endpoint** — Must support `GENERIC_NAME` and `BRAND_NAME` search params (already configured)
- **M2 auth/rate limit middleware** — Endpoint sits behind existing middleware chain

## 9. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-09 | 0.1.0 | calebdunn | Initial spec from /add:spec interview |

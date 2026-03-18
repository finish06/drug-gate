# Drug Info Card with Interactions

**Version:** 1.0
**Milestone:** M6 — SPL Interactions
**Status:** Draft
**Created:** 2026-03-17

## Overview

Provide a single-drug profile endpoint that returns SPL metadata combined with parsed drug interaction text from Section 7 of the FDA label. Accepts drug name or NDC as input. This powers "drug info card" UI components that show comprehensive drug information including interaction warnings.

## User Story

As a **frontend developer building a drug info card**, I want to **look up a single drug by name or NDC and get its SPL metadata plus interaction warnings in one call**, so that **I can render a complete drug profile without multiple API round-trips**.

## Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | `GET /v1/drugs/info?name={drugname}` returns drug profile with interactions | Must |
| AC-002 | `GET /v1/drugs/info?ndc={ndc}` accepts NDC input, resolves to drug name, then fetches SPL | Must |
| AC-003 | NDC input is normalized using existing NDC normalization logic | Must |
| AC-004 | Response includes: drug name, SPL metadata (title, setid, published_date), and parsed interaction sections | Must |
| AC-005 | When multiple SPLs exist, uses the most recently published one | Must |
| AC-006 | Interaction sections are parsed from SPL XML Section 7 (reuses spl-browser parsing) | Must |
| AC-007 | If no SPL found for the drug, return 200 with null interactions (drug name still returned) | Must |
| AC-008 | If upstream unavailable, return 502 | Must |
| AC-009 | Must provide either `name` or `ndc` query param; returns 400 if neither | Must |
| AC-010 | Parsed results cached in Redis with 60-minute sliding TTL | Must |
| AC-011 | All endpoints require valid API key | Must |
| AC-012 | NDC resolution uses existing NDC lookup to find generic_name, then queries SPL by name | Should |
| AC-013 | Response includes a `source` field indicating which SPL the interactions came from | Should |

## User Test Cases

| ID | Scenario | Maps to |
|----|----------|---------|
| TC-001 | Get drug info for "warfarin" by name → returns profile with 5 interaction subsections | AC-001, AC-004, AC-006 |
| TC-002 | Get drug info by NDC "0071-0155-23" → resolves to atorvastatin, returns SPL interactions | AC-002, AC-003, AC-012 |
| TC-003 | Get drug info for drug with no SPL → returns 200 with drug name, null interactions | AC-007 |
| TC-004 | Request with neither name nor ndc → 400 error | AC-009 |
| TC-005 | Drug with multiple SPLs → uses most recent published_date | AC-005 |
| TC-006 | Request without API key → 401 | AC-011 |

## Data Model

### DrugInfoResponse

```go
type DrugInfoResponse struct {
    DrugName     string              `json:"drug_name"`
    InputType    string              `json:"input_type"`    // "name" or "ndc"
    InputValue   string              `json:"input_value"`   // original query
    SPL          *SPLSource          `json:"spl"`           // null if no SPL found
    Interactions []InteractionSection `json:"interactions"`  // empty if no Section 7
}

type SPLSource struct {
    Title         string `json:"title"`
    SetID         string `json:"setid"`
    PublishedDate string `json:"published_date"`
    SPLVersion    int    `json:"spl_version"`
}
```

## API Contract

### GET /v1/drugs/info

Get drug profile with interaction data.

**Query Parameters:**
| Param | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | One of name/ndc | Drug name to look up |
| ndc | string | One of name/ndc | NDC code (any format — normalized internally) |

**Response 200 (with interactions):**
```json
{
  "drug_name": "warfarin",
  "input_type": "name",
  "input_value": "warfarin",
  "spl": {
    "title": "WARFARIN SODIUM TABLET [REMEDYREPACK INC.]",
    "setid": "3f1c3083-91d1-488c-ad07-b198fda21da6",
    "published_date": "Mar 16, 2026",
    "spl_version": 17
  },
  "interactions": [
    {
      "title": "7 DRUG INTERACTIONS",
      "text": "Concomitant use of drugs that increase bleeding risk..."
    },
    {
      "title": "7.1 General Information",
      "text": "Drugs may interact with warfarin sodium through..."
    }
  ]
}
```

**Response 200 (no SPL found):**
```json
{
  "drug_name": "obscure-drug",
  "input_type": "name",
  "input_value": "obscure-drug",
  "spl": null,
  "interactions": []
}
```

**Response 400:** Neither name nor ndc provided
**Response 401:** Invalid API key
**Response 429:** Rate limited
**Response 502:** Upstream unavailable

## Upstream Integration

**Flow for name input:**
1. Query `spls-by-name?DRUGNAME={name}` → get SPL list
2. Sort by `published_date` descending, pick first (most recent)
3. Query `spl-xml?SETID={setid}` → get raw XML
4. Parse Section 7 → return structured interactions

**Flow for NDC input:**
1. Normalize NDC using existing `ndc.Normalize()`
2. Query existing `fda-ndc?NDC={ndc}` → get `generic_name`
3. Use `generic_name` as drug name, follow name input flow above

## Edge Cases

- NDC that doesn't resolve to any drug → 404 from NDC lookup, propagate
- Drug name with SPLs but none have Section 7 → return SPL source with empty interactions
- NDC and name both provided → prefer NDC (more specific)
- Drug name that is a brand name (e.g., "Lipitor") → works directly with spls-by-name

## Dependencies

- SPL Browser spec (spl-browser.md) — reuses XML parsing and client methods
- Existing NDC normalization (`internal/ndc`)
- Existing drug client (`internal/client`) for NDC resolution
- Auth + rate limiting middleware

## Revision History

| Date | Version | Changes |
|------|---------|---------|
| 2026-03-17 | 1.0 | Initial spec from cycle planning interview |

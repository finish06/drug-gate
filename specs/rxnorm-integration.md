# Spec: RxNorm Integration

**Version:** 0.1.0
**Created:** 2026-03-16
**PRD Reference:** docs/prd.md (M4: Interactions & RxNorm)
**Status:** Complete

## 1. Overview

Expose RxNorm drug data through drug-gate, enabling frontend applications to resolve drug names to canonical identifiers (RxCUI), look up NDC codes, find generic equivalents, and retrieve full drug profiles with related concepts. Data is sourced from cash-drugs' RxNorm proxy endpoints (`rxnorm-approximate-match`, `rxnorm-spelling-suggestions`, `rxnorm-ndcs`, `rxnorm-generic-product`, `rxnorm-all-related`) and cached in Redis with appropriate TTLs.

### User Story

As a **frontend developer**, I want to **search for drugs by name and get back structured RxNorm data (identifiers, generics, NDCs, related concepts)**, so that **I can build drug lookup tools, formulary checkers, and clinical features without understanding RxNorm internals**.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | `GET /v1/drugs/rxnorm/search?name={name}` returns up to 5 approximate match candidates with RxCUI and name | Must |
| AC-002 | Search includes spelling suggestions when no approximate matches are found | Must |
| AC-003 | Search response includes `rxcui`, `name`, and `score` for each candidate | Must |
| AC-004 | `GET /v1/drugs/rxnorm/{rxcui}/ndcs` returns NDC codes for the given RxCUI | Must |
| AC-005 | `GET /v1/drugs/rxnorm/{rxcui}/generics` returns generic product info for the given RxCUI | Must |
| AC-006 | `GET /v1/drugs/rxnorm/{rxcui}/related` returns related concepts grouped by type | Must |
| AC-007 | Related concepts are grouped into: `ingredients`, `brand_names`, `dose_forms`, `clinical_drugs`, `branded_drugs` | Must |
| AC-008 | Each related concept entry includes `rxcui` and `name` | Must |
| AC-009 | `GET /v1/drugs/rxnorm/profile?name={name}` returns a unified drug profile | Must |
| AC-010 | Profile resolves drug name via approximate match (best match), then assembles NDCs, generics, and related concepts | Must |
| AC-011 | Profile response includes `query`, `rxcui`, `name`, `brand_names`, `generic`, `ndcs`, and `related` | Must |
| AC-012 | Missing `name` query parameter on search and profile endpoints returns 400 | Must |
| AC-013 | Drug name not found (no approximate matches) returns 404 | Must |
| AC-014 | Invalid/unknown RxCUI returns 404 on granular endpoints | Must |
| AC-015 | Upstream cash-drugs/RxNorm error returns 502 | Must |
| AC-016 | All endpoints are protected by auth middleware (API key required) | Must |
| AC-017 | All endpoints are rate-limited via existing middleware | Must |
| AC-018 | Search results are cached in Redis with 24h TTL | Should |
| AC-019 | RxCUI-based lookups (NDCs, generics, related) are cached with 7d TTL | Should |
| AC-020 | Profile endpoint caches the assembled result with 24h TTL | Should |
| AC-021 | Cache uses sliding TTL (reset on access), consistent with M3 pattern | Should |
| AC-022 | Spelling suggestions are returned as a string array in the search response when matches are empty | Should |
| AC-023 | Profile endpoint returns empty arrays (not null) for missing sections | Should |

## 3. User Test Cases

### TC-001: Search for a drug by name

**Precondition:** cash-drugs RxNorm endpoints are reachable
**Steps:**
1. Send `GET /v1/drugs/rxnorm/search?name=lipitor` with valid API key
2. Observe response
**Expected Result:** 200 OK with up to 5 candidates, each with `rxcui`, `name`, and `score`. Top result should be atorvastatin/Lipitor.
**Maps to:** TBD

### TC-002: Search with misspelling

**Precondition:** cash-drugs RxNorm endpoints are reachable
**Steps:**
1. Send `GET /v1/drugs/rxnorm/search?name=liiptor` with valid API key
2. Observe response
**Expected Result:** 200 OK with approximate matches (fuzzy matching handles misspelling) or spelling suggestions
**Maps to:** TBD

### TC-003: Search with no results

**Precondition:** cash-drugs RxNorm endpoints are reachable
**Steps:**
1. Send `GET /v1/drugs/rxnorm/search?name=notarealdrug99` with valid API key
2. Observe response
**Expected Result:** 404 with `{"error": "not_found", "message": "No drugs found for name 'notarealdrug99'"}`
**Maps to:** TBD

### TC-004: Get NDCs for an RxCUI

**Precondition:** cash-drugs RxNorm endpoints are reachable
**Steps:**
1. Send `GET /v1/drugs/rxnorm/153165/ndcs` with valid API key (153165 = atorvastatin calcium)
2. Observe response
**Expected Result:** 200 OK with array of NDC strings
**Maps to:** TBD

### TC-005: Get generic equivalent

**Precondition:** cash-drugs RxNorm endpoints are reachable
**Steps:**
1. Send `GET /v1/drugs/rxnorm/153165/generics` with valid API key
2. Observe response
**Expected Result:** 200 OK with generic product info (rxcui, name for the generic ingredient)
**Maps to:** TBD

### TC-006: Get related concepts

**Precondition:** cash-drugs RxNorm endpoints are reachable
**Steps:**
1. Send `GET /v1/drugs/rxnorm/153165/related` with valid API key
2. Observe response
**Expected Result:** 200 OK with grouped related concepts: `ingredients`, `brand_names`, `dose_forms`, `clinical_drugs`, `branded_drugs`
**Maps to:** TBD

### TC-007: Full drug profile

**Precondition:** cash-drugs RxNorm endpoints are reachable
**Steps:**
1. Send `GET /v1/drugs/rxnorm/profile?name=lipitor` with valid API key
2. Observe response
**Expected Result:** 200 OK with complete profile: `query`, `rxcui`, `name`, `brand_names`, `generic`, `ndcs`, `related`
**Maps to:** TBD

### TC-008: Profile for unknown drug

**Precondition:** cash-drugs RxNorm endpoints are reachable
**Steps:**
1. Send `GET /v1/drugs/rxnorm/profile?name=notarealdrug99` with valid API key
2. Observe response
**Expected Result:** 404 with `{"error": "not_found", "message": "No drugs found for name 'notarealdrug99'"}`
**Maps to:** TBD

### TC-009: Missing name parameter

**Steps:**
1. Send `GET /v1/drugs/rxnorm/search` with valid API key (no `name` param)
2. Observe response
**Expected Result:** 400 with `{"error": "validation_error", "message": "name query parameter is required"}`
**Maps to:** TBD

### TC-010: Unknown RxCUI

**Steps:**
1. Send `GET /v1/drugs/rxnorm/999999999/ndcs` with valid API key
2. Observe response
**Expected Result:** 404 with `{"error": "not_found", "message": "No data found for RxCUI '999999999'"}`
**Maps to:** TBD

### TC-011: No API key

**Steps:**
1. Send `GET /v1/drugs/rxnorm/search?name=lipitor` without API key
2. Observe response
**Expected Result:** 401 Unauthorized
**Maps to:** TBD

## 4. Data Model

### RxNormSearchResult (search response)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| query | string | Yes | The drug name as submitted by the client |
| candidates | []RxNormCandidate | Yes | Up to 5 approximate match results, empty array if none |
| suggestions | []string | Yes | Spelling suggestions, populated when candidates is empty |

### RxNormCandidate

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| rxcui | string | Yes | RxNorm concept unique identifier |
| name | string | Yes | Drug name |
| score | int | Yes | Match confidence score from RxNorm (higher = better) |

### RxNormNDCResponse (NDCs response)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| rxcui | string | Yes | The queried RxCUI |
| ndcs | []string | Yes | List of NDC codes, empty array if none |

### RxNormGenericResponse (generics response)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| rxcui | string | Yes | The queried RxCUI |
| generics | []RxNormConcept | Yes | Generic product concepts, empty array if none |

### RxNormRelatedResponse (related concepts response)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| rxcui | string | Yes | The queried RxCUI |
| ingredients | []RxNormConcept | Yes | Ingredient concepts (type IN) |
| brand_names | []RxNormConcept | Yes | Brand name concepts (type BN) |
| dose_forms | []RxNormConcept | Yes | Dose form concepts (type DF) |
| clinical_drugs | []RxNormConcept | Yes | Semantic clinical drug concepts (type SCD) |
| branded_drugs | []RxNormConcept | Yes | Semantic branded drug concepts (type SBD) |

### RxNormConcept

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| rxcui | string | Yes | RxNorm concept unique identifier |
| name | string | Yes | Concept name |

### RxNormProfile (profile response)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| query | string | Yes | The drug name as submitted by the client |
| rxcui | string | Yes | Resolved RxCUI (best approximate match) |
| name | string | Yes | Canonical drug name from RxNorm |
| brand_names | []string | Yes | Brand name strings, empty array if none |
| generic | RxNormConcept | No | Generic equivalent (null if not found) |
| ndcs | []string | Yes | NDC codes, empty array if none |
| related | RxNormRelatedResponse | Yes | Grouped related concepts |

## 5. API Contract

### GET /v1/drugs/rxnorm/search?name={name}

**Description:** Search for drugs by name using RxNorm approximate matching. Returns up to 5 candidates ranked by score. Includes spelling suggestions when no matches are found.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| name | string | Yes | Drug name to search for |

**Response (200):**
```json
{
  "query": "lipitor",
  "candidates": [
    {"rxcui": "153165", "name": "atorvastatin calcium", "score": 100},
    {"rxcui": "83367", "name": "atorvastatin", "score": 75}
  ],
  "suggestions": []
}
```

**Response (200 — no matches, with suggestions):**
```json
{
  "query": "liiptor",
  "candidates": [],
  "suggestions": ["lipitor", "lisinopril"]
}
```

### GET /v1/drugs/rxnorm/{rxcui}/ndcs

**Description:** Get NDC codes associated with an RxCUI.

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| rxcui | string | Yes | RxNorm concept unique identifier |

**Response (200):**
```json
{
  "rxcui": "153165",
  "ndcs": ["0071-0155-23", "0071-0156-23", "0071-0157-23"]
}
```

### GET /v1/drugs/rxnorm/{rxcui}/generics

**Description:** Get generic product information for an RxCUI.

**Response (200):**
```json
{
  "rxcui": "153165",
  "generics": [
    {"rxcui": "83367", "name": "atorvastatin"}
  ]
}
```

### GET /v1/drugs/rxnorm/{rxcui}/related

**Description:** Get all related concepts for an RxCUI, grouped by type.

**Response (200):**
```json
{
  "rxcui": "153165",
  "ingredients": [{"rxcui": "83367", "name": "atorvastatin"}],
  "brand_names": [{"rxcui": "153165", "name": "Lipitor"}],
  "dose_forms": [{"rxcui": "317541", "name": "Oral Tablet"}],
  "clinical_drugs": [{"rxcui": "259255", "name": "atorvastatin 10 MG Oral Tablet"}],
  "branded_drugs": [{"rxcui": "617310", "name": "Lipitor 10 MG Oral Tablet"}]
}
```

### GET /v1/drugs/rxnorm/profile?name={name}

**Description:** Unified drug profile. Resolves name via approximate match (best match), then assembles NDCs, generics, and related concepts in one response.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| name | string | Yes | Drug name (generic or brand) |

**Response (200):**
```json
{
  "query": "lipitor",
  "rxcui": "153165",
  "name": "atorvastatin calcium",
  "brand_names": ["Lipitor"],
  "generic": {"rxcui": "83367", "name": "atorvastatin"},
  "ndcs": ["0071-0155-23", "0071-0156-23"],
  "related": {
    "rxcui": "153165",
    "ingredients": [{"rxcui": "83367", "name": "atorvastatin"}],
    "brand_names": [{"rxcui": "153165", "name": "Lipitor"}],
    "dose_forms": [{"rxcui": "317541", "name": "Oral Tablet"}],
    "clinical_drugs": [],
    "branded_drugs": []
  }
}
```

**Error Responses (all endpoints):**

- `400` — Missing required parameter. Body: `{"error": "validation_error", "message": "..."}`
- `401` — Missing or invalid API key (handled by auth middleware)
- `404` — Drug/RxCUI not found. Body: `{"error": "not_found", "message": "..."}`
- `429` — Rate limited (handled by rate limit middleware)
- `502` — Upstream error. Body: `{"error": "upstream_error", "message": "Unable to reach drug data service"}`

## 6. Upstream Integration

### cash-drugs RxNorm endpoints

**Approximate match (search):**
```
GET /api/cache/rxnorm-approximate-match?DRUG_NAME={name}
```
Returns: `{"data": {"approximateGroup": {"candidate": [{"rxcui": "...", "name": "...", "score": "..."}]}}}`

**Spelling suggestions:**
```
GET /api/cache/rxnorm-spelling-suggestions?DRUG_NAME={name}
```
Returns: `{"data": {"suggestionGroup": {"suggestionList": {"suggestion": ["...", "..."]}}}}`

**NDCs for RxCUI:**
```
GET /api/cache/rxnorm-ndcs?RXCUI={rxcui}
```
Returns: `{"data": {"ndcGroup": {"ndcList": {"ndc": ["...", "..."]}}}}`

**Generic product:**
```
GET /api/cache/rxnorm-generic-product?RXCUI={rxcui}
```
Returns: `{"data": {"minConceptGroup": {"minConcept": [{"rxcui": "...", "name": "..."}]}}}`

**All related concepts:**
```
GET /api/cache/rxnorm-all-related?RXCUI={rxcui}
```
Returns: `{"data": {"allRelatedGroup": {"conceptGroup": [{"tty": "IN", "conceptProperties": [{"rxcui": "...", "name": "..."}]}, ...]}}}`

### RxNorm concept type mapping

| RxNorm TTY | Grouped as | Description |
|------------|-----------|-------------|
| IN | `ingredients` | Ingredient |
| BN | `brand_names` | Brand Name |
| DF | `dose_forms` | Dose Form |
| SCD | `clinical_drugs` | Semantic Clinical Drug |
| SBD | `branded_drugs` | Semantic Branded Drug |

All other TTY values are excluded from the grouped response.

## 7. Caching Strategy

### Redis Key Schema

| Data | Redis Key | TTL | Rationale |
|------|-----------|-----|-----------|
| Approximate match results | `cache:rxnorm:search:{name}` | 24h sliding | Names don't change often, but new drugs are added monthly |
| Spelling suggestions | `cache:rxnorm:suggest:{name}` | 24h sliding | Same rationale as search |
| NDCs for RxCUI | `cache:rxnorm:ndcs:{rxcui}` | 7d sliding | NDC assignments are stable |
| Generic product | `cache:rxnorm:generic:{rxcui}` | 7d sliding | Generic relationships are stable |
| Related concepts | `cache:rxnorm:related:{rxcui}` | 7d sliding | Concept relationships are stable |
| Assembled profile | `cache:rxnorm:profile:{name}` | 24h sliding | Composite — cache the assembled result |

All keys use lowercase normalized names. Sliding TTL resets on each access (consistent with M3).

## 8. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Drug name with extra whitespace | Trim before querying |
| Drug name with mixed case | Lowercase for cache key, pass as-is to upstream (RxNorm is case-insensitive) |
| RxCUI with leading zeros | Pass as-is to upstream |
| Approximate match returns 0 candidates | Fetch spelling suggestions, return 404 if suggestions also empty |
| Approximate match returns > 5 candidates | Return only top 5 by score |
| Generic product returns empty | Profile returns `generic: null` |
| Related concepts has no entries for a TTY group | Return empty array for that group |
| Upstream returns partial data (e.g., NDCs work but related fails) | Return 502 — no partial results |
| Redis unavailable on cache write | Serve response anyway, log warning |
| Redis unavailable on cache read | Fetch from cash-drugs directly, log warning |

## 9. Dependencies

- **cash-drugs RxNorm endpoints** — `rxnorm-approximate-match`, `rxnorm-spelling-suggestions`, `rxnorm-ndcs`, `rxnorm-generic-product`, `rxnorm-all-related` (all already configured)
- **Redis** — lazy cache storage with sliding TTL
- **M2 auth/rate limit middleware** — all endpoints sit behind existing middleware chain
- **Profile endpoint depends on all granular endpoints** — orchestrates search → NDCs + generics + related

## 10. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-16 | 0.1.0 | calebdunn | Initial spec from /add:spec interview |

# Spec: Drug Data Listings

**Version:** 0.1.0
**Created:** 2026-03-09
**PRD Reference:** docs/prd.md (M3: Extended Lookups)
**Status:** Approved

## 1. Overview

Three endpoints that serve paginated, filterable lists of drug names, drug classes, and drugs within a class. Data is sourced from cash-drugs bulk endpoints (`drugnames`, `drugclasses`, `fda-ndc`) and cached in Redis on first request with a 60-minute sliding TTL.

### User Story

As a **frontend developer**, I want to **browse and filter drug names, drug classes, and see which drugs belong to a class**, so that **I can build tools and features using real FDA/DailyMed drug data without querying upstream services directly**.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | `GET /v1/drugs/names` returns paginated list of drug names | Must |
| AC-002 | `names` endpoint supports `q` query param for case-insensitive substring filter | Must |
| AC-003 | `names` response includes `name` and `type` ("generic" or "brand") per entry | Must |
| AC-004 | `names` default pagination: `page=1`, `limit=50`, max `limit=100` | Must |
| AC-005 | `GET /v1/drugs/classes` returns paginated list of drug classes | Must |
| AC-006 | `classes` endpoint supports `type` query param to filter by class type (default: `epc`) | Must |
| AC-007 | `classes` response includes `name` and `type` per entry | Must |
| AC-008 | `classes` default pagination: `page=1`, `limit=50`, max `limit=100` | Must |
| AC-009 | `GET /v1/drugs/classes/drugs` returns drugs belonging to a given class | Must |
| AC-010 | `classes/drugs` requires `class` query parameter; returns 400 if missing | Must |
| AC-011 | `classes/drugs` response includes `generic_name` and `brand_name` per entry | Must |
| AC-012 | `classes/drugs` default pagination: `page=1`, `limit=100`, max `limit=500` | Must |
| AC-013 | All three endpoints are protected by auth middleware (API key required) | Must |
| AC-014 | All three endpoints are rate-limited via existing middleware | Must |
| AC-015 | Data is lazy-loaded from cash-drugs on first request and cached in Redis | Must |
| AC-016 | Redis cache uses 60-minute sliding TTL (reset on each access) | Must |
| AC-017 | All responses include pagination metadata: `page`, `limit`, `total`, `total_pages` | Must |
| AC-018 | `limit` values above max are clamped to max (not rejected) | Should |
| AC-019 | `page` values beyond total pages return empty `data` array with correct metadata | Should |
| AC-020 | Upstream cash-drugs error returns 502 | Must |
| AC-021 | `names` endpoint supports `type` query param to filter by name type: `generic`, `brand`, or `all` (default: `all`) | Should |
| AC-022 | `classes/drugs` with unknown class returns empty `data` array (not 404) | Should |

## 3. User Test Cases

### TC-001: Browse drug names

**Precondition:** cash-drugs `drugnames` endpoint returns data
**Steps:**
1. Send `GET /v1/drugs/names?page=1&limit=10` with valid API key
2. Observe response
**Expected Result:** 200 OK with 10 drug name entries, pagination metadata showing total ~104K
**Maps to:** AC-001, AC-003, AC-004, AC-017

### TC-002: Search drug names

**Precondition:** cash-drugs `drugnames` endpoint returns data
**Steps:**
1. Send `GET /v1/drugs/names?q=simva` with valid API key
2. Observe response
**Expected Result:** 200 OK with entries where `name` contains "simva" (case-insensitive), like "simvastatin"
**Maps to:** AC-002, AC-003

### TC-003: Filter drug names by type

**Precondition:** cash-drugs `drugnames` endpoint returns data
**Steps:**
1. Send `GET /v1/drugs/names?type=generic&q=simva` with valid API key
2. Observe response
**Expected Result:** 200 OK with only generic name entries matching "simva"
**Maps to:** AC-021

### TC-004: Browse drug classes (default EPC)

**Precondition:** cash-drugs `drugclasses` endpoint returns data
**Steps:**
1. Send `GET /v1/drugs/classes` with valid API key
2. Observe response
**Expected Result:** 200 OK with EPC-type classes only (default filter), paginated
**Maps to:** AC-005, AC-006, AC-007, AC-008

### TC-005: Browse drug classes by MoA type

**Precondition:** cash-drugs `drugclasses` endpoint returns data
**Steps:**
1. Send `GET /v1/drugs/classes?type=moa` with valid API key
2. Observe response
**Expected Result:** 200 OK with MoA-type classes only
**Maps to:** AC-006

### TC-006: Get drugs in a class

**Precondition:** cash-drugs `fda-ndc` endpoint supports `PHARM_CLASS` search
**Steps:**
1. Send `GET /v1/drugs/classes/drugs?class=HMG-CoA+Reductase+Inhibitor` with valid API key
2. Observe response
**Expected Result:** 200 OK with drugs in that class (e.g., simvastatin, atorvastatin), each with `generic_name` and `brand_name`
**Maps to:** AC-009, AC-011, AC-012

### TC-007: Missing class parameter

**Steps:**
1. Send `GET /v1/drugs/classes/drugs` with valid API key (no `class` param)
2. Observe response
**Expected Result:** 400 with `{"error": "validation_error", "message": "class query parameter is required"}`
**Maps to:** AC-010

### TC-008: Lazy-load caching

**Precondition:** Redis cache is empty for drugnames
**Steps:**
1. Send `GET /v1/drugs/names?page=1&limit=10` with valid API key
2. Observe that data is fetched from cash-drugs and cached
3. Send same request again
4. Observe that data is served from Redis cache (no upstream call)
**Expected Result:** First request triggers upstream fetch + cache write; second request served from cache
**Maps to:** AC-015

### TC-009: No API key

**Steps:**
1. Send `GET /v1/drugs/names` without API key
2. Observe response
**Expected Result:** 401 Unauthorized
**Maps to:** AC-013

### TC-010: Limit clamping

**Steps:**
1. Send `GET /v1/drugs/names?limit=500` with valid API key
2. Observe response
**Expected Result:** 200 OK with `limit: 100` in pagination metadata (clamped from 500)
**Maps to:** AC-018

## 4. Data Model

### DrugNameEntry

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | Yes | Drug name (e.g., "simvastatin", "Zocor") |
| type | string | Yes | "generic" or "brand" |

### DrugClassEntry

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | Yes | Class name (e.g., "HMG-CoA Reductase Inhibitor") |
| type | string | Yes | Classification type: "EPC", "MoA", "PE", "CS" |

### DrugInClassEntry

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| generic_name | string | Yes | Generic/active ingredient name |
| brand_name | string | Yes | Brand name (may be empty string if not available) |

### PaginatedResponse (wrapper for all three endpoints)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| data | []T | Yes | Array of entries for this page |
| pagination | Pagination | Yes | Pagination metadata |

### Pagination

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| page | int | Yes | Current page number (1-based) |
| limit | int | Yes | Items per page (after clamping) |
| total | int | Yes | Total matching entries |
| total_pages | int | Yes | Ceiling of total / limit |

## 5. API Contract

### GET /v1/drugs/names

**Description:** Paginated list of drug names with optional search and type filter.

**Query Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| q | string | No | — | Case-insensitive substring filter on name |
| type | string | No | all | Filter by name type: `generic`, `brand`, `all` |
| page | int | No | 1 | Page number (1-based) |
| limit | int | No | 50 | Items per page (max 100) |

**Response (200):**
```json
{
  "data": [
    {"name": "simvastatin", "type": "generic"},
    {"name": "Zocor", "type": "brand"}
  ],
  "pagination": {
    "page": 1,
    "limit": 50,
    "total": 104448,
    "total_pages": 2089
  }
}
```

**Mapping from upstream:** cash-drugs `drugnames` returns `{"name_type": "G", "drug_name": "simvastatin"}`. Map `name_type` "G" → "generic", "B" → "brand". Map `drug_name` → `name`.

### GET /v1/drugs/classes

**Description:** Paginated list of drug classes filtered by classification type.

**Query Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| type | string | No | epc | Classification type filter: `epc`, `moa`, `pe`, `cs`, `all` |
| page | int | No | 1 | Page number (1-based) |
| limit | int | No | 50 | Items per page (max 100) |

**Response (200):**
```json
{
  "data": [
    {"name": "HMG-CoA Reductase Inhibitor", "type": "EPC"},
    {"name": "Angiotensin 2 Receptor Blocker", "type": "EPC"}
  ],
  "pagination": {
    "page": 1,
    "limit": 50,
    "total": 312,
    "total_pages": 7
  }
}
```

**Mapping from upstream:** cash-drugs `drugclasses` returns class entries. Parse class type from bracket suffix if present (e.g., `[EPC]`). If upstream format differs, adapt mapping accordingly.

### GET /v1/drugs/classes/drugs

**Description:** List drugs belonging to a specific pharmacological class.

**Query Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| class | string | Yes | — | Pharmacological class name (e.g., "HMG-CoA Reductase Inhibitor") |
| page | int | No | 1 | Page number (1-based) |
| limit | int | No | 100 | Items per page (max 500) |

**Response (200):**
```json
{
  "data": [
    {"generic_name": "simvastatin", "brand_name": "Zocor"},
    {"generic_name": "atorvastatin calcium", "brand_name": "Lipitor"}
  ],
  "pagination": {
    "page": 1,
    "limit": 100,
    "total": 8,
    "total_pages": 1
  }
}
```

**Error Responses (all endpoints):**

- `400` — Missing required parameter (class for `/classes/drugs`). Body: `{"error": "validation_error", "message": "..."}`
- `401` — Missing or invalid API key (handled by auth middleware)
- `429` — Rate limited (handled by rate limit middleware)
- `502` — Upstream error. Body: `{"error": "upstream_error", "message": "Unable to reach drug data service"}`

## 6. Upstream Integration

### Drug names — cash-drugs `drugnames` slug

```
GET /api/cache/drugnames
```

Returns full dataset (~104K entries, ~7MB). Response:
```json
{
  "data": [
    {"name_type": "G", "drug_name": "simvastatin"},
    {"name_type": "B", "drug_name": "Zocor"}
  ]
}
```

### Drug classes — cash-drugs `drugclasses` slug

```
GET /api/cache/drugclasses
```

Returns full dataset (~1,216 entries). Response shape TBD — likely same `data`/`meta` wrapper.

### Drugs by class — cash-drugs `fda-ndc` slug

```
GET /api/cache/fda-ndc?PHARM_CLASS={class_name}
```

Returns FDA NDC products matching the pharmacological class. Response:
```json
{
  "data": [
    {
      "generic_name": "simvastatin",
      "brand_name": "Zocor",
      "pharm_class": ["HMG-CoA Reductase Inhibitor [EPC]", "..."]
    }
  ]
}
```

## 7. Caching Strategy

### Lazy-Load on First Request

Data is NOT preloaded at startup. When a frontend request arrives and the data is not in Redis:

1. Fetch full dataset from cash-drugs
2. Store in Redis
3. Set 60-minute TTL
4. Serve the response from the freshly cached data

### 60-Minute Sliding TTL

- Each time cached data is read from Redis, reset the TTL to 60 minutes
- If no requests arrive for 60 minutes, the cache expires and data is evicted
- Next request triggers a fresh load from cash-drugs

### Redis Key Schema

| Data | Redis Key | Type | TTL |
|------|-----------|------|-----|
| Drug names | `cache:drugnames` | String (JSON blob) | 60 min sliding |
| Drug classes | `cache:drugclasses` | String (JSON blob) | 60 min sliding |
| Drugs by class | `cache:drugsbyclass:{class_name}` | String (JSON blob) | 60 min sliding |

### Filtering and Pagination

All filtering (substring search, type filter) and pagination is performed in-memory after loading from Redis cache. The full dataset is stored as-is; drug-gate slices it at query time.

### Cache Miss Flow

```
Frontend → drug-gate → Redis MISS → cash-drugs → Redis SET (60m TTL) → response
```

### Cache Hit Flow

```
Frontend → drug-gate → Redis HIT → EXPIRE reset (60m) → response
```

## 8. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| First request for drug names (cold cache) | Fetch from cash-drugs, cache, respond (may be slower) |
| `q` filter matches nothing | Return empty `data` array with `total: 0` |
| `type` filter with unknown value | Return empty `data` array (treat as no matches) |
| `page` exceeds total pages | Return empty `data` array with correct pagination metadata |
| `limit=0` | Treat as default (50 or 100 depending on endpoint) |
| Negative `page` or `limit` | Treat as default |
| Cash-drugs returns empty dataset | Cache empty result, return empty `data` array |
| Redis unavailable on cache write | Serve response anyway, log warning (don't fail the request) |
| Redis unavailable on cache read | Fetch from cash-drugs directly, log warning |
| Concurrent first requests for same data | Both fetch from cash-drugs; last write wins (acceptable) |
| `class` param with special characters | URL-decode and pass to cash-drugs as-is |

## 9. Dependencies

- **cash-drugs endpoints** — `drugnames`, `drugclasses`, `fda-ndc` with `PHARM_CLASS` search
- **Redis** — lazy cache storage with sliding TTL
- **M2 auth/rate limit middleware** — all endpoints sit behind existing middleware chain
- **M3 drug-class-lookup spec** — shares upstream integration patterns

## 10. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-09 | 0.1.0 | calebdunn | Initial spec from M3 planning session |

# SPL Document Browser

**Version:** 1.0
**Milestone:** M6 — SPL Interactions
**Status:** Draft
**Created:** 2026-03-17

## Overview

Expose Structured Product Label (SPL) metadata from DailyMed via cash-drugs. Frontend applications can search SPLs by drug name, retrieve metadata (title, set ID, published date, version), and access parsed interaction sections from SPL XML documents. This is the foundational layer for drug info cards and the interaction checker.

## User Story

As a **frontend developer building a drug information tool**, I want to **search for SPL documents by drug name and retrieve their metadata and interaction data**, so that **I can display FDA label information and drug interaction warnings in my application**.

## Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | `GET /v1/drugs/spls?name={drugname}` returns paginated SPL metadata entries | Must |
| AC-002 | Each SPL entry includes: title, setid, published_date, spl_version | Must |
| AC-003 | `GET /v1/drugs/spls/{setid}` returns SPL detail with parsed Section 7 (Drug Interactions) | Must |
| AC-004 | Section 7 is extracted from SPL XML via cash-drugs `spl-xml` endpoint | Must |
| AC-005 | Section 7 content is returned as structured subsections (title + text pairs) | Must |
| AC-006 | Parsed interaction data is cached in Redis with 60-minute sliding TTL | Must |
| AC-007 | If upstream returns no SPLs for a drug name, return 200 with empty data array | Must |
| AC-008 | If upstream is unavailable, return 502 with clear error message | Must |
| AC-009 | If setid is not found, return 404 | Must |
| AC-010 | All endpoints require valid API key (X-API-Key header) | Must |
| AC-011 | All endpoints subject to per-key rate limiting | Must |
| AC-012 | Pagination supports `page` and `limit` query params (default page=1, limit=20, max=100) | Should |
| AC-013 | `name` query parameter is case-insensitive | Should |
| AC-014 | SPL XML parsing handles missing Section 7 gracefully (returns empty interactions array) | Must |

## User Test Cases

| ID | Scenario | Maps to |
|----|----------|---------|
| TC-001 | Search SPLs for "lipitor" → returns 4 entries with Pfizer, Viatris labels | AC-001, AC-002 |
| TC-002 | Search SPLs for "metformin" → returns 544 entries, paginated | AC-001, AC-012 |
| TC-003 | Get SPL detail for Pfizer Lipitor setid → returns title + Section 7 subsections | AC-003, AC-004, AC-005 |
| TC-004 | Search SPLs for nonexistent drug → returns 200 with empty data | AC-007 |
| TC-005 | Get SPL detail for nonexistent setid → returns 404 | AC-009 |
| TC-006 | Request without API key → 401 | AC-010 |
| TC-007 | Second request for same drug within 60 min → served from Redis cache | AC-006 |

## Data Model

### SPLEntry (list endpoint response item)

```go
type SPLEntry struct {
    Title         string `json:"title"`
    SetID         string `json:"setid"`
    PublishedDate string `json:"published_date"`
    SPLVersion    int    `json:"spl_version"`
}
```

### SPLDetail (detail endpoint response)

```go
type SPLDetail struct {
    Title         string              `json:"title"`
    SetID         string              `json:"setid"`
    PublishedDate string              `json:"published_date"`
    SPLVersion    int                 `json:"spl_version"`
    Interactions  []InteractionSection `json:"interactions"`
}

type InteractionSection struct {
    Title string `json:"title"`   // e.g., "7.2 CYP450 Interactions"
    Text  string `json:"text"`    // Plain text content, XML tags stripped
}
```

## API Contract

### GET /v1/drugs/spls

Search SPL documents by drug name.

**Query Parameters:**
| Param | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| name | string | Yes | — | Drug name to search |
| limit | int | No | 20 | Results per page (max 100) |
| page | int | No | 1 | Page number |

**Response 200:**
```json
{
  "data": [
    {
      "title": "LIPITOR (ATORVASTATIN CALCIUM) TABLET, FILM COATED [PARKE-DAVIS DIV OF PFIZER INC]",
      "setid": "c6e131fe-e7df-4876-83f7-9156fc4e8228",
      "published_date": "May 02, 2024",
      "spl_version": 42
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 4,
    "total_pages": 1
  }
}
```

**Response 401:** Missing or invalid API key
**Response 429:** Rate limit exceeded
**Response 502:** Upstream cash-drugs unavailable

### GET /v1/drugs/spls/{setid}

Get SPL detail with parsed interaction sections.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| setid | string | SPL set ID (UUID format) |

**Response 200:**
```json
{
  "title": "LIPITOR (ATORVASTATIN CALCIUM) TABLET, FILM COATED [PARKE-DAVIS DIV OF PFIZER INC]",
  "setid": "c6e131fe-e7df-4876-83f7-9156fc4e8228",
  "published_date": "May 02, 2024",
  "spl_version": 42,
  "interactions": [
    {
      "title": "7 DRUG INTERACTIONS",
      "text": "Summary of drug interaction information..."
    },
    {
      "title": "7.1 General Information",
      "text": "Drugs may interact with atorvastatin..."
    }
  ]
}
```

**Response 404:** SetID not found
**Response 502:** Upstream unavailable

## Upstream Integration

| Slug | Params | Use |
|------|--------|-----|
| `spls-by-name` | `DRUGNAME={name}` | Search SPLs by drug name |
| `spl-detail` | `SETID={setid}` | Get SPL metadata by set ID |
| `spl-xml` | `SETID={setid}` | Fetch raw XML for Section 7 parsing |

**Response shape from cash-drugs:**
```json
{
  "data": [
    {
      "published_date": "May 02, 2024",
      "setid": "c6e131fe-e7df-4876-83f7-9156fc4e8228",
      "spl_version": 42,
      "title": "LIPITOR (ATORVASTATIN CALCIUM) TABLET, FILM COATED [PARKE-DAVIS DIV OF PFIZER INC]"
    }
  ],
  "meta": {
    "slug": "spls-by-name",
    "results_count": 4,
    "page_count": 1
  }
}
```

**XML parsing strategy:**
1. Fetch raw XML from `spl-xml?SETID={setid}`
2. Find Section 7 by searching for `<title>7 DRUG INTERACTIONS</title>` and subsections `7.x`
3. Extract text content between `<text>` tags, strip XML markup
4. Return as array of `InteractionSection` (title + clean text)
5. Handle missing Section 7 → empty `interactions` array

## Edge Cases

- Drug names with special characters (e.g., hyphens, parentheses) — URL-encode
- SPLs with no Section 7 (some OTC products) — return empty interactions array
- Very large SPL XML (200KB+) — parse and cache only Section 7, discard rest
- Multiple SPLs for same drug from different manufacturers — return all, sorted by published_date desc
- `spls-by-class` endpoint unreliable (timeouts) — not used in this spec; class-based search deferred

## Dependencies

- Cash-drugs running at configured `CASHDRUGS_URL`
- Redis for caching
- Existing auth middleware (API key validation)
- Existing rate limiting middleware
- Existing paginated response wrapper (`model.PaginatedResponse`)

## Revision History

| Date | Version | Changes |
|------|---------|---------|
| 2026-03-17 | 1.0 | Initial spec from cycle planning interview |

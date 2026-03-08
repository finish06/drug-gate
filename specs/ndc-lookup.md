# Spec: NDC Lookup

**Version:** 0.2.0
**Created:** 2026-03-07
**PRD Reference:** docs/prd.md (M1: NDC Lookup)
**Status:** Complete

## 1. Overview

Accept a product NDC (labeler-product, first 2 segments) with a required dash, query the internal cash-drugs API (`fda-ndc` endpoint) with a try-exact-then-fallback strategy, and return drug name, generic name, and therapeutic class(es) in a clean, frontend-friendly JSON shape.

The lookup uses `product_ndc` (labeler-product, first 2 segments). If a full 3-segment NDC is provided (labeler-product-package), the package segment is stripped automatically. Dash is required. Dashless input is rejected.

### User Story

As a **frontend developer**, I want to **submit a product NDC and get back the drug name and therapeutic classes**, so that **I can display drug information without worrying about upstream API details**.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | Accept product NDC in 5-4 format (e.g., `00069-3150`) | Must |
| AC-002 | Accept product NDC in 4-4 format (e.g., `0069-3150`) | Must |
| AC-003 | Accept product NDC in 5-3 format (e.g., `00069-315`) | Must |
| AC-004 | If full 3-segment NDC provided (2 dashes), strip the package segment automatically | Must |
| AC-005 | Dash is required — reject dashless input with 400 | Must |
| AC-006 | Try exact product NDC first against cash-drugs `fda-ndc` endpoint | Must |
| AC-007 | If 4-4 not found, fallback: pad to 5-4 and retry | Must |
| AC-008 | If 5-3 not found, fallback: pad to 5-4 and retry | Must |
| AC-009 | Return drug brand name from upstream response | Must |
| AC-010 | Return generic name from upstream response | Must |
| AC-011 | Return therapeutic class(es) as an array from upstream response | Must |
| AC-012 | Return consistent JSON response shape for all successful lookups | Must |
| AC-013 | Return 400 with error message for invalid NDC format (no dash, non-numeric, wrong segment lengths) | Must |
| AC-014 | Return 404 when NDC is valid but no drug found upstream (including after fallback) | Must |
| AC-015 | Return 502 when cash-drugs is unreachable or returns an error | Must |
| AC-016 | Return 200 with null/empty fields when upstream returns partial data (e.g., name but no class) | Should |
| AC-017 | Include the product NDC used for the successful match in the response | Should |

## 3. User Test Cases

Human-readable test scenarios. Each maps to an automated test.

### TC-001: Happy path — 5-4 product NDC

**Precondition:** cash-drugs is running with `fda-ndc` endpoint available
**Steps:**
1. Send `GET /v1/drugs/ndc/00069-3150`
2. Observe response
**Expected Result:** 200 OK with JSON containing `ndc`, `name`, `generic_name`, and `classes` fields populated
**Screenshot Checkpoint:** N/A (API only)
**Maps to:** TBD

### TC-002: 4-4 product NDC — exact match found

**Precondition:** cash-drugs returns data for `0069-3150`
**Steps:**
1. Send `GET /v1/drugs/ndc/0069-3150`
2. Observe response
**Expected Result:** 200 OK — drug details returned using exact 4-4 match
**Screenshot Checkpoint:** N/A
**Maps to:** TBD

### TC-003: 4-4 product NDC — fallback to 5-4

**Precondition:** cash-drugs returns 404 for `0069-3150` but 200 for `00069-3150`
**Steps:**
1. Send `GET /v1/drugs/ndc/0069-3150`
2. Observe response
**Expected Result:** 200 OK — drug details returned after fallback pad to 5-4 (`00069-3150`)
**Screenshot Checkpoint:** N/A
**Maps to:** TBD

### TC-004: 5-3 product NDC — exact match found

**Precondition:** cash-drugs returns data for `00069-315`
**Steps:**
1. Send `GET /v1/drugs/ndc/00069-315`
2. Observe response
**Expected Result:** 200 OK — drug details returned using exact 5-3 match
**Screenshot Checkpoint:** N/A
**Maps to:** TBD

### TC-005: 5-3 product NDC — fallback to 5-4

**Precondition:** cash-drugs returns 404 for `00069-315` but 200 for `00069-0315`
**Steps:**
1. Send `GET /v1/drugs/ndc/00069-315`
2. Observe response
**Expected Result:** 200 OK — drug details returned after fallback pad to 5-4 (`00069-0315`)
**Screenshot Checkpoint:** N/A
**Maps to:** TBD

### TC-006: Full 3-segment NDC — package segment stripped

**Precondition:** cash-drugs is running with `fda-ndc` endpoint available, `00069-3150` returns data
**Steps:**
1. Send `GET /v1/drugs/ndc/00069-3150-83`
2. Observe response
**Expected Result:** 200 OK — package segment (`83`) stripped, lookup performed with `00069-3150`
**Screenshot Checkpoint:** N/A
**Maps to:** TBD

### TC-007: Dashless input rejected

**Precondition:** N/A
**Steps:**
1. Send `GET /v1/drugs/ndc/000693150`
2. Observe response
**Expected Result:** 400 Bad Request — "NDC must contain a dash separating labeler and product segments"
**Screenshot Checkpoint:** N/A
**Maps to:** TBD

### TC-008: Invalid NDC — non-numeric characters

**Precondition:** N/A
**Steps:**
1. Send `GET /v1/drugs/ndc/ABCDE-1234`
2. Observe response
**Expected Result:** 400 Bad Request with error message indicating invalid NDC format
**Screenshot Checkpoint:** N/A
**Maps to:** TBD

### TC-009: Invalid NDC — wrong segment lengths

**Precondition:** N/A
**Steps:**
1. Send `GET /v1/drugs/ndc/123456-12345`
2. Observe response
**Expected Result:** 400 Bad Request — segment lengths must be 4-4, 5-3, or 5-4
**Screenshot Checkpoint:** N/A
**Maps to:** TBD

### TC-010: Valid NDC but drug not found (including after fallback)

**Precondition:** cash-drugs is running, NDC does not exist in upstream data
**Steps:**
1. Send `GET /v1/drugs/ndc/99999-9999`
2. Observe response
**Expected Result:** 404 Not Found with error message
**Screenshot Checkpoint:** N/A
**Maps to:** TBD

### TC-011: cash-drugs unavailable

**Precondition:** cash-drugs is not running or unreachable
**Steps:**
1. Send `GET /v1/drugs/ndc/00069-3150`
2. Observe response
**Expected Result:** 502 Bad Gateway with error message indicating upstream service unavailable
**Screenshot Checkpoint:** N/A
**Maps to:** TBD

### TC-012: Partial upstream data — missing therapeutic class

**Precondition:** cash-drugs returns drug name but no therapeutic class for the NDC
**Steps:**
1. Send `GET /v1/drugs/ndc/00069-3150`
2. Observe response
**Expected Result:** 200 OK with `classes` field as empty array, other fields populated
**Screenshot Checkpoint:** N/A
**Maps to:** TBD

## 4. Data Model

### ProductNDC (input parsing)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| raw | string | Yes | The product NDC as received from the client |
| labeler | string | Yes | Labeler segment (first part before dash) |
| product | string | Yes | Product segment (second part after dash) |
| format | string | Yes | Detected format: "5-4", "4-4", or "5-3" |

### Validation Rules

Input must:
- Contain at least one dash (1 or 2 dashes accepted; 3+ rejected)
- Have only digits in each segment
- If 3 segments (2 dashes), strip the package segment (last) and use first 2
- The remaining product NDC must match one of the valid segment patterns:

| Format | Labeler Length | Product Length | Example |
|--------|---------------|---------------|---------|
| 5-4 | 5 | 4 | `00069-3150` |
| 4-4 | 4 | 4 | `0069-3150` |
| 5-3 | 5 | 3 | `00069-315` |

### Fallback Normalization Rules

When exact lookup fails, pad shorter segment to reach 5-4 canonical form:

| Input Format | Fallback | How |
|--------------|----------|-----|
| 5-4 | None — already canonical | — |
| 4-4 | 5-4 | Pad labeler with leading zero: `0` + labeler |
| 5-3 | 5-4 | Pad product with leading zero: `0` + product |

### DrugDetail (response)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| ndc | string | Yes | The product NDC that matched (may differ from input if fallback used) |
| name | string | Yes | Brand name of the drug |
| generic_name | string | No | Generic/active ingredient name |
| classes | []string | No | Therapeutic class(es), empty array if unknown |

### Relationships

- DrugDetail is constructed from cash-drugs `fda-ndc` response data
- ProductNDC is an input-only value object used for parsing/validation, not persisted

## 5. API Contract

### GET /v1/drugs/ndc/{ndc}

**Description:** Look up drug details by product NDC (labeler-product). Dash required.

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| ndc | string | Product NDC or full NDC — labeler-product with dash (e.g., `00069-3150`, `0069-3150`, `00069-315`). Full 3-segment NDCs (e.g., `00069-3150-83`) are accepted — the package segment is stripped automatically. |

**Response (200):**
```json
{
  "ndc": "00069-3150",
  "name": "Lipitor",
  "generic_name": "atorvastatin calcium",
  "classes": ["HMG-CoA Reductase Inhibitor"]
}
```

**Error Responses:**

- `400` — Invalid NDC format. Body: `{ "error": "invalid_ndc", "message": "NDC must contain labeler (4-5 digits) and product (3-4 digits) segments separated by a dash. Full NDCs with package segment are also accepted." }`
- `404` — Drug not found for this NDC. Body: `{ "error": "not_found", "message": "No drug found for NDC 00069-3150" }`
- `502` — Upstream cash-drugs error. Body: `{ "error": "upstream_error", "message": "Unable to reach drug data service" }`

## 6. UI Behavior

N/A — API only, no UI component.

## 7. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Dashless input | Return 400 — dash is required |
| NDC with extra whitespace | Strip whitespace before processing |
| Full 3-segment NDC (e.g., `00069-3150-83`) | Strip package segment, lookup with `00069-3150` |
| NDC with 3+ dashes (e.g., `00069-3150-83-99`) | Return 400 — too many segments |
| NDC with 6+ digit labeler | Return 400 — labeler must be 4 or 5 digits |
| NDC with 5+ digit product | Return 400 — product must be 3 or 4 digits |
| Empty NDC path param | Return 400 invalid format |
| cash-drugs returns 404 for the NDC | Try fallback if applicable, then return 404 |
| cash-drugs returns 502 (its own upstream failure) | Return 502 upstream error |
| cash-drugs times out | Return 502 with timeout message |
| Multiple drugs match same product NDC | Return the first match |
| 5-4 exact match — no fallback needed | Return result immediately, skip fallback |
| 4-4 exact match found | Return result immediately, skip fallback to 5-4 |
| 5-3 exact match found | Return result immediately, skip fallback to 5-4 |

## 8. Dependencies

- **cash-drugs `fda-ndc` endpoint** — New slug must be added to cash-drugs config.yaml with an `NDC` query parameter accepting product_ndc format. This is a prerequisite for integration and E2E testing.
- **cash-drugs running** — Required for integration/E2E tests. Unit tests will mock the cash-drugs HTTP client.

## 9. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-07 | 0.1.0 | calebdunn | Initial spec from /add:spec interview |
| 2026-03-07 | 0.2.0 | calebdunn | Switched to product_ndc (2-segment), dash required, try-exact-then-fallback strategy, auto-strip package segment from full 3-segment NDCs |

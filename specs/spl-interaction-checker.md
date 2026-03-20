# Drug Interaction Checker

**Version:** 1.0
**Milestone:** M6 — SPL Interactions
**Status:** Complete
**Created:** 2026-03-17

## Overview

Multi-drug interaction checker that accepts 2 or more drug names or NDC codes, fetches their SPL interaction sections, and cross-references them to identify potential interactions between the submitted drugs. Returns structured warnings with the relevant interaction text from each drug's FDA label.

## User Story

As a **frontend developer building a drug interaction checker**, I want to **submit multiple drug names or NDCs and get back interaction warnings between them**, so that **I can display clinically relevant interaction alerts to users without building cross-referencing logic in the frontend**.

## Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | `POST /v1/drugs/interactions` accepts a JSON body with 2+ drug identifiers | Must |
| AC-002 | Each drug identifier can be a `name` (string) or `ndc` (string) | Must |
| AC-003 | NDC inputs are normalized and resolved to drug names via existing NDC lookup | Must |
| AC-004 | For each drug, fetch the most recent SPL and parse Section 7 | Must |
| AC-005 | Cross-reference: for each drug pair (A, B), check if drug A's Section 7 text mentions drug B (or its class) | Must |
| AC-006 | Return structured interaction results: which drugs interact, the relevant section text, and the source SPL | Must |
| AC-007 | If a drug has no SPL or no Section 7, include it in response with `has_interactions: false` | Must |
| AC-008 | Minimum 2 drugs required; return 400 if fewer | Must |
| AC-009 | Maximum 10 drugs per request; return 400 if exceeded | Must |
| AC-010 | All upstream data cached in Redis (reuses spl-browser caching) | Must |
| AC-011 | Requires valid API key | Must |
| AC-012 | Subject to rate limiting | Must |
| AC-013 | If any upstream call fails, return partial results with error flags per drug | Should |
| AC-014 | Cross-reference matching is case-insensitive | Must |
| AC-015 | Response includes `checked_pairs` count and `found_interactions` count | Should |

## User Test Cases

| ID | Scenario | Maps to |
|----|----------|---------|
| TC-001 | Submit warfarin + aspirin → interaction found (aspirin in warfarin's 7.3 bleeding risk table) | AC-001, AC-005, AC-006 |
| TC-002 | Submit warfarin + fluconazole → interaction found (fluconazole in warfarin's 7.2 CYP450 table) | AC-005, AC-006 |
| TC-003 | Submit metformin + lisinopril → no interaction found (neither mentions the other) | AC-005, AC-006 |
| TC-004 | Submit 1 drug only → 400 error | AC-008 |
| TC-005 | Submit 11 drugs → 400 error | AC-009 |
| TC-006 | Submit drug by NDC + drug by name → both resolved, interactions checked | AC-002, AC-003 |
| TC-007 | Submit drug with no SPL + drug with SPL → partial result, one flagged has_interactions: false | AC-007, AC-013 |

## Data Model

### InteractionCheckRequest

```go
type InteractionCheckRequest struct {
    Drugs []DrugIdentifier `json:"drugs"`
}

type DrugIdentifier struct {
    Name string `json:"name,omitempty"` // drug name
    NDC  string `json:"ndc,omitempty"`  // NDC code (any format)
}
```

### InteractionCheckResponse

```go
type InteractionCheckResponse struct {
    Drugs            []DrugResult       `json:"drugs"`
    Interactions     []InteractionMatch `json:"interactions"`
    CheckedPairs     int                `json:"checked_pairs"`
    FoundInteractions int               `json:"found_interactions"`
}

type DrugResult struct {
    InputName       string     `json:"input_name"`
    InputType       string     `json:"input_type"`       // "name" or "ndc"
    ResolvedName    string     `json:"resolved_name"`     // after NDC resolution
    HasInteractions bool       `json:"has_interactions"`  // true if Section 7 found
    SPLSetID        string     `json:"spl_setid,omitempty"`
    Error           string     `json:"error,omitempty"`   // if resolution/fetch failed
}

type InteractionMatch struct {
    DrugA       string `json:"drug_a"`
    DrugB       string `json:"drug_b"`
    Source      string `json:"source"`       // which drug's label mentions the other
    SectionTitle string `json:"section_title"` // e.g., "7.3 Drugs that Increase Bleeding Risk"
    Text        string `json:"text"`          // relevant excerpt
    SPLSetID    string `json:"spl_setid"`     // source SPL
}
```

## API Contract

### POST /v1/drugs/interactions

Check interactions between multiple drugs.

**Request Body:**
```json
{
  "drugs": [
    {"name": "warfarin"},
    {"name": "aspirin"},
    {"ndc": "0071-0155-23"}
  ]
}
```

**Response 200:**
```json
{
  "drugs": [
    {
      "input_name": "warfarin",
      "input_type": "name",
      "resolved_name": "warfarin",
      "has_interactions": true,
      "spl_setid": "3f1c3083-91d1-488c-ad07-b198fda21da6"
    },
    {
      "input_name": "aspirin",
      "input_type": "name",
      "resolved_name": "aspirin",
      "has_interactions": true,
      "spl_setid": "abc123..."
    },
    {
      "input_name": "0071-0155-23",
      "input_type": "ndc",
      "resolved_name": "atorvastatin",
      "has_interactions": true,
      "spl_setid": "c6e131fe..."
    }
  ],
  "interactions": [
    {
      "drug_a": "warfarin",
      "drug_b": "aspirin",
      "source": "warfarin",
      "section_title": "7.3 Drugs that Increase Bleeding Risk",
      "text": "Antiplatelet Agents: aspirin, cilostazol, clopidogrel...",
      "spl_setid": "3f1c3083-91d1-488c-ad07-b198fda21da6"
    },
    {
      "drug_a": "warfarin",
      "drug_b": "atorvastatin",
      "source": "warfarin",
      "section_title": "7.2 CYP450 Interactions",
      "text": "CYP3A4 inhibitors: ...atorvastatin...",
      "spl_setid": "3f1c3083-91d1-488c-ad07-b198fda21da6"
    }
  ],
  "checked_pairs": 3,
  "found_interactions": 2
}
```

**Response 400:** Fewer than 2 drugs, more than 10 drugs, or invalid input
**Response 401:** Invalid API key
**Response 429:** Rate limited
**Response 502:** All upstream calls failed

## Cross-Reference Algorithm

For each ordered pair (Drug A, Drug B):
1. Get Drug A's parsed Section 7 subsections
2. For each subsection, search the text for Drug B's name (case-insensitive)
3. Also search for Drug B's known class names (from existing drug class data if available)
4. If match found, create an `InteractionMatch` with the source section and relevant text excerpt
5. Repeat for (Drug B, Drug A) — interactions may be documented in either direction

**Text matching strategy:**
- Word-boundary matching: search for `\b{drug_name}\b` in section text
- Also match common suffixes (e.g., "atorvastatin" matches "atorvastatin calcium")
- Case-insensitive
- Return the full subsection text as context (not just the match)

## Edge Cases

- Same drug submitted twice → skip self-pair, no error
- Drug name is a brand name (e.g., "Lipitor") → spls-by-name handles brand names
- Drug with very long Section 7 (warfarin has 6K+ chars) → return full text, frontend truncates
- NDC that doesn't resolve → include in drugs array with error field, skip its pairs
- Interaction documented in only one direction → still reported (source field indicates which label)
- Generic vs brand name mismatch in cross-reference → match both generic and title-extracted brand name

## Dependencies

- SPL Browser spec (spl-browser.md) — client methods, XML parsing, caching
- Drug Info Card spec (spl-drug-info.md) — NDC resolution flow
- Existing NDC normalization and lookup
- Auth + rate limiting middleware

## Background Indexer

To improve response times for the interaction checker:

1. **Source:** Derive popular drug list from existing `drugnames` cache in Redis
2. **Worker:** Background goroutine that iterates through drug names, fetches SPL XML, parses Section 7, and caches results
3. **Schedule:** Run on startup, then periodically (e.g., every 24 hours)
4. **Cache key pattern:** `spl:interactions:{drugname}` → JSON array of InteractionSection
5. **Priority:** Index most commonly queried drugs first (track query frequency in Redis)
6. **Graceful:** Don't block startup; index in background. If cache miss, fetch on-demand (fallback).

## Revision History

| Date | Version | Changes |
|------|---------|---------|
| 2026-03-17 | 1.0 | Initial spec from cycle planning interview |

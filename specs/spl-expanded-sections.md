# Spec: Expanded SPL Sections (4-6)

**Version:** 0.1.0
**Created:** 2026-03-20
**PRD Reference:** docs/prd.md — M8: Cache Architecture + Clinical Data
**Status:** Approved

## 1. Overview

Extend the existing SPL XML parser to extract sections 4 (Contraindications), 5 (Warnings and Precautions), and 6 (Adverse Reactions) alongside the existing Section 7 (Drug Interactions). Returns raw text per section (structured subsections deferred to a future cycle).

### User Story

As a **frontend developer**, I want **drug info cards to include contraindications, warnings, and adverse reactions**, so that **clinical tools can display comprehensive safety information from a single API call**.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | SPL detail endpoint returns Section 4 (Contraindications) when present in XML | Must |
| AC-002 | SPL detail endpoint returns Section 5 (Warnings and Precautions) when present | Must |
| AC-003 | SPL detail endpoint returns Section 6 (Adverse Reactions) when present | Must |
| AC-004 | Missing sections return empty arrays (not errors) | Must |
| AC-005 | Subsections (e.g., 5.1, 5.2) are captured as separate entries | Must |
| AC-006 | SPLDetail model has new fields: Contraindications, Warnings, AdverseReactions | Must |
| AC-007 | Drug info card response includes new sections | Must |
| AC-008 | Existing Section 7 parsing unchanged (backward compatible) | Must |
| AC-009 | Tested against random 10% of top 200 drugs from live data | Should |

## 3. Data Model Changes

### SPLDetail (extended)

```go
type SPLDetail struct {
    Title              string               `json:"title"`
    SetID              string               `json:"set_id"`
    PublishedDate      string               `json:"published_date"`
    SPLVersion         int                  `json:"spl_version"`
    Interactions       []InteractionSection `json:"interactions"`
    Contraindications  []InteractionSection `json:"contraindications"`
    Warnings           []InteractionSection `json:"warnings"`
    AdverseReactions   []InteractionSection `json:"adverse_reactions"`
}
```

Reuses `InteractionSection` (Title + Text) for all sections — consistent shape.

## 4. Parser Changes

Extend `internal/spl/parser.go` with new regex patterns:

| Section | Numbered Pattern | Unnumbered Pattern |
|---------|-----------------|-------------------|
| 4 | `4 CONTRAINDICATIONS`, `4.1 ...` | `CONTRAINDICATIONS` |
| 5 | `5 WARNINGS AND PRECAUTIONS`, `5.1 ...` | `WARNINGS AND PRECAUTIONS` |
| 6 | `6 ADVERSE REACTIONS`, `6.1 ...` | `ADVERSE REACTIONS` |
| 7 | `7 DRUG INTERACTIONS`, `7.1 ...` | `Drug Interactions` (existing) |

## 5. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| OTC product (no sections 4-6) | Empty arrays for each missing section |
| Section present but no text block | Skip (same as current Section 7 behavior) |
| Very long section text (>10KB) | Return as-is (truncation deferred) |
| Numbered + unnumbered in same doc | Capture both |

## 6. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-20 | 0.1.0 | calebdunn | Initial spec |

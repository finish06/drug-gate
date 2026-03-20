# Project Learnings — drug-gate

> **Tier 3: Project-Specific Knowledge**
> Generated from `.add/learnings.json` — do not edit directly.
> Agents read JSON for filtering; this file is for human review.

## Anti-Patterns
- **[critical] Retro finding: learning checkpoints missed for 6 milestones** (L-012, 2026-03-20)
  M3 through M7 had no learning checkpoints recorded. Root cause: agent did not self-checkpoint after verify/cycle/deploy triggers. Documentation discipline must be enforced alongside code quality.

- **[high] M4 RxNorm: upstream response shape alignment required live testing** (L-007, 2026-03-14)
  RxNorm client needed 2 fix commits to align with actual cash-drugs response shapes. Mock-based unit tests passed but E2E revealed field name mismatches. Always validate against live upstream.

## Technical
- **[medium] cash-drugs uses slug-based routing with query params** (L-001, 2026-03-07)
  Upstream API at /api/cache/{slug}. Key slugs: fda-ndc-by-name (BRAND_NAME), drugnames, drugclasses, spls-by-name (DRUGNAME), spls-by-class (DRUG_CLASS). Flat array response shape (data: [...] not data.results).

- **[medium] Interface-based mocking enables comprehensive handler testing** (L-003, 2026-03-07)
  DrugClient, SPLClient, RxNormClient interfaces with mock implementations allow testing all handler paths without Redis or upstream. Pattern reused across all milestones.

- **[medium] M1 NDC Lookup complete: 42 tests, 97.1% coverage** (L-004, 2026-03-08)
  ACs covered: 17. RED: 32 tests. GREEN: all passing. Blockers: gitignore and Go version mismatch caused 3 fix commits. Spec quality: good.

- **[medium] M2 Security & Rate Limiting: 20 ACs, Redis integration tests behind build tag** (L-005, 2026-03-08)
  All gates pass. Coverage 65.3% total but non-Redis code >80%. Redis implementations at 0% because integration tests use //go:build integration tag. Fixed auth_test.go grace period bug.

- **[medium] M3 Extended Lookups: lazy Redis caching with sliding TTL** (L-006, 2026-03-09)
  4 endpoints (names, classes, class lookup, drugs-by-class). Lazy cache with 60-min sliding TTL using GetEx. ~104K drug names, ~1.2K classes cached. Pagination helper reused across all list endpoints.

- **[medium] M7 Operational Hardening: 4 features, 29 tests, TDD clean** (L-010, 2026-03-20)
  Request ID middleware (8 tests), autocomplete (13 tests), Redis persistence (ops docs), Prometheus alerts (8 tests). All TDD RED→GREEN confirmed. Coverage maintained at 80.7%.

## Architecture
- **[medium] Chi chosen for middleware-heavy gateway architecture** (L-002, 2026-03-07)
  Chi v5 over net/http stdlib — middleware chaining purpose-built for auth, rate limiting, NDC validation, logging, CORS, metrics, request ID. Uses stdlib interfaces.

- **[medium] M6 SPL Interactions: regex XML parsing sufficient for Section 7** (L-008, 2026-03-17)
  SPL XML has namespace issues making full XML parser complex. Regex extraction of Section 7 (Drug Interactions) works reliably. spls-by-class deferred (unreliable/timeouts). Cross-reference uses word-boundary regex matching.

## Process
- **[critical] Human directive: documentation is as important as code** (L-013, 2026-03-20)
  Specs must be updated to Complete when done. All learning checkpoints must be written. Documentation is a first-class deliverable, not an afterthought. This is the #1 improvement priority.

- **[medium] Beta promotion: 10/10 evidence score at alpha→beta** (L-009, 2026-03-17)
  All evidence items present: specs, 80%+ coverage, CI/CD, PR workflow, 2+ environments, conventional commits, TDD evidence, branch protection, release tags, quality gates.

- **[medium] k6 baseline comparison enables performance regression gates** (L-011, 2026-03-20)
  k6 harness covers all 21 endpoints across 4 scenarios (smoke/load/spike/soak). Baselines stored as JSON, comparison tool exits non-zero on >15% regression. Integrated into Makefile.

---
*13 entries. Last updated: 2026-03-20. Source: .add/learnings.json*

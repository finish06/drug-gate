# Project Learnings — drug-gate

> **Tier 3: Project-Specific Knowledge**
>
> This file is maintained automatically by ADD agents. Entries are added at checkpoints
> (after verify, TDD cycles, deployments, away sessions) and reviewed during retrospectives.
>
> This is one of three knowledge tiers agents read before starting work:
> 1. **Tier 1: Plugin-Global** (`knowledge/global.md`) — universal ADD best practices
> 2. **Tier 2: User-Local** (`~/.claude/add/library.md`) — your cross-project wisdom
> 3. **Tier 3: Project-Specific** (this file) — discoveries specific to this project
>
> **Agents:** Read ALL three tiers before starting any task.
> **Humans:** Review with `/add:retro --agent-summary` or during full `/add:retro`.

## Technical Discoveries
<!-- Things learned about the tech stack, libraries, APIs, infrastructure -->
<!-- Format: - {date}: {discovery}. Source: {how we learned this}. -->

- 2026-03-07: cash-drugs upstream API uses slug-based routing at /api/cache/{slug} with query params for filtering. Key slugs for drug-gate: fda-ndc-by-name (BRAND_NAME), drugnames, drugclasses, spls-by-name (DRUGNAME), spls-by-class (DRUG_CLASS). Source: cash-drugs OpenAPI spec and config.yaml.

## Architecture Decisions
<!-- Decisions made and their rationale -->
<!-- Format: - {date}: Chose {X} over {Y} because {reason}. -->

- 2026-03-07: Chose Chi over net/http stdlib because drug-gate is middleware-heavy (auth, rate limiting, NDC validation, logging, CORS) and Chi's middleware chaining is purpose-built for this while using stdlib interfaces.
- 2026-03-07: Chose Redis over in-memory state because rate limit counters and API key storage need to survive restarts and scale across instances.

## What Worked
<!-- Patterns, approaches, tools that proved effective -->

- 2026-03-07: Interface-based mocking (DrugClient interface) enabled comprehensive handler testing with both simple mockDrugClient and callCountMockClient for fallback error paths. Source: TDD cycle for ndc-lookup.
- 2026-03-07: cash-drugs flat array response shape (`data: [...]` not `data.results`) — discovered during integration testing. Always verify upstream response shapes against live service. Source: Integration test failure and fix.

## What Didn't Work
<!-- Patterns, approaches, tools that caused problems -->

- 2026-03-08: `.gitignore` pattern `server` matched `cmd/server/` directory, hiding source from git. Always use anchored patterns (`/server`) for build artifacts. Source: CI failure after first push.
- 2026-03-08: Go version mismatch between go.mod (1.25.5), Dockerfile (1.24), and CI (1.24) caused cascading failures including a `covdata` tool error. Keep all three in sync. Source: 3 fix commits to stabilize CI.
- 2026-03-08: swaggo/swag generates Swagger 2.0, not OpenAPI 3.0. Definition names use full module path prefix (e.g., `github_com_finish06_drug-gate_internal_model.DrugDetailResponse`). Use `strings.HasSuffix` in tests. Source: swagger_test.go AC-006.

## Agent Checkpoints
<!-- Automatic entries from verification, TDD cycles, deploys, away sessions -->
<!-- These are processed and archived during /add:retro -->

- 2026-03-07 [verify]: M1 NDC Lookup — 32 tests passing, coverage: ndc 100%, client 90.9%, handler 100%, middleware 100%. All 17 ACs mapped to tests. Security fix: added url.QueryEscape to client.go:48. Missing: LICENSE file, golangci-lint not installed.
- 2026-03-08 [retro]: M1 complete — 42 tests, 97.1% coverage, 7 commits, 3 specs delivered. Key wins: interface-based mocking, TDD caught security issue. Key issues: gitignore/Go version mismatches caused 3 fix commits. Promoted 3 learnings to user library.
- 2026-03-08 [verify]: Security & rate limiting — all gates pass, 20/20 ACs covered across 6 test files (42+ tests). Coverage 65.3% total; Redis implementations (RedisStore, RedisLimiter) at 0% because integration tests are behind `//go:build integration` tag. Non-Redis code >80%. Fixed pre-existing bug: auth_test.go grace period test used past time instead of future for "within grace period" scenario.

## Profile Update Candidates
<!-- Cross-project patterns flagged for promotion to ~/.claude/add/profile.md -->
<!-- Only promoted during /add:retro with human confirmation -->

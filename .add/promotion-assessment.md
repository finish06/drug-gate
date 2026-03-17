# Alpha → Beta Promotion Assessment

**Date:** 2026-03-16
**Current Maturity:** Alpha
**Target Maturity:** Beta

## Evidence Scan

| # | Evidence Item | Status | Details |
|---|---------------|--------|---------|
| 1 | Feature specs | PASS | 10 specs in `specs/` covering all user-facing features |
| 2 | Test coverage | PASS | 87.4% (threshold: 50% for alpha→beta) |
| 3 | CI/CD pipeline | PASS | `.github/workflows/ci.yml` — vet, unit, integration, coverage, Docker publish |
| 4 | PR workflow | PASS | 10 PRs merged via pull request workflow (PR #1–#10) |
| 5 | Environment separation | PASS | 4 compose files: local, staging, e2e, prod. Staging at 192.168.1.145:8082 |
| 6 | Conventional commits | PASS | All 20 recent commits follow `feat:/fix:/docs:/chore:` convention |
| 7 | TDD evidence | PASS | 35 test files. Test-first discipline visible in commit history (RED→GREEN→REFACTOR) |
| 8 | Branch protection | FAIL | `main` branch is NOT protected. PRs are used by convention, not enforced. |
| 9 | Release tags | PASS | 5 semantic version tags (v0.2.0 through v0.5.1) |
| 10 | Quality gates | PASS | CI runs vet + unit + integration + coverage on every push/PR |

**Score: 9/10** (missing: branch protection)

## Promotion Requirements (Alpha → Beta)

Per `rules/maturity-lifecycle.md`:
- [x] Feature specs for all user-facing features
- [x] 50%+ test coverage (actual: 87.4%)
- [x] CI pipeline active
- [x] PR workflow active
- [x] TDD evidence present

**All required criteria met.**

## Gap: Branch Protection

The only missing item is formal branch protection on `main`. This is currently enforced by convention (all changes go through PRs) but not by GitHub settings. This is a low-risk gap — the team is small and the convention is consistently followed.

**To fix:** Run this command:
```bash
gh api repos/finish06/drug-gate/branches/main/protection \
  -X PUT \
  -F required_status_checks='{"strict":true,"contexts":["test"]}' \
  -F enforce_admins=false \
  -F required_pull_request_reviews='{"required_approving_review_count":1}' \
  -F restrictions=null
```

## What Changes at Beta Maturity

Per the cascade matrix, promoting to beta activates:
- **TDD enforcement** — strict RED→GREEN→REFACTOR policy (already practiced)
- **Agent coordination** — parallel agents with worktree isolation (already used)
- **Environment awareness** — full promotion ladder with auto-promote (staging already set up)
- **Maturity lifecycle** — formal promotion tracking
- **Spec compliance** — all PRs reference spec ACs (already practiced)

No breaking changes. Beta formalizes what's already being done.

## Recommendation

**PROMOTE to Beta.** Evidence score 9/10. All required criteria met. The one gap (branch protection) is low-risk and fixable in 30 seconds.

Run `/add:retro` to formally promote (updates config, activates new rules).

## Metrics at Promotion

| Metric | Value |
|--------|-------|
| Specs | 10 |
| Test files | 35 |
| Coverage | 87.4% |
| E2E tests | 33 passing |
| PRs merged | 10 |
| Release tags | 5 (v0.2.0–v0.5.1) |
| Milestones completed | 7 (M1–M5, M3.5, M4) |
| Endpoints | 21 |
| Lines of Go | ~4,100+ |

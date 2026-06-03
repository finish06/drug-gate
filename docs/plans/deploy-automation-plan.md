# Implementation Plan: Production Deploy Automation

**Spec:** specs/deploy-automation.md (v0.1.0)
**Milestone:** M9.5 — Production Deploy
**Created:** 2026-06-02
**Team Size:** Solo
**Estimated Duration:** ~2-3 days (~17h with contingency)

## Overview

Add tag-triggered, human-approved, health-gated production deploys to the K3s
cluster (namespace `drugs`) via `kubectl` from GitHub Actions, with automatic
rollback on rollout or smoke failure, an audited manual `rollback-prod`
workflow, and an operational runbook. Image build/publish and the K8s manifest
files are out of scope (owned by `docker-build-publish` and the infra repo
respectively).

## Note on TDD

This feature is CI/infra (YAML workflows + a runbook); there is no Go unit under
test, so the RED→GREEN→REFACTOR loop does not apply. Verification is the 6 user
test cases executed against the cluster (Phase 4), plus static checks
(`actionlint`, `kubectl ... --dry-run=client`). The "tests pass" gate here means
the acceptance runs succeed, not a Go suite.

## Objectives

- A `deploy-prod` workflow: tag `v*` → approval → `kubectl set image` → rollout health gate → k6 smoke → auto-rollback on failure
- A `rollback-prod` workflow: `workflow_dispatch` → approval → `kubectl rollout undo`
- A production runbook covering deploy, rollback, Redis recovery, breaker reset
- The `production` GitHub Environment configured (required reviewer + scoped secrets)
- The readiness/liveness probe contract confirmed on the externally-owned manifest

## Acceptance Criteria Analysis

| AC | Complexity | Tasks | Notes |
|----|-----------|-------|-------|
| AC-001 tag `v*` triggers deploy of pinned image | Simple | TASK-004 | Derive version from tag ref |
| AC-002 production Environment approval gate | Simple | TASK-001, TASK-004 | GitHub Environment protection |
| AC-003 `kubectl -n drugs set image` | Simple | TASK-005 | Needs cluster creds |
| AC-004 rollout status health gate (120s) | Medium | TASK-006 | k8s-native |
| AC-005 readiness contract (/health) | Medium | TASK-002 | Manifest owned externally — verify/coordinate |
| AC-006 liveness lightweight (no crashloop) | Medium | TASK-002 | May need a liveness endpoint/probe choice |
| AC-007 auto-rollback on rollout fail | Medium | TASK-006 | `rollout undo` on failure |
| AC-008 post-promote k6 smoke vs prod | Simple | TASK-007 | Reuse tests/k6/staging.js |
| AC-009 auto-rollback on smoke fail | Medium | TASK-007 | `rollout undo` on smoke fail |
| AC-010 `rollback-prod` workflow_dispatch | Simple | TASK-009 | optional `to_revision` |
| AC-011 runbook ops/production-deploy.md | Simple | TASK-010 | |
| AC-012 no unapproved prod deploy | Simple | TASK-001, TASK-004 | Environment gate is the control |
| AC-013 creds only in prod Environment, not logged | Simple | TASK-001, TASK-008 | Secret scoping + masking |
| AC-014 serialize prod deploys (concurrency) | Simple | TASK-008 | `concurrency: group` |
| AC-015 report version + outcome | Simple | TASK-008 | `$GITHUB_STEP_SUMMARY` |
| AC-016 staging unchanged | Simple | TASK-010 | Document boundary only |

## Implementation Phases

### Phase 0: Prerequisites & Coordination (~3.5h)

| Task | Description | AC | Effort | Depends |
|------|-------------|-----|--------|---------|
| TASK-001 | Create GitHub `production` Environment: required reviewer + scoped secrets `KUBECONFIG` (or `K8S_SERVER`+`K8S_SA_TOKEN`+`K8S_CA`), `PROD_API_KEY` | AC-002, AC-013 | 1h | — |
| TASK-002 | Confirm the `drug-gate` Deployment in ns `drugs` has a readiness probe on `/health` and a **lightweight, dependency-free** liveness probe; if missing, file the change in the infra repo. Decide liveness target (e.g., TCP socket or a trivial `/livez`). Document the contract. | AC-005, AC-006 | 2h | — |
| TASK-003 | Verify `tests/k6/staging.js` smoke scenario runs deterministically against an arbitrary `BASE_URL` + `API_KEY` (it already parameterizes these) | AC-008 | 0.5h | — |

> **Liveness sub-decision (TASK-002):** drug-gate currently exposes only the
> dependency-aware `/health` (503 when Redis is down). Using it for liveness
> would crash-loop pods during a Redis outage. Options: (a) TCP/startup probe
> for liveness, or (b) add a trivial `/livez` (always 200 if the process is up).
> Capture the choice here; if (b), that is a small code change tracked as a
> follow-up task.

### Phase 1: deploy-prod workflow (~4h)

| Task | Description | AC | Effort | Depends |
|------|-------------|-----|--------|---------|
| TASK-004 | New `.github/workflows/deploy-prod.yml`: trigger on `push` tag `v*`; deploy job `environment: production`; `azure/setup-kubectl`; auth from env secret | AC-001, AC-002 | 1.5h | TASK-001 |
| TASK-005 | `kubectl -n drugs set image deployment/drug-gate drug-gate=<reg>/drug-gate:${TAG}` | AC-003 | 0.5h | TASK-004 |
| TASK-006 | `kubectl -n drugs rollout status deployment/drug-gate --timeout=120s`; on failure run `kubectl -n drugs rollout undo deployment/drug-gate` and fail the job | AC-004, AC-007 | 1h | TASK-005 |
| TASK-007 | Post-promote: `k6 run tests/k6/staging.js` (smoke) vs prod URL + `PROD_API_KEY`; on failure `rollout undo` + fail | AC-008, AC-009 | 1h | TASK-006 |
| TASK-008 | `concurrency: {group: prod-deploy}`; write deployed version + outcome to `$GITHUB_STEP_SUMMARY`; confirm secret masking | AC-013, AC-014, AC-015 | 0.5h | TASK-007 |

### Phase 2: rollback-prod workflow (~1.5h)

| Task | Description | AC | Effort | Depends |
|------|-------------|-----|--------|---------|
| TASK-009 | New `.github/workflows/rollback-prod.yml`: `workflow_dispatch` with optional `to_revision` input; `environment: production`; `kubectl -n drugs rollout undo deployment/drug-gate [--to-revision=N]`; `rollout status`; report outcome | AC-010 | 1.5h | TASK-001 |

### Phase 3: Runbook & Docs (~2.5h)

| Task | Description | AC | Effort | Depends |
|------|-------------|-----|--------|---------|
| TASK-010 | `ops/production-deploy.md`: deploy procedure, automatic rollback, manual rollback (workflow + `kubectl rollout undo` break-glass incl. `--to-revision` + `rollout history`), Redis recovery, circuit-breaker reset; note staging is unchanged | AC-011, AC-016 | 2h | Phase 1, 2 |
| TASK-011 | CHANGELOG `[Unreleased]` entry; update M9.5 milestone feature position | — | 0.5h | TASK-010 |

### Phase 4: Acceptance Verification (~3h)

Run the spec's user test cases against the cluster. **Prove rollback works
before trusting the deploy** — run TC-002/003 early.

| Task | Description | TC | Effort | Depends |
|------|-------------|-----|--------|---------|
| TASK-012 | Static checks: `actionlint` on both workflows; `kubectl ... --dry-run=client` for set-image/undo | — | 0.5h | Phase 1, 2 |
| TASK-013 | TC-002 + TC-003: deploy a deliberately-broken image (never-ready, then ready-but-smoke-fails) → confirm auto-rollback both ways | TC-002, TC-003 | 1h | Phase 1 |
| TASK-014 | TC-001: real `v*` deploy with approval → rollout + smoke pass → `/version` correct | TC-001 | 0.5h | TASK-013 |
| TASK-015 | TC-004: `rollback-prod` dispatch → approve → prior revision restored | TC-004 | 0.5h | Phase 2 |
| TASK-016 | TC-005 (approval gate enforced) + TC-006 (Redis outage → NotReady, no crashloop) | TC-005, TC-006 | 0.5h | TASK-002 |

## Effort Summary

| Phase | Hours |
|-------|-------|
| Phase 0 Prereqs | 3.5 |
| Phase 1 deploy-prod | 4.0 |
| Phase 2 rollback-prod | 1.5 |
| Phase 3 runbook/docs | 2.5 |
| Phase 4 verification | 3.0 |
| **Subtotal** | **14.5** |
| +15% contingency | ~2.2 |
| **Total** | **~17h (~2-3 days solo)** |

## Dependencies

- **TASK-001 (Environment + secrets)** blocks all `kubectl` steps
- **TASK-002 (probe contract)** blocks the health-gate semantics and TC-006; coordinated with the externally-owned manifest
- `docker-build-publish` release images (`:vX.Y.Z`) must exist for a given tag
- K3s cluster reachable from GitHub-hosted runners (or a self-hosted runner if the API server isn't publicly reachable — **open question**, see Risks)

## Risks

| Risk | Prob | Impact | Mitigation |
|------|------|--------|-----------|
| K3s API server not reachable from GitHub-hosted runners (homelab behind firewall) | Medium | High | Confirm reachability in TASK-001; if not, use a self-hosted runner or a tunnel. **Resolve before Phase 1.** |
| Liveness on `/health` crash-loops pods during Redis outage | Medium | High | TASK-002 chooses a dependency-free liveness probe (TCP or `/livez`) |
| `rollout undo` with no prior revision (first prod deploy) | Low | Medium | Runbook documents redeploying a known-good `v*` tag instead |
| Flaky smoke triggers a needless rollback | Medium | Medium | Keep the smoke scenario minimal + deterministic (TASK-003) |
| Testing on real production | Medium | High | Prove rollback first (TASK-013); deploy in a low-traffic window |
| Cluster creds leak via logs | Low | High | Scope secrets to the `production` Environment; rely on Actions masking; prefer a short-lived SA token over a full kubeconfig |

## Testing Strategy

- **Static:** `actionlint` for workflow syntax; `kubectl --dry-run=client` for command validity
- **Acceptance (operational):** the 6 TCs in Phase 4, run against the cluster — including two deliberate-failure runs to prove auto-rollback
- **No Go unit tests** are added (no application code changes, unless the `/livez` option is chosen in TASK-002 — that would get a small handler test)
- **Quality gate note:** the 80% coverage gate applies to Go code; these YAML/runbook deliverables are verified by the acceptance runs

## Deliverables

- `.github/workflows/deploy-prod.yml`
- `.github/workflows/rollback-prod.yml`
- `ops/production-deploy.md`
- (conditional) `/livez` handler + test, if TASK-002 picks option (b)
- GitHub `production` Environment (config, not a repo file)
- CHANGELOG entry; M9.5 milestone feature position update

## Success Criteria

- [ ] All 16 ACs implemented (13 Must) or contract-verified
- [ ] TC-001…TC-006 pass against the cluster
- [ ] Auto-rollback proven for both rollout-fail and smoke-fail
- [ ] `rollback-prod` workflow restores the prior revision through approval
- [ ] Runbook complete; staging deploy unchanged
- [ ] No prod deploy possible without the approval gate

## Plan History

- 2026-06-02: Initial plan from /add:plan (namespace `drugs`, rollout timeout 120s).

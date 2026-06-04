# Implementation Plan: Production Deploy Automation

**Spec:** specs/deploy-automation.md (v0.2.0)
**Milestone:** M9.5 — Production Deploy
**Created:** 2026-06-02 · **Revised:** 2026-06-03 (pivot)
**Team Size:** Solo
**Estimated Duration:** ~half a day (repo side)

## Overview

Production deploy is now an **announcement**, not a deploy. A `v*.*.*` tag publishes
a NATS event to the homelab bridge; the agent ("Joe") prompts a human in Discord and
owns the cluster deploy/health-gate/rollback. This repo's work is the small notify
workflow plus the `/livez` liveness endpoint and probe contract.

> **Pivot note:** the original plan (kubectl from a self-hosted in-cluster runner,
> GitHub Environment approval, rollout gate, smoke, rollback) was replaced. Those
> deliverables (`deploy-prod.yml`, `rollback-prod.yml`, `.github/actionlint.yaml`)
> were removed. The deploy/gate/rollback now live with Joe (out of scope).

## Note on TDD

`/livez` is the only application code (TDD-able: test → handler — done, shipped #31).
The notify workflow is YAML verified by `actionlint` + the `workflow_dispatch`
end-to-end test (operator confirms the Discord message). No Go suite covers the
workflow.

## Tasks

| Task | Description | AC | Status |
|------|-------------|-----|--------|
| T-1 | `/livez` dependency-free liveness endpoint (TDD) | AC-006 | **DONE** (#31) |
| T-2 | `notify-prod` job in `ci.yml` — `needs: publish`, `v*` tags only; POST event with the exact build digest; Bearer `NATS_BRIDGE_KEY`; fail CI on non-202 | AC-001…AC-005, AC-007, AC-008 | **DONE** (sequenced publish→notify; standalone workflow deleted) |
| T-3 | Rewrite `ops/production-deploy.md` for the tag→event→Discord→Joe flow + rollback/Redis/breaker | AC-009 | **DONE** (this PR) |
| T-4 | Remove obsolete kubectl workflows (`deploy-prod.yml`, `rollback-prod.yml`, `.github/actionlint.yaml`) | AC-010 | **DONE** (this PR) |
| T-5 | Add repo secret `NATS_BRIDGE_KEY` (operator-issued) | AC-003 | **USER** — needs repo settings access |
| T-6 | Confirm cluster Deployment probes: readiness=`/health`, liveness=`/livez` | AC-006 | **USER** — infra manifest |
| T-7 | Verify: `workflow_dispatch` test → operator confirms Discord (bridge 202), then a real tag | AC-001/002/007, TC-001/002 | **USER-supervised** |

## Verification

- **Static:** `actionlint` on `ci.yml`.
- **End-to-end:** cut a `v*` (or `v*-rc`) tag → CI `publish` then `notify-prod` →
  bridge 202 + operator confirms the Discord message (TC-001/TC-002). A bad key →
  `notify-prod` fails red (TC-003).
- `/livez` already covered by its unit test (#31).

## Risks

| Risk | Mitigation |
|------|-----------|
| `NATS_BRIDGE_KEY` mistyped / trailing newline / wrong scope | Use the `workflow_dispatch` test first; check the Publish-event step log for 401/403 (job may still exit 0 — known bridge wart) |
| Digest pre-resolve fails for this registry path | Best-effort; publishes `digest: "pending"`; Joe resolves on promote (AC-005) |
| Repo and operator disagree on `APP_NAME`/subject scope | `APP_NAME: drug-gate`; key scoped `joe.deploy.drug-gate.*`; confirm with operator |
| Liveness on `/health` would crash-loop on Redis outage | Liveness uses `/livez` (dependency-free) |

## Deliverables

- `notify-prod` job in `.github/workflows/ci.yml` (needs: publish, v* tags)
- `ops/production-deploy.md` (rewritten)
- `/livez` handler + test (shipped #31)
- Removal of `deploy-prod.yml`, `rollback-prod.yml`, `.github/actionlint.yaml`, and the standalone `notify-prod-promote.yml`
- (user) `NATS_BRIDGE_KEY` secret; probe wiring on the cluster manifest

## Success Criteria

- [ ] `v*.*.*` tag publishes the event; operator gets the Discord prompt
- [ ] `workflow_dispatch` test passes (bridge 202)
- [ ] Repo contains no kubectl/deploy/rollback (Joe owns it)
- [ ] Runbook reflects the tag→event→Discord→Joe flow
- [ ] `/livez` + probe contract in place

## Plan History

- 2026-06-02: Initial plan (kubectl from in-cluster runner).
- 2026-06-03: Pivot to the homelab notification model; scope shrank to the notify workflow + docs.

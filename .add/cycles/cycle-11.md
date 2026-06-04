# Cycle 11 — M9.5 Production Deploy Automation

**Milestone:** M9.5 — Production Deploy
**Maturity:** beta
**Status:** COMPLETE
**Started:** 2026-06-02
**Completed:** 2026-06-03
**Duration Budget:** 2-3 days

## Retrospective

Delivered `/livez` (TDD) + a working production-release announcement pipeline,
verified end-to-end: `v0.10.1-rc1` → CI `publish` → `notify-prod` → **bridge 202**
→ Joe's Discord prompt. Mid-cycle the deploy model **pivoted twice**: kubectl-from-CI
(Option A) → homelab NATS notification → folded the notify into `ci.yml` as a
`needs: publish` job. The pivots invalidated the kubectl workflows (built then
removed) but the spec/plan/cycle docs were kept in lockstep each step.

**What worked:** sequencing publish→notify (vs the operator's parallel digest-poll)
removed a real race that had timed out at the 5-min job cap. Failing CI on non-202
turned a silent wart into a visible signal. Static validation (actionlint) caught
issues before merge each time.

**What was harder:** the operator's handoff workflow had a latent timeout bug only
exposed by actually running it — the `workflow_dispatch` test is what surfaced it.
Lesson: run the real path early; don't trust a provided CI artifact until it's fired.

**Follow-ups:** `:latest` now points at `v0.10.1-rc1` (corrects on the next real
release). Cluster probe wiring (readiness/liveness) is the last downstream user step.

## Progress

- [x] **Item 2 — `/livez`** (2026-06-02): TDD cycle complete. `internal/handler/livez.go` + tests (RED→GREEN→VERIFY), route wired in `cmd/server/main.go`. Handler coverage 88%, lint clean. Swagger annotation added; regen deferred (`swag` not installed locally). Merged PR #31.
- [x] **Item 1 — execution surface** (2026-06-02): decided **Option A** — GitHub Actions (tag `v*` + `production` Environment approval), deploy job on self-hosted in-cluster runner `runs-on: [self-hosted, k3s]`.
- ~~Item 3 `deploy-prod` workflow~~ / ~~Item 4 `rollback-prod` workflow~~ — **REMOVED in the pivot** (kubectl deploy/rollback now owned by the homelab agent "Joe", not this repo).
- [x] **Item 5 — runbook** (2026-06-03): `ops/production-deploy.md` **rewritten** for the tag→NATS event→Discord→Joe flow.
- [x] **PIVOT (2026-06-03)** — deploy model changed to homelab notification (spec v0.2.0). Removed `deploy-prod.yml`, `rollback-prod.yml`, `.github/actionlint.yaml`.
- [x] **Sequenced publish→notify (2026-06-03, spec v0.3.0)** — the standalone `notify-prod-promote.yml` raced the image build (digest poll timed out before publishing; verified via a `workflow_dispatch` test that was cancelled at 5m). **Deleted it**; the notification is now a **`notify-prod` job in `ci.yml`**, `needs: publish`, `v*` tags only, carrying the exact build digest, failing CI on non-202. actionlint clean.
- [ ] **User: add `NATS_BRIDGE_KEY` secret** (operator-issued `NATS-gh-actions-drug-gate-…`).
- [ ] **User: cluster Deployment probes** readiness=`/health`, liveness=`/livez`.
- [ ] **User-supervised verify:** `workflow_dispatch` test → operator confirms Discord (bridge 202); then a real `v*.*.*` tag.

Validation: `notify-prod-promote.yml` lints clean; the deploy/gate/rollback are now Joe's (out of scope). Item 1's self-hosted-runner decision is moot under the new model.

## Goal

Implement the production deploy automation for drug-gate on K3s (namespace
`drugs`) to the point of **IN_PROGRESS**: deploy-prod + rollback-prod workflows,
the `/livez` liveness endpoint, and the runbook — all statically validated. The
**live** production verification (real `v*` deploy + deliberate-failure rollback
proofs) is a separate human-supervised step and is NOT in this cycle's autonomous
scope (production deploys always require human approval).

## Work Items

| # | Item | Spec/Plan | Current → Target | Assigned | Est. | Validation |
|---|------|-----------|------------------|----------|------|------------|
| 1 | Deploy execution surface decision (spike) | plan TASK-001 + risk | — | Agent-1 + human | 2h | Pinned: where `kubectl` runs (self-hosted/in-cluster/Gitea), how GitHub triggers it |
| 2 | `/livez` liveness endpoint | spec AC-006 | new → DONE | Agent-1 | 1h | TDD: failing test → handler → 200 always; unit + lint pass |
| 3 | `deploy-prod` workflow | spec AC-001/003/004/007/008/009/013/014/015 | new → IN_PROGRESS | Agent-1 | 4h | actionlint clean; `kubectl --dry-run=client` valid; gated on `production` env |
| 4 | `rollback-prod` workflow | spec AC-010 | new → IN_PROGRESS | Agent-1 | 1.5h | actionlint clean; dispatch + `rollout undo` validated dry-run |
| 5 | `ops/production-deploy.md` runbook | spec AC-011/016 | new → DONE | Agent-1 | 2h | Covers deploy, auto + manual rollback, Redis recovery, breaker reset |
| 6 | Probe-contract doc + prod GitHub Environment | spec AC-002/005/013 | — | human-assisted | 1h | `production` Environment created w/ reviewer + secrets; readiness/liveness contract recorded |

**Feature position:** Production Deploy Automation + Rollback — **PLANNED → IN_PROGRESS**.

## Open Decision (Item 1 — resolve first)

The K3s API is not assumed reachable from GitHub-hosted runners. User direction:
**Gitea is in-cluster (`gitea.kube.calebdunn.tech`); ideally GitHub triggers the
deploy.** Candidate designs (pick during the spike):

- **A. GitHub trigger + self-hosted in-cluster runner** — `deploy-prod` job `runs-on: [self-hosted, k3s]`; kubectl has direct API access. GitHub keeps tag-trigger + approval.
- **B. GitHub → Gitea Actions handoff** — GitHub workflow signals an in-cluster Gitea Actions job that runs kubectl. Two CI systems; more moving parts.
- **C. GitHub → in-cluster deploy webhook** — like the staging webhook, but the receiver runs `kubectl rollout` and reports status back (so the health gate/rollback live cluster-side).

Until pinned, the deploy workflow is written against **Option A** as the default
(least new infra, keeps the gate/rollback logic in the GH workflow), with
`runs-on` as the single swap point.

## Dependencies & Serialization

Single feature, single agent → **single-threaded**. Order:

```
Item 1 (surface decision) ─┐
Item 2 (/livez, independent) ─ can proceed in parallel-of-thought, merges first
Item 1 ──> Item 3 (deploy-prod)  [runs-on depends on the decision]
Item 1 ──> Item 4 (rollback-prod)
Item 3,4 ──> Item 5 (runbook references the real commands)
Item 6 (env + contract) ── prerequisite for any live run (not for writing workflows)
```

## Parallel Strategy

N/A — single agent, beta WIP respected (1 feature). No worktrees/file reservations needed.

## Validation Criteria

### Per-Item
- **Item 2 (/livez):** `GET /livez` returns 200 regardless of Redis state; unit test (RED→GREEN); `go vet` + lint clean; coverage maintained ≥80%.
- **Item 3 (deploy-prod):** `actionlint` clean; job bound to `production` environment; `kubectl set image` + `rollout status --timeout=120s` + smoke + `rollout undo`-on-failure present; `kubectl --dry-run=client` validates the commands.
- **Item 4 (rollback-prod):** `actionlint` clean; `workflow_dispatch` with optional `to_revision`; `rollout undo` validated dry-run.
- **Item 5 (runbook):** all five topics present (deploy, auto rollback, manual rollback, Redis recovery, breaker reset).
- **Item 6:** `production` Environment exists with a required reviewer + `KUBECONFIG`/SA + `PROD_API_KEY`; probe contract documented.

### Cycle Success Criteria
- [ ] Execution surface decided (Item 1)
- [ ] `/livez` shipped with a passing test (Item 2)
- [ ] Both workflows written + statically validated (Items 3, 4)
- [ ] Runbook complete (Item 5)
- [ ] Production Environment + probe contract in place (Item 6)
- [ ] Feature at **IN_PROGRESS**; live verification queued for the human
- [ ] Code review on the `/livez` PR (beta requires review)

### NOT in this cycle (human-supervised, → VERIFIED later)
- TC-001 live `v*` production deploy with approval
- TC-002 / TC-003 deliberate-failure auto-rollback proofs
- TC-004 `rollback-prod` live run
- TC-005 / TC-006 approval-gate + Redis-outage-no-crashloop checks against the cluster

## Agent Autonomy & Checkpoints (beta)

- **Plan approval:** this document (human approves before execution).
- **Execution:** agent implements Items 1-5 autonomously; Item 6 is human-assisted (GitHub Environment + cluster secrets need your access).
- **Review gate:** `/livez` change goes through a PR (admin-merge pattern in use this session). Workflows + runbook can be bundled in the same or a follow-up PR.
- **Hard stop:** no live production deploy is run autonomously — that's your supervised step.

## Notes / Risks

- **Surface decision (Item 1) is the critical-path unknown** — everything in Items 3-4 keys off `runs-on`. Resolve before writing the deploy step in earnest.
- **Liveness `/livez`** is the only application code; keep it trivial (no deps) so it can never 503.
- **Stale bookkeeping:** `.add/config.json` `cycle_history` only lists through cycle-8 (missing 9, 10); `cycle-10.md` is an untracked, abandoned PLANNED cycle. Worth reconciling at `--complete`/`/add:retro`.
- **Protected main:** this cycle file + all deliverables land via PR (no direct commits to `main`).

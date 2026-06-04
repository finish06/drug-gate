# Spec: Production Deploy Automation

**Slug:** deploy-automation
**Milestone:** M9.5 — Production Deploy
**Status:** Draft
**Maturity:** beta (targets GA candidate)

## 1. Feature Description

drug-gate releases to production by **announcing** a new version, not by deploying
it. Pushing a `v*.*.*` tag publishes a JSON event to the homelab NATS bridge; the
homelab agent ("Joe") sees the event and asks a human operator in **Discord**
whether to promote. On "yes", Joe runs the actual deploy on the K3s cluster
(namespace `drugs`): `kubectl set image`, rollout health gate, smoke, and rollback
on failure. The git tag is the deploy gate — same shape as `npm publish`.

**This repo's responsibility is the announcement only.** It runs no `kubectl`,
pushes no manifests, and has no GitHub approval gate. The deploy, health gate,
smoke, and rollback are owned by Joe (out of scope here). The repo also exposes
the `/livez` liveness endpoint and depends on a probe contract the cluster manifest
must honor.

Image build/publish is owned by `specs/docker-build-publish.md` (`:beta` on main,
`:vX.Y.Z` + `:latest` on tags); the notification runs in parallel and does not
depend on the image being ready.

> **History:** an earlier draft of this spec deployed via `kubectl` from a
> self-hosted in-cluster GitHub runner. That approach was replaced by the homelab
> notification model below (the operator owns deploy/rollback). See §9.

### User Story

As the operator of drug-gate, I want to cut a `v*` release tag and be asked in
Discord whether to promote — with the homelab agent handling the cluster deploy,
health gate, and rollback — so that releasing is a one-line `git push` and I never
run kubectl or click a GitHub approval.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | Pushing a tag matching `v*.*.*` triggers `notify-prod-promote.yml` | Must |
| AC-002 | The workflow POSTs a JSON event to `https://nats-publish.kube.calebdunn.tech/publish/joe.deploy.drug-gate.tag.<version>` | Must |
| AC-003 | The request authenticates with `NATS_BRIDGE_KEY` (Bearer), stored as a repo secret and never committed or echoed | Must |
| AC-004 | The event payload includes: `tag`, `digest`, `registry_path`, `release_url`, `workflow_run_url`, `actor`, `sha` | Must |
| AC-005 | The notification publishes regardless of image/digest availability — the digest is best-effort and is `"pending"` if unresolved (the build runs in parallel) | Must |
| AC-006 | `GET /livez` is a dependency-free liveness probe (always 200). The cluster Deployment uses readiness → `/health` (200 incl. `degraded` = ready; 503/`error` = not-ready) and liveness → `/livez` (so a Redis outage doesn't crash-loop pods) | Must |
| AC-007 | `workflow_dispatch` with a `tag` input fires the notification without cutting a real tag (end-to-end test) | Must |
| AC-008 | The subject is namespaced `joe.deploy.drug-gate.*`; the key is scoped to that prefix (publishing elsewhere returns 403) | Should |
| AC-009 | `ops/production-deploy.md` documents the tag→event→Discord→Joe flow, the `workflow_dispatch` test, rollback (operator/Joe), Redis recovery, and breaker reset | Must |
| AC-010 | The repo performs NO deploy/rollback itself — kubectl, rollout health gate, smoke, and rollback are owned by the homelab agent (Joe) | Must |
| AC-011 | Staging deploy behavior is unchanged (compose host + signed webhook + k6 smoke on push to `main`) | Should |

## 3. User Test Cases

### TC-001: Tag-triggered promote (happy path)
**Precondition:** `NATS_BRIDGE_KEY` set; Joe + Discord pipeline live.
**Steps:**
1. `git tag v0.11.0 && git push --tags`
2. `publish` builds `:0.11.0`; `notify-prod-promote` publishes the event (bridge 202)
3. Joe posts "drug-gate v0.11.0 published. Promote to prod?" in Discord
4. Human replies `yes`; Joe deploys (set image → rollout status → smoke)
5. `curl https://drug-gate.calebdunn.tech/version`
**Expected Result:** `/version` shows the new build; on a failed deploy Joe rolls back and reports it.
**Maps to:** TBD

### TC-002: workflow_dispatch test (no real release)
**Precondition:** `NATS_BRIDGE_KEY` set.
**Steps:** Actions → notify prod promote → Run workflow → `tag: v0.0.0-test`.
**Expected Result:** Publish-event step shows bridge **202**; operator confirms the Discord message arrived.
**Maps to:** TBD

### TC-003: Bad/misscoped key
**Steps:** Run the workflow with a wrong or wrongly-scoped `NATS_BRIDGE_KEY`.
**Expected Result:** Bridge returns **401** (unknown key) or **403** (wrong app namespace); no Discord message. Diagnosable in the Publish-event step log (note: the job may still exit 0 — a known bridge wart).
**Maps to:** TBD

### TC-004: Redis outage does not crash-loop pods
**Precondition:** liveness=`/livez`, readiness=`/health` on the Deployment.
**Steps:** Make Redis unreachable; observe pod conditions.
**Expected Result:** `/health` → 503/`error`; pods go **NotReady** (off the Service) but are **not** killed by liveness; they recover when Redis returns.
**Maps to:** TBD

### TC-005: Two tags in quick succession
**Steps:** Push `v0.11.0` then `v0.11.1` quickly.
**Expected Result:** Two events publish; Joe asks twice; the human can promote one and skip the other.
**Maps to:** TBD

## 4. Configuration & Secrets

| Secret (repo) | Purpose |
|---------------|---------|
| `NATS_BRIDGE_KEY` | Bearer token for the NATS bridge (`NATS-gh-actions-drug-gate-…`, from the homelab operator) |
| `REGISTRY_USERNAME` / `REGISTRY_PASSWORD` (optional, already present) | Let the workflow resolve the image digest; otherwise `digest: "pending"` |

| Parameter | Value |
|-----------|-------|
| `APP_NAME` | `drug-gate` |
| `IMAGE_PATH` | `dockerhub.calebdunn.tech/finish06/drug-gate` |
| Bridge | `https://nats-publish.kube.calebdunn.tech/publish/<subject>` |
| Subject | `joe.deploy.drug-gate.tag.<version>` |

## 5. Mechanism

**`notify-prod-promote.yml`** (`.github/workflows/`):
```
on: push tags v*.*.*  |  workflow_dispatch(tag)
job publish-event (ubuntu-latest):
  1. resolve tag (from ref or dispatch input)
  2. best-effort digest pre-resolve from the registry (→ "pending" on miss)   (AC-005)
  3. POST {tag,digest,registry_path,release_url,workflow_run_url,actor,sha}    (AC-002, AC-004)
     to .../publish/joe.deploy.drug-gate.tag.<version>
     Authorization: Bearer NATS_BRIDGE_KEY                                     (AC-003)
```

**Downstream (owned by Joe — out of scope here):** consumes the event, prompts in
Discord, and on approval runs `kubectl -n drugs set image` → `rollout status` (health
gate via `/health` readiness) → smoke → `rollout undo` on failure.

## 6. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Image not yet pushed when notify runs | Digest resolves to `"pending"`; event still publishes; Joe resolves the digest on promote (AC-005) |
| Wrong / misscoped `NATS_BRIDGE_KEY` | Bridge 401/403; no Discord message; visible in the step log (job may still exit 0) |
| Bridge down (5xx) | No event; operator owns the bridge — retry or ask in Discord |
| Two tags pushed quickly | Two events; Joe asks twice (TC-005) |
| Human replies `no` / `hold X` | Event archived / deferred; no deploy |
| Non-`v*.*.*` tag (e.g. `v1`, `beta`) | Notify does not trigger (pattern `v*.*.*`) |
| Redis down at deploy time | Pods NotReady via `/health`; not crash-looped (liveness `/livez`); rollout won't promote until healthy (AC-006) |

## 7. Dependencies

- **Homelab NATS bridge** (`nats-publish.kube.calebdunn.tech`), the **Joe** agent, and the **Discord** approval channel — operator-owned
- **`NATS_BRIDGE_KEY`** issued by the operator (scoped `joe.deploy.drug-gate.*`)
- **docker-build-publish** — produces the release images Joe deploys
- **M9 health endpoint** — `/health` three-tier status backs the readiness probe / Joe's health gate
- **`/livez`** liveness endpoint (this repo) + the cluster Deployment probe wiring

## 8. Out of Scope

- The deploy itself — kubectl, rollout health gate, smoke, rollback (owned by Joe)
- The K8s manifest files (infra repo / cluster)
- The NATS bridge, Joe agent, and Discord integration (operator-owned)
- Image building/publishing (`docker-build-publish`)
- Staging deploy (unchanged)

## 9. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-06-02 | 0.1.0 | calebdunn | Initial spec: K3s production deploy via kubectl from a self-hosted in-cluster GitHub runner (tag v* + GitHub Environment approval, rollout gate, smoke, rollback). |
| 2026-06-03 | 0.2.0 | calebdunn | **Pivot:** replaced repo-side kubectl deploy with the homelab notification model. A `v*.*.*` tag publishes a NATS event; Joe prompts in Discord and owns the deploy/gate/rollback. Removed `deploy-prod.yml`/`rollback-prod.yml`; added `notify-prod-promote.yml`. Kept `/livez` + the probe contract. |

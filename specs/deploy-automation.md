# Spec: Production Deploy Automation

**Slug:** deploy-automation
**Milestone:** M9.5 — Production Deploy
**Status:** Draft
**Maturity:** beta (targets GA candidate)

## 1. Feature Description

drug-gate releases to production by **announcing** a new version, not by deploying
it. On a `v*` tag, CI builds + pushes the release image and then a **`notify-prod`
job** publishes a JSON event to the homelab NATS bridge; the agent ("Joe") asks a
human operator in **Discord** whether to promote. On "yes", Joe runs the actual
deploy on the K3s cluster (namespace `drugs`): `kubectl set image`, rollout health
gate, smoke, and rollback on failure. The git tag is the deploy gate — same shape
as `npm publish`.

**This repo's responsibility is the announcement only.** It runs no `kubectl`,
pushes no manifests, and has no GitHub approval gate. The deploy, health gate,
smoke, and rollback are owned by Joe (out of scope here). The repo also exposes
the `/livez` liveness endpoint and depends on a probe contract the cluster manifest
must honor.

The notification is a job in **`ci.yml`** that runs **`needs: publish`** — i.e.
*after* the release image is built and pushed (`specs/docker-build-publish.md`). It
carries the **exact image digest** from the build (no registry polling, no race),
and the job **fails (red CI)** if the bridge does not return 202.

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
| AC-001 | On a `v*` tag, the `notify-prod` job in `ci.yml` runs `needs: publish` (only after the release image is built + pushed) and only for release tags | Must |
| AC-002 | The job POSTs a JSON event to `https://nats-publish.kube.calebdunn.tech/publish/joe.deploy.drug-gate.tag.<tag>` | Must |
| AC-003 | The request authenticates with `NATS_BRIDGE_KEY` (Bearer), stored as a repo secret and never committed or echoed | Must |
| AC-004 | The event payload includes: `tag`, `digest`, `registry_path`, `release_url`, `workflow_run_url`, `actor`, `sha` | Must |
| AC-005 | The event carries the exact image digest from the build step (`publish.outputs.image_digest`) — no registry polling; `"pending"` only if the build produced no digest | Must |
| AC-006 | `GET /livez` is a dependency-free liveness probe (always 200). The cluster Deployment uses readiness → `/health` (200 incl. `degraded` = ready; 503/`error` = not-ready) and liveness → `/livez` (so a Redis outage doesn't crash-loop pods) | Must |
| AC-007 | The `notify-prod` job **fails (non-zero / red CI)** if the bridge does not return 202 — a bad/misscoped key surfaces as a failed run, not a silent green | Must |
| AC-008 | The subject is namespaced `joe.deploy.drug-gate.*`; the key is scoped to that prefix (publishing elsewhere returns 403 → job fails per AC-007) | Should |
| AC-009 | `ops/production-deploy.md` documents the tag→publish→notify→Discord→Joe flow, rollback (operator/Joe), Redis recovery, and breaker reset | Must |
| AC-010 | The repo performs NO deploy/rollback itself — kubectl, rollout health gate, smoke, and rollback are owned by the homelab agent (Joe) | Must |
| AC-011 | Staging deploy behavior is unchanged (compose host + signed webhook + k6 smoke on push to `main`) | Should |

## 3. User Test Cases

### TC-001: Tag-triggered promote (happy path)
**Precondition:** `NATS_BRIDGE_KEY` set; Joe + Discord pipeline live.
**Steps:**
1. `git tag v0.11.0 && git push --tags`
2. CI `publish` builds + pushes `:0.11.0`; then `notify-prod` publishes the event (bridge 202)
3. Joe posts "drug-gate v0.11.0 published. Promote to prod?" in Discord
4. Human replies `yes`; Joe deploys (set image → rollout status → smoke)
5. `curl https://drug-gate.calebdunn.tech/version`
**Expected Result:** `/version` shows the new build; on a failed deploy Joe rolls back and reports it.
**Maps to:** TBD

### TC-002: Notification carries the exact digest
**Precondition:** `NATS_BRIDGE_KEY` set.
**Steps:** Push a `v*` tag; inspect the `notify-prod` job log.
**Expected Result:** The payload `digest` is the `sha256:…` from the build step (not `pending`); subject is `joe.deploy.drug-gate.tag.v…`; bridge returns **202**.
**Maps to:** TBD

### TC-003: Bad/misscoped key fails the job
**Steps:** Push a `v*` tag with a wrong or wrongly-scoped `NATS_BRIDGE_KEY`.
**Expected Result:** Bridge returns **401** (unknown key) or **403** (wrong namespace); the `notify-prod` job **fails (red CI)** with the HTTP code in the log; no Discord message.
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

Two jobs in **`ci.yml`**, on a `v*` tag:
```
test ──► publish (build + push :X.Y.Z / :latest; emit outputs.image_digest)
              └──► notify-prod   needs: publish, if: v* tag && is_release   (AC-001)
                     POST {tag,digest,registry_path,release_url,             (AC-002, AC-004)
                           workflow_run_url,actor,sha}
                       to .../publish/joe.deploy.drug-gate.tag.<tag>
                       Authorization: Bearer NATS_BRIDGE_KEY                 (AC-003)
                     digest = publish.outputs.image_digest (exact, no poll)  (AC-005)
                     non-202 → exit 1 (red CI)                               (AC-007)
```
Because `notify-prod` runs *after* `publish`, the image and its digest already
exist — no polling, no race, no `timeout` hack.

**Downstream (owned by Joe — out of scope here):** consumes the event, prompts in
Discord, and on approval runs `kubectl -n drugs set image` → `rollout status` (health
gate via `/health` readiness) → smoke → `rollout undo` on failure.

## 6. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Image build/push fails | `publish` fails → `notify-prod` is skipped (`needs: publish`); nothing announced |
| Build produced no digest | Payload `digest: "pending"`; event still publishes; Joe resolves on promote (AC-005) |
| Wrong / misscoped `NATS_BRIDGE_KEY` | Bridge 401/403 → `notify-prod` job **fails** with the code in the log; no Discord message (AC-007) |
| Bridge down (5xx) | Job fails (non-202); operator owns the bridge — retry or ask in Discord |
| Two tags pushed quickly | Two CI runs → two events; Joe asks twice |
| Human replies `no` / `hold X` | Event archived / deferred; no deploy |
| Non-`v*` tag / push to main | `notify-prod` does not run (gated on `v*` tag + `is_release`) |
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
| 2026-06-03 | 0.3.0 | calebdunn | Sequenced publish→notify: deleted standalone `notify-prod-promote.yml` (its digest poll raced the build and timed out before publishing). Notification is now a `notify-prod` job in `ci.yml`, `needs: publish`, on `v*` tags only, carrying the exact build digest; fails CI on non-202 (replaces the silent exit-0 wart). |

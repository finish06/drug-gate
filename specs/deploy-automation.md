# Spec: Production Deploy Automation

**Slug:** deploy-automation
**Milestone:** M9.5 — Production Deploy
**Status:** Draft
**Maturity:** beta (targets GA candidate)

## 1. Feature Description

Automate production deployment of drug-gate to the **K3s cluster** with a
health gate and automatic rollback, plus an audited manual rollback path and an
operational runbook. A production deploy is cut by pushing a `v*` release tag;
the deploy runs only after manual approval via a GitHub **production**
Environment. The deploy job points the cluster Deployment at the version-pinned
image, waits for the Kubernetes rollout to pass the readiness gate, runs a k6
smoke test against production, and automatically rolls back (`kubectl rollout
undo`) if either the rollout or the smoke test fails.

Image build and publish are already owned by `specs/docker-build-publish.md`
(`:beta` on main, `:vX.Y.Z` + `:latest` on tags); this spec consumes those
images and is **not** responsible for building them. The K8s manifests
(Deployment/Service/probes) are owned **outside this repo** (infra repo /
cluster); this spec defines the deploy job and the `/health` probe **contract**
it depends on, not the manifest files.

### User Story

As the operator of drug-gate, I want production deploys to be tag-triggered,
human-approved, health-gated, and automatically rolled back on failure — with a
one-command manual rollback and a runbook — so that I can release to the K3s
cluster confidently and recover fast when a deploy goes bad.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | A production deploy is initiated by pushing a git tag matching `v*`; the deploy targets the corresponding version-pinned release image (`<registry>/drug-gate:vX.Y.Z`) built by the existing publish pipeline | Must |
| AC-002 | The deploy job executes only after manual approval is granted on a GitHub **production** Environment (required reviewer); no production change occurs before approval | Must |
| AC-003 | The deploy job updates the cluster via `kubectl -n drugs set image deployment/drug-gate drug-gate=<registry>/drug-gate:vX.Y.Z` (namespace `drugs`), authenticating with cluster credentials stored in the production Environment secret | Must |
| AC-004 | The deploy waits on `kubectl rollout status deployment/drug-gate --timeout=120s`; the rollout completes only when new pods pass their readiness probe within the timeout | Must |
| AC-005 | **Readiness contract:** the production Deployment's readiness probe targets `/health`; HTTP 200 (including `status: "degraded"`) counts as ready, HTTP 503 (`status: "error"`, e.g. Redis down) counts as not-ready | Must |
| AC-006 | **Liveness contract:** the liveness probe uses a lightweight, dependency-free check (NOT the dependency-aware `/health`), so a Redis outage marks pods not-ready without crash-looping them | Must |
| AC-007 | If the rollout health gate fails (timeout or pods not ready), the deploy job automatically runs `kubectl rollout undo deployment/drug-gate` and the job is marked failed | Must |
| AC-008 | After a successful rollout, the job runs the k6 smoke suite (`tests/k6/staging.js`, smoke scenario) against the production URL using a production API key | Must |
| AC-009 | If the post-promote smoke test fails, the job automatically runs `kubectl rollout undo deployment/drug-gate` and is marked failed | Must |
| AC-010 | A separate `rollback-prod` workflow (`workflow_dispatch`) runs `kubectl rollout undo deployment/drug-gate` (optionally `--to-revision=N`) through the same production Environment approval gate | Must |
| AC-011 | A runbook at `ops/production-deploy.md` documents: deploy, automatic rollback, manual rollback (workflow + `kubectl rollout undo` break-glass), Redis recovery, and circuit-breaker reset | Must |
| AC-012 | No path deploys to production without the human approval gate — not on merge to main, not on tag push alone | Must |
| AC-013 | Cluster credentials and the production API key live only in the **production** GitHub Environment (not repo-wide secrets) and are never echoed in job logs | Must |
| AC-014 | Production deploys are serialized — a concurrency group prevents two prod deploys/rollbacks from running simultaneously | Should |
| AC-015 | The deploy and rollback jobs report the target version and final rollout outcome in the job summary/logs | Should |
| AC-016 | Staging deploy behavior is unchanged (compose host + signed webhook + k6 smoke); this milestone is production-only | Should |

## 3. User Test Cases

### TC-001: Tag-triggered, approved production deploy (happy path)
**Precondition:** Release image `:v0.11.0` exists in the registry; production Environment configured with a required reviewer; cluster reachable.
**Steps:**
1. `git tag v0.11.0 && git push --tags`
2. Observe the production Environment enter "waiting for review"; approve it
3. Deploy job runs `kubectl set image …:v0.11.0`, then `kubectl rollout status`
4. Rollout completes (pods ready); k6 smoke runs against the prod URL and passes
5. `curl https://drug-gate.calebdunn.tech/version`
**Expected Result:** Job succeeds; `/version` reports `git_commit`/version for `v0.11.0`; no rollback triggered.
**Maps to:** TBD

### TC-002: Failed rollout → automatic rollback
**Precondition:** A `v*` image that never becomes ready (e.g., bad config / ImagePullBackOff).
**Steps:**
1. Tag + approve the deploy
2. `kubectl rollout status` does not complete within `--timeout=120s`
**Expected Result:** Deploy job runs `kubectl rollout undo deployment/drug-gate`, job is marked failed, production continues serving the previous version.
**Maps to:** TBD

### TC-003: Smoke failure → automatic rollback
**Precondition:** A `v*` image whose pods become ready but fail the smoke suite (e.g., a broken route or wrong data).
**Steps:**
1. Tag + approve the deploy
2. Rollout completes (pods ready); k6 smoke fails
**Expected Result:** Job runs `kubectl rollout undo`, marks the deploy failed, production stays on the previous version.
**Maps to:** TBD

### TC-004: Manual rollback workflow
**Precondition:** Production is on `v0.11.0`; previous revision exists.
**Steps:**
1. `gh workflow run rollback-prod.yml` (optionally `-f to_revision=N`)
2. Approve the production Environment gate
3. Workflow runs `kubectl rollout undo deployment/drug-gate [--to-revision=N]`
**Expected Result:** Production reverts to the prior (or specified) revision; `/version` reflects the rolled-back build; outcome reported in the job summary.
**Maps to:** TBD

### TC-005: Approval gate enforced
**Precondition:** Required reviewer configured on the production Environment.
**Steps:**
1. Push a `v*` tag
2. Do not approve the production Environment
**Expected Result:** The deploy job stays pending; no `kubectl` command runs and production is unchanged until approval is granted (or the wait times out / is rejected).
**Maps to:** TBD

### TC-006: Redis outage does not crash-loop pods
**Precondition:** Liveness probe is a lightweight non-dependency check; readiness probe targets `/health`.
**Steps:**
1. With drug-gate running in the cluster, make Redis unreachable
2. Observe pod conditions
**Expected Result:** `/health` returns 503/`error`; pods go **NotReady** (removed from Service endpoints) but are **not** killed/restarted by liveness; when Redis recovers, pods return to Ready.
**Maps to:** TBD

## 4. Configuration & Secrets

**Production GitHub Environment (`production`):**

| Secret / Setting | Purpose |
|------------------|---------|
| `KUBECONFIG` (or `K8S_SERVER` + `K8S_SA_TOKEN` + `K8S_CA`) | Authenticate `kubectl` to the K3s cluster |
| `PROD_API_KEY` | API key for the k6 production smoke test |
| Required reviewer(s) | Manual approval gate |
| (reuse) `REGISTRY_USERNAME` / `REGISTRY_PASSWORD` | Pull access if needed by the cluster |

**Deploy parameters (defaults — confirm in plan):**

| Parameter | Default |
|-----------|---------|
| Deployment name | `drug-gate` |
| Container name | `drug-gate` |
| Namespace | `drugs` |
| Rollout timeout | `120s` |
| Smoke suite | `tests/k6/staging.js` (smoke scenario) |
| Production URL | `https://drug-gate.calebdunn.tech` |

## 5. Deploy/CI Contract

Two GitHub Actions workflows (or jobs), both pinned to the `production` Environment:

All `kubectl` commands run in namespace `drugs` (`-n drugs`).

**`deploy-prod` (trigger: push tag `v*`)**
```
1. (publish) release image :vX.Y.Z built + pushed   ← existing docker-build-publish
2. environment: production  → await manual approval  (AC-002, AC-012)
3. kubectl -n drugs set image deployment/drug-gate drug-gate=<reg>/drug-gate:vX.Y.Z   (AC-003)
4. kubectl -n drugs rollout status deployment/drug-gate --timeout=120s               (AC-004)
   └─ fail → kubectl -n drugs rollout undo deployment/drug-gate → job fails          (AC-007)
5. k6 run tests/k6/staging.js --env SCENARIO=smoke --env BASE_URL=<prod>             (AC-008)
   └─ fail → kubectl -n drugs rollout undo deployment/drug-gate → job fails          (AC-009)
6. report version + outcome                                                         (AC-015)
concurrency: group=prod-deploy (serialize)                                          (AC-014)
```

**`rollback-prod` (trigger: workflow_dispatch, input `to_revision` optional)**
```
1. environment: production  → await manual approval
2. kubectl -n drugs rollout undo deployment/drug-gate [--to-revision=N]               (AC-010)
3. kubectl -n drugs rollout status deployment/drug-gate + report outcome
```

The readiness/liveness probes (AC-005, AC-006) are defined in the externally-owned
Deployment manifest; this spec asserts the contract the deploy depends on.

## 6. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Tagged image not present in registry | Pods `ImagePullBackOff` → rollout status times out → auto `rollout undo` (AC-007) |
| Rollback with no previous revision (first-ever deploy) | `kubectl rollout undo` has nothing to revert to; runbook documents the manual recovery (redeploy a known-good tag) |
| Smoke test flaky vs. a real regression | Smoke uses the same suite as staging; a smoke failure rolls back. Accept that flaky smoke can trigger a rollback; tune the smoke scenario to be deterministic |
| Approval not granted / times out | Deploy job stays pending then is cancelled; no cluster change |
| Two deploys (or deploy + rollback) at once | Concurrency group serializes; the second waits or is superseded (AC-014) |
| Invalid/expired kubeconfig | Job fails at the auth step before any change is applied |
| Deployment name / namespace mismatch | `kubectl` fails fast; no partial change; surfaced in logs |
| Redis down during deploy | New pods are NotReady (503) → rollout never completes → auto rollback; existing pods keep serving per liveness contract (AC-006) |

## 7. Dependencies

- **M9 — Upstream Resilience (DONE):** three-tier `/health` status (`ok`/`degraded`/`error`, 503 on Redis down) is the basis for the readiness contract
- **docker-build-publish:** produces the version-pinned `:vX.Y.Z` release images this spec deploys
- **K3s cluster** with an existing `drug-gate` Deployment + Service + probes (manifests owned outside this repo)
- **GitHub `production` Environment** with required reviewer(s) and the secrets above
- **`tests/k6/staging.js`** smoke suite (reused against the prod URL)
- **`kubectl` in CI** (e.g., `azure/setup-kubectl`)
- **`ops/`** runbook directory (existing: `redis-persistence.md`, `prometheus-alerts.md`)

## 8. Out of Scope

- The K8s manifest files themselves (Deployment/Service/probes) — owned by the infra repo / cluster
- Any change to the staging deploy (remains compose host + signed webhook + k6 smoke)
- Canary / blue-green deploys — standard rolling update via the Deployment only
- Horizontal Pod Autoscaling / multi-cluster / multi-region
- Image building/publishing (owned by `docker-build-publish`)

## 9. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-06-02 | 0.1.0 | calebdunn | Initial spec from /add:spec interview. K3s production deploy: tag-triggered + GitHub Environment approval, kubectl rollout health gate, k6 smoke gate, auto-rollback, rollback-prod workflow + break-glass, runbook. Manifests owned externally. |

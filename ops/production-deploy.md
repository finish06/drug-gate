# Runbook: Production Deploy & Rollback

Production runs on a **K3s** cluster in namespace **`drugs`** (Deployment
`drug-gate`). Image registry: `dockerhub.calebdunn.tech/finish06/drug-gate`.
Staging is a separate compose host and is **not** covered here (it deploys via
the existing webhook + k6 smoke on push to `main`).

Spec: `specs/deploy-automation.md` · Plan: `docs/plans/deploy-automation-plan.md`

## Prerequisites

- **GitHub `production` Environment** with a required reviewer (the approval gate) and secrets:
  - `KUBECONFIG` — cluster credentials (kubeconfig or SA token) scoped to namespace `drugs`
  - `PROD_API_KEY` — API key for the production k6 smoke test
- **Self-hosted runner** labelled `[self-hosted, k3s]` registered inside the cluster (so `kubectl` reaches the API server). Needs `kubectl` and Docker (for `grafana/k6-action`) available.
- **Deployment probes** (owned by the cluster manifest, not this repo):
  - **readiness** → `GET /health` (HTTP 200 incl. `degraded` = ready; 503/`error` = not-ready)
  - **liveness** → `GET /livez` (dependency-free; never 503 on a Redis outage)

## Deploy (normal release)

1. Cut a release tag:
   ```
   git tag v0.11.0 && git push --tags
   ```
2. CI (`ci.yml` → `publish`) builds and pushes the release image `:0.11.0` + `:latest`.
3. The **Deploy Production** workflow starts and waits at the `production`
   Environment for approval. **Approve only once the release image build is green**
   (the manual gate is what serializes the deploy after the build).
4. On approval the deploy job:
   - `kubectl -n drugs set image deployment/drug-gate drug-gate=…:0.11.0`
   - `kubectl -n drugs rollout status … --timeout=120s` (readiness health gate)
   - runs the k6 smoke suite against `https://drug-gate.calebdunn.tech`
   - **auto-rolls back** (`kubectl rollout undo`) if the rollout or smoke fails
5. Verify: `curl https://drug-gate.calebdunn.tech/version` shows the new `git_commit`.

## Automatic rollback

The deploy workflow rolls back automatically when:
- the rollout does not become ready within `--timeout=120s` (e.g. `ImagePullBackOff`, failing readiness), or
- the post-promote k6 smoke fails.

In both cases the job runs `kubectl -n drugs rollout undo deployment/drug-gate`,
re-checks rollout status, and exits **failed**. Production stays on / returns to
the previous revision.

## Manual rollback

### Audited (preferred) — `rollback-prod` workflow
```
gh workflow run rollback-prod.yml                 # roll back to the previous revision
gh workflow run rollback-prod.yml -f to_revision=7  # roll back to a specific revision
```
Then approve the `production` Environment gate. The workflow runs `kubectl rollout
undo` (and `rollout status`) on the in-cluster runner.

### Break-glass (direct kubectl, needs cluster access)
```
kubectl -n drugs rollout history deployment/drug-gate          # list revisions
kubectl -n drugs rollout undo deployment/drug-gate             # previous revision
kubectl -n drugs rollout undo deployment/drug-gate --to-revision=7
kubectl -n drugs rollout status deployment/drug-gate --timeout=120s
```

> **First-ever deploy:** `rollout undo` has no prior revision to revert to.
> Recover by redeploying a known-good tag: `git tag v0.10.0-redeploy <good-sha>`
> … or `kubectl -n drugs set image …` to the last good image, then `rollout status`.

## Redis recovery

Redis is the **critical** dependency: when it is down, `/health` returns 503 and
pods go **NotReady** (removed from the Service), but `/livez` stays 200 so the
liveness probe does **not** kill them.

1. Check: `kubectl -n drugs get pods` (pods Running but NotReady) and `curl …/health` (`status: error`, Redis dep error).
2. Restore Redis (restart the Redis pod/statefulset, check persistence/AOF — see `ops/redis-persistence.md`).
3. Pods return to Ready automatically once `/health` recovers; no redeploy needed.
4. If pods were killed/rescheduled, confirm with `kubectl -n drugs rollout status deployment/drug-gate`.

## Circuit-breaker reset

The cash-drugs upstream breaker opens after 10 consecutive failures and serves
stale cache; it **auto-recovers** via a half-open probe after a 30s cooldown — no
manual reset is normally required.

- Inspect: `curl …/health` → the `circuit_breaker` dependency reports `degraded` when open.
- Force recovery: once upstream is healthy, the next half-open probe closes it.
  To clear immediately, restart the pods: `kubectl -n drugs rollout restart deployment/drug-gate`
  (breaker state is in-process, so a restart resets it).

## Notes

- All `kubectl` runs in namespace `drugs` on the in-cluster self-hosted runner.
- Production deploys **always** require the human approval gate — never autonomous (AC-012).
- If the runner uses an in-cluster ServiceAccount instead of a `KUBECONFIG` secret, drop the "Configure kubectl" step in both workflows and rely on the runner's default config.

# Runbook: Production Deploy & Rollback

Production runs on a **K3s** cluster in namespace **`drugs`** (Deployment
`drug-gate`). **The repo does not deploy.** Deployment is owned by the homelab
agent ("Joe") via a tag → event → human-approval pipeline. Your job from this
repo is: **push a `v*.*.*` tag.** Joe asks a human in Discord whether to promote,
and Joe runs the kubectl deploy, rollout gate, smoke, and rollback.

Staging is a separate compose host and is **not** covered here (it deploys via
the existing webhook + k6 smoke on push to `main`).

Spec: `specs/deploy-automation.md` · Plan: `docs/plans/deploy-automation-plan.md`

## The flow

```
git tag v0.11.0 && git push --tags
   │
   ▼
ci.yml:  test ──► publish (build + push :0.11.0 / :latest)
                     └──► notify-prod   (needs: publish, v* tags only)
                            POSTs event to the homelab NATS bridge
                            subject: joe.deploy.drug-gate.tag.v0.11.0
                            digest:  exact, from the build (no polling)
                            non-202  → job fails (red CI)
                                 │
                                 ▼
                          Joe posts in Discord:
                          "drug-gate v0.11.0 published. Promote to prod?"
                                 │
                        human: yes / no / hold X
                                 │
                          yes → Joe deploys (kubectl set image,
                                rollout status, smoke) + rolls back on failure
```

`notify-prod` runs **after** `publish`, so the image + digest already exist —
no race, no poll, no timeout.

There is **no GitHub UI approval** and **no kubectl in this repo**. The Discord
reply is the gate.

## Prerequisites (one-time)

- Repo secret **`NATS_BRIDGE_KEY`** = the `NATS-gh-actions-drug-gate-…` key from
  the homelab operator (Settings → Secrets and variables → Actions). Paste exactly,
  no trailing newline.
- (Optional) `REGISTRY_USERNAME` / `REGISTRY_PASSWORD` — already present (used by
  `publish`). Let the notify workflow attach the resolved image digest; otherwise
  it publishes `digest: "pending"` and Joe resolves it on promote.
- **Deployment probes** (owned by the cluster manifest, used by Joe's deploy):
  - **readiness** → `GET /health` (200 incl. `degraded` = ready; 503/`error` = not-ready)
  - **liveness** → `GET /livez` (dependency-free; never 503 on a Redis outage)

## Deploy (normal release)

1. `git tag v0.11.0 && git push --tags`
2. `publish` builds the image; `notify-prod-promote` publishes the event.
3. Watch Discord. Reply `yes` to promote (`no` to skip, `hold X` to defer X min).
4. Joe deploys and reports back. Verify: `curl https://drug-gate.calebdunn.tech/version`.

## Rollback

Rollback is operator/Joe-side, not a repo workflow. In the Discord channel, ask
the operator to roll back; Joe runs `kubectl -n drugs rollout undo deployment/drug-gate`
(optionally `--to-revision=N`). For break-glass with direct cluster access:

```
kubectl -n drugs rollout history deployment/drug-gate
kubectl -n drugs rollout undo deployment/drug-gate            # previous revision
kubectl -n drugs rollout undo deployment/drug-gate --to-revision=7
kubectl -n drugs rollout status deployment/drug-gate --timeout=120s
```

> **First-ever deploy:** `rollout undo` has no prior revision; redeploy a known-good
> tag instead (`kubectl -n drugs set image …` to the last good image, then `rollout status`).

## Testing the notification

There is no separate dispatch tester — the notification only fires on a real `v*`
tag (after a successful image build). To validate end-to-end, cut a throwaway
prerelease tag, e.g. `git tag v0.11.0-rc1 && git push --tags`, and watch the
`notify-prod` job in the CI run:

- Bridge **202** → event published; confirm with the operator that the Discord
  message arrived.
- **401** (`NATS_BRIDGE_KEY` missing/unknown) / **403** (wrong namespace) / **5xx**
  (bridge down) → the `notify-prod` job **fails red** with the HTTP code in the log.

> Note: a `v*-rc` tag still runs the normal `publish` (it pushes `:latest`). Use a
> real version when you actually intend to ship.

## Redis recovery

Redis is the **critical** dependency: when down, `/health` returns 503 and pods go
**NotReady** (removed from the Service), but `/livez` stays 200 so liveness does
**not** kill them.

1. Check: `kubectl -n drugs get pods` (Running but NotReady) and `curl …/health` (`status: error`).
2. Restore Redis (restart pod/statefulset; check AOF — see `ops/redis-persistence.md`).
3. Pods return to Ready automatically once `/health` recovers; no redeploy needed.

## Circuit-breaker reset

The cash-drugs upstream breaker opens after 10 consecutive failures and serves
stale cache; it **auto-recovers** via a half-open probe after 30s — no manual reset
normally needed.

- Inspect: `curl …/health` → the `circuit_breaker` dependency reports `degraded` when open.
- Force-clear: `kubectl -n drugs rollout restart deployment/drug-gate` (breaker state is in-process).

## Notes

- Subject convention: `joe.deploy.<app>.tag.<version>`; the key is scoped to `joe.deploy.drug-gate.*`.
- Production deploys **always** require a human Discord reply — never autonomous.
- Operator owns the bridge (`nats-publish.kube.calebdunn.tech`), the NATS broker, the Joe agent, and key issuance/rotation. Reach them in the deploy-promote Discord channel.

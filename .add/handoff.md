# Session Handoff
**Written:** 2026-06-03

## Status: cycle-11 COMPLETE — M9.5 repo-side VERIFIED (ready to close)

Production-release notification pipeline verified end-to-end: `v0.10.1-rc1` →
CI `publish` → `notify-prod` → **bridge 202** → Joe's Discord prompt (confirmed).
`/add:cycle --complete` ran: M9.5 hill chart updated, cycle-11 → COMPLETE,
`cycle_history` reconciled (added 9 + 11; 10 was abandoned), `current_cycle` = null,
learnings L-029 (process) + L-030 (CI race anti-pattern) written.

**M9.5 next:** ready to close (suggest `/add:milestone` or `/add:retro`). It gates
beta→GA with **M10 Admin Auth Hardening**. Remaining M9.5 work is downstream/user:
cluster Deployment probe wiring (readiness=`/health`, liveness=`/livez`).

## (historical) Deploy model journey

The "other option" arrived and was adopted. Production deploy is now an
**announcement**: a `v*.*.*` tag publishes a NATS event; the homelab agent ("Joe")
prompts a human in Discord to promote and owns the cluster deploy/health-gate/rollback.
This repo no longer runs kubectl.

## Deploy notification (current design — spec v0.3.0)
- Notification is a **`notify-prod` job in `ci.yml`**, `needs: publish`, **`v*` tags only**, carrying the **exact build digest** (`publish.outputs.image_digest`); **fails CI on non-202**.
- Standalone `notify-prod-promote.yml` was **deleted** — its registry digest-poll raced the build and timed out at `timeout-minutes: 5` before publishing (verified: a `workflow_dispatch` test run was cancelled mid-poll, publish step skipped). Sequencing publish→notify removes the race entirely.
- Earlier this session (#33, merged): pivot from kubectl deploy to the notification model; removed `deploy-prod.yml`/`rollback-prod.yml`/`.github/actionlint.yaml`. KEEP `/livez` (#31) + the readiness(`/health`)/liveness(`/livez`) probe contract.

## Still needs the user
- **Add repo secret `NATS_BRIDGE_KEY`** — the `NATS-gh-actions-drug-gate-…` key from the homelab operator (ask in the deploy-promote Discord channel). `REGISTRY_USERNAME`/`REGISTRY_PASSWORD` already exist (optional digest resolve).
- **Cluster Deployment probes:** readiness=`/health`, liveness=`/livez` (infra manifest).
- **Verify:** Actions → notify prod promote → Run workflow → `tag: v0.0.0-test` → operator confirms Discord (bridge **202**). Then a real `v*.*.*` tag.

## Completed earlier this session (all merged)
- #28 CORS fix (live on staging) · #29 roadmap split · #30 deploy-automation spec+plan · #31 `/livez` · #32 kubectl deploy/rollback workflows (now superseded by this pivot)

## Housekeeping debt (deferred)
- `.add/config.json` `cycle_history` stale (missing cycle-9, cycle-10); `cycle-10.md` abandoned. Reconcile at `/add:cycle --complete` / `/add:retro`.
- Swagger not regenerated for `/livez` (swag CLI not installed) — next `make swagger`.

## Resume
After the user wires `NATS_BRIDGE_KEY` + probes and the `workflow_dispatch` test passes, the cycle's repo-side work is VERIFIED → `/add:cycle --complete` (closes cycle-11, updates M9.5 hill chart).

# Session Handoff
**Written:** 2026-06-03

## Status: M9.5 cycle-11 — PIVOTED to homelab notification deploy model

The "other option" arrived and was adopted. Production deploy is now an
**announcement**: a `v*.*.*` tag publishes a NATS event; the homelab agent ("Joe")
prompts a human in Discord to promote and owns the cluster deploy/health-gate/rollback.
This repo no longer runs kubectl.

## This pivot PR (open / merging)
- ADD `.github/workflows/notify-prod-promote.yml` (`v*.*.*` + `workflow_dispatch` → POST event to `nats-publish.kube.calebdunn.tech`, Bearer `NATS_BRIDGE_KEY`)
- REMOVE `deploy-prod.yml`, `rollback-prod.yml`, `.github/actionlint.yaml` (kubectl approach)
- REWRITE `ops/production-deploy.md` (tag→event→Discord→Joe)
- REVISE `specs/deploy-automation.md` → v0.2.0, `docs/plans/deploy-automation-plan.md`, `cycle-11.md`, CHANGELOG
- KEEP `/livez` (#31) + the readiness(`/health`)/liveness(`/livez`) probe contract

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

# Session Handoff
**Written:** 2026-06-02

## In Progress
- **M9.5 — Production Deploy** is now the active milestone (`planning.current_milestone`). Staging deploy automation exists in CI; remaining work is the production path: health-gated deploy, one-command rollback, runbook. Next step: `/add:spec deploy-automation` then `/add:cycle --plan`.

## Completed This Session
- **CORS preflight fix shipped** (PR #28, merged `e923eb7`, `--admin` override). `CORSPreflight` middleware answers keyless preflights before auth → 204 + reflected origin. Live on staging (`/version` = `beta-e923eb77`, git_commit `e923eb7`). Verified: unit (92.4%), e2e (regression tests green in CI), and a live staging preflight returning 204. Production not deployed (needs approval).
- **Roadmap reorg** (PRD §6 → v0.4.0): split M9 into **M9 Upstream Resilience [DONE, v0.8.0]** + **M9.5 Production Deploy [NOW/IN_PROGRESS]**. Reconciled the stale M9 detail block and milestone file (was DONE but bundled production deploy with all criteria unchecked). Created `docs/milestones/M9.5-production-deploy.md`. GA path now gated on M9.5 + M10.

## Decisions Made
- Preflight reflects origin; per-key origin locking enforced on the actual request (user-confirmed).
- e2e CI job is full-suite but non-blocking (cash-drugs pulls live FDA/DailyMed; upstream hiccups must not block PRs).
- M9.5 marked IN_PROGRESS (not NOT_STARTED) because CI already covers version-pinned builds + staging deploy + smoke.

## Blockers
- None. (Pre-existing e2e failures `TestE2E_Health` + `AC010_DrugsbyClassMissingParam` are not data-dependent and look worth a separate look.)

## Next Steps
1. `/add:spec deploy-automation` — spec the production health-gate + rollback + runbook for M9.5.
2. `/add:cycle --plan` to start M9.5 execution.
3. Optionally fix the two pre-existing non-data e2e failures.

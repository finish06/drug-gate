# Session Handoff
**Written:** 2026-06-02

## In Progress
- None — CORS preflight fix is complete and verified locally, not yet committed.

## Completed This Session
- **Fixed CORS preflight 401 bug** (security-rate-limiting AC-021). Browsers strip `X-API-Key` from preflights, but `/v1/*` ran `APIKeyAuth` before CORS, so keyless OPTIONS got 401 and the browser blocked the real request (mislabeled "TLS error" by callers).
- Added `CORSPreflight` middleware (`internal/middleware/cors.go`) ahead of auth: answers keyless preflights with 204, reflects `Origin`, advertises `X-API-Key` in `Access-Control-Allow-Headers`, sets `Vary: Origin`.
- Simplified `PerKeyCORS` — removed its (now-unreachable) preflight block; it only sets ACAO on the actual authenticated request. Origin locking still enforced there.
- Wired `middleware.CORSPreflight` into `/v1` chain in `cmd/server/main.go` before `APIKeyAuth`.
- TDD: RED → GREEN → REFACTOR → VERIFY. Replaced 3 preflight tests that injected a key into a path that never exists in production; added 6 chain-level tests. Middleware coverage 92.4%.
- Docs: spec AC-021 + edge case + revision history; sequence-diagram.md middleware overview + preflight note; CHANGELOG [Unreleased]; learning L-028.

## Decisions Made
- **Preflight origin policy:** reflect the requesting origin at preflight; enforce the per-key allowlist on the actual request (user-confirmed). A preflight can't carry the key, so per-key enforcement at preflight is impossible; the security boundary lives on the real response's ACAO.

## Blockers
- None.

## Next Steps
1. Commit on a `fix/` branch (conventional commit) and open a PR — beta requires PR review before merge; do not merge to main without approval.
2. Verify in a real browser against staging (cross-origin `fetch` from an allowed origin) once deployed.
3. Optional: add an e2e preflight assertion in `tests/e2e/` to lock the behavior at the HTTP boundary.

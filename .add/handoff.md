# Session Handoff
**Written:** 2026-03-25

## In Progress
- Nothing active. M10.5 landing page complete.

## Completed This Session
- Swagger security annotations: ApiKeyAuth + AdminAuth visible in Swagger UI (998f5e1)
- CI deploy webhook: HMAC-signed POST triggers staging deploy after beta push (cf5d899)
- CI k6 smoke tests: 36-check suite runs after deploy webhook (2a1f0ed)
- Swagger host fix: removed hardcoded localhost, UI uses current page URL (0d22f83)
- Documentation refresh: 7 new sequence diagrams, middleware chain updated, CLAUDE.md synced (0d22f83)
- M10.5 Landing Page: marketing page at dg.calebdunn.tech (GitHub Pages + custom domain)
- LANDING_URL redirect: config-driven 302 from GET /, documented for self-hosters
- Umami analytics on landing page
- Retroactive spec for landing page (specs/landing-page.md)
- 5 performance fixes: CacheTTL atomic, stampede prevention, ToLower elimination, RxNorm parallel, interaction pre-alloc (144f001)
- README synced with all current features, endpoints, env vars
- Changelog updated with 15 [Unreleased] entries
- GitHub secrets configured: WEBHOOK_SECRET, STAGING_WEBHOOK_URL, STAGING_API_KEY

## Decisions Made
- Landing page hosted on GitHub Pages (not embedded in Go binary)
- Custom domain: dg.calebdunn.tech via CNAME
- LANDING_URL is config-driven — unset = no redirect (self-hoster friendly)
- CacheTTL changed from package var to atomic.Int64
- Autocomplete stampede uses TryLock + stale serving

## Blockers
- None

## Next Steps
1. Tag v0.9.0 release with all [Unreleased] changes
2. M10: Admin Auth Hardening (HMAC tokens, rotation, audit log)
3. GA promotion requires: M10 + 30 days stability
4. Consider Tier 3 items: PERF-2 (autocomplete 104K deserialization — partially addressed by stampede fix)

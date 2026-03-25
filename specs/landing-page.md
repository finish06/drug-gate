# Landing Page

**Status:** IMPLEMENTED
**Milestone:** M10.5
**Implemented:** 2026-03-24

## Feature Description

A public marketing landing page for drug-gate hosted on GitHub Pages. Explains what the service does, showcases key API endpoints, and provides links to the GitHub repo, Swagger UI, and live demo. The drug-gate binary supports a config-driven redirect from `GET /` to the landing page URL via `LANDING_URL` env var.

## User Story

As a **developer discovering drug-gate**, I want a clear landing page that explains what it does and how to self-host it, so that I can quickly evaluate whether it fits my needs.

## Acceptance Criteria

- AC-001: Landing page is accessible at a public URL (GitHub Pages with custom domain)
- AC-002: Page has a hero section explaining what drug-gate does
- AC-003: Page shows key API endpoints with example JSON responses
- AC-004: Page links to GitHub repo for source code and self-hosting
- AC-005: Page links to Swagger UI for interactive API exploration
- AC-006: Page links to live staging demo
- AC-007: Page includes quick-start instructions (`docker compose up`)
- AC-008: Page is responsive (mobile + desktop)
- AC-009: `LANDING_URL` env var controls 302 redirect from `GET /` — when set, redirects; when unset, no redirect (default for self-hosters)
- AC-010: GitHub Pages auto-deploys from `landing/` directory on push to main
- AC-011: Landing page does not conflict with existing API routes (`/v1/*`, `/admin/*`, `/swagger/*`, `/health`, `/metrics`)

## User Test Cases

- TC-001: Visit `https://dg.calebdunn.tech` — landing page loads with hero, API overview, and links
- TC-002: Click "Try the API" — opens Swagger UI in new tab
- TC-003: Click "View on GitHub" — opens repo in new tab
- TC-004: Click copy button on install bar — `docker compose up` copied to clipboard
- TC-005: Visit `https://drug-gate.staging.calebdunn.tech/` — 302 redirect to landing page
- TC-006: Visit staging `/health` — still returns 200 (no route conflict)
- TC-007: Self-hoster runs without `LANDING_URL` set — `GET /` returns 404 (no redirect)
- TC-008: View page on mobile device — layout is responsive, no horizontal scroll

## Data Model

N/A — static HTML page, no data persistence.

## API Contract

### New behavior on `GET /`

| Condition | Response |
|-----------|----------|
| `LANDING_URL` set | 302 redirect to configured URL |
| `LANDING_URL` unset | No route registered (404) |

## Edge Cases

- `LANDING_URL` set to empty string → treated as unset (no redirect)
- `LANDING_URL` set to invalid URL → Go's `http.Redirect` sends it as-is (browser handles)
- HEAD request to `GET /` → 405 Method Not Allowed (Chi exact method matching)

## Technical Notes

- Landing page: single `landing/index.html` with inline CSS, Google Fonts, Umami analytics
- GitHub Pages: deployed via `.github/workflows/pages.yml` using `actions/deploy-pages@v4`
- Custom domain: `landing/CNAME` file with `dg.calebdunn.tech`
- Redirect: 4 lines in `cmd/server/main.go` — reads `LANDING_URL` env var, registers `r.Get("/", ...)` if set

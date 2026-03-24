# Cycle 9 — M10.5 Landing Page

**Milestone:** M10.5 — Landing Page
**Maturity:** beta
**Status:** PLANNED
**Started:** TBD
**Completed:** TBD
**Duration Budget:** 2-3 days

## Work Items

| Feature | Current Pos | Target Pos | Assigned | Est. Effort | Validation |
|---------|-------------|-----------|----------|-------------|------------|
| Landing page HTML | SHAPED | VERIFIED | Agent-1 | ~3 hours | Polished marketing page with hero, API overview, links |
| GitHub Pages deploy | SHAPED | VERIFIED | Agent-1 | ~30 min | Page live at finish06.github.io/drug-gate |
| Config-driven redirect | SHAPED | VERIFIED | Agent-1 | ~1 hour | `LANDING_URL` env var → 302 redirect from GET / |
| CI for GitHub Pages | SHAPED | VERIFIED | Agent-1 | ~30 min | Auto-deploy on push to main |

## Design Decisions

- **Hosting:** GitHub Pages from repo (gh-pages branch or /docs)
- **Design:** Creative freedom — polished, professional marketing page
- **Sections:** Hero (what it does) + API overview (key endpoints) + links (GitHub, Swagger, demo)
- **Redirect:** `LANDING_URL` env var controls redirect from GET /
  - Set → 302 redirect to that URL
  - Empty/unset → no redirect (default for self-hosters)
- **No framework:** Single index.html with inline CSS

## Dependencies & Serialization

```
Landing page HTML
    ↓
GitHub Pages deploy + CI (needs the HTML)

Config-driven redirect (independent — Go code change)
```

Redirect can be built in parallel with the landing page HTML.

## Validation Criteria

### Per-Item Validation
- **Landing page:** Responsive, looks professional, has hero + API overview + links sections
- **GitHub Pages:** Accessible at finish06.github.io/drug-gate
- **Redirect:** `LANDING_URL=https://finish06.github.io/drug-gate` → GET / returns 302. Unset → no redirect.
- **CI:** Push to main auto-deploys landing page to GitHub Pages

### Cycle Success Criteria
- [ ] Landing page live on GitHub Pages
- [ ] Production & staging configured with LANDING_URL redirect
- [ ] Self-hosters get no redirect by default (LANDING_URL unset)
- [ ] CLAUDE.md updated with LANDING_URL env var
- [ ] Sequence diagram updated if new route added

## Agent Autonomy & Checkpoints

Beta maturity, balanced autonomy. Agent executes, human reviews final design before merge.

## Notes

- Page is a marketing page linked from personal website
- Must not conflict with existing /v1/*, /admin/*, /swagger/*, /health, /metrics routes
- LANDING_URL documented in CLAUDE.md env var table

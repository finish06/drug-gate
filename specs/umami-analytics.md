# Umami Analytics Integration

**Status:** IMPLEMENTED
**Milestone:** —
**Implemented:** 2026-04-08

## Feature Description

Client-side analytics for the drug-gate landing page using a self-hosted Umami instance. Tracks page views automatically and captures custom events on all interactive elements (CTAs, navigation links, copy actions) via `data-umami-event` attributes. Provides visibility into visitor behavior and conversion funnel (page view → Swagger/GitHub click).

## User Story

As a **project maintainer**, I want to see how visitors interact with the landing page — which CTAs they click, whether they copy the install command, and which outbound links they follow — so that I can measure interest and optimize the page layout.

## Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | Umami tracking script loads on the landing page via `<script defer>` tag | Must |
| AC-002 | Script points to self-hosted Umami instance (`umami.calebdunn.tech`) with correct `data-website-id` | Must |
| AC-003 | Page views are tracked automatically on page load (no custom JS required) | Must |
| AC-004 | Hash-based navigation (`#api`, `#features`) is tracked by Umami's built-in History API monitoring | Should |
| AC-005 | Hero "Try the API" button fires custom event `hero-try-api` | Must |
| AC-006 | Hero "View on GitHub" button fires custom event `hero-view-github` | Must |
| AC-007 | Docker install "Copy" button fires custom event `copy-install-command` | Must |
| AC-008 | Nav "GitHub" link fires custom event `nav-github` | Must |
| AC-009 | Nav "Swagger UI" link fires custom event `nav-swagger` | Must |
| AC-010 | CTA "Star on GitHub" button fires custom event `cta-star-github` | Must |
| AC-011 | CTA "Explore Swagger UI" button fires custom event `cta-explore-swagger` | Must |
| AC-012 | All custom events use `data-umami-event` attributes (no inline JS tracking calls) | Must |
| AC-013 | Analytics script does not block page rendering (`defer` attribute present) | Must |
| AC-014 | Landing page load time is not measurably affected by the analytics script | Should |
| AC-015 | Events appear in the Umami dashboard under the Events section | Must |

## User Test Cases

- TC-001: Load landing page — page view appears in Umami dashboard within 30 seconds
- TC-002: Click "Try the API" in hero — `hero-try-api` event recorded in Umami
- TC-003: Click "Copy" on install bar — `copy-install-command` event recorded in Umami
- TC-004: Click nav "Swagger UI" — `nav-swagger` event recorded in Umami
- TC-005: Click CTA "Star on GitHub" — `cta-star-github` event recorded in Umami
- TC-006: Open landing page with browser DevTools Network tab — `script.js` loads from `umami.calebdunn.tech`, no errors
- TC-007: View page with JavaScript disabled — page renders normally (analytics silently skipped)

## Data Model

N/A — all data is stored in the external Umami instance, not in drug-gate.

## API Contract

N/A — client-side only integration. No backend changes.

## Event Naming Convention

Events follow a `{section}-{action}` pattern:

| Section | Action | Event Name |
|---------|--------|------------|
| `hero` | Try the API click | `hero-try-api` |
| `hero` | View on GitHub click | `hero-view-github` |
| `copy` | Install command copy | `copy-install-command` |
| `nav` | GitHub link click | `nav-github` |
| `nav` | Swagger UI link click | `nav-swagger` |
| `cta` | Star on GitHub click | `cta-star-github` |
| `cta` | Explore Swagger UI click | `cta-explore-swagger` |

## Edge Cases

- Umami instance is unreachable → script fails to load silently, page functions normally
- User has ad-blocker that blocks Umami → events not sent, no errors thrown
- User clicks a tracked link before script finishes loading → event may be missed (acceptable — `defer` loads fast)
- Multiple rapid clicks on same element → Umami deduplicates or records each (Umami's default behavior)

## Technical Notes

- Implementation: purely declarative via `data-umami-event` HTML attributes on `landing/index.html`
- No additional JavaScript written — Umami's `script.js` automatically picks up `data-umami-event` attributes
- Umami instance: `https://umami.calebdunn.tech` (self-hosted, separate infrastructure)
- Website ID: `64b09690-82e3-421c-8aa5-ed90f545d53d`
- The landing page is a single static HTML file, not a true SPA — Umami's SPA tracking (History API monitoring) applies to hash navigation only

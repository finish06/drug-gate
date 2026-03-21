# Cycle 7 — Bugathon Tier 2

**Milestone:** M8.5 — Bugathon
**Maturity:** Beta
**Status:** COMPLETE
**Started:** 2026-03-21
**Completed:** 2026-03-21
**Duration Budget:** 5 hours (away mode)

## Work Items

| Bug ID | Bug | Severity | Est. | Validation |
|--------|-----|----------|------|------------|
| DX-1 | Inconsistent error codes across handlers (6 conventions) | HIGH | 1h | Test: all handlers use canonical error codes |
| DX-2 | Whitespace handling inconsistent | HIGH | 30m | Test: leading/trailing whitespace trimmed |
| API-1 | RxNorm NDC returns 404 for valid RxCUI with no data | MEDIUM | 10m | Test: empty NDCs → 200 + empty array |
| DX-3 | Rate limit response missing X-RateLimit-Limit header | MEDIUM | 15m | Test: header present on all rate-limited responses |
| SEC-4 | Rate limit metrics expose full API key | LOW | 15m | Test: metrics use truncated key |
| OBS-1 | Health check doesn't verify Redis or upstream | HIGH | 30m | Test: health reports dependency status |
| DX-4 | Autocomplete lacks pagination wrapper | — | 10m | **Not a bug** — update spec to document no pagination |

## Dependencies & Serialization

DX-1 (error code standardization) should go first — other fixes may reference the canonical codes.

## Execution Plan

### DX-1: Standardize error codes (~1h)
- Audit all handlers for error code strings
- Define canonical set: `unauthorized`, `bad_request`, `not_found`, `upstream_error`, `internal_error`, `rate_limited`
- Update all handlers to use canonical codes
- Test: verify each handler returns expected canonical codes

### DX-2: Whitespace trimming (~30m)
- Add `strings.TrimSpace` to query params: name, q, class, ndc
- Test: " warfarin " treated same as "warfarin"

### API-1: RxNorm empty NDCs (~10m)
- Find where 404 is returned for valid RxCUI with no NDCs
- Change to 200 + empty NDCs array
- Test: valid RxCUI with no NDCs → 200

### DX-3: X-RateLimit-Limit header (~15m)
- Add X-RateLimit-Limit header in rate limit middleware (total quota from API key)
- Test: header present with correct value

### SEC-4: Truncate API key in metrics (~15m)
- Replace full key in Prometheus label with first 8 chars + "..."
- Test: metric label doesn't contain full key

### OBS-1: Health check dependencies (~30m)
- Add Redis ping and upstream health check to /health
- Return degraded status if dependencies unhealthy
- Test: health reports redis/upstream status

### DX-4: Update autocomplete spec (~10m)
- Mark DX-4 as not-a-bug in milestone
- Update specs/drug-autocomplete.md to explicitly document: no pagination, returns capped list

## Validation Criteria

- [ ] All 6 bugs fixed with tests
- [ ] DX-4 documented as intentional design
- [ ] All existing tests pass
- [ ] Coverage stays above 80%
- [ ] PR created

## Agent Autonomy (Away Mode)

**Autonomous:** Fix bugs, write tests, commit per fix, push, create PR.
**Boundaries:** Do NOT merge. Do NOT deploy.

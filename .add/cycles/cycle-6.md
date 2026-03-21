# Cycle 6 — Bugathon Tier 1

**Milestone:** M8.5 — Bugathon
**Maturity:** Beta
**Status:** PLANNED
**Started:** TBD
**Completed:** TBD
**Duration Budget:** 12 hours (away mode)

## Work Items

| Bug ID | Bug | Severity | Est. | Validation |
|--------|-----|----------|------|------------|
| SEC-1 | Admin endpoints open when ADMIN_SECRET unset | CRITICAL | 30m | Test: empty secret rejects all admin requests |
| DAT-1 | SPL search pagination returns total_pages: 0 | CRITICAL | 5m | Test: pagination metadata correct |
| ERR-1 | Autocomplete isUpstreamError uses == not errors.Is | HIGH | 5m | Test: upstream error returns 502 |
| DAT-2 | Drug info returns null for safety fields | HIGH | 15m | Test: empty slices not null |
| SEC-2 | No body size limit on POST /interactions | MEDIUM | 5m | Test: oversized body rejected |
| SEC-3 | Wildcard CORS when no origins configured | HIGH | 30m | Test: require explicit "*" for wildcard |
| CFG-1 | Indexer ignores CACHE_TTL env var | MEDIUM | 15m | Test: indexer uses configured TTL |

## Dependencies & Serialization

All bugs are independent. Fix in severity order: SEC-1 → DAT-1 → ERR-1 → DAT-2 → SEC-2 → SEC-3 → CFG-1.

## Execution Plan

### SEC-1: Admin auth when ADMIN_SECRET unset (~30m)
- Read `internal/middleware/admin.go`
- Fix: if adminSecret is empty string, reject ALL requests (not pass-through)
- Add test: empty secret → 401 on admin endpoints

### DAT-1: SPL pagination total_pages (~5m)
- Read SPL search handler — find where pagination is calculated
- Fix: compute total_pages from total entries / limit
- Add/fix test for pagination metadata

### ERR-1: Autocomplete error matching (~5m)
- Replace `isUpstreamError()` in autocomplete handler with `errors.Is(err, client.ErrUpstream)`
- Remove the fragile `isUpstreamError` helper
- Verify existing test covers 502 path

### DAT-2: Null safety fields (~15m)
- Find where DrugInfoResponse is built with nil sections
- Initialize empty slices for contraindications, warnings, adverse_reactions when nil
- Test: JSON output has `[]` not `null`

### SEC-2: Body size limit on POST /interactions (~5m)
- Add `http.MaxBytesReader` wrapper in the handler or middleware
- Limit to reasonable size (e.g., 1MB)
- Test: oversized body returns 413 or 400

### SEC-3: CORS wildcard requires explicit "*" (~30m)
- Modify CORS middleware: empty origins list = deny (not wildcard)
- Only allow wildcard if origins contains literal `"*"`
- Update existing staging keys if needed
- Test: empty origins → deny, `["*"]` → allow all

### CFG-1: Indexer uses CacheTTL (~15m)
- Read indexer code, find hardcoded TTL
- Wire to service.CacheTTL
- Test: indexer respects configured TTL

## Validation Criteria

- [ ] All 7 Tier 1 bugs fixed with tests
- [ ] All existing tests pass (no regressions)
- [ ] Coverage stays above 80%
- [ ] go vet clean
- [ ] PR created

## Agent Autonomy (Away Mode)

**Autonomous:** Fix bugs, write tests, commit per fix, push, create PR.
**Boundaries:** Do NOT merge. Do NOT deploy. If a fix has wider impact than expected, document and continue.

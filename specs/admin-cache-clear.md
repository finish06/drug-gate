# Spec: Admin Cache Clear

**Version:** 0.1.0
**Created:** 2026-03-16
**PRD Reference:** docs/prd.md
**Status:** Complete

## 1. Overview

An admin endpoint to clear the Redis cache, either entirely or by key prefix. This allows operators to force a fresh data fetch from upstream cash-drugs without restarting the service or manually running `redis-cli`. Useful after upstream data updates, configuration changes, or when debugging stale cache issues.

### User Story

As an **operator/admin**, I want to **clear the drug-gate Redis cache via API**, so that **I can force fresh data from upstream without restarting the service or SSHing into the host**.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | `DELETE /admin/cache` deletes all cache keys matching `cache:*` | Must |
| AC-002 | `DELETE /admin/cache?prefix={prefix}` deletes only keys matching `cache:{prefix}*` | Must |
| AC-003 | Response includes `status` ("ok") and `keys_deleted` count | Must |
| AC-004 | Endpoint is protected by AdminAuth middleware (Bearer token required) | Must |
| AC-005 | Missing or invalid Bearer token returns 401 | Must |
| AC-006 | Empty prefix (no `?prefix` param) clears all `cache:*` keys | Must |
| AC-007 | Prefix that matches no keys returns 200 with `keys_deleted: 0` (not an error) | Should |
| AC-008 | Non-cache keys (API key data, rate limit data) are NOT deleted | Must |
| AC-009 | Operation is logged with slog (prefix used, keys deleted count) | Should |

## 3. User Test Cases

### TC-001: Clear all cache keys

**Precondition:** Redis has cached data (e.g., `cache:drugnames`, `cache:drugclasses`, `cache:rxnorm:search:lipitor`)
**Steps:**
1. Send `DELETE /admin/cache` with valid Bearer token
2. Observe response
**Expected Result:** 200 OK with `{"status": "ok", "keys_deleted": N}` where N > 0. Subsequent drug/RxNorm requests trigger fresh upstream fetches.
**Maps to:** TBD

### TC-002: Clear cache by prefix

**Precondition:** Redis has cached RxNorm data (`cache:rxnorm:search:lipitor`, `cache:rxnorm:ndcs:153165`)
**Steps:**
1. Send `DELETE /admin/cache?prefix=rxnorm` with valid Bearer token
2. Observe response
**Expected Result:** 200 OK with `{"status": "ok", "keys_deleted": N}` where only RxNorm keys were deleted. Drug name/class caches remain.
**Maps to:** TBD

### TC-003: Clear with prefix matching nothing

**Steps:**
1. Send `DELETE /admin/cache?prefix=nonexistent` with valid Bearer token
2. Observe response
**Expected Result:** 200 OK with `{"status": "ok", "keys_deleted": 0}`
**Maps to:** TBD

### TC-004: No auth token

**Steps:**
1. Send `DELETE /admin/cache` without Authorization header
2. Observe response
**Expected Result:** 401 Unauthorized
**Maps to:** TBD

### TC-005: Verify non-cache keys are preserved

**Precondition:** Redis has API key data (`apikey:pk_...`) and cache data (`cache:drugnames`)
**Steps:**
1. Send `DELETE /admin/cache` with valid Bearer token
2. Check that API key data still exists in Redis
**Expected Result:** API keys and rate limit data are unaffected. Only `cache:*` keys are deleted.
**Maps to:** TBD

## 4. Data Model

### CacheClearResponse

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| status | string | Yes | Always "ok" on success |
| keys_deleted | int | Yes | Number of Redis keys deleted |

## 5. API Contract

### DELETE /admin/cache

**Description:** Clear the Redis cache. Deletes all keys matching `cache:*`, or a subset matching `cache:{prefix}*` if the `prefix` query parameter is provided.

**Query Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| prefix | string | No | (none) | Key prefix filter. When set, only deletes keys matching `cache:{prefix}*`. When omitted, deletes all `cache:*` keys. |

**Response (200):**
```json
{
  "status": "ok",
  "keys_deleted": 42
}
```

**Error Responses:**

- `401` — Missing or invalid admin Bearer token (handled by AdminAuth middleware)

### Redis Key Patterns

| Prefix value | Redis SCAN pattern | What gets deleted |
|-------------|-------------------|-------------------|
| *(empty/omitted)* | `cache:*` | All cached data (drug names, classes, RxNorm, etc.) |
| `drugnames` | `cache:drugnames*` | Drug names cache |
| `drugclasses` | `cache:drugclasses*` | Drug classes cache |
| `drugsbyclass` | `cache:drugsbyclass:*` | Drugs-by-class caches |
| `rxnorm` | `cache:rxnorm:*` | All RxNorm caches (search, NDCs, generics, related, profiles) |
| `rxnorm:search` | `cache:rxnorm:search:*` | Only RxNorm search caches |
| `rxnorm:profile` | `cache:rxnorm:profile:*` | Only RxNorm profile caches |

### Keys that are NEVER deleted

| Key pattern | Purpose |
|-------------|---------|
| `apikey:*` | API key metadata |
| `ratelimit:*` | Rate limit sliding windows |

## 6. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Empty Redis (no cache keys) | Return `{"status": "ok", "keys_deleted": 0}` |
| Very large cache (thousands of keys) | Use SCAN-based deletion (not KEYS *) to avoid blocking Redis |
| Prefix with special characters | URL-decode and use as literal prefix |
| Concurrent cache clear + cache write | Acceptable race condition — new writes may land immediately after clear |
| Redis unavailable | Return 502 with upstream_error |

## 7. Dependencies

- **Redis** — uses SCAN + DEL for key deletion
- **AdminAuth middleware** — existing Bearer token auth on `/admin` routes

## 8. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-16 | 0.1.0 | calebdunn | Initial spec from /add:spec interview |

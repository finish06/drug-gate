# Spec: Security & Rate Limiting

**Version:** 0.1.0
**Created:** 2026-03-08
**PRD Reference:** docs/prd.md
**Status:** Approved

## 1. Overview

Publishable API key authentication, CORS origin locking, and per-key rate limiting for drug-gate. Keys are designed to be embedded in frontend JavaScript (like Google Maps API keys) — the data served is public FDA/DailyMed information, so the threat model protects uptime and prevents abuse, not secrets. Keys can also be origin-free for backend/server-to-server use. Key management is via an admin API protected by a static master secret.

### User Story

As a frontend developer integrating with drug-gate, I want my application identified by an API key with origin locking and rate limiting, so that the service is protected from abuse while my legitimate requests are served reliably.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | Requests without `X-API-Key` header return 401 with ErrorResponse | Must |
| AC-002 | Requests with an unknown/invalid API key return 401 with ErrorResponse | Must |
| AC-003 | Valid API key passes authentication and request proceeds to handler | Must |
| AC-004 | Origin-locked key: request from allowed origin passes CORS check | Must |
| AC-005 | Origin-locked key: request from disallowed origin is rejected via CORS | Must |
| AC-006 | Origin-free key (no origins configured): request passes regardless of Origin header | Must |
| AC-007 | Rate limit enforced per API key — requests over 250/min receive 429 | Must |
| AC-008 | 429 response includes `Retry-After` header with seconds until reset | Must |
| AC-009 | Successful responses include `X-RateLimit-Remaining` and `X-RateLimit-Reset` headers | Must |
| AC-010 | API key metadata stored in Redis: app name, allowed origins, rate limit, active flag | Must |
| AC-011 | `POST /admin/keys` creates a new API key and returns it with metadata | Must |
| AC-012 | `DELETE /admin/keys/{key}` deactivates a key (marks inactive, not deleted) | Must |
| AC-013 | `POST /admin/keys/{key}/rotate` generates a new key; old key remains valid for grace period (configurable, default 2 hours) | Must |
| AC-014 | Admin endpoints require `Authorization: Bearer <ADMIN_SECRET>` header | Must |
| AC-015 | Admin endpoints return 401 if ADMIN_SECRET is missing or wrong | Must |
| AC-016 | `GET /admin/keys` lists all API keys with metadata (excludes deactivated by default) | Should |
| AC-017 | `GET /admin/keys/{key}` returns metadata for a single key | Should |
| AC-018 | Rate limit tier is configurable per key (default 250 req/min) | Should |
| AC-019 | Health endpoint (`/health`) and Swagger endpoints (`/swagger/*`, `/openapi.json`) are exempt from API key auth | Must |
| AC-020 | Sliding window rate limiting (not fixed window) to prevent burst-at-boundary abuse | Should |

## 3. User Test Cases

### TC-001: Frontend app makes authenticated request

**Precondition:** API key `pk_test123` exists in Redis with origin `https://myapp.com` and rate tier 250/min
**Steps:**
1. Send `GET /v1/drugs/ndc/00069-3150` with `X-API-Key: pk_test123` and `Origin: https://myapp.com`
2. Observe response status and headers
**Expected Result:** 200 response with drug data, `X-RateLimit-Remaining: 249`, `X-RateLimit-Reset` present, CORS `Access-Control-Allow-Origin: https://myapp.com`
**Screenshot Checkpoint:** N/A (API only)
**Maps to:** TBD

### TC-002: Request without API key

**Precondition:** None
**Steps:**
1. Send `GET /v1/drugs/ndc/00069-3150` without `X-API-Key` header
**Expected Result:** 401 `{"error": "unauthorized", "message": "API key required"}`
**Screenshot Checkpoint:** N/A
**Maps to:** TBD

### TC-003: Request from wrong origin

**Precondition:** API key `pk_test123` exists with origin `https://myapp.com`
**Steps:**
1. Send `GET /v1/drugs/ndc/00069-3150` with `X-API-Key: pk_test123` and `Origin: https://evil.com`
**Expected Result:** CORS rejection — no `Access-Control-Allow-Origin` header, request blocked by browser
**Screenshot Checkpoint:** N/A
**Maps to:** TBD

### TC-004: Rate limit exceeded

**Precondition:** API key `pk_test123` exists with rate tier 5/min (low for testing)
**Steps:**
1. Send 5 requests with valid key — all succeed
2. Send 6th request
**Expected Result:** 429 `{"error": "rate_limited", "message": "Rate limit exceeded"}` with `Retry-After` header
**Screenshot Checkpoint:** N/A
**Maps to:** TBD

### TC-005: Admin creates a new API key

**Precondition:** `ADMIN_SECRET=supersecret` set in environment
**Steps:**
1. Send `POST /admin/keys` with `Authorization: Bearer supersecret` and body `{"app_name": "my-frontend", "origins": ["https://myapp.com"], "rate_limit": 250}`
2. Observe response
**Expected Result:** 201 with `{"key": "pk_...", "app_name": "my-frontend", "origins": ["https://myapp.com"], "rate_limit": 250, "active": true}`
**Screenshot Checkpoint:** N/A
**Maps to:** TBD

### TC-006: Key rotation with grace period

**Precondition:** API key `pk_old` exists and is active
**Steps:**
1. Send `POST /admin/keys/pk_old/rotate` with admin auth
2. Receive new key `pk_new` in response
3. Use `pk_old` — should still work (grace period)
4. Wait for grace period to expire
5. Use `pk_old` — should return 401
**Expected Result:** Both keys work during grace period; old key expires after configured duration (default 2 hours)
**Screenshot Checkpoint:** N/A
**Maps to:** TBD

### TC-007: Backend key without origin restriction

**Precondition:** API key `pk_backend` exists with empty origins list
**Steps:**
1. Send `GET /v1/drugs/ndc/00069-3150` with `X-API-Key: pk_backend` and no Origin header
2. Send same request with `Origin: https://anything.com`
**Expected Result:** Both requests succeed — origin-free keys accept all origins
**Screenshot Checkpoint:** N/A
**Maps to:** TBD

## 4. Data Model

### APIKey (stored in Redis)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| key | string | Yes | The API key value (e.g., `pk_a1b2c3d4e5f6`) |
| app_name | string | Yes | Human-readable application name |
| origins | []string | No | Allowed CORS origins; empty = origin-free (backend key) |
| rate_limit | int | Yes | Max requests per minute (default 250) |
| active | bool | Yes | Whether the key is currently valid |
| created_at | timestamp | Yes | When the key was created |
| expires_at | timestamp | No | When a rotated-out key becomes invalid (grace period) |

### Redis Key Schema

| Redis Key | Type | Value |
|-----------|------|-------|
| `apikey:{key}` | Hash | APIKey fields |
| `ratelimit:{key}:{window}` | Sorted Set or String | Sliding window counter |

### Relationships

- Each API key has zero or more allowed origins
- Rate limit counters are per-key, stored alongside key metadata in Redis
- Rotated keys share the same `app_name`; old key gets `expires_at` set

## 5. API Contract

### POST /admin/keys

**Description:** Create a new API key

**Request:**
```json
{
  "app_name": "my-frontend-app",
  "origins": ["https://myapp.com", "https://staging.myapp.com"],
  "rate_limit": 250
}
```

**Response (201):**
```json
{
  "key": "pk_a1b2c3d4e5f6",
  "app_name": "my-frontend-app",
  "origins": ["https://myapp.com", "https://staging.myapp.com"],
  "rate_limit": 250,
  "active": true,
  "created_at": "2026-03-08T12:00:00Z"
}
```

**Error Responses:**
- `400` — Missing required fields (app_name)
- `401` — Missing or invalid admin secret

### GET /admin/keys

**Description:** List all active API keys

**Response (200):**
```json
{
  "keys": [
    {
      "key": "pk_a1b2c3d4e5f6",
      "app_name": "my-frontend-app",
      "origins": ["https://myapp.com"],
      "rate_limit": 250,
      "active": true,
      "created_at": "2026-03-08T12:00:00Z"
    }
  ]
}
```

### GET /admin/keys/{key}

**Description:** Get metadata for a single key

**Response (200):**
```json
{
  "key": "pk_a1b2c3d4e5f6",
  "app_name": "my-frontend-app",
  "origins": ["https://myapp.com"],
  "rate_limit": 250,
  "active": true,
  "created_at": "2026-03-08T12:00:00Z"
}
```

**Error Responses:**
- `401` — Missing or invalid admin secret
- `404` — Key not found

### DELETE /admin/keys/{key}

**Description:** Deactivate an API key (soft delete — marks inactive)

**Response (200):**
```json
{
  "key": "pk_a1b2c3d4e5f6",
  "active": false,
  "message": "Key deactivated"
}
```

**Error Responses:**
- `401` — Missing or invalid admin secret
- `404` — Key not found

### POST /admin/keys/{key}/rotate

**Description:** Generate a replacement key. Old key remains valid for grace period.

**Request:**
```json
{
  "grace_period_hours": 2
}
```

**Response (200):**
```json
{
  "old_key": "pk_a1b2c3d4e5f6",
  "new_key": "pk_x9y8z7w6v5u4",
  "old_key_expires_at": "2026-03-08T14:00:00Z",
  "message": "New key active. Old key valid until expiry."
}
```

**Error Responses:**
- `401` — Missing or invalid admin secret
- `404` — Key not found

## 6. UI Behavior

N/A — API-only feature. No UI components.

## 7. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| API key exists but is inactive (deactivated) | 401 unauthorized |
| API key is in grace period after rotation | Request succeeds with old key until expiry |
| API key grace period has expired | 401 unauthorized |
| Redis is unreachable | 502 with error message (fail closed — deny requests) |
| CORS preflight (OPTIONS) request | Respond with appropriate CORS headers without requiring API key |
| Rate limit window rolls over | Counter resets, requests succeed again |
| Admin creates key with rate_limit=0 | Reject with 400 — rate limit must be positive |
| Admin creates key with empty app_name | Reject with 400 — app_name required |
| Multiple origins on one key | All listed origins are allowed via CORS |
| Request with Origin header but origin-free key | Passes — CORS allows any origin |

## 8. Dependencies

- **Redis** — required for key storage and rate limit counters (already in docker-compose)
- **M1: NDC Lookup** — existing handlers must be wrapped with auth middleware
- **crypto/rand** — for generating secure random API key values
- **ADMIN_SECRET env var** — must be set for admin endpoints to function

## 9. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-08 | 0.1.0 | calebdunn | Initial spec from /add:spec interview |

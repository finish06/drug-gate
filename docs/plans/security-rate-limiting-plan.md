# Implementation Plan: Security & Rate Limiting

**Spec Version**: 0.1.0
**Spec**: specs/security-rate-limiting.md
**Created**: 2026-03-08
**Team Size**: Solo (agent-assisted)
**Estimated Duration**: 2-3 days

## Overview

Add publishable API key authentication, CORS origin locking, per-key sliding window rate limiting via Redis, and admin key management endpoints to drug-gate. Public endpoints (health, swagger) remain open.

## Objectives

- Protect drug lookup endpoints with API key authentication
- Enforce per-key rate limits (250 req/min default) via Redis
- Support origin-locked (frontend) and origin-free (backend) keys
- Provide admin API for key CRUD and rotation with grace period
- Keep health/swagger endpoints exempt from auth

## Success Criteria

- All 20 acceptance criteria implemented and tested
- Code coverage >= 80%
- All quality gates passing
- Redis integration tested with real Redis (docker-compose)
- Existing M1 tests unaffected

## Implementation Phases

### Phase 1: Foundation — Redis Client & API Key Model (TDD)

Core data structures and Redis storage layer. Everything else depends on this.

| Task ID | Description | ACs | Effort | Dependencies |
|---------|-------------|-----|--------|--------------|
| TASK-001 | Define `APIKey` model struct with JSON/Redis serialization | AC-010 | 30min | None |
| TASK-002 | Create `internal/apikey` package with `Store` interface (Create, Get, List, Deactivate, Rotate) | AC-010 | 1h | TASK-001 |
| TASK-003 | Implement `RedisStore` — Redis-backed implementation of `Store` | AC-010 | 2h | TASK-002 |
| TASK-004 | Key generation — `pk_` prefix + 24 chars from crypto/rand | AC-011 | 30min | TASK-001 |
| TASK-005 | Write unit tests for model + key generation (mock store) | AC-010, AC-011 | 1h | TASK-001, TASK-004 |
| TASK-006 | Write integration tests for RedisStore (requires Redis) | AC-010 | 1.5h | TASK-003 |

**Phase Duration**: 0.5 day
**Blockers**: Redis must be running (docker-compose)

### Phase 2: Auth Middleware (TDD)

API key validation middleware that wraps protected routes.

| Task ID | Description | ACs | Effort | Dependencies |
|---------|-------------|-----|--------|--------------|
| TASK-007 | Write failing tests: missing key → 401, invalid key → 401, valid key → passes | AC-001, AC-002, AC-003 | 1h | TASK-002 |
| TASK-008 | Write failing tests: inactive key → 401, expired grace period key → 401 | AC-012, AC-013 | 30min | TASK-002 |
| TASK-009 | Implement `middleware/auth.go` — APIKeyAuth middleware using Store interface | AC-001, AC-002, AC-003 | 1.5h | TASK-007 |
| TASK-010 | Write failing test: health/swagger exempt from auth | AC-019 | 30min | TASK-009 |
| TASK-011 | Wire auth middleware into router (protect /v1/* only) | AC-019 | 30min | TASK-009 |

**Phase Duration**: 0.5 day
**Blockers**: Phase 1 Store interface

### Phase 3: CORS Middleware (TDD)

Per-key origin validation using the key's allowed origins.

| Task ID | Description | ACs | Effort | Dependencies |
|---------|-------------|-----|--------|--------------|
| TASK-012 | Write failing tests: origin-locked key + allowed origin → passes | AC-004 | 30min | TASK-009 |
| TASK-013 | Write failing tests: origin-locked key + wrong origin → CORS rejection | AC-005 | 30min | TASK-009 |
| TASK-014 | Write failing tests: origin-free key → any origin passes | AC-006 | 30min | TASK-009 |
| TASK-015 | Write failing test: CORS preflight (OPTIONS) works without API key | AC-019 | 30min | TASK-009 |
| TASK-016 | Implement `middleware/cors.go` — per-key CORS based on key metadata | AC-004, AC-005, AC-006 | 1.5h | TASK-012 through TASK-015 |

**Phase Duration**: 0.5 day
**Blockers**: Phase 2 auth middleware (CORS needs key context)

### Phase 4: Rate Limiting (TDD)

Sliding window rate limiter backed by Redis.

| Task ID | Description | ACs | Effort | Dependencies |
|---------|-------------|-----|--------|--------------|
| TASK-017 | Define `RateLimiter` interface (Allow, Remaining, Reset) | AC-007 | 30min | None |
| TASK-018 | Write failing tests: under limit → passes, over limit → 429 | AC-007, AC-008 | 1h | TASK-017 |
| TASK-019 | Write failing tests: response includes X-RateLimit-Remaining + X-RateLimit-Reset | AC-009 | 30min | TASK-017 |
| TASK-020 | Write failing test: Retry-After header on 429 | AC-008 | 15min | TASK-017 |
| TASK-021 | Implement `internal/ratelimit` package — sliding window using Redis sorted sets | AC-007, AC-020 | 2h | TASK-017 |
| TASK-022 | Implement `middleware/ratelimit.go` — rate limit middleware with headers | AC-007, AC-008, AC-009 | 1.5h | TASK-021 |
| TASK-023 | Write integration tests for rate limiter with Redis | AC-007, AC-020 | 1h | TASK-021 |

**Phase Duration**: 0.5 day
**Blockers**: Redis, Phase 2 auth (rate limit keyed by API key)

### Phase 5: Admin API (TDD)

Key management endpoints protected by admin secret.

| Task ID | Description | ACs | Effort | Dependencies |
|---------|-------------|-----|--------|--------------|
| TASK-024 | Write failing tests: admin auth with Bearer secret | AC-014, AC-015 | 30min | None |
| TASK-025 | Implement `middleware/admin.go` — admin secret auth | AC-014, AC-015 | 30min | TASK-024 |
| TASK-026 | Write failing tests: POST /admin/keys creates key | AC-011 | 1h | TASK-002 |
| TASK-027 | Write failing tests: GET /admin/keys lists keys | AC-016 | 30min | TASK-002 |
| TASK-028 | Write failing tests: GET /admin/keys/{key} returns metadata | AC-017 | 30min | TASK-002 |
| TASK-029 | Write failing tests: DELETE /admin/keys/{key} deactivates | AC-012 | 30min | TASK-002 |
| TASK-030 | Write failing tests: POST /admin/keys/{key}/rotate with grace period | AC-013 | 1h | TASK-002 |
| TASK-031 | Implement `internal/handler/admin.go` — all admin endpoints | AC-011, AC-012, AC-013, AC-016, AC-017 | 3h | TASK-026 through TASK-030 |
| TASK-032 | Write edge case tests: empty app_name → 400, rate_limit=0 → 400 | AC-011 | 30min | TASK-031 |

**Phase Duration**: 1 day
**Blockers**: Phase 1 Store

### Phase 6: Integration & Wiring (TDD)

Wire everything together in main.go, update swagger annotations, update docker-compose.

| Task ID | Description | ACs | Effort | Dependencies |
|---------|-------------|-----|--------|--------------|
| TASK-033 | Add Redis client initialization to main.go | AC-010 | 30min | Phase 1 |
| TASK-034 | Wire middleware chain: logger → auth → cors → ratelimit → handler | AC-001 through AC-009 | 1h | Phases 2-4 |
| TASK-035 | Add admin routes under /admin/* with admin auth | AC-014 | 30min | Phase 5 |
| TASK-036 | Add ADMIN_SECRET env var handling | AC-014 | 15min | TASK-025 |
| TASK-037 | Update docker-compose.yml — ensure Redis is available | AC-010 | 15min | None |
| TASK-038 | Update docker-compose.prod.yml — add ADMIN_SECRET env | AC-014 | 15min | None |
| TASK-039 | Add swaggo annotations to all new handlers + middleware responses | — | 1h | TASK-031 |
| TASK-040 | Regenerate swagger docs (`make swagger`) | — | 15min | TASK-039 |
| TASK-041 | Run full test suite — verify M1 tests still pass | — | 15min | All |
| TASK-042 | Update sequence diagram (`docs/sequence-diagram.md`) | — | 30min | TASK-034 |

**Phase Duration**: 0.5 day
**Blockers**: All phases complete

### Phase 7: Verify & Polish

| Task ID | Description | ACs | Effort | Dependencies |
|---------|-------------|-----|--------|--------------|
| TASK-043 | Run /add:verify — all quality gates | All | 15min | TASK-041 |
| TASK-044 | Fix any lint/coverage issues | — | 30min | TASK-043 |
| TASK-045 | Update CHANGELOG.md | — | 15min | TASK-043 |
| TASK-046 | Update specs/security-rate-limiting.md status → Complete | — | 5min | TASK-043 |

**Phase Duration**: 0.5 day

## Effort Summary

| Phase | Description | Estimated Hours |
|-------|-------------|-----------------|
| Phase 1 | Foundation — Redis & Model | 6.5h |
| Phase 2 | Auth Middleware | 4h |
| Phase 3 | CORS Middleware | 3.5h |
| Phase 4 | Rate Limiting | 6.75h |
| Phase 5 | Admin API | 8h |
| Phase 6 | Integration & Wiring | 4.5h |
| Phase 7 | Verify & Polish | 1h |
| **Total** | | **~34h** |

**Solo timeline**: ~3 days of focused work

## Architecture Decisions

### Package Structure (new files)

```
internal/
  apikey/
    model.go          — APIKey struct, key generation
    store.go          — Store interface
    redis_store.go    — Redis implementation
    redis_store_test.go
    model_test.go
  ratelimit/
    limiter.go        — RateLimiter interface
    redis_limiter.go  — Sliding window Redis implementation
    redis_limiter_test.go
  middleware/
    auth.go           — API key auth middleware
    auth_test.go
    cors.go           — Per-key CORS middleware
    cors_test.go
    ratelimit.go      — Rate limit middleware
    ratelimit_test.go
    admin.go          — Admin secret auth middleware
    admin_test.go
  handler/
    admin.go          — Admin key management handlers
    admin_test.go
```

### Middleware Chain Order

```
RequestLogger → APIKeyAuth → PerKeyCORS → RateLimit → Handler
```

- Logger wraps everything (logs all requests including auth failures)
- Auth runs first — no point checking CORS/rate if key is invalid
- CORS runs after auth — needs key metadata to check origins
- Rate limit runs after CORS — only count valid, authorized requests

### Key Context Passing

Auth middleware stores the validated `APIKey` in request context. CORS and rate limit middleware retrieve it:

```go
type contextKey string
const APIKeyContextKey contextKey = "apikey"

// In auth middleware:
ctx := context.WithValue(r.Context(), APIKeyContextKey, key)

// In CORS/ratelimit middleware:
key := r.Context().Value(APIKeyContextKey).(*apikey.APIKey)
```

### Sliding Window Rate Limiting

Using Redis sorted sets for true sliding window (not fixed window):

```
Key: ratelimit:{api_key}
Score: unix timestamp (microseconds)
Member: unique request ID

ZREMRANGEBYSCORE to prune old entries
ZCARD to count current window
ZADD to record new request
```

### Route Groups

```go
// Public (no auth)
r.Get("/health", handler.HealthCheck)
r.Get("/swagger/*", httpSwagger.WrapHandler)
r.Get("/openapi.json", handler.OpenAPIJSON)

// Protected (API key + CORS + rate limit)
r.Group(func(r chi.Router) {
    r.Use(middleware.APIKeyAuth(store))
    r.Use(middleware.PerKeyCORS)
    r.Use(middleware.RateLimit(limiter))
    r.Get("/v1/drugs/ndc/{ndc}", drugHandler.HandleNDCLookup)
})

// Admin (admin secret)
r.Route("/admin", func(r chi.Router) {
    r.Use(middleware.AdminAuth(adminSecret))
    r.Post("/keys", adminHandler.CreateKey)
    r.Get("/keys", adminHandler.ListKeys)
    r.Get("/keys/{key}", adminHandler.GetKey)
    r.Delete("/keys/{key}", adminHandler.DeactivateKey)
    r.Post("/keys/{key}/rotate", adminHandler.RotateKey)
})
```

## Dependencies

### External
- **Redis** — already in docker-compose, needs `github.com/redis/go-redis/v9`
- **crypto/rand** — stdlib, for key generation

### Internal
- M1 complete (handler, middleware, model packages exist)
- Existing ErrorResponse model reused for auth/rate limit errors

## Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|-----------|
| Redis connection issues in tests | Medium | Medium | Use testcontainers or require docker-compose for integration tests; unit tests use mock store |
| Middleware ordering bugs | Medium | High | Test each middleware in isolation AND in combination; integration test full chain |
| Race conditions in rate limiter | Low | High | Redis atomic operations (MULTI/EXEC or Lua script); test concurrent access |
| Grace period expiry precision | Low | Low | Use Redis TTL for automatic cleanup; test with short grace periods |
| CORS preflight complexity | Medium | Medium | Test OPTIONS requests explicitly; follow spec strictly |

## Testing Strategy

1. **Unit Tests** — mock Store interface for all middleware/handler tests
2. **Integration Tests** — real Redis for Store and RateLimiter (tagged, require docker-compose)
3. **Full Chain Tests** — test router with all middleware wired together
4. **Coverage Target** — 80%+ on all new packages

## Deliverables

### Code
- `internal/apikey/` — model, store interface, Redis implementation
- `internal/ratelimit/` — limiter interface, Redis sliding window
- `internal/middleware/auth.go` — API key auth
- `internal/middleware/cors.go` — per-key CORS
- `internal/middleware/ratelimit.go` — rate limit with headers
- `internal/middleware/admin.go` — admin secret auth
- `internal/handler/admin.go` — admin CRUD + rotate

### Tests
- Unit tests for each package (mock-based)
- Integration tests for Redis-backed components
- Full middleware chain tests

### Config
- Updated `docker-compose.yml` and `docker-compose.prod.yml`
- Updated swagger docs
- Updated sequence diagram

## TDD Execution Order

The TDD cycle should follow this order for clean dependency flow:

1. `apikey` model + store interface + mock (RED → GREEN)
2. `apikey` Redis store (RED → GREEN with integration tests)
3. Auth middleware (RED → GREEN with mock store)
4. CORS middleware (RED → GREEN with mock store)
5. `ratelimit` interface + Redis implementation (RED → GREEN)
6. Rate limit middleware (RED → GREEN with mock limiter)
7. Admin auth middleware (RED → GREEN)
8. Admin handlers (RED → GREEN with mock store)
9. Wire everything in main.go
10. Full chain integration test
11. REFACTOR pass
12. VERIFY

## Plan History

- 2026-03-08: Initial plan created from spec v0.1.0

# Implementation Plan: Admin Cache Clear

**Spec:** specs/admin-cache-clear.md v0.1.0
**Created:** 2026-03-16
**Team Size:** Solo
**Estimated Duration:** 1 TDD cycle (~2-3 hours)

## Overview

Add `DELETE /admin/cache` endpoint to clear Redis cache keys by prefix. Uses SCAN-based deletion to avoid blocking Redis. Protected by existing AdminAuth middleware.

## Implementation Phases

### Phase 1: Handler + Redis logic (RED → GREEN)

| Task ID | Description | AC | Effort |
|---------|-------------|-----|--------|
| TASK-001 | Add `ClearCache` method to `AdminHandler` — SCAN for `cache:*` or `cache:{prefix}*`, DEL matched keys, return count | AC-001, AC-002, AC-006, AC-008 | 45m |
| TASK-002 | Add Swagger annotation to `ClearCache` | — | 10m |
| TASK-003 | Register `DELETE /admin/cache` route in `cmd/server/main.go` | AC-004 | 5m |
| TASK-004 | Add slog info line on cache clear (prefix, keys deleted) | AC-009 | 5m |
| TASK-005 | Write handler unit tests (mock Redis): happy path all, happy path prefix, no matches, missing auth | AC-001–007 | 1h |
| TASK-006 | Write integration test (real Redis): populate keys, clear by prefix, verify non-cache keys preserved | AC-008 | 30m |

**Phase effort:** ~2.5h

### Phase 2: Docs + verify

| Task ID | Description | Effort |
|---------|-------------|--------|
| TASK-007 | Regenerate Swagger docs | 5m |
| TASK-008 | Update sequence diagram route table | 10m |
| TASK-009 | Update CLAUDE.md + README.md with new endpoint | 10m |
| TASK-010 | Run full test suite + coverage check | 10m |

**Phase effort:** ~35m

## Effort Summary

| Phase | Estimated |
|-------|-----------|
| Phase 1 | 2.5h |
| Phase 2 | 35m |
| **Total** | **~3h** |

## Key Design Decision

Use Redis `SCAN` (not `KEYS`) to find matching keys, then `DEL` in batches. SCAN is non-blocking and safe for production Redis. The handler needs direct Redis access — either inject `*redis.Client` into AdminHandler or add a method to an existing service.

Simplest approach: add a `ClearCache(ctx, pattern)` method to the existing `AdminHandler` by giving it a `*redis.Client` field, or create a standalone handler function that takes Redis as a dependency. Since AdminHandler already has the Redis-backed `apikey.Store`, adding Redis directly is consistent.

## Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| SCAN on large Redis blocks event loop | Low | Low | SCAN is non-blocking by design, batches of 100 |
| Accidentally deleting non-cache keys | Low | High | Pattern always prefixed with `cache:` — tested in TC-005 |

## File Changes

| File | Change |
|------|--------|
| `internal/handler/admin.go` | Add `ClearCache` method |
| `internal/handler/admin_test.go` | Add cache clear tests |
| `cmd/server/main.go` | Register `DELETE /admin/cache` |

## Next Steps

1. Run `/add:tdd-cycle specs/admin-cache-clear.md`

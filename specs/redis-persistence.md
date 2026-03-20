# Spec: Redis Persistence + Key Backup

**Version:** 0.1.0
**Created:** 2026-03-20
**PRD Reference:** docs/prd.md — M7: Operational Hardening
**Status:** Approved

## 1. Overview

Enable Redis persistence to survive restarts without losing cached data and API keys. Configure AOF (Append Only File) for durability and nightly RDB snapshots for backup. Document restore procedures for local, staging, and future production environments.

### User Story

As an **operator**, I want **Redis data to survive container restarts and have nightly backups**, so that **API keys, rate limit state, and cached drug data aren't lost during maintenance or unexpected outages**.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | Redis in docker-compose uses AOF persistence (`appendonly yes`) | Must |
| AC-002 | Redis data directory is mounted to a named volume in docker-compose | Must |
| AC-003 | AOF fsync policy is `everysec` (default, good balance of durability vs performance) | Must |
| AC-004 | Staging Redis configuration documented with AOF enable commands | Must |
| AC-005 | Production Redis configuration documented (for future setup) | Must |
| AC-006 | Nightly backup cron example documented (BGSAVE + copy RDB) | Must |
| AC-007 | Restore procedure documented for each environment (local, staging, production) | Must |
| AC-008 | Redis data survives `docker-compose down && docker-compose up` | Must |
| AC-009 | Health check still works after persistence is enabled | Must |

## 3. User Test Cases

### TC-001: Data survives restart

**Precondition:** drug-gate and Redis running via docker-compose
**Steps:**
1. Create an API key via admin endpoint
2. Run `docker-compose down`
3. Run `docker-compose up`
4. Query with the API key
**Expected Result:** API key still valid, request succeeds
**Maps to:** AC-001, AC-002, AC-008

### TC-002: AOF is active

**Steps:**
1. Run `docker-compose exec redis redis-cli CONFIG GET appendonly`
**Expected Result:** Returns `appendonly` = `yes`
**Maps to:** AC-001, AC-003

## 4. Data Model

No application data model changes. This is infrastructure configuration.

### Redis Persistence Files

| File | Purpose | Location (container) |
|------|---------|---------------------|
| `appendonly.aof` | Append-only file for write durability | `/data/` |
| `dump.rdb` | Point-in-time snapshot for backup | `/data/` |

## 5. API Contract

No API changes. Existing `/health` endpoint continues to check Redis connectivity.

## 6. Implementation Notes

### docker-compose.yml Changes

```yaml
redis:
  image: redis:alpine
  ports:
    - "6379:6379"
  command: redis-server --appendonly yes --appendfsync everysec
  volumes:
    - redis-data:/data

volumes:
  redis-data:
```

### Staging Configuration (192.168.1.145)

Document the Redis CLI commands to enable persistence on the existing staging Redis instance:
```
redis-cli CONFIG SET appendonly yes
redis-cli CONFIG SET appendfsync everysec
redis-cli CONFIG REWRITE
```

### Backup Cron Example

```bash
# /etc/cron.d/redis-backup (staging)
0 2 * * * redis-cli BGSAVE && sleep 5 && cp /var/lib/redis/dump.rdb /backup/redis/dump-$(date +\%Y\%m\%d).rdb
```

### Restore Procedure

1. Stop Redis
2. Replace `/data/appendonly.aof` and/or `/data/dump.rdb` with backup copies
3. Start Redis — it loads AOF first (if exists), then falls back to RDB

## 7. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Large AOF file (many writes) | Redis auto-rewrites AOF when it grows 100% beyond last rewrite size |
| Corrupted AOF | Use `redis-check-aof --fix` before starting Redis |
| No backup exists on restore | Redis starts empty (clean state) |
| Disk full | Redis logs warning but continues serving from memory; writes may fail |

## 8. Dependencies

- Docker Compose v2+ (named volumes)
- Staging host: 192.168.1.145 with Redis access

## 9. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-20 | 0.1.0 | calebdunn | Initial spec |

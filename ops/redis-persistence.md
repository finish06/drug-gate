# Redis Persistence & Backup — Operations Guide

## Overview

drug-gate uses Redis for API key storage, rate limit counters, and cached drug data. Persistence ensures data survives container restarts. Backups enable recovery from data loss.

## Persistence Strategy

| Method | Purpose | Data Loss Window |
|--------|---------|-----------------|
| AOF (Append Only File) | Durability — logs every write | ~1 second (`appendfsync everysec`) |
| RDB Snapshot | Backup — periodic point-in-time snapshots | Since last snapshot |

AOF is the primary persistence mechanism. RDB snapshots are used for backups.

## Local (docker-compose)

Persistence is enabled automatically via `docker-compose.yml`:

```yaml
redis:
  image: redis:alpine
  command: redis-server --appendonly yes --appendfsync everysec
  volumes:
    - redis-data:/data
```

**Verify persistence is active:**
```bash
docker-compose exec redis redis-cli CONFIG GET appendonly
# Should return: appendonly = yes

docker-compose exec redis redis-cli CONFIG GET appendfsync
# Should return: appendfsync = everysec
```

**Data survives restarts:**
```bash
docker-compose down
docker-compose up -d
# API keys and cached data are preserved
```

**Reset data (if needed):**
```bash
docker-compose down -v  # -v removes the named volume
docker-compose up -d    # Fresh start with empty Redis
```

## Staging (192.168.1.145)

### Enable Persistence

Connect to the staging Redis instance and enable AOF:

```bash
redis-cli -h 192.168.1.145 CONFIG SET appendonly yes
redis-cli -h 192.168.1.145 CONFIG SET appendfsync everysec
redis-cli -h 192.168.1.145 CONFIG REWRITE  # Persist config to redis.conf
```

Verify:
```bash
redis-cli -h 192.168.1.145 CONFIG GET appendonly
redis-cli -h 192.168.1.145 INFO persistence
```

### Nightly Backup Cron

Add to staging host's crontab (`crontab -e`):

```cron
# Redis nightly backup — 2:00 AM
0 2 * * * redis-cli -h 192.168.1.145 BGSAVE && sleep 5 && cp /var/lib/redis/dump.rdb /backup/redis/dump-$(date +\%Y\%m\%d).rdb 2>&1 | logger -t redis-backup

# Clean up backups older than 30 days
0 3 * * * find /backup/redis/ -name "dump-*.rdb" -mtime +30 -delete
```

Create the backup directory:
```bash
sudo mkdir -p /backup/redis
sudo chown redis:redis /backup/redis
```

### Verify Backup

```bash
ls -la /backup/redis/
# Should show daily dump files: dump-20260320.rdb, dump-20260321.rdb, ...
```

## Production (Future)

When production Redis is set up:

1. **Enable AOF** with the same commands as staging
2. **Enable RDB snapshots** as a secondary safety net:
   ```
   redis-cli CONFIG SET save "900 1 300 10 60 10000"
   redis-cli CONFIG REWRITE
   ```
   This saves a snapshot if: 900s with 1+ change, 300s with 10+ changes, or 60s with 10000+ changes.
3. **Offsite backups**: Copy nightly RDB snapshots to a separate host or cloud storage
4. **Monitoring**: Add Prometheus alert for `druggate_redis_up == 0` (see ops/prometheus-alerts.md)
5. **Consider Redis Sentinel** for high availability if uptime requirements exceed 99.9%

## Restore Procedure

### From AOF (preferred — minimal data loss)

```bash
# 1. Stop Redis
redis-cli SHUTDOWN NOSAVE
# or: docker-compose stop redis

# 2. Verify AOF file exists
ls -la /data/appendonly.aof  # In container: /data/
# or: ls -la /var/lib/redis/appendonly.aof  # On staging host

# 3. If AOF is corrupted, fix it:
redis-check-aof --fix /data/appendonly.aof

# 4. Start Redis — it loads AOF automatically
docker-compose up -d redis
# or: systemctl start redis
```

### From RDB Snapshot (when AOF is unavailable)

```bash
# 1. Stop Redis
redis-cli SHUTDOWN NOSAVE

# 2. Copy backup RDB to Redis data directory
cp /backup/redis/dump-20260320.rdb /var/lib/redis/dump.rdb
# In docker: docker cp dump.rdb drug-gate-redis-1:/data/dump.rdb

# 3. Remove AOF if present (so Redis loads RDB instead)
rm /var/lib/redis/appendonly.aof  # or /data/appendonly.aof

# 4. Start Redis
docker-compose up -d redis

# 5. Re-enable AOF after restore
redis-cli CONFIG SET appendonly yes
redis-cli CONFIG REWRITE
```

### Verify Restore

```bash
# Check Redis loaded data
redis-cli DBSIZE
# Should show non-zero key count

# Check API keys are present
redis-cli KEYS "apikey:*"

# Test the API
curl -H "X-API-Key: your-key" http://localhost:8081/v1/drugs/autocomplete?q=met
```

## Troubleshooting

| Issue | Diagnosis | Fix |
|-------|-----------|-----|
| AOF file growing too large | `redis-cli INFO persistence` → check `aof_current_size` | Redis auto-rewrites; or manually: `redis-cli BGREWRITEAOF` |
| Slow startup after restart | Large AOF being replayed | Normal — wait for "Ready to accept connections" in logs |
| Disk full | AOF/RDB can't write | Free disk space, then `BGREWRITEAOF` to compact |
| Corrupted AOF | Redis won't start | Run `redis-check-aof --fix`, then restart |
| Missing data after restore | Wrong file restored or AOF took priority | Verify file dates, remove stale AOF if restoring from RDB |

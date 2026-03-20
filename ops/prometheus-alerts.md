# Prometheus Alert Rules — Operations Guide

## Overview

drug-gate ships alert rules in `prometheus/alerts.yml` that monitor key health signals. This guide explains each alert, how to load the rules, and how to respond when they fire.

## Loading Alert Rules

### Prometheus Configuration

Add the rules file to your `prometheus.yml`:

```yaml
rule_files:
  - /path/to/drug-gate/prometheus/alerts.yml

scrape_configs:
  - job_name: drug-gate
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:8081']  # or staging/prod host
```

Reload Prometheus after updating config:
```bash
# Signal reload
kill -HUP $(pidof prometheus)
# or via API (if --web.enable-lifecycle is set)
curl -X POST http://localhost:9090/-/reload
```

### Verify Rules Loaded

```bash
# Check rules via Prometheus API
curl http://localhost:9090/api/v1/rules | jq '.data.groups[].rules[].name'

# Expected output:
# "DrugGateHighErrorRate"
# "DrugGateHighLatency"
# "DrugGateRedisDown"
# "DrugGateHighRateLimitRejections"
```

### Validate Rules File

If you have `promtool` installed:
```bash
promtool check rules prometheus/alerts.yml
```

## Alert Reference

### DrugGateHighErrorRate

| Field | Value |
|-------|-------|
| Severity | warning |
| Condition | HTTP 5xx rate > 5% over 5 minutes |
| Expression | `sum(rate(druggate_http_requests_total{status_code=~"5.."}[5m])) / sum(rate(druggate_http_requests_total[5m])) > 0.05` |

**What it means:** More than 1 in 20 requests is failing with a server error.

**Response:**
1. Check upstream cash-drugs health: `curl http://host1.du.nn:8083/health`
2. Check Redis connectivity: `redis-cli -h <redis-host> ping`
3. Check recent deployments: `git log --oneline -5`
4. Review drug-gate logs for error patterns: `docker logs drug-gate-1 --tail 100 | jq 'select(.level == "ERROR")'`

### DrugGateHighLatency

| Field | Value |
|-------|-------|
| Severity | warning |
| Condition | p95 latency > 500ms over 5 minutes |
| Expression | `histogram_quantile(0.95, sum(rate(druggate_http_request_duration_seconds_bucket[5m])) by (le)) > 0.5` |

**What it means:** The slowest 5% of requests are taking over half a second.

**Response:**
1. Check Redis latency: `redis-cli -h <redis-host> --latency`
2. Check cache hit rate in Prometheus: `druggate_cache_hits_total` by outcome
3. Check upstream cash-drugs latency (may be propagating slowness)
4. If cache hit rate dropped, data may have been evicted — first requests after eviction are slow (upstream fetch)

### DrugGateRedisDown

| Field | Value |
|-------|-------|
| Severity | **critical** |
| Condition | Redis health check failing for 1+ minutes |
| Expression | `druggate_redis_up == 0` |

**What it means:** Redis is unreachable. This impacts:
- API key validation (all requests may fail with 401)
- Rate limiting (can't check/enforce limits)
- Cached data (all requests go to upstream, increasing latency)

**Response:**
1. Check Redis process: `redis-cli -h <redis-host> ping` or `docker ps | grep redis`
2. Check disk space (AOF/RDB may have filled disk): `df -h`
3. Check Redis logs: `docker logs drug-gate-redis-1 --tail 50`
4. Restart Redis if needed: `docker-compose restart redis` or `systemctl restart redis`
5. After recovery, verify data integrity: `redis-cli DBSIZE`

### DrugGateHighRateLimitRejections

| Field | Value |
|-------|-------|
| Severity | warning |
| Condition | > 50 rate-limit rejections per minute sustained over 5 minutes |
| Expression | `sum(rate(druggate_ratelimit_rejections_total[5m])) * 300 > 50` |

**What it means:** One or more API keys are being throttled at a sustained high rate. Could indicate scraping, misconfigured client, or an attack.

**Response:**
1. Identify the key(s): query `druggate_ratelimit_rejections_total` by `api_key` label
2. Check if the key is legitimate: `curl -H "Authorization: Bearer $ADMIN_SECRET" http://localhost:8081/admin/keys/<key>`
3. If abuse: revoke the key: `curl -X DELETE -H "Authorization: Bearer $ADMIN_SECRET" http://localhost:8081/admin/keys/<key>`
4. If legitimate but rate too low: adjust rate limit tier for that key

## Threshold Tuning

Default thresholds are conservative. Tune based on observed traffic patterns.

| Alert | Default | When to Tighten | When to Loosen |
|-------|---------|----------------|----------------|
| Error Rate | > 5% | Production stability reached, SLAs defined | Early beta, upstream still unstable |
| Latency p95 | > 500ms | Cached responses should be < 50ms | Cold cache scenarios, large responses |
| Redis Down | 1m | Production (may want 30s) | Dev/staging (may want 5m) |
| Rate Limit | 50/min | Low-traffic production | High-traffic with many legitimate keys |

To adjust thresholds, edit `prometheus/alerts.yml` and reload Prometheus.

## Alertmanager Integration

To receive notifications, configure Alertmanager routing:

```yaml
# alertmanager.yml
route:
  group_by: ['alertname', 'service']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  receiver: default
  routes:
    - match:
        severity: critical
      receiver: pagerduty  # or slack-critical
    - match:
        severity: warning
      receiver: slack-warnings

receivers:
  - name: default
    # ...
  - name: slack-warnings
    slack_configs:
      - channel: '#drug-gate-alerts'
  - name: pagerduty
    pagerduty_configs:
      - service_key: '<key>'
```

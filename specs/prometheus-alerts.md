# Spec: Prometheus Alert Rules

**Version:** 0.1.0
**Created:** 2026-03-20
**PRD Reference:** docs/prd.md — M7: Operational Hardening
**Status:** Approved

## 1. Overview

Ship a Prometheus alert rules file that monitors drug-gate's key health signals: error rate, request latency, Redis availability, and rate limit abuse. Paired with an ops guide documenting each alert, its meaning, and recommended response.

### User Story

As an **operator**, I want **Prometheus alerts that fire when drug-gate is unhealthy**, so that **I'm notified before users experience degraded service and can respond with a documented runbook**.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | `prometheus/alerts.yml` contains valid Prometheus alerting rules | Must |
| AC-002 | Alert: `DrugGateHighErrorRate` fires when HTTP 5xx rate > 5% over 5m | Must |
| AC-003 | Alert: `DrugGateHighLatency` fires when p95 latency > 500ms over 5m | Must |
| AC-004 | Alert: `DrugGateRedisDown` fires when `druggate_redis_up == 0` for 1m | Must |
| AC-005 | Alert: `DrugGateHighRateLimitRejections` fires when rate limit rejections > 50/min | Must |
| AC-006 | Each alert has `summary` and `description` annotations | Must |
| AC-007 | Each alert has `severity` label (critical/warning) | Must |
| AC-008 | `ops/prometheus-alerts.md` documents each alert with description and response actions | Must |
| AC-009 | Ops doc includes instructions for loading rules into Prometheus | Must |
| AC-010 | Ops doc includes threshold tuning guide | Should |
| AC-011 | Alert rules YAML passes `promtool check rules` validation | Should |

## 3. User Test Cases

### TC-001: Rules file is valid YAML

**Steps:**
1. Parse `prometheus/alerts.yml` as YAML
2. Verify structure matches Prometheus alerting rules schema
**Expected Result:** Valid YAML with `groups` → `rules` structure, each rule has `alert`, `expr`, `for`, `labels`, `annotations`
**Maps to:** AC-001

### TC-002: Error rate alert expression

**Steps:**
1. Read the `DrugGateHighErrorRate` rule expression
**Expected Result:** Expression calculates 5xx rate as a ratio of total requests over 5m window, threshold > 0.05
**Maps to:** AC-002

### TC-003: Ops doc completeness

**Steps:**
1. Read `ops/prometheus-alerts.md`
2. Verify each alert from the rules file has a corresponding section
**Expected Result:** All 4 alerts documented with description, severity, and recommended response
**Maps to:** AC-008

## 4. Data Model

No application data model changes. Alert rules reference existing Prometheus metrics.

### Metrics Referenced

| Metric | Used By Alert | Type |
|--------|--------------|------|
| `druggate_http_requests_total` | DrugGateHighErrorRate | Counter (labels: route, method, status_code) |
| `druggate_http_request_duration_seconds` | DrugGateHighLatency | Histogram (labels: route, method) |
| `druggate_redis_up` | DrugGateRedisDown | Gauge |
| `druggate_ratelimit_rejections_total` | DrugGateHighRateLimitRejections | Counter (labels: api_key) |

## 5. API Contract

No API changes.

## 6. Alert Definitions

### DrugGateHighErrorRate (warning)
- **Expression:** `sum(rate(druggate_http_requests_total{status_code=~"5.."}[5m])) / sum(rate(druggate_http_requests_total[5m])) > 0.05`
- **For:** 5m
- **Meaning:** More than 5% of requests are returning 5xx errors
- **Response:** Check upstream cash-drugs health, Redis connectivity, recent deployments

### DrugGateHighLatency (warning)
- **Expression:** `histogram_quantile(0.95, sum(rate(druggate_http_request_duration_seconds_bucket[5m])) by (le)) > 0.5`
- **For:** 5m
- **Meaning:** 95th percentile latency exceeds 500ms
- **Response:** Check Redis latency, upstream cash-drugs response times, cache hit rates

### DrugGateRedisDown (critical)
- **Expression:** `druggate_redis_up == 0`
- **For:** 1m
- **Meaning:** Redis health check failing — auth, rate limiting, and caching are impacted
- **Response:** Check Redis process, disk space, network connectivity. Restart if needed.

### DrugGateHighRateLimitRejections (warning)
- **Expression:** `sum(rate(druggate_ratelimit_rejections_total[5m])) * 300 > 50`
- **For:** 5m
- **Meaning:** More than 50 rate limit rejections per minute sustained over 5 minutes
- **Response:** Identify abusing API key, consider revoking or adjusting limits

## 7. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| No traffic (rate denominator is 0) | Error rate expression should handle division by zero (use `> 0` guard) |
| Prometheus not configured to scrape drug-gate | Alerts won't fire — ops doc should include scrape config example |
| Redis flapping (up/down rapidly) | `for: 1m` prevents alert fatigue on brief blips |

## 8. Dependencies

- Existing Prometheus metrics in `internal/metrics/metrics.go`
- Prometheus instance configured to scrape drug-gate's `/metrics` endpoint
- Optional: `promtool` for rule validation

## 9. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-20 | 0.1.0 | calebdunn | Initial spec |

# Cycle 3 — Operational Hardening

**Milestone:** M7 — Operational Hardening
**Maturity:** Beta
**Status:** PLANNED
**Started:** TBD
**Completed:** TBD
**Duration Budget:** 2-3 days (away mode)

## Work Items

| Feature | Current Pos | Target Pos | Assigned | Est. Effort | Validation |
|---------|-------------|-----------|----------|-------------|------------|
| Request ID Correlation | SHAPED | VERIFIED | Agent | ~3 hours | Spec + middleware + slog integration + tests |
| Drug Autocomplete | SHAPED | VERIFIED | Agent | ~3 hours | Spec + handler + service method + tests |
| Redis Persistence + Backup | SHAPED | VERIFIED | Agent | ~2 hours | Spec + docker-compose + staging config + prod docs |
| Prometheus Alert Rules | SHAPED | VERIFIED | Agent | ~2 hours | Spec + rules file + ops docs + tests |

## Dependencies & Serialization

```
Phase 1: Specs (all 4 features — SHAPED → SPECCED)
    ↓
Phase 2: Request ID Middleware (foundational — other features log with it)
    ↓
Phase 3: Drug Autocomplete (benefits from request ID tracing)
    ↓
Phase 4: Redis Persistence (independent infra, but benefits from correlated logs during testing)
    ↓
Phase 5: Prometheus Alert Rules (benefits from all metrics being in place)
    ↓
Phase 6: E2E validation + Swagger + PR
```

## Parallel Strategy

Single-threaded execution. Features advance sequentially within one agent.

## Execution Plan

### Phase 1: Specs (~1.5h)

Write specs for all 4 features before any implementation:
1. `specs/request-id.md` — X-Request-ID middleware, slog integration, header propagation
2. `specs/drug-autocomplete.md` — GET /v1/drugs/autocomplete, prefix matching, sub-50ms
3. `specs/redis-persistence.md` — AOF, snapshots, backup cron, restore procedure
4. `specs/prometheus-alerts.md` — Alert rules file, thresholds, ops documentation

Each spec includes acceptance criteria, test cases, and edge cases.

### Phase 2: Request ID Correlation (~3h)

**RED:**
1. Write tests for X-Request-ID middleware:
   - Generates UUID if no X-Request-ID header present
   - Passes through existing X-Request-ID from client
   - Sets X-Request-ID in response headers
   - Adds request_id to slog context for all downstream logs
2. Write tests for structured log correlation:
   - Request logger includes request_id field
   - Handler logs include request_id from context

**GREEN:**
1. Create `internal/middleware/requestid.go`:
   - Extract or generate X-Request-ID (UUID v4)
   - Store in request context
   - Set response header
   - Add to slog via `slog.With("request_id", id)`
2. Update `internal/middleware/logging.go`:
   - Pull request_id from context, include in log output
3. Wire middleware in `cmd/server/main.go` — first in chain (before logger)

**REFACTOR:**
- Ensure consistent context key usage
- Clean up any duplication

**Commit:** `feat: add X-Request-ID middleware with slog correlation`

### Phase 3: Drug Autocomplete (~3h)

**RED:**
1. Write handler tests for `GET /v1/drugs/autocomplete`:
   - Returns prefix-matched drug names
   - Respects `?q=` prefix parameter (required, min 2 chars)
   - Respects `?limit=` parameter (default 10, max 50)
   - Returns empty array for no matches
   - Returns 400 if q is missing or < 2 chars
   - Case-insensitive matching
   - Response shape: `{"data": [{"name": "...", "type": "generic|brand"}]}`
2. Write service tests:
   - Filters from cached drug names (reuses GetDrugNames)
   - Prefix match, not substring
   - Results sorted alphabetically

**GREEN:**
1. Add `AutocompleteDrugs(ctx, prefix, limit)` method to DrugDataService
   - Call `GetDrugNames()` to get cached list
   - Filter by case-insensitive prefix match on Name field
   - Sort alphabetically, cap at limit
2. Create `internal/handler/autocomplete.go`:
   - Parse query params (q, limit)
   - Validate: q required, len >= 2
   - Call service, return JSON response
3. Wire route in `cmd/server/main.go`: `r.Get("/v1/drugs/autocomplete", ...)`

**REFACTOR:**
- Ensure consistent error response shape with existing handlers

**Commit:** `feat: add drug autocomplete endpoint with prefix matching`

### Phase 4: Redis Persistence + Backup (~2h)

**RED:**
1. Write tests verifying Redis persistence config is documented
2. Write integration test that validates AOF is enabled (if Redis supports CONFIG GET)

**GREEN:**
1. Update `docker-compose.yml`:
   - Add Redis volume mount for data persistence
   - Add Redis command with `--appendonly yes --appendfsync everysec`
2. Create `ops/redis-persistence.md`:
   - Local setup (docker-compose handles it)
   - Staging setup (192.168.1.145 — enable AOF, configure snapshot)
   - Production setup (document for future)
   - Nightly backup cron example: `redis-cli BGSAVE` + copy RDB
   - Restore procedure: stop Redis, replace dump.rdb/appendonly.aof, restart
3. Update staging Redis config (document the commands to run)

**Commit:** `ops: enable Redis AOF persistence, add backup/restore docs`

### Phase 5: Prometheus Alert Rules (~2h)

**RED:**
1. Write tests that validate alert rules YAML is syntactically correct
2. Write tests that verify expected alert names and thresholds

**GREEN:**
1. Create `prometheus/alerts.yml`:
   - `DrugGateHighErrorRate`: error rate > 5% over 5m window
   - `DrugGateHighLatency`: p95 > 500ms over 5m window
   - `DrugGateRedisDown`: druggate_redis_up == 0 for 1m
   - `DrugGateHighRateLimitRejections`: rate limit rejections > 50/min
2. Create `ops/prometheus-alerts.md`:
   - How to load alert rules into Prometheus
   - Alert descriptions and recommended responses
   - Tuning guide for thresholds
3. Update existing Prometheus scrape config if needed

**Commit:** `ops: add Prometheus alert rules and alerting ops guide`

### Phase 6: Finalize (~1.5h)

1. Run full test suite — verify no regressions
2. Run `make lint` and `make vet` — fix any issues
3. Verify coverage stays above 80%
4. Update Swagger annotations for autocomplete endpoint
5. Run `make swagger` to regenerate docs
6. Update M7 milestone hill chart positions
7. Update PRD if needed
8. Create PR with full summary

**Commit:** `docs: update Swagger, M7 milestone, and PRD for operational hardening`

## Validation Criteria

### Per-Item Validation
- **Request ID:** X-Request-ID in all responses, request_id in all log lines, passes through client-provided IDs
- **Autocomplete:** Sub-50ms on cached data, prefix matching works for metformin/lisinopril/simvastatin, 400 on invalid input
- **Redis Persistence:** AOF enabled in docker-compose, staging config documented, restore procedure written
- **Alert Rules:** Valid YAML, 4 alert rules defined, ops doc explains each alert and response

### Cycle Success Criteria
- [ ] All 4 features reach VERIFIED position
- [ ] All acceptance criteria verified by tests
- [ ] Coverage stays above 80%
- [ ] No regressions in existing test suite
- [ ] Swagger docs updated for autocomplete
- [ ] Code committed, pushed, PR created

## Agent Autonomy (Away Mode)

**Level:** High autonomy (beta maturity, 2-3 day away session)

**Autonomous actions:**
- Write specs for all 4 features
- Execute TDD phases sequentially (RED → GREEN → REFACTOR per feature)
- Commit after each phase with conventional commits
- Push regularly
- Create PR when done
- Fix lint/type errors
- Probe staging Redis config if accessible

**Boundaries:**
- Do NOT merge to main
- Do NOT deploy to staging or production
- If Redis persistence testing reveals issues, document and continue
- If coverage drops below 80%, prioritize adding tests before moving to next feature
- If a feature scope grows unexpectedly, implement the core and document deferred items

## Notes

- Autocomplete reuses `DrugDataService.GetDrugNames()` — no new upstream calls needed
- Request ID middleware should be first in the chain (before RequestLogger) so all logs include it
- Redis persistence is primarily an ops/config change, minimal Go code involved
- Alert rules use existing metric names from `internal/metrics/metrics.go`
- The `ops/` directory is new — create it for operational documentation
- Existing middleware pattern: each middleware is a standalone file in `internal/middleware/`

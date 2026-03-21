# Cycle 5 — TTL Configuration

**Milestone:** M8 — Cache Architecture + Clinical Data
**Maturity:** Beta
**Status:** PLANNED
**Started:** TBD
**Completed:** TBD
**Duration Budget:** 12 hours (away mode)

## Work Items

| Feature | Current Pos | Target Pos | Assigned | Est. Effort | Validation |
|---------|-------------|-----------|----------|-------------|------------|
| Configurable Cache TTL | SHAPED | VERIFIED | Agent | ~2 hours | Env var override, tests, docs |

## Dependencies & Serialization

None — single feature, builds on existing CacheAside[T].

## Execution Plan

### Phase 1: Implementation (~1.5h)

**RED:**
1. Test that `CACHE_TTL` env var overrides the default 60m base TTL
2. Test that RxNorm search/profile TTL scales proportionally (or keeps its own ratio)
3. Test that RxNorm lookup TTL (7d) scales proportionally
4. Test that invalid env var value falls back to default with warning log

**GREEN:**
1. Add `CACHE_TTL` env var parsing in `cmd/server/main.go` (same pattern as `SYSTEM_METRICS_INTERVAL`)
2. Pass parsed TTL to service constructors or use a package-level config
3. CacheAside[T] already accepts TTL per instance — just wire the configured value through
4. RxNorm keeps its ratio: if base is 60m → search=24h, lookup=7d. If base is 30m → search=12h, lookup=3.5d

### Phase 2: Finalize (~30m)

1. Update CLAUDE.md environment variables table
2. Update specs/cache-aside.md to mark TTL config as done
3. Run full test suite
4. Commit, push, PR
5. Write learning checkpoint

## Validation Criteria

- [ ] `CACHE_TTL` env var overrides default base TTL
- [ ] RxNorm TTLs scale proportionally
- [ ] Invalid values log warning and use default
- [ ] All existing tests pass
- [ ] Coverage stays above 80%
- [ ] M8 success criterion "TTL configurable per environment" met

## Agent Autonomy (Away Mode)

**Autonomous:** Write tests, implement, commit, push, create PR.
**Boundaries:** Do NOT merge. Do NOT deploy.

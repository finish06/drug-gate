# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Conventional Commits](https://www.conventionalcommits.org/).

## [Unreleased]

## [0.7.1] - 2026-03-21

### Added
- `CACHE_TTL` env var for configurable base cache TTL — RxNorm TTLs scale proportionally
- Health check now verifies Redis and upstream dependencies — returns 503 when degraded
- `X-RateLimit-Limit` header shows total quota on all rate-limited responses
- CI publishes to GHCR alongside private registry on release tags

### Changed
- Error codes standardized to 5 canonical values: `bad_request`, `not_found`, `upstream_error`, `internal_error`, `rate_limited`
- Whitespace trimmed on all query parameters across all endpoints
- RxNorm NDCs and generics endpoints return 200 with empty data instead of 404 for valid RxCUI
- API key truncated to first 12 chars in Prometheus rate limit metrics labels

### Fixed
- **SECURITY:** Admin endpoints open when ADMIN_SECRET unset — empty Bearer token passed auth
- **SECURITY:** No request body size limit on POST /interactions — DoS vector (now 1MB limit)
- **SECURITY:** Wildcard CORS when API key has no origins — now requires explicit `"*"` in origins
- SPL search pagination returns correct `total_pages` (was always 0)
- Autocomplete uses `errors.Is` for upstream error matching (was string comparison, returned 500 instead of 502)
- Drug info safety fields (contraindications, warnings, adverse_reactions) return `[]` not `null`
- SPL indexer respects `CACHE_TTL` env var (was hardcoded 60m)

## [0.7.0] - 2026-03-20

### Added
- SPL detail and drug info endpoints now return sections 4 (Contraindications), 5 (Warnings and Precautions), and 6 (Adverse Reactions) alongside existing Section 7
- Generic `CacheAside[T]` utility for Redis cache-aside pattern — eliminates 211 lines of duplicated boilerplate
- `GET /v1/drugs/autocomplete?q={prefix}&limit={n}` — drug name typeahead endpoint with prefix matching, case-insensitive, sorted alphabetically (default limit 10, max 50)
- `X-Request-ID` middleware — generates UUID v4 or passes through client-provided ID, correlates in slog request logs, set in all response headers
- Prometheus alert rules (`prometheus/alerts.yml`) — 4 alerts: high error rate, high latency, Redis down, rate limit abuse
- Redis AOF persistence in docker-compose with named volume for data durability
- k6 performance test harness (`tests/k6/staging.js`) — 4 scenarios (smoke, load, spike, soak) covering all 21 endpoints
- k6 baseline comparison tool (`tests/k6/compare.js`) — compares runs against stored baselines, exits non-zero on >15% regression
- `ops/redis-persistence.md` — backup cron, restore procedures for local, staging, and production
- `ops/prometheus-alerts.md` — per-alert response procedures, threshold tuning guide, Alertmanager integration

### Changed
- Atomic cache TTL reset, connection pooling, and allocation reduction (performance)
- Request logger now includes `request_id` field when X-Request-ID middleware is active
- All 11 cached service methods migrated to generic `CacheAside[T]` (service files 865 → 654 lines)

### Fixed
- Old-format SPL Drug Interactions titles now handled correctly in XML parser
- DrugClassLookup E2E test tolerant of missing pharm_class data
- RxNorm E2E tests gracefully handle upstream timeouts
- golangci-lint errcheck warnings resolved across test files

## [0.6.1] - 2026-03-20

### Added
- Background SPL interaction indexer — pre-fetches and caches parsed interactions for popular drugs on startup (24h refresh, configurable top-N)
- SPL E2E tests against live cash-drugs (search, detail, drug info, interaction checker)
- Swagger annotations on all 4 SPL endpoints

### Documentation
- PRD updated for beta promotion, M6 milestones, and roadmap through M11

## [0.6.0] - 2026-03-17

### Added
- SPL document browser: `GET /v1/drugs/spls` (search by name) and `GET /v1/drugs/spls/{setid}` (detail with parsed Section 7)
- Drug info card: `GET /v1/drugs/info` — returns SPL metadata + structured interaction sections (by name or NDC)
- Multi-drug interaction checker: `POST /v1/drugs/interactions` — accepts 2-10 drugs, cross-references Section 7 text
- SPL XML parser for Section 7 (Drug Interactions) extraction via regex
- SPL Redis caching with 60-minute sliding TTL

## [0.5.1] - 2026-03-16

### Added
- `GET /version` endpoint — returns build version, git commit, git branch, Go version (public, no auth)
- Build-time injection of `GIT_COMMIT` and `GIT_BRANCH` via Dockerfile and CI ldflags
- RxNorm E2E tests (search, profile, NDCs, related, validation, not-found)
- Admin cache clear E2E test
- Version endpoint E2E test
- Staging auto-deploy via cron (replaces Watchtower)

### Fixed
- RxNorm client JSON parsing aligned with cash-drugs response shapes
- RxNorm scores parsed as floats (upstream sends `"14.335"`, not integers)
- Nameless RxNorm candidates (MMSL source) filtered out of search results

## [0.5.0] - 2026-03-15

### Added
- RxNorm integration: 5 new endpoints under `/v1/drugs/rxnorm/`
  - `GET /search?name=` — approximate match drug search (top 5 candidates + spelling suggestions)
  - `GET /profile?name=` — unified drug profile (RxCUI, generics, NDCs, brand names, related concepts)
  - `GET /{rxcui}/ndcs` — NDC codes for an RxCUI
  - `GET /{rxcui}/generics` — generic product equivalents
  - `GET /{rxcui}/related` — related concepts grouped by type (IN/BN/DF/SCD/SBD)
- RxNorm Redis caching: 24h sliding TTL for search/profile, 7d for RxCUI-based lookups
- Grafana dashboard JSON (`grafana/drug-gate-dashboard.json`) for all drug-gate metrics
- Staging environment on 192.168.1.145:8082 (auto-deploys via cron every 5m)
- `DELETE /admin/cache` endpoint for Redis cache clearing (prefix-based, SCAN-safe)

## [0.4.1] - 2026-03-15

### Fixed
- `DrugClassRaw` JSON struct tags used `class_name`/`class_type` but upstream returns `name`/`type` — caused empty `/v1/drugs/classes` responses
- Client unit test mock JSON updated to match real upstream field names
- CI GitHub Actions bumped to Node.js 24 compatible versions (`actions/checkout` v4→v6, `actions/setup-go` v5→v6)

### Added
- 15 E2E tests covering drug names, drug classes, drugs-by-class, and drug class lookup endpoints
- E2E config updated with `drugnames` and `drugclasses` slugs, `pharm_class` search param on `fda-ndc`
- Docs manifest (`.add/docs-manifest.json`) for incremental documentation freshness checks

### Documentation
- README.md updated with M3 endpoints, Prometheus metrics, `SYSTEM_METRICS_INTERVAL`, architecture diagrams
- Swagger docs regenerated
- Add swag annotations to all 5 admin endpoints (CreateKey, ListKeys, GetKey, DeactivateKey, RotateKey)
- Add apikey/ and ratelimit/ to CLAUDE.md Key Directories
- Update sequence diagrams, CLAUDE.md, and PRD for metrics

## [0.4.0] - 2026-03-14

### Added
- Prometheus metrics endpoint `GET /metrics` with full instrumentation
- HTTP request counter `druggate_http_requests_total` (route, method, status_code)
- HTTP request duration histogram `druggate_http_request_duration_seconds`
- Redis cache hit/miss counter `druggate_cache_hits_total` (key_type, outcome)
- Auth rejection counter `druggate_auth_rejections_total` (reason: missing/invalid/inactive)
- Rate limit rejection counter `druggate_ratelimit_rejections_total` (api_key)
- Redis health gauges `druggate_redis_up` and `druggate_redis_ping_duration_seconds` via background collector (30s interval)
- Container system metrics (CPU, memory, disk, network) via procfs (Linux-only, 15s interval)
- `SYSTEM_METRICS_INTERVAL` environment variable for configurable collection interval

## [0.3.0] - 2026-03-14

### Added
- Drug class lookup endpoint `GET /v1/drugs/class?name=` with generic/brand name fallback
- Paginated drug names listing `GET /v1/drugs/names` with search and type filter
- Paginated drug classes listing `GET /v1/drugs/classes` with type filter (default: EPC)
- Drugs-by-class listing `GET /v1/drugs/classes/drugs?class=` with pagination
- `internal/pharma` package for pharmacological class parsing and brand name deduplication
- `internal/service` package with `DrugDataService` — lazy Redis caching with 60-minute sliding TTL
- Service unit tests (19 tests with miniredis) and integration tests (22 tests with real Redis)
- Swag annotations on all M3 handler endpoints

## [0.2.0] - 2026-03-09

### Added
- API key authentication middleware (`X-API-Key` header validation via Redis store)
- Per-key CORS middleware (origin allowlist per API key)
- Sliding window rate limiting middleware (Redis sorted sets, per-key limits)
- Admin authentication middleware (Bearer token for admin routes)
- Admin key management endpoints: create, list, get, deactivate, rotate (`/admin/keys`)
- Key rotation with configurable grace period (old key stays valid until expiration)
- Redis-backed API key store (`internal/apikey/redis_store.go`)
- Redis-backed rate limiter with Lua script for atomicity (`internal/ratelimit/redis_limiter.go`)
- Integration tests for Redis store and limiter (build tag: `integration`)
- Protected `/v1` route group with auth → CORS → rate limit middleware chain
- `REDIS_URL` and `ADMIN_SECRET` environment variables
- E2E test suite with full-stack docker-compose (`docker-compose.e2e.yml`)
- `make test-integration` and `make test-e2e` targets
- Error-path tests for admin handler and rate limit middleware
- OpenAPI/Swagger documentation at `/swagger/` and `/openapi.json`
- `docker-compose.prod.yml` for production deployment
- NDC lookup endpoint `GET /v1/drugs/ndc/{ndc}` with validation, normalization, and fallback
- NDC parsing: 5-4, 4-4, 5-3 formats with dash required; 3-segment auto-strips package
- Fallback normalization: 4-4 pads labeler to 5-4, 5-3 pads product to 5-4
- cash-drugs HTTP client with upstream error handling (`fda-ndc` endpoint)
- Health check endpoint `GET /health` with status and version
- Build version embedding via `-ldflags` (defaults to `"dev"` for local builds)
- Request logging middleware (slog JSON: method, path, status, duration)
- CI pipeline: test job (vet, unit tests, coverage) + publish job
- Docker publish to `dockerhub.calebdunn.tech/finish06/drug-gate`
- Multi-stage Dockerfile (Go 1.26 builder, alpine runtime)
- docker-compose for local dev (drug-gate + Redis)
- Sequence diagrams in `docs/sequence-diagram.md`
- ADD methodology scaffolding (specs, plans, rules, config)
- MIT License

### Fixed
- URL encoding for NDC query parameter (`url.QueryEscape` for defense-in-depth)
- `.gitignore` pattern `server` matching `cmd/server/` directory (changed to `/server`)
- Go version mismatch between go.mod and Dockerfile/CI (updated to 1.26)
- CI coverage step excluding `cmd/` to avoid `covdata` tool error

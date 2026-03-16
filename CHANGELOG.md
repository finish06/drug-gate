# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Conventional Commits](https://www.conventionalcommits.org/).

## [Unreleased]

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
- `docker-compose.staging.yml` for staging deployment
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

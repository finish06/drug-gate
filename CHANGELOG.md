# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Conventional Commits](https://www.conventionalcommits.org/).

## [Unreleased]

### Added
- E2E test suite with full-stack docker-compose (`docker-compose.e2e.yml`)
- `make test-integration` and `make test-e2e` targets
- Error-path tests for admin handler and rate limit middleware (100% coverage on non-Redis code)
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
- NDC lookup endpoint `GET /v1/drugs/ndc/{ndc}` with validation, normalization, and fallback
- NDC parsing: 5-4, 4-4, 5-3 formats with dash required; 3-segment auto-strips package
- Fallback normalization: 4-4 pads labeler to 5-4, 5-3 pads product to 5-4
- cash-drugs HTTP client with upstream error handling (`fda-ndc` endpoint)
- Health check endpoint `GET /health` with status and version
- Build version embedding via `-ldflags` (defaults to `"dev"` for local builds)
- Request logging middleware (slog JSON: method, path, status, duration)
- CI pipeline: test job (vet, unit tests, coverage) + publish job
- Docker publish to `dockerhub.calebdunn.tech/finish06/drug-gate`
  - `:beta` on every push to main
  - `:vX.Y.Z` + `:latest` on git tag push
- Multi-stage Dockerfile (Go 1.26 builder, alpine runtime)
- docker-compose for local dev (drug-gate + Redis)
- OpenAPI/Swagger documentation at `/swagger/` and `/openapi.json`
- Swaggo annotations on all handlers (NDC lookup, health, swagger)
- `docker-compose.prod.yml` for production deployment
- Sequence diagrams in `docs/sequence-diagram.md`
- ADD methodology scaffolding (specs, plans, rules, config)
- MIT License

### Fixed
- URL encoding for NDC query parameter (`url.QueryEscape` for defense-in-depth)
- `.gitignore` pattern `server` matching `cmd/server/` directory (changed to `/server`)
- Go version mismatch between go.mod (1.25.5) and Dockerfile/CI (1.24 → 1.26)
- golangci-lint errcheck findings across production and test code
- CI coverage step excluding `cmd/` to avoid `covdata` tool error

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Conventional Commits](https://www.conventionalcommits.org/).

## [Unreleased]

### Added
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
- ADD methodology scaffolding (specs, plans, rules, config)
- MIT License

### Fixed
- URL encoding for NDC query parameter (`url.QueryEscape` for defense-in-depth)
- `.gitignore` pattern `server` matching `cmd/server/` directory (changed to `/server`)
- Go version mismatch between go.mod (1.25.5) and Dockerfile/CI (1.24 → 1.26)
- golangci-lint errcheck findings across production and test code
- CI coverage step excluding `cmd/` to avoid `covdata` tool error

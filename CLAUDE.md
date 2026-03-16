# drug-gate

Public-facing Go microservice gateway that provides frontend applications with drug information by querying the internal cash-drugs API. Handles auth, rate limiting, NDC normalization, and data transformation.

## Methodology

This project follows **Agent Driven Development (ADD)** — specs drive agents, humans architect and decide, trust-but-verify ensures quality.

- **PRD:** docs/prd.md
- **Specs:** specs/
- **Plans:** docs/plans/
- **Config:** .add/config.json

Document hierarchy: PRD → Spec → Plan → User Test Cases → Automated Tests → Implementation

## Tech Stack

| Layer | Technology | Version |
|-------|-----------|---------|
| Language | Go | 1.26 |
| Router | Chi | v5 |
| State/Cache | Redis | latest |
| Upstream | cash-drugs | 0.5.0+ (http://host1.du.nn:8083) |
| Metrics | Prometheus client_golang | promhttp + custom collectors |
| Containers | Docker Compose | local dev + production |

## Commands

### Development
```
docker-compose up                   # Start local dev (drug-gate + Redis)
make build                          # Build binary (bin/server)
make run                            # Run locally
make test-unit                      # Run unit tests
make test-coverage                  # Run tests with coverage report
make lint                           # golangci-lint
make vet                            # go vet
make test-integration                # Run Redis integration tests
make test-e2e                        # Run E2E tests (full stack)
make swagger                         # Regenerate Swagger docs
```

### ADD Workflow
```
/add:spec {feature}                  # Create feature specification
/add:plan specs/{feature}.md         # Create implementation plan
/add:tdd-cycle specs/{feature}.md    # Execute TDD cycle
/add:verify                          # Run quality gates
/add:deploy                          # Commit and deploy
/add:away {duration}                 # Human stepping away
```

## Architecture

### Key Directories
```
cmd/server/          — Application entrypoint
internal/
  handler/           — HTTP handlers (Chi routes)
  middleware/        — Auth, rate limiting, logging, CORS, metrics
  client/           — cash-drugs HTTP client
  apikey/           — API key store (Redis-backed CRUD, rotation)
  ratelimit/        — Per-key sliding window rate limiter (Redis)
  ndc/              — NDC normalization logic
  model/            — Request/response types
  pharma/           — Pharm class parsing, brand name deduplication
  service/          — DrugDataService + RxNormService (Redis-cached data layer)
  metrics/          — Prometheus metrics, Redis health collector, system metrics collector
  version/          — Build version (set via -ldflags)
specs/               — Feature specifications
docs/plans/          — Implementation plans
tests/
  unit/              — Pure unit tests
  integration/       — Redis-dependent tests
  e2e/               — End-to-end tests against cash-drugs
```

### Upstream API (cash-drugs)
- Base URL: `http://host1.du.nn:8083`
- Endpoints: `/api/cache/{slug}` with query params
- Key slugs: `fda-ndc`, `drugnames`, `drugclasses`, `spls-by-name`, `spls-by-class`, `rxnorm-approximate-match`, `rxnorm-spelling-suggestions`, `rxnorm-ndcs`, `rxnorm-generic-product`, `rxnorm-all-related`
- OpenAPI spec: `/openapi.json`

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CASHDRUGS_URL` | `http://localhost:8083` | Upstream cash-drugs base URL |
| `LISTEN_ADDR` | `:8081` | HTTP listen address |
| `REDIS_URL` | `redis:6379` | Redis connection address |
| `ADMIN_SECRET` | (none) | Bearer token for admin endpoints |
| `SYSTEM_METRICS_INTERVAL` | `15s` | System metrics collection interval (Go duration, Linux only) |

### Environments

- **Local:** docker-compose up (drug-gate on :8081, Redis on :6379)
- **Staging:** 192.168.1.145:8082 (auto-deploys :beta via Watchtower)
- **Production:** Self-hosted, behind firewall, same network as cash-drugs

## Quality Gates

- **Mode:** Standard
- **Coverage threshold:** 80%
- **Type checking:** go vet (blocking)
- **E2E required:** No

All gates defined in `.add/config.json`. Run `/add:verify` to check.

## Source Control

- **Git host:** GitHub
- **Branching:** Feature branches off `main`
- **Commits:** Conventional commits (feat:, fix:, test:, refactor:, docs:)
- **CI/CD:** GitHub Actions (.github/workflows/ci.yml)
- **Deploy:** Push to main → `:beta`, git tags → `:vX.Y.Z` + `:latest`

## Deploy Expectations

When deploying changes that modify routes, handlers, middleware, or the upstream integration:
- Update `docs/sequence-diagram.md` to reflect the current request flows
- Ensure new endpoints, error paths, and middleware are represented in the Mermaid diagrams

## Collaboration

- **Autonomy level:** Autonomous
- **Review gates:** PR review required before merge
- **Deploy approval:** Required for production

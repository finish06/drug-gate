# Process Observations

2026-03-07 23:45 | verify | All 32 tests pass, 90-100% coverage on all packages (excluding cmd entrypoint), all 17 ACs traced to tests, 1 security fix applied (url.QueryEscape) | Saved potential query injection vector
2026-03-07 23:55 | tdd-cycle | Docker build & publish — 2 new tests, version package, CI publish job with beta/release tags, 34 total tests passing, lint clean | Enables automated container deployment
2026-03-08 04:35 | tdd-cycle | OpenAPI docs — 6 new tests, swaggo annotations on all handlers, /swagger/ UI + /openapi.json, 42 total tests passing, lint clean | Interactive API docs for frontend devs
2026-03-08 05:00 | retro | M1 closed — 3 specs, 42 tests, 97.1% coverage, 7 commits. 3 learnings promoted to user library. Key wins: interface mocking, TDD security catch. Key issues: gitignore/Go version sync | Foundation for M2
2026-03-08 12:10 | verify | Security & rate limiting GREEN phase — all gates pass, 20/20 ACs covered, 65.3% coverage (Redis impls behind integration tag), golangci-lint clean, go vet clean | Completes M2 security feature implementation
2026-03-15 23:15 | docs | Full discovery + manifest created, sequence diagrams verified fresh (14/14 routes covered), Swagger regenerated via swag init, CLAUDE.md verified current — no drift detected | Baseline manifest enables incremental checks going forward
2026-03-16 12:15 | docs | RxNorm docs update — 3 sequence diagrams added (search, profile, granular), route table updated (19 routes), Swagger regenerated with 5 new RxNorm endpoints, CLAUDE.md + README.md synced, CHANGELOG updated | 19/19 routes fully documented

2026-04-11 16:24 | tdd-cycle | health-version-standard — 12 new tests, HealthResponse/DependencyInfo/VersionResponse types, BuildTime ldflag wired through Makefile/Dockerfile/CI, legacy HealthCheck removed, swagger regenerated, 87.7% handler coverage | Brings /health and /version into compliance with cross-service standard (rx-dag reference)

2026-04-11 16:45 | verify | Gate 1-4 pass locally (vet clean, handler lint clean after fixing defer resp.Body.Close, handler pkg 87.7% coverage, 12/12 AC tests mapped). CI still blocked by pre-existing internal/cache singleflight flake (unrelated). | Confirmed health-version-standard changes are production-ready pending unrelated CI fix

2026-04-11 16:52 | verify | fix singleflight flake + health.go errcheck, CI green, beta deployed to staging, manual /health + /version validate, k6 load 10/10 pass (HTTP p95 204.9ms vs baseline 347.7ms, -41.1%) | Unblocked health-version-standard deploy, fixed pre-existing blocker

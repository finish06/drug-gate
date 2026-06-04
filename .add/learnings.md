# Project Learnings — drug-gate

> **Tier 3: Project-Specific Knowledge**
> Generated from `.add/learnings.json` — do not edit directly.
> Agents read JSON for filtering; this file is for human review.

## Anti-Patterns
- **[high] Sequence publish→notify in CI; a parallel digest-poll races the build and times out** (L-030, 2026-06-03)
  A standalone notify workflow polled the registry for the image digest (10x30s/variant) in parallel with the image build; with timeout-minutes:5 the job was cancelled mid-poll and the publish/POST step was SKIPPED — no event sent, masquerading as a key problem. Fix: make notify a needs:publish job in the same workflow so the image+digest already exist (publish.outputs.image_digest, no poll). Also fail CI on non-202 instead of the provider's silent exit-0. General lesson: order dependent CI steps with needs/outputs rather than racing + polling; and run a provided CI artifact for real before trusting it (the dispatch test is what exposed the bug).

- **[high] Auth-before-CORS ordering breaks browser preflight (401 on keyless OPTIONS)** (L-028, 2026-06-02)
  On /v1/* the chain was APIKeyAuth → PerKeyCORS, so CORS preflight (OPTIONS) requests — which browsers send WITHOUT X-API-Key — hit auth first and got 401, blocking the real cross-origin request (surfaced to callers as a misleading 'TLS error'). Fix: a keyless CORSPreflight middleware ahead of auth that returns 204 + reflects Origin + advertises X-API-Key in Access-Control-Allow-Headers. Per-key origin locking can't run at preflight (no key present) — it must live on the actual request (PerKeyCORS sets ACAO there). Pattern: any auth middleware in front of a CORS-exposed route must let CORS preflights pass before authenticating. The spec already documented this (edge case 'preflight without API key') — code had drifted from spec.

- **[high] singleflight test flake: bare sleep races fast schedulers — use release-channel barrier** (L-025, 2026-04-11)
  internal/cache/aside_test.go TestCacheAside_Singleflight_{ConcurrentMiss,ErrorPropagation} failed deterministically on Linux CI but passed on macOS local. Root cause: first fetch fn slept 10ms INSIDE singleflight to give sibling goroutines time to reach sfGroup.Do, but on fast schedulers early goroutines completed before later ones entered Do, so coalescing broke. Fix: replace sleep with a buffered-channel signal (fetchStarted) + a release gate (releaseFetch). General principle: NEVER rely on 'sleep enough to give goroutines time to X' in concurrency tests — use explicit synchronization primitives.

- **[high] M9 Circuit breaker: io.NopCloser breaks HTTP connection reuse** (L-019, 2026-03-21)
  Wrapping resp.Body with io.NopCloser(io.LimitReader(...)) prevents the original Close() from releasing the connection back to the pool. Must use a custom limitedReadCloser that preserves the original Closer. Also, drugnames endpoint returns 7.4MB — initial 5MB limit truncated the response causing silent JSON decode failures (502). Always check actual upstream response sizes before setting limits.

- **[critical] Retro finding: learning checkpoints missed for 6 milestones** (L-012, 2026-03-20)
  M3 through M7 had no learning checkpoints recorded. Root cause: agent did not self-checkpoint after verify/cycle/deploy triggers. Documentation discipline must be enforced alongside code quality.

- **[high] M4 RxNorm: upstream response shape alignment required live testing** (L-007, 2026-03-14)
  RxNorm client needed 2 fix commits to align with actual cash-drugs response shapes. Mock-based unit tests passed but E2E revealed field name mismatches. Always validate against live upstream.

## Technical
- **[medium] post-deploy: health-version-standard live on staging, k6 load -41% vs baseline** (L-026, 2026-04-11)
  beta-70b05045 deployed to staging. /version emits all 7 standard fields. /health returns 200 + status=ok + structured dependencies array (redis 0.52ms, upstream 1.94ms, breaker closed). k6 load 10/10 checks pass, HTTP p95 204.9ms (baseline 347.7ms, -41.1%). MINOR: CI injects build_time in offset format (-05:00) not Z format per spec example.

- **[medium] verify post-TDD: caught unchecked defer resp.Body.Close in health handler** (L-024, 2026-04-11)
  Running golangci-lint after TDD-cycle caught one errcheck finding (defer resp.Body.Close() without error handling). Fixed by wrapping in defer func() { _ = resp.Body.Close() }(). Lesson: run golangci-lint locally as part of the tdd-cycle VERIFY phase, not just go vet.

- **[medium] health-version-standard: clean TDD cycle, breaking health response shape** (L-023, 2026-04-11)
  ACs covered: AC-001 through AC-026. RED: 12 new tests. GREEN: rewrote HealthResponse from map[string]string to []DependencyInfo, added VersionResponse with os/arch/build_time, wired BuildTime through Makefile/Dockerfile/CI. Breaking change — /health 'dependencies' shape flipped from map to array, acceptable at beta with no external monitoring consumers yet.

- **[medium] M8 Expanded SPL sections: regex parser extended for sections 4-6** (L-015, 2026-03-20)
  Same regex approach as Section 7 applied to sections 4 (Contraindications), 5 (Warnings), 6 (Adverse Reactions). SPLDetail model gains 3 new fields. Reuses InteractionSection type. ParseInteractions preserved for backward compat.

- **[medium] M7 Operational Hardening: 4 features, 29 tests, TDD clean** (L-010, 2026-03-20)
  Request ID middleware (8 tests), autocomplete (13 tests), Redis persistence (ops docs), Prometheus alerts (8 tests). All TDD RED→GREEN confirmed. Coverage maintained at 80.7%.

- **[medium] M3 Extended Lookups: lazy Redis caching with sliding TTL** (L-006, 2026-03-09)
  4 endpoints (names, classes, class lookup, drugs-by-class). Lazy cache with 60-min sliding TTL using GetEx. ~104K drug names, ~1.2K classes cached. Pagination helper reused across all list endpoints.

- **[medium] M2 Security & Rate Limiting: 20 ACs, Redis integration tests behind build tag** (L-005, 2026-03-08)
  All gates pass. Coverage 65.3% total but non-Redis code >80%. Redis implementations at 0% because integration tests use //go:build integration tag. Fixed auth_test.go grace period bug.

- **[medium] M1 NDC Lookup complete: 42 tests, 97.1% coverage** (L-004, 2026-03-08)
  ACs covered: 17. RED: 32 tests. GREEN: all passing. Blockers: gitignore and Go version mismatch caused 3 fix commits. Spec quality: good.

- **[medium] Interface-based mocking enables comprehensive handler testing** (L-003, 2026-03-07)
  DrugClient, SPLClient, RxNormClient interfaces with mock implementations allow testing all handler paths without Redis or upstream. Pattern reused across all milestones.

- **[medium] cash-drugs uses slug-based routing with query params** (L-001, 2026-03-07)
  Upstream API at /api/cache/{slug}. Key slugs: fda-ndc-by-name (BRAND_NAME), drugnames, drugclasses, spls-by-name (DRUGNAME), spls-by-class (DRUG_CLASS). Flat array response shape (data: [...] not data.results).

## Architecture
- **[critical] Generic cache-aside without singleflight causes thundering herd on TTL expiry** (L-022, 2026-03-27)
  CacheAside[T].Get had no deduplication — N concurrent cache misses triggered N identical upstream fetches. For the 7.4MB drugnames key at 1K concurrent requests, this is a self-inflicted DDoS on cash-drugs. Fix: add golang.org/x/sync/singleflight to coalesce concurrent fetches per key. Lesson: stampede prevention belongs in the cache layer, not in individual callers.

- **[medium] M8 CacheAside[T]: 211 lines eliminated across 11 methods** (L-014, 2026-03-20)
  Generic CacheAside[T] replaced per-method cache boilerplate in DrugDataService (3), RxNormService (5), SPLService (3). Service files 865→654 lines. Pointer-vs-value return needed care: not-found sentinel via zero-value check (RxCUI=="" for profile). 9 unit tests for the generic.

- **[medium] M6 SPL Interactions: regex XML parsing sufficient for Section 7** (L-008, 2026-03-17)
  SPL XML has namespace issues making full XML parser complex. Regex extraction of Section 7 (Drug Interactions) works reliably. spls-by-class deferred (unreliable/timeouts). Cross-reference uses word-boundary regex matching.

- **[medium] Chi chosen for middleware-heavy gateway architecture** (L-002, 2026-03-07)
  Chi v5 over net/http stdlib — middleware chaining purpose-built for auth, rate limiting, NDC validation, logging, CORS, metrics, request ID. Uses stdlib interfaces.

## Performance
- **[high] Autocomplete optimization: in-memory index gives 119,730x speedup** (L-021, 2026-03-22)
  Replaced per-request 7.4MB JSON deserialize + O(n) linear scan with pre-sorted in-memory index + O(log n) binary search. 30ms→0.25μs, 29MB→1KB, 200K→5 allocs. Key insight: the bottleneck was JSON deserialization from Redis on every request, not the prefix matching itself. Pre-lowercasing entries during index build eliminates repeated ToLower calls during search.

## Process
- **[medium] Cycle 11 complete: M9.5 production-deploy via homelab notification (pivoted twice)** (L-029, 2026-06-03)
  Shipped /livez (TDD) + a v* tag → CI publish → notify-prod → NATS bridge announcement, verified end-to-end (v0.10.1-rc1, bridge 202, Discord prompt). Deploy model pivoted mid-cycle: kubectl-from-CI → homelab NATS notify → folded into ci.yml as a needs:publish job. Spec/plan/cycle docs kept in lockstep. Repo-side VERIFIED; cluster probe wiring is the remaining downstream user step. M9.5 ready to close (gates beta→GA with M10).

- **[medium] docs manifest drift compounds fast when verify gates skip docs check** (L-027, 2026-05-16)
  Manifest sat 7 weeks (2026-03-25 → 2026-05-16) across cycles 7-10 + v0.10.0 release. 34 of 35 fingerprinted files changed, three new components (CacheAside w/ singleflight + GetWithStale, CircuitBreaker, NewSharedTransport) and a model split (internal/model/spl.go) landed without docs updates. Lesson: per-route diagrams are a poor place to document cross-cutting infra — one dedicated 'Upstream Resilience' diagram is cleaner. Action: consider adding `/add:docs --check` as a Gate 2 advisory so manifest drift surfaces before it compounds.

- **[medium] Cycle 8 complete: M9 circuit breaker + stale-cache + parallel interactions** (L-020, 2026-03-21)
  Duration: 1 session. Circuit breaker (9 tests), stale-cache with dual-key strategy (3 tests), parallel interactions with semaphore(5), MaxBytesReader (10MB). Two production bugs found post-merge: NopCloser connection leak and 5MB limit too low. Both fixed same session. Coverage 81.1%.

- **[high] 3-agent bug-finding swarm found critical admin auth vulnerability** (L-018, 2026-03-21)
  spawn-swarm with QA/Principal/Support roles found 35 bugs including a critical auth bypass (empty ADMIN_SECRET). Only the code-review agent caught it — live testing didn't expose it. Multi-perspective swarm audits should be standard before any public launch.

- **[medium] Cycle 6+7 complete: M8.5 Bugathon Tier 1+2 (13 bugs fixed)** (L-017, 2026-03-21)
  Tier 1: 7 security/correctness fixes (admin auth, SPL pagination, error matching, null fields, body limit, CORS wildcard, indexer TTL). Tier 2: 6 DX/reliability fixes. DX-4 marked not-a-bug. Coverage 81.0%. Staging API keys updated for CORS change. 3-agent swarm audit was highly effective for bug discovery.

- **[medium] Cycle 4 complete: M8 CacheAside[T] + SPL sections 4-6** (L-016, 2026-03-21)
  Duration: 1 day. Features: 2 advanced SHAPED→VERIFIED. CacheAside[T] eliminated 211 lines across 11 methods. SPL sections 4-6 added with 6 parser tests. Coverage 81.1%. Pointer-vs-value generics required not-found sentinel pattern. Documentation discipline improved: checkpoints L-014/L-015 written during cycle.

- **[critical] Human directive: documentation is as important as code** (L-013, 2026-03-20)
  Specs must be updated to Complete when done. All learning checkpoints must be written. Documentation is a first-class deliverable, not an afterthought. This is the #1 improvement priority.

- **[medium] k6 baseline comparison enables performance regression gates** (L-011, 2026-03-20)
  k6 harness covers all 21 endpoints across 4 scenarios (smoke/load/spike/soak). Baselines stored as JSON, comparison tool exits non-zero on >15% regression. Integrated into Makefile.

- **[medium] Beta promotion: 10/10 evidence score at alpha→beta** (L-009, 2026-03-17)
  All evidence items present: specs, 80%+ coverage, CI/CD, PR workflow, 2+ environments, conventional commits, TDD evidence, branch protection, release tags, quality gates.

---
*30 entries. Last updated: 2026-06-03. Source: .add/learnings.json*

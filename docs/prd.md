# drug-gate — Product Requirements Document

**Version:** 0.4.0
**Created:** 2026-03-07
**Author:** calebdunn
**Status:** Draft

## 1. Problem Statement

Frontend applications need access to drug information (names, therapeutic classes, interactions, RxNorm data) but should not directly query internal backend services. The internal `cash-drugs` API cache/proxy holds the data but is not designed for public-facing consumption — it lacks authentication, rate limiting, input normalization, and response shaping for frontend needs.

`drug-gate` solves this by providing a secure, rate-controlled gateway that normalizes inputs (e.g., NDC codes in any format), queries `cash-drugs` internally, and transforms responses into the format frontend applications need. It acts as the single entry point for all drug data consumed by external applications.

## 2. Target Users

- **Frontend developers** building patient-facing applications and clinical tools
- These developers need clean, well-documented APIs that accept flexible input formats and return consistently shaped responses
- Applications consuming this API are public-facing ("in the wild") and need protection against abuse

## 3. Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Correct data transformation | 100% fidelity between cash-drugs data and drug-gate responses | Integration tests against live cash-drugs |
| NDC normalization accuracy | Accept all valid NDC formats (10/11 digit, with/without dashes) and resolve correctly | Unit tests covering all NDC format variations |
| Uptime | 99.9% availability | Health check monitoring |
| Response latency | < 200ms p95 for cached lookups | Request metrics |

## 4. Scope

### In Scope (MVP)

- Publishable API key authentication (frontend-safe, origin-locked)
- Rate limiting per client/key
- NDC normalization: accept 10-digit, 11-digit, with dashes, without dashes — normalize to canonical form
- Drug lookup by NDC returning: drug name, therapeutic class(es)
- Query cash-drugs API (`http://host1.du.nn:8083`) as the sole data source
- Response transformation: shape cash-drugs responses into frontend-friendly format
- Health check endpoint
- OpenAPI documentation
- Docker containerization with docker-compose for local dev

### Out of Scope

- Therapy options by drug class (future)
- ~~Drug interactions via SPL data~~ → **Now in scope (M6)**
- ~~RxNorm data integration~~ → **Now in scope (M4, DONE)**
- Direct querying of DailyMed or FDA APIs (cash-drugs handles this)
- User management / registration (API keys provisioned externally for now)
- Frontend UI

## 5. Architecture

### Tech Stack

| Layer | Technology | Version | Notes |
|-------|-----------|---------|-------|
| Language | Go | 1.26 | Same ecosystem as cash-drugs |
| Backend Framework | Chi | v5 | Middleware-first router, stdlib-compatible |
| Cache/State | Redis | latest | Rate limit counters, API key validation, session state |
| Upstream API | cash-drugs | 0.5.0+ | Internal API cache/proxy at host1.du.nn:8083 |

### Infrastructure

| Component | Choice | Notes |
|-----------|--------|-------|
| Git Host | GitHub | New repository |
| Cloud Provider | Self-hosted | Homelab, behind firewall alongside cash-drugs |
| CI/CD | GitHub Actions | .github/workflows/ci.yml |
| Containers | Docker Compose | Local dev with Redis; production pulls from registry |
| IaC | None | Direct deployment |

### Environment Strategy

| Environment | Purpose | URL | Deploy Trigger |
|-------------|---------|-----|----------------|
| Local | Development & unit tests | http://localhost:8081 | Manual |
| Dev | Integration testing | TBD | Push to feature branch |
| Staging | Pre-production validation | TBD | PR to main |
| Production | Live frontend consumers | TBD | Merge to main |

**Environment Tier:** 3 (full pipeline)

Both drug-gate and cash-drugs run in the same physical environment behind the firewall. drug-gate is the only service exposed to frontend applications in the wild.

## 6. Milestones & Roadmap

### Current Maturity: Beta (promoted 2026-03-17)

### Roadmap

| Milestone | Goal | Target Maturity | Status | Success Criteria |
|-----------|------|-----------------|--------|------------------|
| M1: NDC Lookup | Accept NDC, return drug name + classes | alpha | DONE | NDC normalization works, cash-drugs integration verified |
| M2: Security & Rate Limiting | Auth + rate control | alpha | DONE | API key auth, per-key rate limits via Redis |
| M3: Extended Lookups | Filterable drug name, class, and drugs-by-class listings with lazy Redis caching | beta | DONE | Paginated data APIs serving frontend tools from cached cash-drugs data |
| M3.5: Observability | Prometheus metrics, Redis health collector, container system metrics | alpha | DONE | /metrics endpoint, HTTP/cache/auth/rate-limit counters, Redis + system background collectors |
| M4: RxNorm Integration | RxNorm drug search, profiles, NDCs, generics, related concepts | beta | DONE | 5 RxNorm endpoints, Redis caching, 42 tests |
| M6: SPL Interactions | SPL browser, drug info cards, multi-drug interaction checker with XML parsing | beta | DONE | 4 SPL endpoints, background indexer, E2E tests, 80%+ coverage, v0.6.1 tagged |
| M5: Polish & Quality | Version endpoint, RxNorm E2E tests, admin cache clear | beta | DONE | /version endpoint, 33 E2E tests passing, staging auto-deploy, 87.4% coverage |
| M7: Operational Hardening | Redis persistence, structured alerting, drug autocomplete | beta | DONE | AOF persistence, X-Request-ID correlation, Prometheus alert rules, autocomplete endpoint, k6 baselines, 80.7% coverage |
| M8: Cache Architecture + Clinical Data | Generic CacheAside[T], expanded SPL sections | beta | DONE | 211 lines eliminated, configurable TTL, SPL sections 4-6, 81.1% coverage |
| M8.5: Bugathon | Security, correctness, and DX fixes from 3-agent audit | beta | DONE | 13 bugs fixed (7 Tier 1, 6 Tier 2), v0.7.1 tagged |
| M9: Upstream Resilience | Circuit breaker, stale-cache, parallel interactions, MaxBytesReader | beta | DONE | Circuit breaker (10 fails), stale-cache serving, errgroup(5), 10MB limit, v0.8.0 tagged |
| M9.5: Production Deploy | Deploy automation with rollback | GA candidate | NEXT | GH Actions deploy workflow with health gate, version-pinned deploys, one-command rollback, runbook |
| M10: Admin Auth Hardening | HMAC-signed admin tokens, rotation, audit log | GA candidate | LATER | Static bearer token replaced, token rotation without restart, admin audit log, separate rate limits |
| M11: Flagship Aggregation | Unified drug profile, batch drug lookup | GA | LATER | GET /v1/drugs/profile merges all data, POST /v1/drugs/batch handles 5-20 drugs with per-item errors |

### Milestone Detail

#### M1: NDC Lookup [DONE]
**Goal:** Accept an NDC in any format and return drug name + therapeutic class(es) from cash-drugs
**Appetite:** 1-2 cycles
**Target maturity:** alpha
**Features:**
- NDC normalization (10/11 digit, dashes, formatting)
- cash-drugs client (HTTP client for internal API)
- Drug detail endpoint (`GET /v1/drugs/ndc/{ndc}`)
- Response shaping for frontend consumption
- Health check endpoint
**Success criteria:**
- [x] All NDC format variations resolve correctly
- [x] Drug name and therapeutic class(es) returned
- [x] cash-drugs integration tested
- [x] 80% test coverage (90-100% per package, excluding cmd entrypoint)

#### M2: Security & Rate Limiting [DONE]
**Goal:** Protect the API with publishable API keys, CORS origin locking, and per-key rate limiting
**Appetite:** 1-2 cycles
**Target maturity:** alpha

**Auth model: Publishable API keys (frontend-safe)**

API keys are designed to be embedded in frontend JavaScript — they are *publishable*, not secret. This follows the same pattern as Google Maps API keys and Stripe publishable keys. The data served (drug names, therapeutic classes) is public information from DailyMed/FDA, so the threat model is protecting uptime and preventing abuse, not guarding secrets.

Each key identifies *which application* is calling, not which user. Security is enforced through layered controls:

| Layer | Control | Purpose |
|-------|---------|---------|
| 1 | CORS origin lock | Only allowed domains can use the key from a browser |
| 2 | Per-key rate limiting | Prevents scraping and protects cash-drugs from overload |
| 3 | Read-only access | No mutations — worst case is someone reads public drug data |
| 4 | Key rotation | Instant invalidation in Redis if a key is compromised |
| 5 | Request logging | Audit trail per key for abuse detection |

**Features:**
- Publishable API key middleware (`X-API-Key` header)
- Per-key CORS origin allowlist (stored in Redis alongside key metadata)
- Per-key rate limiting via Redis (sliding window, configurable per tier)
- Request logging and audit trail
- Key provisioning CLI or admin endpoint (not exposed to frontends)
**Success criteria:**
- [x] Requests without valid API key rejected with 401
- [x] Requests from non-allowed origins rejected via CORS
- [x] Rate limits enforced per API key (429 + Retry-After header)
- [x] Redis-backed key storage with metadata (app name, origin allowlist, rate tier)
- [x] Key rotation works — old key invalidated, new key active immediately

#### M3: Extended Lookups [DONE]
**Goal:** Expose drug names, drug classes, and drug-to-class relationships as filterable, paginated APIs — enabling any frontend tool to work with FDA/DailyMed drug data
**Appetite:** 2-3 cycles
**Target maturity:** beta

**Data relationship model:**

Frontend apps need to understand how drug data connects. There are three independent data sources in cash-drugs, and the relationships between them matter:

```
┌─────────────────────┐     ┌─────────────────────┐
│  DailyMed Drug Names│     │  DailyMed Drug Classes│
│  (~104K entries)     │     │  (~1.2K entries)      │
│                     │     │                       │
│  • drug name        │     │  • class name          │
│  • type (G/B)       │     │  • type (EPC/MoA/PE/CS)│
│                     │     │                       │
│  NO class info      │     │  NO drug info          │
└─────────────────────┘     └───────────────────────┘
          │                           │
          │    These two lists are    │
          │    INDEPENDENT — no       │
          │    cross-reference        │
          │                           │
          ▼                           ▼
┌──────────────────────────────────────────────────┐
│              FDA NDC Directory                    │
│              (~132K products)                     │
│                                                  │
│  • generic_name ─── links drug to...             │
│  • brand_name                                    │
│  • pharm_class[] ── ...its classes               │
│                                                  │
│  THIS is the only place where                    │
│  drug ↔ class relationships exist                │
└──────────────────────────────────────────────────┘
```

**How frontend apps use the endpoints:**

| I want to... | Endpoint | Data source |
|--------------|----------|-------------|
| Browse/search all drug names | `GET /v1/drugs/names?q=simva` | DailyMed drugnames |
| Browse drug classes by type | `GET /v1/drugs/classes?type=epc` | DailyMed drugclasses |
| Find which class a drug belongs to | `GET /v1/drugs/class?name=simvastatin` | FDA NDC (by generic/brand name) |
| Find all drugs in a class | `GET /v1/drugs/classes/drugs?class=HMG-CoA+Reductase+Inhibitor` | FDA NDC (by pharm_class) |

**Example: Building a "match drug to class" quiz**
1. `GET /v1/drugs/classes?type=epc&limit=4` → get 4 random EPC classes
2. For each class: `GET /v1/drugs/classes/drugs?class={name}&limit=1` → get a drug from each class
3. Present the 4 drugs and 4 classes as a matching exercise

**Example: Drug info card**
1. User types "simva..." → `GET /v1/drugs/names?q=simva` → autocomplete suggestions
2. User selects "simvastatin" → `GET /v1/drugs/class?name=simvastatin` → full class info with brand names

**Caching:** All bulk data is lazy-loaded into Redis on first request with a 60-minute sliding TTL. If no app requests the data for 60 minutes, it's evicted. Next request triggers a fresh fetch from cash-drugs.

**Features:**
- Drug class lookup by name (`GET /v1/drugs/class?name={name}`)
- Paginated drug names with search and type filter (`GET /v1/drugs/names`)
- Paginated drug classes with type filter (`GET /v1/drugs/classes`)
- Drugs-by-class listing (`GET /v1/drugs/classes/drugs?class={name}`)
- Lazy Redis caching with 60-minute sliding TTL
- In-memory filtering and pagination from cached data

**Success criteria:**
- [x] All 4 endpoints return correct, paginated data
- [x] Drug-to-class relationships resolved via FDA NDC data
- [x] Lazy caching works — first request fetches from cash-drugs, subsequent requests served from Redis
- [x] Cache expires after 60 minutes of inactivity
- [x] Upstream errors return 502 with clear message

#### M6: SPL Interactions [IN_PROGRESS]
**Goal:** Expose drug interaction data from FDA Structured Product Labels (SPL) via three complementary APIs
**Appetite:** 1 week
**Target maturity:** beta

**SPL data model:**
```
Drug Name → spls-by-name → SPL metadata (title, setid, version)
                               ↓
SetID → spl-xml → Raw XML (~200KB) → Parse Section 7 → Interaction text
```

**Features:**
- SPL document browser (`GET /v1/drugs/spls`, `GET /v1/drugs/spls/{setid}`)
- Drug info card with interactions (`GET /v1/drugs/info`)
- Multi-drug interaction checker (`POST /v1/drugs/interactions`)
- Background indexer for pre-caching popular drug interactions

**Success criteria:**
- [x] SPL document search by drug name returns metadata
- [x] SPL detail endpoint returns parsed Section 7 from XML
- [x] Drug info card returns SPL metadata + interaction sections
- [x] Multi-drug interaction checker cross-references 2-10 drugs
- [x] Background indexer pre-fetches popular drug interactions
- [x] All endpoints authenticated, rate-limited, and cached
- [x] 80%+ test coverage on new code

#### M7: Operational Hardening [DONE]
**Goal:** Stabilize operations and ship highest-value quick-win feature
**Appetite:** 2 weeks
**Target maturity:** beta

**Features:**
- Redis Persistence + Key Backup (S effort)
  - Enable AOF persistence for Redis
  - Nightly snapshot backup via cron
  - Documented restore procedure
- Request ID Correlation + Structured Alerts (M effort)
  - `X-Request-ID` middleware (generate if absent, propagate through logs)
  - Prometheus alert rules for error rate, latency spikes, and Redis down
  - Structured log entries correlated by request ID
- Drug Autocomplete / Typeahead (S effort)
  - `GET /v1/drugs/autocomplete?q={prefix}&limit=10`
  - Sub-50ms response time target
  - Prefix-matched against cached drug names

**Success criteria:**
- [x] Redis AOF enabled, nightly snapshot cron running, restore procedure tested
- [x] X-Request-ID present in all responses and correlated in logs
- [x] Prometheus alert rules firing correctly for error rate > 5%, p95 latency > 500ms, Redis unreachable
- [x] Autocomplete endpoint returns results in < 50ms for cached data
- [x] 80%+ test coverage on new code

#### M8: Cache Architecture + Clinical Data [NOW — next 2 weeks]
**Goal:** Clean up technical debt that unblocks faster development, double the clinical data coverage
**Appetite:** 2 weeks
**Target maturity:** beta

**Features:**
- Generic CacheAside[T] refactor (M effort)
  - Replace per-endpoint cache boilerplate with a typed generic `CacheAside[T]` utility
  - Eliminate ~300 lines of duplicated cache fetch/store/expire logic
  - Configurable TTL per environment (shorter in dev, longer in production)
- Expanded SPL Sections parsing (M effort)
  - Parse Section 4 — Contraindications
  - Parse Section 5 — Warnings and Precautions
  - Parse Section 6 — Adverse Reactions
  - Alongside existing Section 7 — Drug Interactions

**Success criteria:**
- [ ] CacheAside[T] generic used by all cached endpoints (drug names, classes, NDC, RxNorm, SPL)
- [ ] Net reduction of ~300 lines of cache boilerplate
- [ ] TTL configurable per environment via config/env vars
- [ ] SPL detail endpoint returns sections 4, 5, 6, and 7
- [ ] Drug info card includes contraindications, warnings, and adverse reactions
- [ ] 80%+ test coverage on new code

#### M9: Upstream Resilience + Production Deploy [NEXT — weeks 5-6]
**Goal:** Eliminate single points of failure and establish production-grade deployment
**Appetite:** 2 weeks
**Target maturity:** GA candidate

**Features:**
- Circuit Breaker + Parallel Resolution (M effort)
  - Circuit breaker on cash-drugs HTTP client (open after N consecutive failures, half-open probe, auto-close)
  - Stale-cache serving when circuit is open (return expired cached data rather than 502)
  - Parallelize interaction checker with `errgroup` for multi-drug lookups
  - `MaxBytesReader` on all upstream responses to prevent memory exhaustion
- Production Deploy Automation + Rollback (M effort)
  - Pin deployments to version tags (no `:latest` in production)
  - GitHub Actions deploy workflow with health gate (deploy → health check → promote or rollback)
  - One-command rollback to previous version
  - Documented runbook for production operations

**Success criteria:**
- [ ] Circuit breaker trips after 10 consecutive upstream failures, serves stale cache
- [ ] Circuit auto-recovers via half-open probe after configurable cooldown
- [ ] Multi-drug interaction checker runs parallel upstream calls via errgroup
- [ ] MaxBytesReader limits upstream response size to 5MB
- [ ] Production deploy pinned to version tags, triggered by GH Actions
- [ ] Health gate verifies deployment before promoting
- [ ] One-command rollback documented and tested
- [ ] Runbook covers: deploy, rollback, Redis recovery, circuit breaker reset

#### M10: Admin Auth Hardening [LATER — week 7]
**Goal:** Harden the highest-privilege credential in the system
**Appetite:** 1 week
**Target maturity:** GA candidate

**Features:**
- HMAC-signed short-lived admin tokens (M effort)
  - Replace static `ADMIN_SECRET` bearer token with HMAC-SHA256 signed tokens
  - Tokens include expiration timestamp (configurable, default 15 minutes)
  - Server validates signature and expiry without external dependencies
- Token rotation without restart
  - Hot-reload signing key from Redis or environment
  - Grace period: accept tokens signed with previous key for N minutes after rotation
- Admin action audit log
  - Log all admin actions (key create, key revoke, cache clear) with timestamp, token ID, action, and target
  - Queryable via admin endpoint or structured log output
- Rate limit admin endpoints separately
  - Stricter rate limits on admin endpoints (e.g., 10 req/min vs 100 req/min for data APIs)
  - Separate sliding window from regular API key rate limits

**Success criteria:**
- [ ] Static ADMIN_SECRET no longer accepted; HMAC tokens required
- [ ] Tokens expire after configured TTL; expired tokens rejected with 401
- [ ] Signing key rotation works without service restart
- [ ] Grace period allows old-key tokens during rotation window
- [ ] All admin actions logged with actor, action, target, and timestamp
- [ ] Admin endpoints rate-limited separately from data endpoints
- [ ] 80%+ test coverage on new code

#### M11: Flagship Aggregation [LATER — weeks 8+]
**Goal:** Ship the flagship "Tell Me Everything" endpoint and batch operations that define drug-gate's competitive advantage
**Appetite:** 2-3 weeks
**Target maturity:** GA

**Features:**
- Unified Drug Profile endpoint (L effort)
  - `GET /v1/drugs/profile?name={drug_name}`
  - Merges in a single response: drug classification (NDC/pharm_class), RxNorm data (generic, related concepts, NDCs), SPL interactions (sections 4-7), and brand/generic name mapping
  - Parallel upstream resolution for all data sources
  - Graceful partial responses: if one data source fails, return what succeeded with error annotations
- Batch Drug Lookup (M effort)
  - `POST /v1/drugs/batch` with JSON body: `{"drugs": ["simvastatin", "lisinopril", ...]}` (5-20 drugs)
  - Parallel resolution per drug
  - Per-item error handling: individual drug failures don't fail the batch
  - Response includes per-drug status (success/error) and aggregated data

**Success criteria:**
- [ ] Unified profile endpoint returns classification + RxNorm + SPL data in one call
- [ ] Partial failures return available data with error annotations (not 500)
- [ ] Batch endpoint accepts 5-20 drugs and resolves in parallel
- [ ] Per-item errors reported without failing the batch
- [ ] p95 latency < 500ms for profile, < 2s for batch of 10
- [ ] 80%+ test coverage on new code
- [ ] E2E tests cover happy path and partial failure scenarios

### Maturity Promotion Path

| From | To | Requirements |
|------|-----|-------------|
| alpha → beta | Feature specs for all endpoints, 50%+ coverage, PR workflow active, TDD evidence | **PROMOTED 2026-03-17** (10/10 evidence) |
| beta → GA | Circuit breaker on upstream (M9), production deploy automation with health gate and rollback (M9), HMAC admin auth replacing static tokens (M10), admin audit log (M10), 80%+ coverage sustained, 30+ days production stability, SLAs defined, full CI/CD pipeline with version-pinned deploys | Requires M9 + M10 complete |

## 7. Key Features

### Feature 1: NDC Normalization
Accept NDC codes in any valid format (10-digit, 11-digit, with or without dashes in 4-4-2, 5-3-2, 5-4-1, or 5-4-2 patterns) and normalize to a canonical 11-digit format for upstream lookup via cash-drugs.

### Feature 2: Drug Detail Lookup
Given a normalized NDC, query cash-drugs for drug name and therapeutic class(es). Transform the response into a clean, frontend-friendly JSON shape.

### Feature 3: Publishable API Key Authentication
Middleware that validates publishable API keys on every request. Keys are designed to be embedded in frontend JavaScript — they identify the calling application, not the user. Keys stored in Redis with associated metadata (app name, allowed origins, rate limit tier). CORS enforcement ensures keys only work from registered domains when called from browsers.

### Feature 4: Rate Limiting
Sliding window rate limiter backed by Redis. Configurable per API key tier. Returns standard `429 Too Many Requests` with `Retry-After` header. Protects cash-drugs from being overwhelmed by any single frontend application.

## 8. Non-Functional Requirements

- **Performance:** < 200ms p95 response time for cached drug lookups. drug-gate adds minimal overhead on top of cash-drugs latency.
- **Security:** All endpoints require publishable API key. No direct exposure of cash-drugs internals. Input validation on all parameters. CORS origin-locked per API key. Drug data is public (DailyMed/FDA) — security protects uptime, not secrets.
- **Availability:** Must handle upstream cash-drugs being temporarily unavailable (graceful degradation, error responses).
- **Observability:** Structured logging (slog), request ID tracing, health check endpoint. Prometheus metrics (`/metrics`) covering HTTP request counts/latency, cache hit/miss ratios, auth/rate-limit rejections, Redis health, and container system metrics (CPU, memory, disk, network on Linux).

## 9. Future Discovery

Areas for future exploration as drug-gate evolves beyond MVP:

### Per-User Identity (JWT evolution)
If future requirements need to know *which user* is querying (audit trails, personalization, role-based access), evolve from publishable API keys to a JWT auth flow:
- Frontend authenticates the user (OAuth, login)
- Auth service issues short-lived JWTs with user claims
- drug-gate validates JWT signature (stateless) or checks Redis for revocation
- API keys remain for app-level identification; JWTs add user-level identity

### Local Response Caching
Should drug-gate cache responses in Redis, or rely entirely on cash-drugs caching? Trade-offs:
- Local cache reduces load on cash-drugs and improves latency
- But adds cache invalidation complexity and potential staleness
- Decision depends on observed cash-drugs latency and frontend request patterns

### NDC-to-Drug Mapping Strategy
Which cash-drugs endpoint returns the best data for NDC lookup? Candidates:
- `fda-ndc-by-name` — FDA NDC directory (has NDC → brand name + pharm class)
- `drugnames` + `drugclasses` — DailyMed reference data (broader but requires cross-referencing)
- May need to combine multiple upstream calls and merge results

### Rate Limit Tiers
Single tier for MVP or multiple from the start? Consider:
- MVP: single tier (e.g., 100 req/min per key)
- Later: tiered (free/standard/premium) based on application needs

### Extended Query Patterns (M3+)
- Drug class search: "give me all drugs in this therapeutic class"
- Name search: fuzzy matching, autocomplete support
- SPL interactions: cross-reference structured product labels
- RxNorm: standardized drug identifiers and relationships

## 10. Open Questions

- What NDC-to-drug-name mapping strategy? Which cash-drugs endpoint returns the best data for NDC lookup?
- Rate limit tiers: single tier for MVP or multiple from the start?
- Local caching in Redis or passthrough to cash-drugs only?

## 11. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-07 | 0.1.0 | calebdunn | Initial draft from /add:init interview |
| 2026-03-07 | 0.2.0 | calebdunn | Auth decision: publishable API keys (frontend-safe). Added Future Discovery section. |
| 2026-03-18 | 0.3.0 | calebdunn | Beta promotion. M6 SPL Interactions added. RxNorm and SPL moved from out-of-scope to done/in-progress. |

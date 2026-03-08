# drug-gate — Product Requirements Document

**Version:** 0.2.0
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
- Drug interactions via SPL data (future)
- RxNorm data integration (future)
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

### Current Maturity: Alpha

### Roadmap

| Milestone | Goal | Target Maturity | Status | Success Criteria |
|-----------|------|-----------------|--------|------------------|
| M1: NDC Lookup | Accept NDC, return drug name + classes | alpha | DONE | NDC normalization works, cash-drugs integration verified |
| M2: Security & Rate Limiting | Auth + rate control | alpha | NEXT | API key auth, per-key rate limits via Redis |
| M3: Extended Lookups | Drug class search, name search | beta | LATER | Multiple query patterns supported |
| M4: Interactions & RxNorm | SPL interactions, RxNorm integration | beta | LATER | Clinical data accessible via API |

### Milestone Detail

#### M1: NDC Lookup [NOW]
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

#### M2: Security & Rate Limiting [NEXT]
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
- [ ] Requests without valid API key rejected with 401
- [ ] Requests from non-allowed origins rejected via CORS
- [ ] Rate limits enforced per API key (429 + Retry-After header)
- [ ] Redis-backed key storage with metadata (app name, origin allowlist, rate tier)
- [ ] Key rotation works — old key invalidated, new key active immediately

### Maturity Promotion Path

| From | To | Requirements |
|------|-----|-------------|
| alpha → beta | Feature specs for all endpoints, 50%+ coverage, PR workflow active, TDD evidence |
| beta → ga | 30+ days production stability, SLAs defined, 80%+ coverage, full CI/CD pipeline |

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
- **Observability:** Structured logging (slog), request ID tracing, health check endpoint.

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

## 10. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-07 | 0.1.0 | calebdunn | Initial draft from /add:init interview |
| 2026-03-07 | 0.2.0 | calebdunn | Auth decision: publishable API keys (frontend-safe). Added Future Discovery section. |

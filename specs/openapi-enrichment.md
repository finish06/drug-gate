# Spec: OpenAPI Documentation Enrichment

**Version:** 1.0
**Created:** 2026-03-19
**PRD Reference:** docs/prd.md
**Status:** Complete
**Milestone:** M6 — SPL Interactions (docs phase)

## 1. Overview

Enrich all swaggo annotations across drug-gate handlers to produce an OpenAPI 3.0 spec optimized for machine-to-machine consumption. AI agents and LLM tool-use integrations should be able to read the spec and understand how to authenticate, discover endpoints, chain calls, and handle errors without human guidance.

### User Story

As an **AI agent or API integration tool**, I want **rich, example-laden OpenAPI documentation with clear authentication schemes, endpoint descriptions, and error shapes**, so that **I can autonomously discover, authenticate, and use the drug-gate API without reading source code or external docs**.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | Top-level API description includes a "Getting Started" flow: create key → authenticate → search → details → interactions | Must |
| AC-002 | `securitySchemes` defines `ApiKeyAuth` (apiKey, header, X-API-Key) for `/v1/*` endpoints | Must |
| AC-003 | `securitySchemes` defines `AdminBearerAuth` (http, bearer) for `/admin/*` endpoints | Must |
| AC-004 | Every `/v1/*` endpoint has `@Security ApiKeyAuth` annotation | Must |
| AC-005 | Every `/admin/*` endpoint has `@Security AdminBearerAuth` annotation | Must |
| AC-006 | All endpoints have `@Description` with 2-4 sentences explaining purpose, when to use, and what the response contains | Must |
| AC-007 | All path and query parameters have `@Param` with realistic `example` values | Must |
| AC-008 | All success responses include realistic example bodies using actual drug data (warfarin, lipitor, metformin) | Must |
| AC-009 | All error responses (400, 401, 404, 429, 502) are documented per endpoint with example error bodies | Must |
| AC-010 | Endpoints are grouped into tags: `system`, `drugs`, `rxnorm`, `spl`, `admin` | Must |
| AC-011 | Each tag has a `@Tag` description explaining the group's purpose | Must |
| AC-012 | Response model schemas have field-level descriptions via swaggo struct comments | Should |
| AC-013 | `swag init` generates the spec without errors from the annotated code | Must |
| AC-014 | Generated spec validates against OpenAPI 3.0 schema | Should |
| AC-015 | API version in spec matches v0.6.0 | Must |
| AC-016 | All examples use standard OpenAPI `example` fields (no custom extensions) | Must |

## 3. User Test Cases

### TC-001: AI agent discovers authentication method

**Precondition:** Agent reads `/openapi.json`
**Steps:**
1. Parse `securitySchemes` from the spec
2. Identify `ApiKeyAuth` → header `X-API-Key`
3. Identify `AdminBearerAuth` → bearer token
**Expected Result:** Agent knows to send `X-API-Key` for data endpoints and `Authorization: Bearer` for admin endpoints.
**Maps to:** AC-002, AC-003, AC-004, AC-005

### TC-002: AI agent follows getting-started flow

**Precondition:** Agent reads top-level `description` field
**Steps:**
1. Parse the getting-started instructions
2. Create API key via `POST /admin/keys`
3. Use key to call `GET /v1/drugs/names?q=lipitor`
4. Use result to call `GET /v1/drugs/info?name=atorvastatin`
**Expected Result:** Agent can chain calls in correct order from spec alone.
**Maps to:** AC-001

### TC-003: AI agent understands error recovery

**Precondition:** Agent reads endpoint error responses
**Steps:**
1. Call endpoint without API key → read 401 example
2. Call with invalid NDC → read 400 example
3. Call when upstream is down → read 502 example
**Expected Result:** Agent maps each status code to a specific error shape and can decide retry vs. fail.
**Maps to:** AC-009

### TC-004: Swagger UI renders enriched docs

**Precondition:** `swag init` has been run
**Steps:**
1. Open `/swagger/` in browser
2. Verify tags group endpoints logically
3. Verify example values appear in request/response panels
4. Verify security lock icons appear on protected endpoints
**Expected Result:** Human-readable docs are also improved as a side effect.
**Maps to:** AC-010, AC-011, AC-013

### TC-005: swag init succeeds

**Precondition:** All annotations updated
**Steps:**
1. Run `swag init -g cmd/server/main.go`
**Expected Result:** No errors, `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` regenerated.
**Maps to:** AC-013

## 4. Data Model

No new data models. Existing models gain field-level documentation:

### Models to annotate

- `model.DrugDetailResponse` — NDC lookup response
- `model.ErrorResponse` — standard error shape
- `model.DrugClassResponse` — class lookup response
- `model.DrugNameEntry`, `model.DrugClassEntry`, `model.DrugInClassEntry`
- `model.PaginatedResponse`, `model.Pagination`
- `model.SPLEntry`, `model.SPLDetail`, `model.InteractionSection`
- `model.DrugInfoResponse`, `model.SPLSource`
- `model.InteractionCheckRequest`, `model.InteractionCheckResponse`
- `model.DrugIdentifier`, `model.DrugCheckResult`, `model.InteractionMatch`
- `model.RxNormSearchResult`, `model.RxNormProfile`, `model.RxNormNDCResponse` (in rxnorm.go)

## 5. API Contract

No new endpoints. All 24 existing endpoints get enriched annotations:

### Tag Groups

| Tag | Description | Endpoints |
|-----|-------------|-----------|
| `system` | Health, version, metrics, and documentation endpoints | /health, /version, /metrics, /swagger/*, /openapi.json |
| `drugs` | Drug lookup by NDC, name, and class — core data from FDA/DailyMed | /v1/drugs/ndc/*, /v1/drugs/class, /v1/drugs/names, /v1/drugs/classes, /v1/drugs/classes/drugs |
| `rxnorm` | RxNorm drug search, profiles, NDCs, generics, and related concepts | /v1/drugs/rxnorm/* |
| `spl` | Structured Product Labels — drug interaction data from FDA labels | /v1/drugs/spls, /v1/drugs/spls/*, /v1/drugs/info, /v1/drugs/interactions |
| `admin` | API key management and cache administration (requires admin bearer token) | /admin/* |

### Security Schemes

```
@securityDefinitions.apikey ApiKeyAuth
@in header
@name X-API-Key
@description Publishable API key for frontend applications. Create via POST /admin/keys.

@securityDefinitions.apikey AdminBearerAuth
@in header
@name Authorization
@description Admin bearer token. Set via ADMIN_SECRET environment variable. Format: "Bearer <secret>"
```

## 6. Edge Cases

- swaggo has limited support for `examples` (plural) — use singular `example` on schemas
- swaggo struct tags use `example:"value"` format for field examples
- Security annotations must be on every handler, not just at router level
- Tags must be declared in main.go top-level comments with `@Tag.name` and `@Tag.description`
- POST /v1/drugs/interactions needs `@Accept json` and `@Param body body model.InteractionCheckRequest true "Drug list"`
- Some RxNorm response types are in `internal/model/rxnorm.go` — need swaggo annotations too

## 7. Dependencies

- swaggo/swag CLI installed (`go install github.com/swaggo/swag/cmd/swag@latest`)
- All model types must be exported and in packages swag can reach
- `make swagger` target should run `swag init -g cmd/server/main.go`

## 8. Revision History

| Date | Version | Changes |
|------|---------|---------|
| 2026-03-19 | 1.0 | Initial spec from interview — AI-optimized OpenAPI enrichment |

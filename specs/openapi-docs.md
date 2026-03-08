# Spec: OpenAPI Documentation

**Version:** 0.1.0
**Created:** 2026-03-08
**PRD Reference:** docs/prd.md
**Status:** Draft

## 1. Overview

Auto-generated OpenAPI 3.0 documentation for all API endpoints, served at runtime via Swagger UI. Uses swaggo/swag to generate the spec from Go code annotations. Consumers can explore and test the API interactively from a browser.

### User Story

As a frontend developer integrating with drug-gate, I want interactive API documentation served from the service itself, so that I can discover available endpoints, understand request/response formats, and test calls without reading source code.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | `GET /openapi.json` returns a valid OpenAPI 3.0 JSON spec | Must |
| AC-002 | `GET /swagger/` serves Swagger UI with the generated spec loaded | Must |
| AC-003 | All handler functions have swaggo annotations documenting route, method, parameters, and responses | Must |
| AC-004 | `GET /v1/drugs/ndc/{ndc}` is documented with path parameter and all response schemas (200 DrugDetailResponse, 400 ErrorResponse, 404 ErrorResponse, 502 ErrorResponse) | Must |
| AC-005 | `GET /health` is documented with response schema (200 with status and version fields) | Must |
| AC-006 | Response model schemas (`DrugDetailResponse`, `ErrorResponse`) are documented via swaggo struct annotations | Must |
| AC-007 | The OpenAPI spec includes service metadata: title ("drug-gate API"), description, version, and base URL | Must |
| AC-008 | `swag init` generates the spec without errors from the annotated code | Must |
| AC-009 | The `/swagger/` and `/openapi.json` endpoints are themselves listed in the spec | Should |
| AC-010 | The generated spec validates against the OpenAPI 3.0 schema (no validation errors) | Should |

## 3. User Test Cases

### TC-001: View Swagger UI in browser

**Precondition:** Service is running.
**Steps:**
1. Open `http://localhost:8081/swagger/` in a browser
**Expected Result:** Swagger UI loads showing all documented endpoints. Endpoints are grouped and described.
**Screenshot Checkpoint:** N/A
**Maps to:** AC-002

### TC-002: Retrieve raw OpenAPI spec

**Precondition:** Service is running.
**Steps:**
1. `curl http://localhost:8081/openapi.json`
**Expected Result:** Returns valid JSON with `"openapi": "3.0"` at the top level. Contains paths for `/v1/drugs/ndc/{ndc}`, `/health`.
**Screenshot Checkpoint:** N/A
**Maps to:** AC-001, AC-009

### TC-003: Try NDC lookup from Swagger UI

**Precondition:** Service is running with cash-drugs reachable.
**Steps:**
1. Open Swagger UI
2. Expand `GET /v1/drugs/ndc/{ndc}`
3. Enter `58151-158` as the ndc
4. Click "Execute"
**Expected Result:** Response shows 200 with drug data matching the documented `DrugDetailResponse` schema.
**Screenshot Checkpoint:** N/A
**Maps to:** AC-004

### TC-004: Error responses documented

**Precondition:** Service is running.
**Steps:**
1. Open Swagger UI
2. Expand `GET /v1/drugs/ndc/{ndc}`
3. Check response schemas
**Expected Result:** 200, 400, 404, and 502 responses are documented with example bodies matching `DrugDetailResponse` and `ErrorResponse` schemas.
**Screenshot Checkpoint:** N/A
**Maps to:** AC-004, AC-006

### TC-005: Health endpoint documented

**Precondition:** Service is running.
**Steps:**
1. Open Swagger UI
2. Find `GET /health`
**Expected Result:** Endpoint is listed with 200 response schema showing `status` and `version` fields.
**Screenshot Checkpoint:** N/A
**Maps to:** AC-005

### TC-006: Spec generation from annotations

**Precondition:** swaggo/swag installed.
**Steps:**
1. Run `swag init -g cmd/server/main.go`
**Expected Result:** Generates `docs/swagger.json` and `docs/swagger.yaml` without errors.
**Screenshot Checkpoint:** N/A
**Maps to:** AC-008

## 4. Data Model

No new data entities. Documents existing response models:

| Model | Fields | Usage |
|-------|--------|-------|
| DrugDetailResponse | ndc (string), name (string), generic_name (string), classes ([]string) | 200 OK for NDC lookup |
| ErrorResponse | error (string), message (string) | 400, 404, 502 error responses |

## 5. API Contract

### New Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | /openapi.json | Returns the generated OpenAPI 3.0 spec as JSON |
| GET | /swagger/* | Serves Swagger UI (HTML + JS) with spec loaded |

### Documented Existing Endpoints

| Method | Path | Parameters | Responses |
|--------|------|------------|-----------|
| GET | /v1/drugs/ndc/{ndc} | ndc (path, required) — product NDC with dash (e.g. `58151-158`) | 200 DrugDetailResponse, 400 ErrorResponse (invalid_ndc), 404 ErrorResponse (not_found), 502 ErrorResponse (upstream_error) |
| GET | /health | none | 200 `{"status":"ok","version":"dev"}` |

## 6. UI Behavior

N/A — Swagger UI is provided by the swaggo library, no custom UI work.

## 7. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Swagger UI accessed when cash-drugs is down | UI loads fine, API calls return 502 with documented ErrorResponse |
| Invalid NDC entered in Swagger UI "Try it out" | Returns 400 with documented ErrorResponse (invalid_ndc) |
| Service running without swag init having been run | Build should include generated docs; if missing, endpoint returns 404 |
| Swagger UI accessed with trailing slash vs without | Both `/swagger/` and `/swagger/index.html` work |

## 8. Dependencies

- `github.com/swaggo/swag` — annotation parser and spec generator
- `github.com/swaggo/http-swagger` — Swagger UI middleware for net/http / Chi
- Existing handler and model packages (annotations added to existing code)

## 9. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-08 | 0.1.0 | calebdunn | Initial spec adapted from cash-drugs openapi-docs |

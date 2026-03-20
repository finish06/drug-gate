# Spec: Request ID Correlation

**Version:** 0.1.0
**Created:** 2026-03-20
**PRD Reference:** docs/prd.md — M7: Operational Hardening
**Status:** Approved

## 1. Overview

Add an `X-Request-ID` middleware that generates or propagates a unique request identifier through the entire request lifecycle. The ID appears in response headers and is injected into slog context so all log lines for a given request are correlated.

### User Story

As an **operator**, I want **every request to carry a unique X-Request-ID through logs and response headers**, so that **I can trace a single request across log lines when debugging issues in staging or production**.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | Middleware generates a UUID v4 `X-Request-ID` if the client does not provide one | Must |
| AC-002 | If the client sends an `X-Request-ID` header, the middleware uses it as-is | Must |
| AC-003 | The `X-Request-ID` is set in the response headers | Must |
| AC-004 | The request ID is added to the slog context so all downstream log calls include `request_id` | Must |
| AC-005 | The RequestLogger middleware includes `request_id` in its log output | Must |
| AC-006 | The middleware is wired before RequestLogger in the middleware chain | Must |
| AC-007 | Client-provided request IDs are truncated to 128 characters max (prevent header abuse) | Should |
| AC-008 | Empty `X-Request-ID` header is treated as absent (new ID generated) | Should |

## 3. User Test Cases

### TC-001: Auto-generated request ID

**Steps:**
1. Send `GET /v1/drugs/names` without an X-Request-ID header
2. Check response headers
**Expected Result:** Response includes `X-Request-ID` header with a valid UUID v4 value
**Maps to:** AC-001, AC-003

### TC-002: Client-provided request ID passthrough

**Steps:**
1. Send `GET /v1/drugs/names` with header `X-Request-ID: my-trace-abc123`
2. Check response headers
**Expected Result:** Response `X-Request-ID` is `my-trace-abc123`
**Maps to:** AC-002, AC-003

### TC-003: Request ID appears in logs

**Steps:**
1. Send a request and note the X-Request-ID from the response
2. Check structured log output
**Expected Result:** Log line for that request includes `"request_id": "{the-id}"`
**Maps to:** AC-004, AC-005

## 4. Data Model

No new data models. The request ID is a string stored in the request context.

### Context Key

| Key | Type | Description |
|-----|------|-------------|
| `requestIDKey` | unexported context key | Stores the request ID string in `context.Context` |

### Helper Function

`RequestIDFromContext(ctx context.Context) string` — extracts the request ID from context. Returns empty string if not set.

## 5. API Contract

No new endpoints. Modifies all existing endpoints by adding `X-Request-ID` to response headers.

**Request Header (optional):**
- `X-Request-ID: {client-provided-id}` — if present and non-empty, used as-is (truncated to 128 chars)

**Response Header (always):**
- `X-Request-ID: {uuid-or-client-id}` — the request ID used for this request

## 6. Implementation Notes

- New file: `internal/middleware/requestid.go`
- UUID generation: use `crypto/rand` to generate UUID v4 (no external dependency)
- Context propagation: use `context.WithValue` with unexported key type
- slog integration: use `slog.With("request_id", id)` and inject into the request context via `slog.NewLogLogger` or by wrapping the handler with a logger that includes the ID
- Wire in `cmd/server/main.go` as the FIRST middleware (before RequestLogger)

## 7. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Empty X-Request-ID header | Generate new UUID (treat as absent) |
| Very long X-Request-ID (>128 chars) | Truncate to 128 characters |
| Non-ASCII X-Request-ID | Accept as-is (HTTP headers allow printable ASCII) |
| Concurrent requests | Each request gets its own context-scoped ID |

## 8. Dependencies

- No external dependencies (UUID v4 from crypto/rand)
- Modifies: `internal/middleware/logging.go` (add request_id to log output)
- Modifies: `cmd/server/main.go` (wire middleware)

## 9. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-20 | 0.1.0 | calebdunn | Initial spec |

package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAC001_GeneratesRequestID verifies that the middleware generates a UUID
// when the client does not provide an X-Request-ID header.
func TestAC001_GeneratesRequestID(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestID(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	rid := rr.Header().Get("X-Request-ID")
	if rid == "" {
		t.Fatal("expected X-Request-ID in response, got empty")
	}
	// UUID v4 format: 8-4-4-4-12 hex digits
	parts := strings.Split(rid, "-")
	if len(parts) != 5 {
		t.Errorf("expected UUID format (5 parts), got %q", rid)
	}
}

// TestAC002_PassthroughClientRequestID verifies that the middleware uses
// a client-provided X-Request-ID header as-is.
func TestAC002_PassthroughClientRequestID(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestID(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "my-trace-abc123")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	rid := rr.Header().Get("X-Request-ID")
	if rid != "my-trace-abc123" {
		t.Errorf("expected X-Request-ID = %q, got %q", "my-trace-abc123", rid)
	}
}

// TestAC003_ResponseHeaderSet verifies X-Request-ID is always in the response.
func TestAC003_ResponseHeaderSet(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := RequestID(inner)
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID in response header")
	}
}

// TestAC004_RequestIDInContext verifies that the request ID is stored in the
// request context and accessible via RequestIDFromContext.
func TestAC004_RequestIDInContext(t *testing.T) {
	var ctxID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestID(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "ctx-test-123")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if ctxID != "ctx-test-123" {
		t.Errorf("context request ID = %q, want %q", ctxID, "ctx-test-123")
	}
	// Also verify it matches the response header
	if rr.Header().Get("X-Request-ID") != ctxID {
		t.Errorf("response header %q != context %q", rr.Header().Get("X-Request-ID"), ctxID)
	}
}

// TestAC007_TruncateLongRequestID verifies that client-provided IDs longer
// than 128 characters are truncated.
func TestAC007_TruncateLongRequestID(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestID(inner)
	longID := strings.Repeat("a", 200)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", longID)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	rid := rr.Header().Get("X-Request-ID")
	if len(rid) != 128 {
		t.Errorf("expected truncated ID length 128, got %d", len(rid))
	}
}

// TestAC008_EmptyHeaderGeneratesNew verifies that an empty X-Request-ID
// header is treated as absent (new ID generated).
func TestAC008_EmptyHeaderGeneratesNew(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestID(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	rid := rr.Header().Get("X-Request-ID")
	if rid == "" {
		t.Fatal("expected generated X-Request-ID, got empty")
	}
	// Should be a UUID, not empty string
	parts := strings.Split(rid, "-")
	if len(parts) != 5 {
		t.Errorf("expected UUID format, got %q", rid)
	}
}

// TestRequestIDFromContext_NoValue verifies the helper returns empty
// when no request ID is in the context.
func TestRequestIDFromContext_NoValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	id := RequestIDFromContext(req.Context())
	if id != "" {
		t.Errorf("expected empty string, got %q", id)
	}
}

// TestAC005_RequestLoggerIncludesRequestID verifies that when RequestID
// middleware runs before RequestLogger, the log output includes request_id.
func TestAC005_RequestLoggerIncludesRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	slog.SetDefault(logger)
	defer slog.SetDefault(slog.Default())

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Chain: RequestID → RequestLogger → handler
	handler := RequestID(RequestLogger(inner))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "log-test-456")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "log-test-456") {
		t.Errorf("expected log output to contain request_id, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "request_id") {
		t.Errorf("expected log output to contain 'request_id' key, got: %s", logOutput)
	}
}

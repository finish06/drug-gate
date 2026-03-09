package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/finish06/drug-gate/internal/model"
)

// TestAdminAuth_AC014_ValidSecret verifies that a request with a valid
// Bearer token passes through the middleware and reaches the inner handler.
func TestAdminAuth_AC014_ValidSecret(t *testing.T) {
	const secret = "super-secret-admin-key"

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	handler := AdminAuth(secret)(inner)

	req := httptest.NewRequest(http.MethodGet, "/admin/keys", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != "ok" {
		t.Errorf("body = %q, want %q", rr.Body.String(), "ok")
	}
}

// TestAdminAuth_AC015_MissingHeader verifies that a request without an
// Authorization header is rejected with 401 and an ErrorResponse body.
func TestAdminAuth_AC015_MissingHeader(t *testing.T) {
	const secret = "super-secret-admin-key"

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not be called when Authorization header is missing")
	})

	handler := AdminAuth(secret)(inner)

	req := httptest.NewRequest(http.MethodGet, "/admin/keys", nil)
	// No Authorization header set.
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}

	var errResp model.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if errResp.Error == "" {
		t.Error("expected non-empty Error field in response")
	}
}

// TestAdminAuth_AC015_WrongSecret verifies that a request with an incorrect
// Bearer token is rejected with 401 and an ErrorResponse body.
func TestAdminAuth_AC015_WrongSecret(t *testing.T) {
	const secret = "super-secret-admin-key"

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not be called with wrong secret")
	})

	handler := AdminAuth(secret)(inner)

	req := httptest.NewRequest(http.MethodGet, "/admin/keys", nil)
	req.Header.Set("Authorization", "Bearer wrong-secret")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}

	var errResp model.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if errResp.Error == "" {
		t.Error("expected non-empty Error field in response")
	}
}

// TestAdminAuth_AC015_MalformedAuth verifies that a request with an
// Authorization header that lacks the "Bearer " prefix is rejected with 401.
func TestAdminAuth_AC015_MalformedAuth(t *testing.T) {
	const secret = "super-secret-admin-key"

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not be called with malformed Authorization")
	})

	handler := AdminAuth(secret)(inner)

	req := httptest.NewRequest(http.MethodGet, "/admin/keys", nil)
	req.Header.Set("Authorization", secret) // Missing "Bearer " prefix
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}

	var errResp model.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if errResp.Error == "" {
		t.Error("expected non-empty Error field in response")
	}
}

// TestAdminAuth_AC015_BasicAuthScheme verifies that using "Basic" instead of
// "Bearer" is rejected with 401.
func TestAdminAuth_AC015_BasicAuthScheme(t *testing.T) {
	const secret = "super-secret-admin-key"

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not be called with Basic auth scheme")
	})

	handler := AdminAuth(secret)(inner)

	req := httptest.NewRequest(http.MethodGet, "/admin/keys", nil)
	req.Header.Set("Authorization", "Basic "+secret)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

// TestAdminAuth_AC014_ValidSecret_POST verifies the middleware works for
// non-GET methods (POST) as well.
func TestAdminAuth_AC014_ValidSecret_POST(t *testing.T) {
	const secret = "my-admin-secret"

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	handler := AdminAuth(secret)(inner)

	req := httptest.NewRequest(http.MethodPost, "/admin/keys", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rr.Code)
	}
}

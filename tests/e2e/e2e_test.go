//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

var (
	baseURL     string
	adminSecret string
)

func TestMain(m *testing.M) {
	baseURL = os.Getenv("DRUG_GATE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8081"
	}
	adminSecret = os.Getenv("ADMIN_SECRET")
	if adminSecret == "" {
		adminSecret = "e2e-test-secret"
	}

	// Wait for drug-gate to be ready
	ready := false
	for i := 0; i < 30; i++ {
		resp, err := http.Get(baseURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			ready = true
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(time.Second)
	}
	if !ready {
		fmt.Fprintf(os.Stderr, "drug-gate not ready at %s after 30s\n", baseURL)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// --- Health ---

func TestE2E_Health(t *testing.T) {
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want %q", body["status"], "ok")
	}
}

// --- Public endpoints exempt from auth (AC-019) ---

func TestE2E_AC019_HealthNoAuth(t *testing.T) {
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("health should not require auth, got status %d", resp.StatusCode)
	}
}

func TestE2E_AC019_SwaggerNoAuth(t *testing.T) {
	resp, err := http.Get(baseURL + "/swagger/index.html")
	if err != nil {
		t.Fatalf("GET /swagger: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("swagger should not require auth, got status %d", resp.StatusCode)
	}
}

// --- Auth enforcement (AC-001, AC-002) ---

func TestE2E_AC001_MissingAPIKey(t *testing.T) {
	resp, err := http.Get(baseURL + "/v1/drugs/ndc/00069-3150")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestE2E_AC002_InvalidAPIKey(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/v1/drugs/ndc/00069-3150", nil)
	req.Header.Set("X-API-Key", "pk_bogus_key_that_does_not_exist")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// --- Admin key management (AC-014, AC-015, AC-011) ---

func TestE2E_AC015_AdminNoAuth(t *testing.T) {
	resp, err := http.Get(baseURL + "/admin/keys")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func adminRequest(method, path string, body interface{}) (*http.Response, error) {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = &bytes.Buffer{}
	}

	req, err := http.NewRequest(method, baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+adminSecret)
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}

func createTestKey(t *testing.T, appName string, rateLimit int) string {
	t.Helper()
	resp, err := adminRequest(http.MethodPost, "/admin/keys", map[string]interface{}{
		"app_name":   appName,
		"origins":    []string{},
		"rate_limit": rateLimit,
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create key: status = %d, want 201", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	key, ok := result["key"].(string)
	if !ok || key == "" {
		t.Fatal("create key: missing key in response")
	}
	return key
}

func TestE2E_AC011_CreateAndListKeys(t *testing.T) {
	key := createTestKey(t, "e2e-test-app", 250)

	// List keys and verify it's there
	resp, err := adminRequest(http.MethodGet, "/admin/keys", nil)
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list keys: status = %d, want 200", resp.StatusCode)
	}

	var keys []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&keys)

	found := false
	for _, k := range keys {
		if k["key"] == key {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("created key %s not found in list", key)
	}
}

// --- Full NDC lookup through the stack (AC-003, AC-009) ---

func TestE2E_AC003_AuthenticatedNDCLookup(t *testing.T) {
	key := createTestKey(t, "e2e-ndc-lookup", 250)

	// Retry a few times — cash-drugs may still be starting up
	var resp *http.Response
	for attempt := 0; attempt < 10; attempt++ {
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/v1/drugs/ndc/0069-3150", nil)
		req.Header.Set("X-API-Key", key)

		var err error
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		if resp.StatusCode != http.StatusBadGateway {
			break
		}
		resp.Body.Close()
		t.Logf("attempt %d: upstream not ready (502), retrying...", attempt+1)
		time.Sleep(2 * time.Second)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (key: %s)", resp.StatusCode, key)
	}

	// AC-009: rate limit headers present
	if resp.Header.Get("X-RateLimit-Remaining") == "" {
		t.Error("missing X-RateLimit-Remaining header")
	}
	if resp.Header.Get("X-RateLimit-Reset") == "" {
		t.Error("missing X-RateLimit-Reset header")
	}

	var drug map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&drug); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if drug["ndc"] == nil || drug["ndc"] == "" {
		t.Error("expected non-empty ndc in response")
	}
	// generic_name should always be present from FDA data
	if drug["generic_name"] == nil || drug["generic_name"] == "" {
		t.Error("expected non-empty generic_name in response")
	}

	t.Logf("Drug: name=%v, generic=%v, ndc=%v", drug["name"], drug["generic_name"], drug["ndc"])
}

// --- Rate limiting (AC-007, AC-008) ---

func TestE2E_AC007_RateLimitEnforced(t *testing.T) {
	// Create a key with a very low rate limit
	key := createTestKey(t, "e2e-ratelimit", 3)

	doReq := func() *http.Response {
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/v1/drugs/ndc/00069-3150", nil)
		req.Header.Set("X-API-Key", key)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		return resp
	}

	// Use up the limit
	for i := 0; i < 3; i++ {
		resp := doReq()
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("rate limited too early on request %d", i+1)
		}
	}

	// Next request should be rate limited
	resp := doReq()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", resp.StatusCode)
	}

	// AC-008: Retry-After header
	if resp.Header.Get("Retry-After") == "" {
		t.Error("missing Retry-After header on 429 response")
	}
}

// --- Key deactivation (AC-012) ---

func TestE2E_AC012_DeactivateKey(t *testing.T) {
	key := createTestKey(t, "e2e-deactivate", 250)

	// Verify it works first
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/v1/drugs/ndc/00069-3150", nil)
	req.Header.Set("X-API-Key", key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatal("key should be valid before deactivation")
	}

	// Deactivate
	resp, err = adminRequest(http.MethodDelete, "/admin/keys/"+key, nil)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("deactivate: status = %d, want 200", resp.StatusCode)
	}

	// Now the key should be rejected
	req, _ = http.NewRequest(http.MethodGet, baseURL+"/v1/drugs/ndc/00069-3150", nil)
	req.Header.Set("X-API-Key", key)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET after deactivate: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("deactivated key should return 401, got %d", resp.StatusCode)
	}
}

// --- Key rotation (AC-013) ---

func TestE2E_AC013_RotateKey(t *testing.T) {
	oldKey := createTestKey(t, "e2e-rotate", 250)

	// Rotate with 1 hour grace period
	resp, err := adminRequest(http.MethodPost, "/admin/keys/"+oldKey+"/rotate", map[string]interface{}{
		"grace_period": "1h",
	})
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rotate: status = %d, want 200", resp.StatusCode)
	}

	var rotateResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rotateResp)

	newKey, _ := rotateResp["new_key"].(string)
	if newKey == "" {
		t.Fatal("expected new_key in rotate response")
	}
	if rotateResp["old_key_expires_at"] == nil {
		t.Fatal("expected old_key_expires_at in rotate response")
	}

	// Both keys should work during grace period
	for _, key := range []string{oldKey, newKey} {
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/v1/drugs/ndc/00069-3150", nil)
		req.Header.Set("X-API-Key", key)
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET with %s: %v", key[:10], err)
		}
		r.Body.Close()
		if r.StatusCode == http.StatusUnauthorized {
			t.Errorf("key %s... should work during grace period, got 401", key[:10])
		}
	}

	t.Logf("Old key: %s..., New key: %s...", oldKey[:10], newKey[:10])
}

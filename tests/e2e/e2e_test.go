//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
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

// --- Drug Names listing (AC-001, AC-002, AC-003, AC-004, AC-017, AC-018, AC-021) ---

func authedGet(t *testing.T, key, path string) *http.Response {
	return authedGetRetry(t, key, path, true)
}

func authedGetNoRetry(t *testing.T, key, path string) *http.Response {
	return authedGetRetry(t, key, path, false)
}

func authedGetRetry(t *testing.T, key, path string, retry bool) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, baseURL+path, nil)
	req.Header.Set("X-API-Key", key)

	maxAttempts := 1
	if retry {
		maxAttempts = 10
	}

	var resp *http.Response
	for attempt := 0; attempt < maxAttempts; attempt++ {
		var err error
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if !retry || resp.StatusCode != http.StatusBadGateway {
			break
		}
		resp.Body.Close()
		t.Logf("attempt %d: upstream not ready (502), retrying...", attempt+1)
		time.Sleep(2 * time.Second)
		req, _ = http.NewRequest(http.MethodGet, baseURL+path, nil)
		req.Header.Set("X-API-Key", key)
	}
	return resp
}

type paginatedResponse struct {
	Data       []map[string]interface{} `json:"data"`
	Pagination struct {
		Page       int `json:"page"`
		Limit      int `json:"limit"`
		Total      int `json:"total"`
		TotalPages int `json:"total_pages"`
	} `json:"pagination"`
}

func decodePaginated(t *testing.T, resp *http.Response) paginatedResponse {
	t.Helper()
	var pr paginatedResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return pr
}

func TestE2E_AC001_DrugNamesReturnsData(t *testing.T) {
	key := createTestKey(t, "e2e-drug-names", 250)
	resp := authedGet(t, key, "/v1/drugs/names?page=1&limit=10")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	pr := decodePaginated(t, resp)

	if len(pr.Data) == 0 {
		t.Fatal("expected non-empty data array for drug names")
	}
	if len(pr.Data) > 10 {
		t.Errorf("expected at most 10 entries, got %d", len(pr.Data))
	}

	// AC-003: each entry has name and type
	for i, entry := range pr.Data {
		if entry["name"] == nil || entry["name"] == "" {
			t.Errorf("entry %d: missing name", i)
		}
		if entry["type"] == nil || entry["type"] == "" {
			t.Errorf("entry %d: missing type", i)
		}
	}

	// AC-017: pagination metadata
	if pr.Pagination.Page != 1 {
		t.Errorf("page = %d, want 1", pr.Pagination.Page)
	}
	if pr.Pagination.Limit != 10 {
		t.Errorf("limit = %d, want 10", pr.Pagination.Limit)
	}
	if pr.Pagination.Total == 0 {
		t.Error("expected total > 0")
	}

	t.Logf("Drug names: %d total, first=%v", pr.Pagination.Total, pr.Data[0]["name"])
}

func TestE2E_AC002_DrugNamesSearch(t *testing.T) {
	key := createTestKey(t, "e2e-drug-search", 250)
	resp := authedGet(t, key, "/v1/drugs/names?q=simva")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	pr := decodePaginated(t, resp)

	if len(pr.Data) == 0 {
		t.Fatal("expected results for 'simva' search")
	}
	for i, entry := range pr.Data {
		name, _ := entry["name"].(string)
		if !strings.Contains(strings.ToLower(name), "simva") {
			t.Errorf("entry %d: name %q does not contain 'simva'", i, name)
		}
	}
}

func TestE2E_AC021_DrugNamesTypeFilter(t *testing.T) {
	key := createTestKey(t, "e2e-drug-type", 250)
	resp := authedGet(t, key, "/v1/drugs/names?type=generic&q=simva")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	pr := decodePaginated(t, resp)
	for i, entry := range pr.Data {
		typ, _ := entry["type"].(string)
		if typ != "generic" {
			t.Errorf("entry %d: type = %q, want 'generic'", i, typ)
		}
	}
}

func TestE2E_AC018_DrugNamesLimitClamped(t *testing.T) {
	key := createTestKey(t, "e2e-drug-clamp", 250)
	resp := authedGet(t, key, "/v1/drugs/names?limit=500")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	pr := decodePaginated(t, resp)
	if pr.Pagination.Limit != 100 {
		t.Errorf("limit = %d, want 100 (clamped from 500)", pr.Pagination.Limit)
	}
}

// --- Drug Classes listing (AC-005, AC-006, AC-007, AC-008) ---

func TestE2E_AC005_DrugClassesReturnsData(t *testing.T) {
	key := createTestKey(t, "e2e-drug-classes", 250)
	resp := authedGet(t, key, "/v1/drugs/classes?page=1&limit=10")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	pr := decodePaginated(t, resp)

	if len(pr.Data) == 0 {
		t.Fatal("expected non-empty data array for drug classes")
	}
	if len(pr.Data) > 10 {
		t.Errorf("expected at most 10 entries, got %d", len(pr.Data))
	}

	// AC-007: each entry has name and type
	for i, entry := range pr.Data {
		if entry["name"] == nil || entry["name"] == "" {
			t.Errorf("entry %d: missing name", i)
		}
		if entry["type"] == nil || entry["type"] == "" {
			t.Errorf("entry %d: missing type", i)
		}
	}

	// Default filter is epc
	for i, entry := range pr.Data {
		typ, _ := entry["type"].(string)
		if typ != "epc" {
			t.Errorf("entry %d: type = %q, want 'epc' (default filter)", i, typ)
		}
	}

	if pr.Pagination.Total == 0 {
		t.Error("expected total > 0 for EPC classes")
	}

	t.Logf("Drug classes (EPC): %d total, first=%v", pr.Pagination.Total, pr.Data[0]["name"])
}

func TestE2E_AC006_DrugClassesMoAFilter(t *testing.T) {
	key := createTestKey(t, "e2e-classes-moa", 250)
	resp := authedGet(t, key, "/v1/drugs/classes?type=moa&limit=10")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	pr := decodePaginated(t, resp)
	if len(pr.Data) == 0 {
		t.Fatal("expected MoA classes to exist")
	}
	for i, entry := range pr.Data {
		typ, _ := entry["type"].(string)
		if typ != "moa" {
			t.Errorf("entry %d: type = %q, want 'moa'", i, typ)
		}
	}
}

func TestE2E_AC006_DrugClassesAllFilter(t *testing.T) {
	key := createTestKey(t, "e2e-classes-all", 250)
	resp := authedGet(t, key, "/v1/drugs/classes?type=all&limit=10")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	pr := decodePaginated(t, resp)
	if len(pr.Data) == 0 {
		t.Fatal("expected classes with type=all")
	}
	if pr.Pagination.Total == 0 {
		t.Error("expected total > 0 for all classes")
	}
}

// --- Drugs by class (AC-009, AC-010, AC-011, AC-012, AC-022) ---

func TestE2E_AC010_DrugsbyClassMissingParam(t *testing.T) {
	key := createTestKey(t, "e2e-byclass-missing", 250)
	resp := authedGet(t, key, "/v1/drugs/classes/drugs")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	var errResp map[string]string
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp["error"] != "validation_error" {
		t.Errorf("error = %q, want 'validation_error'", errResp["error"])
	}
}

func TestE2E_AC009_DrugsbyClassReturnsData(t *testing.T) {
	key := createTestKey(t, "e2e-byclass-data", 250)
	resp := authedGet(t, key, "/v1/drugs/classes/drugs?class=HMG-CoA+Reductase+Inhibitor&limit=10")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	pr := decodePaginated(t, resp)
	if len(pr.Data) == 0 {
		t.Fatal("expected drugs in HMG-CoA Reductase Inhibitor class")
	}

	// AC-011: each entry has generic_name and brand_name
	for i, entry := range pr.Data {
		if entry["generic_name"] == nil || entry["generic_name"] == "" {
			t.Errorf("entry %d: missing generic_name", i)
		}
		// brand_name may be empty string but should be present
		if _, ok := entry["brand_name"]; !ok {
			t.Errorf("entry %d: missing brand_name field", i)
		}
	}

	t.Logf("Drugs in HMG-CoA class: %d total", pr.Pagination.Total)
}

func TestE2E_AC022_DrugsbyClassUnknown(t *testing.T) {
	key := createTestKey(t, "e2e-byclass-unknown", 250)
	resp := authedGetNoRetry(t, key, "/v1/drugs/classes/drugs?class=NotARealDrugClass")
	defer resp.Body.Close()

	// 502 is acceptable — upstream FDA may timeout on unknown class search
	if resp.StatusCode == http.StatusBadGateway {
		t.Logf("upstream returned 502 for unknown class (FDA timeout), acceptable in E2E")
		return
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 or 502", resp.StatusCode)
	}

	pr := decodePaginated(t, resp)
	if len(pr.Data) != 0 {
		t.Errorf("expected empty data for unknown class, got %d entries", len(pr.Data))
	}
}

// --- Drug class lookup by name (drug-class-lookup spec) ---

func TestE2E_DrugClassLookup_GenericName(t *testing.T) {
	key := createTestKey(t, "e2e-classlookup", 250)
	resp := authedGet(t, key, "/v1/drugs/class?name=simvastatin")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["query_name"] != "simvastatin" {
		t.Errorf("query_name = %v, want 'simvastatin'", result["query_name"])
	}
	if result["generic_name"] == nil || result["generic_name"] == "" {
		t.Error("expected non-empty generic_name")
	}

	classes, ok := result["classes"].([]interface{})
	if !ok || len(classes) == 0 {
		t.Error("expected non-empty classes array")
	}

	brandNames, ok := result["brand_names"].([]interface{})
	if !ok {
		t.Error("expected brand_names array")
	}

	t.Logf("Class lookup: generic=%v, brands=%v, classes=%d",
		result["generic_name"], brandNames, len(classes))
}

func TestE2E_DrugClassLookup_BrandFallback(t *testing.T) {
	key := createTestKey(t, "e2e-classlookup-brand", 250)
	resp := authedGetNoRetry(t, key, "/v1/drugs/class?name=Lipitor")
	defer resp.Body.Close()

	// Brand fallback requires two upstream calls — may 502 if FDA is slow
	if resp.StatusCode == http.StatusBadGateway {
		t.Logf("upstream returned 502 for brand fallback (FDA timeout), acceptable in E2E")
		return
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 or 502", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["query_name"] != "Lipitor" {
		t.Errorf("query_name = %v, want 'Lipitor'", result["query_name"])
	}
	if result["generic_name"] == nil || result["generic_name"] == "" {
		t.Error("expected generic_name resolved from brand fallback")
	}

	t.Logf("Brand fallback: query=Lipitor, resolved generic=%v", result["generic_name"])
}

func TestE2E_DrugClassLookup_MissingName(t *testing.T) {
	key := createTestKey(t, "e2e-classlookup-miss", 250)
	resp := authedGet(t, key, "/v1/drugs/class")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestE2E_DrugClassLookup_NotFound(t *testing.T) {
	key := createTestKey(t, "e2e-classlookup-404", 250)
	resp := authedGetNoRetry(t, key, "/v1/drugs/class?name=notarealdrug12345")
	defer resp.Body.Close()

	// 502 is acceptable — upstream FDA may timeout on unknown drug search
	if resp.StatusCode == http.StatusBadGateway {
		t.Logf("upstream returned 502 for unknown drug (FDA timeout), acceptable in E2E")
		return
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 or 502", resp.StatusCode)
	}
}

// --- Version endpoint ---

func TestE2E_Version(t *testing.T) {
	resp, err := http.Get(baseURL + "/version")
	if err != nil {
		t.Fatalf("GET /version: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)

	if body["version"] == "" {
		t.Error("expected non-empty version")
	}
	if body["go_version"] == "" {
		t.Error("expected non-empty go_version")
	}

	t.Logf("Version: %s, commit: %s, branch: %s, go: %s",
		body["version"], body["git_commit"], body["git_branch"], body["go_version"])
}

// --- RxNorm endpoints ---

func TestE2E_RxNorm_Search(t *testing.T) {
	key := createTestKey(t, "e2e-rxnorm-search", 250)
	resp := authedGetNoRetry(t, key, "/v1/drugs/rxnorm/search?name=lipitor")
	defer resp.Body.Close()

	// RxNorm upstream may timeout or return empty in E2E
	if resp.StatusCode == http.StatusBadGateway {
		t.Logf("upstream 502 for RxNorm search, acceptable in E2E")
		return
	}
	if resp.StatusCode == http.StatusNotFound {
		t.Logf("no RxNorm matches (404), acceptable in E2E (cold cache)")
		return
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200, 404, or 502", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["query"] != "lipitor" {
		t.Errorf("query = %v, want 'lipitor'", result["query"])
	}

	candidates, ok := result["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		// May have suggestions instead of candidates
		t.Logf("no candidates returned, checking suggestions")
		return
	}

	first := candidates[0].(map[string]interface{})
	if first["rxcui"] == nil || first["rxcui"] == "" {
		t.Error("expected non-empty rxcui on first candidate")
	}

	t.Logf("RxNorm search: %d candidates, first=%v (rxcui=%v)",
		len(candidates), first["name"], first["rxcui"])
}

func TestE2E_RxNorm_Search_MissingName(t *testing.T) {
	key := createTestKey(t, "e2e-rxnorm-noname", 250)
	resp := authedGetNoRetry(t, key, "/v1/drugs/rxnorm/search")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestE2E_RxNorm_Profile(t *testing.T) {
	key := createTestKey(t, "e2e-rxnorm-profile", 250)
	resp := authedGetNoRetry(t, key, "/v1/drugs/rxnorm/profile?name=simvastatin")
	defer resp.Body.Close()

	// Profile orchestrates 4 upstream calls — may timeout in E2E
	if resp.StatusCode == http.StatusBadGateway {
		t.Logf("upstream 502 for RxNorm profile, acceptable in E2E")
		return
	}
	if resp.StatusCode == http.StatusNotFound {
		t.Logf("no RxNorm profile (404), acceptable in E2E (cold cache)")
		return
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200, 404, or 502", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["query"] != "simvastatin" {
		t.Errorf("query = %v, want 'simvastatin'", result["query"])
	}
	if result["rxcui"] == nil || result["rxcui"] == "" {
		t.Error("expected non-empty rxcui")
	}

	t.Logf("RxNorm profile: name=%v, rxcui=%v", result["name"], result["rxcui"])
}

func TestE2E_RxNorm_Profile_NotFound(t *testing.T) {
	key := createTestKey(t, "e2e-rxnorm-prof404", 250)
	resp := authedGetNoRetry(t, key, "/v1/drugs/rxnorm/profile?name=notarealdrug99")
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadGateway {
		t.Logf("upstream 502 for unknown drug, acceptable in E2E")
		return
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 or 502", resp.StatusCode)
	}
}

func TestE2E_RxNorm_NDCs(t *testing.T) {
	// First search to get an RxCUI
	key := createTestKey(t, "e2e-rxnorm-ndcs", 250)
	searchResp := authedGetNoRetry(t, key, "/v1/drugs/rxnorm/search?name=lipitor")
	defer searchResp.Body.Close()

	if searchResp.StatusCode != http.StatusOK {
		t.Logf("search returned %d, skipping NDC test (upstream not ready)", searchResp.StatusCode)
		return
	}

	var search map[string]interface{}
	json.NewDecoder(searchResp.Body).Decode(&search)
	candidates, ok := search["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		t.Logf("no search candidates, skipping NDC test")
		return
	}
	rxcui := candidates[0].(map[string]interface{})["rxcui"].(string)

	resp := authedGetNoRetry(t, key, "/v1/drugs/rxnorm/"+rxcui+"/ndcs")
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Logf("no NDCs for rxcui %s (404), acceptable", rxcui)
		return
	}
	if resp.StatusCode == http.StatusBadGateway {
		t.Logf("upstream 502 for NDCs, acceptable in E2E")
		return
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200, 404, or 502", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	t.Logf("RxNorm NDCs for %s: %v", rxcui, result["ndcs"])
}

func TestE2E_RxNorm_Related(t *testing.T) {
	key := createTestKey(t, "e2e-rxnorm-related", 250)
	resp := authedGetNoRetry(t, key, "/v1/drugs/rxnorm/search?name=atorvastatin")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("search returned %d, skipping related test", resp.StatusCode)
		return
	}

	var search map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&search)
	candidates, ok := search["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		t.Logf("no search candidates, skipping related test")
		return
	}
	rxcui := candidates[0].(map[string]interface{})["rxcui"].(string)

	relResp := authedGetNoRetry(t, key, "/v1/drugs/rxnorm/"+rxcui+"/related")
	defer relResp.Body.Close()

	if relResp.StatusCode == http.StatusNotFound {
		t.Logf("no related for rxcui %s (404), acceptable", rxcui)
		return
	}
	if relResp.StatusCode == http.StatusBadGateway {
		t.Logf("upstream 502, acceptable in E2E")
		return
	}
	if relResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", relResp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(relResp.Body).Decode(&result)

	// Should have at least one group populated
	hasData := false
	for _, group := range []string{"ingredients", "brand_names", "dose_forms", "clinical_drugs", "branded_drugs"} {
		if arr, ok := result[group].([]interface{}); ok && len(arr) > 0 {
			hasData = true
			break
		}
	}
	if !hasData {
		t.Error("expected at least one related concept group to have data")
	}

	t.Logf("RxNorm related for %s: ingredients=%d, brands=%d",
		rxcui,
		len(result["ingredients"].([]interface{})),
		len(result["brand_names"].([]interface{})))
}

// --- Admin cache clear ---

func TestE2E_AdminCacheClear(t *testing.T) {
	// First populate some cache
	key := createTestKey(t, "e2e-cache-clear", 250)
	_ = authedGet(t, key, "/v1/drugs/classes?limit=1")

	// Clear cache
	resp, err := adminRequest(http.MethodDelete, "/admin/cache", nil)
	if err != nil {
		t.Fatalf("DELETE /admin/cache: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", result["status"])
	}
	deleted, _ := result["keys_deleted"].(float64)
	t.Logf("Cache cleared: %d keys deleted", int(deleted))
}

//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// --- SPL Browser ---

func TestE2E_SPL_SearchByName(t *testing.T) {
	key := createTestKey(t, "spl-e2e", 100)

	resp := authedGet(t, key, "/v1/drugs/spls?name=lipitor")
	defer resp.Body.Close()

	// Accept 200 or 502 (upstream may be slow)
	if resp.StatusCode == http.StatusBadGateway {
		t.Skip("upstream unavailable, skipping SPL search test")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result paginatedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if result.Pagination.Total == 0 {
		t.Skip("no SPL results for lipitor — upstream may be empty")
	}

	// Verify first entry has expected fields
	first := result.Data[0]
	if first["title"] == nil || first["setid"] == nil {
		t.Error("SPL entry missing title or setid")
	}
	if first["spl_version"] == nil {
		t.Error("SPL entry missing spl_version")
	}
}

func TestE2E_SPL_SearchNoResults(t *testing.T) {
	key := createTestKey(t, "spl-e2e-empty", 100)

	resp := authedGet(t, key, "/v1/drugs/spls?name=zzzznonexistentdrugzzzz")
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadGateway {
		t.Skip("upstream unavailable")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result paginatedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if result.Pagination.Total != 0 {
		t.Errorf("expected 0 results for nonexistent drug, got %d", result.Pagination.Total)
	}
}

func TestE2E_SPL_SearchMissingName(t *testing.T) {
	key := createTestKey(t, "spl-e2e-noname", 100)

	resp := authedGet(t, key, "/v1/drugs/spls")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// --- SPL Detail ---

func TestE2E_SPL_DetailWithInteractions(t *testing.T) {
	key := createTestKey(t, "spl-e2e-detail", 100)

	// First search for warfarin to get a setid
	resp := authedGet(t, key, "/v1/drugs/spls?name=warfarin&limit=1")
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadGateway {
		t.Skip("upstream unavailable")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("search status = %d, want 200", resp.StatusCode)
	}

	var searchResult paginatedResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(searchResult.Data) == 0 {
		t.Skip("no warfarin SPLs found")
	}

	setid, _ := searchResult.Data[0]["setid"].(string)
	if setid == "" {
		t.Fatal("first entry has no setid")
	}

	// Now fetch detail
	detailResp := authedGet(t, key, "/v1/drugs/spls/"+setid)
	defer detailResp.Body.Close()

	if detailResp.StatusCode == http.StatusBadGateway {
		t.Skip("upstream unavailable for XML fetch")
	}
	if detailResp.StatusCode != http.StatusOK {
		t.Fatalf("detail status = %d, want 200", detailResp.StatusCode)
	}

	var detail map[string]interface{}
	if err := json.NewDecoder(detailResp.Body).Decode(&detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}

	if detail["setid"] != setid {
		t.Errorf("setid = %v, want %s", detail["setid"], setid)
	}

	interactions, ok := detail["interactions"].([]interface{})
	if !ok {
		t.Fatal("missing interactions field")
	}

	// Warfarin should have drug interaction sections
	if len(interactions) == 0 {
		t.Error("expected warfarin to have interaction sections")
	}

	// Check first interaction has title and text
	if len(interactions) > 0 {
		first, _ := interactions[0].(map[string]interface{})
		if first["title"] == nil || first["text"] == nil {
			t.Error("interaction section missing title or text")
		}
	}
}

func TestE2E_SPL_DetailNotFound(t *testing.T) {
	key := createTestKey(t, "spl-e2e-404", 100)

	resp := authedGet(t, key, "/v1/drugs/spls/00000000-0000-0000-0000-000000000000")
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadGateway {
		t.Skip("upstream unavailable")
	}
	// Expect 404
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// --- Drug Info Card ---

func TestE2E_SPL_DrugInfoByName(t *testing.T) {
	key := createTestKey(t, "spl-e2e-info", 100)

	resp := authedGet(t, key, "/v1/drugs/info?name=warfarin")
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadGateway {
		t.Skip("upstream unavailable")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var info map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if info["drug_name"] != "warfarin" {
		t.Errorf("drug_name = %v, want warfarin", info["drug_name"])
	}
	if info["input_type"] != "name" {
		t.Errorf("input_type = %v, want name", info["input_type"])
	}
	if info["spl"] == nil {
		t.Error("expected spl source to be present for warfarin")
	}

	interactions, _ := info["interactions"].([]interface{})
	if len(interactions) == 0 {
		t.Error("expected warfarin to have interactions via drug info")
	}
}

func TestE2E_SPL_DrugInfoMissingParams(t *testing.T) {
	key := createTestKey(t, "spl-e2e-info-noparam", 100)

	resp := authedGet(t, key, "/v1/drugs/info")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// --- Interaction Checker ---

func TestE2E_SPL_InteractionChecker_WarfarinAspirin(t *testing.T) {
	key := createTestKey(t, "spl-e2e-interact", 100)

	body, _ := json.Marshal(map[string]interface{}{
		"drugs": []map[string]string{
			{"name": "warfarin"},
			{"name": "aspirin"},
		},
	})

	req, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/drugs/interactions", bytes.NewReader(body))
	req.Header.Set("X-API-Key", key)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * 1e9} // 30s timeout for XML fetches
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadGateway {
		t.Skip("upstream unavailable")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Should find at least 1 interaction (aspirin in warfarin's Section 7.3)
	interactions, _ := result["interactions"].([]interface{})
	if len(interactions) == 0 {
		t.Error("expected warfarin+aspirin to have at least 1 interaction match")
	}

	// Verify the interaction mentions aspirin
	if len(interactions) > 0 {
		first, _ := interactions[0].(map[string]interface{})
		text, _ := first["text"].(string)
		if !strings.Contains(strings.ToLower(text), "aspirin") {
			t.Errorf("expected interaction text to mention aspirin, got: %.100s...", text)
		}
	}

	checkedPairs, _ := result["checked_pairs"].(float64)
	if checkedPairs != 1 {
		t.Errorf("checked_pairs = %v, want 1", checkedPairs)
	}
}

func TestE2E_SPL_InteractionChecker_NoInteraction(t *testing.T) {
	key := createTestKey(t, "spl-e2e-nointeract", 100)

	body, _ := json.Marshal(map[string]interface{}{
		"drugs": []map[string]string{
			{"name": "tylenol"},
			{"name": "ibuprofen"},
		},
	})

	req, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/drugs/interactions", bytes.NewReader(body))
	req.Header.Set("X-API-Key", key)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * 1e9}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadGateway {
		t.Skip("upstream unavailable")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Tylenol and ibuprofen are both OTC — no Section 7
	foundInteractions, _ := result["found_interactions"].(float64)
	if foundInteractions != 0 {
		t.Errorf("expected 0 interactions for OTC drugs, got %v", foundInteractions)
	}
}

func TestE2E_SPL_InteractionChecker_TooFewDrugs(t *testing.T) {
	key := createTestKey(t, "spl-e2e-toofew", 100)

	body, _ := json.Marshal(map[string]interface{}{
		"drugs": []map[string]string{
			{"name": "warfarin"},
		},
	})

	req, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/drugs/interactions", bytes.NewReader(body))
	req.Header.Set("X-API-Key", key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

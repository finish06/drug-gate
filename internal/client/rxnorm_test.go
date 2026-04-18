package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- JSON deserialization tests (verify struct tags match upstream) ---

func TestRxNormCandidateRaw_MatchesUpstreamJSON(t *testing.T) {
	// Real cash-drugs format: flat array entries
	raw := `{"rxcui":"153165","name":"atorvastatin calcium","score":"100","source":"RXNORM","rank":"1"}`
	var c RxNormCandidateRaw
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.RxCUI != "153165" {
		t.Errorf("RxCUI = %q, want %q", c.RxCUI, "153165")
	}
	if c.Name != "atorvastatin calcium" {
		t.Errorf("Name = %q, want %q", c.Name, "atorvastatin calcium")
	}
	if c.Score != "100" {
		t.Errorf("Score = %q, want %q", c.Score, "100")
	}
}

func TestRxNormConceptRaw_MatchesUpstreamJSON(t *testing.T) {
	// Real cash-drugs format: flat entries with tty field
	raw := `{"rxcui":"83367","name":"atorvastatin","tty":"IN","language":"ENG"}`
	var c RxNormConceptRaw
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.RxCUI != "83367" {
		t.Errorf("RxCUI = %q, want %q", c.RxCUI, "83367")
	}
	if c.Name != "atorvastatin" {
		t.Errorf("Name = %q, want %q", c.Name, "atorvastatin")
	}
	if c.TTY != "IN" {
		t.Errorf("TTY = %q, want %q", c.TTY, "IN")
	}
}

// --- HTTP client tests (mock cash-drugs response shapes) ---

func rxnormServer(t *testing.T, expectedPath string, response any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != expectedPath {
			t.Errorf("path = %q, want %q", r.URL.Path, expectedPath)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
}

func TestHTTPRxNormClient_SearchApproximate_HappyPath(t *testing.T) {
	// cash-drugs returns flat array in data[]
	resp := map[string]any{
		"data": []map[string]any{
			{"rxcui": "153165", "name": "atorvastatin calcium", "score": "100"},
			{"rxcui": "83367", "name": "atorvastatin", "score": "75"},
		},
	}
	srv := rxnormServer(t, "/api/cache/rxnorm-approximate-match", resp)
	defer srv.Close()

	c := NewHTTPRxNormClient(srv.URL)
	candidates, err := c.SearchApproximate(context.Background(), "lipitor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("got %d candidates, want 2", len(candidates))
	}
	if candidates[0].RxCUI != "153165" {
		t.Errorf("candidates[0].RxCUI = %q, want %q", candidates[0].RxCUI, "153165")
	}
	if candidates[0].Name != "atorvastatin calcium" {
		t.Errorf("candidates[0].Name = %q, want %q", candidates[0].Name, "atorvastatin calcium")
	}
}

func TestHTTPRxNormClient_SearchApproximate_EmptyResults(t *testing.T) {
	resp := map[string]any{
		"data": []map[string]any{},
	}
	srv := rxnormServer(t, "/api/cache/rxnorm-approximate-match", resp)
	defer srv.Close()

	c := NewHTTPRxNormClient(srv.URL)
	candidates, err := c.SearchApproximate(context.Background(), "notadrug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 0 {
		t.Errorf("got %d candidates, want 0", len(candidates))
	}
}

func TestHTTPRxNormClient_SearchApproximate_Unreachable(t *testing.T) {
	c := NewHTTPRxNormClient("http://localhost:1")
	_, err := c.SearchApproximate(context.Background(), "test")
	if err == nil {
		t.Error("expected error for unreachable, got nil")
	}
}

func TestHTTPRxNormClient_FetchSpellingSuggestions_HappyPath(t *testing.T) {
	// cash-drugs wraps in data[] array with nested suggestionGroup
	resp := map[string]any{
		"data": []map[string]any{
			{
				"suggestionGroup": map[string]any{
					"suggestionList": map[string]any{
						"suggestion": []string{"lipitor", "lisinopril"},
					},
				},
			},
		},
	}
	srv := rxnormServer(t, "/api/cache/rxnorm-spelling-suggestions", resp)
	defer srv.Close()

	c := NewHTTPRxNormClient(srv.URL)
	suggestions, err := c.FetchSpellingSuggestions(context.Background(), "liiptor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(suggestions) != 2 {
		t.Fatalf("got %d suggestions, want 2", len(suggestions))
	}
	if suggestions[0] != "lipitor" {
		t.Errorf("suggestions[0] = %q, want %q", suggestions[0], "lipitor")
	}
}

func TestHTTPRxNormClient_FetchNDCs_HappyPath(t *testing.T) {
	// cash-drugs wraps in data[] array with nested ndcGroup
	resp := map[string]any{
		"data": []map[string]any{
			{
				"ndcGroup": map[string]any{
					"ndcList": map[string]any{
						"ndc": []string{"0071-0155-23", "0071-0156-23"},
					},
				},
			},
		},
	}
	srv := rxnormServer(t, "/api/cache/rxnorm-ndcs", resp)
	defer srv.Close()

	c := NewHTTPRxNormClient(srv.URL)
	ndcs, err := c.FetchNDCs(context.Background(), "153165")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ndcs) != 2 {
		t.Fatalf("got %d ndcs, want 2", len(ndcs))
	}
	if ndcs[0] != "0071-0155-23" {
		t.Errorf("ndcs[0] = %q, want %q", ndcs[0], "0071-0155-23")
	}
}

func TestHTTPRxNormClient_FetchNDCs_EmptyResults(t *testing.T) {
	// cash-drugs returns nested ndcGroup with empty ndcList
	resp := map[string]any{
		"data": []map[string]any{
			{
				"ndcGroup": map[string]any{
					"ndcList": map[string]any{},
				},
			},
		},
	}
	srv := rxnormServer(t, "/api/cache/rxnorm-ndcs", resp)
	defer srv.Close()

	c := NewHTTPRxNormClient(srv.URL)
	ndcs, err := c.FetchNDCs(context.Background(), "999999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ndcs) != 0 {
		t.Errorf("got %d ndcs, want 0", len(ndcs))
	}
}

func TestHTTPRxNormClient_FetchGenericProduct_HappyPath(t *testing.T) {
	// cash-drugs returns flat array of concepts in data[]
	resp := map[string]any{
		"data": []map[string]any{
			{"rxcui": "83367", "name": "atorvastatin", "tty": "IN"},
		},
	}
	srv := rxnormServer(t, "/api/cache/rxnorm-generic-product", resp)
	defer srv.Close()

	c := NewHTTPRxNormClient(srv.URL)
	generics, err := c.FetchGenericProduct(context.Background(), "153165")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(generics) != 1 {
		t.Fatalf("got %d generics, want 1", len(generics))
	}
	if generics[0].RxCUI != "83367" {
		t.Errorf("generics[0].RxCUI = %q, want %q", generics[0].RxCUI, "83367")
	}
}

func TestHTTPRxNormClient_FetchAllRelated_HappyPath(t *testing.T) {
	// cash-drugs returns flat array with tty on each entry — client re-groups
	resp := map[string]any{
		"data": []map[string]any{
			{"rxcui": "83367", "name": "atorvastatin", "tty": "IN"},
			{"rxcui": "153165", "name": "Lipitor", "tty": "BN"},
			{"rxcui": "317541", "name": "Oral Tablet", "tty": "DF"},
		},
	}
	srv := rxnormServer(t, "/api/cache/rxnorm-all-related", resp)
	defer srv.Close()

	c := NewHTTPRxNormClient(srv.URL)
	groups, err := c.FetchAllRelated(context.Background(), "153165")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 3 {
		t.Fatalf("got %d groups, want 3", len(groups))
	}
	// Find the IN group
	found := false
	for _, g := range groups {
		if g.TTY == "IN" {
			found = true
			if len(g.ConceptProperties) != 1 || g.ConceptProperties[0].Name != "atorvastatin" {
				t.Errorf("IN group ConceptProperties = %v, want [atorvastatin]", g.ConceptProperties)
			}
		}
	}
	if !found {
		t.Error("expected IN group in results")
	}
}

func TestHTTPRxNormClient_FetchSpellingSuggestions_Upstream500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewHTTPRxNormClient(srv.URL)
	_, err := c.FetchSpellingSuggestions(context.Background(), "lipator")
	if err == nil {
		t.Error("expected error for 500 response, got nil")
	}
}

func TestHTTPRxNormClient_FetchNDCs_Upstream500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewHTTPRxNormClient(srv.URL)
	_, err := c.FetchNDCs(context.Background(), "153165")
	if err == nil {
		t.Error("expected error for 500 response, got nil")
	}
}

func TestHTTPRxNormClient_FetchGenericProduct_Upstream500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewHTTPRxNormClient(srv.URL)
	_, err := c.FetchGenericProduct(context.Background(), "153165")
	if err == nil {
		t.Error("expected error for 500 response, got nil")
	}
}

func TestHTTPRxNormClient_FetchAllRelated_Upstream500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewHTTPRxNormClient(srv.URL)
	_, err := c.FetchAllRelated(context.Background(), "153165")
	if err == nil {
		t.Error("expected error for 500 response, got nil")
	}
}

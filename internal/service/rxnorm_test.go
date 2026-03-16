package service

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/finish06/drug-gate/internal/client"
	"github.com/redis/go-redis/v9"
)

type mockRxNormClient struct {
	candidates  []client.RxNormCandidateRaw
	searchErr   error
	suggestions []string
	suggestErr  error
	ndcs        []string
	ndcErr      error
	generics    []client.RxNormConceptRaw
	genericErr  error
	groups      []client.RxNormConceptGroupRaw
	relatedErr  error

	searchCount  int
	suggestCount int
	ndcCount     int
	genericCount int
	relatedCount int
}

func (m *mockRxNormClient) SearchApproximate(_ context.Context, _ string) ([]client.RxNormCandidateRaw, error) {
	m.searchCount++
	return m.candidates, m.searchErr
}
func (m *mockRxNormClient) FetchSpellingSuggestions(_ context.Context, _ string) ([]string, error) {
	m.suggestCount++
	return m.suggestions, m.suggestErr
}
func (m *mockRxNormClient) FetchNDCs(_ context.Context, _ string) ([]string, error) {
	m.ndcCount++
	return m.ndcs, m.ndcErr
}
func (m *mockRxNormClient) FetchGenericProduct(_ context.Context, _ string) ([]client.RxNormConceptRaw, error) {
	m.genericCount++
	return m.generics, m.genericErr
}
func (m *mockRxNormClient) FetchAllRelated(_ context.Context, _ string) ([]client.RxNormConceptGroupRaw, error) {
	m.relatedCount++
	return m.groups, m.relatedErr
}

func setupRxNormTest(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, rdb
}

func TestRxNormService_Search_HappyPath(t *testing.T) {
	_, rdb := setupRxNormTest(t)
	mc := &mockRxNormClient{
		candidates: []client.RxNormCandidateRaw{
			{RxCUI: "153165", Name: "atorvastatin calcium", Score: "100"},
			{RxCUI: "83367", Name: "atorvastatin", Score: "75"},
		},
	}
	svc := NewRxNormService(mc, rdb)

	result, err := svc.Search(context.Background(), "lipitor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Query != "lipitor" {
		t.Errorf("Query = %q, want %q", result.Query, "lipitor")
	}
	if len(result.Candidates) != 2 {
		t.Fatalf("got %d candidates, want 2", len(result.Candidates))
	}
	if result.Candidates[0].Score != 100 {
		t.Errorf("Candidates[0].Score = %d, want 100", result.Candidates[0].Score)
	}
	if len(result.Suggestions) != 0 {
		t.Errorf("Suggestions should be empty, got %d", len(result.Suggestions))
	}
}

func TestRxNormService_Search_CapsAt5(t *testing.T) {
	_, rdb := setupRxNormTest(t)
	candidates := make([]client.RxNormCandidateRaw, 8)
	for i := range candidates {
		candidates[i] = client.RxNormCandidateRaw{RxCUI: "1000" + string(rune('0'+i)), Name: "drug", Score: "50"}
	}
	mc := &mockRxNormClient{candidates: candidates}
	svc := NewRxNormService(mc, rdb)

	result, err := svc.Search(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Candidates) != 5 {
		t.Errorf("got %d candidates, want 5 (capped)", len(result.Candidates))
	}
}

func TestRxNormService_Search_NoResults_FetchesSuggestions(t *testing.T) {
	_, rdb := setupRxNormTest(t)
	mc := &mockRxNormClient{
		candidates:  nil,
		suggestions: []string{"lipitor", "lisinopril"},
	}
	svc := NewRxNormService(mc, rdb)

	result, err := svc.Search(context.Background(), "liiptor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Candidates) != 0 {
		t.Errorf("got %d candidates, want 0", len(result.Candidates))
	}
	if len(result.Suggestions) != 2 {
		t.Errorf("got %d suggestions, want 2", len(result.Suggestions))
	}
	if mc.suggestCount != 1 {
		t.Errorf("suggestCount = %d, want 1", mc.suggestCount)
	}
}

func TestRxNormService_Search_CachesResult(t *testing.T) {
	_, rdb := setupRxNormTest(t)
	mc := &mockRxNormClient{
		candidates: []client.RxNormCandidateRaw{
			{RxCUI: "153165", Name: "atorvastatin calcium", Score: "100"},
		},
	}
	svc := NewRxNormService(mc, rdb)

	_, _ = svc.Search(context.Background(), "lipitor")
	result, _ := svc.Search(context.Background(), "lipitor")

	if mc.searchCount != 1 {
		t.Errorf("searchCount = %d, want 1 (second call should hit cache)", mc.searchCount)
	}
	if len(result.Candidates) != 1 {
		t.Errorf("cached result should have 1 candidate, got %d", len(result.Candidates))
	}
}

func TestRxNormService_GetNDCs_HappyPath(t *testing.T) {
	_, rdb := setupRxNormTest(t)
	mc := &mockRxNormClient{ndcs: []string{"0071-0155-23", "0071-0156-23"}}
	svc := NewRxNormService(mc, rdb)

	result, err := svc.GetNDCs(context.Background(), "153165")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RxCUI != "153165" {
		t.Errorf("RxCUI = %q, want %q", result.RxCUI, "153165")
	}
	if len(result.NDCs) != 2 {
		t.Errorf("got %d NDCs, want 2", len(result.NDCs))
	}
}

func TestRxNormService_GetNDCs_CachesResult(t *testing.T) {
	_, rdb := setupRxNormTest(t)
	mc := &mockRxNormClient{ndcs: []string{"0071-0155-23"}}
	svc := NewRxNormService(mc, rdb)

	_, _ = svc.GetNDCs(context.Background(), "153165")
	_, _ = svc.GetNDCs(context.Background(), "153165")

	if mc.ndcCount != 1 {
		t.Errorf("ndcCount = %d, want 1 (second call should hit cache)", mc.ndcCount)
	}
}

func TestRxNormService_GetGenerics_HappyPath(t *testing.T) {
	_, rdb := setupRxNormTest(t)
	mc := &mockRxNormClient{
		generics: []client.RxNormConceptRaw{{RxCUI: "83367", Name: "atorvastatin"}},
	}
	svc := NewRxNormService(mc, rdb)

	result, err := svc.GetGenerics(context.Background(), "153165")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Generics) != 1 {
		t.Fatalf("got %d generics, want 1", len(result.Generics))
	}
	if result.Generics[0].Name != "atorvastatin" {
		t.Errorf("Generics[0].Name = %q, want %q", result.Generics[0].Name, "atorvastatin")
	}
}

func TestRxNormService_GetRelated_GroupsByTTY(t *testing.T) {
	_, rdb := setupRxNormTest(t)
	mc := &mockRxNormClient{
		groups: []client.RxNormConceptGroupRaw{
			{TTY: "IN", ConceptProperties: []client.RxNormConceptRaw{{RxCUI: "83367", Name: "atorvastatin"}}},
			{TTY: "BN", ConceptProperties: []client.RxNormConceptRaw{{RxCUI: "153165", Name: "Lipitor"}}},
			{TTY: "DF", ConceptProperties: []client.RxNormConceptRaw{{RxCUI: "317541", Name: "Oral Tablet"}}},
			{TTY: "SCD", ConceptProperties: []client.RxNormConceptRaw{{RxCUI: "259255", Name: "atorvastatin 10 MG Oral Tablet"}}},
			{TTY: "SBD", ConceptProperties: []client.RxNormConceptRaw{{RxCUI: "617310", Name: "Lipitor 10 MG Oral Tablet"}}},
			{TTY: "GPCK", ConceptProperties: []client.RxNormConceptRaw{{RxCUI: "999", Name: "should be excluded"}}},
		},
	}
	svc := NewRxNormService(mc, rdb)

	result, err := svc.GetRelated(context.Background(), "153165")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Ingredients) != 1 || result.Ingredients[0].Name != "atorvastatin" {
		t.Errorf("Ingredients = %v, want [atorvastatin]", result.Ingredients)
	}
	if len(result.BrandNames) != 1 || result.BrandNames[0].Name != "Lipitor" {
		t.Errorf("BrandNames = %v, want [Lipitor]", result.BrandNames)
	}
	if len(result.DoseForms) != 1 {
		t.Errorf("DoseForms len = %d, want 1", len(result.DoseForms))
	}
	if len(result.ClinicalDrugs) != 1 {
		t.Errorf("ClinicalDrugs len = %d, want 1", len(result.ClinicalDrugs))
	}
	if len(result.BrandedDrugs) != 1 {
		t.Errorf("BrandedDrugs len = %d, want 1", len(result.BrandedDrugs))
	}
}

func TestRxNormService_GetProfile_HappyPath(t *testing.T) {
	_, rdb := setupRxNormTest(t)
	mc := &mockRxNormClient{
		candidates: []client.RxNormCandidateRaw{
			{RxCUI: "153165", Name: "atorvastatin calcium", Score: "100"},
		},
		ndcs:     []string{"0071-0155-23"},
		generics: []client.RxNormConceptRaw{{RxCUI: "83367", Name: "atorvastatin"}},
		groups: []client.RxNormConceptGroupRaw{
			{TTY: "IN", ConceptProperties: []client.RxNormConceptRaw{{RxCUI: "83367", Name: "atorvastatin"}}},
			{TTY: "BN", ConceptProperties: []client.RxNormConceptRaw{{RxCUI: "153165", Name: "Lipitor"}}},
		},
	}
	svc := NewRxNormService(mc, rdb)

	profile, err := svc.GetProfile(context.Background(), "lipitor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile == nil {
		t.Fatal("expected non-nil profile")
	}
	if profile.Query != "lipitor" {
		t.Errorf("Query = %q, want %q", profile.Query, "lipitor")
	}
	if profile.RxCUI != "153165" {
		t.Errorf("RxCUI = %q, want %q", profile.RxCUI, "153165")
	}
	if profile.Name != "atorvastatin calcium" {
		t.Errorf("Name = %q, want %q", profile.Name, "atorvastatin calcium")
	}
	if len(profile.BrandNames) != 1 || profile.BrandNames[0] != "Lipitor" {
		t.Errorf("BrandNames = %v, want [Lipitor]", profile.BrandNames)
	}
	if profile.Generic == nil || profile.Generic.Name != "atorvastatin" {
		t.Errorf("Generic = %v, want atorvastatin", profile.Generic)
	}
	if len(profile.NDCs) != 1 {
		t.Errorf("NDCs len = %d, want 1", len(profile.NDCs))
	}
	if profile.Related == nil {
		t.Error("Related should not be nil")
	}
}

func TestRxNormService_GetProfile_NotFound(t *testing.T) {
	_, rdb := setupRxNormTest(t)
	mc := &mockRxNormClient{
		candidates:  nil,
		suggestions: []string{},
	}
	svc := NewRxNormService(mc, rdb)

	profile, err := svc.GetProfile(context.Background(), "notadrug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile != nil {
		t.Errorf("expected nil profile for not found, got %+v", profile)
	}
}

func TestRxNormService_GetProfile_CachesAssembledResult(t *testing.T) {
	_, rdb := setupRxNormTest(t)
	mc := &mockRxNormClient{
		candidates: []client.RxNormCandidateRaw{
			{RxCUI: "153165", Name: "atorvastatin calcium", Score: "100"},
		},
		ndcs:     []string{"0071-0155-23"},
		generics: []client.RxNormConceptRaw{{RxCUI: "83367", Name: "atorvastatin"}},
		groups: []client.RxNormConceptGroupRaw{
			{TTY: "IN", ConceptProperties: []client.RxNormConceptRaw{{RxCUI: "83367", Name: "atorvastatin"}}},
		},
	}
	svc := NewRxNormService(mc, rdb)

	_, _ = svc.GetProfile(context.Background(), "lipitor")
	profile, _ := svc.GetProfile(context.Background(), "lipitor")

	// Search is called once for the first GetProfile, but the assembled profile is cached
	// so the second call should not trigger any additional upstream calls
	if profile == nil {
		t.Fatal("expected cached profile")
	}
	if mc.searchCount != 1 {
		t.Errorf("searchCount = %d, want 1", mc.searchCount)
	}
}

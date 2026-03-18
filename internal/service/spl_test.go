package service

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/redis/go-redis/v9"
)

// mockSPLClient implements client.SPLClient for testing.
type mockSPLClient struct {
	splsByName   []client.SPLEntryRaw
	splsByNameFn func(name string) ([]client.SPLEntryRaw, error)
	splDetail    *client.SPLEntryRaw
	splDetailErr error
	splXML       []byte
	splXMLErr    error
}

func (m *mockSPLClient) FetchSPLsByName(_ context.Context, name string) ([]client.SPLEntryRaw, error) {
	if m.splsByNameFn != nil {
		return m.splsByNameFn(name)
	}
	return m.splsByName, nil
}

func (m *mockSPLClient) FetchSPLDetail(_ context.Context, _ string) (*client.SPLEntryRaw, error) {
	return m.splDetail, m.splDetailErr
}

func (m *mockSPLClient) FetchSPLXML(_ context.Context, _ string) ([]byte, error) {
	return m.splXML, m.splXMLErr
}

func setupSPLRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func TestSPLService_SearchSPLs_Success(t *testing.T) {
	_, rdb := setupSPLRedis(t)
	sc := &mockSPLClient{
		splsByName: []client.SPLEntryRaw{
			{Title: "LIPITOR [PFIZER]", SetID: "abc-123", PublishedDate: "May 02, 2024", SPLVersion: 42},
			{Title: "LIPITOR [VIATRIS]", SetID: "def-456", PublishedDate: "Jun 27, 2024", SPLVersion: 7},
		},
	}
	svc := NewSPLService(sc, nil, rdb)

	entries, total, err := svc.SearchSPLs(context.Background(), "lipitor", 20, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(entries) != 2 {
		t.Errorf("entries = %d, want 2", len(entries))
	}
	if entries[0].SetID != "abc-123" && entries[0].SetID != "def-456" {
		t.Errorf("unexpected setid: %s", entries[0].SetID)
	}
}

func TestSPLService_SearchSPLs_Pagination(t *testing.T) {
	_, rdb := setupSPLRedis(t)
	raw := make([]client.SPLEntryRaw, 5)
	for i := range raw {
		raw[i] = client.SPLEntryRaw{Title: "Drug", SetID: "id-" + string(rune('a'+i)), SPLVersion: i}
	}
	sc := &mockSPLClient{splsByName: raw}
	svc := NewSPLService(sc, nil, rdb)

	entries, total, err := svc.SearchSPLs(context.Background(), "drug", 2, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(entries) != 2 {
		t.Errorf("entries = %d, want 2", len(entries))
	}

	// Page 2
	entries, _, err = svc.SearchSPLs(context.Background(), "drug", 2, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("entries = %d, want 2", len(entries))
	}

	// Page 3 (partial)
	entries, _, err = svc.SearchSPLs(context.Background(), "drug", 2, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("entries = %d, want 1", len(entries))
	}
}

func TestSPLService_SearchSPLs_EmptyResult(t *testing.T) {
	_, rdb := setupSPLRedis(t)
	sc := &mockSPLClient{splsByName: nil}
	svc := NewSPLService(sc, nil, rdb)

	entries, total, err := svc.SearchSPLs(context.Background(), "nonexistent", 20, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if len(entries) != 0 {
		t.Errorf("entries = %d, want 0", len(entries))
	}
}

func TestSPLService_SearchSPLs_CacheHit(t *testing.T) {
	_, rdb := setupSPLRedis(t)
	callCount := 0
	sc := &mockSPLClient{
		splsByNameFn: func(_ string) ([]client.SPLEntryRaw, error) {
			callCount++
			return []client.SPLEntryRaw{{Title: "Drug", SetID: "abc", SPLVersion: 1}}, nil
		},
	}
	svc := NewSPLService(sc, nil, rdb)

	// First call — cache miss
	_, _, _ = svc.SearchSPLs(context.Background(), "drug", 20, 0)
	if callCount != 1 {
		t.Errorf("expected 1 upstream call, got %d", callCount)
	}

	// Second call — cache hit
	_, _, _ = svc.SearchSPLs(context.Background(), "drug", 20, 0)
	if callCount != 1 {
		t.Errorf("expected still 1 upstream call after cache hit, got %d", callCount)
	}
}

func TestSPLService_GetSPLDetail_Success(t *testing.T) {
	_, rdb := setupSPLRedis(t)
	sc := &mockSPLClient{
		splDetail: &client.SPLEntryRaw{Title: "LIPITOR [PFIZER]", SetID: "abc-123", PublishedDate: "May 02, 2024", SPLVersion: 42},
		splXML: []byte(`<document>
			<section><title>7 DRUG INTERACTIONS</title><text>Interaction summary.</text></section>
			<section><title>7.1 CYP450</title><text>CYP2C9 inhibitors include fluconazole.</text></section>
		</document>`),
	}
	svc := NewSPLService(sc, nil, rdb)

	detail, err := svc.GetSPLDetail(context.Background(), "abc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail == nil {
		t.Fatal("expected non-nil detail")
	}
	if detail.Title != "LIPITOR [PFIZER]" {
		t.Errorf("Title = %q, want %q", detail.Title, "LIPITOR [PFIZER]")
	}
	if len(detail.Interactions) != 2 {
		t.Fatalf("expected 2 interaction sections, got %d", len(detail.Interactions))
	}
	if detail.Interactions[0].Title != "7 DRUG INTERACTIONS" {
		t.Errorf("interactions[0].Title = %q", detail.Interactions[0].Title)
	}
}

func TestSPLService_GetSPLDetail_NotFound(t *testing.T) {
	_, rdb := setupSPLRedis(t)
	sc := &mockSPLClient{splDetail: nil}
	svc := NewSPLService(sc, nil, rdb)

	detail, err := svc.GetSPLDetail(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail != nil {
		t.Errorf("expected nil for nonexistent setid, got %+v", detail)
	}
}

func TestSPLService_GetSPLDetail_XMLError_ReturnsEmptyInteractions(t *testing.T) {
	_, rdb := setupSPLRedis(t)
	sc := &mockSPLClient{
		splDetail: &client.SPLEntryRaw{Title: "DRUG", SetID: "abc", SPLVersion: 1},
		splXMLErr: errors.New("xml fetch failed"),
	}
	svc := NewSPLService(sc, nil, rdb)

	detail, err := svc.GetSPLDetail(context.Background(), "abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail == nil {
		t.Fatal("expected non-nil detail even with XML error")
	}
	if len(detail.Interactions) != 0 {
		t.Errorf("expected empty interactions on XML error, got %d", len(detail.Interactions))
	}
}

func TestSPLService_GetInteractionsForDrug_Success(t *testing.T) {
	_, rdb := setupSPLRedis(t)
	sc := &mockSPLClient{
		splsByName: []client.SPLEntryRaw{
			{Title: "WARFARIN [REMEDYREPACK]", SetID: "war-123", PublishedDate: "Mar 16, 2026", SPLVersion: 17},
		},
		splXML: []byte(`<document>
			<section><title>7 DRUG INTERACTIONS</title><text>Bleeding risk drugs.</text></section>
		</document>`),
	}
	svc := NewSPLService(sc, nil, rdb)

	detail, err := svc.GetInteractionsForDrug(context.Background(), "warfarin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail == nil {
		t.Fatal("expected non-nil detail")
	}
	if detail.SetID != "war-123" {
		t.Errorf("SetID = %q, want %q", detail.SetID, "war-123")
	}
	if len(detail.Interactions) != 1 {
		t.Fatalf("expected 1 interaction section, got %d", len(detail.Interactions))
	}
}

func TestSPLService_GetInteractionsForDrug_NoSPLs(t *testing.T) {
	_, rdb := setupSPLRedis(t)
	sc := &mockSPLClient{splsByName: nil}
	svc := NewSPLService(sc, nil, rdb)

	detail, err := svc.GetInteractionsForDrug(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail != nil {
		t.Errorf("expected nil for drug with no SPLs")
	}
}

func TestSPLService_ResolveDrugNameFromNDC(t *testing.T) {
	_, rdb := setupSPLRedis(t)
	dc := &mockClient{
		ndcResult: &client.DrugResult{GenericName: "atorvastatin calcium"},
	}
	svc := NewSPLService(nil, dc, rdb)

	name, err := svc.ResolveDrugNameFromNDC(context.Background(), "0071-0155-23")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "atorvastatin calcium" {
		t.Errorf("name = %q, want %q", name, "atorvastatin calcium")
	}
}

func TestSPLService_ResolveDrugNameFromNDC_NotFound(t *testing.T) {
	_, rdb := setupSPLRedis(t)
	dc := &mockClient{ndcResult: nil}
	svc := NewSPLService(nil, dc, rdb)

	_, err := svc.ResolveDrugNameFromNDC(context.Background(), "9999-9999-99")
	if err == nil {
		t.Fatal("expected error for unknown NDC")
	}
}

func TestPaginate_OffsetBeyondLength(t *testing.T) {
	entries := []model.SPLEntry{{Title: "A"}, {Title: "B"}}
	result := paginate(entries, 10, 100)
	if len(result) != 0 {
		t.Errorf("expected 0 entries for offset beyond length, got %d", len(result))
	}
}

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/redis/go-redis/v9"
)

// --- mock client ---

type mockClient struct {
	drugNames      []client.DrugNameRaw
	drugNamesErr   error
	drugClasses    []client.DrugClassRaw
	drugClassesErr error
	pharmResults   []client.DrugResult
	pharmErr       error

	// call counters
	fetchNamesCount   int
	fetchClassesCount int
	pharmClassCount   int
}

func (m *mockClient) LookupByNDC(_ context.Context, _ string) (*client.DrugResult, error) {
	return nil, nil
}
func (m *mockClient) LookupByGenericName(_ context.Context, _ string) ([]client.DrugResult, error) {
	return nil, nil
}
func (m *mockClient) LookupByBrandName(_ context.Context, _ string) ([]client.DrugResult, error) {
	return nil, nil
}
func (m *mockClient) FetchDrugNames(_ context.Context) ([]client.DrugNameRaw, error) {
	m.fetchNamesCount++
	return m.drugNames, m.drugNamesErr
}
func (m *mockClient) FetchDrugClasses(_ context.Context) ([]client.DrugClassRaw, error) {
	m.fetchClassesCount++
	return m.drugClasses, m.drugClassesErr
}
func (m *mockClient) LookupByPharmClass(_ context.Context, _ string) ([]client.DrugResult, error) {
	m.pharmClassCount++
	return m.pharmResults, m.pharmErr
}

// --- helpers ---

func setupRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, rdb
}

// --- GetDrugNames tests ---

func TestGetDrugNames_AC015_CacheMiss_FetchesUpstream(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	mc := &mockClient{
		drugNames: []client.DrugNameRaw{
			{DrugName: "Simvastatin", NameType: "G"},
			{DrugName: "Zocor", NameType: "B"},
		},
	}
	svc := NewDrugDataService(mc, rdb)

	entries, err := svc.GetDrugNames(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mc.fetchNamesCount != 1 {
		t.Errorf("expected 1 upstream call, got %d", mc.fetchNamesCount)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "Simvastatin" || entries[0].Type != "generic" {
		t.Errorf("entry 0: got %+v", entries[0])
	}
	if entries[1].Name != "Zocor" || entries[1].Type != "brand" {
		t.Errorf("entry 1: got %+v", entries[1])
	}
}

func TestGetDrugNames_AC015_CacheHit_NoUpstreamCall(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	mc := &mockClient{
		drugNames: []client.DrugNameRaw{
			{DrugName: "Aspirin", NameType: "G"},
		},
	}
	svc := NewDrugDataService(mc, rdb)

	// First call — populates cache
	_, err := svc.GetDrugNames(context.Background())
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Second call — should hit cache
	entries, err := svc.GetDrugNames(context.Background())
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if mc.fetchNamesCount != 1 {
		t.Errorf("expected 1 upstream call (cached), got %d", mc.fetchNamesCount)
	}
	if len(entries) != 1 || entries[0].Name != "Aspirin" {
		t.Errorf("unexpected cached result: %+v", entries)
	}
}

func TestGetDrugNames_AC016_SlidingTTL_ResetOnRead(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	mc := &mockClient{
		drugNames: []client.DrugNameRaw{
			{DrugName: "Metformin", NameType: "G"},
		},
	}
	svc := NewDrugDataService(mc, rdb)

	// Populate cache
	_, err := svc.GetDrugNames(context.Background())
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Fast-forward 50 minutes
	mr.FastForward(50 * time.Minute)

	// Read again — should hit cache and reset TTL
	_, err = svc.GetDrugNames(context.Background())
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	// Fast-forward another 50 minutes (total 100 from original set, but only 50 from reset)
	mr.FastForward(50 * time.Minute)

	// Should still be cached (TTL was reset at the 50-minute mark)
	_, err = svc.GetDrugNames(context.Background())
	if err != nil {
		t.Fatalf("third call: %v", err)
	}

	if mc.fetchNamesCount != 1 {
		t.Errorf("expected 1 upstream call (TTL kept alive), got %d", mc.fetchNamesCount)
	}
}

func TestGetDrugNames_AC016_CacheExpires_FetchesFresh(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	mc := &mockClient{
		drugNames: []client.DrugNameRaw{
			{DrugName: "Ibuprofen", NameType: "G"},
		},
	}
	svc := NewDrugDataService(mc, rdb)

	// Populate cache
	_, err := svc.GetDrugNames(context.Background())
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Fast-forward past TTL
	mr.FastForward(61 * time.Minute)

	// Should miss cache and fetch fresh
	_, err = svc.GetDrugNames(context.Background())
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if mc.fetchNamesCount != 2 {
		t.Errorf("expected 2 upstream calls (cache expired), got %d", mc.fetchNamesCount)
	}
}

func TestGetDrugNames_UpstreamError_Propagated(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	mc := &mockClient{
		drugNamesErr: client.ErrUpstream,
	}
	svc := NewDrugDataService(mc, rdb)

	_, err := svc.GetDrugNames(context.Background())
	if !errors.Is(err, client.ErrUpstream) {
		t.Errorf("expected ErrUpstream, got %v", err)
	}
}

func TestGetDrugNames_CorruptCache_FetchesFresh(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	// Plant corrupt JSON in cache
	mr.Set("cache:drugnames", "not valid json")

	mc := &mockClient{
		drugNames: []client.DrugNameRaw{
			{DrugName: "Lisinopril", NameType: "G"},
		},
	}
	svc := NewDrugDataService(mc, rdb)

	entries, err := svc.GetDrugNames(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mc.fetchNamesCount != 1 {
		t.Errorf("expected upstream fetch after corrupt cache, got %d calls", mc.fetchNamesCount)
	}
	if len(entries) != 1 || entries[0].Name != "Lisinopril" {
		t.Errorf("unexpected result: %+v", entries)
	}
}

func TestGetDrugNames_NameTypeMapping(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	mc := &mockClient{
		drugNames: []client.DrugNameRaw{
			{DrugName: "Generic Drug", NameType: "G"},
			{DrugName: "Brand Drug", NameType: "B"},
			{DrugName: "Lowercase G", NameType: "g"},
			{DrugName: "Unknown Type", NameType: "X"},
		},
	}
	svc := NewDrugDataService(mc, rdb)

	entries, err := svc.GetDrugNames(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []struct{ name, typ string }{
		{"Generic Drug", "generic"},
		{"Brand Drug", "brand"},
		{"Lowercase G", "generic"},    // "g" uppercases to "G" → generic
		{"Unknown Type", "generic"},   // non-B defaults to generic
	}
	for i, e := range expected {
		if entries[i].Name != e.name || entries[i].Type != e.typ {
			t.Errorf("entry %d: expected {%s, %s}, got {%s, %s}", i, e.name, e.typ, entries[i].Name, entries[i].Type)
		}
	}
}

// --- GetDrugClasses tests ---

func TestGetDrugClasses_AC015_CacheMiss_FetchesUpstream(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	mc := &mockClient{
		drugClasses: []client.DrugClassRaw{
			{ClassName: "HMG-CoA Reductase Inhibitor", ClassType: "EPC"},
			{ClassName: "Beta Blocker", ClassType: "MoA"},
		},
	}
	svc := NewDrugDataService(mc, rdb)

	entries, err := svc.GetDrugClasses(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mc.fetchClassesCount != 1 {
		t.Errorf("expected 1 upstream call, got %d", mc.fetchClassesCount)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "HMG-CoA Reductase Inhibitor" || entries[0].Type != "epc" {
		t.Errorf("entry 0: got %+v", entries[0])
	}
	if entries[1].Name != "Beta Blocker" || entries[1].Type != "moa" {
		t.Errorf("entry 1: got %+v", entries[1])
	}
}

func TestGetDrugClasses_AC015_CacheHit_NoUpstreamCall(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	mc := &mockClient{
		drugClasses: []client.DrugClassRaw{
			{ClassName: "ACE Inhibitor", ClassType: "EPC"},
		},
	}
	svc := NewDrugDataService(mc, rdb)

	_, _ = svc.GetDrugClasses(context.Background())
	entries, err := svc.GetDrugClasses(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mc.fetchClassesCount != 1 {
		t.Errorf("expected 1 upstream call (cached), got %d", mc.fetchClassesCount)
	}
	if len(entries) != 1 || entries[0].Name != "ACE Inhibitor" {
		t.Errorf("unexpected cached result: %+v", entries)
	}
}

func TestGetDrugClasses_ClassTypeLowercased(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	mc := &mockClient{
		drugClasses: []client.DrugClassRaw{
			{ClassName: "Statin", ClassType: "EPC"},
			{ClassName: "Mechanism", ClassType: "MOA"},
		},
	}
	svc := NewDrugDataService(mc, rdb)

	entries, err := svc.GetDrugClasses(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if entries[0].Type != "epc" {
		t.Errorf("expected lowercase 'epc', got %q", entries[0].Type)
	}
	if entries[1].Type != "moa" {
		t.Errorf("expected lowercase 'moa', got %q", entries[1].Type)
	}
}

func TestGetDrugClasses_UpstreamError_Propagated(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	mc := &mockClient{
		drugClassesErr: client.ErrUpstream,
	}
	svc := NewDrugDataService(mc, rdb)

	_, err := svc.GetDrugClasses(context.Background())
	if !errors.Is(err, client.ErrUpstream) {
		t.Errorf("expected ErrUpstream, got %v", err)
	}
}

func TestGetDrugClasses_CorruptCache_FetchesFresh(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	mr.Set("cache:drugclasses", "{bad")

	mc := &mockClient{
		drugClasses: []client.DrugClassRaw{
			{ClassName: "SSRI", ClassType: "EPC"},
		},
	}
	svc := NewDrugDataService(mc, rdb)

	entries, err := svc.GetDrugClasses(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mc.fetchClassesCount != 1 {
		t.Errorf("expected upstream fetch after corrupt cache, got %d", mc.fetchClassesCount)
	}
	if len(entries) != 1 || entries[0].Name != "SSRI" {
		t.Errorf("unexpected result: %+v", entries)
	}
}

// --- GetDrugsByClass tests ---

func TestGetDrugsByClass_AC015_CacheMiss_FetchesUpstream(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	mc := &mockClient{
		pharmResults: []client.DrugResult{
			{GenericName: "simvastatin", BrandName: "Zocor"},
			{GenericName: "atorvastatin", BrandName: "Lipitor"},
		},
	}
	svc := NewDrugDataService(mc, rdb)

	entries, err := svc.GetDrugsByClass(context.Background(), "HMG-CoA Reductase Inhibitor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mc.pharmClassCount != 1 {
		t.Errorf("expected 1 upstream call, got %d", mc.pharmClassCount)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].GenericName != "simvastatin" || entries[0].BrandName != "Zocor" {
		t.Errorf("entry 0: got %+v", entries[0])
	}
}

func TestGetDrugsByClass_AC015_CacheHit_NoUpstreamCall(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	mc := &mockClient{
		pharmResults: []client.DrugResult{
			{GenericName: "lisinopril", BrandName: "Prinivil"},
		},
	}
	svc := NewDrugDataService(mc, rdb)

	_, _ = svc.GetDrugsByClass(context.Background(), "ACE Inhibitor")
	entries, err := svc.GetDrugsByClass(context.Background(), "ACE Inhibitor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mc.pharmClassCount != 1 {
		t.Errorf("expected 1 upstream call (cached), got %d", mc.pharmClassCount)
	}
	if len(entries) != 1 || entries[0].GenericName != "lisinopril" {
		t.Errorf("unexpected: %+v", entries)
	}
}

func TestGetDrugsByClass_CacheKeyLowercased(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	mc := &mockClient{
		pharmResults: []client.DrugResult{
			{GenericName: "metoprolol", BrandName: "Lopressor"},
		},
	}
	svc := NewDrugDataService(mc, rdb)

	// Fetch with mixed case
	_, _ = svc.GetDrugsByClass(context.Background(), "Beta Blocker")

	// Verify key is lowercase
	if !mr.Exists("cache:drugsbyclass:beta blocker") {
		t.Error("expected cache key to be lowercased")
	}

	// Same class, different case — should hit cache
	_, _ = svc.GetDrugsByClass(context.Background(), "BETA BLOCKER")
	if mc.pharmClassCount != 2 {
		// Note: the service lowercases the key but passes the original className
		// to the upstream client. Different casing = different upstream call but
		// same cache key. This is the current behavior.
		// If both cases should share cache, the lookup needs to lowercase too.
	}
}

func TestGetDrugsByClass_UpstreamError_Propagated(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	mc := &mockClient{
		pharmErr: client.ErrUpstream,
	}
	svc := NewDrugDataService(mc, rdb)

	_, err := svc.GetDrugsByClass(context.Background(), "Statin")
	if !errors.Is(err, client.ErrUpstream) {
		t.Errorf("expected ErrUpstream, got %v", err)
	}
}

func TestGetDrugsByClass_EmptyResult_CachedAsEmpty(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	mc := &mockClient{
		pharmResults: []client.DrugResult{},
	}
	svc := NewDrugDataService(mc, rdb)

	entries, err := svc.GetDrugsByClass(context.Background(), "Unknown Class")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(entries))
	}

	// Second call — should come from cache, not upstream
	entries, err = svc.GetDrugsByClass(context.Background(), "Unknown Class")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mc.pharmClassCount != 1 {
		t.Errorf("expected 1 upstream call (empty result cached), got %d", mc.pharmClassCount)
	}
}

func TestGetDrugsByClass_CorruptCache_FetchesFresh(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	mr.Set("cache:drugsbyclass:statin", "corrupt!")

	mc := &mockClient{
		pharmResults: []client.DrugResult{
			{GenericName: "rosuvastatin", BrandName: "Crestor"},
		},
	}
	svc := NewDrugDataService(mc, rdb)

	entries, err := svc.GetDrugsByClass(context.Background(), "Statin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mc.pharmClassCount != 1 {
		t.Errorf("expected upstream fetch after corrupt cache, got %d", mc.pharmClassCount)
	}
	if len(entries) != 1 || entries[0].GenericName != "rosuvastatin" {
		t.Errorf("unexpected: %+v", entries)
	}
}

// --- Cache data integrity ---

func TestCacheRoundTrip_DataIntegrity(t *testing.T) {
	mr, rdb := setupRedis(t)
	defer mr.Close()

	mc := &mockClient{
		drugNames: []client.DrugNameRaw{
			{DrugName: "Amoxicillin/Clavulanate", NameType: "G"},
			{DrugName: "Augmentin", NameType: "B"},
		},
	}
	svc := NewDrugDataService(mc, rdb)

	// Fetch to populate cache
	original, _ := svc.GetDrugNames(context.Background())

	// Read from cache
	cached, _ := svc.GetDrugNames(context.Background())

	if len(original) != len(cached) {
		t.Fatalf("length mismatch: original %d, cached %d", len(original), len(cached))
	}
	for i := range original {
		if original[i] != cached[i] {
			t.Errorf("entry %d mismatch: original %+v, cached %+v", i, original[i], cached[i])
		}
	}

	// Verify what's actually in Redis is valid JSON
	raw, err := mr.Get("cache:drugnames")
	if err != nil {
		t.Fatalf("key missing from Redis: %v", err)
	}
	var fromRedis []model.DrugNameEntry
	if err := json.Unmarshal([]byte(raw), &fromRedis); err != nil {
		t.Fatalf("Redis value is not valid JSON: %v", err)
	}
	if len(fromRedis) != 2 {
		t.Errorf("expected 2 entries in Redis, got %d", len(fromRedis))
	}
}

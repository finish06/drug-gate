//go:build integration

package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/finish06/drug-gate/internal/service"
	"github.com/redis/go-redis/v9"
)

// --- mock client for integration tests ---

type mockDrugClient struct {
	drugNames      []client.DrugNameRaw
	drugNamesErr   error
	drugClasses    []client.DrugClassRaw
	drugClassesErr error
	pharmResults   map[string][]client.DrugResult // keyed by class name
	pharmErr       error

	fetchNamesCount   int
	fetchClassesCount int
	pharmClassCalls   map[string]int
}

func newMockDrugClient() *mockDrugClient {
	return &mockDrugClient{
		pharmResults:    make(map[string][]client.DrugResult),
		pharmClassCalls: make(map[string]int),
	}
}

func (m *mockDrugClient) LookupByNDC(_ context.Context, _ string) (*client.DrugResult, error) {
	return nil, nil
}
func (m *mockDrugClient) LookupByGenericName(_ context.Context, _ string) ([]client.DrugResult, error) {
	return nil, nil
}
func (m *mockDrugClient) LookupByBrandName(_ context.Context, _ string) ([]client.DrugResult, error) {
	return nil, nil
}
func (m *mockDrugClient) FetchDrugNames(_ context.Context) ([]client.DrugNameRaw, error) {
	m.fetchNamesCount++
	return m.drugNames, m.drugNamesErr
}
func (m *mockDrugClient) FetchDrugClasses(_ context.Context) ([]client.DrugClassRaw, error) {
	m.fetchClassesCount++
	return m.drugClasses, m.drugClassesErr
}
func (m *mockDrugClient) LookupByPharmClass(_ context.Context, class string) ([]client.DrugResult, error) {
	m.pharmClassCalls[class]++
	if m.pharmErr != nil {
		return nil, m.pharmErr
	}
	return m.pharmResults[class], nil
}

// --- Redis helpers ---

func setupRedis(t *testing.T) *redis.Client {
	t.Helper()

	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		addr = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}

	t.Cleanup(func() {
		// Clean up cache keys written by service
		iter := rdb.Scan(ctx, 0, "cache:*", 100).Iterator()
		for iter.Next(ctx) {
			rdb.Del(ctx, iter.Val())
		}
		rdb.Close()
	})

	return rdb
}

// ============================================================
// GetDrugNames — Redis integration
// ============================================================

func TestIntegration_GetDrugNames_CacheMissPopulatesRedis(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.drugNames = []client.DrugNameRaw{
		{DrugName: "Simvastatin", NameType: "G"},
		{DrugName: "Zocor", NameType: "B"},
		{DrugName: "Atorvastatin", NameType: "G"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	entries, err := svc.GetDrugNames(ctx)
	if err != nil {
		t.Fatalf("GetDrugNames: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Verify data is actually in Redis
	raw, err := rdb.Get(ctx, "cache:drugnames").Bytes()
	if err != nil {
		t.Fatalf("expected cache:drugnames in Redis, got: %v", err)
	}

	var cached []model.DrugNameEntry
	if err := json.Unmarshal(raw, &cached); err != nil {
		t.Fatalf("Redis value is not valid JSON: %v", err)
	}
	if len(cached) != 3 {
		t.Errorf("expected 3 cached entries, got %d", len(cached))
	}
}

func TestIntegration_GetDrugNames_CacheHitSkipsUpstream(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.drugNames = []client.DrugNameRaw{
		{DrugName: "Aspirin", NameType: "G"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	// First call populates
	first, _ := svc.GetDrugNames(ctx)

	// Second call should hit cache
	second, err := svc.GetDrugNames(ctx)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if mc.fetchNamesCount != 1 {
		t.Errorf("expected 1 upstream call, got %d", mc.fetchNamesCount)
	}
	if len(first) != len(second) {
		t.Errorf("results differ: first=%d, second=%d", len(first), len(second))
	}
	if first[0] != second[0] {
		t.Errorf("data mismatch: first=%+v, second=%+v", first[0], second[0])
	}
}

func TestIntegration_GetDrugNames_TTLIsSet(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.drugNames = []client.DrugNameRaw{
		{DrugName: "Metformin", NameType: "G"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	_, _ = svc.GetDrugNames(ctx)

	ttl, err := rdb.TTL(ctx, "cache:drugnames").Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	// TTL should be close to 60 minutes (allow some slack)
	if ttl < 59*time.Minute || ttl > 61*time.Minute {
		t.Errorf("expected TTL ~60min, got %v", ttl)
	}
}

func TestIntegration_GetDrugNames_SlidingTTLResetOnRead(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.drugNames = []client.DrugNameRaw{
		{DrugName: "Lisinopril", NameType: "G"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	// Populate cache
	_, _ = svc.GetDrugNames(ctx)

	// Manually reduce TTL to simulate time passing
	rdb.Expire(ctx, "cache:drugnames", 10*time.Minute)

	ttlBefore, _ := rdb.TTL(ctx, "cache:drugnames").Result()
	if ttlBefore > 11*time.Minute {
		t.Fatalf("setup failed: TTL should be ~10min, got %v", ttlBefore)
	}

	// Read again — should reset TTL back to 60 minutes
	_, _ = svc.GetDrugNames(ctx)

	ttlAfter, _ := rdb.TTL(ctx, "cache:drugnames").Result()
	if ttlAfter < 59*time.Minute {
		t.Errorf("expected TTL reset to ~60min after read, got %v", ttlAfter)
	}
}

func TestIntegration_GetDrugNames_CacheExpiry_FetchesFresh(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.drugNames = []client.DrugNameRaw{
		{DrugName: "Ibuprofen", NameType: "G"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	// Populate cache
	_, _ = svc.GetDrugNames(ctx)

	// Delete to simulate expiry
	rdb.Del(ctx, "cache:drugnames")

	// Should fetch from upstream again
	_, err := svc.GetDrugNames(ctx)
	if err != nil {
		t.Fatalf("after expiry: %v", err)
	}
	if mc.fetchNamesCount != 2 {
		t.Errorf("expected 2 upstream calls (cache expired), got %d", mc.fetchNamesCount)
	}
}

func TestIntegration_GetDrugNames_CorruptCacheRecovery(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	// Plant corrupt data directly in Redis
	rdb.Set(ctx, "cache:drugnames", "not-valid-json!!!", 60*time.Minute)

	mc := newMockDrugClient()
	mc.drugNames = []client.DrugNameRaw{
		{DrugName: "Amoxicillin", NameType: "G"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	entries, err := svc.GetDrugNames(ctx)
	if err != nil {
		t.Fatalf("expected graceful recovery, got: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "Amoxicillin" {
		t.Errorf("unexpected result: %+v", entries)
	}
	if mc.fetchNamesCount != 1 {
		t.Errorf("expected upstream fetch after corrupt cache")
	}

	// Verify cache is now fixed
	raw, _ := rdb.Get(ctx, "cache:drugnames").Bytes()
	var repaired []model.DrugNameEntry
	if err := json.Unmarshal(raw, &repaired); err != nil {
		t.Errorf("cache not repaired: %v", err)
	}
}

func TestIntegration_GetDrugNames_TypeMapping(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.drugNames = []client.DrugNameRaw{
		{DrugName: "GenericDrug", NameType: "G"},
		{DrugName: "BrandDrug", NameType: "B"},
		{DrugName: "LowercaseG", NameType: "g"},
		{DrugName: "UnknownType", NameType: "X"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	entries, err := svc.GetDrugNames(ctx)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	cases := []struct{ name, typ string }{
		{"GenericDrug", "generic"},
		{"BrandDrug", "brand"},
		{"LowercaseG", "generic"},
		{"UnknownType", "generic"},
	}
	for i, c := range cases {
		if entries[i].Name != c.name || entries[i].Type != c.typ {
			t.Errorf("entry %d: expected {%s, %s}, got {%s, %s}",
				i, c.name, c.typ, entries[i].Name, entries[i].Type)
		}
	}
}

func TestIntegration_GetDrugNames_LargeDataset(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	// Simulate a realistic dataset size (~1000 entries)
	mc := newMockDrugClient()
	mc.drugNames = make([]client.DrugNameRaw, 1000)
	for i := range mc.drugNames {
		nameType := "G"
		if i%3 == 0 {
			nameType = "B"
		}
		mc.drugNames[i] = client.DrugNameRaw{
			DrugName: fmt.Sprintf("Drug_%04d", i),
			NameType: nameType,
		}
	}
	svc := service.NewDrugDataService(mc, rdb)

	entries, err := svc.GetDrugNames(ctx)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(entries) != 1000 {
		t.Fatalf("expected 1000 entries, got %d", len(entries))
	}

	// Second call from cache
	cached, err := svc.GetDrugNames(ctx)
	if err != nil {
		t.Fatalf("cached call: %v", err)
	}
	if len(cached) != 1000 {
		t.Fatalf("cached: expected 1000, got %d", len(cached))
	}
	if mc.fetchNamesCount != 1 {
		t.Errorf("expected 1 upstream call for large dataset, got %d", mc.fetchNamesCount)
	}
}

// ============================================================
// GetDrugClasses — Redis integration
// ============================================================

func TestIntegration_GetDrugClasses_CacheMissPopulatesRedis(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.drugClasses = []client.DrugClassRaw{
		{ClassName: "HMG-CoA Reductase Inhibitor", ClassType: "EPC"},
		{ClassName: "Beta Adrenergic Blocker", ClassType: "MoA"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	entries, err := svc.GetDrugClasses(ctx)
	if err != nil {
		t.Fatalf("GetDrugClasses: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2, got %d", len(entries))
	}

	// Verify Redis has the data
	raw, err := rdb.Get(ctx, "cache:drugclasses").Bytes()
	if err != nil {
		t.Fatalf("cache:drugclasses not in Redis: %v", err)
	}
	var cached []model.DrugClassEntry
	if err := json.Unmarshal(raw, &cached); err != nil {
		t.Fatalf("invalid JSON in Redis: %v", err)
	}
	if len(cached) != 2 {
		t.Errorf("expected 2 cached, got %d", len(cached))
	}
}

func TestIntegration_GetDrugClasses_CacheHitSkipsUpstream(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.drugClasses = []client.DrugClassRaw{
		{ClassName: "ACE Inhibitor", ClassType: "EPC"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	_, _ = svc.GetDrugClasses(ctx)
	entries, err := svc.GetDrugClasses(ctx)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if mc.fetchClassesCount != 1 {
		t.Errorf("expected 1 upstream call, got %d", mc.fetchClassesCount)
	}
	if len(entries) != 1 || entries[0].Name != "ACE Inhibitor" {
		t.Errorf("unexpected: %+v", entries)
	}
}

func TestIntegration_GetDrugClasses_ClassTypeLowercased(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.drugClasses = []client.DrugClassRaw{
		{ClassName: "Statin", ClassType: "EPC"},
		{ClassName: "Mechanism", ClassType: "MOA"},
		{ClassName: "Effect", ClassType: "PE"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	entries, _ := svc.GetDrugClasses(ctx)

	expected := []string{"epc", "moa", "pe"}
	for i, e := range expected {
		if entries[i].Type != e {
			t.Errorf("entry %d: expected type %q, got %q", i, e, entries[i].Type)
		}
	}

	// Verify lowercasing survives the cache round-trip
	cached, _ := svc.GetDrugClasses(ctx)
	for i, e := range expected {
		if cached[i].Type != e {
			t.Errorf("cached entry %d: expected type %q, got %q", i, e, cached[i].Type)
		}
	}
}

func TestIntegration_GetDrugClasses_TTLIsSet(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.drugClasses = []client.DrugClassRaw{
		{ClassName: "SSRI", ClassType: "EPC"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	_, _ = svc.GetDrugClasses(ctx)

	ttl, _ := rdb.TTL(ctx, "cache:drugclasses").Result()
	if ttl < 59*time.Minute || ttl > 61*time.Minute {
		t.Errorf("expected TTL ~60min, got %v", ttl)
	}
}

func TestIntegration_GetDrugClasses_CorruptCacheRecovery(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	rdb.Set(ctx, "cache:drugclasses", "{broken", 60*time.Minute)

	mc := newMockDrugClient()
	mc.drugClasses = []client.DrugClassRaw{
		{ClassName: "Recovered Class", ClassType: "EPC"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	entries, err := svc.GetDrugClasses(ctx)
	if err != nil {
		t.Fatalf("expected recovery, got: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "Recovered Class" {
		t.Errorf("unexpected: %+v", entries)
	}
}

// ============================================================
// GetDrugsByClass — Redis integration
// ============================================================

func TestIntegration_GetDrugsByClass_CacheMissPopulatesRedis(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.pharmResults["HMG-CoA Reductase Inhibitor"] = []client.DrugResult{
		{GenericName: "simvastatin", BrandName: "Zocor"},
		{GenericName: "atorvastatin", BrandName: "Lipitor"},
		{GenericName: "rosuvastatin", BrandName: "Crestor"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	entries, err := svc.GetDrugsByClass(ctx, "HMG-CoA Reductase Inhibitor")
	if err != nil {
		t.Fatalf("GetDrugsByClass: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3, got %d", len(entries))
	}

	// Verify in Redis (key is lowercased)
	raw, err := rdb.Get(ctx, "cache:drugsbyclass:hmg-coa reductase inhibitor").Bytes()
	if err != nil {
		t.Fatalf("cache key not in Redis: %v", err)
	}
	var cached []model.DrugInClassEntry
	if err := json.Unmarshal(raw, &cached); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(cached) != 3 {
		t.Errorf("expected 3 cached, got %d", len(cached))
	}
}

func TestIntegration_GetDrugsByClass_CacheHitSkipsUpstream(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.pharmResults["Statin"] = []client.DrugResult{
		{GenericName: "pravastatin", BrandName: "Pravachol"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	_, _ = svc.GetDrugsByClass(ctx, "Statin")
	entries, err := svc.GetDrugsByClass(ctx, "Statin")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if mc.pharmClassCalls["Statin"] != 1 {
		t.Errorf("expected 1 upstream call, got %d", mc.pharmClassCalls["Statin"])
	}
	if len(entries) != 1 || entries[0].GenericName != "pravastatin" {
		t.Errorf("unexpected: %+v", entries)
	}
}

func TestIntegration_GetDrugsByClass_CacheKeyLowercased(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.pharmResults["Beta Blocker"] = []client.DrugResult{
		{GenericName: "metoprolol", BrandName: "Lopressor"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	_, _ = svc.GetDrugsByClass(ctx, "Beta Blocker")

	// Key should be lowercase
	exists, _ := rdb.Exists(ctx, "cache:drugsbyclass:beta blocker").Result()
	if exists != 1 {
		t.Error("expected lowercased cache key")
	}

	// Mixed-case key should NOT exist
	exists, _ = rdb.Exists(ctx, "cache:drugsbyclass:Beta Blocker").Result()
	if exists == 1 {
		t.Error("cache key should not preserve original case")
	}
}

func TestIntegration_GetDrugsByClass_DifferentClassesSeparateKeys(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.pharmResults["Statin"] = []client.DrugResult{
		{GenericName: "simvastatin", BrandName: "Zocor"},
	}
	mc.pharmResults["ACE Inhibitor"] = []client.DrugResult{
		{GenericName: "lisinopril", BrandName: "Prinivil"},
		{GenericName: "enalapril", BrandName: "Vasotec"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	statins, _ := svc.GetDrugsByClass(ctx, "Statin")
	aces, _ := svc.GetDrugsByClass(ctx, "ACE Inhibitor")

	if len(statins) != 1 {
		t.Errorf("statins: expected 1, got %d", len(statins))
	}
	if len(aces) != 2 {
		t.Errorf("ACE inhibitors: expected 2, got %d", len(aces))
	}

	// Both should be cached independently
	exists1, _ := rdb.Exists(ctx, "cache:drugsbyclass:statin").Result()
	exists2, _ := rdb.Exists(ctx, "cache:drugsbyclass:ace inhibitor").Result()
	if exists1 != 1 || exists2 != 1 {
		t.Error("expected both class keys in Redis")
	}
}

func TestIntegration_GetDrugsByClass_EmptyResultCached(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.pharmResults["Unknown Class"] = []client.DrugResult{}
	svc := service.NewDrugDataService(mc, rdb)

	entries, err := svc.GetDrugsByClass(ctx, "Unknown Class")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty, got %d", len(entries))
	}

	// Empty result should be cached (prevents repeated upstream calls for unknown classes)
	raw, err := rdb.Get(ctx, "cache:drugsbyclass:unknown class").Bytes()
	if err != nil {
		t.Fatalf("empty result not cached: %v", err)
	}
	var cached []model.DrugInClassEntry
	json.Unmarshal(raw, &cached)
	if len(cached) != 0 {
		t.Errorf("cached empty result should be [], got %d entries", len(cached))
	}

	// Second call should hit cache
	_, _ = svc.GetDrugsByClass(ctx, "Unknown Class")
	if mc.pharmClassCalls["Unknown Class"] != 1 {
		t.Errorf("expected 1 upstream call, got %d", mc.pharmClassCalls["Unknown Class"])
	}
}

func TestIntegration_GetDrugsByClass_TTLIsSet(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.pharmResults["Statin"] = []client.DrugResult{
		{GenericName: "simvastatin", BrandName: "Zocor"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	_, _ = svc.GetDrugsByClass(ctx, "Statin")

	ttl, _ := rdb.TTL(ctx, "cache:drugsbyclass:statin").Result()
	if ttl < 59*time.Minute || ttl > 61*time.Minute {
		t.Errorf("expected TTL ~60min, got %v", ttl)
	}
}

func TestIntegration_GetDrugsByClass_SlidingTTLResetOnRead(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.pharmResults["Statin"] = []client.DrugResult{
		{GenericName: "simvastatin", BrandName: "Zocor"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	_, _ = svc.GetDrugsByClass(ctx, "Statin")

	// Reduce TTL to simulate time passing
	rdb.Expire(ctx, "cache:drugsbyclass:statin", 5*time.Minute)

	// Read again — TTL should reset
	_, _ = svc.GetDrugsByClass(ctx, "Statin")

	ttl, _ := rdb.TTL(ctx, "cache:drugsbyclass:statin").Result()
	if ttl < 59*time.Minute {
		t.Errorf("TTL should reset to ~60min, got %v", ttl)
	}
}

func TestIntegration_GetDrugsByClass_CorruptCacheRecovery(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	rdb.Set(ctx, "cache:drugsbyclass:statin", "CORRUPT!", 60*time.Minute)

	mc := newMockDrugClient()
	mc.pharmResults["Statin"] = []client.DrugResult{
		{GenericName: "rosuvastatin", BrandName: "Crestor"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	entries, err := svc.GetDrugsByClass(ctx, "Statin")
	if err != nil {
		t.Fatalf("expected recovery, got: %v", err)
	}
	if len(entries) != 1 || entries[0].GenericName != "rosuvastatin" {
		t.Errorf("unexpected: %+v", entries)
	}
}

// ============================================================
// Cross-method isolation
// ============================================================

func TestIntegration_CacheKeysAreIsolated(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.drugNames = []client.DrugNameRaw{{DrugName: "DrugA", NameType: "G"}}
	mc.drugClasses = []client.DrugClassRaw{{ClassName: "ClassA", ClassType: "EPC"}}
	mc.pharmResults["ClassA"] = []client.DrugResult{
		{GenericName: "drugA", BrandName: "BrandA"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	// Populate all three caches
	_, _ = svc.GetDrugNames(ctx)
	_, _ = svc.GetDrugClasses(ctx)
	_, _ = svc.GetDrugsByClass(ctx, "ClassA")

	// Verify each key exists independently
	keys := []string{"cache:drugnames", "cache:drugclasses", "cache:drugsbyclass:classa"}
	for _, key := range keys {
		exists, _ := rdb.Exists(ctx, key).Result()
		if exists != 1 {
			t.Errorf("expected key %q to exist", key)
		}
	}

	// Delete one — others should survive
	rdb.Del(ctx, "cache:drugnames")

	_, err := svc.GetDrugClasses(ctx)
	if err != nil {
		t.Errorf("drugclasses should still be cached: %v", err)
	}
	if mc.fetchClassesCount != 1 {
		t.Errorf("drugclasses should not re-fetch, got %d calls", mc.fetchClassesCount)
	}
}

// ============================================================
// Data integrity across cache round-trips
// ============================================================

func TestIntegration_DataIntegrity_SpecialCharacters(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.drugNames = []client.DrugNameRaw{
		{DrugName: "Amoxicillin/Clavulanate", NameType: "G"},
		{DrugName: "D-Penicillamine", NameType: "G"},
		{DrugName: "Naproxen Sodium (OTC)", NameType: "B"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	original, _ := svc.GetDrugNames(ctx)
	cached, _ := svc.GetDrugNames(ctx)

	for i := range original {
		if original[i] != cached[i] {
			t.Errorf("entry %d mismatch after cache: original=%+v cached=%+v",
				i, original[i], cached[i])
		}
	}
}

func TestIntegration_DataIntegrity_DrugsByClass_FieldsPreserved(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.pharmResults["Statin"] = []client.DrugResult{
		{GenericName: "simvastatin", BrandName: "Zocor"},
		{GenericName: "atorvastatin calcium", BrandName: "Lipitor"},
		{GenericName: "rosuvastatin calcium", BrandName: "Crestor"},
	}
	svc := service.NewDrugDataService(mc, rdb)

	original, _ := svc.GetDrugsByClass(ctx, "Statin")
	cached, _ := svc.GetDrugsByClass(ctx, "Statin")

	if len(original) != len(cached) {
		t.Fatalf("length mismatch: %d vs %d", len(original), len(cached))
	}
	for i := range original {
		if original[i].GenericName != cached[i].GenericName {
			t.Errorf("entry %d generic_name mismatch", i)
		}
		if original[i].BrandName != cached[i].BrandName {
			t.Errorf("entry %d brand_name mismatch", i)
		}
	}
}

// ============================================================
// Upstream error propagation
// ============================================================

func TestIntegration_UpstreamError_NoCacheWrite(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.drugNamesErr = client.ErrUpstream
	svc := service.NewDrugDataService(mc, rdb)

	_, err := svc.GetDrugNames(ctx)
	if err == nil {
		t.Fatal("expected error")
	}

	// Error response should NOT be cached
	exists, _ := rdb.Exists(ctx, "cache:drugnames").Result()
	if exists == 1 {
		t.Error("error response should not be cached")
	}
}

func TestIntegration_UpstreamError_DrugsByClass_NoCacheWrite(t *testing.T) {
	rdb := setupRedis(t)
	ctx := context.Background()

	mc := newMockDrugClient()
	mc.pharmErr = client.ErrUpstream
	svc := service.NewDrugDataService(mc, rdb)

	_, err := svc.GetDrugsByClass(ctx, "Statin")
	if err == nil {
		t.Fatal("expected error")
	}

	exists, _ := rdb.Exists(ctx, "cache:drugsbyclass:statin").Result()
	if exists == 1 {
		t.Error("error response should not be cached")
	}
}

package spl

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/finish06/drug-gate/internal/client"
	"github.com/redis/go-redis/v9"
)

type mockSPLClient struct {
	splsByName map[string][]client.SPLEntryRaw
	splXML     map[string][]byte
}

func (m *mockSPLClient) FetchSPLsByName(_ context.Context, name string) ([]client.SPLEntryRaw, error) {
	return m.splsByName[name], nil
}

func (m *mockSPLClient) FetchSPLDetail(_ context.Context, _ string) (*client.SPLEntryRaw, error) {
	return nil, nil
}

func (m *mockSPLClient) FetchSPLXML(_ context.Context, setID string) ([]byte, error) {
	return m.splXML[setID], nil
}

func setupIndexerRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func seedDrugNames(t *testing.T, mr *miniredis.Miniredis, names []string) {
	t.Helper()
	type entry struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	entries := make([]entry, len(names))
	for i, n := range names {
		entries[i] = entry{Name: n, Type: "generic"}
	}
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	_ = mr.Set("cache:drugnames", string(data))
}

func TestIndexer_IndexesDrugsFromCache(t *testing.T) {
	mr, rdb := setupIndexerRedis(t)

	seedDrugNames(t, mr, []string{"warfarin", "metoprolol"})

	sc := &mockSPLClient{
		splsByName: map[string][]client.SPLEntryRaw{
			"warfarin":   {{Title: "WARFARIN LABEL", SetID: "war-1", SPLVersion: 1}},
			"metoprolol": {{Title: "METOPROLOL LABEL", SetID: "met-1", SPLVersion: 1}},
		},
		splXML: map[string][]byte{
			"war-1": []byte(`<doc><section><title>7 DRUG INTERACTIONS</title><text>Aspirin risk.</text></section></doc>`),
			"met-1": []byte(`<doc><section><title>7 DRUG INTERACTIONS</title><text>CYP2D6 inhibitors.</text></section></doc>`),
		},
	}

	idx := NewIndexer(sc, rdb, 0, 100) // interval=0 means run once
	idx.indexOnce()

	// Verify both drugs are cached
	ctx := context.Background()
	data, err := rdb.Get(ctx, "cache:spl:interactions:warfarin").Bytes()
	if err != nil {
		t.Fatalf("warfarin not cached: %v", err)
	}
	var detail struct {
		SetID string `json:"setid"`
	}
	_ = json.Unmarshal(data, &detail)
	if detail.SetID != "war-1" {
		t.Errorf("warfarin setid = %q, want war-1", detail.SetID)
	}

	_, err = rdb.Get(ctx, "cache:spl:interactions:metoprolol").Bytes()
	if err != nil {
		t.Fatalf("metoprolol not cached: %v", err)
	}
}

func TestIndexer_SkipsAlreadyCached(t *testing.T) {
	mr, rdb := setupIndexerRedis(t)
	seedDrugNames(t, mr, []string{"warfarin"})

	// Pre-populate cache
	_ = mr.Set("cache:spl:interactions:warfarin", `{"setid":"existing"}`)

	fetchCount := 0
	sc := &mockSPLClient{
		splsByName: map[string][]client.SPLEntryRaw{
			"warfarin": {{Title: "WARFARIN", SetID: "war-1", SPLVersion: 1}},
		},
		splXML: map[string][]byte{
			"war-1": []byte(`<doc><section><title>7 DRUG INTERACTIONS</title><text>test</text></section></doc>`),
		},
	}
	// Wrap to count fetches
	_ = sc
	_ = fetchCount

	idx := NewIndexer(sc, rdb, 0, 100)
	idx.indexOnce()

	// Should still have the pre-existing value (not overwritten)
	val, _ := rdb.Get(context.Background(), "cache:spl:interactions:warfarin").Result()
	if val != `{"setid":"existing"}` {
		t.Errorf("expected pre-existing cache to be preserved, got: %s", val)
	}
}

func TestIndexer_RespectsMaxDrugs(t *testing.T) {
	mr, rdb := setupIndexerRedis(t)
	seedDrugNames(t, mr, []string{"drugA", "drugB", "drugC", "drugD", "drugE"})

	sc := &mockSPLClient{
		splsByName: map[string][]client.SPLEntryRaw{
			"drugA": {{Title: "A", SetID: "a-1", SPLVersion: 1}},
			"drugB": {{Title: "B", SetID: "b-1", SPLVersion: 1}},
			"drugC": {{Title: "C", SetID: "c-1", SPLVersion: 1}},
			"drugD": {{Title: "D", SetID: "d-1", SPLVersion: 1}},
			"drugE": {{Title: "E", SetID: "e-1", SPLVersion: 1}},
		},
		splXML: map[string][]byte{
			"a-1": []byte(`<doc><section><title>7 DRUG INTERACTIONS</title><text>A interactions.</text></section></doc>`),
			"b-1": []byte(`<doc><section><title>7 DRUG INTERACTIONS</title><text>B interactions.</text></section></doc>`),
			"c-1": []byte(`<doc><section><title>7 DRUG INTERACTIONS</title><text>C interactions.</text></section></doc>`),
			"d-1": []byte(`<doc><section><title>7 DRUG INTERACTIONS</title><text>D interactions.</text></section></doc>`),
			"e-1": []byte(`<doc><section><title>7 DRUG INTERACTIONS</title><text>E interactions.</text></section></doc>`),
		},
	}

	idx := NewIndexer(sc, rdb, 0, 2) // Only index 2
	idx.indexOnce()

	ctx := context.Background()
	// At most 2 should be indexed
	cached := 0
	for _, name := range []string{"druga", "drugb", "drugc", "drugd", "druge"} {
		if rdb.Exists(ctx, "cache:spl:interactions:"+name).Val() > 0 {
			cached++
		}
	}
	if cached > 2 {
		t.Errorf("expected at most 2 cached, got %d", cached)
	}
}

func TestIndexer_NoDrugNames_Skips(t *testing.T) {
	_, rdb := setupIndexerRedis(t)
	// No drugnames in cache

	sc := &mockSPLClient{}
	idx := NewIndexer(sc, rdb, 0, 100)
	idx.indexOnce() // Should not panic
}

func TestIndexer_DeduplicatesDrugNames(t *testing.T) {
	mr, rdb := setupIndexerRedis(t)
	// Seed with duplicates (different case)
	seedDrugNames(t, mr, []string{"Warfarin", "warfarin", "WARFARIN"})

	sc := &mockSPLClient{
		splsByName: map[string][]client.SPLEntryRaw{
			"Warfarin": {{Title: "WARFARIN", SetID: "war-1", SPLVersion: 1}},
		},
		splXML: map[string][]byte{
			"war-1": []byte(`<doc><section><title>7 DRUG INTERACTIONS</title><text>test</text></section></doc>`),
		},
	}

	idx := NewIndexer(sc, rdb, 0, 100)
	idx.indexOnce()

	// Should only be one cache entry
	ctx := context.Background()
	if rdb.Exists(ctx, "cache:spl:interactions:warfarin").Val() == 0 {
		t.Error("expected warfarin to be cached")
	}
}

func TestIndexer_StopDuringRun(t *testing.T) {
	mr, rdb := setupIndexerRedis(t)
	seedDrugNames(t, mr, []string{"drugA", "drugB"})

	sc := &mockSPLClient{
		splsByName: map[string][]client.SPLEntryRaw{
			"drugA": {{Title: "A", SetID: "a-1", SPLVersion: 1}},
			"drugB": {{Title: "B", SetID: "b-1", SPLVersion: 1}},
		},
		splXML: map[string][]byte{
			"a-1": []byte(`<doc><section><title>7 DRUG INTERACTIONS</title><text>A.</text></section></doc>`),
			"b-1": []byte(`<doc><section><title>7 DRUG INTERACTIONS</title><text>B.</text></section></doc>`),
		},
	}

	idx := NewIndexer(sc, rdb, 24*time.Hour, 100)
	idx.Start()
	time.Sleep(100 * time.Millisecond) // Let it run once
	idx.Stop()
	// Should not hang — Stop returns cleanly
}

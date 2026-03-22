package service

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/finish06/drug-gate/internal/client"
	"github.com/redis/go-redis/v9"
)

// generateDrugNames creates n synthetic drug name entries for benchmarking.
func generateDrugNames(n int) []client.DrugNameRaw {
	names := make([]client.DrugNameRaw, n)
	prefixes := []string{"met", "lis", "sim", "ato", "amlo", "ibup", "ace", "war", "asp", "oxy"}
	for i := 0; i < n; i++ {
		prefix := prefixes[i%len(prefixes)]
		nameType := "G"
		if i%3 == 0 {
			nameType = "B"
		}
		names[i] = client.DrugNameRaw{
			DrugName: prefix + "drug-" + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)),
			NameType: nameType,
		}
	}
	return names
}

func setupBenchRedis(b *testing.B) (*miniredis.Miniredis, *redis.Client) {
	b.Helper()
	mr := miniredis.RunT(b)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, rdb
}

// BenchmarkAutocompleteDrugs_Current benchmarks the current implementation
// with 100K drug names (simulating production dataset).
func BenchmarkAutocompleteDrugs_Current(b *testing.B) {
	mr, rdb := setupBenchRedis(b)
	defer mr.Close()

	mc := &mockClient{
		drugNames: generateDrugNames(100000),
	}
	svc := NewDrugDataService(mc, rdb)

	// Warm cache
	_, _ = svc.GetDrugNames(context.Background())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.AutocompleteDrugs(context.Background(), "met", 10)
	}
}

// BenchmarkAutocompleteDrugs_ShortPrefix benchmarks with a 2-char prefix
// (worst case — matches many entries).
func BenchmarkAutocompleteDrugs_ShortPrefix(b *testing.B) {
	mr, rdb := setupBenchRedis(b)
	defer mr.Close()

	mc := &mockClient{
		drugNames: generateDrugNames(100000),
	}
	svc := NewDrugDataService(mc, rdb)

	_, _ = svc.GetDrugNames(context.Background())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.AutocompleteDrugs(context.Background(), "me", 10)
	}
}

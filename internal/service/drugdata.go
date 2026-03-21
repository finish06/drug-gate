package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/finish06/drug-gate/internal/cache"
	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/metrics"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/redis/go-redis/v9"
)

// DefaultCacheTTL is the default sliding TTL for cached data (60 minutes).
const DefaultCacheTTL = 60 * time.Minute

// CacheTTL is the active base cache TTL used by DrugDataService and SPLService.
// Set via SetCacheTTL before creating services. Defaults to DefaultCacheTTL.
var CacheTTL = DefaultCacheTTL

// SetCacheTTL sets the base cache TTL for all services that use it.
// RxNorm TTLs scale proportionally from this base.
func SetCacheTTL(ttl time.Duration) {
	CacheTTL = ttl
}

// DrugDataService provides drug data with lazy Redis caching.
type DrugDataService struct {
	client  client.DrugClient
	rdb     *redis.Client
	metrics *metrics.Metrics
}

// NewDrugDataService creates a service with the given client and Redis connection.
// Pass optional metrics to record cache hit/miss counters.
func NewDrugDataService(c client.DrugClient, rdb *redis.Client, m ...*metrics.Metrics) *DrugDataService {
	var met *metrics.Metrics
	if len(m) > 0 {
		met = m[0]
	}
	return &DrugDataService{client: c, rdb: rdb, metrics: met}
}

// GetDrugNames returns all drug names, loading from cache or upstream.
func (s *DrugDataService) GetDrugNames(ctx context.Context) ([]model.DrugNameEntry, error) {
	ca := cache.New[[]model.DrugNameEntry](s.rdb, s.metrics, "cache:drugnames", CacheTTL, "drugnames")
	return ca.Get(ctx, func(ctx context.Context) ([]model.DrugNameEntry, error) {
		raw, err := s.client.FetchDrugNames(ctx)
		if err != nil {
			return nil, err
		}
		entries := make([]model.DrugNameEntry, len(raw))
		for i, r := range raw {
			nameType := "generic"
			if strings.ToUpper(r.NameType) == "B" {
				nameType = "brand"
			}
			entries[i] = model.DrugNameEntry{
				Name: r.DrugName,
				Type: nameType,
			}
		}
		return entries, nil
	})
}

// GetDrugClasses returns all drug classes, loading from cache or upstream.
func (s *DrugDataService) GetDrugClasses(ctx context.Context) ([]model.DrugClassEntry, error) {
	ca := cache.New[[]model.DrugClassEntry](s.rdb, s.metrics, "cache:drugclasses", CacheTTL, "drugclasses")
	return ca.Get(ctx, func(ctx context.Context) ([]model.DrugClassEntry, error) {
		raw, err := s.client.FetchDrugClasses(ctx)
		if err != nil {
			return nil, err
		}
		entries := make([]model.DrugClassEntry, len(raw))
		for i, r := range raw {
			entries[i] = model.DrugClassEntry{
				Name: r.ClassName,
				Type: strings.ToLower(r.ClassType),
			}
		}
		return entries, nil
	})
}

// GetDrugsByClass returns drugs in a given pharmacological class, with caching.
func (s *DrugDataService) GetDrugsByClass(ctx context.Context, className string) ([]model.DrugInClassEntry, error) {
	key := "cache:drugsbyclass:" + strings.ToLower(className)
	ca := cache.New[[]model.DrugInClassEntry](s.rdb, s.metrics, key, CacheTTL, "drugsbyclass")
	return ca.Get(ctx, func(ctx context.Context) ([]model.DrugInClassEntry, error) {
		results, err := s.client.LookupByPharmClass(ctx, className)
		if err != nil {
			return nil, err
		}
		entries := make([]model.DrugInClassEntry, len(results))
		for i, r := range results {
			entries[i] = model.DrugInClassEntry{
				GenericName: r.GenericName,
				BrandName:   r.BrandName,
			}
		}
		return entries, nil
	})
}

// AutocompleteDrugs returns drug names matching the given prefix, sorted
// alphabetically and capped at limit. Reuses the cached drug names from
// GetDrugNames — no additional upstream calls.
func (s *DrugDataService) AutocompleteDrugs(ctx context.Context, prefix string, limit int) ([]model.DrugNameEntry, error) {
	names, err := s.GetDrugNames(ctx)
	if err != nil {
		return nil, err
	}

	lowerPrefix := strings.ToLower(prefix)
	var matches []model.DrugNameEntry
	for _, n := range names {
		if strings.HasPrefix(strings.ToLower(n.Name), lowerPrefix) {
			matches = append(matches, n)
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return strings.ToLower(matches[i].Name) < strings.ToLower(matches[j].Name)
	})

	if len(matches) > limit {
		matches = matches[:limit]
	}

	if matches == nil {
		matches = []model.DrugNameEntry{}
	}

	return matches, nil
}

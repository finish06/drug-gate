package service

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"github.com/finish06/drug-gate/internal/cache"
	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/metrics"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/redis/go-redis/v9"
)

// DefaultCacheTTL is the default sliding TTL for cached data (60 minutes).
const DefaultCacheTTL = 60 * time.Minute

// cacheTTL stores the active base cache TTL as an atomic value for safe concurrent access.
var cacheTTL atomic.Int64

func init() {
	cacheTTL.Store(int64(DefaultCacheTTL))
}

// CacheTTLValue returns the current base cache TTL (thread-safe).
func CacheTTLValue() time.Duration {
	return time.Duration(cacheTTL.Load())
}

// SetCacheTTL sets the base cache TTL for all services that use it.
// RxNorm TTLs scale proportionally from this base. Thread-safe.
func SetCacheTTL(ttl time.Duration) {
	cacheTTL.Store(int64(ttl))
}

// DrugDataService provides drug data with lazy Redis caching.
type DrugDataService struct {
	client      client.DrugClient
	rdb         *redis.Client
	metrics     *metrics.Metrics
	acIndex     *drugNameIndex
}

// NewDrugDataService creates a service with the given client and Redis connection.
// Pass optional metrics to record cache hit/miss counters.
func NewDrugDataService(c client.DrugClient, rdb *redis.Client, m ...*metrics.Metrics) *DrugDataService {
	var met *metrics.Metrics
	if len(m) > 0 {
		met = m[0]
	}
	return &DrugDataService{
		client:  c,
		rdb:     rdb,
		metrics: met,
		acIndex: newDrugNameIndex(CacheTTLValue()),
	}
}

// GetDrugNames returns all drug names, loading from cache or upstream.
// NameLower is always populated for efficient search filtering.
func (s *DrugDataService) GetDrugNames(ctx context.Context) ([]model.DrugNameEntry, error) {
	ca := cache.New[[]model.DrugNameEntry](s.rdb, s.metrics, "cache:drugnames", CacheTTLValue(), "drugnames")
	entries, err := ca.Get(ctx, func(ctx context.Context) ([]model.DrugNameEntry, error) {
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
				Name:      r.DrugName,
				Type:      nameType,
				NameLower: strings.ToLower(r.DrugName),
			}
		}
		return entries, nil
	})
	if err != nil {
		return nil, err
	}
	// Ensure NameLower is populated (lost during JSON cache round-trip)
	for i := range entries {
		if entries[i].NameLower == "" {
			entries[i].NameLower = strings.ToLower(entries[i].Name)
		}
	}
	return entries, nil
}

// GetDrugClasses returns all drug classes, loading from cache or upstream.
func (s *DrugDataService) GetDrugClasses(ctx context.Context) ([]model.DrugClassEntry, error) {
	ca := cache.New[[]model.DrugClassEntry](s.rdb, s.metrics, "cache:drugclasses", CacheTTLValue(), "drugclasses")
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
	ca := cache.New[[]model.DrugInClassEntry](s.rdb, s.metrics, key, CacheTTLValue(), "drugsbyclass")
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
// alphabetically and capped at limit. Uses an in-memory pre-sorted index
// for O(log n) prefix lookup — avoids deserializing 7.4MB JSON per request.
//
// Stampede prevention: when the index is stale, only one goroutine rebuilds it.
// Others either serve stale data (if available) or wait for the rebuild.
func (s *DrugDataService) AutocompleteDrugs(ctx context.Context, prefix string, limit int) ([]model.DrugNameEntry, error) {
	if s.acIndex.isStale() {
		// TryLock: only one goroutine rebuilds; others skip if index has data
		if s.acIndex.loading.TryLock() {
			defer s.acIndex.loading.Unlock()
			// Double-check after acquiring lock
			if s.acIndex.isStale() {
				names, err := s.GetDrugNames(ctx)
				if err != nil {
					// If index has stale data, serve it rather than error
					if !s.acIndex.isEmpty() {
						return s.acIndex.search(prefix, limit), nil
					}
					return nil, err
				}
				s.acIndex.load(names)
			}
		}
		// If we couldn't get the lock but index is empty, wait for rebuild
		if s.acIndex.isEmpty() {
			s.acIndex.loading.Lock()
			s.acIndex.loading.Unlock()
			// After waiting, check if rebuild happened
			if s.acIndex.isEmpty() {
				names, err := s.GetDrugNames(ctx)
				if err != nil {
					return nil, err
				}
				s.acIndex.load(names)
			}
		}
	}

	return s.acIndex.search(prefix, limit), nil
}

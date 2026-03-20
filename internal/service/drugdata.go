package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/metrics"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/redis/go-redis/v9"
)

const cacheTTL = 60 * time.Minute

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

func (s *DrugDataService) recordCache(keyType, outcome string) {
	if s.metrics != nil {
		s.metrics.CacheHitsTotal.WithLabelValues(keyType, outcome).Inc()
	}
}

// GetDrugNames returns all drug names, loading from cache or upstream.
func (s *DrugDataService) GetDrugNames(ctx context.Context) ([]model.DrugNameEntry, error) {
	const key = "cache:drugnames"

	// Try cache
	data, err := s.rdb.GetEx(ctx, key, cacheTTL).Bytes()
	if err == nil {
		var entries []model.DrugNameEntry
		if err := json.Unmarshal(data, &entries); err == nil {
			s.recordCache("drugnames", "hit")
			return entries, nil
		}
		slog.Warn("failed to unmarshal cached drug names, fetching fresh")
	}

	s.recordCache("drugnames", "miss")

	// Cache miss — fetch from upstream
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

	// Cache the result
	if encoded, err := json.Marshal(entries); err == nil {
		if err := s.rdb.Set(ctx, key, encoded, cacheTTL).Err(); err != nil {
			slog.Warn("failed to cache drug names", "err", err)
		}
	}

	return entries, nil
}

// GetDrugClasses returns all drug classes, loading from cache or upstream.
func (s *DrugDataService) GetDrugClasses(ctx context.Context) ([]model.DrugClassEntry, error) {
	const key = "cache:drugclasses"

	// Try cache
	data, err := s.rdb.GetEx(ctx, key, cacheTTL).Bytes()
	if err == nil {
		var entries []model.DrugClassEntry
		if err := json.Unmarshal(data, &entries); err == nil {
			s.recordCache("drugclasses", "hit")
			return entries, nil
		}
		slog.Warn("failed to unmarshal cached drug classes, fetching fresh")
	}

	s.recordCache("drugclasses", "miss")

	// Cache miss — fetch from upstream
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

	if encoded, err := json.Marshal(entries); err == nil {
		if err := s.rdb.Set(ctx, key, encoded, cacheTTL).Err(); err != nil {
			slog.Warn("failed to cache drug classes", "err", err)
		}
	}

	return entries, nil
}

// GetDrugsByClass returns drugs in a given pharmacological class, with caching.
func (s *DrugDataService) GetDrugsByClass(ctx context.Context, className string) ([]model.DrugInClassEntry, error) {
	key := "cache:drugsbyclass:" + strings.ToLower(className)

	// Try cache
	data, err := s.rdb.GetEx(ctx, key, cacheTTL).Bytes()
	if err == nil {
		var entries []model.DrugInClassEntry
		if err := json.Unmarshal(data, &entries); err == nil {
			s.recordCache("drugsbyclass", "hit")
			return entries, nil
		}
		slog.Warn("failed to unmarshal cached drugs-by-class, fetching fresh")
	}

	s.recordCache("drugsbyclass", "miss")

	// Cache miss — fetch from upstream
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

	if encoded, err := json.Marshal(entries); err == nil {
		if err := s.rdb.Set(ctx, key, encoded, cacheTTL).Err(); err != nil {
			slog.Warn("failed to cache drugs-by-class", "err", err)
		}
	}

	return entries, nil
}

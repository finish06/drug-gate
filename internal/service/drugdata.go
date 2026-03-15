package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/redis/go-redis/v9"
)

const cacheTTL = 60 * time.Minute

// DrugDataService provides drug data with lazy Redis caching.
type DrugDataService struct {
	client client.DrugClient
	rdb    *redis.Client
}

// NewDrugDataService creates a service with the given client and Redis connection.
func NewDrugDataService(c client.DrugClient, rdb *redis.Client) *DrugDataService {
	return &DrugDataService{client: c, rdb: rdb}
}

// GetDrugNames returns all drug names, loading from cache or upstream.
func (s *DrugDataService) GetDrugNames(ctx context.Context) ([]model.DrugNameEntry, error) {
	const key = "cache:drugnames"

	// Try cache
	data, err := s.rdb.Get(ctx, key).Bytes()
	if err == nil {
		// Cache hit — reset sliding TTL
		s.rdb.Expire(ctx, key, cacheTTL)
		var entries []model.DrugNameEntry
		if err := json.Unmarshal(data, &entries); err == nil {
			return entries, nil
		}
		slog.Warn("failed to unmarshal cached drug names, fetching fresh")
	}

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
	data, err := s.rdb.Get(ctx, key).Bytes()
	if err == nil {
		s.rdb.Expire(ctx, key, cacheTTL)
		var entries []model.DrugClassEntry
		if err := json.Unmarshal(data, &entries); err == nil {
			return entries, nil
		}
		slog.Warn("failed to unmarshal cached drug classes, fetching fresh")
	}

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
	key := fmt.Sprintf("cache:drugsbyclass:%s", strings.ToLower(className))

	// Try cache
	data, err := s.rdb.Get(ctx, key).Bytes()
	if err == nil {
		s.rdb.Expire(ctx, key, cacheTTL)
		var entries []model.DrugInClassEntry
		if err := json.Unmarshal(data, &entries); err == nil {
			return entries, nil
		}
		slog.Warn("failed to unmarshal cached drugs-by-class, fetching fresh")
	}

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

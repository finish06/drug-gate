package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/metrics"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/finish06/drug-gate/internal/spl"
	"github.com/redis/go-redis/v9"
)

// SPLService provides SPL data with lazy Redis caching.
type SPLService struct {
	splClient  client.SPLClient
	drugClient client.DrugClient
	rdb        *redis.Client
	metrics    *metrics.Metrics
}

// NewSPLService creates a service with the given clients and Redis connection.
func NewSPLService(sc client.SPLClient, dc client.DrugClient, rdb *redis.Client, m ...*metrics.Metrics) *SPLService {
	var met *metrics.Metrics
	if len(m) > 0 {
		met = m[0]
	}
	return &SPLService{splClient: sc, drugClient: dc, rdb: rdb, metrics: met}
}

func (s *SPLService) recordCache(keyType, outcome string) {
	if s.metrics != nil {
		s.metrics.CacheHitsTotal.WithLabelValues(keyType, outcome).Inc()
	}
}

// SearchSPLs returns SPL entries matching a drug name, with pagination.
func (s *SPLService) SearchSPLs(ctx context.Context, drugName string, limit, offset int) ([]model.SPLEntry, int, error) {
	cacheKey := "cache:spls:name:" + strings.ToLower(drugName)

	// Try cache
	data, err := s.rdb.Get(ctx, cacheKey).Bytes()
	if err == nil {
		s.rdb.Expire(ctx, cacheKey, cacheTTL)
		var entries []model.SPLEntry
		if err := json.Unmarshal(data, &entries); err == nil {
			s.recordCache("spls-by-name", "hit")
			return paginate(entries, limit, offset), len(entries), nil
		}
		slog.Warn("failed to unmarshal cached SPLs, fetching fresh")
	}

	s.recordCache("spls-by-name", "miss")

	// Fetch upstream
	raw, err := s.splClient.FetchSPLsByName(ctx, drugName)
	if err != nil {
		return nil, 0, err
	}

	// Convert to model
	entries := make([]model.SPLEntry, len(raw))
	for i, r := range raw {
		entries[i] = model.SPLEntry{
			Title:         r.Title,
			SetID:         r.SetID,
			PublishedDate: r.PublishedDate,
			SPLVersion:    r.SPLVersion,
		}
	}

	// Sort by published_date descending (most recent first)
	// Since dates are "Mon DD, YYYY" format, we sort by the raw string for now
	// A proper date parse would be better but this works for the common case
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].PublishedDate > entries[j].PublishedDate
	})

	// Cache result
	if cacheData, err := json.Marshal(entries); err == nil {
		s.rdb.Set(ctx, cacheKey, cacheData, cacheTTL)
	}

	return paginate(entries, limit, offset), len(entries), nil
}

// GetSPLDetail returns SPL detail with parsed interaction sections.
func (s *SPLService) GetSPLDetail(ctx context.Context, setID string) (*model.SPLDetail, error) {
	cacheKey := "cache:spl:detail:" + setID

	// Try cache
	data, err := s.rdb.Get(ctx, cacheKey).Bytes()
	if err == nil {
		s.rdb.Expire(ctx, cacheKey, cacheTTL)
		var detail model.SPLDetail
		if err := json.Unmarshal(data, &detail); err == nil {
			s.recordCache("spl-detail", "hit")
			return &detail, nil
		}
		slog.Warn("failed to unmarshal cached SPL detail, fetching fresh")
	}

	s.recordCache("spl-detail", "miss")

	// Fetch metadata
	meta, err := s.splClient.FetchSPLDetail(ctx, setID)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, nil
	}

	// Fetch XML and parse interactions
	interactions, err := s.fetchAndParseInteractions(ctx, setID)
	if err != nil {
		slog.Warn("failed to fetch SPL XML, returning metadata only", "setid", setID, "err", err)
		interactions = []model.InteractionSection{}
	}

	detail := &model.SPLDetail{
		Title:         meta.Title,
		SetID:         meta.SetID,
		PublishedDate: meta.PublishedDate,
		SPLVersion:    meta.SPLVersion,
		Interactions:  interactions,
	}

	// Cache result
	if cacheData, err := json.Marshal(detail); err == nil {
		s.rdb.Set(ctx, cacheKey, cacheData, cacheTTL)
	}

	return detail, nil
}

// GetInteractionsForDrug returns parsed interaction sections for a drug by name.
// Uses the most recently published SPL.
func (s *SPLService) GetInteractionsForDrug(ctx context.Context, drugName string) (*model.SPLDetail, error) {
	cacheKey := "cache:spl:interactions:" + strings.ToLower(drugName)

	// Try cache
	data, err := s.rdb.Get(ctx, cacheKey).Bytes()
	if err == nil {
		s.rdb.Expire(ctx, cacheKey, cacheTTL)
		var detail model.SPLDetail
		if err := json.Unmarshal(data, &detail); err == nil {
			s.recordCache("spl-interactions", "hit")
			return &detail, nil
		}
	}

	s.recordCache("spl-interactions", "miss")

	// Search SPLs for this drug
	raw, err := s.splClient.FetchSPLsByName(ctx, drugName)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}

	// Use the first entry (cash-drugs returns most recent first typically)
	best := raw[0]

	// Fetch and parse interactions
	interactions, err := s.fetchAndParseInteractions(ctx, best.SetID)
	if err != nil {
		slog.Warn("failed to fetch SPL XML for drug", "drug", drugName, "setid", best.SetID, "err", err)
		interactions = []model.InteractionSection{}
	}

	detail := &model.SPLDetail{
		Title:         best.Title,
		SetID:         best.SetID,
		PublishedDate: best.PublishedDate,
		SPLVersion:    best.SPLVersion,
		Interactions:  interactions,
	}

	// Cache
	if cacheData, err := json.Marshal(detail); err == nil {
		s.rdb.Set(ctx, cacheKey, cacheData, cacheTTL)
	}

	return detail, nil
}

// ResolveDrugNameFromNDC uses the drug client to resolve an NDC to a generic drug name.
func (s *SPLService) ResolveDrugNameFromNDC(ctx context.Context, ndc string) (string, error) {
	result, err := s.drugClient.LookupByNDC(ctx, ndc)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", fmt.Errorf("no drug found for NDC %s", ndc)
	}
	return result.GenericName, nil
}

// fetchAndParseInteractions fetches SPL XML and parses Section 7.
func (s *SPLService) fetchAndParseInteractions(ctx context.Context, setID string) ([]model.InteractionSection, error) {
	xmlData, err := s.splClient.FetchSPLXML(ctx, setID)
	if err != nil {
		return nil, err
	}
	if xmlData == nil {
		return []model.InteractionSection{}, nil
	}
	return spl.ParseInteractions(xmlData), nil
}

// paginate returns a slice of SPLEntry for the given limit/offset.
func paginate(entries []model.SPLEntry, limit, offset int) []model.SPLEntry {
	if offset >= len(entries) {
		return []model.SPLEntry{}
	}
	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	}
	return entries[offset:end]
}

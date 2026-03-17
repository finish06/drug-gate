package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/metrics"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/redis/go-redis/v9"
)

const (
	rxnormSearchTTL = 24 * time.Hour
	rxnormLookupTTL = 7 * 24 * time.Hour
	maxCandidates   = 5
)

// RxNormService provides RxNorm data with lazy Redis caching.
type RxNormService struct {
	client  client.RxNormClient
	rdb     *redis.Client
	metrics *metrics.Metrics
}

// NewRxNormService creates a service with the given client and Redis connection.
func NewRxNormService(c client.RxNormClient, rdb *redis.Client, m ...*metrics.Metrics) *RxNormService {
	var met *metrics.Metrics
	if len(m) > 0 {
		met = m[0]
	}
	return &RxNormService{client: c, rdb: rdb, metrics: met}
}

func (s *RxNormService) recordCache(keyType, outcome string) {
	if s.metrics != nil {
		s.metrics.CacheHitsTotal.WithLabelValues(keyType, outcome).Inc()
	}
}

// Search performs an approximate match search, returning up to 5 candidates.
// Falls back to spelling suggestions when no candidates are found.
func (s *RxNormService) Search(ctx context.Context, name string) (*model.RxNormSearchResult, error) {
	key := fmt.Sprintf("cache:rxnorm:search:%s", strings.ToLower(name))

	data, err := s.rdb.Get(ctx, key).Bytes()
	if err == nil {
		s.rdb.Expire(ctx, key, rxnormSearchTTL)
		var result model.RxNormSearchResult
		if err := json.Unmarshal(data, &result); err == nil {
			s.recordCache("rxnorm-search", "hit")
			return &result, nil
		}
	}

	s.recordCache("rxnorm-search", "miss")

	rawCandidates, err := s.client.SearchApproximate(ctx, name)
	if err != nil {
		return nil, err
	}

	candidates := make([]model.RxNormCandidate, 0, len(rawCandidates))
	for _, rc := range rawCandidates {
		// Skip entries with no name (e.g., MMSL source entries)
		if rc.Name == "" {
			continue
		}
		// Scores are floats from RxNorm — parse and truncate to int
		score := 0
		if f, err := strconv.ParseFloat(rc.Score, 64); err == nil {
			score = int(f)
		} else {
			slog.Warn("failed to parse rxnorm score, defaulting to 0", "score_raw", rc.Score, "rxcui", rc.RxCUI)
		}
		candidates = append(candidates, model.RxNormCandidate{
			RxCUI: rc.RxCUI,
			Name:  rc.Name,
			Score: score,
		})
	}

	// Sort by score descending, cap at maxCandidates
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	if len(candidates) > maxCandidates {
		candidates = candidates[:maxCandidates]
	}

	result := &model.RxNormSearchResult{
		Query:       name,
		Candidates:  candidates,
		Suggestions: []string{},
	}

	// If no candidates, fetch spelling suggestions
	if len(candidates) == 0 {
		suggestions, err := s.client.FetchSpellingSuggestions(ctx, name)
		if err != nil {
			return nil, err
		}
		if suggestions == nil {
			suggestions = []string{}
		}
		result.Suggestions = suggestions
	}

	if encoded, err := json.Marshal(result); err == nil {
		if err := s.rdb.Set(ctx, key, encoded, rxnormSearchTTL).Err(); err != nil {
			slog.Warn("failed to cache rxnorm search", "err", err)
		}
	}

	return result, nil
}

// GetNDCs returns NDC codes for the given RxCUI.
func (s *RxNormService) GetNDCs(ctx context.Context, rxcui string) (*model.RxNormNDCResponse, error) {
	key := fmt.Sprintf("cache:rxnorm:ndcs:%s", rxcui)

	data, err := s.rdb.Get(ctx, key).Bytes()
	if err == nil {
		s.rdb.Expire(ctx, key, rxnormLookupTTL)
		var result model.RxNormNDCResponse
		if err := json.Unmarshal(data, &result); err == nil {
			s.recordCache("rxnorm-ndcs", "hit")
			return &result, nil
		}
	}

	s.recordCache("rxnorm-ndcs", "miss")

	ndcs, err := s.client.FetchNDCs(ctx, rxcui)
	if err != nil {
		return nil, err
	}
	if ndcs == nil {
		ndcs = []string{}
	}

	result := &model.RxNormNDCResponse{RxCUI: rxcui, NDCs: ndcs}

	if encoded, err := json.Marshal(result); err == nil {
		if err := s.rdb.Set(ctx, key, encoded, rxnormLookupTTL).Err(); err != nil {
			slog.Warn("failed to cache rxnorm ndcs", "err", err)
		}
	}

	return result, nil
}

// GetGenerics returns generic product info for the given RxCUI.
func (s *RxNormService) GetGenerics(ctx context.Context, rxcui string) (*model.RxNormGenericResponse, error) {
	key := fmt.Sprintf("cache:rxnorm:generic:%s", rxcui)

	data, err := s.rdb.Get(ctx, key).Bytes()
	if err == nil {
		s.rdb.Expire(ctx, key, rxnormLookupTTL)
		var result model.RxNormGenericResponse
		if err := json.Unmarshal(data, &result); err == nil {
			s.recordCache("rxnorm-generic", "hit")
			return &result, nil
		}
	}

	s.recordCache("rxnorm-generic", "miss")

	raw, err := s.client.FetchGenericProduct(ctx, rxcui)
	if err != nil {
		return nil, err
	}

	generics := make([]model.RxNormConcept, len(raw))
	for i, r := range raw {
		generics[i] = model.RxNormConcept{RxCUI: r.RxCUI, Name: r.Name}
	}

	result := &model.RxNormGenericResponse{RxCUI: rxcui, Generics: generics}

	if encoded, err := json.Marshal(result); err == nil {
		if err := s.rdb.Set(ctx, key, encoded, rxnormLookupTTL).Err(); err != nil {
			slog.Warn("failed to cache rxnorm generics", "err", err)
		}
	}

	return result, nil
}

// GetRelated returns related concepts grouped by type for the given RxCUI.
func (s *RxNormService) GetRelated(ctx context.Context, rxcui string) (*model.RxNormRelatedResponse, error) {
	key := fmt.Sprintf("cache:rxnorm:related:%s", rxcui)

	data, err := s.rdb.Get(ctx, key).Bytes()
	if err == nil {
		s.rdb.Expire(ctx, key, rxnormLookupTTL)
		var result model.RxNormRelatedResponse
		if err := json.Unmarshal(data, &result); err == nil {
			s.recordCache("rxnorm-related", "hit")
			return &result, nil
		}
	}

	s.recordCache("rxnorm-related", "miss")

	groups, err := s.client.FetchAllRelated(ctx, rxcui)
	if err != nil {
		return nil, err
	}

	result := &model.RxNormRelatedResponse{
		RxCUI:         rxcui,
		Ingredients:   []model.RxNormConcept{},
		BrandNames:    []model.RxNormConcept{},
		DoseForms:     []model.RxNormConcept{},
		ClinicalDrugs: []model.RxNormConcept{},
		BrandedDrugs:  []model.RxNormConcept{},
	}

	for _, g := range groups {
		concepts := make([]model.RxNormConcept, len(g.ConceptProperties))
		for i, cp := range g.ConceptProperties {
			concepts[i] = model.RxNormConcept{RxCUI: cp.RxCUI, Name: cp.Name}
		}
		switch g.TTY {
		case "IN":
			result.Ingredients = concepts
		case "BN":
			result.BrandNames = concepts
		case "DF":
			result.DoseForms = concepts
		case "SCD":
			result.ClinicalDrugs = concepts
		case "SBD":
			result.BrandedDrugs = concepts
		}
	}

	if encoded, err := json.Marshal(result); err == nil {
		if err := s.rdb.Set(ctx, key, encoded, rxnormLookupTTL).Err(); err != nil {
			slog.Warn("failed to cache rxnorm related", "err", err)
		}
	}

	return result, nil
}

// GetProfile assembles a unified drug profile by orchestrating search, NDCs, generics, and related.
func (s *RxNormService) GetProfile(ctx context.Context, name string) (*model.RxNormProfile, error) {
	key := fmt.Sprintf("cache:rxnorm:profile:%s", strings.ToLower(name))

	data, err := s.rdb.Get(ctx, key).Bytes()
	if err == nil {
		s.rdb.Expire(ctx, key, rxnormSearchTTL)
		var result model.RxNormProfile
		if err := json.Unmarshal(data, &result); err == nil {
			s.recordCache("rxnorm-profile", "hit")
			return &result, nil
		}
	}

	s.recordCache("rxnorm-profile", "miss")

	// Step 1: Search for the drug
	searchResult, err := s.Search(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(searchResult.Candidates) == 0 {
		return nil, nil // not found
	}

	best := searchResult.Candidates[0]

	// Step 2: Fetch NDCs, generics, related for the best match
	ndcResp, err := s.GetNDCs(ctx, best.RxCUI)
	if err != nil {
		return nil, err
	}

	genResp, err := s.GetGenerics(ctx, best.RxCUI)
	if err != nil {
		return nil, err
	}

	relResp, err := s.GetRelated(ctx, best.RxCUI)
	if err != nil {
		return nil, err
	}

	// Extract brand names from related BN concepts
	brandNames := make([]string, len(relResp.BrandNames))
	for i, bn := range relResp.BrandNames {
		brandNames[i] = bn.Name
	}

	var generic *model.RxNormConcept
	if len(genResp.Generics) > 0 {
		generic = &genResp.Generics[0]
	}

	profile := &model.RxNormProfile{
		Query:      name,
		RxCUI:      best.RxCUI,
		Name:       best.Name,
		BrandNames: brandNames,
		Generic:    generic,
		NDCs:       ndcResp.NDCs,
		Related:    relResp,
	}

	if encoded, err := json.Marshal(profile); err == nil {
		if err := s.rdb.Set(ctx, key, encoded, rxnormSearchTTL).Err(); err != nil {
			slog.Warn("failed to cache rxnorm profile", "err", err)
		}
	}

	return profile, nil
}

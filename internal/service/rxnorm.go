package service

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/finish06/drug-gate/internal/cache"
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

// Search performs an approximate match search, returning up to 5 candidates.
// Falls back to spelling suggestions when no candidates are found.
func (s *RxNormService) Search(ctx context.Context, name string) (*model.RxNormSearchResult, error) {
	key := "cache:rxnorm:search:" + strings.ToLower(name)
	ca := cache.New[model.RxNormSearchResult](s.rdb, s.metrics, key, rxnormSearchTTL, "rxnorm-search")
	result, err := ca.Get(ctx, func(ctx context.Context) (model.RxNormSearchResult, error) {
		rawCandidates, err := s.client.SearchApproximate(ctx, name)
		if err != nil {
			return model.RxNormSearchResult{}, err
		}

		candidates := make([]model.RxNormCandidate, 0, len(rawCandidates))
		for _, rc := range rawCandidates {
			if rc.Name == "" {
				continue
			}
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

		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Score > candidates[j].Score
		})
		if len(candidates) > maxCandidates {
			candidates = candidates[:maxCandidates]
		}

		result := model.RxNormSearchResult{
			Query:       name,
			Candidates:  candidates,
			Suggestions: []string{},
		}

		if len(candidates) == 0 {
			suggestions, err := s.client.FetchSpellingSuggestions(ctx, name)
			if err != nil {
				return model.RxNormSearchResult{}, err
			}
			if suggestions == nil {
				suggestions = []string{}
			}
			result.Suggestions = suggestions
		}

		return result, nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetNDCs returns NDC codes for the given RxCUI.
func (s *RxNormService) GetNDCs(ctx context.Context, rxcui string) (*model.RxNormNDCResponse, error) {
	key := "cache:rxnorm:ndcs:" + rxcui
	ca := cache.New[model.RxNormNDCResponse](s.rdb, s.metrics, key, rxnormLookupTTL, "rxnorm-ndcs")
	result, err := ca.Get(ctx, func(ctx context.Context) (model.RxNormNDCResponse, error) {
		ndcs, err := s.client.FetchNDCs(ctx, rxcui)
		if err != nil {
			return model.RxNormNDCResponse{}, err
		}
		if ndcs == nil {
			ndcs = []string{}
		}
		return model.RxNormNDCResponse{RxCUI: rxcui, NDCs: ndcs}, nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetGenerics returns generic product info for the given RxCUI.
func (s *RxNormService) GetGenerics(ctx context.Context, rxcui string) (*model.RxNormGenericResponse, error) {
	key := "cache:rxnorm:generic:" + rxcui
	ca := cache.New[model.RxNormGenericResponse](s.rdb, s.metrics, key, rxnormLookupTTL, "rxnorm-generic")
	result, err := ca.Get(ctx, func(ctx context.Context) (model.RxNormGenericResponse, error) {
		raw, err := s.client.FetchGenericProduct(ctx, rxcui)
		if err != nil {
			return model.RxNormGenericResponse{}, err
		}
		generics := make([]model.RxNormConcept, len(raw))
		for i, r := range raw {
			generics[i] = model.RxNormConcept{RxCUI: r.RxCUI, Name: r.Name}
		}
		return model.RxNormGenericResponse{RxCUI: rxcui, Generics: generics}, nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetRelated returns related concepts grouped by type for the given RxCUI.
func (s *RxNormService) GetRelated(ctx context.Context, rxcui string) (*model.RxNormRelatedResponse, error) {
	key := "cache:rxnorm:related:" + rxcui
	ca := cache.New[model.RxNormRelatedResponse](s.rdb, s.metrics, key, rxnormLookupTTL, "rxnorm-related")
	result, err := ca.Get(ctx, func(ctx context.Context) (model.RxNormRelatedResponse, error) {
		groups, err := s.client.FetchAllRelated(ctx, rxcui)
		if err != nil {
			return model.RxNormRelatedResponse{}, err
		}
		resp := model.RxNormRelatedResponse{
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
				resp.Ingredients = concepts
			case "BN":
				resp.BrandNames = concepts
			case "DF":
				resp.DoseForms = concepts
			case "SCD":
				resp.ClinicalDrugs = concepts
			case "SBD":
				resp.BrandedDrugs = concepts
			}
		}
		return resp, nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetProfile assembles a unified drug profile by orchestrating search, NDCs, generics, and related.
func (s *RxNormService) GetProfile(ctx context.Context, name string) (*model.RxNormProfile, error) {
	key := "cache:rxnorm:profile:" + strings.ToLower(name)
	ca := cache.New[model.RxNormProfile](s.rdb, s.metrics, key, rxnormSearchTTL, "rxnorm-profile")
	result, err := ca.Get(ctx, func(ctx context.Context) (model.RxNormProfile, error) {
		searchResult, err := s.Search(ctx, name)
		if err != nil {
			return model.RxNormProfile{}, err
		}
		if searchResult == nil || len(searchResult.Candidates) == 0 {
			return model.RxNormProfile{}, nil
		}

		best := searchResult.Candidates[0]

		ndcResp, err := s.GetNDCs(ctx, best.RxCUI)
		if err != nil {
			return model.RxNormProfile{}, err
		}

		genResp, err := s.GetGenerics(ctx, best.RxCUI)
		if err != nil {
			return model.RxNormProfile{}, err
		}

		relResp, err := s.GetRelated(ctx, best.RxCUI)
		if err != nil {
			return model.RxNormProfile{}, err
		}

		brandNames := make([]string, len(relResp.BrandNames))
		for i, bn := range relResp.BrandNames {
			brandNames[i] = bn.Name
		}

		var generic *model.RxNormConcept
		if len(genResp.Generics) > 0 {
			generic = &genResp.Generics[0]
		}

		return model.RxNormProfile{
			Query:      name,
			RxCUI:      best.RxCUI,
			Name:       best.Name,
			BrandNames: brandNames,
			Generic:    generic,
			NDCs:       ndcResp.NDCs,
			Related:    relResp,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	// Not-found: Search returned no candidates → zero-value profile cached
	if result.RxCUI == "" {
		return nil, nil
	}
	return &result, nil
}

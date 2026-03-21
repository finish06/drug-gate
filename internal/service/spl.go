package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/finish06/drug-gate/internal/cache"
	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/metrics"
	"github.com/finish06/drug-gate/internal/model"
	splpkg "github.com/finish06/drug-gate/internal/spl"
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

// SearchSPLs returns SPL entries matching a drug name, with pagination.
func (s *SPLService) SearchSPLs(ctx context.Context, drugName string, limit, offset int) ([]model.SPLEntry, int, error) {
	cacheKey := "cache:spls:name:" + strings.ToLower(drugName)
	ca := cache.New[[]model.SPLEntry](s.rdb, s.metrics, cacheKey, cacheTTL, "spls-by-name")
	entries, err := ca.Get(ctx, func(ctx context.Context) ([]model.SPLEntry, error) {
		raw, err := s.splClient.FetchSPLsByName(ctx, drugName)
		if err != nil {
			return nil, err
		}
		entries := make([]model.SPLEntry, len(raw))
		for i, r := range raw {
			entries[i] = model.SPLEntry{
				Title:         r.Title,
				SetID:         r.SetID,
				PublishedDate: r.PublishedDate,
				SPLVersion:    r.SPLVersion,
			}
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].PublishedDate > entries[j].PublishedDate
		})
		return entries, nil
	})
	if err != nil {
		return nil, 0, err
	}
	return paginate(entries, limit, offset), len(entries), nil
}

// GetSPLDetail returns SPL detail with parsed interaction sections.
func (s *SPLService) GetSPLDetail(ctx context.Context, setID string) (*model.SPLDetail, error) {
	cacheKey := "cache:spl:detail:" + setID
	ca := cache.New[model.SPLDetail](s.rdb, s.metrics, cacheKey, cacheTTL, "spl-detail")
	result, err := ca.Get(ctx, func(ctx context.Context) (model.SPLDetail, error) {
		meta, err := s.splClient.FetchSPLDetail(ctx, setID)
		if err != nil {
			return model.SPLDetail{}, err
		}
		if meta == nil {
			return model.SPLDetail{}, nil
		}

		interactions, err := s.fetchAndParseInteractions(ctx, setID)
		if err != nil {
			slog.Warn("failed to fetch SPL XML, returning metadata only", "setid", setID, "err", err)
			interactions = []model.InteractionSection{}
		}

		return model.SPLDetail{
			Title:         meta.Title,
			SetID:         meta.SetID,
			PublishedDate: meta.PublishedDate,
			SPLVersion:    meta.SPLVersion,
			Interactions:  interactions,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	if result.SetID == "" {
		return nil, nil
	}
	return &result, nil
}

// GetInteractionsForDrug returns parsed interaction sections for a drug by name.
// Uses the most recently published SPL.
func (s *SPLService) GetInteractionsForDrug(ctx context.Context, drugName string) (*model.SPLDetail, error) {
	cacheKey := "cache:spl:interactions:" + strings.ToLower(drugName)
	ca := cache.New[model.SPLDetail](s.rdb, s.metrics, cacheKey, cacheTTL, "spl-interactions")
	result, err := ca.Get(ctx, func(ctx context.Context) (model.SPLDetail, error) {
		raw, err := s.splClient.FetchSPLsByName(ctx, drugName)
		if err != nil {
			return model.SPLDetail{}, err
		}
		if len(raw) == 0 {
			return model.SPLDetail{}, nil
		}

		best := raw[0]

		interactions, err := s.fetchAndParseInteractions(ctx, best.SetID)
		if err != nil {
			slog.Warn("failed to fetch SPL XML for drug", "drug", drugName, "setid", best.SetID, "err", err)
			interactions = []model.InteractionSection{}
		}

		return model.SPLDetail{
			Title:         best.Title,
			SetID:         best.SetID,
			PublishedDate: best.PublishedDate,
			SPLVersion:    best.SPLVersion,
			Interactions:  interactions,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	if result.SetID == "" {
		return nil, nil
	}
	return &result, nil
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

// CheckInteractions resolves multiple drugs and cross-references their interaction sections.
func (s *SPLService) CheckInteractions(ctx context.Context, drugs []model.DrugIdentifier) (*model.InteractionCheckResponse, error) {
	type resolvedDrug struct {
		result model.DrugCheckResult
		detail *model.SPLDetail
	}

	resolved := make([]resolvedDrug, len(drugs))

	// Phase 1: Resolve all drugs and fetch their interaction data
	for i, d := range drugs {
		var drugName, inputName, inputType string

		if d.NDC != "" {
			inputType = "ndc"
			inputName = d.NDC
			name, err := s.ResolveDrugNameFromNDC(ctx, d.NDC)
			if err != nil {
				resolved[i] = resolvedDrug{
					result: model.DrugCheckResult{
						InputName: inputName, InputType: inputType,
						Error: err.Error(),
					},
				}
				continue
			}
			drugName = name
		} else {
			inputType = "name"
			inputName = d.Name
			drugName = d.Name
		}

		detail, err := s.GetInteractionsForDrug(ctx, drugName)
		if err != nil {
			resolved[i] = resolvedDrug{
				result: model.DrugCheckResult{
					InputName: inputName, InputType: inputType, ResolvedName: drugName,
					Error: err.Error(),
				},
			}
			continue
		}

		hasInteractions := detail != nil && len(detail.Interactions) > 0
		setID := ""
		if detail != nil {
			setID = detail.SetID
		}

		resolved[i] = resolvedDrug{
			result: model.DrugCheckResult{
				InputName:       inputName,
				InputType:       inputType,
				ResolvedName:    drugName,
				HasInteractions: hasInteractions,
				SPLSetID:        setID,
			},
			detail: detail,
		}
	}

	// Phase 2: Cross-reference all pairs
	var matches []model.InteractionMatch
	checkedPairs := 0

	for i := 0; i < len(resolved); i++ {
		for j := i + 1; j < len(resolved); j++ {
			a := resolved[i]
			b := resolved[j]

			if a.result.Error != "" || b.result.Error != "" {
				continue
			}
			if strings.EqualFold(a.result.ResolvedName, b.result.ResolvedName) {
				continue
			}

			checkedPairs++

			xrefs := splpkg.CrossReference(a.result.ResolvedName, a.detail, b.result.ResolvedName)
			for _, xr := range xrefs {
				matches = append(matches, model.InteractionMatch{
					DrugA: xr.DrugA, DrugB: xr.DrugB, Source: xr.Source,
					SectionTitle: xr.SectionTitle, Text: xr.Text, SPLSetID: xr.SPLSetID,
				})
			}

			xrefs = splpkg.CrossReference(b.result.ResolvedName, b.detail, a.result.ResolvedName)
			for _, xr := range xrefs {
				matches = append(matches, model.InteractionMatch{
					DrugA: xr.DrugA, DrugB: xr.DrugB, Source: xr.Source,
					SectionTitle: xr.SectionTitle, Text: xr.Text, SPLSetID: xr.SPLSetID,
				})
			}
		}
	}

	drugResults := make([]model.DrugCheckResult, len(resolved))
	for i, r := range resolved {
		drugResults[i] = r.result
	}

	return &model.InteractionCheckResponse{
		Drugs:             drugResults,
		Interactions:      matches,
		CheckedPairs:      checkedPairs,
		FoundInteractions: len(matches),
	}, nil
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
	return splpkg.ParseInteractions(xmlData), nil
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

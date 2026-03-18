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

// CheckInteractions resolves multiple drugs and cross-references their interaction sections.
func (s *SPLService) CheckInteractions(ctx context.Context, drugs []model.DrugIdentifier) (*model.InteractionCheckResponse, error) {
	type resolvedDrug struct {
		result  model.DrugCheckResult
		detail  *model.SPLDetail
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

			// Skip if either drug failed to resolve
			if a.result.Error != "" || b.result.Error != "" {
				continue
			}
			// Skip self-pair (same resolved name)
			if strings.EqualFold(a.result.ResolvedName, b.result.ResolvedName) {
				continue
			}

			checkedPairs++

			// Check A's sections for mentions of B
			xrefs := splpkg.CrossReference(a.result.ResolvedName, a.detail, b.result.ResolvedName)
			for _, xr := range xrefs {
				matches = append(matches, model.InteractionMatch{
					DrugA:        xr.DrugA,
					DrugB:        xr.DrugB,
					Source:       xr.Source,
					SectionTitle: xr.SectionTitle,
					Text:         xr.Text,
					SPLSetID:     xr.SPLSetID,
				})
			}

			// Check B's sections for mentions of A
			xrefs = splpkg.CrossReference(b.result.ResolvedName, b.detail, a.result.ResolvedName)
			for _, xr := range xrefs {
				matches = append(matches, model.InteractionMatch{
					DrugA:        xr.DrugA,
					DrugB:        xr.DrugB,
					Source:       xr.Source,
					SectionTitle: xr.SectionTitle,
					Text:         xr.Text,
					SPLSetID:     xr.SPLSetID,
				})
			}
		}
	}

	// Build response
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

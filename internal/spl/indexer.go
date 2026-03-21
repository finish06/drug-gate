package spl

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/redis/go-redis/v9"
)

const (
	// interactionCachePrefix is the Redis key prefix for cached interaction data.
	interactionCachePrefix = "cache:spl:interactions:"
)

// DefaultIndexerCacheTTL is the default TTL for cached interaction data.
const DefaultIndexerCacheTTL = 60 * time.Minute

// IndexerCacheTTL is the active TTL used by the indexer for caching.
// Set via SetIndexerCacheTTL to align with service.CacheTTL.
var IndexerCacheTTL = DefaultIndexerCacheTTL

// SetIndexerCacheTTL updates the TTL used by the indexer.
func SetIndexerCacheTTL(ttl time.Duration) {
	IndexerCacheTTL = ttl
}

// Indexer pre-fetches and caches parsed interaction data for popular drugs.
type Indexer struct {
	splClient client.SPLClient
	rdb       *redis.Client
	interval  time.Duration
	maxDrugs  int

	stopOnce sync.Once
	stopCh   chan struct{}
}

// NewIndexer creates a background indexer.
// interval controls how often the indexer re-indexes (0 = run once only).
// maxDrugs limits how many drugs to index per run.
func NewIndexer(sc client.SPLClient, rdb *redis.Client, interval time.Duration, maxDrugs int) *Indexer {
	return &Indexer{
		splClient: sc,
		rdb:       rdb,
		interval:  interval,
		maxDrugs:  maxDrugs,
		stopCh:    make(chan struct{}),
	}
}

// Start begins background indexing in a goroutine. Does not block.
func (idx *Indexer) Start() {
	go idx.run()
}

// Stop signals the indexer to stop and waits for the current iteration to finish.
func (idx *Indexer) Stop() {
	idx.stopOnce.Do(func() {
		close(idx.stopCh)
	})
}

func (idx *Indexer) run() {
	// Run immediately on start
	idx.indexOnce()

	if idx.interval <= 0 {
		return
	}

	ticker := time.NewTicker(idx.interval)
	defer ticker.Stop()

	for {
		select {
		case <-idx.stopCh:
			return
		case <-ticker.C:
			idx.indexOnce()
		}
	}
}

func (idx *Indexer) indexOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Get drug names from the drugnames cache
	drugNames, err := idx.getDrugNamesFromCache(ctx)
	if err != nil {
		slog.Warn("indexer: failed to read drug names from cache", "err", err)
		return
	}
	if len(drugNames) == 0 {
		slog.Info("indexer: no drug names in cache, skipping")
		return
	}

	// Limit to maxDrugs
	if len(drugNames) > idx.maxDrugs {
		drugNames = drugNames[:idx.maxDrugs]
	}

	indexed := 0
	skipped := 0
	for _, name := range drugNames {
		select {
		case <-idx.stopCh:
			slog.Info("indexer: stopped mid-run", "indexed", indexed, "skipped", skipped)
			return
		default:
		}

		cacheKey := interactionCachePrefix + strings.ToLower(name)

		// Skip if already cached
		if idx.rdb.Exists(ctx, cacheKey).Val() > 0 {
			skipped++
			continue
		}

		// Fetch SPLs for this drug
		spls, err := idx.splClient.FetchSPLsByName(ctx, name)
		if err != nil {
			slog.Debug("indexer: failed to fetch SPLs", "drug", name, "err", err)
			continue
		}
		if len(spls) == 0 {
			continue
		}

		// Fetch XML and parse interactions from the first (most recent) SPL
		xmlData, err := idx.splClient.FetchSPLXML(ctx, spls[0].SetID)
		if err != nil {
			slog.Debug("indexer: failed to fetch XML", "drug", name, "setid", spls[0].SetID, "err", err)
			continue
		}
		if xmlData == nil {
			continue
		}

		interactions := ParseInteractions(xmlData)

		detail := model.SPLDetail{
			Title:         spls[0].Title,
			SetID:         spls[0].SetID,
			PublishedDate: spls[0].PublishedDate,
			SPLVersion:    spls[0].SPLVersion,
			Interactions:  interactions,
		}

		data, err := json.Marshal(detail)
		if err != nil {
			continue
		}

		idx.rdb.Set(ctx, cacheKey, data, IndexerCacheTTL)
		indexed++
	}

	slog.Info("indexer: run complete", "indexed", indexed, "skipped", skipped, "total_drugs", len(drugNames))
}

// getDrugNamesFromCache reads the drugnames list from Redis cache.
func (idx *Indexer) getDrugNamesFromCache(ctx context.Context) ([]string, error) {
	data, err := idx.rdb.Get(ctx, "cache:drugnames").Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// The drugnames cache stores []model.DrugNameEntry as JSON
	var entries []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	seen := make(map[string]bool)
	for _, e := range entries {
		lower := strings.ToLower(e.Name)
		if !seen[lower] {
			seen[lower] = true
			names = append(names, e.Name)
		}
	}

	return names, nil
}

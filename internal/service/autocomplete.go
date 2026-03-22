package service

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/finish06/drug-gate/internal/model"
)

// drugNameIndex is a pre-sorted, pre-lowercased index of drug names
// for fast prefix matching. Loaded once from GetDrugNames and refreshed
// periodically to avoid 7.4MB JSON deserialization on every autocomplete request.
type drugNameIndex struct {
	mu       sync.RWMutex
	entries  []indexedEntry
	loadedAt time.Time
	maxAge   time.Duration
}

type indexedEntry struct {
	lower string // pre-lowercased name for prefix matching
	entry model.DrugNameEntry
}

func newDrugNameIndex(maxAge time.Duration) *drugNameIndex {
	return &drugNameIndex{maxAge: maxAge}
}

// load populates the index from drug name entries. Sorts by lowercase name.
func (idx *drugNameIndex) load(entries []model.DrugNameEntry) {
	indexed := make([]indexedEntry, len(entries))
	for i, e := range entries {
		indexed[i] = indexedEntry{
			lower: strings.ToLower(e.Name),
			entry: e,
		}
	}

	sort.Slice(indexed, func(i, j int) bool {
		return indexed[i].lower < indexed[j].lower
	})

	idx.mu.Lock()
	idx.entries = indexed
	idx.loadedAt = time.Now()
	idx.mu.Unlock()
}

// isStale returns true if the index needs refreshing.
func (idx *drugNameIndex) isStale() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.entries) == 0 || time.Since(idx.loadedAt) > idx.maxAge
}

// search returns entries matching the prefix, capped at limit.
// Uses binary search to find the start position, then scans forward.
func (idx *drugNameIndex) search(prefix string, limit int) []model.DrugNameEntry {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.entries) == 0 {
		return []model.DrugNameEntry{}
	}

	lowerPrefix := strings.ToLower(prefix)

	// Binary search for the first entry >= prefix
	start := sort.Search(len(idx.entries), func(i int) bool {
		return idx.entries[i].lower >= lowerPrefix
	})

	var results []model.DrugNameEntry
	for i := start; i < len(idx.entries) && len(results) < limit; i++ {
		if !strings.HasPrefix(idx.entries[i].lower, lowerPrefix) {
			break // past prefix range in sorted order
		}
		results = append(results, idx.entries[i].entry)
	}

	if results == nil {
		results = []model.DrugNameEntry{}
	}

	return results
}

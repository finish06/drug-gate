package spl

import (
	"regexp"
	"strings"

	"github.com/finish06/drug-gate/internal/model"
)

// CrossReferenceResult holds the interactions found between two drugs.
type CrossReferenceResult struct {
	DrugA        string
	DrugB        string
	Source       string // which drug's label contained the match
	SectionTitle string
	Text         string
	SPLSetID     string
}

// CrossReference checks if any of drugB's names appear in drugA's interaction sections.
// Returns all matches found. Matching is case-insensitive and uses word boundaries.
func CrossReference(drugA string, splA *model.SPLDetail, drugB string) []CrossReferenceResult {
	if splA == nil || len(splA.Interactions) == 0 {
		return nil
	}

	// Build a regex for drugB name with word boundary
	// Escape special regex chars in drug name
	escaped := regexp.QuoteMeta(strings.ToLower(drugB))
	pattern, err := regexp.Compile(`(?i)\b` + escaped + `\b`)
	if err != nil {
		return nil
	}

	var results []CrossReferenceResult

	for _, section := range splA.Interactions {
		if pattern.MatchString(section.Text) {
			results = append(results, CrossReferenceResult{
				DrugA:        drugA,
				DrugB:        drugB,
				Source:       drugA,
				SectionTitle: section.Title,
				Text:         section.Text,
				SPLSetID:     splA.SetID,
			})
		}
	}

	return results
}

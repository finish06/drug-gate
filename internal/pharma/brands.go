package pharma

import (
	"strings"
	"unicode"
)

// DeduplicateBrandNames deduplicates brand names case-insensitively
// and normalizes them to title case. Empty strings are filtered out.
// Returns an empty (non-nil) slice for nil or empty input.
func DeduplicateBrandNames(names []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, name := range names {
		if name == "" {
			continue
		}
		lower := strings.ToLower(name)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		result = append(result, toTitleCase(name))
	}
	return result
}

// toTitleCase converts a string to title case: first letter uppercase, rest lowercase.
func toTitleCase(s string) string {
	runes := []rune(strings.ToLower(s))
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
	}
	return string(runes)
}

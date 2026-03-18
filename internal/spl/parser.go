package spl

import (
	"regexp"
	"strings"

	"github.com/finish06/drug-gate/internal/model"
)

var (
	// titleRe matches Section 7 titles in both formats:
	// - Numbered: "7 DRUG INTERACTIONS", "7.1 General Information", etc.
	// - Unnumbered (older SPLs): "Drug Interactions" under PRECAUTIONS
	titleRe = regexp.MustCompile(`<title>((?:7(?:\.\d+)?[^<]*)|(?:Drug Interactions[^<]*))</title>`)

	// xmlTagRe strips all XML tags.
	xmlTagRe = regexp.MustCompile(`<[^>]+>`)

	// whitespaceRe collapses multiple whitespace characters.
	whitespaceRe = regexp.MustCompile(`\s+`)
)

// ParseInteractions extracts Section 7 (Drug Interactions) from SPL XML.
// Returns an empty slice if no Section 7 is found (e.g., OTC products).
func ParseInteractions(xmlData []byte) []model.InteractionSection {
	xml := string(xmlData)

	// Find all Section 7 titles and their positions
	matches := titleRe.FindAllStringSubmatchIndex(xml, -1)
	if len(matches) == 0 {
		return []model.InteractionSection{}
	}

	var sections []model.InteractionSection

	for _, match := range matches {
		// match[2]:match[3] is the captured title group
		title := strings.TrimSpace(xml[match[2]:match[3]])

		// Find the <text> block after this title
		afterTitle := xml[match[1]:]
		text := extractTextBlock(afterTitle)
		if text == "" {
			continue
		}

		sections = append(sections, model.InteractionSection{
			Title: title,
			Text:  text,
		})
	}

	if sections == nil {
		return []model.InteractionSection{}
	}

	return sections
}

// extractTextBlock finds the first <text>...</text> block and returns cleaned content.
func extractTextBlock(xmlFragment string) string {
	start := strings.Index(xmlFragment, "<text>")
	if start < 0 {
		return ""
	}
	end := strings.Index(xmlFragment[start:], "</text>")
	if end < 0 {
		return ""
	}

	raw := xmlFragment[start+6 : start+end]
	return cleanXMLText(raw)
}

// cleanXMLText strips XML tags and normalizes whitespace.
func cleanXMLText(raw string) string {
	// Strip all XML tags
	text := xmlTagRe.ReplaceAllString(raw, " ")
	// Collapse whitespace
	text = whitespaceRe.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

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

	// Section 4: Contraindications
	contraTitleRe = regexp.MustCompile(`<title>((?:4(?:\.\d+)?[^<]*)|(?:CONTRAINDICATIONS[^<]*))</title>`)

	// Section 5: Warnings and Precautions
	warningsTitleRe = regexp.MustCompile(`<title>((?:5(?:\.\d+)?[^<]*)|(?:WARNINGS AND PRECAUTIONS[^<]*))</title>`)

	// Section 6: Adverse Reactions
	adverseTitleRe = regexp.MustCompile(`<title>((?:6(?:\.\d+)?[^<]*)|(?:ADVERSE REACTIONS[^<]*))</title>`)

	// xmlTagRe strips all XML tags.
	xmlTagRe = regexp.MustCompile(`<[^>]+>`)

	// whitespaceRe collapses multiple whitespace characters.
	whitespaceRe = regexp.MustCompile(`\s+`)
)

// SectionsResult holds all parsed clinical sections from an SPL document.
type SectionsResult struct {
	Interactions      []model.InteractionSection
	Contraindications []model.InteractionSection
	Warnings          []model.InteractionSection
	AdverseReactions  []model.InteractionSection
}

// ParseInteractions extracts Section 7 (Drug Interactions) from SPL XML.
// Returns an empty slice if no Section 7 is found (e.g., OTC products).
// This function is preserved for backward compatibility.
func ParseInteractions(xmlData []byte) []model.InteractionSection {
	return parseSectionsByRegex(string(xmlData), titleRe)
}

// ParseSections extracts sections 4 (Contraindications), 5 (Warnings),
// 6 (Adverse Reactions), and 7 (Interactions) from SPL XML.
// Missing sections return empty slices.
func ParseSections(xmlData []byte) SectionsResult {
	xml := string(xmlData)
	return SectionsResult{
		Interactions:      parseSectionsByRegex(xml, titleRe),
		Contraindications: parseSectionsByRegex(xml, contraTitleRe),
		Warnings:          parseSectionsByRegex(xml, warningsTitleRe),
		AdverseReactions:  parseSectionsByRegex(xml, adverseTitleRe),
	}
}

// parseSectionsByRegex extracts sections matching the given title regex.
func parseSectionsByRegex(xml string, re *regexp.Regexp) []model.InteractionSection {
	matches := re.FindAllStringSubmatchIndex(xml, -1)
	if len(matches) == 0 {
		return []model.InteractionSection{}
	}

	var sections []model.InteractionSection

	for _, match := range matches {
		title := strings.TrimSpace(xml[match[2]:match[3]])

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

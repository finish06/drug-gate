package spl

import (
	"testing"

	"github.com/finish06/drug-gate/internal/model"
)

func TestCrossReference_MatchFound(t *testing.T) {
	splA := &model.SPLDetail{
		SetID: "war-123",
		Interactions: []model.InteractionSection{
			{Title: "7.3 Drugs that Increase Bleeding Risk", Text: "Antiplatelet Agents aspirin, clopidogrel, dipyridamole"},
			{Title: "7.2 CYP450 Interactions", Text: "CYP3A4 inhibitors atorvastatin, clarithromycin"},
		},
	}

	results := CrossReference("warfarin", splA, "aspirin")
	if len(results) != 1 {
		t.Fatalf("expected 1 match, got %d", len(results))
	}
	if results[0].SectionTitle != "7.3 Drugs that Increase Bleeding Risk" {
		t.Errorf("section = %q", results[0].SectionTitle)
	}
	if results[0].Source != "warfarin" {
		t.Errorf("source = %q, want warfarin", results[0].Source)
	}
	if results[0].SPLSetID != "war-123" {
		t.Errorf("setid = %q, want war-123", results[0].SPLSetID)
	}
}

func TestCrossReference_CaseInsensitive(t *testing.T) {
	splA := &model.SPLDetail{
		SetID: "abc",
		Interactions: []model.InteractionSection{
			{Title: "7.2 CYP450", Text: "CYP2C9 inhibitors include Fluconazole and voriconazole"},
		},
	}

	results := CrossReference("warfarin", splA, "fluconazole")
	if len(results) != 1 {
		t.Fatalf("expected 1 match (case-insensitive), got %d", len(results))
	}
}

func TestCrossReference_NoMatch(t *testing.T) {
	splA := &model.SPLDetail{
		SetID: "abc",
		Interactions: []model.InteractionSection{
			{Title: "7 DRUG INTERACTIONS", Text: "Some general interaction info about warfarin"},
		},
	}

	results := CrossReference("warfarin", splA, "lisinopril")
	if len(results) != 0 {
		t.Errorf("expected 0 matches, got %d", len(results))
	}
}

func TestCrossReference_MultipleMatches(t *testing.T) {
	splA := &model.SPLDetail{
		SetID: "abc",
		Interactions: []model.InteractionSection{
			{Title: "7.2 CYP450", Text: "CYP3A4 inhibitors include atorvastatin"},
			{Title: "7.3 Bleeding Risk", Text: "Also atorvastatin may contribute to bleeding risk"},
		},
	}

	results := CrossReference("warfarin", splA, "atorvastatin")
	if len(results) != 2 {
		t.Fatalf("expected 2 matches across sections, got %d", len(results))
	}
}

func TestCrossReference_NilSPL(t *testing.T) {
	results := CrossReference("warfarin", nil, "aspirin")
	if len(results) != 0 {
		t.Errorf("expected 0 for nil SPL, got %d", len(results))
	}
}

func TestCrossReference_EmptyInteractions(t *testing.T) {
	splA := &model.SPLDetail{
		SetID:        "abc",
		Interactions: []model.InteractionSection{},
	}

	results := CrossReference("warfarin", splA, "aspirin")
	if len(results) != 0 {
		t.Errorf("expected 0 for empty interactions, got %d", len(results))
	}
}

func TestCrossReference_WordBoundary(t *testing.T) {
	splA := &model.SPLDetail{
		SetID: "abc",
		Interactions: []model.InteractionSection{
			{Title: "7.1 General", Text: "metformin hydrochloride is metabolized differently"},
		},
	}

	// "met" should NOT match "metformin" (word boundary)
	results := CrossReference("drug", splA, "met")
	if len(results) != 0 {
		t.Errorf("expected 0 matches for partial word 'met', got %d", len(results))
	}

	// "metformin" should match
	results = CrossReference("drug", splA, "metformin")
	if len(results) != 1 {
		t.Errorf("expected 1 match for 'metformin', got %d", len(results))
	}
}

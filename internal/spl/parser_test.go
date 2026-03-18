package spl

import (
	"strings"
	"testing"
)

const testSPLXML = `<?xml version="1.0" encoding="UTF-8"?>
<document xmlns="urn:hl7-org:v3">
  <component>
    <structuredBody>
      <component>
        <section>
          <code code="34073-7" codeSystem="2.16.840.1.113883.6.1" displayName="DRUG INTERACTIONS SECTION"/>
          <title>7 DRUG INTERACTIONS</title>
          <text>
            Concomitant use of drugs that increase bleeding risk,
            antibiotics, antifungals, botanical (herbal) products.
          </text>
        </section>
      </component>
      <component>
        <section>
          <code code="43685-7" codeSystem="2.16.840.1.113883.6.1"/>
          <title>7.1 General Information</title>
          <text>
            Drugs may interact with warfarin sodium through
            <content styleCode="bold">pharmacodynamic</content> or
            pharmacokinetic mechanisms.
          </text>
        </section>
      </component>
      <component>
        <section>
          <code code="43685-7" codeSystem="2.16.840.1.113883.6.1"/>
          <title>7.2 CYP450 Interactions</title>
          <text>
            <paragraph>CYP450 isozymes involved in the metabolism of warfarin include
            CYP2C9, 2C19, 2C8, 2C18, 1A2, and 3A4.</paragraph>
            <table>
              <thead><tr><th>Enzyme</th><th>Inhibitors</th><th>Inducers</th></tr></thead>
              <tbody>
                <tr><td>CYP2C9</td><td>amiodarone, fluconazole, voriconazole</td><td>rifampin</td></tr>
                <tr><td>CYP3A4</td><td>atorvastatin, clarithromycin, erythromycin</td><td>phenytoin</td></tr>
              </tbody>
            </table>
          </text>
        </section>
      </component>
      <component>
        <section>
          <code code="43685-7" codeSystem="2.16.840.1.113883.6.1"/>
          <title>7.3 Drugs that Increase Bleeding Risk</title>
          <text>
            <table>
              <thead><tr><th>Drug Class</th><th>Specific Drugs</th></tr></thead>
              <tbody>
                <tr><td>Antiplatelet Agents</td><td>aspirin, clopidogrel, dipyridamole</td></tr>
                <tr><td>NSAIDs</td><td>ibuprofen, naproxen, celecoxib</td></tr>
              </tbody>
            </table>
          </text>
        </section>
      </component>
    </structuredBody>
  </component>
</document>`

func TestParseInteractions_FullSPL(t *testing.T) {
	sections := ParseInteractions([]byte(testSPLXML))

	if len(sections) != 4 {
		t.Fatalf("expected 4 sections, got %d", len(sections))
	}

	// Section 7 header
	if sections[0].Title != "7 DRUG INTERACTIONS" {
		t.Errorf("section[0] title = %q, want %q", sections[0].Title, "7 DRUG INTERACTIONS")
	}
	if !strings.Contains(sections[0].Text, "bleeding risk") {
		t.Errorf("section[0] text should contain 'bleeding risk', got: %s", sections[0].Text)
	}

	// 7.1 General Information
	if sections[1].Title != "7.1 General Information" {
		t.Errorf("section[1] title = %q, want %q", sections[1].Title, "7.1 General Information")
	}
	if !strings.Contains(sections[1].Text, "pharmacodynamic") {
		t.Errorf("section[1] text should contain 'pharmacodynamic'")
	}
	// XML tags should be stripped
	if strings.Contains(sections[1].Text, "<content") {
		t.Errorf("section[1] text should not contain XML tags")
	}

	// 7.2 CYP450 — should contain table content as text
	if sections[2].Title != "7.2 CYP450 Interactions" {
		t.Errorf("section[2] title = %q, want %q", sections[2].Title, "7.2 CYP450 Interactions")
	}
	if !strings.Contains(sections[2].Text, "fluconazole") {
		t.Errorf("section[2] text should contain 'fluconazole'")
	}
	if !strings.Contains(sections[2].Text, "atorvastatin") {
		t.Errorf("section[2] text should contain 'atorvastatin'")
	}

	// 7.3 Bleeding risk drugs
	if !strings.Contains(sections[3].Text, "aspirin") {
		t.Errorf("section[3] text should contain 'aspirin'")
	}
}

func TestParseInteractions_NoSection7(t *testing.T) {
	xml := `<?xml version="1.0"?>
<document>
  <component>
    <section>
      <title>1 INDICATIONS AND USAGE</title>
      <text>Some OTC product with no drug interactions section.</text>
    </section>
  </component>
</document>`

	sections := ParseInteractions([]byte(xml))
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for SPL with no Section 7, got %d", len(sections))
	}
}

func TestParseInteractions_EmptyXML(t *testing.T) {
	sections := ParseInteractions([]byte(""))
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for empty XML, got %d", len(sections))
	}
}

func TestParseInteractions_OnlyMainSection(t *testing.T) {
	xml := `<document>
  <section>
    <title>7 DRUG INTERACTIONS</title>
    <text>Brief interaction summary only.</text>
  </section>
</document>`

	sections := ParseInteractions([]byte(xml))
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0].Title != "7 DRUG INTERACTIONS" {
		t.Errorf("title = %q, want %q", sections[0].Title, "7 DRUG INTERACTIONS")
	}
	if sections[0].Text != "Brief interaction summary only." {
		t.Errorf("text = %q, want %q", sections[0].Text, "Brief interaction summary only.")
	}
}

func TestParseInteractions_OldFormatBareTitle(t *testing.T) {
	// Older SPLs use "Drug Interactions" without section number (under PRECAUTIONS)
	xml := `<document>
  <section>
    <title>PRECAUTIONS</title>
    <text>General precautions text.</text>
  </section>
  <section>
    <title>Drug Interactions</title>
    <text>
      Lisinopril may interact with potassium-sparing diuretics,
      lithium, and non-steroidal anti-inflammatory agents.
    </text>
  </section>
</document>`

	sections := ParseInteractions([]byte(xml))
	if len(sections) != 1 {
		t.Fatalf("expected 1 section for old-format SPL, got %d", len(sections))
	}
	if sections[0].Title != "Drug Interactions" {
		t.Errorf("title = %q, want %q", sections[0].Title, "Drug Interactions")
	}
	if !strings.Contains(sections[0].Text, "lithium") {
		t.Error("expected text to contain 'lithium'")
	}
}

func TestParseInteractions_OTCNoInteractions(t *testing.T) {
	// OTC products like Tylenol typically have no drug interactions section
	xml := `<document>
  <section>
    <title>ACTIVE INGREDIENT</title>
    <text>Acetaminophen 500 mg</text>
  </section>
  <section>
    <title>WARNINGS</title>
    <text>Liver warning: This product contains acetaminophen.</text>
  </section>
  <section>
    <title>DIRECTIONS</title>
    <text>Adults: take 2 caplets every 6 hours.</text>
  </section>
</document>`

	sections := ParseInteractions([]byte(xml))
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for OTC product, got %d", len(sections))
	}
}

func TestParseInteractions_TextBlockWithoutClosingTag(t *testing.T) {
	// Malformed XML — <text> without </text>
	xml := `<document>
  <section>
    <title>7 DRUG INTERACTIONS</title>
    <text>This text block never closes
  </section>
</document>`

	sections := ParseInteractions([]byte(xml))
	// Should handle gracefully — skip the section
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for malformed text block, got %d", len(sections))
	}
}

func TestParseInteractions_MixedNumberedAndBare(t *testing.T) {
	// Edge case: both formats in same document (shouldn't happen but be safe)
	xml := `<document>
  <section>
    <title>7 DRUG INTERACTIONS</title>
    <text>Numbered section summary.</text>
  </section>
  <section>
    <title>Drug Interactions</title>
    <text>Bare section content.</text>
  </section>
</document>`

	sections := ParseInteractions([]byte(xml))
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
}

func TestCleanXMLText(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "strips tags",
			raw:  `<content styleCode="bold">important</content> text`,
			want: "important text",
		},
		{
			name: "collapses whitespace",
			raw:  "line one\n\n   line two\t\tline three",
			want: "line one line two line three",
		},
		{
			name: "strips nested tags",
			raw:  `<paragraph><content styleCode="italics">see section <linkHtml href="#x">7</linkHtml></content></paragraph>`,
			want: "see section 7",
		},
		{
			name: "empty input",
			raw:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanXMLText(tt.raw)
			if got != tt.want {
				t.Errorf("cleanXMLText(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

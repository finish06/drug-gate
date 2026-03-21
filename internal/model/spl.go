package model

// SPLEntry represents SPL metadata from DailyMed (list endpoint response item).
type SPLEntry struct {
	Title         string `json:"title"`
	SetID         string `json:"setid"`
	PublishedDate string `json:"published_date"`
	SPLVersion    int    `json:"spl_version"`
}

// InteractionSection represents a parsed subsection from an SPL section.
// Reused for sections 4 (Contraindications), 5 (Warnings), 6 (Adverse Reactions), and 7 (Interactions).
type InteractionSection struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// SPLDetail is the detail endpoint response with parsed clinical sections.
type SPLDetail struct {
	Title              string               `json:"title"`
	SetID              string               `json:"setid"`
	PublishedDate      string               `json:"published_date"`
	SPLVersion         int                  `json:"spl_version"`
	Interactions       []InteractionSection `json:"interactions"`
	Contraindications  []InteractionSection `json:"contraindications"`
	Warnings           []InteractionSection `json:"warnings"`
	AdverseReactions   []InteractionSection `json:"adverse_reactions"`
}

// DrugInfoResponse is the response for the drug info card endpoint.
type DrugInfoResponse struct {
	DrugName          string               `json:"drug_name"`
	InputType         string               `json:"input_type"`
	InputValue        string               `json:"input_value"`
	SPL               *SPLSource           `json:"spl"`
	Interactions      []InteractionSection `json:"interactions"`
	Contraindications []InteractionSection `json:"contraindications"`
	Warnings          []InteractionSection `json:"warnings"`
	AdverseReactions  []InteractionSection `json:"adverse_reactions"`
}

// SPLSource identifies which SPL provided the interaction data.
type SPLSource struct {
	Title         string `json:"title"`
	SetID         string `json:"setid"`
	PublishedDate string `json:"published_date"`
	SPLVersion    int    `json:"spl_version"`
}

// InteractionCheckRequest is the request body for the interaction checker.
type InteractionCheckRequest struct {
	Drugs []DrugIdentifier `json:"drugs"`
}

// DrugIdentifier identifies a drug by name or NDC.
type DrugIdentifier struct {
	Name string `json:"name,omitempty"`
	NDC  string `json:"ndc,omitempty"`
}

// InteractionCheckResponse is the response for the interaction checker.
type InteractionCheckResponse struct {
	Drugs             []DrugCheckResult  `json:"drugs"`
	Interactions      []InteractionMatch `json:"interactions"`
	CheckedPairs      int                `json:"checked_pairs"`
	FoundInteractions int                `json:"found_interactions"`
}

// DrugCheckResult represents one drug's resolution status in the interaction check.
type DrugCheckResult struct {
	InputName       string `json:"input_name"`
	InputType       string `json:"input_type"`
	ResolvedName    string `json:"resolved_name"`
	HasInteractions bool   `json:"has_interactions"`
	SPLSetID        string `json:"spl_setid,omitempty"`
	Error           string `json:"error,omitempty"`
}

// InteractionMatch represents a found interaction between two drugs.
type InteractionMatch struct {
	DrugA        string `json:"drug_a"`
	DrugB        string `json:"drug_b"`
	Source       string `json:"source"`
	SectionTitle string `json:"section_title"`
	Text         string `json:"text"`
	SPLSetID     string `json:"spl_setid"`
}

package model

// SPLEntry represents SPL metadata from DailyMed (list endpoint response item).
type SPLEntry struct {
	Title         string `json:"title" example:"WARFARIN SODIUM tablet"`
	SetID         string `json:"setid" example:"b76006e5-1005-4a64-97a1-7cc20c0e6a1a"`
	PublishedDate string `json:"published_date" example:"2024-03-15"`
	SPLVersion    int    `json:"spl_version" example:"12"`
}

// InteractionSection represents a parsed subsection from an SPL section.
// Reused for sections 4 (Contraindications), 5 (Warnings), 6 (Adverse Reactions), and 7 (Interactions).
type InteractionSection struct {
	Title string `json:"title" example:"Drug Interactions"`
	Text  string `json:"text" example:"Concomitant use of warfarin and aspirin increases the risk of bleeding."`
}

// SPLDetail is the detail endpoint response with parsed clinical sections.
type SPLDetail struct {
	Title             string               `json:"title" example:"WARFARIN SODIUM tablet"`
	SetID             string               `json:"setid" example:"b76006e5-1005-4a64-97a1-7cc20c0e6a1a"`
	PublishedDate     string               `json:"published_date" example:"2024-03-15"`
	SPLVersion        int                  `json:"spl_version" example:"12"`
	Interactions      []InteractionSection `json:"interactions"`
	Contraindications []InteractionSection `json:"contraindications"`
	Warnings          []InteractionSection `json:"warnings"`
	AdverseReactions  []InteractionSection `json:"adverse_reactions"`
}

// DrugInfoResponse is the response for the drug info card endpoint.
type DrugInfoResponse struct {
	DrugName          string               `json:"drug_name" example:"warfarin"`
	InputType         string               `json:"input_type" example:"name"`
	InputValue        string               `json:"input_value" example:"warfarin"`
	SPL               *SPLSource           `json:"spl"`
	Interactions      []InteractionSection `json:"interactions"`
	Contraindications []InteractionSection `json:"contraindications"`
	Warnings          []InteractionSection `json:"warnings"`
	AdverseReactions  []InteractionSection `json:"adverse_reactions"`
}

// SPLSource identifies which SPL provided the interaction data.
type SPLSource struct {
	Title         string `json:"title" example:"WARFARIN SODIUM tablet"`
	SetID         string `json:"setid" example:"b76006e5-1005-4a64-97a1-7cc20c0e6a1a"`
	PublishedDate string `json:"published_date" example:"2024-03-15"`
	SPLVersion    int    `json:"spl_version" example:"12"`
}

// InteractionCheckRequest is the request body for the interaction checker.
type InteractionCheckRequest struct {
	Drugs []DrugIdentifier `json:"drugs"`
}

// DrugIdentifier identifies a drug by name or NDC.
type DrugIdentifier struct {
	Name string `json:"name,omitempty" example:"warfarin"`
	NDC  string `json:"ndc,omitempty" example:"00069-3150"`
}

// InteractionCheckResponse is the response for the interaction checker.
type InteractionCheckResponse struct {
	Drugs             []DrugCheckResult  `json:"drugs"`
	Interactions      []InteractionMatch `json:"interactions"`
	CheckedPairs      int                `json:"checked_pairs" example:"3"`
	FoundInteractions int                `json:"found_interactions" example:"2"`
}

// DrugCheckResult represents one drug's resolution status in the interaction check.
type DrugCheckResult struct {
	InputName       string `json:"input_name" example:"warfarin"`
	InputType       string `json:"input_type" example:"name"`
	ResolvedName    string `json:"resolved_name" example:"WARFARIN SODIUM"`
	HasInteractions bool   `json:"has_interactions" example:"true"`
	SPLSetID        string `json:"spl_setid,omitempty" example:"b76006e5-1005-4a64-97a1-7cc20c0e6a1a"`
	Error           string `json:"error,omitempty"`
}

// InteractionMatch represents a found interaction between two drugs.
type InteractionMatch struct {
	DrugA        string `json:"drug_a" example:"warfarin"`
	DrugB        string `json:"drug_b" example:"aspirin"`
	Source       string `json:"source" example:"warfarin"`
	SectionTitle string `json:"section_title" example:"Drug Interactions"`
	Text         string `json:"text" example:"Aspirin can increase the anticoagulant effect of warfarin."`
	SPLSetID     string `json:"spl_setid" example:"b76006e5-1005-4a64-97a1-7cc20c0e6a1a"`
}

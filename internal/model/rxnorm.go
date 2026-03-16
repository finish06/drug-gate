package model

// RxNormCandidate represents a single approximate match result.
type RxNormCandidate struct {
	RxCUI string `json:"rxcui"`
	Name  string `json:"name"`
	Score int    `json:"score"`
}

// RxNormSearchResult is the response for drug name search.
type RxNormSearchResult struct {
	Query       string            `json:"query"`
	Candidates  []RxNormCandidate `json:"candidates"`
	Suggestions []string          `json:"suggestions"`
}

// RxNormConcept represents a single RxNorm concept (used in generics, related).
type RxNormConcept struct {
	RxCUI string `json:"rxcui"`
	Name  string `json:"name"`
}

// RxNormNDCResponse is the response for NDCs by RxCUI.
type RxNormNDCResponse struct {
	RxCUI string   `json:"rxcui"`
	NDCs  []string `json:"ndcs"`
}

// RxNormGenericResponse is the response for generic products by RxCUI.
type RxNormGenericResponse struct {
	RxCUI    string          `json:"rxcui"`
	Generics []RxNormConcept `json:"generics"`
}

// RxNormRelatedResponse is the response for related concepts grouped by type.
type RxNormRelatedResponse struct {
	RxCUI         string          `json:"rxcui"`
	Ingredients   []RxNormConcept `json:"ingredients"`
	BrandNames    []RxNormConcept `json:"brand_names"`
	DoseForms     []RxNormConcept `json:"dose_forms"`
	ClinicalDrugs []RxNormConcept `json:"clinical_drugs"`
	BrandedDrugs  []RxNormConcept `json:"branded_drugs"`
}

// RxNormProfile is the unified drug profile response.
type RxNormProfile struct {
	Query      string                 `json:"query"`
	RxCUI      string                 `json:"rxcui"`
	Name       string                 `json:"name"`
	BrandNames []string               `json:"brand_names"`
	Generic    *RxNormConcept         `json:"generic"`
	NDCs       []string               `json:"ndcs"`
	Related    *RxNormRelatedResponse `json:"related"`
}

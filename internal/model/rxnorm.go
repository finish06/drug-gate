package model

// RxNormCandidate represents a single approximate match result.
type RxNormCandidate struct {
	RxCUI string `json:"rxcui" example:"153165"`
	Name  string `json:"name" example:"atorvastatin"`
	Score int    `json:"score" example:"94"`
}

// RxNormSearchResult is the response for drug name search.
type RxNormSearchResult struct {
	Query       string            `json:"query" example:"lipitor"`
	Candidates  []RxNormCandidate `json:"candidates"`
	Suggestions []string          `json:"suggestions" example:"lipitor"`
}

// RxNormConcept represents a single RxNorm concept (used in generics, related).
type RxNormConcept struct {
	RxCUI string `json:"rxcui" example:"314076"`
	Name  string `json:"name" example:"atorvastatin calcium 10 MG Oral Tablet"`
}

// RxNormNDCResponse is the response for NDCs by RxCUI.
type RxNormNDCResponse struct {
	RxCUI string   `json:"rxcui" example:"314076"`
	NDCs  []string `json:"ndcs" example:"00069-3150-30"`
}

// RxNormGenericResponse is the response for generic products by RxCUI.
type RxNormGenericResponse struct {
	RxCUI    string          `json:"rxcui" example:"314076"`
	Generics []RxNormConcept `json:"generics"`
}

// RxNormRelatedResponse is the response for related concepts grouped by type.
type RxNormRelatedResponse struct {
	RxCUI         string          `json:"rxcui" example:"314076"`
	Ingredients   []RxNormConcept `json:"ingredients"`
	BrandNames    []RxNormConcept `json:"brand_names"`
	DoseForms     []RxNormConcept `json:"dose_forms"`
	ClinicalDrugs []RxNormConcept `json:"clinical_drugs"`
	BrandedDrugs  []RxNormConcept `json:"branded_drugs"`
}

// RxNormProfile is the unified drug profile response.
type RxNormProfile struct {
	Query      string                 `json:"query" example:"metformin"`
	RxCUI      string                 `json:"rxcui" example:"6809"`
	Name       string                 `json:"name" example:"metformin"`
	BrandNames []string               `json:"brand_names" example:"Glucophage"`
	Generic    *RxNormConcept         `json:"generic"`
	NDCs       []string               `json:"ndcs" example:"00087-6060-05"`
	Related    *RxNormRelatedResponse `json:"related"`
}

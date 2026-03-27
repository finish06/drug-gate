package model

// DrugDetailResponse is the frontend-facing drug information response.
type DrugDetailResponse struct {
	NDC         string   `json:"ndc" example:"00069-3150"`
	Name        string   `json:"name" example:"Lipitor"`
	GenericName string   `json:"generic_name" example:"ATORVASTATIN CALCIUM"`
	Classes     []string `json:"classes" example:"HMG-CoA Reductase Inhibitor"`
}

// ErrorResponse is the standard error response shape.
type ErrorResponse struct {
	Error   string `json:"error" example:"not_found"`
	Message string `json:"message" example:"No drug found for NDC 99999-9999"`
}

// DrugClassResponse is the response for drug class lookup by name.
type DrugClassResponse struct {
	QueryName   string      `json:"query_name" example:"simvastatin"`
	GenericName string      `json:"generic_name" example:"SIMVASTATIN"`
	BrandNames  []string    `json:"brand_names" example:"Zocor"`
	Classes     []DrugClass `json:"classes"`
}

// DrugClass represents a parsed pharmacological class.
type DrugClass struct {
	Name string `json:"name" example:"HMG-CoA Reductase Inhibitor"`
	Type string `json:"type" example:"EPC"`
}

// DrugNameEntry represents a single drug name in the listing.
type DrugNameEntry struct {
	Name      string `json:"name" example:"Lipitor"`
	Type      string `json:"type" example:"brand"`
	NameLower string `json:"-"` // pre-lowercased for search filtering, not serialized
}

// DrugClassEntry represents a single drug class in the listing.
type DrugClassEntry struct {
	Name string `json:"name" example:"HMG-CoA Reductase Inhibitor"`
	Type string `json:"type" example:"epc"`
}

// DrugInClassEntry represents a drug belonging to a specific class.
type DrugInClassEntry struct {
	GenericName string `json:"generic_name" example:"ATORVASTATIN CALCIUM"`
	BrandName   string `json:"brand_name" example:"Lipitor"`
}

// PaginatedResponse wraps any data list with pagination metadata.
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

// Pagination holds pagination metadata for list responses.
type Pagination struct {
	Page       int `json:"page" example:"1"`
	Limit      int `json:"limit" example:"50"`
	Total      int `json:"total" example:"1234"`
	TotalPages int `json:"total_pages" example:"25"`
}

package model

// DrugDetailResponse is the frontend-facing drug information response.
type DrugDetailResponse struct {
	NDC         string   `json:"ndc"`
	Name        string   `json:"name"`
	GenericName string   `json:"generic_name"`
	Classes     []string `json:"classes"`
}

// ErrorResponse is the standard error response shape.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// DrugClassResponse is the response for drug class lookup by name.
type DrugClassResponse struct {
	QueryName   string      `json:"query_name"`
	GenericName string      `json:"generic_name"`
	BrandNames  []string    `json:"brand_names"`
	Classes     []DrugClass `json:"classes"`
}

// DrugClass represents a parsed pharmacological class.
type DrugClass struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// DrugNameEntry represents a single drug name in the listing.
type DrugNameEntry struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	NameLower string `json:"-"` // pre-lowercased for search filtering, not serialized
}

// DrugClassEntry represents a single drug class in the listing.
type DrugClassEntry struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// DrugInClassEntry represents a drug belonging to a specific class.
type DrugInClassEntry struct {
	GenericName string `json:"generic_name"`
	BrandName   string `json:"brand_name"`
}

// PaginatedResponse wraps any data list with pagination metadata.
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

// Pagination holds pagination metadata for list responses.
type Pagination struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

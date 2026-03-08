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

package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	_ "github.com/finish06/drug-gate/docs"
	"github.com/go-chi/chi/v5"
)

func newSwaggerTestRouter() *chi.Mux {
	r := chi.NewRouter()
	RegisterSwaggerRoutes(r)
	return r
}

// AC-001: GET /openapi.json returns valid Swagger/OpenAPI spec
func TestSwagger_AC001_OpenAPIJSON(t *testing.T) {
	router := newSwaggerTestRouter()
	rr := doRequest(router, "/openapi.json")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", rr.Code, rr.Body.String())
	}

	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var spec map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &spec); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	// Should have swagger or openapi version field
	if _, ok := spec["swagger"]; !ok {
		if _, ok2 := spec["openapi"]; !ok2 {
			t.Error("spec missing 'swagger' or 'openapi' version field")
		}
	}
}

// AC-002: GET /swagger/ serves Swagger UI
func TestSwagger_AC002_SwaggerUI(t *testing.T) {
	router := newSwaggerTestRouter()
	rr := doRequest(router, "/swagger/index.html")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "swagger") && !strings.Contains(body, "Swagger") {
		t.Error("swagger UI page does not contain 'swagger'")
	}
}

// AC-004: NDC lookup endpoint documented in spec
func TestSwagger_AC004_NDCEndpointDocumented(t *testing.T) {
	router := newSwaggerTestRouter()
	rr := doRequest(router, "/openapi.json")

	var spec map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &spec); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("spec missing 'paths' object")
	}

	if _, ok := paths["/v1/drugs/ndc/{ndc}"]; !ok {
		t.Error("spec missing path '/v1/drugs/ndc/{ndc}'")
	}
}

// AC-005: Health endpoint documented in spec
func TestSwagger_AC005_HealthDocumented(t *testing.T) {
	router := newSwaggerTestRouter()
	rr := doRequest(router, "/openapi.json")

	var spec map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &spec); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("spec missing 'paths' object")
	}

	if _, ok := paths["/health"]; !ok {
		t.Error("spec missing path '/health'")
	}
}

// AC-007: Service metadata in spec
func TestSwagger_AC007_ServiceMetadata(t *testing.T) {
	router := newSwaggerTestRouter()
	rr := doRequest(router, "/openapi.json")

	var spec map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &spec); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	info, ok := spec["info"].(map[string]any)
	if !ok {
		t.Fatal("spec missing 'info' object")
	}

	title, _ := info["title"].(string)
	if title == "" {
		t.Error("spec info.title is empty")
	}
	if !strings.Contains(strings.ToLower(title), "drug-gate") {
		t.Errorf("info.title = %q, want it to contain 'drug-gate'", title)
	}

	version, _ := info["version"].(string)
	if version == "" {
		t.Error("spec info.version is empty")
	}

	description, _ := info["description"].(string)
	if description == "" {
		t.Error("spec info.description is empty")
	}
}

// AC-006: Response models defined in spec
func TestSwagger_AC006_ResponseModels(t *testing.T) {
	router := newSwaggerTestRouter()
	rr := doRequest(router, "/openapi.json")

	var spec map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &spec); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	definitions, ok := spec["definitions"].(map[string]any)
	if !ok {
		t.Fatal("spec missing 'definitions' object")
	}

	// swaggo uses full module path as prefix
	for _, model := range []string{"DrugDetailResponse", "ErrorResponse"} {
		found := false
		for key := range definitions {
			if strings.HasSuffix(key, model) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("spec missing definition ending with %q", model)
		}
	}
}

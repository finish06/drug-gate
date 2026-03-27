package handler

import (
	"net/http"

	"github.com/finish06/drug-gate/docs"
	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"
)

// OpenAPIJSON serves the generated Swagger/OpenAPI spec as JSON.
//
// @Summary      OpenAPI spec
// @Description  Returns the generated OpenAPI 2.0 (Swagger) specification as JSON. This is the machine-readable API contract used by Swagger UI and client code generators. The spec is embedded at build time via swaggo.
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /openapi.json [get]
func OpenAPIJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(docs.SwaggerInfo.ReadDoc()))
}

// RegisterSwaggerRoutes adds /swagger/* and /openapi.json to the router.
func RegisterSwaggerRoutes(r chi.Router) {
	r.Get("/swagger/*", httpSwagger.WrapHandler)
	r.Get("/openapi.json", OpenAPIJSON)
}

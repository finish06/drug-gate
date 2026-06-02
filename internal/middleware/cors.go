package middleware

import (
	"net/http"

	"github.com/finish06/drug-gate/internal/apikey"
)

// CORSPreflight answers CORS preflight (OPTIONS) requests before authentication.
//
// Browsers strip custom headers — including X-API-Key — from the preflight, so a
// preflight can never be authenticated. It must therefore be handled ahead of
// APIKeyAuth, or auth rejects it with 401 and the browser blocks the real request.
//
// The preflight only signals that the browser may send the actual request; it
// carries no data. Per-key origin enforcement happens on the actual request (see
// PerKeyCORS), which does carry the key. So the preflight reflects the requesting
// origin permissively — a disallowed origin still gets no Access-Control-Allow-Origin
// on the real response and is blocked from reading it.
func CORSPreflight(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// A CORS preflight is an OPTIONS request carrying both Origin and
		// Access-Control-Request-Method. Anything else is not a preflight.
		if r.Method == http.MethodOptions && origin != "" && r.Header.Get("Access-Control-Request-Method") != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.Header().Add("Vary", "Origin")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// PerKeyCORS sets CORS headers on actual (non-preflight) requests based on the
// authenticated API key's allowed origins. Preflight requests are handled earlier
// by CORSPreflight, before auth, so they never reach this middleware.
func PerKeyCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ak, ok := r.Context().Value(APIKeyContextKey).(*apikey.APIKey)
		if !ok || ak == nil {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		allowedOrigin := resolveOrigin(ak, origin)
		if allowedOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		}

		next.ServeHTTP(w, r)
	})
}

// resolveOrigin returns the allowed origin string, or empty if not allowed.
// Keys with no origins configured deny all cross-origin requests.
// To allow all origins, include explicit "*" in the origins list.
func resolveOrigin(ak *apikey.APIKey, origin string) string {
	// No origins configured — deny cross-origin requests
	if len(ak.Origins) == 0 {
		return ""
	}

	// Check if origin is in the allowed list
	for _, o := range ak.Origins {
		if o == "*" {
			return "*"
		}
		if o == origin {
			return origin
		}
	}

	return ""
}

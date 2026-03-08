package middleware

import (
	"net/http"

	"github.com/finish06/drug-gate/internal/apikey"
)

// PerKeyCORS sets CORS headers based on the authenticated API key's allowed origins.
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
		}

		// Handle preflight
		if r.Method == http.MethodOptions {
			if allowedOrigin != "" {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key, Authorization")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// resolveOrigin returns the allowed origin string, or empty if not allowed.
func resolveOrigin(ak *apikey.APIKey, origin string) string {
	// Origin-free key — allow all
	if len(ak.Origins) == 0 {
		return "*"
	}

	// Check if origin is in the allowed list
	for _, o := range ak.Origins {
		if o == origin {
			return origin
		}
	}

	return ""
}

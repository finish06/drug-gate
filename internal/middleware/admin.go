package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/finish06/drug-gate/internal/model"
)

// AdminAuth returns middleware that validates the Authorization header
// against a static admin secret (Bearer token).
func AdminAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Reject all requests if no admin secret is configured
			if secret == "" {
				writeAdminError(w, "Admin endpoints are disabled (ADMIN_SECRET not configured)")
				return
			}

			auth := r.Header.Get("Authorization")
			if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
				writeAdminError(w, "Admin authorization required")
				return
			}

			token := strings.TrimPrefix(auth, "Bearer ")
			if token != secret {
				writeAdminError(w, "Invalid admin secret")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeAdminError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(model.ErrorResponse{
		Error:   "unauthorized",
		Message: message,
	})
}

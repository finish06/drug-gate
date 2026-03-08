package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/finish06/drug-gate/internal/apikey"
	"github.com/finish06/drug-gate/internal/model"
)

type contextKey string

// APIKeyContextKey is the context key for the authenticated API key.
const APIKeyContextKey contextKey = "apikey"

// APIKeyAuth returns middleware that validates the X-API-Key header against the store.
func APIKeyAuth(store apikey.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			keyStr := r.Header.Get("X-API-Key")
			if keyStr == "" {
				writeAuthError(w, "API key required")
				return
			}

			ak, err := store.Get(r.Context(), keyStr)
			if err != nil || ak == nil {
				writeAuthError(w, "Invalid API key")
				return
			}

			if !ak.Active {
				writeAuthError(w, "API key is inactive")
				return
			}

			// Check expiration (grace period)
			if ak.ExpiresAt != nil && ak.ExpiresAt.Before(time.Now().UTC()) {
				writeAuthError(w, "API key has expired")
				return
			}

			ctx := context.WithValue(r.Context(), APIKeyContextKey, ak)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(model.ErrorResponse{
		Error:   "unauthorized",
		Message: message,
	})
}

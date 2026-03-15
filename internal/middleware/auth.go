package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/finish06/drug-gate/internal/apikey"
	"github.com/finish06/drug-gate/internal/metrics"
	"github.com/finish06/drug-gate/internal/model"
)

type contextKey string

// APIKeyContextKey is the context key for the authenticated API key.
const APIKeyContextKey contextKey = "apikey"

// APIKeyAuth returns middleware that validates the X-API-Key header against the store.
// Pass nil for m to disable metrics recording.
func APIKeyAuth(store apikey.Store, m ...*metrics.Metrics) func(http.Handler) http.Handler {
	var met *metrics.Metrics
	if len(m) > 0 {
		met = m[0]
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			keyStr := r.Header.Get("X-API-Key")
			if keyStr == "" {
				if met != nil {
					met.AuthRejectionsTotal.WithLabelValues("missing").Inc()
				}
				writeAuthError(w, "API key required")
				return
			}

			ak, err := store.Get(r.Context(), keyStr)
			if err != nil || ak == nil {
				if met != nil {
					met.AuthRejectionsTotal.WithLabelValues("invalid").Inc()
				}
				writeAuthError(w, "Invalid API key")
				return
			}

			if !ak.Active {
				if met != nil {
					met.AuthRejectionsTotal.WithLabelValues("inactive").Inc()
				}
				writeAuthError(w, "API key is inactive")
				return
			}

			// Check expiration (grace period)
			if ak.ExpiresAt != nil && ak.ExpiresAt.Before(time.Now().UTC()) {
				if met != nil {
					met.AuthRejectionsTotal.WithLabelValues("invalid").Inc()
				}
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

package middleware

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/finish06/drug-gate/internal/apikey"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/finish06/drug-gate/internal/ratelimit"
)

// RateLimit returns middleware that enforces per-key rate limiting.
// It reads the APIKey from context (set by APIKeyAuth) and calls the Limiter.
func RateLimit(limiter ratelimit.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ak, ok := r.Context().Value(APIKeyContextKey).(*apikey.APIKey)
			if !ok || ak == nil {
				next.ServeHTTP(w, r)
				return
			}

			result, err := limiter.Allow(r.Context(), ak.Key, ak.RateLimit)
			if err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			// Always set rate limit headers.
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

			if !result.Allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(model.ErrorResponse{
					Error:   "rate_limited",
					Message: "Rate limit exceeded",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

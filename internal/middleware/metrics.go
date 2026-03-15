package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/finish06/drug-gate/internal/metrics"
	"github.com/go-chi/chi/v5"
)

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (sw *statusWriter) WriteHeader(code int) {
	if !sw.written {
		sw.statusCode = code
		sw.written = true
	}
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if !sw.written {
		sw.written = true
	}
	return sw.ResponseWriter.Write(b)
}

// MetricsMiddleware returns Chi middleware that instruments HTTP requests
// with Prometheus counters and histograms.
func MetricsMiddleware(m *metrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &statusWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(ww, r)

			// After the handler runs, Chi's RouteContext is populated
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = r.URL.Path
			}

			duration := time.Since(start).Seconds()
			m.HTTPRequestsTotal.WithLabelValues(route, r.Method, strconv.Itoa(ww.statusCode)).Inc()
			m.HTTPRequestDuration.WithLabelValues(route, r.Method).Observe(duration)
		})
	}
}

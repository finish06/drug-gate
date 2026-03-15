package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/finish06/drug-gate/internal/metrics"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func newTestMetrics(t *testing.T) (*metrics.Metrics, *prometheus.Registry) {
	t.Helper()
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)
	return m, reg
}

func findMetricFamily(families []*dto.MetricFamily, name string) *dto.MetricFamily {
	for _, mf := range families {
		if mf.GetName() == name {
			return mf
		}
	}
	return nil
}

func getLabelValue(metric *dto.Metric, labelName string) string {
	for _, lp := range metric.GetLabel() {
		if lp.GetName() == labelName {
			return lp.GetValue()
		}
	}
	return ""
}

func TestMetricsMiddleware_CounterIncrements(t *testing.T) {
	m, reg := newTestMetrics(t)

	r := chi.NewRouter()
	r.Use(MetricsMiddleware(m))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather: %v", err)
	}

	mf := findMetricFamily(families, "druggate_http_requests_total")
	if mf == nil {
		t.Fatal("druggate_http_requests_total not found")
	}

	if len(mf.GetMetric()) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(mf.GetMetric()))
	}

	metric := mf.GetMetric()[0]
	if got := metric.GetCounter().GetValue(); got != 1 {
		t.Errorf("expected counter value 1, got %v", got)
	}

	if got := getLabelValue(metric, "method"); got != "GET" {
		t.Errorf("expected method=GET, got %q", got)
	}

	if got := getLabelValue(metric, "status_code"); got != "200" {
		t.Errorf("expected status_code=200, got %q", got)
	}

	if got := getLabelValue(metric, "route"); got != "/test" {
		t.Errorf("expected route=/test, got %q", got)
	}
}

func TestMetricsMiddleware_HistogramRecordsDuration(t *testing.T) {
	m, reg := newTestMetrics(t)

	r := chi.NewRouter()
	r.Use(MetricsMiddleware(m))
	r.Get("/slow", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	_ = rr

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather: %v", err)
	}

	mf := findMetricFamily(families, "druggate_http_request_duration_seconds")
	if mf == nil {
		t.Fatal("druggate_http_request_duration_seconds not found")
	}

	metric := mf.GetMetric()[0]
	hist := metric.GetHistogram()
	if hist == nil {
		t.Fatal("expected histogram data")
	}

	if hist.GetSampleCount() != 1 {
		t.Errorf("expected 1 observation, got %d", hist.GetSampleCount())
	}

	if hist.GetSampleSum() <= 0 {
		t.Error("expected positive duration sum")
	}
}

func TestMetricsMiddleware_RoutePatternUsed(t *testing.T) {
	m, reg := newTestMetrics(t)

	r := chi.NewRouter()
	r.Use(MetricsMiddleware(m))
	r.Get("/v1/drugs/ndc/{ndc}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Request with actual NDC value — route label should be the pattern, not the value
	req := httptest.NewRequest(http.MethodGet, "/v1/drugs/ndc/00069-3150", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	_ = rr

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather: %v", err)
	}

	mf := findMetricFamily(families, "druggate_http_requests_total")
	if mf == nil {
		t.Fatal("druggate_http_requests_total not found")
	}

	metric := mf.GetMetric()[0]
	route := getLabelValue(metric, "route")
	if route != "/v1/drugs/ndc/{ndc}" {
		t.Errorf("expected route=/v1/drugs/ndc/{ndc}, got %q (should use pattern, not actual path)", route)
	}
}

func TestMetricsMiddleware_CapturesStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fresh registry per sub-test to avoid label collisions
			m, reg := newTestMetrics(t)

			r := chi.NewRouter()
			r.Use(MetricsMiddleware(m))
			r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			req := httptest.NewRequest(http.MethodGet, "/status", nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != tt.statusCode {
				t.Fatalf("handler returned %d, expected %d", rr.Code, tt.statusCode)
			}

			families, err := reg.Gather()
			if err != nil {
				t.Fatalf("failed to gather: %v", err)
			}

			mf := findMetricFamily(families, "druggate_http_requests_total")
			if mf == nil {
				t.Fatal("druggate_http_requests_total not found")
			}

			metric := mf.GetMetric()[0]
			got := getLabelValue(metric, "status_code")
			expected := strconv.Itoa(tt.statusCode)
			if got != expected {
				t.Errorf("expected status_code=%s, got %q", expected, got)
			}
		})
	}
}

func TestMetricsMiddleware_MultipleRequests(t *testing.T) {
	m, reg := newTestMetrics(t)

	r := chi.NewRouter()
	r.Use(MetricsMiddleware(m))
	r.Get("/multi", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/multi", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		_ = rr
	}

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather: %v", err)
	}

	mf := findMetricFamily(families, "druggate_http_requests_total")
	if mf == nil {
		t.Fatal("druggate_http_requests_total not found")
	}

	metric := mf.GetMetric()[0]
	if got := metric.GetCounter().GetValue(); got != 5 {
		t.Errorf("expected counter value 5 after 5 requests, got %v", got)
	}
}

package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestNewMetrics_RegistersWithoutPanic(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}
}

func TestNewMetrics_AllNamesHaveDruggatePrefix(t *testing.T) {
	reg := prometheus.NewRegistry()
	NewMetrics(reg)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	if len(families) == 0 {
		t.Fatal("no metric families registered")
	}

	for _, mf := range families {
		name := mf.GetName()
		if !strings.HasPrefix(name, "druggate_") {
			t.Errorf("metric %q does not have druggate_ prefix", name)
		}
	}
}

func TestNewMetrics_CounterVecsAcceptExpectedLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	// HTTPRequestsTotal accepts route, method, status_code
	m.HTTPRequestsTotal.WithLabelValues("/v1/drugs/names", "GET", "200").Inc()

	// CacheHitsTotal accepts key_type, outcome
	m.CacheHitsTotal.WithLabelValues("drugnames", "hit").Inc()

	// RateLimitRejectionsTotal accepts api_key
	m.RateLimitRejectionsTotal.WithLabelValues("pk_test123").Inc()

	// AuthRejectionsTotal accepts reason
	m.AuthRejectionsTotal.WithLabelValues("missing").Inc()
	m.AuthRejectionsTotal.WithLabelValues("invalid").Inc()
	m.AuthRejectionsTotal.WithLabelValues("inactive").Inc()

	// Verify the values were recorded
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := make(map[string]bool)
	for _, mf := range families {
		found[mf.GetName()] = true
	}

	expectedMetrics := []string{
		"druggate_http_requests_total",
		"druggate_cache_hits_total",
		"druggate_ratelimit_rejections_total",
		"druggate_auth_rejections_total",
	}
	for _, name := range expectedMetrics {
		if !found[name] {
			t.Errorf("expected metric %q not found in gathered families", name)
		}
	}
}

func TestNewMetrics_HistogramAcceptsExpectedLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.HTTPRequestDuration.WithLabelValues("/v1/drugs/names", "GET").Observe(0.042)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	for _, mf := range families {
		if mf.GetName() == "druggate_http_request_duration_seconds" {
			if mf.GetType() != dto.MetricType_HISTOGRAM {
				t.Errorf("expected HISTOGRAM type, got %v", mf.GetType())
			}
			return
		}
	}
	t.Error("druggate_http_request_duration_seconds not found in gathered families")
}

func TestNewMetrics_GaugesExist(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	// Set gauges to verify they work
	m.RedisUp.Set(1)
	m.RedisPingDuration.Set(0.001)
	m.ContainerCPUUsage.Set(12.5)
	m.ContainerCPUCores.Set(4)
	m.ContainerMemoryRSS.Set(1024 * 1024 * 100)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	expectedGauges := []string{
		"druggate_redis_up",
		"druggate_redis_ping_duration_seconds",
		"druggate_container_cpu_usage_seconds_total",
		"druggate_container_cpu_cores_available",
		"druggate_container_memory_rss_bytes",
	}

	found := make(map[string]bool)
	for _, mf := range families {
		found[mf.GetName()] = true
	}

	for _, name := range expectedGauges {
		if !found[name] {
			t.Errorf("expected gauge %q not found", name)
		}
	}
}

func TestNewMetrics_GaugeVecsAcceptInterfaceLabel(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.ContainerNetworkReceiveBytes.WithLabelValues("eth0").Set(1024)
	m.ContainerNetworkTransmitBytes.WithLabelValues("eth0").Set(2048)
	m.ContainerNetworkReceivePackets.WithLabelValues("lo").Set(100)
	m.ContainerNetworkTransmitPackets.WithLabelValues("lo").Set(200)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := make(map[string]bool)
	for _, mf := range families {
		found[mf.GetName()] = true
	}

	expectedVecs := []string{
		"druggate_container_network_receive_bytes_total",
		"druggate_container_network_transmit_bytes_total",
		"druggate_container_network_receive_packets_total",
		"druggate_container_network_transmit_packets_total",
	}
	for _, name := range expectedVecs {
		if !found[name] {
			t.Errorf("expected gauge vec %q not found", name)
		}
	}
}

func TestNewMetrics_RegisterTwicePanics(t *testing.T) {
	reg := prometheus.NewRegistry()
	NewMetrics(reg)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when registering metrics twice, but did not panic")
		}
	}()

	// Registering the same metrics again on the same registry should panic
	NewMetrics(reg)
}

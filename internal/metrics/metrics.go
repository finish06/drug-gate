package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "druggate"

// Metrics holds all Prometheus metric collectors for the service.
type Metrics struct {
	HTTPRequestsTotal        *prometheus.CounterVec
	HTTPRequestDuration      *prometheus.HistogramVec
	CacheHitsTotal           *prometheus.CounterVec
	RateLimitRejectionsTotal *prometheus.CounterVec
	AuthRejectionsTotal      *prometheus.CounterVec
	RedisUp                  prometheus.Gauge
	RedisPingDuration        prometheus.Gauge

	// Container system metrics
	ContainerCPUUsage               prometheus.Gauge
	ContainerCPUCores               prometheus.Gauge
	ContainerMemoryRSS              prometheus.Gauge
	ContainerMemoryVMS              prometheus.Gauge
	ContainerMemoryLimit            prometheus.Gauge
	ContainerMemoryUsageRatio       prometheus.Gauge
	ContainerDiskTotal              prometheus.Gauge
	ContainerDiskFree               prometheus.Gauge
	ContainerDiskUsed               prometheus.Gauge
	ContainerNetworkReceiveBytes    *prometheus.GaugeVec
	ContainerNetworkTransmitBytes   *prometheus.GaugeVec
	ContainerNetworkReceivePackets  *prometheus.GaugeVec
	ContainerNetworkTransmitPackets *prometheus.GaugeVec
}

// NewMetrics creates and registers all Prometheus metrics with the given registry.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "http_requests_total",
				Help:      "Total HTTP requests.",
			},
			[]string{"route", "method", "status_code"},
		),
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request latency in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"route", "method"},
		),
		CacheHitsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "cache_hits_total",
				Help:      "Redis cache outcomes by key type and result.",
			},
			[]string{"key_type", "outcome"},
		),
		RateLimitRejectionsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "ratelimit_rejections_total",
				Help:      "Rate limit 429 rejections by API key.",
			},
			[]string{"api_key"},
		),
		AuthRejectionsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "auth_rejections_total",
				Help:      "Auth failures by reason.",
			},
			[]string{"reason"},
		),
		RedisUp: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "redis_up",
				Help:      "Redis health status (1 = healthy, 0 = unhealthy).",
			},
		),
		RedisPingDuration: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "redis_ping_duration_seconds",
				Help:      "Last Redis ping latency in seconds.",
			},
		),

		// Container system metrics
		ContainerCPUUsage: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "container",
				Name:      "cpu_usage_seconds_total",
				Help:      "Total CPU time consumed by the container in seconds.",
			},
		),
		ContainerCPUCores: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "container",
				Name:      "cpu_cores_available",
				Help:      "Number of CPU cores available to the container.",
			},
		),
		ContainerMemoryRSS: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "container",
				Name:      "memory_rss_bytes",
				Help:      "Resident set size memory in bytes.",
			},
		),
		ContainerMemoryVMS: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "container",
				Name:      "memory_vms_bytes",
				Help:      "Virtual memory size in bytes.",
			},
		),
		ContainerMemoryLimit: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "container",
				Name:      "memory_limit_bytes",
				Help:      "Memory limit in bytes (-1 if unlimited).",
			},
		),
		ContainerMemoryUsageRatio: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "container",
				Name:      "memory_usage_ratio",
				Help:      "Memory usage ratio (RSS / limit).",
			},
		),
		ContainerDiskTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "container",
				Name:      "disk_total_bytes",
				Help:      "Total disk space in bytes.",
			},
		),
		ContainerDiskFree: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "container",
				Name:      "disk_free_bytes",
				Help:      "Free disk space in bytes.",
			},
		),
		ContainerDiskUsed: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "container",
				Name:      "disk_used_bytes",
				Help:      "Used disk space in bytes.",
			},
		),
		ContainerNetworkReceiveBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "container",
				Name:      "network_receive_bytes_total",
				Help:      "Total bytes received per network interface.",
			},
			[]string{"interface"},
		),
		ContainerNetworkTransmitBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "container",
				Name:      "network_transmit_bytes_total",
				Help:      "Total bytes transmitted per network interface.",
			},
			[]string{"interface"},
		),
		ContainerNetworkReceivePackets: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "container",
				Name:      "network_receive_packets_total",
				Help:      "Total packets received per network interface.",
			},
			[]string{"interface"},
		),
		ContainerNetworkTransmitPackets: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "container",
				Name:      "network_transmit_packets_total",
				Help:      "Total packets transmitted per network interface.",
			},
			[]string{"interface"},
		),
	}

	reg.MustRegister(
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.CacheHitsTotal,
		m.RateLimitRejectionsTotal,
		m.AuthRejectionsTotal,
		m.RedisUp,
		m.RedisPingDuration,
		m.ContainerCPUUsage,
		m.ContainerCPUCores,
		m.ContainerMemoryRSS,
		m.ContainerMemoryVMS,
		m.ContainerMemoryLimit,
		m.ContainerMemoryUsageRatio,
		m.ContainerDiskTotal,
		m.ContainerDiskFree,
		m.ContainerDiskUsed,
		m.ContainerNetworkReceiveBytes,
		m.ContainerNetworkTransmitBytes,
		m.ContainerNetworkReceivePackets,
		m.ContainerNetworkTransmitPackets,
	)

	return m
}

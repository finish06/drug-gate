package metrics_test

import (
	"strings"
	"testing"
	"time"

	"github.com/finish06/drug-gate/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// --- Mock SystemSource ---

type mockSystemSource struct {
	cpuUserSec float64
	cpuSysSec  float64
	cpuErr     error

	memInfo *metrics.MemInfo
	memErr  error

	diskInfo *metrics.DiskInfo
	diskErr  error

	netStats []metrics.NetStat
	netErr   error
}

func (m *mockSystemSource) CPUUsage() (float64, float64, error) {
	return m.cpuUserSec, m.cpuSysSec, m.cpuErr
}

func (m *mockSystemSource) MemoryInfo() (*metrics.MemInfo, error) {
	return m.memInfo, m.memErr
}

func (m *mockSystemSource) DiskUsage(path string) (*metrics.DiskInfo, error) {
	return m.diskInfo, m.diskErr
}

func (m *mockSystemSource) NetworkStats() ([]metrics.NetStat, error) {
	return m.netStats, m.netErr
}

// --- panicSystemSource for panic recovery testing ---

type panicSystemSource struct {
	mockSystemSource
	panicOnCPU bool
}

func (p *panicSystemSource) CPUUsage() (float64, float64, error) {
	if p.panicOnCPU {
		panic("intentional panic in CPUUsage")
	}
	return p.cpuUserSec, p.cpuSysSec, p.cpuErr
}

// --- Tests ---

// TestSystemCollector_CollectsOnStart verifies all gauges are updated immediately on Start.
func TestSystemCollector_CollectsOnStart(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	src := &mockSystemSource{
		cpuUserSec: 1.5,
		cpuSysSec:  0.5,
		memInfo: &metrics.MemInfo{
			RSS:            33554432,
			VMS:            268435456,
			Limit:          536870912,
			LimitAvailable: true,
		},
		diskInfo: &metrics.DiskInfo{
			Total: 107374182400,
			Free:  53687091200,
			Used:  53687091200,
		},
		netStats: []metrics.NetStat{
			{Interface: "eth0", RxBytes: 98765432, TxBytes: 45678901, RxPackets: 654321, TxPackets: 321098},
		},
	}

	collector := metrics.NewSystemCollector(src, m, 1*time.Hour, "/")
	collector.Start()
	time.Sleep(100 * time.Millisecond)
	collector.Stop()

	// Verify CPU
	cpuVal := testutil.ToFloat64(m.ContainerCPUUsage)
	if cpuVal != 2.0 {
		t.Errorf("expected CPU usage=2.0, got %f", cpuVal)
	}

	// Verify cores > 0
	cores := testutil.ToFloat64(m.ContainerCPUCores)
	if cores <= 0 {
		t.Errorf("expected CPUCores > 0, got %f", cores)
	}

	// Verify memory
	if v := testutil.ToFloat64(m.ContainerMemoryRSS); v != 33554432 {
		t.Errorf("expected MemoryRSS=33554432, got %f", v)
	}
	if v := testutil.ToFloat64(m.ContainerMemoryVMS); v != 268435456 {
		t.Errorf("expected MemoryVMS=268435456, got %f", v)
	}

	// Verify disk
	if v := testutil.ToFloat64(m.ContainerDiskTotal); v != 107374182400 {
		t.Errorf("expected DiskTotal=107374182400, got %f", v)
	}

	// Verify network
	rxBytes := testutil.ToFloat64(m.ContainerNetworkReceiveBytes.WithLabelValues("eth0"))
	if rxBytes != 98765432 {
		t.Errorf("expected eth0 RxBytes=98765432, got %f", rxBytes)
	}
}

// TestSystemCollector_CPUMetrics verifies CPU usage and cores are set correctly.
func TestSystemCollector_CPUMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	src := &mockSystemSource{
		cpuUserSec: 3.25,
		cpuSysSec:  1.75,
		memInfo:    &metrics.MemInfo{RSS: 1000, VMS: 2000, Limit: -1},
		diskInfo:   &metrics.DiskInfo{Total: 100, Free: 50, Used: 50},
	}

	collector := metrics.NewSystemCollector(src, m, 1*time.Hour, "/")
	collector.Start()
	time.Sleep(100 * time.Millisecond)
	collector.Stop()

	cpuVal := testutil.ToFloat64(m.ContainerCPUUsage)
	if cpuVal != 5.0 { // 3.25 + 1.75
		t.Errorf("expected CPU usage=5.0, got %f", cpuVal)
	}

	cores := testutil.ToFloat64(m.ContainerCPUCores)
	if cores <= 0 {
		t.Errorf("expected CPUCores > 0, got %f", cores)
	}
}

// TestSystemCollector_MemoryMetrics verifies RSS, VMS, limit, and ratio are set.
func TestSystemCollector_MemoryMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	src := &mockSystemSource{
		memInfo: &metrics.MemInfo{
			RSS:            33554432,
			VMS:            268435456,
			Limit:          536870912,
			LimitAvailable: true,
		},
		diskInfo: &metrics.DiskInfo{Total: 100, Free: 50, Used: 50},
	}

	collector := metrics.NewSystemCollector(src, m, 1*time.Hour, "/")
	collector.Start()
	time.Sleep(100 * time.Millisecond)
	collector.Stop()

	if v := testutil.ToFloat64(m.ContainerMemoryRSS); v != 33554432 {
		t.Errorf("expected MemoryRSS=33554432, got %f", v)
	}
	if v := testutil.ToFloat64(m.ContainerMemoryVMS); v != 268435456 {
		t.Errorf("expected MemoryVMS=268435456, got %f", v)
	}
	if v := testutil.ToFloat64(m.ContainerMemoryLimit); v != 536870912 {
		t.Errorf("expected MemoryLimit=536870912, got %f", v)
	}

	expectedRatio := float64(33554432) / float64(536870912) // ~0.0625
	ratioVal := testutil.ToFloat64(m.ContainerMemoryUsageRatio)
	if ratioVal < expectedRatio-0.001 || ratioVal > expectedRatio+0.001 {
		t.Errorf("expected MemoryUsageRatio~%f, got %f", expectedRatio, ratioVal)
	}
}

// TestSystemCollector_MemoryNoLimit verifies ratio is not set when limit is -1.
func TestSystemCollector_MemoryNoLimit(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	src := &mockSystemSource{
		memInfo: &metrics.MemInfo{
			RSS:            33554432,
			VMS:            268435456,
			Limit:          -1,
			LimitAvailable: true,
		},
		diskInfo: &metrics.DiskInfo{Total: 100, Free: 50, Used: 50},
	}

	collector := metrics.NewSystemCollector(src, m, 1*time.Hour, "/")
	collector.Start()
	time.Sleep(100 * time.Millisecond)
	collector.Stop()

	ratioVal := testutil.ToFloat64(m.ContainerMemoryUsageRatio)
	if ratioVal != 0 {
		t.Errorf("expected MemoryUsageRatio=0 when limit=-1, got %f", ratioVal)
	}
}

// TestSystemCollector_DiskMetrics verifies total, free, and used disk values.
func TestSystemCollector_DiskMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	src := &mockSystemSource{
		memInfo: &metrics.MemInfo{RSS: 1000, VMS: 2000, Limit: -1},
		diskInfo: &metrics.DiskInfo{
			Total: 107374182400,
			Free:  53687091200,
			Used:  53687091200,
		},
	}

	collector := metrics.NewSystemCollector(src, m, 1*time.Hour, "/")
	collector.Start()
	time.Sleep(100 * time.Millisecond)
	collector.Stop()

	if v := testutil.ToFloat64(m.ContainerDiskTotal); v != 107374182400 {
		t.Errorf("expected DiskTotal=107374182400, got %f", v)
	}
	if v := testutil.ToFloat64(m.ContainerDiskFree); v != 53687091200 {
		t.Errorf("expected DiskFree=53687091200, got %f", v)
	}
	if v := testutil.ToFloat64(m.ContainerDiskUsed); v != 53687091200 {
		t.Errorf("expected DiskUsed=53687091200, got %f", v)
	}
}

// TestSystemCollector_NetworkMetrics verifies per-interface bytes and packets.
func TestSystemCollector_NetworkMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	src := &mockSystemSource{
		memInfo:  &metrics.MemInfo{RSS: 1000, VMS: 2000, Limit: -1},
		diskInfo: &metrics.DiskInfo{Total: 100, Free: 50, Used: 50},
		netStats: []metrics.NetStat{
			{Interface: "eth0", RxBytes: 98765432, TxBytes: 45678901, RxPackets: 654321, TxPackets: 321098},
			{Interface: "lo", RxBytes: 1234567, TxBytes: 1234567, RxPackets: 12345, TxPackets: 12345},
		},
	}

	collector := metrics.NewSystemCollector(src, m, 1*time.Hour, "/")
	collector.Start()
	time.Sleep(100 * time.Millisecond)
	collector.Stop()

	// eth0
	if v := testutil.ToFloat64(m.ContainerNetworkReceiveBytes.WithLabelValues("eth0")); v != 98765432 {
		t.Errorf("expected eth0 RxBytes=98765432, got %f", v)
	}
	if v := testutil.ToFloat64(m.ContainerNetworkTransmitBytes.WithLabelValues("eth0")); v != 45678901 {
		t.Errorf("expected eth0 TxBytes=45678901, got %f", v)
	}
	if v := testutil.ToFloat64(m.ContainerNetworkReceivePackets.WithLabelValues("eth0")); v != 654321 {
		t.Errorf("expected eth0 RxPackets=654321, got %f", v)
	}
	if v := testutil.ToFloat64(m.ContainerNetworkTransmitPackets.WithLabelValues("eth0")); v != 321098 {
		t.Errorf("expected eth0 TxPackets=321098, got %f", v)
	}

	// lo
	if v := testutil.ToFloat64(m.ContainerNetworkReceiveBytes.WithLabelValues("lo")); v != 1234567 {
		t.Errorf("expected lo RxBytes=1234567, got %f", v)
	}
}

// TestSystemCollector_PanicRecovery verifies the collector continues after a source panic.
func TestSystemCollector_PanicRecovery(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	src := &panicSystemSource{
		panicOnCPU: true,
		mockSystemSource: mockSystemSource{
			memInfo:  &metrics.MemInfo{RSS: 1000, VMS: 2000, Limit: -1},
			diskInfo: &metrics.DiskInfo{Total: 100, Free: 50, Used: 50},
		},
	}

	collector := metrics.NewSystemCollector(src, m, 50*time.Millisecond, "/")
	collector.Start()
	time.Sleep(200 * time.Millisecond) // Let several ticks pass
	collector.Stop()

	// If we got here without hanging or crashing, the panic was recovered
}

// TestSystemCollector_Lifecycle verifies Start/Stop lifecycle including double Stop.
func TestSystemCollector_Lifecycle(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	src := &mockSystemSource{
		memInfo:  &metrics.MemInfo{RSS: 1000, VMS: 2000, Limit: -1},
		diskInfo: &metrics.DiskInfo{Total: 100, Free: 50, Used: 50},
	}

	collector := metrics.NewSystemCollector(src, m, 1*time.Hour, "/")
	collector.Start()

	done := make(chan struct{})
	go func() {
		collector.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("collector.Stop() did not return within 2 seconds")
	}

	// Double Stop should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("double Stop() panicked: %v", r)
		}
	}()
	collector.Stop()
}

// TestSystemCollector_ContainerMetricPrefix verifies all container metrics use druggate_container_ prefix.
func TestSystemCollector_ContainerMetricPrefix(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	// Initialize gauge vecs to ensure they show up
	m.ContainerNetworkReceiveBytes.WithLabelValues("test").Set(1)
	m.ContainerNetworkTransmitBytes.WithLabelValues("test").Set(1)
	m.ContainerNetworkReceivePackets.WithLabelValues("test").Set(1)
	m.ContainerNetworkTransmitPackets.WithLabelValues("test").Set(1)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	containerMetrics := 0
	for _, f := range families {
		if strings.HasPrefix(*f.Name, "druggate_container_") {
			containerMetrics++
		}
	}

	// We expect 13 container metrics (9 gauges + 4 gauge vecs)
	if containerMetrics < 13 {
		t.Errorf("expected at least 13 container metrics with druggate_container_ prefix, found %d", containerMetrics)
	}
}

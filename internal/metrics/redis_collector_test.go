package metrics

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"
)

func newTestRedisClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: addr,
	})
}

func TestRedisCollector_HealthyRedis(t *testing.T) {
	mr := miniredis.RunT(t)

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	rdb := newTestRedisClient(mr.Addr())
	defer rdb.Close()

	c := NewRedisCollector(rdb, m, 1*time.Second)
	c.Start()

	// Give the initial collect a moment to complete
	time.Sleep(100 * time.Millisecond)

	redisUp := testutil.ToFloat64(m.RedisUp)
	if redisUp != 1 {
		t.Errorf("expected RedisUp=1, got %v", redisUp)
	}

	pingDuration := testutil.ToFloat64(m.RedisPingDuration)
	if pingDuration <= 0 {
		t.Errorf("expected RedisPingDuration > 0, got %v", pingDuration)
	}

	c.Stop()
}

func TestRedisCollector_UnhealthyRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	addr := mr.Addr()

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	rdb := newTestRedisClient(addr)
	defer rdb.Close()

	// Close miniredis before creating collector to simulate unhealthy Redis
	mr.Close()

	c := NewRedisCollector(rdb, m, 1*time.Second)
	c.Start()

	// Give the initial collect a moment to complete
	time.Sleep(100 * time.Millisecond)

	redisUp := testutil.ToFloat64(m.RedisUp)
	if redisUp != 0 {
		t.Errorf("expected RedisUp=0, got %v", redisUp)
	}

	c.Stop()
}

func TestRedisCollector_Lifecycle(t *testing.T) {
	mr := miniredis.RunT(t)

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	rdb := newTestRedisClient(mr.Addr())
	defer rdb.Close()

	c := NewRedisCollector(rdb, m, 50*time.Millisecond)
	c.Start()

	// Let it run a few collection cycles
	time.Sleep(200 * time.Millisecond)

	// Stop should return without hanging
	done := make(chan struct{})
	go func() {
		c.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success — Stop returned cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return within 2 seconds")
	}

	// Calling Stop again should not panic (stopOnce protects)
	c.Stop()
}

func TestRedisCollector_CollectsOnStart(t *testing.T) {
	mr := miniredis.RunT(t)

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	rdb := newTestRedisClient(mr.Addr())
	defer rdb.Close()

	// Use a very long interval so only the immediate collect fires
	c := NewRedisCollector(rdb, m, 1*time.Hour)
	c.Start()

	// Give the initial collect a moment to complete
	time.Sleep(100 * time.Millisecond)

	redisUp := testutil.ToFloat64(m.RedisUp)
	if redisUp != 1 {
		t.Errorf("expected RedisUp=1 from immediate collect, got %v", redisUp)
	}

	pingDuration := testutil.ToFloat64(m.RedisPingDuration)
	if pingDuration <= 0 {
		t.Errorf("expected RedisPingDuration > 0 from immediate collect, got %v", pingDuration)
	}

	c.Stop()
}

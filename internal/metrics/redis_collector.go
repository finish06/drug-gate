package metrics

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCollector periodically collects Redis health metrics.
type RedisCollector struct {
	rdb      *redis.Client
	metrics  *Metrics
	interval time.Duration
	stopCh   chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

// NewRedisCollector creates a new background Redis health metrics collector.
func NewRedisCollector(rdb *redis.Client, m *Metrics, interval time.Duration) *RedisCollector {
	return &RedisCollector{
		rdb:      rdb,
		metrics:  m,
		interval: interval,
		stopCh:   make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start begins the background collection loop.
func (c *RedisCollector) Start() {
	go func() {
		defer close(c.done)

		// Collect once immediately
		c.collect()

		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.collect()
			case <-c.stopCh:
				return
			}
		}
	}()
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *RedisCollector) Stop() {
	c.stopOnce.Do(func() { close(c.stopCh) })
	<-c.done
}

func (c *RedisCollector) collect() {
	if c.rdb == nil {
		c.metrics.RedisUp.Set(0)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ping Redis
	pingStart := time.Now()
	err := c.rdb.Ping(ctx).Err()
	pingDuration := time.Since(pingStart).Seconds()

	if err != nil {
		c.metrics.RedisUp.Set(0)
		slog.Debug("redis ping failed", "component", "metrics", "error", err)
		return
	}

	c.metrics.RedisUp.Set(1)
	c.metrics.RedisPingDuration.Set(pingDuration)
}

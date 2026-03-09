package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Compile-time check that RedisLimiter implements Limiter.
var _ Limiter = (*RedisLimiter)(nil)

// Window is the sliding window duration for rate limiting.
const Window = time.Minute

// luaRateLimit is an atomic sliding window rate limit script.
// It prunes expired entries, counts current usage, and conditionally adds a new entry.
// Returns: [allowed (0/1), count_after]
var luaRateLimit = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]

-- Remove entries outside the window
redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)

-- Count current entries
local count = redis.call('ZCARD', key)

if count < limit then
    redis.call('ZADD', key, now, member)
    redis.call('PEXPIRE', key, window)
    return {1, count + 1}
else
    return {0, count}
end
`)

// RedisLimiter implements a sliding window rate limiter using Redis sorted sets.
type RedisLimiter struct {
	client *redis.Client
}

// NewRedisLimiter creates a new RedisLimiter.
func NewRedisLimiter(client *redis.Client) *RedisLimiter {
	return &RedisLimiter{client: client}
}

// Allow checks if a request is allowed under the rate limit for the given key.
func (l *RedisLimiter) Allow(ctx context.Context, key string, limit int) (*Result, error) {
	now := time.Now()
	nowMicro := now.UnixMicro()
	windowMicro := Window.Microseconds()
	member := fmt.Sprintf("%d:%d", nowMicro, now.UnixNano()) // unique member

	redisKey := "ratelimit:" + key

	res, err := luaRateLimit.Run(ctx, l.client, []string{redisKey},
		nowMicro, windowMicro, limit, member,
	).Int64Slice()
	if err != nil {
		return nil, fmt.Errorf("rate limit script: %w", err)
	}

	allowed := res[0] == 1
	count := int(res[1])
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}

	resetAt := now.Add(Window)

	result := &Result{
		Allowed:   allowed,
		Remaining: remaining,
		ResetAt:   resetAt,
	}

	if !allowed {
		result.RetryAfter = Window
	}

	return result, nil
}

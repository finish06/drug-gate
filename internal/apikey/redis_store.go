package apikey

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Compile-time check that RedisStore implements Store.
var _ Store = (*RedisStore)(nil)

// RedisStore is a Redis-backed implementation of the Store interface.
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a new RedisStore with the given Redis client.
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func redisKey(key string) string {
	return "apikey:" + key
}

// Create generates a new API key and stores it in Redis.
func (s *RedisStore) Create(ctx context.Context, appName string, origins []string, rateLimit int) (*APIKey, error) {
	keyStr, err := GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	ak := &APIKey{
		Key:       keyStr,
		AppName:   appName,
		Origins:   origins,
		RateLimit: rateLimit,
		Active:    true,
		CreatedAt: time.Now().UTC(),
	}

	data, err := json.Marshal(ak)
	if err != nil {
		return nil, fmt.Errorf("marshal key: %w", err)
	}

	if err := s.client.Set(ctx, redisKey(keyStr), data, 0).Err(); err != nil {
		return nil, fmt.Errorf("redis set: %w", err)
	}

	slog.Info("api key stored", "key", keyStr, "app_name", appName)
	return ak, nil
}

// Get retrieves an API key from Redis. Returns (nil, nil) if not found.
func (s *RedisStore) Get(ctx context.Context, key string) (*APIKey, error) {
	data, err := s.client.Get(ctx, redisKey(key)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}

	var ak APIKey
	if err := json.Unmarshal(data, &ak); err != nil {
		return nil, fmt.Errorf("unmarshal key: %w", err)
	}
	return &ak, nil
}

// List returns all API keys from Redis.
func (s *RedisStore) List(ctx context.Context) ([]APIKey, error) {
	var keys []APIKey
	var cursor uint64

	for {
		var scanKeys []string
		var err error
		scanKeys, cursor, err = s.client.Scan(ctx, cursor, "apikey:*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("redis scan: %w", err)
		}

		for _, rk := range scanKeys {
			data, err := s.client.Get(ctx, rk).Bytes()
			if err != nil {
				slog.Warn("failed to get key during list", "redis_key", rk, "err", err)
				continue
			}

			var ak APIKey
			if err := json.Unmarshal(data, &ak); err != nil {
				slog.Warn("failed to unmarshal key during list", "redis_key", rk, "err", err)
				continue
			}
			keys = append(keys, ak)
		}

		if cursor == 0 {
			break
		}
	}

	if keys == nil {
		keys = []APIKey{}
	}
	return keys, nil
}

// Deactivate sets an API key to inactive.
func (s *RedisStore) Deactivate(ctx context.Context, key string) error {
	ak, err := s.Get(ctx, key)
	if err != nil {
		return err
	}
	if ak == nil {
		return ErrKeyNotFound
	}

	ak.Active = false
	data, err := json.Marshal(ak)
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}

	if err := s.client.Set(ctx, redisKey(key), data, 0).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	slog.Info("api key deactivated", "key", key)
	return nil
}

// Rotate creates a new key with the same metadata as the old key, and sets
// an expiration on the old key for the grace period.
func (s *RedisStore) Rotate(ctx context.Context, oldKey string, gracePeriod time.Duration) (*APIKey, error) {
	old, err := s.Get(ctx, oldKey)
	if err != nil {
		return nil, err
	}
	if old == nil {
		return nil, ErrKeyNotFound
	}

	// Set expiration on old key
	exp := time.Now().UTC().Add(gracePeriod)
	old.ExpiresAt = &exp

	oldData, err := json.Marshal(old)
	if err != nil {
		return nil, fmt.Errorf("marshal old key: %w", err)
	}
	if err := s.client.Set(ctx, redisKey(oldKey), oldData, 0).Err(); err != nil {
		return nil, fmt.Errorf("redis set old key: %w", err)
	}

	// Create new key with same metadata
	newAK, err := s.Create(ctx, old.AppName, old.Origins, old.RateLimit)
	if err != nil {
		return nil, fmt.Errorf("create rotated key: %w", err)
	}

	slog.Info("api key rotated", "old_key", oldKey, "new_key", newAK.Key)
	return newAK, nil
}

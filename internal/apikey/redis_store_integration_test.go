//go:build integration

package apikey_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/finish06/drug-gate/internal/apikey"
	"github.com/redis/go-redis/v9"
)

func setupRedisStore(t *testing.T) *apikey.RedisStore {
	t.Helper()

	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		addr = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}

	// Flush test keys before each test
	t.Cleanup(func() {
		iter := rdb.Scan(ctx, 0, "apikey:*", 100).Iterator()
		for iter.Next(ctx) {
			rdb.Del(ctx, iter.Val())
		}
		rdb.Close()
	})

	return apikey.NewRedisStore(rdb)
}

func TestRedisStore_CreateAndGet(t *testing.T) {
	store := setupRedisStore(t)
	ctx := context.Background()

	ak, err := store.Create(ctx, "test-app", []string{"https://example.com"}, 100)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ak.Key == "" {
		t.Fatal("expected non-empty key")
	}
	if ak.AppName != "test-app" {
		t.Errorf("AppName = %q, want %q", ak.AppName, "test-app")
	}
	if !ak.Active {
		t.Error("expected active=true")
	}

	got, err := store.Get(ctx, ak.Key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected key, got nil")
	}
	if got.AppName != "test-app" {
		t.Errorf("AppName = %q, want %q", got.AppName, "test-app")
	}
}

func TestRedisStore_GetNotFound(t *testing.T) {
	store := setupRedisStore(t)
	ctx := context.Background()

	ak, err := store.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ak != nil {
		t.Error("expected nil for nonexistent key")
	}
}

func TestRedisStore_List(t *testing.T) {
	store := setupRedisStore(t)
	ctx := context.Background()

	_, _ = store.Create(ctx, "app-1", nil, 50)
	_, _ = store.Create(ctx, "app-2", []string{"https://two.com"}, 75)

	keys, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("len = %d, want 2", len(keys))
	}
}

func TestRedisStore_Deactivate(t *testing.T) {
	store := setupRedisStore(t)
	ctx := context.Background()

	ak, _ := store.Create(ctx, "deact-app", nil, 30)

	if err := store.Deactivate(ctx, ak.Key); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}

	got, _ := store.Get(ctx, ak.Key)
	if got.Active {
		t.Error("expected active=false after deactivation")
	}
}

func TestRedisStore_Rotate(t *testing.T) {
	store := setupRedisStore(t)
	ctx := context.Background()

	old, _ := store.Create(ctx, "rotate-app", []string{"https://rotate.com"}, 90)

	newKey, err := store.Rotate(ctx, old.Key, 24*time.Hour)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if newKey.Key == old.Key {
		t.Error("expected different key after rotation")
	}
	if newKey.AppName != old.AppName {
		t.Errorf("AppName = %q, want %q", newKey.AppName, old.AppName)
	}

	// Old key should have ExpiresAt set
	oldUpdated, _ := store.Get(ctx, old.Key)
	if oldUpdated.ExpiresAt == nil {
		t.Error("expected ExpiresAt on old key after rotation")
	}
}

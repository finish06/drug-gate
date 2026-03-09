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

func TestRedisStore_Rotate_PreservesMetadata(t *testing.T) {
	store := setupRedisStore(t)
	ctx := context.Background()

	origins := []string{"https://a.com", "https://b.com"}
	old, _ := store.Create(ctx, "meta-app", origins, 200)

	newKey, err := store.Rotate(ctx, old.Key, 1*time.Hour)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	if len(newKey.Origins) != len(origins) {
		t.Errorf("origins len = %d, want %d", len(newKey.Origins), len(origins))
	}
	for i, o := range origins {
		if newKey.Origins[i] != o {
			t.Errorf("origin[%d] = %q, want %q", i, newKey.Origins[i], o)
		}
	}
	if newKey.RateLimit != 200 {
		t.Errorf("RateLimit = %d, want 200", newKey.RateLimit)
	}
	if !newKey.Active {
		t.Error("expected new key to be active")
	}
}

func TestRedisStore_Rotate_NotFound(t *testing.T) {
	store := setupRedisStore(t)
	ctx := context.Background()

	_, err := store.Rotate(ctx, "pk_nonexistent", 1*time.Hour)
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
	if err != apikey.ErrKeyNotFound {
		t.Errorf("err = %v, want ErrKeyNotFound", err)
	}
}

func TestRedisStore_Deactivate_NotFound(t *testing.T) {
	store := setupRedisStore(t)
	ctx := context.Background()

	err := store.Deactivate(ctx, "pk_nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
	if err != apikey.ErrKeyNotFound {
		t.Errorf("err = %v, want ErrKeyNotFound", err)
	}
}

func TestRedisStore_Deactivate_StillRetrievable(t *testing.T) {
	store := setupRedisStore(t)
	ctx := context.Background()

	ak, _ := store.Create(ctx, "deact-retrieve", nil, 50)
	_ = store.Deactivate(ctx, ak.Key)

	got, err := store.Get(ctx, ak.Key)
	if err != nil {
		t.Fatalf("Get after deactivate: %v", err)
	}
	if got == nil {
		t.Fatal("expected key to still exist after deactivation")
	}
	if got.Active {
		t.Error("expected active=false")
	}
	if got.AppName != "deact-retrieve" {
		t.Errorf("AppName = %q, want %q", got.AppName, "deact-retrieve")
	}
}

func TestRedisStore_ListEmpty(t *testing.T) {
	store := setupRedisStore(t)
	ctx := context.Background()

	keys, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if keys == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(keys) != 0 {
		t.Errorf("len = %d, want 0", len(keys))
	}
}

func TestRedisStore_Create_OriginsPreserved(t *testing.T) {
	store := setupRedisStore(t)
	ctx := context.Background()

	origins := []string{"https://x.com", "https://y.com", "https://z.com"}
	ak, err := store.Create(ctx, "origins-app", origins, 100)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, _ := store.Get(ctx, ak.Key)
	if len(got.Origins) != 3 {
		t.Fatalf("origins len = %d, want 3", len(got.Origins))
	}
	for i, o := range origins {
		if got.Origins[i] != o {
			t.Errorf("origin[%d] = %q, want %q", i, got.Origins[i], o)
		}
	}
}

func TestRedisStore_Create_HasPkPrefix(t *testing.T) {
	store := setupRedisStore(t)
	ctx := context.Background()

	ak, err := store.Create(ctx, "prefix-app", nil, 50)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(ak.Key) < 3 || ak.Key[:3] != "pk_" {
		t.Errorf("key %q does not have pk_ prefix", ak.Key)
	}
}

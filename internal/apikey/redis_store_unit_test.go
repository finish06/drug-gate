package apikey

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestStore creates a RedisStore backed by miniredis for unit testing.
func newTestStore(t *testing.T) *RedisStore {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisStore(client)
}

// newTestStoreWithMini returns both store and miniredis for direct data manipulation.
func newTestStoreWithMini(t *testing.T) (*RedisStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisStore(client), mr
}

func TestRedisStore_Create_StoresKeyWithPrefix(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	origins := []string{"https://example.com", "https://app.example.com"}
	ak, err := store.Create(ctx, "test-app", origins, 100)
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	if !strings.HasPrefix(ak.Key, "pk_") {
		t.Errorf("expected key with 'pk_' prefix, got %q", ak.Key)
	}
	if ak.AppName != "test-app" {
		t.Errorf("expected AppName 'test-app', got %q", ak.AppName)
	}
	if len(ak.Origins) != 2 {
		t.Errorf("expected 2 origins, got %d", len(ak.Origins))
	}
	if ak.RateLimit != 100 {
		t.Errorf("expected RateLimit 100, got %d", ak.RateLimit)
	}
	if !ak.Active {
		t.Error("expected Active to be true on creation")
	}
	if ak.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if ak.ExpiresAt != nil {
		t.Error("expected ExpiresAt to be nil on creation")
	}

	// Verify it's actually stored in Redis by fetching it back
	fetched, err := store.Get(ctx, ak.Key)
	if err != nil {
		t.Fatalf("Get() after Create() returned error: %v", err)
	}
	if fetched == nil {
		t.Fatal("expected Get() to return the created key, got nil")
	}
	if fetched.Key != ak.Key {
		t.Errorf("expected key %q, got %q", ak.Key, fetched.Key)
	}
}

func TestRedisStore_Get_ExistingKey(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	created, err := store.Create(ctx, "get-app", []string{"https://get.com"}, 75)
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	fetched, err := store.Get(ctx, created.Key)
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if fetched == nil {
		t.Fatal("expected non-nil result from Get()")
	}
	if fetched.Key != created.Key {
		t.Errorf("expected key %q, got %q", created.Key, fetched.Key)
	}
	if fetched.AppName != "get-app" {
		t.Errorf("expected AppName 'get-app', got %q", fetched.AppName)
	}
	if fetched.RateLimit != 75 {
		t.Errorf("expected RateLimit 75, got %d", fetched.RateLimit)
	}
}

func TestRedisStore_Get_NonexistentKey_ReturnsNil(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	fetched, err := store.Get(ctx, "pk_nonexistent")
	if err != nil {
		t.Fatalf("Get() returned unexpected error: %v", err)
	}
	if fetched != nil {
		t.Errorf("expected nil for nonexistent key, got %+v", fetched)
	}
}

func TestRedisStore_List_ReturnsAllKeys(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.Create(ctx, "app-1", nil, 10)
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}
	_, err = store.Create(ctx, "app-2", []string{"https://two.com"}, 20)
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	keys, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}

	// Verify both app names are present
	names := map[string]bool{}
	for _, k := range keys {
		names[k.AppName] = true
	}
	if !names["app-1"] {
		t.Error("expected app-1 in list results")
	}
	if !names["app-2"] {
		t.Error("expected app-2 in list results")
	}
}

func TestRedisStore_List_EmptyStore(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	keys, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

func TestRedisStore_Deactivate_SetsActiveFalse(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	ak, err := store.Create(ctx, "deact-app", nil, 30)
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}
	if !ak.Active {
		t.Fatal("expected key to be active after creation")
	}

	err = store.Deactivate(ctx, ak.Key)
	if err != nil {
		t.Fatalf("Deactivate() returned error: %v", err)
	}

	fetched, err := store.Get(ctx, ak.Key)
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if fetched == nil {
		t.Fatal("expected key to still exist after deactivation")
	}
	if fetched.Active {
		t.Error("expected Active to be false after deactivation")
	}
}

func TestRedisStore_Deactivate_NonexistentKey_ReturnsError(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.Deactivate(ctx, "pk_nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent key, got nil")
	}
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestRedisStore_Rotate_CreatesNewKey(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	old, err := store.Create(ctx, "rotate-app", []string{"https://rotate.com"}, 90)
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	gracePeriod := 24 * time.Hour
	newKey, err := store.Rotate(ctx, old.Key, gracePeriod)
	if err != nil {
		t.Fatalf("Rotate() returned error: %v", err)
	}

	if newKey.Key == old.Key {
		t.Error("expected new key to differ from old key")
	}
	if !strings.HasPrefix(newKey.Key, "pk_") {
		t.Errorf("expected new key with 'pk_' prefix, got %q", newKey.Key)
	}
	if !newKey.Active {
		t.Error("expected new key to be active")
	}
}

func TestRedisStore_Rotate_SetsExpiresAtOnOldKey(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	old, err := store.Create(ctx, "rotate-app", nil, 50)
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	gracePeriod := 2 * time.Hour
	before := time.Now().UTC()
	_, err = store.Rotate(ctx, old.Key, gracePeriod)
	if err != nil {
		t.Fatalf("Rotate() returned error: %v", err)
	}

	fetched, err := store.Get(ctx, old.Key)
	if err != nil {
		t.Fatalf("Get() returned error for old key: %v", err)
	}
	if fetched == nil {
		t.Fatal("expected old key to still exist after rotation")
	}
	if fetched.ExpiresAt == nil {
		t.Fatal("expected ExpiresAt to be set on old key after rotation")
	}

	expectedExpiry := before.Add(gracePeriod)
	if fetched.ExpiresAt.Before(expectedExpiry.Add(-1 * time.Second)) {
		t.Errorf("ExpiresAt too early: got %v, expected around %v", fetched.ExpiresAt, expectedExpiry)
	}
	if fetched.ExpiresAt.After(expectedExpiry.Add(1 * time.Second)) {
		t.Errorf("ExpiresAt too late: got %v, expected around %v", fetched.ExpiresAt, expectedExpiry)
	}
}

func TestRedisStore_Rotate_NonexistentKey_ReturnsError(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.Rotate(ctx, "pk_nonexistent", time.Hour)
	if err == nil {
		t.Fatal("expected error for nonexistent key, got nil")
	}
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestRedisStore_Rotate_PreservesMetadata(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	origins := []string{"https://a.com", "https://b.com"}
	old, err := store.Create(ctx, "meta-app", origins, 150)
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	newKey, err := store.Rotate(ctx, old.Key, time.Hour)
	if err != nil {
		t.Fatalf("Rotate() returned error: %v", err)
	}

	if newKey.AppName != old.AppName {
		t.Errorf("expected AppName %q, got %q", old.AppName, newKey.AppName)
	}
	if len(newKey.Origins) != len(old.Origins) {
		t.Fatalf("expected %d origins, got %d", len(old.Origins), len(newKey.Origins))
	}
	for i, o := range old.Origins {
		if newKey.Origins[i] != o {
			t.Errorf("expected origin[%d] %q, got %q", i, o, newKey.Origins[i])
		}
	}
	if newKey.RateLimit != old.RateLimit {
		t.Errorf("expected RateLimit %d, got %d", old.RateLimit, newKey.RateLimit)
	}
}

func TestRedisStore_GenerateKey_PrefixAndLength(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() returned error: %v", err)
	}
	if !strings.HasPrefix(key, "pk_") {
		t.Errorf("expected 'pk_' prefix, got %q", key)
	}
	// pk_ (3) + 48 hex chars (24 bytes) = 51
	if len(key) != 51 {
		t.Errorf("expected key length 51, got %d (%q)", len(key), key)
	}
}

func TestRedisStore_DeactivatedKey_StillRetrievable(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	ak, err := store.Create(ctx, "still-here", []string{"https://still.com"}, 40)
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	err = store.Deactivate(ctx, ak.Key)
	if err != nil {
		t.Fatalf("Deactivate() returned error: %v", err)
	}

	fetched, err := store.Get(ctx, ak.Key)
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if fetched == nil {
		t.Fatal("expected deactivated key to be retrievable, got nil")
	}
	if fetched.Active {
		t.Error("expected Active to be false")
	}
	if fetched.AppName != "still-here" {
		t.Errorf("expected AppName 'still-here', got %q", fetched.AppName)
	}
}

func TestRedisStore_Get_CorruptData_ReturnsError(t *testing.T) {
	store, mr := newTestStoreWithMini(t)
	ctx := context.Background()

	// Plant corrupt JSON directly in Redis
	_ = mr.Set("apikey:pk_corrupt", "not-valid-json{{{")

	_, err := store.Get(ctx, "pk_corrupt")
	if err == nil {
		t.Fatal("expected error for corrupt data, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestRedisStore_List_SkipsCorruptEntries(t *testing.T) {
	store, mr := newTestStoreWithMini(t)
	ctx := context.Background()

	// Create one valid key
	_, err := store.Create(ctx, "valid-app", nil, 10)
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	// Plant corrupt JSON directly
	_ = mr.Set("apikey:pk_corrupt1", "bad-json")
	_ = mr.Set("apikey:pk_corrupt2", "{invalid")

	keys, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	// Should return only the valid key, skipping corrupt ones
	if len(keys) != 1 {
		t.Errorf("expected 1 valid key (corrupt skipped), got %d", len(keys))
	}
	if keys[0].AppName != "valid-app" {
		t.Errorf("expected AppName 'valid-app', got %q", keys[0].AppName)
	}
}

func TestRedisStore_GenerateKey_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		key, err := GenerateKey()
		if err != nil {
			t.Fatalf("GenerateKey() #%d returned error: %v", i, err)
		}
		if seen[key] {
			t.Fatalf("duplicate key generated: %q", key)
		}
		seen[key] = true
	}
}

func TestRedisStore_CreateMultipleKeys_ListReturnsAll(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	count := 5
	for i := 0; i < count; i++ {
		_, err := store.Create(ctx, "multi-app", nil, 10+i)
		if err != nil {
			t.Fatalf("Create() #%d returned error: %v", i, err)
		}
	}

	keys, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(keys) != count {
		t.Errorf("expected %d keys, got %d", count, len(keys))
	}
}

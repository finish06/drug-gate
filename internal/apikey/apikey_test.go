package apikey

import (
	"context"
	"strings"
	"testing"
	"time"
)

// MockStore implements Store for unit testing.
type MockStore struct {
	keys map[string]*APIKey
}

func NewMockStore() *MockStore {
	return &MockStore{
		keys: make(map[string]*APIKey),
	}
}

func (m *MockStore) Create(ctx context.Context, appName string, origins []string, rateLimit int) (*APIKey, error) {
	key, err := GenerateKey()
	if err != nil {
		return nil, err
	}

	ak := &APIKey{
		Key:       key,
		AppName:   appName,
		Origins:   origins,
		RateLimit: rateLimit,
		Active:    true,
		CreatedAt: time.Now().UTC(),
	}
	m.keys[key] = ak
	return ak, nil
}

func (m *MockStore) Get(ctx context.Context, key string) (*APIKey, error) {
	ak, ok := m.keys[key]
	if !ok {
		return nil, ErrKeyNotFound
	}
	return ak, nil
}

func (m *MockStore) List(ctx context.Context) ([]APIKey, error) {
	result := make([]APIKey, 0, len(m.keys))
	for _, ak := range m.keys {
		result = append(result, *ak)
	}
	return result, nil
}

func (m *MockStore) Deactivate(ctx context.Context, key string) error {
	ak, ok := m.keys[key]
	if !ok {
		return ErrKeyNotFound
	}
	ak.Active = false
	return nil
}

func (m *MockStore) Rotate(ctx context.Context, oldKey string, gracePeriod time.Duration) (*APIKey, error) {
	old, ok := m.keys[oldKey]
	if !ok {
		return nil, ErrKeyNotFound
	}

	// Set expiration on old key
	exp := time.Now().UTC().Add(gracePeriod)
	old.ExpiresAt = &exp

	// Create new key with same metadata
	newKey, err := GenerateKey()
	if err != nil {
		return nil, err
	}

	ak := &APIKey{
		Key:       newKey,
		AppName:   old.AppName,
		Origins:   old.Origins,
		RateLimit: old.RateLimit,
		Active:    true,
		CreatedAt: time.Now().UTC(),
	}
	m.keys[newKey] = ak
	return ak, nil
}

// --- Key generation tests ---

func TestGenerateKey_AC011_HasPrefix(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() returned error: %v", err)
	}
	if !strings.HasPrefix(key, "pk_") {
		t.Errorf("expected key to have prefix 'pk_', got %q", key)
	}
}

func TestGenerateKey_AC011_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key, err := GenerateKey()
		if err != nil {
			t.Fatalf("GenerateKey() returned error on iteration %d: %v", i, err)
		}
		if seen[key] {
			t.Fatalf("duplicate key generated: %q", key)
		}
		seen[key] = true
	}
}

func TestGenerateKey_AC011_MinLength(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() returned error: %v", err)
	}
	// pk_ prefix (3 chars) + at least 32 chars of entropy
	if len(key) < 35 {
		t.Errorf("expected key length >= 35, got %d (%q)", len(key), key)
	}
}

// --- MockStore Create tests ---

func TestMockStore_AC010_CreateStoresMetadata(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	origins := []string{"https://example.com", "https://app.example.com"}
	ak, err := store.Create(ctx, "test-app", origins, 100)
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	// AC-010: Verify all metadata fields are stored
	if ak.Key == "" {
		t.Error("expected Key to be non-empty")
	}
	if ak.AppName != "test-app" {
		t.Errorf("expected AppName 'test-app', got %q", ak.AppName)
	}
	if len(ak.Origins) != 2 {
		t.Errorf("expected 2 origins, got %d", len(ak.Origins))
	}
	if ak.Origins[0] != "https://example.com" {
		t.Errorf("expected first origin 'https://example.com', got %q", ak.Origins[0])
	}
	if ak.Origins[1] != "https://app.example.com" {
		t.Errorf("expected second origin 'https://app.example.com', got %q", ak.Origins[1])
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
}

func TestMockStore_AC011_CreateGeneratesPrefixedKey(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	ak, err := store.Create(ctx, "my-app", nil, 50)
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}
	if !strings.HasPrefix(ak.Key, "pk_") {
		t.Errorf("expected key with 'pk_' prefix, got %q", ak.Key)
	}
}

func TestMockStore_AC004_CreateWithOrigins(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	origins := []string{"https://myapp.com"}
	ak, err := store.Create(ctx, "origin-app", origins, 60)
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}
	if len(ak.Origins) != 1 || ak.Origins[0] != "https://myapp.com" {
		t.Errorf("expected origins [https://myapp.com], got %v", ak.Origins)
	}
}

func TestMockStore_AC006_CreateWithoutOrigins(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	// nil origins = origin-free key, accepts requests from any origin
	ak, err := store.Create(ctx, "open-app", nil, 200)
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}
	if ak.Origins != nil {
		t.Errorf("expected nil origins for origin-free key, got %v", ak.Origins)
	}
}

func TestMockStore_AC006_CreateWithEmptyOrigins(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	// Empty slice also means origin-free
	ak, err := store.Create(ctx, "open-app", []string{}, 200)
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}
	if len(ak.Origins) != 0 {
		t.Errorf("expected empty origins for origin-free key, got %v", ak.Origins)
	}
}

// --- MockStore Get tests ---

func TestMockStore_AC010_GetReturnsStoredKey(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	created, err := store.Create(ctx, "get-app", []string{"https://get.com"}, 75)
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	fetched, err := store.Get(ctx, created.Key)
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if fetched.Key != created.Key {
		t.Errorf("expected key %q, got %q", created.Key, fetched.Key)
	}
	if fetched.AppName != "get-app" {
		t.Errorf("expected AppName 'get-app', got %q", fetched.AppName)
	}
}

func TestMockStore_GetNonexistentKey_ReturnsError(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	_, err := store.Get(ctx, "pk_nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent key, got nil")
	}
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

// --- MockStore List tests ---

func TestMockStore_AC010_ListReturnsAllKeys(t *testing.T) {
	store := NewMockStore()
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
}

func TestMockStore_ListEmpty(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	keys, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

// --- MockStore Deactivate tests ---

func TestMockStore_AC012_DeactivateSetsActiveFalse(t *testing.T) {
	store := NewMockStore()
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
	if fetched.Active {
		t.Error("expected Active to be false after deactivation")
	}
}

func TestMockStore_AC012_DeactivateNonexistentKey_ReturnsError(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	err := store.Deactivate(ctx, "pk_nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent key, got nil")
	}
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

// --- MockStore Rotate tests ---

func TestMockStore_AC013_RotateCreatesNewKey(t *testing.T) {
	store := NewMockStore()
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

	// New key should be different from old
	if newKey.Key == old.Key {
		t.Error("expected new key to differ from old key")
	}

	// New key should have pk_ prefix
	if !strings.HasPrefix(newKey.Key, "pk_") {
		t.Errorf("expected new key with 'pk_' prefix, got %q", newKey.Key)
	}

	// New key should be active
	if !newKey.Active {
		t.Error("expected new key to be active")
	}

	// New key should inherit metadata
	if newKey.AppName != old.AppName {
		t.Errorf("expected AppName %q, got %q", old.AppName, newKey.AppName)
	}
	if len(newKey.Origins) != len(old.Origins) {
		t.Errorf("expected %d origins, got %d", len(old.Origins), len(newKey.Origins))
	}
	if newKey.RateLimit != old.RateLimit {
		t.Errorf("expected RateLimit %d, got %d", old.RateLimit, newKey.RateLimit)
	}
}

func TestMockStore_AC013_RotateSetsExpiresAtOnOldKey(t *testing.T) {
	store := NewMockStore()
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

	// Fetch old key and verify ExpiresAt is set
	fetched, err := store.Get(ctx, old.Key)
	if err != nil {
		t.Fatalf("Get() returned error for old key: %v", err)
	}
	if fetched.ExpiresAt == nil {
		t.Fatal("expected ExpiresAt to be set on old key after rotation")
	}

	expectedExpiry := before.Add(gracePeriod)
	// Allow 1 second tolerance for timing
	if fetched.ExpiresAt.Before(expectedExpiry.Add(-1 * time.Second)) {
		t.Errorf("ExpiresAt too early: got %v, expected around %v", fetched.ExpiresAt, expectedExpiry)
	}
	if fetched.ExpiresAt.After(expectedExpiry.Add(1 * time.Second)) {
		t.Errorf("ExpiresAt too late: got %v, expected around %v", fetched.ExpiresAt, expectedExpiry)
	}
}

func TestMockStore_AC013_RotateNonexistentKey_ReturnsError(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	_, err := store.Rotate(ctx, "pk_nonexistent", time.Hour)
	if err == nil {
		t.Fatal("expected error for nonexistent key, got nil")
	}
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

// --- Store interface compliance ---

func TestMockStore_ImplementsStoreInterface(t *testing.T) {
	var _ Store = (*MockStore)(nil)
}

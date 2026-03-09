package apikey

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// ErrKeyNotFound is returned when a key does not exist in the store.
var ErrKeyNotFound = errors.New("api key not found")

// APIKey represents an API key stored in Redis.
type APIKey struct {
	Key       string     `json:"key"`
	AppName   string     `json:"app_name"`
	Origins   []string   `json:"origins"`
	RateLimit int        `json:"rate_limit"`
	Active    bool       `json:"active"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Store defines the interface for API key CRUD operations.
type Store interface {
	Create(ctx context.Context, appName string, origins []string, rateLimit int) (*APIKey, error)
	Get(ctx context.Context, key string) (*APIKey, error)
	List(ctx context.Context) ([]APIKey, error)
	Deactivate(ctx context.Context, key string) error
	Rotate(ctx context.Context, oldKey string, gracePeriod time.Duration) (*APIKey, error)
}

// GenerateKey produces a new API key with the "pk_" prefix and 24 bytes of random hex.
func GenerateKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "pk_" + hex.EncodeToString(b), nil
}

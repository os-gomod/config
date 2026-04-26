// Package secure provides secret management abstractions for storing
// and retrieving sensitive configuration values from external secret
// management systems (Vault, KMS, etc.).
package secure

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Store is the interface for secret storage backends.
// Implementations provide secure read/write access to secrets.
type Store interface {
	// GetSecret retrieves a secret value by path.
	GetSecret(ctx context.Context, path string) (string, error)
	// SetSecret writes a secret value at the given path.
	SetSecret(ctx context.Context, path, value string) error
	// DeleteSecret removes a secret at the given path.
	DeleteSecret(ctx context.Context, path string) error
	// ListSecrets lists all secret paths under the given prefix.
	ListSecrets(ctx context.Context, path string) ([]string, error)
}

// Config holds common configuration for secret store backends.
type Config struct {
	// Address is the backend address (URL, host:port, etc.).
	Address string

	// Token is the authentication token.
	Token string

	// Namespace is the logical namespace/prefix for secrets.
	Namespace string

	// Timeout is the per-operation timeout.
	Timeout time.Duration

	// MaxRetries is the number of retry attempts for transient errors.
	MaxRetries int

	// RetryDelay is the base delay between retries.
	RetryDelay time.Duration

	// TLSConfig holds TLS-related configuration.
	TLSConfig TLSConfig
}

// TLSConfig holds TLS configuration for secure connections.
type TLSConfig struct {
	// CACertPath is the path to the CA certificate.
	CACertPath string
	// CertPath is the path to the client certificate.
	CertPath string
	// KeyPath is the path to the client private key.
	KeyPath string
	// InsecureSkipVerify disables TLS verification.
	InsecureSkipVerify bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
		Namespace:  "secret",
	}
}

// Status represents the health status of a secret store.
type Status struct {
	Available   bool          `json:"available"`
	LastError   string        `json:"last_error,omitempty"`
	LastCheck   time.Time     `json:"last_check"`
	Latency     time.Duration `json:"latency_ms"`
	SecretCount int           `json:"secret_count"`
}

// String returns a human-readable status summary.
func (s Status) String() string {
	status := "available"
	if !s.Available {
		status = "unavailable"
	}
	var result string
	result = "status=" + status
	if s.LastError != "" {
		result += fmt.Sprintf(" err=%q", s.LastError)
	}
	if s.Latency > 0 {
		result += fmt.Sprintf(" latency=%dms", s.Latency.Milliseconds())
	}
	result += fmt.Sprintf(" secrets=%d", s.SecretCount)
	return result
}

// CachedStore wraps a Store with an in-memory cache and mutex protection.
type CachedStore struct {
	store Store
	mu    sync.RWMutex
	cache map[string]cacheEntry
	ttl   time.Duration
}

type cacheEntry struct {
	value     string
	expiresAt time.Time
}

// NewCachedStore creates a cached wrapper around a Store.
func NewCachedStore(store Store, ttl time.Duration) *CachedStore {
	return &CachedStore{
		store: store,
		cache: make(map[string]cacheEntry),
		ttl:   ttl,
	}
}

// GetSecret retrieves a secret, using the cache when possible.
func (c *CachedStore) GetSecret(ctx context.Context, path string) (string, error) {
	c.mu.RLock()
	if entry, ok := c.cache[path]; ok && time.Now().Before(entry.expiresAt) {
		c.mu.RUnlock()
		return entry.value, nil
	}
	c.mu.RUnlock()

	value, err := c.store.GetSecret(ctx, path)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	c.cache[path] = cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return value, nil
}

// SetSecret writes a secret through the underlying store and updates the cache.
func (c *CachedStore) SetSecret(ctx context.Context, path, value string) error {
	if err := c.store.SetSecret(ctx, path, value); err != nil {
		return err
	}

	c.mu.Lock()
	c.cache[path] = cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return nil
}

// DeleteSecret deletes a secret and removes it from the cache.
func (c *CachedStore) DeleteSecret(ctx context.Context, path string) error {
	if err := c.store.DeleteSecret(ctx, path); err != nil {
		return err
	}

	c.mu.Lock()
	delete(c.cache, path)
	c.mu.Unlock()

	return nil
}

// ListSecrets lists secrets under a path prefix.
func (c *CachedStore) ListSecrets(ctx context.Context, path string) ([]string, error) {
	return c.store.ListSecrets(ctx, path)
}

// Invalidate removes a single path from the cache.
func (c *CachedStore) Invalidate(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cache, path)
}

// InvalidateAll clears the entire cache.
func (c *CachedStore) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]cacheEntry)
}

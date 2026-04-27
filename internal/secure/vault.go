package secure

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// VaultStore implements Store for HashiCorp Vault.
// It uses the Vault KV v2 secrets engine by default.
type VaultStore struct {
	config Config
	mu     sync.RWMutex
	status Status
	client vaultClient
}

// vaultClient abstracts the Vault HTTP API.
type vaultClient interface {
	read(path string) (map[string]any, error)
	write(path string, data map[string]any) error
	delete(path string) error
	list(path string) ([]string, error)
	health() error
}

// stubClient is a placeholder that implements vaultClient.
type stubClient struct {
	secrets map[string]string
}

func (s *stubClient) read(path string) (map[string]any, error) {
	val, ok := s.secrets[path]
	if !ok {
		return nil, fmt.Errorf("secret not found: %s", path)
	}
	return map[string]any{"value": val}, nil
}

func (s *stubClient) write(path string, data map[string]any) error {
	if v, ok := data["value"].(string); ok {
		s.secrets[path] = v
		return nil
	}
	return errors.New("value must be a string")
}

func (s *stubClient) delete(path string) error {
	delete(s.secrets, path)
	return nil
}

func (s *stubClient) list(path string) ([]string, error) {
	var result []string
	for k := range s.secrets {
		if len(k) >= len(path) && k[:len(path)] == path {
			result = append(result, k)
		}
	}
	return result, nil
}

func (s *stubClient) health() error {
	return nil
}

// NewVaultStore creates a VaultStore with the given configuration.
// The actual Vault API client is a stub; replace with a real implementation
// that uses github.com/hashicorp/vault/api in production.
//
//nolint:gocritic // Config is intentionally copied to avoid external mutation
func NewVaultStore(cfg Config) *VaultStore {
	store := &VaultStore{
		config: cfg,
		status: Status{
			Available: false,
		},
	}

	store.client = &stubClient{
		secrets: make(map[string]string),
	}

	return store
}

// Connect initializes the connection to Vault and performs a health check.
//
//nolint:dupl // similar to KMSStore.Connect by design
func (s *VaultStore) Connect(ctx context.Context) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.client.health(); err != nil {
		s.status = Status{
			Available: false,
			LastError: err.Error(),
			LastCheck: time.Now().UTC(),
		}
		return fmt.Errorf("vault: health check failed: %w", err)
	}

	s.status = Status{
		Available: true,
		LastCheck: time.Now().UTC(),
	}
	return nil
}

// GetSecret retrieves a secret from Vault KV v2.
func (s *VaultStore) GetSecret(_ context.Context, path string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.status.Available {
		return "", errors.New("vault: store is not available")
	}

	start := time.Now()
	defer func() {
		s.status.Latency = time.Since(start)
		s.status.LastCheck = time.Now().UTC()
	}()

	fullPath := s.fullPath(path)
	data, err := s.client.read(fullPath)
	if err != nil {
		s.status.LastError = err.Error()
		return "", fmt.Errorf("vault: read %q failed: %w", fullPath, err)
	}

	val, ok := data["value"].(string)
	if !ok {
		return "", fmt.Errorf("vault: secret at %q has no string value", fullPath)
	}

	s.status.LastError = ""
	return val, nil
}

// SetSecret writes a secret to Vault KV v2.
func (s *VaultStore) SetSecret(_ context.Context, path, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.status.Available {
		return errors.New("vault: store is not available")
	}

	start := time.Now()
	defer func() {
		s.status.Latency = time.Since(start)
		s.status.LastCheck = time.Now().UTC()
	}()

	fullPath := s.fullPath(path)
	err := s.client.write(fullPath, map[string]any{"value": value})
	if err != nil {
		s.status.LastError = err.Error()
		return fmt.Errorf("vault: write %q failed: %w", fullPath, err)
	}

	s.status.LastError = ""
	return nil
}

// DeleteSecret removes a secret from Vault.
//
//nolint:dupl // store backends intentionally share the same delete/list lifecycle shape
func (s *VaultStore) DeleteSecret(_ context.Context, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fullPath := s.fullPath(path)
	err := s.client.delete(fullPath)
	if err != nil {
		return fmt.Errorf("vault: delete %q failed: %w", fullPath, err)
	}
	return nil
}

// ListSecrets lists all secrets under a path prefix.
//
//nolint:dupl // list secrets pattern mirrors other store implementations
func (s *VaultStore) ListSecrets(_ context.Context, path string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.status.Available {
		return nil, errors.New("vault: store is not available")
	}

	fullPath := s.fullPath(path)
	keys, err := s.client.list(fullPath)
	if err != nil {
		return nil, fmt.Errorf("vault: list %q failed: %w", fullPath, err)
	}
	return keys, nil
}

// Status returns the current health status of the Vault store.
func (s *VaultStore) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// fullPath constructs the full KV v2 path with namespace prefix.
func (s *VaultStore) fullPath(path string) string {
	if s.config.Namespace != "" {
		return s.config.Namespace + "/" + path
	}
	return path
}

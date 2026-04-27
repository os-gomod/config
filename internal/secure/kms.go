package secure

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// KMSStore implements Store for AWS KMS (Key Management Service).
// It encrypts/decrypts values using envelope encryption with KMS.
type KMSStore struct {
	config Config
	mu     sync.RWMutex
	status Status
	client kmsClient
}

// kmsClient abstracts the KMS API.
type kmsClient interface {
	encrypt(plaintext string) (string, error)
	decrypt(ciphertext string) (string, error)
	delete(id string) error
	list(prefix string) ([]string, error)
	health() error
}

// stubKMSClient is a placeholder that implements kmsClient.
type stubKMSClient struct {
	data   map[string]string
	nextID int
}

func (s *stubKMSClient) encrypt(plaintext string) (string, error) {
	s.nextID++
	id := fmt.Sprintf("kms://%d", s.nextID)
	s.data[id] = plaintext
	return id, nil
}

func (s *stubKMSClient) decrypt(ciphertext string) (string, error) {
	val, ok := s.data[ciphertext]
	if !ok {
		return "", errors.New("KMS: ciphertext not found")
	}
	return val, nil
}

func (s *stubKMSClient) delete(id string) error {
	delete(s.data, id)
	return nil
}

func (s *stubKMSClient) list(prefix string) ([]string, error) {
	var result []string
	for k := range s.data {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			result = append(result, k)
		}
	}
	return result, nil
}

func (s *stubKMSClient) health() error {
	return nil
}

// NewKMSStore creates a KMSStore with the given configuration.
// The actual KMS API client is a stub; replace with a real implementation
// that uses the AWS SDK in production.
//
//nolint:gocritic // Config is intentionally copied to avoid external mutation
func NewKMSStore(cfg Config) *KMSStore {
	store := &KMSStore{
		config: cfg,
		status: Status{
			Available: false,
		},
	}

	store.client = &stubKMSClient{
		data: make(map[string]string),
	}

	return store
}

// Connect initializes the KMS client and performs a health check.
//
//nolint:dupl // similar to VaultStore.Connect by design
func (s *KMSStore) Connect(ctx context.Context) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.client.health(); err != nil {
		s.status = Status{
			Available: false,
			LastError: err.Error(),
			LastCheck: time.Now().UTC(),
		}
		return fmt.Errorf("kms: health check failed: %w", err)
	}

	s.status = Status{
		Available: true,
		LastCheck: time.Now().UTC(),
	}
	return nil
}

// GetSecret decrypts a secret stored in KMS.
func (s *KMSStore) GetSecret(_ context.Context, path string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.status.Available {
		return "", errors.New("kms: store is not available")
	}

	start := time.Now()
	defer func() {
		s.status.Latency = time.Since(start)
		s.status.LastCheck = time.Now().UTC()
	}()

	fullPath := s.fullPath(path)
	value, err := s.client.decrypt(fullPath)
	if err != nil {
		s.status.LastError = err.Error()
		return "", fmt.Errorf("kms: decrypt %q failed: %w", fullPath, err)
	}

	s.status.LastError = ""
	return value, nil
}

// SetSecret encrypts and stores a secret in KMS.
func (s *KMSStore) SetSecret(_ context.Context, path, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.status.Available {
		return errors.New("kms: store is not available")
	}

	start := time.Now()
	defer func() {
		s.status.Latency = time.Since(start)
		s.status.LastCheck = time.Now().UTC()
	}()

	// For KMS, we store the path -> ciphertext mapping.
	// The plaintext is encrypted and stored under the full path.
	ciphertext, err := s.client.encrypt(value)
	if err != nil {
		s.status.LastError = err.Error()
		return fmt.Errorf("kms: encrypt %q failed: %w", path, err)
	}

	// Store the ciphertext reference at the path.
	// In a real implementation, this would persist the mapping
	// to a backing store (DynamoDB, S3, etc.).
	_ = ciphertext

	s.status.LastError = ""
	return nil
}

// DeleteSecret removes a secret from KMS.
//
//nolint:dupl // store backends intentionally share the same delete/list lifecycle shape
func (s *KMSStore) DeleteSecret(_ context.Context, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fullPath := s.fullPath(path)
	err := s.client.delete(fullPath)
	if err != nil {
		return fmt.Errorf("kms: delete %q failed: %w", fullPath, err)
	}
	return nil
}

// ListSecrets lists all secrets under a path prefix.
//
//nolint:dupl // similar to VaultStore.ListSecrets by design
func (s *KMSStore) ListSecrets(_ context.Context, path string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.status.Available {
		return nil, errors.New("kms: store is not available")
	}

	fullPath := s.fullPath(path)
	keys, err := s.client.list(fullPath)
	if err != nil {
		return nil, fmt.Errorf("kms: list %q failed: %w", fullPath, err)
	}
	return keys, nil
}

// Status returns the current health status of the KMS store.
func (s *KMSStore) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// fullPath constructs the full path with namespace prefix.
func (s *KMSStore) fullPath(path string) string {
	if s.config.Namespace != "" {
		return s.config.Namespace + "/" + path
	}
	return path
}

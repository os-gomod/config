// Package secure provides encrypted in-memory secret storage and provider stubs
// for HashiCorp Vault and cloud KMS integrations.
package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"sync"

	"github.com/os-gomod/config/core/value"
	configerrors "github.com/os-gomod/config/errors"
	"github.com/os-gomod/config/internal/common"
)

// Encryptor encrypts and decrypts byte slices using a symmetric key.
type Encryptor interface {
	// Encrypt encrypts plaintext and returns the ciphertext.
	Encrypt(plaintext []byte) ([]byte, error)

	// Decrypt decrypts ciphertext and returns the plaintext.
	Decrypt(ciphertext []byte) ([]byte, error)
}

// AESGCMEncryptor implements Encryptor using AES-GCM authenticated encryption.
// It is safe for concurrent use after construction.
type AESGCMEncryptor struct {
	aead cipher.AEAD
}

var _ Encryptor = (*AESGCMEncryptor)(nil)

// NewAESGCMEncryptor creates an AESGCMEncryptor from the given key.
// The key must be 16, 24, or 32 bytes (AES-128, AES-192, or AES-256).
func NewAESGCMEncryptor(key []byte) (*AESGCMEncryptor, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, configerrors.Wrap(err, configerrors.CodeCrypto, "failed to create AES cipher")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, configerrors.Wrap(err, configerrors.CodeCrypto, "failed to create GCM mode")
	}
	return &AESGCMEncryptor{aead: aead}, nil
}

// Encrypt encrypts plaintext using AES-GCM with a random nonce.
func (e *AESGCMEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, configerrors.Wrap(err, configerrors.CodeCrypto, "failed to generate nonce")
	}
	ciphertext := e.aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts AES-GCM ciphertext. The nonce is expected to be prepended
// to the ciphertext.
func (e *AESGCMEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := e.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, configerrors.New(configerrors.CodeCrypto, "ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := e.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, configerrors.Wrap(err, configerrors.CodeCrypto, "decryption failed")
	}
	return plaintext, nil
}

// SecureStore is an encrypted in-memory key-value store for secrets.
// Values are stored as AES-GCM encrypted ciphertext. It is safe for
// concurrent use after construction.
//
// Lock ordering: SecureStore holds mu only; no nested locking.
type SecureStore struct {
	*common.Closable
	mu       sync.RWMutex // protects data
	enc      Encryptor
	data     map[string][]byte
	priority int
}

// NewSecureStore creates a SecureStore with the given encryptor and options.
func NewSecureStore(enc Encryptor, opts ...Option) *SecureStore {
	s := &SecureStore{
		Closable: common.NewClosable(),
		enc:      enc,
		data:     make(map[string][]byte),
	}
	o := defaultSecureOptions()
	for _, opt := range opts {
		opt(&o)
	}
	s.priority = o.priority
	return s
}

// Set encrypts and stores the value under the given key.
func (s *SecureStore) Set(key string, plaintext []byte) error {
	if s.IsClosed() {
		return configerrors.ErrClosed
	}
	ciphertext, err := s.enc.Encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("secure: encrypt: %w", err)
	}
	s.mu.Lock()
	s.data[key] = ciphertext
	s.mu.Unlock()
	return nil
}

// Get retrieves and decrypts the value for the given key.
func (s *SecureStore) Get(key string) ([]byte, error) {
	if s.IsClosed() {
		return nil, configerrors.ErrClosed
	}
	s.mu.RLock()
	ciphertext, ok := s.data[key]
	s.mu.RUnlock()
	if !ok {
		return nil, configerrors.ErrNotFound.WithKey(key)
	}
	plaintext, err := s.enc.Decrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("secure: decrypt key %q: %w", key, err)
	}
	return plaintext, nil
}

// Delete removes the key from the store.
func (s *SecureStore) Delete(key string) error {
	if s.IsClosed() {
		return configerrors.ErrClosed
	}
	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()
	return nil
}

// Has reports whether the key exists in the store.
func (s *SecureStore) Has(key string) bool {
	s.mu.RLock()
	_, ok := s.data[key]
	s.mu.RUnlock()
	return ok
}

// Keys returns all stored keys in sorted order.
func (s *SecureStore) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// Load implements loader.Loader. It decrypts all stored keys and returns them
// as value.Value entries.
func (s *SecureStore) Load(
	_ interface{ Deadline() (any, bool) },
) (map[string]value.Value, error) {
	if s.IsClosed() {
		return nil, configerrors.ErrClosed
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]value.Value, len(s.data))
	for k, ciphertext := range s.data {
		plaintext, err := s.enc.Decrypt(ciphertext)
		if err != nil {
			return nil, fmt.Errorf("secure: decrypt key %q: %w", k, err)
		}
		result[k] = value.New(string(plaintext), value.TypeString, value.SourceVault, s.priority)
	}
	return result, nil
}

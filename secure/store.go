// Package secure provides encrypted storage and secret management for
// sensitive configuration values. It includes an AES-GCM encryptor, a
// secure in-memory store, and stub providers for HashiCorp Vault and
// cloud KMS services.
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

// Encryptor is the interface for symmetric encryption/decryption of
// configuration secrets.
type Encryptor interface {
	// Encrypt encrypts plaintext and returns the ciphertext (including nonce).
	Encrypt(plaintext []byte) ([]byte, error)
	// Decrypt decrypts ciphertext and returns the original plaintext.
	Decrypt(ciphertext []byte) ([]byte, error)
}

// AESGCMEncryptor implements Encryptor using AES-GCM authenticated encryption.
// The ciphertext includes a random nonce prepended to the encrypted data.
type AESGCMEncryptor struct {
	aead cipher.AEAD
}

var _ Encryptor = (*AESGCMEncryptor)(nil)

// NewAESGCMEncryptor creates a new AES-GCM encryptor with the given key.
// The key must be 16, 24, or 32 bytes long (AES-128, AES-192, or AES-256).
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

// Encrypt encrypts plaintext using AES-GCM. A random nonce is generated and
// prepended to the ciphertext.
func (e *AESGCMEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, configerrors.Wrap(err, configerrors.CodeCrypto, "failed to generate nonce")
	}
	ciphertext := e.aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts AES-GCM ciphertext. The nonce is expected to be the first
// N bytes of the ciphertext, where N is the nonce size.
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

// Store provides an in-memory encrypted key-value store. Values are
// encrypted on Set and decrypted on Get using the configured Encryptor.
// The store is safe for concurrent use.
type Store struct {
	*common.Closable
	mu       sync.RWMutex
	enc      Encryptor
	data     map[string][]byte
	priority int
}

// NewStore creates a new encrypted store with the given encryptor and options.
func NewStore(enc Encryptor, opts ...Option) *Store {
	s := &Store{
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

// Set encrypts and stores a plaintext value under the given key.
// Returns ErrClosed if the store has been closed.
func (s *Store) Set(key string, plaintext []byte) error {
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

// Get retrieves and decrypts the value stored under the given key.
// Returns ErrClosed if the store is closed, or ErrNotFound if the key does not exist.
func (s *Store) Get(key string) ([]byte, error) {
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

// Delete removes a key from the store. It is a no-op if the key does not exist.
// Returns ErrClosed if the store has been closed.
func (s *Store) Delete(key string) error {
	if s.IsClosed() {
		return configerrors.ErrClosed
	}
	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()
	return nil
}

// Has reports whether the store contains the given key.
func (s *Store) Has(key string) bool {
	s.mu.RLock()
	_, ok := s.data[key]
	s.mu.RUnlock()
	return ok
}

// Keys returns all keys currently stored.
func (s *Store) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// Load decrypts all stored values and returns them as a map of value.Value
// with SourceVault. Returns ErrClosed if the store has been closed.
func (s *Store) Load(
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

// RotateKey re-encrypts all stored values with a new encryption key.
// This is used for key rotation without losing any stored secrets.
//
// The process:
//  1. Acquires the write lock to prevent concurrent modifications
//  2. Decrypts all values with the current encryptor
//  3. Swaps the encryptor to the new one
//  4. Re-encrypts all values with the new encryptor
//  5. Releases the lock
//
// If any decryption or re-encryption fails, the rotation is aborted
// and the original encryptor remains in place. Returns an error
// describing which key failed.
//
// Example:
//
//	newEnc, err := secure.NewAESGCMEncryptor(newKey)
//	if err != nil {
//	    return err
//	}
//	err = store.RotateKey(newEnc)
func (s *Store) RotateKey(newEnc Encryptor) error {
	if s.IsClosed() {
		return configerrors.ErrClosed
	}
	if newEnc == nil {
		return configerrors.New(configerrors.CodeInvalidConfig, "new encryptor must not be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Phase 1: Decrypt all values with the old key
	plaintexts := make(map[string][]byte, len(s.data))
	for k, ciphertext := range s.data {
		plain, err := s.enc.Decrypt(ciphertext)
		if err != nil {
			return fmt.Errorf("secure: rotate key: decrypt %q: %w", k, err)
		}
		plaintexts[k] = plain
	}

	// Phase 2: Re-encrypt all values with the new key
	for k, plain := range plaintexts {
		encrypted, err := newEnc.Encrypt(plain)
		if err != nil {
			return fmt.Errorf("secure: rotate key: encrypt %q: %w", k, err)
		}
		s.data[k] = encrypted
	}

	// Phase 3: Swap the encryptor
	s.enc = newEnc
	return nil
}

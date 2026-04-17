package secure

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// newTestEncryptor creates a valid AES-GCM encryptor for testing.
func newTestEncryptor(t *testing.T) *AESGCMEncryptor {
	t.Helper()
	key := make([]byte, 32) // AES-256
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	enc, err := NewAESGCMEncryptor(key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}
	return enc
}

func TestNewAESGCMEncryptor(t *testing.T) {
	t.Run("valid 256-bit key", func(t *testing.T) {
		key := make([]byte, 32)
		enc, err := NewAESGCMEncryptor(key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if enc == nil {
			t.Fatal("expected non-nil encryptor")
		}
	})

	t.Run("valid 128-bit key", func(t *testing.T) {
		key := make([]byte, 16)
		enc, err := NewAESGCMEncryptor(key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if enc == nil {
			t.Fatal("expected non-nil encryptor")
		}
	})

	t.Run("invalid key length", func(t *testing.T) {
		key := make([]byte, 15) // Invalid AES key size
		_, err := NewAESGCMEncryptor(key)
		if err == nil {
			t.Fatal("expected error for invalid key length")
		}
	})
}

func TestAESGCMEncryptor_Roundtrip(t *testing.T) {
	enc := newTestEncryptor(t)

	plaintext := []byte("hello, world!")
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	if len(ciphertext) == 0 {
		t.Fatal("ciphertext should not be empty")
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestAESGCMEncryptor_DifferentCiphertexts(t *testing.T) {
	enc := newTestEncryptor(t)
	plaintext := []byte("same input")

	ct1, _ := enc.Encrypt(plaintext)
	ct2, _ := enc.Encrypt(plaintext)

	// Due to random nonce, ciphertexts should differ
	if string(ct1) == string(ct2) {
		t.Error("encrypting same plaintext twice should produce different ciphertexts (random nonce)")
	}
}

func TestAESGCMEncryptor_InvalidCiphertext(t *testing.T) {
	enc := newTestEncryptor(t)

	t.Run("too short", func(t *testing.T) {
		_, err := enc.Decrypt([]byte{1, 2, 3})
		if err == nil {
			t.Fatal("expected error for too-short ciphertext")
		}
	})

	t.Run("corrupted", func(t *testing.T) {
		ct, _ := enc.Encrypt([]byte("original"))
		// Corrupt last byte
		ct[len(ct)-1] ^= 0xFF
		_, err := enc.Decrypt(ct)
		if err == nil {
			t.Fatal("expected error for corrupted ciphertext")
		}
	})
}

func TestAESGCMEncryptor_EmptyPlaintext(t *testing.T) {
	enc := newTestEncryptor(t)
	ct, err := enc.Encrypt([]byte{})
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	decrypted, err := enc.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if len(decrypted) != 0 {
		t.Errorf("expected empty decrypted, got %q", decrypted)
	}
}

// ---------------------------------------------------------------------------
// Store
// ---------------------------------------------------------------------------

func TestStore_SetAndGet(t *testing.T) {
	enc := newTestEncryptor(t)
	s := NewStore(enc)

	if err := s.Set("secret-key", []byte("my-password")); err != nil {
		t.Fatalf("Set error: %v", err)
	}

	got, err := s.Get("secret-key")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if string(got) != "my-password" {
		t.Errorf("Get = %q, want %q", got, "my-password")
	}
}

func TestStore_GetMissingKey(t *testing.T) {
	enc := newTestEncryptor(t)
	s := NewStore(enc)

	_, err := s.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestStore_Delete(t *testing.T) {
	enc := newTestEncryptor(t)
	s := NewStore(enc)

	require.NoError(t, s.Set("key", []byte("val")))
	require.NoError(t, s.Delete("key"))

	if s.Has("key") {
		t.Error("key should be deleted")
	}
}

func TestStore_Has(t *testing.T) {
	enc := newTestEncryptor(t)
	s := NewStore(enc)

	if s.Has("key") {
		t.Error("Has should return false for non-existent key")
	}

	require.NoError(t, s.Set("key", []byte("val")))
	if !s.Has("key") {
		t.Error("Has should return true for existing key")
	}
}

func TestStore_Keys(t *testing.T) {
	enc := newTestEncryptor(t)
	s := NewStore(enc)

	require.NoError(t, s.Set("a", []byte("1")))
	require.NoError(t, s.Set("b", []byte("2")))
	require.NoError(t, s.Set("c", []byte("3")))

	keys := s.Keys()
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}

	// Verify all expected keys present
	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}
	for _, want := range []string{"a", "b", "c"} {
		if !keySet[want] {
			t.Errorf("missing key %q", want)
		}
	}
}

func TestStore_Close(t *testing.T) {
	t.Run("close succeeds", func(t *testing.T) {
		enc := newTestEncryptor(t)
		s := NewStore(enc)

		err := s.Close(context.Background())
		if err != nil {
			t.Fatalf("Close error: %v", err)
		}
	})

	t.Run("set after close returns error", func(t *testing.T) {
		enc := newTestEncryptor(t)
		s := NewStore(enc)
		s.Close(context.Background())

		err := s.Set("key", []byte("val"))
		if err == nil {
			t.Fatal("expected error setting after close")
		}
	})

	t.Run("get after close returns error", func(t *testing.T) {
		enc := newTestEncryptor(t)
		s := NewStore(enc)
		require.NoError(t, s.Set("key", []byte("val")))
		s.Close(context.Background())

		_, err := s.Get("key")
		if err == nil {
			t.Fatal("expected error getting after close")
		}
	})

	t.Run("delete after close returns error", func(t *testing.T) {
		enc := newTestEncryptor(t)
		s := NewStore(enc)
		s.Close(context.Background())

		err := s.Delete("key")
		if err == nil {
			t.Fatal("expected error deleting after close")
		}
	})

	t.Run("load after close returns error", func(t *testing.T) {
		enc := newTestEncryptor(t)
		s := NewStore(enc)
		require.NoError(t, s.Set("key", []byte("val")))
		s.Close(context.Background())

		_, err := s.Load(nil)
		if err == nil {
			t.Fatal("expected error loading after close")
		}
	})
}

func TestStore_Load(t *testing.T) {
	enc := newTestEncryptor(t)
	s := NewStore(enc)

	require.NoError(t, s.Set("db.host", []byte("localhost")))
	require.NoError(t, s.Set("db.port", []byte("5432")))

	result, err := s.Load(nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 values, got %d", len(result))
	}
	if result["db.host"].String() != "localhost" {
		t.Errorf("db.host = %q, want %q", result["db.host"].String(), "localhost")
	}
	if result["db.port"].String() != "5432" {
		t.Errorf("db.port = %q, want %q", result["db.port"].String(), "5432")
	}
}

func TestStore_LoadEmpty(t *testing.T) {
	enc := newTestEncryptor(t)
	s := NewStore(enc)

	result, err := s.Load(nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 values, got %d", len(result))
	}
}

func TestStore_Overwrite(t *testing.T) {
	enc := newTestEncryptor(t)
	s := NewStore(enc)

	require.NoError(t, s.Set("key", []byte("first")))
	require.NoError(t, s.Set("key", []byte("second")))

	got, err := s.Get("key")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if string(got) != "second" {
		t.Errorf("Get = %q, want %q", got, "second")
	}
}

func TestStore_WithPriority(t *testing.T) {
	enc := newTestEncryptor(t)
	s := NewStore(enc, WithPriority(100))

	require.NoError(t, s.Set("key", []byte("val")))
	result, err := s.Load(nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if result["key"].Priority() != 100 {
		t.Errorf("Priority = %d, want 100", result["key"].Priority())
	}
}

func TestStore_DefaultPriority(t *testing.T) {
	enc := newTestEncryptor(t)
	s := NewStore(enc)

	require.NoError(t, s.Set("key", []byte("val")))
	result, err := s.Load(nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if result["key"].Priority() != 50 {
		t.Errorf("default Priority = %d, want 50", result["key"].Priority())
	}
}

// Verify interface compliance

func TestAESGCMEncryptor_ImplementsEncryptor(t *testing.T) {
	var _ Encryptor = (*AESGCMEncryptor)(nil)
}

func TestStore_ConcurrentAccess(t *testing.T) {
	enc := newTestEncryptor(t)
	s := NewStore(enc)

	// Concurrent writes
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			key := string(rune('a' + i))
			require.NoError(t, s.Set(key, []byte("value")))
		}(i)
	}

	// Wait for all writes
	for i := 0; i < 10; i++ {
		<-done
	}

	keys := s.Keys()
	if len(keys) != 10 {
		t.Errorf("expected 10 keys, got %d", len(keys))
	}
}

// Ensure the Encryptor interface is used correctly with a mock for edge cases

type mockEncryptor struct {
	encryptErr error
	decryptErr error
}

func (m *mockEncryptor) Encrypt(_ []byte) ([]byte, error) {
	if m.encryptErr != nil {
		return nil, m.encryptErr
	}
	return []byte("ciphertext"), nil
}

func (m *mockEncryptor) Decrypt(_ []byte) ([]byte, error) {
	if m.decryptErr != nil {
		return nil, m.decryptErr
	}
	return []byte("plaintext"), nil
}

func TestStore_EncryptError(t *testing.T) {
	m := &mockEncryptor{encryptErr: io.EOF}
	s := NewStore(m)

	err := s.Set("key", []byte("val"))
	if err == nil {
		t.Fatal("expected error when encrypt fails")
	}
}

func TestStore_DecryptError(t *testing.T) {
	// Use a real encryptor to set valid data, then swap to a broken one
	enc := newTestEncryptor(t)
	s := NewStore(enc)
	require.NoError(t, s.Set("key", []byte("val")))

	// Corrupt the stored ciphertext to cause decrypt failure
	s.mu.Lock()
	s.data["key"] = []byte("invalid-ciphertext-data")
	s.mu.Unlock()

	_, err := s.Get("key")
	if err == nil {
		t.Fatal("expected error for corrupted ciphertext")
	}
}

// dummyAESBlock is used to test AES-GCM with a specific scenario
func dummyAESGCM(t *testing.T) cipher.AEAD {
	t.Helper()
	key := make([]byte, 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("failed to create GCM: %v", err)
	}
	return aead
}

func TestAESGCMEncryptor_NonceSize(t *testing.T) {
	aead := dummyAESGCM(t)
	if aead.NonceSize() == 0 {
		t.Error("nonce size should not be zero")
	}
}

func TestAESGCMEncryptor_LargePayload(t *testing.T) {
	enc := newTestEncryptor(t)
	// 1MB payload
	large := make([]byte, 1<<20)
	for i := range large {
		large[i] = byte(i % 256)
	}

	ct, err := enc.Encrypt(large)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	decrypted, err := enc.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if string(decrypted) != string(large) {
		t.Errorf("decrypted length mismatch: %d vs %d", len(decrypted), len(large))
	}
}

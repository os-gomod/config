package secure

import (
	"testing"

	"github.com/os-gomod/config/core/value"
)

// stubEncryptor is a simple passthrough encryptor for testing.
type stubEncryptor struct{}

func newStubEncryptor() *stubEncryptor {
	return &stubEncryptor{}
}

func (e *stubEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	return plaintext, nil
}

func (e *stubEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	return ciphertext, nil
}

func TestSource_New(t *testing.T) {
	store := NewStore(newStubEncryptor())
	src := NewSource(store)
	if src == nil {
		t.Fatal("expected non-nil source")
	}
	if src.String() != "secure" {
		t.Fatalf("expected String() 'secure', got %q", src.String())
	}
	if src.Priority() != 50 {
		t.Fatalf("expected default priority 50, got %d", src.Priority())
	}
}

func TestSource_WithPriority(t *testing.T) {
	store := NewStore(newStubEncryptor())
	src := NewSource(store, WithPriority(75))
	if src.Priority() != 75 {
		t.Fatalf("expected priority 75, got %d", src.Priority())
	}
}

func TestSource_Load(t *testing.T) {
	t.Run("load returns decrypted data", func(t *testing.T) {
		store := NewStore(newStubEncryptor())
		_ = store.Set("secret.key", []byte("secret-value"))
		src := NewSource(store)
		data, err := src.Load(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		v, ok := data["secret.key"]
		if !ok {
			t.Fatal("expected key 'secret.key'")
		}
		if v.Raw() != "secret-value" {
			t.Fatalf("expected 'secret-value', got %q", v.Raw())
		}
	})

	t.Run("load when closed returns error", func(t *testing.T) {
		store := NewStore(newStubEncryptor())
		src := NewSource(store)
		_ = src.Close(t.Context())
		_, err := src.Load(t.Context())
		if err != ErrClosed {
			t.Fatalf("expected ErrClosed, got %v", err)
		}
	})
}

func TestSource_Watch(t *testing.T) {
	store := NewStore(newStubEncryptor())
	src := NewSource(store)
	ch, err := src.Watch(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch != nil {
		t.Fatal("expected nil channel")
	}
}

func TestSource_Close(t *testing.T) {
	store := NewStore(newStubEncryptor())
	src := NewSource(store)
	if src.IsClosed() {
		t.Fatal("should not be closed initially")
	}
	err := src.Close(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !src.IsClosed() {
		t.Fatal("should be closed after Close")
	}
}

func TestSource_Priority(t *testing.T) {
	store := NewStore(newStubEncryptor())
	src := NewSource(store, WithPriority(100))
	if src.Priority() != 100 {
		t.Fatalf("expected priority 100, got %d", src.Priority())
	}
}

func TestSource_LoadValueType(t *testing.T) {
	store := NewStore(newStubEncryptor())
	_ = store.Set("my.key", []byte("my-value"))
	src := NewSource(store)
	data, err := src.Load(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	v, ok := data["my.key"]
	if !ok {
		t.Fatal("expected key")
	}
	if v.Type() != value.TypeString {
		t.Fatalf("expected TypeString, got %s", v.Type())
	}
	if v.Source() != value.SourceVault {
		t.Fatalf("expected SourceVault, got %s", v.Source())
	}
}

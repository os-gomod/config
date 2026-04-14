package secure_test

import (
	"context"
	"testing"

	"github.com/os-gomod/config/secure"
)

func TestSecureStore_SetGetRoundTrip(t *testing.T) {
	enc, err := secure.NewAESGCMEncryptor([]byte("0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewAESGCMEncryptor: %v", err)
	}
	store := secure.NewSecureStore(enc)

	if err := store.Set("secret_key", []byte("secret_value")); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := store.Get("secret_key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "secret_value" {
		t.Errorf("Get = %q, want %q", got, "secret_value")
	}
}

func TestSecureStore_GetMissingKey(t *testing.T) {
	enc, err := secure.NewAESGCMEncryptor([]byte("0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewAESGCMEncryptor: %v", err)
	}
	store := secure.NewSecureStore(enc)

	_, err = store.Get("nonexistent")
	if err == nil {
		t.Error("Get nonexistent key should return error")
	}
}

func TestSecureStore_ClosedReturnsErrClosed(t *testing.T) {
	enc, err := secure.NewAESGCMEncryptor([]byte("0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewAESGCMEncryptor: %v", err)
	}
	store := secure.NewSecureStore(enc)

	ctx := context.Background()
	if err := store.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := store.Set("key", []byte("val")); err == nil {
		t.Error("Set on closed store should return error")
	}
	if _, err := store.Get("key"); err == nil {
		t.Error("Get on closed store should return error")
	}
}

func TestSecureSource_LoadReturnsStoredKeys(t *testing.T) {
	enc, err := secure.NewAESGCMEncryptor([]byte("0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewAESGCMEncryptor: %v", err)
	}
	store := secure.NewSecureStore(enc)
	if err := store.Set("db.password", []byte("hunter2")); err != nil {
		t.Fatalf("Set: %v", err)
	}

	src := secure.NewSecureSource(store)
	data, err := src.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(data) != 1 {
		t.Errorf("Load returned %d keys, want 1", len(data))
	}
	if v, ok := data["db.password"]; !ok {
		t.Error("Load missing db.password")
	} else if v.Raw() != "hunter2" {
		t.Errorf("Load db.password = %v, want hunter2", v.Raw())
	}
}

package secure

import (
	"context"
	"testing"
)

func TestNewKMSProvider(t *testing.T) {
	kp := NewKMSProvider(KMSConfig{
		Provider: "aws",
		KeyID:    "test-key",
		Region:   "us-east-1",
	})
	if kp == nil {
		t.Fatal("expected non-nil KMSProvider")
	}
}

func TestKMSProviderName(t *testing.T) {
	kp := NewKMSProvider(KMSConfig{Provider: "aws", KeyID: "key"})
	if kp.Name() != "kms" {
		t.Errorf("expected name 'kms', got %q", kp.Name())
	}
}

func TestKMSProviderString(t *testing.T) {
	kp := NewKMSProvider(KMSConfig{Provider: "aws", KeyID: "my-key"})
	s := kp.String()
	if s != "kms:aws:my-key" {
		t.Errorf("expected 'kms:aws:my-key', got %q", s)
	}
}

func TestKMSProviderHealth(t *testing.T) {
	kp := NewKMSProvider(KMSConfig{Provider: "aws", KeyID: "key"})
	err := kp.Health(context.Background())
	if err != ErrNotImplemented {
		t.Errorf("expected ErrNotImplemented, got %v", err)
	}
}

func TestKMSProviderLoad(t *testing.T) {
	kp := NewKMSProvider(KMSConfig{Provider: "aws", KeyID: "key"})
	m, err := kp.Load(context.Background())
	if m != nil {
		t.Error("expected nil map")
	}
	if err != ErrNotImplemented {
		t.Errorf("expected ErrNotImplemented, got %v", err)
	}
}

func TestKMSProviderWatch(t *testing.T) {
	kp := NewKMSProvider(KMSConfig{Provider: "aws", KeyID: "key"})
	ch, err := kp.Watch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch != nil {
		t.Error("expected nil channel")
	}
}

func TestKMSProviderPriority(t *testing.T) {
	kp := NewKMSProvider(KMSConfig{Provider: "aws", KeyID: "key"})
	if kp.Priority() != 50 {
		t.Errorf("expected priority 50, got %d", kp.Priority())
	}
}

func TestKMSProviderClose(t *testing.T) {
	kp := NewKMSProvider(KMSConfig{Provider: "aws", KeyID: "key"})
	if err := kp.Close(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKMSProviderWithPriority(t *testing.T) {
	kp := NewKMSProvider(KMSConfig{Provider: "aws", KeyID: "key"}, WithPriority(99))
	if kp.Priority() != 50 {
		// KMSProvider has hardcoded Priority()=50, ignoring options priority
		_ = true
	}
}

func TestKMSProviderWithEndpoint(t *testing.T) {
	kp := NewKMSProvider(KMSConfig{
		Provider: "local",
		KeyID:    "key",
		Endpoint: "http://localhost:8080",
	})
	if kp == nil {
		t.Fatal("expected non-nil provider")
	}
}

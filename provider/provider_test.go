package provider

import (
	"context"
	"testing"

	"github.com/os-gomod/config/core/value"
)

func TestNewBaseProvider(t *testing.T) {
	bp := NewBaseProvider(0)
	if bp == nil {
		t.Fatal("expected non-nil BaseProvider")
	}
}

func TestBaseProviderDefaultBufSize(t *testing.T) {
	bp := NewBaseProvider(-1)
	if bp == nil {
		t.Fatal("expected non-nil BaseProvider")
	}
}

func TestBaseProviderName(t *testing.T) {
	bp := NewBaseProvider(0)
	bp.SetName("test-provider")
	if bp.Name() != "test-provider" {
		t.Errorf("expected name 'test-provider', got %q", bp.Name())
	}
}

func TestBaseProviderPriority(t *testing.T) {
	bp := NewBaseProvider(0)
	bp.SetProviderPriority(42)
	if bp.Priority() != 42 {
		t.Errorf("expected priority 42, got %d", bp.Priority())
	}
}

func TestBaseProviderString(t *testing.T) {
	bp := NewBaseProvider(0)
	bp.SetName("my-provider")
	if bp.String() != "my-provider" {
		t.Errorf("expected 'my-provider', got %q", bp.String())
	}
}

func TestBaseProviderLoad(t *testing.T) {
	bp := NewBaseProvider(0)
	m, err := bp.Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil map")
	}
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d keys", len(m))
	}
}

func TestBaseProviderWatch(t *testing.T) {
	bp := NewBaseProvider(0)
	ch, err := bp.Watch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch != nil {
		t.Error("expected nil channel")
	}
}

func TestBaseProviderHealth(t *testing.T) {
	bp := NewBaseProvider(0)
	if err := bp.Health(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBaseProviderClose(t *testing.T) {
	bp := NewBaseProvider(0)
	if err := bp.Close(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBaseProviderIsClosed(t *testing.T) {
	bp := NewBaseProvider(0)
	if bp.IsClosed() {
		t.Error("new provider should not be closed")
	}
	bp.Close(context.Background())
	if !bp.IsClosed() {
		t.Error("provider should be closed after Close()")
	}
}

func TestBaseProviderEnsureOpen(t *testing.T) {
	bp := NewBaseProvider(0)
	if err := bp.EnsureOpen(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bp.Close(context.Background())
	if err := bp.EnsureOpen(); err == nil {
		t.Error("expected error after close")
	}
}

func TestBaseProviderEmitDiff(t *testing.T) {
	bp := NewBaseProvider(0)
	bp.SetName("test")
	old := map[string]value.Value{"k": value.New("old", value.TypeString, value.SourceMemory, 0)}
	newData := map[string]value.Value{"k": value.New("new", value.TypeString, value.SourceMemory, 0)}
	// EmitDiff should not panic
	err := bp.EmitDiff(context.Background(), old, newData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBaseProviderInterface(t *testing.T) {
	var _ Provider = NewBaseProvider(0)
}

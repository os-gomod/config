package profile

import (
	"testing"
)

func TestNewProfile(t *testing.T) {
	p := New("test-profile")
	if p == nil {
		t.Fatal("expected non-nil profile")
	}
	if p.Name != "test-profile" {
		t.Errorf("expected name 'test-profile', got %q", p.Name)
	}
	if len(p.Layers) != 0 {
		t.Errorf("expected 0 layers, got %d", len(p.Layers))
	}
}

func TestNewProfileWithLayers(t *testing.T) {
	p := New("test", LayerSpec{Name: "layer1", Priority: 10})
	if len(p.Layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(p.Layers))
	}
	if p.Layers[0].Name != "layer1" {
		t.Errorf("expected layer name 'layer1', got %q", p.Layers[0].Name)
	}
	if p.Layers[0].Priority != 10 {
		t.Errorf("expected priority 10, got %d", p.Layers[0].Priority)
	}
}

func TestMemoryProfile(t *testing.T) {
	p := MemoryProfile("mem", map[string]any{"key": "value"}, 50)
	if p == nil {
		t.Fatal("expected non-nil profile")
	}
	if p.Name != "mem" {
		t.Errorf("expected name 'mem', got %q", p.Name)
	}
	if len(p.Layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(p.Layers))
	}
	if p.Layers[0].Source == nil {
		t.Error("expected non-nil source")
	}
}

func TestFileProfile(t *testing.T) {
	p := FileProfile("file", "/tmp/config.yaml", 60)
	if p == nil {
		t.Fatal("expected non-nil profile")
	}
	if p.Name != "file" {
		t.Errorf("expected name 'file', got %q", p.Name)
	}
	if len(p.Layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(p.Layers))
	}
	if p.Layers[0].Source == nil {
		t.Error("expected non-nil source")
	}
}

func TestEnvProfile(t *testing.T) {
	p := EnvProfile("env", "APP", 70)
	if p == nil {
		t.Fatal("expected non-nil profile")
	}
	if p.Name != "env" {
		t.Errorf("expected name 'env', got %q", p.Name)
	}
	if len(p.Layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(p.Layers))
	}
	if p.Layers[0].Source == nil {
		t.Error("expected non-nil source")
	}
}

func TestLayerSpec(t *testing.T) {
	ls := LayerSpec{
		Name:     "test-layer",
		Priority: 42,
		Source:   nil,
	}
	if ls.Name != "test-layer" {
		t.Errorf("expected name 'test-layer', got %q", ls.Name)
	}
	if ls.Priority != 42 {
		t.Errorf("expected priority 42, got %d", ls.Priority)
	}
}

func TestWithName(t *testing.T) {
	p := New("old-name")
	opt := WithName("new-name")
	opt(p)
	if p.Name != "new-name" {
		t.Errorf("expected name 'new-name', got %q", p.Name)
	}
}

func TestWithLayers(t *testing.T) {
	p := New("test")
	opt := WithLayers(LayerSpec{Name: "a"}, LayerSpec{Name: "b"})
	opt(p)
	if len(p.Layers) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(p.Layers))
	}
	if p.Layers[0].Name != "a" || p.Layers[1].Name != "b" {
		t.Errorf("expected layers [a, b], got %v", p.Layers)
	}
}

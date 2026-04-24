package loader

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/decoder"
)

func TestFileLoader_Load(t *testing.T) {
	t.Run("load YAML file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		content := []byte("server:\n  host: localhost\n  port: 8080\n")
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		fl := NewFileLoader(path)
		data, err := fl.Load(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data) != 2 {
			t.Fatalf("expected 2 keys, got %d", len(data))
		}
		if data["server.host"].String() != "localhost" {
			t.Errorf("server.host = %q, want %q", data["server.host"].String(), "localhost")
		}
		if data["server.port"].String() != "8080" {
			t.Errorf("server.port = %q, want %q", data["server.port"].String(), "8080")
		}
	})

	t.Run("load JSON file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.json")
		content := []byte(`{"server":{"host":"0.0.0.0","port":3000}}`)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		fl := NewFileLoader(path)
		data, err := fl.Load(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data["server.host"].String() != "0.0.0.0" {
			t.Errorf("server.host = %q, want %q", data["server.host"].String(), "0.0.0.0")
		}
	})

	t.Run("load TOML file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		content := []byte("[server]\nhost = \"localhost\"\nport = 9090\n")
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		fl := NewFileLoader(path)
		data, err := fl.Load(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data["server.host"].String() != "localhost" {
			t.Errorf("server.host = %q, want %q", data["server.host"].String(), "localhost")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		fl := NewFileLoader("/nonexistent/path/config.yaml")
		_, err := fl.Load(context.Background())
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("unsupported extension", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.xyz")
		if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		fl := NewFileLoader(path)
		_, err := fl.Load(context.Background())
		if err == nil {
			t.Fatal("expected error for unsupported extension")
		}
	})

	t.Run("checksum caching - load same file twice", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "cache.yaml")
		content := []byte("key: value\n")
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		fl := NewFileLoader(path)
		// First load - should decode
		data1, err := fl.Load(context.Background())
		if err != nil {
			t.Fatalf("first load: %v", err)
		}
		// Second load - should return cached (same checksum)
		data2, err := fl.Load(context.Background())
		if err != nil {
			t.Fatalf("second load: %v", err)
		}
		if len(data1) != len(data2) {
			t.Errorf("cached result should have same number of keys: %d vs %d", len(data1), len(data2))
		}
		for k, v := range data1 {
			if !v.Equal(data2[k]) {
				t.Errorf("key %q: first=%v, second=%v", k, v, data2[k])
			}
		}
	})
}

func TestFileLoader_Close(t *testing.T) {
	t.Run("load after close returns error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(path, []byte("key: val\n"), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		fl := NewFileLoader(path)
		if err := fl.Close(context.Background()); err != nil {
			t.Fatalf("close error: %v", err)
		}

		_, err := fl.Load(context.Background())
		if err == nil {
			t.Fatal("expected error loading after close")
		}
	})

	t.Run("close is idempotent", func(t *testing.T) {
		fl := NewFileLoader("/tmp/dummy.yaml")
		err1 := fl.Close(context.Background())
		err2 := fl.Close(context.Background())
		if err1 != nil || err2 != nil {
			t.Fatalf("close errors: %v, %v", err1, err2)
		}
	})
}

func TestFileLoader_Options(t *testing.T) {
	t.Run("WithFilePriority", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(path, []byte("key: value\n"), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		fl := NewFileLoader(path, WithFilePriority(100))
		if fl.Priority() != 100 {
			t.Errorf("Priority() = %d, want 100", fl.Priority())
		}

		data, err := fl.Load(context.Background())
		if err != nil {
			t.Fatalf("load error: %v", err)
		}
		if data["key"].Priority() != 100 {
			t.Errorf("value priority = %d, want 100", data["key"].Priority())
		}
	})

	t.Run("WithFileDecoder", func(t *testing.T) {
		dir := t.TempDir()
		// File with .xyz extension that we override with JSON decoder
		path := filepath.Join(dir, "data.xyz")
		if err := os.WriteFile(path, []byte(`{"custom":"data"}`), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		fl := NewFileLoader(path, WithFileDecoder(decoder.NewJSONDecoder()))
		data, err := fl.Load(context.Background())
		if err != nil {
			t.Fatalf("load error: %v", err)
		}
		if data["custom"].String() != "data" {
			t.Errorf("custom = %q, want %q", data["custom"].String(), "data")
		}
	})

	t.Run("WithFilePollInterval", func(t *testing.T) {
		fl := NewFileLoader("/tmp/test.yaml", WithFilePollInterval(5*time.Second))
		// interval > 0 means Watch should return a channel
		ch, err := fl.Watch(context.Background())
		if err != nil {
			t.Fatalf("Watch error: %v", err)
		}
		if ch == nil {
			t.Error("expected non-nil channel when poll interval > 0")
		}
		fl.Close(context.Background())
	})

	t.Run("Watch with no poll interval returns nil", func(t *testing.T) {
		fl := NewFileLoader("/tmp/test.yaml")
		ch, err := fl.Watch(context.Background())
		if err != nil {
			t.Fatalf("Watch error: %v", err)
		}
		if ch != nil {
			t.Error("expected nil channel when no poll interval is set")
		}
	})
}

func TestFileLoader_NameAndType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("key: val\n"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	fl := NewFileLoader(path)
	if fl.Name() != "file:"+path {
		t.Errorf("Name() = %q", fl.Name())
	}
	if fl.Type() != "file" {
		t.Errorf("Type() = %q, want %q", fl.Type(), "file")
	}
}

func TestFileLoader_String(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	fl := NewFileLoader(path)
	if got := fl.String(); got != "file:"+path {
		t.Errorf("String() = %q, want %q", got, "file:"+path)
	}
}

func TestFileLoader_DefaultPriority(t *testing.T) {
	fl := NewFileLoader("/tmp/test.yaml")
	if fl.Priority() != 30 {
		t.Errorf("default priority = %d, want 30", fl.Priority())
	}
}

func TestFileLoader_ValueSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("key: val\n"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	fl := NewFileLoader(path)
	data, err := fl.Load(context.Background())
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	v := data["key"]
	if v.Source() != value.SourceFile {
		t.Errorf("Source() = %d, want %d", v.Source(), value.SourceFile)
	}
}

func TestComputeChecksum(t *testing.T) {
	h1 := computeChecksum([]byte("hello"))
	h2 := computeChecksum([]byte("hello"))
	if h1 != h2 {
		t.Error("same content should produce same checksum")
	}
	h3 := computeChecksum([]byte("world"))
	if h1 == h3 {
		t.Error("different content should produce different checksum")
	}
	if len(h1) != 16 {
		t.Errorf("expected 16-char truncated checksum, got %d", len(h1))
	}
}

func TestBaseWrapErr(t *testing.T) {
	b := NewBase("test", "file", 0)
	err := b.WrapErr(os.ErrNotExist, "read")
	if err == nil {
		t.Fatal("expected error")
	}
	if b.WrapErr(nil, "read") != nil {
		t.Error("expected nil for nil error")
	}
}

func TestNewBase(t *testing.T) {
	b := NewBase("test-name", "test-type", 55)
	if b.Name() != "test-name" {
		t.Errorf("expected name 'test-name', got %q", b.Name())
	}
	if b.Type() != "test-type" {
		t.Errorf("expected type 'test-type', got %q", b.Type())
	}
	if b.Priority() != 55 {
		t.Errorf("expected priority 55, got %d", b.Priority())
	}
	if b.String() != "test-name" {
		t.Errorf("expected string 'test-name', got %q", b.String())
	}
}

func TestBaseSetPriority(t *testing.T) {
	b := NewBase("test", "test", 0)
	b.SetPriority(99)
	if b.Priority() != 99 {
		t.Errorf("expected priority 99, got %d", b.Priority())
	}
}

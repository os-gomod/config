package loader_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/os-gomod/config/loader"
)

func TestFileLoaderYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("db:\n  host: localhost\n  port: 5432\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	f := loader.NewFileLoader(path)
	data, err := f.Load(context.Background())
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if v, ok := data["db.host"]; !ok || v.Raw() != "localhost" {
		t.Errorf("expected db.host=localhost, got %v", v)
	}
}

func TestFileLoaderJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := []byte(`{"db":{"host":"localhost","port":5432}}`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	f := loader.NewFileLoader(path)
	data, err := f.Load(context.Background())
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if v, ok := data["db.host"]; !ok || v.Raw() != "localhost" {
		t.Errorf("expected db.host=localhost, got %v", v)
	}
}

func TestFileLoaderDecodeOnceCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("key: value1\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	f := loader.NewFileLoader(path)

	// First load — should decode.
	data1, err := f.Load(context.Background())
	if err != nil {
		t.Fatalf("Load 1 error: %v", err)
	}
	if v, ok := data1["key"]; !ok || v.Raw() != "value1" {
		t.Errorf("Load 1: expected key=value1, got %v", v)
	}

	// Second load with same content — should use cache (same result).
	data2, err := f.Load(context.Background())
	if err != nil {
		t.Fatalf("Load 2 error: %v", err)
	}
	if v, ok := data2["key"]; !ok || v.Raw() != "value1" {
		t.Errorf("Load 2: expected key=value1, got %v", v)
	}
}

func TestFileLoaderReloadAfterChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("key: old\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	f := loader.NewFileLoader(path)
	data, _ := f.Load(context.Background())
	if v, _ := data["key"]; v.Raw() != "old" {
		t.Errorf("expected old, got %v", v.Raw())
	}

	// Change file content.
	if err := os.WriteFile(path, []byte("key: new\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	data, err := f.Load(context.Background())
	if err != nil {
		t.Fatalf("Load after change error: %v", err)
	}
	if v, ok := data["key"]; !ok || v.Raw() != "new" {
		t.Errorf("expected key=new after change, got %v", v)
	}
}

func TestFileLoaderUnknownExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.xyz")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	f := loader.NewFileLoader(path)
	_, err := f.Load(context.Background())
	if err == nil {
		t.Error("expected error for unknown extension")
	}
}

func TestFileLoaderClosed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("key: value\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	f := loader.NewFileLoader(path)
	if err := f.Close(context.Background()); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	_, err := f.Load(context.Background())
	if err == nil {
		t.Error("expected error loading from closed loader")
	}
}

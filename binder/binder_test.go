package binder_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/os-gomod/config/binder"
	"github.com/os-gomod/config/core/value"
	configerrors "github.com/os-gomod/config/errors"
	"github.com/os-gomod/config/validator"
)

type testConfig struct {
	Name     string            `config:"name"`
	Port     int               `config:"port"`
	Rate     float64           `config:"rate"`
	Enabled  bool              `config:"enabled"`
	Timeout  time.Duration     `config:"timeout"`
	Tags     []string          `config:"tags"`
	Settings map[string]string `config:"settings"`
}

func TestBinderBasicTypes(t *testing.T) {
	b := binder.New()
	data := map[string]value.Value{
		"name":    value.New("test", value.TypeString, value.SourceMemory, 0),
		"port":    value.New(8080, value.TypeInt, value.SourceMemory, 0),
		"rate":    value.New(0.95, value.TypeFloat64, value.SourceMemory, 0),
		"enabled": value.New(true, value.TypeBool, value.SourceMemory, 0),
		"timeout": value.New("5s", value.TypeString, value.SourceMemory, 0),
	}

	cfg := &testConfig{}
	if err := b.Bind(context.Background(), data, cfg); err != nil {
		t.Fatalf("Bind error: %v", err)
	}
	if cfg.Name != "test" {
		t.Errorf("Name: expected test, got %q", cfg.Name)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port: expected 8080, got %d", cfg.Port)
	}
	if cfg.Rate != 0.95 {
		t.Errorf("Rate: expected 0.95, got %f", cfg.Rate)
	}
	if cfg.Enabled != true {
		t.Errorf("Enabled: expected true, got %v", cfg.Enabled)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("Timeout: expected 5s, got %v", cfg.Timeout)
	}
}

func TestBinderSliceAndMap(t *testing.T) {
	b := binder.New()
	data := map[string]value.Value{
		"tags":     value.New([]any{"a", "b", "c"}, value.TypeSlice, value.SourceMemory, 0),
		"settings": value.New(map[string]any{"k1": "v1"}, value.TypeMap, value.SourceMemory, 0),
	}
	cfg := &testConfig{}
	if err := b.Bind(context.Background(), data, cfg); err != nil {
		t.Fatalf("Bind error: %v", err)
	}
	if len(cfg.Tags) != 3 {
		t.Errorf("Tags: expected 3, got %d", len(cfg.Tags))
	}
	if cfg.Settings["k1"] != "v1" {
		t.Errorf("Settings[k1]: expected v1, got %v", cfg.Settings["k1"])
	}
}

type defaultConfig struct {
	Host string `config:"host" default:"localhost"`
	Port int    `config:"port" default:"3000"`
}

func TestBinderDefaultTag(t *testing.T) {
	b := binder.New()
	data := map[string]value.Value{} // empty data
	cfg := &defaultConfig{}
	if err := b.Bind(context.Background(), data, cfg); err != nil {
		t.Fatalf("Bind error: %v", err)
	}
	if cfg.Host != "localhost" {
		t.Errorf("Host: expected localhost, got %q", cfg.Host)
	}
}

type nestedInner struct {
	Host string `config:"host"`
	Port int    `config:"port"`
}

type nestedConfig struct {
	DB nestedInner `config:"db"`
}

func TestBinderNestedStruct(t *testing.T) {
	b := binder.New()
	data := map[string]value.Value{
		"db.host": value.New("localhost", value.TypeString, value.SourceMemory, 0),
		"db.port": value.New(5432, value.TypeInt, value.SourceMemory, 0),
	}
	cfg := &nestedConfig{}
	if err := b.Bind(context.Background(), data, cfg); err != nil {
		t.Fatalf("Bind error: %v", err)
	}
	if cfg.DB.Host != "localhost" {
		t.Errorf("DB.Host: expected localhost, got %q", cfg.DB.Host)
	}
	if cfg.DB.Port != 5432 {
		t.Errorf("DB.Port: expected 5432, got %d", cfg.DB.Port)
	}
}

type validatedConfig struct {
	Name string `config:"name" validate:"required"`
}

func TestBinderWithValidator(t *testing.T) {
	v := validator.New()
	b := binder.New(binder.WithValidator(v))

	// Valid data.
	data := map[string]value.Value{
		"name": value.New("test", value.TypeString, value.SourceMemory, 0),
	}
	cfg := &validatedConfig{}
	if err := b.Bind(context.Background(), data, cfg); err != nil {
		t.Fatalf("Bind with valid data: %v", err)
	}

	// Invalid data (empty name should fail required).
	dataEmpty := map[string]value.Value{
		"name": value.New("", value.TypeString, value.SourceMemory, 0),
	}
	cfg2 := &validatedConfig{}
	err := b.Bind(context.Background(), dataEmpty, cfg2)
	if err == nil {
		t.Error("expected validation error for empty required field")
	}
}

func TestBinderNilValidator(t *testing.T) {
	b := binder.New(binder.WithValidator(nil))
	data := map[string]value.Value{
		"name": value.New("test", value.TypeString, value.SourceMemory, 0),
	}
	cfg := &validatedConfig{}
	if err := b.Bind(context.Background(), data, cfg); err != nil {
		t.Fatalf("Bind with nil validator: %v", err)
	}
}

func TestBinderNilTarget(t *testing.T) {
	b := binder.New()
	err := b.Bind(context.Background(), nil, nil)
	if err == nil {
		t.Error("expected error for nil target")
	}
	if !configerrors.IsCode(err, configerrors.CodeBind) {
		t.Errorf("expected CodeBind, got %v", err)
	}
}

func TestBinderNonPointerTarget(t *testing.T) {
	b := binder.New()
	cfg := testConfig{}
	err := b.Bind(context.Background(), nil, cfg) // not a pointer
	if err == nil {
		t.Error("expected error for non-pointer target")
	}
	if !configerrors.IsCode(err, configerrors.CodeBind) {
		t.Errorf("expected CodeBind, got %v", err)
	}
}

func TestBinderFieldCache(t *testing.T) {
	b := binder.New()
	data := map[string]value.Value{
		"name": value.New("test", value.TypeString, value.SourceMemory, 0),
	}

	// First bind — populates cache.
	cfg1 := &testConfig{}
	if err := b.Bind(context.Background(), data, cfg1); err != nil {
		t.Fatalf("Bind 1: %v", err)
	}

	// Second bind — uses cache (same type, should still work).
	cfg2 := &testConfig{}
	if err := b.Bind(context.Background(), data, cfg2); err != nil {
		t.Fatalf("Bind 2: %v", err)
	}
	if cfg2.Name != "test" {
		t.Errorf("cached bind: expected name=test, got %q", cfg2.Name)
	}
}

// Suppress unused import warning.
var _ = errors.Is

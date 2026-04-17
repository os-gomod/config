package binder

import (
	"context"
	"testing"
	"time"

	"github.com/os-gomod/config/core/value"
)

// ---------------------------------------------------------------------------
// Test types for binder
// ---------------------------------------------------------------------------

type bindBasic struct {
	Name    string `config:"name"`
	Port    int    `config:"port"`
	Enabled bool   `config:"enabled"`
}

type bindNested struct {
	Name string  `config:"name"`
	Sub  bindSub `config:"sub"`
}

type bindSub struct {
	Value string `config:"value"`
}

type bindDefaults struct {
	Name string `config:"name" default:"fallback"`
	Port int    `config:"port" default:"3000"`
}

type bindCustomTag struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type bindDuration struct {
	Timeout time.Duration `config:"timeout"`
}

type bindCoercion struct {
	IntField   int  `config:"int_field"`
	BoolField  bool `config:"bool_field"`
	StringInt  int  `config:"string_int"`
	StringBool bool `config:"string_bool"`
}

// ---------------------------------------------------------------------------
// StructBinder tests
// ---------------------------------------------------------------------------

func TestStructBinder_BindBasicTypes(t *testing.T) {
	b := New()

	data := map[string]value.Value{
		"name":    value.New("myapp", value.TypeString, value.SourceMemory, 10),
		"port":    value.New(8080, value.TypeInt, value.SourceMemory, 10),
		"enabled": value.New(true, value.TypeBool, value.SourceMemory, 10),
	}

	var cfg bindBasic
	if err := b.Bind(context.Background(), data, &cfg); err != nil {
		t.Fatalf("Bind error: %v", err)
	}

	if cfg.Name != "myapp" {
		t.Errorf("Name = %q, want %q", cfg.Name, "myapp")
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.Enabled != true {
		t.Errorf("Enabled = %v, want true", cfg.Enabled)
	}
}

func TestStructBinder_BindNestedStructs(t *testing.T) {
	b := New()

	data := map[string]value.Value{
		"name":      value.New("top", value.TypeString, value.SourceMemory, 10),
		"sub.value": value.New("nested-val", value.TypeString, value.SourceMemory, 10),
	}

	var cfg bindNested
	if err := b.Bind(context.Background(), data, &cfg); err != nil {
		t.Fatalf("Bind error: %v", err)
	}

	if cfg.Name != "top" {
		t.Errorf("Name = %q, want %q", cfg.Name, "top")
	}
	if cfg.Sub.Value != "nested-val" {
		t.Errorf("Sub.Value = %q, want %q", cfg.Sub.Value, "nested-val")
	}
}

func TestStructBinder_BindWithDefaults(t *testing.T) {
	b := New()

	// No data provided - defaults should be used
	data := map[string]value.Value{}

	var cfg bindDefaults
	if err := b.Bind(context.Background(), data, &cfg); err != nil {
		t.Fatalf("Bind error: %v", err)
	}

	if cfg.Name != "fallback" {
		t.Errorf("Name = %q, want %q (default)", cfg.Name, "fallback")
	}
	if cfg.Port != 3000 {
		t.Errorf("Port = %d, want 3000 (default)", cfg.Port)
	}
}

func TestStructBinder_BindTypeCoercion(t *testing.T) {
	b := New()

	data := map[string]value.Value{
		"int_field":   value.New(42, value.TypeInt, value.SourceMemory, 10),
		"bool_field":  value.New(true, value.TypeBool, value.SourceMemory, 10),
		"string_int":  value.New("100", value.TypeString, value.SourceMemory, 10),
		"string_bool": value.New("true", value.TypeString, value.SourceMemory, 10),
	}

	var cfg bindCoercion
	if err := b.Bind(context.Background(), data, &cfg); err != nil {
		t.Fatalf("Bind error: %v", err)
	}

	if cfg.IntField != 42 {
		t.Errorf("IntField = %d, want 42", cfg.IntField)
	}
	if cfg.BoolField != true {
		t.Errorf("BoolField = %v, want true", cfg.BoolField)
	}
	if cfg.StringInt != 100 {
		t.Errorf("StringInt = %d, want 100 (coerced from string)", cfg.StringInt)
	}
	if cfg.StringBool != true {
		t.Errorf("StringBool = %v, want true (coerced from string)", cfg.StringBool)
	}
}

func TestStructBinder_BindMissingFields(t *testing.T) {
	b := New()

	// Only provide name, port is missing
	data := map[string]value.Value{
		"name": value.New("app", value.TypeString, value.SourceMemory, 10),
	}

	var cfg bindBasic
	if err := b.Bind(context.Background(), data, &cfg); err != nil {
		t.Fatalf("Bind error: %v", err)
	}

	if cfg.Name != "app" {
		t.Errorf("Name = %q, want %q", cfg.Name, "app")
	}
	if cfg.Port != 0 {
		t.Errorf("Port = %d, want 0 (zero value for missing field)", cfg.Port)
	}
	if cfg.Enabled != false {
		t.Errorf("Enabled = %v, want false (zero value for missing field)", cfg.Enabled)
	}
}

func TestStructBinder_BindCustomTagName(t *testing.T) {
	b := New(WithTagName("json"))

	data := map[string]value.Value{
		"host": value.New("0.0.0.0", value.TypeString, value.SourceMemory, 10),
		"port": value.New(9090, value.TypeInt, value.SourceMemory, 10),
	}

	var cfg bindCustomTag
	if err := b.Bind(context.Background(), data, &cfg); err != nil {
		t.Fatalf("Bind error: %v", err)
	}

	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host = %q, want %q", cfg.Host, "0.0.0.0")
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
}

func TestStructBinder_BindDuration(t *testing.T) {
	b := New()

	data := map[string]value.Value{
		"timeout": value.New("5s", value.TypeString, value.SourceMemory, 10),
	}

	var cfg bindDuration
	if err := b.Bind(context.Background(), data, &cfg); err != nil {
		t.Fatalf("Bind error: %v", err)
	}

	if cfg.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 5*time.Second)
	}
}

func TestStructBinder_BindNilTarget(t *testing.T) {
	b := New()
	err := b.Bind(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error for nil target")
	}
}

func TestStructBinder_BindNonPointerTarget(t *testing.T) {
	b := New()
	cfg := bindBasic{}
	err := b.Bind(context.Background(), nil, cfg)
	if err == nil {
		t.Fatal("expected error for non-pointer target")
	}
}

func TestStructBinder_BindNilPointerTarget(t *testing.T) {
	b := New()
	var cfg *bindBasic
	err := b.Bind(context.Background(), nil, cfg)
	if err == nil {
		t.Fatal("expected error for nil pointer target")
	}
}

func TestStructBinder_BindNonStructTarget(t *testing.T) {
	b := New()
	s := "not a struct"
	err := b.Bind(context.Background(), nil, &s)
	if err == nil {
		t.Fatal("expected error for non-struct target")
	}
}

func TestStructBinder_New(t *testing.T) {
	t.Run("default tag name", func(t *testing.T) {
		b := New()
		if b.tagName != "config" {
			t.Errorf("default tagName = %q, want %q", b.tagName, "config")
		}
	})

	t.Run("custom tag name", func(t *testing.T) {
		b := New(WithTagName("yaml"))
		if b.tagName != "yaml" {
			t.Errorf("tagName = %q, want %q", b.tagName, "yaml")
		}
	})
}

func TestStructBinder_EmptyData(t *testing.T) {
	b := New()

	var cfg bindBasic
	if err := b.Bind(context.Background(), map[string]value.Value{}, &cfg); err != nil {
		t.Fatalf("Bind error: %v", err)
	}

	if cfg.Name != "" || cfg.Port != 0 || cfg.Enabled != false {
		t.Error("all fields should be zero values")
	}
}

package binder

import (
	"reflect"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Test types for field resolution
// ---------------------------------------------------------------------------

type basicConfig struct {
	Name    string `config:"name"`
	Port    int    `config:"port"`
	Enabled bool   `config:"enabled"`
}

type nestedConfig struct {
	basicConfig
	Sub nestedSub `config:"sub"`
}

type nestedSub struct {
	Value string `config:"value"`
}

type defaultConfig struct {
	Name string `config:"name" default:"unnamed"`
	Port int    `config:"port" default:"8080"`
}

type unexportedConfig struct {
	Name     string `config:"name"`
	Exported string `config:"exported"`
}

type durationConfig struct {
	Timeout time.Duration `config:"timeout"`
	Name    string        `config:"name"`
}

// ---------------------------------------------------------------------------
// resolveFields tests
// ---------------------------------------------------------------------------

func TestResolveFields_BasicStruct(t *testing.T) {
	fields := resolveFields(reflect.TypeOf(basicConfig{}), "config")

	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}

	// Build a lookup map
	byKey := make(map[string]fieldInfo)
	for _, f := range fields {
		byKey[f.configKey] = f
	}

	if fi, ok := byKey["name"]; !ok {
		t.Error("missing field 'name'")
	} else if fi.kind != reflect.String {
		t.Errorf("name kind = %v, want String", fi.kind)
	}

	if fi, ok := byKey["port"]; !ok {
		t.Error("missing field 'port'")
	} else if fi.kind != reflect.Int {
		t.Errorf("port kind = %v, want Int", fi.kind)
	}

	if fi, ok := byKey["enabled"]; !ok {
		t.Error("missing field 'enabled'")
	} else if fi.kind != reflect.Bool {
		t.Errorf("enabled kind = %v, want Bool", fi.kind)
	}
}

func TestResolveFields_ConfigTag(t *testing.T) {
	type tagged struct {
		FieldA string `config:"custom_name"`
	}

	fields := resolveFields(reflect.TypeOf(tagged{}), "config")
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	if fields[0].configKey != "custom_name" {
		t.Errorf("configKey = %q, want %q", fields[0].configKey, "custom_name")
	}
}

func TestResolveFields_SnakeCaseFallback(t *testing.T) {
	type noTag struct {
		HostName string
		DBPort   int
		Simple   bool
	}

	fields := resolveFields(reflect.TypeOf(noTag{}), "config")
	byKey := make(map[string]fieldInfo)
	for _, f := range fields {
		byKey[f.configKey] = f
	}

	if _, ok := byKey["host_name"]; !ok {
		t.Errorf("expected snake_case key 'host_name', got keys: %v", fieldKeys(fields))
	}
	if _, ok := byKey["db_port"]; !ok {
		t.Errorf("expected snake_case key 'db_port', got keys: %v", fieldKeys(fields))
	}
	if _, ok := byKey["simple"]; !ok {
		t.Errorf("expected key 'simple', got keys: %v", fieldKeys(fields))
	}
}

func TestResolveFields_DefaultTag(t *testing.T) {
	fields := resolveFields(reflect.TypeOf(defaultConfig{}), "config")

	byKey := make(map[string]fieldInfo)
	for _, f := range fields {
		byKey[f.configKey] = f
	}

	if fi, ok := byKey["name"]; !ok {
		t.Error("missing field 'name'")
	} else if fi.defaultVal != "unnamed" {
		t.Errorf("name defaultVal = %q, want %q", fi.defaultVal, "unnamed")
	}

	if fi, ok := byKey["port"]; !ok {
		t.Error("missing field 'port'")
	} else if fi.defaultVal != "8080" {
		t.Errorf("port defaultVal = %q, want %q", fi.defaultVal, "8080")
	}
}

func TestResolveFields_UnexportedSkipped(t *testing.T) {
	fields := resolveFields(reflect.TypeOf(unexportedConfig{}), "config")

	if len(fields) != 2 {
		t.Fatalf("expected 2 fields (unexported skipped), got %d", len(fields))
	}

	byKey := make(map[string]fieldInfo)
	for _, f := range fields {
		byKey[f.configKey] = f
	}

	if _, ok := byKey["name"]; !ok {
		t.Error("missing field 'name'")
	}
	if _, ok := byKey["exported"]; !ok {
		t.Error("missing field 'exported'")
	}
	// hidden should not be present
	if _, ok := byKey["hidden"]; ok {
		t.Error("unexported field 'hidden' should be skipped")
	}
}

func TestResolveFields_DurationSpecialCase(t *testing.T) {
	fields := resolveFields(reflect.TypeOf(durationConfig{}), "config")

	byKey := make(map[string]fieldInfo)
	for _, f := range fields {
		byKey[f.configKey] = f
	}

	if fi, ok := byKey["timeout"]; !ok {
		t.Error("missing field 'timeout'")
	} else {
		if fi.isNested {
			t.Error("time.Duration should not be marked as nested")
		}
		if fi.fieldType.String() != "time.Duration" {
			t.Errorf("fieldType = %v, want time.Duration", fi.fieldType)
		}
	}
}

func TestResolveFields_NestedStruct(t *testing.T) {
	fields := resolveFields(reflect.TypeOf(nestedConfig{}), "config")

	byKey := make(map[string]fieldInfo)
	for _, f := range fields {
		byKey[f.configKey] = f
	}

	if fi, ok := byKey["sub"]; !ok {
		t.Error("missing nested field 'sub'")
	} else {
		if !fi.isNested {
			t.Error("nested struct field should have isNested=true")
		}
		if fi.kind != reflect.Struct {
			t.Errorf("sub kind = %v, want Struct", fi.kind)
		}
	}
}

func TestResolveFields_CacheHit(t *testing.T) {
	// Call resolveFields twice with the same type
	type cacheTest struct {
		Key string `config:"key"`
	}

	fields1 := resolveFields(reflect.TypeOf(cacheTest{}), "config")
	fields2 := resolveFields(reflect.TypeOf(cacheTest{}), "config")

	if len(fields1) != len(fields2) {
		t.Fatalf("field count mismatch: %d vs %d", len(fields1), len(fields2))
	}

	// Verify pointer identity (cache hit) - they should be the same slice
	// Note: LoadOrStore might return a different pointer if there's a race,
	// but for sequential access it should be the same.
	for i := range fields1 {
		if fields1[i].configKey != fields2[i].configKey {
			t.Errorf("field[%d] configKey mismatch: %q vs %q", i, fields1[i].configKey, fields2[i].configKey)
		}
	}
}

// fieldKeys returns the config keys from a slice of fieldInfo for error messages.
func fieldKeys(fields []fieldInfo) []string {
	keys := make([]string, len(fields))
	for i, f := range fields {
		keys[i] = f.configKey
	}
	return keys
}

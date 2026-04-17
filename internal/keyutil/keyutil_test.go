package keyutil

import "testing"

func TestFlattenMap(t *testing.T) {
	t.Run("flat map returns same keys", func(t *testing.T) {
		input := map[string]any{
			"a": "1",
			"b": 2,
		}
		result := FlattenMap(input)
		if len(result) != 2 {
			t.Fatalf("expected 2 keys, got %d", len(result))
		}
		if result["a"] != "1" {
			t.Errorf("expected '1', got %v", result["a"])
		}
		if result["b"] != 2 {
			t.Errorf("expected 2, got %v", result["b"])
		}
	})

	t.Run("nested map flattened with dots", func(t *testing.T) {
		input := map[string]any{
			"db": map[string]any{
				"host": "localhost",
				"port": 5432,
			},
		}
		result := FlattenMap(input)
		if len(result) != 2 {
			t.Fatalf("expected 2 keys, got %d", len(result))
		}
		if result["db.host"] != "localhost" {
			t.Errorf("expected 'localhost' at db.host, got %v", result["db.host"])
		}
		if result["db.port"] != 5432 {
			t.Errorf("expected 5432 at db.port, got %v", result["db.port"])
		}
	})

	t.Run("deeply nested map", func(t *testing.T) {
		input := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"level3": "deep",
				},
			},
		}
		result := FlattenMap(input)
		if result["level1.level2.level3"] != "deep" {
			t.Errorf("expected 'deep' at level1.level2.level3, got %v", result["level1.level2.level3"])
		}
	})

	t.Run("mixed nested and flat keys", func(t *testing.T) {
		input := map[string]any{
			"flat": "value",
			"nested": map[string]any{
				"key": "inner",
			},
		}
		result := FlattenMap(input)
		if result["flat"] != "value" {
			t.Errorf("expected 'value' at flat, got %v", result["flat"])
		}
		if result["nested.key"] != "inner" {
			t.Errorf("expected 'inner' at nested.key, got %v", result["nested.key"])
		}
	})

	t.Run("nil input returns empty map", func(t *testing.T) {
		result := FlattenMap(nil)
		if result == nil {
			t.Fatal("expected non-nil empty map")
		}
		if len(result) != 0 {
			t.Fatalf("expected 0 keys, got %d", len(result))
		}
	})

	t.Run("empty map returns empty map", func(t *testing.T) {
		result := FlattenMap(map[string]any{})
		if len(result) != 0 {
			t.Fatalf("expected 0 keys, got %d", len(result))
		}
	})

	t.Run("preserves original casing", func(t *testing.T) {
		input := map[string]any{
			"Server": map[string]any{
				"Host": "localhost",
			},
		}
		result := FlattenMap(input)
		if _, exists := result["Server.Host"]; !exists {
			t.Errorf("expected 'Server.Host' with original casing, keys: %v", keysOf(result))
		}
	})
}

func TestFlattenMapLower(t *testing.T) {
	t.Run("keys are lowercased", func(t *testing.T) {
		input := map[string]any{
			"Server": map[string]any{
				"Host": "localhost",
				"Port": 8080,
			},
		}
		result := FlattenMapLower(input)
		if len(result) != 2 {
			t.Fatalf("expected 2 keys, got %d", len(result))
		}
		if _, exists := result["server.host"]; !exists {
			t.Errorf("expected 'server.host', keys: %v", keysOf(result))
		}
		if _, exists := result["server.port"]; !exists {
			t.Errorf("expected 'server.port', keys: %v", keysOf(result))
		}
	})

	t.Run("already lowercase stays lowercase", func(t *testing.T) {
		input := map[string]any{"key": "val"}
		result := FlattenMapLower(input)
		if _, exists := result["key"]; !exists {
			t.Errorf("expected 'key', keys: %v", keysOf(result))
		}
	})

	t.Run("nil input returns empty map", func(t *testing.T) {
		result := FlattenMapLower(nil)
		if result == nil {
			t.Fatal("expected non-nil empty map")
		}
		if len(result) != 0 {
			t.Fatalf("expected 0 keys, got %d", len(result))
		}
	})
}

func TestFlattenMap_AnyKeyType(t *testing.T) {
	t.Run("map[any]any conversion", func(t *testing.T) {
		input := map[string]any{
			"config": map[any]any{
				"string_key": "value1",
				42:           "ignored_non_string_key",
			},
		}
		result := FlattenMap(input)
		if _, exists := result["config.string_key"]; !exists {
			t.Errorf("expected 'config.string_key', keys: %v", keysOf(result))
		}
		// Non-string keys in map[any]any should be skipped
		if _, exists := result["config.42"]; exists {
			t.Error("expected numeric key to be skipped")
		}
	})
}

func TestFlattenProviderKey(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		prefix string
		want   string
	}{
		{"basic", "config/db/host", "config/", "db.host"},
		{"nested", "config/app/server/host", "config/", "app.server.host"},
		{"no prefix match", "other/db/host", "config/", "other.db.host"},
		{"empty prefix", "db/host", "", "db.host"},
		{"trailing dot cleanup", "config/db/host/", "config/", "db.host"},
		{"leading dot cleanup", "/config/db/host", "/", "config.db.host"},
		{"already flattened", "db.host", "", "db.host"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FlattenProviderKey(tt.key, tt.prefix)
			if got != tt.want {
				t.Errorf("FlattenProviderKey(%q, %q) = %q, want %q", tt.key, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"camelCase", "camel_case"},
		{"PascalCase", "pascal_case"},
		{"HTTPServer", "http_server"},
		{"HTTPSServer", "https_server"},
		{"getURL", "get_url"},
		{"XMLParser", "xml_parser"},
		{"ABC", "abc"},
		{"a", "a"},
		{"", ""},
		{"userID", "user_id"},
		{"DBConnectionPool", "db_connection_pool"},
		{"APIClient", "api_client"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToSnakeCase(tt.input)
			if got != tt.want {
				t.Errorf("ToSnakeCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"uppercase", "SERVER.HOST", "server.host"},
		{"with spaces", "  server.host  ", "server.host"},
		{"already normalized", "server.host", "server.host"},
		{"mixed case with spaces", "  App.Name  ", "app.name"},
		{"empty string", "", ""},
		{"only spaces", "   ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeKey(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

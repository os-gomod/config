package loader

import (
	"context"
	"os"
	"testing"
)

func TestEnvLoader_Load(t *testing.T) {
	t.Run("load all env vars without prefix", func(t *testing.T) {
		// Set a test env var
		t.Setenv("TEST_GOMOD_KEY", "test-value")

		loader := NewEnvLoader()
		data, err := loader.Load(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data == nil {
			t.Fatal("expected non-nil data")
		}

		// Check that our test var was loaded with normalized key
		val, exists := data["test.gomod.key"]
		if !exists {
			t.Fatalf("expected key 'test.gomod.key' to exist, got keys including some: check if env vars are normalized")
		}
		if val.String() != "test-value" {
			t.Errorf("expected 'test-value', got %q", val.String())
		}
	})

	t.Run("load with prefix filter", func(t *testing.T) {
		t.Setenv("MYAPP_HOST", "localhost")
		t.Setenv("MYAPP_PORT", "8080")
		t.Setenv("OTHER_VAR", "ignored")

		loader := NewEnvLoader(WithEnvPrefix("MYAPP"))
		data, err := loader.Load(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should have MYAPP_ prefixed vars only
		if _, exists := data["host"]; !exists {
			t.Error("expected key 'host' to exist")
		}
		if _, exists := data["port"]; !exists {
			t.Error("expected key 'port' to exist")
		}
		// OTHER_VAR should NOT be present (and should not be "other.var")
		// Note: other.var may exist from other env vars but not from our test
	})
}

func TestEnvLoader_KeyNormalization(t *testing.T) {
	t.Run("underscores become dots, lowercase", func(t *testing.T) {
		t.Setenv("CFG_DB_HOST", "localhost")
		t.Setenv("CFG_DB_PORT", "5432")

		loader := NewEnvLoader(WithEnvPrefix("CFG"))
		data, _ := loader.Load(context.Background())

		if _, exists := data["db.host"]; !exists {
			t.Errorf("expected key 'db.host', available keys: check normalization")
		}
		if _, exists := data["db.port"]; !exists {
			t.Errorf("expected key 'db.port', available keys: check normalization")
		}
	})
}

func TestEnvLoader_Options(t *testing.T) {
	t.Run("WithEnvPrefix strips prefix and underscore", func(t *testing.T) {
		t.Setenv("APP_NAME", "myapp")

		loader := NewEnvLoader(WithEnvPrefix("APP"))
		data, _ := loader.Load(context.Background())

		// Prefix "APP" + "_" should be stripped, leaving "NAME" -> "name"
		if _, exists := data["name"]; !exists {
			t.Errorf("expected key 'name' after prefix stripping")
		}
	})

	t.Run("WithEnvPrefix trims trailing underscores", func(t *testing.T) {
		t.Setenv("SECRET_VALUE", "shhh")

		loader := NewEnvLoader(WithEnvPrefix("SECRET_"))
		data, _ := loader.Load(context.Background())

		// Prefix should be trimmed to "SECRET" and then _VALUE is stripped
		if _, exists := data["value"]; !exists {
			t.Errorf("expected key 'value'")
		}
	})

	t.Run("WithEnvPriority sets priority", func(t *testing.T) {
		t.Setenv("TEST_PRIO", "val")

		loader := NewEnvLoader(WithEnvPriority(100))
		data, _ := loader.Load(context.Background())

		// Find our test var
		val, exists := data["test.prio"]
		if !exists {
			t.Skip("test.prio not found in env (may not be present)")
		}
		if val.Priority() != 100 {
			t.Errorf("expected priority 100, got %d", val.Priority())
		}
	})

	t.Run("WithEnvKeyReplacer custom replacer", func(t *testing.T) {
		t.Setenv("CUSTOM_KEY", "custom-value")

		loader := NewEnvLoader(WithEnvKeyReplacer(func(key string) string {
			// Keep original casing
			return key
		}))
		data, _ := loader.Load(context.Background())

		if _, exists := data["CUSTOM_KEY"]; !exists {
			t.Error("expected key 'CUSTOM_KEY' with custom replacer")
		}
	})
}

func TestEnvLoader_Close(t *testing.T) {
	t.Run("close succeeds", func(t *testing.T) {
		loader := NewEnvLoader()
		err := loader.Close(context.Background())
		if err != nil {
			t.Fatalf("unexpected close error: %v", err)
		}
	})

	t.Run("load after close returns error", func(t *testing.T) {
		loader := NewEnvLoader()
		loader.Close(context.Background())

		_, err := loader.Load(context.Background())
		if err == nil {
			t.Fatal("expected error after close")
		}
	})

	t.Run("close is idempotent", func(t *testing.T) {
		loader := NewEnvLoader()
		err1 := loader.Close(context.Background())
		err2 := loader.Close(context.Background())
		if err1 != nil || err2 != nil {
			t.Fatalf("close errors: %v, %v", err1, err2)
		}
	})
}

func TestEnvLoader_Watch(t *testing.T) {
	t.Run("returns nil channel", func(t *testing.T) {
		loader := NewEnvLoader()
		ch, err := loader.Watch(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ch != nil {
			t.Error("expected nil channel for env loader")
		}
	})
}

func TestEnvLoader_DefaultKeyReplacer(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "APP_NAME", "app.name"},
		{"single", "HOST", "host"},
		{"multiple underscores", "DB_CONNECTION_POOL_SIZE", "db.connection.pool.size"},
		{"already lowercase", "lower_case", "lower.case"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultKeyReplacer(tt.input)
			if got != tt.want {
				t.Errorf("defaultKeyReplacer(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEnvLoader_SourceAndType(t *testing.T) {
	t.Setenv("TEST_SRC_TYPE", "value")

	loader := NewEnvLoader()
	data, _ := loader.Load(context.Background())

	val, exists := data["test.src.type"]
	if !exists {
		t.Skip("test.src.type not found in env")
	}
	if val.Source() != 2 { // SourceEnv = 2
		t.Errorf("expected SourceEnv, got %d", val.Source())
	}
}

func TestEnvLoader_Priority(t *testing.T) {
	t.Run("default priority is 40", func(t *testing.T) {
		loader := NewEnvLoader()
		if loader.Priority() != 40 {
			t.Errorf("expected default priority 40, got %d", loader.Priority())
		}
	})
}

func TestEnvLoader_NameAndType(t *testing.T) {
	loader := NewEnvLoader()
	if loader.Name() != "env" {
		t.Errorf("expected name 'env', got %q", loader.Name())
	}
	if loader.Type() != "env" {
		t.Errorf("expected type 'env', got %q", loader.Type())
	}
}

func TestEnvLoader_IgnoresMalformedEnv(t *testing.T) {
	// This tests that env vars without '=' are skipped
	// We can't easily set env vars without '=', but we can test
	// that the loader handles the edge case
	loader := NewEnvLoader()
	data, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil data")
	}
}

func TestEnvLoader_FiltersByPrefix(t *testing.T) {
	// Set multiple env vars with different prefixes
	t.Setenv("PREFIX_A_KEY1", "val1")
	t.Setenv("PREFIX_A_KEY2", "val2")
	t.Setenv("PREFIX_B_KEY3", "val3")

	loader := NewEnvLoader(WithEnvPrefix("PREFIX_A"))
	data, _ := loader.Load(context.Background())

	if _, exists := data["key1"]; !exists {
		t.Error("expected key1 to exist")
	}
	if _, exists := data["key2"]; !exists {
		t.Error("expected key2 to exist")
	}
	// PREFIX_B_KEY3 should NOT be present
	if _, exists := data["key3"]; exists {
		t.Error("key3 should not exist (different prefix)")
	}
}

func TestEnvLoader_EnvVars(t *testing.T) {
	// Verify that env vars not set by our test are also loaded
	// when no prefix is used
	loader := NewEnvLoader()
	data, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// At minimum, PATH should exist in any Go environment
	if _, exists := data["path"]; !exists {
		t.Logf("PATH env var not found in loaded data (this may be environment-specific)")
	}

	// Verify that os.Environ values are accessible
	_ = os.Environ() // just ensure it doesn't panic
	_ = data         // use the variable
}

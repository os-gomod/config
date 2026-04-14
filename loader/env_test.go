package loader_test

import (
	"context"
	"os"
	"testing"

	"github.com/os-gomod/config/loader"
)

func TestEnvLoaderWithPrefix(t *testing.T) {
	os.Setenv("MYAPP_DB_HOST", "localhost")
	os.Setenv("MYAPP_DB_PORT", "5432")
	os.Setenv("OTHER_KEY", "ignored")
	defer os.Unsetenv("MYAPP_DB_HOST")
	defer os.Unsetenv("MYAPP_DB_PORT")
	defer os.Unsetenv("OTHER_KEY")

	e := loader.NewEnvLoader(loader.WithEnvPrefix("MYAPP"))
	data, err := e.Load(context.Background())
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if v, ok := data["db.host"]; !ok || v.Raw() != "localhost" {
		t.Errorf("expected db.host=localhost, got %v", v)
	}
	if _, ok := data["other.key"]; ok {
		t.Error("expected OTHER_KEY to be filtered out by prefix")
	}
}

func TestEnvLoaderWithPrefixTrailingUnderscore(t *testing.T) {
	os.Setenv("MYAPP_DB_HOST", "localhost")
	defer os.Unsetenv("MYAPP_DB_HOST")

	e := loader.NewEnvLoader(loader.WithEnvPrefix("MYAPP_"))
	data, err := e.Load(context.Background())
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if v, ok := data["db.host"]; !ok || v.Raw() != "localhost" {
		t.Errorf("expected db.host=localhost, got %v", v)
	}
}

func TestEnvLoaderNoPrefix(t *testing.T) {
	os.Setenv("TEST_NO_PREFIX_KEY", "value")
	defer os.Unsetenv("TEST_NO_PREFIX_KEY")

	e := loader.NewEnvLoader()
	data, err := e.Load(context.Background())
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if _, ok := data["test.no.prefix.key"]; !ok {
		t.Error("expected test.no.prefix.key to be present")
	}
}

func TestEnvLoaderCustomReplacer(t *testing.T) {
	os.Setenv("CUSTOM__KEY", "value")
	defer os.Unsetenv("CUSTOM__KEY")

	e := loader.NewEnvLoader(loader.WithEnvKeyReplacer(func(key string) string {
		// Replace double underscore with dot, lowercase.
		return replaceDoubleUnderscore(key)
	}))
	data, err := e.Load(context.Background())
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if _, ok := data["custom.key"]; !ok {
		t.Error("expected custom.key after custom replacer")
	}
}

func replaceDoubleUnderscore(key string) string {
	result := make([]byte, 0, len(key))
	for i := 0; i < len(key); i++ {
		if key[i] == '_' && i+1 < len(key) && key[i+1] == '_' {
			result = append(result, '.')
			i++ // skip next underscore
		} else {
			result = append(result, key[i])
		}
	}
	// Lowercase.
	for i, c := range result {
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		}
	}
	return string(result)
}

func TestEnvLoaderEmptyEnv(t *testing.T) {
	// With a prefix that no env var has, result should be empty.
	e := loader.NewEnvLoader(loader.WithEnvPrefix("NONEXISTENT_PREFIX_XYZ"))
	data, err := e.Load(context.Background())
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty map, got %d keys", len(data))
	}
}

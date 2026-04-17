package fuzz

import (
	"encoding/json"
	"testing"

	"github.com/os-gomod/config/decoder"
	"github.com/os-gomod/config/internal/keyutil"
)

// ---------------------------------------------------------------------------
// FuzzYAMLDecoder
// ---------------------------------------------------------------------------

func FuzzYAMLDecoder(f *testing.F) {
	// Seed with valid and edge-case YAML inputs.
	seeds := []string{
		"key: value\n",
		"a:\n  b: 1\n",
		"port: 8080\nenabled: true\n",
		"empty: \"\"\n",
		"nested:\n  deep:\n    value: hello\n",
		"list:\n  - one\n  - two\n",
		"null_key:\n",
		"",
		"key: \"\"\n",
		"escape: \"line1\\nline2\"\n",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		d := decoder.NewYAMLDecoder()
		result, err := d.Decode(data)
		// Must not panic; errors are acceptable for invalid input.
		if err != nil {
			return
		}
		// Verify the result is a valid map (may be empty).
		if result == nil {
			t.Error("expected non-nil map on success")
		}
	})
}

// ---------------------------------------------------------------------------
// FuzzJSONDecoder
// ---------------------------------------------------------------------------

func FuzzJSONDecoder(f *testing.F) {
	seeds := []string{
		`{"key": "value"}`,
		`{"a": {"b": 1}}`,
		`{"port": 8080, "enabled": true}`,
		`{"empty": ""}`,
		`{"nested": {"deep": {"value": "hello"}}}`,
		`{"list": [1, 2, 3]}`,
		`{}`,
		`{"null_key": null}`,
		`{"num": 3.14}`,
		`{"escape": "line1\nline2"}`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		d := decoder.NewJSONDecoder()
		result, err := d.Decode(data)
		if err != nil {
			return
		}
		if result == nil {
			t.Error("expected non-nil map on success")
		}
	})
}

// ---------------------------------------------------------------------------
// FuzzEnvDecoder
// ---------------------------------------------------------------------------

func FuzzEnvDecoder(f *testing.F) {
	seeds := []string{
		"DB_HOST=localhost\nDB_PORT=5432\n",
		"APP_NAME=\"my app\"\n",
		"# comment\nKEY=VALUE\n",
		"EMPTY=\n",
		"QUOTED='single'\n",
		"SPACE=value with spaces\n",
		"",
		"NO_VALUE\n",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		d := decoder.NewEnvDecoder()
		result, err := d.Decode(data)
		// EnvDecoder never returns an error in current implementation,
		// but we still verify the result is valid.
		if err != nil {
			return
		}
		if result == nil {
			t.Error("expected non-nil map on success")
		}
	})
}

// ---------------------------------------------------------------------------
// FuzzFlattenMap
// ---------------------------------------------------------------------------

func FuzzFlattenMap(f *testing.F) {
	// Seed with JSON-encoded maps (valid fuzz type is []byte).
	seedData := []string{
		`{"key": "value"}`,
		`{"a": {"b": "c"}}`,
		`{"x": 1, "y": true, "z": "str"}`,
		`{"deep": {"nested": {"val": 42}}}`,
		`{}`,
	}
	for _, s := range seedData {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var input map[string]any
		if err := json.Unmarshal(data, &input); err != nil {
			return // skip invalid JSON
		}
		result := keyutil.FlattenMap(input)
		if result == nil {
			t.Error("expected non-nil result")
		}
		// All keys should be dot-separated (no nested maps in result).
		for k, v := range result {
			if nested, ok := v.(map[string]any); ok {
				t.Errorf("key %q should not contain nested map: %v", k, nested)
			}
		}
	})
}

package decoder_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/os-gomod/config/decoder"
)

func TestYAMLDecoder(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]any
		wantErr  bool
	}{
		{
			name:  "nested map",
			input: "db:\n  host: localhost\n  port: 5432\n",
			expected: map[string]any{
				"db.host": "localhost",
				"db.port": 5432,
			},
			wantErr: false,
		},
		{
			name:  "sequence",
			input: "servers:\n  - a\n  - b\n",
			expected: map[string]any{
				"servers": []any{"a", "b"},
			},
			wantErr: false,
		},
		{
			name:  "scalar",
			input: "name: test\n",
			expected: map[string]any{
				"name": "test",
			},
			wantErr: false,
		},
		{
			name:    "invalid yaml",
			input:   ":\n  :\n  - invalid:\n    - {",
			wantErr: true,
		},
	}

	d := decoder.NewYAMLDecoder()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.Decode([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			for k, v := range tt.expected {
				gotVal, ok := got[k]
				if !ok {
					t.Errorf("missing key %q", k)
					continue
				}
				if !reflect.DeepEqual(gotVal, v) {
					t.Errorf("key %q: expected %v (%T), got %v (%T)", k, v, v, gotVal, gotVal)
				}
			}
		})
	}
}

func TestJSONDecoder(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]any
		wantErr  bool
	}{
		{
			name:  "nested object",
			input: `{"db":{"host":"localhost","port":5432}}`,
			expected: map[string]any{
				"db.host": "localhost",
				"db.port": float64(5432),
			},
			wantErr: false,
		},
		{
			name:  "array",
			input: `{"items":["a","b"]}`,
			expected: map[string]any{
				"items": []any{"a", "b"},
			},
			wantErr: false,
		},
		{
			name:  "null value",
			input: `{"key":null}`,
			expected: map[string]any{
				"key": nil,
			},
			wantErr: false,
		},
		{
			name:    "invalid json",
			input:   `{invalid`,
			wantErr: true,
		},
	}

	d := decoder.NewJSONDecoder()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.Decode([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			for k, v := range tt.expected {
				gotVal, ok := got[k]
				if !ok {
					t.Errorf("missing key %q", k)
				}
				if !reflect.DeepEqual(gotVal, v) {
					t.Errorf("key %q: expected %v (%T), got %v (%T)", k, v, v, gotVal, gotVal)
				}
			}
		})
	}
}

func TestEnvDecoder(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]any
	}{
		{
			name:  "KEY=VALUE",
			input: "DB_HOST=localhost\nDB_PORT=5432\n",
			expected: map[string]any{
				"db.host": "localhost",
				"db.port": "5432",
			},
		},
		{
			name:  "quoted values",
			input: "DB_HOST=\"localhost\"\nDB_USER='admin'\n",
			expected: map[string]any{
				"db.host": "localhost",
				"db.user": "admin",
			},
		},
		{
			name:  "comments ignored",
			input: "# This is a comment\nDB_HOST=localhost\n# Another comment\n",
			expected: map[string]any{
				"db.host": "localhost",
			},
		},
		{
			name:     "empty input",
			input:    "",
			expected: map[string]any{},
		},
	}

	d := decoder.NewEnvDecoder()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.Decode([]byte(tt.input))
			if err != nil {
				t.Fatalf("Decode() error: %v", err)
			}
			if len(got) != len(tt.expected) {
				t.Errorf("expected %d keys, got %d", len(tt.expected), len(got))
			}
			for k, v := range tt.expected {
				gotVal, ok := got[k]
				if !ok {
					t.Errorf("missing key %q", k)
				}
				if gotVal != v {
					t.Errorf("key %q: expected %v, got %v", k, v, gotVal)
				}
			}
		})
	}
}

func TestINIDecoder(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]any
	}{
		{
			name:  "section prefix",
			input: "[db]\nhost = localhost\nport = 5432\n",
			expected: map[string]any{
				"db.host": "localhost",
				"db.port": "5432",
			},
		},
		{
			name:  "comments ignored",
			input: "[db]\n# comment\nhost = localhost\n; semicolon comment\n",
			expected: map[string]any{
				"db.host": "localhost",
			},
		},
		{
			name:  "no section",
			input: "key = value\n",
			expected: map[string]any{
				"key": "value",
			},
		},
	}

	d := decoder.NewINIDecoder()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.Decode([]byte(tt.input))
			if err != nil {
				t.Fatalf("Decode() error: %v", err)
			}
			if len(got) != len(tt.expected) {
				t.Errorf("expected %d keys, got %d", len(tt.expected), len(got))
			}
			for k, v := range tt.expected {
				gotVal, ok := got[k]
				if !ok {
					t.Errorf("missing key %q", k)
				}
				if gotVal != v {
					t.Errorf("key %q: expected %v, got %v", k, v, gotVal)
				}
			}
		})
	}
}

func TestRegistry(t *testing.T) {
	t.Run("Register and ForExtension", func(t *testing.T) {
		r := decoder.NewRegistry()
		d := decoder.NewYAMLDecoder()
		if err := r.Register(d); err != nil {
			t.Fatalf("Register error: %v", err)
		}
		got, err := r.ForExtension(".yaml")
		if err != nil {
			t.Fatalf("ForExtension error: %v", err)
		}
		if got.MediaType() != "application/x-yaml" {
			t.Errorf("expected YAML decoder, got %q", got.MediaType())
		}
	})

	t.Run("ForExtension not found", func(t *testing.T) {
		r := decoder.NewRegistry()
		_, err := r.ForExtension(".xyz")
		if err == nil {
			t.Error("expected error for unknown extension")
		}
		if !errors.Is(err, decoder.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("ForMediaType", func(t *testing.T) {
		r := decoder.NewRegistry()
		_ = r.Register(decoder.NewJSONDecoder())
		got, err := r.ForMediaType("application/json")
		if err != nil {
			t.Fatalf("ForMediaType error: %v", err)
		}
		if got.MediaType() != "application/json" {
			t.Errorf("expected JSON decoder, got %q", got.MediaType())
		}
	})

	t.Run("ErrAlreadyRegistered", func(t *testing.T) {
		r := decoder.NewRegistry()
		_ = r.Register(decoder.NewYAMLDecoder())
		err := r.Register(decoder.NewYAMLDecoder())
		if err == nil {
			t.Error("expected ErrAlreadyRegistered")
		}
		if !errors.Is(err, decoder.ErrAlreadyRegistered) {
			t.Errorf("expected ErrAlreadyRegistered, got %v", err)
		}
	})

	t.Run("MustRegister panics on duplicate", func(t *testing.T) {
		r := decoder.NewRegistry()
		r.MustRegister(decoder.NewYAMLDecoder())
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic from MustRegister")
			}
		}()
		r.MustRegister(decoder.NewYAMLDecoder())
	})

	t.Run("Names returns sorted extensions", func(t *testing.T) {
		r := decoder.NewRegistry()
		r.MustRegister(decoder.NewJSONDecoder())
		r.MustRegister(decoder.NewYAMLDecoder())
		names := r.Names()
		if len(names) < 3 {
			t.Errorf("expected at least 3 extensions, got %d", len(names))
		}
		// Check sorted order.
		for i := 1; i < len(names); i++ {
			if names[i] < names[i-1] {
				t.Errorf("names not sorted: %q before %q", names[i-1], names[i])
			}
		}
	})
}

func TestDefaultRegistry(t *testing.T) {
	// DefaultRegistry should have YAML, JSON, env, and INI decoders.
	exts := []string{".yaml", ".yml", ".json", ".env", ".ini"}
	for _, ext := range exts {
		_, err := decoder.DefaultRegistry.ForExtension(ext)
		if err != nil {
			t.Errorf("DefaultRegistry missing decoder for %q: %v", ext, err)
		}
	}
}

func TestFlattenDeeplyNested(t *testing.T) {
	d := decoder.NewYAMLDecoder()
	input := "a:\n  b:\n    c:\n      d: deep\n"
	got, err := d.Decode([]byte(input))
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	val, ok := got["a.b.c.d"]
	if !ok {
		t.Error("missing key a.b.c.d")
	}
	if val != "deep" {
		t.Errorf("expected 'deep', got %v", val)
	}
}

// Suppress unused import.
var _ = fmt.Sprintf

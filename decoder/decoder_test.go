package decoder

import (
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// YAML Decoder
// ---------------------------------------------------------------------------

func TestYAMLDecoder_Decode(t *testing.T) {
	d := NewYAMLDecoder()

	t.Run("valid YAML with nested maps", func(t *testing.T) {
		src := "server:\n  host: localhost\n  port: 8080\ndatabase:\n  name: mydb\n  connection:\n    pool_size: 10\n"
		out, err := d.Decode([]byte(src))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := out["server.host"]; got != "localhost" {
			t.Errorf("server.host = %v, want %q", got, "localhost")
		}
		if got := out["server.port"]; got != 8080 {
			t.Errorf("server.port = %v, want 8080", got)
		}
		if got := out["database.name"]; got != "mydb" {
			t.Errorf("database.name = %v, want %q", got, "mydb")
		}
		if got := out["database.connection.pool_size"]; got != 10 {
			t.Errorf("database.connection.pool_size = %v, want 10", got)
		}
	})

	t.Run("keys are lowercased", func(t *testing.T) {
		src := "Server: {Host: localhost}\n"
		out, err := d.Decode([]byte(src))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := out["server.host"]; !ok {
			t.Errorf("expected lowercased key server.host, got keys: %v", keys(out))
		}
	})

	t.Run("invalid YAML", func(t *testing.T) {
		_, err := d.Decode([]byte(":\t:\t"))
		if err == nil {
			t.Fatal("expected error for invalid YAML")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		out, err := d.Decode([]byte(""))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out) != 0 {
			t.Errorf("expected empty map, got %d keys", len(out))
		}
	})
}

func TestYAMLDecoder_Metadata(t *testing.T) {
	d := NewYAMLDecoder()
	if d.MediaType() != "application/x-yaml" {
		t.Errorf("MediaType() = %q, want %q", d.MediaType(), "application/x-yaml")
	}
	exts := d.Extensions()
	wantExts := []string{".yaml", ".yml"}
	if len(exts) != len(wantExts) {
		t.Fatalf("Extensions() = %v, want %v", exts, wantExts)
	}
	for i, e := range wantExts {
		if exts[i] != e {
			t.Errorf("Extensions()[%d] = %q, want %q", i, exts[i], e)
		}
	}
}

// ---------------------------------------------------------------------------
// JSON Decoder
// ---------------------------------------------------------------------------

func TestJSONDecoder_Decode(t *testing.T) {
	d := NewJSONDecoder()

	t.Run("valid JSON with nested maps", func(t *testing.T) {
		src := `{"server":{"host":"localhost","port":8080},"debug":true}`
		out, err := d.Decode([]byte(src))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := out["server.host"]; got != "localhost" {
			t.Errorf("server.host = %v, want %q", got, "localhost")
		}
		// JSON numbers decode as float64
		if got, ok := out["server.port"].(float64); !ok || got != 8080 {
			t.Errorf("server.port = %v, want 8080", out["server.port"])
		}
		if got := out["debug"]; got != true {
			t.Errorf("debug = %v, want true", got)
		}
	})

	t.Run("keys are lowercased", func(t *testing.T) {
		src := `{"Server":{"Host":"localhost"}}`
		out, err := d.Decode([]byte(src))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := out["server.host"]; !ok {
			t.Errorf("expected lowercased key server.host, got keys: %v", keys(out))
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := d.Decode([]byte("{invalid json"))
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		out, err := d.Decode([]byte("{}"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out) != 0 {
			t.Errorf("expected empty map, got %d keys", len(out))
		}
	})
}

func TestJSONDecoder_Metadata(t *testing.T) {
	d := NewJSONDecoder()
	if d.MediaType() != "application/json" {
		t.Errorf("MediaType() = %q, want %q", d.MediaType(), "application/json")
	}
	exts := d.Extensions()
	if len(exts) != 1 || exts[0] != ".json" {
		t.Errorf("Extensions() = %v, want [.json]", exts)
	}
}

// ---------------------------------------------------------------------------
// TOML Decoder
// ---------------------------------------------------------------------------

func TestTOMLDecoder_Decode(t *testing.T) {
	d := NewTOMLDecoder()

	t.Run("valid TOML", func(t *testing.T) {
		src := "[server]\nhost = \"localhost\"\nport = 8080\n\n[database]\nname = \"mydb\"\n"
		out, err := d.Decode([]byte(src))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := out["server.host"]; got != "localhost" {
			t.Errorf("server.host = %v, want %q", got, "localhost")
		}
		// TOML integers decode as int64
		if got, ok := out["server.port"].(int64); !ok || got != 8080 {
			t.Errorf("server.port = %v, want 8080", out["server.port"])
		}
		if got := out["database.name"]; got != "mydb" {
			t.Errorf("database.name = %v, want %q", got, "mydb")
		}
	})

	t.Run("invalid TOML", func(t *testing.T) {
		_, err := d.Decode([]byte("[invalid\ntoml = ="))
		if err == nil {
			t.Fatal("expected error for invalid TOML")
		}
	})
}

func TestTOMLDecoder_Metadata(t *testing.T) {
	d := NewTOMLDecoder()
	if d.MediaType() != "application/toml" {
		t.Errorf("MediaType() = %q, want %q", d.MediaType(), "application/toml")
	}
	exts := d.Extensions()
	if len(exts) != 1 || exts[0] != ".toml" {
		t.Errorf("Extensions() = %v, want [.toml]", exts)
	}
}

// ---------------------------------------------------------------------------
// HCL Decoder
// ---------------------------------------------------------------------------

func TestHCLDecoder_Decode(t *testing.T) {
	d := NewHCLDecoder()

	t.Run("valid HCL parses without error", func(t *testing.T) {
		src := "name = \"hello\"\ncount = 42\ndebug = true\n"
		out, err := d.Decode([]byte(src))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// hclsimple.Decode with *map[string]any returns *hcl.Attribute objects
		// rather than native Go types. Verify keys exist.
		if _, ok := out["name"]; !ok {
			t.Error("expected key 'name'")
		}
		if _, ok := out["count"]; !ok {
			t.Error("expected key 'count'")
		}
		if _, ok := out["debug"]; !ok {
			t.Error("expected key 'debug'")
		}
	})

	t.Run("invalid HCL", func(t *testing.T) {
		_, err := d.Decode([]byte("<<invalid>>"))
		if err == nil {
			t.Fatal("expected error for invalid HCL")
		}
	})
}

func TestHCLDecoder_Metadata(t *testing.T) {
	d := NewHCLDecoder()
	if d.MediaType() != "application/hcl" {
		t.Errorf("MediaType() = %q, want %q", d.MediaType(), "application/hcl")
	}
	exts := d.Extensions()
	if len(exts) != 1 || exts[0] != ".hcl" {
		t.Errorf("Extensions() = %v, want [.hcl]", exts)
	}
}

// ---------------------------------------------------------------------------
// ENV Decoder
// ---------------------------------------------------------------------------

func TestEnvDecoder_Decode(t *testing.T) {
	d := NewEnvDecoder()

	t.Run("key=value pairs", func(t *testing.T) {
		src := "DB_HOST=localhost\nDB_PORT=5432\nAPP_NAME=myapp"
		out, err := d.Decode([]byte(src))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := out["db.host"]; got != "localhost" {
			t.Errorf("db.host = %v, want %q", got, "localhost")
		}
		if got := out["db.port"]; got != "5432" {
			t.Errorf("db.port = %v, want %q", got, "5432")
		}
		if got := out["app.name"]; got != "myapp" {
			t.Errorf("app.name = %v, want %q", got, "myapp")
		}
	})

	t.Run("quoted values are unquoted", func(t *testing.T) {
		src := "NAME=\"my app\"\nDESC='test value'"
		out, err := d.Decode([]byte(src))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := out["name"]; got != "my app" {
			t.Errorf("name = %v, want %q", got, "my app")
		}
		if got := out["desc"]; got != "test value" {
			t.Errorf("desc = %v, want %q", got, "test value")
		}
	})

	t.Run("comments are skipped", func(t *testing.T) {
		src := "# this is a comment\nKEY=val"
		out, err := d.Decode([]byte(src))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := out["key"]; !ok {
			t.Error("expected key 'key' to exist")
		}
	})

	t.Run("empty lines are skipped", func(t *testing.T) {
		src := "\n\nKEY=val\n\n"
		out, err := d.Decode([]byte(src))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out) != 1 {
			t.Errorf("expected 1 key, got %d", len(out))
		}
	})

	t.Run("underscores become dots and keys are lowercased", func(t *testing.T) {
		src := "DB_CONNECTION_POOL=10"
		out, err := d.Decode([]byte(src))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := out["db.connection.pool"]; !ok {
			t.Errorf("expected key 'db.connection.pool', got keys: %v", keys(out))
		}
	})
}

func TestEnvDecoder_Metadata(t *testing.T) {
	d := NewEnvDecoder()
	if d.MediaType() != "text/x-env" {
		t.Errorf("MediaType() = %q, want %q", d.MediaType(), "text/x-env")
	}
	exts := d.Extensions()
	if len(exts) != 1 || exts[0] != ".env" {
		t.Errorf("Extensions() = %v, want [.env]", exts)
	}
}

// ---------------------------------------------------------------------------
// INI Decoder
// ---------------------------------------------------------------------------

func TestINIDecoder_Decode(t *testing.T) {
	d := NewINIDecoder()

	t.Run("sections and key=value", func(t *testing.T) {
		src := "[server]\nhost = localhost\nport = 8080\n[database]\nname = mydb"
		out, err := d.Decode([]byte(src))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := out["server.host"]; got != "localhost" {
			t.Errorf("server.host = %v, want %q", got, "localhost")
		}
		if got := out["server.port"]; got != "8080" {
			t.Errorf("server.port = %v, want %q", got, "8080")
		}
		if got := out["database.name"]; got != "mydb" {
			t.Errorf("database.name = %v, want %q", got, "mydb")
		}
	})

	t.Run("comments are skipped", func(t *testing.T) {
		src := "; semicolon comment\n# hash comment\nkey=val"
		out, err := d.Decode([]byte(src))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out) != 1 {
			t.Errorf("expected 1 key, got %d: %v", len(out), keys(out))
		}
	})

	t.Run("keys without section", func(t *testing.T) {
		src := "global_key=value"
		out, err := d.Decode([]byte(src))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := out["global_key"]; got != "value" {
			t.Errorf("global_key = %v, want %q", got, "value")
		}
	})

	t.Run("empty lines are skipped", func(t *testing.T) {
		src := "\n\n[sec]\nkey=val\n"
		out, err := d.Decode([]byte(src))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out) != 1 {
			t.Errorf("expected 1 key, got %d", len(out))
		}
	})
}

func TestINIDecoder_Metadata(t *testing.T) {
	d := NewINIDecoder()
	if d.MediaType() != "text/x-ini" {
		t.Errorf("MediaType() = %q, want %q", d.MediaType(), "text/x-ini")
	}
	exts := d.Extensions()
	if len(exts) != 1 || exts[0] != ".ini" {
		t.Errorf("Extensions() = %v, want [.ini]", exts)
	}
}

// ---------------------------------------------------------------------------
// unquote helper
// ---------------------------------------------------------------------------

func TestUnquote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`"hello"`, "hello"},
		{`'hello'`, "hello"},
		{`hello`, "hello"},
		{`""`, ""},
		{`''`, ""},
		{`"`, `"`},
		{`a"b`, `a"b`},
		{`"a`, `"a`},
		{`'a`, `'a`},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("unquote(%q)", tt.input), func(t *testing.T) {
			got := unquote(tt.input)
			if got != tt.want {
				t.Errorf("unquote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

// mockDecoder is a test decoder for Registry tests.
type mockDecoder struct {
	mediaType  string
	extensions []string
}

func (m *mockDecoder) Decode(_ []byte) (map[string]any, error) { return nil, nil }
func (m *mockDecoder) MediaType() string                       { return m.mediaType }
func (m *mockDecoder) Extensions() []string                    { return m.extensions }

func TestRegistry_Register(t *testing.T) {
	t.Run("successful registration", func(t *testing.T) {
		r := NewRegistry()
		d := &mockDecoder{mediaType: "application/test", extensions: []string{".test"}}
		if err := r.Register(d); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, err := r.ForExtension(".test")
		if err != nil {
			t.Fatalf("ForExtension: %v", err)
		}
		if got != d {
			t.Error("returned decoder is not the one registered")
		}
	})

	t.Run("duplicate extension error", func(t *testing.T) {
		r := NewRegistry()
		d1 := &mockDecoder{mediaType: "application/a", extensions: []string{".dup"}}
		d2 := &mockDecoder{mediaType: "application/b", extensions: []string{".dup"}}
		if err := r.Register(d1); err != nil {
			t.Fatalf("first register: %v", err)
		}
		if err := r.Register(d2); err != ErrAlreadyRegistered {
			t.Errorf("expected ErrAlreadyRegistered, got %v", err)
		}
	})

	t.Run("duplicate media type error", func(t *testing.T) {
		r := NewRegistry()
		d1 := &mockDecoder{mediaType: "application/same", extensions: []string{".a"}}
		d2 := &mockDecoder{mediaType: "application/same", extensions: []string{".b"}}
		if err := r.Register(d1); err != nil {
			t.Fatalf("first register: %v", err)
		}
		if err := r.Register(d2); err != ErrAlreadyRegistered {
			t.Errorf("expected ErrAlreadyRegistered, got %v", err)
		}
	})
}

func TestRegistry_MustRegister(t *testing.T) {
	t.Run("panics on duplicate", func(t *testing.T) {
		r := NewRegistry()
		d := &mockDecoder{mediaType: "application/test", extensions: []string{".dup"}}
		r.MustRegister(d)
		defer func() {
			if rec := recover(); rec == nil {
				t.Error("expected panic on duplicate MustRegister")
			}
		}()
		r.MustRegister(d)
	})
}

func TestRegistry_ForExtension(t *testing.T) {
	r := NewRegistry()
	d := &mockDecoder{mediaType: "application/x-custom", extensions: []string{".custom", ".cust"}}
	r.MustRegister(d)

	t.Run("first extension", func(t *testing.T) {
		got, err := r.ForExtension(".custom")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != d {
			t.Error("wrong decoder")
		}
	})

	t.Run("second extension", func(t *testing.T) {
		got, err := r.ForExtension(".cust")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != d {
			t.Error("wrong decoder")
		}
	})

	t.Run("unknown extension", func(t *testing.T) {
		_, err := r.ForExtension(".unknown")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestRegistry_ForMediaType(t *testing.T) {
	r := NewRegistry()
	d := &mockDecoder{mediaType: "application/x-custom", extensions: []string{".custom"}}
	r.MustRegister(d)

	t.Run("known media type", func(t *testing.T) {
		got, err := r.ForMediaType("application/x-custom")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != d {
			t.Error("wrong decoder")
		}
	})

	t.Run("unknown media type", func(t *testing.T) {
		_, err := r.ForMediaType("application/unknown")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestRegistry_Names(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&mockDecoder{mediaType: "a", extensions: []string{".yaml"}})
	r.MustRegister(&mockDecoder{mediaType: "b", extensions: []string{".json"}})
	r.MustRegister(&mockDecoder{mediaType: "c", extensions: []string{".toml", ".tml"}})

	names := r.Names()
	if len(names) != 4 {
		t.Errorf("expected 4 names, got %d: %v", len(names), names)
	}
	// Verify sorted
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("names not sorted: %v", names)
			break
		}
	}
}

func TestDefaultRegistry(t *testing.T) {
	r := DefaultRegistry

	expectedExts := []string{".env", ".hcl", ".ini", ".json", ".toml", ".yaml", ".yml"}
	names := r.Names()
	for _, ext := range expectedExts {
		found := false
		for _, n := range names {
			if n == ext {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultRegistry missing extension %q (got %v)", ext, names)
		}
	}

	// Verify we can look up each one
	for _, ext := range expectedExts {
		_, err := r.ForExtension(ext)
		if err != nil {
			t.Errorf("ForExtension(%q): %v", ext, err)
		}
	}
}

// ---------------------------------------------------------------------------
// decodeAndFlatten (shared helper)
// ---------------------------------------------------------------------------

func TestDecodeAndFlatten_NilInput(t *testing.T) {
	// When yaml.Unmarshal gets nil/empty input, it returns empty map
	d := NewYAMLDecoder()
	out, err := d.Decode(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty map, got %d keys", len(out))
	}
}

func TestDecodeAndFlatten_NestedFlattening(t *testing.T) {
	tests := []struct {
		name  string
		dec   Decoder
		input string
		want  map[string]any
	}{
		{
			name:  "YAML deeply nested",
			dec:   NewYAMLDecoder(),
			input: "a:\n  b:\n    c:\n      d: 42",
			want:  map[string]any{"a.b.c.d": 42},
		},
		{
			name:  "JSON deeply nested",
			dec:   NewJSONDecoder(),
			input: `{"a":{"b":{"c":{"d":42}}}}`,
			// JSON numbers decode as float64
			want: map[string]any{"a.b.c.d": float64(42)},
		},
		{
			name:  "TOML deeply nested",
			dec:   NewTOMLDecoder(),
			input: "[a.b]\nc = 42\n",
			// TOML integers decode as int64
			want: map[string]any{"a.b.c": int64(42)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := tt.dec.Decode([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for k, v := range tt.want {
				if got, ok := out[k]; !ok {
					t.Errorf("missing key %q", k)
				} else if got != v {
					t.Errorf("key %q = %v (type %T), want %v (type %T)", k, got, got, v, v)
				}
			}
		})
	}
}

// keys returns sorted keys of a map for readable error messages.
func keys(m map[string]any) string {
	s := make([]string, 0, len(m))
	for k := range m {
		s = append(s, k)
	}
	return strings.Join(s, ", ")
}

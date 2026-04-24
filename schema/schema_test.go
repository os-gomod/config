package schema

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSchemaMarshalJSON(t *testing.T) {
	s := &Schema{
		Type:        "object",
		Title:       "Test",
		Description: "Test schema",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
		},
		Required: []string{"name"},
	}

	data, err := s.MarshalJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(data, []byte(`"type": "object"`)) {
		t.Error("JSON should contain type")
	}
	if !bytes.Contains(data, []byte(`"title": "Test"`)) {
		t.Error("JSON should contain title")
	}
	if !bytes.Contains(data, []byte(`"name"`)) {
		t.Error("JSON should contain property 'name'")
	}
	// Should be pretty-printed (indented)
	if !bytes.Contains(data, []byte("  \"type\"")) {
		t.Error("JSON should be pretty-printed")
	}
}

func TestSchemaWriteTo(t *testing.T) {
	s := &Schema{Type: "string"}
	var buf bytes.Buffer
	n, err := s.WriteTo(&buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n <= 0 {
		t.Errorf("expected positive byte count, got %d", n)
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Error("WriteTo should append a newline")
	}
}

func TestSchemaWriteToError(t *testing.T) {
	s := &Schema{Type: "string"}
	n, err := s.WriteTo(&errorWriter{})
	if err == nil {
		t.Fatal("expected error from errorWriter")
	}
	if n != 0 {
		t.Errorf("expected 0 bytes, got %d", n)
	}
}

func TestSchemaEmpty(t *testing.T) {
	s := &Schema{}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty schema should produce "{}"
	if string(data) != "{}" {
		t.Errorf("expected '{}', got %s", string(data))
	}
}

func TestSchemaWithAllFields(t *testing.T) {
	f := 3.14
	n := 5
	s := &Schema{
		Type:                 "object",
		Format:               "duration",
		Default:              "default_val",
		Minimum:              &f,
		Maximum:              &f,
		MinLength:            &n,
		MaxLength:            &n,
		Pattern:              "^[a-z]+$",
		Items:                &Schema{Type: "string"},
		AdditionalProperties: &Schema{Type: "string"},
		Enum:                 []any{"a", "b", "c"},
	}

	data, err := s.MarshalJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	js := string(data)
	if !strings.Contains(js, "format") {
		t.Error("JSON should contain format")
	}
	if !strings.Contains(js, "minimum") {
		t.Error("JSON should contain minimum")
	}
	if !strings.Contains(js, "enum") {
		t.Error("JSON should contain enum")
	}
	if !strings.Contains(js, "pattern") {
		t.Error("JSON should contain pattern")
	}
}

func TestSchemaOmitEmpty(t *testing.T) {
	s := &Schema{
		Type: "string",
	}
	data, err := s.MarshalJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	js := string(data)
	if strings.Contains(js, "properties") {
		t.Error("empty properties should be omitted")
	}
	if strings.Contains(js, "required") {
		t.Error("empty required should be omitted")
	}
	if strings.Contains(js, "items") {
		t.Error("nil items should be omitted")
	}
}

func TestSchemaTypeConstants(t *testing.T) {
	types := []string{"string", "integer", "number", "boolean", "array", "object"}
	for _, typ := range types {
		s := &Schema{Type: typ}
		if s.Type != typ {
			t.Errorf("expected type %q, got %q", typ, s.Type)
		}
	}
}

type errorWriter struct{}

func (w *errorWriter) Write(p []byte) (n int, err error) {
	return 0, &json.InvalidUnmarshalError{}
}

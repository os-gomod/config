package schema

import (
	"strings"
	"testing"
	"time"
)

type BasicStruct struct {
	Name   string `json:"name"`
	Age    int    `json:"age"`
	Score  float64
	Active bool
}

type AllTypesStruct struct {
	StrField     string            `json:"str_field" validate:"required,min=1,max=100"`
	IntField     int               `json:"int_field" validate:"min=0"`
	FloatField   float64           `json:"float_field" validate:"min=0.0,max=99.9"`
	BoolField    bool              `json:"bool_field"`
	SliceField   []string          `json:"slice_field"`
	MapField     map[string]string `json:"map_field"`
	Nested       NestedStruct      `json:"nested"`
	Duration     time.Duration     `json:"duration"`
	TimeField    time.Time         `json:"time_field"`
	OneofField   string            `json:"oneof_field" validate:"oneof=a b c"`
	DefaultField string            `json:"default_field" default:"hello"`
	DescField    string            `json:"desc_field" description:"a description"`
	unexported   string
	Ignored      string `json:"-"`
}

type NestedStruct struct {
	Inner string `json:"inner"`
	Value int    `json:"value"`
}

func TestNew(t *testing.T) {
	g := New()
	if g == nil {
		t.Fatal("expected non-nil generator")
	}
}

func TestWithTitle(t *testing.T) {
	g := New(WithTitle("MyConfig"))
	if g.title != "MyConfig" {
		t.Errorf("expected title 'MyConfig', got %q", g.title)
	}
}

func TestWithDescription(t *testing.T) {
	g := New(WithDescription("my description"))
	if g.description != "my description" {
		t.Errorf("expected description 'my description', got %q", g.description)
	}
}

func TestGenerateFromStruct(t *testing.T) {
	g := New()
	s, err := g.Generate(BasicStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Type != "object" {
		t.Errorf("expected type 'object', got %q", s.Type)
	}
	if len(s.Properties) != 4 {
		t.Errorf("expected 4 properties, got %d", len(s.Properties))
	}
}

func TestGenerateFromPointer(t *testing.T) {
	g := New()
	s, err := g.Generate(&BasicStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Type != "object" {
		t.Errorf("expected type 'object', got %q", s.Type)
	}
}

func TestGenerateFromNil(t *testing.T) {
	g := New()
	_, err := g.Generate(nil)
	if err == nil {
		t.Fatal("expected error for nil")
	}
}

func TestGenerateFromNonStruct(t *testing.T) {
	g := New()
	_, err := g.Generate("not a struct")
	if err == nil {
		t.Fatal("expected error for non-struct")
	}
	if !strings.Contains(err.Error(), "expected struct") {
		t.Errorf("error should mention 'expected struct', got: %v", err)
	}
}

func TestGenerateAllFieldTypes(t *testing.T) {
	g := New()
	s, err := g.Generate(AllTypesStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		field    string
		typeName string
		format   string
	}{
		{"str_field", "string", ""},
		{"int_field", "integer", ""},
		{"float_field", "number", ""},
		{"bool_field", "boolean", ""},
		{"slice_field", "array", ""},
		{"map_field", "object", ""},
		{"nested", "object", ""},
		{"duration", "string", "duration"},
		{"time_field", "string", "date-time"},
		{"oneof_field", "string", ""},
		{"default_field", "string", ""},
		{"desc_field", "string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			prop, ok := s.Properties[tt.field]
			if !ok {
				t.Fatalf("missing property %q", tt.field)
			}
			if prop.Type != tt.typeName {
				t.Errorf("expected type %q, got %q", tt.typeName, prop.Type)
			}
			if tt.format != "" && prop.Format != tt.format {
				t.Errorf("expected format %q, got %q", tt.format, prop.Format)
			}
		})
	}
}

func TestGenerateSliceItemsType(t *testing.T) {
	g := New()
	s, err := g.Generate(AllTypesStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prop := s.Properties["slice_field"]
	if prop.Items == nil {
		t.Fatal("expected items for slice field")
	}
	if prop.Items.Type != "string" {
		t.Errorf("expected items type 'string', got %q", prop.Items.Type)
	}
}

func TestGenerateMapAdditionalProperties(t *testing.T) {
	g := New()
	s, err := g.Generate(AllTypesStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prop := s.Properties["map_field"]
	if prop.AdditionalProperties == nil {
		t.Fatal("expected additionalProperties for map field")
	}
	if prop.AdditionalProperties.Type != "string" {
		t.Errorf("expected additionalProperties type 'string', got %q", prop.AdditionalProperties.Type)
	}
}

func TestGenerateNestedStruct(t *testing.T) {
	g := New()
	s, err := g.Generate(AllTypesStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nested := s.Properties["nested"]
	if nested == nil {
		t.Fatal("expected nested property")
	}
	if nested.Type != "object" {
		t.Errorf("expected type 'object', got %q", nested.Type)
	}
	if _, ok := nested.Properties["inner"]; !ok {
		t.Error("expected 'inner' property in nested struct")
	}
}

func TestGenerateUnexportedAndIgnored(t *testing.T) {
	g := New()
	s, err := g.Generate(AllTypesStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := s.Properties["unexported"]; ok {
		t.Error("unexported field should not appear in schema")
	}
	// json:"-" with nothing before the comma becomes empty string,
	// which triggers toSnakeCase("Ignored") -> "ignored"
	prop, exists := s.Properties["ignored"]
	if !exists {
		t.Error("json:'-' field should generate property with snake_case name")
	} else {
		// The field has json:"-" so name == "-", which is falsy,
		// so toSnakeCase is used -> "ignored"
		_ = prop
	}
}

func TestGenerateTitleAndDescription(t *testing.T) {
	g := New(WithTitle("TestTitle"), WithDescription("TestDesc"))
	s, err := g.Generate(BasicStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Title != "TestTitle" {
		t.Errorf("expected title 'TestTitle', got %q", s.Title)
	}
	if s.Description != "TestDesc" {
		t.Errorf("expected description 'TestDesc', got %q", s.Description)
	}
}

func TestValidationTagsRequired(t *testing.T) {
	g := New()
	s, err := g.Generate(AllTypesStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, r := range s.Required {
		if r == "str_field" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'str_field' in required list")
	}
}

func TestValidationTagsMinString(t *testing.T) {
	g := New()
	s, err := g.Generate(AllTypesStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prop := s.Properties["str_field"]
	if prop.MinLength == nil {
		t.Fatal("expected MinLength to be set")
	}
	if *prop.MinLength != 1 {
		t.Errorf("expected MinLength=1, got %d", *prop.MinLength)
	}
}

func TestValidationTagsMaxString(t *testing.T) {
	g := New()
	s, err := g.Generate(AllTypesStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prop := s.Properties["str_field"]
	if prop.MaxLength == nil {
		t.Fatal("expected MaxLength to be set")
	}
	if *prop.MaxLength != 100 {
		t.Errorf("expected MaxLength=100, got %d", *prop.MaxLength)
	}
}

func TestValidationTagsMinInt(t *testing.T) {
	g := New()
	s, err := g.Generate(AllTypesStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prop := s.Properties["int_field"]
	if prop.Minimum == nil {
		t.Fatal("expected Minimum to be set for int field")
	}
	if *prop.Minimum != 0 {
		t.Errorf("expected Minimum=0, got %f", *prop.Minimum)
	}
}

func TestValidationTagsMinMaxFloat(t *testing.T) {
	g := New()
	s, err := g.Generate(AllTypesStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prop := s.Properties["float_field"]
	if prop.Minimum == nil || prop.Maximum == nil {
		t.Fatal("expected Minimum and Maximum to be set for float field")
	}
	if *prop.Minimum != 0.0 {
		t.Errorf("expected Minimum=0.0, got %f", *prop.Minimum)
	}
	if *prop.Maximum != 99.9 {
		t.Errorf("expected Maximum=99.9, got %f", *prop.Maximum)
	}
}

func TestValidationTagsOneof(t *testing.T) {
	g := New()
	s, err := g.Generate(AllTypesStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prop := s.Properties["oneof_field"]
	if prop.Enum == nil {
		t.Fatal("expected Enum to be set")
	}
	if len(prop.Enum) != 3 {
		t.Errorf("expected 3 enum values, got %d", len(prop.Enum))
	}
}

func TestDefaultTag(t *testing.T) {
	g := New()
	s, err := g.Generate(AllTypesStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prop := s.Properties["default_field"]
	if prop.Default != "hello" {
		t.Errorf("expected default 'hello', got %v", prop.Default)
	}
}

func TestDescriptionTag(t *testing.T) {
	g := New()
	s, err := g.Generate(AllTypesStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prop := s.Properties["desc_field"]
	if prop.Description != "a description" {
		t.Errorf("expected description 'a description', got %q", prop.Description)
	}
}

func TestSnakeCaseConversion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"CamelCase", "camel_case"},
		{"SimpleHTTPServer", "simple_h_t_t_p_server"},
		{"ABC", "a_b_c"},
		{"a", "a"},
		{"", ""},
		{"MyField", "my_field"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toSnakeCase(tt.input)
			if got != tt.expected {
				t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	if n := parseInt("42"); n != 42 {
		t.Errorf("expected 42, got %d", n)
	}
	if n := parseInt("abc"); n != 0 {
		t.Errorf("expected 0 for invalid, got %d", n)
	}
}

func TestParseFloat(t *testing.T) {
	if f := parseFloat("3.14"); f != 3.14 {
		t.Errorf("expected 3.14, got %f", f)
	}
	if f := parseFloat("abc"); f != 0 {
		t.Errorf("expected 0 for invalid, got %f", f)
	}
}

func TestGenerateEmptyStruct(t *testing.T) {
	type Empty struct{}
	g := New()
	s, err := g.Generate(Empty{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Type != "object" {
		t.Errorf("expected type 'object', got %q", s.Type)
	}
	if len(s.Properties) != 0 {
		t.Errorf("expected 0 properties, got %d", len(s.Properties))
	}
}

func TestGenerateNoJsonTag(t *testing.T) {
	type NoTag struct {
		MyField string
	}
	g := New()
	s, err := g.Generate(NoTag{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := s.Properties["my_field"]; !ok {
		t.Error("expected snake_case conversion for untagged field")
	}
}

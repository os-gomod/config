package schema

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	configerrors "github.com/os-gomod/config/errors"
)

// Generator produces JSON Schema definitions from Go struct types using
// reflection. It supports struct tags (json, default, description, validate)
// and special handling for time.Duration, time.Time, slices, maps, and nested structs.
type Generator struct {
	title       string
	description string
}

// New creates a new Schema Generator with the given options.
func New(opts ...Option) *Generator {
	g := &Generator{}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Option configures a Generator.
type Option func(*Generator)

// WithTitle sets the schema title (top-level).
func WithTitle(title string) Option {
	return func(g *Generator) { g.title = title }
}

// WithDescription sets the schema description (top-level).
func WithDescription(desc string) Option {
	return func(g *Generator) { g.description = desc }
}

// Generate produces a JSON Schema from the given struct value or struct pointer.
// Returns an error if the value is nil or not a struct (or pointer to struct).
func (g *Generator) Generate(v any) (*Schema, error) {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil, configerrors.New(
			configerrors.CodeInvalidConfig,
			"cannot generate schema for nil",
		)
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, configerrors.Newf(
			configerrors.CodeInvalidConfig,
			"expected struct type, got %s",
			t.Kind(),
		)
	}
	schema := g.generateStruct(t)
	schema.Title = g.title
	schema.Description = g.description
	return schema, nil
}

// generateStruct creates an object schema with properties for each exported struct field.
func (g *Generator) generateStruct(t reflect.Type) *Schema {
	schema := &Schema{
		Type:       "object",
		Properties: make(map[string]*Schema),
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := field.Tag.Get("json")
		if name == "" || name == "-" {
			name = toSnakeCase(field.Name)
		} else {
			if idx := strings.IndexByte(name, ','); idx >= 0 {
				name = name[:idx]
			}
		}
		propSchema := g.generateField(&field)
		schema.Properties[name] = propSchema
		if def := field.Tag.Get("default"); def != "" {
			propSchema.Default = def
		}
		if desc := field.Tag.Get("description"); desc != "" {
			propSchema.Description = desc
		}
		validateTag := field.Tag.Get("validate")
		if validateTag != "" {
			g.applyValidationTags(propSchema, validateTag, name, schema)
		}
	}
	return schema
}

// generateField produces a schema for a single struct field.
func (g *Generator) generateField(field *reflect.StructField) *Schema {
	ft := field.Type
	return g.generateType(ft)
}

// generateType produces a schema based on the Go type. Special cases:
//   - time.Duration -> string, format "duration"
//   - time.Time -> string, format "date-time"
//   - Slice -> array with items schema
//   - Map with string keys -> object with additionalProperties
func (g *Generator) generateType(t reflect.Type) *Schema {
	if t.String() == "time.Duration" {
		return &Schema{Type: "string", Format: "duration"}
	}
	if t == reflect.TypeOf(time.Time{}) {
		return &Schema{Type: "string", Format: "date-time"}
	}
	switch t.Kind() {
	case reflect.String:
		return &Schema{Type: "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &Schema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return &Schema{Type: "number"}
	case reflect.Bool:
		return &Schema{Type: "boolean"}
	case reflect.Slice:
		return &Schema{
			Type:  "array",
			Items: g.generateType(t.Elem()),
		}
	case reflect.Map:
		if t.Key().Kind() == reflect.String {
			return &Schema{
				Type:                 "object",
				AdditionalProperties: g.generateType(t.Elem()),
			}
		}
		return &Schema{Type: "object"}
	case reflect.Struct:
		return g.generateStruct(t)
	case reflect.Ptr:
		return g.generateType(t.Elem())
	default:
		return &Schema{Type: "string"}
	}
}

// applyValidationTags parses validator-style tags and applies them as schema
// constraints. Supported rules: required, min=, max=, oneof=.
func (g *Generator) applyValidationTags(propSchema *Schema, tag, fieldName string, parent *Schema) {
	rules := strings.Split(tag, ",")
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		if rule == "required" {
			parent.Required = append(parent.Required, fieldName)
			continue
		}
		if strings.HasPrefix(rule, "min=") {
			val := strings.TrimPrefix(rule, "min=")
			if propSchema.Type == "string" {
				n := parseInt(val)
				propSchema.MinLength = &n
			} else {
				f := parseFloat(val)
				propSchema.Minimum = &f
			}
			continue
		}
		if strings.HasPrefix(rule, "max=") {
			val := strings.TrimPrefix(rule, "max=")
			if propSchema.Type == "string" {
				n := parseInt(val)
				propSchema.MaxLength = &n
			} else {
				f := parseFloat(val)
				propSchema.Maximum = &f
			}
			continue
		}
		if strings.HasPrefix(rule, "oneof=") {
			val := strings.TrimPrefix(rule, "oneof=")
			opts := strings.Split(val, " ")
			enum := make([]any, len(opts))
			for i, o := range opts {
				enum[i] = o
			}
			propSchema.Enum = enum
			continue
		}
	}
}

// toSnakeCase converts a CamelCase string to snake_case.
func toSnakeCase(s string) string {
	var result []byte
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		if r >= 'A' && r <= 'Z' {
			result = append(result, byte(r-'A'+'a'))
		} else if r >= 0 && r <= unicodeMaxASCII {
			result = append(result, byte(r))
		}
	}
	return string(result)
}

// unicodeMaxASCII is the maximum value for ASCII-range byte conversion.
const unicodeMaxASCII = 255

// parseInt parses an integer from a string, returning 0 on failure.
func parseInt(s string) int {
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}

// parseFloat parses a float from a string, returning 0 on failure.
func parseFloat(s string) float64 {
	var f float64
	_, _ = fmt.Sscanf(s, "%f", &f)
	return f
}

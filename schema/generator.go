package schema

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	_errors "github.com/os-gomod/config/errors"
)

// Generator builds JSON Schema documents from Go struct types via reflection.
type Generator struct {
	title       string
	description string
}

// New returns a Generator.
func New(opts ...Option) *Generator {
	g := &Generator{}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Option configures a Generator.
type Option func(*Generator)

// WithTitle sets the root schema title.
func WithTitle(title string) Option {
	return func(g *Generator) { g.title = title }
}

// WithDescription sets the root schema description.
func WithDescription(desc string) Option {
	return func(g *Generator) { g.description = desc }
}

// Generate returns the JSON Schema for v (struct or pointer to struct).
// Returns an error if v is not a struct type.
func (g *Generator) Generate(v any) (*Schema, error) {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil, _errors.New(
			_errors.CodeInvalidConfig,
			"cannot generate schema for nil",
		)
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, _errors.Newf(
			_errors.CodeInvalidConfig,
			"expected struct type, got %s",
			t.Kind(),
		)
	}
	schema := g.generateStruct(t)
	schema.Title = g.title
	schema.Description = g.description
	return schema, nil
}

// generateStruct produces a JSON Schema object for the given struct type.
func (g *Generator) generateStruct(t reflect.Type) *Schema {
	schema := &Schema{
		Type:       "object",
		Properties: make(map[string]*Schema),
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue // skip unexported
		}
		name := field.Tag.Get("json")
		if name == "" || name == "-" {
			name = toSnakeCase(field.Name)
		} else {
			// Strip omitempty.
			if idx := strings.IndexByte(name, ','); idx >= 0 {
				name = name[:idx]
			}
		}
		propSchema := g.generateField(field)
		schema.Properties[name] = propSchema

		// Handle "default" tag.
		if def := field.Tag.Get("default"); def != "" {
			propSchema.Default = def
		}

		// Handle "description" tag.
		if desc := field.Tag.Get("description"); desc != "" {
			propSchema.Description = desc
		}

		// Handle "validate" tag.
		validateTag := field.Tag.Get("validate")
		if validateTag != "" {
			g.applyValidationTags(propSchema, validateTag, name, schema)
		}
	}
	return schema
}

// generateField returns a Schema for the given struct field.
func (g *Generator) generateField(field reflect.StructField) *Schema {
	ft := field.Type
	return g.generateType(ft)
}

// generateType returns a Schema for the given reflect.Type.
func (g *Generator) generateType(t reflect.Type) *Schema {
	// Handle time.Duration specially.
	if t.String() == "time.Duration" {
		return &Schema{Type: "string", Format: "duration"}
	}
	// Handle time.Time specially.
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

// applyValidationTags parses the validate tag and applies schema constraints.
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

// toSnakeCase converts CamelCase to snake_case.
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
		// Non-ASCII runes are skipped to avoid overflow.
	}
	return string(result)
}

// unicodeMaxASCII is the maximum rune value that safely converts to a byte without overflow.
const unicodeMaxASCII = 255

// parseInt parses a string as an int, returning 0 on failure.
func parseInt(s string) int {
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}

// parseFloat parses a string as a float64, returning 0 on failure.
func parseFloat(s string) float64 {
	var f float64
	_, _ = fmt.Sscanf(s, "%f", &f)
	return f
}

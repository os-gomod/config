// Package schema provides infrastructure for generating configuration schemas
// from Go struct types using reflection. Schemas describe the structure, types,
// and constraints of configuration objects for documentation and validation.
package schema

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Schema
// ---------------------------------------------------------------------------

// Schema describes the structure of a configuration object.
type Schema struct {
	Name     string   // Schema name (typically the struct type name).
	Type     string   // Top-level type ("struct", "map", etc.).
	Fields   []Field  // Fields in the schema.
	Required []string // Names of required fields.
}

// Field describes a single field in a configuration schema.
type Field struct {
	Name     string            // Field name (from config tag or struct field name).
	Type     string            // Go type name.
	Kind     string            // Reflect kind (string, int, struct, etc.).
	Required bool              // Whether the field is required.
	Default  any               // Default value (from tag or zero value).
	Tags     map[string]string // All struct tags for the field.
	Doc      string            // Documentation comment.
	Fields   []Field           // Nested fields (for struct types).
}

// ---------------------------------------------------------------------------
// Generator
// ---------------------------------------------------------------------------

// Generator creates configuration schemas from Go struct types using reflection.
// It is instance-based — NO global generator.
type Generator struct{}

// New creates a new Schema Generator.
func New() *Generator {
	return &Generator{}
}

// Generate creates a Schema from the given value. The value must be a struct
// or a pointer to a struct. The schema includes all exported fields with
// their types, tags, and requirement information.
func (g *Generator) Generate(v any) (*Schema, error) {
	if v == nil {
		return nil, errors.New("cannot generate schema from nil")
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil, errors.New("cannot generate schema from nil pointer")
		}
		rv = rv.Elem()
	}

	rt := rv.Type()
	if rt.Kind() != reflect.Struct {
		return nil, fmt.Errorf("schema generation requires a struct, got %s", rt.Kind())
	}

	schema := &Schema{
		Name:   rt.Name(),
		Type:   "struct",
		Fields: make([]Field, 0, rt.NumField()),
	}

	for i := range rt.NumField() {
		field := rt.Field(i)

		// Skip unexported fields.
		if !field.IsExported() {
			continue
		}

		f := g.generateField(field)
		schema.Fields = append(schema.Fields, f)

		if f.Required {
			schema.Required = append(schema.Required, f.Name)
		}
	}

	return schema, nil
}

// generateField creates a Field descriptor from a reflect.StructField.
func (g *Generator) generateField(sf reflect.StructField) Field {
	f := Field{
		Name:   fieldName(sf),
		Type:   typeName(sf.Type),
		Kind:   sf.Type.Kind().String(),
		Tags:   parseTags(sf.Tag),
		Fields: make([]Field, 0),
		Doc:    sf.Tag.Get("doc"),
	}

	// Parse the "config" tag for required/default.
	configTag := sf.Tag.Get("config")
	if configTag != "" {
		parts := strings.Split(configTag, ",")
		for _, part := range parts[1:] {
			part = strings.TrimSpace(part)
			if part == "required" {
				f.Required = true
			}
			if strings.HasPrefix(part, "default=") {
				f.Default = strings.TrimPrefix(part, "default=")
			}
		}
	}

	// If the field is a struct (but not special types), generate nested fields.
	if sf.Type.Kind() == reflect.Struct && !isSpecialType(sf.Type) {
		for i := range sf.Type.NumField() {
			nestedField := sf.Type.Field(i)
			if !nestedField.IsExported() {
				continue
			}
			f.Fields = append(f.Fields, g.generateField(nestedField))
		}
	}

	return f
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// fieldName extracts the config key name from a struct field.
// Uses the "config" tag if present, otherwise the lowercase field name.
func fieldName(sf reflect.StructField) string {
	tag := sf.Tag.Get("config")
	if tag == "" {
		return strings.ToLower(sf.Name)
	}
	parts := strings.Split(tag, ",")
	if parts[0] == "" {
		return strings.ToLower(sf.Name)
	}
	return parts[0]
}

// typeName returns a human-readable type name for the given reflect.Type.
func typeName(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Ptr:
		return "*" + typeName(t.Elem())
	case reflect.Slice:
		return "[]" + typeName(t.Elem())
	case reflect.Map:
		return fmt.Sprintf("map[%s]%s", typeName(t.Key()), typeName(t.Elem()))
	case reflect.Struct:
		if isSpecialType(t) {
			return t.String()
		}
		return t.Name()
	case reflect.Interface:
		return "any"
	default:
		return t.String()
	}
}

// parseTags extracts all struct tags into a map.
func parseTags(tag reflect.StructTag) map[string]string {
	result := make(map[string]string)
	for _, key := range []string{"config", "json", "yaml", "toml", "validate", "doc", "default"} {
		if val, ok := tag.Lookup(key); ok {
			result[key] = val
		}
	}
	return result
}

// isSpecialType returns true for types that should not be recursed into
// during schema generation.
func isSpecialType(t reflect.Type) bool {
	// time.Time is a struct but should be treated as a leaf type.
	var tm time.Time
	if t == reflect.TypeOf(tm) {
		return true
	}
	// time.Duration is actually an int64.
	var d time.Duration
	return t == reflect.TypeOf(d)
}

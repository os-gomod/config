// Package schema provides JSON Schema generation from Go struct types.
// It reflects on struct fields and tags to produce a Schema object that
// describes the configuration structure, including types, defaults,
// validation constraints, and descriptions.
//
// Supported struct tags:
//   - json: field name and omitempty
//   - default: default value
//   - description: field description
//   - validate: validation rules (required, min, max, oneof)
package schema

import (
	"encoding/json"
	"io"
)

// Schema represents a JSON Schema object that describes the structure of
// a configuration type. It supports object, array, string, integer, number,
// and boolean types with optional constraints and metadata.
type Schema struct {
	Type                 string             `json:"type,omitempty"`
	Title                string             `json:"title,omitempty"`
	Description          string             `json:"description,omitempty"`
	Default              any                `json:"default,omitempty"`
	Enum                 []any              `json:"enum,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	AdditionalProperties *Schema            `json:"additionalProperties,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	Format               string             `json:"format,omitempty"`
	Minimum              *float64           `json:"minimum,omitempty"`
	Maximum              *float64           `json:"maximum,omitempty"`
	MinLength            *int               `json:"minLength,omitempty"`
	MaxLength            *int               `json:"maxLength,omitempty"`
	Pattern              string             `json:"pattern,omitempty"`
}

// MarshalJSON returns a pretty-printed JSON representation of the schema
// with 2-space indentation.
func (s *Schema) MarshalJSON() ([]byte, error) {
	type Alias Schema
	return json.MarshalIndent((*Alias)(s), "", "  ")
}

// WriteTo writes the pretty-printed JSON schema to the given writer,
// followed by a newline. Returns the number of bytes written.
func (s *Schema) WriteTo(w io.Writer) (int64, error) {
	data, err := s.MarshalJSON()
	if err != nil {
		return 0, err
	}
	n, writeErr := w.Write(data)
	if writeErr != nil {
		return 0, writeErr
	}
	if _, newlineErr := w.Write([]byte("\n")); newlineErr != nil {
		return int64(n), newlineErr
	}
	return int64(n + 1), nil
}

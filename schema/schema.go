// Package schema generates JSON Schema (draft-07) documents from annotated
// Go structs. It reads "config", "default", "validate", and "description"
// struct tags to produce schemas suitable for CI linting and IDE autocompletion.
package schema

import (
	"encoding/json"
	"io"
)

// Schema represents a JSON Schema (draft-07) node.
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

// MarshalJSON returns the schema as indented JSON.
func (s *Schema) MarshalJSON() ([]byte, error) {
	// Use the standard json.Marshal with the struct tags defined above.
	// We wrap it with indentation for readability.
	type Alias Schema
	return json.MarshalIndent((*Alias)(s), "", "  ")
}

// WriteTo writes the schema as indented JSON to w.
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

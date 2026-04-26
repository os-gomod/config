package schema

import (
	"fmt"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Generator
// ---------------------------------------------------------------------------

// Generator is defined in schema.go. This file provides additional
// generation methods and rendering utilities.

// HasField returns true if the schema has a field with the given name.
func (s *Schema) HasField(name string) bool {
	for _, f := range s.Fields {
		if f.Name == name {
			return true
		}
	}
	return false
}

// GetField returns the field with the given name, or nil if not found.
func (s *Schema) GetField(name string) *Field {
	for i := range s.Fields {
		if s.Fields[i].Name == name {
			return &s.Fields[i]
		}
	}
	return nil
}

// FieldNames returns the names of all fields in order.
func (s *Schema) FieldNames() []string {
	names := make([]string, len(s.Fields))
	for i, f := range s.Fields {
		names[i] = f.Name
	}
	return names
}

// RequiredFieldNames returns the names of all required fields.
func (s *Schema) RequiredFieldNames() []string {
	result := make([]string, len(s.Required))
	copy(result, s.Required)
	sort.Strings(result)
	return result
}

// Len returns the number of fields.
func (s *Schema) Len() int {
	return len(s.Fields)
}

// IsEmpty returns true if the schema has no fields.
func (s *Schema) IsEmpty() bool {
	return len(s.Fields) == 0
}

// ---------------------------------------------------------------------------
// Schema rendering
// ---------------------------------------------------------------------------

// String returns a human-readable representation of the schema.
func (s *Schema) String() string {
	var b strings.Builder

	fmt.Fprintf(&b, "Schema: %s (%s)\n", s.Name, s.Type)
	b.WriteString(strings.Repeat("=", 40+len(s.Name)) + "\n\n")

	if len(s.Required) > 0 {
		sort.Strings(s.Required)
		b.WriteString("Required: " + strings.Join(s.Required, ", ") + "\n\n")
	}

	for _, f := range s.Fields {
		renderField(&b, f, "  ")
		b.WriteString("\n")
	}

	return b.String()
}

// renderField writes a field's representation to the builder.
func renderField(b *strings.Builder, f Field, indent string) {
	// Field name and type.
	req := ""
	if f.Required {
		req = " (required)"
	}
	fmt.Fprintf(b, "%s%s: %s%s\n", indent, f.Name, f.Type, req)

	// Default value.
	if f.Default != nil {
		fmt.Fprintf(b, "%s  default: %v\n", indent, f.Default)
	}

	// Documentation.
	if f.Doc != "" {
		fmt.Fprintf(b, "%s  doc: %s\n", indent, f.Doc)
	}

	// Tags (excluding doc and default which are shown separately).
	if len(f.Tags) > 0 {
		var tagParts []string
		for k, v := range f.Tags {
			if k == "doc" {
				continue
			}
			tagParts = append(tagParts, fmt.Sprintf("%s:%q", k, v))
		}
		sort.Strings(tagParts)
		if len(tagParts) > 0 {
			fmt.Fprintf(b, "%s  tags: %s\n", indent, strings.Join(tagParts, " "))
		}
	}

	// Nested fields.
	if len(f.Fields) > 0 {
		fmt.Fprintf(b, "%s  {\n", indent)
		for _, nested := range f.Fields {
			renderField(b, nested, indent+"    ")
		}
		fmt.Fprintf(b, "%s  }\n", indent)
	}
}

// ---------------------------------------------------------------------------
// Schema comparison and validation
// ---------------------------------------------------------------------------

// Validate checks that the schema is well-formed (no duplicate field names,
// required fields reference existing fields, etc.).
func (s *Schema) Validate() []string {
	var issues []string

	seen := make(map[string]int)
	for i, f := range s.Fields {
		if prev, exists := seen[f.Name]; exists {
			issues = append(issues,
				fmt.Sprintf("duplicate field name %q at positions %d and %d",
					f.Name, prev, i))
		}
		seen[f.Name] = i
	}

	for _, req := range s.Required {
		if _, exists := seen[req]; !exists {
			issues = append(issues,
				fmt.Sprintf("required field %q does not exist in schema", req))
		}
	}

	return issues
}

// ---------------------------------------------------------------------------
// Field methods
// ---------------------------------------------------------------------------

// IsLeaf returns true if the field has no nested fields.
func (f *Field) IsLeaf() bool {
	return len(f.Fields) == 0
}

// HasTag returns true if the field has the given tag.
func (f *Field) HasTag(key string) bool {
	_, exists := f.Tags[key]
	return exists
}

// GetTag returns the value of a tag, or "" if not present.
func (f *Field) GetTag(key string) string {
	if f.Tags == nil {
		return ""
	}
	return f.Tags[key]
}

// ---------------------------------------------------------------------------
// Schema merging
// ---------------------------------------------------------------------------

// Merge combines two schemas. Fields from the overlay take precedence
// when names conflict. Required fields are the union of both schemas.
func Merge(base, overlay *Schema) *Schema {
	if base == nil {
		return overlay
	}
	if overlay == nil {
		return base
	}

	result := &Schema{
		Name:     overlay.Name,
		Type:     overlay.Type,
		Fields:   make([]Field, 0),
		Required: make([]string, 0),
	}

	// Collect all fields, overlay overrides base.
	fieldMap := make(map[string]Field)
	for _, f := range base.Fields {
		fieldMap[f.Name] = f
	}
	for _, f := range overlay.Fields {
		fieldMap[f.Name] = f
	}

	// Convert to ordered slice.
	for _, f := range fieldMap {
		result.Fields = append(result.Fields, f)
	}

	// Sort fields by name for deterministic output.
	sort.Slice(result.Fields, func(i, j int) bool {
		return result.Fields[i].Name < result.Fields[j].Name
	})

	// Union of required fields.
	reqSet := make(map[string]struct{})
	for _, r := range base.Required {
		reqSet[r] = struct{}{}
	}
	for _, r := range overlay.Required {
		reqSet[r] = struct{}{}
	}
	for r := range reqSet {
		result.Required = append(result.Required, r)
	}
	sort.Strings(result.Required)

	return result
}

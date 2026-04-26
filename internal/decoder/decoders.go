package decoder

import (
	"encoding/json"
	"fmt"

	"github.com/BurntSushi/toml"

	yamlv3 "gopkg.in/yaml.v3"

	"github.com/os-gomod/config/v2/internal/domain/errors"
)

// ---------------------------------------------------------------------------
// YAMLDecoder
// ---------------------------------------------------------------------------

// YAMLDecoder decodes YAML-formatted bytes into maps.
type YAMLDecoder struct{}

// NewYAMLDecoder creates a new YAML decoder.
func NewYAMLDecoder() *YAMLDecoder {
	return &YAMLDecoder{}
}

// Decode parses YAML bytes and returns the result as a map[string]any.
//
//nolint:dupl // decoder pattern is intentionally similar
func (d *YAMLDecoder) Decode(src []byte) (map[string]any, error) {
	if len(src) == 0 {
		return make(map[string]any), nil
	}

	var raw any
	if err := yamlv3.Unmarshal(src, &raw); err != nil {
		return nil, errors.Build(errors.CodeParseError,
			"YAML decode failed",
			errors.WithOperation("yaml.decode")).
			Wrap(err)
	}

	result, ok := normalizeMap(raw)
	if !ok {
		return nil, errors.New(errors.CodeTypeMismatch,
			"YAML root must be a mapping (object), got scalar or array")
	}

	return result, nil
}

// MediaType returns "application/yaml".
func (d *YAMLDecoder) MediaType() string {
	return "application/yaml"
}

// Extensions returns the file extensions handled by this decoder.
func (d *YAMLDecoder) Extensions() []string {
	return []string{".yaml", ".yml"}
}

// ---------------------------------------------------------------------------
// JSONDecoder
// ---------------------------------------------------------------------------

// JSONDecoder decodes JSON-formatted bytes into maps.
type JSONDecoder struct{}

// NewJSONDecoder creates a new JSON decoder.
func NewJSONDecoder() *JSONDecoder {
	return &JSONDecoder{}
}

// Decode parses JSON bytes and returns the result as a map[string]any.
//
//nolint:dupl // decoder pattern is intentionally similar
func (d *JSONDecoder) Decode(src []byte) (map[string]any, error) {
	if len(src) == 0 {
		return make(map[string]any), nil
	}

	var raw any
	if err := json.Unmarshal(src, &raw); err != nil {
		return nil, errors.Build(errors.CodeParseError,
			"JSON decode failed",
			errors.WithOperation("json.decode")).
			Wrap(err)
	}

	result, ok := normalizeMap(raw)
	if !ok {
		return nil, errors.New(errors.CodeTypeMismatch,
			"JSON root must be an object, got scalar or array")
	}

	return result, nil
}

// MediaType returns "application/json".
func (d *JSONDecoder) MediaType() string {
	return "application/json"
}

// Extensions returns the file extensions handled by this decoder.
func (d *JSONDecoder) Extensions() []string {
	return []string{".json"}
}

// ---------------------------------------------------------------------------
// TOMLDecoder
// ---------------------------------------------------------------------------

// TOMLDecoder decodes TOML-formatted bytes into maps.
type TOMLDecoder struct{}

// NewTOMLDecoder creates a new TOML decoder.
func NewTOMLDecoder() *TOMLDecoder {
	return &TOMLDecoder{}
}

// Decode parses TOML bytes and returns the result as a map[string]any.
func (d *TOMLDecoder) Decode(src []byte) (map[string]any, error) {
	if len(src) == 0 {
		return make(map[string]any), nil
	}

	var result map[string]any
	if _, err := toml.Decode(string(src), &result); err != nil {
		return nil, errors.Build(errors.CodeParseError,
			"TOML decode failed",
			errors.WithOperation("toml.decode")).
			Wrap(err)
	}

	if result == nil {
		return make(map[string]any), nil
	}

	return result, nil
}

// MediaType returns "application/toml".
func (d *TOMLDecoder) MediaType() string {
	return "application/toml"
}

// Extensions returns the file extensions handled by this decoder.
func (d *TOMLDecoder) Extensions() []string {
	return []string{".toml"}
}

// ---------------------------------------------------------------------------
// Helper: normalizeMap
// ---------------------------------------------------------------------------

// normalizeMap converts a decoded value to map[string]any, handling
// the case where YAML/JSON may produce map[interface{}]any.
func normalizeMap(raw any) (map[string]any, bool) {
	switch v := raw.(type) {
	case map[string]any:
		normalizeMapValues(v)
		return v, true
	case map[any]any:
		result := make(map[string]any, len(v))
		for key, val := range v {
			strKey, ok := key.(string)
			if !ok {
				// Non-string key in YAML map — convert via fmt.Sprint.
				strKey = fmt.Sprintf("%v", key)
			}
			result[strKey] = normalizeValue(val)
		}
		return result, true
	default:
		return nil, false
	}
}

// normalizeValue recursively normalizes values, converting map[any]any to
// map[string]any throughout the tree.
func normalizeValue(raw any) any {
	switch v := raw.(type) {
	case map[any]any:
		result := make(map[string]any, len(v))
		for key, val := range v {
			strKey, ok := key.(string)
			if !ok {
				strKey = fmt.Sprintf("%v", key)
			}
			result[strKey] = normalizeValue(val)
		}
		return result
	case []any:
		for i, item := range v {
			v[i] = normalizeValue(item)
		}
		return v
	default:
		return raw
	}
}

// normalizeMapValues recursively normalizes the values in a map[string]any.
func normalizeMapValues(m map[string]any) {
	for k, v := range m {
		m[k] = normalizeValue(v)
	}
}

package decoder

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// YAMLDecoder decodes YAML bytes into a flat key-value map.
// It handles nested mappings, sequences, and scalar values.
type YAMLDecoder struct{}

var _ Decoder = (*YAMLDecoder)(nil)

// NewYAMLDecoder returns a YAMLDecoder.
func NewYAMLDecoder() *YAMLDecoder { return &YAMLDecoder{} }

// Decode parses YAML bytes into a flat, dot-separated key-value map.
func (d *YAMLDecoder) Decode(src []byte) (map[string]any, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(src, &raw); err != nil {
		return nil, fmt.Errorf("yaml decode: %w", err)
	}
	out := make(map[string]any, len(raw))
	flatten("", raw, out)
	return out, nil
}

// MediaType returns the MIME type for YAML.
func (d *YAMLDecoder) MediaType() string { return "application/x-yaml" }

// Extensions returns the file extensions for YAML.
func (d *YAMLDecoder) Extensions() []string { return []string{".yaml", ".yml"} }

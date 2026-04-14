package decoder

import (
	"encoding/json"
	"fmt"
)

// JSONDecoder decodes JSON bytes into a flat key-value map.
type JSONDecoder struct{}

var _ Decoder = (*JSONDecoder)(nil)

// NewJSONDecoder returns a JSONDecoder.
func NewJSONDecoder() *JSONDecoder { return &JSONDecoder{} }

// Decode parses JSON bytes into a flat, dot-separated key-value map.
func (d *JSONDecoder) Decode(src []byte) (map[string]any, error) {
	var raw map[string]any
	if err := json.Unmarshal(src, &raw); err != nil {
		return nil, fmt.Errorf("json decode: %w", err)
	}
	out := make(map[string]any, len(raw))
	flatten("", raw, out)
	return out, nil
}

// MediaType returns the MIME type for JSON.
func (d *JSONDecoder) MediaType() string { return "application/json" }

// Extensions returns the file extensions for JSON.
func (d *JSONDecoder) Extensions() []string { return []string{".json"} }

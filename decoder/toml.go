//go:build toml

// Package decoder — TOML support.
// Build with -tags toml to include TOML decoding.
// Requires: github.com/BurntSushi/toml
package decoder

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

// TOMLDecoder decodes TOML bytes into a flat key-value map.
// Available only when compiled with the "toml" build tag.
type TOMLDecoder struct{}

var _ Decoder = (*TOMLDecoder)(nil)

// NewTOMLDecoder returns a TOMLDecoder.
func NewTOMLDecoder() *TOMLDecoder { return &TOMLDecoder{} }

// Decode parses TOML bytes into a flat, dot-separated key-value map.
func (d *TOMLDecoder) Decode(src []byte) (map[string]any, error) {
	var raw map[string]any
	if err := toml.Unmarshal(src, &raw); err != nil {
		return nil, fmt.Errorf("toml decode: %w", err)
	}
	out := make(map[string]any, len(raw))
	flatten("", raw, out)
	return out, nil
}

// MediaType returns the MIME type for TOML.
func (d *TOMLDecoder) MediaType() string { return "application/toml" }

// Extensions returns the file extensions for TOML.
func (d *TOMLDecoder) Extensions() []string { return []string{".toml"} }

package decoder

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"gopkg.in/yaml.v3"

	"github.com/os-gomod/config/internal/keyutil"
)

// YAMLDecoder decodes YAML content into a flat key-value map.
type YAMLDecoder struct{}

var _ Decoder = (*YAMLDecoder)(nil)

// NewYAMLDecoder creates a new YAML decoder.
func NewYAMLDecoder() *YAMLDecoder { return &YAMLDecoder{} }

// Decode parses YAML bytes and returns a flattened map with lowercased keys.
func (d *YAMLDecoder) Decode(src []byte) (map[string]any, error) {
	return decodeAndFlatten(src, "yaml", func(raw *map[string]any) error {
		err := yaml.Unmarshal(src, raw)
		return err
	})
}

// MediaType returns the MIME type for YAML.
func (d *YAMLDecoder) MediaType() string { return "application/x-yaml" }

// Extensions returns the file extensions associated with YAML.
func (d *YAMLDecoder) Extensions() []string { return []string{".yaml", ".yml"} }

// JSONDecoder decodes JSON content into a flat key-value map.
type JSONDecoder struct{}

var _ Decoder = (*JSONDecoder)(nil)

// NewJSONDecoder creates a new JSON decoder.
func NewJSONDecoder() *JSONDecoder { return &JSONDecoder{} }

// Decode parses JSON bytes and returns a flattened map with lowercased keys.
func (d *JSONDecoder) Decode(src []byte) (map[string]any, error) {
	return decodeAndFlatten(src, "json", func(raw *map[string]any) error {
		return json.Unmarshal(src, raw)
	})
}

// MediaType returns the MIME type for JSON.
func (d *JSONDecoder) MediaType() string { return "application/json" }

// Extensions returns the file extensions associated with JSON.
func (d *JSONDecoder) Extensions() []string { return []string{".json"} }

// TOMLDecoder decodes TOML content into a flat key-value map.
type TOMLDecoder struct{}

var _ Decoder = (*TOMLDecoder)(nil)

// NewTOMLDecoder creates a new TOML decoder.
func NewTOMLDecoder() *TOMLDecoder { return &TOMLDecoder{} }

// Decode parses TOML bytes and returns a flattened map with lowercased keys.
func (d *TOMLDecoder) Decode(src []byte) (map[string]any, error) {
	return decodeAndFlatten(src, "toml", func(raw *map[string]any) error {
		_, err := toml.Decode(string(src), raw)
		return err
	})
}

// MediaType returns the MIME type for TOML.
func (d *TOMLDecoder) MediaType() string { return "application/toml" }

// Extensions returns the file extensions associated with TOML.
func (d *TOMLDecoder) Extensions() []string { return []string{".toml"} }

// HCLDecoder decodes HCL content into a flat key-value map.
type HCLDecoder struct{}

var _ Decoder = (*HCLDecoder)(nil)

// NewHCLDecoder creates a new HCL decoder.
func NewHCLDecoder() *HCLDecoder { return &HCLDecoder{} }

// Decode parses HCL bytes and returns a flattened map with lowercased keys.
func (d *HCLDecoder) Decode(src []byte) (map[string]any, error) {
	return decodeAndFlatten(src, "hcl", func(raw *map[string]any) error {
		return hclsimple.Decode("config.hcl", src, nil, raw)
	})
}

// MediaType returns the MIME type for HCL.
func (d *HCLDecoder) MediaType() string { return "application/hcl" }

// Extensions returns the file extensions associated with HCL.
func (d *HCLDecoder) Extensions() []string { return []string{".hcl"} }

// unmarshalFunc is a function that unmarshals source bytes into a map.
type unmarshalFunc func(raw *map[string]any) error

// decodeAndFlatten is a shared helper that unmarshals content and flattens
// the result into a dot-separated map with lowercased keys.
// This eliminates the identical Decode() pattern previously duplicated
// across yaml, json, toml, and hcl decoders.
func decodeAndFlatten(_ []byte, name string, unmarshal unmarshalFunc) (map[string]any, error) {
	var raw map[string]any
	if err := unmarshal(&raw); err != nil {
		return nil, fmt.Errorf("%s decode: %w", name, err)
	}
	return keyutil.FlattenMapLower(raw), nil
}

// INIDecoder decodes INI-style content into a flat key-value map.
// Lines starting with # or ; are treated as comments. Section headers
// (e.g., [section]) are prefixed to keys as "section.key".
type INIDecoder struct{}

var _ Decoder = (*INIDecoder)(nil)

// NewINIDecoder creates a new INI decoder.
func NewINIDecoder() *INIDecoder { return &INIDecoder{} }

// Decode parses INI bytes and returns a flattened map with lowercased keys.
func (d *INIDecoder) Decode(src []byte) (map[string]any, error) {
	out := make(map[string]any)
	section := ""
	lines := strings.Split(string(src), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if section != "" {
			key = section + "." + key
		}
		key = strings.ToLower(key)
		out[key] = val
	}
	return out, nil
}

// MediaType returns the MIME type for INI.
func (d *INIDecoder) MediaType() string { return "text/x-ini" }

// Extensions returns the file extensions associated with INI.
func (d *INIDecoder) Extensions() []string { return []string{".ini"} }

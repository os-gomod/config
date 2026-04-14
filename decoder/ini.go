package decoder

import (
	"strings"
)

// INIDecoder decodes INI-format bytes into a flat key-value map.
// Section names become key prefixes: [db]\nhost=x -> db.host=x.
type INIDecoder struct{}

var _ Decoder = (*INIDecoder)(nil)

// NewINIDecoder returns an INIDecoder.
func NewINIDecoder() *INIDecoder { return &INIDecoder{} }

// Decode parses INI-format bytes into a flat, dot-separated key-value map.
// Section headers ([section]) become key prefixes. Lines starting with # or ;
// are comments. Keys and values are trimmed of whitespace.
func (d *INIDecoder) Decode(src []byte) (map[string]any, error) {
	out := make(map[string]any)
	var section string
	lines := strings.Split(string(src), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Comment lines.
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		// Section header.
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.ToLower(line[1 : len(line)-1]))
			continue
		}
		// Key = Value pair.
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(line[:idx]))
		val := strings.TrimSpace(line[idx+1:])
		if section != "" {
			key = section + "." + key
		}
		out[key] = val
	}
	return out, nil
}

// MediaType returns the MIME type for INI files.
func (d *INIDecoder) MediaType() string { return "text/x-ini" }

// Extensions returns the file extensions for INI files.
func (d *INIDecoder) Extensions() []string { return []string{".ini"} }

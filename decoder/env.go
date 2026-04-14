package decoder

import (
	"fmt"
	"strings"
)

// EnvDecoder decodes dotenv-format bytes (KEY=VALUE pairs) into a flat key-value map.
// Lines starting with # are comments. Quoted values are unquoted.
// Keys are lowercased and underscores are replaced with dots.
type EnvDecoder struct{}

var _ Decoder = (*EnvDecoder)(nil)

// NewEnvDecoder returns an EnvDecoder.
func NewEnvDecoder() *EnvDecoder { return &EnvDecoder{} }

// Decode parses dotenv-format bytes into a flat, dot-separated key-value map.
// Blank lines and lines starting with # are ignored. Values surrounded by
// single or double quotes are unquoted. Keys are normalised by lowercasing
// and replacing underscores with dots: DB_HOST -> db.host.
func (d *EnvDecoder) Decode(src []byte) (map[string]any, error) {
	out := make(map[string]any)
	lines := strings.Split(string(src), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		val = unquote(val)
		normalisedKey := strings.ToLower(strings.ReplaceAll(key, "_", "."))
		out[normalisedKey] = val
	}
	return out, nil
}

// MediaType returns the MIME type for dotenv files.
func (d *EnvDecoder) MediaType() string { return "text/x-env" }

// Extensions returns the file extensions for dotenv files.
func (d *EnvDecoder) Extensions() []string { return []string{".env"} }

// unquote strips surrounding single or double quotes from a string value.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// Compile-time check for EnvDecoder.
var _ Decoder = (*EnvDecoder)(nil)

// ensure EnvDecoder satisfies Decoder at compile time — already done above,
// this comment prevents unused-import-style lint noise.
var _ = fmt.Sprintf

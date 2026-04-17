package decoder

import (
	"strings"
)

// EnvDecoder decodes .env file format into a flat key-value map.
// Keys are normalized: underscores become dots, and values are unquoted.
//
// Example .env file:
//
//	DB_HOST=localhost
//	DB_PORT=5432
//	APP_NAME="my app"
//
// Produces:
//
//	{"db.host": "localhost", "db.port": "5432", "app.name": "my app"}
type EnvDecoder struct{}

var _ Decoder = (*EnvDecoder)(nil)

// NewEnvDecoder creates a new ENV decoder.
func NewEnvDecoder() *EnvDecoder { return &EnvDecoder{} }

// Decode parses .env bytes and returns a flattened map.
// Lines starting with # are treated as comments.
// Values surrounded by single or double quotes are unquoted.
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

// MediaType returns the MIME type for ENV files.
func (d *EnvDecoder) MediaType() string { return "text/x-env" }

// Extensions returns the file extensions associated with ENV files.
func (d *EnvDecoder) Extensions() []string { return []string{".env"} }

// unquote removes surrounding single or double quotes from a value.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

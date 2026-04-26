package decoder

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/os-gomod/config/v2/internal/domain/errors"
)

// ---------------------------------------------------------------------------
// EnvDecoder
// ---------------------------------------------------------------------------

// EnvDecoder parses KEY=VALUE formatted text (e.g., .env files or
// environment variable dumps). Lines starting with # are treated as comments.
// Blank lines are ignored.
type EnvDecoder struct {
	Separator string // separator used for key splitting; default is "_"
}

// NewEnvDecoder creates a new Env decoder with default separator "_".
func NewEnvDecoder() *EnvDecoder {
	return &EnvDecoder{
		Separator: "_",
	}
}

// Decode parses "KEY1=value1\nKEY2=value2" format and returns the result.
// Comments (lines starting with #) and blank lines are skipped.
// Multi-word keys are converted to dot.notation using the Separator.
func (d *EnvDecoder) Decode(src []byte) (map[string]any, error) {
	result := make(map[string]any)

	scanner := bufio.NewScanner(bytes.NewReader(src))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on first '=' only.
		idx := strings.Index(line, "=")
		if idx < 0 {
			// Line without '=' — skip.
			continue
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		// Remove surrounding quotes if present.
		val = unquote(val)

		if key == "" {
			continue
		}

		// Convert key to dot notation.
		configKey := envDecodeKey(key, d.Separator)

		// Store as a flat key-value pair.
		result[configKey] = val
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Build(errors.CodeParseError,
			fmt.Sprintf("env decode failed at line %d", lineNum),
			errors.WithOperation("env.decode")).Wrap(err)
	}

	return result, nil
}

// MediaType returns "text/x-env".
func (d *EnvDecoder) MediaType() string {
	return "text/x-env"
}

// Extensions returns the file extensions handled by this decoder.
func (d *EnvDecoder) Extensions() []string {
	return []string{".env"}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// envDecodeKey converts an env-style key to dot.notation.
// For example, with separator "_":
//
//	APP_DB_HOST -> app.db.host
//	SERVER_PORT -> server.port
func envDecodeKey(key, separator string) string {
	if separator == "" {
		separator = "_"
	}

	parts := strings.Split(key, separator)
	for i, part := range parts {
		parts[i] = strings.ToLower(part)
	}
	return strings.Join(parts, ".")
}

// unquote removes surrounding single or double quotes from a value.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

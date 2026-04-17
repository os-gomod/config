package loader

import (
	"context"
	"os"
	"strings"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
)

// EnvLoader reads configuration from OS environment variables. It supports
// optional prefix filtering (e.g., "APP_" to only read APP_* variables)
// and key transformation via a replacer function.
//
// By default, keys are transformed by converting to lowercase and replacing
// underscores with dots (e.g., "DB_HOST" becomes "db.host").
//
// Example:
//
//	el := loader.NewEnvLoader(
//	    loader.WithEnvPrefix("APP"),
//	    loader.WithEnvPriority(40),
//	)
//	// Reads APP_DB_HOST -> key "db.host", APP_SERVER_PORT -> key "server.port"
type EnvLoader struct {
	*Base
	prefix      string
	keyReplacer func(string) string
}

var _ Loader = (*EnvLoader)(nil)

// EnvOption configures an EnvLoader.
type EnvOption func(*EnvLoader)

// WithEnvPrefix sets the prefix filter for environment variables.
// Only variables starting with the prefix (case-insensitive, uppercased)
// followed by an underscore are included. The prefix is stripped from the key.
func WithEnvPrefix(prefix string) EnvOption {
	return func(e *EnvLoader) { e.prefix = strings.TrimRight(strings.ToUpper(prefix), "_") }
}

// WithEnvPriority sets the priority for values loaded from environment variables.
func WithEnvPriority(p int) EnvOption {
	return func(e *EnvLoader) { e.SetPriority(p) }
}

// WithEnvKeyReplacer sets a custom key transformation function.
// The function receives the raw environment variable name (after prefix stripping)
// and should return the config key to use.
func WithEnvKeyReplacer(fn func(string) string) EnvOption {
	return func(e *EnvLoader) { e.keyReplacer = fn }
}

// NewEnvLoader creates a new EnvLoader with the given options.
// Default priority is 40. The default key replacer converts to lowercase
// and replaces underscores with dots.
func NewEnvLoader(opts ...EnvOption) *EnvLoader {
	e := &EnvLoader{
		Base:        NewBase("env", "env", 40),
		keyReplacer: defaultKeyReplacer,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Load reads all matching OS environment variables and returns a flat
// key-value map. If a prefix is configured, only variables with that
// prefix are included. All values are typed as string with SourceEnv.
func (e *EnvLoader) Load(_ context.Context) (map[string]value.Value, error) {
	if e.IsClosed() {
		return nil, ErrClosed
	}
	replacer := e.keyReplacer
	if replacer == nil {
		replacer = defaultKeyReplacer
	}
	result := make(map[string]value.Value)
	for _, env := range os.Environ() {
		idx := strings.IndexByte(env, '=')
		if idx < 0 {
			continue
		}
		envKey, envVal := env[:idx], env[idx+1:]
		var key string
		if e.prefix != "" {
			if !strings.HasPrefix(envKey, e.prefix+"_") {
				continue
			}
			key = strings.TrimPrefix(envKey, e.prefix+"_")
		} else {
			key = envKey
		}
		key = replacer(key)
		result[key] = value.New(envVal, value.TypeString, value.SourceEnv, e.Priority())
	}
	return result, nil
}

// Watch returns nil since EnvLoader does not support change watching.
func (e *EnvLoader) Watch(_ context.Context) (<-chan event.Event, error) { return nil, nil }

// Close releases resources held by the EnvLoader.
func (e *EnvLoader) Close(ctx context.Context) error { return e.Base.Close(ctx) }

// defaultKeyReplacer converts an uppercase key with underscores to a
// lowercase key with dots (e.g., "DB_HOST" -> "db.host").
func defaultKeyReplacer(key string) string {
	return strings.ToLower(strings.ReplaceAll(key, "_", "."))
}

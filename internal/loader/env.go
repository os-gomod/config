package loader

import (
	"context"
	"os"
	"strings"

	"github.com/os-gomod/config/v2/internal/domain/event"
	"github.com/os-gomod/config/v2/internal/domain/value"
)

// ---------------------------------------------------------------------------
// EnvLoader
// ---------------------------------------------------------------------------

// EnvLoader reads configuration from environment variables matching a prefix.
// Environment keys are converted from ENV_STYLE (e.g., APP_DB_HOST) to
// dot.notation (e.g., app.db.host).
type EnvLoader struct {
	*Base
	prefix    string
	separator string // default "_"
}

// EnvOption configures an EnvLoader during construction.
type EnvOption func(*EnvLoader)

// WithEnvPrefix sets the prefix filter. Only env vars starting with this
// prefix will be loaded. Default is empty (all vars).
func WithEnvPrefix(prefix string) EnvOption {
	return func(e *EnvLoader) {
		e.prefix = prefix
	}
}

// WithEnvSeparator sets the separator used to split env var keys into
// nested config keys. Default is "_".
func WithEnvSeparator(sep string) EnvOption {
	return func(e *EnvLoader) {
		if sep != "" {
			e.separator = sep
		}
	}
}

// WithEnvPriority sets the loader priority.
func WithEnvPriority(p int) EnvOption {
	return func(e *EnvLoader) {
		e.priority = p
	}
}

// NewEnvLoader creates a new environment variable loader.
func NewEnvLoader(name string, opts ...EnvOption) *EnvLoader {
	e := &EnvLoader{
		Base:      NewBase(name, "env", 0),
		separator: "_",
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Load reads all environment variables matching the configured prefix,
// converts their keys to dot notation, and returns the result.
func (e *EnvLoader) Load(ctx context.Context) (map[string]value.Value, error) {
	if err := e.CheckClosed(); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, e.WrapErr(ctx.Err(), "load")
	default:
	}

	envVars := os.Environ()
	result := make(map[string]value.Value)

	for _, env := range envVars {
		// Split KEY=VALUE.
		idx := strings.Index(env, "=")
		if idx < 0 {
			continue
		}
		key := env[:idx]
		val := env[idx+1:]

		// Filter by prefix if set.
		if e.prefix != "" && !strings.HasPrefix(key, e.prefix) {
			continue
		}

		// Strip prefix and convert to dot notation.
		configKey := envKeyToConfigKey(key, e.prefix, e.separator)
		if configKey == "" {
			continue
		}

		result[configKey] = value.FromRaw(val, value.TypeString, value.SourceEnv, e.priority)
	}

	return result, nil
}

// Watch for env variable changes is a no-op for environment loaders since
// there is no portable way to watch env vars. It returns an empty channel
// that is closed when the context is cancelled.
func (e *EnvLoader) Watch(ctx context.Context) (<-chan event.Event, error) {
	if err := e.CheckClosed(); err != nil {
		return nil, err
	}

	ch := make(chan event.Event)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

// Close implements Loader.Close.
func (e *EnvLoader) Close(_ context.Context) error {
	_ = e.CloseBase()
	return nil
}

// Prefix returns the configured environment variable prefix.
func (e *EnvLoader) Prefix() string {
	return e.prefix
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// envKeyToConfigKey converts an environment variable key to a dot-notation
// config key. For example, with prefix "APP_" and separator "_":
//
//	APP_DB_HOST -> db.host
//	APP_SERVER_PORT -> server.port
//
// If the prefix is empty, the entire key is converted.
func envKeyToConfigKey(envKey, prefix, separator string) string {
	key := envKey
	if prefix != "" {
		if !strings.HasPrefix(key, prefix) {
			return ""
		}
		key = strings.TrimPrefix(key, prefix)
	}

	// Remove leading separator.
	key = strings.TrimLeft(key, separator)
	if key == "" {
		return ""
	}

	// Convert to lowercase and replace separator with dots.
	parts := strings.Split(key, separator)
	for i, part := range parts {
		parts[i] = strings.ToLower(part)
	}
	return strings.Join(parts, ".")
}

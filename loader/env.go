package loader

import (
	"context"
	"os"
	"strings"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
)

// EnvLoader loads config from OS environment variables.
// Variable names are normalised: PREFIX_DB_HOST -> db.host.
type EnvLoader struct {
	*Base
	prefix      string
	keyReplacer func(string) string // nil = default normaliser
}

var _ Loader = (*EnvLoader)(nil)

// EnvOption configures an EnvLoader during creation.
type EnvOption func(*EnvLoader)

// WithEnvPrefix filters to variables with the given prefix.
func WithEnvPrefix(prefix string) EnvOption {
	return func(e *EnvLoader) { e.prefix = strings.TrimRight(strings.ToUpper(prefix), "_") }
}

// WithEnvPriority sets the merge priority.
func WithEnvPriority(p int) EnvOption {
	return func(e *EnvLoader) { e.SetPriority(p) }
}

// WithEnvKeyReplacer overrides the default key normaliser.
// The function receives the raw env key (after prefix stripping) and
// must return the dot-separated lowercase config key.
func WithEnvKeyReplacer(fn func(string) string) EnvOption {
	return func(e *EnvLoader) { e.keyReplacer = fn }
}

// NewEnvLoader creates an EnvLoader with the given options.
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

// Load implements Loader. It reads OS environment variables, applies the
// prefix filter, and normalises keys using the configured replacer.
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

// Watch implements Loader. Environment variables do not support watching;
// it always returns (nil, nil).
func (e *EnvLoader) Watch(_ context.Context) (<-chan event.Event, error) { return nil, nil }

// Close implements Loader.
func (e *EnvLoader) Close(ctx context.Context) error { return e.Base.Close(ctx) }

// defaultKeyReplacer converts DB_HOST -> db.host.
func defaultKeyReplacer(key string) string {
	return strings.ToLower(strings.ReplaceAll(key, "_", "."))
}

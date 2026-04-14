// Package decoder provides pluggable format parsers that convert raw file bytes
// into a flat dot-separated key-value map. Register decoders in the DefaultRegistry
// or supply them directly to a loader.FileLoader.
package decoder

import (
	"sort"
	"strings"
	"sync"

	configerrors "github.com/os-gomod/config/errors"
)

// Decoder parses raw bytes into a flat, dot-separated, lowercase key-value map.
// Nested structures are flattened: {"db":{"host":"x"}} -> {"db.host":"x"}.
type Decoder interface {
	// Decode parses src and returns a flat key-value map.
	Decode(src []byte) (map[string]any, error)

	// MediaType returns the MIME type handled by this decoder,
	// e.g. "application/json".
	MediaType() string

	// Extensions returns the file extensions handled by this decoder,
	// e.g. []string{".yaml", ".yml"}.
	Extensions() []string
}

// Registry maps file extensions and MIME types to Decoder implementations.
// It is safe for concurrent reads; writes require external synchronization
// or must occur before any concurrent reads (typically at init time).
type Registry struct {
	// mu protects byExt and byMedia.
	mu      sync.RWMutex
	byExt   map[string]Decoder
	byMedia map[string]Decoder
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		byExt:   make(map[string]Decoder),
		byMedia: make(map[string]Decoder),
	}
}

// Register adds d to the registry.
// Returns ErrAlreadyRegistered if any extension or the media type is already mapped.
func (r *Registry) Register(d Decoder) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, ext := range d.Extensions() {
		if _, exists := r.byExt[ext]; exists {
			return ErrAlreadyRegistered
		}
	}
	if _, exists := r.byMedia[d.MediaType()]; exists {
		return ErrAlreadyRegistered
	}

	for _, ext := range d.Extensions() {
		r.byExt[ext] = d
	}
	r.byMedia[d.MediaType()] = d
	return nil
}

// MustRegister is like Register but panics on error.
func (r *Registry) MustRegister(d Decoder) {
	if err := r.Register(d); err != nil {
		panic(err)
	}
}

// ForExtension returns the Decoder for ext (e.g. ".yaml").
// Returns ErrNotFound if not registered.
func (r *Registry) ForExtension(ext string) (Decoder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.byExt[ext]
	if !ok {
		return nil, ErrNotFound
	}
	return d, nil
}

// ForMediaType returns the Decoder for the given MIME type.
// Returns ErrNotFound if not registered.
func (r *Registry) ForMediaType(mt string) (Decoder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.byMedia[mt]
	if !ok {
		return nil, ErrNotFound
	}
	return d, nil
}

// Names returns all registered extensions in sorted order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.byExt))
	for ext := range r.byExt {
		names = append(names, ext)
	}
	sort.Strings(names)
	return names
}

var (
	// ErrNotFound is returned when no decoder is registered for an extension or media type.
	ErrNotFound = configerrors.New(configerrors.CodeNotFound, "decoder not found")
	// ErrAlreadyRegistered is returned when a decoder is registered twice.
	ErrAlreadyRegistered = configerrors.New(
		configerrors.CodeAlreadyExists,
		"decoder already registered",
	)
)

// DefaultRegistry is pre-populated with YAML, JSON, dotenv, and INI decoders.
// TOML and HCL are included only when the respective build tags are active.
// This is the only package-level variable permitted in decoder/.
var DefaultRegistry = newDefaultRegistry()

// newDefaultRegistry creates and populates the default decoder registry.
func newDefaultRegistry() *Registry {
	r := NewRegistry()
	r.MustRegister(NewYAMLDecoder())
	r.MustRegister(NewJSONDecoder())
	r.MustRegister(NewEnvDecoder())
	r.MustRegister(NewINIDecoder())
	return r
}

// flatten recursively flattens a nested map into dot-separated lowercase keys.
// Example: {"db": {"host": "x"}} -> {"db.host": "x"}.
func flatten(prefix string, in, out map[string]any) {
	for k, v := range in {
		key := strings.ToLower(k)
		if prefix != "" {
			key = prefix + "." + key
		}
		switch val := v.(type) {
		case map[string]any:
			flatten(key, val, out)
		case map[any]any:
			converted := make(map[string]any, len(val))
			for mk, mv := range val {
				if ks, ok := mk.(string); ok {
					converted[ks] = mv
				}
			}
			flatten(key, converted, out)
		default:
			out[key] = v
		}
	}
}

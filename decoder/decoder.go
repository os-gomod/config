// Package decoder provides content format decoders that convert raw bytes into
// flat key-value maps. Each decoder supports automatic format detection via file
// extension or MIME type, and all decoders produce lowercased dot-separated keys
// via the shared keyutil.FlattenMapLower function.
//
// # Supported Formats
//
//   - YAML (.yaml, .yml)
//   - JSON (.json)
//   - TOML (.toml)
//   - HCL (.hcl)
//   - INI (.ini)
//   - ENV (.env)
//
// # Registration
//
// All built-in decoders are pre-registered in DefaultRegistry. To add custom
// decoders, use Registry.Register() or implement the Decoder interface.
package decoder

import (
	"sort"
	"sync"

	configerrors "github.com/os-gomod/config/errors"
)

// Decoder converts raw bytes into a flat map[string]any of configuration values.
// Implementations must handle their specific format's syntax and produce
// dot-separated, lowercased keys.
type Decoder interface {
	// Decode parses src and returns a flattened key-value map.
	Decode(src []byte) (map[string]any, error)

	// MediaType returns the MIME type associated with this decoder.
	MediaType() string

	// Extensions returns the file extensions handled by this decoder (including the dot).
	Extensions() []string
}

// Registry manages a collection of decoders indexed by file extension and MIME type.
// It is safe for concurrent use.
type Registry struct {
	mu      sync.RWMutex
	byExt   map[string]Decoder
	byMedia map[string]Decoder
}

// NewRegistry creates an empty decoder registry.
func NewRegistry() *Registry {
	return &Registry{
		byExt:   make(map[string]Decoder),
		byMedia: make(map[string]Decoder),
	}
}

// Register adds a decoder to the registry for all its declared extensions and media type.
// Returns ErrAlreadyRegistered if any extension or media type is already claimed.
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

// MustRegister adds a decoder, panicking on registration failure.
func (r *Registry) MustRegister(d Decoder) {
	if err := r.Register(d); err != nil {
		panic(err)
	}
}

// ForExtension returns the decoder for the given file extension (e.g., ".yaml").
func (r *Registry) ForExtension(ext string) (Decoder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.byExt[ext]
	if !ok {
		return nil, ErrNotFound
	}
	return d, nil
}

// ForMediaType returns the decoder for the given MIME type.
func (r *Registry) ForMediaType(mt string) (Decoder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.byMedia[mt]
	if !ok {
		return nil, ErrNotFound
	}
	return d, nil
}

// Names returns a sorted list of all registered file extensions.
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
	// ErrNotFound is returned when no decoder is registered for a given extension or media type.
	ErrNotFound = configerrors.New(configerrors.CodeNotFound, "decoder not found")
	// ErrAlreadyRegistered is returned when attempting to register a decoder for an
	// already-claimed extension or media type.
	ErrAlreadyRegistered = configerrors.New(
		configerrors.CodeAlreadyExists,
		"decoder already registered",
	)
)

// DefaultRegistry contains all built-in decoders pre-registered.
// It is safe for concurrent use.
var DefaultRegistry = newDefaultRegistry()

func newDefaultRegistry() *Registry {
	r := NewRegistry()
	r.MustRegister(NewYAMLDecoder())
	r.MustRegister(NewJSONDecoder())
	r.MustRegister(NewTOMLDecoder())
	r.MustRegister(NewHCLDecoder())
	r.MustRegister(NewEnvDecoder())
	r.MustRegister(NewINIDecoder())
	return r
}

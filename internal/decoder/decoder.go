// Package decoder provides infrastructure adapters for decoding configuration
// data from various formats (YAML, JSON, TOML, ENV) into Go maps.
// All registries are instance-based — NO global variables.
package decoder

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/os-gomod/config/v2/internal/domain/errors"
)

// ---------------------------------------------------------------------------
// Decoder interface
// ---------------------------------------------------------------------------

// Decoder decodes raw bytes into a map[string]any.
type Decoder interface {
	// Decode parses src and returns the decoded data.
	Decode(src []byte) (map[string]any, error)
	// MediaType returns the MIME type this decoder handles (e.g., "application/yaml").
	MediaType() string
	// Extensions returns file extensions this decoder handles (e.g., ".yaml", ".yml").
	Extensions() []string
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

// Registry is an instance-based decoder registry. Decoders can be looked up
// by file extension or media type. There are NO global default registries;
// use NewRegistry or NewDefaultRegistry to create one.
type Registry struct {
	mu      sync.RWMutex
	byExt   map[string]Decoder
	byMedia map[string]Decoder
}

// NewRegistry creates a new empty decoder Registry.
func NewRegistry() *Registry {
	return &Registry{
		byExt:   make(map[string]Decoder),
		byMedia: make(map[string]Decoder),
	}
}

// NewDefaultRegistry creates a Registry pre-loaded with standard decoders:
// YAML, JSON, TOML, and ENV. This is a function, NOT a global variable.
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	// Ignore registration errors for known-good decoders.
	_ = r.Register(NewYAMLDecoder())
	_ = r.Register(NewJSONDecoder())
	_ = r.Register(NewTOMLDecoder())
	_ = r.Register(NewEnvDecoder())
	return r
}

// Register adds a Decoder to the registry, indexed by all its extensions
// and media type. Returns an error if a conflict is detected.
func (r *Registry) Register(d Decoder) error {
	if d == nil {
		return errors.New(errors.CodeInvalidConfig, "decoder must not be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Register by each extension.
	for _, ext := range d.Extensions() {
		normalized := strings.ToLower(ext)
		if existing, exists := r.byExt[normalized]; exists {
			return errors.New(errors.CodeConflict,
				fmt.Sprintf("extension %q is already registered by %q, cannot register %q",
					normalized, existing.MediaType(), d.MediaType()))
		}
		r.byExt[normalized] = d
	}

	// Register by media type.
	mt := strings.ToLower(d.MediaType())
	if existing, exists := r.byMedia[mt]; exists {
		return errors.New(errors.CodeConflict,
			fmt.Sprintf("media type %q is already registered by %q, cannot register %q",
				mt, existing.MediaType(), d.MediaType()))
	}
	r.byMedia[mt] = d

	return nil
}

// ForExtension returns a Decoder for the given file extension (e.g., ".yaml").
// The extension is normalized to lowercase. Returns an error if no decoder is found.
func (r *Registry) ForExtension(ext string) (Decoder, error) {
	return r.lookup(
		ext,
		r.byExt,
		"no decoder registered for extension %q",
	)
}

// ForMediaType returns a Decoder for the given media type (e.g., "application/yaml").
func (r *Registry) ForMediaType(mt string) (Decoder, error) {
	return r.lookup(
		mt,
		r.byMedia,
		"no decoder registered for media type %q",
	)
}

// Names returns a sorted list of unique media types for all registered decoders.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]struct{}, len(r.byMedia))
	names := make([]string, 0, len(r.byMedia))
	for mt := range r.byMedia {
		if _, exists := seen[mt]; !exists {
			seen[mt] = struct{}{}
			names = append(names, mt)
		}
	}
	sort.Strings(names)
	return names
}

// Extensions returns all registered file extensions, sorted.
func (r *Registry) Extensions() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	exts := make([]string, 0, len(r.byExt))
	for ext := range r.byExt {
		exts = append(exts, ext)
	}
	sort.Strings(exts)
	return exts
}

// Len returns the number of unique decoders registered.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byMedia)
}

// HasExtension returns true if a decoder is registered for the extension.
func (r *Registry) HasExtension(ext string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.byExt[strings.ToLower(ext)]
	return exists
}

// HasMediaType returns true if a decoder is registered for the media type.
func (r *Registry) HasMediaType(mt string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.byMedia[strings.ToLower(mt)]
	return exists
}

func (r *Registry) lookup(
	key string,
	table map[string]Decoder,
	notFoundMsg string,
) (Decoder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalized := strings.ToLower(key)

	d, exists := table[normalized]
	if !exists {
		return nil, errors.New(errors.CodeNotFound,
			fmt.Sprintf(notFoundMsg, normalized))
	}

	return d, nil
}

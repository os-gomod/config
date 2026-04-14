// Package plugin provides the extension point for third-party plugins that
// enrich the config framework with additional loaders, providers, decoders,
// validators, or event middleware.
package plugin

import (
	gvalidator "github.com/go-playground/validator/v10"

	"github.com/os-gomod/config/decoder"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/loader"
	"github.com/os-gomod/config/provider"
)

// Plugin is the interface all plugins must implement.
type Plugin interface {
	// Name returns a unique plugin identifier used for registration and logging.
	Name() string

	// Init is called once when the plugin is registered.
	// The plugin uses h to register its extensions.
	Init(h Host) error
}

// Host is the surface area exposed to a plugin during Init.
// It provides access to all framework registries.
type Host interface {
	// RegisterLoader adds a Loader factory under name.
	RegisterLoader(name string, f loader.Factory) error

	// RegisterProvider adds a Provider factory under name.
	RegisterProvider(name string, f provider.Factory) error

	// RegisterDecoder adds a Decoder to the decoder registry.
	RegisterDecoder(d decoder.Decoder) error

	// RegisterValidator adds a named validation tag function.
	RegisterValidator(tag string, fn gvalidator.Func) error

	// Subscribe registers a catch-all event observer.
	// Returns an unsubscribe function.
	Subscribe(obs event.Observer) func()
}

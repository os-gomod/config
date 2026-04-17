// Package plugin provides a plugin system for extending the configuration library
// at runtime. Plugins can register custom loaders, providers, decoders, validators,
// and event subscribers through the Host interface.
package plugin

import (
	gvalidator "github.com/go-playground/validator/v10"

	"github.com/os-gomod/config/decoder"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/loader"
	"github.com/os-gomod/config/provider"
)

// Plugin is the interface that configuration plugins must implement.
// Each plugin has a unique name and an Init method that receives a Host
// through which it can register extensions.
type Plugin interface {
	// Name returns the unique identifier for this plugin.
	Name() string
	// Init is called when the plugin is registered with the Host.
	// Use the Host to register loaders, providers, decoders, validators, etc.
	Init(h Host) error
}

// Host is the interface provided to plugins during initialization. It allows
// plugins to register their extensions with the configuration engine.
type Host interface {
	// RegisterLoader adds a named loader factory to the engine.
	RegisterLoader(name string, f loader.Factory) error
	// RegisterProvider adds a named provider factory to the engine.
	RegisterProvider(name string, f provider.Factory) error
	// RegisterDecoder adds a content format decoder to the engine.
	RegisterDecoder(d decoder.Decoder) error
	// RegisterValidator adds a custom validation tag function.
	RegisterValidator(tag string, fn gvalidator.Func) error
	// Subscribe registers an event observer that receives all configuration events.
	// Returns an unsubscribe function.
	Subscribe(obs event.Observer) func()
}

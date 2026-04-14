// Package provider defines the Provider interface for remote configuration stores
// and houses concrete implementations (Consul, etcd, NATS).
// Remote providers differ from local loaders in that they support health checks
// and carry an identifying Name.
package provider

import (
	"context"

	"github.com/os-gomod/config/loader"
)

// Provider is a remote configuration source.
// It extends loader.Loader with health-check and naming capabilities.
type Provider interface {
	loader.Loader

	// Name returns the provider's registered identifier, e.g. "consul".
	Name() string

	// Health checks the liveness of the remote connection.
	// Returns nil if the provider is reachable and operational.
	Health(ctx context.Context) error
}

// Factory creates a Provider from a generic configuration map.
type Factory func(cfg map[string]any) (Provider, error)

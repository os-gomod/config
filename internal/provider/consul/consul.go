// Package consul provides a stub Consul provider for loading configuration.
// This is a lightweight in-memory implementation that conforms to the
// provider.Provider interface without importing the hashicorp/consul/api SDK.
package consul

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/errors"
	"github.com/os-gomod/config/v2/internal/domain/event"
	"github.com/os-gomod/config/v2/internal/domain/value"
	"github.com/os-gomod/config/v2/internal/provider"
)

// Provider loads configuration from an in-memory Consul-like KV store.
// It implements provider.Provider. This is a stub suitable for testing
// and development without a real Consul agent.
type Provider struct {
	mu     sync.RWMutex
	store  map[string]string // in-memory KV store
	prefix string            // KV path prefix, e.g. "config/myapp/"
	addr   string            // configured address (informational only)
	closed bool
}

// NewProvider creates a Consul stub provider from configuration. The cfg map
// may contain:
//
//	"address"    - Consul agent address (default: "127.0.0.1:8500")
//	"prefix"     - KV path prefix (default: "")
//	"datacenter" - datacenter override (informational only)
//	"token"      - ACL token (informational only)
//	"scheme"     - "http" or "https" (informational only)
//	"timeout"    - HTTP timeout as duration string (informational only)
func NewProvider(cfg map[string]any) (provider.Provider, error) {
	if cfg == nil {
		cfg = make(map[string]any)
	}

	addr, _ := cfg["address"].(string)
	if addr == "" {
		addr = "127.0.0.1:8500"
	}

	prefix, _ := cfg["prefix"].(string)
	if prefix != "" && !endsWithSlash(prefix) {
		prefix += "/"
	}

	return &Provider{
		store:  make(map[string]string),
		prefix: prefix,
		addr:   addr,
	}, nil
}

// Load reads all keys under the configured prefix from the in-memory store.
func (p *Provider) Load(ctx context.Context) (map[string]value.Value, error) {
	select {
	case <-ctx.Done():
		return nil, errors.New(errors.CodeContextCanceled, "consul load cancelled").WithSource("consul")
	default:
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, errors.New(errors.CodeClosed, "consul provider is closed").WithSource("consul")
	}

	result := make(map[string]value.Value)
	for key, val := range p.store {
		if p.prefix != "" {
			if len(key) <= len(p.prefix) || key[:len(p.prefix)] != p.prefix {
				continue
			}
		}
		configKey := trimPrefix(key, p.prefix)
		result[configKey] = value.FromRaw(val, value.TypeString, value.SourceHTTP, 0)
	}

	return result, nil
}

// Watch monitors the in-memory store for changes. Since this is an in-memory
// stub, it immediately closes the channel after a brief delay.
func (p *Provider) Watch(ctx context.Context) (<-chan event.Event, error) {
	ch := make(chan event.Event, 16)

	go func() {
		defer close(ch)
		// In a real implementation, this would long-poll Consul's blocking API.
		// For the stub, we wait until context is cancelled.
		<-ctx.Done()
	}()

	return ch, nil
}

// Close releases the provider's resources.
func (p *Provider) Close(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	p.store = nil
	return nil
}

// String returns "consul".
func (p *Provider) String() string {
	return "consul"
}

// Prefix returns the configured KV prefix.
func (p *Provider) Prefix() string {
	return p.prefix
}

// Set stores a key-value pair in the in-memory store (for testing purposes).
func (p *Provider) Set(key, val string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.store[key] = val
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func endsWithSlash(s string) bool {
	return s != "" && s[len(s)-1] == '/'
}

func trimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

// Ensure unused imports are consumed.
var (
	_ = fmt.Sprintf
	_ = time.Duration(0)
)

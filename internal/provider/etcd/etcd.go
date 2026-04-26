// Package etcd provides a stub etcd provider for loading configuration.
// This is a lightweight in-memory implementation that conforms to the
// provider.Provider interface without importing the go.etcd.io/etcd/client/v3 SDK.
package etcd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/errors"
	"github.com/os-gomod/config/v2/internal/domain/event"
	"github.com/os-gomod/config/v2/internal/domain/value"
	"github.com/os-gomod/config/v2/internal/provider"
)

// Provider loads configuration from an in-memory etcd-like KV store.
// It implements provider.Provider. This is a stub suitable for testing
// and development without a real etcd cluster.
type Provider struct {
	mu     sync.RWMutex
	store  map[string]string // in-memory KV store
	prefix string            // key prefix, e.g. "/config/myapp/"
	closed bool
}

// NewProvider creates an etcd stub provider from configuration. The cfg map
// may contain:
//
//	"endpoints"    - slice of etcd endpoints (default: ["127.0.0.1:2379"])
//	"prefix"       - key prefix for config keys (default: "")
//	"dial_timeout" - connection timeout as duration string (informational only)
//	"username"     - etcd authentication username (informational only)
//	"password"     - etcd authentication password (informational only)
func NewProvider(cfg map[string]any) (provider.Provider, error) {
	if cfg == nil {
		cfg = make(map[string]any)
	}

	prefix, _ := cfg["prefix"].(string)
	if prefix != "" && !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	return &Provider{
		store:  make(map[string]string),
		prefix: prefix,
	}, nil
}

// Load reads all keys under the configured prefix from the in-memory store.
func (p *Provider) Load(ctx context.Context) (map[string]value.Value, error) {
	select {
	case <-ctx.Done():
		return nil, errors.New(errors.CodeContextCanceled, "etcd load cancelled").WithSource("etcd")
	default:
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, errors.New(errors.CodeClosed, "etcd client is closed").WithSource("etcd")
	}

	result := make(map[string]value.Value)
	for key, val := range p.store {
		if p.prefix != "" {
			if !strings.HasPrefix(key, p.prefix) {
				continue
			}
		}
		configKey := strings.TrimPrefix(key, p.prefix)
		configKey = strings.TrimPrefix(configKey, "/")
		result[configKey] = value.FromRaw(val, value.TypeString, value.SourceRemote, 0)
	}

	return result, nil
}

// Watch monitors the in-memory store for changes. Since this is an in-memory
// stub, it waits until context is cancelled.
func (p *Provider) Watch(ctx context.Context) (<-chan event.Event, error) {
	ch := make(chan event.Event, 16)

	go func() {
		defer close(ch)
		// In a real implementation, this would use etcd's Watch API.
		// For the stub, we wait until context is cancelled.
		<-ctx.Done()
	}()

	return ch, nil
}

// Close releases the etcd client resources.
func (p *Provider) Close(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	p.store = nil
	return nil
}

// String returns "etcd".
func (p *Provider) String() string {
	return "etcd"
}

// Prefix returns the configured key prefix.
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
// Ensure unused imports are consumed.
// ---------------------------------------------------------------------------

var (
	_ = fmt.Sprintf
	_ = time.Duration(0)
)

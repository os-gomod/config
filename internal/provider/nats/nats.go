// Package nats provides a stub NATS provider for loading configuration.
// This is a lightweight in-memory implementation that conforms to the
// provider.Provider interface without importing the github.com/nats-io/nats.go SDK.
package nats

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

// Provider loads configuration from an in-memory NATS-like KV store.
// It implements provider.Provider. This is a stub suitable for testing
// and development without a real NATS server.
type Provider struct {
	mu      sync.RWMutex
	store   map[string]string // in-memory KV store
	subject string            // KV bucket name (used as subject prefix)
	closed  bool
}

// NewProvider creates a NATS stub provider from configuration. The cfg map
// may contain:
//
//	"url"      - NATS server URL (default: "nats://127.0.0.1:4222")
//	"subject"  - KV bucket name to read config from (default: "CONFIG")
//	"username" - NATS authentication username (informational only)
//	"password" - NATS authentication password (informational only)
//	"token"    - NATS authentication token (informational only)
//	"timeout"  - connection timeout as duration string (informational only)
func NewProvider(cfg map[string]any) (provider.Provider, error) {
	if cfg == nil {
		cfg = make(map[string]any)
	}

	subject, _ := cfg["subject"].(string)
	if subject == "" {
		subject = "CONFIG"
	}

	return &Provider{
		store:   make(map[string]string),
		subject: subject,
	}, nil
}

// Load reads all key-value pairs from the in-memory store under the subject prefix.
func (p *Provider) Load(ctx context.Context) (map[string]value.Value, error) {
	select {
	case <-ctx.Done():
		return nil, errors.New(errors.CodeContextCanceled, "nats load cancelled").WithSource("nats")
	default:
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, errors.New(errors.CodeClosed, "nats provider is closed").WithSource("nats")
	}

	result := make(map[string]value.Value)
	prefix := p.subject + "."
	for key, val := range p.store {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		configKey := key[len(prefix):]
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
		// In a real implementation, this would use NATS JetStream KV Watch.
		// For the stub, we wait until context is cancelled.
		<-ctx.Done()
	}()

	return ch, nil
}

// Close releases the NATS connection resources.
func (p *Provider) Close(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	p.store = nil
	return nil
}

// String returns "nats".
func (p *Provider) String() string {
	return "nats"
}

// Subject returns the configured KV bucket name.
func (p *Provider) Subject() string {
	return p.subject
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

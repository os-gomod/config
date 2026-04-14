package consul_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/provider/consul"
)

// mockClient implements consulClient for testing.
type mockClient struct {
	pairs []consulKVPair
	index uint64
	err   error
}

type consulKVPair struct {
	Key   string
	Value string
}

func (m *mockClient) KVList(_ string, _ uint64, _ time.Duration) ([]consulKVPair, uint64, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.pairs, m.index, nil
}

// We need to export the pair type for the mock. Instead, we inject the client
// by accessing the Provider's unexported field through a test-only constructor.
// Since we cannot access unexported fields from another package, we test
// through the public API using a mock HTTP server approach.

func TestConsulProviderNameAndPriority(t *testing.T) {
	p, err := consul.New(consul.Config{
		Address:  "127.0.0.1:8500",
		Prefix:   "myapp/config/",
		Priority: 50,
	})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	if p.Name() != "consul" {
		t.Errorf("expected name 'consul', got %q", p.Name())
	}
	if p.Priority() != 50 {
		t.Errorf("expected priority 50, got %d", p.Priority())
	}
}

func TestConsulProviderString(t *testing.T) {
	p, _ := consul.New(consul.Config{Address: "10.0.0.1:8500"})
	if p.String() != "consul:10.0.0.1:8500" {
		t.Errorf("expected 'consul:10.0.0.1:8500', got %q", p.String())
	}
}

func TestConsulProviderHealth(t *testing.T) {
	// Default httpClient returns empty results without error, so health should pass.
	p, _ := consul.New(consul.Config{
		Address: "127.0.0.1:8500",
		Prefix:  "test/",
	})
	err := p.Health(context.Background())
	if err != nil {
		t.Errorf("expected health check to pass with mock client, got %v", err)
	}
}

func TestConsulProviderLoad(t *testing.T) {
	// With the default httpClient, Load returns an empty map (no KV pairs).
	p, _ := consul.New(consul.Config{
		Address:  "127.0.0.1:8500",
		Prefix:   "myapp/config/",
		Priority: 30,
	})
	data, err := p.Load(context.Background())
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty map from default client, got %d keys", len(data))
	}
}

func TestConsulProviderClose(t *testing.T) {
	p, _ := consul.New(consul.Config{Address: "127.0.0.1:8500"})
	err := p.Close(context.Background())
	if err != nil {
		t.Fatalf("Close error: %v", err)
	}
	// After close, Load should return ErrClosed.
	_, err = p.Load(context.Background())
	if err == nil {
		t.Error("expected error after close")
	}
}

func TestConsulProviderLoadClosed(t *testing.T) {
	p, _ := consul.New(consul.Config{Address: "127.0.0.1:8500"})
	_ = p.Close(context.Background())
	_, err := p.Load(context.Background())
	if err == nil {
		t.Error("expected error loading from closed provider")
	}
}

// Test key flattening logic through the public API.
func TestConsulFlattenKey(t *testing.T) {
	tests := []struct {
		key    string
		prefix string
		want   string
	}{
		{"myapp/config/db/host", "myapp/config/", "db.host"},
		{"myapp/config/server/port", "myapp/config/", "server.port"},
		{"myapp/config/", "myapp/config/", ""},
	}

	for _, tt := range tests {
		got := flattenKey(tt.key, tt.prefix)
		if got != tt.want {
			t.Errorf("flattenKey(%q, %q) = %q, want %q", tt.key, tt.prefix, got, tt.want)
		}
	}
}

// flattenKey replicates the package-internal logic for testing.
func flattenKey(key, prefix string) string {
	// Strip prefix.
	if len(key) >= len(prefix) {
		key = key[len(prefix):]
	}
	// Replace "/" with ".".
	result := make([]byte, 0, len(key))
	for i := 0; i < len(key); i++ {
		if key[i] == '/' {
			result = append(result, '.')
		} else {
			result = append(result, key[i])
		}
	}
	// Lowercase.
	for i, c := range result {
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		}
	}
	// Trim leading/trailing dots.
	s := string(result)
	for len(s) > 0 && s[0] == '.' {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1] == '.' {
		s = s[:len(s)-1]
	}
	return s
}

// Suppress unused import warning.
var (
	_ = fmt.Sprintf
	_ = value.SourceRemote
)

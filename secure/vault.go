package secure

import (
	"context"
	"log/slog"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/common"
)

// VaultConfig holds the connection parameters for the HashiCorp Vault provider.
type VaultConfig struct {
	Address   string // Vault server address (e.g., "https://vault.example.com:8200")
	Token     string // Vault authentication token
	MountPath string // KV secrets engine mount path (e.g., "secret")
	Namespace string // Vault namespace (optional)
}

// VaultProvider is a stub implementation of a HashiCorp Vault configuration provider.
// All operations currently return ErrNotImplemented. This is a placeholder for
// future implementation.
type VaultProvider struct {
	*common.Closable
	config VaultConfig
	logger *slog.Logger
}

// NewVaultProvider creates a new Vault provider with the given configuration.
// A warning is logged indicating that all operations are stubs.
func NewVaultProvider(cfg VaultConfig, opts ...Option) *VaultProvider {
	o := defaultSecureOptions()
	for _, opt := range opts {
		opt(&o)
	}
	vp := &VaultProvider{
		Closable: common.NewClosable(),
		config:   cfg,
		logger:   o.logger,
	}
	if vp.logger == nil {
		vp.logger = slog.Default()
	}
	vp.logger.Warn("secure: VaultProvider is a stub; all operations return ErrNotImplemented")
	return vp
}

// Name returns "vault".
func (v *VaultProvider) Name() string { return "vault" }

// Health always returns ErrNotImplemented (stub).
func (v *VaultProvider) Health(_ context.Context) error {
	v.logger.Warn("secure: VaultProvider.Health is a stub")
	return ErrNotImplemented
}

// Load always returns ErrNotImplemented (stub).
func (v *VaultProvider) Load(_ context.Context) (map[string]value.Value, error) {
	v.logger.Warn("secure: VaultProvider.Load is a stub")
	return nil, ErrNotImplemented
}

// Watch returns nil since VaultProvider is a stub.
func (v *VaultProvider) Watch(_ context.Context) (<-chan event.Event, error) {
	return nil, nil
}

// Priority returns the provider priority (always 50).
func (v *VaultProvider) Priority() int { return 50 }

// String returns a human-readable description including the Vault address.
func (v *VaultProvider) String() string { return "vault:" + v.config.Address }

// Close releases resources held by the VaultProvider.
func (v *VaultProvider) Close(ctx context.Context) error { return v.Closable.Close(ctx) }

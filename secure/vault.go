package secure

import (
	"context"
	"log/slog"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/common"
)

// VaultConfig holds the configuration for connecting to HashiCorp Vault.
type VaultConfig struct {
	// Address is the Vault server address (e.g. "https://vault.example.com:8200").
	Address string
	// Token is the Vault authentication token.
	Token string
	// MountPath is the KV secrets engine mount path. Default: "secret".
	MountPath string
	// Namespace is the Vault namespace (Enterprise only).
	Namespace string
}

// VaultProvider is a stub provider for HashiCorp Vault integration.
// All operations return ErrNotImplemented. Integrate a real Vault SDK
// before using in production.
//
// Lock ordering: VaultProvider holds no locks of its own beyond the embedded Closable.
type VaultProvider struct {
	*common.Closable
	config VaultConfig
	logger *slog.Logger
}

// NewVaultProvider returns a stub VaultProvider that logs a warning on first use.
// Integrate a real Vault SDK before using in production.
// Use WithLogger to supply a custom structured logger.
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

// Name returns the provider name "vault".
func (v *VaultProvider) Name() string { return "vault" }

// Health checks the Vault connection. Always returns ErrNotImplemented in this stub.
func (v *VaultProvider) Health(_ context.Context) error {
	v.logger.Warn("secure: VaultProvider.Health is a stub")
	return ErrNotImplemented
}

// Load implements loader.Loader. Always returns ErrNotImplemented in this stub.
func (v *VaultProvider) Load(_ context.Context) (map[string]value.Value, error) {
	v.logger.Warn("secure: VaultProvider.Load is a stub")
	return nil, ErrNotImplemented
}

// Watch implements loader.Loader. Always returns (nil, nil) in this stub.
func (v *VaultProvider) Watch(_ context.Context) (<-chan event.Event, error) {
	return nil, nil
}

// Priority returns the merge priority (default 50 for remote providers).
func (v *VaultProvider) Priority() int { return 50 }

// String returns a human-readable identifier for this provider.
func (v *VaultProvider) String() string { return "vault:" + v.config.Address }

// Close implements loader.Loader.
func (v *VaultProvider) Close(ctx context.Context) error { return v.Closable.Close(ctx) }

package secure

import (
	"context"
	"log/slog"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/common"
)

// KMSConfig holds the connection parameters for a cloud KMS provider.
type KMSConfig struct {
	Provider string // KMS provider name (e.g., "aws", "gcp", "azure")
	KeyID    string // KMS key identifier
	Region   string // Cloud region (optional)
	Endpoint string // Custom KMS endpoint (optional)
}

// KMSProvider is a stub implementation of a cloud KMS configuration provider.
// All operations currently return ErrNotImplemented. This is a placeholder for
// future implementation.
type KMSProvider struct {
	*common.Closable
	logger *slog.Logger
	kmsCfg KMSConfig
}

// NewKMSProvider creates a new KMS provider with the given configuration.
// A warning is logged indicating that all operations are stubs.
func NewKMSProvider(cfg KMSConfig, opts ...Option) *KMSProvider {
	o := defaultSecureOptions()
	for _, opt := range opts {
		opt(&o)
	}
	kp := &KMSProvider{
		Closable: common.NewClosable(),
		logger:   o.logger,
		kmsCfg:   cfg,
	}
	if kp.logger == nil {
		kp.logger = slog.Default()
	}
	kp.logger.Warn("secure: KMSProvider is a stub; all operations return ErrNotImplemented")
	return kp
}

// Name returns "kms".
func (k *KMSProvider) Name() string { return "kms" }

// Health always returns ErrNotImplemented (stub).
func (k *KMSProvider) Health(_ context.Context) error {
	k.logger.Warn("secure: KMSProvider.Health is a stub")
	return ErrNotImplemented
}

// Load always returns ErrNotImplemented (stub).
func (k *KMSProvider) Load(_ context.Context) (map[string]value.Value, error) {
	k.logger.Warn("secure: KMSProvider.Load is a stub")
	return nil, ErrNotImplemented
}

// Watch returns nil since KMSProvider is a stub.
func (k *KMSProvider) Watch(_ context.Context) (<-chan event.Event, error) {
	return nil, nil
}

// Priority returns the provider priority (always 50).
func (k *KMSProvider) Priority() int { return 50 }

// String returns a human-readable description including the provider and key ID.
func (k *KMSProvider) String() string { return "kms:" + k.kmsCfg.Provider + ":" + k.kmsCfg.KeyID }

// Close releases resources held by the KMSProvider.
func (k *KMSProvider) Close(ctx context.Context) error { return k.Closable.Close(ctx) }

package secure

import (
	"context"
	"log/slog"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/common"
)

// KMSConfig holds the configuration for connecting to a cloud KMS provider.
type KMSConfig struct {
	// Provider is the cloud provider name (e.g. "aws", "gcp", "azure").
	Provider string
	// KeyID is the KMS key identifier or ARN.
	KeyID string
	// Region is the cloud region (provider-specific).
	Region string
	// Endpoint overrides the default KMS endpoint URL.
	Endpoint string
}

// KMSProvider is a stub provider for cloud KMS integration.
// All operations return ErrNotImplemented. Integrate a real cloud KMS SDK
// before using in production.
//
// Lock ordering: KMSProvider holds no locks of its own beyond the embedded Closable.
type KMSProvider struct {
	*common.Closable
	logger *slog.Logger
	kmsCfg KMSConfig
}

// NewKMSProvider returns a stub KMSProvider.
// Use WithLogger to supply a custom structured logger.
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

// Name returns the provider name "kms".
func (k *KMSProvider) Name() string { return "kms" }

// Health checks the KMS connection. Always returns ErrNotImplemented in this stub.
func (k *KMSProvider) Health(_ context.Context) error {
	k.logger.Warn("secure: KMSProvider.Health is a stub")
	return ErrNotImplemented
}

// Load implements loader.Loader. Always returns ErrNotImplemented in this stub.
func (k *KMSProvider) Load(_ context.Context) (map[string]value.Value, error) {
	k.logger.Warn("secure: KMSProvider.Load is a stub")
	return nil, ErrNotImplemented
}

// Watch implements loader.Loader. Always returns (nil, nil) in this stub.
func (k *KMSProvider) Watch(_ context.Context) (<-chan event.Event, error) {
	return nil, nil
}

// Priority returns the merge priority (default 50 for remote providers).
func (k *KMSProvider) Priority() int { return 50 }

// String returns a human-readable identifier for this provider.
func (k *KMSProvider) String() string { return "kms:" + k.kmsCfg.Provider + ":" + k.kmsCfg.KeyID }

// Close implements loader.Loader.
func (k *KMSProvider) Close(ctx context.Context) error { return k.Closable.Close(ctx) }

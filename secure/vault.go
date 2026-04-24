package secure

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	vault "github.com/hashicorp/vault/api"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/internal/keyutil"
	"github.com/os-gomod/config/internal/providerbase"
	"github.com/os-gomod/config/provider"
)

// VaultConfig holds the configuration for connecting to HashiCorp Vault.
type VaultConfig struct {
	// Address is the Vault server address. Defaults to http://127.0.0.1:8200.
	Address string
	// Token is the Vault authentication token.
	Token string
	// MountPath is the KV secrets engine mount path. Defaults to "secret".
	MountPath string
	// Path is the secret path within the KV engine (e.g., "config/myapp").
	// All keys under this path are loaded as configuration values.
	Path string
	// Namespace is the Vault namespace (Vault Enterprise). Optional.
	Namespace string
	// Version is the KV engine version (1 or 2). Defaults to 2.
	Version int
	// PollInterval enables polling-based watching when > 0.
	PollInterval time.Duration
	// Timeout is the HTTP timeout for Vault operations. Defaults to 5s.
	Timeout time.Duration
	// Priority determines merge order. Higher values win. Defaults to 50.
	Priority int
}

// Provider is a Vault KV configuration provider backed by RemoteProvider.
// It delegates all provider operations (Load, Watch, Health, Close) to
// providerbase.RemoteProvider[*vault.Client], eliminating the duplicated
// pollController and lifecycle management that was previously inline.
//
// Secrets loaded from Vault are tagged with SourceVault and their keys
// are automatically recognized by value.IsSecret() for redaction in
// Explain(), Snapshot(), events, and logs.
type Provider struct {
	*providerbase.RemoteProvider[*vault.Client]
	client *vault.Client
	cfg    VaultConfig
}

var _ provider.Provider = (*Provider)(nil)

// VaultProvider is an alias for Provider for backward compatibility.
//
// Deprecated: Use Provider instead.
type VaultProvider = Provider

// normalizeVaultConfig applies defaults to VaultConfig and merges option overrides.
func normalizeVaultConfig(cfg VaultConfig, o secureOptions) VaultConfig {
	if cfg.Address == "" {
		cfg.Address = "http://127.0.0.1:8200"
	}
	if cfg.MountPath == "" {
		cfg.MountPath = "secret"
	}
	if cfg.Version == 0 {
		cfg.Version = 2
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.Priority == 0 {
		cfg.Priority = o.priority
	}
	return cfg
}

// extractSecretData extracts the secret data from a Vault API response,
// handling both KV v1 and v2 formats.
func extractSecretData(secret *vault.Secret, version int) map[string]any {
	if secret == nil || secret.Data == nil {
		return make(map[string]any)
	}

	if version == 2 {
		dataRaw, ok := secret.Data["data"]
		if !ok {
			return make(map[string]any)
		}
		secretData, ok := dataRaw.(map[string]any)
		if !ok {
			return make(map[string]any)
		}
		return secretData
	}

	return secret.Data
}

// buildSecretPath constructs the correct Vault path based on KV version.
func buildSecretPath(mountPath, secretPath string, version int) string {
	if version == 1 {
		return mountPath + "/" + secretPath
	}
	return mountPath + "/data/" + secretPath
}

// NewVaultProvider creates a new Vault provider and immediately verifies
// the connection. Returns an error if the Vault server is unreachable.
//
// The Vault client is created and health-checked eagerly during construction.
// All subsequent Load/Watch/Health calls use the validated client through
// providerbase's lifecycle management (lazy init with instant return since
// the client is already ready).
func NewVaultProvider(cfg VaultConfig, opts ...Option) (*Provider, error) {
	o := defaultSecureOptions()
	for _, opt := range opts {
		opt(&o)
	}

	cfg = normalizeVaultConfig(cfg, o)

	vaultCfg := vault.DefaultConfig()
	vaultCfg.Address = cfg.Address
	if cfg.Timeout > 0 {
		vaultCfg.Timeout = cfg.Timeout
	}

	client, err := vault.NewClient(vaultCfg)
	if err != nil {
		return nil, fmt.Errorf("vault: create client: %w", err)
	}

	if cfg.Token != "" {
		client.SetToken(cfg.Token)
	}
	if cfg.Namespace != "" {
		client.SetNamespace(cfg.Namespace)
	}

	// Verify connectivity
	if errVer := verifyVaultHealth(client, cfg); errVer != nil {
		return nil, errVer
	}

	// Capture config values for closures
	mountPath := cfg.MountPath
	secretPath := cfg.Path
	version := cfg.Version
	timeout := cfg.Timeout
	priority := cfg.Priority
	capturedClient := client

	p := &Provider{
		client: client,
		cfg:    cfg,
	}
	p.RemoteProvider = providerbase.New(providerbase.Config[*vault.Client]{
		Name:         "vault",
		Priority:     priority,
		PollInterval: cfg.PollInterval,
		StringFormat: "vault:" + cfg.Address + "/" + cfg.MountPath + "/" + cfg.Path,
		InitFn: func() (*vault.Client, error) {
			return capturedClient, nil
		},
		FetchFn: func(ctx context.Context, cli *vault.Client) (map[string]value.Value, error) {
			return vaultFetch(ctx, cli, mountPath, secretPath, version, timeout, priority)
		},
		HealthFn: func(ctx context.Context, cli *vault.Client) error {
			return vaultHealth(ctx, cli, timeout)
		},
		CloseFn: func(_ *vault.Client) error {
			return nil
		},
	})

	return p, nil
}

// verifyVaultHealth checks that the Vault server is initialized and unsealed.
func verifyVaultHealth(client *vault.Client, cfg VaultConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	health, err := client.Sys().HealthWithContext(ctx)
	if err != nil {
		return fmt.Errorf("vault: health check failed: %w", err)
	}
	if health == nil || !health.Initialized {
		return fmt.Errorf("vault: server not initialized at %s", cfg.Address)
	}
	if health.Sealed {
		return fmt.Errorf("vault: server is sealed at %s", cfg.Address)
	}
	return nil
}

// vaultFetch reads secrets from Vault and converts them to config values.
func vaultFetch(ctx context.Context, cli *vault.Client, mountPath, secretPath string,
	version int, timeout time.Duration, priority int,
) (map[string]value.Value, error) {
	fetchCtx, fetchCancel := context.WithTimeout(ctx, timeout)
	defer fetchCancel()

	path := buildSecretPath(mountPath, secretPath, version)
	secret, err := cli.Logical().ReadWithContext(fetchCtx, path)
	if err != nil {
		return nil, fmt.Errorf("vault: read %q: %w", path, err)
	}

	secretData := extractSecretData(secret, version)

	result := make(map[string]value.Value, len(secretData))
	for k, val := range secretData {
		flatKey := keyutil.FlattenProviderKey(k, secretPath)
		if flatKey == "" {
			flatKey = k
		}
		strVal := formatSecretValue(val)
		result[flatKey] = value.New(strVal, value.TypeString, value.SourceVault, priority)
	}
	return result, nil
}

// vaultHealth performs a health check on the Vault server.
func vaultHealth(ctx context.Context, cli *vault.Client, timeout time.Duration) error {
	healthCtx, healthCancel := context.WithTimeout(ctx, timeout)
	defer healthCancel()

	h, err := cli.Sys().HealthWithContext(healthCtx)
	if err != nil {
		return fmt.Errorf("vault: health check: %w", err)
	}
	if h.Sealed {
		return fmt.Errorf("vault: server is sealed")
	}
	return nil
}

// SetToken updates the Vault client token. This is useful for token
// rotation without recreating the entire provider. The call is
// serialized under the providerbase mutex to prevent concurrent
// Load/Watch/Health from observing a partially-updated token.
func (p *Provider) SetToken(token string) {
	p.Mu.Lock()
	defer p.Mu.Unlock()
	p.client.SetToken(token)
}

// formatSecretValue converts a Vault secret value to its string representation.
// Non-string values (int, float, bool, nested maps) are converted to strings
// to maintain compatibility with the config value system.
func formatSecretValue(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case float64:
		// JSON numbers are float64 by default
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case map[string]any:
		// Flatten nested maps into dot-separated keys
		return flattenMap(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// flattenMap converts a nested map into a flat string representation
// with dot-separated keys, suitable for Vault secret data that may
// contain nested JSON objects.
func flattenMap(m map[string]any) string {
	var sb strings.Builder
	for k, v := range m {
		if sb.Len() > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(k)
		sb.WriteString("=")
		fmt.Fprintf(&sb, "%v", v)
	}
	return sb.String()
}

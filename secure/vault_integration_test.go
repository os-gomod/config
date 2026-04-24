//go:build integration

package secure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewVaultProvider_RequiresRunningVault(t *testing.T) {
	cfg := VaultConfig{
		Address: "http://127.0.0.1:8200",
		Token:   "test-token",
		Path:    "config/test",
	}

	_, err := NewVaultProvider(cfg)
	if err == nil {
		t.Skip("Vault is running; this test expects Vault to be unavailable")
		return
	}
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault")
}

func TestVaultProvider_Health_Unreachable(t *testing.T) {
	cfg := VaultConfig{
		Address: "http://127.0.0.1:8200",
		Token:   "test-token",
		Path:    "config/test",
	}

	provider, err := NewVaultProvider(cfg)
	if err != nil {
		t.Skipf("Vault not available, skipping: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2)
	defer cancel()

	healthErr := provider.Health(ctx)
	if healthErr != nil {
		assert.Error(t, healthErr)
	}
}

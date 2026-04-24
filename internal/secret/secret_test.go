package secret

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSecret_PasswordSuffix(t *testing.T) {
	assert.True(t, IsSecret("db.password"))
}

func TestIsSecret_TokenSuffix(t *testing.T) {
	assert.True(t, IsSecret("auth.token"))
}

func TestIsSecret_SecretSuffix(t *testing.T) {
	assert.True(t, IsSecret("api.secret"))
}

func TestIsSecret_KeySuffix(t *testing.T) {
	assert.True(t, IsSecret("encryption.key"))
}

func TestIsSecret_NonSecret(t *testing.T) {
	assert.False(t, IsSecret("database.host"))
}

func TestIsSecret_EmptyKey(t *testing.T) {
	assert.False(t, IsSecret(""))
}

func TestIsSecret_CaseInsensitive(t *testing.T) {
	assert.True(t, IsSecret("DB.PASSWORD"))
	assert.True(t, IsSecret("Auth.Token"))
	assert.True(t, IsSecret("API.SECRET"))
	assert.True(t, IsSecret("Encryption.KEY"))
}

func TestRedactValue(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		expected string
	}{
		{"secret password key", "db.password", "s3cretP@ss", RedactMask},
		{"secret token key", "auth.token", "abc123", RedactMask},
		{"secret key suffix", "encryption.key", "my-key-id", RedactMask},
		{"secret apikey suffix", "services.apikey", "ak-xyz", RedactMask},
		{"secret credentials suffix", "aws.credentials", "cred-data", RedactMask},
		{"secret auth suffix", "login.auth", "auth-data", RedactMask},
		{"non-secret key passes through", "database.host", "localhost", "localhost"},
		{"non-secret port passes through", "server.port", "8080", "8080"},
		{"non-secret name passes through", "app.name", "myapp", "myapp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactValue(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

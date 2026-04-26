package loader

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/os-gomod/config/v2/internal/domain/value"
)

func TestFileLoader_Load_FlattensNestedKeys(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
app:
  name: file-app
server:
  port: 8080
  tls:
    enabled: true
features:
  flags:
    - alpha
    - beta
`), 0o600))

	loader := NewFileLoader("test-file", []string{path}, nil, WithPriority(30))

	data, err := loader.Load(context.Background())
	require.NoError(t, err)

	assert.Len(t, data, 4)
	assert.Equal(t, "file-app", data["app.name"].Raw())
	assert.Equal(t, 8080, data["server.port"].Raw())
	assert.Equal(t, true, data["server.tls.enabled"].Raw())
	assert.Equal(t, []any{"alpha", "beta"}, data["features.flags"].Raw())
	assert.Equal(t, value.SourceFile, data["app.name"].Source())
	assert.Equal(t, 30, data["app.name"].Priority())
	_, exists := data["app"]
	assert.False(t, exists)
}

func TestFlattenConfigMap(t *testing.T) {
	t.Parallel()

	flat := flattenConfigMap(map[string]any{
		"app": map[string]any{
			"name": "demo",
		},
		"nested": map[any]any{
			"port": 8080,
			9:      "numeric",
		},
	})

	assert.Equal(t, "demo", flat["app.name"])
	assert.Equal(t, 8080, flat["nested.port"])
	assert.Equal(t, "numeric", flat["nested.9"])
}

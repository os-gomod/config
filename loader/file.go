package loader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/decoder"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/pollwatch"
)

// FileLoader loads configuration from a local file.
// It supports automatic format detection based on file extension,
// content-based caching (SHA-256 checksum), and optional polling-based
// file watching for live reload.
//
// Supported formats: .yaml/.yml, .json, .toml, .ini, .hcl, .env.
type FileLoader struct {
	*Base
	*pollwatch.Controller
	path     string
	dec      decoder.Decoder
	interval time.Duration

	mu           sync.RWMutex
	lastChecksum string
	lastData     map[string]value.Value
}

var _ Loader = (*FileLoader)(nil)

// FileOption configures a FileLoader.
type FileOption func(*FileLoader)

// WithFilePriority sets the priority for values loaded from this file.
func WithFilePriority(p int) FileOption {
	return func(f *FileLoader) { f.SetPriority(p) }
}

// WithFileDecoder sets a custom decoder instead of auto-detecting from extension.
func WithFileDecoder(d decoder.Decoder) FileOption {
	return func(f *FileLoader) { f.dec = d }
}

// WithFilePollInterval sets the polling interval for file watching.
// If not set or <= 0, Watch() returns nil (no watching).
func WithFilePollInterval(d time.Duration) FileOption {
	return func(f *FileLoader) { f.interval = d }
}

// NewFileLoader creates a loader for the given file path.
func NewFileLoader(path string, opts ...FileOption) *FileLoader {
	cleanPath := filepath.Clean(path)
	base := NewBase("file:"+cleanPath, "file", 30)
	f := &FileLoader{
		path:     cleanPath,
		lastData: make(map[string]value.Value),
	}
	f.Base = base
	f.Controller = pollwatch.NewController(64)
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Load reads the file, detects its format, decodes it, and returns flat key-value pairs.
// Uses SHA-256 checksum caching to avoid redundant decoding when the file hasn't changed.
func (f *FileLoader) Load(_ context.Context) (map[string]value.Value, error) {
	if f.IsClosed() {
		return nil, ErrClosed
	}
	content, err := os.ReadFile(f.path)
	if err != nil {
		return nil, f.WrapErr(err, "read file")
	}

	checksum := computeChecksum(content)
	f.mu.RLock()
	if checksum == f.lastChecksum {
		cached := value.Copy(f.lastData)
		f.mu.RUnlock()
		return cached, nil
	}
	f.mu.RUnlock()

	dec := f.dec
	if dec == nil {
		ext := strings.ToLower(filepath.Ext(f.path))
		dec, err = decoder.DefaultRegistry.ForExtension(ext)
		if err != nil {
			return nil, f.WrapErr(
				fmt.Errorf("no decoder for extension %q: %w", ext, err),
				"detect decoder",
			)
		}
	}

	flat, err := dec.Decode(content)
	if err != nil {
		return nil, f.WrapErr(err, "decode file")
	}

	result := make(map[string]value.Value, len(flat))
	for k, v := range flat {
		result[k] = value.New(v, value.InferType(v), value.SourceFile, f.Priority())
	}

	f.mu.Lock()
	f.lastChecksum = checksum
	f.lastData = result
	f.mu.Unlock()

	return value.Copy(result), nil
}

// Watch starts polling the file at the configured interval and emits
// change events when the file content changes.
func (f *FileLoader) Watch(ctx context.Context) (<-chan event.Event, error) {
	if f.interval <= 0 {
		return nil, nil
	}
	return f.StartPolling(ctx, f.interval, func(ctx context.Context) {
		if f.IsClosed() {
			return
		}
		newData, err := f.Load(ctx)
		if err != nil {
			return
		}
		oldData := f.lastDataSafe()
		_ = f.EmitDiff(ctx, oldData, newData,
			event.WithLabel("source", "file"),
			event.WithLabel("type", f.Type()),
		)
	}), nil
}

// Close stops file watching and releases resources.
func (f *FileLoader) Close(ctx context.Context) error {
	if f.Controller != nil {
		f.Controller.Close()
	}
	return f.Base.Close(ctx)
}

// lastDataSafe returns a thread-safe copy of the last loaded data.
func (f *FileLoader) lastDataSafe() map[string]value.Value {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return value.Copy(f.lastData)
}

// computeChecksum returns the first 16 hex characters of the SHA-256 hash of data.
func computeChecksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])[:16]
}

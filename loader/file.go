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
	"github.com/os-gomod/config/internal/providerbase"
)

// Priority constants for built-in loaders.
// These replace magic numbers throughout the codebase.
const (
	// PriorityDefault is the default merge priority for unspecified sources.
	PriorityDefault = 0
	// PriorityMemory is the priority for in-memory configuration.
	PriorityMemory = 20
	// PriorityFile is the priority for file-based configuration.
	PriorityFile = 30
	// PriorityEnv is the priority for environment variable configuration.
	PriorityEnv = 40
)

// FileLoader loads configuration from a local file.
// It supports checksum-based caching and optional polling-based watching
// via the unified PollableSource.
type FileLoader struct {
	*Base
	*providerbase.PollableSource
	path         string
	dec          decoder.Decoder
	interval     time.Duration
	mu           sync.RWMutex
	lastChecksum string
}

var _ Loader = (*FileLoader)(nil)

// FileOption configures a FileLoader.
type FileOption func(*FileLoader)

// WithFilePriority sets the merge priority.
func WithFilePriority(p int) FileOption {
	return func(f *FileLoader) { f.SetPriority(p) }
}

// WithFileDecoder sets the decoder to use. If nil, the decoder is
// auto-detected from the file extension.
func WithFileDecoder(d decoder.Decoder) FileOption {
	return func(f *FileLoader) { f.dec = d }
}

// WithFilePollInterval sets the polling interval for watching file changes.
func WithFilePollInterval(d time.Duration) FileOption {
	return func(f *FileLoader) { f.interval = d }
}

// NewFileLoader creates a new FileLoader for the given file path.
func NewFileLoader(path string, opts ...FileOption) *FileLoader {
	cleanPath := filepath.Clean(path)
	ctrl := pollwatch.NewController(providerbase.DefaultEventBufSize)
	base := NewBase("file:"+cleanPath, "file", PriorityFile)
	f := &FileLoader{
		PollableSource: providerbase.NewPollableSource(ctrl),
		path:           cleanPath,
	}
	f.Base = base
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Load reads and decodes the file, returning a map of configuration values.
// If the file content has not changed since the last call (checksum match),
// it returns the PollableSource's cached last data for efficiency.
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
		f.mu.RUnlock()
		return f.LastData(), nil
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
	f.SetLastData(result)
	f.mu.Unlock()
	return result, nil
}

// Watch starts monitoring the file for changes using the unified PollableSource.
// This replaces the previous hand-rolled StartPolling+EmitDiff implementation.
func (f *FileLoader) Watch(ctx context.Context) (<-chan event.Event, error) {
	if f.interval <= 0 {
		return nil, nil
	}
	return f.PollableSource.Watch(ctx, f.interval, f.Load,
		event.WithLabel("source", "file"),
		event.WithLabel("type", f.Type()),
	)
}

// Close stops watching and closes the loader.
func (f *FileLoader) Close(ctx context.Context) error {
	return f.Base.Close(ctx)
}

func computeChecksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])[:16]
}

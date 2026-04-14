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
)

// FileLoader loads config from a file on disk.
// The format is auto-detected from the file extension using decoder.DefaultRegistry,
// or an explicit Decoder may be supplied via WithFileDecoder.
//
// FileLoader caches the last-decoded result by file checksum to avoid
// re-parsing unchanged files on repeated Load calls or polling cycles.
type FileLoader struct {
	*Base
	*Controller // embedded for Watch support; nil if polling disabled

	path     string
	dec      decoder.Decoder // nil = auto-detect
	interval time.Duration   // 0 = no polling

	// mu protects lastChecksum and lastData.
	mu           sync.RWMutex
	lastChecksum string
	lastData     map[string]value.Value
}

var _ Loader = (*FileLoader)(nil)

// FileOption configures a FileLoader during creation.
type FileOption func(*FileLoader)

// WithFilePriority sets the merge priority.
func WithFilePriority(p int) FileOption {
	return func(f *FileLoader) { f.SetPriority(p) }
}

// WithFileDecoder sets an explicit decoder, bypassing extension auto-detection.
func WithFileDecoder(d decoder.Decoder) FileOption {
	return func(f *FileLoader) { f.dec = d }
}

// WithFilePollInterval enables polling at the given interval.
// Zero disables polling (file-system events used if available; fallback to no watch).
func WithFilePollInterval(d time.Duration) FileOption {
	return func(f *FileLoader) { f.interval = d }
}

// NewFileLoader creates a FileLoader that reads from the given file path.
// The file format is auto-detected from the extension unless an explicit
// decoder is provided via WithFileDecoder.
func NewFileLoader(path string, opts ...FileOption) *FileLoader {
	cleanPath := filepath.Clean(path)
	base := NewBase("file:"+cleanPath, "file", 30)
	f := &FileLoader{
		path:     cleanPath,
		lastData: make(map[string]value.Value),
	}
	f.Base = base
	f.Controller = NewController(64)
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Load implements Loader.
// It reads the file, checks its checksum against the cached value,
// and only decodes when the content has changed.
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

// Watch implements Loader. If a poll interval is configured, it starts a
// polling goroutine. Otherwise returns (nil, nil).
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

// Close implements Loader.
func (f *FileLoader) Close(ctx context.Context) error {
	if f.Controller != nil {
		f.Controller.Close()
	}
	return f.Base.Close(ctx)
}

// lastDataSafe returns a safe copy of the last loaded data.
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

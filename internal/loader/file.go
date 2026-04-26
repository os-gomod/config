package loader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/os-gomod/config/v2/internal/decoder"
	"github.com/os-gomod/config/v2/internal/domain/event"
	"github.com/os-gomod/config/v2/internal/domain/value"
)

// ---------------------------------------------------------------------------
// FileLoader
// ---------------------------------------------------------------------------

// FileLoader reads configuration from files on disk and optionally watches
// for changes using a polling mechanism.
type FileLoader struct {
	*Base
	paths         []string
	decoder       decoder.Decoder
	watchInterval time.Duration
	mu            sync.RWMutex
	lastModTimes  map[string]time.Time
}

// FileOption configures a FileLoader during construction.
type FileOption func(*FileLoader)

// WithWatchInterval sets the polling interval for file watching.
// Defaults to 5 seconds if not specified.
func WithWatchInterval(d time.Duration) FileOption {
	return func(f *FileLoader) {
		if d > 0 {
			f.watchInterval = d
		}
	}
}

// WithDecoder sets the decoder explicitly. If not set, the decoder is
// determined by file extension.
func WithDecoder(d decoder.Decoder) FileOption {
	return func(f *FileLoader) {
		f.decoder = d
	}
}

// WithPriority sets the loader priority.
func WithPriority(p int) FileOption {
	return func(f *FileLoader) {
		f.priority = p
	}
}

// NewFileLoader creates a new FileLoader for the given paths.
// If no decoder is specified via options, it is auto-detected from the
// first file's extension.
func NewFileLoader(name string, paths []string, dec decoder.Decoder, opts ...FileOption) *FileLoader {
	f := &FileLoader{
		Base:          NewBase(name, "file", 0),
		paths:         paths,
		decoder:       dec,
		watchInterval: 5 * time.Second,
		lastModTimes:  make(map[string]time.Time, len(paths)),
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Load reads all configured files, decodes them, and merges the results
// into a single map. If multiple files define the same key, the last file wins.
func (f *FileLoader) Load(ctx context.Context) (map[string]value.Value, error) {
	if err := f.CheckClosed(); err != nil {
		return nil, err
	}

	result := make(map[string]value.Value)

	for _, path := range f.paths {
		select {
		case <-ctx.Done():
			return nil, f.WrapErr(ctx.Err(), "load")
		default:
		}

		data, err := f.loadFile(ctx, path)
		if err != nil {
			return nil, f.WrapErr(err, "load")
		}

		// Merge file data into result (later files override earlier ones).
		for k, v := range data {
			result[k] = v
		}
	}

	return result, nil
}

// loadFile reads a single file, decodes it, and returns the data as
// a map of Values.
func (f *FileLoader) loadFile(_ context.Context, path string) (map[string]value.Value, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path %q: %w", path, err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat %q: %w", absPath, err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("%q is a directory, not a file", absPath)
	}

	src, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", absPath, err)
	}

	dec := f.decoder
	if dec == nil {
		dec, err = decoderForExtension(absPath)
		if err != nil {
			return nil, err
		}
	}

	rawMap, err := dec.Decode(src)
	if err != nil {
		return nil, fmt.Errorf("decode %q: %w", absPath, err)
	}
	flatMap := flattenConfigMap(rawMap)

	// Update last modification time.
	f.mu.Lock()
	f.lastModTimes[absPath] = info.ModTime()
	f.mu.Unlock()

	// Convert to map[string]value.Value.
	result := make(map[string]value.Value, len(flatMap))
	for k, v := range flatMap {
		result[k] = value.FromRaw(v, value.TypeUnknown, value.SourceFile, f.priority)
	}

	return result, nil
}

// Watch starts watching files for changes using a polling mechanism.
// It returns a channel that receives events when files change.
func (f *FileLoader) Watch(ctx context.Context) (<-chan event.Event, error) {
	if err := f.CheckClosed(); err != nil {
		return nil, err
	}

	ch := make(chan event.Event, 16)

	go f.watchLoop(ctx, ch)

	return ch, nil
}

// watchLoop polls files at the configured interval and sends change events.
func (f *FileLoader) watchLoop(ctx context.Context, ch chan<- event.Event) {
	defer close(ch)

	ticker := time.NewTicker(f.watchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.checkChanges(ctx, ch)
		}
	}
}

// checkChanges checks all watched files for modifications.
func (f *FileLoader) checkChanges(ctx context.Context, ch chan<- event.Event) {
	for _, path := range f.paths {
		select {
		case <-ctx.Done():
			return
		default:
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}

		info, err := os.Stat(absPath)
		if err != nil {
			// File might have been deleted — send error event.
			ch <- event.NewErrorEvent(absPath, err,
				event.WithSource(f.name),
			)
			continue
		}

		f.mu.RLock()
		lastMod, exists := f.lastModTimes[absPath]
		f.mu.RUnlock()

		if exists && info.ModTime().After(lastMod) {
			// File was modified — reload and emit reload event.
			evt := event.NewReloadEvent(f.name,
				event.WithMetadata(map[string]any{
					"path":     absPath,
					"old_time": lastMod.String(),
					"new_time": info.ModTime().String(),
				}),
			)
			ch <- evt
		} else if !exists {
			// New file detected.
			f.mu.Lock()
			f.lastModTimes[absPath] = info.ModTime()
			f.mu.Unlock()
		}
	}
}

// Close implements Loader.Close and releases file handles.
func (f *FileLoader) Close(_ context.Context) error {
	_ = f.CloseBase()
	return nil
}

// Paths returns the configured file paths.
func (f *FileLoader) Paths() []string {
	return append([]string{}, f.paths...)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// decoderForExtension selects a decoder based on file extension.
// Falls back to YAML for unknown extensions.
func decoderForExtension(path string) (decoder.Decoder, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return decoder.NewYAMLDecoder(), nil
	case ".json":
		return decoder.NewJSONDecoder(), nil
	case ".toml":
		return decoder.NewTOMLDecoder(), nil
	case ".env":
		return decoder.NewEnvDecoder(), nil
	default:
		return nil, fmt.Errorf("unsupported file extension %q for path %q", ext, path)
	}
}

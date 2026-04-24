package providerbase

import (
	"context"
	"sync"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/pollwatch"
)

// PollableSource provides unified polling semantics for any data source.
// It wraps a pollwatch.Controller and adds last-data tracking for diff emission.
//
// This is the single polling implementation that replaces the duplicated
// Watch+PollLoadAndWatch pattern that was previously repeated in:
//   - loader/polling.go  (PollingLoader)
//   - loader/file.go     (FileLoader)
//   - provider/provider.go (BaseProvider.PollLoadAndWatch)
//
// Usage:
//
//	ps := providerbase.NewPollableSource(controller)
//	ch, _ := ps.Watch(ctx, 5*time.Second, myLoadFn,
//	    event.WithLabel("source", "my-source"),
//	)
type PollableSource struct {
	ctrl     *pollwatch.Controller
	mu       sync.RWMutex
	lastData map[string]value.Value
}

// NewPollableSource creates a PollableSource backed by the given pollwatch.Controller.
// The controller must not be nil.
func NewPollableSource(ctrl *pollwatch.Controller) *PollableSource {
	return &PollableSource{ctrl: ctrl}
}

// Watch starts a polling loop that calls loadFn at the given interval,
// diffs against the last known data, and emits change events through
// the pollwatch controller's event channel.
//
// On the first successful load, data is captured without emitting events
// (because there is no previous state to diff against). Subsequent loads
// that produce different data will emit create/update/delete events.
//
// Returns nil if interval <= 0, indicating the source does not support
// polling-based watching.
func (ps *PollableSource) Watch(
	ctx context.Context,
	interval time.Duration,
	loadFn func(context.Context) (map[string]value.Value, error),
	labels ...event.Option,
) (<-chan event.Event, error) {
	if interval <= 0 {
		return nil, nil
	}
	return ps.ctrl.StartPolling(ctx, interval, func(ctx context.Context) {
		newData, err := loadFn(ctx)
		if err != nil {
			return
		}
		oldData := ps.swapLastData(newData)
		if oldData != nil {
			_ = ps.ctrl.EmitDiff(ctx, oldData, newData, labels...)
		}
	}), nil
}

// LastData returns a copy of the last successfully loaded data.
// Returns nil if no data has been loaded yet.
func (ps *PollableSource) LastData() map[string]value.Value {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return value.Copy(ps.lastData)
}

// SetLastData replaces the tracked last data with a copy of the given data.
func (ps *PollableSource) SetLastData(data map[string]value.Value) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.lastData = data
}

// swapLastData atomically replaces the last data and returns the previous value.
// The caller is responsible for diffing old vs new.
func (ps *PollableSource) swapLastData(data map[string]value.Value) map[string]value.Value {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	old := ps.lastData
	ps.lastData = data
	return old
}

package providerbase

import (
	"context"

	"github.com/os-gomod/config/internal/common"
	"github.com/os-gomod/config/internal/pollwatch"
)

// DefaultEventBufSize is the default buffer size for event channels
// used by providers and watchers throughout the config system.
const DefaultEventBufSize = 64

// ProviderLifecycle combines the generic ClientLifecycle mixin with
// poll watch capabilities needed by all remote providers.
//
// It manages:
//   - Open/close state (via ClientLifecycle.Closable)
//   - Client initialization mutex (Mu) — guards lazy init
//   - Watch deduplication mutex (WatchMu) — prevents concurrent watchers
//   - Poll-based event emission (via pollwatch.Controller)
//
// This eliminates the repeated embedding of *common.Closable, sync.Mutex,
// sync.Mutex, and *pollwatch.Controller that was previously duplicated
// across BaseProvider, FileLoader, and PollingLoader.
type ProviderLifecycle struct {
	*common.ClientLifecycle
	Controller *pollwatch.Controller
}

// NewProviderLifecycle creates a ProviderLifecycle backed by the given
// pollwatch.Controller. Both the ClientLifecycle and the controller
// share the same lifecycle — CloseProvider shuts them both down.
func NewProviderLifecycle(ctrl *pollwatch.Controller) *ProviderLifecycle {
	return &ProviderLifecycle{
		ClientLifecycle: common.NewClientLifecycle(),
		Controller:      ctrl,
	}
}

// CloseProvider performs an orderly shutdown in the correct sequence:
//  1. Acquires WatchMu to prevent new watchers from starting
//  2. Stops any active polling via the controller
//  3. Closes the underlying ClientLifecycle (signals Done)
func (pl *ProviderLifecycle) CloseProvider(ctx context.Context) error {
	pl.WatchMu.Lock()
	defer pl.WatchMu.Unlock()
	pl.Controller.Close()
	pl.Close(ctx)
	return nil
}

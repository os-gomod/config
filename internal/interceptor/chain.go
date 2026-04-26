package interceptor

import (
	"context"
	"sync"
)

// Chain manages ordered execution of typed interceptors.
// Interceptors are executed in registration order (first registered runs first).
// If any interceptor returns an error, execution stops and the error is propagated.
//
// Chain is safe for concurrent use. Interceptors can be added at any time,
// and execution takes a snapshot of registered interceptors under read lock.
type Chain struct {
	setInterceptors    []SetInterceptor
	deleteInterceptors []DeleteInterceptor
	reloadInterceptors []ReloadInterceptor
	bindInterceptors   []BindInterceptor
	closeInterceptors  []CloseInterceptor
	mu                 sync.RWMutex
}

// NewChain creates an empty interceptor chain.
func NewChain() *Chain {
	return &Chain{}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

// AddSetInterceptor adds a SetInterceptor to the chain.
// Interceptors are called in the order they are added.
func (c *Chain) AddSetInterceptor(i SetInterceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setInterceptors = append(c.setInterceptors, i)
}

// AddDeleteInterceptor adds a DeleteInterceptor to the chain.
func (c *Chain) AddDeleteInterceptor(i DeleteInterceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleteInterceptors = append(c.deleteInterceptors, i)
}

// AddReloadInterceptor adds a ReloadInterceptor to the chain.
func (c *Chain) AddReloadInterceptor(i ReloadInterceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reloadInterceptors = append(c.reloadInterceptors, i)
}

// AddBindInterceptor adds a BindInterceptor to the chain.
func (c *Chain) AddBindInterceptor(i BindInterceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bindInterceptors = append(c.bindInterceptors, i)
}

// AddCloseInterceptor adds a CloseInterceptor to the chain.
func (c *Chain) AddCloseInterceptor(i CloseInterceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeInterceptors = append(c.closeInterceptors, i)
}

// ---------------------------------------------------------------------------
// Set execution
// ---------------------------------------------------------------------------

//nolint:dupl // interceptor chain pattern produces similar code
func (c *Chain) BeforeSet(ctx context.Context, req *SetRequest) error {
	c.mu.RLock()
	interceptors := make([]SetInterceptor, len(c.setInterceptors))
	copy(interceptors, c.setInterceptors)
	c.mu.RUnlock()

	for _, i := range interceptors {
		if err := i.BeforeSet(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

//nolint:dupl // interceptor chain pattern produces similar code
func (c *Chain) AfterSet(ctx context.Context, req *SetRequest, res *SetResponse) error {
	c.mu.RLock()
	interceptors := make([]SetInterceptor, len(c.setInterceptors))
	copy(interceptors, c.setInterceptors)
	c.mu.RUnlock()

	for _, i := range interceptors {
		if err := i.AfterSet(ctx, req, res); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Delete execution
// ---------------------------------------------------------------------------

//nolint:dupl // interceptor chain pattern produces similar code
func (c *Chain) BeforeDelete(ctx context.Context, req *DeleteRequest) error {
	c.mu.RLock()
	interceptors := make([]DeleteInterceptor, len(c.deleteInterceptors))
	copy(interceptors, c.deleteInterceptors)
	c.mu.RUnlock()

	for _, i := range interceptors {
		if err := i.BeforeDelete(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

//nolint:dupl // interceptor chain pattern produces similar code
func (c *Chain) AfterDelete(ctx context.Context, req *DeleteRequest, res *DeleteResponse) error {
	c.mu.RLock()
	interceptors := make([]DeleteInterceptor, len(c.deleteInterceptors))
	copy(interceptors, c.deleteInterceptors)
	c.mu.RUnlock()

	for _, i := range interceptors {
		if err := i.AfterDelete(ctx, req, res); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Reload execution
// ---------------------------------------------------------------------------

//nolint:dupl // interceptor chain pattern produces similar code
func (c *Chain) BeforeReload(ctx context.Context, req *ReloadRequest) error {
	c.mu.RLock()
	interceptors := make([]ReloadInterceptor, len(c.reloadInterceptors))
	copy(interceptors, c.reloadInterceptors)
	c.mu.RUnlock()

	for _, i := range interceptors {
		if err := i.BeforeReload(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

//nolint:dupl // interceptor chain pattern produces similar code
func (c *Chain) AfterReload(ctx context.Context, req *ReloadRequest, res *ReloadResponse) error {
	c.mu.RLock()
	interceptors := make([]ReloadInterceptor, len(c.reloadInterceptors))
	copy(interceptors, c.reloadInterceptors)
	c.mu.RUnlock()

	for _, i := range interceptors {
		if err := i.AfterReload(ctx, req, res); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Bind execution
// ---------------------------------------------------------------------------

//nolint:dupl // interceptor chain pattern produces similar code
func (c *Chain) BeforeBind(ctx context.Context, req *BindRequest) error {
	c.mu.RLock()
	interceptors := make([]BindInterceptor, len(c.bindInterceptors))
	copy(interceptors, c.bindInterceptors)
	c.mu.RUnlock()

	for _, i := range interceptors {
		if err := i.BeforeBind(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

//nolint:dupl // interceptor chain pattern produces similar code
func (c *Chain) AfterBind(ctx context.Context, req *BindRequest, res *BindResponse) error {
	c.mu.RLock()
	interceptors := make([]BindInterceptor, len(c.bindInterceptors))
	copy(interceptors, c.bindInterceptors)
	c.mu.RUnlock()

	for _, i := range interceptors {
		if err := i.AfterBind(ctx, req, res); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Close execution
// ---------------------------------------------------------------------------

//nolint:dupl // interceptor chain pattern produces similar code
func (c *Chain) BeforeClose(ctx context.Context, req *CloseRequest) error {
	c.mu.RLock()
	interceptors := make([]CloseInterceptor, len(c.closeInterceptors))
	copy(interceptors, c.closeInterceptors)
	c.mu.RUnlock()

	for _, i := range interceptors {
		if err := i.BeforeClose(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

//nolint:dupl // interceptor chain pattern produces similar code
func (c *Chain) AfterClose(ctx context.Context, req *CloseRequest, res *CloseResponse) error {
	c.mu.RLock()
	interceptors := make([]CloseInterceptor, len(c.closeInterceptors))
	copy(interceptors, c.closeInterceptors)
	c.mu.RUnlock()

	for _, i := range interceptors {
		if err := i.AfterClose(ctx, req, res); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Has* inspection methods
// ---------------------------------------------------------------------------

// HasSetInterceptors returns true if any SetInterceptors are registered.
func (c *Chain) HasSetInterceptors() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.setInterceptors) > 0
}

// HasDeleteInterceptors returns true if any DeleteInterceptors are registered.
func (c *Chain) HasDeleteInterceptors() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.deleteInterceptors) > 0
}

// HasReloadInterceptors returns true if any ReloadInterceptors are registered.
func (c *Chain) HasReloadInterceptors() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.reloadInterceptors) > 0
}

// HasBindInterceptors returns true if any BindInterceptors are registered.
func (c *Chain) HasBindInterceptors() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.bindInterceptors) > 0
}

// HasCloseInterceptors returns true if any CloseInterceptors are registered.
func (c *Chain) HasCloseInterceptors() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.closeInterceptors) > 0
}

// ---------------------------------------------------------------------------
// Size inspection methods
// ---------------------------------------------------------------------------

// SetInterceptorCount returns the number of registered SetInterceptors.
func (c *Chain) SetInterceptorCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.setInterceptors)
}

// DeleteInterceptorCount returns the number of registered DeleteInterceptors.
func (c *Chain) DeleteInterceptorCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.deleteInterceptors)
}

// ReloadInterceptorCount returns the number of registered ReloadInterceptors.
func (c *Chain) ReloadInterceptorCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.reloadInterceptors)
}

// BindInterceptorCount returns the number of registered BindInterceptors.
func (c *Chain) BindInterceptorCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.bindInterceptors)
}

// CloseInterceptorCount returns the number of registered CloseInterceptors.
func (c *Chain) CloseInterceptorCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.closeInterceptors)
}

// TotalCount returns the total number of all registered interceptors.
func (c *Chain) TotalCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.setInterceptors) +
		len(c.deleteInterceptors) +
		len(c.reloadInterceptors) +
		len(c.bindInterceptors) +
		len(c.closeInterceptors)
}

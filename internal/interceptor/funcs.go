package interceptor

import (
	"context"
)

// ---------------------------------------------------------------------------
// SetFunc — functional adapter for SetInterceptor
// ---------------------------------------------------------------------------

// SetFunc allows creating a SetInterceptor from two function literals
// instead of defining a full struct. Nil functions are treated as no-ops.
//
// Usage:
//
//	chain.AddSetInterceptor(&interceptor.SetFunc{
//	    BeforeFn: func(ctx context.Context, req *interceptor.SetRequest) error {
//	        if req.Key == "" {
//	            return errors.New("key is required")
//	        }
//	        return nil
//	    },
//	})
type SetFunc struct {
	BeforeFn func(ctx context.Context, req *SetRequest) error
	AfterFn  func(ctx context.Context, req *SetRequest, res *SetResponse) error
}

// BeforeSet calls the BeforeFn if it is non-nil; otherwise returns nil.
func (f *SetFunc) BeforeSet(ctx context.Context, req *SetRequest) error {
	if f.BeforeFn != nil {
		return f.BeforeFn(ctx, req)
	}
	return nil
}

// AfterSet calls the AfterFn if it is non-nil; otherwise returns nil.
func (f *SetFunc) AfterSet(ctx context.Context, req *SetRequest, res *SetResponse) error {
	if f.AfterFn != nil {
		return f.AfterFn(ctx, req, res)
	}
	return nil
}

// Ensure SetFunc implements SetInterceptor at compile time.
var _ SetInterceptor = (*SetFunc)(nil)

// ---------------------------------------------------------------------------
// DeleteFunc — functional adapter for DeleteInterceptor
// ---------------------------------------------------------------------------

// DeleteFunc allows creating a DeleteInterceptor from two function literals.
type DeleteFunc struct {
	BeforeFn func(ctx context.Context, req *DeleteRequest) error
	AfterFn  func(ctx context.Context, req *DeleteRequest, res *DeleteResponse) error
}

// BeforeDelete calls the BeforeFn if it is non-nil; otherwise returns nil.
func (f *DeleteFunc) BeforeDelete(ctx context.Context, req *DeleteRequest) error {
	if f.BeforeFn != nil {
		return f.BeforeFn(ctx, req)
	}
	return nil
}

// AfterDelete calls the AfterFn if it is non-nil; otherwise returns nil.
func (f *DeleteFunc) AfterDelete(ctx context.Context, req *DeleteRequest, res *DeleteResponse) error {
	if f.AfterFn != nil {
		return f.AfterFn(ctx, req, res)
	}
	return nil
}

// Ensure DeleteFunc implements DeleteInterceptor at compile time.
var _ DeleteInterceptor = (*DeleteFunc)(nil)

// ---------------------------------------------------------------------------
// ReloadFunc — functional adapter for ReloadInterceptor
// ---------------------------------------------------------------------------

// ReloadFunc allows creating a ReloadInterceptor from two function literals.
type ReloadFunc struct {
	BeforeFn func(ctx context.Context, req *ReloadRequest) error
	AfterFn  func(ctx context.Context, req *ReloadRequest, res *ReloadResponse) error
}

// BeforeReload calls the BeforeFn if it is non-nil; otherwise returns nil.
func (f *ReloadFunc) BeforeReload(ctx context.Context, req *ReloadRequest) error {
	if f.BeforeFn != nil {
		return f.BeforeFn(ctx, req)
	}
	return nil
}

// AfterReload calls the AfterFn if it is non-nil; otherwise returns nil.
func (f *ReloadFunc) AfterReload(ctx context.Context, req *ReloadRequest, res *ReloadResponse) error {
	if f.AfterFn != nil {
		return f.AfterFn(ctx, req, res)
	}
	return nil
}

// Ensure ReloadFunc implements ReloadInterceptor at compile time.
var _ ReloadInterceptor = (*ReloadFunc)(nil)

// ---------------------------------------------------------------------------
// BindFunc — functional adapter for BindInterceptor
// ---------------------------------------------------------------------------

// BindFunc allows creating a BindInterceptor from two function literals.
type BindFunc struct {
	BeforeFn func(ctx context.Context, req *BindRequest) error
	AfterFn  func(ctx context.Context, req *BindRequest, res *BindResponse) error
}

// BeforeBind calls the BeforeFn if it is non-nil; otherwise returns nil.
func (f *BindFunc) BeforeBind(ctx context.Context, req *BindRequest) error {
	if f.BeforeFn != nil {
		return f.BeforeFn(ctx, req)
	}
	return nil
}

// AfterBind calls the AfterFn if it is non-nil; otherwise returns nil.
func (f *BindFunc) AfterBind(ctx context.Context, req *BindRequest, res *BindResponse) error {
	if f.AfterFn != nil {
		return f.AfterFn(ctx, req, res)
	}
	return nil
}

// Ensure BindFunc implements BindInterceptor at compile time.
var _ BindInterceptor = (*BindFunc)(nil)

// ---------------------------------------------------------------------------
// CloseFunc — functional adapter for CloseInterceptor
// ---------------------------------------------------------------------------

// CloseFunc allows creating a CloseInterceptor from two function literals.
type CloseFunc struct {
	BeforeFn func(ctx context.Context, req *CloseRequest) error
	AfterFn  func(ctx context.Context, req *CloseRequest, res *CloseResponse) error
}

// BeforeClose calls the BeforeFn if it is non-nil; otherwise returns nil.
func (f *CloseFunc) BeforeClose(ctx context.Context, req *CloseRequest) error {
	if f.BeforeFn != nil {
		return f.BeforeFn(ctx, req)
	}
	return nil
}

// AfterClose calls the AfterFn if it is non-nil; otherwise returns nil.
func (f *CloseFunc) AfterClose(ctx context.Context, req *CloseRequest, res *CloseResponse) error {
	if f.AfterFn != nil {
		return f.AfterFn(ctx, req, res)
	}
	return nil
}

// Ensure CloseFunc implements CloseInterceptor at compile time.
var _ CloseInterceptor = (*CloseFunc)(nil)

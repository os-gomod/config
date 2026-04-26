package interceptor

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Chain creation
// ---------------------------------------------------------------------------

func TestNewChain(t *testing.T) {
	c := NewChain()
	require.NotNil(t, c)
	assert.False(t, c.HasSetInterceptors())
	assert.False(t, c.HasDeleteInterceptors())
	assert.False(t, c.HasReloadInterceptors())
	assert.False(t, c.HasBindInterceptors())
	assert.False(t, c.HasCloseInterceptors())
	assert.Equal(t, 0, c.TotalCount())
}

// ---------------------------------------------------------------------------
// Add and execute Set interceptors
// ---------------------------------------------------------------------------

func TestBeforeSet(t *testing.T) {
	c := NewChain()

	var called bool
	c.AddSetInterceptor(&SetFunc{
		BeforeFn: func(ctx context.Context, req *SetRequest) error {
			called = true
			assert.Equal(t, "my.key", req.Key)
			assert.Equal(t, 42, req.Value)
			return nil
		},
	})

	err := c.BeforeSet(context.Background(), &SetRequest{Key: "my.key", Value: 42})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestAfterSet(t *testing.T) {
	c := NewChain()

	var called bool
	c.AddSetInterceptor(&SetFunc{
		AfterFn: func(ctx context.Context, req *SetRequest, res *SetResponse) error {
			called = true
			assert.Equal(t, "my.key", req.Key)
			assert.Equal(t, "new.key", res.Key)
			return nil
		},
	})

	err := c.AfterSet(context.Background(),
		&SetRequest{Key: "my.key", Value: 42},
		&SetResponse{Key: "new.key"},
	)
	require.NoError(t, err)
	assert.True(t, called)
}

// ---------------------------------------------------------------------------
// Add and execute Delete interceptors
// ---------------------------------------------------------------------------

func TestBeforeDelete(t *testing.T) {
	c := NewChain()

	var called bool
	c.AddDeleteInterceptor(&DeleteFunc{
		BeforeFn: func(ctx context.Context, req *DeleteRequest) error {
			called = true
			assert.Equal(t, "rm.key", req.Key)
			return nil
		},
	})

	err := c.BeforeDelete(context.Background(), &DeleteRequest{Key: "rm.key"})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestAfterDelete(t *testing.T) {
	c := NewChain()

	var called bool
	c.AddDeleteInterceptor(&DeleteFunc{
		AfterFn: func(ctx context.Context, req *DeleteRequest, res *DeleteResponse) error {
			called = true
			assert.True(t, res.Found)
			return nil
		},
	})

	err := c.AfterDelete(context.Background(),
		&DeleteRequest{Key: "rm.key"},
		&DeleteResponse{Found: true},
	)
	require.NoError(t, err)
	assert.True(t, called)
}

// ---------------------------------------------------------------------------
// Reload interceptors
// ---------------------------------------------------------------------------

func TestBeforeReload(t *testing.T) {
	c := NewChain()

	var called bool
	c.AddReloadInterceptor(&ReloadFunc{
		BeforeFn: func(ctx context.Context, req *ReloadRequest) error {
			called = true
			return nil
		},
	})

	err := c.BeforeReload(context.Background(), &ReloadRequest{})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestAfterReload(t *testing.T) {
	c := NewChain()

	var called bool
	c.AddReloadInterceptor(&ReloadFunc{
		AfterFn: func(ctx context.Context, req *ReloadRequest, res *ReloadResponse) error {
			called = true
			assert.True(t, res.Changed)
			return nil
		},
	})

	err := c.AfterReload(context.Background(),
		&ReloadRequest{},
		&ReloadResponse{Changed: true},
	)
	require.NoError(t, err)
	assert.True(t, called)
}

// ---------------------------------------------------------------------------
// Bind interceptors
// ---------------------------------------------------------------------------

func TestBeforeBind(t *testing.T) {
	c := NewChain()

	var target struct{ Name string }
	var called bool
	c.AddBindInterceptor(&BindFunc{
		BeforeFn: func(ctx context.Context, req *BindRequest) error {
			called = true
			assert.NotNil(t, req.Target)
			return nil
		},
	})

	err := c.BeforeBind(context.Background(), &BindRequest{Target: &target})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestAfterBind(t *testing.T) {
	c := NewChain()

	var called bool
	c.AddBindInterceptor(&BindFunc{
		AfterFn: func(ctx context.Context, req *BindRequest, res *BindResponse) error {
			called = true
			assert.NotNil(t, res.Target)
			return nil
		},
	})

	err := c.AfterBind(context.Background(),
		&BindRequest{Target: &struct{}{}},
		&BindResponse{Target: &struct{}{}},
	)
	require.NoError(t, err)
	assert.True(t, called)
}

// ---------------------------------------------------------------------------
// Close interceptors
// ---------------------------------------------------------------------------

func TestBeforeClose(t *testing.T) {
	c := NewChain()

	var called bool
	c.AddCloseInterceptor(&CloseFunc{
		BeforeFn: func(ctx context.Context, req *CloseRequest) error {
			called = true
			return nil
		},
	})

	err := c.BeforeClose(context.Background(), &CloseRequest{})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestAfterClose(t *testing.T) {
	c := NewChain()

	var called bool
	c.AddCloseInterceptor(&CloseFunc{
		AfterFn: func(ctx context.Context, req *CloseRequest, res *CloseResponse) error {
			called = true
			return nil
		},
	})

	err := c.AfterClose(context.Background(), &CloseRequest{}, &CloseResponse{})
	require.NoError(t, err)
	assert.True(t, called)
}

// ---------------------------------------------------------------------------
// Execution order
// ---------------------------------------------------------------------------

func TestBeforeSet_ExecutionOrder(t *testing.T) {
	c := NewChain()

	var order []int
	c.AddSetInterceptor(&SetFunc{
		BeforeFn: func(ctx context.Context, req *SetRequest) error {
			order = append(order, 1)
			return nil
		},
	})
	c.AddSetInterceptor(&SetFunc{
		BeforeFn: func(ctx context.Context, req *SetRequest) error {
			order = append(order, 2)
			return nil
		},
	})
	c.AddSetInterceptor(&SetFunc{
		BeforeFn: func(ctx context.Context, req *SetRequest) error {
			order = append(order, 3)
			return nil
		},
	})

	err := c.BeforeSet(context.Background(), &SetRequest{})
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, order)
}

func TestAfterSet_ExecutionOrder(t *testing.T) {
	c := NewChain()

	var order []int
	c.AddSetInterceptor(&SetFunc{
		AfterFn: func(ctx context.Context, req *SetRequest, res *SetResponse) error {
			order = append(order, 1)
			return nil
		},
	})
	c.AddSetInterceptor(&SetFunc{
		AfterFn: func(ctx context.Context, req *SetRequest, res *SetResponse) error {
			order = append(order, 2)
			return nil
		},
	})

	err := c.AfterSet(context.Background(), &SetRequest{}, &SetResponse{})
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, order)
}

// ---------------------------------------------------------------------------
// Error stopping
// ---------------------------------------------------------------------------

func TestBeforeSet_StopsOnFirstError(t *testing.T) {
	c := NewChain()

	var secondCalled bool
	c.AddSetInterceptor(&SetFunc{
		BeforeFn: func(ctx context.Context, req *SetRequest) error {
			return errors.New("first interceptor error")
		},
	})
	c.AddSetInterceptor(&SetFunc{
		BeforeFn: func(ctx context.Context, req *SetRequest) error {
			secondCalled = true
			return nil
		},
	})

	err := c.BeforeSet(context.Background(), &SetRequest{})
	require.Error(t, err)
	assert.Equal(t, "first interceptor error", err.Error())
	assert.False(t, secondCalled, "second interceptor should not be called")
}

func TestAfterSet_StopsOnFirstError(t *testing.T) {
	c := NewChain()

	c.AddSetInterceptor(&SetFunc{
		AfterFn: func(ctx context.Context, req *SetRequest, res *SetResponse) error {
			return errors.New("after error")
		},
	})
	c.AddSetInterceptor(&SetFunc{
		AfterFn: func(ctx context.Context, req *SetRequest, res *SetResponse) error {
			return nil
		},
	})

	err := c.AfterSet(context.Background(), &SetRequest{}, &SetResponse{})
	require.Error(t, err)
}

func TestBeforeDelete_StopsOnError(t *testing.T) {
	c := NewChain()

	c.AddDeleteInterceptor(&DeleteFunc{
		BeforeFn: func(ctx context.Context, req *DeleteRequest) error {
			return errors.New("delete blocked")
		},
	})

	err := c.BeforeDelete(context.Background(), &DeleteRequest{Key: "x"})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Functional adapters — nil functions are no-ops
// ---------------------------------------------------------------------------

func TestSetFunc_NilFunctions(t *testing.T) {
	f := &SetFunc{}
	err := f.BeforeSet(context.Background(), &SetRequest{})
	assert.NoError(t, err)
	err = f.AfterSet(context.Background(), &SetRequest{}, &SetResponse{})
	assert.NoError(t, err)
}

func TestDeleteFunc_NilFunctions(t *testing.T) {
	f := &DeleteFunc{}
	err := f.BeforeDelete(context.Background(), &DeleteRequest{})
	assert.NoError(t, err)
	err = f.AfterDelete(context.Background(), &DeleteRequest{}, &DeleteResponse{})
	assert.NoError(t, err)
}

func TestReloadFunc_NilFunctions(t *testing.T) {
	f := &ReloadFunc{}
	err := f.BeforeReload(context.Background(), &ReloadRequest{})
	assert.NoError(t, err)
	err = f.AfterReload(context.Background(), &ReloadRequest{}, &ReloadResponse{})
	assert.NoError(t, err)
}

func TestBindFunc_NilFunctions(t *testing.T) {
	f := &BindFunc{}
	err := f.BeforeBind(context.Background(), &BindRequest{})
	assert.NoError(t, err)
	err = f.AfterBind(context.Background(), &BindRequest{}, &BindResponse{})
	assert.NoError(t, err)
}

func TestCloseFunc_NilFunctions(t *testing.T) {
	f := &CloseFunc{}
	err := f.BeforeClose(context.Background(), &CloseRequest{})
	assert.NoError(t, err)
	err = f.AfterClose(context.Background(), &CloseRequest{}, &CloseResponse{})
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Has/Count inspection
// ---------------------------------------------------------------------------

func TestHasInterceptors(t *testing.T) {
	c := NewChain()

	assert.False(t, c.HasSetInterceptors())
	assert.False(t, c.HasDeleteInterceptors())

	c.AddSetInterceptor(&SetFunc{})
	assert.True(t, c.HasSetInterceptors())
	assert.Equal(t, 1, c.SetInterceptorCount())

	c.AddDeleteInterceptor(&DeleteFunc{})
	assert.True(t, c.HasDeleteInterceptors())
	assert.Equal(t, 1, c.DeleteInterceptorCount())

	c.AddReloadInterceptor(&ReloadFunc{})
	assert.True(t, c.HasReloadInterceptors())
	assert.Equal(t, 1, c.ReloadInterceptorCount())

	c.AddBindInterceptor(&BindFunc{})
	assert.True(t, c.HasBindInterceptors())
	assert.Equal(t, 1, c.BindInterceptorCount())

	c.AddCloseInterceptor(&CloseFunc{})
	assert.True(t, c.HasCloseInterceptors())
	assert.Equal(t, 1, c.CloseInterceptorCount())

	assert.Equal(t, 5, c.TotalCount())
}

func TestInterceptorCounts(t *testing.T) {
	c := NewChain()
	assert.Equal(t, 0, c.SetInterceptorCount())
	assert.Equal(t, 0, c.DeleteInterceptorCount())
	assert.Equal(t, 0, c.ReloadInterceptorCount())
	assert.Equal(t, 0, c.BindInterceptorCount())
	assert.Equal(t, 0, c.CloseInterceptorCount())

	c.AddSetInterceptor(&SetFunc{})
	c.AddSetInterceptor(&SetFunc{})
	assert.Equal(t, 2, c.SetInterceptorCount())
}

// ---------------------------------------------------------------------------
// Empty chain (no interceptors)
// ---------------------------------------------------------------------------

func TestEmptyChain_NoErrors(t *testing.T) {
	c := NewChain()

	assert.NoError(t, c.BeforeSet(context.Background(), &SetRequest{}))
	assert.NoError(t, c.AfterSet(context.Background(), &SetRequest{}, &SetResponse{}))
	assert.NoError(t, c.BeforeDelete(context.Background(), &DeleteRequest{}))
	assert.NoError(t, c.AfterDelete(context.Background(), &DeleteRequest{}, &DeleteResponse{}))
	assert.NoError(t, c.BeforeReload(context.Background(), &ReloadRequest{}))
	assert.NoError(t, c.AfterReload(context.Background(), &ReloadRequest{}, &ReloadResponse{}))
	assert.NoError(t, c.BeforeBind(context.Background(), &BindRequest{}))
	assert.NoError(t, c.AfterBind(context.Background(), &BindRequest{}, &BindResponse{}))
	assert.NoError(t, c.BeforeClose(context.Background(), &CloseRequest{}))
	assert.NoError(t, c.AfterClose(context.Background(), &CloseRequest{}, &CloseResponse{}))
}

// ---------------------------------------------------------------------------
// Compile-time interface checks
// ---------------------------------------------------------------------------

func TestSetFunc_ImplementsSetInterceptor(t *testing.T) {
	var _ SetInterceptor = (*SetFunc)(nil)
}

func TestDeleteFunc_ImplementsDeleteInterceptor(t *testing.T) {
	var _ DeleteInterceptor = (*DeleteFunc)(nil)
}

func TestReloadFunc_ImplementsReloadInterceptor(t *testing.T) {
	var _ ReloadInterceptor = (*ReloadFunc)(nil)
}

func TestBindFunc_ImplementsBindInterceptor(t *testing.T) {
	var _ BindInterceptor = (*BindFunc)(nil)
}

func TestCloseFunc_ImplementsCloseInterceptor(t *testing.T) {
	var _ CloseInterceptor = (*CloseFunc)(nil)
}

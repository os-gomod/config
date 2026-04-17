package event

import "testing"

func TestHookType_String(t *testing.T) {
	tests := []struct {
		ht   HookType
		want string
	}{
		{HookBeforeReload, "before_reload"},
		{HookAfterReload, "after_reload"},
		{HookBeforeSet, "before_set"},
		{HookAfterSet, "after_set"},
		{HookBeforeDelete, "before_delete"},
		{HookAfterDelete, "after_delete"},
		{HookBeforeValidate, "before_validate"},
		{HookAfterValidate, "after_validate"},
		{HookBeforeClose, "before_close"},
		{HookAfterClose, "after_close"},
		{HookType(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.ht.String(); got != tt.want {
				t.Errorf("HookType.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

package event

// HookType enumerates the lifecycle hook points in the config framework.
type HookType uint8

const (
	// HookBeforeReload fires before a full config reload.
	HookBeforeReload HookType = iota
	// HookAfterReload fires after a full config reload completes.
	HookAfterReload
	// HookBeforeSet fires before a single key is set.
	HookBeforeSet
	// HookAfterSet fires after a single key is set.
	HookAfterSet
	// HookBeforeDelete fires before a key is deleted.
	HookBeforeDelete
	// HookAfterDelete fires after a key is deleted.
	HookAfterDelete
	// HookBeforeValidate fires before config validation runs.
	HookBeforeValidate
	// HookAfterValidate fires after config validation completes.
	HookAfterValidate
	// HookBeforeClose fires before the config engine is closed.
	HookBeforeClose
	// HookAfterClose fires after the config engine is closed.
	HookAfterClose
)

var hookNames = [...]string{
	HookBeforeReload:   "before_reload",
	HookAfterReload:    "after_reload",
	HookBeforeSet:      "before_set",
	HookAfterSet:       "after_set",
	HookBeforeDelete:   "before_delete",
	HookAfterDelete:    "after_delete",
	HookBeforeValidate: "before_validate",
	HookAfterValidate:  "after_validate",
	HookBeforeClose:    "before_close",
	HookAfterClose:     "after_close",
}

// String returns the human-readable name of the HookType.
func (h HookType) String() string {
	if int(h) < len(hookNames) {
		return hookNames[h]
	}
	return "unknown"
}

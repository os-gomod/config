package event

// HookType identifies the lifecycle point at which a hook is invoked.
// Hooks can be registered for before/after operations such as reload, set,
// delete, validate, and close.
type HookType uint8

const (
	// HookBeforeReload fires before the configuration is reloaded from sources.
	HookBeforeReload HookType = iota
	// HookAfterReload fires after the configuration has been successfully reloaded.
	HookAfterReload
	// HookBeforeSet fires before a single configuration value is set.
	HookBeforeSet
	// HookAfterSet fires after a single configuration value has been set.
	HookAfterSet
	// HookBeforeDelete fires before a configuration key is deleted.
	HookBeforeDelete
	// HookAfterDelete fires after a configuration key has been deleted.
	HookAfterDelete
	// HookBeforeValidate fires before configuration validation runs.
	HookBeforeValidate
	// HookAfterValidate fires after configuration validation completes.
	HookAfterValidate
	// HookBeforeClose fires before the configuration engine is closed.
	HookBeforeClose
	// HookAfterClose fires after the configuration engine has been closed.
	HookAfterClose
)

// hookNames maps HookType constants to their string representations.
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

// String returns the human-readable name of the hook type.
// Returns "unknown" for undefined hook types.
func (h HookType) String() string {
	if int(h) < len(hookNames) {
		return hookNames[h]
	}
	return "unknown"
}

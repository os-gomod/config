package profile

import "time"

// Option configures a Profile during construction.
type Option func(*Profile)

// WithDescription sets the profile description.
func WithDescription(desc string) Option {
	return func(p *Profile) {
		p.Description = desc
	}
}

// WithParent sets the parent profile name.
func WithParent(parent string) Option {
	return func(p *Profile) {
		p.Parent = parent
	}
}

// WithOption sets a custom option key-value pair.
func WithOption(key string, value any) Option {
	return func(p *Profile) {
		if p.Options == nil {
			p.Options = make(map[string]any)
		}
		p.Options[key] = value
	}
}

// WithLayerOpt configures a LayerDef during construction.
type LayerOpt func(*LayerDef)

// LayerEnabled sets the layer as enabled (default).
func LayerEnabled() LayerOpt {
	return func(l *LayerDef) {
		l.Enabled = true
	}
}

// LayerDisabled sets the layer as disabled.
func LayerDisabled() LayerOpt {
	return func(l *LayerDef) {
		l.Enabled = false
	}
}

// LayerPriority sets the layer priority.
func LayerPriority(p int) LayerOpt {
	return func(l *LayerDef) {
		l.Priority = p
	}
}

// LayerFormat sets the content format.
func LayerFormat(format string) LayerOpt {
	return func(l *LayerDef) {
		l.Format = format
	}
}

// LayerOption sets a custom layer option.
func LayerOption(key string, value any) LayerOpt {
	return func(l *LayerDef) {
		if l.Opts == nil {
			l.Opts = make(map[string]any)
		}
		l.Opts[key] = value
	}
}

// Common profile options.
const (
	OptNamespace    = "namespace"
	OptStrictReload = "strict_reload"
	OptMaxWorkers   = "max_workers"
	OptDeltaReload  = "delta_reload"
	OptBatchReload  = "batch_reload"
	OptCacheTTL     = "cache_ttl"
	OptDebounce     = "debounce"
)

// DurationOption sets a time.Duration option.
func DurationOption(key string, d time.Duration) Option {
	return func(p *Profile) {
		if p.Options == nil {
			p.Options = make(map[string]any)
		}
		p.Options[key] = d
	}
}

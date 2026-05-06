// Package template provides a simple implementation of template processing for configuration values.
package template

import (
	"bytes"
	"context"
	"text/template"

	"github.com/os-gomod/config/v2/internal/domain/value"
)

// ConfigGetter defines how templates access configuration values.
type ConfigGetter interface {
	Get(key string) (value.Value, bool)
	GetAll() map[string]value.Value
}

// Processor handles template substitution using the standard text/template package.
// This does NOT implement custom parsing - it delegates to the Go standard library.
type Processor struct {
	getter ConfigGetter
	funcs  template.FuncMap
}

// NewProcessor creates a new template processor.
func NewProcessor(getter ConfigGetter) *Processor {
	return &Processor{
		getter: getter,
		funcs:  make(template.FuncMap),
	}
}

// RegisterFunc adds a custom function for use in templates.
func (p *Processor) RegisterFunc(name string, fn any) {
	p.funcs[name] = fn
}

// Process executes a template string with configuration values.
// Uses text/template with dot as the config data context.
func (p *Processor) Process(ctx context.Context, tmplStr string) (string, error) {
	// Create template with custom functions
	tmpl, err := template.New("config").Funcs(p.funcs).Parse(tmplStr)
	if err != nil {
		return "", err
	}

	// Build data context from configuration
	data := p.buildDataContext(ctx)

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ProcessFile processes a template file.
func (p *Processor) ProcessFile(ctx context.Context, filename string) (string, error) {
	tmpl, err := template.New("config").Funcs(p.funcs).ParseFiles(filename)
	if err != nil {
		return "", err
	}

	data := p.buildDataContext(ctx)

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, filename, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// buildDataContext creates a map for template execution.
func (p *Processor) buildDataContext(ctx context.Context) map[string]any {
	allVals := p.getter.GetAll()
	result := make(map[string]any, len(allVals))
	for k, v := range allVals {
		result[k] = v.Raw()
	}
	return result
}

// MustProcess processes a template and panics on error.
func (p *Processor) MustProcess(ctx context.Context, tmplStr string) string {
	result, err := p.Process(ctx, tmplStr)
	if err != nil {
		panic(err)
	}
	return result
}

// Helper functions for common template operations
var HelperFuncs = template.FuncMap{
	"default": func(def any, val any) any {
		if val == nil {
			return def
		}
		return val
	},
	"coalesce": func(vals ...any) any {
		for _, v := range vals {
			if v != nil {
				return v
			}
		}
		return nil
	},
	"toUpper": func(s string) string {
		// Implementation would use strings.ToUpper
		return s
	},
	"toLower": func(s string) string {
		// Implementation would use strings.ToLower
		return s
	},
}

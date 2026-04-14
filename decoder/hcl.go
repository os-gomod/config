//go:build hcl

// Package decoder — HCL support.
// Build with -tags hcl to include HCL decoding.
// Requires: github.com/hashicorp/hcl/v2
package decoder

import (
	"fmt"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

// HCLDecoder decodes HCL (HashiCorp Configuration Language) bytes into a flat key-value map.
// Available only when compiled with the "hcl" build tag.
// Requires: github.com/hashicorp/hcl/v2
type HCLDecoder struct{}

var _ Decoder = (*HCLDecoder)(nil)

// NewHCLDecoder returns an HCLDecoder.
func NewHCLDecoder() *HCLDecoder { return &HCLDecoder{} }

// Decode parses HCL bytes into a flat, dot-separated key-value map.
func (d *HCLDecoder) Decode(src []byte) (map[string]any, error) {
	var raw map[string]any
	if err := hclsimple.Decode("config.hcl", src, nil, &raw); err != nil {
		return nil, fmt.Errorf("hcl decode: %w", err)
	}
	out := make(map[string]any, len(raw))
	flatten("", raw, out)
	return out, nil
}

// MediaType returns the MIME type for HCL.
func (d *HCLDecoder) MediaType() string { return "application/hcl" }

// Extensions returns the file extensions for HCL.
func (d *HCLDecoder) Extensions() []string { return []string{".hcl"} }

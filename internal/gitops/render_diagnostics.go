package gitops

import (
	"encoding/json"
)

// RenderDiagnostics captures structured diagnostic information about a
// rendering operation. It records which descriptors were evaluated, why
// each was included or excluded, and which files were produced.
type RenderDiagnostics struct {
	Cluster     string               `json:"cluster"`
	Descriptors []DescriptorDecision `json:"descriptors"`
	Actions     []ActionDiagnostic   `json:"actions,omitempty"`
	Undeclared  []string             `json:"undeclared,omitempty"`
}

// DescriptorDecision records the evaluation result for one descriptor.
type DescriptorDecision struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason"`
}

// ActionDiagnostic records one file action produced by the renderer.
type ActionDiagnostic struct {
	Owner    string `json:"owner"`
	Output   string `json:"output"`
	Rendered bool   `json:"rendered"`
}

// JSON returns the diagnostics as indented JSON bytes.
func (d *RenderDiagnostics) JSON() ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}

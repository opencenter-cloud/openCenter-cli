// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package tofuoutput reads OpenTofu outputs (`tofu output -json`) once, so
// every consumer shares the same envelope semantics instead of restating the
// {"value": ...} unwrap and its tolerances.
package tofuoutput

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Outputs is one `tofu output -json` payload: output name → raw value node.
type Outputs map[string]json.RawMessage

// Run executes `tofu output -json` in dir with the given binary (default
// "tofu") and parses the payload. A cluster directory that has never been
// applied is an empty result, not an error: "no state yet" is an ordinary
// observation for every consumer, and each of them would otherwise re-derive
// the same stderr sniffing.
func Run(ctx context.Context, dir, bin string) (Outputs, error) {
	if strings.TrimSpace(bin) == "" {
		bin = "tofu"
	}
	cmd := exec.CommandContext(ctx, bin, "output", "-json")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		if len(output) == 0 {
			if ee, ok := err.(*exec.ExitError); ok && looksLikeMissingState(string(ee.Stderr)) {
				return Outputs{}, nil
			}
		}
		return nil, fmt.Errorf("read OpenTofu outputs in %s: %w", dir, err)
	}
	return Parse(output)
}

// Parse decodes an already-captured `tofu output -json` payload. Callers that
// run the binary through their own command runner (which captures output
// rather than returning it) parse bytes instead of calling Run.
func Parse(data []byte) (Outputs, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse OpenTofu outputs: %w", err)
	}
	return Outputs(raw), nil
}

// String returns the string value of one output. `tofu output -json` wraps
// each output as {"value": ...}; a bare value is accepted too, which keeps the
// reader working with small test runners and older Terraform-compatible
// implementations. A missing output, a null value, or a value that is not a
// string (a contract scalar can only hold strings) reports ok=false.
func (o Outputs) String(name string) (string, bool) {
	value, ok := o[name]
	if !ok || string(value) == "null" {
		return "", false
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(value, &envelope); err == nil {
		if nested, exists := envelope["value"]; exists {
			if string(nested) == "null" {
				return "", false
			}
			value = nested
		}
	}
	var text string
	if err := json.Unmarshal(value, &text); err != nil {
		return "", false
	}
	return text, true
}

// Raw returns the unwrapped value node of one output for callers that need a
// non-string type (a number, a list). The ok result follows the same rules as
// String for missing and null outputs.
func (o Outputs) Raw(name string) (json.RawMessage, bool) {
	value, ok := o[name]
	if !ok || string(value) == "null" {
		return nil, false
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(value, &envelope); err == nil {
		if nested, exists := envelope["value"]; exists {
			if string(nested) == "null" {
				return nil, false
			}
			return nested, true
		}
	}
	return value, true
}

// LooksLikeMissingState reports whether an error or stderr string indicates that
// OpenTofu state was never initialized or applied in the directory.
func LooksLikeMissingState(stderr string) bool {
	lowered := strings.ToLower(stderr)
	return strings.Contains(lowered, "no state file was found") ||
		strings.Contains(lowered, "command \"output\" failed")
}

func looksLikeMissingState(stderr string) bool {
	return LooksLikeMissingState(stderr)
}

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

package tofuoutput

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseAndStringEnvelope(t *testing.T) {
	payload := []byte(`{
		"opencenter_network_id": {"value": "net-1", "type": "string"},
		"opencenter_subnet_id": {"value": null, "type": "string"},
		"list_output": {"value": ["a", "b"], "type": ["list", "string"]},
		"number_output": {"value": 6443, "type": "number"}
	}`)
	outputs, err := Parse(payload)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if value, ok := outputs.String("opencenter_network_id"); !ok || value != "net-1" {
		t.Fatalf("String(network) = %q,%v want net-1,true", value, ok)
	}
	if _, ok := outputs.String("opencenter_subnet_id"); ok {
		t.Fatal("null output must report ok=false")
	}
	if _, ok := outputs.String("list_output"); ok {
		t.Fatal("list output is not a string scalar; must report ok=false")
	}
	if _, ok := outputs.String("missing"); ok {
		t.Fatal("missing output must report ok=false")
	}
	raw, ok := outputs.Raw("number_output")
	if !ok || string(raw) != "6443" {
		t.Fatalf("Raw(number) = %s,%v want 6443,true", raw, ok)
	}
}

// TestStringAcceptsBareValue pins the tolerance for runners and older
// Terraform-compatible implementations that emit the value without the
// {"value": ...} envelope.
func TestStringAcceptsBareValue(t *testing.T) {
	outputs, err := Parse([]byte(`{"opencenter_network_id": "net-bare"}`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if value, ok := outputs.String("opencenter_network_id"); !ok || value != "net-bare" {
		t.Fatalf("String(bare) = %q,%v want net-bare,true", value, ok)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := Parse([]byte("not json")); err == nil {
		t.Fatal("Parse() on garbage must error")
	}
}

// TestRunMissingStateIsEmpty is the contract both consumers depend on: a
// directory that was never applied yields an empty result, not an error.
func TestRunMissingStateIsEmpty(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script stub")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "tofu")
	script := "#!/bin/sh\necho 'No state file was found!' >&2\nexit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	outputs, err := Run(context.Background(), dir, bin)
	if err != nil {
		t.Fatalf("Run() on missing state error = %v", err)
	}
	if len(outputs) != 0 {
		t.Fatalf("missing state must yield empty outputs, got %d", len(outputs))
	}
}

func TestRunFailurePropagates(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script stub")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "tofu")
	script := "#!/bin/sh\necho 'planned destroy failure' >&2\nexit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	if _, err := Run(context.Background(), dir, bin); err == nil {
		t.Fatal("Run() on a real failure must error")
	}
}

func TestRunParsesPayload(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script stub")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "tofu")
	script := "#!/bin/sh\necho '{\"opencenter_network_id\": {\"value\": \"net-run\"}}'\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	outputs, err := Run(context.Background(), dir, bin)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if value, ok := outputs.String("opencenter_network_id"); !ok || value != "net-run" {
		t.Fatalf("String() = %q,%v want net-run,true", value, ok)
	}
}

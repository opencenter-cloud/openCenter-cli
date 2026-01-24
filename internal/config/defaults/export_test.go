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

package defaults

import (
	"strings"
	"testing"
)

// TestExportEffectiveConfig verifies that effective configuration export includes defaults.
func TestExportEffectiveConfig(t *testing.T) {
	cfg := &TestConfig{
		ImageID:             "test-image",
		DefaultStorageClass: "test-storage",
	}

	appliedDefaults := map[string]DefaultSource{
		"ImageID":             SourceProviderRegion,
		"DefaultStorageClass": SourceProviderRegion,
	}

	output, err := ExportEffectiveConfig(cfg, appliedDefaults)
	if err != nil {
		t.Fatalf("ExportEffectiveConfig failed: %v", err)
	}

	// Verify output contains header
	if !strings.Contains(output, "# Effective Configuration") {
		t.Error("Expected output to contain header comment")
	}

	// Verify output contains applied defaults section
	if !strings.Contains(output, "# Applied Defaults:") {
		t.Error("Expected output to contain applied defaults section")
	}

	// Verify output contains the configuration values
	if !strings.Contains(output, "test-image") {
		t.Error("Expected output to contain ImageID value")
	}

	if !strings.Contains(output, "test-storage") {
		t.Error("Expected output to contain DefaultStorageClass value")
	}
}

// TestFormatAppliedDefaults verifies formatting of applied defaults.
func TestFormatAppliedDefaults(t *testing.T) {
	appliedDefaults := map[string]DefaultSource{
		"ImageID":             SourceProviderRegion,
		"DefaultStorageClass": SourceGlobal,
	}

	output := FormatAppliedDefaults(appliedDefaults)

	if !strings.Contains(output, "Applied Defaults:") {
		t.Error("Expected output to contain 'Applied Defaults:' header")
	}

	if !strings.Contains(output, "ImageID") {
		t.Error("Expected output to contain ImageID field")
	}

	if !strings.Contains(output, "provider_region") {
		t.Error("Expected output to contain provider_region source")
	}
}

// TestFormatAppliedDefaults_Empty verifies handling of empty defaults.
func TestFormatAppliedDefaults_Empty(t *testing.T) {
	appliedDefaults := map[string]DefaultSource{}

	output := FormatAppliedDefaults(appliedDefaults)

	if !strings.Contains(output, "No defaults were applied") {
		t.Error("Expected output to indicate no defaults were applied")
	}
}

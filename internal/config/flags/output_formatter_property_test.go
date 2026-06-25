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

package flags

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"gopkg.in/yaml.v3"
)

// TestOutputFormatConsistency tests Property 11: Output format consistency
// Feature: cli-configuration-enhancement, Property 11: Output format consistency
func TestOutputFormatConsistency(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("output format consistency", prop.ForAll(
		func(configData map[string]interface{}) bool {
			// Skip empty configurations to focus on meaningful formatting
			if len(configData) == 0 {
				return true
			}

			// Create a configuration
			config := &Configuration{
				Data: configData,
				Sources: []ConfigSource{
					{Type: SourceCLI, Path: "test", Priority: 1},
				},
				Metadata: ConfigMetadata{
					ProcessedAt: time.Now(),
				},
			}

			formatter := NewDefaultOutputFormatter()

			// Test JSON format
			jsonOutput, err := formatter.FormatConfiguration(config, OutputFormatJSON, OutputModeNormal)
			if err != nil {
				t.Logf("JSON formatting failed: %v", err)
				return false
			}

			// Verify JSON output is valid JSON
			var jsonData interface{}
			if err := json.Unmarshal([]byte(extractConfigFromOutput(jsonOutput)), &jsonData); err != nil {
				t.Logf("JSON output is not valid JSON: %v", err)
				return false
			}

			// Test YAML format
			yamlOutput, err := formatter.FormatConfiguration(config, OutputFormatYAML, OutputModeNormal)
			if err != nil {
				t.Logf("YAML formatting failed: %v", err)
				return false
			}

			// Verify YAML output is valid YAML
			var yamlData interface{}
			if err := yaml.Unmarshal([]byte(extractConfigFromOutput(yamlOutput)), &yamlData); err != nil {
				t.Logf("YAML output is not valid YAML: %v", err)
				return false
			}

			// Test diff format (should be same as YAML for single config)
			diffOutput, err := formatter.FormatConfiguration(config, OutputFormatDiff, OutputModeNormal)
			if err != nil {
				t.Logf("Diff formatting failed: %v", err)
				return false
			}

			// Verify diff output is valid YAML
			var diffData interface{}
			if err := yaml.Unmarshal([]byte(extractConfigFromOutput(diffOutput)), &diffData); err != nil {
				t.Logf("Diff output is not valid YAML: %v", err)
				return false
			}

			return true
		},
		genConfigData(),
	))

	properties.Property("output mode consistency", prop.ForAll(
		func(configData map[string]interface{}) bool {
			// Skip empty configurations
			if len(configData) == 0 {
				return true
			}

			config := &Configuration{
				Data: configData,
				Sources: []ConfigSource{
					{Type: SourceCLI, Path: "test", Priority: 1},
				},
				Metadata: ConfigMetadata{
					ProcessedAt: time.Now(),
				},
			}

			formatter := NewDefaultOutputFormatter()

			// Test all output modes
			modes := []OutputMode{OutputModeNormal, OutputModeDryRun, OutputModeQuiet}

			for _, mode := range modes {
				output, err := formatter.FormatConfiguration(config, OutputFormatYAML, mode)
				if err != nil {
					t.Logf("Formatting failed for mode %s: %v", mode, err)
					return false
				}

				// Verify output is not empty
				if strings.TrimSpace(output) == "" {
					t.Logf("Output is empty for mode %s", mode)
					return false
				}

				// Verify mode-specific characteristics
				switch mode {
				case OutputModeQuiet:
					// Quiet mode should not contain headers
					if strings.Contains(output, "Configuration (processed at") {
						t.Logf("Quiet mode contains headers")
						return false
					}
				case OutputModeDryRun:
					// Dry-run mode should contain dry-run indicator
					if !strings.Contains(output, "DRY RUN") {
						t.Logf("Dry-run mode missing DRY RUN indicator")
						return false
					}
				case OutputModeNormal:
					// Normal mode should contain headers
					if !strings.Contains(output, "Configuration (processed at") {
						t.Logf("Normal mode missing headers")
						return false
					}
				}
			}

			return true
		},
		genConfigData(),
	))

	properties.Property("diff format consistency", prop.ForAll(
		func(originalData, updatedData map[string]interface{}) bool {
			// Skip cases where both configs are empty
			if len(originalData) == 0 && len(updatedData) == 0 {
				return true
			}

			original := &Configuration{
				Data: originalData,
				Sources: []ConfigSource{
					{Type: SourceFile, Path: "original.yaml", Priority: 1},
				},
				Metadata: ConfigMetadata{
					ProcessedAt: time.Now(),
				},
			}

			updated := &Configuration{
				Data: updatedData,
				Sources: []ConfigSource{
					{Type: SourceCLI, Path: "cli", Priority: 2},
				},
				Metadata: ConfigMetadata{
					ProcessedAt: time.Now(),
				},
			}

			formatter := NewDefaultOutputFormatter()

			// Test diff formatting
			diffOutput, err := formatter.FormatDiff(original, updated, OutputModeNormal)
			if err != nil {
				t.Logf("Diff formatting failed: %v", err)
				return false
			}

			// Verify diff output is not empty
			if strings.TrimSpace(diffOutput) == "" {
				t.Logf("Diff output is empty")
				return false
			}

			// If configs are identical, should indicate no changes
			if deepEqual(originalData, updatedData) {
				if !strings.Contains(diffOutput, "No changes detected") {
					t.Logf("Identical configs should show no changes")
					return false
				}
			}

			return true
		},
		genConfigData(),
		genConfigData(),
	))

	properties.Property("conflict formatting consistency", prop.ForAll(
		func(numConflicts int) bool {
			// Generate simple conflicts manually
			conflicts := make([]ConfigConflict, numConflicts)
			for i := 0; i < numConflicts; i++ {
				conflicts[i] = ConfigConflict{
					Path: fmt.Sprintf("path%d", i),
					Sources: []ConflictSource{
						{Type: SourceFile, Path: "file.yaml", Priority: 1, Value: fmt.Sprintf("value%d", i)},
						{Type: SourceCLI, Path: "cli", Priority: 2, Value: fmt.Sprintf("newvalue%d", i)},
					},
					Resolution:    fmt.Sprintf("Resolved using CLI (priority 2)"),
					ResolvedValue: fmt.Sprintf("newvalue%d", i),
				}
			}

			formatter := NewDefaultOutputFormatter()

			// Test conflict formatting in all modes
			modes := []OutputMode{OutputModeNormal, OutputModeDryRun, OutputModeQuiet}

			for _, mode := range modes {
				output, err := formatter.FormatConflicts(conflicts, mode)
				if err != nil {
					t.Logf("Conflict formatting failed for mode %s: %v", mode, err)
					return false
				}

				// Verify output is not empty (even for no conflicts)
				if strings.TrimSpace(output) == "" && mode != OutputModeQuiet {
					t.Logf("Conflict output is empty for mode %s", mode)
					return false
				}

				// If no conflicts, should indicate that
				if len(conflicts) == 0 && mode != OutputModeQuiet {
					if !strings.Contains(output, "No configuration conflicts detected") {
						t.Logf("No conflicts should be indicated for mode %s", mode)
						return false
					}
				}
			}

			return true
		},
		gen.IntRange(0, 3), // Generate 0-3 conflicts
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// genOutputConflicts generates random conflict data for output testing

// extractConfigFromOutput extracts the configuration part from formatted output
func extractConfigFromOutput(output string) string {
	lines := strings.Split(output, "\n")
	configStarted := false
	var configLines []string

	for _, line := range lines {
		if strings.Contains(line, "Configuration:") {
			configStarted = true
			continue
		}

		if configStarted {
			// Stop at empty line or next section
			if strings.TrimSpace(line) == "" && len(configLines) > 0 {
				break
			}
			if strings.HasPrefix(line, "NOTE:") || strings.HasPrefix(line, "Sources:") {
				break
			}
			configLines = append(configLines, line)
		}
	}

	// If no "Configuration:" header found, assume entire output is config
	if !configStarted {
		return output
	}

	return strings.Join(configLines, "\n")
}

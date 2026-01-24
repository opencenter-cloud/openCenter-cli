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
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ExportEffectiveConfig exports the configuration with applied defaults as comments.
// This allows users to see which values came from defaults vs explicit configuration.
func ExportEffectiveConfig(cfg interface{}, appliedDefaults map[string]DefaultSource) (string, error) {
	// Marshal the configuration to YAML
	yamlBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal configuration: %w", err)
	}

	yamlStr := string(yamlBytes)

	// Add header comment explaining the output
	header := `# Effective Configuration
# This configuration includes both explicit values and applied defaults.
# Fields marked with comments indicate the source of their default values.
#
# Default Sources:
#   - explicit: User-provided value
#   - cli_config: From CLI configuration file
#   - provider_region: From provider-region registry
#   - provider: From provider-level defaults
#   - global: From global defaults
#

`

	// Add comments for applied defaults
	// This is a simplified implementation - a full implementation would
	// parse the YAML and insert comments inline
	if len(appliedDefaults) > 0 {
		header += "# Applied Defaults:\n"
		for field, source := range appliedDefaults {
			header += fmt.Sprintf("#   %s: %s\n", field, source)
		}
		header += "\n"
	}

	return header + yamlStr, nil
}

// FormatAppliedDefaults returns a formatted string of applied defaults for display.
func FormatAppliedDefaults(appliedDefaults map[string]DefaultSource) string {
	if len(appliedDefaults) == 0 {
		return "No defaults were applied (all values are explicit)"
	}

	var sb strings.Builder
	sb.WriteString("Applied Defaults:\n")

	for field, source := range appliedDefaults {
		sb.WriteString(fmt.Sprintf("  %s: %s\n", field, source))
	}

	return sb.String()
}

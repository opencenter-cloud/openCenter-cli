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
	"reflect"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// OutputFormat defines the supported output formats
type OutputFormat string

const (
	OutputFormatJSON OutputFormat = "json"
	OutputFormatYAML OutputFormat = "yaml"
	OutputFormatDiff OutputFormat = "diff"
)

// OutputMode defines the output mode
type OutputMode string

const (
	OutputModeNormal OutputMode = "normal"  // Normal output
	OutputModeDryRun OutputMode = "dry-run" // Preview mode without applying changes
	OutputModeQuiet  OutputMode = "quiet"   // Minimal output for scripting
)

// OutputFormatter handles formatting configuration output in different formats
type OutputFormatter interface {
	// FormatConfiguration formats a configuration in the specified format
	FormatConfiguration(config *Configuration, format OutputFormat, mode OutputMode) (string, error)

	// FormatDiff formats a diff between two configurations
	FormatDiff(original, updated *Configuration, mode OutputMode) (string, error)

	// FormatConflicts formats conflict information
	FormatConflicts(conflicts []ConfigConflict, mode OutputMode) (string, error)
}

// DefaultOutputFormatter implements the OutputFormatter interface
type DefaultOutputFormatter struct{}

// NewDefaultOutputFormatter creates a new default output formatter
func NewDefaultOutputFormatter() *DefaultOutputFormatter {
	return &DefaultOutputFormatter{}
}

// FormatConfiguration formats a configuration in the specified format
func (f *DefaultOutputFormatter) FormatConfiguration(config *Configuration, format OutputFormat, mode OutputMode) (string, error) {
	if config == nil {
		return "", fmt.Errorf("configuration cannot be nil")
	}

	switch mode {
	case OutputModeQuiet:
		return f.formatQuiet(config, format)
	case OutputModeDryRun:
		return f.formatDryRun(config, format)
	default:
		return f.formatNormal(config, format)
	}
}

// formatNormal formats configuration in normal mode
func (f *DefaultOutputFormatter) formatNormal(config *Configuration, format OutputFormat) (string, error) {
	var result strings.Builder

	// Add header with metadata
	fmt.Fprintf(&result, "Configuration (processed at %s):\n", config.Metadata.ProcessedAt.Format("2006-01-02 15:04:05"))

	if len(config.Sources) > 0 {
		result.WriteString("Sources:\n")
		for _, source := range config.Sources {
			fmt.Fprintf(&result, "  - %s: %s (priority %d)\n", source.Type, source.Path, source.Priority)
		}
		result.WriteString("\n")
	}

	// Format the configuration data
	configOutput, err := f.formatData(config.Data, format)
	if err != nil {
		return "", fmt.Errorf("failed to format configuration data: %w", err)
	}

	result.WriteString("Configuration:\n")
	result.WriteString(configOutput)

	return result.String(), nil
}

// formatDryRun formats configuration in dry-run mode
func (f *DefaultOutputFormatter) formatDryRun(config *Configuration, format OutputFormat) (string, error) {
	var result strings.Builder

	result.WriteString("DRY RUN - Configuration preview (no changes will be applied):\n\n")

	// Add source information
	if len(config.Sources) > 0 {
		result.WriteString("Configuration sources:\n")
		for _, source := range config.Sources {
			fmt.Fprintf(&result, "  - %s: %s (priority %d)\n", source.Type, source.Path, source.Priority)
		}
		result.WriteString("\n")
	}

	// Format the configuration data
	configOutput, err := f.formatData(config.Data, format)
	if err != nil {
		return "", fmt.Errorf("failed to format configuration data: %w", err)
	}

	result.WriteString("Resulting configuration:\n")
	result.WriteString(configOutput)
	result.WriteString("\nNOTE: This is a preview. Use without --dry-run to apply changes.\n")

	return result.String(), nil
}

// formatQuiet formats configuration in quiet mode
func (f *DefaultOutputFormatter) formatQuiet(config *Configuration, format OutputFormat) (string, error) {
	// In quiet mode, only output the configuration data without headers
	return f.formatData(config.Data, format)
}

// formatData formats configuration data in the specified format
func (f *DefaultOutputFormatter) formatData(data map[string]interface{}, format OutputFormat) (string, error) {
	switch format {
	case OutputFormatJSON:
		return f.formatJSON(data)
	case OutputFormatYAML:
		return f.formatYAML(data)
	case OutputFormatDiff:
		// For single configuration, diff format is the same as YAML
		return f.formatYAML(data)
	default:
		return "", fmt.Errorf("unsupported output format: %s", format)
	}
}

// formatJSON formats data as JSON
func (f *DefaultOutputFormatter) formatJSON(data map[string]interface{}) (string, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(jsonBytes), nil
}

// formatYAML formats data as YAML
func (f *DefaultOutputFormatter) formatYAML(data map[string]interface{}) (string, error) {
	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML: %w", err)
	}
	return string(yamlBytes), nil
}

// FormatDiff formats a diff between two configurations
func (f *DefaultOutputFormatter) FormatDiff(original, updated *Configuration, mode OutputMode) (string, error) {
	if original == nil || updated == nil {
		return "", fmt.Errorf("both original and updated configurations must be provided")
	}

	var result strings.Builder

	if mode != OutputModeQuiet {
		result.WriteString("Configuration Diff:\n")
		result.WriteString("==================\n\n")
	}

	// Generate diff
	diff := f.generateDiff(original.Data, updated.Data, "")

	if len(diff) == 0 {
		if mode != OutputModeQuiet {
			result.WriteString("No changes detected.\n")
		}
		return result.String(), nil
	}

	// Format diff output
	for _, change := range diff {
		switch change.Type {
		case DiffTypeAdded:
			fmt.Fprintf(&result, "+ %s: %v\n", change.Path, change.NewValue)
		case DiffTypeRemoved:
			fmt.Fprintf(&result, "- %s: %v\n", change.Path, change.OldValue)
		case DiffTypeModified:
			fmt.Fprintf(&result, "~ %s: %v -> %v\n", change.Path, change.OldValue, change.NewValue)
		}
	}

	if mode != OutputModeQuiet {
		fmt.Fprintf(&result, "\nSummary: %d changes\n", len(diff))
	}

	return result.String(), nil
}

// DiffType represents the type of change in a diff
type DiffType string

const (
	DiffTypeAdded    DiffType = "added"
	DiffTypeRemoved  DiffType = "removed"
	DiffTypeModified DiffType = "modified"
)

// DiffChange represents a single change in a diff
type DiffChange struct {
	Type     DiffType
	Path     string
	OldValue interface{}
	NewValue interface{}
}

// generateDiff generates a diff between two configuration maps
func (f *DefaultOutputFormatter) generateDiff(original, updated map[string]interface{}, prefix string) []DiffChange {
	var changes []DiffChange

	// Find all keys in both maps
	allKeys := make(map[string]bool)
	for key := range original {
		allKeys[key] = true
	}
	for key := range updated {
		allKeys[key] = true
	}

	// Sort keys for consistent output
	keys := make([]string, 0, len(allKeys))
	for key := range allKeys {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		currentPath := key
		if prefix != "" {
			currentPath = prefix + "." + key
		}

		originalValue, originalExists := original[key]
		updatedValue, updatedExists := updated[key]

		if !originalExists && updatedExists {
			// Added
			changes = append(changes, DiffChange{
				Type:     DiffTypeAdded,
				Path:     currentPath,
				NewValue: updatedValue,
			})
		} else if originalExists && !updatedExists {
			// Removed
			changes = append(changes, DiffChange{
				Type:     DiffTypeRemoved,
				Path:     currentPath,
				OldValue: originalValue,
			})
		} else if originalExists && updatedExists {
			// Check if values are different
			if !reflect.DeepEqual(originalValue, updatedValue) {
				// Check if both are maps (nested objects)
				if originalMap, originalIsMap := originalValue.(map[string]interface{}); originalIsMap {
					if updatedMap, updatedIsMap := updatedValue.(map[string]interface{}); updatedIsMap {
						// Recursively diff nested objects
						nestedChanges := f.generateDiff(originalMap, updatedMap, currentPath)
						changes = append(changes, nestedChanges...)
						continue
					}
				}

				// Modified
				changes = append(changes, DiffChange{
					Type:     DiffTypeModified,
					Path:     currentPath,
					OldValue: originalValue,
					NewValue: updatedValue,
				})
			}
		}
	}

	return changes
}

// FormatConflicts formats conflict information
func (f *DefaultOutputFormatter) FormatConflicts(conflicts []ConfigConflict, mode OutputMode) (string, error) {
	if len(conflicts) == 0 {
		if mode == OutputModeQuiet {
			return "", nil
		}
		return "No configuration conflicts detected.\n", nil
	}

	var result strings.Builder

	if mode != OutputModeQuiet {
		fmt.Fprintf(&result, "Configuration Conflicts (%d):\n", len(conflicts))
		result.WriteString("==============================\n\n")
	}

	for i, conflict := range conflicts {
		if mode != OutputModeQuiet {
			fmt.Fprintf(&result, "%d. Path: %s\n", i+1, conflict.Path)
			result.WriteString("   Sources:\n")

			for _, source := range conflict.Sources {
				fmt.Fprintf(&result, "   - %s '%s' (priority %d): %v\n",
					source.Type, source.Path, source.Priority, source.Value)
			}

			fmt.Fprintf(&result, "   Resolution: %s\n", conflict.Resolution)
			fmt.Fprintf(&result, "   Resolved Value: %v\n\n", conflict.ResolvedValue)
		} else {
			// Quiet mode: just show path and resolved value
			fmt.Fprintf(&result, "%s: %v\n", conflict.Path, conflict.ResolvedValue)
		}
	}

	return result.String(), nil
}

// OutputOptions contains options for output formatting
type OutputOptions struct {
	Format OutputFormat `json:"format"`
	Mode   OutputMode   `json:"mode"`
}

// ParseOutputFormat parses a string into an OutputFormat
func ParseOutputFormat(format string) (OutputFormat, error) {
	switch strings.ToLower(format) {
	case "json":
		return OutputFormatJSON, nil
	case "yaml", "yml":
		return OutputFormatYAML, nil
	case "diff":
		return OutputFormatDiff, nil
	default:
		return "", fmt.Errorf("unsupported output format: %s (supported: json, yaml, diff)", format)
	}
}

// ParseOutputMode parses a string into an OutputMode
func ParseOutputMode(mode string) (OutputMode, error) {
	switch strings.ToLower(mode) {
	case "normal", "":
		return OutputModeNormal, nil
	case "dry-run", "dryrun":
		return OutputModeDryRun, nil
	case "quiet":
		return OutputModeQuiet, nil
	default:
		return "", fmt.Errorf("unsupported output mode: %s (supported: normal, dry-run, quiet)", mode)
	}
}

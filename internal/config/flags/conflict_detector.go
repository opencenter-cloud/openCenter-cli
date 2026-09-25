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
	"fmt"
	"reflect"
	"strings"
)

// ConflictDetector detects and reports configuration conflicts
type ConflictDetector struct {
	conflicts []ConfigConflict
}

// ConfigConflict represents a conflict between configuration sources
type ConfigConflict struct {
	Path          string           `json:"path"`
	Sources       []ConflictSource `json:"sources"`
	Resolution    string           `json:"resolution"`
	ResolvedValue interface{}      `json:"resolved_value"`
}

// ConflictSource represents a source involved in a conflict
type ConflictSource struct {
	Type     SourceType  `json:"type"`
	Path     string      `json:"path"`
	Value    interface{} `json:"value"`
	Priority int         `json:"priority"`
}

// NewConflictDetector creates a new conflict detector
func NewConflictDetector() *ConflictDetector {
	return &ConflictDetector{
		conflicts: []ConfigConflict{},
	}
}

// DetectConflicts detects conflicts between configurations
func (d *ConflictDetector) DetectConflicts(configs []Configuration) ([]ConfigConflict, error) {
	d.conflicts = []ConfigConflict{}

	// Build a map of all paths and their sources
	pathSources := make(map[string][]ConflictSource)

	for _, config := range configs {
		if err := d.extractPathSources(config, "", pathSources); err != nil {
			return nil, fmt.Errorf("failed to extract path sources: %w", err)
		}
	}

	// Identify conflicts (paths with multiple different values)
	for path, sources := range pathSources {
		if len(sources) > 1 {
			conflict := d.analyzeConflict(path, sources)
			if conflict != nil {
				d.conflicts = append(d.conflicts, *conflict)
			}
		}
	}

	return d.conflicts, nil
}

// extractPathSources recursively extracts all paths and their sources from a configuration
func (d *ConflictDetector) extractPathSources(config Configuration, prefix string, pathSources map[string][]ConflictSource) error {
	return d.extractPathSourcesFromData(config.Data, config.Sources, prefix, pathSources)
}

// extractPathSourcesFromData recursively extracts paths from configuration data
func (d *ConflictDetector) extractPathSourcesFromData(data map[string]interface{}, sources []ConfigSource, prefix string, pathSources map[string][]ConflictSource) error {
	for key, value := range data {
		currentPath := key
		if prefix != "" {
			currentPath = prefix + "." + key
		}

		// Only add sources for leaf values (not nested objects)
		if nestedMap, ok := value.(map[string]interface{}); ok {
			// Recursively process nested objects
			if err := d.extractPathSourcesFromData(nestedMap, sources, currentPath, pathSources); err != nil {
				return err
			}
		} else {
			// This is a leaf value, add sources for this path
			for _, source := range sources {
				pathSources[currentPath] = append(pathSources[currentPath], ConflictSource{
					Type:     source.Type,
					Path:     source.Path,
					Value:    value,
					Priority: source.Priority,
				})
			}
		}
	}

	return nil
}

// analyzeConflict analyzes a conflict and determines if it's a real conflict
func (d *ConflictDetector) analyzeConflict(path string, sources []ConflictSource) *ConfigConflict {
	// Check if all values are actually the same (no real conflict)
	if d.allValuesEqual(sources) {
		return nil
	}

	// Find the highest priority source
	highestPriority := -1
	var resolvedValue interface{}
	var resolution string

	for _, source := range sources {
		if source.Priority > highestPriority {
			highestPriority = source.Priority
			resolvedValue = source.Value
			resolution = fmt.Sprintf("Resolved using %s source '%s' (priority %d)", source.Type, source.Path, source.Priority)
		}
	}

	return &ConfigConflict{
		Path:          path,
		Sources:       sources,
		Resolution:    resolution,
		ResolvedValue: resolvedValue,
	}
}

// allValuesEqual checks if all sources have the same value
func (d *ConflictDetector) allValuesEqual(sources []ConflictSource) bool {
	if len(sources) <= 1 {
		return true
	}

	firstValue := sources[0].Value
	for i := 1; i < len(sources); i++ {
		if !reflect.DeepEqual(firstValue, sources[i].Value) {
			return false
		}
	}

	return true
}

// GetConflictReport generates a human-readable conflict report
func (d *ConflictDetector) GetConflictReport() string {
	if len(d.conflicts) == 0 {
		return "No configuration conflicts detected."
	}

	var report strings.Builder
	fmt.Fprintf(&report, "Configuration conflicts detected (%d):\n\n", len(d.conflicts))

	for i, conflict := range d.conflicts {
		fmt.Fprintf(&report, "%d. Path: %s\n", i+1, conflict.Path)
		report.WriteString("   Sources:\n")

		for _, source := range conflict.Sources {
			fmt.Fprintf(&report, "   - %s '%s' (priority %d): %v\n",
				source.Type, source.Path, source.Priority, source.Value)
		}

		fmt.Fprintf(&report, "   Resolution: %s\n", conflict.Resolution)
		fmt.Fprintf(&report, "   Resolved Value: %v\n\n", conflict.ResolvedValue)
	}

	return report.String()
}

// HasConflicts returns true if any conflicts were detected
func (d *ConflictDetector) HasConflicts() bool {
	return len(d.conflicts) > 0
}

// GetConflicts returns all detected conflicts
func (d *ConflictDetector) GetConflicts() []ConfigConflict {
	return d.conflicts
}

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

package config

import (
	"fmt"
	"reflect"
	"strings"
)

// ChangeType represents the type of change detected in a configuration comparison.
type ChangeType string

const (
	// ChangeTypeAdded indicates a field was added in the new configuration.
	ChangeTypeAdded ChangeType = "added"
	// ChangeTypeRemoved indicates a field was removed in the new configuration.
	ChangeTypeRemoved ChangeType = "removed"
	// ChangeTypeModified indicates a field value was changed.
	ChangeTypeModified ChangeType = "modified"
	// ChangeTypeUnchanged indicates a field value remained the same.
	ChangeTypeUnchanged ChangeType = "unchanged"
)

// ConfigChange represents a single change detected between two configurations.
type ConfigChange struct {
	// Path is the dot-separated path to the changed field (e.g., "opencenter.cluster.cluster_name").
	Path string
	// Type indicates the type of change (added, removed, modified, unchanged).
	Type ChangeType
	// OldValue is the value in the old configuration (nil if added).
	OldValue interface{}
	// NewValue is the value in the new configuration (nil if removed).
	NewValue interface{}
	// Description provides a human-readable description of the change.
	Description string
}

// ConfigDiff represents the complete set of differences between two configurations.
type ConfigDiff struct {
	// Changes is the list of all detected changes.
	Changes []ConfigChange
	// HasChanges indicates whether any changes were detected.
	HasChanges bool
	// Summary provides a high-level summary of changes by type.
	Summary DiffSummary
}

// DiffSummary provides statistics about the types of changes detected.
type DiffSummary struct {
	Added    int
	Removed  int
	Modified int
}

// CompareConfigs compares two configurations and returns a detailed diff.
// It performs a deep comparison of all fields and nested structures.
//
// Inputs:
//   - oldConfig: The original configuration.
//   - newConfig: The new configuration to compare against.
//
// Outputs:
//   - *ConfigDiff: A detailed diff containing all detected changes.
func CompareConfigs(oldConfig, newConfig Config) *ConfigDiff {
	diff := &ConfigDiff{
		Changes: []ConfigChange{},
	}

	// Compare using reflection to handle all fields
	compareValues("", reflect.ValueOf(oldConfig), reflect.ValueOf(newConfig), diff)

	// Calculate summary
	for _, change := range diff.Changes {
		switch change.Type {
		case ChangeTypeAdded:
			diff.Summary.Added++
		case ChangeTypeRemoved:
			diff.Summary.Removed++
		case ChangeTypeModified:
			diff.Summary.Modified++
		}
	}

	diff.HasChanges = len(diff.Changes) > 0

	return diff
}

// compareValues recursively compares two reflect.Value instances and records differences.
func compareValues(path string, oldVal, newVal reflect.Value, diff *ConfigDiff) {
	// Handle invalid values
	if !oldVal.IsValid() && !newVal.IsValid() {
		return
	}

	if !oldVal.IsValid() {
		// Field was added
		diff.Changes = append(diff.Changes, ConfigChange{
			Path:        path,
			Type:        ChangeTypeAdded,
			OldValue:    nil,
			NewValue:    getInterfaceValue(newVal),
			Description: fmt.Sprintf("Field '%s' was added", path),
		})
		return
	}

	if !newVal.IsValid() {
		// Field was removed
		diff.Changes = append(diff.Changes, ConfigChange{
			Path:        path,
			Type:        ChangeTypeRemoved,
			OldValue:    getInterfaceValue(oldVal),
			NewValue:    nil,
			Description: fmt.Sprintf("Field '%s' was removed", path),
		})
		return
	}

	// Dereference pointers
	if oldVal.Kind() == reflect.Ptr {
		if oldVal.IsNil() && newVal.IsNil() {
			return
		}
		if oldVal.IsNil() {
			diff.Changes = append(diff.Changes, ConfigChange{
				Path:        path,
				Type:        ChangeTypeAdded,
				OldValue:    nil,
				NewValue:    getInterfaceValue(newVal),
				Description: fmt.Sprintf("Field '%s' was set from nil", path),
			})
			return
		}
		if newVal.IsNil() {
			diff.Changes = append(diff.Changes, ConfigChange{
				Path:        path,
				Type:        ChangeTypeRemoved,
				OldValue:    getInterfaceValue(oldVal),
				NewValue:    nil,
				Description: fmt.Sprintf("Field '%s' was set to nil", path),
			})
			return
		}
		oldVal = oldVal.Elem()
		newVal = newVal.Elem()
	}

	// Special handling for time.Time - compare as values, not as structs
	if oldVal.Type().String() == "time.Time" && newVal.Type().String() == "time.Time" {
		if !reflect.DeepEqual(oldVal.Interface(), newVal.Interface()) {
			diff.Changes = append(diff.Changes, ConfigChange{
				Path:        path,
				Type:        ChangeTypeModified,
				OldValue:    oldVal.Interface(),
				NewValue:    newVal.Interface(),
				Description: fmt.Sprintf("Field '%s' changed from '%v' to '%v'", path, oldVal.Interface(), newVal.Interface()),
			})
		}
		return
	}

	// Compare based on kind
	switch oldVal.Kind() {
	case reflect.Struct:
		compareStructs(path, oldVal, newVal, diff)
	case reflect.Map:
		compareMaps(path, oldVal, newVal, diff)
	case reflect.Slice, reflect.Array:
		compareSlices(path, oldVal, newVal, diff)
	case reflect.Interface:
		// Handle interface values by comparing their concrete values
		if oldVal.IsNil() && newVal.IsNil() {
			return
		}
		if oldVal.IsNil() || newVal.IsNil() {
			if !reflect.DeepEqual(oldVal.Interface(), newVal.Interface()) {
				diff.Changes = append(diff.Changes, ConfigChange{
					Path:        path,
					Type:        ChangeTypeModified,
					OldValue:    getInterfaceValue(oldVal),
					NewValue:    getInterfaceValue(newVal),
					Description: fmt.Sprintf("Field '%s' changed", path),
				})
			}
			return
		}
		compareValues(path, oldVal.Elem(), newVal.Elem(), diff)
	default:
		// Compare primitive values
		if !reflect.DeepEqual(oldVal.Interface(), newVal.Interface()) {
			diff.Changes = append(diff.Changes, ConfigChange{
				Path:        path,
				Type:        ChangeTypeModified,
				OldValue:    oldVal.Interface(),
				NewValue:    newVal.Interface(),
				Description: fmt.Sprintf("Field '%s' changed from '%v' to '%v'", path, oldVal.Interface(), newVal.Interface()),
			})
		}
	}
}

// compareStructs compares two struct values field by field.
func compareStructs(path string, oldVal, newVal reflect.Value, diff *ConfigDiff) {
	typ := oldVal.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		fieldPath := field.Name
		if path != "" {
			fieldPath = path + "." + field.Name
		}

		oldFieldVal := oldVal.Field(i)
		newFieldVal := newVal.Field(i)

		compareValues(fieldPath, oldFieldVal, newFieldVal, diff)
	}
}

// compareMaps compares two map values key by key.
func compareMaps(path string, oldVal, newVal reflect.Value, diff *ConfigDiff) {
	if oldVal.IsNil() && newVal.IsNil() {
		return
	}

	if oldVal.IsNil() {
		diff.Changes = append(diff.Changes, ConfigChange{
			Path:        path,
			Type:        ChangeTypeAdded,
			OldValue:    nil,
			NewValue:    newVal.Interface(),
			Description: fmt.Sprintf("Map '%s' was added", path),
		})
		return
	}

	if newVal.IsNil() {
		diff.Changes = append(diff.Changes, ConfigChange{
			Path:        path,
			Type:        ChangeTypeRemoved,
			OldValue:    oldVal.Interface(),
			NewValue:    nil,
			Description: fmt.Sprintf("Map '%s' was removed", path),
		})
		return
	}

	// Track all keys from both maps
	allKeys := make(map[interface{}]bool)
	for _, key := range oldVal.MapKeys() {
		allKeys[key.Interface()] = true
	}
	for _, key := range newVal.MapKeys() {
		allKeys[key.Interface()] = true
	}

	// Compare each key
	for key := range allKeys {
		keyVal := reflect.ValueOf(key)
		keyPath := fmt.Sprintf("%s[%v]", path, key)

		oldMapVal := oldVal.MapIndex(keyVal)
		newMapVal := newVal.MapIndex(keyVal)

		compareValues(keyPath, oldMapVal, newMapVal, diff)
	}
}

// compareSlices compares two slice values element by element.
func compareSlices(path string, oldVal, newVal reflect.Value, diff *ConfigDiff) {
	if oldVal.IsNil() && newVal.IsNil() {
		return
	}

	if oldVal.IsNil() {
		diff.Changes = append(diff.Changes, ConfigChange{
			Path:        path,
			Type:        ChangeTypeAdded,
			OldValue:    nil,
			NewValue:    getInterfaceValue(newVal),
			Description: fmt.Sprintf("Slice '%s' was added", path),
		})
		return
	}

	if newVal.IsNil() {
		diff.Changes = append(diff.Changes, ConfigChange{
			Path:        path,
			Type:        ChangeTypeRemoved,
			OldValue:    getInterfaceValue(oldVal),
			NewValue:    nil,
			Description: fmt.Sprintf("Slice '%s' was removed", path),
		})
		return
	}

	oldLen := oldVal.Len()
	newLen := newVal.Len()

	// Compare length change
	if oldLen != newLen {
		diff.Changes = append(diff.Changes, ConfigChange{
			Path:        path + ".length",
			Type:        ChangeTypeModified,
			OldValue:    oldLen,
			NewValue:    newLen,
			Description: fmt.Sprintf("Slice '%s' length changed from %d to %d", path, oldLen, newLen),
		})
	}

	// Compare elements up to the shorter length
	minLen := oldLen
	if newLen < minLen {
		minLen = newLen
	}

	for i := 0; i < minLen; i++ {
		elemPath := fmt.Sprintf("%s[%d]", path, i)
		compareValues(elemPath, oldVal.Index(i), newVal.Index(i), diff)
	}

	// Handle extra elements in old slice (removed)
	for i := minLen; i < oldLen; i++ {
		elemPath := fmt.Sprintf("%s[%d]", path, i)
		diff.Changes = append(diff.Changes, ConfigChange{
			Path:        elemPath,
			Type:        ChangeTypeRemoved,
			OldValue:    oldVal.Index(i).Interface(),
			NewValue:    nil,
			Description: fmt.Sprintf("Element at '%s' was removed", elemPath),
		})
	}

	// Handle extra elements in new slice (added)
	for i := minLen; i < newLen; i++ {
		elemPath := fmt.Sprintf("%s[%d]", path, i)
		diff.Changes = append(diff.Changes, ConfigChange{
			Path:        elemPath,
			Type:        ChangeTypeAdded,
			OldValue:    nil,
			NewValue:    newVal.Index(i).Interface(),
			Description: fmt.Sprintf("Element at '%s' was added", elemPath),
		})
	}
}

// getInterfaceValue safely extracts the interface value from a reflect.Value.
func getInterfaceValue(val reflect.Value) interface{} {
	if !val.IsValid() {
		return nil
	}
	if val.Kind() == reflect.Ptr && val.IsNil() {
		return nil
	}
	return val.Interface()
}

// FormatDiff returns a human-readable string representation of the diff.
func (d *ConfigDiff) FormatDiff() string {
	if !d.HasChanges {
		return "No changes detected"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Configuration Diff Summary:\n"))
	sb.WriteString(fmt.Sprintf("  Added:    %d\n", d.Summary.Added))
	sb.WriteString(fmt.Sprintf("  Removed:  %d\n", d.Summary.Removed))
	sb.WriteString(fmt.Sprintf("  Modified: %d\n", d.Summary.Modified))
	sb.WriteString(fmt.Sprintf("\nDetailed Changes:\n"))

	for _, change := range d.Changes {
		sb.WriteString(fmt.Sprintf("  [%s] %s\n", change.Type, change.Description))
	}

	return sb.String()
}

// FilterChangesByType returns only changes of the specified type.
func (d *ConfigDiff) FilterChangesByType(changeType ChangeType) []ConfigChange {
	var filtered []ConfigChange
	for _, change := range d.Changes {
		if change.Type == changeType {
			filtered = append(filtered, change)
		}
	}
	return filtered
}

// FilterChangesByPath returns only changes matching the specified path prefix.
func (d *ConfigDiff) FilterChangesByPath(pathPrefix string) []ConfigChange {
	var filtered []ConfigChange
	for _, change := range d.Changes {
		if strings.HasPrefix(change.Path, pathPrefix) {
			filtered = append(filtered, change)
		}
	}
	return filtered
}

// HasChangesInPath checks if there are any changes in the specified path.
func (d *ConfigDiff) HasChangesInPath(pathPrefix string) bool {
	for _, change := range d.Changes {
		if strings.HasPrefix(change.Path, pathPrefix) {
			return true
		}
	}
	return false
}

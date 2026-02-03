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

package validators

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
)

// ConfigStructureValidator validates that configuration uses v2 field structure only.
// It rejects v1 field locations and provides migration guidance.
//
// Validates: Requirements 1.4
type ConfigStructureValidator struct{}

// NewConfigStructureValidator creates a new configuration structure validator.
func NewConfigStructureValidator() *ConfigStructureValidator {
	return &ConfigStructureValidator{}
}

// Name returns the validator name.
func (v *ConfigStructureValidator) Name() string {
	return "config-structure"
}

// Priority returns the validator priority.
// Config structure validation is fast (format check), so it has high priority.
func (v *ConfigStructureValidator) Priority() int {
	return validation.PriorityHigh
}

// Validate validates that the configuration uses v2 field structure.
// It checks for v1 field locations and provides migration examples.
func (v *ConfigStructureValidator) Validate(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
	result := &validation.ValidationResult{
		Valid:    true,
		Errors:   []*validation.ValidationIssue{},
		Warnings: []*validation.ValidationIssue{},
		Info:     []*validation.ValidationIssue{},
	}

	// Check if value is a map (YAML unmarshaled config)
	configMap, ok := value.(map[string]interface{})
	if !ok {
		// Try to convert struct to map using reflection
		configMap = structToMap(value)
		if configMap == nil {
			result.AddError("config-structure", "value must be a configuration map or struct")
			return result, nil
		}
	}

	// Check for v1 field locations
	v.checkV1FieldLocations(result, configMap)

	return result, nil
}

// checkV1FieldLocations checks for v1 field locations in the configuration.
func (v *ConfigStructureValidator) checkV1FieldLocations(result *validation.ValidationResult, configMap map[string]interface{}) {
	// Check for opencenter.cluster.networking (v1 location)
	if opencenter, ok := configMap["opencenter"].(map[string]interface{}); ok {
		if cluster, ok := opencenter["cluster"].(map[string]interface{}); ok {
			// Check for v1 networking location
			if networking, ok := cluster["networking"].(map[string]interface{}); ok {
				if vrrpIP, exists := networking["vrrp_ip"]; exists && vrrpIP != nil {
					result.AddError("config-structure",
						"Configuration uses v1 field location: opencenter.cluster.networking.vrrp_ip",
						"v2.0.0 requires v2 field structure",
						"Migration: opencenter.cluster.networking.vrrp_ip → opencenter.infrastructure.networking.vrrp_ip",
						"To upgrade: Install opencenter v1.x and run 'opencenter cluster migrate-config'")
				}
			}

			// Check for v1 kubernetes flavor fields
			if kubernetes, ok := cluster["kubernetes"].(map[string]interface{}); ok {
				v1FlavorFields := []string{
					"flavor_control_plane",
					"flavor_worker",
					"flavor_etcd",
				}

				for _, field := range v1FlavorFields {
					if value, exists := kubernetes[field]; exists && value != nil {
						result.AddError("config-structure",
							fmt.Sprintf("Configuration uses v1 field location: opencenter.cluster.kubernetes.%s", field),
							"v2.0.0 requires v2 field structure",
							fmt.Sprintf("Migration: opencenter.cluster.kubernetes.%s → opencenter.infrastructure.compute.%s", field, field),
							"To upgrade: Install opencenter v1.x and run 'opencenter cluster migrate-config'")
					}
				}
			}
		}

		// Check for v1 storage location (top-level under opencenter)
		if storage, ok := opencenter["storage"].(map[string]interface{}); ok {
			// Check if storage has any non-empty values
			if hasNonEmptyValues(storage) {
				result.AddError("config-structure",
					"Configuration uses v1 field location: opencenter.storage",
					"v2.0.0 requires v2 field structure",
					"Migration: opencenter.storage → opencenter.infrastructure.storage",
					"To upgrade: Install opencenter v1.x and run 'opencenter cluster migrate-config'")
			}
		}
	}

	// Check for top-level storage field (another v1 pattern)
	if storage, ok := configMap["storage"].(map[string]interface{}); ok {
		if hasNonEmptyValues(storage) {
			result.AddError("config-structure",
				"Configuration uses v1 field location: storage (top-level)",
				"v2.0.0 requires v2 field structure",
				"Migration: storage → opencenter.infrastructure.storage",
				"To upgrade: Install opencenter v1.x and run 'opencenter cluster migrate-config'")
		}
	}
}

// hasNonEmptyValues checks if a map has any non-empty values.
func hasNonEmptyValues(m map[string]interface{}) bool {
	for _, v := range m {
		if v != nil {
			// Check if value is not an empty string, empty map, or empty slice
			switch val := v.(type) {
			case string:
				if val != "" {
					return true
				}
			case map[string]interface{}:
				if len(val) > 0 && hasNonEmptyValues(val) {
					return true
				}
			case []interface{}:
				if len(val) > 0 {
					return true
				}
			default:
				return true
			}
		}
	}
	return false
}

// structToMap converts a struct to a map using reflection.
func structToMap(value interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Get YAML tag name
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}

		// Parse YAML tag (handle "name,omitempty" format)
		tagParts := strings.Split(yamlTag, ",")
		fieldName := tagParts[0]

		// Skip if field is zero value and has omitempty
		if len(tagParts) > 1 && contains(tagParts[1:], "omitempty") {
			if fieldValue.IsZero() {
				continue
			}
		}

		// Convert field value to interface
		if fieldValue.CanInterface() {
			result[fieldName] = fieldValue.Interface()
		}
	}

	return result
}

// contains checks if a slice contains a string.
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

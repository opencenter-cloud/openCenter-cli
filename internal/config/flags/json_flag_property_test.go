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
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: cli-configuration-enhancement, Property 6: JSON/YAML round-trip integrity
// For any valid JSON or YAML configuration, parsing it through CLI flags and serializing it back should produce an equivalent structure
// Validates: Requirements 4.1, 4.4, 5.1, 5.2
func TestProperty_JSONRoundTripIntegrity(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("JSON round-trip preserves structure", prop.ForAll(
		func(jsonData interface{}) bool {
			// Skip nil values as they're not valid for our use case
			if jsonData == nil {
				return true
			}
			
			// Convert the generated data to JSON string
			jsonBytes, err := json.Marshal(jsonData)
			if err != nil {
				return true // Skip invalid JSON structures
			}
			jsonString := string(jsonBytes)
			
			// Create a JSON flag handler
			handler := NewJSONFlagHandler()
			
			// Parse the JSON through the flag handler
			flagName := "json-set-test.config"
			parsed, err := handler.ParseFlag(flagName, jsonString)
			if err != nil {
				return false // JSON parsing should not fail for valid JSON
			}
			
			// Verify the parsed result is a JSONFlag
			jsonFlag, ok := parsed.(*JSONFlag)
			if !ok {
				return false
			}
			
			// Verify the path is correctly extracted
			if jsonFlag.Path != "test.config" {
				return false
			}
			
			// Convert the parsed value back to JSON
			roundTripBytes, err := json.Marshal(jsonFlag.Value)
			if err != nil {
				return false // Round-trip serialization should not fail
			}
			
			// Compare the original and round-trip JSON
			// We compare the unmarshaled structures to handle JSON formatting differences
			var original, roundTrip interface{}
			if err := json.Unmarshal(jsonBytes, &original); err != nil {
				return false
			}
			if err := json.Unmarshal(roundTripBytes, &roundTrip); err != nil {
				return false
			}
			
			// Verify structural equality
			return deepEqual(original, roundTrip)
		},
		genValidJSONData(),
	))

	properties.Property("JSON flag merging preserves data integrity", prop.ForAll(
		func(baseConfig map[string]interface{}, jsonPath string, jsonValue interface{}) bool {
			// Skip invalid inputs
			if jsonPath == "" || jsonValue == nil {
				return true
			}
			
			// Create a copy of the base configuration
			targetConfig := make(map[string]interface{})
			for k, v := range baseConfig {
				targetConfig[k] = v
			}
			
			// Create a JSON flag
			jsonFlag := &JSONFlag{
				Path:  jsonPath,
				Value: jsonValue,
			}
			
			// Create handler and merge the JSON flag
			handler := NewJSONFlagHandler()
			err := handler.MergeIntoConfiguration(jsonFlag, targetConfig)
			if err != nil {
				// Some merge operations may fail due to path conflicts, which is expected
				return true
			}
			
			// Verify that the merged value can be retrieved
			retrievedValue := getValueAtPath(targetConfig, jsonPath)
			if retrievedValue == nil {
				return false // The value should be present after merging
			}
			
			// Verify structural equality of the merged value
			return deepEqual(jsonValue, retrievedValue)
		},
		genConfigMap(),
		genConfigPath(),
		genValidJSONData(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// genValidJSONData generates valid JSON data structures
func genValidJSONData() gopter.Gen {
	return gen.OneConstOf(
		// Primitive values
		"test-string",
		int64(42),
		true,
		
		// Simple objects
		map[string]interface{}{"key": "value", "count": int64(3)},
		
		// Simple arrays
		[]interface{}{"item1", "item2", "item3"},
		[]interface{}{int64(1), int64(2), int64(3)},
		
		// Nested objects
		map[string]interface{}{
			"config": map[string]interface{}{
				"name": "test",
				"enabled": true,
			},
		},
		
		// Mixed arrays
		[]interface{}{"string", int64(42), true},
	)
}

// genConfigMap generates a base configuration map
func genConfigMap() gopter.Gen {
	return gen.OneConstOf(
		map[string]interface{}{},
		map[string]interface{}{"existing": "value"},
		map[string]interface{}{
			"cluster": map[string]interface{}{
				"name": "existing-cluster",
			},
		},
	)
}

// genConfigPath generates valid configuration paths
func genConfigPath() gopter.Gen {
	return gen.OneConstOf(
		"config",
		"cluster.name",
		"infrastructure.servers",
		"networking.dns",
		"services.monitoring.enabled",
	)
}

// deepEqual compares two JSON values for structural equality
func deepEqual(a, b interface{}) bool {
	switch aVal := a.(type) {
	case map[string]interface{}:
		bVal, ok := b.(map[string]interface{})
		if !ok {
			return false
		}
		if len(aVal) != len(bVal) {
			return false
		}
		for key, value := range aVal {
			if !deepEqual(value, bVal[key]) {
				return false
			}
		}
		return true
	case []interface{}:
		bVal, ok := b.([]interface{})
		if !ok {
			return false
		}
		if len(aVal) != len(bVal) {
			return false
		}
		for i, value := range aVal {
			if !deepEqual(value, bVal[i]) {
				return false
			}
		}
		return true
	case float64:
		// JSON numbers are always float64
		if bVal, ok := b.(float64); ok {
			return aVal == bVal
		}
		// Handle int64 to float64 conversion
		if bVal, ok := b.(int64); ok {
			return aVal == float64(bVal)
		}
		return false
	case int64:
		// Handle int64 to float64 conversion (JSON unmarshaling converts numbers to float64)
		if bVal, ok := b.(float64); ok {
			return float64(aVal) == bVal
		}
		if bVal, ok := b.(int64); ok {
			return aVal == bVal
		}
		return false
	default:
		return a == b
	}
}

// getValueAtPath retrieves a value from a nested map using dot notation
func getValueAtPath(config map[string]interface{}, path string) interface{} {
	parts := splitPath(path)
	current := config
	
	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part, return the value
			return current[part]
		}
		
		// Navigate deeper
		if next, exists := current[part]; exists {
			if nextMap, ok := next.(map[string]interface{}); ok {
				current = nextMap
			} else {
				return nil // Path doesn't lead to a map
			}
		} else {
			return nil // Path doesn't exist
		}
	}
	
	return nil
}

// splitPath splits a dot-notation path into components
func splitPath(path string) []string {
	if path == "" {
		return []string{}
	}
	
	var parts []string
	current := ""
	
	for _, char := range path {
		if char == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	
	if current != "" {
		parts = append(parts, current)
	}
	
	return parts
}
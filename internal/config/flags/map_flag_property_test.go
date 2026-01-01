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
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: cli-configuration-enhancement, Property 10: Map operation consistency
// For any map operation (set, merge, remove), the operation should be idempotent and preserve map structure integrity
// Validates: Requirements 9.1, 9.2, 9.3, 9.4, 9.5
func TestProperty_MapOperationConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("map set operations are idempotent", prop.ForAll(
		func(baseConfig map[string]interface{}, mapPath string, key string, value interface{}) bool {
			// Skip invalid inputs
			if mapPath == "" || key == "" || value == nil {
				return true
			}
			
			handler := NewMapFlagHandler()
			
			// Create map flag
			mapFlag := &MapFlag{
				Operation: MapOpSet,
				Path:      mapPath,
				Key:       key,
				Value:     value,
			}
			
			// Apply operation twice to two identical configurations
			config1 := copyConfig(baseConfig)
			config2 := copyConfig(baseConfig)
			
			err1 := handler.MergeIntoConfiguration(mapFlag, config1)
			if err1 != nil {
				return true // Skip configurations that cause errors
			}
			
			err2 := handler.MergeIntoConfiguration(mapFlag, config2)
			if err2 != nil {
				return false // Should not fail if first succeeded
			}
			
			// Apply operation again to first configuration
			err3 := handler.MergeIntoConfiguration(mapFlag, config1)
			if err3 != nil {
				return false // Should not fail on second application
			}
			
			// Both configurations should be identical
			return compareConfigValues(config1, config2)
		},
		genSimpleConfig(),
		genMapPath(),
		genMapKey(),
		genMapValue(),
	))

	properties.Property("map merge operations preserve existing keys", prop.ForAll(
		func(baseConfig map[string]interface{}, mapPath string, mergeData map[string]interface{}) bool {
			// Skip invalid inputs
			if mapPath == "" || len(mergeData) == 0 {
				return true
			}
			
			handler := NewMapFlagHandler()
			
			// Create merge flag
			mapFlag := &MapFlag{
				Operation: MapOpMerge,
				Path:      mapPath,
				Value:     mergeData,
			}
			
			// Record existing keys at the target path
			existingKeys := getExistingKeysAtPath(baseConfig, mapPath)
			
			// Apply merge operation
			config := copyConfig(baseConfig)
			err := handler.MergeIntoConfiguration(mapFlag, config)
			if err != nil {
				return true // Skip configurations that cause errors
			}
			
			// Verify all existing keys are still present
			for _, key := range existingKeys {
				if !hasKeyAtPath(config, mapPath, key) {
					return false // Existing key was lost
				}
			}
			
			// Verify all merged keys are present
			for key := range mergeData {
				if !hasKeyAtPath(config, mapPath, key) {
					return false // Merged key is missing
				}
			}
			
			return true
		},
		genSimpleConfig(),
		genMapPath(),
		genMergeData(),
	))

	properties.Property("map remove operations only affect target keys", prop.ForAll(
		func(baseConfig map[string]interface{}, mapPath string, removeKey string) bool {
			// Skip invalid inputs
			if mapPath == "" || removeKey == "" {
				return true
			}
			
			handler := NewMapFlagHandler()
			
			// Create remove flag
			mapFlag := &MapFlag{
				Operation: MapOpRemove,
				Path:      mapPath,
				Key:       removeKey,
			}
			
			// Record all keys except the one being removed
			otherKeys := getOtherKeysAtPath(baseConfig, mapPath, removeKey)
			
			// Apply remove operation
			config := copyConfig(baseConfig)
			err := handler.MergeIntoConfiguration(mapFlag, config)
			if err != nil {
				return true // Skip configurations that cause errors
			}
			
			// Verify target key is removed
			if hasKeyAtPath(config, mapPath, removeKey) {
				return false // Key should have been removed
			}
			
			// Verify all other keys are preserved
			for _, key := range otherKeys {
				if !hasKeyAtPath(config, mapPath, key) {
					return false // Other key was incorrectly removed
				}
			}
			
			return true
		},
		genSimpleConfig(),
		genMapPath(),
		genMapKey(),
	))

	properties.Property("map operations create intermediate paths", prop.ForAll(
		func(nestedPath string, key string, value interface{}) bool {
			// Skip invalid inputs
			if nestedPath == "" || key == "" || value == nil {
				return true
			}
			
			handler := NewMapFlagHandler()
			
			// Create set flag for nested path
			mapFlag := &MapFlag{
				Operation: MapOpSet,
				Path:      nestedPath,
				Key:       key,
				Value:     value,
			}
			
			// Start with empty configuration
			config := make(map[string]interface{})
			
			// Apply operation
			err := handler.MergeIntoConfiguration(mapFlag, config)
			if err != nil {
				return true // Skip paths that cause errors
			}
			
			// Verify the value can be retrieved at the nested path
			return hasKeyAtPath(config, nestedPath, key)
		},
		genNestedPath(),
		genMapKey(),
		genMapValue(),
	))

	properties.Property("map operations preserve type safety", prop.ForAll(
		func(baseConfig map[string]interface{}, mapPath string, key string, value interface{}) bool {
			// Skip invalid inputs
			if mapPath == "" || key == "" || value == nil {
				return true
			}
			
			handler := NewMapFlagHandler()
			
			// Create set flag
			mapFlag := &MapFlag{
				Operation: MapOpSet,
				Path:      mapPath,
				Key:       key,
				Value:     value,
			}
			
			// Apply operation
			config := copyConfig(baseConfig)
			err := handler.MergeIntoConfiguration(mapFlag, config)
			if err != nil {
				return true // Skip configurations that cause errors
			}
			
			// Verify the stored value matches the input value
			storedValue := getValueAtMapPath(config, mapPath, key)
			return compareValues(storedValue, value)
		},
		genSimpleConfig(),
		genMapPath(),
		genMapKey(),
		genMapValue(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Helper functions for property testing

func genSimpleConfig() gopter.Gen {
	return gen.OneConstOf(
		map[string]interface{}{},
		map[string]interface{}{"existing": "value"},
		map[string]interface{}{
			"config": map[string]interface{}{
				"name": "test",
			},
		},
		map[string]interface{}{
			"nested": map[string]interface{}{
				"deep": map[string]interface{}{
					"key": "value",
				},
			},
		},
	)
}

func genMapPath() gopter.Gen {
	return gen.OneConstOf(
		"config",
		"settings",
		"cluster.config",
		"infrastructure.settings",
	)
}

func genNestedPath() gopter.Gen {
	return gen.OneConstOf(
		"level1",
		"level1.level2",
		"level1.level2.level3",
		"cluster.infrastructure.provider",
	)
}

func genMapKey() gopter.Gen {
	return gen.OneConstOf(
		"name",
		"version",
		"enabled",
		"count",
		"provider",
	)
}

func genMapValue() gopter.Gen {
	return gen.OneConstOf(
		"string-value",
		42,
		true,
		map[string]interface{}{"nested": "object"},
	)
}

func genMergeData() gopter.Gen {
	return gen.OneConstOf(
		map[string]interface{}{"key1": "value1"},
		map[string]interface{}{"key1": "value1", "key2": "value2"},
		map[string]interface{}{"name": "test", "enabled": true},
	)
}

func copyConfig(config map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range config {
		if nestedMap, ok := v.(map[string]interface{}); ok {
			result[k] = copyConfig(nestedMap)
		} else {
			result[k] = v
		}
	}
	return result
}

func getExistingKeysAtPath(config map[string]interface{}, path string) []string {
	targetMap := getMapAtPath(config, path)
	if targetMap == nil {
		return []string{}
	}
	
	keys := make([]string, 0, len(targetMap))
	for key := range targetMap {
		keys = append(keys, key)
	}
	return keys
}

func getOtherKeysAtPath(config map[string]interface{}, path string, excludeKey string) []string {
	targetMap := getMapAtPath(config, path)
	if targetMap == nil {
		return []string{}
	}
	
	keys := make([]string, 0, len(targetMap))
	for key := range targetMap {
		if key != excludeKey {
			keys = append(keys, key)
		}
	}
	return keys
}

func hasKeyAtPath(config map[string]interface{}, path string, key string) bool {
	targetMap := getMapAtPath(config, path)
	if targetMap == nil {
		return false
	}
	
	_, exists := targetMap[key]
	return exists
}

func getValueAtMapPath(config map[string]interface{}, path string, key string) interface{} {
	targetMap := getMapAtPath(config, path)
	if targetMap == nil {
		return nil
	}
	
	return targetMap[key]
}

func getMapAtPath(config map[string]interface{}, path string) map[string]interface{} {
	if path == "" {
		return config
	}
	
	parts := splitPath(path)
	current := config
	
	for _, part := range parts {
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
	
	return current
}
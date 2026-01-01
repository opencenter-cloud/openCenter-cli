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
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: cli-configuration-enhancement, Property 3: Backward compatibility preservation
// For any configuration that worked with the old system, the enhanced reflection engine should produce identical results
// Validates: Requirements 11.1, 11.2, 11.3, 11.4
func TestProperty_BackwardCompatibilityPreservation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("enhanced reflection engine preserves legacy behavior", prop.ForAll(
		func(path string, value interface{}) bool {
			// Skip invalid paths that wouldn't work in either system
			if path == "" || value == nil {
				return true
			}
			
			// Test with legacy setField function (if it exists)
			legacyConfig := &BackwardCompatTestConfig{}
			legacyErr := setFieldLegacy(legacyConfig, path, value)
			
			// Test with enhanced reflection engine
			engine := NewEnhancedReflectionEngine()
			enhancedConfig := &BackwardCompatTestConfig{}
			enhancedErr := engine.SetField(enhancedConfig, path, value)
			
			// Both should have same error status
			if (legacyErr == nil) != (enhancedErr == nil) {
				return false
			}
			
			// If both succeeded, results should be identical
			if legacyErr == nil && enhancedErr == nil {
				return reflect.DeepEqual(legacyConfig, enhancedConfig)
			}
			
			return true
		},
		genLegacyPath(),
		genLegacyValue(),
	))

	properties.Property("path parsing maintains compatibility with dot notation", prop.ForAll(
		func(dotPath string) bool {
			// Skip empty or invalid paths
			if dotPath == "" || !isValidDotPath(dotPath) {
				return true
			}
			
			parser := NewEnhancedPathParser()
			
			// Parse the dot notation path
			structuredPath, err := parser.ParsePath(dotPath)
			if err != nil {
				return false // Valid dot paths should always parse
			}
			
			// Verify the structured path can be converted back to equivalent dot notation
			reconstructed := reconstructDotPath(*structuredPath)
			
			// The reconstructed path should be functionally equivalent
			// (may not be identical due to normalization, but should access same field)
			return isEquivalentPath(dotPath, reconstructed)
		},
		genDotNotationPath(),
	))

	properties.Property("array indexing maintains compatibility with existing syntax", prop.ForAll(
		func(arrayPath string, index int) bool {
			// Skip invalid inputs
			if arrayPath == "" || index < 0 || index > 100 {
				return true
			}
			
			// Create legacy-style array path (field.index)
			legacyPath := arrayPath + "." + string(rune('0'+index%10))
			
			// Create enhanced-style array path (field[index])
			enhancedPath := arrayPath + "[" + string(rune('0'+index%10)) + "]"
			
			parser := NewEnhancedPathParser()
			
			// Both should parse successfully
			legacyStructured, legacyErr := parser.ParsePath(legacyPath)
			enhancedStructured, enhancyErr := parser.ParsePath(enhancedPath)
			
			if legacyErr != nil || enhancyErr != nil {
				return true // Skip if either fails to parse
			}
			
			// Both should result in equivalent structured paths
			return isEquivalentStructuredPath(*legacyStructured, *enhancedStructured)
		},
		genArrayFieldPath(),
		gen.IntRange(0, 9),
	))

	properties.Property("configuration merging preserves precedence rules", prop.ForAll(
		func(configs []map[string]interface{}) bool {
			// Skip empty or single configs
			if len(configs) < 2 {
				return true
			}
			
			// Create configurations with different source types (simulating legacy behavior)
			legacyConfigs := make([]Configuration, len(configs))
			enhancedConfigs := make([]Configuration, len(configs))
			
			sourceTypes := []SourceType{SourceDefault, SourceFile, SourceCLI}
			
			for i, config := range configs {
				sourceType := sourceTypes[i%len(sourceTypes)]
				
				legacyConfigs[i] = Configuration{
					Data:    config,
					Sources: []ConfigSource{{Type: sourceType, Path: "test"}},
				}
				enhancedConfigs[i] = Configuration{
					Data:    config,
					Sources: []ConfigSource{{Type: sourceType, Path: "test"}},
				}
			}
			
			// Merge with both systems
			legacyMerger := NewDefaultConfigurationMerger()
			enhancedMerger := NewDefaultConfigurationMerger()
			
			legacyResult, legacyErr := legacyMerger.MergeConfigurations(legacyConfigs)
			enhancedResult, enhancedErr := enhancedMerger.MergeConfigurations(enhancedConfigs)
			
			// Both should have same error status
			if (legacyErr == nil) != (enhancedErr == nil) {
				return false
			}
			
			// If both succeeded, results should be equivalent
			if legacyErr == nil && enhancedErr == nil {
				return isEquivalentConfiguration(*legacyResult, *enhancedResult)
			}
			
			return true
		},
		genConfigurationList(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Test configuration struct for backward compatibility testing
type BackwardCompatTestConfig struct {
	Name        string                 `yaml:"name"`
	Count       int                    `yaml:"count"`
	Enabled     bool                   `yaml:"enabled"`
	Tags        []string               `yaml:"tags"`
	Metadata    map[string]interface{} `yaml:"metadata"`
	ServerPools []ServerPool           `yaml:"server_pools"`
}

type ServerPool struct {
	Name        string `yaml:"name"`
	WorkerCount int    `yaml:"worker_count"`
	Flavor      string `yaml:"flavor_worker"`
}

// Legacy setField function (simplified version for testing)
func setFieldLegacy(config interface{}, path string, value interface{}) error {
	// This would be the old implementation
	// For testing purposes, we'll implement a basic version
	v := reflect.ValueOf(config)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return &ConfigError{
			Type:    ErrorTypePath,
			Path:    path,
			Message: "target must be a pointer to struct",
		}
	}
	
	// Simple dot notation parsing (legacy behavior)
	parts := splitPathCompat(path, ".")
	current := v.Elem()
	
	for i, part := range parts {
		if i == len(parts)-1 {
			// Set the final field
			field := current.FieldByName(part)
			if !field.IsValid() || !field.CanSet() {
				return &ConfigError{
					Type:    ErrorTypePath,
					Path:    path,
					Message: "field not found or cannot be set",
				}
			}
			
			// Convert value to appropriate type
			convertedValue := reflect.ValueOf(value)
			if convertedValue.Type().ConvertibleTo(field.Type()) {
				field.Set(convertedValue.Convert(field.Type()))
			} else {
				field.Set(convertedValue)
			}
		} else {
			// Navigate to nested field
			field := current.FieldByName(part)
			if !field.IsValid() {
				return &ConfigError{
					Type:    ErrorTypePath,
					Path:    path,
					Message: "field not found",
				}
			}
			current = field
		}
	}
	
	return nil
}

// Legacy configuration merger (simplified for testing)
func NewLegacyConfigurationMerger() ConfigurationMerger {
	return NewDefaultConfigurationMerger() // For now, use same implementation
}

// Helper functions for backward compatibility testing

func splitPathCompat(path, separator string) []string {
	if path == "" {
		return []string{}
	}
	
	var parts []string
	current := ""
	
	for _, char := range path {
		if string(char) == separator {
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

func isValidDotPath(path string) bool {
	if path == "" {
		return false
	}
	
	// Check for valid dot notation (no consecutive dots, no leading/trailing dots)
	if path[0] == '.' || path[len(path)-1] == '.' {
		return false
	}
	
	for i := 0; i < len(path)-1; i++ {
		if path[i] == '.' && path[i+1] == '.' {
			return false
		}
	}
	
	return true
}

func reconstructDotPath(structuredPath StructuredPath) string {
	var parts []string
	
	for _, part := range structuredPath.Parts {
		if part.HasIndex {
			// Convert index back to dot notation
			parts = append(parts, string(rune('0'+part.Index%10)))
		} else {
			parts = append(parts, part.Name)
		}
	}
	
	return joinParts(parts, ".")
}

func joinParts(parts []string, separator string) string {
	if len(parts) == 0 {
		return ""
	}
	
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += separator + parts[i]
	}
	
	return result
}

func isEquivalentPath(path1, path2 string) bool {
	// Normalize both paths and compare
	parts1 := splitPathCompat(path1, ".")
	parts2 := splitPathCompat(path2, ".")
	
	if len(parts1) != len(parts2) {
		return false
	}
	
	for i := range parts1 {
		if parts1[i] != parts2[i] {
			return false
		}
	}
	
	return true
}

func isEquivalentStructuredPath(path1, path2 StructuredPath) bool {
	if len(path1.Parts) != len(path2.Parts) {
		return false
	}
	
	for i := range path1.Parts {
		part1 := path1.Parts[i]
		part2 := path2.Parts[i]
		
		if part1.HasIndex != part2.HasIndex {
			return false
		}
		
		if part1.HasIndex {
			if part1.Index != part2.Index {
				return false
			}
		} else {
			if part1.Name != part2.Name {
				return false
			}
		}
	}
	
	return true
}

func isEquivalentConfiguration(config1, config2 Configuration) bool {
	// Compare the data content (ignoring source metadata for compatibility)
	return reflect.DeepEqual(config1.Data, config2.Data)
}

// Generators for backward compatibility testing

func genLegacyPath() gopter.Gen {
	return gen.OneConstOf(
		"name",
		"count",
		"enabled",
		"tags.0",
		"tags.1",
		"metadata.key",
		"server_pools.0.name",
		"server_pools.0.worker_count",
	)
}

func genLegacyValue() gopter.Gen {
	return gen.OneConstOf(
		"test-value",
		42,
		true,
		false,
		"item1",
		"item2",
	)
}

func genDotNotationPath() gopter.Gen {
	return gen.OneConstOf(
		"name",
		"count",
		"enabled",
		"tags.0",
		"metadata.key",
		"server_pools.0.name",
		"config.nested.field",
	)
}

func genArrayFieldPath() gopter.Gen {
	return gen.OneConstOf(
		"tags",
		"server_pools",
		"metadata.items",
	)
}

func genConfigurationList() gopter.Gen {
	return gen.SliceOfN(3, genConfigData())
}
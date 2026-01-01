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
	"time"
)

// ConfigurationMerger combines multiple configuration sources with precedence rules
type ConfigurationMerger interface {
	// MergeConfigurations combines configs with precedence rules
	MergeConfigurations(configs []Configuration) (*Configuration, error)
	
	// SetMergeStrategy defines how conflicts are resolved
	SetMergeStrategy(strategy MergeStrategy) error
	
	// AddConfigSource adds a new configuration source
	AddConfigSource(source ConfigSource) error
}

// MergeStrategy defines conflict resolution behavior
type MergeStrategy struct {
	ArrayMergeMode  ArrayMergeMode  // append, replace, merge
	ObjectMergeMode ObjectMergeMode // deep, shallow, replace
	Precedence      []SourceType    // order of precedence
}

// ArrayMergeMode defines how arrays are merged
type ArrayMergeMode string

const (
	ArrayMergeAppend  ArrayMergeMode = "append"  // Append arrays together
	ArrayMergeReplace ArrayMergeMode = "replace" // Replace with higher precedence array
	ArrayMergeMerge   ArrayMergeMode = "merge"   // Merge arrays by index
)

// ObjectMergeMode defines how objects are merged
type ObjectMergeMode string

const (
	ObjectMergeDeep    ObjectMergeMode = "deep"    // Deep merge objects recursively
	ObjectMergeShallow ObjectMergeMode = "shallow" // Shallow merge objects
	ObjectMergeReplace ObjectMergeMode = "replace" // Replace with higher precedence object
)

// SourceType defines the type of configuration source
type SourceType string

const (
	SourceCLI      SourceType = "cli"
	SourceFile     SourceType = "file"
	SourceTemplate SourceType = "template"
	SourceDefault  SourceType = "default"
)

// Configuration represents a complete configuration
type Configuration struct {
	Data      map[string]interface{} `json:"data"`
	Sources   []ConfigSource         `json:"sources"`
	Metadata  ConfigMetadata         `json:"metadata"`
}

// ConfigSource tracks where configuration came from
type ConfigSource struct {
	Type     SourceType `json:"type"`
	Path     string     `json:"path"`
	Priority int        `json:"priority"`
}

// ConfigMetadata tracks configuration source and processing info
type ConfigMetadata struct {
	Sources     []ConfigSource `json:"sources"`
	ProcessedAt time.Time      `json:"processed_at"`
	Version     string         `json:"version"`
	Checksum    string         `json:"checksum"`
}

// DefaultConfigurationMerger implements the ConfigurationMerger interface
type DefaultConfigurationMerger struct {
	strategy MergeStrategy
	sources  []ConfigSource
}

// NewDefaultConfigurationMerger creates a new configuration merger with default settings
func NewDefaultConfigurationMerger() *DefaultConfigurationMerger {
	return &DefaultConfigurationMerger{
		strategy: MergeStrategy{
			ArrayMergeMode:  ArrayMergeAppend,
			ObjectMergeMode: ObjectMergeDeep,
			Precedence: []SourceType{
				SourceDefault,  // Lowest precedence
				SourceFile,
				SourceTemplate,
				SourceCLI,      // Highest precedence
			},
		},
		sources: []ConfigSource{},
	}
}

// SetMergeStrategy defines how conflicts are resolved
func (m *DefaultConfigurationMerger) SetMergeStrategy(strategy MergeStrategy) error {
	if len(strategy.Precedence) == 0 {
		return fmt.Errorf("precedence order cannot be empty")
	}
	
	m.strategy = strategy
	return nil
}

// AddConfigSource adds a new configuration source
func (m *DefaultConfigurationMerger) AddConfigSource(source ConfigSource) error {
	if source.Type == "" {
		return fmt.Errorf("config source type cannot be empty")
	}
	
	m.sources = append(m.sources, source)
	return nil
}

// MergeConfigurations combines configs with precedence rules
func (m *DefaultConfigurationMerger) MergeConfigurations(configs []Configuration) (*Configuration, error) {
	if len(configs) == 0 {
		return &Configuration{
			Data:     make(map[string]interface{}),
			Sources:  []ConfigSource{},
			Metadata: ConfigMetadata{ProcessedAt: time.Now()},
		}, nil
	}
	
	// Sort configurations by precedence
	sortedConfigs, err := m.sortConfigsByPrecedence(configs)
	if err != nil {
		return nil, fmt.Errorf("failed to sort configurations by precedence: %w", err)
	}
	
	// Start with an empty result configuration
	result := &Configuration{
		Data:     make(map[string]interface{}),
		Sources:  []ConfigSource{},
		Metadata: ConfigMetadata{ProcessedAt: time.Now()},
	}
	
	// Merge configurations in precedence order (lowest to highest)
	for _, config := range sortedConfigs {
		if err := m.mergeConfiguration(result, &config); err != nil {
			return nil, fmt.Errorf("failed to merge configuration: %w", err)
		}
	}
	
	// Update metadata
	result.Metadata.Sources = result.Sources
	
	return result, nil
}

// sortConfigsByPrecedence sorts configurations by their source precedence
func (m *DefaultConfigurationMerger) sortConfigsByPrecedence(configs []Configuration) ([]Configuration, error) {
	// Create a precedence map for quick lookup
	precedenceMap := make(map[SourceType]int)
	for i, sourceType := range m.strategy.Precedence {
		precedenceMap[sourceType] = i
	}
	
	// Sort configurations by precedence
	sortedConfigs := make([]Configuration, len(configs))
	copy(sortedConfigs, configs)
	
	// Simple bubble sort by precedence (could be optimized for large datasets)
	for i := 0; i < len(sortedConfigs); i++ {
		for j := i + 1; j < len(sortedConfigs); j++ {
			iPrecedence := m.getConfigPrecedence(sortedConfigs[i], precedenceMap)
			jPrecedence := m.getConfigPrecedence(sortedConfigs[j], precedenceMap)
			
			if iPrecedence > jPrecedence {
				sortedConfigs[i], sortedConfigs[j] = sortedConfigs[j], sortedConfigs[i]
			}
		}
	}
	
	return sortedConfigs, nil
}

// getConfigPrecedence determines the precedence of a configuration
func (m *DefaultConfigurationMerger) getConfigPrecedence(config Configuration, precedenceMap map[SourceType]int) int {
	// Use the highest precedence source in the configuration
	maxPrecedence := -1
	for _, source := range config.Sources {
		if precedence, exists := precedenceMap[source.Type]; exists {
			if precedence > maxPrecedence {
				maxPrecedence = precedence
			}
		}
	}
	
	// If no known source type, assign lowest precedence
	if maxPrecedence == -1 {
		return -1
	}
	
	return maxPrecedence
}

// mergeConfiguration merges a source configuration into the target configuration
func (m *DefaultConfigurationMerger) mergeConfiguration(target, source *Configuration) error {
	// Merge data
	if err := m.mergeData(target.Data, source.Data); err != nil {
		return fmt.Errorf("failed to merge configuration data: %w", err)
	}
	
	// Merge sources
	target.Sources = append(target.Sources, source.Sources...)
	
	return nil
}

// mergeData merges source data into target data based on merge strategy
func (m *DefaultConfigurationMerger) mergeData(target, source map[string]interface{}) error {
	for key, sourceValue := range source {
		targetValue, exists := target[key]
		
		if !exists {
			// Key doesn't exist in target, just copy it
			target[key] = sourceValue
			continue
		}
		
		// Key exists in both, need to merge based on strategy
		mergedValue, err := m.mergeValues(targetValue, sourceValue)
		if err != nil {
			return fmt.Errorf("failed to merge values for key '%s': %w", key, err)
		}
		
		target[key] = mergedValue
	}
	
	return nil
}

// mergeValues merges two values based on their types and merge strategy
func (m *DefaultConfigurationMerger) mergeValues(target, source interface{}) (interface{}, error) {
	// Handle nil values
	if source == nil {
		return target, nil
	}
	if target == nil {
		return source, nil
	}
	
	// Handle different type combinations
	switch sourceVal := source.(type) {
	case map[string]interface{}:
		if targetMap, ok := target.(map[string]interface{}); ok {
			// Both are objects, merge based on object merge mode
			return m.mergeObjects(targetMap, sourceVal)
		} else {
			// Target is not an object, replace based on object merge mode
			if m.strategy.ObjectMergeMode == ObjectMergeReplace {
				return source, nil
			}
			return source, nil // Default to replace for type mismatches
		}
	case []interface{}:
		if targetArray, ok := target.([]interface{}); ok {
			// Both are arrays, merge based on array merge mode
			return m.mergeArrays(targetArray, sourceVal)
		} else {
			// Target is not an array, replace based on array merge mode
			if m.strategy.ArrayMergeMode == ArrayMergeReplace {
				return source, nil
			}
			return source, nil // Default to replace for type mismatches
		}
	default:
		// Primitive values, source takes precedence
		return source, nil
	}
}

// mergeObjects merges two objects based on the object merge strategy
func (m *DefaultConfigurationMerger) mergeObjects(target, source map[string]interface{}) (interface{}, error) {
	switch m.strategy.ObjectMergeMode {
	case ObjectMergeReplace:
		return source, nil
	case ObjectMergeShallow:
		// Shallow merge: only merge top-level keys
		result := make(map[string]interface{})
		for k, v := range target {
			result[k] = v
		}
		for k, v := range source {
			result[k] = v
		}
		return result, nil
	case ObjectMergeDeep:
		// Deep merge: recursively merge nested objects
		result := make(map[string]interface{})
		for k, v := range target {
			result[k] = v
		}
		for k, v := range source {
			if existingValue, exists := result[k]; exists {
				mergedValue, err := m.mergeValues(existingValue, v)
				if err != nil {
					return nil, fmt.Errorf("failed to deep merge key '%s': %w", k, err)
				}
				result[k] = mergedValue
			} else {
				result[k] = v
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported object merge mode: %s", m.strategy.ObjectMergeMode)
	}
}

// mergeArrays merges two arrays based on the array merge strategy
func (m *DefaultConfigurationMerger) mergeArrays(target, source []interface{}) (interface{}, error) {
	switch m.strategy.ArrayMergeMode {
	case ArrayMergeReplace:
		return source, nil
	case ArrayMergeAppend:
		result := make([]interface{}, len(target)+len(source))
		copy(result, target)
		copy(result[len(target):], source)
		return result, nil
	case ArrayMergeMerge:
		// Merge by index: source elements override target elements at same index
		maxLen := len(target)
		if len(source) > maxLen {
			maxLen = len(source)
		}
		result := make([]interface{}, maxLen)
		
		// Copy target elements
		for i, v := range target {
			result[i] = v
		}
		
		// Override with source elements
		for i, v := range source {
			if i < len(result) {
				result[i] = v
			}
		}
		
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported array merge mode: %s", m.strategy.ArrayMergeMode)
	}
}
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
)

// JSONFlagHandler handles --json-set flags with JSON-formatted configuration data
type JSONFlagHandler struct{}

// NewJSONFlagHandler creates a new JSON flag handler
func NewJSONFlagHandler() *JSONFlagHandler {
	return &JSONFlagHandler{}
}

// CanHandle returns true if this handler can process the given flag
func (h *JSONFlagHandler) CanHandle(flagName string) bool {
	return strings.HasPrefix(flagName, "json-set")
}

// ParseFlag processes a single flag and returns the parsed result
func (h *JSONFlagHandler) ParseFlag(flagName, value string) (interface{}, error) {
	return h.parseJSONFlag(flagName, value)
}

// GetFlagType returns the type of flags this handler processes
func (h *JSONFlagHandler) GetFlagType() FlagType {
	return FlagTypeJSON
}

// parseJSONFlag parses a JSON flag value and returns the parsed data
func (h *JSONFlagHandler) parseJSONFlag(flagName, value string) (*JSONFlag, error) {
	if value == "" {
		return nil, fmt.Errorf("JSON flag value cannot be empty")
	}
	
	// Extract the path from the flag name
	path := h.extractPath(flagName)
	if path == "" {
		return nil, fmt.Errorf("invalid JSON flag format: expected --json-set path=value, got %s", flagName)
	}
	
	// Parse the JSON value
	var parsedValue interface{}
	if err := json.Unmarshal([]byte(value), &parsedValue); err != nil {
		return nil, fmt.Errorf("invalid JSON syntax in flag '%s': %w. Expected format: --json-set 'path={\"key\": \"value\"}'", flagName, err)
	}
	
	// Validate the parsed JSON
	if err := h.validateJSONValue(parsedValue); err != nil {
		return nil, fmt.Errorf("invalid JSON value in flag '%s': %w", flagName, err)
	}
	
	config := &JSONFlag{
		Path:  path,
		Value: parsedValue,
	}
	
	return config, nil
}

// extractPath extracts the configuration path from a JSON flag name
func (h *JSONFlagHandler) extractPath(flagName string) string {
	// Handle different JSON flag formats:
	// --json-set path=value -> path
	// --json-set-path=value -> path
	if strings.HasPrefix(flagName, "json-set-") {
		return strings.TrimPrefix(flagName, "json-set-")
	}
	
	// For --json-set path=value format, the path is extracted differently
	// This will be handled by the parser that splits on '=' first
	return ""
}

// validateJSONValue validates that the parsed JSON value is acceptable
func (h *JSONFlagHandler) validateJSONValue(value interface{}) error {
	switch v := value.(type) {
	case nil:
		return fmt.Errorf("JSON value cannot be null")
	case map[string]interface{}:
		// Recursively validate nested objects
		for key, val := range v {
			if key == "" {
				return fmt.Errorf("JSON object keys cannot be empty")
			}
			if err := h.validateJSONValue(val); err != nil {
				return fmt.Errorf("invalid value for key '%s': %w", key, err)
			}
		}
	case []interface{}:
		// Validate array elements
		for i, val := range v {
			if err := h.validateJSONValue(val); err != nil {
				return fmt.Errorf("invalid value at index %d: %w", i, err)
			}
		}
	case string, float64, bool:
		// These are valid JSON primitive types
		return nil
	default:
		return fmt.Errorf("unsupported JSON value type: %T", v)
	}
	
	return nil
}

// MergeIntoConfiguration merges the JSON configuration into an existing configuration
func (h *JSONFlagHandler) MergeIntoConfiguration(config *JSONFlag, target map[string]interface{}) error {
	if config == nil {
		return fmt.Errorf("JSON config cannot be nil")
	}
	
	if target == nil {
		return fmt.Errorf("target configuration cannot be nil")
	}
	
	// Split the path into components
	pathParts := strings.Split(config.Path, ".")
	if len(pathParts) == 0 {
		return fmt.Errorf("invalid configuration path: '%s'", config.Path)
	}
	
	// Navigate to the target location and set the value
	current := target
	for i, part := range pathParts[:len(pathParts)-1] {
		if part == "" {
			return fmt.Errorf("empty path component at position %d in path '%s'", i, config.Path)
		}
		
		// Create nested maps as needed
		if _, exists := current[part]; !exists {
			current[part] = make(map[string]interface{})
		}
		
		// Ensure the current value is a map
		if nextMap, ok := current[part].(map[string]interface{}); ok {
			current = nextMap
		} else {
			return fmt.Errorf("path conflict: '%s' is not an object at path '%s'", part, strings.Join(pathParts[:i+1], "."))
		}
	}
	
	// Set the final value
	finalKey := pathParts[len(pathParts)-1]
	if finalKey == "" {
		return fmt.Errorf("empty final key in path '%s'", config.Path)
	}
	
	// Merge the value based on its type
	if err := h.mergeValue(current, finalKey, config.Value); err != nil {
		return fmt.Errorf("failed to merge JSON value at path '%s': %w", config.Path, err)
	}
	
	return nil
}

// mergeValue merges a JSON value into the target configuration
func (h *JSONFlagHandler) mergeValue(target map[string]interface{}, key string, value interface{}) error {
	existingValue, exists := target[key]
	
	if !exists {
		// Simple case: key doesn't exist, just set it
		target[key] = value
		return nil
	}
	
	// Handle merging based on value types
	switch newVal := value.(type) {
	case map[string]interface{}:
		// Merge objects
		if existingMap, ok := existingValue.(map[string]interface{}); ok {
			return h.mergeObjects(existingMap, newVal)
		} else {
			// Replace non-object with object
			target[key] = value
		}
	case []interface{}:
		// For arrays, replace by default (could be configurable in the future)
		target[key] = value
	default:
		// For primitives, replace the existing value
		target[key] = value
	}
	
	return nil
}

// mergeObjects performs a deep merge of two JSON objects
func (h *JSONFlagHandler) mergeObjects(target, source map[string]interface{}) error {
	for key, value := range source {
		if err := h.mergeValue(target, key, value); err != nil {
			return fmt.Errorf("failed to merge key '%s': %w", key, err)
		}
	}
	return nil
}
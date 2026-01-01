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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// YAMLFlagHandler handles --yaml-set, --yaml-data, and --yaml-file flags
type YAMLFlagHandler struct{}

// NewYAMLFlagHandler creates a new YAML flag handler
func NewYAMLFlagHandler() *YAMLFlagHandler {
	return &YAMLFlagHandler{}
}

// CanHandle returns true if this handler can process the given flag
func (h *YAMLFlagHandler) CanHandle(flagName string) bool {
	return strings.HasPrefix(flagName, "yaml-set") ||
		strings.HasPrefix(flagName, "yaml-data") ||
		strings.HasPrefix(flagName, "yaml-file")
}

// ParseFlag processes a single flag and returns the parsed result
func (h *YAMLFlagHandler) ParseFlag(flagName, value string) (interface{}, error) {
	return h.parseYAMLFlag(flagName, value)
}

// GetFlagType returns the type of flags this handler processes
func (h *YAMLFlagHandler) GetFlagType() FlagType {
	return FlagTypeYAML
}

// parseYAMLFlag parses a YAML flag value and returns the parsed data
func (h *YAMLFlagHandler) parseYAMLFlag(flagName, value string) (*YAMLFlag, error) {
	if value == "" {
		return nil, fmt.Errorf("YAML flag value cannot be empty")
	}
	
	var parsedValue interface{}
	var path string
	var err error
	
	if strings.HasPrefix(flagName, "yaml-file") {
		// Handle --yaml-file path=filename format
		path, parsedValue, err = h.parseYAMLFile(flagName, value)
		if err != nil {
			return nil, err
		}
	} else {
		// Handle --yaml-set and --yaml-data flags
		path = h.extractPath(flagName)
		if path == "" {
			return nil, fmt.Errorf("invalid YAML flag format: expected --yaml-set path or --yaml-data path, got %s", flagName)
		}
		
		// Parse the YAML value
		if err := yaml.Unmarshal([]byte(value), &parsedValue); err != nil {
			return nil, fmt.Errorf("invalid YAML syntax in flag '%s': %w", flagName, err)
		}
	}
	
	// Validate the parsed YAML
	if err := h.validateYAMLValue(parsedValue); err != nil {
		return nil, fmt.Errorf("invalid YAML value in flag '%s': %w", flagName, err)
	}
	
	yamlFlag := &YAMLFlag{
		Path:  path,
		Value: parsedValue,
	}
	
	return yamlFlag, nil
}

// parseYAMLFile handles --yaml-file flags that load external YAML files
func (h *YAMLFlagHandler) parseYAMLFile(flagName, value string) (string, interface{}, error) {
	// Parse path=filename format
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid --yaml-file format: expected path=filename, got %s", value)
	}
	
	path := strings.TrimSpace(parts[0])
	filename := strings.TrimSpace(parts[1])
	
	if path == "" {
		return "", nil, fmt.Errorf("empty path in --yaml-file flag")
	}
	
	if filename == "" {
		return "", nil, fmt.Errorf("empty filename in --yaml-file flag")
	}
	
	// Load and parse the YAML file
	parsedValue, err := h.loadYAMLFile(filename)
	if err != nil {
		return "", nil, fmt.Errorf("failed to load YAML file '%s': %w", filename, err)
	}
	
	return path, parsedValue, nil
}

// loadYAMLFile loads and parses a YAML file, supporting multi-document YAML
func (h *YAMLFlagHandler) loadYAMLFile(filename string) (interface{}, error) {
	// Resolve relative paths
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve file path: %w", err)
	}
	
	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("YAML file does not exist: %s", absPath)
	}
	
	// Read the file
	data, err := ioutil.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file: %w", err)
	}
	
	// Handle multi-document YAML
	documents := strings.Split(string(data), "---")
	if len(documents) == 1 {
		// Single document
		var parsedValue interface{}
		if err := yaml.Unmarshal(data, &parsedValue); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
		return parsedValue, nil
	}
	
	// Multi-document YAML
	var parsedDocuments []interface{}
	for i, doc := range documents {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue // Skip empty documents
		}
		
		var parsedDoc interface{}
		if err := yaml.Unmarshal([]byte(doc), &parsedDoc); err != nil {
			return nil, fmt.Errorf("failed to parse YAML document %d: %w", i+1, err)
		}
		parsedDocuments = append(parsedDocuments, parsedDoc)
	}
	
	if len(parsedDocuments) == 1 {
		return parsedDocuments[0], nil
	}
	
	return parsedDocuments, nil
}

// extractPath extracts the configuration path from a YAML flag name
func (h *YAMLFlagHandler) extractPath(flagName string) string {
	// Handle different YAML flag formats:
	// --yaml-set-path -> path
	// --yaml-data-path -> path
	if strings.HasPrefix(flagName, "yaml-set-") {
		return strings.TrimPrefix(flagName, "yaml-set-")
	}
	if strings.HasPrefix(flagName, "yaml-data-") {
		return strings.TrimPrefix(flagName, "yaml-data-")
	}
	
	return ""
}

// validateYAMLValue validates that the parsed YAML value is acceptable
func (h *YAMLFlagHandler) validateYAMLValue(value interface{}) error {
	switch v := value.(type) {
	case nil:
		return fmt.Errorf("YAML value cannot be null")
	case map[string]interface{}:
		// Recursively validate nested objects
		for key, val := range v {
			if key == "" {
				return fmt.Errorf("YAML object keys cannot be empty")
			}
			if err := h.validateYAMLValue(val); err != nil {
				return fmt.Errorf("invalid value for key '%s': %w", key, err)
			}
		}
	case []interface{}:
		// Validate array elements
		for i, val := range v {
			if err := h.validateYAMLValue(val); err != nil {
				return fmt.Errorf("invalid value at index %d: %w", i, err)
			}
		}
		// Handle multi-document YAML if it's an array of documents
		for i, doc := range v {
			if err := h.validateYAMLValue(doc); err != nil {
				return fmt.Errorf("invalid document %d: %w", i+1, err)
			}
		}
	case string, int, int64, float64, bool:
		// These are valid YAML primitive types
		return nil
	default:
		return fmt.Errorf("unsupported YAML value type: %T", v)
	}
	
	return nil
}

// MergeIntoConfiguration merges the YAML configuration into an existing configuration
func (h *YAMLFlagHandler) MergeIntoConfiguration(config *YAMLFlag, target map[string]interface{}) error {
	if config == nil {
		return fmt.Errorf("YAML config cannot be nil")
	}
	
	if target == nil {
		return fmt.Errorf("target configuration cannot be nil")
	}
	
	// Handle multi-document YAML
	if docs, ok := config.Value.([]interface{}); ok {
		// For multi-document YAML, merge each document
		for i, doc := range docs {
			docFlag := &YAMLFlag{
				Path:  fmt.Sprintf("%s.doc%d", config.Path, i),
				Value: doc,
			}
			if err := h.MergeIntoConfiguration(docFlag, target); err != nil {
				return fmt.Errorf("failed to merge YAML document %d: %w", i+1, err)
			}
		}
		return nil
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
		return fmt.Errorf("failed to merge YAML value at path '%s': %w", config.Path, err)
	}
	
	return nil
}

// mergeValue merges a YAML value into the target configuration
func (h *YAMLFlagHandler) mergeValue(target map[string]interface{}, key string, value interface{}) error {
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

// mergeObjects performs a deep merge of two YAML objects
func (h *YAMLFlagHandler) mergeObjects(target, source map[string]interface{}) error {
	for key, value := range source {
		if err := h.mergeValue(target, key, value); err != nil {
			return fmt.Errorf("failed to merge key '%s': %w", key, err)
		}
	}
	return nil
}
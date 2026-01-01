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

// MapFlagHandler handles advanced map operations
type MapFlagHandler struct{}

// MapOperation represents different map operations
type MapOperation string

const (
	MapOpSet    MapOperation = "set"
	MapOpMerge  MapOperation = "merge"
	MapOpRemove MapOperation = "remove"
)

// MapFlag represents a map operation flag
type MapFlag struct {
	Operation MapOperation
	Path      string
	Key       string // For set/remove operations
	Value     interface{}
}

// GetPath returns the configuration path this flag affects
func (f *MapFlag) GetPath() string {
	return f.Path
}

// NewMapFlagHandler creates a new map flag handler
func NewMapFlagHandler() *MapFlagHandler {
	return &MapFlagHandler{}
}

// CanHandle returns true if this handler can process the given flag
func (h *MapFlagHandler) CanHandle(flagName string) bool {
	return strings.HasPrefix(flagName, "map-set") ||
		strings.HasPrefix(flagName, "map-merge") ||
		strings.HasPrefix(flagName, "map-remove")
}

// GetFlagType returns the type of flags this handler processes
func (h *MapFlagHandler) GetFlagType() FlagType {
	return FlagTypeMapOp
}

// ParseFlag parses map operation flags
func (h *MapFlagHandler) ParseFlag(flagName, flagValue string) (interface{}, error) {
	var operation MapOperation
	
	// Determine operation from flag name
	switch {
	case strings.HasPrefix(flagName, "map-set"):
		operation = MapOpSet
	case strings.HasPrefix(flagName, "map-merge"):
		operation = MapOpMerge
	case strings.HasPrefix(flagName, "map-remove"):
		operation = MapOpRemove
	default:
		return nil, fmt.Errorf("unsupported map operation flag: %s", flagName)
	}
	
	// Parse path and value
	path, key, value, err := h.parseMapFlag(flagValue, operation)
	if err != nil {
		return nil, fmt.Errorf("failed to parse map flag '%s=%s': %w", flagName, flagValue, err)
	}
	
	return &MapFlag{
		Operation: operation,
		Path:      path,
		Key:       key,
		Value:     value,
	}, nil
}

// MergeIntoConfiguration applies the map operation to the configuration
func (h *MapFlagHandler) MergeIntoConfiguration(flag ParsedFlag, config map[string]interface{}) error {
	mapFlag, ok := flag.(*MapFlag)
	if !ok {
		return fmt.Errorf("expected MapFlag, got %T", flag)
	}
	
	// Navigate to the map location
	pathParts := strings.Split(mapFlag.Path, ".")
	current := config
	
	// Navigate to the target map
	for i, part := range pathParts {
		if i == len(pathParts)-1 {
			// This is the target map
			break
		}
		
		if next, exists := current[part]; exists {
			if nextMap, ok := next.(map[string]interface{}); ok {
				current = nextMap
			} else {
				return fmt.Errorf("path '%s' at part '%s' is not an object", strings.Join(pathParts[:i+1], "."), part)
			}
		} else {
			// Create intermediate objects
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
		}
	}
	
	// Get the map field name
	mapField := pathParts[len(pathParts)-1]
	
	// Get or create the target map
	var targetMap map[string]interface{}
	if existing, exists := current[mapField]; exists {
		if existingMap, ok := existing.(map[string]interface{}); ok {
			targetMap = existingMap
		} else {
			return fmt.Errorf("field '%s' is not a map", mapFlag.Path)
		}
	} else {
		targetMap = make(map[string]interface{})
		current[mapField] = targetMap
	}
	
	// Apply the operation
	switch mapFlag.Operation {
	case MapOpSet:
		targetMap[mapFlag.Key] = mapFlag.Value
	case MapOpMerge:
		// Merge the provided map into the target map
		if mergeMap, ok := mapFlag.Value.(map[string]interface{}); ok {
			for k, v := range mergeMap {
				targetMap[k] = v
			}
		} else {
			return fmt.Errorf("merge operation requires a map value, got %T", mapFlag.Value)
		}
	case MapOpRemove:
		delete(targetMap, mapFlag.Key)
	default:
		return fmt.Errorf("unsupported map operation: %s", mapFlag.Operation)
	}
	
	return nil
}

// parseMapFlag parses the flag value based on operation type
func (h *MapFlagHandler) parseMapFlag(flagValue string, operation MapOperation) (string, string, interface{}, error) {
	switch operation {
	case MapOpSet:
		// Format: path.key=value
		parts := strings.SplitN(flagValue, "=", 2)
		if len(parts) != 2 {
			return "", "", nil, fmt.Errorf("expected format 'path.key=value', got '%s'", flagValue)
		}
		
		pathAndKey := parts[0]
		value := parts[1]
		
		// Split path and key
		lastDot := strings.LastIndex(pathAndKey, ".")
		if lastDot == -1 {
			return "", "", nil, fmt.Errorf("map-set requires path.key format, got '%s'", pathAndKey)
		}
		
		path := pathAndKey[:lastDot]
		key := pathAndKey[lastDot+1:]
		
		return path, key, h.parseValue(value), nil
		
	case MapOpMerge:
		// Format: path={"key": "value"}
		parts := strings.SplitN(flagValue, "=", 2)
		if len(parts) != 2 {
			return "", "", nil, fmt.Errorf("expected format 'path={\"key\": \"value\"}', got '%s'", flagValue)
		}
		
		path := parts[0]
		jsonValue := parts[1]
		
		// Parse JSON
		var mergeData map[string]interface{}
		if err := json.Unmarshal([]byte(jsonValue), &mergeData); err != nil {
			return "", "", nil, fmt.Errorf("invalid JSON for merge operation: %w", err)
		}
		
		return path, "", mergeData, nil
		
	case MapOpRemove:
		// Format: path.key
		lastDot := strings.LastIndex(flagValue, ".")
		if lastDot == -1 {
			return "", "", nil, fmt.Errorf("map-remove requires path.key format, got '%s'", flagValue)
		}
		
		path := flagValue[:lastDot]
		key := flagValue[lastDot+1:]
		
		return path, key, nil, nil
		
	default:
		return "", "", nil, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// parseValue attempts to parse a string value into appropriate type
func (h *MapFlagHandler) parseValue(value string) interface{} {
	// Try to parse as JSON first
	var jsonValue interface{}
	if err := json.Unmarshal([]byte(value), &jsonValue); err == nil {
		return jsonValue
	}
	
	// Return as string if JSON parsing fails
	return value
}
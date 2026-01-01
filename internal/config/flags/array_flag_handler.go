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
	"strconv"
	"strings"
)

// ArrayOperationFlagHandler handles advanced array operations
type ArrayOperationFlagHandler struct{}

// ArrayOperation represents different array operations
type ArrayOperation string

const (
	ArrayOpAppend ArrayOperation = "append"
	ArrayOpInsert ArrayOperation = "insert"
	ArrayOpRemove ArrayOperation = "remove"
)

// ArrayOperationFlag represents an array operation flag
type ArrayOperationFlag struct {
	Operation ArrayOperation
	Path      string
	Index     int // -1 for append/remove operations
	Value     interface{}
}

// GetPath returns the configuration path this flag affects
func (f *ArrayOperationFlag) GetPath() string {
	return f.Path
}

// NewArrayFlagHandler creates a new array flag handler
func NewArrayFlagHandler() *ArrayOperationFlagHandler {
	return &ArrayOperationFlagHandler{}
}

// CanHandle returns true if this handler can process the given flag
func (h *ArrayOperationFlagHandler) CanHandle(flagName string) bool {
	return strings.HasPrefix(flagName, "array-append") ||
		strings.HasPrefix(flagName, "array-insert") ||
		strings.HasPrefix(flagName, "array-remove")
}

// GetFlagType returns the type of flags this handler processes
func (h *ArrayOperationFlagHandler) GetFlagType() FlagType {
	return FlagTypeArrayOp
}

// ParseFlag parses array operation flags
func (h *ArrayOperationFlagHandler) ParseFlag(flagName, flagValue string) (interface{}, error) {
	var operation ArrayOperation
	
	// Determine operation from flag name
	switch {
	case strings.HasPrefix(flagName, "array-append"):
		operation = ArrayOpAppend
	case strings.HasPrefix(flagName, "array-insert"):
		operation = ArrayOpInsert
	case strings.HasPrefix(flagName, "array-remove"):
		operation = ArrayOpRemove
	default:
		return nil, fmt.Errorf("unsupported array operation flag: %s", flagName)
	}
	
	// Parse path and value
	path, value, index, err := h.parseArrayFlag(flagValue, operation)
	if err != nil {
		return nil, fmt.Errorf("failed to parse array flag '%s=%s': %w", flagName, flagValue, err)
	}
	
	return &ArrayOperationFlag{
		Operation: operation,
		Path:      path,
		Index:     index,
		Value:     value,
	}, nil
}

// MergeIntoConfiguration applies the array operation to the configuration
func (h *ArrayOperationFlagHandler) MergeIntoConfiguration(flag ParsedFlag, config map[string]interface{}) error {
	arrayFlag, ok := flag.(*ArrayOperationFlag)
	if !ok {
		return fmt.Errorf("expected ArrayOperationFlag, got %T", flag)
	}
	
	// Navigate to the array location
	pathParts := strings.Split(arrayFlag.Path, ".")
	current := config
	
	// Navigate to the parent of the array
	for i, part := range pathParts[:len(pathParts)-1] {
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
	
	// Get the array field name
	arrayField := pathParts[len(pathParts)-1]
	
	// Get or create the array
	var targetArray []interface{}
	if existing, exists := current[arrayField]; exists {
		if existingArray, ok := existing.([]interface{}); ok {
			targetArray = existingArray
		} else {
			return fmt.Errorf("field '%s' is not an array", arrayFlag.Path)
		}
	} else {
		targetArray = []interface{}{}
	}
	
	// Apply the operation
	switch arrayFlag.Operation {
	case ArrayOpAppend:
		targetArray = append(targetArray, arrayFlag.Value)
	case ArrayOpInsert:
		if arrayFlag.Index < 0 || arrayFlag.Index > len(targetArray) {
			return fmt.Errorf("insert index %d is out of bounds for array of length %d", arrayFlag.Index, len(targetArray))
		}
		// Insert at index
		targetArray = append(targetArray[:arrayFlag.Index], append([]interface{}{arrayFlag.Value}, targetArray[arrayFlag.Index:]...)...)
	case ArrayOpRemove:
		// Remove matching values
		newArray := []interface{}{}
		for _, item := range targetArray {
			if !h.valuesEqual(item, arrayFlag.Value) {
				newArray = append(newArray, item)
			}
		}
		targetArray = newArray
	default:
		return fmt.Errorf("unsupported array operation: %s", arrayFlag.Operation)
	}
	
	// Update the configuration
	current[arrayField] = targetArray
	return nil
}

// parseArrayFlag parses the flag value based on operation type
func (h *ArrayOperationFlagHandler) parseArrayFlag(flagValue string, operation ArrayOperation) (string, interface{}, int, error) {
	switch operation {
	case ArrayOpAppend, ArrayOpRemove:
		// Format: path=value
		parts := strings.SplitN(flagValue, "=", 2)
		if len(parts) != 2 {
			return "", nil, -1, fmt.Errorf("expected format 'path=value', got '%s'", flagValue)
		}
		return parts[0], h.parseValue(parts[1]), -1, nil
		
	case ArrayOpInsert:
		// Format: path[index]=value
		if !strings.Contains(flagValue, "[") || !strings.Contains(flagValue, "]") {
			return "", nil, -1, fmt.Errorf("insert operation requires format 'path[index]=value', got '%s'", flagValue)
		}
		
		// Extract path, index, and value
		bracketStart := strings.Index(flagValue, "[")
		bracketEnd := strings.Index(flagValue, "]")
		equalPos := strings.Index(flagValue, "=")
		
		if bracketStart == -1 || bracketEnd == -1 || equalPos == -1 || bracketEnd >= equalPos {
			return "", nil, -1, fmt.Errorf("invalid insert format, expected 'path[index]=value', got '%s'", flagValue)
		}
		
		path := flagValue[:bracketStart]
		indexStr := flagValue[bracketStart+1 : bracketEnd]
		value := flagValue[equalPos+1:]
		
		index, err := strconv.Atoi(indexStr)
		if err != nil {
			return "", nil, -1, fmt.Errorf("invalid array index '%s': %w", indexStr, err)
		}
		
		return path, h.parseValue(value), index, nil
		
	default:
		return "", nil, -1, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// parseValue attempts to parse a string value into appropriate type
func (h *ArrayOperationFlagHandler) parseValue(value string) interface{} {
	// Try to parse as number
	if intVal, err := strconv.Atoi(value); err == nil {
		return intVal
	}
	
	// Try to parse as boolean
	if boolVal, err := strconv.ParseBool(value); err == nil {
		return boolVal
	}
	
	// Return as string
	return value
}

// valuesEqual compares two values for equality
func (h *ArrayOperationFlagHandler) valuesEqual(a, b interface{}) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}
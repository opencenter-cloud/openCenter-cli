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
	"reflect"
	"strconv"
	"strings"
)

// ReflectionEngine handles struct field manipulation with enhanced path support
type ReflectionEngine interface {
	// SetField sets a field using enhanced path syntax
	SetField(obj interface{}, path string, value interface{}) error
	
	// SetFieldWithStructuredPath sets a field using a pre-parsed structured path
	SetFieldWithStructuredPath(obj interface{}, structuredPath *StructuredPath, value interface{}) error
	
	// GetField retrieves a field value using path syntax
	GetField(obj interface{}, path string) (interface{}, error)
	
	// ExpandArray automatically expands arrays to accommodate indices
	ExpandArray(obj interface{}, path string, index int) error
	
	// SupportedSyntax returns supported path syntax patterns
	SupportedSyntax() []string
}

// EnhancedReflectionEngine implements ReflectionEngine with support for all Go types
type EnhancedReflectionEngine struct {
	pathParser PathParser
}

// NewEnhancedReflectionEngine creates a new enhanced reflection engine
func NewEnhancedReflectionEngine() *EnhancedReflectionEngine {
	return &EnhancedReflectionEngine{
		pathParser: NewEnhancedPathParser(),
	}
}

// SetField sets a field using enhanced path syntax
func (e *EnhancedReflectionEngine) SetField(obj interface{}, path string, value interface{}) error {
	structuredPath, err := e.pathParser.ParsePath(path)
	if err != nil {
		return fmt.Errorf("failed to parse path '%s': %w", path, err)
	}
	
	return e.SetFieldWithStructuredPath(obj, structuredPath, value)
}

// SetFieldWithStructuredPath sets a field using a pre-parsed structured path
func (e *EnhancedReflectionEngine) SetFieldWithStructuredPath(obj interface{}, structuredPath *StructuredPath, value interface{}) error {
	if obj == nil {
		return fmt.Errorf("cannot set field on nil object")
	}
	
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("object must be a pointer to be settable")
	}
	
	v = v.Elem()
	return e.setFieldRecursive(v, structuredPath.Parts, 0, value)
}

// setFieldRecursive recursively traverses the path parts and sets the final value
func (e *EnhancedReflectionEngine) setFieldRecursive(v reflect.Value, parts []PathPart, partIndex int, value interface{}) error {
	if partIndex >= len(parts) {
		return fmt.Errorf("path index out of bounds")
	}
	
	part := parts[partIndex]
	isLastPart := partIndex == len(parts)-1
	
	// Handle different part types
	if part.Name != "" {
		// This is a named field
		return e.handleNamedField(v, part, parts, partIndex, isLastPart, value)
	} else if part.HasIndex {
		// This is a numeric index (from dot syntax like field.0.subfield)
		return e.handleIndexAccess(v, part, parts, partIndex, isLastPart, value)
	} else {
		return fmt.Errorf("invalid path part: no name and no index")
	}
}

// handleNamedField handles a named field (with or without array index)
func (e *EnhancedReflectionEngine) handleNamedField(v reflect.Value, part PathPart, parts []PathPart, partIndex int, isLastPart bool, value interface{}) error {
	field := e.findField(v, part.Name)
	if !field.IsValid() {
		// Check if this is a map
		if v.Kind() == reflect.Map {
			return e.handleMapField(v, part, parts, partIndex, isLastPart, value)
		}
		return fmt.Errorf("field '%s' not found in struct '%s'", part.Name, v.Type().Name())
	}
	
	// If this part has an array index, handle array access
	if part.HasIndex {
		return e.handleFieldWithArrayIndex(field, part, parts, partIndex, isLastPart, value)
	}
	
	// No array index, handle as regular field
	if isLastPart {
		return e.setFieldValue(field, value)
	}
	
	// Not the last part, need to traverse deeper
	return e.traverseField(field, parts, partIndex+1, value)
}

// handleFieldWithArrayIndex handles a field that has an array index (bracket syntax)
func (e *EnhancedReflectionEngine) handleFieldWithArrayIndex(field reflect.Value, part PathPart, parts []PathPart, partIndex int, isLastPart bool, value interface{}) error {
	// Ensure the field is a slice or array
	if field.Kind() != reflect.Slice && field.Kind() != reflect.Array {
		return fmt.Errorf("field '%s' is not a slice or array, cannot use index access", part.Name)
	}
	
	// Expand slice if necessary
	if err := e.expandSliceToIndex(field, part.Index); err != nil {
		return fmt.Errorf("failed to expand slice for field '%s': %w", part.Name, err)
	}
	
	// Get the element at the specified index
	element := field.Index(part.Index)
	
	if isLastPart {
		return e.setFieldValue(element, value)
	}
	
	// Continue traversing with the array element
	return e.setFieldRecursive(element, parts, partIndex+1, value)
}

// handleIndexAccess handles pure index access (from dot syntax like field.0)
func (e *EnhancedReflectionEngine) handleIndexAccess(v reflect.Value, part PathPart, parts []PathPart, partIndex int, isLastPart bool, value interface{}) error {
	// The current value should be a slice or array
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return fmt.Errorf("cannot use index access on non-slice/array type %s", v.Type())
	}
	
	// Expand slice if necessary
	if err := e.expandSliceToIndex(v, part.Index); err != nil {
		return fmt.Errorf("failed to expand slice for index access: %w", err)
	}
	
	// Get the element at the specified index
	element := v.Index(part.Index)
	
	if isLastPart {
		return e.setFieldValue(element, value)
	}
	
	// Continue traversing with the array element
	return e.setFieldRecursive(element, parts, partIndex+1, value)
}

// handleMapField handles setting values in maps
func (e *EnhancedReflectionEngine) handleMapField(v reflect.Value, part PathPart, parts []PathPart, partIndex int, isLastPart bool, value interface{}) error {
	if v.Type().Key().Kind() != reflect.String {
		return fmt.Errorf("map key type must be string for path-based setting, got %s", v.Type().Key().Kind())
	}
	
	if isLastPart {
		// Set the map value directly
		mapValue := reflect.New(v.Type().Elem()).Elem()
		if err := e.setValueFromInterface(mapValue, value); err != nil {
			return fmt.Errorf("failed to set map value for key '%s': %w", part.Name, err)
		}
		v.SetMapIndex(reflect.ValueOf(part.Name), mapValue)
		return nil
	}
	
	// Get or create the nested map/struct
	existing := v.MapIndex(reflect.ValueOf(part.Name))
	if !existing.IsValid() {
		// Create new value
		newValue := reflect.New(v.Type().Elem()).Elem()
		v.SetMapIndex(reflect.ValueOf(part.Name), newValue)
		existing = v.MapIndex(reflect.ValueOf(part.Name))
	}
	
	return e.setFieldRecursive(existing, parts, partIndex+1, value)
}

// traverseField traverses into a field for continued path processing
func (e *EnhancedReflectionEngine) traverseField(field reflect.Value, parts []PathPart, nextPartIndex int, value interface{}) error {
	switch field.Kind() {
	case reflect.Struct:
		return e.setFieldRecursive(field, parts, nextPartIndex, value)
	case reflect.Ptr:
		if field.Type().Elem().Kind() == reflect.Struct {
			if field.IsNil() {
				field.Set(reflect.New(field.Type().Elem()))
			}
			return e.setFieldRecursive(field.Elem(), parts, nextPartIndex, value)
		}
		return fmt.Errorf("pointer field does not point to a struct")
	case reflect.Map:
		if field.IsNil() {
			field.Set(reflect.MakeMap(field.Type()))
		}
		return e.setFieldRecursive(field, parts, nextPartIndex, value)
	case reflect.Slice:
		return e.setFieldRecursive(field, parts, nextPartIndex, value)
	default:
		return fmt.Errorf("cannot traverse into field of type %s", field.Type())
	}
}

// expandSliceToIndex expands a slice to ensure it has at least index+1 elements
func (e *EnhancedReflectionEngine) expandSliceToIndex(slice reflect.Value, index int) error {
	if slice.Kind() != reflect.Slice {
		return fmt.Errorf("cannot expand non-slice type %s", slice.Type())
	}
	
	currentLen := slice.Len()
	requiredLen := index + 1
	
	if currentLen >= requiredLen {
		return nil // Already large enough
	}
	
	// Create new slice with required capacity
	elemType := slice.Type().Elem()
	newSlice := reflect.MakeSlice(slice.Type(), requiredLen, requiredLen)
	
	// Copy existing elements
	reflect.Copy(newSlice, slice)
	
	// Initialize new elements with zero values
	for i := currentLen; i < requiredLen; i++ {
		newSlice.Index(i).Set(reflect.Zero(elemType))
	}
	
	// Set the expanded slice back
	slice.Set(newSlice)
	return nil
}

// findField finds a field by yaml tag or name
func (e *EnhancedReflectionEngine) findField(v reflect.Value, name string) reflect.Value {
	if v.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		
		// Check yaml tag first
		yamlTag := field.Tag.Get("yaml")
		if yamlTag != "" {
			yamlName := strings.Split(yamlTag, ",")[0]
			if yamlName == name {
				return v.Field(i)
			}
		}
		
		// Check field name
		if field.Name == name {
			return v.Field(i)
		}
	}
	
	return reflect.Value{}
}

// setFieldValue sets a reflect.Value from an interface{} value
func (e *EnhancedReflectionEngine) setFieldValue(field reflect.Value, value interface{}) error {
	return e.setValueFromInterface(field, value)
}

// setValueFromInterface sets a reflect.Value from an interface{} value with type conversion
func (e *EnhancedReflectionEngine) setValueFromInterface(field reflect.Value, value interface{}) error {
	if !field.CanSet() {
		return fmt.Errorf("cannot set field value")
	}
	
	// For interface{} fields with string values, always use setReflectValue for type conversion
	if field.Kind() == reflect.Interface && field.Type().NumMethod() == 0 {
		if strValue, ok := value.(string); ok {
			return e.setReflectValue(field, strValue)
		}
	}
	
	// If value is already the correct type, set it directly
	valueReflect := reflect.ValueOf(value)
	if valueReflect.Type().AssignableTo(field.Type()) {
		field.Set(valueReflect)
		return nil
	}
	
	// Try to convert string values to appropriate types
	if strValue, ok := value.(string); ok {
		return e.setReflectValue(field, strValue)
	}
	
	// Try to convert the value
	if valueReflect.Type().ConvertibleTo(field.Type()) {
		field.Set(valueReflect.Convert(field.Type()))
		return nil
	}
	
	// As a last resort, convert to string and try string conversion
	return e.setReflectValue(field, fmt.Sprintf("%v", value))
}

// setReflectValue converts string value to the field's type and sets it
func (e *EnhancedReflectionEngine) setReflectValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer value: '%s'", value)
		}
		field.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		i, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid unsigned integer value: '%s'", value)
		}
		field.SetUint(i)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid float value: '%s'", value)
		}
		field.SetFloat(f)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value: '%s'", value)
		}
		field.SetBool(b)
	case reflect.Interface:
		// Handle interface{} types by storing the appropriately converted value
		// Try int first, then float, then bool, then string
		if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			field.Set(reflect.ValueOf(i))
		} else if f, err := strconv.ParseFloat(value, 64); err == nil {
			field.Set(reflect.ValueOf(f))
		} else if b, err := strconv.ParseBool(value); err == nil {
			field.Set(reflect.ValueOf(b))
		} else {
			field.Set(reflect.ValueOf(value))
		}
	default:
		return fmt.Errorf("unsupported field type: %s", field.Type())
	}
	return nil
}

// GetField retrieves a field value using path syntax
func (e *EnhancedReflectionEngine) GetField(obj interface{}, path string) (interface{}, error) {
	structuredPath, err := e.pathParser.ParsePath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path '%s': %w", path, err)
	}
	
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	
	return e.getFieldRecursive(v, structuredPath.Parts, 0)
}

// getFieldRecursive recursively traverses the path parts and gets the final value
func (e *EnhancedReflectionEngine) getFieldRecursive(v reflect.Value, parts []PathPart, partIndex int) (interface{}, error) {
	if partIndex >= len(parts) {
		return v.Interface(), nil
	}
	
	part := parts[partIndex]
	isLastPart := partIndex == len(parts)-1
	
	if part.Name != "" {
		field := e.findField(v, part.Name)
		if !field.IsValid() {
			if v.Kind() == reflect.Map {
				mapValue := v.MapIndex(reflect.ValueOf(part.Name))
				if !mapValue.IsValid() {
					return nil, fmt.Errorf("map key '%s' not found", part.Name)
				}
				if isLastPart {
					return mapValue.Interface(), nil
				}
				return e.getFieldRecursive(mapValue, parts, partIndex+1)
			}
			return nil, fmt.Errorf("field '%s' not found", part.Name)
		}
		
		if part.HasIndex {
			if field.Kind() != reflect.Slice && field.Kind() != reflect.Array {
				return nil, fmt.Errorf("field '%s' is not a slice or array", part.Name)
			}
			if part.Index >= field.Len() {
				return nil, fmt.Errorf("index %d out of bounds for slice of length %d", part.Index, field.Len())
			}
			element := field.Index(part.Index)
			if isLastPart {
				return element.Interface(), nil
			}
			return e.getFieldRecursive(element, parts, partIndex+1)
		}
		
		if isLastPart {
			return field.Interface(), nil
		}
		return e.getFieldRecursive(field, parts, partIndex+1)
	} else if part.HasIndex {
		if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
			return nil, fmt.Errorf("cannot use index access on non-slice/array type")
		}
		if part.Index >= v.Len() {
			return nil, fmt.Errorf("index %d out of bounds for slice of length %d", part.Index, v.Len())
		}
		element := v.Index(part.Index)
		if isLastPart {
			return element.Interface(), nil
		}
		return e.getFieldRecursive(element, parts, partIndex+1)
	}
	
	return nil, fmt.Errorf("invalid path part")
}

// ExpandArray automatically expands arrays to accommodate indices
func (e *EnhancedReflectionEngine) ExpandArray(obj interface{}, path string, index int) error {
	structuredPath, err := e.pathParser.ParsePath(path)
	if err != nil {
		return fmt.Errorf("failed to parse path '%s': %w", path, err)
	}
	
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("object must be a pointer to be settable")
	}
	v = v.Elem()
	
	// Navigate to the array field
	for i, part := range structuredPath.Parts[:len(structuredPath.Parts)-1] {
		if part.Name != "" {
			field := e.findField(v, part.Name)
			if !field.IsValid() {
				return fmt.Errorf("field '%s' not found", part.Name)
			}
			
			if part.HasIndex {
				if field.Kind() != reflect.Slice {
					return fmt.Errorf("field '%s' is not a slice", part.Name)
				}
				if err := e.expandSliceToIndex(field, part.Index); err != nil {
					return err
				}
				v = field.Index(part.Index)
			} else {
				v = field
			}
		} else if part.HasIndex {
			if v.Kind() != reflect.Slice {
				return fmt.Errorf("cannot use index access on non-slice type at part %d", i)
			}
			if err := e.expandSliceToIndex(v, part.Index); err != nil {
				return err
			}
			v = v.Index(part.Index)
		}
	}
	
	// Handle the final part
	lastPart := structuredPath.Parts[len(structuredPath.Parts)-1]
	if lastPart.Name != "" {
		field := e.findField(v, lastPart.Name)
		if !field.IsValid() {
			return fmt.Errorf("field '%s' not found", lastPart.Name)
		}
		if field.Kind() != reflect.Slice {
			return fmt.Errorf("field '%s' is not a slice", lastPart.Name)
		}
		return e.expandSliceToIndex(field, index)
	}
	
	// The current value should be a slice
	if v.Kind() != reflect.Slice {
		return fmt.Errorf("cannot expand non-slice type")
	}
	return e.expandSliceToIndex(v, index)
}

// SupportedSyntax returns supported path syntax patterns
func (e *EnhancedReflectionEngine) SupportedSyntax() []string {
	return e.pathParser.SupportedSyntax()
}
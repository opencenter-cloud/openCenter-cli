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

package v2

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// ReferenceResolver resolves ${ref:path.to.value}, ${env:VAR}, and ${file:path} references
// in v2 configuration structures.
// Requirements: 4.2 (Epic 4: Medium Priority TODO Resolution)
type ReferenceResolver struct {
	cache            map[string]interface{}
	referencePattern *regexp.Regexp
	envPattern       *regexp.Regexp
	filePattern      *regexp.Regexp
	visited          map[string]bool // For circular reference detection
	maxDepth         int             // Maximum recursion depth
	fileSystem       fs.FileSystem
	root             reflect.Value
}

// NewReferenceResolver creates a new reference resolver with caching support.
func NewReferenceResolver() *ReferenceResolver {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	return &ReferenceResolver{
		cache:            make(map[string]interface{}),
		referencePattern: regexp.MustCompile(`\$\{ref:([^}]+)\}`),
		envPattern:       regexp.MustCompile(`\$\{env:([^}]+)\}`),
		filePattern:      regexp.MustCompile(`\$\{file:([^}]+)\}`),
		visited:          make(map[string]bool),
		maxDepth:         10, // Prevent infinite recursion
		fileSystem:       fileSystem,
	}
}

// Resolve resolves all references in the configuration using reflection.
// It works with any struct type, making it compatible with v2.Config.
// Requirements: 4.2.2
func (r *ReferenceResolver) Resolve(cfg interface{}) error {
	if cfg == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	// Reset visited map for each resolution
	r.visited = make(map[string]bool)

	// Start resolving from the root
	v := reflect.ValueOf(cfg)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	r.root = v

	return r.resolveValue(v, "", 0)
}

// resolveValue recursively resolves references in different value types.
// Requirements: 4.2.3
func (r *ReferenceResolver) resolveValue(v reflect.Value, path string, depth int) error {
	// Check recursion depth
	if depth > r.maxDepth {
		return fmt.Errorf("maximum recursion depth exceeded at path '%s'", path)
	}

	// Handle pointers
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	// Handle different types
	switch v.Kind() {
	case reflect.String:
		return r.resolveStringValue(v, path)

	case reflect.Struct:
		return r.resolveStructValue(v, path, depth)

	case reflect.Map:
		return r.resolveMapValue(v, path, depth)

	case reflect.Slice, reflect.Array:
		return r.resolveSliceValue(v, path, depth)

	case reflect.Interface:
		if !v.IsNil() {
			return r.resolveValue(v.Elem(), path, depth)
		}
	}

	return nil
}

// resolveStringValue resolves references in a string value.
func (r *ReferenceResolver) resolveStringValue(v reflect.Value, path string) error {
	if !v.CanSet() {
		return nil
	}

	str := v.String()
	if str == "" {
		return nil
	}

	// Check for any reference patterns
	hasRef := r.referencePattern.MatchString(str) ||
		r.envPattern.MatchString(str) ||
		r.filePattern.MatchString(str)

	if !hasRef {
		return nil
	}

	// Resolve the string
	resolved, err := r.resolveReference(str, path)
	if err != nil {
		return err
	}

	v.SetString(resolved)
	return nil
}

// resolveStructValue resolves references in struct fields.
func (r *ReferenceResolver) resolveStructValue(v reflect.Value, path string, depth int) error {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		// Get field name from yaml tag or use field name
		fieldName := fieldType.Name
		if tag := fieldType.Tag.Get("yaml"); tag != "" {
			parts := strings.Split(tag, ",")
			if parts[0] != "" && parts[0] != "-" {
				fieldName = parts[0]
			}
		}

		// Build field path
		fieldPath := fieldName
		if path != "" {
			fieldPath = path + "." + fieldName
		}

		// Recursively resolve the field
		if err := r.resolveValue(field, fieldPath, depth+1); err != nil {
			return err
		}
	}

	return nil
}

// resolveMapValue resolves references in map values.
func (r *ReferenceResolver) resolveMapValue(v reflect.Value, path string, depth int) error {
	if v.IsNil() {
		return nil
	}

	// Create a new map to store resolved values
	iter := v.MapRange()
	for iter.Next() {
		key := iter.Key()
		val := iter.Value()

		keyStr := fmt.Sprintf("%v", key.Interface())
		mapPath := path + "." + keyStr
		if path == "" {
			mapPath = keyStr
		}

		// Handle interface{} values that might contain strings
		if val.Kind() == reflect.Interface && !val.IsNil() {
			val = val.Elem()
		}

		// For string values, check for references
		if val.Kind() == reflect.String {
			str := val.String()
			if r.referencePattern.MatchString(str) ||
				r.envPattern.MatchString(str) ||
				r.filePattern.MatchString(str) {

				resolved, err := r.resolveReference(str, mapPath)
				if err != nil {
					return err
				}

				// Set the resolved value back in the map
				v.SetMapIndex(key, reflect.ValueOf(resolved))
				continue
			}
		}

		// For non-string values or strings without references, recurse
		if err := r.resolveValue(val, mapPath, depth+1); err != nil {
			return err
		}
	}

	return nil
}

// resolveSliceValue resolves references in slice/array elements.
func (r *ReferenceResolver) resolveSliceValue(v reflect.Value, path string, depth int) error {
	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		elemPath := fmt.Sprintf("%s[%d]", path, i)

		if err := r.resolveValue(elem, elemPath, depth+1); err != nil {
			return err
		}
	}

	return nil
}

// resolveReference resolves a single reference string that may contain multiple reference types.
// Supports ${ref:path}, ${env:VAR}, and ${file:path} syntax.
// Requirements: 4.2.4
func (r *ReferenceResolver) resolveReference(str string, currentPath string) (string, error) {
	result := str
	addedCurrent := false
	if currentPath != "" && !r.visited[currentPath] {
		r.visited[currentPath] = true
		addedCurrent = true
	}
	if addedCurrent {
		defer delete(r.visited, currentPath)
	}

	// Resolve ${ref:} references
	refMatches := r.referencePattern.FindAllStringSubmatch(result, -1)
	for _, match := range refMatches {
		if len(match) > 1 {
			refPath := match[1]

			// Check for circular references
			if r.visited[refPath] {
				return "", fmt.Errorf("circular reference detected: ${ref:%s} at path '%s'", refPath, currentPath)
			}

			// Check cache first
			if cached, ok := r.cache[refPath]; ok {
				result = strings.Replace(result, match[0], fmt.Sprint(cached), 1)
				continue
			}

			// Mark as visited
			r.visited[refPath] = true
			resolvedValue, err := r.lookupReferenceValue(refPath)
			delete(r.visited, refPath)
			if err != nil {
				return "", err
			}

			// Cache resolved reference values for repeated lookups.
			r.cache[refPath] = resolvedValue
			result = strings.Replace(result, match[0], fmt.Sprint(resolvedValue), 1)
		}
	}

	// Resolve ${env:} references
	envMatches := r.envPattern.FindAllStringSubmatch(result, -1)
	for _, match := range envMatches {
		if len(match) > 1 {
			envVar := match[1]
			envValue := os.Getenv(envVar)
			if envValue == "" {
				return "", fmt.Errorf("environment variable ${env:%s} is not set or empty", envVar)
			}

			// Cache the environment variable value
			r.cache["env:"+envVar] = envValue

			result = strings.Replace(result, match[0], envValue, 1)
		}
	}

	// Resolve ${file:} references
	fileMatches := r.filePattern.FindAllStringSubmatch(result, -1)
	for _, match := range fileMatches {
		if len(match) > 1 {
			filePath := match[1]

			// Check cache first
			cacheKey := "file:" + filePath
			if cached, ok := r.cache[cacheKey]; ok {
				result = strings.Replace(result, match[0], fmt.Sprint(cached), 1)
				continue
			}

			// Read file content
			content, err := r.fileSystem.ReadFile(filePath)
			if err != nil {
				return "", fmt.Errorf("failed to read file ${file:%s}: %w", filePath, err)
			}

			fileContent := strings.TrimSpace(string(content))

			// Cache the file content
			r.cache[cacheKey] = fileContent

			result = strings.Replace(result, match[0], fileContent, 1)
		}
	}

	return result, nil
}

func (r *ReferenceResolver) lookupReferenceValue(refPath string) (interface{}, error) {
	value, err := r.lookupPathValue(r.root, refPath)
	if err != nil {
		return nil, fmt.Errorf("reference ${ref:%s} cannot be resolved: %w", refPath, err)
	}

	value = dereferenceValue(value)
	if !value.IsValid() {
		return nil, fmt.Errorf("reference ${ref:%s} cannot be resolved: value is nil", refPath)
	}

	if value.Kind() == reflect.String {
		str := value.String()
		if r.referencePattern.MatchString(str) || r.envPattern.MatchString(str) || r.filePattern.MatchString(str) {
			return r.resolveReference(str, refPath)
		}
	}

	return value.Interface(), nil
}

func (r *ReferenceResolver) lookupPathValue(root reflect.Value, path string) (reflect.Value, error) {
	current := dereferenceValue(root)
	if !current.IsValid() {
		return reflect.Value{}, fmt.Errorf("root configuration is nil")
	}

	segments := strings.Split(path, ".")
	for _, segment := range segments {
		name, indices, err := parseReferenceSegment(segment)
		if err != nil {
			return reflect.Value{}, err
		}

		if name != "" {
			current, err = resolveReferenceSegment(current, name)
			if err != nil {
				return reflect.Value{}, err
			}
		}

		for _, index := range indices {
			current = dereferenceValue(current)
			if !current.IsValid() {
				return reflect.Value{}, fmt.Errorf("path %q resolved to nil before index %d", path, index)
			}
			if current.Kind() != reflect.Slice && current.Kind() != reflect.Array {
				return reflect.Value{}, fmt.Errorf("segment %q is not indexable", segment)
			}
			if index < 0 || index >= current.Len() {
				return reflect.Value{}, fmt.Errorf("index %d out of range for segment %q", index, segment)
			}
			current = current.Index(index)
		}
	}

	return current, nil
}

func dereferenceValue(v reflect.Value) reflect.Value {
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer) {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

func resolveReferenceSegment(v reflect.Value, segment string) (reflect.Value, error) {
	v = dereferenceValue(v)
	if !v.IsValid() {
		return reflect.Value{}, fmt.Errorf("segment %q resolved to nil", segment)
	}

	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			fieldType := t.Field(i)
			fieldName := fieldType.Name
			if tag := fieldType.Tag.Get("yaml"); tag != "" {
				parts := strings.Split(tag, ",")
				if parts[0] != "" && parts[0] != "-" {
					fieldName = parts[0]
				}
			}
			if fieldName == segment || strings.EqualFold(fieldType.Name, segment) {
				return v.Field(i), nil
			}
		}
		return reflect.Value{}, fmt.Errorf("unknown struct field %q", segment)
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return reflect.Value{}, fmt.Errorf("map segment %q requires string keys", segment)
		}
		resolved := v.MapIndex(reflect.ValueOf(segment))
		if !resolved.IsValid() {
			return reflect.Value{}, fmt.Errorf("unknown map key %q", segment)
		}
		return resolved, nil
	default:
		return reflect.Value{}, fmt.Errorf("segment %q cannot be resolved from %s", segment, v.Kind())
	}
}

func parseReferenceSegment(segment string) (string, []int, error) {
	name := segment
	var indices []int

	for {
		open := strings.Index(name, "[")
		if open == -1 {
			break
		}
		close := strings.Index(name[open:], "]")
		if close == -1 {
			return "", nil, fmt.Errorf("invalid indexed reference segment %q", segment)
		}
		close += open

		indexValue := name[open+1 : close]
		var index int
		if _, err := fmt.Sscanf(indexValue, "%d", &index); err != nil {
			return "", nil, fmt.Errorf("invalid index %q in segment %q", indexValue, segment)
		}
		indices = append(indices, index)
		name = name[:open] + name[close+1:]
	}

	return name, indices, nil
}

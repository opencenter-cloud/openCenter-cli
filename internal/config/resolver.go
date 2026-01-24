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

package config

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// ReferenceResolver resolves ${path.to.value} references in configuration.
type ReferenceResolver interface {
	// Resolve resolves all references in the configuration
	Resolve(cfg *Config) error
	
	// BuildDependencyGraph builds a dependency graph from the configuration
	BuildDependencyGraph(cfg *Config) (*DependencyGraph, error)
	
	// DetectCycles detects circular dependencies in the graph
	DetectCycles(graph *DependencyGraph) error
}

// DependencyGraph represents a directed graph of configuration dependencies.
type DependencyGraph struct {
	Nodes map[string]*Node
	Edges map[string][]string
}

// Node represents a node in the dependency graph.
type Node struct {
	Path  string
	Value interface{}
}

// referenceResolver implements the ReferenceResolver interface.
type referenceResolver struct {
	referencePattern *regexp.Regexp
}

// NewReferenceResolver creates a new reference resolver.
func NewReferenceResolver() ReferenceResolver {
	return &referenceResolver{
		referencePattern: regexp.MustCompile(`\$\{([^}]+)\}`),
	}
}

// Resolve resolves all references in the configuration.
func (r *referenceResolver) Resolve(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	// Build dependency graph
	graph, err := r.BuildDependencyGraph(cfg)
	if err != nil {
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}

	// Detect cycles
	if err := r.DetectCycles(graph); err != nil {
		return err
	}

	// Get resolution order via topological sort
	order, err := r.topologicalSort(graph)
	if err != nil {
		return fmt.Errorf("failed to determine resolution order: %w", err)
	}

	// Resolve references in order
	for _, path := range order {
		if err := r.resolveNodeReferences(cfg, path); err != nil {
			return fmt.Errorf("failed to resolve references for path '%s': %w", path, err)
		}
	}

	return nil
}

// BuildDependencyGraph builds a dependency graph from the configuration.
func (r *referenceResolver) BuildDependencyGraph(cfg *Config) (*DependencyGraph, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	graph := &DependencyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string][]string),
	}

	// Walk the configuration struct to find all references
	if err := r.walkStruct(reflect.ValueOf(cfg), "", graph); err != nil {
		return nil, err
	}

	return graph, nil
}

// DetectCycles detects circular dependencies in the graph.
func (r *referenceResolver) DetectCycles(graph *DependencyGraph) error {
	if graph == nil {
		return fmt.Errorf("dependency graph cannot be nil")
	}

	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for node := range graph.Nodes {
		if !visited[node] {
			if cycle := r.detectCycleUtil(node, visited, recStack, graph, []string{}); cycle != nil {
				return fmt.Errorf("circular reference detected: %s", strings.Join(cycle, " -> "))
			}
		}
	}

	return nil
}

// detectCycleUtil is a utility function for cycle detection using DFS.
func (r *referenceResolver) detectCycleUtil(node string, visited, recStack map[string]bool, graph *DependencyGraph, path []string) []string {
	visited[node] = true
	recStack[node] = true
	path = append(path, node)

	// Check all neighbors
	for _, neighbor := range graph.Edges[node] {
		if !visited[neighbor] {
			if cycle := r.detectCycleUtil(neighbor, visited, recStack, graph, path); cycle != nil {
				return cycle
			}
		} else if recStack[neighbor] {
			// Found a cycle - return the cycle path
			cycleStart := 0
			for i, p := range path {
				if p == neighbor {
					cycleStart = i
					break
				}
			}
			return append(path[cycleStart:], neighbor)
		}
	}

	recStack[node] = false
	return nil
}

// topologicalSort performs a topological sort on the dependency graph.
func (r *referenceResolver) topologicalSort(graph *DependencyGraph) ([]string, error) {
	visited := make(map[string]bool)
	stack := []string{}

	var visit func(string) error
	visit = func(node string) error {
		if visited[node] {
			return nil
		}
		visited[node] = true

		// Visit all dependencies first
		for _, dep := range graph.Edges[node] {
			if err := visit(dep); err != nil {
				return err
			}
		}

		stack = append(stack, node)
		return nil
	}

	// Visit all nodes
	for node := range graph.Nodes {
		if err := visit(node); err != nil {
			return nil, err
		}
	}

	// The stack is already in the correct order (dependencies before dependents)
	// No need to reverse
	return stack, nil
}

// walkStruct walks through a struct using reflection to find references.
func (r *referenceResolver) walkStruct(v reflect.Value, path string, graph *DependencyGraph) error {
	// Handle pointers
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
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

			// Recursively walk the field
			if err := r.walkStruct(field, fieldPath, graph); err != nil {
				return err
			}
		}

	case reflect.String:
		str := v.String()
		if r.referencePattern.MatchString(str) {
			// Found a reference - add to graph
			node := &Node{
				Path:  path,
				Value: str,
			}
			graph.Nodes[path] = node

			// Extract all references from this string
			matches := r.referencePattern.FindAllStringSubmatch(str, -1)
			for _, match := range matches {
				if len(match) > 1 {
					refPath := match[1]
					// Add edge from this node to the referenced node
					graph.Edges[path] = append(graph.Edges[path], refPath)
				}
			}
		}

	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i)
			elemPath := fmt.Sprintf("%s[%d]", path, i)
			if err := r.walkStruct(elem, elemPath, graph); err != nil {
				return err
			}
		}

	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			key := iter.Key()
			val := iter.Value()
			keyStr := fmt.Sprintf("%v", key.Interface())
			mapPath := fmt.Sprintf("%s.%s", path, keyStr)
			if err := r.walkStruct(val, mapPath, graph); err != nil {
				return err
			}
		}
	}

	return nil
}

// resolveNodeReferences resolves all references in a specific node.
func (r *referenceResolver) resolveNodeReferences(cfg *Config, path string) error {
	// Get the value at the path
	val, err := r.getValueAtPath(cfg, path)
	if err != nil {
		// If the path doesn't exist, it might not have been set yet, skip it
		return nil
	}

	// If it's a string with references, resolve them
	if strVal, ok := val.(string); ok {
		if r.referencePattern.MatchString(strVal) {
			resolved, err := r.resolveString(cfg, strVal)
			if err != nil {
				return err
			}

			// Set the resolved value back
			if err := r.setValueAtPath(cfg, path, resolved); err != nil {
				return err
			}
		}
	}

	return nil
}

// resolveString resolves all references in a string.
func (r *referenceResolver) resolveString(cfg *Config, str string) (string, error) {
	result := str

	matches := r.referencePattern.FindAllStringSubmatch(str, -1)
	for _, match := range matches {
		if len(match) > 1 {
			refPath := match[1]
			refValue, err := r.getValueAtPath(cfg, refPath)
			if err != nil {
				return "", fmt.Errorf("reference ${%s} not found: %w", refPath, err)
			}

			// Convert value to string
			refStr := fmt.Sprintf("%v", refValue)
			
			// If the referenced value still contains references, resolve them recursively
			if r.referencePattern.MatchString(refStr) {
				refStr, err = r.resolveString(cfg, refStr)
				if err != nil {
					return "", fmt.Errorf("failed to resolve nested reference in ${%s}: %w", refPath, err)
				}
			}

			// Replace the reference with the actual value
			result = strings.Replace(result, match[0], refStr, 1)
		}
	}

	return result, nil
}

// getValueAtPath retrieves a value from the configuration using a dot-separated path.
func (r *referenceResolver) getValueAtPath(cfg *Config, path string) (interface{}, error) {
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	parts := strings.Split(path, ".")
	v := reflect.ValueOf(cfg)

	for i, part := range parts {
		// Handle pointers
		if v.Kind() == reflect.Ptr {
			if v.IsNil() {
				return nil, fmt.Errorf("nil pointer at path segment '%s'", strings.Join(parts[:i+1], "."))
			}
			v = v.Elem()
		}

		// Handle array/slice indices
		if strings.Contains(part, "[") && strings.Contains(part, "]") {
			// Parse array access like "field[0]"
			fieldName := part[:strings.Index(part, "[")]
			indexStr := part[strings.Index(part, "[")+1 : strings.Index(part, "]")]
			
			// Get the field
			v = r.getFieldByName(v, fieldName)
			if !v.IsValid() {
				return nil, fmt.Errorf("field '%s' not found at path '%s'", fieldName, strings.Join(parts[:i+1], "."))
			}

			// Parse index
			var index int
			if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
				return nil, fmt.Errorf("invalid array index '%s' at path '%s'", indexStr, strings.Join(parts[:i+1], "."))
			}

			// Access array element
			if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
				return nil, fmt.Errorf("field '%s' is not an array at path '%s'", fieldName, strings.Join(parts[:i+1], "."))
			}
			if index < 0 || index >= v.Len() {
				return nil, fmt.Errorf("array index %d out of bounds at path '%s'", index, strings.Join(parts[:i+1], "."))
			}
			v = v.Index(index)
		} else {
			// Regular field access
			v = r.getFieldByName(v, part)
			if !v.IsValid() {
				return nil, fmt.Errorf("field '%s' not found at path '%s'", part, strings.Join(parts[:i+1], "."))
			}
		}
	}

	if !v.CanInterface() {
		return nil, fmt.Errorf("cannot access value at path '%s'", path)
	}

	return v.Interface(), nil
}

// setValueAtPath sets a value in the configuration using a dot-separated path.
func (r *referenceResolver) setValueAtPath(cfg *Config, path string, value interface{}) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	parts := strings.Split(path, ".")
	v := reflect.ValueOf(cfg)

	// Navigate to the parent of the target field
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]

		// Handle pointers
		if v.Kind() == reflect.Ptr {
			if v.IsNil() {
				return fmt.Errorf("nil pointer at path segment '%s'", strings.Join(parts[:i+1], "."))
			}
			v = v.Elem()
		}

		// Handle array/slice indices
		if strings.Contains(part, "[") && strings.Contains(part, "]") {
			fieldName := part[:strings.Index(part, "[")]
			indexStr := part[strings.Index(part, "[")+1 : strings.Index(part, "]")]
			
			v = r.getFieldByName(v, fieldName)
			if !v.IsValid() {
				return fmt.Errorf("field '%s' not found at path '%s'", fieldName, strings.Join(parts[:i+1], "."))
			}

			var index int
			if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
				return fmt.Errorf("invalid array index '%s' at path '%s'", indexStr, strings.Join(parts[:i+1], "."))
			}

			if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
				return fmt.Errorf("field '%s' is not an array at path '%s'", fieldName, strings.Join(parts[:i+1], "."))
			}
			if index < 0 || index >= v.Len() {
				return fmt.Errorf("array index %d out of bounds at path '%s'", index, strings.Join(parts[:i+1], "."))
			}
			v = v.Index(index)
		} else {
			v = r.getFieldByName(v, part)
			if !v.IsValid() {
				return fmt.Errorf("field '%s' not found at path '%s'", part, strings.Join(parts[:i+1], "."))
			}
		}
	}

	// Handle pointers for the parent
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return fmt.Errorf("nil pointer at path '%s'", strings.Join(parts[:len(parts)-1], "."))
		}
		v = v.Elem()
	}

	// Get the final field
	lastPart := parts[len(parts)-1]
	field := r.getFieldByName(v, lastPart)
	if !field.IsValid() {
		return fmt.Errorf("field '%s' not found at path '%s'", lastPart, path)
	}

	if !field.CanSet() {
		return fmt.Errorf("cannot set field '%s' at path '%s'", lastPart, path)
	}

	// Set the value
	newValue := reflect.ValueOf(value)
	if !newValue.Type().AssignableTo(field.Type()) {
		return fmt.Errorf("cannot assign value of type %s to field of type %s at path '%s'", 
			newValue.Type(), field.Type(), path)
	}

	field.Set(newValue)
	return nil
}

// getFieldByName gets a struct field by name, checking both the field name and yaml tag.
func (r *referenceResolver) getFieldByName(v reflect.Value, name string) reflect.Value {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return reflect.Value{}
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)

		// Check field name
		if field.Name == name {
			return v.Field(i)
		}

		// Check yaml tag
		if tag := field.Tag.Get("yaml"); tag != "" {
			parts := strings.Split(tag, ",")
			if parts[0] == name {
				return v.Field(i)
			}
		}
	}

	return reflect.Value{}
}

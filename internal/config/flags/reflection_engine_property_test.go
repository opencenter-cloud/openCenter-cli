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
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: cli-configuration-enhancement, Property 8: Reflection engine type safety
// For any Go type (slice, map, pointer, struct), the reflection engine should handle 
// type-appropriate operations and automatic initialization correctly
// Validates: Requirements 3.1, 3.2, 3.3, 3.5
func TestProperty_ReflectionEngineTypeSafety(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("slice operations maintain type safety", prop.ForAll(
		func(index int, stringValue string, intValue int, boolValue bool) bool {
			engine := NewEnhancedReflectionEngine()
			config := &TestReflectionConfig{}
			
			// Test string slice
			stringPath := fmt.Sprintf("stringSlice[%d]", index)
			if err := engine.SetField(config, stringPath, stringValue); err != nil {
				return false
			}
			
			// Test int slice
			intPath := fmt.Sprintf("intSlice[%d]", index)
			if err := engine.SetField(config, intPath, fmt.Sprintf("%d", intValue)); err != nil {
				return false
			}
			
			// Test bool slice
			boolPath := fmt.Sprintf("boolSlice[%d]", index)
			if err := engine.SetField(config, boolPath, fmt.Sprintf("%t", boolValue)); err != nil {
				return false
			}
			
			// Verify slices were expanded correctly
			expectedLen := index + 1
			if len(config.StringSlice) != expectedLen || len(config.IntSlice) != expectedLen || len(config.BoolSlice) != expectedLen {
				return false
			}
			
			// Verify values were set with correct types
			if config.StringSlice[index] != stringValue {
				return false
			}
			if config.IntSlice[index] != intValue {
				return false
			}
			if config.BoolSlice[index] != boolValue {
				return false
			}
			
			// Verify other elements are zero-initialized with correct types
			for i := 0; i < index; i++ {
				if config.StringSlice[i] != "" || config.IntSlice[i] != 0 || config.BoolSlice[i] != false {
					return false
				}
			}
			
			return true
		},
		gen.IntRange(0, 5),    // index
		genReflectionString(), // string value
		gen.IntRange(-100, 100), // int value
		gen.Bool(),            // bool value
	))

	properties.Property("map operations handle automatic initialization", prop.ForAll(
		func(key string, value string) bool {
			engine := NewEnhancedReflectionEngine()
			config := &TestReflectionConfig{}
			
			// Set value in map (should auto-initialize map)
			path := fmt.Sprintf("stringMap.%s", key)
			if err := engine.SetField(config, path, value); err != nil {
				return false
			}
			
			// Verify map was initialized
			if config.StringMap == nil {
				return false
			}
			
			// Verify value was set correctly
			if config.StringMap[key] != value {
				return false
			}
			
			return true
		},
		genReflectionMapKey(),           // map key
		genReflectionString(), // map value
	))

	properties.Property("pointer fields are automatically initialized", prop.ForAll(
		func(value string) bool {
			engine := NewEnhancedReflectionEngine()
			config := &TestReflectionConfig{}
			
			// Set value in nested pointer struct (should auto-initialize pointer)
			path := fmt.Sprintf("nestedPtr.data")
			if err := engine.SetField(config, path, value); err != nil {
				return false
			}
			
			// Verify pointer was initialized
			if config.NestedPtr == nil {
				return false
			}
			
			// Verify value was set correctly
			if config.NestedPtr.Data != value {
				return false
			}
			
			return true
		},
		genReflectionString(), // value
	))

	properties.Property("nested struct operations preserve structure", prop.ForAll(
		func(name string, count int, enabled bool) bool {
			engine := NewEnhancedReflectionEngine()
			config := &TestReflectionConfig{}
			
			// Set values in nested struct
			if err := engine.SetField(config, "nested.name", name); err != nil {
				return false
			}
			if err := engine.SetField(config, "nested.count", fmt.Sprintf("%d", count)); err != nil {
				return false
			}
			if err := engine.SetField(config, "nested.enabled", fmt.Sprintf("%t", enabled)); err != nil {
				return false
			}
			
			// Verify all values were set correctly with proper types
			if config.Nested.Name != name {
				return false
			}
			if config.Nested.Count != count {
				return false
			}
			if config.Nested.Enabled != enabled {
				return false
			}
			
			return true
		},
		genReflectionString(), // name
		gen.IntRange(0, 100),  // count
		gen.Bool(),            // enabled
	))

	properties.Property("interface fields handle type conversion", prop.ForAll(
		func(stringValue string, intValue int, boolValue bool) bool {
			engine := NewEnhancedReflectionEngine()
			config := &TestReflectionConfig{}
			
			// Set different types in interface{} slice
			stringPath := fmt.Sprintf("interfaceSlice[0]")
			if err := engine.SetField(config, stringPath, stringValue); err != nil {
				return false
			}
			
			intPath := fmt.Sprintf("interfaceSlice[1]")
			if err := engine.SetField(config, intPath, fmt.Sprintf("%d", intValue)); err != nil {
				return false
			}
			
			boolPath := fmt.Sprintf("interfaceSlice[2]")
			if err := engine.SetField(config, boolPath, fmt.Sprintf("%t", boolValue)); err != nil {
				return false
			}
			
			// Verify slice was expanded
			if len(config.InterfaceSlice) != 3 {
				return false
			}
			
			// Verify values were set (interface{} fields should contain the converted values)
			// When setting from strings, interface{} fields use the automatic type conversion
			if config.InterfaceSlice[0] != stringValue {
				return false
			}
			
			// For interface{} fields, the setReflectValue method converts strings to appropriate types
			// Check what type was actually stored
			switch v := config.InterfaceSlice[1].(type) {
			case int64:
				if v != int64(intValue) {
					return false
				}
			case string:
				// If it remained a string, check if it matches the input
				if v != fmt.Sprintf("%d", intValue) {
					return false
				}
			default:
				return false
			}
			
			// Bool strings are converted to bool or remain as strings
			switch v := config.InterfaceSlice[2].(type) {
			case bool:
				if v != boolValue {
					return false
				}
			case string:
				// If it remained a string, check if it matches the input
				if v != fmt.Sprintf("%t", boolValue) {
					return false
				}
			default:
				return false
			}
			
			return true
		},
		genReflectionString(), // string value
		gen.IntRange(-100, 100), // int value
		gen.Bool(),            // bool value
	))

	properties.Property("complex nested operations maintain integrity", prop.ForAll(
		func(outerIndex int, innerKey string, value string) bool {
			engine := NewEnhancedReflectionEngine()
			config := &TestReflectionConfig{}
			
			// Set value in complex nested structure: slice of structs with maps
			path := fmt.Sprintf("complexNested[%d].data.%s", outerIndex, innerKey)
			if err := engine.SetField(config, path, value); err != nil {
				return false
			}
			
			// Verify structure was created correctly
			if len(config.ComplexNested) != outerIndex+1 {
				return false
			}
			
			// Verify the map was initialized
			if config.ComplexNested[outerIndex].Data == nil {
				return false
			}
			
			// Verify the value was set
			if config.ComplexNested[outerIndex].Data[innerKey] != value {
				return false
			}
			
			// Verify other elements are properly zero-initialized
			for i := 0; i < outerIndex; i++ {
				if config.ComplexNested[i].Data != nil {
					return false
				}
			}
			
			return true
		},
		gen.IntRange(0, 3),    // outer index
		genReflectionMapKey(),           // inner key
		genReflectionString(), // value
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Test structures for reflection engine property testing

type TestReflectionConfig struct {
	StringSlice     []string                 `yaml:"stringSlice"`
	IntSlice        []int                    `yaml:"intSlice"`
	BoolSlice       []bool                   `yaml:"boolSlice"`
	InterfaceSlice  []interface{}            `yaml:"interfaceSlice"`
	StringMap       map[string]string        `yaml:"stringMap"`
	Nested          TestNestedReflection     `yaml:"nested"`
	NestedPtr       *TestNestedReflection    `yaml:"nestedPtr"`
	ComplexNested   []TestComplexNested      `yaml:"complexNested"`
}

type TestNestedReflection struct {
	Name    string `yaml:"name"`
	Count   int    `yaml:"count"`
	Enabled bool   `yaml:"enabled"`
	Data    string `yaml:"data"`
}

type TestComplexNested struct {
	ID   int               `yaml:"id"`
	Data map[string]string `yaml:"data"`
}

// Generators for reflection engine property tests

func genReflectionString() gopter.Gen {
	return gen.OneConstOf(
		"test", "data", "value", "config", "item",
		"alpha", "beta", "gamma", "reflection-test",
		"property-value", "engine-test", "type-safe",
	)
}

func genReflectionMapKey() gopter.Gen {
	return gen.OneConstOf(
		"key1", "key2", "key3", "config", "data",
		"setting", "option", "param", "field",
		"test-key", "map-key", "property",
	)
}
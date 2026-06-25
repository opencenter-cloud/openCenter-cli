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

// Feature: cli-configuration-enhancement, Property 1: Array manipulation consistency
// For any configuration object, array path, and index, all array operations (set, append, insert, remove)
// should maintain array integrity and preserve non-affected elements
// Validates: Requirements 1.1, 1.2, 1.3, 1.4, 1.5, 8.1, 8.2, 8.3, 8.4, 8.5
func TestProperty_ArrayManipulationConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("array expansion maintains integrity", prop.ForAll(
		func(initialSize int, targetIndex int, value string) bool {
			engine := NewEnhancedReflectionEngine()
			config := &TestArrayConfig{}

			// Initialize array with some values
			for i := 0; i < initialSize; i++ {
				path := fmt.Sprintf("items[%d].value", i)
				if err := engine.SetField(config, path, fmt.Sprintf("initial-%d", i)); err != nil {
					return false
				}
			}

			// Record initial state
			initialLen := len(config.Items)
			initialValues := make([]string, initialLen)
			for i, item := range config.Items {
				initialValues[i] = item.Value
			}

			// Set value at target index (may expand array)
			path := fmt.Sprintf("items[%d].value", targetIndex)
			if err := engine.SetField(config, path, value); err != nil {
				return false
			}

			// Verify array integrity
			expectedLen := max(initialLen, targetIndex+1)
			if len(config.Items) != expectedLen {
				return false
			}

			// Verify existing values are preserved (except at target index)
			for i := 0; i < initialLen; i++ {
				if i != targetIndex && config.Items[i].Value != initialValues[i] {
					return false
				}
			}

			// Verify new value is set correctly
			if config.Items[targetIndex].Value != value {
				return false
			}

			// Verify intermediate elements are zero-initialized
			for i := initialLen; i < targetIndex; i++ {
				if config.Items[i].Value != "" {
					return false
				}
			}

			return true
		},
		gen.IntRange(0, 5),  // initial size
		gen.IntRange(0, 10), // target index
		genTestValue(),      // value to set
	))

	properties.Property("nested array operations preserve structure", prop.ForAll(
		func(outerIndex int, innerIndex int, value string) bool {
			engine := NewEnhancedReflectionEngine()
			config := &TestNestedArrayConfig{}

			// Set a value in nested array structure
			path := fmt.Sprintf("groups[%d].items[%d].data", outerIndex, innerIndex)
			if err := engine.SetField(config, path, value); err != nil {
				return false
			}

			// Verify structure was created correctly
			if len(config.Groups) != outerIndex+1 {
				return false
			}

			if len(config.Groups[outerIndex].Items) != innerIndex+1 {
				return false
			}

			if config.Groups[outerIndex].Items[innerIndex].Data != value {
				return false
			}

			// Verify other elements are zero-initialized
			for i := 0; i < outerIndex; i++ {
				if len(config.Groups[i].Items) != 0 {
					return false
				}
			}

			for i := 0; i < innerIndex; i++ {
				if config.Groups[outerIndex].Items[i].Data != "" {
					return false
				}
			}

			return true
		},
		gen.IntRange(0, 3), // outer index
		gen.IntRange(0, 3), // inner index
		genTestValue(),     // value to set
	))

	properties.Property("bracket and dot syntax produce identical array results", prop.ForAll(
		func(index int, value string) bool {
			engine := NewEnhancedReflectionEngine()

			// Test with bracket syntax
			bracketConfig := &TestArrayConfig{}
			bracketPath := fmt.Sprintf("items[%d].value", index)
			if err := engine.SetField(bracketConfig, bracketPath, value); err != nil {
				return false
			}

			// Test with dot syntax
			dotConfig := &TestArrayConfig{}
			dotPath := fmt.Sprintf("items.%d.value", index)
			if err := engine.SetField(dotConfig, dotPath, value); err != nil {
				return false
			}

			// Both should have same array length
			if len(bracketConfig.Items) != len(dotConfig.Items) {
				return false
			}

			// Both should have same value at target index
			if bracketConfig.Items[index].Value != dotConfig.Items[index].Value {
				return false
			}

			// Both should have same value as what we set
			if bracketConfig.Items[index].Value != value || dotConfig.Items[index].Value != value {
				return false
			}

			// All other elements should be zero-initialized in both
			for i := 0; i < len(bracketConfig.Items); i++ {
				if i != index {
					if bracketConfig.Items[i].Value != "" || dotConfig.Items[i].Value != "" {
						return false
					}
				}
			}

			return true
		},
		gen.IntRange(0, 5), // index
		genTestValue(),     // value to set
	))

	properties.Property("array operations preserve type safety", prop.ForAll(
		func(index int, intValue int, stringValue string, boolValue bool) bool {
			engine := NewEnhancedReflectionEngine()
			config := &TestTypedArrayConfig{}

			// Set different types in typed arrays
			intPath := fmt.Sprintf("integers[%d]", index)
			if err := engine.SetField(config, intPath, fmt.Sprintf("%d", intValue)); err != nil {
				return false
			}

			stringPath := fmt.Sprintf("strings[%d]", index)
			if err := engine.SetField(config, stringPath, stringValue); err != nil {
				return false
			}

			boolPath := fmt.Sprintf("booleans[%d]", index)
			if err := engine.SetField(config, boolPath, fmt.Sprintf("%t", boolValue)); err != nil {
				return false
			}

			// Verify types are preserved
			if len(config.Integers) != index+1 || config.Integers[index] != intValue {
				return false
			}

			if len(config.Strings) != index+1 || config.Strings[index] != stringValue {
				return false
			}

			if len(config.Booleans) != index+1 || config.Booleans[index] != boolValue {
				return false
			}

			return true
		},
		gen.IntRange(0, 3),      // index
		gen.IntRange(-100, 100), // int value
		genTestValue(),          // string value
		gen.Bool(),              // bool value
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Test structures for array manipulation property testing

type TestArrayConfig struct {
	Items []TestArrayItem `yaml:"items"`
}

type TestArrayItem struct {
	Value string `yaml:"value"`
	ID    int    `yaml:"id"`
}

type TestNestedArrayConfig struct {
	Groups []TestGroup `yaml:"groups"`
}

type TestGroup struct {
	Name  string          `yaml:"name"`
	Items []TestGroupItem `yaml:"items"`
}

type TestGroupItem struct {
	Data string `yaml:"data"`
}

type TestTypedArrayConfig struct {
	Integers []int    `yaml:"integers"`
	Strings  []string `yaml:"strings"`
	Booleans []bool   `yaml:"booleans"`
}

// Generators for array manipulation property tests

func genTestValue() gopter.Gen {
	return gen.OneConstOf(
		"test-value", "data", "content", "item", "element",
		"alpha", "beta", "gamma", "delta", "epsilon",
		"config-1", "config-2", "value-a", "value-b",
	)
}


// Helper function for max (Go 1.21+ has this built-in)
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

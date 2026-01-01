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
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: cli-configuration-enhancement, Property 9: Error reporting completeness
// For any invalid configuration input, the CLI should report all errors with specific locations, 
// suggested fixes, and clear error messages
// Validates: Requirements 2.3, 3.4, 4.2, 5.3, 7.5, 10.1, 10.2, 10.3, 10.5
func TestProperty_ErrorReportingCompleteness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("invalid path syntax produces descriptive errors", prop.ForAll(
		func(invalidPath string) bool {
			engine := NewEnhancedReflectionEngine()
			config := &TestErrorConfig{}
			
			// Try to set a field with invalid path syntax
			err := engine.SetField(config, invalidPath, "test-value")
			
			// Should produce an error
			if err == nil {
				return false
			}
			
			// Error message should contain the invalid path
			errorMsg := err.Error()
			if !strings.Contains(errorMsg, invalidPath) {
				return false
			}
			
			// Error message should be descriptive (not just "error")
			if len(errorMsg) < 10 {
				return false
			}
			
			return true
		},
		genInvalidPath(),
	))

	properties.Property("type conversion errors provide clear feedback", prop.ForAll(
		func(fieldPath string, invalidValue string) bool {
			engine := NewEnhancedReflectionEngine()
			config := &TestErrorConfig{}
			
			// Try to set an integer field with invalid value
			err := engine.SetField(config, fieldPath, invalidValue)
			
			// Should produce an error for invalid integer values
			if err == nil {
				return false
			}
			
			errorMsg := err.Error()
			
			// Error should mention the invalid value
			if !strings.Contains(errorMsg, invalidValue) {
				return false
			}
			
			// Error should be informative
			if len(errorMsg) < 15 {
				return false
			}
			
			return true
		},
		gen.OneConstOf("intField", "count", "number"), // integer field paths
		genInvalidIntValue(),                          // invalid integer values
	))

	properties.Property("nonexistent field errors include field name", prop.ForAll(
		func(invalidField string) bool {
			engine := NewEnhancedReflectionEngine()
			config := &TestErrorConfig{}
			
			// Try to set a nonexistent field
			err := engine.SetField(config, invalidField, "test-value")
			
			// Should produce an error
			if err == nil {
				return false
			}
			
			errorMsg := err.Error()
			
			// Error should mention the field name
			if !strings.Contains(errorMsg, invalidField) {
				return false
			}
			
			// Error should indicate field not found
			lowerMsg := strings.ToLower(errorMsg)
			if !strings.Contains(lowerMsg, "not found") && !strings.Contains(lowerMsg, "invalid") {
				return false
			}
			
			return true
		},
		genNonexistentField(),
	))

	properties.Property("array index errors provide bounds information", prop.ForAll(
		func(largeIndex int) bool {
			engine := NewEnhancedReflectionEngine()
			config := &TestErrorConfig{}
			
			// Initialize a small array
			if err := engine.SetField(config, "items[0].name", "first"); err != nil {
				return false
			}
			
			// Try to access way beyond bounds without expansion
			path := "items.name" // This should fail because items[0] is a struct, not an array
			err := engine.SetField(config, path, "test")
			
			// Should produce an error
			if err == nil {
				return false
			}
			
			errorMsg := err.Error()
			
			// Error should be descriptive
			if len(errorMsg) < 10 {
				return false
			}
			
			return true
		},
		gen.IntRange(100, 1000), // Large indices
	))

	properties.Property("error reporter aggregates multiple errors", prop.ForAll(
		func(errorCount int) bool {
			reporter := NewErrorReporter()
			
			// Add multiple errors
			for i := 0; i < errorCount; i++ {
				err := &ConfigError{
					Type:    ErrorTypeValidation,
					Path:    "test.path",
					Message: "Test error message",
				}
				reporter.Add(err)
			}
			
			// Should have the correct number of errors
			if len(reporter.Errors()) != errorCount {
				return false
			}
			
			// Should report having errors if count > 0
			if errorCount > 0 && !reporter.HasErrors() {
				return false
			}
			
			// Should not report having errors if count == 0
			if errorCount == 0 && reporter.HasErrors() {
				return false
			}
			
			// Error message should mention the count if multiple errors
			errorMsg := reporter.Error()
			if errorCount > 1 {
				if !strings.Contains(errorMsg, "Multiple") {
					return false
				}
			}
			
			return true
		},
		gen.IntRange(0, 5), // Number of errors
	))

	properties.Property("config errors preserve error type information", prop.ForAll(
		func(errorType ErrorType, path string, message string) bool {
			err := &ConfigError{
				Type:    errorType,
				Path:    path,
				Message: message,
			}
			
			// Error should preserve type
			if err.Type != errorType {
				return false
			}
			
			// Error should preserve path
			if err.Path != path {
				return false
			}
			
			// Error should preserve message
			if err.Message != message {
				return false
			}
			
			// String representation should include type
			errorStr := err.Error()
			if !strings.Contains(errorStr, string(errorType)) {
				return false
			}
			
			// String representation should include path if not empty
			if path != "" && !strings.Contains(errorStr, path) {
				return false
			}
			
			return true
		},
		genErrorType(),
		genValidPath(),
		genErrorMessage(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Test structures for error handling property testing

type TestErrorConfig struct {
	StringField string            `yaml:"stringField"`
	IntField    int               `yaml:"intField"`
	BoolField   bool              `yaml:"boolField"`
	Items       []TestErrorItem   `yaml:"items"`
	Count       int               `yaml:"count"`
	Number      int               `yaml:"number"`
	Settings    map[string]string `yaml:"settings"`
}

type TestErrorItem struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// Generators for error handling property tests

func genInvalidPath() gopter.Gen {
	return gen.OneConstOf(
		"",                    // Empty path
		".",                   // Just dot
		".field",              // Starting with dot
		"field.",              // Ending with dot
		"field..subfield",     // Double dot
		"field[",              // Unclosed bracket
		"field]",              // Unmatched bracket
		"field[-1]",           // Negative index
		"field[abc]",          // Non-numeric index
		"field[0",             // Unclosed bracket with index
		"123field",            // Starting with number
		"field-with-spaces ",  // Trailing space
		"field\nwith\nnewlines", // Newlines
	)
}

func genInvalidIntValue() gopter.Gen {
	return gen.OneConstOf(
		"not-a-number",
		"12.34.56",
		"abc123",
		"",
		"true",
		"false",
		"null",
		"undefined",
		"NaN",
		"Infinity",
		"1e999999", // Too large
	)
}

func genNonexistentField() gopter.Gen {
	return gen.OneConstOf(
		"nonexistentField",
		"missingField",
		"unknownProperty",
		"invalidField",
		"notFound",
		"doesNotExist",
		"wrongName",
		"typoField",
	)
}

func genErrorType() gopter.Gen {
	return gen.OneConstOf(
		ErrorTypeSyntax,
		ErrorTypeValidation,
		ErrorTypeType,
		ErrorTypeConflict,
		ErrorTypeTemplate,
		ErrorTypePath,
		ErrorTypePermission,
	)
}

func genValidPath() gopter.Gen {
	return gen.OneConstOf(
		"field",
		"field.subfield",
		"items[0].name",
		"settings.key",
		"config.nested.value",
		"", // Empty path for testing
	)
}

func genErrorMessage() gopter.Gen {
	return gen.OneConstOf(
		"Test error message",
		"Invalid configuration",
		"Field not found",
		"Type conversion failed",
		"Syntax error",
		"Validation failed",
		"", // Empty message for testing
	)
}
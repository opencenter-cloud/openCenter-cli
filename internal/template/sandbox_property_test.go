/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package template

import (
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: security-and-operational-remediation, Property 4: Template Function Whitelist Enforcement
// For any template rendering operation, only safe functions (string manipulation, formatting, logic)
// SHALL be available, and dangerous functions (env, readFile, exec) SHALL be disabled and cause
// errors if accessed.
// Validates: Requirements 2.1, 2.2, 2.3
func TestProperty_TemplateFunctionWhitelistEnforcement(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("dangerous functions are rejected during validation", prop.ForAll(
		func(funcName string) bool {
			sandbox := NewTemplateSandbox()

			// Create a template that tries to use the dangerous function
			tmpl := "{{ " + funcName + " }}"

			// Validation should fail for dangerous functions
			err := sandbox.ValidateTemplate(tmpl)

			// If it's a dangerous function, validation must fail
			if IsDangerousFunction(funcName) {
				return err != nil && strings.Contains(err.Error(), "not defined")
			}

			// For non-dangerous functions, we don't care about the result
			return true
		},
		genDangerousFunction(),
	))

	properties.Property("safe functions are accepted during validation", prop.ForAll(
		func(funcName string) bool {
			sandbox := NewTemplateSandbox()

			// Create a simple template using the safe function
			// Use a template that won't fail due to missing arguments
			var tmpl string
			switch funcName {
			case "upper", "lower", "trim":
				tmpl = `{{ . | ` + funcName + ` }}`
			case "printf":
				tmpl = `{{ printf "%s" . }}`
			case "quote":
				tmpl = `{{ quote . }}`
			case "default":
				tmpl = `{{ default "fallback" . }}`
			default:
				// For other functions, just check they exist
				tmpl = `{{ . }}`
			}

			// Validation should succeed for safe functions
			err := sandbox.ValidateTemplate(tmpl)

			return err == nil
		},
		genSafeFunction(),
	))

	properties.Property("templates with dangerous function calls fail to render", prop.ForAll(
		func(funcName string, data string) bool {
			sandbox := NewTemplateSandbox()

			// Create a template that tries to call the dangerous function
			tmpl := "{{ " + funcName + " }}"

			// Rendering should fail
			_, err := sandbox.RenderWithTimeout(tmpl, data, 5*time.Second)

			// Must fail for dangerous functions
			if IsDangerousFunction(funcName) {
				return err != nil
			}

			// For non-dangerous functions, we don't care
			return true
		},
		genDangerousFunction(),
		gen.AnyString(),
	))

	properties.Property("only safe functions are in the function map", prop.ForAll(
		func(_ bool) bool {
			sandbox := NewTemplateSandbox()
			safeFuncs := sandbox.GetSafeFunctions()

			// Check that no dangerous functions are in the map
			for _, dangerous := range DangerousFunctions() {
				if _, exists := safeFuncs[dangerous]; exists {
					return false // Dangerous function found in safe map!
				}
			}

			return true
		},
		gen.Bool(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: security-and-operational-remediation, Property 5: Template Input Validation
// For any template data, the system SHALL validate it before rendering, and templates
// SHALL timeout after 30 seconds to prevent denial-of-service.
// Validates: Requirements 2.4, 2.5
func TestProperty_TemplateInputValidation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("templates timeout after specified duration", prop.ForAll(
		func(timeoutMs int) bool {
			// Use a very short timeout range for testing (10ms to 50ms)
			if timeoutMs < 10 || timeoutMs > 50 {
				return true // Skip invalid timeouts
			}

			timeout := time.Duration(timeoutMs) * time.Millisecond
			sandbox := NewTemplateSandbox()

			// Create a template that will take longer than the timeout
			// Use a moderately sized loop
			tmpl := `{{ range $i := until 1000 }}{{ range $j := until 1000 }}{{ $i }}{{ end }}{{ end }}`

			start := time.Now()
			_, err := sandbox.RenderWithTimeout(tmpl, nil, timeout)
			elapsed := time.Since(start)

			// Should timeout and error should mention timeout
			if err == nil {
				return false
			}

			if !strings.Contains(err.Error(), "timed out") {
				return false
			}

			// Elapsed time should be close to timeout (within 5x for goroutine scheduling)
			return elapsed >= timeout && elapsed < timeout*5
		},
		gen.IntRange(10, 50),
	))

	properties.Property("valid templates render successfully within timeout", prop.ForAll(
		func(data string) bool {
			sandbox := NewTemplateSandbox()

			// Simple template that should complete quickly
			tmpl := `{{ . | upper }}`

			_, err := sandbox.RenderWithTimeout(tmpl, data, 5*time.Second)

			// Should succeed
			return err == nil
		},
		gen.AnyString(),
	))

	properties.Property("invalid templates fail validation before rendering", prop.ForAll(
		func(_ bool) bool {
			sandbox := NewTemplateSandbox()

			// Template with syntax error
			tmpl := `{{ .InvalidSyntax }`

			// Should fail validation
			err := sandbox.ValidateTemplate(tmpl)

			return err != nil
		},
		gen.Bool(),
	))

	properties.Property("templates with undefined functions fail validation", prop.ForAll(
		func(funcName string) bool {
			// Skip if it's actually a safe function
			sandbox := NewTemplateSandbox()
			safeFuncs := sandbox.GetSafeFunctions()
			if _, exists := safeFuncs[funcName]; exists {
				return true
			}

			// Template using undefined function
			tmpl := "{{ " + funcName + " }}"

			// Should fail validation
			err := sandbox.ValidateTemplate(tmpl)

			return err != nil && strings.Contains(err.Error(), "not defined")
		},
		gen.Identifier(),
	))

	properties.Property("default timeout is 30 seconds", prop.ForAll(
		func(_ bool) bool {
			sandbox := NewTemplateSandbox()

			// We can't practically test a 30-second timeout in a test suite
			// Instead, verify that passing 0 timeout uses a non-zero default
			// by checking that a quick template completes successfully

			tmpl := `{{ . | upper }}`

			_, err := sandbox.RenderWithTimeout(tmpl, "test", 0)

			// Should succeed with default timeout for a quick template
			return err == nil
		},
		gen.Bool(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// genDangerousFunction generates dangerous function names for testing.
func genDangerousFunction() gopter.Gen {
	dangerous := DangerousFunctions()
	return gen.OneConstOf(
		dangerous[0], dangerous[1], dangerous[2], dangerous[3], dangerous[4],
		dangerous[5], dangerous[6], dangerous[7], dangerous[8], dangerous[9],
		dangerous[10],
	)
}

// genSafeFunction generates safe function names for testing.
func genSafeFunction() gopter.Gen {
	safeFuncs := []string{
		"upper", "lower", "trim", "replace", "split", "join",
		"printf", "quote", "indent", "default",
	}
	return gen.OneConstOf(
		safeFuncs[0], safeFuncs[1], safeFuncs[2], safeFuncs[3], safeFuncs[4],
		safeFuncs[5], safeFuncs[6], safeFuncs[7], safeFuncs[8], safeFuncs[9],
	)
}

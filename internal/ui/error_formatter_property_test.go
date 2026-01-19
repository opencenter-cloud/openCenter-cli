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

package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: security-and-operational-remediation, Property 19: Error Message Structure
// For any error, the formatted output SHALL include error code, short description, context,
// fix command, and documentation link.
// **Validates: Requirements 15.1, 15.2, 15.4, 15.8**
func TestProperty_ErrorMessageStructure(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	formatter := NewDefaultErrorFormatter()

	// Property 19.1: FormatWithErrorInfo includes all required fields
	properties.Property("FormatWithErrorInfo includes all required fields", prop.ForAll(
		func(errorMsg string) bool {
			// Skip empty error messages
			if errorMsg == "" {
				return true
			}

			// Create an error
			err := errors.New(errorMsg)

			// Get error info for a known code
			info, ok := formatter.GetErrorInfo("E1001")
			if !ok {
				t.Fatal("Error code E1001 should exist in registry")
			}

			// Format with error info
			formatted := formatter.FormatWithErrorInfo(err, info)

			// Verify all required fields are present
			hasCode := strings.Contains(formatted, info.Code)
			hasTitle := strings.Contains(formatted, info.Title)
			hasDescription := strings.Contains(formatted, info.Description)
			hasFix := strings.Contains(formatted, info.Fix)
			hasFixCommand := strings.Contains(formatted, info.FixCommand)
			hasDocsURL := strings.Contains(formatted, info.DocsURL)

			return hasCode && hasTitle && hasDescription && hasFix && hasFixCommand && hasDocsURL
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property 19.2: Error codes are present in formatted output
	properties.Property("error codes are present in formatted output", prop.ForAll(
		func(errorMsg string, code string) bool {
			// Skip empty inputs
			if errorMsg == "" || code == "" {
				return true
			}

			err := errors.New(errorMsg)
			formatted := formatter.FormatWithCode(err, code)

			// The code should appear in the formatted output
			return strings.Contains(formatted, code)
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.RegexMatch("E[1-6][0-9]{3}"),
	))

	// Property 19.3: Fix suggestions are included when provided
	properties.Property("fix suggestions are included when provided", prop.ForAll(
		func(errorMsg string, fix string) bool {
			// Skip empty inputs
			if errorMsg == "" || fix == "" {
				return true
			}

			err := errors.New(errorMsg)
			formatted := formatter.FormatWithFix(err, fix)

			// The fix should appear in the formatted output
			return strings.Contains(formatted, fix)
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property 19.4: Documentation links follow expected format
	properties.Property("documentation links follow expected format", prop.ForAll(
		func(_ bool) bool {
			// Check all error codes in registry have valid docs URLs
			for code, info := range formatter.errorRegistry {
				if info.DocsURL == "" {
					return false
				}
				// Should start with https://docs.opencenter.cloud/errors/
				expectedPrefix := "https://docs.opencenter.cloud/errors/"
				if !strings.HasPrefix(info.DocsURL, expectedPrefix) {
					return false
				}
				// Should end with the error code
				if !strings.HasSuffix(info.DocsURL, code) {
					return false
				}
			}
			return true
		},
		gen.Const(true),
	))

	// Property 19.5: Error codes follow E[1-6]xxx pattern
	properties.Property("error codes follow E[1-6]xxx pattern", prop.ForAll(
		func(_ bool) bool {
			// Check all error codes in registry follow the pattern
			for code := range formatter.errorRegistry {
				if len(code) != 5 {
					return false
				}
				if code[0] != 'E' {
					return false
				}
				category := code[1]
				if category < '1' || category > '6' {
					return false
				}
				// Check remaining digits are numeric
				for i := 2; i < 5; i++ {
					if code[i] < '0' || code[i] > '9' {
						return false
					}
				}
			}
			return true
		},
		gen.Const(true),
	))

	// Property 19.6: Credentials are masked in formatted errors
	properties.Property("credentials are masked in formatted errors", prop.ForAll(
		func(password string) bool {
			// Skip short passwords
			if len(password) < 3 {
				return true
			}

			// Create error with credential
			err := fmt.Errorf("authentication failed with password=%s", password)

			// Format the error
			formatted := formatter.Format(err)

			// The original password should not appear
			return !strings.Contains(formatted, password)
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) >= 3 }),
	))

	// Property 19.7: Empty errors return empty strings
	properties.Property("empty errors return empty strings", prop.ForAll(
		func(_ bool) bool {
			formatted := formatter.Format(nil)
			return formatted == ""
		},
		gen.Const(true),
	))

	// Property 19.8: FormatMultipleWithLimit respects limit
	properties.Property("FormatMultipleWithLimit respects limit", prop.ForAll(
		func(errorCount int) bool {
			// Generate between 1 and 20 errors
			if errorCount < 1 || errorCount > 20 {
				return true
			}

			errs := make([]error, errorCount)
			for i := 0; i < errorCount; i++ {
				errs[i] = fmt.Errorf("error %d", i+1)
			}

			// Format with limit of 5
			formatted := formatter.FormatMultipleWithLimit(errs, 5, false)

			// If we have more than 5 errors, should mention "more errors"
			if errorCount > 5 {
				return strings.Contains(formatted, "more errors")
			}

			return true
		},
		gen.IntRange(1, 20),
	))

	// Property 19.9: Severity classification is consistent
	properties.Property("severity classification is consistent", prop.ForAll(
		func(_ bool) bool {
			// Check that all error codes have a severity assigned
			for _, info := range formatter.errorRegistry {
				if info.Severity < SeverityInfo || info.Severity > SeverityCritical {
					return false
				}
			}
			return true
		},
		gen.Const(true),
	))

	// Property 19.10: Error titles are non-empty
	properties.Property("error titles are non-empty", prop.ForAll(
		func(_ bool) bool {
			// Check all error codes have non-empty titles
			for _, info := range formatter.errorRegistry {
				if info.Title == "" {
					return false
				}
			}
			return true
		},
		gen.Const(true),
	))

	// Property 19.11: GetErrorInfo returns correct info for valid codes
	properties.Property("GetErrorInfo returns correct info for valid codes", prop.ForAll(
		func(_ bool) bool {
			// Test a few known error codes
			knownCodes := []string{"E1001", "E1002", "E2001", "E3001", "E4001", "E5001", "E6001"}

			for _, code := range knownCodes {
				info, ok := formatter.GetErrorInfo(code)
				if !ok {
					return false
				}
				if info.Code != code {
					return false
				}
			}
			return true
		},
		gen.Const(true),
	))

	// Property 19.12: GetErrorInfo returns false for invalid codes
	properties.Property("GetErrorInfo returns false for invalid codes", prop.ForAll(
		func(invalidCode string) bool {
			// Generate an invalid code
			if invalidCode == "" || len(invalidCode) < 5 {
				invalidCode = "INVALID"
			}

			// Should not exist in registry
			_, ok := formatter.GetErrorInfo(invalidCode)

			// For truly invalid codes, should return false
			// (unless by chance it matches a real code)
			if !strings.HasPrefix(invalidCode, "E") || len(invalidCode) != 5 {
				return !ok
			}

			return true
		},
		gen.AlphaString(),
	))

	// Property 19.13: Fix commands are actionable
	properties.Property("fix commands are actionable", prop.ForAll(
		func(_ bool) bool {
			// Check that all fix commands start with a command name
			for _, info := range formatter.errorRegistry {
				if info.FixCommand == "" {
					continue
				}
				// Should start with a command (openCenter, chmod, ls, etc.)
				words := strings.Fields(info.FixCommand)
				if len(words) == 0 {
					return false
				}
			}
			return true
		},
		gen.Const(true),
	))

	// Property 19.14: Hints provide additional context
	properties.Property("hints provide additional context", prop.ForAll(
		func(_ bool) bool {
			// Check that hints are different from descriptions
			for _, info := range formatter.errorRegistry {
				if info.Hint == "" {
					continue
				}
				// Hint should not be identical to description
				if info.Hint == info.Description {
					return false
				}
			}
			return true
		},
		gen.Const(true),
	))

	// Property 19.15: Error registry contains expected categories
	properties.Property("error registry contains expected categories", prop.ForAll(
		func(_ bool) bool {
			// Check that we have errors in each category (E1xxx through E6xxx)
			categories := make(map[byte]bool)

			for code := range formatter.errorRegistry {
				if len(code) >= 2 {
					categories[code[1]] = true
				}
			}

			// Should have at least one error in each category 1-6
			for cat := byte('1'); cat <= byte('6'); cat++ {
				if !categories[cat] {
					return false
				}
			}

			return true
		},
		gen.Const(true),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

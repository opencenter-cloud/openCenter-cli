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

package security

import (
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: security-and-operational-remediation, Property 1: Input Validation Rejects Invalid Patterns
// For any user-provided input (cluster name, organization name, path, URL), if the input contains
// invalid characters or patterns (path traversal, shell metacharacters, invalid schemes), then the
// validation SHALL reject it with a structured error.
// **Validates: Requirements 1.1, 1.2, 1.5, 1.6, 1.8, 6.1, 6.2, 6.3, 6.6**
func TestProperty_InputValidationRejectsInvalidPatterns(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	validator := NewDefaultInputValidator()

	// Property 1.1: Cluster names with path traversal are rejected
	properties.Property("cluster names with path traversal are rejected", prop.ForAll(
		func(name string) bool {
			// If name contains path traversal sequences, it must be rejected
			if strings.Contains(name, "..") {
				err := validator.ValidateClusterName(name)
				return err != nil && IsValidationError(err)
			}
			return true // Names without ".." may pass or fail based on other rules
		},
		gen.AnyString(),
	))

	// Property 1.2: Cluster names with path separators are rejected
	properties.Property("cluster names with path separators are rejected", prop.ForAll(
		func(name string) bool {
			// If name contains path separators, it must be rejected
			if strings.Contains(name, "/") || strings.Contains(name, "\\") {
				err := validator.ValidateClusterName(name)
				return err != nil && IsValidationError(err)
			}
			return true // Names without separators may pass or fail based on other rules
		},
		gen.AnyString(),
	))

	// Property 1.3: Organization names with path traversal are rejected
	properties.Property("organization names with path traversal are rejected", prop.ForAll(
		func(org string) bool {
			// If org contains path traversal sequences, it must be rejected
			if strings.Contains(org, "..") {
				err := validator.ValidateOrganizationName(org)
				return err != nil && IsValidationError(err)
			}
			return true // Orgs without ".." may pass or fail based on other rules
		},
		gen.AnyString(),
	))

	// Property 1.4: Organization names with path separators are rejected
	properties.Property("organization names with path separators are rejected", prop.ForAll(
		func(org string) bool {
			// If org contains path separators, it must be rejected
			if strings.Contains(org, "/") || strings.Contains(org, "\\") {
				err := validator.ValidateOrganizationName(org)
				return err != nil && IsValidationError(err)
			}
			return true // Orgs without separators may pass or fail based on other rules
		},
		gen.AnyString(),
	))

	// Property 1.5: Paths with path traversal are rejected
	properties.Property("paths with path traversal are rejected", prop.ForAll(
		func(path string) bool {
			// If path contains "..", it must be rejected
			if strings.Contains(path, "..") {
				err := validator.ValidatePath(path)
				return err != nil && IsValidationError(err)
			}
			return true // Paths without ".." may be valid
		},
		gen.AnyString(),
	))

	// Property 1.6: URLs with invalid schemes are rejected
	properties.Property("URLs with invalid schemes are rejected", prop.ForAll(
		func(scheme string, host string) bool {
			// Skip empty schemes or hosts
			if scheme == "" || host == "" {
				return true
			}

			// Build URL with potentially invalid scheme
			urlStr := scheme + "://" + host

			// If scheme is not http or https, it should be rejected
			lowerScheme := strings.ToLower(scheme)
			if lowerScheme != "http" && lowerScheme != "https" {
				err := validator.ValidateURL(urlStr)
				return err != nil && IsValidationError(err)
			}
			return true // http/https may pass or fail based on other rules
		},
		gen.AlphaString(),
		gen.AlphaString(),
	))

	// Property 1.7: External URLs without HTTPS are rejected
	properties.Property("external URLs without HTTPS are rejected", prop.ForAll(
		func(host string) bool {
			// Skip empty or local hosts
			if host == "" || host == "localhost" || strings.HasPrefix(host, "127.") ||
				strings.HasPrefix(host, "192.168.") || strings.HasPrefix(host, "10.") {
				return true
			}

			// Build HTTP URL with external host
			urlStr := "http://" + host + ".com"

			// External HTTP URLs should be rejected
			err := validator.ValidateURL(urlStr)
			return err != nil && IsValidationError(err)
		},
		gen.AlphaString(),
	))

	// Property 1.8: Environment variables with shell metacharacters are rejected
	properties.Property("environment variables with shell metacharacters are rejected", prop.ForAll(
		func(value string) bool {
			// If value contains dangerous shell metacharacters, it must be rejected
			dangerousChars := []string{";", "|", "&", "`", "\n", "\r"}
			for _, char := range dangerousChars {
				if strings.Contains(value, char) {
					err := validator.ValidateEnvironmentVariable("TEST_VAR", value)
					return err != nil && IsValidationError(err)
				}
			}
			return true // Values without dangerous chars may be valid
		},
		gen.AnyString(),
	))

	// Property 1.9: EDITOR with shell metacharacters is rejected
	properties.Property("EDITOR with shell metacharacters is rejected", prop.ForAll(
		func(editor string) bool {
			// If editor contains shell metacharacters, it must be rejected
			shellMetachars := []string{";", "|", "&", "`", "\n", "\r"}
			for _, char := range shellMetachars {
				if strings.Contains(editor, char) {
					err := validator.ValidateEnvironmentVariable("EDITOR", editor)
					return err != nil && IsValidationError(err)
				}
			}
			return true // Editors without metacharacters may pass or fail based on whitelist
		},
		gen.AnyString(),
	))

	// Property 1.10: Shell input with dangerous metacharacters is rejected
	properties.Property("shell input with dangerous metacharacters is rejected", prop.ForAll(
		func(input string) bool {
			// If input contains dangerous metacharacters, sanitization must fail
			dangerousChars := []string{";", "|", "&", "`", "\n", "\r"}
			for _, char := range dangerousChars {
				if strings.Contains(input, char) {
					_, err := validator.SanitizeShellInput(input)
					return err != nil && IsValidationError(err)
				}
			}
			return true // Input without dangerous chars may be sanitized successfully
		},
		gen.AnyString(),
	))

	// Property 1.11: Valid cluster names are accepted
	properties.Property("valid cluster names are accepted", prop.ForAll(
		func(name string) bool {
			// Generate a valid cluster name
			if name == "" {
				return true // Skip empty
			}

			// Ensure it starts with alphanumeric and contains only valid chars
			validName := genValidClusterName(name)
			err := validator.ValidateClusterName(validName)
			return err == nil
		},
		gen.AlphaString(),
	))

	// Property 1.12: Valid organization names are accepted
	properties.Property("valid organization names are accepted", prop.ForAll(
		func(org string) bool {
			// Generate a valid organization name
			if org == "" {
				return true // Skip empty
			}

			// Ensure it starts with alphanumeric and contains only valid chars
			validOrg := genValidClusterName(org)
			err := validator.ValidateOrganizationName(validOrg)
			return err == nil
		},
		gen.AlphaString(),
	))

	// Property 1.13: Valid HTTPS URLs are accepted
	properties.Property("valid HTTPS URLs are accepted", prop.ForAll(
		func(host string) bool {
			// Skip empty hosts
			if host == "" {
				return true
			}

			// Build valid HTTPS URL
			validHost := genValidHostname(host)
			urlStr := "https://" + validHost + ".com"

			err := validator.ValidateURL(urlStr)
			return err == nil
		},
		gen.AlphaString(),
	))

	// Property 1.14: Local HTTP URLs are accepted
	properties.Property("local HTTP URLs are accepted", prop.ForAll(
		func(port int) bool {
			// Skip invalid ports
			if port < 1 || port > 65535 {
				return true
			}

			// Test localhost HTTP URLs
			localURLs := []string{
				"http://localhost:8080",
				"http://127.0.0.1:5000",
				"http://192.168.1.1:3000",
			}

			for _, urlStr := range localURLs {
				err := validator.ValidateURL(urlStr)
				if err != nil {
					return false
				}
			}
			return true
		},
		gen.IntRange(1, 65535),
	))

	// Property 1.15: Safe shell input is sanitized correctly
	properties.Property("safe shell input is sanitized correctly", prop.ForAll(
		func(input string) bool {
			// If input doesn't contain dangerous characters, sanitization should succeed
			dangerousChars := []string{";", "|", "&", "`", "\n", "\r"}
			hasDangerous := false
			for _, char := range dangerousChars {
				if strings.Contains(input, char) {
					hasDangerous = true
					break
				}
			}

			if !hasDangerous {
				_, err := validator.SanitizeShellInput(input)
				return err == nil
			}
			return true // Dangerous input should be rejected (tested in other properties)
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Helper functions for generating valid test data

// genValidClusterName generates a valid cluster name from arbitrary input
func genValidClusterName(input string) string {
	if input == "" {
		return "a"
	}

	// Ensure it starts with alphanumeric
	var result strings.Builder
	result.WriteString("a") // Always start with 'a'

	// Add valid characters from input (alphanumeric, hyphen, underscore)
	for _, ch := range input {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
			result.WriteRune(ch)
		}
		// Stop at 63 characters
		if result.Len() >= 63 {
			break
		}
	}

	name := result.String()
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}

// genValidHostname generates a valid hostname from arbitrary input
func genValidHostname(input string) string {
	if input == "" {
		return "example"
	}

	// Keep only alphanumeric and hyphens
	var result strings.Builder
	for _, ch := range input {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '-' {
			result.WriteRune(ch)
		}
		// Stop at reasonable length
		if result.Len() >= 63 {
			break
		}
	}

	hostname := result.String()
	if hostname == "" {
		hostname = "example"
	}
	// Ensure it doesn't start or end with hyphen
	hostname = strings.Trim(hostname, "-")
	if hostname == "" {
		hostname = "example"
	}
	return hostname
}

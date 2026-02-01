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

package validation

import (
	"fmt"
	"testing"
)

// TestSuggestionEngine_Demo demonstrates the complete SuggestionEngine functionality.
func TestSuggestionEngine_Demo(t *testing.T) {
	// Create suggestion engine
	suggestionEngine := NewSuggestionEngine()

	// Create a validation result with errors
	result := &ValidationResult{
		Valid: false,
		Errors: []*ValidationIssue{
			{
				Severity: SeverityError,
				Field:    "cluster.provider",
				Message:  "invalid value 'openstck'",
				Code:     "E002",
			},
			{
				Severity: SeverityError,
				Field:    "cluster.email",
				Message:  "invalid email format",
				Code:     "E001",
			},
			{
				Severity: SeverityError,
				Field:    "network.cidr",
				Message:  "invalid CIDR notation",
				Code:     "E003",
			},
		},
	}

	// Enhance with suggestions
	context := map[string]interface{}{
		"valid_values": []string{"openstack", "aws", "gcp", "azure"},
	}

	suggestionEngine.EnhanceResult(result, context)

	// Verify suggestions were added
	if len(result.Errors) != 3 {
		t.Fatalf("Expected 3 errors, got %d", len(result.Errors))
	}

	// Check typo suggestion for provider
	providerError := result.Errors[0]
	if len(providerError.Suggestions) == 0 {
		t.Error("Expected suggestions for provider typo")
	}

	foundTypoSuggestion := false
	for _, s := range providerError.Suggestions {
		if s == "Did you mean \"openstack\"?" {
			foundTypoSuggestion = true
			break
		}
	}
	if !foundTypoSuggestion {
		t.Errorf("Expected typo suggestion for 'openstack', got: %v", providerError.Suggestions)
	}

	// Check context suggestion for email
	emailError := result.Errors[1]
	if len(emailError.Suggestions) == 0 {
		t.Error("Expected suggestions for email")
	}

	foundEmailSuggestion := false
	for _, s := range emailError.Suggestions {
		if s == "Ensure email is in format: user@example.com" {
			foundEmailSuggestion = true
			break
		}
	}
	if !foundEmailSuggestion {
		t.Errorf("Expected email format suggestion, got: %v", emailError.Suggestions)
	}

	// Check context suggestion for CIDR
	cidrError := result.Errors[2]
	if len(cidrError.Suggestions) == 0 {
		t.Error("Expected suggestions for CIDR")
	}

	foundCIDRSuggestion := false
	for _, s := range cidrError.Suggestions {
		if s == "Use CIDR notation (e.g., 10.0.0.0/16)" || s == "Verify CIDR notation is correct" {
			foundCIDRSuggestion = true
			break
		}
	}
	if !foundCIDRSuggestion {
		t.Errorf("Expected CIDR suggestion, got: %v", cidrError.Suggestions)
	}

	// Print demo output
	t.Log("\n=== SuggestionEngine Demo ===")
	for i, err := range result.Errors {
		t.Logf("\nError %d:", i+1)
		t.Logf("  Field: %s", err.Field)
		t.Logf("  Message: %s", err.Message)
		t.Logf("  Code: %s", err.Code)
		t.Logf("  Suggestions:")
		for _, s := range err.Suggestions {
			t.Logf("    - %s", s)
		}
	}
}

// TestLevenshteinDistance_Demo demonstrates the Levenshtein distance algorithm.
func TestLevenshteinDistance_Demo(t *testing.T) {
	tests := []struct {
		s1       string
		s2       string
		expected int
	}{
		{"openstack", "openstck", 1},   // Missing 'a'
		{"kubernetes", "kuberntes", 1}, // Missing 'e'
		{"aws", "azs", 1},              // Substitution
		{"gcp", "gpc", 2},              // Two substitutions
		{"azure", "azur", 1},           // Missing 'e'
	}

	t.Log("\n=== Levenshtein Distance Demo ===")
	for _, tt := range tests {
		distance := levenshteinDistance(tt.s1, tt.s2)
		if distance != tt.expected {
			t.Errorf("levenshteinDistance(%q, %q) = %d, expected %d", tt.s1, tt.s2, distance, tt.expected)
		}
		t.Logf("Distance between %q and %q: %d", tt.s1, tt.s2, distance)
	}
}

// ExampleSuggestionEngine_complete demonstrates the complete workflow.
func ExampleSuggestionEngine_complete() {
	// Create suggestion engine
	engine := NewSuggestionEngine()

	// Create a validation result with an error
	result := &ValidationResult{
		Valid: false,
		Errors: []*ValidationIssue{
			{
				Severity: SeverityError,
				Field:    "cluster.provider",
				Message:  "invalid value 'openstck'",
			},
		},
	}

	// Enhance with suggestions
	context := map[string]interface{}{
		"valid_values": []string{"openstack", "aws", "gcp"},
	}

	engine.EnhanceResult(result, context)

	// Print the enhanced result
	for _, err := range result.Errors {
		fmt.Printf("Error: %s\n", err.Message)
		fmt.Printf("Suggestions:\n")
		for _, s := range err.Suggestions {
			fmt.Printf("  - %s\n", s)
		}
	}

	// Output:
	// Error: invalid value 'openstck'
	// Suggestions:
	//   - Did you mean "openstack"?
	//   - Use alphanumeric characters, hyphens, and underscores only
}

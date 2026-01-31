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
	"testing"
)

func TestNewSuggestionEngine(t *testing.T) {
	engine := NewSuggestionEngine()
	if engine == nil {
		t.Fatal("NewSuggestionEngine returned nil")
	}

	if len(engine.rules) == 0 {
		t.Error("Expected default rules to be registered")
	}
}

func TestSuggestionEngine_AddRule(t *testing.T) {
	engine := NewSuggestionEngine()
	initialCount := len(engine.rules)

	customRule := &TypoSuggestionRule{}
	engine.AddRule(customRule)

	if len(engine.rules) != initialCount+1 {
		t.Error("Rule was not added")
	}
}

func TestSuggestionEngine_EnhanceResult(t *testing.T) {
	engine := NewSuggestionEngine()

	result := &ValidationResult{
		Valid: false,
		Errors: []*ValidationIssue{
			{
				Severity: SeverityError,
				Field:    "cluster.email",
				Message:  "invalid email format",
			},
		},
	}

	context := make(map[string]interface{})
	engine.EnhanceResult(result, context)

	if len(result.Errors[0].Suggestions) == 0 {
		t.Error("Expected suggestions to be added")
	}
}

func TestSuggestionEngine_EnhanceResultNil(t *testing.T) {
	engine := NewSuggestionEngine()

	// Should not panic
	engine.EnhanceResult(nil, nil)
}

func TestTypoSuggestionRule_Name(t *testing.T) {
	rule := &TypoSuggestionRule{}
	if rule.Name() != "typo" {
		t.Errorf("Expected name %q, got %q", "typo", rule.Name())
	}
}

func TestTypoSuggestionRule_Generate(t *testing.T) {
	rule := &TypoSuggestionRule{}

	issue := &ValidationIssue{
		Field:   "provider",
		Message: "invalid value 'openstck'",
	}

	context := map[string]interface{}{
		"valid_values": []string{"openstack", "aws", "gcp"},
	}

	suggestions := rule.Generate(issue, context)

	if len(suggestions) == 0 {
		t.Error("Expected typo suggestions")
	}

	// Should suggest "openstack" for "openstck"
	found := false
	for _, s := range suggestions {
		if s == "Did you mean \"openstack\"?" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected suggestion for 'openstack', got: %v", suggestions)
	}
}

func TestTypoSuggestionRule_GenerateNoContext(t *testing.T) {
	rule := &TypoSuggestionRule{}

	issue := &ValidationIssue{
		Field:   "provider",
		Message: "invalid value",
	}

	suggestions := rule.Generate(issue, nil)

	if len(suggestions) != 0 {
		t.Error("Expected no suggestions without context")
	}
}

func TestContextSuggestionRule_Name(t *testing.T) {
	rule := &ContextSuggestionRule{}
	if rule.Name() != "context" {
		t.Errorf("Expected name %q, got %q", "context", rule.Name())
	}
}

func TestContextSuggestionRule_Generate(t *testing.T) {
	rule := &ContextSuggestionRule{}

	tests := []struct {
		field    string
		expected string
	}{
		{"cluster.email", "Ensure email is in format: user@example.com"},
		{"cluster.url", "Ensure URL includes protocol (http:// or https://)"},
		{"network.cidr", "Use CIDR notation (e.g., 10.0.0.0/16)"},
		{"network.ip", "Ensure IP address is in valid format (e.g., 192.168.1.1)"},
		{"service.port", "Port must be between 1 and 65535"},
		{"cluster.name", "Use alphanumeric characters, hyphens, and underscores only"},
		{"kubernetes.version", "Use semantic versioning format (e.g., 1.2.3)"},
		{"node.count", "Value must be a positive number"},
		{"feature.enabled", "Value must be true or false"},
	}

	for _, tt := range tests {
		issue := &ValidationIssue{
			Field:   tt.field,
			Message: "validation failed",
		}

		suggestions := rule.Generate(issue, nil)

		found := false
		for _, s := range suggestions {
			if s == tt.expected {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Expected suggestion %q for field %q, got: %v", tt.expected, tt.field, suggestions)
		}
	}
}

func TestContextSuggestionRule_GenerateWithErrorCode(t *testing.T) {
	rule := &ContextSuggestionRule{}

	tests := []struct {
		code     string
		expected string
	}{
		{"E001", "This field is required and cannot be empty"},
		{"E002", "Check the allowed values for this field"},
		{"E003", "Verify CIDR notation is correct"},
		{"E004", "Verify IP address format"},
		{"E005", "Ensure IP is within the specified subnet range"},
		{"E006", "Check field dependencies and requirements"},
		{"E007", "Verify value is within acceptable range"},
	}

	for _, tt := range tests {
		issue := &ValidationIssue{
			Field:   "test.field",
			Message: "validation failed",
			Code:    tt.code,
		}

		suggestions := rule.Generate(issue, nil)

		found := false
		for _, s := range suggestions {
			if s == tt.expected {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Expected suggestion %q for code %q, got: %v", tt.expected, tt.code, suggestions)
		}
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		s1       string
		s2       string
		expected int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "a", 1},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"abc", "def", 3},
		{"kitten", "sitting", 3},
		{"openstack", "openstck", 1},
	}

	for _, tt := range tests {
		distance := levenshteinDistance(tt.s1, tt.s2)
		if distance != tt.expected {
			t.Errorf("levenshteinDistance(%q, %q) = %d, expected %d", tt.s1, tt.s2, distance, tt.expected)
		}
	}
}

func TestExtractInvalidValue(t *testing.T) {
	tests := []struct {
		message  string
		expected string
	}{
		{"invalid value 'xyz'", "xyz"},
		{"value 'abc' is invalid", "abc"},
		{"'test' is not valid", "test"},
		{"no value here", ""},
		{"", ""},
	}

	for _, tt := range tests {
		issue := &ValidationIssue{
			Message: tt.message,
		}

		value := extractInvalidValue(issue)
		if value != tt.expected {
			t.Errorf("extractInvalidValue(%q) = %q, expected %q", tt.message, value, tt.expected)
		}
	}
}

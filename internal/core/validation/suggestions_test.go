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
		t.Error("expected default rules to be registered")
	}
}

func TestSuggestionEngine_AddRule(t *testing.T) {
	engine := NewSuggestionEngine()
	initialCount := len(engine.rules)

	rule := &mockSuggestionRule{name: "test"}
	engine.AddRule(rule)

	if len(engine.rules) != initialCount+1 {
		t.Errorf("expected %d rules, got %d", initialCount+1, len(engine.rules))
	}
}

func TestSuggestionEngine_EnhanceResult(t *testing.T) {
	engine := NewSuggestionEngine()

	result := &ValidationResult{
		Valid: false,
		Errors: []*ValidationIssue{
			{
				Field:   "email",
				Message: "invalid email format",
			},
		},
	}

	context := make(map[string]interface{})
	engine.EnhanceResult(result, context)

	// Should have suggestions added from context rule (email field)
	if len(result.Errors[0].Suggestions) == 0 {
		t.Error("expected suggestions to be added")
	}
}

func TestSuggestionEngine_EnhanceResultNil(t *testing.T) {
	engine := NewSuggestionEngine()
	context := make(map[string]interface{})

	// Should not panic with nil result
	engine.EnhanceResult(nil, context)
}

func TestTypoSuggestionRule_Name(t *testing.T) {
	rule := &TypoSuggestionRule{}
	if rule.Name() != "typo" {
		t.Errorf("expected name 'typo', got %q", rule.Name())
	}
}

func TestTypoSuggestionRule_Generate(t *testing.T) {
	rule := &TypoSuggestionRule{}

	tests := []struct {
		name               string
		issue              *ValidationIssue
		context            map[string]interface{}
		wantMinSuggestions int
		wantMaxSuggestions int
	}{
		{
			name: "close match",
			issue: &ValidationIssue{
				Field:   "test",
				Message: "invalid value 'tst'",
			},
			context: map[string]interface{}{
				"valid_values": []string{"test", "prod", "dev"},
			},
			wantMinSuggestions: 1, // Should suggest at least "test"
			wantMaxSuggestions: 3, // May suggest up to 3 close matches
		},
		{
			name: "no close match",
			issue: &ValidationIssue{
				Field:   "test",
				Message: "invalid value 'abcdefghijk'",
			},
			context: map[string]interface{}{
				"valid_values": []string{"test", "prod", "dev"},
			},
			wantMinSuggestions: 0,
			wantMaxSuggestions: 0,
		},
		{
			name: "no valid values",
			issue: &ValidationIssue{
				Field:   "test",
				Message: "invalid value 'test'",
			},
			context:            map[string]interface{}{},
			wantMinSuggestions: 0,
			wantMaxSuggestions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := rule.Generate(tt.issue, tt.context)
			if len(suggestions) < tt.wantMinSuggestions || len(suggestions) > tt.wantMaxSuggestions {
				t.Errorf("expected %d-%d suggestions, got %d: %v", tt.wantMinSuggestions, tt.wantMaxSuggestions, len(suggestions), suggestions)
			}
		})
	}
}

func TestContextSuggestionRule_Name(t *testing.T) {
	rule := &ContextSuggestionRule{}
	if rule.Name() != "context" {
		t.Errorf("expected name 'context', got %q", rule.Name())
	}
}

func TestContextSuggestionRule_Generate(t *testing.T) {
	rule := &ContextSuggestionRule{}

	tests := []struct {
		name            string
		issue           *ValidationIssue
		wantSuggestions bool
	}{
		{
			name: "email field",
			issue: &ValidationIssue{
				Field:   "email",
				Message: "invalid email",
			},
			wantSuggestions: true,
		},
		{
			name: "url field",
			issue: &ValidationIssue{
				Field:   "api_url",
				Message: "invalid url",
			},
			wantSuggestions: true,
		},
		{
			name: "port field",
			issue: &ValidationIssue{
				Field:   "port",
				Message: "invalid port",
			},
			wantSuggestions: true,
		},
		{
			name: "cluster name field",
			issue: &ValidationIssue{
				Field:   "cluster_name",
				Message: "invalid name",
			},
			wantSuggestions: true,
		},
		{
			name: "generic field",
			issue: &ValidationIssue{
				Field:   "other",
				Message: "invalid value",
			},
			wantSuggestions: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context := make(map[string]interface{})
			suggestions := rule.Generate(tt.issue, context)
			hasSuggestions := len(suggestions) > 0
			if hasSuggestions != tt.wantSuggestions {
				t.Errorf("expected suggestions=%v, got %d suggestions: %v", tt.wantSuggestions, len(suggestions), suggestions)
			}
		})
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
		{"test", "tst", 1},
		{"test", "best", 1},
	}

	for _, tt := range tests {
		t.Run(tt.s1+"_"+tt.s2, func(t *testing.T) {
			got := levenshteinDistance(tt.s1, tt.s2)
			if got != tt.expected {
				t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tt.s1, tt.s2, got, tt.expected)
			}
		})
	}
}

func TestExtractInvalidValue(t *testing.T) {
	tests := []struct {
		name     string
		issue    *ValidationIssue
		expected string
	}{
		{
			name: "pattern: invalid value 'xyz'",
			issue: &ValidationIssue{
				Message: "invalid value 'xyz'",
			},
			expected: "xyz",
		},
		{
			name: "pattern: value 'abc' is invalid",
			issue: &ValidationIssue{
				Message: "value 'abc' is invalid",
			},
			expected: "abc",
		},
		{
			name: "pattern: 'test' is not valid",
			issue: &ValidationIssue{
				Message: "'test' is not valid",
			},
			expected: "test",
		},
		{
			name: "no pattern match",
			issue: &ValidationIssue{
				Message: "something went wrong",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractInvalidValue(tt.issue)
			if got != tt.expected {
				t.Errorf("extractInvalidValue() = %q, want %q", got, tt.expected)
			}
		})
	}
}

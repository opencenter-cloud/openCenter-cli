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
	"sort"
	"strings"
	"sync"
)

// SuggestionRule defines the interface for suggestion generation rules.
type SuggestionRule interface {
	// Name returns the unique name of the rule.
	Name() string
	// Generate generates suggestions for a validation issue.
	Generate(issue *ValidationIssue, context map[string]interface{}) []string
}

// SuggestionEngine generates helpful suggestions for validation errors.
type SuggestionEngine struct {
	mu    sync.RWMutex
	rules []SuggestionRule
}

// NewSuggestionEngine creates a new suggestion engine with default rules.
func NewSuggestionEngine() *SuggestionEngine {
	engine := &SuggestionEngine{
		rules: []SuggestionRule{},
	}

	// Register default rules
	engine.AddRule(&TypoSuggestionRule{})
	engine.AddRule(&ContextSuggestionRule{})

	return engine
}

// AddRule adds a suggestion rule to the engine.
func (e *SuggestionEngine) AddRule(rule SuggestionRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append(e.rules, rule)
}

// EnhanceResult enhances a validation result with suggestions.
func (e *SuggestionEngine) EnhanceResult(result *ValidationResult, context map[string]interface{}) {
	if result == nil {
		return
	}

	// Enhance errors
	for _, issue := range result.Errors {
		e.enhanceIssue(issue, context)
	}

	// Enhance warnings
	for _, issue := range result.Warnings {
		e.enhanceIssue(issue, context)
	}
}

// enhanceIssue enhances a single validation issue with suggestions.
func (e *SuggestionEngine) enhanceIssue(issue *ValidationIssue, context map[string]interface{}) {
	if issue == nil {
		return
	}

	// Collect suggestions from all rules
	suggestions := make(map[string]bool)

	// Keep existing suggestions
	for _, s := range issue.Suggestions {
		suggestions[s] = true
	}

	// Generate new suggestions from rules
	e.mu.RLock()
	rules := make([]SuggestionRule, len(e.rules))
	copy(rules, e.rules)
	e.mu.RUnlock()

	for _, rule := range rules {
		newSuggestions := rule.Generate(issue, context)
		for _, s := range newSuggestions {
			suggestions[s] = true
		}
	}

	// Convert back to slice
	issue.Suggestions = make([]string, 0, len(suggestions))
	for s := range suggestions {
		issue.Suggestions = append(issue.Suggestions, s)
	}

	// Sort for consistent output
	sort.Strings(issue.Suggestions)
}

// TypoSuggestionRule suggests corrections for typos using Levenshtein distance.
type TypoSuggestionRule struct{}

// Name returns the rule name.
func (r *TypoSuggestionRule) Name() string {
	return "typo"
}

// Generate generates typo correction suggestions.
func (r *TypoSuggestionRule) Generate(issue *ValidationIssue, context map[string]interface{}) []string {
	var suggestions []string

	// Check if context contains valid values for comparison
	validValues, ok := context["valid_values"].([]string)
	if !ok || len(validValues) == 0 {
		return suggestions
	}

	// Extract the invalid value from the issue
	invalidValue := extractInvalidValue(issue)
	if invalidValue == "" {
		return suggestions
	}

	// Find close matches using Levenshtein distance
	const maxDistance = 3
	matches := make([]struct {
		value    string
		distance int
	}, 0)

	for _, valid := range validValues {
		distance := levenshteinDistance(strings.ToLower(invalidValue), strings.ToLower(valid))
		if distance <= maxDistance {
			matches = append(matches, struct {
				value    string
				distance int
			}{valid, distance})
		}
	}

	// Sort by distance
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].distance < matches[j].distance
	})

	// Generate suggestions for top matches
	for i, match := range matches {
		if i >= 3 { // Limit to top 3 suggestions
			break
		}
		suggestions = append(suggestions, fmt.Sprintf("Did you mean %q?", match.value))
	}

	return suggestions
}

// ContextSuggestionRule generates context-aware suggestions based on field type.
type ContextSuggestionRule struct{}

// Name returns the rule name.
func (r *ContextSuggestionRule) Name() string {
	return "context"
}

// Generate generates context-aware suggestions.
func (r *ContextSuggestionRule) Generate(issue *ValidationIssue, context map[string]interface{}) []string {
	var suggestions []string

	// Generate suggestions based on field name patterns
	field := strings.ToLower(issue.Field)

	switch {
	case strings.Contains(field, "email"):
		suggestions = append(suggestions, "Ensure email is in format: user@example.com")
	case strings.Contains(field, "url") || strings.Contains(field, "endpoint"):
		suggestions = append(suggestions, "Ensure URL includes protocol (http:// or https://)")
	case strings.Contains(field, "cidr") || strings.Contains(field, "subnet"):
		suggestions = append(suggestions, "Use CIDR notation (e.g., 10.0.0.0/16)")
	case strings.Contains(field, "ip") || strings.Contains(field, "address"):
		suggestions = append(suggestions, "Ensure IP address is in valid format (e.g., 192.168.1.1)")
	case strings.Contains(field, "port"):
		suggestions = append(suggestions, "Port must be between 1 and 65535")
	case strings.Contains(field, "name") || strings.Contains(field, "cluster"):
		suggestions = append(suggestions, "Use alphanumeric characters, hyphens, and underscores only")
	case strings.Contains(field, "version"):
		suggestions = append(suggestions, "Use semantic versioning format (e.g., 1.2.3)")
	case strings.Contains(field, "count") || strings.Contains(field, "size"):
		suggestions = append(suggestions, "Value must be a positive number")
	case strings.Contains(field, "enabled") || strings.Contains(field, "enable"):
		suggestions = append(suggestions, "Value must be true or false")
	}

	// Add suggestions based on error code
	if issue.Code != "" {
		switch issue.Code {
		case "E001":
			suggestions = append(suggestions, "This field is required and cannot be empty")
		case "E002":
			suggestions = append(suggestions, "Check the allowed values for this field")
		case "E003":
			suggestions = append(suggestions, "Verify CIDR notation is correct")
		case "E004":
			suggestions = append(suggestions, "Verify IP address format")
		case "E005":
			suggestions = append(suggestions, "Ensure IP is within the specified subnet range")
		case "E006":
			suggestions = append(suggestions, "Check field dependencies and requirements")
		case "E007":
			suggestions = append(suggestions, "Verify value is within acceptable range")
		}
	}

	return suggestions
}

// extractInvalidValue attempts to extract the invalid value from the issue message.
func extractInvalidValue(issue *ValidationIssue) string {
	// Try to extract value from common message patterns
	message := issue.Message

	// Pattern: "invalid value 'xyz'"
	if idx := strings.Index(message, "invalid value '"); idx != -1 {
		start := idx + len("invalid value '")
		if end := strings.Index(message[start:], "'"); end != -1 {
			return message[start : start+end]
		}
	}

	// Pattern: "value 'xyz' is invalid"
	if idx := strings.Index(message, "value '"); idx != -1 {
		start := idx + len("value '")
		if end := strings.Index(message[start:], "'"); end != -1 {
			return message[start : start+end]
		}
	}

	// Pattern: "'xyz' is not valid"
	if idx := strings.Index(message, "'"); idx != -1 {
		start := idx + 1
		if end := strings.Index(message[start:], "'"); end != -1 {
			return message[start : start+end]
		}
	}

	return ""
}

// levenshteinDistance calculates the Levenshtein distance between two strings.
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create matrix
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

// min returns the minimum of three integers.
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

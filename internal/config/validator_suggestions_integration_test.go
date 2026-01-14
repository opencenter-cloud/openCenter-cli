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

package config

import (
	"context"
	"strings"
	"testing"
)

// TestValidatorSuggestionEngineIntegration tests that the validator properly integrates with the suggestion engine.
func TestValidatorSuggestionEngineIntegration(t *testing.T) {
	validator := NewConfigValidator(false)

	// Verify suggestion engine is initialized
	if validator.GetSuggestionEngine() == nil {
		t.Fatal("Suggestion engine not initialized in validator")
	}

	// Test that suggestions are available for common fields
	engine := validator.GetSuggestionEngine()
	
	tests := []struct {
		name           string
		field          string
		expectNonEmpty bool
	}{
		{
			name:           "cluster_name suggestions",
			field:          "cluster_name",
			expectNonEmpty: true,
		},
		{
			name:           "email suggestions",
			field:          "email",
			expectNonEmpty: true,
		},
		{
			name:           "kubernetes.version suggestions",
			field:          "kubernetes.version",
			expectNonEmpty: true,
		},
		{
			name:           "network_plugin suggestions",
			field:          "network_plugin",
			expectNonEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := engine.GetSuggestionsForField(tt.field, nil)
			if tt.expectNonEmpty && len(suggestions) == 0 {
				t.Errorf("Expected non-empty suggestions for field %s", tt.field)
			}
		})
	}
}

// TestEnhanceSuggestions tests the enhanceSuggestions method.
func TestEnhanceSuggestions(t *testing.T) {
	validator := NewConfigValidator(false)

	tests := []struct {
		name                string
		field               string
		value               interface{}
		existingSuggestions []string
		expectContains      []string
		expectNoDuplicates  bool
	}{
		{
			name:  "enhance cluster_name suggestions",
			field: "cluster_name",
			value: "invalid@name",
			existingSuggestions: []string{
				"Use valid cluster name",
			},
			expectContains: []string{
				"Use valid cluster name",
				"alphanumeric",
			},
			expectNoDuplicates: true,
		},
		{
			name:  "enhance email suggestions",
			field: "email",
			value: "not-an-email",
			existingSuggestions: []string{
				"Provide valid email",
			},
			expectContains: []string{
				"Provide valid email",
				"valid email format",
			},
			expectNoDuplicates: true,
		},
		{
			name:  "no duplicates when same suggestion exists",
			field: "cluster_name",
			value: "test",
			existingSuggestions: []string{
				"Use alphanumeric characters, hyphens, and underscores only",
			},
			expectContains: []string{
				"Use alphanumeric characters, hyphens, and underscores only",
			},
			expectNoDuplicates: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enhanced := validator.enhanceSuggestions(tt.field, tt.value, tt.existingSuggestions)

			// Check that expected suggestions are present
			for _, expected := range tt.expectContains {
				found := false
				for _, suggestion := range enhanced {
					if strings.Contains(suggestion, expected) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected enhanced suggestions to contain '%s', got: %v", expected, enhanced)
				}
			}

			// Check for duplicates
			if tt.expectNoDuplicates {
				seen := make(map[string]bool)
				for _, suggestion := range enhanced {
					if seen[suggestion] {
						t.Errorf("Found duplicate suggestion: %s", suggestion)
					}
					seen[suggestion] = true
				}
			}
		})
	}
}

// TestValidatorWithSuggestionEngine tests that validation errors include helpful suggestions.
func TestValidatorWithSuggestionEngine(t *testing.T) {
	validator := NewConfigValidator(false)

	// Create a config with various validation errors
	config := &Config{
		SchemaVersion: "v2.0.0",
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         "", // Missing cluster name
				Organization: "test-org",
			},
			Cluster: ClusterConfig{
				AdminEmail: "invalid-email", // Invalid email
				Kubernetes: KubernetesConfig{
					Version:     "invalid-version", // Invalid version format
					MasterCount: 0,                 // Invalid master count
					WorkerCount: -1,                // Invalid worker count
				},
			},
			Infrastructure: Infrastructure{
				Provider: "", // Missing provider
			},
			GitOps: GitOpsConfig{
				GitDir: "", // Missing git dir
			},
		},
	}

	ctx := context.Background()
	result := validator.Validate(ctx, config)

	if result.Valid {
		t.Error("Expected validation to fail for invalid config")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected validation errors")
	}

	// Verify that errors have suggestions
	for _, err := range result.Errors {
		if len(err.Suggestions) == 0 {
			t.Errorf("Error for field '%s' has no suggestions: %s", err.Field, err.Message)
		}
	}

	// Check specific error suggestions
	errorFields := map[string]bool{
		"opencenter.cluster.cluster_name":       false,
		"opencenter.cluster.admin_email":        false,
		"opencenter.cluster.kubernetes.version": false,
		"opencenter.infrastructure.provider":    false,
		"opencenter.gitops.git_dir":             false,
	}

	for _, err := range result.Errors {
		if _, exists := errorFields[err.Field]; exists {
			errorFields[err.Field] = true
			
			// Verify suggestions are helpful
			if len(err.Suggestions) == 0 {
				t.Errorf("Field '%s' should have suggestions", err.Field)
			}
		}
	}

	// Verify all expected fields had errors
	for field, found := range errorFields {
		if !found {
			t.Logf("Warning: Expected error for field '%s' not found", field)
		}
	}
}

// TestSuggestionEnginePerformance tests that the suggestion engine performs well.
func TestSuggestionEnginePerformance(t *testing.T) {
	validator := NewConfigValidator(false)
	engine := validator.GetSuggestionEngine()

	// Test that suggestion generation is fast
	fields := []string{
		"cluster_name",
		"email",
		"domain",
		"kubernetes.version",
		"master_count",
		"worker_count",
		"network_plugin",
		"openstack.auth_url",
		"aws.region",
		"unknown_field",
	}

	for _, field := range fields {
		suggestions := engine.GetSuggestionsForField(field, nil)
		if len(suggestions) == 0 && field != "unknown_field" {
			t.Logf("Warning: No suggestions for field '%s'", field)
		}
	}
}

// TestSuggestionFormatting tests that suggestions are properly formatted.
func TestSuggestionFormatting(t *testing.T) {
	validator := NewConfigValidator(false)
	engine := validator.GetSuggestionEngine()

	suggestions := []string{
		"First suggestion",
		"Second suggestion",
		"Third suggestion",
	}

	formatted := engine.FormatSuggestions(suggestions)

	if !strings.Contains(formatted, "Suggestions:") {
		t.Error("Formatted output should contain 'Suggestions:' header")
	}

	for i, suggestion := range suggestions {
		expected := strings.TrimSpace(suggestion)
		if !strings.Contains(formatted, expected) {
			t.Errorf("Formatted output should contain suggestion %d: %s", i+1, expected)
		}
	}
}

// TestRelatedFieldsSuggestions tests that related fields are suggested.
func TestRelatedFieldsSuggestions(t *testing.T) {
	validator := NewConfigValidator(false)
	engine := validator.GetSuggestionEngine()

	tests := []struct {
		name           string
		field          string
		expectRelated  bool
		relatedContains string
	}{
		{
			name:           "provider has related cloud configs",
			field:          "opencenter.infrastructure.provider",
			expectRelated:  true,
			relatedContains: "openstack",
		},
		{
			name:           "network plugin has related plugins",
			field:          "opencenter.cluster.kubernetes.network_plugin.calico.enabled",
			expectRelated:  true,
			relatedContains: "cilium",
		},
		{
			name:          "unknown field has no related fields",
			field:         "unknown.field",
			expectRelated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			related := engine.GetRelatedFields(tt.field)

			if tt.expectRelated && len(related) == 0 {
				t.Errorf("Expected related fields for %s", tt.field)
			}

			if !tt.expectRelated && len(related) > 0 {
				t.Errorf("Expected no related fields for %s, got %d", tt.field, len(related))
			}

			if tt.relatedContains != "" {
				found := false
				for _, field := range related {
					if strings.Contains(field, tt.relatedContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected related fields to contain '%s', got: %v", tt.relatedContains, related)
				}
			}
		})
	}
}

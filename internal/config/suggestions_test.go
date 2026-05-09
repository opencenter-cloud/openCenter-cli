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
	"strings"
	"testing"
)

func TestNewSuggestionEngine(t *testing.T) {
	engine := NewSuggestionEngine()
	if engine == nil {
		t.Fatal("NewSuggestionEngine returned nil")
	}
	if engine.fieldSuggestions == nil {
		t.Error("fieldSuggestions map not initialized")
	}
	if engine.typeSuggestions == nil {
		t.Error("typeSuggestions map not initialized")
	}
}

func TestGetSuggestionsForField(t *testing.T) {
	engine := NewSuggestionEngine()

	tests := []struct {
		name           string
		field          string
		value          interface{}
		expectNonZero  bool
		expectContains string
	}{
		{
			name:           "cluster_name field",
			field:          "cluster_name",
			value:          "test-cluster",
			expectNonZero:  true,
			expectContains: "alphanumeric",
		},
		{
			name:           "email field",
			field:          "email",
			value:          "invalid-email",
			expectNonZero:  true,
			expectContains: "valid email format",
		},
		{
			name:           "domain field",
			field:          "domain",
			value:          "example.com",
			expectNonZero:  true,
			expectContains: "valid domain format",
		},
		{
			name:           "kubernetes.version field",
			field:          "kubernetes.version",
			value:          "1.31.4",
			expectNonZero:  true,
			expectContains: "semantic versioning",
		},
		{
			name:           "master_count field",
			field:          "master_count",
			value:          0,
			expectNonZero:  true,
			expectContains: "odd numbers",
		},
		{
			name:           "network_plugin field",
			field:          "network_plugin",
			value:          nil,
			expectNonZero:  true,
			expectContains: "Calico",
		},
		{
			name:           "openstack.auth_url field",
			field:          "openstack.auth_url",
			value:          "https://keystone.example.com/v3/",
			expectNonZero:  true,
			expectContains: "Keystone",
		},
		{
			name:           "unknown field with password",
			field:          "some_password",
			value:          "secret123",
			expectNonZero:  true,
			expectContains: "SOPS",
		},
		{
			name:           "unknown field with url",
			field:          "some_url",
			value:          "http://example.com",
			expectNonZero:  true,
			expectContains: "properly formatted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := engine.GetSuggestionsForField(tt.field, tt.value)

			if tt.expectNonZero && len(suggestions) == 0 {
				t.Errorf("Expected non-zero suggestions for field %s, got none", tt.field)
			}

			if tt.expectContains != "" {
				found := false
				for _, suggestion := range suggestions {
					if strings.Contains(suggestion, tt.expectContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestions to contain '%s', got: %v", tt.expectContains, suggestions)
				}
			}
		})
	}
}

func TestGetSuggestionsForType(t *testing.T) {
	engine := NewSuggestionEngine()

	tests := []struct {
		name           string
		errorType      string
		expectNonZero  bool
		expectContains string
	}{
		{
			name:           "validation error type",
			errorType:      "validation",
			expectNonZero:  true,
			expectContains: "configuration file",
		},
		{
			name:           "provider error type",
			errorType:      "provider",
			expectNonZero:  true,
			expectContains: "provider-specific",
		},
		{
			name:           "network error type",
			errorType:      "network",
			expectNonZero:  true,
			expectContains: "network configuration",
		},
		{
			name:           "service error type",
			errorType:      "service",
			expectNonZero:  true,
			expectContains: "service dependencies",
		},
		{
			name:           "secret error type",
			errorType:      "secret",
			expectNonZero:  true,
			expectContains: "SOPS",
		},
		{
			name:          "unknown error type",
			errorType:     "unknown_type",
			expectNonZero: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := engine.GetSuggestionsForType(tt.errorType)

			if tt.expectNonZero && len(suggestions) == 0 {
				t.Errorf("Expected non-zero suggestions for type %s, got none", tt.errorType)
			}

			if !tt.expectNonZero && len(suggestions) > 0 {
				t.Errorf("Expected zero suggestions for type %s, got %d", tt.errorType, len(suggestions))
			}

			if tt.expectContains != "" {
				found := false
				for _, suggestion := range suggestions {
					if strings.Contains(suggestion, tt.expectContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestions to contain '%s', got: %v", tt.expectContains, suggestions)
				}
			}
		})
	}
}

func TestGetSuggestionsForMissingField(t *testing.T) {
	engine := NewSuggestionEngine()

	tests := []struct {
		name           string
		field          string
		expectContains []string
	}{
		{
			name:  "missing cluster_name",
			field: "cluster_name",
			expectContains: []string{
				"Add the required field",
				"alphanumeric",
			},
		},
		{
			name:  "missing email",
			field: "email",
			expectContains: []string{
				"Add the required field",
				"valid email format",
			},
		},
		{
			name:  "missing unknown field",
			field: "unknown_field",
			expectContains: []string{
				"Add the required field",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := engine.GetSuggestionsForMissingField(tt.field)

			if len(suggestions) == 0 {
				t.Errorf("Expected non-zero suggestions for missing field %s", tt.field)
			}

			for _, expected := range tt.expectContains {
				found := false
				for _, suggestion := range suggestions {
					if strings.Contains(suggestion, expected) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestions to contain '%s', got: %v", expected, suggestions)
				}
			}
		})
	}
}

func TestGetSuggestionsForInvalidValue(t *testing.T) {
	engine := NewSuggestionEngine()

	tests := []struct {
		name           string
		field          string
		value          interface{}
		expectedFormat string
		expectContains []string
	}{
		{
			name:           "invalid cluster name",
			field:          "cluster_name",
			value:          "Invalid@Name!",
			expectedFormat: "alphanumeric-with-hyphens",
			expectContains: []string{
				"invalid",
				"alphanumeric",
			},
		},
		{
			name:           "invalid email",
			field:          "email",
			value:          "not-an-email",
			expectedFormat: "user@example.com",
			expectContains: []string{
				"invalid",
				"valid email format",
			},
		},
		{
			name:           "invalid count",
			field:          "master_count",
			value:          -1,
			expectedFormat: "positive integer",
			expectContains: []string{
				"invalid",
				"odd numbers",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := engine.GetSuggestionsForInvalidValue(tt.field, tt.value, tt.expectedFormat)

			if len(suggestions) == 0 {
				t.Errorf("Expected non-zero suggestions for invalid value in field %s", tt.field)
			}

			for _, expected := range tt.expectContains {
				found := false
				for _, suggestion := range suggestions {
					if strings.Contains(suggestion, expected) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestions to contain '%s', got: %v", expected, suggestions)
				}
			}
		})
	}
}

func TestGetSuggestionsForConflict(t *testing.T) {
	engine := NewSuggestionEngine()

	tests := []struct {
		name           string
		field1         string
		field2         string
		expectContains []string
	}{
		{
			name:   "conflicting network plugins",
			field1: "calico.enabled",
			field2: "cilium.enabled",
			expectContains: []string{
				"conflicting",
				"only one",
			},
		},
		{
			name:   "conflicting storage backends",
			field1: "swift_config",
			field2: "s3_config",
			expectContains: []string{
				"conflicting",
				"only one",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := engine.GetSuggestionsForConflict(tt.field1, tt.field2)

			if len(suggestions) == 0 {
				t.Errorf("Expected non-zero suggestions for conflict between %s and %s", tt.field1, tt.field2)
			}

			for _, expected := range tt.expectContains {
				found := false
				for _, suggestion := range suggestions {
					if strings.Contains(suggestion, expected) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestions to contain '%s', got: %v", expected, suggestions)
				}
			}
		})
	}
}

func TestGenerateContextAwareSuggestions(t *testing.T) {
	engine := NewSuggestionEngine()

	tests := []struct {
		name           string
		field          string
		value          interface{}
		expectContains string
	}{
		{
			name:           "password field",
			field:          "admin_password",
			value:          "secret",
			expectContains: "SOPS",
		},
		{
			name:           "secret field",
			field:          "api_secret",
			value:          "token",
			expectContains: "SOPS",
		},
		{
			name:           "token field",
			field:          "access_token",
			value:          "abc123",
			expectContains: "SOPS",
		},
		{
			name:           "url field",
			field:          "auth_url",
			value:          "http://example.com",
			expectContains: "properly formatted",
		},
		{
			name:           "endpoint field",
			field:          "api_endpoint",
			value:          "https://api.example.com",
			expectContains: "protocol",
		},
		{
			name:           "email field",
			field:          "admin_email",
			value:          "admin@example.com",
			expectContains: "valid email format",
		},
		{
			name:           "count field",
			field:          "replica_count",
			value:          3,
			expectContains: "positive integers",
		},
		{
			name:           "path field",
			field:          "config_path",
			value:          "/etc/config",
			expectContains: "absolute or relative",
		},
		{
			name:           "region field",
			field:          "aws_region",
			value:          "us-east-1",
			expectContains: "valid cloud provider region",
		},
		{
			name:           "enabled field",
			field:          "feature_enabled",
			value:          true,
			expectContains: "true to enable",
		},
		{
			name:           "unknown field",
			field:          "custom_field",
			value:          "value",
			expectContains: "documentation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := engine.generateContextAwareSuggestions(tt.field, tt.value)

			if len(suggestions) == 0 {
				t.Errorf("Expected non-zero suggestions for field %s", tt.field)
			}

			found := false
			for _, suggestion := range suggestions {
				if strings.Contains(suggestion, tt.expectContains) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected suggestions to contain '%s', got: %v", tt.expectContains, suggestions)
			}
		})
	}
}

func TestFormatSuggestions(t *testing.T) {
	engine := NewSuggestionEngine()

	tests := []struct {
		name           string
		suggestions    []string
		expectContains []string
		expectEmpty    bool
	}{
		{
			name:        "empty suggestions",
			suggestions: []string{},
			expectEmpty: true,
		},
		{
			name: "single suggestion",
			suggestions: []string{
				"Use alphanumeric characters only",
			},
			expectContains: []string{
				"Suggestions:",
				"1. Use alphanumeric characters only",
			},
		},
		{
			name: "multiple suggestions",
			suggestions: []string{
				"First suggestion",
				"Second suggestion",
				"Third suggestion",
			},
			expectContains: []string{
				"Suggestions:",
				"1. First suggestion",
				"2. Second suggestion",
				"3. Third suggestion",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted := engine.FormatSuggestions(tt.suggestions)

			if tt.expectEmpty {
				if formatted != "" {
					t.Errorf("Expected empty string, got: %s", formatted)
				}
				return
			}

			for _, expected := range tt.expectContains {
				if !strings.Contains(formatted, expected) {
					t.Errorf("Expected formatted output to contain '%s', got: %s", expected, formatted)
				}
			}
		})
	}
}

func TestGetRelatedFields(t *testing.T) {
	engine := NewSuggestionEngine()

	tests := []struct {
		name           string
		field          string
		expectNonZero  bool
		expectContains string
	}{
		{
			name:           "provider field",
			field:          "opencenter.infrastructure.provider",
			expectNonZero:  true,
			expectContains: "openstack",
		},
		{
			name:           "network plugin field",
			field:          "opencenter.cluster.kubernetes.network_plugin.calico.enabled",
			expectNonZero:  true,
			expectContains: "cilium",
		},
		{
			name:           "backend type field",
			field:          "opentofu.backend.type",
			expectNonZero:  true,
			expectContains: "s3",
		},
		{
			name:           "loki storage type field",
			field:          "opencenter.services.loki.storage_type",
			expectNonZero:  true,
			expectContains: "swift",
		},
		{
			name:          "unknown field",
			field:         "unknown.field",
			expectNonZero: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			related := engine.GetRelatedFields(tt.field)

			if tt.expectNonZero && len(related) == 0 {
				t.Errorf("Expected non-zero related fields for %s, got none", tt.field)
			}

			if !tt.expectNonZero && len(related) > 0 {
				t.Errorf("Expected zero related fields for %s, got %d", tt.field, len(related))
			}

			if tt.expectContains != "" {
				found := false
				for _, field := range related {
					if strings.Contains(field, tt.expectContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected related fields to contain '%s', got: %v", tt.expectContains, related)
				}
			}
		})
	}
}

// Benchmark tests
func BenchmarkGetSuggestionsForField(b *testing.B) {
	engine := NewSuggestionEngine()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.GetSuggestionsForField("cluster_name", "test-cluster")
	}
}

func BenchmarkGetSuggestionsForType(b *testing.B) {
	engine := NewSuggestionEngine()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.GetSuggestionsForType("validation")
	}
}

func BenchmarkGenerateContextAwareSuggestions(b *testing.B) {
	engine := NewSuggestionEngine()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.generateContextAwareSuggestions("admin_password", "secret")
	}
}

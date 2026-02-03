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

package validators

import (
	"context"
	"strings"
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
)

func TestServiceValidator_Name(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		want        string
	}{
		{
			name:        "with service name",
			serviceName: "loki",
			want:        "service:loki",
		},
		{
			name:        "with hyphenated service name",
			serviceName: "cert-manager",
			want:        "service:cert-manager",
		},
		{
			name:        "empty service name",
			serviceName: "",
			want:        "service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewServiceValidator(tt.serviceName)
			if got := v.Name(); got != tt.want {
				t.Errorf("Name() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServiceValidator_Validate_ValidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config interface{}
	}{
		{
			name: "valid map config",
			config: map[string]interface{}{
				"enabled":   true,
				"namespace": "loki-system",
				"name":      "loki",
			},
		},
		{
			name: "valid BaseServiceConfig",
			config: &BaseServiceConfig{
				Enabled:   true,
				Namespace: "prometheus-system",
				Name:      "prometheus",
			},
		},
		{
			name: "valid with hyphens",
			config: map[string]interface{}{
				"enabled":   true,
				"namespace": "cert-manager-system",
				"name":      "cert-manager",
			},
		},
		{
			name: "valid with numbers",
			config: map[string]interface{}{
				"enabled":   true,
				"namespace": "service-v2",
				"name":      "service-v2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewServiceValidator("test-service")
			result, err := v.Validate(context.Background(), tt.config)

			if err != nil {
				t.Errorf("Validate() error = %v, want nil", err)
				return
			}

			if !result.Valid {
				t.Errorf("Validate() result.Valid = false, want true")
				for _, e := range result.Errors {
					t.Logf("Error: %s: %s", e.Field, e.Message)
				}
			}
		})
	}
}

func TestServiceValidator_Validate_InvalidNamespace(t *testing.T) {
	tests := []struct {
		name              string
		namespace         string
		wantError         bool
		wantErrorContains string
	}{
		{
			name:              "empty namespace",
			namespace:         "",
			wantError:         false, // Warning, not error
			wantErrorContains: "",
		},
		{
			name:              "namespace too long",
			namespace:         strings.Repeat("a", 64),
			wantError:         true,
			wantErrorContains: "too long",
		},
		{
			name:              "namespace with uppercase",
			namespace:         "Loki-System",
			wantError:         true,
			wantErrorContains: "invalid namespace format",
		},
		{
			name:              "namespace with underscore",
			namespace:         "loki_system",
			wantError:         true,
			wantErrorContains: "invalid namespace format",
		},
		{
			name:              "namespace starting with hyphen",
			namespace:         "-loki-system",
			wantError:         true,
			wantErrorContains: "invalid namespace format",
		},
		{
			name:              "namespace ending with hyphen",
			namespace:         "loki-system-",
			wantError:         true,
			wantErrorContains: "invalid namespace format",
		},
		{
			name:              "namespace with special characters",
			namespace:         "loki@system",
			wantError:         true,
			wantErrorContains: "invalid namespace format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewServiceValidator("loki")
			config := map[string]interface{}{
				"enabled":   true,
				"namespace": tt.namespace,
				"name":      "loki",
			}

			result, err := v.Validate(context.Background(), config)
			if err != nil {
				t.Errorf("Validate() error = %v, want nil", err)
				return
			}

			if tt.wantError {
				if result.Valid {
					t.Errorf("Validate() result.Valid = true, want false")
				}
				if len(result.Errors) == 0 {
					t.Errorf("Validate() no errors, want at least one")
				}
				if tt.wantErrorContains != "" {
					found := false
					for _, e := range result.Errors {
						if strings.Contains(e.Message, tt.wantErrorContains) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Validate() errors don't contain %q", tt.wantErrorContains)
					}
				}
			}
		})
	}
}

func TestServiceValidator_Validate_InvalidName(t *testing.T) {
	tests := []struct {
		name              string
		serviceName       string
		wantError         bool
		wantErrorContains string
	}{
		{
			name:              "empty name",
			serviceName:       "",
			wantError:         true,
			wantErrorContains: "required",
		},
		{
			name:              "name too long",
			serviceName:       strings.Repeat("a", 64),
			wantError:         true,
			wantErrorContains: "too long",
		},
		{
			name:              "name with uppercase",
			serviceName:       "Loki",
			wantError:         true,
			wantErrorContains: "invalid service name format",
		},
		{
			name:              "name starting with hyphen",
			serviceName:       "-loki",
			wantError:         true,
			wantErrorContains: "invalid service name format",
		},
		{
			name:              "name ending with hyphen",
			serviceName:       "loki-",
			wantError:         true,
			wantErrorContains: "invalid service name format",
		},
		{
			name:              "name with special characters",
			serviceName:       "loki@service",
			wantError:         true,
			wantErrorContains: "invalid service name format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewServiceValidator("loki")
			config := map[string]interface{}{
				"enabled":   true,
				"namespace": "loki-system",
				"name":      tt.serviceName,
			}

			result, err := v.Validate(context.Background(), config)
			if err != nil {
				t.Errorf("Validate() error = %v, want nil", err)
				return
			}

			if tt.wantError {
				if result.Valid {
					t.Errorf("Validate() result.Valid = true, want false")
				}
				if len(result.Errors) == 0 {
					t.Errorf("Validate() no errors, want at least one")
				}
				if tt.wantErrorContains != "" {
					found := false
					for _, e := range result.Errors {
						if strings.Contains(e.Message, tt.wantErrorContains) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Validate() errors don't contain %q", tt.wantErrorContains)
					}
				}
			}
		})
	}
}

func TestServiceValidator_Validate_Suggestions(t *testing.T) {
	tests := []struct {
		name              string
		config            interface{}
		wantSuggestions   bool
		suggestionKeyword string
	}{
		{
			name: "uppercase namespace",
			config: map[string]interface{}{
				"enabled":   true,
				"namespace": "Loki-System",
				"name":      "loki",
			},
			wantSuggestions:   true,
			suggestionKeyword: "lowercase",
		},
		{
			name: "underscore in namespace",
			config: map[string]interface{}{
				"enabled":   true,
				"namespace": "loki_system",
				"name":      "loki",
			},
			wantSuggestions:   true,
			suggestionKeyword: "hyphens",
		},
		{
			name: "empty name",
			config: map[string]interface{}{
				"enabled":   true,
				"namespace": "loki-system",
				"name":      "",
			},
			wantSuggestions:   true,
			suggestionKeyword: "name",
		},
		{
			name: "name too long",
			config: map[string]interface{}{
				"enabled":   true,
				"namespace": "loki-system",
				"name":      strings.Repeat("a", 64),
			},
			wantSuggestions:   true,
			suggestionKeyword: "Shorten",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewServiceValidator("loki")
			result, err := v.Validate(context.Background(), tt.config)

			if err != nil {
				t.Errorf("Validate() error = %v, want nil", err)
				return
			}

			if tt.wantSuggestions {
				hasSuggestions := false
				for _, e := range result.Errors {
					if len(e.Suggestions) > 0 {
						hasSuggestions = true
						// Check if suggestions contain the keyword
						found := false
						for _, s := range e.Suggestions {
							if strings.Contains(s, tt.suggestionKeyword) {
								found = true
								break
							}
						}
						if !found {
							t.Errorf("Suggestions don't contain keyword %q, got: %v", tt.suggestionKeyword, e.Suggestions)
						}
						break
					}
				}
				if !hasSuggestions {
					t.Errorf("Validate() no suggestions found, want suggestions")
				}
			}
		})
	}
}

func TestServiceValidator_Validate_DisabledService(t *testing.T) {
	v := NewServiceValidator("loki")
	config := map[string]interface{}{
		"enabled":   false,
		"namespace": "loki-system",
		"name":      "loki",
	}

	result, err := v.Validate(context.Background(), config)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
		return
	}

	// Disabled service should still be valid
	if !result.Valid {
		t.Errorf("Validate() result.Valid = false, want true for disabled service")
	}

	// Should have info message about disabled state
	if len(result.Info) == 0 {
		t.Errorf("Validate() no info messages, want info about disabled state")
	}
}

func TestServiceValidator_Validate_InvalidType(t *testing.T) {
	v := NewServiceValidator("loki")
	config := "invalid config type"

	result, err := v.Validate(context.Background(), config)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
		return
	}

	if result.Valid {
		t.Errorf("Validate() result.Valid = true, want false")
	}

	if len(result.Errors) == 0 {
		t.Errorf("Validate() no errors, want error for invalid type")
	}

	// Check for suggestions
	if len(result.Errors[0].Suggestions) == 0 {
		t.Errorf("Validate() no suggestions, want suggestions for invalid type")
	}
}

func TestServiceValidator_ExtensionValidator(t *testing.T) {
	// Create a mock extension validator
	extensionCalled := false
	extensionValidator := validation.NewValidatorFunc("extension", func(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
		extensionCalled = true
		result := validation.NewValidationResult()
		result.AddWarning("extension", "extension validator called")
		return result, nil
	})

	v := NewServiceValidator("loki")
	v.SetExtensionValidator(extensionValidator)

	config := map[string]interface{}{
		"enabled":   true,
		"namespace": "loki-system",
		"name":      "loki",
	}

	result, err := v.Validate(context.Background(), config)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
		return
	}

	if !extensionCalled {
		t.Errorf("Extension validator not called")
	}

	if len(result.Warnings) == 0 {
		t.Errorf("Validate() no warnings from extension validator")
	}
}

func TestServiceValidator_NameMismatchWarning(t *testing.T) {
	v := NewServiceValidator("loki")
	config := map[string]interface{}{
		"enabled":   true,
		"namespace": "loki-system",
		"name":      "prometheus", // Mismatched name
	}

	result, err := v.Validate(context.Background(), config)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
		return
	}

	// Should have warning about name mismatch
	if len(result.Warnings) == 0 {
		t.Errorf("Validate() no warnings, want warning about name mismatch")
	}

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w.Message, "doesn't match") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Validate() warnings don't contain name mismatch warning")
	}
}

func TestServiceValidator_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		config  interface{}
		wantErr bool
	}{
		{
			name: "single character namespace",
			config: map[string]interface{}{
				"enabled":   true,
				"namespace": "a",
				"name":      "loki",
			},
			wantErr: false,
		},
		{
			name: "single character name",
			config: map[string]interface{}{
				"enabled":   true,
				"namespace": "loki-system",
				"name":      "l",
			},
			wantErr: false,
		},
		{
			name: "exactly 63 character namespace",
			config: map[string]interface{}{
				"enabled":   true,
				"namespace": strings.Repeat("a", 63),
				"name":      "loki",
			},
			wantErr: false,
		},
		{
			name: "exactly 63 character name",
			config: map[string]interface{}{
				"enabled":   true,
				"namespace": "loki-system",
				"name":      strings.Repeat("a", 63),
			},
			wantErr: false,
		},
		{
			name: "namespace with multiple hyphens",
			config: map[string]interface{}{
				"enabled":   true,
				"namespace": "loki-monitoring-system",
				"name":      "loki",
			},
			wantErr: false,
		},
		{
			name: "name with underscores (valid)",
			config: map[string]interface{}{
				"enabled":   true,
				"namespace": "loki-system",
				"name":      "loki_service",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewServiceValidator("loki")
			result, err := v.Validate(context.Background(), tt.config)

			if err != nil {
				t.Errorf("Validate() error = %v, want nil", err)
				return
			}

			if tt.wantErr && result.Valid {
				t.Errorf("Validate() result.Valid = true, want false")
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("Validate() result.Valid = false, want true")
				for _, e := range result.Errors {
					t.Logf("Error: %s: %s", e.Field, e.Message)
				}
			}
		})
	}
}

func TestBaseServiceConfig(t *testing.T) {
	config := &BaseServiceConfig{
		Enabled:   true,
		Namespace: "test-namespace",
		Name:      "test-service",
	}

	if config.GetEnabled() != true {
		t.Errorf("GetEnabled() = false, want true")
	}
	if config.GetNamespace() != "test-namespace" {
		t.Errorf("GetNamespace() = %q, want %q", config.GetNamespace(), "test-namespace")
	}
	if config.GetName() != "test-service" {
		t.Errorf("GetName() = %q, want %q", config.GetName(), "test-service")
	}
}

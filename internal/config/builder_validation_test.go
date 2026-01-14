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

// TestValidationErrorsAggregatedWithContext tests that validation errors are properly
// aggregated and reported with context and suggestions.
func TestValidationErrorsAggregatedWithContext(t *testing.T) {
	// Create a builder with multiple validation errors
	builder := NewConfigBuilder("test-cluster").
		WithOrganization(""). // Missing organization
		WithProvider("").     // Missing provider
		WithMasterCount(0).   // Invalid master count
		WithWorkerCount(-1)   // Negative worker count

	// Run validation
	errors := builder.Validate()

	// Verify we got all expected errors
	if len(errors) < 4 {
		t.Errorf("Expected at least 4 validation errors, got %d", len(errors))
	}

	// Verify each error has suggestions
	for _, err := range errors {
		if len(err.Suggestions) == 0 {
			t.Errorf("Error '%s' should have suggestions but has none", err.Message)
		}
	}

	// Verify specific error messages and suggestions
	foundOrgError := false
	foundProviderError := false
	foundMasterCountError := false
	foundWorkerCountError := false

	for _, err := range errors {
		switch {
		case strings.Contains(err.Message, "organization is required"):
			foundOrgError = true
			if !containsSuggestion(err.Suggestions, "WithOrganization") {
				t.Error("Organization error should suggest using WithOrganization")
			}

		case strings.Contains(err.Message, "provider is required"):
			foundProviderError = true
			if !containsSuggestion(err.Suggestions, "WithProvider") {
				t.Error("Provider error should suggest using WithProvider")
			}

		case strings.Contains(err.Message, "master count must be at least 1"):
			foundMasterCountError = true
			if !containsSuggestion(err.Suggestions, "WithMasterCount") {
				t.Error("Master count error should suggest using WithMasterCount")
			}

		case strings.Contains(err.Message, "worker count cannot be negative"):
			foundWorkerCountError = true
			if !containsSuggestion(err.Suggestions, "WithWorkerCount") {
				t.Error("Worker count error should suggest using WithWorkerCount")
			}
		}
	}

	if !foundOrgError {
		t.Error("Expected organization error not found")
	}
	if !foundProviderError {
		t.Error("Expected provider error not found")
	}
	if !foundMasterCountError {
		t.Error("Expected master count error not found")
	}
	if !foundWorkerCountError {
		t.Error("Expected worker count error not found")
	}
}

// TestValidationReportWithStructuredErrors tests the GetValidationReport method
// which returns structured errors with full context.
func TestValidationReportWithStructuredErrors(t *testing.T) {
	builder := NewConfigBuilder("test-cluster").
		WithOrganization(""). // Missing organization
		WithProvider("invalid-provider"). // Invalid provider
		WithMasterCount(2) // Even master count

	// Get validation report
	fluentBuilder, ok := builder.(*FluentConfigBuilder)
	if !ok {
		t.Fatal("Builder should be a FluentConfigBuilder")
	}

	report := fluentBuilder.GetValidationReport()

	// Verify report structure
	if report.Valid {
		t.Error("Report should indicate validation failed")
	}

	if len(report.Errors) == 0 {
		t.Error("Report should contain errors")
	}

	// Verify each error has proper structure
	for _, err := range report.Errors {
		if err.Field == "" {
			t.Error("Structured error should have a field")
		}

		if err.Message == "" {
			t.Error("Structured error should have a message")
		}

		if err.Operation != "configuration_validation" {
			t.Errorf("Expected operation 'configuration_validation', got '%s'", err.Operation)
		}

		if len(err.Suggestions) == 0 {
			t.Errorf("Structured error for field '%s' should have suggestions", err.Field)
		}
	}
}

// TestBuildFailureWithDetailedErrors tests that Build() returns detailed error information
// when validation fails.
func TestBuildFailureWithDetailedErrors(t *testing.T) {
	builder := NewConfigBuilder("test-cluster").
		WithOrganization(""). // Missing organization
		WithProvider("")      // Missing provider

	// Attempt to build
	_, err := builder.Build()

	// Verify error is returned
	if err == nil {
		t.Fatal("Build should fail with validation errors")
	}

	// Verify error message contains useful information
	errorMsg := err.Error()

	if !strings.Contains(errorMsg, "validation failed") {
		t.Error("Error message should mention validation failure")
	}

	if !strings.Contains(errorMsg, "errors") {
		t.Error("Error message should mention number of errors")
	}

	// The error message should contain a summary
	if !strings.Contains(errorMsg, "VALIDATION") {
		t.Error("Error message should contain error type summary")
	}
}

// TestValidationErrorSuggestions tests that validation errors provide helpful suggestions.
func TestValidationErrorSuggestions(t *testing.T) {
	tests := []struct {
		name              string
		setupBuilder      func(ConfigBuilder) ConfigBuilder
		expectedField     string
		expectedSuggestion string
	}{
		{
			name: "missing organization",
			setupBuilder: func(b ConfigBuilder) ConfigBuilder {
				return b.WithOrganization("")
			},
			expectedField:      "opencenter.meta.organization",
			expectedSuggestion: "WithOrganization",
		},
		{
			name: "missing provider",
			setupBuilder: func(b ConfigBuilder) ConfigBuilder {
				return b.WithProvider("")
			},
			expectedField:      "opencenter.infrastructure.provider",
			expectedSuggestion: "WithProvider",
		},
		{
			name: "invalid master count",
			setupBuilder: func(b ConfigBuilder) ConfigBuilder {
				return b.WithMasterCount(0)
			},
			expectedField:      "opencenter.cluster.kubernetes.master_count",
			expectedSuggestion: "WithMasterCount",
		},
		{
			name: "even master count",
			setupBuilder: func(b ConfigBuilder) ConfigBuilder {
				return b.WithMasterCount(2)
			},
			expectedField:      "opencenter.cluster.kubernetes.master_count",
			expectedSuggestion: "odd numbers",
		},
		{
			name: "negative worker count",
			setupBuilder: func(b ConfigBuilder) ConfigBuilder {
				return b.WithWorkerCount(-1)
			},
			expectedField:      "opencenter.cluster.kubernetes.worker_count",
			expectedSuggestion: "WithWorkerCount",
		},
		{
			name: "missing node subnet",
			setupBuilder: func(b ConfigBuilder) ConfigBuilder {
				return b.WithSubnetNodes("")
			},
			expectedField:      "networking.subnet_nodes",
			expectedSuggestion: "WithSubnetNodes",
		},
		{
			name: "missing pod subnet",
			setupBuilder: func(b ConfigBuilder) ConfigBuilder {
				return b.WithSubnetPods("")
			},
			expectedField:      "networking.subnet_pods",
			expectedSuggestion: "WithSubnetPods",
		},
		{
			name: "missing service subnet",
			setupBuilder: func(b ConfigBuilder) ConfigBuilder {
				return b.WithSubnetServices("")
			},
			expectedField:      "networking.subnet_services",
			expectedSuggestion: "WithSubnetServices",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewConfigBuilder("test-cluster")
			builder = tt.setupBuilder(builder)

			errors := builder.Validate()

			// Find the error for the expected field
			var foundError *ValidationError
			for i := range errors {
				if errors[i].Field == tt.expectedField {
					foundError = &errors[i]
					break
				}
			}

			if foundError == nil {
				t.Fatalf("Expected error for field '%s' not found", tt.expectedField)
			}

			// Verify suggestions contain expected text
			if !containsSuggestion(foundError.Suggestions, tt.expectedSuggestion) {
				t.Errorf("Expected suggestion containing '%s', got: %v", 
					tt.expectedSuggestion, foundError.Suggestions)
			}
		})
	}
}

// TestValidationErrorFormatting tests that validation errors are formatted correctly
// with field, message, and suggestions.
func TestValidationErrorFormatting(t *testing.T) {
	err := ValidationError{
		Field:   "test.field",
		Message: "test error message",
		Suggestions: []string{
			"First suggestion",
			"Second suggestion",
		},
	}

	errorStr := err.Error()

	// Verify field is included
	if !strings.Contains(errorStr, "test.field") {
		t.Error("Error string should contain field name")
	}

	// Verify message is included
	if !strings.Contains(errorStr, "test error message") {
		t.Error("Error string should contain error message")
	}

	// Verify suggestions are included
	if !strings.Contains(errorStr, "Suggestions:") {
		t.Error("Error string should contain suggestions header")
	}

	if !strings.Contains(errorStr, "First suggestion") {
		t.Error("Error string should contain first suggestion")
	}

	if !strings.Contains(errorStr, "Second suggestion") {
		t.Error("Error string should contain second suggestion")
	}
}

// TestProviderSpecificValidationSuggestions tests that provider-specific validation
// errors provide relevant suggestions.
func TestProviderSpecificValidationSuggestions(t *testing.T) {
	tests := []struct {
		name              string
		provider          string
		setupBuilder      func(ConfigBuilder) ConfigBuilder
		expectedSuggestion string
	}{
		{
			name:     "OpenStack missing auth URL",
			provider: "openstack",
			setupBuilder: func(b ConfigBuilder) ConfigBuilder {
				return b.WithProvider("openstack").
					WithOpenStackConfig(SimplifiedOpenStackCloud{
						AuthURL: "", // Missing auth URL
					})
			},
			expectedSuggestion: "auth_url",
		},
		{
			name:     "AWS missing region",
			provider: "aws",
			setupBuilder: func(b ConfigBuilder) ConfigBuilder {
				return b.WithProvider("aws").
					WithAWSConfig(SimplifiedAWSCloud{
						Region: "", // Missing region
					})
			},
			expectedSuggestion: "region",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewConfigBuilder("test-cluster").
				WithOrganization("test-org")
			builder = tt.setupBuilder(builder)

			errors := builder.Validate()

			// Find provider-specific error
			foundProviderError := false
			for _, err := range errors {
				if strings.Contains(strings.ToLower(err.Field), tt.provider) {
					foundProviderError = true
					if !containsSuggestion(err.Suggestions, tt.expectedSuggestion) {
						t.Errorf("Expected suggestion containing '%s', got: %v",
							tt.expectedSuggestion, err.Suggestions)
					}
					break
				}
			}

			if !foundProviderError {
				t.Errorf("Expected provider-specific error for %s not found", tt.provider)
			}
		})
	}
}

// Helper function to check if suggestions contain a specific text
func containsSuggestion(suggestions []string, text string) bool {
	for _, suggestion := range suggestions {
		if strings.Contains(suggestion, text) {
			return true
		}
	}
	return false
}

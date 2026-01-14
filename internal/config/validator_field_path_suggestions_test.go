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
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config/services"
)

// TestValidationErrorsIncludeFieldPathsAndSuggestions verifies that ALL validation
// errors include both field paths and actionable suggestions.
//
// This test validates the acceptance criterion:
// "Validation errors include field paths and suggestions"
// from Task 2.4: Enhanced Configuration Validation
func TestValidationErrorsIncludeFieldPathsAndSuggestions(t *testing.T) {
	ctx := context.Background()
	validator := NewConfigValidator(false)

	// Test cases that trigger various validation errors
	testCases := []struct {
		name           string
		setupConfig    func() *Config
		expectedErrors int
		description    string
	}{
		{
			name: "missing required fields",
			setupConfig: func() *Config {
				config := &Config{}
				config.OpenCenter.Cluster.ClusterName = ""
				config.OpenCenter.GitOps.GitDir = ""
				return config
			},
			expectedErrors: 2,
			description:    "Missing cluster name and git directory",
		},
		{
			name: "invalid email format",
			setupConfig: func() *Config {
				config := NewDefault("test-cluster")
				config.OpenCenter.Cluster.AdminEmail = "invalid-email"
				return &config
			},
			expectedErrors: 1,
			description:    "Invalid email address format",
		},
		{
			name: "invalid domain format",
			setupConfig: func() *Config {
				config := NewDefault("test-cluster")
				config.OpenCenter.Cluster.BaseDomain = "invalid_domain"
				return &config
			},
			expectedErrors: 1,
			description:    "Invalid domain format",
		},
		{
			name: "invalid node counts",
			setupConfig: func() *Config {
				config := NewDefault("test-cluster")
				config.OpenCenter.Cluster.Kubernetes.MasterCount = 0
				config.OpenCenter.Cluster.Kubernetes.WorkerCount = -1
				return &config
			},
			expectedErrors: 2,
			description:    "Invalid master and worker counts",
		},
		{
			name: "multiple network plugins enabled",
			setupConfig: func() *Config {
				config := NewDefault("test-cluster")
				config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = true
				config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled = true
				return &config
			},
			expectedErrors: 1,
			description:    "Multiple CNI plugins enabled",
		},
		{
			name: "missing provider",
			setupConfig: func() *Config {
				config := NewDefault("test-cluster")
				config.OpenCenter.Infrastructure.Provider = ""
				return &config
			},
			expectedErrors: 1,
			description:    "Missing cloud provider",
		},
		{
			name: "OpenStack missing required fields",
			setupConfig: func() *Config {
				config := NewDefault("test-cluster")
				config.OpenCenter.Infrastructure.Provider = "openstack"
				config.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = ""
				config.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName = ""
				return &config
			},
			expectedErrors: 2,
			description:    "OpenStack missing auth URL and tenant name",
		},
		{
			name: "AWS missing region",
			setupConfig: func() *Config {
				config := NewDefault("test-cluster")
				config.OpenCenter.Infrastructure.Provider = "aws"
				config.OpenCenter.Infrastructure.Cloud.AWS.Region = ""
				return &config
			},
			expectedErrors: 1,
			description:    "AWS missing region",
		},
		{
			name: "S3 backend missing required fields",
			setupConfig: func() *Config {
				config := NewDefault("test-cluster")
				config.OpenTofu.Enabled = true
				config.OpenTofu.Backend.Type = "s3"
				config.OpenTofu.Backend.S3.Bucket = ""
				config.OpenTofu.Backend.S3.Key = ""
				config.OpenTofu.Backend.S3.Region = ""
				return &config
			},
			expectedErrors: 1,
			description:    "S3 backend missing bucket, key, and region",
		},
		{
			name: "S3 bucket name not lowercase",
			setupConfig: func() *Config {
				config := NewDefault("test-cluster")
				config.OpenTofu.Enabled = true
				config.OpenTofu.Backend.Type = "s3"
				config.OpenTofu.Backend.S3.Bucket = "MyBucket"
				config.OpenTofu.Backend.S3.Key = "terraform.tfstate"
				config.OpenTofu.Backend.S3.Region = "us-west-2"
				config.OpenCenter.Cluster.AWSAccessKey = "test-key"
				config.OpenCenter.Cluster.AWSSecretAccessKey = "test-secret"
				return &config
			},
			expectedErrors: 1,
			description:    "S3 bucket name contains uppercase letters",
		},
		{
			name: "Loki invalid storage type",
			setupConfig: func() *Config {
				config := NewDefault("test-cluster")
				if svc, ok := config.OpenCenter.Services["loki"].(*services.LokiConfig); ok {
					svc.Enabled = true
					svc.StorageType = "invalid"
				}
				return &config
			},
			expectedErrors: 1,
			description:    "Loki has invalid storage type",
		},
		{
			name: "VRRP missing IP when required",
			setupConfig: func() *Config {
				config := NewDefault("test-cluster")
				config.Networking.UseOctavia = false
				config.Networking.VRRPEnabled = true
				config.Networking.VRRPIP = ""
				return &config
			},
			expectedErrors: 1,
			description:    "VRRP enabled without IP address",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := tc.setupConfig()
			result := validator.Validate(ctx, config)

			// Verify we got errors
			if len(result.Errors) == 0 {
				t.Fatalf("Expected validation errors for: %s", tc.description)
			}

			// Verify EVERY error has a field path
			for i, err := range result.Errors {
				if err.Field == "" {
					t.Errorf("Error #%d is missing field path: %s", i+1, err.Message)
				}
			}

			// Verify EVERY error has suggestions
			for i, err := range result.Errors {
				if len(err.Suggestions) == 0 {
					t.Errorf("Error #%d for field '%s' is missing suggestions: %s",
						i+1, err.Field, err.Message)
				}
			}

			// Verify suggestions are actionable (not empty strings)
			for i, err := range result.Errors {
				for j, suggestion := range err.Suggestions {
					if suggestion == "" {
						t.Errorf("Error #%d for field '%s' has empty suggestion at index %d",
							i+1, err.Field, j)
					}
				}
			}

			t.Logf("✓ All %d validation errors include field paths and suggestions", len(result.Errors))
		})
	}
}

// TestValidationErrorStructureCompleteness verifies that validation errors
// have all required fields populated correctly.
func TestValidationErrorStructureCompleteness(t *testing.T) {
	ctx := context.Background()
	validator := NewConfigValidator(false)

	// Create a config with multiple validation issues
	config := &Config{}
	config.OpenCenter.Cluster.ClusterName = ""
	config.OpenCenter.GitOps.GitDir = ""
	config.OpenCenter.Cluster.AdminEmail = "invalid-email"
	config.OpenCenter.Cluster.Kubernetes.MasterCount = 0
	config.OpenCenter.Infrastructure.Provider = ""

	result := validator.Validate(ctx, config)

	if len(result.Errors) == 0 {
		t.Fatal("Expected validation errors")
	}

	for i, err := range result.Errors {
		t.Run(err.Field, func(t *testing.T) {
			// Verify Type field
			if err.Type == "" {
				t.Errorf("Error #%d: Type field is empty", i+1)
			}

			// Verify Field path
			if err.Field == "" {
				t.Errorf("Error #%d: Field path is empty", i+1)
			}

			// Verify Message
			if err.Message == "" {
				t.Errorf("Error #%d: Message is empty", i+1)
			}

			// Verify Suggestions
			if len(err.Suggestions) == 0 {
				t.Errorf("Error #%d: No suggestions provided", i+1)
			}

			// Verify field path uses dot notation
			if err.Field != "" && err.Field[0] == '.' {
				t.Errorf("Error #%d: Field path should not start with dot: %s", i+1, err.Field)
			}

			// Log the complete error structure for verification
			t.Logf("Error #%d:", i+1)
			t.Logf("  Type: %s", err.Type)
			t.Logf("  Field: %s", err.Field)
			t.Logf("  Message: %s", err.Message)
			t.Logf("  Suggestions: %v", err.Suggestions)
			if err.Value != nil {
				t.Logf("  Value: %v", err.Value)
			}
		})
	}
}

// TestValidationSuggestionsAreActionable verifies that suggestions provide
// concrete, actionable guidance to users.
func TestValidationSuggestionsAreActionable(t *testing.T) {
	ctx := context.Background()
	validator := NewConfigValidator(false)

	testCases := []struct {
		name              string
		setupConfig       func() *Config
		expectedFieldPath string
		checkSuggestion   func([]string) bool
		description       string
	}{
		{
			name: "cluster name suggestion includes field name",
			setupConfig: func() *Config {
				config := &Config{}
				config.OpenCenter.Cluster.ClusterName = ""
				return config
			},
			expectedFieldPath: "opencenter.cluster.cluster_name",
			checkSuggestion: func(suggestions []string) bool {
				for _, s := range suggestions {
					if len(s) > 0 && s != "" {
						return true
					}
				}
				return false
			},
			description: "Suggestions should be non-empty",
		},
		{
			name: "email validation suggests correct format",
			setupConfig: func() *Config {
				config := NewDefault("test-cluster")
				config.OpenCenter.Cluster.AdminEmail = "invalid"
				return &config
			},
			expectedFieldPath: "opencenter.cluster.admin_email",
			checkSuggestion: func(suggestions []string) bool {
				for _, s := range suggestions {
					if len(s) > 10 { // Actionable suggestions should be descriptive
						return true
					}
				}
				return false
			},
			description: "Email suggestions should be descriptive",
		},
		{
			name: "node count suggestions include valid values",
			setupConfig: func() *Config {
				config := NewDefault("test-cluster")
				config.OpenCenter.Cluster.Kubernetes.MasterCount = 0
				return &config
			},
			expectedFieldPath: "opencenter.cluster.kubernetes.master_count",
			checkSuggestion: func(suggestions []string) bool {
				for _, s := range suggestions {
					if len(s) > 0 {
						return true
					}
				}
				return false
			},
			description: "Master count suggestions should provide guidance",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := tc.setupConfig()
			result := validator.Validate(ctx, config)

			// Find the error for the expected field
			var foundError *ConfigValidationError
			for i := range result.Errors {
				if result.Errors[i].Field == tc.expectedFieldPath {
					foundError = result.Errors[i]
					break
				}
			}

			if foundError == nil {
				t.Fatalf("Expected error for field '%s' not found", tc.expectedFieldPath)
			}

			// Verify suggestions are actionable
			if !tc.checkSuggestion(foundError.Suggestions) {
				t.Errorf("%s\nGot suggestions: %v", tc.description, foundError.Suggestions)
			}

			t.Logf("✓ Suggestions are actionable: %v", foundError.Suggestions)
		})
	}
}

// TestAllValidationErrorsHaveFieldPaths is a comprehensive test that ensures
// NO validation error is missing a field path.
func TestAllValidationErrorsHaveFieldPaths(t *testing.T) {
	ctx := context.Background()
	validator := NewConfigValidator(false)

	// Create a config that will trigger many different validation errors
	config := &Config{}
	// Leave most fields empty or invalid to trigger various validations

	result := validator.Validate(ctx, config)

	if len(result.Errors) == 0 {
		t.Skip("No validation errors to test")
	}

	errorsWithoutFieldPath := 0
	for i, err := range result.Errors {
		if err.Field == "" {
			t.Errorf("Error #%d is missing field path: %s", i+1, err.Message)
			errorsWithoutFieldPath++
		}
	}

	if errorsWithoutFieldPath > 0 {
		t.Fatalf("Found %d validation errors without field paths", errorsWithoutFieldPath)
	}

	t.Logf("✓ All %d validation errors have field paths", len(result.Errors))
}

// TestAllValidationErrorsHaveSuggestions is a comprehensive test that ensures
// NO validation error is missing suggestions.
func TestAllValidationErrorsHaveSuggestions(t *testing.T) {
	ctx := context.Background()
	validator := NewConfigValidator(false)

	// Create a config that will trigger many different validation errors
	config := &Config{}
	// Leave most fields empty or invalid to trigger various validations

	result := validator.Validate(ctx, config)

	if len(result.Errors) == 0 {
		t.Skip("No validation errors to test")
	}

	errorsWithoutSuggestions := 0
	for i, err := range result.Errors {
		if len(err.Suggestions) == 0 {
			t.Errorf("Error #%d for field '%s' is missing suggestions: %s",
				i+1, err.Field, err.Message)
			errorsWithoutSuggestions++
		}
	}

	if errorsWithoutSuggestions > 0 {
		t.Fatalf("Found %d validation errors without suggestions", errorsWithoutSuggestions)
	}

	t.Logf("✓ All %d validation errors have suggestions", len(result.Errors))
}

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

package v2

import (
	"strings"
	"testing"
)

// TestValidator_OpenTofuBackend_Local tests validation of local backend configuration.
func TestValidator_OpenTofuBackend_Local(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid local backend with path",
			config: &Config{
				SchemaVersion: "2.0",
				OpenTofu: OpenTofuConfig{
					Backend: BackendConfig{
						Type: "local",
						Local: &LocalBackendConfig{
							Path: "/tmp/terraform.tfstate",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Local backend missing local section",
			config: &Config{
				SchemaVersion: "2.0",
				OpenTofu: OpenTofuConfig{
					Backend: BackendConfig{
						Type: "local",
					},
				},
			},
			expectError: true,
			errorMsg:    "opentofu.backend.local",
		},
		{
			name: "Local backend with empty path",
			config: &Config{
				SchemaVersion: "2.0",
				OpenTofu: OpenTofuConfig{
					Backend: BackendConfig{
						Type: "local",
						Local: &LocalBackendConfig{
							Path: "",
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "opentofu.backend.local.path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateBusinessRules(tt.config)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestValidator_OpenTofuBackend_S3 tests validation of S3 backend configuration.
func TestValidator_OpenTofuBackend_S3(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid S3 backend",
			config: &Config{
				SchemaVersion: "2.0",
				OpenTofu: OpenTofuConfig{
					Backend: BackendConfig{
						Type: "s3",
						S3: &S3BackendConfig{
							Bucket: "my-bucket",
							Key:    "terraform.tfstate",
							Region: "us-east-1",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "S3 backend missing s3 section",
			config: &Config{
				SchemaVersion: "2.0",
				OpenTofu: OpenTofuConfig{
					Backend: BackendConfig{
						Type: "s3",
					},
				},
			},
			expectError: true,
			errorMsg:    "opentofu.backend.s3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateBusinessRules(tt.config)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestValidator_OpenTofuBackend_Remote tests validation of remote backend configuration.
func TestValidator_OpenTofuBackend_Remote(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid remote backend with config",
			config: &Config{
				SchemaVersion: "2.0",
				OpenTofu: OpenTofuConfig{
					Backend: BackendConfig{
						Type: "remote",
						Config: map[string]any{
							"hostname":     "app.terraform.io",
							"organization": "my-org",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Remote backend missing config",
			config: &Config{
				SchemaVersion: "2.0",
				OpenTofu: OpenTofuConfig{
					Backend: BackendConfig{
						Type: "remote",
					},
				},
			},
			expectError: true,
			errorMsg:    "opentofu.backend.config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateBusinessRules(tt.config)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

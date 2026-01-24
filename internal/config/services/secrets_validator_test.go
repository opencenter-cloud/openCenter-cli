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

package services

import (
	"strings"
	"testing"
)

func TestSecretsValidator_ValidateRequiredSecrets_CertManagerRoute53(t *testing.T) {
	validator := NewSecretsValidator()

	// Test cert-manager with route53 DNS provider
	services := map[string]any{
		"cert-manager": &CertManagerConfig{
			BaseConfig: BaseConfig{
				Enabled: true,
			},
			DNSProvider: "route53",
		},
	}

	// Missing AWS credentials
	secrets := map[string]any{
		"service_secrets": map[string]any{
			"cert_manager": map[string]any{},
		},
	}

	errors := validator.ValidateRequiredSecrets(services, secrets)

	if len(errors) != 2 {
		t.Errorf("Expected 2 errors for missing AWS credentials, got %d", len(errors))
	}

	// Check that errors mention the required secrets
	errorStr := strings.Join(errors, "\n")
	if !strings.Contains(errorStr, "aws_access_key") {
		t.Error("Expected error to mention aws_access_key")
	}
	if !strings.Contains(errorStr, "aws_secret_key") {
		t.Error("Expected error to mention aws_secret_key")
	}
}

func TestSecretsValidator_ValidateRequiredSecrets_CertManagerRoute53WithSecrets(t *testing.T) {
	validator := NewSecretsValidator()

	// Test cert-manager with route53 DNS provider and proper secrets
	services := map[string]any{
		"cert-manager": &CertManagerConfig{
			BaseConfig: BaseConfig{
				Enabled: true,
			},
			DNSProvider: "route53",
		},
	}

	// AWS credentials configured
	secrets := map[string]any{
		"service_secrets": map[string]any{
			"cert_manager": map[string]any{
				"aws_access_key": "AKIAIOSFODNN7EXAMPLE",
				"aws_secret_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
		},
	}

	errors := validator.ValidateRequiredSecrets(services, secrets)

	if len(errors) != 0 {
		t.Errorf("Expected no errors when secrets are configured, got %d: %v", len(errors), errors)
	}
}

func TestSecretsValidator_ValidateRequiredSecrets_CertManagerCloudflare(t *testing.T) {
	validator := NewSecretsValidator()

	// Test cert-manager with cloudflare DNS provider
	services := map[string]any{
		"cert-manager": &CertManagerConfig{
			BaseConfig: BaseConfig{
				Enabled: true,
			},
			DNSProvider: "cloudflare",
		},
	}

	// Missing Cloudflare API token
	secrets := map[string]any{
		"service_secrets": map[string]any{
			"cert_manager": map[string]any{},
		},
	}

	errors := validator.ValidateRequiredSecrets(services, secrets)

	if len(errors) != 1 {
		t.Errorf("Expected 1 error for missing Cloudflare API token, got %d", len(errors))
	}

	if len(errors) > 0 && !strings.Contains(errors[0], "cloudflare_api_token") {
		t.Error("Expected error to mention cloudflare_api_token")
	}
}

func TestSecretsValidator_ValidateRequiredSecrets_LokiSwift(t *testing.T) {
	validator := NewSecretsValidator()

	// Test loki with swift storage
	services := map[string]any{
		"loki": &LokiConfig{
			BaseConfig: BaseConfig{
				Enabled: true,
			},
			StorageType: "swift",
		},
	}

	// Missing Swift password
	secrets := map[string]any{
		"service_secrets": map[string]any{
			"loki": map[string]any{},
		},
	}

	errors := validator.ValidateRequiredSecrets(services, secrets)

	if len(errors) != 1 {
		t.Errorf("Expected 1 error for missing Swift password, got %d", len(errors))
	}

	if len(errors) > 0 && !strings.Contains(errors[0], "swift_password") {
		t.Error("Expected error to mention swift_password")
	}
}

func TestSecretsValidator_ValidateRequiredSecrets_LokiS3(t *testing.T) {
	validator := NewSecretsValidator()

	// Test loki with S3 storage
	services := map[string]any{
		"loki": &LokiConfig{
			BaseConfig: BaseConfig{
				Enabled: true,
			},
			StorageType: "s3",
		},
	}

	// Missing S3 credentials
	secrets := map[string]any{
		"service_secrets": map[string]any{
			"loki": map[string]any{},
		},
	}

	errors := validator.ValidateRequiredSecrets(services, secrets)

	if len(errors) != 2 {
		t.Errorf("Expected 2 errors for missing S3 credentials, got %d", len(errors))
	}

	errorStr := strings.Join(errors, "\n")
	if !strings.Contains(errorStr, "s3_access_key") {
		t.Error("Expected error to mention s3_access_key")
	}
	if !strings.Contains(errorStr, "s3_secret_key") {
		t.Error("Expected error to mention s3_secret_key")
	}
}

func TestSecretsValidator_ValidateRequiredSecrets_TempoSwift(t *testing.T) {
	validator := NewSecretsValidator()

	// Test tempo with swift storage
	services := map[string]any{
		"tempo": &TempoConfig{
			BaseConfig: BaseConfig{
				Enabled: true,
			},
			StorageType: "swift",
		},
	}

	// Missing Swift password
	secrets := map[string]any{
		"service_secrets": map[string]any{
			"tempo": map[string]any{},
		},
	}

	errors := validator.ValidateRequiredSecrets(services, secrets)

	if len(errors) != 1 {
		t.Errorf("Expected 1 error for missing Swift password, got %d", len(errors))
	}

	if len(errors) > 0 && !strings.Contains(errors[0], "swift_password") {
		t.Error("Expected error to mention swift_password")
	}
}

func TestSecretsValidator_ValidateRequiredSecrets_VeleroMultipleStorageTypes(t *testing.T) {
	validator := NewSecretsValidator()

	testCases := []struct {
		name          string
		storageType   string
		expectedError string
	}{
		{
			name:          "swift storage",
			storageType:   "swift",
			expectedError: "swift_password",
		},
		{
			name:          "s3 storage",
			storageType:   "s3",
			expectedError: "s3_access_key",
		},
		{
			name:          "gcs storage",
			storageType:   "gcs",
			expectedError: "gcp_service_account_key",
		},
		{
			name:          "azure storage",
			storageType:   "azure",
			expectedError: "azure_storage_account_key",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			services := map[string]any{
				"velero": &VeleroConfig{
					BaseConfig: BaseConfig{
						Enabled: true,
					},
					StorageType: tc.storageType,
				},
			}

			secrets := map[string]any{
				"service_secrets": map[string]any{
					"velero": map[string]any{},
				},
			}

			errors := validator.ValidateRequiredSecrets(services, secrets)

			if len(errors) == 0 {
				t.Errorf("Expected error for missing %s secret", tc.expectedError)
				return
			}

			errorStr := strings.Join(errors, "\n")
			if !strings.Contains(errorStr, tc.expectedError) {
				t.Errorf("Expected error to mention %s, got: %v", tc.expectedError, errors)
			}
		})
	}
}

func TestSecretsValidator_ValidateRequiredSecrets_KeycloakAlways(t *testing.T) {
	validator := NewSecretsValidator()

	// Test keycloak which always requires admin password
	services := map[string]any{
		"keycloak": &KeycloakConfig{
			BaseConfig: BaseConfig{
				Enabled: true,
			},
		},
	}

	// Missing admin password
	secrets := map[string]any{
		"service_secrets": map[string]any{
			"keycloak": map[string]any{},
		},
	}

	errors := validator.ValidateRequiredSecrets(services, secrets)

	if len(errors) != 1 {
		t.Errorf("Expected 1 error for missing admin password, got %d", len(errors))
	}

	if len(errors) > 0 && !strings.Contains(errors[0], "admin_password") {
		t.Error("Expected error to mention admin_password")
	}
}

func TestSecretsValidator_ValidateRequiredSecrets_ServiceDisabled(t *testing.T) {
	validator := NewSecretsValidator()

	// Test that disabled services don't require secrets
	services := map[string]any{
		"cert-manager": &CertManagerConfig{
			BaseConfig: BaseConfig{
				Enabled: false, // Disabled
			},
			DNSProvider: "route53",
		},
	}

	// No secrets configured
	secrets := map[string]any{
		"service_secrets": map[string]any{},
	}

	errors := validator.ValidateRequiredSecrets(services, secrets)

	if len(errors) != 0 {
		t.Errorf("Expected no errors for disabled service, got %d: %v", len(errors), errors)
	}
}

func TestSecretsValidator_ValidateRequiredSecrets_MultipleServices(t *testing.T) {
	validator := NewSecretsValidator()

	// Test multiple services with missing secrets
	services := map[string]any{
		"cert-manager": &CertManagerConfig{
			BaseConfig: BaseConfig{
				Enabled: true,
			},
			DNSProvider: "route53",
		},
		"loki": &LokiConfig{
			BaseConfig: BaseConfig{
				Enabled: true,
			},
			StorageType: "swift",
		},
		"keycloak": &KeycloakConfig{
			BaseConfig: BaseConfig{
				Enabled: true,
			},
		},
	}

	// No secrets configured
	secrets := map[string]any{
		"service_secrets": map[string]any{},
	}

	errors := validator.ValidateRequiredSecrets(services, secrets)

	// Should have errors for:
	// - cert-manager: aws_access_key, aws_secret_key (2)
	// - loki: swift_password (1)
	// - keycloak: admin_password (1)
	// Total: 4 errors
	if len(errors) != 4 {
		t.Errorf("Expected 4 errors for multiple services with missing secrets, got %d: %v", len(errors), errors)
	}
}

func TestSecretsValidator_ValidateRequiredSecrets_ErrorFormat(t *testing.T) {
	validator := NewSecretsValidator()

	services := map[string]any{
		"cert-manager": &CertManagerConfig{
			BaseConfig: BaseConfig{
				Enabled: true,
			},
			DNSProvider: "route53",
		},
	}

	secrets := map[string]any{
		"service_secrets": map[string]any{},
	}

	errors := validator.ValidateRequiredSecrets(services, secrets)

	if len(errors) == 0 {
		t.Fatal("Expected errors but got none")
	}

	// Check error format includes E006 code
	for _, err := range errors {
		if !strings.HasPrefix(err, "E006:") {
			t.Errorf("Expected error to start with 'E006:', got: %s", err)
		}
		if !strings.Contains(err, "services.cert-manager") {
			t.Errorf("Expected error to mention service name, got: %s", err)
		}
		if !strings.Contains(err, "requires") {
			t.Errorf("Expected error to mention 'requires', got: %s", err)
		}
	}
}

func TestSecretsValidator_GetSecretMappings(t *testing.T) {
	validator := NewSecretsValidator()

	mappings := validator.GetSecretMappings()

	if len(mappings) == 0 {
		t.Error("Expected non-empty secret mappings")
	}

	// Check that cert-manager mapping exists
	found := false
	for _, mapping := range mappings {
		if mapping.Service == "cert-manager" {
			found = true
			if len(mapping.Requirements) == 0 {
				t.Error("Expected cert-manager to have secret requirements")
			}
		}
	}

	if !found {
		t.Error("Expected to find cert-manager in secret mappings")
	}
}

func TestSecretsValidator_AddSecretMapping(t *testing.T) {
	validator := NewSecretsValidator()

	initialCount := len(validator.GetSecretMappings())

	// Add a custom secret mapping
	validator.AddSecretMapping(ServiceSecretMapping{
		Service: "custom-service",
		Requirements: []SecretRequirement{
			{
				SecretPath:  "service_secrets.custom_service.api_key",
				Condition:   "always",
				Description: "API key for custom service",
			},
		},
	})

	newCount := len(validator.GetSecretMappings())

	if newCount != initialCount+1 {
		t.Errorf("Expected %d mappings after adding one, got %d", initialCount+1, newCount)
	}
}

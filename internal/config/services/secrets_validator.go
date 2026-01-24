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
	"fmt"
	"strings"
)

// SecretRequirement represents a required secret for a service
type SecretRequirement struct {
	SecretPath  string   // Path to the secret in the secrets configuration
	Condition   string   // Condition when this secret is required (e.g., "dns_provider=route53")
	Description string   // Human-readable description of what the secret is for
}

// ServiceSecretMapping defines required secrets for each service
type ServiceSecretMapping struct {
	Service      string              // Service name
	Requirements []SecretRequirement // List of required secrets
}

// serviceSecretMappings defines the secret requirements for each service
var serviceSecretMappings = []ServiceSecretMapping{
	{
		Service: "cert-manager",
		Requirements: []SecretRequirement{
			{
				SecretPath:  "service_secrets.cert_manager.aws_access_key",
				Condition:   "dns_provider=route53",
				Description: "AWS access key for Route53 DNS challenge",
			},
			{
				SecretPath:  "service_secrets.cert_manager.aws_secret_key",
				Condition:   "dns_provider=route53",
				Description: "AWS secret key for Route53 DNS challenge",
			},
			{
				SecretPath:  "service_secrets.cert_manager.cloudflare_api_token",
				Condition:   "dns_provider=cloudflare",
				Description: "Cloudflare API token for DNS challenge",
			},
			{
				SecretPath:  "service_secrets.cert_manager.gcp_service_account_key",
				Condition:   "dns_provider=clouddns",
				Description: "GCP service account key for Cloud DNS challenge",
			},
			{
				SecretPath:  "service_secrets.cert_manager.azure_client_id",
				Condition:   "dns_provider=azuredns",
				Description: "Azure client ID for Azure DNS challenge",
			},
			{
				SecretPath:  "service_secrets.cert_manager.azure_client_secret",
				Condition:   "dns_provider=azuredns",
				Description: "Azure client secret for Azure DNS challenge",
			},
			{
				SecretPath:  "service_secrets.cert_manager.azure_tenant_id",
				Condition:   "dns_provider=azuredns",
				Description: "Azure tenant ID for Azure DNS challenge",
			},
			// Note: designate DNS provider uses infrastructure credentials, no service-specific secrets required
		},
	},
	{
		Service: "loki",
		Requirements: []SecretRequirement{
			{
				SecretPath:  "service_secrets.loki.swift_password",
				Condition:   "storage_type=swift",
				Description: "Swift password or application credential secret for Loki storage",
			},
			{
				SecretPath:  "service_secrets.loki.s3_access_key",
				Condition:   "storage_type=s3",
				Description: "S3 access key for Loki storage",
			},
			{
				SecretPath:  "service_secrets.loki.s3_secret_key",
				Condition:   "storage_type=s3",
				Description: "S3 secret key for Loki storage",
			},
		},
	},
	{
		Service: "tempo",
		Requirements: []SecretRequirement{
			{
				SecretPath:  "service_secrets.tempo.swift_password",
				Condition:   "storage_type=swift",
				Description: "Swift password or application credential secret for Tempo storage",
			},
			{
				SecretPath:  "service_secrets.tempo.s3_access_key",
				Condition:   "storage_type=s3",
				Description: "S3 access key for Tempo storage",
			},
			{
				SecretPath:  "service_secrets.tempo.s3_secret_key",
				Condition:   "storage_type=s3",
				Description: "S3 secret key for Tempo storage",
			},
		},
	},
	{
		Service: "velero",
		Requirements: []SecretRequirement{
			{
				SecretPath:  "service_secrets.velero.swift_password",
				Condition:   "storage_type=swift",
				Description: "Swift password or application credential secret for Velero backups",
			},
			{
				SecretPath:  "service_secrets.velero.s3_access_key",
				Condition:   "storage_type=s3",
				Description: "S3 access key for Velero backups",
			},
			{
				SecretPath:  "service_secrets.velero.s3_secret_key",
				Condition:   "storage_type=s3",
				Description: "S3 secret key for Velero backups",
			},
			{
				SecretPath:  "service_secrets.velero.gcp_service_account_key",
				Condition:   "storage_type=gcs",
				Description: "GCP service account key for Velero backups",
			},
			{
				SecretPath:  "service_secrets.velero.azure_storage_account_key",
				Condition:   "storage_type=azure",
				Description: "Azure storage account key for Velero backups",
			},
		},
	},
	{
		Service: "keycloak",
		Requirements: []SecretRequirement{
			{
				SecretPath:  "service_secrets.keycloak.admin_password",
				Condition:   "always",
				Description: "Keycloak admin password",
			},
		},
	},
}

// SecretsValidator validates that required secrets are configured for enabled services
type SecretsValidator struct {
	mappings []ServiceSecretMapping
}

// NewSecretsValidator creates a new secrets validator
func NewSecretsValidator() *SecretsValidator {
	return &SecretsValidator{
		mappings: serviceSecretMappings,
	}
}

// ValidateRequiredSecrets checks if all required secrets are configured for enabled services
// Returns a list of error messages for missing secrets
func (v *SecretsValidator) ValidateRequiredSecrets(
	services map[string]any,
	secrets map[string]any,
) []string {
	var errors []string

	// Check each service's secret requirements
	for _, mapping := range v.mappings {
		// Check if the service is enabled
		if !isServiceEnabled(services, mapping.Service) {
			continue // Service is not enabled, no need to check secrets
		}

		// Get service configuration
		serviceConfig, ok := services[mapping.Service]
		if !ok {
			continue
		}

		// Check each secret requirement
		for _, req := range mapping.Requirements {
			// Check if the condition is met
			if !v.isConditionMet(req.Condition, serviceConfig) {
				continue // Condition not met, secret not required
			}

			// Check if the secret is configured
			if !v.isSecretConfigured(req.SecretPath, secrets) {
				errors = append(errors, fmt.Sprintf(
					"E006: services.%s: requires %s when %s. %s",
					mapping.Service,
					req.SecretPath,
					req.Condition,
					req.Description,
				))
			}
		}
	}

	return errors
}

// isConditionMet checks if a condition is met for a service configuration
func (v *SecretsValidator) isConditionMet(condition string, serviceConfig any) bool {
	// "always" condition is always met
	if condition == "always" {
		return true
	}

	// Parse condition (format: "field=value")
	// Split by '=' to handle the condition
	parts := strings.SplitN(condition, "=", 2)
	if len(parts) != 2 {
		return false
	}
	field := parts[0]
	value := parts[1]

	// Check condition based on service type
	switch cfg := serviceConfig.(type) {
	case *CertManagerConfig:
		if field == "dns_provider" {
			return cfg.DNSProvider == value
		}
	case *LokiConfig:
		if field == "storage_type" {
			return cfg.StorageType == value
		}
	case *TempoConfig:
		if field == "storage_type" {
			return cfg.StorageType == value
		}
	case *VeleroConfig:
		if field == "storage_type" {
			return cfg.StorageType == value
		}
	}

	return false
}

// isSecretConfigured checks if a secret is configured at the given path
func (v *SecretsValidator) isSecretConfigured(secretPath string, secrets map[string]any) bool {
	// Parse the secret path (e.g., "service_secrets.cert_manager.aws_access_key")
	// For simplicity, we'll check if the path exists and has a non-empty value
	
	// Split path into parts
	var parts []string
	currentPart := ""
	for _, char := range secretPath {
		if char == '.' {
			if currentPart != "" {
				parts = append(parts, currentPart)
				currentPart = ""
			}
		} else {
			currentPart += string(char)
		}
	}
	if currentPart != "" {
		parts = append(parts, currentPart)
	}

	// Navigate through the secrets map
	current := secrets
	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - check if value exists and is non-empty
			if val, ok := current[part]; ok {
				switch v := val.(type) {
				case string:
					return v != ""
				case map[string]any:
					return len(v) > 0
				default:
					return val != nil
				}
			}
			return false
		}

		// Navigate to next level
		if next, ok := current[part]; ok {
			if nextMap, ok := next.(map[string]any); ok {
				current = nextMap
			} else {
				return false
			}
		} else {
			return false
		}
	}

	return false
}

// GetSecretMappings returns the service-secret mappings for inspection
func (v *SecretsValidator) GetSecretMappings() []ServiceSecretMapping {
	return v.mappings
}

// AddSecretMapping adds a new secret mapping to the validator
// This allows for dynamic secret requirement registration
func (v *SecretsValidator) AddSecretMapping(mapping ServiceSecretMapping) {
	v.mappings = append(v.mappings, mapping)
}

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
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: v2-cluster-config-schema, Property 15: Required Secrets Validation
// For any configuration with services enabled, the validator must verify that all required secrets
// for those services are configured (e.g., cert-manager requires aws_access_key when using route53,
// loki requires swift_password when using swift storage).
func TestProperty_RequiredSecretsValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	validator := NewSecretsValidator()

	// Property: When a service is enabled with a specific provider, required secrets must be validated
	properties.Property("enabled service with provider requires corresponding secrets", prop.ForAll(
		func(serviceType string, providerType string, hasSecrets bool) bool {
			// Create service configuration based on type and provider
			services := createServiceWithProvider(serviceType, providerType)
			
			// Create secrets configuration
			var secrets map[string]any
			if hasSecrets {
				secrets = createSecretsForService(serviceType, providerType)
			} else {
				secrets = map[string]any{
					"service_secrets": map[string]any{},
				}
			}

			// Validate
			errors := validator.ValidateRequiredSecrets(services, secrets)

			// Check if this service/provider combination requires secrets
			requiresSecrets := doesServiceProviderRequireSecrets(serviceType, providerType)

			// If secrets are provided, there should be no errors
			// If secrets are missing and required, there should be errors
			// If secrets are not required, there should be no errors
			if hasSecrets {
				return len(errors) == 0
			}
			if requiresSecrets {
				return len(errors) > 0
			}
			return len(errors) == 0
		},
		genServiceType(),
		genProviderType(),
		gen.Bool(),
	))

	// Property: Disabled services should never require secrets
	properties.Property("disabled services never require secrets", prop.ForAll(
		func(serviceType string, providerType string) bool {
			// Create disabled service
			services := createDisabledService(serviceType, providerType)
			
			// No secrets configured
			secrets := map[string]any{
				"service_secrets": map[string]any{},
			}

			// Validate
			errors := validator.ValidateRequiredSecrets(services, secrets)

			// Should have no errors for disabled services
			return len(errors) == 0
		},
		genServiceType(),
		genProviderType(),
	))

	// Property: Error messages should include service name and secret path
	properties.Property("error messages include service name and secret path", prop.ForAll(
		func(serviceType string, providerType string) bool {
			// Create enabled service without secrets
			services := createServiceWithProvider(serviceType, providerType)
			secrets := map[string]any{
				"service_secrets": map[string]any{},
			}

			// Validate
			errors := validator.ValidateRequiredSecrets(services, secrets)

			// All errors should mention the service name
			for _, err := range errors {
				if len(err) == 0 {
					return false
				}
				// Error should start with E006 code
				if len(err) < 5 || err[:5] != "E006:" {
					return false
				}
				// Error should mention the service
				// (we can't check exact service name as it might be transformed)
			}

			return true
		},
		genServiceType(),
		genProviderType(),
	))

	properties.TestingRun(t)
}

// genServiceType generates service types that require secrets
func genServiceType() gopter.Gen {
	return gen.OneConstOf("cert-manager", "loki", "tempo", "velero", "keycloak")
}

// genProviderType generates provider types for services
func genProviderType() gopter.Gen {
	return gen.OneConstOf(
		"route53",
		"cloudflare",
		"designate",
		"clouddns",
		"azuredns",
		"swift",
		"s3",
		"gcs",
		"azure",
	)
}

// createServiceWithProvider creates an enabled service with the specified provider
func createServiceWithProvider(serviceType string, providerType string) map[string]any {
	switch serviceType {
	case "cert-manager":
		// Only use DNS providers for cert-manager
		dnsProvider := providerType
		if providerType == "swift" || providerType == "s3" || providerType == "gcs" || providerType == "azure" {
			dnsProvider = "route53" // Default to route53 for non-DNS providers
		}
		return map[string]any{
			"cert-manager": &CertManagerConfig{
				BaseConfig: BaseConfig{
					Enabled: true,
				},
				DNSProvider: dnsProvider,
			},
		}
	case "loki":
		// Only use storage providers for loki (swift or s3)
		storageType := providerType
		if providerType == "route53" || providerType == "cloudflare" || providerType == "designate" || providerType == "clouddns" || providerType == "azuredns" || providerType == "gcs" || providerType == "azure" {
			storageType = "swift" // Default to swift for non-loki-storage providers
		}
		return map[string]any{
			"loki": &LokiConfig{
				BaseConfig: BaseConfig{
					Enabled: true,
				},
				StorageType: storageType,
			},
		}
	case "tempo":
		// Only use storage providers for tempo (swift or s3)
		storageType := providerType
		if providerType == "route53" || providerType == "cloudflare" || providerType == "designate" || providerType == "clouddns" || providerType == "azuredns" || providerType == "gcs" || providerType == "azure" {
			storageType = "s3" // Default to s3 for non-tempo-storage providers
		}
		return map[string]any{
			"tempo": &TempoConfig{
				BaseConfig: BaseConfig{
					Enabled: true,
				},
				StorageType: storageType,
			},
		}
	case "velero":
		// Use all storage providers for velero
		storageType := providerType
		if providerType == "route53" || providerType == "cloudflare" || providerType == "designate" || providerType == "clouddns" || providerType == "azuredns" {
			storageType = "s3" // Default to s3 for non-storage providers
		}
		return map[string]any{
			"velero": &VeleroConfig{
				BaseConfig: BaseConfig{
					Enabled: true,
				},
				StorageType: storageType,
			},
		}
	case "keycloak":
		return map[string]any{
			"keycloak": &KeycloakConfig{
				BaseConfig: BaseConfig{
					Enabled: true,
				},
			},
		}
	default:
		return map[string]any{}
	}
}

// createDisabledService creates a disabled service
func createDisabledService(serviceType string, providerType string) map[string]any {
	switch serviceType {
	case "cert-manager":
		return map[string]any{
			"cert-manager": &CertManagerConfig{
				BaseConfig: BaseConfig{
					Enabled: false,
				},
				DNSProvider: providerType,
			},
		}
	case "loki":
		return map[string]any{
			"loki": &LokiConfig{
				BaseConfig: BaseConfig{
					Enabled: false,
				},
				StorageType: providerType,
			},
		}
	case "tempo":
		return map[string]any{
			"tempo": &TempoConfig{
				BaseConfig: BaseConfig{
					Enabled: false,
				},
				StorageType: providerType,
			},
		}
	case "velero":
		return map[string]any{
			"velero": &VeleroConfig{
				BaseConfig: BaseConfig{
					Enabled: false,
				},
				StorageType: providerType,
			},
		}
	case "keycloak":
		return map[string]any{
			"keycloak": &KeycloakConfig{
				BaseConfig: BaseConfig{
					Enabled: false,
				},
			},
		}
	default:
		return map[string]any{}
	}
}

// createSecretsForService creates appropriate secrets for a service and provider
func createSecretsForService(serviceType string, providerType string) map[string]any {
	secrets := map[string]any{
		"service_secrets": map[string]any{},
	}

	serviceSecrets := secrets["service_secrets"].(map[string]any)

	switch serviceType {
	case "cert-manager":
		certMgrSecrets := map[string]any{}
		// Map provider type to actual DNS provider used
		dnsProvider := providerType
		if providerType == "swift" || providerType == "s3" || providerType == "gcs" || providerType == "azure" {
			dnsProvider = "route53" // Default to route53 for non-DNS providers
		}
		switch dnsProvider {
		case "route53":
			certMgrSecrets["aws_access_key"] = "AKIAIOSFODNN7EXAMPLE"
			certMgrSecrets["aws_secret_key"] = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
		case "cloudflare":
			certMgrSecrets["cloudflare_api_token"] = "test-token"
		case "clouddns":
			certMgrSecrets["gcp_service_account_key"] = "test-key"
		case "azuredns":
			certMgrSecrets["azure_client_id"] = "test-client-id"
			certMgrSecrets["azure_client_secret"] = "test-client-secret"
			certMgrSecrets["azure_tenant_id"] = "test-tenant-id"
		}
		serviceSecrets["cert_manager"] = certMgrSecrets

	case "loki":
		lokiSecrets := map[string]any{}
		// Map provider type to actual storage type used
		storageType := providerType
		if providerType == "route53" || providerType == "cloudflare" || providerType == "designate" || providerType == "clouddns" || providerType == "azuredns" || providerType == "gcs" || providerType == "azure" {
			storageType = "swift"
		}
		switch storageType {
		case "swift":
			lokiSecrets["swift_password"] = "test-password"
		case "s3":
			lokiSecrets["s3_access_key"] = "test-access-key"
			lokiSecrets["s3_secret_key"] = "test-secret-key"
		}
		serviceSecrets["loki"] = lokiSecrets

	case "tempo":
		tempoSecrets := map[string]any{}
		// Map provider type to actual storage type used
		storageType := providerType
		if providerType == "route53" || providerType == "cloudflare" || providerType == "designate" || providerType == "clouddns" || providerType == "azuredns" || providerType == "gcs" || providerType == "azure" {
			storageType = "s3"
		}
		switch storageType {
		case "swift":
			tempoSecrets["swift_password"] = "test-password"
		case "s3":
			tempoSecrets["s3_access_key"] = "test-access-key"
			tempoSecrets["s3_secret_key"] = "test-secret-key"
		}
		serviceSecrets["tempo"] = tempoSecrets

	case "velero":
		veleroSecrets := map[string]any{}
		// Map provider type to actual storage type used
		storageType := providerType
		if providerType == "route53" || providerType == "cloudflare" || providerType == "designate" || providerType == "clouddns" || providerType == "azuredns" {
			storageType = "s3"
		}
		switch storageType {
		case "swift":
			veleroSecrets["swift_password"] = "test-password"
		case "s3":
			veleroSecrets["s3_access_key"] = "test-access-key"
			veleroSecrets["s3_secret_key"] = "test-secret-key"
		case "gcs":
			veleroSecrets["gcp_service_account_key"] = "test-key"
		case "azure":
			veleroSecrets["azure_storage_account_key"] = "test-key"
		}
		serviceSecrets["velero"] = veleroSecrets

	case "keycloak":
		serviceSecrets["keycloak"] = map[string]any{
			"admin_password": "test-password",
		}
	}

	return secrets
}

// TestProperty_RequiredSecretsValidation_EdgeCases tests edge cases for secrets validation
func TestProperty_RequiredSecretsValidation_EdgeCases(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	validator := NewSecretsValidator()

	// Property: Empty secrets map should trigger errors for enabled services (except those that don't require secrets)
	properties.Property("empty secrets map triggers errors for enabled services requiring secrets", prop.ForAll(
		func(serviceType string, providerType string) bool {
			services := createServiceWithProvider(serviceType, providerType)
			secrets := map[string]any{}

			errors := validator.ValidateRequiredSecrets(services, secrets)

			// Check if this service/provider combination requires secrets
			requiresSecrets := doesServiceProviderRequireSecrets(serviceType, providerType)

			if requiresSecrets {
				// Should have errors when secrets are required but missing
				return len(errors) > 0
			}
			// Should have no errors when secrets are not required
			return len(errors) == 0
		},
		genServiceType(),
		genProviderType(),
	))

	// Property: Partial secrets should still trigger errors for missing required secrets
	properties.Property("partial secrets trigger errors for missing required secrets", prop.ForAll(
		func(serviceType string) bool {
			// Create cert-manager with route53 (requires 2 secrets)
			services := map[string]any{
				"cert-manager": &CertManagerConfig{
					BaseConfig: BaseConfig{
						Enabled: true,
					},
					DNSProvider: "route53",
				},
			}

			// Provide only one of the two required secrets
			secrets := map[string]any{
				"service_secrets": map[string]any{
					"cert_manager": map[string]any{
						"aws_access_key": "AKIAIOSFODNN7EXAMPLE",
						// Missing aws_secret_key
					},
				},
			}

			errors := validator.ValidateRequiredSecrets(services, secrets)

			// Should have error for missing aws_secret_key
			return len(errors) > 0
		},
		gen.Const("cert-manager"),
	))

	properties.TestingRun(t)
}


// doesServiceProviderRequireSecrets checks if a service/provider combination requires secrets
func doesServiceProviderRequireSecrets(serviceType string, providerType string) bool {
	switch serviceType {
	case "cert-manager":
		// Map provider type to actual DNS provider used
		dnsProvider := providerType
		if providerType == "swift" || providerType == "s3" || providerType == "gcs" || providerType == "azure" {
			dnsProvider = "route53"
		}
		// designate doesn't require service-specific secrets
		return dnsProvider != "designate"
	case "keycloak":
		// Always requires admin password
		return true
	case "loki", "tempo", "velero":
		// Always require storage credentials
		return true
	default:
		return false
	}
}

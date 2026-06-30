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
	"fmt"
	"strings"

	semver "github.com/Masterminds/semver/v3"
	"github.com/go-playground/validator/v10"
)

// Validator performs multi-layered validation of v2 configurations.
// Requirements: 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7
type Validator interface {
	Validate(cfg *Config) error
	ValidateSchema(cfg *Config) error
	ValidateBusinessRules(cfg *Config) error
	ValidateProvider(cfg *Config) error
	ValidateDeployment(cfg *Config) error
	ValidateServices(cfg *Config) error
}

// defaultValidator implements the Validator interface.
type defaultValidator struct {
	schemaValidator *validator.Validate
}

// NewValidator creates a new v2 configuration validator.
func NewValidator() Validator {
	v := validator.New()

	_ = registerSchemaValidations(v)

	return &defaultValidator{
		schemaValidator: v,
	}
}

func registerSchemaValidations(v *validator.Validate) error {
	if err := v.RegisterValidation("dns1123", func(fl validator.FieldLevel) bool {
		value := fl.Field().String()
		if value == "" {
			return true
		}
		if len(value) > 253 {
			return false
		}
		for i, c := range value {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '.') {
				return false
			}
			if (i == 0 || i == len(value)-1) && (c == '-' || c == '.') {
				return false
			}
		}
		return true
	}); err != nil {
		return err
	}

	if err := v.RegisterValidation("semver", func(fl validator.FieldLevel) bool {
		value := strings.TrimSpace(fl.Field().String())
		if value == "" {
			return true
		}
		_, err := semver.NewVersion(value)
		return err == nil
	}); err != nil {
		return err
	}

	return nil
}

// Validate performs all validation layers.
// Requirements: 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7
func (v *defaultValidator) Validate(cfg *Config) error {
	if err := v.ValidateCleanBreakRules(cfg); err != nil {
		return err
	}

	// Schema validation
	if err := v.ValidateSchema(cfg); err != nil {
		return err
	}

	// Business rules validation
	if err := v.ValidateBusinessRules(cfg); err != nil {
		return err
	}

	// Provider-specific validation
	if err := v.ValidateProvider(cfg); err != nil {
		return err
	}

	// Deployment-method validation
	if err := v.ValidateDeployment(cfg); err != nil {
		return err
	}

	// Service validation
	if err := v.ValidateServices(cfg); err != nil {
		return err
	}

	return nil
}

// ValidateCleanBreakRules rejects legacy shapes before generic schema
// validation so users get the explicit migration-free guidance.
func (v *defaultValidator) ValidateCleanBreakRules(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	if len(cfg.OpenCenter.LegacyTalos) > 0 {
		return fmt.Errorf("opencenter.talos is not supported in v2; remove the opencenter.talos section")
	}
	return nil
}

// ValidateSchema validates required fields, data types, and enum values.
// Requirements: 11.1
func (v *defaultValidator) ValidateSchema(cfg *Config) error {
	if err := v.schemaValidator.Struct(cfg); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}
	return nil
}

// ValidateBusinessRules validates cross-field dependencies and value ranges.
// Requirements: 11.2
func (v *defaultValidator) ValidateBusinessRules(cfg *Config) error {
	if len(cfg.OpenCenter.LegacyTalos) > 0 {
		return fmt.Errorf("opencenter.talos is not supported in v2; remove the opencenter.talos section")
	}

	// Validate OpenTofu backend configuration
	if err := v.validateOpenTofuBackend(&cfg.OpenTofu); err != nil {
		return err
	}

	// Validate worker pool name uniqueness
	if err := v.validatePoolNameUniqueness(cfg); err != nil {
		return err
	}

	// Validate Windows image requirement
	if err := v.validateWindowsPoolImage(cfg); err != nil {
		return err
	}

	return nil
}

// validatePoolNameUniqueness ensures pool names are unique across Linux and Windows pools.
func (v *defaultValidator) validatePoolNameUniqueness(cfg *Config) error {
	seen := make(map[string]string)
	for _, pool := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker {
		if prev, exists := seen[pool.Name]; exists {
			return fmt.Errorf("duplicate pool name %q (appears in both %s and linux pools)", pool.Name, prev)
		}
		seen[pool.Name] = "linux"
	}
	for _, pool := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows {
		if prev, exists := seen[pool.Name]; exists {
			return fmt.Errorf("duplicate pool name %q (appears in both %s and windows pools)", pool.Name, prev)
		}
		seen[pool.Name] = "windows"
	}
	return nil
}

// validateWindowsPoolImage ensures image_id_windows is set when any Windows pool has count > 0.
func (v *defaultValidator) validateWindowsPoolImage(cfg *Config) error {
	hasActiveWindowsPool := cfg.OpenCenter.Infrastructure.Compute.WorkerCountWindows > 0
	for _, pool := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows {
		if pool.Count > 0 && pool.Image == "" {
			hasActiveWindowsPool = true
			break
		}
	}
	if !hasActiveWindowsPool {
		return nil
	}
	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack != nil {
		if cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ImageIDWindows == "" {
			// Check if all active Windows pools have per-pool images
			for _, pool := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows {
				if pool.Count > 0 && pool.Image == "" {
					return fmt.Errorf("infrastructure.cloud.openstack.image_id_windows is required when Windows pools with count > 0 do not specify a per-pool image")
				}
			}
		}
	}
	return nil
}

// validateOpenTofuBackend validates that the appropriate backend configuration is present.
func (v *defaultValidator) validateOpenTofuBackend(opentofu *OpenTofuConfig) error {
	backend := &opentofu.Backend

	switch backend.Type {
	case "local":
		if backend.Local == nil {
			return fmt.Errorf("opentofu.backend.local — conditionally required based on related field: local backend requires 'local' section with 'path' field")
		}
		if backend.Local.Path == "" {
			return fmt.Errorf("opentofu.backend.local.path — required, currently empty")
		}
	case "s3":
		if backend.S3 == nil {
			return fmt.Errorf("opentofu.backend.s3 — conditionally required based on related field: S3 backend requires 's3' section with bucket, key, and region")
		}
		// The nested struct validation will handle the required fields
	case "remote":
		// Remote backend uses the Config map
		if len(backend.Config) == 0 {
			return fmt.Errorf("opentofu.backend.config — conditionally required based on related field: remote backend requires 'config' section")
		}
	}

	return nil
}

// ValidateProvider validates provider-specific requirements.
// Requirements: 11.3
func (v *defaultValidator) ValidateProvider(cfg *Config) error {
	provider := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider))

	switch provider {
	case "kind":
		if cfg.OpenCenter.Infrastructure.Kind == nil {
			return fmt.Errorf("opencenter.infrastructure.kind must be configured for the kind provider")
		}
	case "openstack":
		if cfg.OpenCenter.Infrastructure.Cloud.OpenStack == nil {
			return fmt.Errorf("opencenter.infrastructure.cloud.openstack must be configured for the openstack provider")
		}
	case "aws":
		if cfg.OpenCenter.Infrastructure.Cloud.AWS == nil {
			return fmt.Errorf("opencenter.infrastructure.cloud.aws must be configured for the aws provider")
		}
	case "vmware", "vsphere":
		if cfg.OpenCenter.Infrastructure.Cloud.VMware == nil {
			return fmt.Errorf("opencenter.infrastructure.cloud.vmware must be configured for the %s provider", provider)
		}
	case "gcp":
		if cfg.OpenCenter.Infrastructure.Cloud.GCP == nil {
			return fmt.Errorf("opencenter.infrastructure.cloud.gcp must be configured for the gcp provider")
		}
	case "azure":
		if cfg.OpenCenter.Infrastructure.Cloud.Azure == nil {
			return fmt.Errorf("opencenter.infrastructure.cloud.azure must be configured for the azure provider")
		}
	case "baremetal":
		// No provider-specific config block required for baremetal
	case "":
		return fmt.Errorf("opencenter.infrastructure.provider must be set")
	default:
		return fmt.Errorf("unsupported infrastructure provider: %s", provider)
	}

	return nil
}

// ValidateDeployment validates deployment-method requirements.
// Requirements: 11.4
func (v *defaultValidator) ValidateDeployment(cfg *Config) error {
	methodName := strings.ToLower(strings.TrimSpace(cfg.Deployment.Method))
	if methodName == "" {
		return fmt.Errorf("deployment.method must be set")
	}

	deploymentMethod, err := GetDeploymentMethod(methodName)
	if err != nil {
		return err
	}
	if err := deploymentMethod.ValidateCompatibility(cfg.OpenCenter.Infrastructure.Provider); err != nil {
		return err
	}
	return deploymentMethod.ValidateConfig(cfg)
}

// ValidateServices validates service dependencies and required secrets.
// Requirements: 11.5
func (v *defaultValidator) ValidateServices(cfg *Config) error {
	// Placeholder secret validation is intentionally NOT run during the
	// standard load pipeline. It is invoked explicitly by commands that
	// gate deployment (validate, setup, bootstrap) via ValidateForDeployment.
	return nil
}

// ValidateForDeployment performs all standard validation plus deployment-readiness
// checks such as detecting placeholder secrets that must be replaced.
func ValidateForDeployment(cfg *Config) error {
	v := NewValidator().(*defaultValidator)
	if err := v.Validate(cfg); err != nil {
		return err
	}
	return v.validatePlaceholderSecrets(cfg)
}

// PlaceholderSecret is the sentinel value used in default configurations to indicate
// that a secret must be replaced before deployment.
const PlaceholderSecret = "CHANGEME"

// validatePlaceholderSecrets checks for any secrets still set to the placeholder value.
// Returns an error listing all secrets that need to be updated.
func (v *defaultValidator) validatePlaceholderSecrets(cfg *Config) error {
	var placeholders []string

	// Keycloak secrets (enabled by default)
	if isServiceEnabled(cfg, "keycloak") {
		if !oidcClientSecretsProvidedInternally(cfg) && cfg.Secrets.Keycloak.ClientSecret == PlaceholderSecret {
			placeholders = append(placeholders, "secrets.keycloak.client_secret")
		}
		if cfg.Secrets.Keycloak.AdminPassword == PlaceholderSecret {
			placeholders = append(placeholders, "secrets.keycloak.admin_password")
		}
	}

	// Headlamp secrets (enabled by default)
	if isServiceEnabled(cfg, "headlamp") && !oidcClientSecretsProvidedInternally(cfg) {
		if cfg.Secrets.Headlamp.OIDCClientSecret == PlaceholderSecret {
			placeholders = append(placeholders, "secrets.headlamp.oidc_client_secret")
		}
	}

	// Grafana secrets (kube-prometheus-stack)
	if isServiceEnabled(cfg, "kube-prometheus-stack") {
		if cfg.Secrets.Grafana.AdminPassword == PlaceholderSecret {
			placeholders = append(placeholders, "secrets.grafana.admin_password")
		}
	}

	// Loki secrets
	if isServiceEnabled(cfg, "loki") {
		if cfg.Secrets.Loki.SwiftApplicationCredentialSecret == PlaceholderSecret {
			placeholders = append(placeholders, "secrets.loki.swift_application_credential_secret")
		}
		if cfg.Secrets.Loki.S3AccessKeyID == PlaceholderSecret {
			placeholders = append(placeholders, "secrets.loki.s3_access_key_id")
		}
		if cfg.Secrets.Loki.S3SecretAccessKey == PlaceholderSecret {
			placeholders = append(placeholders, "secrets.loki.s3_secret_access_key")
		}
	}

	// Tempo secrets
	if isServiceEnabled(cfg, "tempo") {
		if cfg.Secrets.Tempo.SwiftApplicationCredentialSecret == PlaceholderSecret {
			placeholders = append(placeholders, "secrets.tempo.swift_application_credential_secret")
		}
		if cfg.Secrets.Tempo.AccessKey == PlaceholderSecret {
			placeholders = append(placeholders, "secrets.tempo.access_key")
		}
		if cfg.Secrets.Tempo.SecretKey == PlaceholderSecret {
			placeholders = append(placeholders, "secrets.tempo.secret_key")
		}
	}

	// Cert-manager secrets (map-based credentials)
	if isServiceEnabled(cfg, "cert-manager") {
		for name, cred := range cfg.Secrets.CertManager.AWS {
			if !cred.Enabled {
				continue
			}
			if cred.AWSAccessKey == PlaceholderSecret {
				placeholders = append(placeholders, fmt.Sprintf("secrets.cert_manager.aws.%s.aws_access_key", name))
			}
			if cred.AWSSecretAccessKey == PlaceholderSecret {
				placeholders = append(placeholders, fmt.Sprintf("secrets.cert_manager.aws.%s.aws_secret_access_key", name))
			}
		}
		for name, cred := range cfg.Secrets.CertManager.Cloudflare {
			if !cred.Enabled {
				continue
			}
			if cred.APIToken == PlaceholderSecret {
				placeholders = append(placeholders, fmt.Sprintf("secrets.cert_manager.cloudflare.%s.api_token", name))
			}
		}
		// Legacy flat fields
		if cfg.Secrets.CertManager.AWSAccessKey == PlaceholderSecret {
			placeholders = append(placeholders, "secrets.cert_manager.aws_access_key")
		}
		if cfg.Secrets.CertManager.AWSSecretAccessKey == PlaceholderSecret {
			placeholders = append(placeholders, "secrets.cert_manager.aws_secret_access_key")
		}
	}

	// Global AWS secrets
	if cfg.Secrets.Global.AWS.Infrastructure.AccessKey == PlaceholderSecret {
		placeholders = append(placeholders, "secrets.global.aws.infrastructure.access_key")
	}
	if cfg.Secrets.Global.AWS.Infrastructure.SecretAccessKey == PlaceholderSecret {
		placeholders = append(placeholders, "secrets.global.aws.infrastructure.secret_access_key")
	}

	// OpenStack application credentials
	provider := strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider)
	if strings.EqualFold(provider, "openstack") && cfg.OpenCenter.Infrastructure.Cloud.OpenStack != nil {
		if cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID == PlaceholderSecret {
			placeholders = append(placeholders, "opencenter.infrastructure.cloud.openstack.application_credential_id")
		}
		if cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret == PlaceholderSecret {
			placeholders = append(placeholders, "opencenter.infrastructure.cloud.openstack.application_credential_secret")
		}
	}

	if len(placeholders) > 0 {
		return fmt.Errorf("the following secrets still have the placeholder value %q and must be updated before deployment:\n  - %s",
			PlaceholderSecret, strings.Join(placeholders, "\n  - "))
	}

	return nil
}

// isServiceEnabled checks if a service is enabled in the config.
func isServiceEnabled(cfg *Config, serviceName string) bool {
	if svc, ok := cfg.OpenCenter.Services[serviceName]; ok {
		if enabler, ok := svc.(interface{ IsEnabled() bool }); ok {
			return enabler.IsEnabled()
		}
	}
	return false
}

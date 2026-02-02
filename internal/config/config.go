// Copyrigho 2025 Victor Palma <victor.palma@rackspace.com>
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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/config/services"
)

// Types are now defined in types.go

// Default functions are now in defaults.go

// I/O functions are now in persistence.go

// Validate performs a set of invariant checks on the configuration.
//
// Inputs:
//   - cfg: The configuration to validate.
//
// Outputs:
//   - []string: A list of error messages describing any validation failures.
//     If the list is empty, the configuration is valid.
func Validate(cfg Config) []string {
	var errs []string
	// Required cluster name and opencenter.gitops.git_dir
	if cfg.ClusterName() == "" {
		errs = append(errs, "opencenter.cluster.cluster_name must be set")
	}
	if cfg.GitOps().GitDir == "" {
		errs = append(errs, "GitOps directory must be set")
	}
	// OpenTofu validation
	if cfg.OpenTofu.Enabled {
		if cfg.OpenTofu.Path == "" {
			errs = append(errs, "opentofu.path must be set when opentofu.enabled=true")
		}
		bt := strings.ToLower(strings.TrimSpace(cfg.OpenTofu.Backend.Type))
		if bt == "" {
			bt = "local"
		}
		switch bt {
		case "local":
			if cfg.OpenTofu.Backend.Local.Path == "" {
				errs = append(errs, "opentofu.backend.local.path must be set for local backend")
			}
		case "s3", "aws":
			s3 := cfg.OpenTofu.Backend.S3
			if s3.Bucket == "" || s3.Key == "" || s3.Region == "" {
				errs = append(errs, "opentofu.backend.s3 requires bucket, key, and region")
			}
			// When using S3/AWS backend, AWS credentials must be provided via opencenter cluster or global AWS secrets
			clusterAccessKey := strings.TrimSpace(cfg.OpenCenter.Cluster.AWSAccessKey)
			clusterSecretKey := strings.TrimSpace(cfg.OpenCenter.Cluster.AWSSecretAccessKey)

			// Check new global infrastructure credentials
			globalInfraAccessKey := strings.TrimSpace(cfg.Secrets.Global.AWS.Infrastructure.AccessKey)
			globalInfraSecretKey := strings.TrimSpace(cfg.Secrets.Global.AWS.Infrastructure.SecretAccessKey)

			// Check if any valid credential combination exists
			hasClusterCreds := clusterAccessKey != "" && clusterSecretKey != ""
			hasInfraCreds := globalInfraAccessKey != "" && globalInfraSecretKey != ""

			if !hasClusterCreds && !hasInfraCreds {
				errs = append(errs, "AWS credentials required for S3/AWS backend: either set opencenter.cluster.aws_access_key/aws_secret_access_key or secrets.global.aws.infrastructure.access_key/secret_access_key")
			}
		default:
			errs = append(errs, "opentofu.backend.type must be 'local', 's3', or 'aws'")
		}
	}
	// iac validation is intentionally minimal for variables.tf-aligned shape

	// Network plugin validation - ensure only one is enabled
	networkPlugins := []struct {
		name    string
		enabled bool
	}{
		{"Calico", cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled},
		{"Cilium", cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled},
		{"Kube-OVN", cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled},
	}

	enabledCount := 0
	var enabledPlugins []string
	for _, plugin := range networkPlugins {
		if plugin.enabled {
			enabledCount++
			enabledPlugins = append(enabledPlugins, plugin.name)
		}
	}

	if enabledCount == 0 {
		errs = append(errs, "at least one network plugin (Calico, Cilium, or Kube-OVN) must be enabled")
	} else if enabledCount > 1 {
		errs = append(errs, fmt.Sprintf("only one network plugin can be enabled at a time, but found: %s", strings.Join(enabledPlugins, ", ")))
	}

	// Windows node validation - exclude Windows blocks when worker_count_windows = 0
	if cfg.OpenCenter.Cluster.Kubernetes.WorkerCountWindows == 0 {
		// Windows workers should be disabled when count is 0
		if cfg.OpenCenter.Cluster.Kubernetes.WindowsWorkers.Enabled {
			errs = append(errs, "windows_workers.enabled must be false when worker_count_windows is 0")
		}
	}

	// Validate services: only one of release or branch can be set
	for serviceName, serviceCfgAny := range cfg.OpenCenter.Services {
		// All services embed BaseConfig, but we can't cast directly to *BaseConfig
		// because they are different types. We use reflection to access the fields.
		val := reflect.ValueOf(serviceCfgAny)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		if val.Kind() == reflect.Struct {
			// Check if struct has BaseConfig embedded or Release/Branch fields directly
			// Since BaseConfig is embedded, its fields are promoted
			releaseField := val.FieldByName("Release")
			branchField := val.FieldByName("Branch")

			if releaseField.IsValid() && branchField.IsValid() {
				release := releaseField.String()
				branch := branchField.String()

				if release != "" && branch != "" {
					errs = append(errs, fmt.Sprintf("service '%s': only one of 'release' or 'branch' can be set, not both", serviceName))
				}
			}
		}
	}

	// Validate managed services: only one of release or branch can be set
	for serviceName, serviceCfgAny := range cfg.OpenCenter.ManagedService {
		val := reflect.ValueOf(serviceCfgAny)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		if val.Kind() == reflect.Struct {
			releaseField := val.FieldByName("Release")
			branchField := val.FieldByName("Branch")

			if releaseField.IsValid() && branchField.IsValid() {
				release := releaseField.String()
				branch := branchField.String()

				if release != "" && branch != "" {
					errs = append(errs, fmt.Sprintf("managed-service '%s': only one of 'release' or 'branch' can be set, not both", serviceName))
				}
			}
		}
	}

	// Validate GitOps: only one of release or branch can be set
	if cfg.OpenCenter.GitOps.Release != "" && cfg.OpenCenter.GitOps.Branch != "" {
		errs = append(errs, "gitops: only one of 'release' or 'branch' can be set, not both")
	}

	// Validate service secrets
	errs = append(errs, validateServiceSecretsSimple(cfg)...)

	// Validate OpenStack provider configuration
	if cfg.OpenCenter.Infrastructure.Provider == "openstack" {
		if cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL == "" {
			errs = append(errs, "opencenter.infrastructure.cloud.openstack.auth_url must be set when provider is openstack")
		}
		if cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region == "" {
			errs = append(errs, "opencenter.infrastructure.cloud.openstack.region must be set when provider is openstack")
		}
	}
	// Validate Barbican configuration if enabled
	if cfg.OpenCenter.Secrets.Backend == "barbican" {
		if cfg.OpenCenter.Secrets.Barbican.AuthURL == "" {
			errs = append(errs, "opencenter.secrets.barbican.auth_url must be set when secrets backend is barbican")
		}
	}

	return errs
}

// validateServiceSecretsSimple validates service-specific secrets configuration.
// This function checks that required secrets are present when corresponding services are enabled.
//
// Deprecated: This function will be migrated to use internal/core/validation.ValidationEngine in v2.0.0.
// For now, it remains as an internal helper for the main Validate function.
func validateServiceSecretsSimple(cfg Config) []string {
	var errs []string

	isEnabled := func(name string) bool {
		svc, exists := cfg.OpenCenter.Services[name]
		if !exists {
			return false
		}
		if svcConf, ok := svc.(services.ServiceConfig); ok {
			return svcConf.IsEnabled()
		}
		return false
	}

	// Validate cert-manager secrets
	if isEnabled("cert-manager") {
		accessKey, secretKey := cfg.GetCertManagerAWSCredentials()
		if accessKey == "" {
			errs = append(errs, "AWS credentials required for cert-manager: either set secrets.cert_manager.aws_access_key or secrets.global.aws.application.access_key or secrets.global.aws.infrastructure.access_key")
		}
		if secretKey == "" {
			errs = append(errs, "AWS credentials required for cert-manager: either set secrets.cert_manager.aws_secret_access_key or secrets.global.aws.application.secret_access_key or secrets.global.aws.infrastructure.secret_access_key")
		}
	}

	// Validate loki secrets
	if isEnabled("loki") {
		// Check for Swift credentials (legacy)
		if cfg.Secrets.Loki.SwiftPassword == "" {
			// If no Swift password, check for S3 credentials (with fallback)
			accessKey, secretKey := cfg.GetLokiS3Credentials()
			if accessKey == "" || secretKey == "" {
				errs = append(errs, "Loki requires either Swift password (secrets.loki.swift_password) or S3 credentials (secrets.loki.s3_access_key_id/secrets.loki.s3_secret_access_key or secrets.global.aws.application.access_key/secret_access_key or secrets.global.aws.infrastructure.access_key/secret_access_key)")
			}
		}
	}

	// Validate tempo secrets
	if isEnabled("tempo") {
		accessKey, secretKey := cfg.GetTempoS3Credentials()
		if accessKey == "" {
			errs = append(errs, "S3 credentials required for Tempo: either set secrets.tempo.access_key or secrets.global.aws.application.access_key or secrets.global.aws.infrastructure.access_key")
		}
		if secretKey == "" {
			errs = append(errs, "S3 credentials required for Tempo: either set secrets.tempo.secret_key or secrets.global.aws.application.secret_access_key or secrets.global.aws.infrastructure.secret_access_key")
		}
	}

	// Validate keycloak secrets
	if isEnabled("keycloak") {
		if cfg.Secrets.Keycloak.AdminPassword == "" {
			errs = append(errs, "secrets.keycloak.admin_password is required when keycloak is enabled")
		}
	}

	return errs
}

// ToJSON marshals the configuration to JSON. This is used for generating
// the JSON schema and for other tools that consume JSON.
//
// Outputs:
//   - []byte: The JSON-encoded configuration.
//   - error: An error if the configuration cannot be marshaled.
func (c Config) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// GetAWSCredentials returns AWS credentials with service-specific override and fallback logic.
// It first tries service-specific credentials, then falls back to global infrastructure credentials.
//
// Parameters:
//   - serviceAccessKey: Service-specific AWS access key
//   - serviceSecretKey: Service-specific AWS secret access key
//
// Returns:
//   - accessKey: The resolved AWS access key
//   - secretKey: The resolved AWS secret access key
func (c Config) GetAWSCredentials(serviceAccessKey, serviceSecretKey string) (accessKey, secretKey string) {
	// Use service-specific credentials if provided
	if serviceAccessKey != "" && serviceSecretKey != "" {
		return serviceAccessKey, serviceSecretKey
	}

	// Fall back to global infrastructure AWS credentials
	return c.Secrets.Global.AWS.Infrastructure.AccessKey, c.Secrets.Global.AWS.Infrastructure.SecretAccessKey
}

// GetCertManagerAWSCredentials returns cert-manager AWS credentials with fallback to global AWS application credentials.
func (c Config) GetCertManagerAWSCredentials() (accessKey, secretKey string) {
	// Use service-specific credentials if provided
	if c.Secrets.CertManager.AWSAccessKey != "" && c.Secrets.CertManager.AWSSecretAccessKey != "" {
		return c.Secrets.CertManager.AWSAccessKey, c.Secrets.CertManager.AWSSecretAccessKey
	}

	// Fall back to global application AWS credentials
	return c.GetAWSApplicationCredentials()
}

// GetLokiS3Credentials returns Loki S3 credentials with fallback to global AWS application credentials.
func (c Config) GetLokiS3Credentials() (accessKey, secretKey string) {
	// Use service-specific credentials if provided
	if c.Secrets.Loki.S3AccessKeyID != "" && c.Secrets.Loki.S3SecretAccessKey != "" {
		return c.Secrets.Loki.S3AccessKeyID, c.Secrets.Loki.S3SecretAccessKey
	}

	// Fall back to global application AWS credentials
	return c.GetAWSApplicationCredentials()
}

// GetTempoS3Credentials returns Tempo S3 credentials with fallback to global AWS application credentials.
func (c Config) GetTempoS3Credentials() (accessKey, secretKey string) {
	// Use service-specific credentials if provided
	if c.Secrets.Tempo.AccessKey != "" && c.Secrets.Tempo.SecretKey != "" {
		return c.Secrets.Tempo.AccessKey, c.Secrets.Tempo.SecretKey
	}

	// Fall back to global application AWS credentials
	return c.GetAWSApplicationCredentials()
}

// GetS3BackendCredentials returns S3 backend credentials with fallback to global AWS credentials.
func (c Config) GetS3BackendCredentials() (accessKey, secretKey string) {
	return c.GetAWSCredentials(c.OpenCenter.Cluster.AWSAccessKey, c.OpenCenter.Cluster.AWSSecretAccessKey)
}

// GetAWSApplicationCredentials returns AWS application credentials with fallback logic.
// It first tries the global application credentials, then falls back to infrastructure credentials.
//
// Returns:
//   - accessKey: The resolved AWS access key
//   - secretKey: The resolved AWS secret access key
func (c Config) GetAWSApplicationCredentials() (accessKey, secretKey string) {
	// Use global application AWS credentials if provided
	if c.Secrets.Global.AWS.Application.AccessKey != "" && c.Secrets.Global.AWS.Application.SecretAccessKey != "" {
		return c.Secrets.Global.AWS.Application.AccessKey, c.Secrets.Global.AWS.Application.SecretAccessKey
	}

	// Fall back to infrastructure credentials
	return c.Secrets.Global.AWS.Infrastructure.AccessKey, c.Secrets.Global.AWS.Infrastructure.SecretAccessKey
}

// Template-friendly functions that return single values for use in Go templates

// GetCertManagerAWSAccessKey returns cert-manager AWS access key with fallback.
func (c Config) GetCertManagerAWSAccessKey() string {
	accessKey, _ := c.GetCertManagerAWSCredentials()
	return accessKey
}

// GetCertManagerAWSSecretKey returns cert-manager AWS secret key with fallback.
func (c Config) GetCertManagerAWSSecretKey() string {
	_, secretKey := c.GetCertManagerAWSCredentials()
	return secretKey
}

// GetLokiS3AccessKey returns Loki S3 access key with fallback.
func (c Config) GetLokiS3AccessKey() string {
	accessKey, _ := c.GetLokiS3Credentials()
	return accessKey
}

// GetLokiS3SecretKey returns Loki S3 secret key with fallback.
func (c Config) GetLokiS3SecretKey() string {
	_, secretKey := c.GetLokiS3Credentials()
	return secretKey
}

// GetTempoS3AccessKey returns Tempo S3 access key with fallback.
func (c Config) GetTempoS3AccessKey() string {
	accessKey, _ := c.GetTempoS3Credentials()
	return accessKey
}

// GetTempoS3SecretKey returns Tempo S3 secret key with fallback.
func (c Config) GetTempoS3SecretKey() string {
	_, secretKey := c.GetTempoS3Credentials()
	return secretKey
}

// GetS3BackendAccessKey returns S3 backend access key with fallback.
func (c Config) GetS3BackendAccessKey() string {
	accessKey, _ := c.GetS3BackendCredentials()
	return accessKey
}

// GetS3BackendSecretKey returns S3 backend secret key with fallback.
func (c Config) GetS3BackendSecretKey() string {
	_, secretKey := c.GetS3BackendCredentials()
	return secretKey
}

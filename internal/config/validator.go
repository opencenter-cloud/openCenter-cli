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
	"fmt"
	"regexp"
	"strings"
)

// ClusterConfigValidator implements the ConfigValidatorInterface for comprehensive configuration validation.
type ClusterConfigValidator struct {
	autoRepair bool
}

// NewConfigValidator creates a new configuration validator.
func NewConfigValidator(autoRepair bool) *ClusterConfigValidator {
	return &ClusterConfigValidator{
		autoRepair: autoRepair,
	}
}

// Validate performs comprehensive validation on a configuration.
func (cv *ClusterConfigValidator) Validate(ctx context.Context, config *Config) *ConfigValidationResult {
	if config == nil {
		return &ConfigValidationResult{
			Valid: false,
			Errors: []*ConfigValidationError{
				{
					Type:    "validation",
					Field:   "config",
					Message: "configuration cannot be nil",
					Suggestions: []string{
						"Ensure configuration is properly loaded",
						"Check if configuration file exists",
					},
				},
			},
		}
	}

	result := &ConfigValidationResult{
		Valid:    true,
		Errors:   []*ConfigValidationError{},
		Warnings: []*ConfigValidationError{},
		Repaired: []*ConfigValidationError{},
	}

	// Validate structure
	cv.validateStructureWithResult(ctx, config, result)

	// Validate semantics
	cv.validateSemanticsWithResult(ctx, config, result)

	// Validate networking
	cv.validateNetworkingWithResult(ctx, config, result)

	// Validate cloud provider
	cv.validateCloudProviderWithResult(ctx, config, result)

	// Set overall validity
	result.Valid = len(result.Errors) == 0

	return result
}

// ValidateStructure validates the basic structure of a configuration.
func (cv *ClusterConfigValidator) ValidateStructure(ctx context.Context, config *Config) *ConfigValidationResult {
	result := &ConfigValidationResult{
		Valid:    true,
		Errors:   []*ConfigValidationError{},
		Warnings: []*ConfigValidationError{},
		Repaired: []*ConfigValidationError{},
	}

	cv.validateStructureWithResult(ctx, config, result)
	result.Valid = len(result.Errors) == 0

	return result
}

// ValidateSemantics validates the semantic correctness of a configuration.
func (cv *ClusterConfigValidator) ValidateSemantics(ctx context.Context, config *Config) *ConfigValidationResult {
	result := &ConfigValidationResult{
		Valid:    true,
		Errors:   []*ConfigValidationError{},
		Warnings: []*ConfigValidationError{},
		Repaired: []*ConfigValidationError{},
	}

	cv.validateSemanticsWithResult(ctx, config, result)
	result.Valid = len(result.Errors) == 0

	return result
}

// ValidateNetworking validates network plugin configuration.
func (cv *ClusterConfigValidator) ValidateNetworking(ctx context.Context, config *Config) *ConfigValidationResult {
	result := &ConfigValidationResult{
		Valid:    true,
		Errors:   []*ConfigValidationError{},
		Warnings: []*ConfigValidationError{},
		Repaired: []*ConfigValidationError{},
	}

	cv.validateNetworkingWithResult(ctx, config, result)
	result.Valid = len(result.Errors) == 0

	return result
}

// ValidateCloudProvider validates cloud provider specific configuration.
func (cv *ClusterConfigValidator) ValidateCloudProvider(ctx context.Context, config *Config) *ConfigValidationResult {
	result := &ConfigValidationResult{
		Valid:    true,
		Errors:   []*ConfigValidationError{},
		Warnings: []*ConfigValidationError{},
		Repaired: []*ConfigValidationError{},
	}

	cv.validateCloudProviderWithResult(ctx, config, result)
	result.Valid = len(result.Errors) == 0

	return result
}

// validateStructureWithResult validates the basic structure of a configuration.
func (cv *ClusterConfigValidator) validateStructureWithResult(ctx context.Context, config *Config, result *ConfigValidationResult) {
	// Validate required fields
	if config.ClusterName() == "" {
		result.Errors = append(result.Errors, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.cluster.cluster_name",
			Message: "cluster name must be set",
			Suggestions: []string{
				"Set opencenter.cluster.cluster_name to a valid cluster name",
				"Cluster name should be alphanumeric with hyphens and underscores",
			},
		})
	}

	if config.GitOps().GitDir == "" {
		result.Errors = append(result.Errors, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.gitops.git_dir",
			Message: "GitOps directory must be set",
			Suggestions: []string{
				"Set opencenter.gitops.git_dir to a valid directory path",
				"Use a path where GitOps repository will be created",
			},
		})
	}

	// Validate cluster name format
	if config.ClusterName() != "" {
		if err := ValidateClusterName(config.ClusterName()); err != nil {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "opencenter.cluster.cluster_name",
				Value:   config.ClusterName(),
				Message: fmt.Sprintf("invalid cluster name format: %v", err),
				Suggestions: []string{
					"Use alphanumeric characters, hyphens, and underscores only",
					"Start with an alphanumeric character",
					"Keep length under 255 characters",
				},
			})
		}
	}

	// Validate admin email format
	if config.OpenCenter.Cluster.AdminEmail != "" {
		if !cv.isValidEmail(config.OpenCenter.Cluster.AdminEmail) {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "opencenter.cluster.admin_email",
				Value:   config.OpenCenter.Cluster.AdminEmail,
				Message: "invalid email address format",
				Suggestions: []string{
					"Use valid email format (e.g., admin@example.com)",
					"Ensure email contains @ symbol and valid domain",
				},
			})
		}
	}

	// Validate base domain format
	if config.OpenCenter.Cluster.BaseDomain != "" {
		if !cv.isValidDomain(config.OpenCenter.Cluster.BaseDomain) {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "opencenter.cluster.base_domain",
				Value:   config.OpenCenter.Cluster.BaseDomain,
				Message: "invalid domain format",
				Suggestions: []string{
					"Use valid domain format (e.g., k8s.opencenter.cloud)",
					"Domain must contain at least one dot and valid TLD",
				},
			})
		}
	}

	// Validate cluster FQDN format
	if config.OpenCenter.Cluster.ClusterFQDN != "" {
		if !cv.isValidDomain(config.OpenCenter.Cluster.ClusterFQDN) {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "opencenter.cluster.cluster_fqdn",
				Value:   config.OpenCenter.Cluster.ClusterFQDN,
				Message: "invalid FQDN format",
				Suggestions: []string{
					"Use valid FQDN format (e.g., my-cluster.sjc3.k8s.opencenter.cloud)",
					"FQDN must contain at least one dot and valid TLD",
				},
			})
		}
	}

	// Validate Kubernetes version format
	k8sVersion := config.OpenCenter.Cluster.Kubernetes.Version
	if k8sVersion != "" && !cv.isValidKubernetesVersion(k8sVersion) {
		result.Warnings = append(result.Warnings, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.cluster.kubernetes.version",
			Value:   k8sVersion,
			Message: "Kubernetes version format may be invalid",
			Suggestions: []string{
				"Use semantic versioning format (e.g., 1.31.4)",
				"Check Kubernetes release notes for supported versions",
			},
		})
	}

	// Validate node counts
	if config.OpenCenter.Cluster.Kubernetes.MasterCount < 1 {
		result.Errors = append(result.Errors, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.cluster.kubernetes.master_count",
			Value:   config.OpenCenter.Cluster.Kubernetes.MasterCount,
			Message: "master count must be at least 1",
			Suggestions: []string{
				"Set master_count to 1 for development or 3 for production",
				"Use odd numbers (1, 3, 5) for etcd quorum",
			},
		})
	}

	if config.OpenCenter.Cluster.Kubernetes.WorkerCount < 0 {
		result.Errors = append(result.Errors, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.cluster.kubernetes.worker_count",
			Value:   config.OpenCenter.Cluster.Kubernetes.WorkerCount,
			Message: "worker count cannot be negative",
			Suggestions: []string{
				"Set worker_count to 0 or higher",
				"Use at least 2 workers for production workloads",
			},
		})
	}

	if config.OpenCenter.Cluster.Kubernetes.WorkerCountWindows < 0 {
		result.Errors = append(result.Errors, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.cluster.kubernetes.worker_count_windows",
			Value:   config.OpenCenter.Cluster.Kubernetes.WorkerCountWindows,
			Message: "Windows worker count cannot be negative",
			Suggestions: []string{
				"Set worker_count_windows to 0 if not using Windows nodes",
				"Set to positive number if Windows workloads are needed",
			},
		})
	}
}

// validateSemanticsWithResult validates the semantic correctness of a configuration.
func (cv *ClusterConfigValidator) validateSemanticsWithResult(ctx context.Context, config *Config, result *ConfigValidationResult) {
	// Validate OpenTofu configuration
	if config.OpenTofu.Enabled {
		if config.OpenTofu.Path == "" {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "opentofu.path",
				Message: "OpenTofu path must be set when enabled",
				Suggestions: []string{
					"Set opentofu.path to the directory containing Terraform files",
					"Use 'opentofu' for default path",
				},
			})
		}

		// Validate backend configuration
		backendType := strings.ToLower(strings.TrimSpace(config.OpenTofu.Backend.Type))
		if backendType == "" {
			backendType = "local"
		}

		switch backendType {
		case "local":
			if config.OpenTofu.Backend.Local.Path == "" {
				result.Errors = append(result.Errors, &ConfigValidationError{
					Type:    "validation",
					Field:   "opentofu.backend.local.path",
					Message: "local backend path must be set",
					Suggestions: []string{
						"Set opentofu.backend.local.path to state file location",
						"Use 'terraform.tfstate' for default",
					},
				})
			}
		case "s3":
			s3 := config.OpenTofu.Backend.S3
			if s3.Bucket == "" || s3.Key == "" || s3.Region == "" {
				result.Errors = append(result.Errors, &ConfigValidationError{
					Type:    "validation",
					Field:   "opentofu.backend.s3",
					Message: "S3 backend requires bucket, key, and region",
					Suggestions: []string{
						"Set opentofu.backend.s3.bucket to S3 bucket name",
						"Set opentofu.backend.s3.key to state file path",
						"Set opentofu.backend.s3.region to AWS region",
					},
				})
			}

			// Validate S3 bucket name is lowercase
			if s3.Bucket != "" && s3.Bucket != strings.ToLower(s3.Bucket) {
				result.Errors = append(result.Errors, &ConfigValidationError{
					Type:    "validation",
					Field:   "opentofu.backend.s3.bucket",
					Value:   s3.Bucket,
					Message: "S3 bucket name must be lowercase",
					Suggestions: []string{
						fmt.Sprintf("Change bucket name to: %s", strings.ToLower(s3.Bucket)),
						"S3 bucket names must be lowercase per AWS requirements",
					},
				})
			}

			// Validate AWS credentials for S3 backend
			if strings.TrimSpace(config.OpenCenter.Cluster.AWSAccessKey) == "" ||
				strings.TrimSpace(config.OpenCenter.Cluster.AWSSecretAccessKey) == "" {
				result.Errors = append(result.Errors, &ConfigValidationError{
					Type:    "validation",
					Field:   "opencenter.cluster.aws_access_key",
					Message: "AWS credentials required for S3 backend",
					Suggestions: []string{
						"Set opencenter.cluster.aws_access_key",
						"Set opencenter.cluster.aws_secret_access_key",
						"Use SOPS to encrypt sensitive credentials",
					},
				})
			}
		default:
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "opentofu.backend.type",
				Value:   backendType,
				Message: "backend type must be 'local' or 's3'",
				Suggestions: []string{
					"Use 'local' for local state storage",
					"Use 's3' for remote state in AWS S3",
				},
			})
		}
	}

	// Validate Windows workers configuration
	if config.OpenCenter.Cluster.Kubernetes.WorkerCountWindows == 0 {
		if config.OpenCenter.Cluster.Kubernetes.WindowsWorkers.Enabled {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "opencenter.cluster.kubernetes.windows_workers.enabled",
				Message: "Windows workers enabled but worker_count_windows is 0",
				Suggestions: []string{
					"Set worker_count_windows to positive number",
					"Or disable Windows workers by setting enabled to false",
				},
			})
		}
	}

	// Validate SSH authorized keys
	if len(config.OpenCenter.Cluster.SSHAuthorizedKeys) == 0 {
		result.Warnings = append(result.Warnings, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.cluster.ssh_authorized_keys",
			Message: "no SSH authorized keys configured",
			Suggestions: []string{
				"Add SSH public keys for cluster access",
				"Use ssh-keygen to generate key pairs if needed",
			},
		})
	}

	// Validate service-specific configuration and secrets
	for serviceName, svc := range config.OpenCenter.Services {
		cv.validateService(serviceName, svc, config.Secrets, result)
	}

	// Validate managed service configuration and secrets
	for serviceName, svc := range config.OpenCenter.ManagedService {
		cv.validateManagedService(serviceName, svc, config.Secrets, result)
	}

	// Validate service secrets (consolidated validation)
	cv.validateServiceSecrets(config, result)
}

// validateNetworkingWithResult validates network plugin configuration.
func (cv *ClusterConfigValidator) validateNetworkingWithResult(ctx context.Context, config *Config, result *ConfigValidationResult) {
	networkPlugins := []struct {
		name    string
		enabled bool
	}{
		{"Calico", config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled},
		{"Cilium", config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled},
		{"Kube-OVN", config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled},
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
		result.Errors = append(result.Errors, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.cluster.kubernetes.network_plugin",
			Message: "at least one network plugin must be enabled",
			Suggestions: []string{
				"Enable Calico for most use cases",
				"Enable Cilium for advanced networking features",
				"Enable Kube-OVN for overlay networking",
			},
		})
	} else if enabledCount > 1 {
		result.Errors = append(result.Errors, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.cluster.kubernetes.network_plugin",
			Value:   enabledPlugins,
			Message: fmt.Sprintf("only one network plugin can be enabled, found: %s", strings.Join(enabledPlugins, ", ")),
			Suggestions: []string{
				"Choose one network plugin and disable others",
				"Calico is recommended for most deployments",
				"Cilium provides advanced features like eBPF",
			},
		})
	}

	// Validate Calico specific configuration
	if config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled {
		calico := config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico
		if calico.CNIIface == "" {
			result.Warnings = append(result.Warnings, &ConfigValidationError{
				Type:    "validation",
				Field:   "opencenter.cluster.kubernetes.network_plugin.calico.cni_iface",
				Message: "CNI interface not specified for Calico",
				Suggestions: []string{
					"Set cni_iface to the network interface name (e.g., 'enp3s0')",
					"Use 'interface' for automatic detection",
				},
			})
		}
	}

	// Validate Kube-OVN and Cilium integration
	if config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled &&
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.CiliumIntegration &&
		!config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled {
		result.Warnings = append(result.Warnings, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.cluster.kubernetes.network_plugin.kube-ovn.cilium_integration",
			Message: "Cilium integration enabled but Cilium is not enabled",
			Suggestions: []string{
				"Enable Cilium if using Cilium integration",
				"Or disable cilium_integration in Kube-OVN config",
			},
		})
	}

	// Validate subnet configurations
	if config.OpenCenter.Cluster.Kubernetes.SubnetPods == "" {
		result.Warnings = append(result.Warnings, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.cluster.kubernetes.subnet_pods",
			Message: "pod subnet not specified",
			Suggestions: []string{
				"Set subnet_pods to a CIDR range (e.g., '10.42.0.0/16')",
				"Ensure it doesn't conflict with node or service subnets",
			},
		})
	}

	if config.OpenCenter.Cluster.Kubernetes.SubnetServices == "" {
		result.Warnings = append(result.Warnings, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.cluster.kubernetes.subnet_services",
			Message: "service subnet not specified",
			Suggestions: []string{
				"Set subnet_services to a CIDR range (e.g., '10.43.0.0/16')",
				"Ensure it doesn't conflict with node or pod subnets",
			},
		})
	}
}

// validateCloudProviderWithResult validates cloud provider specific configuration.
func (cv *ClusterConfigValidator) validateCloudProviderWithResult(ctx context.Context, config *Config, result *ConfigValidationResult) {
	provider := config.OpenCenter.Infrastructure.Provider
	if provider == "" {
		result.Errors = append(result.Errors, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.infrastructure.provider",
			Message: "cloud provider must be specified",
			Suggestions: []string{
				"Set provider to 'openstack' for OpenStack clouds",
				"Set provider to 'aws' for Amazon Web Services",
				"Set provider to 'kind' for local development",
			},
		})
		return
	}

	switch provider {
	case "openstack":
		cv.validateOpenStackConfig(config, result)
	case "aws":
		cv.validateAWSConfig(config, result)
	case "kind":
		// Kind has minimal configuration requirements
		break
	default:
		result.Warnings = append(result.Warnings, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.infrastructure.provider",
			Value:   provider,
			Message: fmt.Sprintf("unknown cloud provider: %s", provider),
			Suggestions: []string{
				"Use 'openstack' for OpenStack deployments",
				"Use 'aws' for AWS deployments",
				"Use 'kind' for local development",
			},
		})
	}
}

// validateOpenStackConfig validates OpenStack-specific configuration.
func (cv *ClusterConfigValidator) validateOpenStackConfig(config *Config, result *ConfigValidationResult) {
	os := config.OpenCenter.Infrastructure.Cloud.OpenStack

	if os.AuthURL == "" {
		result.Errors = append(result.Errors, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.infrastructure.cloud.openstack.auth_url",
			Message: "OpenStack auth URL must be specified",
			Suggestions: []string{
				"Set auth_url to your OpenStack Keystone endpoint",
				"Example: https://keystone.api.example.com/v3/",
			},
		})
	}

	if os.Region == "" {
		result.Warnings = append(result.Warnings, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.infrastructure.cloud.openstack.region",
			Message: "OpenStack region not specified",
			Suggestions: []string{
				"Set region to your OpenStack region name",
				"Check with your OpenStack administrator for available regions",
			},
		})
	}

	if os.TenantName == "" {
		result.Errors = append(result.Errors, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.infrastructure.cloud.openstack.tenant_name",
			Message: "OpenStack tenant name must be specified",
			Suggestions: []string{
				"Set tenant_name to your OpenStack project/tenant",
				"Use project ID or project name",
			},
		})
	}

	// Validate credentials
	if os.ApplicationCredentialID == "" || os.ApplicationCredentialSecret == "" {
		result.Warnings = append(result.Warnings, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.infrastructure.cloud.openstack.application_credential_id",
			Message: "OpenStack application credentials not configured",
			Suggestions: []string{
				"Set application_credential_id and application_credential_secret",
				"Use SOPS to encrypt sensitive credentials",
				"Create application credentials in OpenStack dashboard",
			},
		})
	}
}

// validateAWSConfig validates AWS-specific configuration.
func (cv *ClusterConfigValidator) validateAWSConfig(config *Config, result *ConfigValidationResult) {
	aws := config.OpenCenter.Infrastructure.Cloud.AWS

	if aws.Region == "" {
		result.Errors = append(result.Errors, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.infrastructure.cloud.aws.region",
			Message: "AWS region must be specified",
			Suggestions: []string{
				"Set region to an AWS region (e.g., 'us-east-1')",
				"Check AWS documentation for available regions",
			},
		})
	}

	if aws.VPCID == "" {
		result.Warnings = append(result.Warnings, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.infrastructure.cloud.aws.vpc_id",
			Message: "AWS VPC ID not specified",
			Suggestions: []string{
				"Set vpc_id to use existing VPC",
				"Leave empty to create new VPC",
			},
		})
	}

	// Validate subnet configuration
	if len(aws.PrivateSubnets) == 0 && len(aws.PublicSubnets) == 0 {
		result.Warnings = append(result.Warnings, &ConfigValidationError{
			Type:    "validation",
			Field:   "opencenter.infrastructure.cloud.aws.private_subnets",
			Message: "no subnets configured for AWS",
			Suggestions: []string{
				"Configure private_subnets for internal resources",
				"Configure public_subnets for internet-facing resources",
			},
		})
	}
}

// isValidKubernetesVersion checks if a Kubernetes version string is valid.
func (cv *ClusterConfigValidator) isValidKubernetesVersion(version string) bool {
	// Basic semantic version check (major.minor.patch)
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}

	// Check if all parts are numeric
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
		}
	}

	return true
}

// isValidEmail checks if an email address is valid.
func (cv *ClusterConfigValidator) isValidEmail(email string) bool {
	if email == "" {
		return false
	}

	// Basic email validation using regex
	// This pattern checks for: local-part@domain
	// where local-part can contain alphanumeric, dots, hyphens, underscores
	// and domain must have at least one dot
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// isValidDomain checks if a domain name is valid.
func (cv *ClusterConfigValidator) isValidDomain(domain string) bool {
	if domain == "" {
		return false
	}

	// Domain validation using regex
	// Allows alphanumeric characters, hyphens, and dots
	// Must have at least one dot and valid TLD
	domainRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
	return domainRegex.MatchString(domain)
}

// validateService validates service-specific configuration (non-secret fields).
func (cv *ClusterConfigValidator) validateService(serviceName string, svc ServiceCfg, secrets Secrets, result *ConfigValidationResult) {
	if !svc.Enabled {
		return
	}

	// Validate service-specific required configuration fields (non-secrets)
	switch serviceName {
	case "loki":
		// Check required configuration
		if svc.SwiftAuthURL == "" {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "opencenter.services.loki.swift_auth_url",
				Message: "Swift auth URL is required when Loki is enabled",
				Suggestions: []string{
					"Set swift_auth_url to your Swift/OpenStack Keystone endpoint",
				},
			})
		}
		if svc.SwiftUsername == "" {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "opencenter.services.loki.swift_username",
				Message: "Swift username is required when Loki is enabled",
				Suggestions: []string{
					"Set swift_username for Swift storage authentication",
				},
			})
		}
		if svc.SwiftProjectName == "" {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "opencenter.services.loki.swift_project_name",
				Message: "Swift project name is required when Loki is enabled",
				Suggestions: []string{
					"Set swift_project_name for Swift storage authentication",
				},
			})
		}
	}
}

// validateManagedService validates managed service-specific configuration and secrets.
func (cv *ClusterConfigValidator) validateManagedService(serviceName string, svc ServiceCfg, secrets Secrets, result *ConfigValidationResult) {
	if !svc.Enabled {
		return
	}

	// Validate managed service-specific required fields and secrets
	switch serviceName {
	case "alert-proxy":
		// Check required secrets
		if secrets.AlertProxy.CoreDeviceId == "" {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "secrets.alert_proxy.core_device_id",
				Message: "Alert proxy core device ID is required when alert-proxy is enabled",
				Suggestions: []string{
					"Set secrets.alert_proxy.core_device_id for alert proxy configuration",
					"Use SOPS to encrypt sensitive credentials",
				},
			})
		}
		if secrets.AlertProxy.AccountServiceToken == "" {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "secrets.alert_proxy.account_service_token",
				Message: "Alert proxy account service token is required when alert-proxy is enabled",
				Suggestions: []string{
					"Set secrets.alert_proxy.account_service_token for alert proxy configuration",
					"Use SOPS to encrypt sensitive credentials",
				},
			})
		}
		if secrets.AlertProxy.CoreAccountNumber == "" {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "secrets.alert_proxy.core_account_number",
				Message: "Alert proxy core account number is required when alert-proxy is enabled",
				Suggestions: []string{
					"Set secrets.alert_proxy.core_account_number for alert proxy configuration",
					"Use SOPS to encrypt sensitive credentials",
				},
			})
		}

		// Check required configuration
		if svc.AlertManagerBaseUrl == "" {
			result.Warnings = append(result.Warnings, &ConfigValidationError{
				Type:    "validation",
				Field:   "opencenter.managed-service.alert-proxy.alert_manager_base_url",
				Message: "Alert manager base URL should be set when alert-proxy is enabled",
				Suggestions: []string{
					"Set alert_manager_base_url to your AlertManager endpoint",
					"Example: http://observability-kube-prometh-alertmanager.observability.svc.cluster.local:9093/api/v2/alerts",
				},
			})
		}
	}
}

// validateServiceSecrets validates service-specific secrets configuration.
// This function checks that required secrets are present when corresponding services are enabled.
func (cv *ClusterConfigValidator) validateServiceSecrets(config *Config, result *ConfigValidationResult) {
	// Validate cert-manager secrets
	if svc, exists := config.OpenCenter.Services["cert-manager"]; exists && svc.Enabled {
		if config.Secrets.CertManager.AWSAccessKey == "" {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "secrets.cert_manager.aws_access_key",
				Message: "cert-manager requires aws_access_key",
				Suggestions: []string{
					"Set secrets.cert_manager.aws_access_key for Route53 DNS validation",
					"Use SOPS to encrypt sensitive credentials",
				},
			})
		}
		if config.Secrets.CertManager.AWSSecretAccessKey == "" {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "secrets.cert_manager.aws_secret_access_key",
				Message: "cert-manager requires aws_secret_access_key",
				Suggestions: []string{
					"Set secrets.cert_manager.aws_secret_access_key for Route53 DNS validation",
					"Use SOPS to encrypt sensitive credentials",
				},
			})
		}
	}

	// Validate loki secrets
	if svc, exists := config.OpenCenter.Services["loki"]; exists && svc.Enabled {
		if config.Secrets.Loki.SwiftPassword == "" {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "secrets.loki.swift_password",
				Message: "loki requires swift_password",
				Suggestions: []string{
					"Set secrets.loki.swift_password for Swift storage authentication",
					"Use SOPS to encrypt sensitive credentials",
				},
			})
		}
	}

	// Validate keycloak secrets
	if svc, exists := config.OpenCenter.Services["keycloak"]; exists && svc.Enabled {
		if config.Secrets.Keycloak.ClientSecret == "" {
			result.Warnings = append(result.Warnings, &ConfigValidationError{
				Type:    "validation",
				Field:   "secrets.keycloak.client_secret",
				Message: "keycloak client_secret should be set",
				Suggestions: []string{
					"Set secrets.keycloak.client_secret for OIDC authentication",
					"Use SOPS to encrypt sensitive credentials",
				},
			})
		}
		if config.Secrets.Keycloak.AdminPassword == "" {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "secrets.keycloak.admin_password",
				Message: "keycloak requires admin_password",
				Suggestions: []string{
					"Set secrets.keycloak.admin_password for Keycloak admin user",
					"Use SOPS to encrypt sensitive credentials",
				},
			})
		}
	}

	// Validate headlamp secrets
	if svc, exists := config.OpenCenter.Services["headlamp"]; exists && svc.Enabled {
		if config.Secrets.Headlamp.OIDCClientSecret == "" {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "secrets.headlamp.oidc_client_secret",
				Message: "Headlamp OIDC client secret is required when Headlamp is enabled",
				Suggestions: []string{
					"Set secrets.headlamp.oidc_client_secret for OIDC authentication",
					"Use SOPS to encrypt sensitive credentials",
				},
			})
		}
	}

	// Validate weave-gitops secrets
	if svc, exists := config.OpenCenter.Services["weave-gitops"]; exists && svc.Enabled {
		if config.Secrets.WeaveGitOps.PasswordHash == "" {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "secrets.weave_gitops.password_hash",
				Message: "Weave GitOps password hash is required when Weave GitOps is enabled",
				Suggestions: []string{
					"Set secrets.weave_gitops.password_hash (bcrypt hash)",
					"Use 'htpasswd -nbBC 10 admin <password>' to generate hash",
					"Use SOPS to encrypt sensitive credentials",
				},
			})
		}
	}

	// Validate kube-prometheus-stack (Grafana) secrets
	if svc, exists := config.OpenCenter.Services["kube-prometheus-stack"]; exists && svc.Enabled {
		if config.Secrets.Grafana.AdminPassword == "" {
			result.Errors = append(result.Errors, &ConfigValidationError{
				Type:    "validation",
				Field:   "secrets.grafana.admin_password",
				Message: "Grafana admin password is required when kube-prometheus-stack is enabled",
				Suggestions: []string{
					"Set secrets.grafana.admin_password for Grafana admin user",
					"Use SOPS to encrypt sensitive credentials",
				},
			})
		}
	}
}

// SetAutoRepair enables or disables automatic repair of configuration issues.
func (cv *ClusterConfigValidator) SetAutoRepair(autoRepair bool) {
	cv.autoRepair = autoRepair
}

// IsAutoRepairEnabled returns whether automatic repair is enabled.
func (cv *ClusterConfigValidator) IsAutoRepairEnabled() bool {
	return cv.autoRepair
}

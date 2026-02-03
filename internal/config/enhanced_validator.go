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
	"net/url"
	"reflect"
	"strings"
	"time"

	corevalidation "github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation/validators"
	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
)

// EnhancedConfigValidator provides comprehensive configuration validation with structured error handling.
type EnhancedConfigValidator struct {
	errorHandler          errors.ErrorHandler
	errorWrapper          errors.ErrorWrapper
	autoRepair            bool
	cloudValidators       map[string]CloudProviderValidator
	connectivityValidator *ConnectivityValidator
	validationEngine      *corevalidation.ValidationEngine
}

// CloudProviderValidator defines the interface for cloud provider-specific validation.
type CloudProviderValidator interface {
	ValidateCredentials(ctx context.Context, config *Config) []*errors.StructuredError
	ValidateConfiguration(ctx context.Context, config *Config) []*errors.StructuredError
	ValidateConnectivity(ctx context.Context, config *Config) []*errors.StructuredError
	GetRequiredFields() []string
}

// NewEnhancedConfigValidator creates a new enhanced configuration validator.
func NewEnhancedConfigValidator(autoRepair bool) *EnhancedConfigValidator {
	validator := &EnhancedConfigValidator{
		errorHandler:          errors.NewDefaultErrorHandler(),
		errorWrapper:          errors.NewDefaultErrorWrapper(),
		autoRepair:            autoRepair,
		cloudValidators:       make(map[string]CloudProviderValidator),
		connectivityValidator: NewConnectivityValidator(10 * time.Second),
		validationEngine:      corevalidation.NewValidationEngine(),
	}

	// Register cloud provider validators
	validator.cloudValidators["openstack"] = NewOpenStackValidator()
	validator.cloudValidators["aws"] = NewAWSValidator()
	validator.cloudValidators["vsphere"] = NewVSphereValidator()

	// Register core validators with the validation engine
	validator.validationEngine.MustRegister(validators.NewClusterNameValidator())
	validator.validationEngine.MustRegister(validators.NewNetworkValidator())
	validator.validationEngine.MustRegister(validators.NewConfigValidator())
	validator.validationEngine.MustRegister(validators.NewFileValidator())
	validator.validationEngine.MustRegister(validators.NewSecurityValidator())

	return validator
}

// Validate implements ConfigValidatorInterface.
func (v *EnhancedConfigValidator) Validate(ctx context.Context, config *Config) *ConfigValidationResult {
	result := v.ValidateComprehensive(ctx, config)

	// Convert structured errors to config validation errors
	configResult := &ConfigValidationResult{
		Valid:    result.Valid,
		Errors:   []*ConfigValidationError{},
		Warnings: []*ConfigValidationError{},
	}

	for _, err := range result.Errors {
		configResult.Errors = append(configResult.Errors, &ConfigValidationError{
			Type:        string(err.Type),
			Field:       err.Field,
			Message:     err.Message,
			Suggestions: err.Suggestions,
		})
	}

	for _, warning := range result.Warnings {
		configResult.Warnings = append(configResult.Warnings, &ConfigValidationError{
			Type:        string(warning.Type),
			Field:       warning.Field,
			Message:     warning.Message,
			Suggestions: warning.Suggestions,
		})
	}

	return configResult
}

// ValidateStructure implements ConfigValidatorInterface.
func (v *EnhancedConfigValidator) ValidateStructure(ctx context.Context, config *Config) *ConfigValidationResult {
	aggregator := errors.NewValidationAggregator()
	v.validateBasicStructure(ctx, config, aggregator)

	result := aggregator.ToValidationResult()
	return v.convertToConfigValidationResult(result)
}

// ValidateSemantics implements ConfigValidatorInterface.
func (v *EnhancedConfigValidator) ValidateSemantics(ctx context.Context, config *Config) *ConfigValidationResult {
	aggregator := errors.NewValidationAggregator()
	v.validateCrossFieldDependencies(ctx, config, aggregator)

	result := aggregator.ToValidationResult()
	return v.convertToConfigValidationResult(result)
}

// ValidateNetworking implements ConfigValidatorInterface.
func (v *EnhancedConfigValidator) ValidateNetworking(ctx context.Context, config *Config) *ConfigValidationResult {
	aggregator := errors.NewValidationAggregator()
	v.validateNetworkConfiguration(ctx, config, aggregator)

	result := aggregator.ToValidationResult()
	return v.convertToConfigValidationResult(result)
}

// ValidateCloudProvider implements ConfigValidatorInterface.
func (v *EnhancedConfigValidator) ValidateCloudProvider(ctx context.Context, config *Config) *ConfigValidationResult {
	aggregator := errors.NewValidationAggregator()
	v.validateCloudProviderConfiguration(ctx, config, aggregator)

	result := aggregator.ToValidationResult()
	return v.convertToConfigValidationResult(result)
}

// convertToConfigValidationResult converts ValidationResult to ConfigValidationResult.
func (v *EnhancedConfigValidator) convertToConfigValidationResult(result *errors.ValidationResult) *ConfigValidationResult {
	configResult := &ConfigValidationResult{
		Valid:    result.Valid,
		Errors:   []*ConfigValidationError{},
		Warnings: []*ConfigValidationError{},
	}

	for _, err := range result.Errors {
		configResult.Errors = append(configResult.Errors, &ConfigValidationError{
			Type:        string(err.Type),
			Field:       err.Field,
			Message:     err.Message,
			Suggestions: err.Suggestions,
		})
	}

	for _, warning := range result.Warnings {
		configResult.Warnings = append(configResult.Warnings, &ConfigValidationError{
			Type:        string(warning.Type),
			Field:       warning.Field,
			Message:     warning.Message,
			Suggestions: warning.Suggestions,
		})
	}

	return configResult
}

// ValidateComprehensive performs comprehensive validation with structured error handling.
func (v *EnhancedConfigValidator) ValidateComprehensive(ctx context.Context, config *Config) *errors.ValidationResult {
	aggregator := errors.NewValidationAggregator()

	// Validate basic structure
	v.validateBasicStructure(ctx, config, aggregator)

	// Validate cross-field dependencies
	v.validateCrossFieldDependencies(ctx, config, aggregator)

	// Validate network configuration
	v.validateNetworkConfiguration(ctx, config, aggregator)

	// Validate cloud provider configuration
	v.validateCloudProviderConfiguration(ctx, config, aggregator)

	// Validate security configuration
	v.validateSecurityConfiguration(ctx, config, aggregator)

	// Validate credential format and security
	v.validateCredentialSecurity(ctx, config, aggregator)

	// Validate cloud provider connectivity (as warnings)
	v.validateConnectivity(ctx, config, aggregator)

	return aggregator.ToValidationResult()
}

// validateBasicStructure validates the basic structure and required fields.
func (v *EnhancedConfigValidator) validateBasicStructure(ctx context.Context, config *Config, aggregator *errors.ValidationAggregator) {
	if config == nil {
		aggregator.AddError(errors.CreateConfigError("config", "configuration cannot be nil", nil))
		return
	}

	// Use ValidationEngine for cluster name validation
	if config.ClusterName() != "" {
		result, err := v.validationEngine.Validate(ctx, "cluster-name", config.ClusterName())
		if err != nil {
			aggregator.AddError(errors.CreateValidationError(
				"opencenter.cluster.cluster_name",
				fmt.Sprintf("cluster name validation failed: %v", err),
				"Ensure cluster name is valid",
			))
		} else if result != nil && !result.Valid {
			for _, issue := range result.Errors {
				aggregator.AddError(errors.CreateValidationError(
					"opencenter.cluster.cluster_name",
					issue.Message,
					issue.Suggestions...,
				))
			}
		}
	} else {
		aggregator.AddError(errors.CreateValidationError(
			"opencenter.cluster.cluster_name",
			"cluster name is required",
			"Set opencenter.cluster.cluster_name to a valid cluster name",
			"Use alphanumeric characters, hyphens, and underscores only",
		))
	}

	// Validate GitOps directory
	if config.GitOps().GitDir == "" {
		aggregator.AddError(errors.CreateValidationError(
			"opencenter.gitops.git_dir",
			"GitOps directory is required",
			"Set opencenter.gitops.git_dir to a valid directory path",
			"Use a path where GitOps repository will be created",
		))
	}

	// Validate Kubernetes version
	if k8sVersion := config.OpenCenter.Cluster.Kubernetes.Version; k8sVersion != "" {
		if !v.isValidKubernetesVersion(k8sVersion) {
			aggregator.AddWarning(errors.CreateValidationError(
				"opencenter.cluster.kubernetes.version",
				"Kubernetes version format may be invalid",
				"Use semantic versioning format (e.g., 1.31.4)",
				"Check Kubernetes release notes for supported versions",
			))
		}
	}

	// Validate node counts
	v.validateNodeCounts(config, aggregator)
}

// validateCrossFieldDependencies validates dependencies between configuration fields.
func (v *EnhancedConfigValidator) validateCrossFieldDependencies(ctx context.Context, config *Config, aggregator *errors.ValidationAggregator) {
	if config == nil {
		return
	}

	// Validate admin email using ValidationEngine
	if config.OpenCenter.Cluster.AdminEmail != "" {
		result, err := v.validationEngine.Validate(ctx, "config", map[string]interface{}{
			"type":  "email",
			"value": config.OpenCenter.Cluster.AdminEmail,
		})
		if err != nil {
			aggregator.AddError(errors.CreateValidationError(
				"opencenter.cluster.admin_email",
				fmt.Sprintf("email validation failed: %v", err),
				"Ensure email is in valid format",
			))
		} else if result != nil && !result.Valid {
			for _, issue := range result.Errors {
				aggregator.AddError(errors.CreateValidationError(
					"opencenter.cluster.admin_email",
					issue.Message,
					issue.Suggestions...,
				))
			}
		}
	}

	// Validate cluster FQDN using ValidationEngine
	if config.OpenCenter.Cluster.ClusterFQDN != "" {
		result, err := v.validationEngine.Validate(ctx, "config", map[string]interface{}{
			"type":  "fqdn",
			"value": config.OpenCenter.Cluster.ClusterFQDN,
		})
		if err != nil {
			aggregator.AddError(errors.CreateValidationError(
				"opencenter.cluster.cluster_fqdn",
				fmt.Sprintf("FQDN validation failed: %v", err),
				"Ensure FQDN is in valid format",
			))
		} else if result != nil && !result.Valid {
			for _, issue := range result.Errors {
				aggregator.AddError(errors.CreateValidationError(
					"opencenter.cluster.cluster_fqdn",
					issue.Message,
					issue.Suggestions...,
				))
			}
		}
	}

	// Validate Windows workers configuration
	if config.OpenCenter.Cluster.Kubernetes.WorkerCountWindows == 0 {
		if config.OpenCenter.Cluster.Kubernetes.WindowsWorkers.Enabled {
			aggregator.AddError(errors.CreateValidationError(
				"opencenter.cluster.kubernetes.windows_workers.enabled",
				"Windows workers enabled but worker_count_windows is 0",
				"Set worker_count_windows to positive number",
				"Or disable Windows workers by setting enabled to false",
			))
		}
	}

	// Validate OpenTofu backend configuration
	if config.OpenTofu.Enabled {
		v.validateOpenTofuConfiguration(config, aggregator)
	}

	// Validate SSH keys
	if len(config.OpenCenter.Cluster.SSHAuthorizedKeys) == 0 {
		aggregator.AddWarning(errors.CreateValidationError(
			"opencenter.cluster.ssh_authorized_keys",
			"no SSH authorized keys configured",
			"Add SSH public keys for cluster access",
			"Use ssh-keygen to generate key pairs if needed",
		))
	}

	// Validate service-specific secrets
	v.validateServiceSecrets(config, aggregator)

	// Validate Loki storage configuration
	v.validateLokiStorageConfiguration(config, aggregator)

	// Validate VRRP configuration
	v.validateVRRPConfiguration(config, aggregator)
}

// validateNetworkConfiguration validates network plugin and subnet configuration.
func (v *EnhancedConfigValidator) validateNetworkConfiguration(ctx context.Context, config *Config, aggregator *errors.ValidationAggregator) {
	if config == nil {
		return
	}

	// Use ValidationEngine for network validation
	networkConfig := &validators.NetworkConfig{
		SubnetNodes:    config.OpenCenter.Cluster.Networking.SubnetNodes,
		SubnetPods:     config.OpenCenter.Cluster.Kubernetes.SubnetPods,
		SubnetServices: config.OpenCenter.Cluster.Kubernetes.SubnetServices,
		DNSNameservers: config.OpenCenter.Cluster.Networking.DNSNameservers,
		VRRPEnabled:    config.OpenCenter.Cluster.Kubernetes.Networking.VRRPEnabled,
		VRRPIP:         config.OpenCenter.Cluster.Kubernetes.Networking.VRRPIP,
	}

	result, err := v.validationEngine.Validate(ctx, "network", networkConfig)
	if err != nil {
		aggregator.AddError(errors.CreateValidationError(
			"opencenter.cluster.networking",
			fmt.Sprintf("network validation failed: %v", err),
			"Check network configuration",
		))
	} else if result != nil && !result.Valid {
		for _, issue := range result.Errors {
			aggregator.AddError(errors.CreateValidationError(
				fmt.Sprintf("opencenter.cluster.networking.%s", issue.Field),
				issue.Message,
				issue.Suggestions...,
			))
		}
		for _, warning := range result.Warnings {
			aggregator.AddWarning(errors.CreateValidationError(
				fmt.Sprintf("opencenter.cluster.networking.%s", warning.Field),
				warning.Message,
				warning.Suggestions...,
			))
		}
	}

	// Validate network plugin mutual exclusivity
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
		aggregator.AddError(errors.CreateValidationError(
			"opencenter.cluster.kubernetes.network_plugin",
			"at least one network plugin must be enabled",
			"Enable Calico for most use cases",
			"Enable Cilium for advanced networking features",
			"Enable Kube-OVN for overlay networking",
		))
	} else if enabledCount > 1 {
		aggregator.AddError(errors.CreateValidationError(
			"opencenter.cluster.kubernetes.network_plugin",
			fmt.Sprintf("only one network plugin can be enabled, found: %s", strings.Join(enabledPlugins, ", ")),
			"Choose one network plugin and disable others",
			"Calico is recommended for most deployments",
			"Cilium provides advanced features like eBPF",
		))
	}

	// Validate network plugin specific configuration
	v.validateNetworkPluginConfiguration(config, aggregator)
}

// validateCloudProviderConfiguration validates cloud provider specific configuration.
func (v *EnhancedConfigValidator) validateCloudProviderConfiguration(ctx context.Context, config *Config, aggregator *errors.ValidationAggregator) {
	if config == nil {
		return
	}
	provider := config.OpenCenter.Infrastructure.Provider
	if provider == "" {
		aggregator.AddError(errors.CreateValidationError(
			"opencenter.infrastructure.provider",
			"cloud provider must be specified",
			"Set provider to 'openstack' for OpenStack clouds",
			"Set provider to 'aws' for Amazon Web Services",
			"Set provider to 'kind' for local development",
		))
		return
	}

	// Use cloud provider specific validator if available
	if validator, exists := v.cloudValidators[provider]; exists {
		// Validate credentials
		credentialErrors := validator.ValidateCredentials(ctx, config)
		for _, err := range credentialErrors {
			aggregator.AddError(err)
		}

		// Validate configuration
		configErrors := validator.ValidateConfiguration(ctx, config)
		for _, err := range configErrors {
			aggregator.AddError(err)
		}

		// Validate connectivity (as warnings since they require network access)
		connectivityErrors := validator.ValidateConnectivity(ctx, config)
		for _, err := range connectivityErrors {
			aggregator.AddWarning(err)
		}
	} else {
		aggregator.AddWarning(errors.CreateValidationError(
			"opencenter.infrastructure.provider",
			fmt.Sprintf("unknown cloud provider: %s", provider),
			"Use 'openstack' for OpenStack deployments",
			"Use 'aws' for AWS deployments",
			"Use 'kind' for local development",
		))
	}
}

// validateSecurityConfiguration validates security-related configuration.
func (v *EnhancedConfigValidator) validateSecurityConfiguration(ctx context.Context, config *Config, aggregator *errors.ValidationAggregator) {
	if config == nil {
		return
	}
	// Validate SSH key format
	for i, key := range config.OpenCenter.Cluster.SSHAuthorizedKeys {
		if !v.isValidSSHPublicKey(key) {
			aggregator.AddError(errors.CreateValidationError(
				fmt.Sprintf("opencenter.cluster.ssh_authorized_keys[%d]", i),
				"invalid SSH public key format",
				"Ensure SSH key is in proper format (ssh-rsa, ssh-ed25519, etc.)",
				"Use ssh-keygen to generate valid key pairs",
			))
		}
	}

	// Validate SOPS configuration if present
	// This would be expanded based on SOPS configuration structure
}

// Helper validation methods

func (v *EnhancedConfigValidator) isValidKubernetesVersion(version string) bool {
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

func (v *EnhancedConfigValidator) validateNodeCounts(config *Config, aggregator *errors.ValidationAggregator) {
	if config.OpenCenter.Cluster.Kubernetes.MasterCount < 1 {
		aggregator.AddError(errors.CreateValidationError(
			"opencenter.cluster.kubernetes.master_count",
			"master count must be at least 1",
			"Set master_count to 1 for development or 3 for production",
			"Use odd numbers (1, 3, 5) for etcd quorum",
		))
	}

	if config.OpenCenter.Cluster.Kubernetes.WorkerCount < 0 {
		aggregator.AddError(errors.CreateValidationError(
			"opencenter.cluster.kubernetes.worker_count",
			"worker count cannot be negative",
			"Set worker_count to 0 or higher",
			"Use at least 2 workers for production workloads",
		))
	}

	if config.OpenCenter.Cluster.Kubernetes.WorkerCountWindows < 0 {
		aggregator.AddError(errors.CreateValidationError(
			"opencenter.cluster.kubernetes.worker_count_windows",
			"Windows worker count cannot be negative",
			"Set worker_count_windows to 0 if not using Windows nodes",
			"Set to positive number if Windows workloads are needed",
		))
	}
}

func (v *EnhancedConfigValidator) validateOpenTofuConfiguration(config *Config, aggregator *errors.ValidationAggregator) {
	if config.OpenTofu.Path == "" {
		aggregator.AddError(errors.CreateValidationError(
			"opentofu.path",
			"OpenTofu path must be set when enabled",
			"Set opentofu.path to the directory containing Terraform files",
			"Use 'opentofu' for default path",
		))
	}

	// Validate backend configuration
	backendType := strings.ToLower(strings.TrimSpace(config.OpenTofu.Backend.Type))
	if backendType == "" {
		backendType = "local"
	}

	switch backendType {
	case "local":
		if config.OpenTofu.Backend.Local.Path == "" {
			aggregator.AddError(errors.CreateValidationError(
				"opentofu.backend.local.path",
				"local backend path must be set",
				"Set opentofu.backend.local.path to state file location",
				"Use 'terraform.tfstate' for default",
			))
		}
	case "s3", "aws":
		v.validateS3BackendConfiguration(config, aggregator)
	default:
		aggregator.AddError(errors.CreateValidationError(
			"opentofu.backend.type",
			fmt.Sprintf("backend type must be 'local', 's3', or 'aws', got '%s'", backendType),
			"Use 'local' for local state storage",
			"Use 's3' or 'aws' for remote state in AWS S3",
		))
	}
}

func (v *EnhancedConfigValidator) validateS3BackendConfiguration(config *Config, aggregator *errors.ValidationAggregator) {
	s3 := config.OpenTofu.Backend.S3
	if s3.Bucket == "" || s3.Key == "" || s3.Region == "" {
		aggregator.AddError(errors.CreateValidationError(
			"opentofu.backend.s3",
			"S3 backend requires bucket, key, and region",
			"Set opentofu.backend.s3.bucket to S3 bucket name",
			"Set opentofu.backend.s3.key to state file path",
			"Set opentofu.backend.s3.region to AWS region",
		))
	}

	// Validate AWS credentials for S3 backend - check actual fields without fallback
	// During validation, we require explicit credentials to be set
	legacyAccessKey := strings.TrimSpace(config.OpenCenter.Cluster.AWSAccessKey)
	legacySecretKey := strings.TrimSpace(config.OpenCenter.Cluster.AWSSecretAccessKey)
	infraAccessKey := strings.TrimSpace(config.Secrets.Global.AWS.Infrastructure.AccessKey)
	infraSecretKey := strings.TrimSpace(config.Secrets.Global.AWS.Infrastructure.SecretAccessKey)

	hasLegacyCredentials := legacyAccessKey != "" && legacySecretKey != ""
	hasInfraCredentials := infraAccessKey != "" && infraSecretKey != ""

	if !hasLegacyCredentials && !hasInfraCredentials {
		aggregator.AddError(errors.CreateCredentialError(
			"AWS",
			"opencenter.cluster.aws_access_key or secrets.global.aws.infrastructure.access_key",
			"AWS credentials required for S3 backend: either set opencenter.cluster.aws_access_key/aws_secret_access_key or secrets.global.aws.infrastructure.access_key/secret_access_key",
			nil,
		))
	}
}

func (v *EnhancedConfigValidator) validateNetworkPluginConfiguration(config *Config, aggregator *errors.ValidationAggregator) {
	// Validate Calico specific configuration
	if config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled {
		calico := config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico
		if calico.CNIIface == "" {
			aggregator.AddWarning(errors.CreateValidationError(
				"opencenter.cluster.kubernetes.network_plugin.calico.cni_iface",
				"CNI interface not specified for Calico",
				"Set cni_iface to the network interface name (e.g., 'enp3s0')",
				"Use 'interface' for automatic detection",
			))
		}
	}

	// Validate Kube-OVN and Cilium integration
	if config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled &&
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.CiliumIntegration &&
		!config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled {
		aggregator.AddWarning(errors.CreateValidationError(
			"opencenter.cluster.kubernetes.network_plugin.kube-ovn.cilium_integration",
			"Cilium integration enabled but Cilium is not enabled",
			"Enable Cilium if using Cilium integration",
			"Or disable cilium_integration in Kube-OVN config",
		))
	}
}

func (v *EnhancedConfigValidator) isValidSSHPublicKey(key string) bool {
	// Basic SSH public key format validation
	parts := strings.Fields(key)
	if len(parts) < 2 {
		return false
	}

	// Check for valid key types
	validTypes := []string{"ssh-rsa", "ssh-dss", "ssh-ed25519", "ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521"}
	keyType := parts[0]

	for _, validType := range validTypes {
		if keyType == validType {
			return true
		}
	}

	return false
}

func (v *EnhancedConfigValidator) isValidURL(rawURL string) bool {
	_, err := url.Parse(rawURL)
	return err == nil
}

// validateCredentialSecurity validates credential format and security.
func (v *EnhancedConfigValidator) validateCredentialSecurity(ctx context.Context, config *Config, aggregator *errors.ValidationAggregator) {
	if config == nil {
		return
	}
	// Validate credential format
	formatErrors := v.connectivityValidator.ValidateCredentialFormat(ctx, config)
	for _, err := range formatErrors {
		aggregator.AddError(err)
	}

	// Validate credential security (as warnings)
	securityErrors := v.connectivityValidator.ValidateCredentialSecurity(ctx, config)
	for _, err := range securityErrors {
		aggregator.AddWarning(err)
	}
}

// validateConnectivity validates cloud provider connectivity.
func (v *EnhancedConfigValidator) validateConnectivity(ctx context.Context, config *Config, aggregator *errors.ValidationAggregator) {
	if config == nil {
		return
	}
	// Validate connectivity (as warnings since they require network access)
	connectivityErrors := v.connectivityValidator.ValidateCloudProviderConnectivity(ctx, config)
	for _, err := range connectivityErrors {
		aggregator.AddWarning(err)
	}
}

// ValidatePreFlight performs pre-flight validation including connectivity checks.
func (v *EnhancedConfigValidator) ValidatePreFlight(ctx context.Context, config *Config) *errors.ValidationResult {
	aggregator := errors.NewValidationAggregator()

	// Perform all comprehensive validation
	result := v.ValidateComprehensive(ctx, config)

	// Add all errors and warnings from comprehensive validation
	for _, err := range result.Errors {
		aggregator.AddError(err)
	}
	for _, warning := range result.Warnings {
		aggregator.AddWarning(warning)
	}

	// Add additional pre-flight specific checks
	v.validatePreFlightRequirements(ctx, config, aggregator)

	return aggregator.ToValidationResult()
}

// validatePreFlightRequirements validates pre-flight specific requirements.
func (v *EnhancedConfigValidator) validatePreFlightRequirements(ctx context.Context, config *Config, aggregator *errors.ValidationAggregator) {
	// Validate that required tools are available (this would be expanded)
	provider := config.OpenCenter.Infrastructure.Provider

	switch provider {
	case "openstack":
		// Check for OpenStack CLI tools (as warnings)
		aggregator.AddWarning(errors.CreateValidationError(
			"tools.openstack",
			"OpenStack CLI tools should be installed for full functionality",
			"Install python-openstackclient package",
			"Run 'pip install python-openstackclient'",
		))
	case "aws":
		// Check for AWS CLI tools (as warnings)
		aggregator.AddWarning(errors.CreateValidationError(
			"tools.aws",
			"AWS CLI tools should be installed for full functionality",
			"Install AWS CLI v2",
			"Follow AWS CLI installation guide",
		))
	}

	// Validate OpenTofu/Terraform availability
	if config.OpenTofu.Enabled {
		aggregator.AddWarning(errors.CreateValidationError(
			"tools.opentofu",
			"OpenTofu should be installed when enabled",
			"Install OpenTofu binary",
			"Download from https://opentofu.org/",
		))
	}
}

// GetValidationEngine returns the core validation engine for external use.
func (v *EnhancedConfigValidator) GetValidationEngine() *corevalidation.ValidationEngine {
	return v.validationEngine
}

// validateServiceSecrets validates that required secrets are configured for enabled services
func (v *EnhancedConfigValidator) validateServiceSecrets(config *Config, aggregator *errors.ValidationAggregator) {
	// Define interface for checking if a service is enabled
	type enabledChecker interface {
		IsEnabled() bool
	}

	// Check cert-manager secrets
	if svc, ok := config.OpenCenter.Services["cert-manager"]; ok {
		if checker, ok := svc.(enabledChecker); ok && checker.IsEnabled() {
			if config.Secrets.CertManager.AWSAccessKey == "" {
				aggregator.AddError(errors.CreateValidationError(
					"secrets.cert_manager.aws_access_key",
					"cert-manager requires AWS access key for Route53 DNS validation",
					"Set secrets.cert_manager.aws_access_key",
					"Or configure a different DNS provider",
				))
			}
			if config.Secrets.CertManager.AWSSecretAccessKey == "" {
				aggregator.AddError(errors.CreateValidationError(
					"secrets.cert_manager.aws_secret_access_key",
					"cert-manager requires AWS secret access key for Route53 DNS validation",
					"Set secrets.cert_manager.aws_secret_access_key",
					"Or configure a different DNS provider",
				))
			}
		}
	}

	// Check loki secrets
	if svc, ok := config.OpenCenter.Services["loki"]; ok {
		if checker, ok := svc.(enabledChecker); ok && checker.IsEnabled() {
			// Use reflection to check SwiftAuthURL field
			v := reflect.ValueOf(svc)
			if v.Kind() == reflect.Ptr {
				v = v.Elem()
			}

			swiftAuthURLField := v.FieldByName("SwiftAuthURL")
			if swiftAuthURLField.IsValid() && swiftAuthURLField.String() == "" {
				aggregator.AddError(errors.CreateValidationError(
					"opencenter.services.loki.swift_auth_url",
					"loki requires Swift auth URL when using Swift storage",
					"Set opencenter.services.loki.swift_auth_url",
					"Or configure a different storage backend",
				))
			}
		}
	}

	// Check keycloak secrets
	if svc, ok := config.OpenCenter.Services["keycloak"]; ok {
		if checker, ok := svc.(enabledChecker); ok && checker.IsEnabled() {
			if config.Secrets.Keycloak.AdminPassword == "" {
				aggregator.AddError(errors.CreateValidationError(
					"secrets.keycloak.admin_password",
					"keycloak requires admin password",
					"Set secrets.keycloak.admin_password",
					"Use a strong password for the Keycloak admin user",
				))
			}
		}
	}

	// Check weave-gitops secrets
	if svc, ok := config.OpenCenter.Services["weave-gitops"]; ok {
		if checker, ok := svc.(enabledChecker); ok && checker.IsEnabled() {
			if config.Secrets.WeaveGitOps.PasswordHash == "" {
				aggregator.AddError(errors.CreateValidationError(
					"secrets.weave_gitops.password_hash",
					"weave-gitops requires password hash",
					"Set secrets.weave_gitops.password_hash",
					"Generate bcrypt hash of your password",
				))
			}
		}
	}

	// Check grafana secrets (kube-prometheus-stack)
	if svc, ok := config.OpenCenter.Services["kube-prometheus-stack"]; ok {
		if checker, ok := svc.(enabledChecker); ok && checker.IsEnabled() {
			if config.Secrets.Grafana.AdminPassword == "" {
				aggregator.AddError(errors.CreateValidationError(
					"secrets.grafana.admin_password",
					"kube-prometheus-stack requires Grafana admin password",
					"Set secrets.grafana.admin_password",
					"Use a strong password for the Grafana admin user",
				))
			}
		}
	}
}

// validateLokiStorageConfiguration validates Loki storage backend configuration
func (v *EnhancedConfigValidator) validateLokiStorageConfiguration(config *Config, aggregator *errors.ValidationAggregator) {
	// Define interface for checking if a service is enabled
	type enabledChecker interface {
		IsEnabled() bool
	}

	// Check if Loki is enabled
	if svc, ok := config.OpenCenter.Services["loki"]; ok {
		if checker, ok := svc.(enabledChecker); ok && checker.IsEnabled() {
			// Use reflection to check Loki-specific fields
			v := reflect.ValueOf(svc)
			if v.Kind() == reflect.Ptr {
				v = v.Elem()
			}

			// Get storage type
			storageTypeField := v.FieldByName("StorageType")
			var storageType string
			if storageTypeField.IsValid() {
				storageType = storageTypeField.String()
			}

			// Get S3 fields
			s3EndpointField := v.FieldByName("S3Endpoint")
			s3RegionField := v.FieldByName("S3Region")
			hasS3Config := false
			if s3EndpointField.IsValid() && s3EndpointField.String() != "" {
				hasS3Config = true
			}
			if s3RegionField.IsValid() && s3RegionField.String() != "" {
				hasS3Config = true
			}

			// Get Swift fields
			swiftAuthURLField := v.FieldByName("SwiftAuthURL")
			swiftRegionField := v.FieldByName("SwiftRegion")
			swiftAppCredIDField := v.FieldByName("SwiftApplicationCredentialID")
			hasSwiftConfig := false
			if swiftAuthURLField.IsValid() && swiftAuthURLField.String() != "" {
				hasSwiftConfig = true
			}
			if swiftRegionField.IsValid() && swiftRegionField.String() != "" {
				hasSwiftConfig = true
			}
			if swiftAppCredIDField.IsValid() && swiftAppCredIDField.String() != "" {
				hasSwiftConfig = true
			}

			// Check for Swift credentials when Swift storage is configured
			if hasSwiftConfig {
				hasSwiftPassword := config.Secrets.Loki.SwiftPassword != ""
				hasSwiftAppCred := config.Secrets.Loki.SwiftApplicationCredentialSecret != ""

				if !hasSwiftPassword && !hasSwiftAppCred {
					aggregator.AddError(errors.CreateValidationError(
						"secrets.loki",
						"Swift authentication credentials are required when using Swift storage",
						"Set secrets.loki.swift_password for password authentication",
						"Or set secrets.loki.swift_application_credential_secret for application credential authentication",
					))
				}
			}

			// Check for conflicting storage backends
			if hasS3Config && hasSwiftConfig {
				aggregator.AddError(errors.CreateValidationError(
					"opencenter.services.loki",
					"Cannot configure both S3 and Swift storage backends for Loki",
					"Choose either S3 or Swift storage",
					"Remove configuration for the unused storage backend",
				))
			}

			// Check for storage type mismatch
			if storageType == "swift" && hasS3Config && !hasSwiftConfig {
				aggregator.AddError(errors.CreateValidationError(
					"opencenter.services.loki.loki_storage_type",
					"Storage type is set to 'swift' but only S3 configuration is present",
					"Set storage type to 's3' or provide Swift configuration",
					"Ensure storage_type matches the configured backend",
				))
			}

			if storageType == "s3" && hasSwiftConfig && !hasS3Config {
				aggregator.AddError(errors.CreateValidationError(
					"opencenter.services.loki.loki_storage_type",
					"Storage type is set to 's3' but only Swift configuration is present",
					"Set storage type to 'swift' or provide S3 configuration",
					"Ensure storage_type matches the configured backend",
				))
			}
		}
	}
}

// validateVRRPConfiguration validates VRRP networking configuration
func (v *EnhancedConfigValidator) validateVRRPConfiguration(config *Config, aggregator *errors.ValidationAggregator) {
	networking := config.OpenCenter.Cluster.Kubernetes.Networking

	// When VRRP is enabled and Octavia is not used, VRRP IP must be set
	if networking.VRRPEnabled && !networking.UseOctavia && networking.VRRPIP == "" {
		aggregator.AddError(errors.CreateValidationError(
			"networking.vrrp_ip",
			"vrrp_ip must be set when use_octavia is false and VRRP is enabled",
			"Set networking.vrrp_ip to a valid IP address",
			"Or enable use_octavia to use Octavia load balancer",
			"Or disable VRRP by setting vrrp_enabled to false",
		))
	}
}

// validateCrossFieldDependencies validates dependencies between different configuration fields

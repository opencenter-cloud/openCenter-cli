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
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rackerlabs/openCenter-cli/internal/util/errors"
)

// EnhancedConfigValidator provides comprehensive configuration validation with structured error handling.
type EnhancedConfigValidator struct {
	errorHandler          errors.ErrorHandler
	errorWrapper          errors.ErrorWrapper
	autoRepair            bool
	cloudValidators       map[string]CloudProviderValidator
	connectivityValidator *ConnectivityValidator
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
	}

	// Register cloud provider validators
	validator.cloudValidators["openstack"] = NewOpenStackValidator()
	validator.cloudValidators["aws"] = NewAWSValidator()
	validator.cloudValidators["vsphere"] = NewVSphereValidator()

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

	// Validate cluster name
	if config.ClusterName() == "" {
		aggregator.AddError(errors.CreateValidationError(
			"opencenter.cluster.cluster_name",
			"cluster name is required",
			"Set opencenter.cluster.cluster_name to a valid cluster name",
			"Use alphanumeric characters, hyphens, and underscores only",
		))
	} else if err := v.validateClusterNameFormat(config.ClusterName()); err != nil {
		aggregator.AddError(errors.CreateValidationError(
			"opencenter.cluster.cluster_name",
			fmt.Sprintf("invalid cluster name format: %v", err),
			"Use alphanumeric characters, hyphens, and underscores only",
			"Start with an alphanumeric character",
			"Keep length under 255 characters",
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
}

// validateNetworkConfiguration validates network plugin and subnet configuration.
func (v *EnhancedConfigValidator) validateNetworkConfiguration(ctx context.Context, config *Config, aggregator *errors.ValidationAggregator) {
	if config == nil {
		return
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

	// Validate subnet configurations
	v.validateSubnetConfiguration(config, aggregator)

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

func (v *EnhancedConfigValidator) validateClusterNameFormat(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("cluster name cannot be empty")
	}
	if len(name) > 255 {
		return fmt.Errorf("cluster name too long (max 255 characters)")
	}

	// Check for valid characters (alphanumeric, hyphens, underscores)
	validName := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
	if !validName.MatchString(name) {
		return fmt.Errorf("cluster name must start with alphanumeric character and contain only alphanumeric, hyphens, and underscores")
	}

	return nil
}

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

	// Validate AWS credentials for S3 backend (with fallback support)
	accessKey, secretKey := config.GetS3BackendCredentials()
	if accessKey == "" || secretKey == "" {
		aggregator.AddError(errors.CreateCredentialError(
			"AWS",
			"opencenter.cluster.aws_access_key or secrets.aws.access_key",
			"AWS credentials required for S3/AWS backend: either set opencenter.cluster.aws_access_key/aws_secret_access_key or secrets.aws.access_key/secret_access_key",
			nil,
		))
	}
}

func (v *EnhancedConfigValidator) validateSubnetConfiguration(config *Config, aggregator *errors.ValidationAggregator) {
	// Validate pod subnet
	if podSubnet := config.OpenCenter.Cluster.Kubernetes.SubnetPods; podSubnet != "" {
		if !v.isValidCIDR(podSubnet) {
			aggregator.AddError(errors.CreateValidationError(
				"opencenter.cluster.kubernetes.subnet_pods",
				"invalid pod subnet CIDR format",
				"Use valid CIDR notation (e.g., '10.42.0.0/16')",
				"Ensure subnet doesn't conflict with node or service subnets",
			))
		}
	} else {
		aggregator.AddWarning(errors.CreateValidationError(
			"opencenter.cluster.kubernetes.subnet_pods",
			"pod subnet not specified",
			"Set subnet_pods to a CIDR range (e.g., '10.42.0.0/16')",
			"Ensure it doesn't conflict with node or service subnets",
		))
	}

	// Validate service subnet
	if serviceSubnet := config.OpenCenter.Cluster.Kubernetes.SubnetServices; serviceSubnet != "" {
		if !v.isValidCIDR(serviceSubnet) {
			aggregator.AddError(errors.CreateValidationError(
				"opencenter.cluster.kubernetes.subnet_services",
				"invalid service subnet CIDR format",
				"Use valid CIDR notation (e.g., '10.43.0.0/16')",
				"Ensure subnet doesn't conflict with node or pod subnets",
			))
		}
	} else {
		aggregator.AddWarning(errors.CreateValidationError(
			"opencenter.cluster.kubernetes.subnet_services",
			"service subnet not specified",
			"Set subnet_services to a CIDR range (e.g., '10.43.0.0/16')",
			"Ensure it doesn't conflict with node or pod subnets",
		))
	}

	// Check for subnet conflicts
	if config.OpenCenter.Cluster.Kubernetes.SubnetPods != "" &&
		config.OpenCenter.Cluster.Kubernetes.SubnetServices != "" {
		if v.subnetsOverlap(config.OpenCenter.Cluster.Kubernetes.SubnetPods,
			config.OpenCenter.Cluster.Kubernetes.SubnetServices) {
			aggregator.AddError(errors.CreateValidationError(
				"opencenter.cluster.kubernetes.subnet_pods",
				"pod and service subnets overlap",
				"Use non-overlapping CIDR ranges for pods and services",
				"Example: pods=10.42.0.0/16, services=10.43.0.0/16",
			))
		}
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

func (v *EnhancedConfigValidator) isValidCIDR(cidr string) bool {
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}

func (v *EnhancedConfigValidator) subnetsOverlap(cidr1, cidr2 string) bool {
	_, net1, err1 := net.ParseCIDR(cidr1)
	_, net2, err2 := net.ParseCIDR(cidr2)

	if err1 != nil || err2 != nil {
		return false // Can't determine overlap if parsing fails
	}

	return net1.Contains(net2.IP) || net2.Contains(net1.IP)
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

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
	"fmt"
	"time"

	"github.com/rackerlabs/openCenter-cli/internal/config/services"
	"github.com/rackerlabs/openCenter-cli/internal/util/errors"
)

// ConfigBuilder provides a fluent interface for constructing cluster configurations.
// It enables type-safe configuration building with method chaining and validation.
type ConfigBuilder interface {
	// Core cluster configuration
	WithProvider(provider string) ConfigBuilder
	WithOrganization(org string) ConfigBuilder
	WithClusterName(name string) ConfigBuilder
	WithEnvironment(env string) ConfigBuilder
	WithRegion(region string) ConfigBuilder
	
	// Kubernetes configuration
	WithKubernetesVersion(version string) ConfigBuilder
	WithNodeCounts(masters, workers int) ConfigBuilder
	WithMasterCount(count int) ConfigBuilder
	WithWorkerCount(count int) ConfigBuilder
	WithWindowsWorkerCount(count int) ConfigBuilder
	
	// Networking configuration
	WithNetworking(config Networking) ConfigBuilder
	WithSubnetNodes(subnet string) ConfigBuilder
	WithSubnetPods(subnet string) ConfigBuilder
	WithSubnetServices(subnet string) ConfigBuilder
	WithDNSNameservers(nameservers []string) ConfigBuilder
	WithNTPServers(servers []string) ConfigBuilder
	
	// Infrastructure configuration
	WithSSHUser(user string) ConfigBuilder
	WithSSHAuthorizedKeys(keys []string) ConfigBuilder
	WithBaseDomain(domain string) ConfigBuilder
	WithAdminEmail(email string) ConfigBuilder
	
	// Service configuration
	WithServices(services ...string) ConfigBuilder
	WithService(name string, enabled bool) ConfigBuilder
	
	// Cloud provider configuration
	WithAWSConfig(config SimplifiedAWSCloud) ConfigBuilder
	WithOpenStackConfig(config SimplifiedOpenStackCloud) ConfigBuilder
	
	// Secrets configuration
	WithSecretsBackend(backend string) ConfigBuilder
	WithBarbicanConfig(config BarbicanConfig) ConfigBuilder
	
	// GitOps configuration
	WithGitOpsConfig(config GitOpsConfig) ConfigBuilder
	WithGitURL(url string) ConfigBuilder
	WithGitBranch(branch string) ConfigBuilder
	
	// Storage configuration
	WithStorageConfig(config StorageConfig) ConfigBuilder
	WithDefaultStorageClass(class string) ConfigBuilder
	
	// Security configuration
	WithSecurityConfig(config Security) ConfigBuilder
	WithK8sHardening(enabled bool) ConfigBuilder
	WithOSHardening(enabled bool) ConfigBuilder
	
	// Talos configuration
	WithTalosConfig(config *TalosConfig) ConfigBuilder
	WithTalosEnabled(enabled bool) ConfigBuilder
	
	// Generic override (string-based, runtime validation)
	WithOverride(path string, value interface{}) ConfigBuilder
	
	// Type-safe path override (compile-time type safety)
	WithPath(path TypedConfigPath[string], value string) ConfigBuilder
	WithPathInt(path TypedConfigPath[int], value int) ConfigBuilder
	WithPathBool(path TypedConfigPath[bool], value bool) ConfigBuilder
	WithPathStringSlice(path TypedConfigPath[[]string], value []string) ConfigBuilder
	
	// Conditional configuration based on provider
	WhenProvider(provider string, configureFn func(ConfigBuilder) ConfigBuilder) ConfigBuilder
	WhenProviderIn(providers []string, configureFn func(ConfigBuilder) ConfigBuilder) ConfigBuilder
	WhenNotProvider(provider string, configureFn func(ConfigBuilder) ConfigBuilder) ConfigBuilder
	
	// Metadata
	WithMetadata(metadata ConfigMetadata) ConfigBuilder
	WithTag(key, value string) ConfigBuilder
	WithAnnotation(key, value string) ConfigBuilder
	
	// Build and validation
	Build() (Config, error)
	Validate() []ValidationError
}

// FluentConfigBuilder implements ConfigBuilder with a fluent API pattern.
type FluentConfigBuilder struct {
	config          Config
	errors          []ValidationError
	validators      []BuilderValidator
	errorAggregator *errors.ValidationAggregator
	errorHandler    errors.ErrorHandler
}

// NewConfigBuilder creates a new FluentConfigBuilder with default values.
func NewConfigBuilder(clusterName string) ConfigBuilder {
	return &FluentConfigBuilder{
		config:          defaultConfig(clusterName),
		errors:          []ValidationError{},
		validators:      []BuilderValidator{},
		errorAggregator: errors.NewValidationAggregator(),
		errorHandler:    errors.NewDefaultErrorHandler(),
	}
}

// NewConfigBuilderFromConfig creates a new FluentConfigBuilder from an existing configuration.
func NewConfigBuilderFromConfig(config Config) ConfigBuilder {
	return &FluentConfigBuilder{
		config:          config,
		errors:          []ValidationError{},
		validators:      []BuilderValidator{},
		errorAggregator: errors.NewValidationAggregator(),
		errorHandler:    errors.NewDefaultErrorHandler(),
	}
}

// WithProvider sets the infrastructure provider.
func (b *FluentConfigBuilder) WithProvider(provider string) ConfigBuilder {
	b.config.OpenCenter.Infrastructure.Provider = provider
	return b
}

// WithOrganization sets the organization name.
func (b *FluentConfigBuilder) WithOrganization(org string) ConfigBuilder {
	b.config.OpenCenter.Meta.Organization = org
	return b
}

// WithClusterName sets the cluster name.
func (b *FluentConfigBuilder) WithClusterName(name string) ConfigBuilder {
	b.config.OpenCenter.Meta.Name = name
	b.config.OpenCenter.Cluster.ClusterName = name
	return b
}

// WithEnvironment sets the environment (e.g., dev, staging, prod).
func (b *FluentConfigBuilder) WithEnvironment(env string) ConfigBuilder {
	b.config.OpenCenter.Meta.Env = env
	return b
}

// WithRegion sets the region.
func (b *FluentConfigBuilder) WithRegion(region string) ConfigBuilder {
	b.config.OpenCenter.Meta.Region = region
	return b
}

// WithKubernetesVersion sets the Kubernetes version.
func (b *FluentConfigBuilder) WithKubernetesVersion(version string) ConfigBuilder {
	b.config.OpenCenter.Cluster.Kubernetes.Version = version
	return b
}

// WithNodeCounts sets both master and worker node counts.
func (b *FluentConfigBuilder) WithNodeCounts(masters, workers int) ConfigBuilder {
	b.config.OpenCenter.Cluster.Kubernetes.MasterCount = masters
	b.config.OpenCenter.Cluster.Kubernetes.WorkerCount = workers
	return b
}

// WithMasterCount sets the master node count.
func (b *FluentConfigBuilder) WithMasterCount(count int) ConfigBuilder {
	b.config.OpenCenter.Cluster.Kubernetes.MasterCount = count
	return b
}

// WithWorkerCount sets the worker node count.
func (b *FluentConfigBuilder) WithWorkerCount(count int) ConfigBuilder {
	b.config.OpenCenter.Cluster.Kubernetes.WorkerCount = count
	return b
}

// WithWindowsWorkerCount sets the Windows worker node count.
func (b *FluentConfigBuilder) WithWindowsWorkerCount(count int) ConfigBuilder {
	b.config.OpenCenter.Cluster.Kubernetes.WorkerCountWindows = count
	return b
}

// WithNetworking sets the complete networking configuration.
func (b *FluentConfigBuilder) WithNetworking(config Networking) ConfigBuilder {
	b.config.Networking = config
	return b
}

// WithSubnetNodes sets the node subnet.
func (b *FluentConfigBuilder) WithSubnetNodes(subnet string) ConfigBuilder {
	b.config.Networking.SubnetNodes = subnet
	return b
}

// WithSubnetPods sets the pod subnet.
func (b *FluentConfigBuilder) WithSubnetPods(subnet string) ConfigBuilder {
	b.config.Networking.SubnetPods = subnet
	b.config.OpenCenter.Cluster.Kubernetes.SubnetPods = subnet
	return b
}

// WithSubnetServices sets the service subnet.
func (b *FluentConfigBuilder) WithSubnetServices(subnet string) ConfigBuilder {
	b.config.Networking.SubnetServices = subnet
	b.config.OpenCenter.Cluster.Kubernetes.SubnetServices = subnet
	return b
}

// WithDNSNameservers sets the DNS nameservers.
func (b *FluentConfigBuilder) WithDNSNameservers(nameservers []string) ConfigBuilder {
	b.config.Networking.DNSNameservers = nameservers
	return b
}

// WithNTPServers sets the NTP servers.
func (b *FluentConfigBuilder) WithNTPServers(servers []string) ConfigBuilder {
	b.config.Networking.NTPServers = servers
	return b
}

// WithSSHUser sets the SSH user for cluster nodes.
func (b *FluentConfigBuilder) WithSSHUser(user string) ConfigBuilder {
	b.config.OpenCenter.Infrastructure.SSHUser = user
	return b
}

// WithSSHAuthorizedKeys sets the SSH authorized keys.
func (b *FluentConfigBuilder) WithSSHAuthorizedKeys(keys []string) ConfigBuilder {
	b.config.OpenCenter.Cluster.SSHAuthorizedKeys = keys
	return b
}

// WithBaseDomain sets the base domain for the cluster.
func (b *FluentConfigBuilder) WithBaseDomain(domain string) ConfigBuilder {
	b.config.OpenCenter.Cluster.BaseDomain = domain
	return b
}

// WithAdminEmail sets the admin email address.
func (b *FluentConfigBuilder) WithAdminEmail(email string) ConfigBuilder {
	b.config.OpenCenter.Cluster.AdminEmail = email
	return b
}

// WithServices enables multiple services by name.
func (b *FluentConfigBuilder) WithServices(serviceNames ...string) ConfigBuilder {
	for _, name := range serviceNames {
		b.WithService(name, true)
	}
	return b
}

// WithService enables or disables a specific service.
func (b *FluentConfigBuilder) WithService(name string, enabled bool) ConfigBuilder {
	if svc, exists := b.config.OpenCenter.Services[name]; exists {
		// Use type assertion to access the Enabled field
		switch s := svc.(type) {
		case *services.CalicoConfig:
			s.Enabled = enabled
		case *services.CertManagerConfig:
			s.Enabled = enabled
		case *services.EtcdBackupConfig:
			s.Enabled = enabled
		case *services.HeadlampConfig:
			s.Enabled = enabled
		case *services.KeycloakConfig:
			s.Enabled = enabled
		case *services.PrometheusStackConfig:
			s.Enabled = enabled
		case *services.LokiConfig:
			s.Enabled = enabled
		case *services.VeleroConfig:
			s.Enabled = enabled
		case *services.VSphereCSIConfig:
			s.Enabled = enabled
		case *services.WeaveGitOpsConfig:
			s.Enabled = enabled
		case *services.AlertProxyConfig:
			s.Enabled = enabled
		case *services.DefaultServiceConfig:
			s.Enabled = enabled
		}
	}
	return b
}

// WithAWSConfig sets the AWS cloud configuration.
func (b *FluentConfigBuilder) WithAWSConfig(config SimplifiedAWSCloud) ConfigBuilder {
	b.config.OpenCenter.Infrastructure.Cloud.AWS = config
	return b
}

// WithOpenStackConfig sets the OpenStack cloud configuration.
func (b *FluentConfigBuilder) WithOpenStackConfig(config SimplifiedOpenStackCloud) ConfigBuilder {
	b.config.OpenCenter.Infrastructure.Cloud.OpenStack = config
	return b
}

// WithSecretsBackend sets the secrets backend type.
func (b *FluentConfigBuilder) WithSecretsBackend(backend string) ConfigBuilder {
	b.config.OpenCenter.Secrets.Backend = backend
	return b
}

// WithBarbicanConfig sets the Barbican configuration.
func (b *FluentConfigBuilder) WithBarbicanConfig(config BarbicanConfig) ConfigBuilder {
	b.config.OpenCenter.Secrets.Barbican = config
	return b
}

// WithGitOpsConfig sets the complete GitOps configuration.
func (b *FluentConfigBuilder) WithGitOpsConfig(config GitOpsConfig) ConfigBuilder {
	b.config.OpenCenter.GitOps = config
	return b
}

// WithGitURL sets the Git repository URL.
func (b *FluentConfigBuilder) WithGitURL(url string) ConfigBuilder {
	b.config.OpenCenter.GitOps.GitURL = url
	return b
}

// WithGitBranch sets the Git branch.
func (b *FluentConfigBuilder) WithGitBranch(branch string) ConfigBuilder {
	b.config.OpenCenter.GitOps.GitBranch = branch
	return b
}

// WithStorageConfig sets the complete storage configuration.
func (b *FluentConfigBuilder) WithStorageConfig(config StorageConfig) ConfigBuilder {
	b.config.OpenCenter.Storage = config
	return b
}

// WithDefaultStorageClass sets the default storage class.
func (b *FluentConfigBuilder) WithDefaultStorageClass(class string) ConfigBuilder {
	b.config.OpenCenter.Storage.DefaultStorageClass = class
	return b
}

// WithSecurityConfig sets the complete security configuration.
func (b *FluentConfigBuilder) WithSecurityConfig(config Security) ConfigBuilder {
	b.config.Security = config
	return b
}

// WithK8sHardening enables or disables Kubernetes hardening.
func (b *FluentConfigBuilder) WithK8sHardening(enabled bool) ConfigBuilder {
	b.config.Security.K8sHardening = enabled
	return b
}

// WithOSHardening enables or disables OS hardening.
func (b *FluentConfigBuilder) WithOSHardening(enabled bool) ConfigBuilder {
	b.config.Security.OSHardening = enabled
	return b
}

// WithTalosConfig sets the complete Talos configuration.
func (b *FluentConfigBuilder) WithTalosConfig(config *TalosConfig) ConfigBuilder {
	b.config.OpenCenter.Talos = config
	return b
}

// WithTalosEnabled enables or disables Talos.
func (b *FluentConfigBuilder) WithTalosEnabled(enabled bool) ConfigBuilder {
	if enabled && b.config.OpenCenter.Talos == nil {
		b.config.OpenCenter.Talos = DefaultTalosConfig(b.config.OpenCenter.Cluster.ClusterName)
	} else if !enabled {
		b.config.OpenCenter.Talos = nil
	}
	return b
}

// WithOverride sets a generic configuration override at the specified path.
// Note: This method uses runtime validation. For compile-time type safety,
// use the WithPath* methods with TypedConfigPaths constants.
func (b *FluentConfigBuilder) WithOverride(path string, value interface{}) ConfigBuilder {
	if b.config.Overrides == nil {
		b.config.Overrides = make(map[string]any)
	}
	b.config.Overrides[path] = value
	return b
}

// WithPath sets a string configuration value at the specified type-safe path.
// This method provides compile-time type safety for configuration paths.
//
// Example:
//   builder.WithPath(TypedConfigPaths.ClusterName, "my-cluster")
func (b *FluentConfigBuilder) WithPath(path TypedConfigPath[string], value string) ConfigBuilder {
	if b.config.Overrides == nil {
		b.config.Overrides = make(map[string]any)
	}
	b.config.Overrides[path.Path()] = value
	return b
}

// WithPathInt sets an integer configuration value at the specified type-safe path.
// This method provides compile-time type safety for configuration paths.
//
// Example:
//   builder.WithPathInt(TypedConfigPaths.MasterCount, 3)
func (b *FluentConfigBuilder) WithPathInt(path TypedConfigPath[int], value int) ConfigBuilder {
	if b.config.Overrides == nil {
		b.config.Overrides = make(map[string]any)
	}
	b.config.Overrides[path.Path()] = value
	return b
}

// WithPathBool sets a boolean configuration value at the specified type-safe path.
// This method provides compile-time type safety for configuration paths.
//
// Example:
//   builder.WithPathBool(TypedConfigPaths.K8sHardening, true)
func (b *FluentConfigBuilder) WithPathBool(path TypedConfigPath[bool], value bool) ConfigBuilder {
	if b.config.Overrides == nil {
		b.config.Overrides = make(map[string]any)
	}
	b.config.Overrides[path.Path()] = value
	return b
}

// WithPathStringSlice sets a string slice configuration value at the specified type-safe path.
// This method provides compile-time type safety for configuration paths.
//
// Example:
//   builder.WithPathStringSlice(TypedConfigPaths.DNSNameservers, []string{"8.8.8.8", "8.8.4.4"})
func (b *FluentConfigBuilder) WithPathStringSlice(path TypedConfigPath[[]string], value []string) ConfigBuilder {
	if b.config.Overrides == nil {
		b.config.Overrides = make(map[string]any)
	}
	b.config.Overrides[path.Path()] = value
	return b
}

// WhenProvider conditionally applies configuration only when the provider matches.
// This enables provider-specific configuration in a fluent, readable way.
//
// Example:
//   builder.WhenProvider("openstack", func(b ConfigBuilder) ConfigBuilder {
//       return b.WithOpenStackConfig(osConfig)
//   }).WhenProvider("aws", func(b ConfigBuilder) ConfigBuilder {
//       return b.WithAWSConfig(awsConfig)
//   })
func (b *FluentConfigBuilder) WhenProvider(provider string, configureFn func(ConfigBuilder) ConfigBuilder) ConfigBuilder {
	if b.config.OpenCenter.Infrastructure.Provider == provider {
		return configureFn(b)
	}
	return b
}

// WhenProviderIn conditionally applies configuration when the provider is in the given list.
// This is useful for applying configuration to multiple providers at once.
//
// Example:
//   builder.WhenProviderIn([]string{"openstack", "aws"}, func(b ConfigBuilder) ConfigBuilder {
//       return b.WithK8sHardening(true).WithOSHardening(true)
//   })
func (b *FluentConfigBuilder) WhenProviderIn(providers []string, configureFn func(ConfigBuilder) ConfigBuilder) ConfigBuilder {
	currentProvider := b.config.OpenCenter.Infrastructure.Provider
	for _, provider := range providers {
		if currentProvider == provider {
			return configureFn(b)
		}
	}
	return b
}

// WhenNotProvider conditionally applies configuration when the provider does NOT match.
// This is useful for excluding specific providers from certain configurations.
//
// Example:
//   builder.WhenNotProvider("kind", func(b ConfigBuilder) ConfigBuilder {
//       return b.WithK8sHardening(true)
//   })
func (b *FluentConfigBuilder) WhenNotProvider(provider string, configureFn func(ConfigBuilder) ConfigBuilder) ConfigBuilder {
	if b.config.OpenCenter.Infrastructure.Provider != provider {
		return configureFn(b)
	}
	return b
}

// WithMetadata sets the complete metadata.
func (b *FluentConfigBuilder) WithMetadata(metadata ConfigMetadata) ConfigBuilder {
	b.config.Metadata = metadata
	return b
}

// WithTag adds or updates a metadata tag.
func (b *FluentConfigBuilder) WithTag(key, value string) ConfigBuilder {
	if b.config.Metadata.Tags == nil {
		b.config.Metadata.Tags = make(map[string]string)
	}
	b.config.Metadata.Tags[key] = value
	return b
}

// WithAnnotation adds or updates a metadata annotation.
func (b *FluentConfigBuilder) WithAnnotation(key, value string) ConfigBuilder {
	if b.config.Metadata.Annotations == nil {
		b.config.Metadata.Annotations = make(map[string]string)
	}
	b.config.Metadata.Annotations[key] = value
	return b
}

// Build constructs the final configuration and validates it.
func (b *FluentConfigBuilder) Build() (Config, error) {
	// Update metadata timestamps
	if b.config.Metadata.CreatedAt.IsZero() {
		b.config.Metadata.CreatedAt = time.Now()
	}
	b.config.Metadata.UpdatedAt = time.Now()
	
	// Run validation
	validationErrors := b.Validate()
	if len(validationErrors) > 0 {
		// Convert validation errors to structured errors with context
		for _, valErr := range validationErrors {
			structuredErr := errors.CreateValidationError(
				valErr.Field,
				valErr.Message,
			)
			// Add operation context
			structuredErr.Operation = "configuration_build"
			b.errorAggregator.AddError(structuredErr)
		}
		
		// Get formatted error summary
		summary := b.errorAggregator.GetSummary()
		return Config{}, fmt.Errorf("configuration validation failed with %d errors:\n%s", 
			len(validationErrors), summary)
	}
	
	return b.config, nil
}

// Validate runs all registered validators and returns any validation errors.
func (b *FluentConfigBuilder) Validate() []ValidationError {
	// Clear previous errors
	b.errors = []ValidationError{}
	b.errorAggregator.ClearAll()
	
	// Run basic validation
	b.validateRequired()
	b.validateProvider()
	b.validateNodeCounts()
	b.validateNetworking()
	
	// Run custom validators
	for _, validator := range b.validators {
		errors := validator.Validate(b.config)
		b.errors = append(b.errors, errors...)
	}
	
	return b.errors
}

// GetValidationReport returns a detailed validation report with context and suggestions.
func (b *FluentConfigBuilder) GetValidationReport() *errors.ValidationResult {
	// Run validation if not already done
	if len(b.errors) == 0 && !b.errorAggregator.HasErrors() {
		b.Validate()
	}
	
	// Convert validation errors to structured errors
	for _, valErr := range b.errors {
		structuredErr := &errors.StructuredError{
			Type:        errors.ValidationError,
			Field:       valErr.Field,
			Message:     valErr.Message,
			Suggestions: valErr.Suggestions,
			Context:     valErr.Context,
			Operation:   "configuration_validation",
			Retryable:   false,
		}
		b.errorAggregator.AddError(structuredErr)
	}
	
	return b.errorAggregator.ToValidationResult()
}

// AddValidator adds a custom validator to the builder.
func (b *FluentConfigBuilder) AddValidator(validator BuilderValidator) ConfigBuilder {
	b.validators = append(b.validators, validator)
	return b
}

// validateRequired checks that required fields are set.
func (b *FluentConfigBuilder) validateRequired() {
	if b.config.OpenCenter.Meta.Name == "" {
		b.errors = append(b.errors, ValidationError{
			Field:   "opencenter.meta.name",
			Message: "cluster name is required",
			Suggestions: []string{
				"Set cluster name with: builder.WithClusterName(\"my-cluster\")",
				"Cluster name should be lowercase alphanumeric with hyphens",
				"Example: \"production-cluster-01\"",
			},
		})
	}
	
	if b.config.OpenCenter.Meta.Organization == "" {
		b.errors = append(b.errors, ValidationError{
			Field:   "opencenter.meta.organization",
			Message: "organization is required",
			Suggestions: []string{
				"Set organization with: builder.WithOrganization(\"my-org\")",
				"Organization name should match your company or team name",
				"Example: \"acme-corp\"",
			},
		})
	}
	
	if b.config.OpenCenter.Infrastructure.Provider == "" {
		b.errors = append(b.errors, ValidationError{
			Field:   "opencenter.infrastructure.provider",
			Message: "provider is required",
			Suggestions: []string{
				"Set provider with: builder.WithProvider(\"openstack\")",
				"Supported providers: openstack, aws, baremetal, kind",
				"Choose based on your infrastructure platform",
			},
		})
	}
}

// validateProvider checks provider-specific requirements.
func (b *FluentConfigBuilder) validateProvider() {
	provider := b.config.OpenCenter.Infrastructure.Provider
	
	switch provider {
	case "openstack":
		if b.config.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL == "" {
			b.errors = append(b.errors, ValidationError{
				Field:   "opencenter.infrastructure.cloud.openstack.auth_url",
				Message: "OpenStack auth URL is required",
				Suggestions: []string{
					"Set auth URL with: builder.WithOpenStackConfig(config)",
					"Example: \"https://openstack.example.com:5000/v3\"",
					"Get auth URL from your OpenStack dashboard or administrator",
					"Verify connectivity: curl -k <auth_url>",
				},
			})
		}
	case "aws":
		if b.config.OpenCenter.Infrastructure.Cloud.AWS.Region == "" {
			b.errors = append(b.errors, ValidationError{
				Field:   "opencenter.infrastructure.cloud.aws.region",
				Message: "AWS region is required",
				Suggestions: []string{
					"Set region with: builder.WithAWSConfig(config)",
					"Example regions: us-east-1, us-west-2, eu-west-1",
					"List available regions: aws ec2 describe-regions",
					"Choose region closest to your users for better latency",
				},
			})
		}
	case "baremetal", "kind":
		// No specific validation for these providers
	default:
		b.errors = append(b.errors, ValidationError{
			Field:   "opencenter.infrastructure.provider",
			Message: fmt.Sprintf("unsupported provider: %s", provider),
			Suggestions: []string{
				"Supported providers: openstack, aws, baremetal, kind",
				"Check provider name for typos",
				"Refer to documentation for provider-specific setup",
			},
		})
	}
}

// validateNodeCounts checks that node counts are valid.
func (b *FluentConfigBuilder) validateNodeCounts() {
	if b.config.OpenCenter.Cluster.Kubernetes.MasterCount < 1 {
		b.errors = append(b.errors, ValidationError{
			Field:   "opencenter.cluster.kubernetes.master_count",
			Message: "master count must be at least 1",
			Suggestions: []string{
				"Set master count with: builder.WithMasterCount(3)",
				"Recommended: 3 masters for high availability",
				"Minimum: 1 master for development/testing",
				"Use odd numbers (1, 3, 5) for proper etcd quorum",
			},
		})
	}
	
	if b.config.OpenCenter.Cluster.Kubernetes.MasterCount%2 == 0 {
		b.errors = append(b.errors, ValidationError{
			Field:   "opencenter.cluster.kubernetes.master_count",
			Message: "master count should be odd for HA quorum",
			Suggestions: []string{
				"Use odd numbers: 1, 3, 5, 7 for proper etcd quorum",
				"Even numbers can cause split-brain scenarios",
				"Recommended: 3 masters for production clusters",
				"Learn more: https://etcd.io/docs/v3.5/faq/#why-an-odd-number-of-cluster-members",
			},
		})
	}
	
	if b.config.OpenCenter.Cluster.Kubernetes.WorkerCount < 0 {
		b.errors = append(b.errors, ValidationError{
			Field:   "opencenter.cluster.kubernetes.worker_count",
			Message: "worker count cannot be negative",
			Suggestions: []string{
				"Set worker count with: builder.WithWorkerCount(3)",
				"Minimum: 0 workers (masters can run workloads)",
				"Recommended: 3+ workers for production workloads",
				"Scale based on expected workload requirements",
			},
		})
	}
}

// validateNetworking checks networking configuration.
func (b *FluentConfigBuilder) validateNetworking() {
	if b.config.Networking.SubnetNodes == "" {
		b.errors = append(b.errors, ValidationError{
			Field:   "networking.subnet_nodes",
			Message: "node subnet is required",
			Suggestions: []string{
				"Set node subnet with: builder.WithSubnetNodes(\"10.0.0.0/24\")",
				"Use CIDR notation for subnet specification",
				"Example: \"192.168.1.0/24\" for 254 usable addresses",
				"Ensure subnet doesn't overlap with existing networks",
			},
		})
	}
	
	if b.config.Networking.SubnetPods == "" {
		b.errors = append(b.errors, ValidationError{
			Field:   "networking.subnet_pods",
			Message: "pod subnet is required",
			Suggestions: []string{
				"Set pod subnet with: builder.WithSubnetPods(\"10.244.0.0/16\")",
				"Use large subnet for pod networking (e.g., /16)",
				"Default Kubernetes pod CIDR: 10.244.0.0/16",
				"Ensure pod subnet doesn't overlap with node or service subnets",
			},
		})
	}
	
	if b.config.Networking.SubnetServices == "" {
		b.errors = append(b.errors, ValidationError{
			Field:   "networking.subnet_services",
			Message: "service subnet is required",
			Suggestions: []string{
				"Set service subnet with: builder.WithSubnetServices(\"10.96.0.0/12\")",
				"Use /12 or larger for service networking",
				"Default Kubernetes service CIDR: 10.96.0.0/12",
				"Ensure service subnet doesn't overlap with node or pod subnets",
			},
		})
	}
}

// BuilderValidator defines an interface for custom configuration validators used by the builder.
type BuilderValidator interface {
	Validate(config Config) []ValidationError
}

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field       string
	Message     string
	Suggestions []string
	Context     map[string]interface{}
}

// Error implements the error interface for ValidationError.
func (e ValidationError) Error() string {
	msg := fmt.Sprintf("%s: %s", e.Field, e.Message)
	if len(e.Suggestions) > 0 {
		msg += "\nSuggestions:"
		for _, suggestion := range e.Suggestions {
			msg += "\n  - " + suggestion
		}
	}
	return msg
}

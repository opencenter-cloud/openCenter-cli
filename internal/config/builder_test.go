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
	"strings"
	"testing"
)

// TestFluentAPIMethodChaining verifies that all builder methods return the builder instance
// to enable method chaining.
func TestFluentAPIMethodChaining(t *testing.T) {
	// Test that we can chain multiple method calls together
	builder := NewConfigBuilder("test-cluster").
		WithOrganization("test-org").
		WithProvider("openstack").
		WithRegion("us-east-1").
		WithEnvironment("dev").
		WithKubernetesVersion("1.33.7").
		WithNodeCounts(3, 5).
		WithSubnetNodes("10.0.0.0/24").
		WithSubnetPods("10.42.0.0/16").
		WithSubnetServices("10.43.0.0/16").
		WithSSHUser("ubuntu").
		WithBaseDomain("example.com").
		WithAdminEmail("admin@example.com").
		WithK8sHardening(true).
		WithOSHardening(true).
		WithTag("environment", "test").
		WithAnnotation("created-by", "test").
		WithOpenStackConfig(SimplifiedOpenStackCloud{
			AuthURL:    "https://identity.example.com/v3",
			Region:     "us-east-1",
			TenantName: "test-tenant",
		})

	// Verify the builder is not nil (method chaining worked)
	if builder == nil {
		t.Fatal("Builder should not be nil after method chaining")
	}

	// Build the configuration
	config, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	// Verify the configuration values were set correctly
	if config.OpenCenter.Meta.Name != "test-cluster" {
		t.Errorf("Expected cluster name 'test-cluster', got '%s'", config.OpenCenter.Meta.Name)
	}

	if config.OpenCenter.Meta.Organization != "test-org" {
		t.Errorf("Expected organization 'test-org', got '%s'", config.OpenCenter.Meta.Organization)
	}

	if config.OpenCenter.Infrastructure.Provider != "openstack" {
		t.Errorf("Expected provider 'openstack', got '%s'", config.OpenCenter.Infrastructure.Provider)
	}

	if config.OpenCenter.Meta.Region != "us-east-1" {
		t.Errorf("Expected region 'us-east-1', got '%s'", config.OpenCenter.Meta.Region)
	}

	if config.OpenCenter.Meta.Env != "dev" {
		t.Errorf("Expected environment 'dev', got '%s'", config.OpenCenter.Meta.Env)
	}

	if config.OpenCenter.Cluster.Kubernetes.Version != "1.33.7" {
		t.Errorf("Expected Kubernetes version '1.33.7', got '%s'", config.OpenCenter.Cluster.Kubernetes.Version)
	}

	if config.OpenCenter.Cluster.Kubernetes.MasterCount != 3 {
		t.Errorf("Expected master count 3, got %d", config.OpenCenter.Cluster.Kubernetes.MasterCount)
	}

	if config.OpenCenter.Cluster.Kubernetes.WorkerCount != 5 {
		t.Errorf("Expected worker count 5, got %d", config.OpenCenter.Cluster.Kubernetes.WorkerCount)
	}

	if config.OpenCenter.Cluster.Kubernetes.Networking.SubnetNodes != "10.0.0.0/24" {
		t.Errorf("Expected subnet nodes '10.0.0.0/24', got '%s'", config.OpenCenter.Cluster.Kubernetes.Networking.SubnetNodes)
	}

	if config.OpenCenter.Cluster.Kubernetes.Networking.SubnetPods != "10.42.0.0/16" {
		t.Errorf("Expected subnet pods '10.42.0.0/16', got '%s'", config.OpenCenter.Cluster.Kubernetes.Networking.SubnetPods)
	}

	if config.OpenCenter.Cluster.Kubernetes.Networking.SubnetServices != "10.43.0.0/16" {
		t.Errorf("Expected subnet services '10.43.0.0/16', got '%s'", config.OpenCenter.Cluster.Kubernetes.Networking.SubnetServices)
	}

	if config.OpenCenter.Infrastructure.SSHUser != "ubuntu" {
		t.Errorf("Expected SSH user 'ubuntu', got '%s'", config.OpenCenter.Infrastructure.SSHUser)
	}

	if config.OpenCenter.Cluster.BaseDomain != "example.com" {
		t.Errorf("Expected base domain 'example.com', got '%s'", config.OpenCenter.Cluster.BaseDomain)
	}

	if config.OpenCenter.Cluster.AdminEmail != "admin@example.com" {
		t.Errorf("Expected admin email 'admin@example.com', got '%s'", config.OpenCenter.Cluster.AdminEmail)
	}

	if !config.Security.K8sHardening {
		t.Error("Expected K8s hardening to be enabled")
	}

	if !config.Security.OSHardening {
		t.Error("Expected OS hardening to be enabled")
	}

	if config.Metadata.Tags["environment"] != "test" {
		t.Errorf("Expected tag 'environment' to be 'test', got '%s'", config.Metadata.Tags["environment"])
	}

	if config.Metadata.Annotations["created-by"] != "test" {
		t.Errorf("Expected annotation 'created-by' to be 'test', got '%s'", config.Metadata.Annotations["created-by"])
	}

	if config.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL != "https://identity.example.com/v3" {
		t.Errorf("Expected OpenStack auth URL 'https://identity.example.com/v3', got '%s'", config.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL)
	}
}

// TestBuilderReturnsBuilderInstance verifies that each method returns a ConfigBuilder instance.
func TestBuilderReturnsBuilderInstance(t *testing.T) {
	builder := NewConfigBuilder("test-cluster")

	// Test each method individually to ensure it returns a ConfigBuilder
	tests := []struct {
		name   string
		method func(ConfigBuilder) ConfigBuilder
	}{
		{"WithProvider", func(b ConfigBuilder) ConfigBuilder { return b.WithProvider("openstack") }},
		{"WithOrganization", func(b ConfigBuilder) ConfigBuilder { return b.WithOrganization("test-org") }},
		{"WithClusterName", func(b ConfigBuilder) ConfigBuilder { return b.WithClusterName("new-name") }},
		{"WithEnvironment", func(b ConfigBuilder) ConfigBuilder { return b.WithEnvironment("prod") }},
		{"WithRegion", func(b ConfigBuilder) ConfigBuilder { return b.WithRegion("us-west-1") }},
		{"WithKubernetesVersion", func(b ConfigBuilder) ConfigBuilder { return b.WithKubernetesVersion("1.33.0") }},
		{"WithNodeCounts", func(b ConfigBuilder) ConfigBuilder { return b.WithNodeCounts(3, 2) }},
		{"WithMasterCount", func(b ConfigBuilder) ConfigBuilder { return b.WithMasterCount(3) }},
		{"WithWorkerCount", func(b ConfigBuilder) ConfigBuilder { return b.WithWorkerCount(2) }},
		{"WithWindowsWorkerCount", func(b ConfigBuilder) ConfigBuilder { return b.WithWindowsWorkerCount(1) }},
		{"WithSubnetNodes", func(b ConfigBuilder) ConfigBuilder { return b.WithSubnetNodes("10.0.0.0/24") }},
		{"WithSubnetPods", func(b ConfigBuilder) ConfigBuilder { return b.WithSubnetPods("10.42.0.0/16") }},
		{"WithSubnetServices", func(b ConfigBuilder) ConfigBuilder { return b.WithSubnetServices("10.43.0.0/16") }},
		{"WithDNSNameservers", func(b ConfigBuilder) ConfigBuilder { return b.WithDNSNameservers([]string{"8.8.8.8"}) }},
		{"WithNTPServers", func(b ConfigBuilder) ConfigBuilder { return b.WithNTPServers([]string{"time.google.com"}) }},
		{"WithSSHUser", func(b ConfigBuilder) ConfigBuilder { return b.WithSSHUser("ubuntu") }},
		{"WithSSHAuthorizedKeys", func(b ConfigBuilder) ConfigBuilder { return b.WithSSHAuthorizedKeys([]string{"ssh-rsa ..."}) }},
		{"WithBaseDomain", func(b ConfigBuilder) ConfigBuilder { return b.WithBaseDomain("example.com") }},
		{"WithAdminEmail", func(b ConfigBuilder) ConfigBuilder { return b.WithAdminEmail("admin@example.com") }},
		{"WithServices", func(b ConfigBuilder) ConfigBuilder { return b.WithServices("calico", "cert-manager") }},
		{"WithService", func(b ConfigBuilder) ConfigBuilder { return b.WithService("calico", true) }},
		{"WithSecretsBackend", func(b ConfigBuilder) ConfigBuilder { return b.WithSecretsBackend("barbican") }},
		{"WithGitURL", func(b ConfigBuilder) ConfigBuilder { return b.WithGitURL("https://github.com/example/repo") }},
		{"WithGitBranch", func(b ConfigBuilder) ConfigBuilder { return b.WithGitBranch("main") }},
		{"WithDefaultStorageClass", func(b ConfigBuilder) ConfigBuilder { return b.WithDefaultStorageClass("standard") }},
		{"WithK8sHardening", func(b ConfigBuilder) ConfigBuilder { return b.WithK8sHardening(true) }},
		{"WithOSHardening", func(b ConfigBuilder) ConfigBuilder { return b.WithOSHardening(true) }},
		{"WithTalosEnabled", func(b ConfigBuilder) ConfigBuilder { return b.WithTalosEnabled(false) }},
		{"WithOverride", func(b ConfigBuilder) ConfigBuilder { return b.WithOverride("test.path", "value") }},
		{"WithTag", func(b ConfigBuilder) ConfigBuilder { return b.WithTag("key", "value") }},
		{"WithAnnotation", func(b ConfigBuilder) ConfigBuilder { return b.WithAnnotation("key", "value") }},
		{"WhenProvider", func(b ConfigBuilder) ConfigBuilder {
			return b.WhenProvider("openstack", func(b2 ConfigBuilder) ConfigBuilder { return b2 })
		}},
		{"WhenProviderIn", func(b ConfigBuilder) ConfigBuilder {
			return b.WhenProviderIn([]string{"openstack"}, func(b2 ConfigBuilder) ConfigBuilder { return b2 })
		}},
		{"WhenNotProvider", func(b ConfigBuilder) ConfigBuilder {
			return b.WhenNotProvider("kind", func(b2 ConfigBuilder) ConfigBuilder { return b2 })
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method(builder)
			if result == nil {
				t.Errorf("%s returned nil instead of ConfigBuilder", tt.name)
			}
		})
	}
}

// TestBuilderValidation tests that the builder validates configurations correctly.
func TestBuilderValidation(t *testing.T) {
	tests := []struct {
		name          string
		setupBuilder  func(ConfigBuilder) ConfigBuilder
		expectErrors  bool
		errorContains string
	}{
		{
			name: "valid configuration",
			setupBuilder: func(b ConfigBuilder) ConfigBuilder {
				return b.WithOrganization("test-org").
					WithProvider("openstack").
					WithMasterCount(3).
					WithWorkerCount(2).
					WithOpenStackConfig(SimplifiedOpenStackCloud{
						AuthURL:    "https://identity.example.com/v3",
						Region:     "us-east-1",
						TenantName: "test-tenant",
					})
			},
			expectErrors: false,
		},
		{
			name: "missing organization",
			setupBuilder: func(b ConfigBuilder) ConfigBuilder {
				return b.WithOrganization("").
					WithProvider("openstack")
			},
			expectErrors:  true,
			errorContains: "organization is required",
		},
		{
			name: "missing provider",
			setupBuilder: func(b ConfigBuilder) ConfigBuilder {
				return b.WithOrganization("test-org").
					WithProvider("")
			},
			expectErrors:  true,
			errorContains: "provider is required",
		},
		{
			name: "invalid master count",
			setupBuilder: func(b ConfigBuilder) ConfigBuilder {
				return b.WithOrganization("test-org").
					WithProvider("openstack").
					WithMasterCount(0)
			},
			expectErrors:  true,
			errorContains: "master count must be at least 1",
		},
		{
			name: "even master count",
			setupBuilder: func(b ConfigBuilder) ConfigBuilder {
				return b.WithOrganization("test-org").
					WithProvider("openstack").
					WithMasterCount(2)
			},
			expectErrors:  true,
			errorContains: "master count should be odd",
		},
		{
			name: "negative worker count",
			setupBuilder: func(b ConfigBuilder) ConfigBuilder {
				return b.WithOrganization("test-org").
					WithProvider("openstack").
					WithWorkerCount(-1)
			},
			expectErrors:  true,
			errorContains: "worker count cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewConfigBuilder("test-cluster")
			builder = tt.setupBuilder(builder)

			errors := builder.Validate()

			if tt.expectErrors && len(errors) == 0 {
				t.Error("Expected validation errors but got none")
			}

			if !tt.expectErrors && len(errors) > 0 {
				t.Errorf("Expected no validation errors but got: %v", errors)
			}

			if tt.expectErrors && tt.errorContains != "" {
				found := false
				for _, err := range errors {
					if strings.Contains(err.Message, tt.errorContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing '%s' but got: %v", tt.errorContains, errors)
				}
			}
		})
	}
}

// TestBuilderFromExistingConfig tests creating a builder from an existing configuration.
func TestBuilderFromExistingConfig(t *testing.T) {
	// Create an initial configuration
	originalConfig := defaultConfig("original-cluster")
	originalConfig.OpenCenter.Meta.Organization = "original-org"
	originalConfig.OpenCenter.Infrastructure.Provider = "aws"
	originalConfig.OpenCenter.Infrastructure.Cloud.AWS.Region = "us-east-1"

	// Create a builder from the existing config
	builder := NewConfigBuilderFromConfig(originalConfig)

	// Modify the configuration
	builder = builder.
		WithClusterName("modified-cluster").
		WithOrganization("modified-org").
		WithProvider("aws").
		WithAWSConfig(SimplifiedAWSCloud{
			Region: "us-west-2",
		})

	// Build the new configuration
	newConfig, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	// Verify the modifications
	if newConfig.OpenCenter.Meta.Name != "modified-cluster" {
		t.Errorf("Expected cluster name 'modified-cluster', got '%s'", newConfig.OpenCenter.Meta.Name)
	}

	if newConfig.OpenCenter.Meta.Organization != "modified-org" {
		t.Errorf("Expected organization 'modified-org', got '%s'", newConfig.OpenCenter.Meta.Organization)
	}

	if newConfig.OpenCenter.Infrastructure.Provider != "aws" {
		t.Errorf("Expected provider 'aws', got '%s'", newConfig.OpenCenter.Infrastructure.Provider)
	}

	if newConfig.OpenCenter.Infrastructure.Cloud.AWS.Region != "us-west-2" {
		t.Errorf("Expected AWS region 'us-west-2', got '%s'", newConfig.OpenCenter.Infrastructure.Cloud.AWS.Region)
	}
}

// TestBuilderMetadataTimestamps tests that metadata timestamps are set correctly.
func TestBuilderMetadataTimestamps(t *testing.T) {
	builder := NewConfigBuilder("test-cluster").
		WithOrganization("test-org").
		WithProvider("aws").
		WithAWSConfig(SimplifiedAWSCloud{
			Region: "us-east-1",
		})

	config, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	// Verify timestamps are set
	if config.Metadata.CreatedAt.IsZero() {
		t.Error("CreatedAt timestamp should be set")
	}

	if config.Metadata.UpdatedAt.IsZero() {
		t.Error("UpdatedAt timestamp should be set")
	}

	// Verify UpdatedAt is not before CreatedAt
	if config.Metadata.UpdatedAt.Before(config.Metadata.CreatedAt) {
		t.Error("UpdatedAt should not be before CreatedAt")
	}
}

// TestTypeSafePathMethods tests the type-safe path methods for compile-time validation.
func TestTypeSafePathMethods(t *testing.T) {
	builder := NewConfigBuilder("test-cluster").
		WithPath(TypedConfigPaths.Organization, "typed-org").
		WithPath(TypedConfigPaths.Provider, "openstack").
		WithPath(TypedConfigPaths.Environment, "staging").
		WithPathInt(TypedConfigPaths.MasterCount, 5).
		WithPathInt(TypedConfigPaths.WorkerCount, 8).
		WithPathBool(TypedConfigPaths.K8sHardening, true).
		WithPathBool(TypedConfigPaths.OSHardening, false).
		WithPathStringSlice(TypedConfigPaths.DNSNameservers, []string{"8.8.8.8", "8.8.4.4"}).
		WithPathStringSlice(TypedConfigPaths.NTPServers, []string{"time.google.com", "time.cloudflare.com"})

	// Verify the builder is not nil
	if builder == nil {
		t.Fatal("Builder should not be nil")
	}

	// Access the internal config to verify overrides
	fluentBuilder, ok := builder.(*FluentConfigBuilder)
	if !ok {
		t.Fatal("Builder should be a FluentConfigBuilder")
	}

	// Verify string values
	if val, exists := fluentBuilder.config.Overrides[TypedConfigPaths.Organization.Path()]; !exists {
		t.Error("Organization override should exist")
	} else if strVal, ok := val.(string); !ok || strVal != "typed-org" {
		t.Errorf("Expected organization 'typed-org', got %v", val)
	}

	if val, exists := fluentBuilder.config.Overrides[TypedConfigPaths.Provider.Path()]; !exists {
		t.Error("Provider override should exist")
	} else if strVal, ok := val.(string); !ok || strVal != "openstack" {
		t.Errorf("Expected provider 'openstack', got %v", val)
	}

	if val, exists := fluentBuilder.config.Overrides[TypedConfigPaths.Environment.Path()]; !exists {
		t.Error("Environment override should exist")
	} else if strVal, ok := val.(string); !ok || strVal != "staging" {
		t.Errorf("Expected environment 'staging', got %v", val)
	}

	// Verify int values
	if val, exists := fluentBuilder.config.Overrides[TypedConfigPaths.MasterCount.Path()]; !exists {
		t.Error("MasterCount override should exist")
	} else if intVal, ok := val.(int); !ok || intVal != 5 {
		t.Errorf("Expected master count 5, got %v", val)
	}

	if val, exists := fluentBuilder.config.Overrides[TypedConfigPaths.WorkerCount.Path()]; !exists {
		t.Error("WorkerCount override should exist")
	} else if intVal, ok := val.(int); !ok || intVal != 8 {
		t.Errorf("Expected worker count 8, got %v", val)
	}

	// Verify bool values
	if val, exists := fluentBuilder.config.Overrides[TypedConfigPaths.K8sHardening.Path()]; !exists {
		t.Error("K8sHardening override should exist")
	} else if boolVal, ok := val.(bool); !ok || !boolVal {
		t.Errorf("Expected K8s hardening true, got %v", val)
	}

	if val, exists := fluentBuilder.config.Overrides[TypedConfigPaths.OSHardening.Path()]; !exists {
		t.Error("OSHardening override should exist")
	} else if boolVal, ok := val.(bool); !ok || boolVal {
		t.Errorf("Expected OS hardening false, got %v", val)
	}

	// Verify string slice values
	if val, exists := fluentBuilder.config.Overrides[TypedConfigPaths.DNSNameservers.Path()]; !exists {
		t.Error("DNSNameservers override should exist")
	} else if sliceVal, ok := val.([]string); !ok {
		t.Errorf("Expected DNS nameservers to be []string, got %T", val)
	} else if len(sliceVal) != 2 || sliceVal[0] != "8.8.8.8" || sliceVal[1] != "8.8.4.4" {
		t.Errorf("Expected DNS nameservers ['8.8.8.8', '8.8.4.4'], got %v", sliceVal)
	}

	if val, exists := fluentBuilder.config.Overrides[TypedConfigPaths.NTPServers.Path()]; !exists {
		t.Error("NTPServers override should exist")
	} else if sliceVal, ok := val.([]string); !ok {
		t.Errorf("Expected NTP servers to be []string, got %T", val)
	} else if len(sliceVal) != 2 || sliceVal[0] != "time.google.com" || sliceVal[1] != "time.cloudflare.com" {
		t.Errorf("Expected NTP servers ['time.google.com', 'time.cloudflare.com'], got %v", sliceVal)
	}
}

// TestTypeSafePathsPreventCompileTimeErrors demonstrates that type-safe paths
// prevent common errors at compile time rather than runtime.
func TestTypeSafePathsPreventCompileTimeErrors(t *testing.T) {
	// This test documents the compile-time safety provided by typed paths.
	// The following code would NOT compile, demonstrating type safety:

	// Example 1: Wrong type for string path
	// builder.WithPath(TypedConfigPaths.Organization, 123)  // Compile error: cannot use 123 (type int) as type string

	// Example 2: Wrong type for int path
	// builder.WithPathInt(TypedConfigPaths.MasterCount, "3")  // Compile error: cannot use "3" (type string) as type int

	// Example 3: Wrong method for path type
	// builder.WithPath(TypedConfigPaths.MasterCount, "3")  // Compile error: TypedConfigPaths.MasterCount is ConfigPath[int], not ConfigPath[string]

	// Example 4: Wrong type for bool path
	// builder.WithPathBool(TypedConfigPaths.K8sHardening, "true")  // Compile error: cannot use "true" (type string) as type bool

	// Example 5: Wrong type for slice path
	// builder.WithPathStringSlice(TypedConfigPaths.DNSNameservers, "8.8.8.8")  // Compile error: cannot use "8.8.8.8" (type string) as type []string

	// This test passes because the above errors are caught at compile time,
	// not runtime. The type system prevents these errors from ever occurring.
	t.Log("Type-safe paths prevent compile-time errors")
}

// TestComparisonTypeSafeVsUntypedPaths compares the safety of typed vs untyped paths.
func TestComparisonTypeSafeVsUntypedPaths(t *testing.T) {
	// Type-safe approach - errors caught at compile time
	typeSafeBuilder := NewConfigBuilder("test-cluster").
		WithPath(TypedConfigPaths.Organization, "safe-org").
		WithPathInt(TypedConfigPaths.MasterCount, 3).
		WithPathBool(TypedConfigPaths.K8sHardening, true)

	// Untyped approach - errors only caught at runtime (if at all)
	untypedBuilder := NewConfigBuilder("test-cluster").
		WithOverride("opencenter.meta.organization", "unsafe-org").
		WithOverride("opencenter.cluster.kubernetes.master_count", 3).
		WithOverride("security.k8s_hardening", true)

	// Both should work for valid inputs
	if typeSafeBuilder == nil || untypedBuilder == nil {
		t.Fatal("Builders should not be nil")
	}

	// The untyped approach allows errors that would be caught at compile time with typed paths:
	// untypedBuilder.WithOverride("opencenter.meta.organization", 123)  // No compile error, but wrong type
	// untypedBuilder.WithOverride("opencenter.cluster.kubernetes.master_count", "3")  // No compile error, but wrong type
	// untypedBuilder.WithOverride("typo.in.path", "value")  // No compile error, but invalid path

	t.Log("Type-safe paths provide compile-time validation that untyped paths cannot")
}

// TestTypeSafePathMethodChaining verifies that type-safe methods support fluent chaining.
func TestTypeSafePathMethodChaining(t *testing.T) {
	// Verify that type-safe methods can be chained with regular methods
	builder := NewConfigBuilder("test-cluster").
		WithOrganization("test-org").
		WithProvider("openstack").
		WithPath(TypedConfigPaths.Environment, "production").
		WithMasterCount(3).
		WithPathInt(TypedConfigPaths.WorkerCount, 5).
		WithK8sHardening(true).
		WithPathBool(TypedConfigPaths.OSHardening, false).
		WithDNSNameservers([]string{"8.8.8.8"}).
		WithPathStringSlice(TypedConfigPaths.NTPServers, []string{"time.google.com"})

	if builder == nil {
		t.Fatal("Builder should not be nil after mixed method chaining")
	}

	// Verify that both typed and untyped methods work together
	fluentBuilder := builder.(*FluentConfigBuilder)

	// Check regular method results
	if fluentBuilder.config.OpenCenter.Meta.Organization != "test-org" {
		t.Errorf("Expected organization 'test-org', got '%s'", fluentBuilder.config.OpenCenter.Meta.Organization)
	}

	if fluentBuilder.config.OpenCenter.Infrastructure.Provider != "openstack" {
		t.Errorf("Expected provider 'openstack', got '%s'", fluentBuilder.config.OpenCenter.Infrastructure.Provider)
	}

	// Check type-safe method results in overrides
	if val, exists := fluentBuilder.config.Overrides[TypedConfigPaths.Environment.Path()]; !exists {
		t.Error("Environment override should exist")
	} else if strVal, ok := val.(string); !ok || strVal != "production" {
		t.Errorf("Expected environment 'production', got %v", val)
	}
}

// TestConditionalConfigurationWhenProvider tests provider-specific conditional configuration.
func TestConditionalConfigurationWhenProvider(t *testing.T) {
	tests := []struct {
		name                string
		provider            string
		expectOpenStackAuth bool
		expectAWSRegion     bool
		expectHardening     bool
	}{
		{
			name:                "openstack provider applies openstack config",
			provider:            "openstack",
			expectOpenStackAuth: true,
			expectAWSRegion:     false,
			expectHardening:     false,
		},
		{
			name:                "aws provider applies aws config",
			provider:            "aws",
			expectOpenStackAuth: false,
			expectAWSRegion:     true,
			expectHardening:     false,
		},
		{
			name:                "kind provider applies neither cloud config",
			provider:            "kind",
			expectOpenStackAuth: false,
			expectAWSRegion:     false,
			expectHardening:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewConfigBuilder("test-cluster").
				WithOrganization("test-org").
				WithProvider(tt.provider).
				WhenProvider("openstack", func(b ConfigBuilder) ConfigBuilder {
					return b.WithOpenStackConfig(SimplifiedOpenStackCloud{
						AuthURL:    "https://identity.example.com/v3",
						Region:     "us-east-1",
						TenantName: "test-tenant",
					})
				}).
				WhenProvider("aws", func(b ConfigBuilder) ConfigBuilder {
					return b.WithAWSConfig(SimplifiedAWSCloud{
						Region: "us-west-2",
					})
				}).
				WhenProvider("kind", func(b ConfigBuilder) ConfigBuilder {
					return b.WithK8sHardening(true).WithOSHardening(true)
				})

			// Access the internal config
			fluentBuilder, ok := builder.(*FluentConfigBuilder)
			if !ok {
				t.Fatal("Builder should be a FluentConfigBuilder")
			}

			// Verify OpenStack configuration
			if tt.expectOpenStackAuth {
				if fluentBuilder.config.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL != "https://identity.example.com/v3" {
					t.Error("Expected OpenStack auth URL to be set")
				}
			} else {
				if fluentBuilder.config.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL != "" {
					t.Error("Expected OpenStack auth URL to be empty")
				}
			}

			// Verify AWS configuration
			if tt.expectAWSRegion {
				if fluentBuilder.config.OpenCenter.Infrastructure.Cloud.AWS.Region != "us-west-2" {
					t.Error("Expected AWS region to be set")
				}
			} else {
				if fluentBuilder.config.OpenCenter.Infrastructure.Cloud.AWS.Region != "" {
					t.Error("Expected AWS region to be empty")
				}
			}

			// Verify hardening configuration
			if tt.expectHardening {
				if !fluentBuilder.config.Security.K8sHardening {
					t.Error("Expected K8s hardening to be enabled")
				}
				if !fluentBuilder.config.Security.OSHardening {
					t.Error("Expected OS hardening to be enabled")
				}
			}
		})
	}
}

// TestConditionalConfigurationWhenProviderIn tests multi-provider conditional configuration.
func TestConditionalConfigurationWhenProviderIn(t *testing.T) {
	tests := []struct {
		name            string
		provider        string
		expectHardening bool
		expectStorage   bool
	}{
		{
			name:            "openstack in cloud providers list",
			provider:        "openstack",
			expectHardening: true,
			expectStorage:   false,
		},
		{
			name:            "aws in cloud providers list",
			provider:        "aws",
			expectHardening: true,
			expectStorage:   false,
		},
		{
			name:            "baremetal in on-prem providers list",
			provider:        "baremetal",
			expectHardening: true, // Default is true, not overridden
			expectStorage:   true,
		},
		{
			name:            "kind not in any list",
			provider:        "kind",
			expectHardening: true, // Default is true, not overridden
			expectStorage:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewConfigBuilder("test-cluster").
				WithOrganization("test-org").
				WithProvider(tt.provider).
				WhenProviderIn([]string{"openstack", "aws"}, func(b ConfigBuilder) ConfigBuilder {
					return b.WithK8sHardening(true).WithOSHardening(true)
				}).
				WhenProviderIn([]string{"baremetal", "vsphere"}, func(b ConfigBuilder) ConfigBuilder {
					return b.WithDefaultStorageClass("local-path")
				})

			// Access the internal config
			fluentBuilder, ok := builder.(*FluentConfigBuilder)
			if !ok {
				t.Fatal("Builder should be a FluentConfigBuilder")
			}

			// Verify hardening configuration
			if tt.expectHardening {
				if !fluentBuilder.config.Security.K8sHardening {
					t.Error("Expected K8s hardening to be enabled")
				}
				if !fluentBuilder.config.Security.OSHardening {
					t.Error("Expected OS hardening to be enabled")
				}
			} else {
				if fluentBuilder.config.Security.K8sHardening {
					t.Error("Expected K8s hardening to be disabled")
				}
				if fluentBuilder.config.Security.OSHardening {
					t.Error("Expected OS hardening to be disabled")
				}
			}

			// Verify storage configuration
			if tt.expectStorage {
				if fluentBuilder.config.OpenCenter.Storage.DefaultStorageClass != "local-path" {
					t.Error("Expected default storage class to be 'local-path'")
				}
			} else {
				if fluentBuilder.config.OpenCenter.Storage.DefaultStorageClass == "local-path" {
					t.Error("Expected default storage class to not be 'local-path'")
				}
			}
		})
	}
}

// TestConditionalConfigurationWhenNotProvider tests negative conditional configuration.
func TestConditionalConfigurationWhenNotProvider(t *testing.T) {
	tests := []struct {
		name            string
		provider        string
		expectHardening bool
	}{
		{
			name:            "openstack gets hardening (not kind)",
			provider:        "openstack",
			expectHardening: true,
		},
		{
			name:            "aws gets hardening (not kind)",
			provider:        "aws",
			expectHardening: true,
		},
		{
			name:            "kind does not get hardening override (keeps defaults)",
			provider:        "kind",
			expectHardening: true, // Default is true, not overridden by WhenNotProvider
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewConfigBuilder("test-cluster").
				WithOrganization("test-org").
				WithProvider(tt.provider).
				WhenNotProvider("kind", func(b ConfigBuilder) ConfigBuilder {
					return b.WithK8sHardening(true).WithOSHardening(true)
				})

			// Access the internal config
			fluentBuilder, ok := builder.(*FluentConfigBuilder)
			if !ok {
				t.Fatal("Builder should be a FluentConfigBuilder")
			}

			// Verify hardening configuration
			if tt.expectHardening {
				if !fluentBuilder.config.Security.K8sHardening {
					t.Error("Expected K8s hardening to be enabled")
				}
				if !fluentBuilder.config.Security.OSHardening {
					t.Error("Expected OS hardening to be enabled")
				}
			} else {
				if fluentBuilder.config.Security.K8sHardening {
					t.Error("Expected K8s hardening to be disabled")
				}
				if fluentBuilder.config.Security.OSHardening {
					t.Error("Expected OS hardening to be disabled")
				}
			}
		})
	}
}

// TestConditionalConfigurationChaining tests that conditional methods support fluent chaining.
func TestConditionalConfigurationChaining(t *testing.T) {
	// Test complex chaining with multiple conditional configurations
	builder := NewConfigBuilder("test-cluster").
		WithOrganization("test-org").
		WithProvider("openstack").
		WithMasterCount(3).
		WithWorkerCount(5).
		WhenProvider("openstack", func(b ConfigBuilder) ConfigBuilder {
			return b.WithOpenStackConfig(SimplifiedOpenStackCloud{
				AuthURL:    "https://identity.example.com/v3",
				Region:     "us-east-1",
				TenantName: "test-tenant",
			})
		}).
		WhenProviderIn([]string{"openstack", "aws"}, func(b ConfigBuilder) ConfigBuilder {
			return b.WithK8sHardening(true)
		}).
		WhenNotProvider("kind", func(b ConfigBuilder) ConfigBuilder {
			return b.WithOSHardening(true)
		}).
		WithBaseDomain("example.com").
		WithAdminEmail("admin@example.com")

	if builder == nil {
		t.Fatal("Builder should not be nil after conditional chaining")
	}

	// Build and verify
	config, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	// Verify all configurations were applied correctly
	if config.OpenCenter.Meta.Name != "test-cluster" {
		t.Errorf("Expected cluster name 'test-cluster', got '%s'", config.OpenCenter.Meta.Name)
	}

	if config.OpenCenter.Meta.Organization != "test-org" {
		t.Errorf("Expected organization 'test-org', got '%s'", config.OpenCenter.Meta.Organization)
	}

	if config.OpenCenter.Infrastructure.Provider != "openstack" {
		t.Errorf("Expected provider 'openstack', got '%s'", config.OpenCenter.Infrastructure.Provider)
	}

	if config.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL != "https://identity.example.com/v3" {
		t.Error("Expected OpenStack auth URL to be set")
	}

	if !config.Security.K8sHardening {
		t.Error("Expected K8s hardening to be enabled")
	}

	if !config.Security.OSHardening {
		t.Error("Expected OS hardening to be enabled")
	}

	if config.OpenCenter.Cluster.BaseDomain != "example.com" {
		t.Errorf("Expected base domain 'example.com', got '%s'", config.OpenCenter.Cluster.BaseDomain)
	}
}

// TestConditionalConfigurationNestedCallbacks tests nested conditional configurations.
func TestConditionalConfigurationNestedCallbacks(t *testing.T) {
	builder := NewConfigBuilder("test-cluster").
		WithOrganization("test-org").
		WithProvider("openstack").
		WhenProvider("openstack", func(b ConfigBuilder) ConfigBuilder {
			return b.
				WithOpenStackConfig(SimplifiedOpenStackCloud{
					AuthURL:    "https://identity.example.com/v3",
					Region:     "us-east-1",
					TenantName: "test-tenant",
				}).
				WhenProviderIn([]string{"openstack", "aws"}, func(b2 ConfigBuilder) ConfigBuilder {
					return b2.WithK8sHardening(true)
				})
		})

	if builder == nil {
		t.Fatal("Builder should not be nil after nested conditional configuration")
	}

	// Access the internal config
	fluentBuilder, ok := builder.(*FluentConfigBuilder)
	if !ok {
		t.Fatal("Builder should be a FluentConfigBuilder")
	}

	// Verify both outer and inner conditionals were applied
	if fluentBuilder.config.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL != "https://identity.example.com/v3" {
		t.Error("Expected OpenStack auth URL to be set from outer conditional")
	}

	if !fluentBuilder.config.Security.K8sHardening {
		t.Error("Expected K8s hardening to be enabled from nested conditional")
	}
}

// TestConditionalConfigurationWithTypeSafePaths tests conditional configuration with type-safe paths.
func TestConditionalConfigurationWithTypeSafePaths(t *testing.T) {
	builder := NewConfigBuilder("test-cluster").
		WithOrganization("test-org").
		WithProvider("openstack").
		WhenProvider("openstack", func(b ConfigBuilder) ConfigBuilder {
			return b.
				WithPath(TypedConfigPaths.Environment, "production").
				WithPathInt(TypedConfigPaths.MasterCount, 5).
				WithPathBool(TypedConfigPaths.K8sHardening, true)
		}).
		WhenProvider("aws", func(b ConfigBuilder) ConfigBuilder {
			return b.
				WithPath(TypedConfigPaths.Environment, "staging").
				WithPathInt(TypedConfigPaths.MasterCount, 3)
		})

	// Access the internal config
	fluentBuilder, ok := builder.(*FluentConfigBuilder)
	if !ok {
		t.Fatal("Builder should be a FluentConfigBuilder")
	}

	// Verify OpenStack-specific configuration was applied
	if val, exists := fluentBuilder.config.Overrides[TypedConfigPaths.Environment.Path()]; !exists {
		t.Error("Environment override should exist")
	} else if strVal, ok := val.(string); !ok || strVal != "production" {
		t.Errorf("Expected environment 'production', got %v", val)
	}

	if val, exists := fluentBuilder.config.Overrides[TypedConfigPaths.MasterCount.Path()]; !exists {
		t.Error("MasterCount override should exist")
	} else if intVal, ok := val.(int); !ok || intVal != 5 {
		t.Errorf("Expected master count 5, got %v", val)
	}

	if val, exists := fluentBuilder.config.Overrides[TypedConfigPaths.K8sHardening.Path()]; !exists {
		t.Error("K8sHardening override should exist")
	} else if boolVal, ok := val.(bool); !ok || !boolVal {
		t.Errorf("Expected K8s hardening true, got %v", val)
	}
}

// TestConditionalConfigurationRealWorldScenario tests a realistic multi-provider scenario.
func TestConditionalConfigurationRealWorldScenario(t *testing.T) {
	// Simulate a configuration that adapts to different providers
	createClusterConfig := func(provider string) ConfigBuilder {
		return NewConfigBuilder("multi-cloud-cluster").
			WithOrganization("acme-corp").
			WithProvider(provider).
			WithKubernetesVersion("1.33.7").
			WithBaseDomain("acme.com").
			WithAdminEmail("admin@acme.com").
			// Cloud provider specific configuration
			WhenProvider("openstack", func(b ConfigBuilder) ConfigBuilder {
				return b.
					WithOpenStackConfig(SimplifiedOpenStackCloud{
						AuthURL:    "https://openstack.acme.com:5000/v3",
						Region:     "us-east-1",
						TenantName: "production",
					}).
					WithNodeCounts(5, 10).
					WithSubnetNodes("10.0.0.0/24")
			}).
			WhenProvider("aws", func(b ConfigBuilder) ConfigBuilder {
				return b.
					WithAWSConfig(SimplifiedAWSCloud{
						Region: "us-west-2",
					}).
					WithNodeCounts(3, 6).
					WithSubnetNodes("172.16.0.0/24")
			}).
			// Production hardening for cloud providers
			WhenProviderIn([]string{"openstack", "aws"}, func(b ConfigBuilder) ConfigBuilder {
				return b.
					WithK8sHardening(true).
					WithOSHardening(true).
					WithTag("environment", "production").
					WithTag("managed-by", "openCenter")
			}).
			// Development settings for local clusters
			WhenProvider("kind", func(b ConfigBuilder) ConfigBuilder {
				return b.
					WithNodeCounts(1, 2).
					WithK8sHardening(false).
					WithOSHardening(false).
					WithTag("environment", "development")
			}).
			// Disable hardening for local development
			WhenNotProvider("kind", func(b ConfigBuilder) ConfigBuilder {
				return b.WithServices("cert-manager", "prometheus-stack")
			})
	}

	// Test OpenStack configuration
	t.Run("openstack configuration", func(t *testing.T) {
		builder := createClusterConfig("openstack")
		config, err := builder.Build()
		if err != nil {
			t.Fatalf("Build() failed: %v", err)
		}

		if config.OpenCenter.Infrastructure.Provider != "openstack" {
			t.Error("Expected provider to be openstack")
		}

		if config.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL != "https://openstack.acme.com:5000/v3" {
			t.Error("Expected OpenStack auth URL to be set")
		}

		if config.OpenCenter.Cluster.Kubernetes.MasterCount != 5 {
			t.Errorf("Expected 5 masters, got %d", config.OpenCenter.Cluster.Kubernetes.MasterCount)
		}

		if !config.Security.K8sHardening || !config.Security.OSHardening {
			t.Error("Expected hardening to be enabled for production")
		}

		if config.Metadata.Tags["environment"] != "production" {
			t.Error("Expected production environment tag")
		}
	})

	// Test AWS configuration
	t.Run("aws configuration", func(t *testing.T) {
		builder := createClusterConfig("aws")
		config, err := builder.Build()
		if err != nil {
			t.Fatalf("Build() failed: %v", err)
		}

		if config.OpenCenter.Infrastructure.Provider != "aws" {
			t.Error("Expected provider to be aws")
		}

		if config.OpenCenter.Infrastructure.Cloud.AWS.Region != "us-west-2" {
			t.Error("Expected AWS region to be set")
		}

		if config.OpenCenter.Cluster.Kubernetes.MasterCount != 3 {
			t.Errorf("Expected 3 masters, got %d", config.OpenCenter.Cluster.Kubernetes.MasterCount)
		}

		if !config.Security.K8sHardening || !config.Security.OSHardening {
			t.Error("Expected hardening to be enabled for production")
		}
	})

	// Test Kind configuration
	t.Run("kind configuration", func(t *testing.T) {
		builder := createClusterConfig("kind")

		// Access the internal config to check before Build (which will fail validation)
		fluentBuilder, ok := builder.(*FluentConfigBuilder)
		if !ok {
			t.Fatal("Builder should be a FluentConfigBuilder")
		}

		if fluentBuilder.config.OpenCenter.Infrastructure.Provider != "kind" {
			t.Error("Expected provider to be kind")
		}

		if fluentBuilder.config.OpenCenter.Cluster.Kubernetes.MasterCount != 1 {
			t.Errorf("Expected 1 master, got %d", fluentBuilder.config.OpenCenter.Cluster.Kubernetes.MasterCount)
		}

		if fluentBuilder.config.Security.K8sHardening || fluentBuilder.config.Security.OSHardening {
			t.Error("Expected hardening to be disabled for development")
		}

		if fluentBuilder.config.Metadata.Tags["environment"] != "development" {
			t.Error("Expected development environment tag")
		}
	})
}

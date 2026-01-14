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
	"testing"
)

// TestConfigPathsExist verifies that all ConfigPaths constants are properly defined.
func TestConfigPathsExist(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		expectedPath string
	}{
		{"Organization", TypedConfigPaths.Organization.Path(), "opencenter.meta.organization"},
		{"ClusterName", TypedConfigPaths.ClusterName.Path(), "opencenter.meta.name"},
		{"Environment", TypedConfigPaths.Environment.Path(), "opencenter.meta.env"},
		{"Region", TypedConfigPaths.Region.Path(), "opencenter.meta.region"},
		{"Provider", TypedConfigPaths.Provider.Path(), "opencenter.infrastructure.provider"},
		{"SSHUser", TypedConfigPaths.SSHUser.Path(), "opencenter.infrastructure.ssh_user"},
		{"KubernetesVersion", TypedConfigPaths.KubernetesVersion.Path(), "opencenter.cluster.kubernetes.version"},
		{"MasterCount", TypedConfigPaths.MasterCount.Path(), "opencenter.cluster.kubernetes.master_count"},
		{"WorkerCount", TypedConfigPaths.WorkerCount.Path(), "opencenter.cluster.kubernetes.worker_count"},
		{"WindowsWorkerCount", TypedConfigPaths.WindowsWorkerCount.Path(), "opencenter.cluster.kubernetes.worker_count_windows"},
		{"SubnetPods", TypedConfigPaths.SubnetPods.Path(), "opencenter.cluster.kubernetes.subnet_pods"},
		{"SubnetServices", TypedConfigPaths.SubnetServices.Path(), "opencenter.cluster.kubernetes.subnet_services"},
		{"SubnetNodes", TypedConfigPaths.SubnetNodes.Path(), "networking.subnet_nodes"},
		{"DNSNameservers", TypedConfigPaths.DNSNameservers.Path(), "networking.dns_nameservers"},
		{"NTPServers", TypedConfigPaths.NTPServers.Path(), "networking.ntp_servers"},
		{"BaseDomain", TypedConfigPaths.BaseDomain.Path(), "opencenter.cluster.base_domain"},
		{"AdminEmail", TypedConfigPaths.AdminEmail.Path(), "opencenter.cluster.admin_email"},
		{"SSHAuthorizedKeys", TypedConfigPaths.SSHAuthorizedKeys.Path(), "opencenter.cluster.ssh_authorized_keys"},
		{"K8sHardening", TypedConfigPaths.K8sHardening.Path(), "security.k8s_hardening"},
		{"OSHardening", TypedConfigPaths.OSHardening.Path(), "security.os_hardening"},
		{"DefaultStorageClass", TypedConfigPaths.DefaultStorageClass.Path(), "opencenter.storage.default_storage_class"},
		{"GitURL", TypedConfigPaths.GitURL.Path(), "opencenter.gitops.git_url"},
		{"GitBranch", TypedConfigPaths.GitBranch.Path(), "opencenter.gitops.git_branch"},
		{"SecretsBackend", TypedConfigPaths.SecretsBackend.Path(), "opencenter.secrets.backend"},
		{"OpenStackAuthURL", TypedConfigPaths.OpenStackAuthURL.Path(), "opencenter.infrastructure.cloud.openstack.auth_url"},
		{"OpenStackRegion", TypedConfigPaths.OpenStackRegion.Path(), "opencenter.infrastructure.cloud.openstack.region"},
		{"OpenStackTenantName", TypedConfigPaths.OpenStackTenantName.Path(), "opencenter.infrastructure.cloud.openstack.tenant_name"},
		{"AWSRegion", TypedConfigPaths.AWSRegion.Path(), "opencenter.infrastructure.cloud.aws.region"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.path != tt.expectedPath {
				t.Errorf("Expected path '%s', got '%s'", tt.expectedPath, tt.path)
			}
		})
	}
}

// TestConfigPathTypeSystem verifies that the type system prevents incorrect type usage at compile time.
// This test demonstrates that the type system works correctly by showing valid usage patterns.
func TestConfigPathTypeSystem(t *testing.T) {
	// These should compile successfully because types match
	
	// String paths
	var _ TypedConfigPath[string] = TypedConfigPaths.Organization
	var _ TypedConfigPath[string] = TypedConfigPaths.ClusterName
	var _ TypedConfigPath[string] = TypedConfigPaths.Provider
	
	// Int paths
	var _ TypedConfigPath[int] = TypedConfigPaths.MasterCount
	var _ TypedConfigPath[int] = TypedConfigPaths.WorkerCount
	var _ TypedConfigPath[int] = TypedConfigPaths.WindowsWorkerCount
	
	// Bool paths
	var _ TypedConfigPath[bool] = TypedConfigPaths.K8sHardening
	var _ TypedConfigPath[bool] = TypedConfigPaths.OSHardening
	
	// String slice paths
	var _ TypedConfigPath[[]string] = TypedConfigPaths.DNSNameservers
	var _ TypedConfigPath[[]string] = TypedConfigPaths.NTPServers
	var _ TypedConfigPath[[]string] = TypedConfigPaths.SSHAuthorizedKeys
	
	// The following would NOT compile (demonstrating compile-time type safety):
	// var _ TypedConfigPath[int] = TypedConfigPaths.Organization  // Type mismatch: string vs int
	// var _ TypedConfigPath[bool] = TypedConfigPaths.MasterCount  // Type mismatch: int vs bool
	// var _ TypedConfigPath[string] = TypedConfigPaths.K8sHardening  // Type mismatch: bool vs string
}

// TestConfigPathUsageWithBuilder demonstrates type-safe path usage with the builder.
func TestConfigPathUsageWithBuilder(t *testing.T) {
	builder := NewConfigBuilder("test-cluster")
	
	// These should compile and work correctly
	builder = builder.
		WithPath(TypedConfigPaths.Organization, "test-org").
		WithPath(TypedConfigPaths.Provider, "openstack").
		WithPathInt(TypedConfigPaths.MasterCount, 3).
		WithPathInt(TypedConfigPaths.WorkerCount, 5).
		WithPathBool(TypedConfigPaths.K8sHardening, true).
		WithPathBool(TypedConfigPaths.OSHardening, true).
		WithPathStringSlice(TypedConfigPaths.DNSNameservers, []string{"8.8.8.8", "8.8.4.4"})
	
	// The following would NOT compile (demonstrating compile-time type safety):
	// builder.WithPath(TypedConfigPaths.MasterCount, "3")  // Type error: expects int, got string
	// builder.WithPathInt(TypedConfigPaths.Organization, 3)  // Type error: expects string path, got int path
	// builder.WithPathBool(TypedConfigPaths.Provider, true)  // Type error: expects string path, got bool path
	
	if builder == nil {
		t.Fatal("Builder should not be nil")
	}
}

// TestTypeSafePathsInOverrides verifies that type-safe paths are stored correctly in overrides.
func TestTypeSafePathsInOverrides(t *testing.T) {
	builder := NewConfigBuilder("test-cluster").
		WithPath(TypedConfigPaths.Organization, "typed-org").
		WithPathInt(TypedConfigPaths.MasterCount, 7).
		WithPathBool(TypedConfigPaths.K8sHardening, true).
		WithPathStringSlice(TypedConfigPaths.DNSNameservers, []string{"1.1.1.1"})
	
	// Access the internal config to verify overrides
	fluentBuilder, ok := builder.(*FluentConfigBuilder)
	if !ok {
		t.Fatal("Builder should be a FluentConfigBuilder")
	}
	
	// Verify string path
	if val, exists := fluentBuilder.config.Overrides[TypedConfigPaths.Organization.Path()]; !exists {
		t.Error("Organization override should exist")
	} else if strVal, ok := val.(string); !ok || strVal != "typed-org" {
		t.Errorf("Expected organization 'typed-org', got %v", val)
	}
	
	// Verify int path
	if val, exists := fluentBuilder.config.Overrides[TypedConfigPaths.MasterCount.Path()]; !exists {
		t.Error("MasterCount override should exist")
	} else if intVal, ok := val.(int); !ok || intVal != 7 {
		t.Errorf("Expected master count 7, got %v", val)
	}
	
	// Verify bool path
	if val, exists := fluentBuilder.config.Overrides[TypedConfigPaths.K8sHardening.Path()]; !exists {
		t.Error("K8sHardening override should exist")
	} else if boolVal, ok := val.(bool); !ok || !boolVal {
		t.Errorf("Expected K8s hardening true, got %v", val)
	}
	
	// Verify string slice path
	if val, exists := fluentBuilder.config.Overrides[TypedConfigPaths.DNSNameservers.Path()]; !exists {
		t.Error("DNSNameservers override should exist")
	} else if sliceVal, ok := val.([]string); !ok || len(sliceVal) != 1 || sliceVal[0] != "1.1.1.1" {
		t.Errorf("Expected DNS nameservers ['1.1.1.1'], got %v", val)
	}
}

// TestTypeSafeVsUntypedPaths compares type-safe and untyped path usage.
func TestTypeSafeVsUntypedPaths(t *testing.T) {
	// Type-safe approach (compile-time validation)
	typeSafeBuilder := NewConfigBuilder("test-cluster").
		WithPath(TypedConfigPaths.Organization, "test-org").
		WithPathInt(TypedConfigPaths.MasterCount, 3)
	
	// Untyped approach (runtime validation only)
	untypedBuilder := NewConfigBuilder("test-cluster").
		WithOverride("opencenter.meta.organization", "test-org").
		WithOverride("opencenter.cluster.kubernetes.master_count", 3)
	
	// Both should work, but type-safe approach prevents errors at compile time
	if typeSafeBuilder == nil || untypedBuilder == nil {
		t.Fatal("Builders should not be nil")
	}
	
	// Verify both produce the same result
	typeSafeConfig := typeSafeBuilder.(*FluentConfigBuilder).config
	untypedConfig := untypedBuilder.(*FluentConfigBuilder).config
	
	if typeSafeConfig.Overrides[TypedConfigPaths.Organization.Path()] != untypedConfig.Overrides["opencenter.meta.organization"] {
		t.Errorf("Type-safe and untyped approaches should produce the same result, got %v vs %v",
			typeSafeConfig.Overrides[TypedConfigPaths.Organization.Path()],
			untypedConfig.Overrides["opencenter.meta.organization"])
	}
	
	if typeSafeConfig.Overrides[TypedConfigPaths.MasterCount.Path()] != untypedConfig.Overrides["opencenter.cluster.kubernetes.master_count"] {
		t.Errorf("Type-safe and untyped approaches should produce the same result for master count, got %v vs %v",
			typeSafeConfig.Overrides[TypedConfigPaths.MasterCount.Path()],
			untypedConfig.Overrides["opencenter.cluster.kubernetes.master_count"])
	}
}

// TestMethodChainingWithTypeSafePaths verifies that type-safe methods support method chaining.
func TestMethodChainingWithTypeSafePaths(t *testing.T) {
	// This should compile and work correctly
	builder := NewConfigBuilder("test-cluster").
		WithPath(TypedConfigPaths.Organization, "chain-org").
		WithPath(TypedConfigPaths.Provider, "openstack").
		WithPath(TypedConfigPaths.Environment, "prod").
		WithPathInt(TypedConfigPaths.MasterCount, 5).
		WithPathInt(TypedConfigPaths.WorkerCount, 10).
		WithPathBool(TypedConfigPaths.K8sHardening, true).
		WithPathBool(TypedConfigPaths.OSHardening, false).
		WithPathStringSlice(TypedConfigPaths.DNSNameservers, []string{"8.8.8.8"}).
		WithPathStringSlice(TypedConfigPaths.NTPServers, []string{"time.google.com"})
	
	if builder == nil {
		t.Fatal("Builder should not be nil after method chaining")
	}
	
	// Verify all values were set
	fluentBuilder := builder.(*FluentConfigBuilder)
	
	tests := []struct {
		path     string
		expected interface{}
	}{
		{TypedConfigPaths.Organization.Path(), "chain-org"},
		{TypedConfigPaths.Provider.Path(), "openstack"},
		{TypedConfigPaths.Environment.Path(), "prod"},
		{TypedConfigPaths.MasterCount.Path(), 5},
		{TypedConfigPaths.WorkerCount.Path(), 10},
		{TypedConfigPaths.K8sHardening.Path(), true},
		{TypedConfigPaths.OSHardening.Path(), false},
	}
	
	for _, tt := range tests {
		if val, exists := fluentBuilder.config.Overrides[tt.path]; !exists {
			t.Errorf("Override for path '%s' should exist", tt.path)
		} else if val != tt.expected {
			t.Errorf("Expected value %v for path '%s', got %v", tt.expected, tt.path, val)
		}
	}
}

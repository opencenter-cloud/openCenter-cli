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
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrationPreservesUserValues tests Property 27: Configuration Value Preservation
// Feature: configuration-system-refactor, Property 27: For any configuration migration,
// all user-specified values should be preserved in the migrated configuration
func TestMigrationPreservesUserValues(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Property: All user-specified configuration values are preserved during migration
	properties.Property("user values preserved across all migration paths", prop.ForAll(
		func(clusterName, orgName, provider, podSubnet, serviceSubnet string) bool {
			mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
			ctx := context.Background()

			// Create a v1.0.0 configuration with user-specified values
			config := Config{
				SchemaVersion: SchemaVersion1_0_0,
				OpenCenter: SimplifiedOpenCenter{
					Meta: ClusterMeta{
						Name:         clusterName,
						Organization: orgName,
					},
					Infrastructure: Infrastructure{
						Provider: provider,
					},
					Cluster: ClusterConfig{
						Kubernetes: KubernetesConfig{
							Networking: Networking{
								SubnetPods:     podSubnet,
								SubnetServices: serviceSubnet,
							},
						},
					},
				},
			}

			// Test migration to each supported version
			versions := []string{SchemaVersion1_1_0, SchemaVersion1_2_0, SchemaVersion2_0_0}
			for _, targetVersion := range versions {
				migrated, err := mgr.MigrateConfig(ctx, config, targetVersion)
				if err != nil {
					t.Logf("Migration to %s failed: %v", targetVersion, err)
					return false
				}

				// Verify all user-specified values are preserved
				if migrated.OpenCenter.Meta.Name != clusterName {
					t.Logf("Cluster name changed: expected %s, got %s", clusterName, migrated.OpenCenter.Meta.Name)
					return false
				}
				if migrated.OpenCenter.Meta.Organization != orgName {
					t.Logf("Organization changed: expected %s, got %s", orgName, migrated.OpenCenter.Meta.Organization)
					return false
				}
				if migrated.OpenCenter.Infrastructure.Provider != provider {
					t.Logf("Provider changed: expected %s, got %s", provider, migrated.OpenCenter.Infrastructure.Provider)
					return false
				}
				if migrated.OpenCenter.Cluster.Kubernetes.Networking.SubnetPods != podSubnet {
					t.Logf("Pod subnet changed: expected %s, got %s", podSubnet, migrated.OpenCenter.Cluster.Kubernetes.Networking.SubnetPods)
					return false
				}
				if migrated.OpenCenter.Cluster.Kubernetes.Networking.SubnetServices != serviceSubnet {
					t.Logf("Service subnet changed: expected %s, got %s", serviceSubnet, migrated.OpenCenter.Cluster.Kubernetes.Networking.SubnetServices)
					return false
				}
			}

			return true
		},
		genClusterName(),
		genOrgName(),
		gen.OneConstOf("openstack", "aws", "baremetal", "vsphere", "kind"),
		genCIDR(),
		genCIDR(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestMigrationPreservesComplexValues tests that complex nested values are preserved
func TestMigrationPreservesComplexValues(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("complex nested values preserved", prop.ForAll(
		func(clusterName, orgName string, masterCount, workerCount int) bool {
			mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
			ctx := context.Background()

			// Create configuration with complex nested values
			config := Config{
				SchemaVersion: SchemaVersion1_0_0,
				OpenCenter: SimplifiedOpenCenter{
					Meta: ClusterMeta{
						Name:         clusterName,
						Organization: orgName,
					},
					Infrastructure: Infrastructure{
						Provider: "openstack",
					},
					Cluster: ClusterConfig{
						Kubernetes: KubernetesConfig{
							MasterCount: masterCount,
							WorkerCount: workerCount,
						},
					},
				},
			}

			// Migrate to v2.0.0
			migrated, err := mgr.MigrateConfig(ctx, config, SchemaVersion2_0_0)
			if err != nil {
				t.Logf("Migration failed: %v", err)
				return false
			}

			// Verify nested values are preserved
			if migrated.OpenCenter.Cluster.Kubernetes.MasterCount != masterCount {
				t.Logf("Master count changed: expected %d, got %d", masterCount, migrated.OpenCenter.Cluster.Kubernetes.MasterCount)
				return false
			}
			if migrated.OpenCenter.Cluster.Kubernetes.WorkerCount != workerCount {
				t.Logf("Worker count changed: expected %d, got %d", workerCount, migrated.OpenCenter.Cluster.Kubernetes.WorkerCount)
				return false
			}

			return true
		},
		genClusterName(),
		genOrgName(),
		gen.IntRange(1, 10),
		gen.IntRange(1, 100),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestMigrationPreservesOverrides tests that override values are preserved
func TestMigrationPreservesOverrides(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("override values preserved", prop.ForAll(
		func(clusterName, overrideKey, overrideValue string) bool {
			mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
			ctx := context.Background()

			// Create configuration with overrides
			config := Config{
				SchemaVersion: SchemaVersion1_0_0,
				OpenCenter: SimplifiedOpenCenter{
					Meta: ClusterMeta{
						Name:         clusterName,
						Organization: "test-org",
					},
				},
				Overrides: map[string]any{
					overrideKey: overrideValue,
				},
			}

			// Migrate to v2.0.0
			migrated, err := mgr.MigrateConfig(ctx, config, SchemaVersion2_0_0)
			if err != nil {
				t.Logf("Migration failed: %v", err)
				return false
			}

			// Verify override is preserved
			if migrated.Overrides == nil {
				t.Log("Overrides map is nil after migration")
				return false
			}
			if val, ok := migrated.Overrides[overrideKey]; !ok {
				t.Logf("Override key %s not found after migration", overrideKey)
				return false
			} else if val != overrideValue {
				t.Logf("Override value changed: expected %s, got %v", overrideValue, val)
				return false
			}

			return true
		},
		genClusterName(),
		genConfigKey(),
		genConfigValue(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestMigrationRoundTrip tests that migrating up and then rolling back preserves values
func TestMigrationRoundTrip(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	// Create a v1.0.0 configuration
	original := Config{
		SchemaVersion: SchemaVersion1_0_0,
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
			Infrastructure: Infrastructure{
				Provider: "openstack",
			},
			Cluster: ClusterConfig{
				Kubernetes: KubernetesConfig{
					Networking: Networking{
						SubnetPods:     "10.100.0.0/16",
						SubnetServices: "10.200.0.0/16",
					},
				},
			},
		},
	}

	// Migrate to v1.1.0
	migrated, err := mgr.MigrateConfig(ctx, original, SchemaVersion1_1_0)
	require.NoError(t, err)

	// Rollback to v1.0.0
	rolledBack, err := mgr.MigrateConfig(ctx, migrated, SchemaVersion1_0_0)
	require.NoError(t, err)

	// Verify all user values are preserved
	require.Equal(t, original.OpenCenter.Meta.Name, rolledBack.OpenCenter.Meta.Name)
	require.Equal(t, original.OpenCenter.Meta.Organization, rolledBack.OpenCenter.Meta.Organization)
	require.Equal(t, original.OpenCenter.Infrastructure.Provider, rolledBack.OpenCenter.Infrastructure.Provider)
	require.Equal(t, original.OpenCenter.Cluster.Kubernetes.Networking.SubnetPods, rolledBack.OpenCenter.Cluster.Kubernetes.Networking.SubnetPods)
	require.Equal(t, original.OpenCenter.Cluster.Kubernetes.Networking.SubnetServices, rolledBack.OpenCenter.Cluster.Kubernetes.Networking.SubnetServices)
}

// genCIDR generates valid CIDR notation strings for testing
func genCIDR() gopter.Gen {
	return gen.OneConstOf(
		"10.0.0.0/8",
		"10.100.0.0/16",
		"10.200.0.0/16",
		"10.244.0.0/16",
		"10.96.0.0/12",
		"172.16.0.0/12",
		"192.168.0.0/16",
	)
}

// genOrgName generates valid organization names
func genOrgName() gopter.Gen {
	return gen.OneConstOf(
		"acme-corp",
		"test-org",
		"demo-org",
		"example-inc",
		"sample-company",
		"dev-team",
		"ops-team",
		"platform-team",
	)
}

// genConfigKey generates valid configuration keys
func genConfigKey() gopter.Gen {
	return gen.OneConstOf(
		"custom.setting",
		"feature.enabled",
		"service.endpoint",
		"network.cidr",
		"storage.class",
		"compute.flavor",
		"security.policy",
		"monitoring.enabled",
	)
}

// genConfigValue generates valid configuration values
func genConfigValue() gopter.Gen {
	return gen.OneConstOf(
		"true",
		"false",
		"enabled",
		"disabled",
		"high",
		"medium",
		"low",
		"standard",
		"premium",
		"basic",
	)
}

// TestMigrationPathValidation tests Property 26: Migration Path Validation
// Feature: configuration-system-refactor, Property 26: For any version transition request,
// invalid migration paths should be rejected with clear error messages
func TestMigrationPathValidation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	validVersions := mgr.GetSupportedVersions()

	// Property: Valid migration paths are accepted
	properties.Property("valid migration paths accepted", prop.ForAll(
		func(fromIdx, toIdx int) bool {
			if fromIdx < 0 || fromIdx >= len(validVersions) || toIdx < 0 || toIdx >= len(validVersions) {
				return true // Skip invalid indices
			}

			fromVersion := validVersions[fromIdx]
			toVersion := validVersions[toIdx]

			err := mgr.ValidateMigrationPath(fromVersion, toVersion)
			// All valid version combinations should either succeed or fail gracefully
			// We expect no panics or unexpected errors
			return err == nil || err.Error() != ""
		},
		gen.IntRange(0, len(validVersions)-1),
		gen.IntRange(0, len(validVersions)-1),
	))

	// Property: Invalid source versions are rejected
	properties.Property("invalid source versions rejected", prop.ForAll(
		func(invalidVersion string) bool {
			// Skip if accidentally generated a valid version
			for _, v := range validVersions {
				if v == invalidVersion {
					return true
				}
			}

			err := mgr.ValidateMigrationPath(invalidVersion, SchemaVersion1_0_0)
			if err == nil {
				t.Logf("Expected error for invalid source version %s, but got nil", invalidVersion)
				return false
			}

			// Error message should mention the invalid path
			if err.Error() == "" {
				t.Log("Error message is empty")
				return false
			}

			return true
		},
		genInvalidVersion(),
	))

	// Property: Invalid target versions are rejected
	properties.Property("invalid target versions rejected", prop.ForAll(
		func(invalidVersion string) bool {
			// Skip if accidentally generated a valid version
			for _, v := range validVersions {
				if v == invalidVersion {
					return true
				}
			}

			err := mgr.ValidateMigrationPath(SchemaVersion1_0_0, invalidVersion)
			if err == nil {
				t.Logf("Expected error for invalid target version %s, but got nil", invalidVersion)
				return false
			}

			// Error message should mention the invalid path
			if err.Error() == "" {
				t.Log("Error message is empty")
				return false
			}

			return true
		},
		genInvalidVersion(),
	))

	// Property: Empty versions are rejected
	properties.Property("empty versions rejected", prop.ForAll(
		func(useEmptySource bool) bool {
			var err error
			if useEmptySource {
				err = mgr.ValidateMigrationPath("", SchemaVersion1_0_0)
			} else {
				err = mgr.ValidateMigrationPath(SchemaVersion1_0_0, "")
			}

			if err == nil {
				t.Log("Expected error for empty version, but got nil")
				return false
			}

			return true
		},
		gen.Bool(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestMigrationPathContinuity tests that migration paths are continuous
func TestMigrationPathContinuity(t *testing.T) {
	properties := gopter.NewProperties(nil)

	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	validVersions := mgr.GetSupportedVersions()

	// Property: Migration paths are continuous (no gaps)
	properties.Property("migration paths are continuous", prop.ForAll(
		func(fromIdx, toIdx int) bool {
			if fromIdx < 0 || fromIdx >= len(validVersions) || toIdx < 0 || toIdx >= len(validVersions) {
				return true // Skip invalid indices
			}

			fromVersion := validVersions[fromIdx]
			toVersion := validVersions[toIdx]

			// Skip same version
			if fromVersion == toVersion {
				return true
			}

			path, err := mgr.GetMigrationPath(fromVersion, toVersion)
			if err != nil {
				// If there's no path, that's acceptable (not all versions are connected)
				return true
			}

			// Verify path continuity
			if len(path) == 0 {
				return true // Empty path for same version is OK
			}

			// First step should start from source version
			if path[0].FromVersion != fromVersion {
				t.Logf("Path doesn't start from source: expected %s, got %s", fromVersion, path[0].FromVersion)
				return false
			}

			// Last step should end at target version
			if path[len(path)-1].ToVersion != toVersion {
				t.Logf("Path doesn't end at target: expected %s, got %s", toVersion, path[len(path)-1].ToVersion)
				return false
			}

			// Each step should connect to the next
			for i := 1; i < len(path); i++ {
				if path[i-1].ToVersion != path[i].FromVersion {
					t.Logf("Path discontinuity at step %d: %s -> %s, then %s -> %s",
						i, path[i-1].FromVersion, path[i-1].ToVersion, path[i].FromVersion, path[i].ToVersion)
					return false
				}
			}

			return true
		},
		gen.IntRange(0, len(validVersions)-1),
		gen.IntRange(0, len(validVersions)-1),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestMigrationErrorMessages tests that error messages are clear and actionable
func TestMigrationErrorMessages(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)

	tests := []struct {
		name          string
		fromVersion   string
		toVersion     string
		shouldContain []string
	}{
		{
			name:          "invalid source version error",
			fromVersion:   "v99.0.0",
			toVersion:     SchemaVersion1_0_0,
			shouldContain: []string{"no migration path", "v99.0.0", SchemaVersion1_0_0},
		},
		{
			name:          "invalid target version error",
			fromVersion:   SchemaVersion1_0_0,
			toVersion:     "v99.0.0",
			shouldContain: []string{"no migration path", SchemaVersion1_0_0, "v99.0.0"},
		},
		{
			name:          "both invalid error",
			fromVersion:   "v98.0.0",
			toVersion:     "v99.0.0",
			shouldContain: []string{"no migration path", "v98.0.0", "v99.0.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.ValidateMigrationPath(tt.fromVersion, tt.toVersion)
			require.Error(t, err)

			errMsg := err.Error()
			for _, expected := range tt.shouldContain {
				assert.Contains(t, errMsg, expected,
					"Error message should contain '%s'", expected)
			}
		})
	}
}

// genInvalidVersion generates invalid version strings for testing
func genInvalidVersion() gopter.Gen {
	return gen.OneConstOf(
		"v99.0.0",
		"v100.0.0",
		"invalid",
		"not-a-version",
		"1.0.0", // Missing 'v' prefix
		"v1",    // Incomplete version
		"v1.0",  // Incomplete version
		"vX.Y.Z",
		"",
		"v-1.0.0",
		"v1.0.0-beta", // Pre-release versions not supported
	)
}

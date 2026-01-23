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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewVersionedSchemaManager(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)

	assert.NotNil(t, mgr)
	assert.Equal(t, CurrentSchemaVersion, mgr.GetCurrentVersion())
	assert.NotEmpty(t, mgr.GetSupportedVersions())
}

func TestSchemaManager_GetSupportedVersions(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)

	versions := mgr.GetSupportedVersions()

	// Should include at least the current version and 1.0.0
	assert.Contains(t, versions, CurrentSchemaVersion)
	assert.Contains(t, versions, SchemaVersion1_0_0)
	assert.GreaterOrEqual(t, len(versions), 2)
}

func TestSchemaManager_ValidateMigrationPath(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)

	tests := []struct {
		name        string
		fromVersion string
		toVersion   string
		wantErr     bool
		errContains string
	}{
		{
			name:        "same version",
			fromVersion: SchemaVersion1_0_0,
			toVersion:   SchemaVersion1_0_0,
			wantErr:     false,
		},
		{
			name:        "direct migration 1.0.0 to v1.1.0",
			fromVersion: SchemaVersion1_0_0,
			toVersion:   SchemaVersion1_1_0,
			wantErr:     false,
		},
		{
			name:        "multi-step migration 1.0.0 to v2.0.0",
			fromVersion: SchemaVersion1_0_0,
			toVersion:   SchemaVersion2_0_0,
			wantErr:     false,
		},
		{
			name:        "invalid source version",
			fromVersion: "v99.0.0",
			toVersion:   SchemaVersion1_0_0,
			wantErr:     true,
			errContains: "no migration path found",
		},
		{
			name:        "invalid target version",
			fromVersion: SchemaVersion1_0_0,
			toVersion:   "v99.0.0",
			wantErr:     true,
			errContains: "no migration path found",
		},
		{
			name:        "both versions invalid",
			fromVersion: "v98.0.0",
			toVersion:   "v99.0.0",
			wantErr:     true,
			errContains: "no migration path found",
		},
		{
			name:        "empty source version",
			fromVersion: "",
			toVersion:   SchemaVersion1_0_0,
			wantErr:     true,
			errContains: "no migration path found",
		},
		{
			name:        "empty target version",
			fromVersion: SchemaVersion1_0_0,
			toVersion:   "",
			wantErr:     true,
			errContains: "no migration path found",
		},
		{
			name:        "malformed version string",
			fromVersion: "not-a-version",
			toVersion:   SchemaVersion1_0_0,
			wantErr:     true,
			errContains: "no migration path found",
		},
		{
			name:        "rollback path v1.1.0 to 1.0.0",
			fromVersion: SchemaVersion1_1_0,
			toVersion:   SchemaVersion1_0_0,
			wantErr:     false,
		},
		{
			name:        "rollback path v2.0.0 to v1.2.0",
			fromVersion: SchemaVersion2_0_0,
			toVersion:   SchemaVersion1_2_0,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.ValidateMigrationPath(tt.fromVersion, tt.toVersion)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaManager_GetMigrationPath(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)

	tests := []struct {
		name          string
		fromVersion   string
		toVersion     string
		expectedSteps int
		wantErr       bool
	}{
		{
			name:          "same version",
			fromVersion:   SchemaVersion1_0_0,
			toVersion:     SchemaVersion1_0_0,
			expectedSteps: 0,
			wantErr:       false,
		},
		{
			name:          "single step 1.0.0 to v1.1.0",
			fromVersion:   SchemaVersion1_0_0,
			toVersion:     SchemaVersion1_1_0,
			expectedSteps: 1,
			wantErr:       false,
		},
		{
			name:          "two steps 1.0.0 to v1.2.0",
			fromVersion:   SchemaVersion1_0_0,
			toVersion:     SchemaVersion1_2_0,
			expectedSteps: 2,
			wantErr:       false,
		},
		{
			name:          "three steps 1.0.0 to v2.0.0",
			fromVersion:   SchemaVersion1_0_0,
			toVersion:     SchemaVersion2_0_0,
			expectedSteps: 3,
			wantErr:       false,
		},
		{
			name:        "invalid path",
			fromVersion: "v99.0.0",
			toVersion:   SchemaVersion1_0_0,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := mgr.GetMigrationPath(tt.fromVersion, tt.toVersion)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, path)
			} else {
				assert.NoError(t, err)
				assert.Len(t, path, tt.expectedSteps)

				// Verify path continuity
				if len(path) > 0 {
					assert.Equal(t, tt.fromVersion, path[0].FromVersion)
					assert.Equal(t, tt.toVersion, path[len(path)-1].ToVersion)

					for i := 1; i < len(path); i++ {
						assert.Equal(t, path[i-1].ToVersion, path[i].FromVersion,
							"migration path should be continuous")
					}
				}
			}
		})
	}
}

func TestSchemaManager_MigrateConfig_SameVersion(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	config := Config{
		SchemaVersion: SchemaVersion1_0_0,
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
		},
	}

	result, err := mgr.MigrateConfig(ctx, config, SchemaVersion1_0_0)

	require.NoError(t, err)
	assert.Equal(t, SchemaVersion1_0_0, result.SchemaVersion)
	assert.Equal(t, "test-cluster", result.OpenCenter.Meta.Name)
}

func TestSchemaManager_MigrateConfig_V1_0_to_V1_1(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	config := Config{
		SchemaVersion: SchemaVersion1_0_0,
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
		},
	}

	result, err := mgr.MigrateConfig(ctx, config, SchemaVersion1_1_0)

	require.NoError(t, err)
	assert.Equal(t, SchemaVersion1_1_0, result.SchemaVersion)
	assert.Equal(t, "test-cluster", result.OpenCenter.Meta.Name)
	assert.Equal(t, "test-org", result.OpenCenter.Meta.Organization)

	// Check metadata was added
	assert.False(t, result.Metadata.CreatedAt.IsZero())
	assert.False(t, result.Metadata.UpdatedAt.IsZero())
	assert.NotNil(t, result.Metadata.Tags)
	assert.NotNil(t, result.Metadata.Annotations)
	assert.Equal(t, SchemaVersion1_0_0, result.Metadata.Annotations["migrated_from"])
}

func TestSchemaManager_MigrateConfig_V1_0_to_V2_0(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	config := Config{
		SchemaVersion: SchemaVersion1_0_0,
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
		},
	}

	result, err := mgr.MigrateConfig(ctx, config, SchemaVersion2_0_0)

	require.NoError(t, err)
	assert.Equal(t, SchemaVersion2_0_0, result.SchemaVersion)
	assert.Equal(t, "test-cluster", result.OpenCenter.Meta.Name)
	assert.Equal(t, "test-org", result.OpenCenter.Meta.Organization)

	// Check metadata was added through intermediate migrations
	assert.False(t, result.Metadata.CreatedAt.IsZero())
	assert.NotNil(t, result.Metadata.Tags)
	assert.NotNil(t, result.Metadata.Annotations)

	// Check v2.0.0 specific features
	assert.Equal(t, "true", result.Metadata.Annotations["major_version_upgrade"])
	assert.Equal(t, SchemaVersion2_0_0, result.Metadata.Tags["schema_version"])
}

func TestSchemaManager_MigrateConfig_PreservesUserData(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	config := Config{
		SchemaVersion: SchemaVersion1_0_0,
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         "production-cluster",
				Organization: "acme-corp",
			},
			Infrastructure: Infrastructure{
				Provider: "openstack",
			},
		},
		Networking: LegacyNetworking{
			SubnetPods:     "10.100.0.0/16",
			SubnetServices: "10.200.0.0/16",
		},
	}

	result, err := mgr.MigrateConfig(ctx, config, SchemaVersion2_0_0)

	require.NoError(t, err)

	// Verify all user data is preserved
	assert.Equal(t, "production-cluster", result.OpenCenter.Meta.Name)
	assert.Equal(t, "acme-corp", result.OpenCenter.Meta.Organization)
	assert.Equal(t, "openstack", result.OpenCenter.Infrastructure.Provider)
	assert.Equal(t, "10.100.0.0/16", result.OpenCenter.Cluster.Kubernetes.Networking.SubnetPods)
	assert.Equal(t, "10.200.0.0/16", result.OpenCenter.Cluster.Kubernetes.Networking.SubnetServices)
}

func TestSchemaManager_MigrateConfigDryRun(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	config := Config{
		SchemaVersion: SchemaVersion1_0_0,
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
		},
	}

	plan, err := mgr.MigrateConfigDryRun(ctx, config, SchemaVersion2_0_0)

	require.NoError(t, err)
	assert.NotNil(t, plan)
	assert.Equal(t, SchemaVersion1_0_0, plan.FromVersion)
	assert.Equal(t, SchemaVersion2_0_0, plan.ToVersion)
	assert.Len(t, plan.Steps, 3) // 1.0.0 -> v1.1.0 -> v1.2.0 -> v2.0.0

	// Verify step continuity
	assert.Equal(t, SchemaVersion1_0_0, plan.Steps[0].FromVersion)
	assert.Equal(t, SchemaVersion1_1_0, plan.Steps[0].ToVersion)
	assert.Equal(t, SchemaVersion1_1_0, plan.Steps[1].FromVersion)
	assert.Equal(t, SchemaVersion1_2_0, plan.Steps[1].ToVersion)
	assert.Equal(t, SchemaVersion1_2_0, plan.Steps[2].FromVersion)
	assert.Equal(t, SchemaVersion2_0_0, plan.Steps[2].ToVersion)

	// Verify changes are detected
	assert.NotEmpty(t, plan.Changes, "dry-run should detect configuration changes")

	// Verify original config is unchanged
	assert.Equal(t, SchemaVersion1_0_0, config.SchemaVersion)
	assert.True(t, config.Metadata.CreatedAt.IsZero(), "original config should not be modified")
}

func TestSchemaManager_MigrateConfigDryRun_DetectsChanges(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	config := Config{
		SchemaVersion: SchemaVersion1_0_0,
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
		},
	}

	plan, err := mgr.MigrateConfigDryRun(ctx, config, SchemaVersion1_1_0)

	require.NoError(t, err)
	assert.NotNil(t, plan)

	// Should detect metadata additions
	hasMetadataChanges := false
	hasSchemaVersionChange := false

	for _, change := range plan.Changes {
		if change.Path == "metadata.created_at" && change.Type == SchemaChangeTypeAdd {
			hasMetadataChanges = true
		}
		if change.Path == "schema_version" && change.Type == SchemaChangeTypeModify {
			hasSchemaVersionChange = true
		}
	}

	assert.True(t, hasMetadataChanges, "should detect metadata additions")
	assert.True(t, hasSchemaVersionChange, "should detect schema version change")
}

func TestSchemaManager_MigrateConfigDryRun_SameVersion(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	config := Config{
		SchemaVersion: SchemaVersion1_0_0,
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
		},
	}

	plan, err := mgr.MigrateConfigDryRun(ctx, config, SchemaVersion1_0_0)

	require.NoError(t, err)
	assert.NotNil(t, plan)
	assert.Equal(t, SchemaVersion1_0_0, plan.FromVersion)
	assert.Equal(t, SchemaVersion1_0_0, plan.ToVersion)
	assert.Empty(t, plan.Steps, "no migration steps needed for same version")
	assert.Empty(t, plan.Changes, "no changes for same version")
}

func TestSchemaManager_MigrateConfigDryRun_InvalidPath(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	config := Config{
		SchemaVersion: SchemaVersion1_0_0,
	}

	plan, err := mgr.MigrateConfigDryRun(ctx, config, "v99.0.0")

	assert.Error(t, err)
	assert.Nil(t, plan)
	assert.Contains(t, err.Error(), "no migration path found")
}

func TestSchemaManager_MigrateConfigDryRun_PreservesOriginal(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	originalConfig := Config{
		SchemaVersion: SchemaVersion1_0_0,
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
			Infrastructure: Infrastructure{
				Provider: "openstack",
			},
		},
		Networking: LegacyNetworking{
			SubnetPods:     "10.100.0.0/16",
			SubnetServices: "10.200.0.0/16",
		},
	}

	// Make a copy to compare later
	configCopy := originalConfig

	plan, err := mgr.MigrateConfigDryRun(ctx, originalConfig, SchemaVersion2_0_0)

	require.NoError(t, err)
	assert.NotNil(t, plan)

	// Verify original config is completely unchanged
	assert.Equal(t, configCopy.SchemaVersion, originalConfig.SchemaVersion)
	assert.Equal(t, configCopy.OpenCenter.Meta.Name, originalConfig.OpenCenter.Meta.Name)
	assert.Equal(t, configCopy.OpenCenter.Meta.Organization, originalConfig.OpenCenter.Meta.Organization)
	assert.Equal(t, configCopy.OpenCenter.Infrastructure.Provider, originalConfig.OpenCenter.Infrastructure.Provider)
	assert.Equal(t, configCopy.OpenCenter.Cluster.Kubernetes.Networking.SubnetPods, originalConfig.OpenCenter.Cluster.Kubernetes.Networking.SubnetPods)
	assert.Equal(t, configCopy.OpenCenter.Cluster.Kubernetes.Networking.SubnetServices, originalConfig.OpenCenter.Cluster.Kubernetes.Networking.SubnetServices)
	assert.True(t, originalConfig.Metadata.CreatedAt.IsZero(), "metadata should not be added to original")
}

func TestDetectConfigChanges(t *testing.T) {
	tests := []struct {
		name          string
		before        Config
		after         Config
		expectedCount int
		checkChange   func(t *testing.T, changes []SchemaChange)
	}{
		{
			name: "detects metadata addition",
			before: Config{
				SchemaVersion: SchemaVersion1_0_0,
			},
			after: Config{
				SchemaVersion: SchemaVersion1_1_0,
				Metadata: ConfigMetadata{
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
					Tags:      map[string]string{"env": "test"},
				},
			},
			expectedCount: 4, // schema_version, created_at, updated_at, tags
			checkChange: func(t *testing.T, changes []SchemaChange) {
				hasCreatedAt := false
				hasUpdatedAt := false
				hasTags := false
				for _, change := range changes {
					if change.Path == "metadata.created_at" {
						hasCreatedAt = true
						assert.Equal(t, SchemaChangeTypeAdd, change.Type)
					}
					if change.Path == "metadata.updated_at" {
						hasUpdatedAt = true
						assert.Equal(t, SchemaChangeTypeAdd, change.Type)
					}
					if change.Path == "metadata.tags" {
						hasTags = true
						assert.Equal(t, SchemaChangeTypeAdd, change.Type)
					}
				}
				assert.True(t, hasCreatedAt, "should detect created_at addition")
				assert.True(t, hasUpdatedAt, "should detect updated_at addition")
				assert.True(t, hasTags, "should detect tags addition")
			},
		},
		{
			name: "detects tag modifications",
			before: Config{
				Metadata: ConfigMetadata{
					Tags: map[string]string{"env": "dev"},
				},
			},
			after: Config{
				Metadata: ConfigMetadata{
					Tags: map[string]string{"env": "prod"},
				},
			},
			expectedCount: 1,
			checkChange: func(t *testing.T, changes []SchemaChange) {
				assert.Equal(t, SchemaChangeTypeModify, changes[0].Type)
				assert.Equal(t, "metadata.tags.env", changes[0].Path)
				assert.Equal(t, "dev", changes[0].OldValue)
				assert.Equal(t, "prod", changes[0].NewValue)
			},
		},
		{
			name: "detects annotation additions",
			before: Config{
				Metadata: ConfigMetadata{
					Annotations: map[string]string{},
				},
			},
			after: Config{
				Metadata: ConfigMetadata{
					Annotations: map[string]string{"migrated_from": "1.0.0"},
				},
			},
			expectedCount: 1,
			checkChange: func(t *testing.T, changes []SchemaChange) {
				assert.Equal(t, SchemaChangeTypeAdd, changes[0].Type)
				assert.Equal(t, "metadata.annotations.migrated_from", changes[0].Path)
				assert.Equal(t, "1.0.0", changes[0].NewValue)
			},
		},
		{
			name: "no changes for identical configs",
			before: Config{
				SchemaVersion: SchemaVersion1_0_0,
			},
			after: Config{
				SchemaVersion: SchemaVersion1_0_0,
			},
			expectedCount: 0,
			checkChange: func(t *testing.T, changes []SchemaChange) {
				assert.Empty(t, changes)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := detectConfigChanges(tt.before, tt.after, "1.0.0", "v1.1.0")
			assert.Len(t, changes, tt.expectedCount)
			if tt.checkChange != nil {
				tt.checkChange(t, changes)
			}
		})
	}
}

func TestFormatMigrationPlan(t *testing.T) {
	tests := []struct {
		name     string
		plan     *SchemaMigrationPlan
		contains []string
	}{
		{
			name: "formats complete migration plan",
			plan: &SchemaMigrationPlan{
				FromVersion: SchemaVersion1_0_0,
				ToVersion:   SchemaVersion2_0_0,
				Steps: []SchemaMigrationStep{
					{
						FromVersion: SchemaVersion1_0_0,
						ToVersion:   SchemaVersion1_1_0,
						Description: "Add metadata fields",
					},
					{
						FromVersion: SchemaVersion1_1_0,
						ToVersion:   SchemaVersion2_0_0,
						Description: "Major refactor",
					},
				},
				Changes: []SchemaChange{
					{
						Type:        SchemaChangeTypeAdd,
						Path:        "metadata.created_at",
						Description: "Added creation timestamp",
					},
					{
						Type:        SchemaChangeTypeModify,
						Path:        "schema_version",
						OldValue:    SchemaVersion1_0_0,
						NewValue:    SchemaVersion2_0_0,
						Description: "Schema version updated",
					},
				},
			},
			contains: []string{
				"Migration Plan: 1.0.0 → v2.0.0",
				"Migration Steps:",
				"1. 1.0.0 → v1.1.0",
				"Add metadata fields",
				"2. v1.1.0 → v2.0.0",
				"Major refactor",
				"Configuration Changes:",
				"Additions:",
				"+ metadata.created_at",
				"Modifications:",
				"~ schema_version",
				"dry-run preview",
			},
		},
		{
			name: "formats plan with no changes",
			plan: &SchemaMigrationPlan{
				FromVersion: SchemaVersion1_0_0,
				ToVersion:   SchemaVersion1_0_0,
				Steps:       []SchemaMigrationStep{},
				Changes:     []SchemaChange{},
			},
			contains: []string{
				"Migration Plan: 1.0.0 → 1.0.0",
				"No migration steps required",
				"No configuration changes detected",
			},
		},
		{
			name:     "handles nil plan",
			plan:     nil,
			contains: []string{"No migration plan available"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := FormatMigrationPlan(tt.plan)
			for _, expected := range tt.contains {
				assert.Contains(t, output, expected)
			}
		})
	}
}

func TestSchemaManager_RegisterMigration(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)

	tests := []struct {
		name      string
		migration SchemaVersionMigration
		wantErr   bool
	}{
		{
			name: "valid migration",
			migration: SchemaVersionMigration{
				FromVersion: "v3.0.0",
				ToVersion:   "v3.1.0",
				Description: "Test migration",
				Migrate: func(ctx context.Context, config Config) (Config, error) {
					return config, nil
				},
			},
			wantErr: false,
		},
		{
			name: "missing from version",
			migration: SchemaVersionMigration{
				ToVersion:   "v3.1.0",
				Description: "Test migration",
				Migrate: func(ctx context.Context, config Config) (Config, error) {
					return config, nil
				},
			},
			wantErr: true,
		},
		{
			name: "missing to version",
			migration: SchemaVersionMigration{
				FromVersion: "v3.0.0",
				Description: "Test migration",
				Migrate: func(ctx context.Context, config Config) (Config, error) {
					return config, nil
				},
			},
			wantErr: true,
		},
		{
			name: "missing migrate function",
			migration: SchemaVersionMigration{
				FromVersion: "v3.0.0",
				ToVersion:   "v3.1.0",
				Description: "Test migration",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.RegisterMigration(tt.migration)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDetectSchemaVersion(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name: "explicit v2.0.0",
			config: Config{
				SchemaVersion: SchemaVersion2_0_0,
			},
			expected: SchemaVersion2_0_0,
		},
		{
			name: "explicit v1.1.0",
			config: Config{
				SchemaVersion: SchemaVersion1_1_0,
			},
			expected: SchemaVersion1_1_0,
		},
		{
			name: "infer v2.0.0 from tags",
			config: Config{
				Metadata: ConfigMetadata{
					CreatedAt: time.Now(),
					Tags: map[string]string{
						"schema_version": SchemaVersion2_0_0,
					},
				},
			},
			expected: SchemaVersion2_0_0,
		},
		{
			name: "infer v1.1.0 from metadata",
			config: Config{
				Metadata: ConfigMetadata{
					CreatedAt: time.Now(),
					Tags:      map[string]string{},
				},
			},
			expected: SchemaVersion1_1_0,
		},
		{
			name: "infer 1.0.0 from no metadata",
			config: Config{
				OpenCenter: SimplifiedOpenCenter{
					Meta: ClusterMeta{
						Name: "test",
					},
				},
			},
			expected: SchemaVersion1_0_0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectSchemaVersion(tt.config)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetMigrationDescription(t *testing.T) {
	tests := []struct {
		name        string
		fromVersion string
		toVersion   string
		contains    string
	}{
		{
			name:        "1.0.0 to v1.1.0",
			fromVersion: SchemaVersion1_0_0,
			toVersion:   SchemaVersion1_1_0,
			contains:    "metadata",
		},
		{
			name:        "v1.1.0 to v1.2.0",
			fromVersion: SchemaVersion1_1_0,
			toVersion:   SchemaVersion1_2_0,
			contains:    "plugin",
		},
		{
			name:        "v1.2.0 to v2.0.0",
			fromVersion: SchemaVersion1_2_0,
			toVersion:   SchemaVersion2_0_0,
			contains:    "Major",
		},
		{
			name:        "unknown migration",
			fromVersion: "v99.0.0",
			toVersion:   "1.0.0.0",
			contains:    "Migration from",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc := GetMigrationDescription(tt.fromVersion, tt.toVersion)
			assert.Contains(t, desc, tt.contains)
		})
	}
}

func TestMigrationKey(t *testing.T) {
	key := migrationKey("1.0.0", "v1.1.0")
	assert.Equal(t, "1.0.0->v1.1.0", key)
}

func TestSchemaManager_RollbackConfig_V1_1_to_V1_0(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	// Start with a v1.1.0 config
	config := Config{
		SchemaVersion: SchemaVersion1_1_0,
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
		},
		Metadata: ConfigMetadata{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Tags:      map[string]string{"env": "test"},
			Annotations: map[string]string{
				"migrated_from": SchemaVersion1_0_0,
			},
		},
	}

	// Rollback to 1.0.0
	result, err := mgr.RollbackConfig(ctx, config, SchemaVersion1_0_0)

	require.NoError(t, err)
	assert.Equal(t, SchemaVersion1_0_0, result.SchemaVersion)
	assert.Equal(t, "test-cluster", result.OpenCenter.Meta.Name)
	assert.Equal(t, "test-org", result.OpenCenter.Meta.Organization)

	// Metadata should be removed in 1.0.0
	assert.True(t, result.Metadata.CreatedAt.IsZero())
	assert.True(t, result.Metadata.UpdatedAt.IsZero())
	assert.Nil(t, result.Metadata.Tags)
	assert.Nil(t, result.Metadata.Annotations)
}

func TestSchemaManager_RollbackConfig_V2_0_to_V1_2(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	// Start with a v2.0.0 config
	config := Config{
		SchemaVersion: SchemaVersion2_0_0,
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         "production-cluster",
				Organization: "acme-corp",
			},
		},
		Metadata: ConfigMetadata{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Tags: map[string]string{
				"schema_version": SchemaVersion2_0_0,
				"env":            "production",
			},
			Annotations: map[string]string{
				"major_version_upgrade": "true",
			},
		},
	}

	// Rollback to v1.2.0
	result, err := mgr.RollbackConfig(ctx, config, SchemaVersion1_2_0)

	require.NoError(t, err)
	assert.Equal(t, SchemaVersion1_2_0, result.SchemaVersion)
	assert.Equal(t, "production-cluster", result.OpenCenter.Meta.Name)
	assert.Equal(t, "acme-corp", result.OpenCenter.Meta.Organization)

	// v2.0.0 specific annotations should be removed
	assert.NotContains(t, result.Metadata.Annotations, "major_version_upgrade")
	assert.NotEqual(t, SchemaVersion2_0_0, result.Metadata.Tags["schema_version"])
}

func TestSchemaManager_RollbackConfig_MultiStep(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	// Start with a v2.0.0 config
	config := Config{
		SchemaVersion: SchemaVersion2_0_0,
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
		},
		Metadata: ConfigMetadata{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Tags: map[string]string{
				"schema_version": SchemaVersion2_0_0,
			},
			Annotations: map[string]string{
				"major_version_upgrade": "true",
			},
		},
	}

	// Rollback all the way to 1.0.0 (multi-step rollback)
	result, err := mgr.RollbackConfig(ctx, config, SchemaVersion1_0_0)

	require.NoError(t, err)
	assert.Equal(t, SchemaVersion1_0_0, result.SchemaVersion)
	assert.Equal(t, "test-cluster", result.OpenCenter.Meta.Name)

	// All metadata should be removed in 1.0.0
	assert.True(t, result.Metadata.CreatedAt.IsZero())
	assert.Nil(t, result.Metadata.Tags)
	assert.Nil(t, result.Metadata.Annotations)
}

func TestSchemaManager_RollbackConfig_SameVersion(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	config := Config{
		SchemaVersion: SchemaVersion1_1_0,
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
		},
	}

	// Rollback to same version should be no-op
	result, err := mgr.RollbackConfig(ctx, config, SchemaVersion1_1_0)

	require.NoError(t, err)
	assert.Equal(t, SchemaVersion1_1_0, result.SchemaVersion)
	assert.Equal(t, "test-cluster", result.OpenCenter.Meta.Name)
}

func TestSchemaManager_RollbackConfig_PreservesUserData(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	config := Config{
		SchemaVersion: SchemaVersion1_1_0,
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         "production-cluster",
				Organization: "acme-corp",
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
		Metadata: ConfigMetadata{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Tags:      map[string]string{"env": "prod"},
		},
	}

	// Rollback to 1.0.0
	result, err := mgr.RollbackConfig(ctx, config, SchemaVersion1_0_0)

	require.NoError(t, err)

	// Verify all user data is preserved
	assert.Equal(t, "production-cluster", result.OpenCenter.Meta.Name)
	assert.Equal(t, "acme-corp", result.OpenCenter.Meta.Organization)
	assert.Equal(t, "openstack", result.OpenCenter.Infrastructure.Provider)
	assert.Equal(t, "10.100.0.0/16", result.OpenCenter.Cluster.Kubernetes.Networking.SubnetPods)
	assert.Equal(t, "10.200.0.0/16", result.OpenCenter.Cluster.Kubernetes.Networking.SubnetServices)

	// Metadata should be removed
	assert.True(t, result.Metadata.CreatedAt.IsZero())
	assert.Nil(t, result.Metadata.Tags)
}

func TestSchemaManager_RollbackConfig_InvalidPath(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	config := Config{
		SchemaVersion: SchemaVersion1_0_0,
	}

	// Try to rollback to non-existent version
	result, err := mgr.RollbackConfig(ctx, config, "v99.0.0")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no migration path found")
	assert.Equal(t, SchemaVersion1_0_0, result.SchemaVersion) // Original config returned
}

func TestSchemaManager_MigrateAndRollback_RoundTrip(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	// Start with 1.0.0 config
	originalConfig := Config{
		SchemaVersion: SchemaVersion1_0_0,
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
			Infrastructure: Infrastructure{
				Provider: "openstack",
			},
		},
		Networking: LegacyNetworking{
			SubnetPods:     "10.100.0.0/16",
			SubnetServices: "10.200.0.0/16",
		},
	}

	// Migrate to v1.1.0
	migrated, err := mgr.MigrateConfig(ctx, originalConfig, SchemaVersion1_1_0)
	require.NoError(t, err)
	assert.Equal(t, SchemaVersion1_1_0, migrated.SchemaVersion)
	assert.False(t, migrated.Metadata.CreatedAt.IsZero())

	// Rollback to 1.0.0
	rolledBack, err := mgr.RollbackConfig(ctx, migrated, SchemaVersion1_0_0)
	require.NoError(t, err)
	assert.Equal(t, SchemaVersion1_0_0, rolledBack.SchemaVersion)

	// Verify user data is preserved
	assert.Equal(t, originalConfig.OpenCenter.Meta.Name, rolledBack.OpenCenter.Meta.Name)
	assert.Equal(t, originalConfig.OpenCenter.Meta.Organization, rolledBack.OpenCenter.Meta.Organization)
	assert.Equal(t, originalConfig.OpenCenter.Infrastructure.Provider, rolledBack.OpenCenter.Infrastructure.Provider)
	assert.Equal(t, originalConfig.OpenCenter.Cluster.Kubernetes.Networking.SubnetPods, rolledBack.OpenCenter.Cluster.Kubernetes.Networking.SubnetPods)
	assert.Equal(t, originalConfig.OpenCenter.Cluster.Kubernetes.Networking.SubnetServices, rolledBack.OpenCenter.Cluster.Kubernetes.Networking.SubnetServices)

	// Verify metadata is removed
	assert.True(t, rolledBack.Metadata.CreatedAt.IsZero())
	assert.Nil(t, rolledBack.Metadata.Tags)
}

func TestSchemaManager_MigrateAndRollback_MultiStep_RoundTrip(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	// Start with 1.0.0 config
	originalConfig := Config{
		SchemaVersion: SchemaVersion1_0_0,
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         "production-cluster",
				Organization: "acme-corp",
			},
		},
	}

	// Migrate to v2.0.0 (multi-step)
	migrated, err := mgr.MigrateConfig(ctx, originalConfig, SchemaVersion2_0_0)
	require.NoError(t, err)
	assert.Equal(t, SchemaVersion2_0_0, migrated.SchemaVersion)

	// Rollback to 1.0.0 (multi-step)
	rolledBack, err := mgr.RollbackConfig(ctx, migrated, SchemaVersion1_0_0)
	require.NoError(t, err)
	assert.Equal(t, SchemaVersion1_0_0, rolledBack.SchemaVersion)

	// Verify user data is preserved
	assert.Equal(t, originalConfig.OpenCenter.Meta.Name, rolledBack.OpenCenter.Meta.Name)
	assert.Equal(t, originalConfig.OpenCenter.Meta.Organization, rolledBack.OpenCenter.Meta.Organization)

	// Verify metadata is removed
	assert.True(t, rolledBack.Metadata.CreatedAt.IsZero())
}

func TestRollbackMigrations_EmptyList(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	config := Config{
		SchemaVersion: SchemaVersion1_0_0,
	}

	// Rollback with empty migration list should succeed
	err := mgr.rollbackMigrations(ctx, config, []SchemaVersionMigration{})
	assert.NoError(t, err)
}

func TestRollbackMigrations_MissingRollbackFunction(t *testing.T) {
	mgr := NewVersionedSchemaManager(CurrentSchemaVersion, nil)
	ctx := context.Background()

	config := Config{
		SchemaVersion: SchemaVersion1_0_0,
	}

	// Create a migration without rollback function
	migration := SchemaVersionMigration{
		FromVersion: SchemaVersion1_0_0,
		ToVersion:   SchemaVersion1_1_0,
		Migrate: func(ctx context.Context, c Config) (Config, error) {
			c.SchemaVersion = SchemaVersion1_1_0
			return c, nil
		},
		Rollback: nil, // No rollback function
	}

	// Should fail when trying to rollback
	err := mgr.rollbackMigrations(ctx, config, []SchemaVersionMigration{migration})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not support rollback")
}

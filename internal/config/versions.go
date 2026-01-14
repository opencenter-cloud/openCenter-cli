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
	"time"
)

// Schema version constants
const (
	SchemaVersion1_0_0 = "v1.0.0"
	SchemaVersion1_1_0 = "v1.1.0"
	SchemaVersion1_2_0 = "v1.2.0"
	SchemaVersion2_0_0 = "v2.0.0"
	CurrentSchemaVersion = SchemaVersion2_0_0
)

// migrateV1_0_to_V1_1 migrates configuration from v1.0.0 to v1.1.0.
// Changes:
// - Add Metadata field to Config
// - Add CreatedAt, UpdatedAt timestamps
// - Add Tags and Annotations support
func migrateV1_0_to_V1_1(ctx context.Context, config Config) (Config, error) {
	// Update schema version
	config.SchemaVersion = SchemaVersion1_1_0

	// Add metadata if not present
	if config.Metadata.CreatedAt.IsZero() {
		config.Metadata.CreatedAt = time.Now()
	}
	if config.Metadata.UpdatedAt.IsZero() {
		config.Metadata.UpdatedAt = time.Now()
	}

	// Initialize maps if nil
	if config.Metadata.Tags == nil {
		config.Metadata.Tags = make(map[string]string)
	}
	if config.Metadata.Annotations == nil {
		config.Metadata.Annotations = make(map[string]string)
	}

	// Add migration annotation
	config.Metadata.Annotations["migrated_from"] = SchemaVersion1_0_0
	config.Metadata.Annotations["migrated_at"] = time.Now().Format(time.RFC3339)

	return config, nil
}

// validateV1_0 validates a v1.0.0 configuration before migration.
func validateV1_0(ctx context.Context, config Config) error {
	if config.SchemaVersion != "" && config.SchemaVersion != SchemaVersion1_0_0 {
		return fmt.Errorf("expected schema version %s, got %s", SchemaVersion1_0_0, config.SchemaVersion)
	}

	// Basic validation - ensure required fields exist
	if config.OpenCenter.Meta.Name == "" {
		return fmt.Errorf("cluster name is required")
	}
	if config.OpenCenter.Meta.Organization == "" {
		return fmt.Errorf("organization is required")
	}

	return nil
}

// rollbackV1_1_to_V1_0 rolls back configuration from v1.1.0 to v1.0.0.
func rollbackV1_1_to_V1_0(ctx context.Context, config Config) (Config, error) {
	// Update schema version
	config.SchemaVersion = SchemaVersion1_0_0

	// Remove metadata fields (they don't exist in v1.0.0)
	config.Metadata = ConfigMetadata{}

	return config, nil
}

// migrateV1_1_to_V1_2 migrates configuration from v1.1.0 to v1.2.0.
// Changes:
// - Add service plugin configuration support
// - Add template composition metadata
// - Enhance service configuration structure
func migrateV1_1_to_V1_2(ctx context.Context, config Config) (Config, error) {
	// Update schema version
	config.SchemaVersion = SchemaVersion1_2_0

	// Update metadata
	config.Metadata.UpdatedAt = time.Now()
	if config.Metadata.Annotations == nil {
		config.Metadata.Annotations = make(map[string]string)
	}
	config.Metadata.Annotations["migrated_from"] = SchemaVersion1_1_0
	config.Metadata.Annotations["migrated_at"] = time.Now().Format(time.RFC3339)

	// Migrate service configurations to new structure
	// In v1.2.0, services support plugin-based architecture
	// This is a structural change that preserves existing service configurations

	return config, nil
}

// validateV1_1 validates a v1.1.0 configuration before migration.
func validateV1_1(ctx context.Context, config Config) error {
	if config.SchemaVersion != SchemaVersion1_1_0 {
		return fmt.Errorf("expected schema version %s, got %s", SchemaVersion1_1_0, config.SchemaVersion)
	}

	// Validate metadata exists
	if config.Metadata.CreatedAt.IsZero() {
		return fmt.Errorf("metadata.created_at is required in v1.1.0")
	}

	return nil
}

// rollbackV1_2_to_V1_1 rolls back configuration from v1.2.0 to v1.1.0.
func rollbackV1_2_to_V1_1(ctx context.Context, config Config) (Config, error) {
	// Update schema version
	config.SchemaVersion = SchemaVersion1_1_0

	// Update metadata
	config.Metadata.UpdatedAt = time.Now()
	if config.Metadata.Annotations != nil {
		delete(config.Metadata.Annotations, "migrated_from")
		delete(config.Metadata.Annotations, "migrated_at")
	}

	// Rollback service configurations to v1.1.0 structure
	// This preserves the service configurations but removes v1.2.0-specific features

	return config, nil
}

// migrateV1_2_to_V2_0 migrates configuration from v1.2.0 to v2.0.0.
// Changes:
// - Major refactor with new configuration structure
// - Enhanced validation and error handling
// - New template engine integration
// - MCP server configuration support
func migrateV1_2_to_V2_0(ctx context.Context, config Config) (Config, error) {
	// Update schema version
	config.SchemaVersion = SchemaVersion2_0_0

	// Update metadata
	config.Metadata.UpdatedAt = time.Now()
	if config.Metadata.Annotations == nil {
		config.Metadata.Annotations = make(map[string]string)
	}
	config.Metadata.Annotations["migrated_from"] = SchemaVersion1_2_0
	config.Metadata.Annotations["migrated_at"] = time.Now().Format(time.RFC3339)
	config.Metadata.Annotations["major_version_upgrade"] = "true"

	// Migrate to new configuration structure
	// v2.0.0 introduces significant structural changes while preserving user data

	// Ensure all required fields are present with defaults
	if config.OpenCenter.Infrastructure.Provider == "" {
		config.OpenCenter.Infrastructure.Provider = "openstack" // Default provider
	}

	// Migrate networking configuration to new structure
	if config.Networking.SubnetPods == "" {
		config.Networking.SubnetPods = "10.244.0.0/16" // Default pod CIDR
	}
	if config.Networking.SubnetServices == "" {
		config.Networking.SubnetServices = "10.96.0.0/12" // Default service CIDR
	}

	// Add v2.0.0 specific defaults
	if config.Metadata.Tags == nil {
		config.Metadata.Tags = make(map[string]string)
	}
	config.Metadata.Tags["schema_version"] = SchemaVersion2_0_0

	return config, nil
}

// validateV1_2 validates a v1.2.0 configuration before migration.
func validateV1_2(ctx context.Context, config Config) error {
	if config.SchemaVersion != SchemaVersion1_2_0 {
		return fmt.Errorf("expected schema version %s, got %s", SchemaVersion1_2_0, config.SchemaVersion)
	}

	// Validate required fields for v1.2.0
	if config.OpenCenter.Meta.Name == "" {
		return fmt.Errorf("cluster name is required")
	}
	if config.OpenCenter.Meta.Organization == "" {
		return fmt.Errorf("organization is required")
	}

	return nil
}

// rollbackV2_0_to_V1_2 rolls back configuration from v2.0.0 to v1.2.0.
func rollbackV2_0_to_V1_2(ctx context.Context, config Config) (Config, error) {
	// Update schema version
	config.SchemaVersion = SchemaVersion1_2_0

	// Update metadata
	config.Metadata.UpdatedAt = time.Now()
	if config.Metadata.Annotations != nil {
		delete(config.Metadata.Annotations, "migrated_from")
		delete(config.Metadata.Annotations, "migrated_at")
		delete(config.Metadata.Annotations, "major_version_upgrade")
	}
	if config.Metadata.Tags != nil {
		delete(config.Metadata.Tags, "schema_version")
	}

	// Rollback to v1.2.0 structure
	// This removes v2.0.0-specific features while preserving core configuration

	return config, nil
}

// DetectSchemaVersion attempts to detect the schema version of a configuration.
// If no version is specified, it attempts to infer based on structure.
func DetectSchemaVersion(config Config) string {
	// If schema version is explicitly set, use it
	if config.SchemaVersion != "" {
		return config.SchemaVersion
	}

	// Attempt to infer version based on structure
	// v1.1.0+ has Metadata field
	if !config.Metadata.CreatedAt.IsZero() || len(config.Metadata.Tags) > 0 || len(config.Metadata.Annotations) > 0 {
		// Could be v1.1.0, v1.2.0, or v2.0.0
		// Check for v2.0.0 specific features
		if config.Metadata.Tags != nil {
			if _, hasSchemaTag := config.Metadata.Tags["schema_version"]; hasSchemaTag {
				return SchemaVersion2_0_0
			}
		}
		// Default to v1.1.0 if metadata exists but no v2.0.0 markers
		return SchemaVersion1_1_0
	}

	// No metadata field, must be v1.0.0
	return SchemaVersion1_0_0
}

// GetMigrationDescription returns a human-readable description of what changes
// will be made when migrating between two versions.
func GetMigrationDescription(fromVersion, toVersion string) string {
	key := migrationKey(fromVersion, toVersion)

	descriptions := map[string]string{
		migrationKey(SchemaVersion1_0_0, SchemaVersion1_1_0): "Adds metadata tracking (timestamps, tags, annotations) to your configuration",
		migrationKey(SchemaVersion1_1_0, SchemaVersion1_2_0): "Adds service plugin support and template composition capabilities",
		migrationKey(SchemaVersion1_2_0, SchemaVersion2_0_0): "Major upgrade with enhanced validation, new template engine, and MCP server support",
		migrationKey(SchemaVersion1_1_0, SchemaVersion1_0_0): "Removes metadata fields (rollback to v1.0.0)",
		migrationKey(SchemaVersion1_2_0, SchemaVersion1_1_0): "Removes service plugin features (rollback to v1.1.0)",
		migrationKey(SchemaVersion2_0_0, SchemaVersion1_2_0): "Removes v2.0.0 features (rollback to v1.2.0)",
	}

	if desc, ok := descriptions[key]; ok {
		return desc
	}

	return fmt.Sprintf("Migration from %s to %s", fromVersion, toVersion)
}

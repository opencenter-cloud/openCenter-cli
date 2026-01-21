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
)

// SchemaVersionMigrator defines the interface for schema version migration operations.
type SchemaVersionMigrator interface {
	// MigrateConfig migrates a configuration to the target schema version
	MigrateConfig(ctx context.Context, config Config, targetVersion string) (Config, error)

	// RollbackConfig rolls back a configuration to a previous schema version
	RollbackConfig(ctx context.Context, config Config, targetVersion string) (Config, error)

	// MigrateConfigDryRun previews migration changes without applying them
	MigrateConfigDryRun(ctx context.Context, config Config, targetVersion string) (*SchemaMigrationPlan, error)

	// GetCurrentVersion returns the current schema version supported by this CLI
	GetCurrentVersion() string

	// GetSupportedVersions returns all schema versions supported by this migration manager
	GetSupportedVersions() []string

	// ValidateMigrationPath validates that a migration path exists between two versions
	ValidateMigrationPath(fromVersion, toVersion string) error

	// GetMigrationPath returns the sequence of migrations needed to go from one version to another
	GetMigrationPath(fromVersion, toVersion string) ([]SchemaVersionMigration, error)
}

// SchemaVersionMigration represents a single migration step between schema versions.
type SchemaVersionMigration struct {
	FromVersion string
	ToVersion   string
	Description string
	Migrate     func(context.Context, Config) (Config, error)
	Validate    func(context.Context, Config) error
	Rollback    func(context.Context, Config) (Config, error)
}

// SchemaMigrationPlan describes the changes that will be made during a migration.
type SchemaMigrationPlan struct {
	FromVersion string
	ToVersion   string
	Steps       []SchemaMigrationStep
	Changes     []SchemaChange
}

// SchemaMigrationStep represents a single step in a migration plan.
type SchemaMigrationStep struct {
	FromVersion string
	ToVersion   string
	Description string
}

// SchemaChange describes a specific change that will be made to the configuration.
type SchemaChange struct {
	Type        SchemaChangeType
	Path        string
	OldValue    interface{}
	NewValue    interface{}
	Description string
}

// SchemaChangeType represents the type of change being made.
type SchemaChangeType string

const (
	SchemaChangeTypeAdd    SchemaChangeType = "add"
	SchemaChangeTypeRemove SchemaChangeType = "remove"
	SchemaChangeTypeModify SchemaChangeType = "modify"
	SchemaChangeTypeRename SchemaChangeType = "rename"
)

// VersionedSchemaManager implements SchemaVersionMigrator with support for multiple schema versions.
type VersionedSchemaManager struct {
	currentVersion string
	migrations     map[string]SchemaVersionMigration // key: "fromVersion->toVersion"
	validator      ConfigValidatorInterface
}

// NewVersionedSchemaManager creates a new migration manager with the specified current version.
func NewVersionedSchemaManager(currentVersion string, validator ConfigValidatorInterface) *VersionedSchemaManager {
	mgr := &VersionedSchemaManager{
		currentVersion: currentVersion,
		migrations:     make(map[string]SchemaVersionMigration),
		validator:      validator,
	}

	// Register all known migrations
	mgr.registerMigrations()

	return mgr
}

// RegisterMigration registers a migration between two schema versions.
func (m *VersionedSchemaManager) RegisterMigration(migration SchemaVersionMigration) error {
	if migration.FromVersion == "" || migration.ToVersion == "" {
		return fmt.Errorf("migration must specify both FromVersion and ToVersion")
	}

	if migration.Migrate == nil {
		return fmt.Errorf("migration must provide a Migrate function")
	}

	key := migrationKey(migration.FromVersion, migration.ToVersion)
	if _, exists := m.migrations[key]; exists {
		return fmt.Errorf("migration from %s to %s already registered", migration.FromVersion, migration.ToVersion)
	}

	m.migrations[key] = migration
	return nil
}

// RollbackConfig rolls back a configuration to a previous schema version.
// This is useful for reverting migrations that caused issues.
func (m *VersionedSchemaManager) RollbackConfig(ctx context.Context, config Config, targetVersion string) (Config, error) {
	// Get current version from config
	currentVersion := config.SchemaVersion
	if currentVersion == "" {
		currentVersion = "1.0.0" // Default to 1.0.0 if not specified
	}

	// If already at target version, return as-is
	if currentVersion == targetVersion {
		return config, nil
	}

	// Get migration path (this will find the rollback path if it exists)
	migrations, err := m.GetMigrationPath(currentVersion, targetVersion)
	if err != nil {
		return config, fmt.Errorf("failed to determine rollback path: %w", err)
	}

	// Apply each migration in sequence (these should be rollback migrations)
	result := config
	for i, migration := range migrations {
		// Validate before migration
		if migration.Validate != nil {
			if err := migration.Validate(ctx, result); err != nil {
				return config, fmt.Errorf("validation failed before rollback step %d (%s -> %s): %w",
					i+1, migration.FromVersion, migration.ToVersion, err)
			}
		}

		// Apply migration (which is actually a rollback in this case)
		migrated, err := migration.Migrate(ctx, result)
		if err != nil {
			return config, fmt.Errorf("rollback failed at step %d (%s -> %s): %w",
				i+1, migration.FromVersion, migration.ToVersion, err)
		}

		result = migrated
	}

	// Validate final result
	if m.validator != nil {
		validationResult := m.validator.Validate(ctx, &result)
		if !validationResult.Valid {
			return config, fmt.Errorf("rolled back configuration failed validation: %v", validationResult.Errors)
		}
	}

	return result, nil
}

// MigrateConfig migrates a configuration to the target schema version.
func (m *VersionedSchemaManager) MigrateConfig(ctx context.Context, config Config, targetVersion string) (Config, error) {
	// Get current version from config
	currentVersion := config.SchemaVersion
	if currentVersion == "" {
		currentVersion = "1.0.0" // Default to 1.0.0 if not specified
	}

	// If already at target version, return as-is
	if currentVersion == targetVersion {
		return config, nil
	}

	// Get migration path
	migrations, err := m.GetMigrationPath(currentVersion, targetVersion)
	if err != nil {
		return config, fmt.Errorf("failed to determine migration path: %w", err)
	}

	// Apply each migration in sequence
	result := config
	for i, migration := range migrations {
		// Validate before migration
		if migration.Validate != nil {
			if err := migration.Validate(ctx, result); err != nil {
				return config, fmt.Errorf("validation failed before migration step %d (%s -> %s): %w",
					i+1, migration.FromVersion, migration.ToVersion, err)
			}
		}

		// Apply migration
		migrated, err := migration.Migrate(ctx, result)
		if err != nil {
			// Attempt rollback of previous migrations
			if rollbackErr := m.rollbackMigrations(ctx, config, migrations[:i]); rollbackErr != nil {
				return config, fmt.Errorf("migration failed at step %d (%s -> %s): %w; rollback also failed: %v",
					i+1, migration.FromVersion, migration.ToVersion, err, rollbackErr)
			}
			return config, fmt.Errorf("migration failed at step %d (%s -> %s): %w (rolled back successfully)",
				i+1, migration.FromVersion, migration.ToVersion, err)
		}

		result = migrated
	}

	// Validate final result
	if m.validator != nil {
		validationResult := m.validator.Validate(ctx, &result)
		if !validationResult.Valid {
			return config, fmt.Errorf("migrated configuration failed validation: %v", validationResult.Errors)
		}
	}

	return result, nil
}

// MigrateConfigDryRun previews migration changes without applying them.
func (m *VersionedSchemaManager) MigrateConfigDryRun(ctx context.Context, config Config, targetVersion string) (*SchemaMigrationPlan, error) {
	currentVersion := config.SchemaVersion
	if currentVersion == "" {
		currentVersion = "1.0.0"
	}

	// Get migration path
	migrations, err := m.GetMigrationPath(currentVersion, targetVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to determine migration path: %w", err)
	}

	// Build migration plan
	plan := &SchemaMigrationPlan{
		FromVersion: currentVersion,
		ToVersion:   targetVersion,
		Steps:       make([]SchemaMigrationStep, 0, len(migrations)),
		Changes:     make([]SchemaChange, 0),
	}

	for _, migration := range migrations {
		plan.Steps = append(plan.Steps, SchemaMigrationStep{
			FromVersion: migration.FromVersion,
			ToVersion:   migration.ToVersion,
			Description: migration.Description,
		})
	}

	// Actually perform the migration in memory to detect changes
	originalConfig := config
	result := config
	for i, migration := range migrations {
		// Validate before migration
		if migration.Validate != nil {
			if err := migration.Validate(ctx, result); err != nil {
				return nil, fmt.Errorf("validation failed before migration step %d (%s -> %s): %w",
					i+1, migration.FromVersion, migration.ToVersion, err)
			}
		}

		// Apply migration in memory
		migrated, err := migration.Migrate(ctx, result)
		if err != nil {
			return nil, fmt.Errorf("dry-run migration failed at step %d (%s -> %s): %w",
				i+1, migration.FromVersion, migration.ToVersion, err)
		}

		// Detect changes between result and migrated
		stepChanges := detectConfigChanges(result, migrated, migration.FromVersion, migration.ToVersion)
		plan.Changes = append(plan.Changes, stepChanges...)

		result = migrated
	}

	// Add summary change for schema version
	if originalConfig.SchemaVersion != result.SchemaVersion {
		plan.Changes = append([]SchemaChange{{
			Type:        SchemaChangeTypeModify,
			Path:        "schema_version",
			OldValue:    originalConfig.SchemaVersion,
			NewValue:    result.SchemaVersion,
			Description: fmt.Sprintf("Schema version updated from %s to %s", originalConfig.SchemaVersion, result.SchemaVersion),
		}}, plan.Changes...)
	}

	return plan, nil
}

// GetCurrentVersion returns the current schema version supported by this CLI.
func (m *VersionedSchemaManager) GetCurrentVersion() string {
	return m.currentVersion
}

// GetSupportedVersions returns all schema versions that can be migrated to/from.
func (m *VersionedSchemaManager) GetSupportedVersions() []string {
	versionSet := make(map[string]bool)
	versionSet[m.currentVersion] = true

	for _, migration := range m.migrations {
		versionSet[migration.FromVersion] = true
		versionSet[migration.ToVersion] = true
	}

	versions := make([]string, 0, len(versionSet))
	for version := range versionSet {
		versions = append(versions, version)
	}

	return versions
}

// ValidateMigrationPath validates that a migration path exists between two versions.
func (m *VersionedSchemaManager) ValidateMigrationPath(fromVersion, toVersion string) error {
	_, err := m.GetMigrationPath(fromVersion, toVersion)
	return err
}

// GetMigrationPath returns the sequence of migrations needed to go from one version to another.
func (m *VersionedSchemaManager) GetMigrationPath(fromVersion, toVersion string) ([]SchemaVersionMigration, error) {
	if fromVersion == toVersion {
		return []SchemaVersionMigration{}, nil
	}

	// Use breadth-first search to find shortest migration path
	type node struct {
		version string
		path    []SchemaVersionMigration
	}

	queue := []node{{version: fromVersion, path: []SchemaVersionMigration{}}}
	visited := make(map[string]bool)
	visited[fromVersion] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Check all possible next migrations from current version
		for _, migration := range m.migrations {
			if migration.FromVersion != current.version {
				continue
			}

			nextVersion := migration.ToVersion

			// Found target version
			if nextVersion == toVersion {
				return append(current.path, migration), nil
			}

			// Add to queue if not visited
			if !visited[nextVersion] {
				visited[nextVersion] = true
				queue = append(queue, node{
					version: nextVersion,
					path:    append(append([]SchemaVersionMigration{}, current.path...), migration),
				})
			}
		}
	}

	return nil, fmt.Errorf("no migration path found from %s to %s", fromVersion, toVersion)
}

// rollbackMigrations attempts to rollback a sequence of migrations.
// It applies rollback functions in reverse order to restore the original configuration.
func (m *VersionedSchemaManager) rollbackMigrations(ctx context.Context, originalConfig Config, migrations []SchemaVersionMigration) error {
	if len(migrations) == 0 {
		return nil // Nothing to rollback
	}

	// Start with the original configuration
	result := originalConfig

	// Apply each migration forward to build intermediate states
	intermediateStates := make([]Config, len(migrations)+1)
	intermediateStates[0] = originalConfig

	for i, migration := range migrations {
		migrated, err := migration.Migrate(ctx, result)
		if err != nil {
			// If we can't even replay the migration, we can't rollback properly
			return fmt.Errorf("failed to replay migration %s -> %s during rollback: %w",
				migration.FromVersion, migration.ToVersion, err)
		}
		result = migrated
		intermediateStates[i+1] = migrated
	}

	// Now rollback in reverse order
	for i := len(migrations) - 1; i >= 0; i-- {
		migration := migrations[i]
		if migration.Rollback == nil {
			return fmt.Errorf("migration %s -> %s does not support rollback", migration.FromVersion, migration.ToVersion)
		}

		// Apply rollback to the state after this migration
		rolledBack, err := migration.Rollback(ctx, intermediateStates[i+1])
		if err != nil {
			return fmt.Errorf("failed to rollback migration %s -> %s: %w",
				migration.FromVersion, migration.ToVersion, err)
		}

		// Verify rollback restored the correct state
		// We compare with the state before this migration was applied
		if rolledBack.SchemaVersion != intermediateStates[i].SchemaVersion {
			return fmt.Errorf("rollback of migration %s -> %s did not restore correct schema version (expected %s, got %s)",
				migration.FromVersion, migration.ToVersion,
				intermediateStates[i].SchemaVersion, rolledBack.SchemaVersion)
		}
	}

	return nil
}

// migrationKey creates a unique key for a migration.
func migrationKey(fromVersion, toVersion string) string {
	return fromVersion + "->" + toVersion
}

// detectConfigChanges compares two configurations and returns a list of changes.
func detectConfigChanges(before, after Config, fromVersion, toVersion string) []SchemaChange {
	changes := make([]SchemaChange, 0)

	// Check for metadata changes
	if before.Metadata.CreatedAt.IsZero() && !after.Metadata.CreatedAt.IsZero() {
		changes = append(changes, SchemaChange{
			Type:        SchemaChangeTypeAdd,
			Path:        "metadata.created_at",
			OldValue:    nil,
			NewValue:    after.Metadata.CreatedAt,
			Description: "Added creation timestamp to metadata",
		})
	}

	if before.Metadata.UpdatedAt.IsZero() && !after.Metadata.UpdatedAt.IsZero() {
		changes = append(changes, SchemaChange{
			Type:        SchemaChangeTypeAdd,
			Path:        "metadata.updated_at",
			OldValue:    nil,
			NewValue:    after.Metadata.UpdatedAt,
			Description: "Added update timestamp to metadata",
		})
	}

	// Check for tags changes
	if before.Metadata.Tags == nil && after.Metadata.Tags != nil {
		changes = append(changes, SchemaChange{
			Type:        SchemaChangeTypeAdd,
			Path:        "metadata.tags",
			OldValue:    nil,
			NewValue:    after.Metadata.Tags,
			Description: "Added tags to metadata",
		})
	} else if before.Metadata.Tags != nil && after.Metadata.Tags != nil {
		// Check for new or modified tags
		for key, newValue := range after.Metadata.Tags {
			if oldValue, exists := before.Metadata.Tags[key]; !exists {
				changes = append(changes, SchemaChange{
					Type:        SchemaChangeTypeAdd,
					Path:        fmt.Sprintf("metadata.tags.%s", key),
					OldValue:    nil,
					NewValue:    newValue,
					Description: fmt.Sprintf("Added tag '%s' with value '%s'", key, newValue),
				})
			} else if oldValue != newValue {
				changes = append(changes, SchemaChange{
					Type:        SchemaChangeTypeModify,
					Path:        fmt.Sprintf("metadata.tags.%s", key),
					OldValue:    oldValue,
					NewValue:    newValue,
					Description: fmt.Sprintf("Modified tag '%s' from '%s' to '%s'", key, oldValue, newValue),
				})
			}
		}
	}

	// Check for annotations changes
	if before.Metadata.Annotations == nil && after.Metadata.Annotations != nil {
		changes = append(changes, SchemaChange{
			Type:        SchemaChangeTypeAdd,
			Path:        "metadata.annotations",
			OldValue:    nil,
			NewValue:    after.Metadata.Annotations,
			Description: "Added annotations to metadata",
		})
	} else if before.Metadata.Annotations != nil && after.Metadata.Annotations != nil {
		// Check for new or modified annotations
		for key, newValue := range after.Metadata.Annotations {
			if oldValue, exists := before.Metadata.Annotations[key]; !exists {
				changes = append(changes, SchemaChange{
					Type:        SchemaChangeTypeAdd,
					Path:        fmt.Sprintf("metadata.annotations.%s", key),
					OldValue:    nil,
					NewValue:    newValue,
					Description: fmt.Sprintf("Added annotation '%s' with value '%s'", key, newValue),
				})
			} else if oldValue != newValue {
				changes = append(changes, SchemaChange{
					Type:        SchemaChangeTypeModify,
					Path:        fmt.Sprintf("metadata.annotations.%s", key),
					OldValue:    oldValue,
					NewValue:    newValue,
					Description: fmt.Sprintf("Modified annotation '%s' from '%s' to '%s'", key, oldValue, newValue),
				})
			}
		}
	}

	// Check for schema version change
	if before.SchemaVersion != after.SchemaVersion {
		changes = append(changes, SchemaChange{
			Type:        SchemaChangeTypeModify,
			Path:        "schema_version",
			OldValue:    before.SchemaVersion,
			NewValue:    after.SchemaVersion,
			Description: fmt.Sprintf("Schema version migrated from %s to %s", before.SchemaVersion, after.SchemaVersion),
		})
	}

	// Check for CreatedBy field
	if before.Metadata.CreatedBy == "" && after.Metadata.CreatedBy != "" {
		changes = append(changes, SchemaChange{
			Type:        SchemaChangeTypeAdd,
			Path:        "metadata.created_by",
			OldValue:    nil,
			NewValue:    after.Metadata.CreatedBy,
			Description: fmt.Sprintf("Added created_by field with value '%s'", after.Metadata.CreatedBy),
		})
	}

	return changes
}

// FormatMigrationPlan formats a migration plan for display to users.
func FormatMigrationPlan(plan *SchemaMigrationPlan) string {
	if plan == nil {
		return "No migration plan available"
	}

	var output string

	// Header
	output += fmt.Sprintf("Migration Plan: %s → %s\n", plan.FromVersion, plan.ToVersion)
	output += fmt.Sprintf("=====================================\n\n")

	// Migration steps
	if len(plan.Steps) > 0 {
		output += "Migration Steps:\n"
		for i, step := range plan.Steps {
			output += fmt.Sprintf("  %d. %s → %s\n", i+1, step.FromVersion, step.ToVersion)
			if step.Description != "" {
				output += fmt.Sprintf("     %s\n", step.Description)
			}
		}
		output += "\n"
	} else {
		output += "No migration steps required (already at target version)\n\n"
	}

	// Configuration changes
	if len(plan.Changes) > 0 {
		output += "Configuration Changes:\n"

		// Group changes by type
		addChanges := []SchemaChange{}
		modifyChanges := []SchemaChange{}
		removeChanges := []SchemaChange{}
		renameChanges := []SchemaChange{}

		for _, change := range plan.Changes {
			switch change.Type {
			case SchemaChangeTypeAdd:
				addChanges = append(addChanges, change)
			case SchemaChangeTypeModify:
				modifyChanges = append(modifyChanges, change)
			case SchemaChangeTypeRemove:
				removeChanges = append(removeChanges, change)
			case SchemaChangeTypeRename:
				renameChanges = append(renameChanges, change)
			}
		}

		// Display additions
		if len(addChanges) > 0 {
			output += "\n  Additions:\n"
			for _, change := range addChanges {
				output += fmt.Sprintf("    + %s\n", change.Path)
				if change.Description != "" {
					output += fmt.Sprintf("      %s\n", change.Description)
				}
			}
		}

		// Display modifications
		if len(modifyChanges) > 0 {
			output += "\n  Modifications:\n"
			for _, change := range modifyChanges {
				output += fmt.Sprintf("    ~ %s\n", change.Path)
				if change.Description != "" {
					output += fmt.Sprintf("      %s\n", change.Description)
				}
			}
		}

		// Display removals
		if len(removeChanges) > 0 {
			output += "\n  Removals:\n"
			for _, change := range removeChanges {
				output += fmt.Sprintf("    - %s\n", change.Path)
				if change.Description != "" {
					output += fmt.Sprintf("      %s\n", change.Description)
				}
			}
		}

		// Display renames
		if len(renameChanges) > 0 {
			output += "\n  Renames:\n"
			for _, change := range renameChanges {
				output += fmt.Sprintf("    → %s\n", change.Path)
				if change.Description != "" {
					output += fmt.Sprintf("      %s\n", change.Description)
				}
			}
		}

		output += "\n"
	} else {
		output += "No configuration changes detected\n\n"
	}

	output += "Note: This is a dry-run preview. No changes have been applied.\n"

	return output
}

// registerMigrations registers all known schema migrations.
func (m *VersionedSchemaManager) registerMigrations() {
	// Register migration from 1.0.0 to v1.1.0
	m.RegisterMigration(SchemaVersionMigration{
		FromVersion: "1.0.0",
		ToVersion:   "v1.1.0",
		Description: "Add metadata fields and enhanced configuration structure",
		Migrate:     migrateV1_0_to_V1_1,
		Validate:    validateV1_0,
		Rollback:    rollbackV1_1_to_V1_0,
	})

	// Register rollback from v1.1.0 to 1.0.0
	m.RegisterMigration(SchemaVersionMigration{
		FromVersion: "v1.1.0",
		ToVersion:   "1.0.0",
		Description: "Remove metadata fields (rollback to 1.0.0)",
		Migrate:     rollbackV1_1_to_V1_0,
		Validate:    validateV1_1,
		Rollback:    migrateV1_0_to_V1_1,
	})

	// Register migration from v1.1.0 to v1.2.0
	m.RegisterMigration(SchemaVersionMigration{
		FromVersion: "v1.1.0",
		ToVersion:   "v1.2.0",
		Description: "Add service plugin support and template composition",
		Migrate:     migrateV1_1_to_V1_2,
		Validate:    validateV1_1,
		Rollback:    rollbackV1_2_to_V1_1,
	})

	// Register rollback from v1.2.0 to v1.1.0
	m.RegisterMigration(SchemaVersionMigration{
		FromVersion: "v1.2.0",
		ToVersion:   "v1.1.0",
		Description: "Remove service plugin features (rollback to v1.1.0)",
		Migrate:     rollbackV1_2_to_V1_1,
		Validate:    validateV1_2,
		Rollback:    migrateV1_1_to_V1_2,
	})

	// Register migration from v1.2.0 to v2.0.0
	m.RegisterMigration(SchemaVersionMigration{
		FromVersion: "v1.2.0",
		ToVersion:   "v2.0.0",
		Description: "Major refactor with new configuration structure",
		Migrate:     migrateV1_2_to_V2_0,
		Validate:    validateV1_2,
		Rollback:    rollbackV2_0_to_V1_2,
	})

	// Register rollback from v2.0.0 to v1.2.0
	m.RegisterMigration(SchemaVersionMigration{
		FromVersion: "v2.0.0",
		ToVersion:   "v1.2.0",
		Description: "Remove v2.0.0 features (rollback to v1.2.0)",
		Migrate:     rollbackV2_0_to_V1_2,
		Validate:    nil, // v2.0.0 validation not needed for rollback
		Rollback:    migrateV1_2_to_V2_0,
	})
}

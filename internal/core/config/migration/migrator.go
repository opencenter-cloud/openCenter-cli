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

// Package migration provides configuration migration between schema versions.
// It implements a graph-based migration system that can find optimal paths
// between any two configuration versions.
package migration

import (
	"fmt"

	"github.com/rackerlabs/opencenter-cli/internal/config"
)

// MigrationFunc is a function that migrates a configuration from one version to another.
// It takes a source configuration and returns a migrated configuration or an error.
type MigrationFunc func(*config.Config) (*config.Config, error)

// Migration represents a single migration step between two versions.
type Migration struct {
	// From is the source version (e.g., "legacy", "1.0")
	From string

	// To is the target version (e.g., "1.0", "2.0")
	To string

	// Migrate is the function that performs the migration
	Migrate MigrationFunc

	// Description provides human-readable information about what this migration does
	Description string
}

// Migrator manages configuration migrations between different schema versions.
// It maintains a registry of available migrations and can find optimal migration
// paths between any two versions.
type Migrator struct {
	// migrations is a map of "from->to" to Migration
	migrations map[string]*Migration

	// graph represents the migration graph for path finding
	// Key is version, value is list of versions it can migrate to
	graph map[string][]string
}

// NewMigrator creates a new Migrator with no registered migrations.
func NewMigrator() *Migrator {
	return &Migrator{
		migrations: make(map[string]*Migration),
		graph:      make(map[string][]string),
	}
}

// Register adds a migration to the migrator's registry.
// It returns an error if a migration for the same from->to path already exists.
func (m *Migrator) Register(migration *Migration) error {
	if migration == nil {
		return fmt.Errorf("migration cannot be nil")
	}

	if migration.From == "" {
		return fmt.Errorf("migration source version cannot be empty")
	}

	if migration.To == "" {
		return fmt.Errorf("migration target version cannot be empty")
	}

	if migration.Migrate == nil {
		return fmt.Errorf("migration function cannot be nil")
	}

	key := migrationKey(migration.From, migration.To)

	// Check for duplicate registration
	if _, exists := m.migrations[key]; exists {
		return fmt.Errorf("migration from %s to %s already registered", migration.From, migration.To)
	}

	// Register the migration
	m.migrations[key] = migration

	// Update the graph
	m.graph[migration.From] = append(m.graph[migration.From], migration.To)

	return nil
}

// Migrate migrates a configuration from its current version to the target version.
// It automatically finds the optimal migration path and applies all necessary migrations.
// Returns the migrated configuration or an error if migration fails or no path exists.
func (m *Migrator) Migrate(cfg *config.Config, targetVersion string) (*config.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	// Determine current version
	currentVersion := cfg.SchemaVersion
	if currentVersion == "" {
		// Empty version could be legacy or v1.0 (backward compatibility)
		// We'll treat it as legacy if it has legacy structure
		currentVersion = "legacy"
	}

	// If already at target version, return as-is
	if currentVersion == targetVersion {
		return cfg, nil
	}

	// Find migration path
	path, err := m.FindPath(currentVersion, targetVersion)
	if err != nil {
		return nil, fmt.Errorf("cannot find migration path from %s to %s: %w", currentVersion, targetVersion, err)
	}

	// Apply migrations along the path
	result := cfg
	for i := 0; i < len(path)-1; i++ {
		from := path[i]
		to := path[i+1]

		migration, err := m.getMigration(from, to)
		if err != nil {
			return nil, fmt.Errorf("migration step %s->%s failed: %w", from, to, err)
		}

		result, err = migration.Migrate(result)
		if err != nil {
			return nil, fmt.Errorf("migration from %s to %s failed: %w", from, to, err)
		}

		// Update schema version after successful migration
		result.SchemaVersion = to
	}

	return result, nil
}

// FindPath finds the shortest migration path between two versions using BFS.
// Returns a slice of versions representing the path, or an error if no path exists.
// The path includes both the source and target versions.
func (m *Migrator) FindPath(from, to string) ([]string, error) {
	if from == to {
		return []string{from}, nil
	}

	// BFS to find shortest path
	queue := [][]string{{from}}
	visited := make(map[string]bool)
	visited[from] = true

	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]

		current := path[len(path)-1]

		// Check all neighbors
		for _, next := range m.graph[current] {
			if next == to {
				// Found the target
				return append(path, next), nil
			}

			if !visited[next] {
				visited[next] = true
				newPath := make([]string, len(path)+1)
				copy(newPath, path)
				newPath[len(path)] = next
				queue = append(queue, newPath)
			}
		}
	}

	return nil, fmt.Errorf("no migration path exists from %s to %s", from, to)
}

// CanMigrate checks if a migration path exists from the source to target version.
func (m *Migrator) CanMigrate(from, to string) bool {
	_, err := m.FindPath(from, to)
	return err == nil
}

// AvailableMigrations returns a list of all registered migrations.
func (m *Migrator) AvailableMigrations() []*Migration {
	migrations := make([]*Migration, 0, len(m.migrations))
	for _, migration := range m.migrations {
		migrations = append(migrations, migration)
	}
	return migrations
}

// getMigration retrieves a migration for the given from->to path.
func (m *Migrator) getMigration(from, to string) (*Migration, error) {
	key := migrationKey(from, to)
	migration, exists := m.migrations[key]
	if !exists {
		return nil, fmt.Errorf("no migration registered from %s to %s", from, to)
	}
	return migration, nil
}

// migrationKey creates a unique key for a migration path.
func migrationKey(from, to string) string {
	return fmt.Sprintf("%s->%s", from, to)
}

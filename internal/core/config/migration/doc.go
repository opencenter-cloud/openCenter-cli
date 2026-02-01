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
//
// # Overview
//
// The migration package implements a graph-based migration system that can
// automatically find and execute optimal migration paths between any two
// configuration schema versions. It supports:
//
//   - Automatic path finding using BFS
//   - Multi-hop migrations (e.g., legacy -> 1.0 -> 2.0)
//   - Validation at each migration step
//   - Rollback support (future enhancement)
//
// # Architecture
//
// The migration system consists of:
//
//   - Migrator: Core migration engine with path finding
//   - Migration: Individual migration step (from version X to Y)
//   - MigrationFunc: Function that performs the actual migration
//
// # Usage
//
// Basic usage with the default migrator:
//
//	migrator, err := migration.DefaultMigrator()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Migrate a configuration to v2.0
//	migrated, err := migrator.Migrate(cfg, "2.0")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # Custom Migrations
//
// To register a custom migration:
//
//	migrator := migration.NewMigrator()
//
//	customMigration := &migration.Migration{
//	    From: "1.5",
//	    To:   "2.0",
//	    Migrate: func(cfg *config.Config) (*config.Config, error) {
//	        // Perform migration logic
//	        cfg.SchemaVersion = "2.0"
//	        return cfg, nil
//	    },
//	    Description: "Custom migration from 1.5 to 2.0",
//	}
//
//	if err := migrator.Register(customMigration); err != nil {
//	    log.Fatal(err)
//	}
//
// # Migration Graph
//
// The current migration graph supports:
//
//	legacy -> 1.0 -> 2.0
//
// To view the migration graph:
//
//	migrator, _ := migration.DefaultMigrator()
//	fmt.Println(migration.MigrationGraph(migrator))
//
// # Path Finding
//
// The migrator uses BFS to find the shortest path between versions:
//
//	path, err := migrator.FindPath("legacy", "2.0")
//	// Returns: ["legacy", "1.0", "2.0"]
//
// # Error Handling
//
// Migrations can fail at any step. When a migration fails:
//
//   - The error includes the source and target versions
//   - The original configuration is not modified
//   - Partial migrations are not applied
//
// # Thread Safety
//
// The Migrator is safe for concurrent use after all migrations are registered.
// Registration should be done during initialization, not during concurrent use.
//
// # Performance
//
// Migration performance depends on:
//
//   - Number of hops in the path
//   - Complexity of each migration step
//   - Size of the configuration
//
// Typical performance:
//
//   - Single-hop migration: <10ms
//   - Multi-hop migration: <50ms
//   - Path finding: <1ms
//
// # Best Practices
//
//  1. Use DefaultMigrator() for standard migrations
//  2. Register custom migrations during initialization
//  3. Validate configurations after migration
//  4. Test migrations with real configuration files
//  5. Document breaking changes in migration descriptions
//
// # Future Enhancements
//
//   - Rollback support for failed migrations
//   - Migration hooks for custom validation
//   - Parallel migration execution
//   - Migration dry-run mode
//   - Migration history tracking
package migration

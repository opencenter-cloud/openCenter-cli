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

package migration

import "fmt"

// DefaultMigrator returns a Migrator with all standard migrations registered.
// This is the recommended way to get a migrator for production use.
func DefaultMigrator() (*Migrator, error) {
	m := NewMigrator()

	// Register all standard migrations
	migrations := []*Migration{
		LegacyToV1Migration(),
		V1ToV2Migration(),
	}

	for _, migration := range migrations {
		if err := m.Register(migration); err != nil {
			return nil, fmt.Errorf("failed to register migration %s->%s: %w",
				migration.From, migration.To, err)
		}
	}

	return m, nil
}

// MigrationGraph returns a human-readable representation of the migration graph.
// This is useful for debugging and documentation.
func MigrationGraph(m *Migrator) string {
	if m == nil {
		return "No migrator provided"
	}

	result := "Migration Graph:\n"
	for from, targets := range m.graph {
		for _, to := range targets {
			migration, _ := m.getMigration(from, to)
			result += fmt.Sprintf("  %s -> %s: %s\n", from, to, migration.Description)
		}
	}

	return result
}

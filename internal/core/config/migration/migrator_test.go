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

import (
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/config"
)

func TestNewMigrator(t *testing.T) {
	m := NewMigrator()
	if m == nil {
		t.Fatal("NewMigrator returned nil")
	}

	if m.migrations == nil {
		t.Error("migrations map not initialized")
	}

	if m.graph == nil {
		t.Error("graph map not initialized")
	}
}

func TestMigrator_Register(t *testing.T) {
	tests := []struct {
		name        string
		migration   *Migration
		wantErr     bool
		errContains string
	}{
		{
			name: "valid migration",
			migration: &Migration{
				From:        "1.0",
				To:          "2.0",
				Migrate:     func(c *config.Config) (*config.Config, error) { return c, nil },
				Description: "Test migration",
			},
			wantErr: false,
		},
		{
			name:        "nil migration",
			migration:   nil,
			wantErr:     true,
			errContains: "cannot be nil",
		},
		{
			name: "empty from version",
			migration: &Migration{
				From:    "",
				To:      "2.0",
				Migrate: func(c *config.Config) (*config.Config, error) { return c, nil },
			},
			wantErr:     true,
			errContains: "source version cannot be empty",
		},
		{
			name: "empty to version",
			migration: &Migration{
				From:    "1.0",
				To:      "",
				Migrate: func(c *config.Config) (*config.Config, error) { return c, nil },
			},
			wantErr:     true,
			errContains: "target version cannot be empty",
		},
		{
			name: "nil migrate function",
			migration: &Migration{
				From:    "1.0",
				To:      "2.0",
				Migrate: nil,
			},
			wantErr:     true,
			errContains: "function cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMigrator()
			err := m.Register(tt.migration)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestMigrator_RegisterDuplicate(t *testing.T) {
	m := NewMigrator()

	migration := &Migration{
		From:        "1.0",
		To:          "2.0",
		Migrate:     func(c *config.Config) (*config.Config, error) { return c, nil },
		Description: "Test migration",
	}

	// First registration should succeed
	if err := m.Register(migration); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Second registration should fail
	err := m.Register(migration)
	if err == nil {
		t.Error("expected error for duplicate registration, got nil")
	}
	if !contains(err.Error(), "already registered") {
		t.Errorf("error should mention already registered, got: %v", err)
	}
}

func TestMigrator_FindPath(t *testing.T) {
	m := NewMigrator()

	// Register migrations: legacy -> 1.0 -> 2.0
	_ = m.Register(&Migration{
		From:    "legacy",
		To:      "1.0",
		Migrate: func(c *config.Config) (*config.Config, error) { return c, nil },
	})
	_ = m.Register(&Migration{
		From:    "1.0",
		To:      "2.0",
		Migrate: func(c *config.Config) (*config.Config, error) { return c, nil },
	})

	tests := []struct {
		name     string
		from     string
		to       string
		wantPath []string
		wantErr  bool
	}{
		{
			name:     "same version",
			from:     "1.0",
			to:       "1.0",
			wantPath: []string{"1.0"},
			wantErr:  false,
		},
		{
			name:     "direct path",
			from:     "1.0",
			to:       "2.0",
			wantPath: []string{"1.0", "2.0"},
			wantErr:  false,
		},
		{
			name:     "multi-hop path",
			from:     "legacy",
			to:       "2.0",
			wantPath: []string{"legacy", "1.0", "2.0"},
			wantErr:  false,
		},
		{
			name:    "no path exists",
			from:    "2.0",
			to:      "1.0",
			wantErr: true,
		},
		{
			name:    "unknown source",
			from:    "unknown",
			to:      "2.0",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := m.FindPath(tt.from, tt.to)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if !equalSlices(path, tt.wantPath) {
					t.Errorf("path = %v, want %v", path, tt.wantPath)
				}
			}
		})
	}
}

func TestMigrator_CanMigrate(t *testing.T) {
	m := NewMigrator()

	// Register migrations
	_ = m.Register(&Migration{
		From:    "legacy",
		To:      "1.0",
		Migrate: func(c *config.Config) (*config.Config, error) { return c, nil },
	})
	_ = m.Register(&Migration{
		From:    "1.0",
		To:      "2.0",
		Migrate: func(c *config.Config) (*config.Config, error) { return c, nil },
	})

	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{
			name: "can migrate direct",
			from: "1.0",
			to:   "2.0",
			want: true,
		},
		{
			name: "can migrate multi-hop",
			from: "legacy",
			to:   "2.0",
			want: true,
		},
		{
			name: "cannot migrate backward",
			from: "2.0",
			to:   "1.0",
			want: false,
		},
		{
			name: "cannot migrate unknown",
			from: "unknown",
			to:   "2.0",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.CanMigrate(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("CanMigrate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMigrator_Migrate(t *testing.T) {
	m := NewMigrator()

	// Register test migrations
	_ = m.Register(&Migration{
		From: "1.0",
		To:   "2.0",
		Migrate: func(c *config.Config) (*config.Config, error) {
			c.SchemaVersion = "2.0"
			return c, nil
		},
	})

	tests := []struct {
		name          string
		cfg           *config.Config
		targetVersion string
		wantVersion   string
		wantErr       bool
	}{
		{
			name: "successful migration",
			cfg: &config.Config{
				SchemaVersion: "1.0",
			},
			targetVersion: "2.0",
			wantVersion:   "2.0",
			wantErr:       false,
		},
		{
			name: "already at target version",
			cfg: &config.Config{
				SchemaVersion: "2.0",
			},
			targetVersion: "2.0",
			wantVersion:   "2.0",
			wantErr:       false,
		},
		{
			name:          "nil config",
			cfg:           nil,
			targetVersion: "2.0",
			wantErr:       true,
		},
		{
			name: "no migration path",
			cfg: &config.Config{
				SchemaVersion: "2.0",
			},
			targetVersion: "1.0",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := m.Migrate(tt.cfg, tt.targetVersion)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result.SchemaVersion != tt.wantVersion {
					t.Errorf("version = %s, want %s", result.SchemaVersion, tt.wantVersion)
				}
			}
		})
	}
}

func TestMigrator_AvailableMigrations(t *testing.T) {
	m := NewMigrator()

	// Initially empty
	migrations := m.AvailableMigrations()
	if len(migrations) != 0 {
		t.Errorf("expected 0 migrations, got %d", len(migrations))
	}

	// Register some migrations
	_ = m.Register(&Migration{
		From:    "legacy",
		To:      "1.0",
		Migrate: func(c *config.Config) (*config.Config, error) { return c, nil },
	})
	_ = m.Register(&Migration{
		From:    "1.0",
		To:      "2.0",
		Migrate: func(c *config.Config) (*config.Config, error) { return c, nil },
	})

	migrations = m.AvailableMigrations()
	if len(migrations) != 2 {
		t.Errorf("expected 2 migrations, got %d", len(migrations))
	}
}

func TestDefaultMigrator(t *testing.T) {
	m, err := DefaultMigrator()
	if err != nil {
		t.Fatalf("DefaultMigrator failed: %v", err)
	}

	if m == nil {
		t.Fatal("DefaultMigrator returned nil")
	}

	// Should have at least legacy->1.0 and 1.0->2.0
	migrations := m.AvailableMigrations()
	if len(migrations) < 2 {
		t.Errorf("expected at least 2 migrations, got %d", len(migrations))
	}

	// Should be able to migrate from legacy to 2.0
	if !m.CanMigrate("legacy", "2.0") {
		t.Error("should be able to migrate from legacy to 2.0")
	}
}

func TestMigrationGraph(t *testing.T) {
	m := NewMigrator()
	_ = m.Register(&Migration{
		From:        "legacy",
		To:          "1.0",
		Migrate:     func(c *config.Config) (*config.Config, error) { return c, nil },
		Description: "Legacy to v1",
	})

	graph := MigrationGraph(m)
	if graph == "" {
		t.Error("MigrationGraph returned empty string")
	}
	if !contains(graph, "legacy") {
		t.Error("graph should contain 'legacy'")
	}
	if !contains(graph, "1.0") {
		t.Error("graph should contain '1.0'")
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

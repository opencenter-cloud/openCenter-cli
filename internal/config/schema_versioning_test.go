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

// TestDetectSchemaMigrationNeeded tests the schema migration detection functionality.
func TestDetectSchemaMigrationNeeded(t *testing.T) {
	tests := []struct {
		name              string
		config            Config
		expectedNeedsMig  bool
		expectedVersion   string
		expectError       bool
	}{
		{
			name: "config with current schema version",
			config: Config{
				SchemaVersion: SchemaVersion,
			},
			expectedNeedsMig: false,
			expectedVersion:  SchemaVersion,
			expectError:      false,
		},
		{
			name: "config with no schema version",
			config: Config{
				SchemaVersion: "",
			},
			expectedNeedsMig: true,
			expectedVersion:  "",
			expectError:      false,
		},
		{
			name: "config with old schema version",
			config: Config{
				SchemaVersion: "0.9.0",
			},
			expectedNeedsMig: true,
			expectedVersion:  "0.9.0",
			expectError:      false,
		},
		{
			name: "config with future schema version",
			config: Config{
				SchemaVersion: "2.0.0",
			},
			expectedNeedsMig: true,
			expectedVersion:  "2.0.0",
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			needsMig, version, err := DetectSchemaMigrationNeeded(tt.config)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if needsMig != tt.expectedNeedsMig {
				t.Errorf("expected needsMigration=%v, got %v", tt.expectedNeedsMig, needsMig)
			}

			if version != tt.expectedVersion {
				t.Errorf("expected version=%q, got %q", tt.expectedVersion, version)
			}
		})
	}
}

// TestGetMigrationPath tests the migration path determination.
func TestGetMigrationPath(t *testing.T) {
	tests := []struct {
		name          string
		fromVersion   string
		toVersion     string
		expectedPath  []string
		expectError   bool
	}{
		{
			name:          "same version - no migration",
			fromVersion:   SchemaVersion,
			toVersion:     SchemaVersion,
			expectedPath:  []string{},
			expectError:   false,
		},
		{
			name:          "empty from version to current",
			fromVersion:   "",
			toVersion:     SchemaVersion,
			expectedPath:  []string{SchemaVersion},
			expectError:   false,
		},
		{
			name:          "old version to current",
			fromVersion:   "0.9.0",
			toVersion:     SchemaVersion,
			expectedPath:  []string{SchemaVersion},
			expectError:   false,
		},
		{
			name:          "migration to non-current version",
			fromVersion:   "0.9.0",
			toVersion:     "0.8.0",
			expectedPath:  nil,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := GetMigrationPath(tt.fromVersion, tt.toVersion)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError {
				if len(path) != len(tt.expectedPath) {
					t.Errorf("expected path length %d, got %d", len(tt.expectedPath), len(path))
				}
				for i, v := range tt.expectedPath {
					if i >= len(path) || path[i] != v {
						t.Errorf("expected path[%d]=%q, got %q", i, v, path[i])
					}
				}
			}
		})
	}
}

// TestNeedsMigration tests the convenience function for migration detection.
func TestNeedsMigration(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected bool
	}{
		{
			name: "current version - no migration",
			config: Config{
				SchemaVersion: SchemaVersion,
			},
			expected: false,
		},
		{
			name: "no version - needs migration",
			config: Config{
				SchemaVersion: "",
			},
			expected: true,
		},
		{
			name: "old version - needs migration",
			config: Config{
				SchemaVersion: "0.9.0",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NeedsMigration(tt.config)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestSetSchemaVersion tests setting the schema version on a config.
func TestSetSchemaVersion(t *testing.T) {
	config := Config{
		SchemaVersion: "0.9.0",
	}

	newVersion := "1.0.0"
	SetSchemaVersion(&config, newVersion)

	if config.SchemaVersion != newVersion {
		t.Errorf("expected schema version %q, got %q", newVersion, config.SchemaVersion)
	}
}

// TestValidateSchemaVersion tests the schema version validation.
func TestValidateSchemaVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected bool
	}{
		{
			name:     "current version is valid",
			version:  SchemaVersion,
			expected: true,
		},
		{
			name:     "old version is invalid",
			version:  "0.9.0",
			expected: false,
		},
		{
			name:     "empty version is invalid",
			version:  "",
			expected: false,
		},
		{
			name:     "future version is invalid",
			version:  "2.0.0",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSchemaVersion(tt.version)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestDefaultConfigHasSchemaVersion tests that default configs include schema version.
func TestDefaultConfigHasSchemaVersion(t *testing.T) {
	config := NewDefault("test-cluster")

	if config.SchemaVersion == "" {
		t.Error("default config should have schema version set")
	}

	if config.SchemaVersion != SchemaVersion {
		t.Errorf("expected schema version %q, got %q", SchemaVersion, config.SchemaVersion)
	}
}

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
	"os"
	"path/filepath"
	"testing"
)

// Test v1 config detection and routing
// Requirements: 13.2, 13.3
func TestDetectSchemaVersion_V1Config(t *testing.T) {
	// Test explicit v1 version
	v1ConfigExplicit := `
schema_version: "1.0"
opencenter:
  meta:
    name: test-cluster
`

	info, err := DetectSchemaVersionFromBytes([]byte(v1ConfigExplicit))
	if err != nil {
		t.Fatalf("Failed to detect schema version: %v", err)
	}

	if info.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", info.Version)
	}

	if !info.IsV1 {
		t.Error("Expected IsV1 to be true")
	}

	if info.IsV2 {
		t.Error("Expected IsV2 to be false")
	}
}

// Test v2 config detection and routing
// Requirements: 13.2
func TestDetectSchemaVersion_V2Config(t *testing.T) {
	v2Config := `
schema_version: "2.0"
opencenter:
  meta:
    name: test-cluster
`

	info, err := DetectSchemaVersionFromBytes([]byte(v2Config))
	if err != nil {
		t.Fatalf("Failed to detect schema version: %v", err)
	}

	if info.Version != "2.0" {
		t.Errorf("Expected version 2.0, got %s", info.Version)
	}

	if info.IsV1 {
		t.Error("Expected IsV1 to be false")
	}

	if !info.IsV2 {
		t.Error("Expected IsV2 to be true")
	}
}

// Test default to v1 when schema_version missing
// Requirements: 13.3
func TestDetectSchemaVersion_MissingVersion(t *testing.T) {
	configNoVersion := `
opencenter:
  meta:
    name: test-cluster
`

	info, err := DetectSchemaVersionFromBytes([]byte(configNoVersion))
	if err != nil {
		t.Fatalf("Failed to detect schema version: %v", err)
	}

	// Should default to v1 for backward compatibility
	if info.Version != "1.0" {
		t.Errorf("Expected default version 1.0, got %s", info.Version)
	}

	if !info.IsV1 {
		t.Error("Expected IsV1 to be true for missing version")
	}

	if info.IsV2 {
		t.Error("Expected IsV2 to be false for missing version")
	}
}

// Test unsupported version detection
func TestDetectSchemaVersion_UnsupportedVersion(t *testing.T) {
	unsupportedConfig := `
schema_version: "3.0"
opencenter:
  meta:
    name: test-cluster
`

	_, err := DetectSchemaVersionFromBytes([]byte(unsupportedConfig))
	if err == nil {
		t.Error("Expected error for unsupported schema version")
	}
}

// Test detection from file
func TestDetectSchemaVersion_FromFile(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test-config.yaml")

	v1Config := `
schema_version: "1.0"
opencenter:
  meta:
    name: test-cluster
`

	if err := os.WriteFile(testFile, []byte(v1Config), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	info, err := DetectSchemaVersionFromFile(testFile)
	if err != nil {
		t.Fatalf("Failed to detect schema version from file: %v", err)
	}

	if info.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", info.Version)
	}
}

// Test invalid YAML handling
func TestDetectSchemaVersion_InvalidYAML(t *testing.T) {
	invalidYAML := `
this is not valid yaml: [
`

	_, err := DetectSchemaVersionFromBytes([]byte(invalidYAML))
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

// Test deprecation warnings for v1
// Requirements: 13.5
func TestBackwardCompatibility_DeprecationWarnings(t *testing.T) {
	v1Config := createTestV1ConfigForWarnings()

	bcm := NewBackwardCompatibilityManager()
	warnings := bcm.CheckV1Deprecations(&v1Config)

	if len(warnings) == 0 {
		t.Error("Expected deprecation warnings for v1 configuration")
	}

	// Check for schema version warning
	foundSchemaWarning := false
	for _, warning := range warnings {
		if warning.Field == "schema_version" {
			foundSchemaWarning = true
			break
		}
	}

	if !foundSchemaWarning {
		t.Error("Expected schema version deprecation warning")
	}
}

// Test supported versions
func TestBackwardCompatibility_SupportedVersions(t *testing.T) {
	versions := GetSupportedVersions()

	if len(versions) == 0 {
		t.Error("Expected at least one supported version")
	}

	// Should support both v1 and v2 during transition period
	if !SupportsBothVersions() {
		t.Error("Expected both versions to be supported during transition")
	}

	// Check that v1 is supported
	if !IsVersionSupported("1.0") {
		t.Error("Expected v1 to be supported")
	}

	// Check that v2 is supported
	if !IsVersionSupported("2.0") {
		t.Error("Expected v2 to be supported")
	}
}

// Test migration recommendation
func TestBackwardCompatibility_MigrationRecommendation(t *testing.T) {
	v1Config := Config{
		SchemaVersion: "1.0",
	}

	if !ShouldMigrateToV2(&v1Config) {
		t.Error("Expected migration recommendation for v1 config")
	}

	v2Config := Config{
		SchemaVersion: "2.0",
	}

	if ShouldMigrateToV2(&v2Config) {
		t.Error("Did not expect migration recommendation for v2 config")
	}
}

// Test migration command generation
func TestBackwardCompatibility_MigrationCommand(t *testing.T) {
	configPath := "/path/to/config.yaml"
	cmd := GetMigrationCommand(configPath)

	if cmd == "" {
		t.Error("Expected non-empty migration command")
	}

	// Should contain the config path
	if len(cmd) < len(configPath) {
		t.Error("Migration command should contain config path")
	}
}

// Test v1 compatibility validation
func TestBackwardCompatibility_V1Validation(t *testing.T) {
	v1Config := Config{
		SchemaVersion: "1.0",
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name: "test-cluster",
			},
		},
	}

	// Should not error - v1 is still supported
	err := ValidateV1Compatibility(&v1Config)
	if err != nil {
		t.Errorf("Expected v1 config to be compatible: %v", err)
	}
}

// Helper functions

func createTestV1ConfigForWarnings() Config {
	return Config{
		SchemaVersion: "1.0",
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name: "test-cluster",
			},
			Cluster: ClusterConfig{
				Networking: ClusterNetworkingConfig{
					VRRPIP: "10.2.128.100",
				},
				Kubernetes: KubernetesConfig{
					FlavorMaster: "gp.0.4.8",
					FlavorWorker: "gp.0.4.16",
				},
			},
			Storage: StorageConfig{
				DefaultStorageClass: "csi-cinder-sc-delete",
			},
		},
	}
}

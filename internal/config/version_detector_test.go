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
	"strings"
	"testing"
)

// Test v1 config detection and rejection
// Requirements: 13.2, 13.3
// In v2.0.0, v1 configs should be rejected with a clear error message
func TestDetectSchemaVersion_V1Config(t *testing.T) {
	// Test explicit v1 version - should be rejected
	v1ConfigExplicit := `
schema_version: "1.0"
opencenter:
  meta:
    name: test-cluster
`

	_, err := DetectSchemaVersionFromBytes([]byte(v1ConfigExplicit))
	if err == nil {
		t.Fatal("Expected error for v1 config, got nil")
	}

	expectedMsg := "v1 configurations are not supported"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain %q, got: %v", expectedMsg, err)
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

	_, err := DetectSchemaVersionFromBytes([]byte(configNoVersion))
	if err == nil {
		t.Fatal("Expected error for config with missing schema_version, got nil")
	}

	expectedMsg := "v1 configurations are not supported"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain %q, got: %v", expectedMsg, err)
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

	_, err := DetectSchemaVersionFromFile(testFile)
	if err == nil {
		t.Fatal("Expected error for v1 config file, got nil")
	}

	expectedMsg := "v1 configurations are not supported"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain %q, got: %v", expectedMsg, err)
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

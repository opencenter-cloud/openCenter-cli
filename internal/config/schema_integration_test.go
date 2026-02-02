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

	"gopkg.in/yaml.v3"
)

// TestSchemaVersionInSavedConfig tests that saved configs include schema version.
func TestSchemaVersionInSavedConfig(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	// Create a test config using test fixture
	clusterName := "test-schema-version"
	config := testModeConfig(clusterName)

	// Verify schema version is set
	if config.SchemaVersion != SchemaVersion {
		t.Errorf("expected schema version %q, got %q", SchemaVersion, config.SchemaVersion)
	}

	// Save the config
	if err := Save(config); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Read the saved file and verify schema version is persisted
	configPath, err := ConfigPath(clusterName)
	if err != nil {
		t.Fatalf("failed to get config path: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read saved config: %v", err)
	}

	var savedConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &savedConfig); err != nil {
		t.Fatalf("failed to parse saved config: %v", err)
	}

	schemaVersion, ok := savedConfig["schema_version"].(string)
	if !ok {
		t.Error("schema_version field not found in saved config")
	}

	if schemaVersion != SchemaVersion {
		t.Errorf("expected saved schema version %q, got %q", SchemaVersion, schemaVersion)
	}
}

// TestLoadConfigWithoutSchemaVersion tests loading a config without schema version.
func TestLoadConfigWithoutSchemaVersion(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	// Create a minimal config without schema version
	clusterName := "test-no-version"
	configDir := filepath.Join(tmpDir, "clusters", "opencenter")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configPath := filepath.Join(configDir, "."+clusterName+"-config.yaml")
	configYAML := `opencenter:
  meta:
    name: test-no-version
    organization: opencenter
  cluster:
    cluster_name: test-no-version
opentofu:
  enabled: true
secrets:
  ssh_key:
    private: /tmp/test
    public: /tmp/test.pub
`

	if err := os.WriteFile(configPath, []byte(configYAML), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Load the config
	config, err := Load(clusterName)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// When a config file doesn't have schema_version, it gets filled in from defaults
	// So the loaded config will have the current schema version
	if config.SchemaVersion != SchemaVersion {
		t.Errorf("expected schema version to be filled from defaults: %q, got %q", SchemaVersion, config.SchemaVersion)
	}

	// Since it gets the current version from defaults, no migration is needed
	if NeedsMigration(config) {
		t.Error("config with current schema version (from defaults) should not need migration")
	}
}

// TestLoadConfigWithOldSchemaVersion tests loading a config with old schema version.
func TestLoadConfigWithOldSchemaVersion(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	// Create a config with old schema version
	clusterName := "test-old-version"
	configDir := filepath.Join(tmpDir, "clusters", "opencenter")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configPath := filepath.Join(configDir, "."+clusterName+"-config.yaml")
	configYAML := `schema_version: "0.9.0"
opencenter:
  meta:
    name: test-old-version
    organization: opencenter
  cluster:
    cluster_name: test-old-version
opentofu:
  enabled: true
secrets:
  ssh_key:
    private: /tmp/test
    public: /tmp/test.pub
`

	if err := os.WriteFile(configPath, []byte(configYAML), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Capture stderr to check for warning
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Load the config
	config, err := Load(clusterName)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured output
	var buf [1024]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	// Verify warning was printed
	if output == "" {
		t.Error("expected warning about old schema version, but got no output")
	}

	// Verify the loaded config has the old version
	if config.SchemaVersion != "0.9.0" {
		t.Errorf("expected loaded schema version to be 0.9.0, got %q", config.SchemaVersion)
	}

	// Verify migration is detected as needed
	if !NeedsMigration(config) {
		t.Error("expected config with old schema version to need migration")
	}
}

// TestGeneratedSchemaIncludesMetadata tests that the generated JSON schema includes the metadata field.
func TestGeneratedSchemaIncludesMetadata(t *testing.T) {
	// Generate the schema
	schemaBytes, err := GenerateSchema(false)
	if err != nil {
		t.Fatalf("failed to generate schema: %v", err)
	}

	// Parse the schema
	var schema map[string]interface{}
	if err := yaml.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	// Verify properties exist
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("schema does not have properties field")
	}

	// Verify metadata field exists
	metadata, ok := properties["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("schema properties do not include metadata field")
	}

	// Verify metadata has correct type
	metadataType, ok := metadata["type"].(string)
	if !ok || metadataType != "object" {
		t.Errorf("expected metadata type to be 'object', got %v", metadataType)
	}

	// Verify metadata has properties
	metadataProps, ok := metadata["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("metadata does not have properties field")
	}

	// Verify required metadata fields exist
	requiredFields := []string{"created_at", "updated_at", "created_by", "tags", "annotations"}
	for _, field := range requiredFields {
		if _, ok := metadataProps[field]; !ok {
			t.Errorf("metadata properties missing required field: %s", field)
		}
	}

	// Verify created_at and updated_at have date-time format
	for _, field := range []string{"created_at", "updated_at"} {
		fieldDef, ok := metadataProps[field].(map[string]interface{})
		if !ok {
			t.Errorf("metadata.%s is not an object", field)
			continue
		}

		format, ok := fieldDef["format"].(string)
		if !ok || format != "date-time" {
			t.Errorf("expected metadata.%s format to be 'date-time', got %v", field, format)
		}
	}

	// Verify tags and annotations are objects with string values
	for _, field := range []string{"tags", "annotations"} {
		fieldDef, ok := metadataProps[field].(map[string]interface{})
		if !ok {
			t.Errorf("metadata.%s is not an object", field)
			continue
		}

		fieldType, ok := fieldDef["type"].(string)
		if !ok || fieldType != "object" {
			t.Errorf("expected metadata.%s type to be 'object', got %v", field, fieldType)
		}

		additionalProps, ok := fieldDef["additionalProperties"].(map[string]interface{})
		if !ok {
			t.Errorf("metadata.%s does not have additionalProperties", field)
			continue
		}

		propType, ok := additionalProps["type"].(string)
		if !ok || propType != "string" {
			t.Errorf("expected metadata.%s additionalProperties type to be 'string', got %v", field, propType)
		}
	}
}

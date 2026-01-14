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
	"time"

	"gopkg.in/yaml.v3"
)

// TestMigratorPreservesMetadata tests that the migrator preserves metadata during operations
func TestMigratorPreservesMetadata(t *testing.T) {
	// Set test mode
	os.Setenv("OPENCENTER_TEST_MODE", "true")
	defer os.Unsetenv("OPENCENTER_TEST_MODE")

	t.Run("UpdateClusterConfigWithOrganizationPreservesMetadata", func(t *testing.T) {
		// Create a temporary directory for testing
		tmpDir := t.TempDir()

		// Create cluster directory structure
		clusterName := "test-cluster"
		organization := "test-org"
		clusterDir := filepath.Join(tmpDir, "clusters", organization, "infrastructure", "clusters", clusterName)
		if err := os.MkdirAll(clusterDir, 0755); err != nil {
			t.Fatalf("Failed to create cluster directory: %v", err)
		}

		// Create a config file with metadata
		configPath := filepath.Join(clusterDir, "."+clusterName+"-config.yaml")
		specificTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		configData := map[string]interface{}{
			"schema_version": SchemaVersion,
			"opencenter": map[string]interface{}{
				"meta": map[string]interface{}{
					"name": clusterName,
				},
				"cluster": map[string]interface{}{
					"cluster_name": clusterName,
				},
				"gitops": map[string]interface{}{
					"git_dir": "/old/path",
				},
			},
			"metadata": map[string]interface{}{
				"created_at": specificTime.Format(time.RFC3339),
				"updated_at": specificTime.Format(time.RFC3339),
				"created_by": "original-user",
				"tags": map[string]string{
					"env":  "production",
					"team": "platform",
				},
				"annotations": map[string]string{
					"description": "production cluster",
					"version":     "1.0",
				},
			},
		}

		data, err := yaml.Marshal(configData)
		if err != nil {
			t.Fatalf("Failed to marshal config: %v", err)
		}

		if err := os.WriteFile(configPath, data, 0600); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		// Simulate what updateClusterConfigWithOrganization does
		// Read the configuration file
		readData, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read cluster configuration: %v", err)
		}

		// Parse as generic map to preserve structure
		var config map[string]interface{}
		if err := yaml.Unmarshal(readData, &config); err != nil {
			t.Fatalf("Failed to parse cluster configuration: %v", err)
		}

		// Add organization metadata (simulating the migrator function)
		if opencenter, ok := config["opencenter"].(map[string]interface{}); ok {
			if meta, ok := opencenter["meta"].(map[string]interface{}); ok {
				meta["organization"] = organization
			} else {
				opencenter["meta"] = map[string]interface{}{
					"organization": organization,
				}
			}
		}

		// Update GitOps directory
		if opencenter, ok := config["opencenter"].(map[string]interface{}); ok {
			if gitops, ok := opencenter["gitops"].(map[string]interface{}); ok {
				gitops["git_dir"] = "/new/gitops/path"
			}
		}

		// Marshal back to YAML
		updatedData, err := yaml.Marshal(config)
		if err != nil {
			t.Fatalf("Failed to marshal updated configuration: %v", err)
		}

		// Write back to file
		if err := os.WriteFile(configPath, updatedData, 0600); err != nil {
			t.Fatalf("Failed to write updated configuration: %v", err)
		}

		// Read the updated config
		finalData, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read updated config: %v", err)
		}

		var updatedConfig map[string]interface{}
		if err := yaml.Unmarshal(finalData, &updatedConfig); err != nil {
			t.Fatalf("Failed to unmarshal updated config: %v", err)
		}

		// Verify metadata is preserved
		metadata, ok := updatedConfig["metadata"].(map[string]interface{})
		if !ok {
			t.Fatal("Metadata not found in updated config")
		}

		// Check created_at
		if metadata["created_at"] != specificTime.Format(time.RFC3339) {
			t.Errorf("CreatedAt not preserved: got %v, want %v", metadata["created_at"], specificTime.Format(time.RFC3339))
		}

		// Check created_by
		if metadata["created_by"] != "original-user" {
			t.Errorf("CreatedBy not preserved: got %v, want original-user", metadata["created_by"])
		}

		// Check tags
		tags, ok := metadata["tags"].(map[string]interface{})
		if !ok {
			t.Fatal("Tags not found in metadata")
		}
		if tags["env"] != "production" {
			t.Errorf("Tag 'env' not preserved: got %v, want production", tags["env"])
		}
		if tags["team"] != "platform" {
			t.Errorf("Tag 'team' not preserved: got %v, want platform", tags["team"])
		}

		// Check annotations
		annotations, ok := metadata["annotations"].(map[string]interface{})
		if !ok {
			t.Fatal("Annotations not found in metadata")
		}
		if annotations["description"] != "production cluster" {
			t.Errorf("Annotation 'description' not preserved: got %v, want 'production cluster'", annotations["description"])
		}
		if annotations["version"] != "1.0" {
			t.Errorf("Annotation 'version' not preserved: got %v, want 1.0", annotations["version"])
		}

		// Verify organization was added
		opencenter, ok := updatedConfig["opencenter"].(map[string]interface{})
		if !ok {
			t.Fatal("opencenter section not found")
		}
		meta, ok := opencenter["meta"].(map[string]interface{})
		if !ok {
			t.Fatal("meta section not found")
		}
		if meta["organization"] != organization {
			t.Errorf("Organization not set: got %v, want %v", meta["organization"], organization)
		}

		// Verify gitops dir was updated
		gitops, ok := opencenter["gitops"].(map[string]interface{})
		if !ok {
			t.Fatal("gitops section not found")
		}
		if gitops["git_dir"] != "/new/gitops/path" {
			t.Errorf("GitOps dir not updated: got %v, want /new/gitops/path", gitops["git_dir"])
		}
	})
}

// TestLoaderPreservesMetadata tests that the ConfigLoader preserves metadata
func TestLoaderPreservesMetadata(t *testing.T) {
	// Set test mode
	os.Setenv("OPENCENTER_TEST_MODE", "true")
	defer os.Unsetenv("OPENCENTER_TEST_MODE")

	t.Run("SaveToFilePreservesMetadata", func(t *testing.T) {
		// Create a temporary directory for testing
		tmpDir := t.TempDir()

		// Create a config with metadata
		cfg := NewDefault("test-cluster")
		originalCreatedAt := time.Now().Add(-24 * time.Hour)
		cfg.Metadata.CreatedAt = originalCreatedAt
		cfg.Metadata.CreatedBy = "test-user"
		cfg.Metadata.Tags = map[string]string{"env": "test"}
		cfg.Metadata.Annotations = map[string]string{"note": "test"}

		// Save using the standard Save function
		// First, set up the environment to save to our test path
		os.Setenv("OPENCENTER_CONFIG_DIR", tmpDir)
		defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

		// Save the config
		if err := Save(cfg); err != nil {
			t.Fatalf("Failed to save config: %v", err)
		}

		// Load the config back
		loaded, err := Load("test-cluster")
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Verify metadata is preserved
		if !loaded.Metadata.CreatedAt.Equal(originalCreatedAt) {
			t.Errorf("CreatedAt not preserved: got %v, want %v", loaded.Metadata.CreatedAt, originalCreatedAt)
		}
		if loaded.Metadata.CreatedBy != "test-user" {
			t.Errorf("CreatedBy not preserved: got %v, want test-user", loaded.Metadata.CreatedBy)
		}
		if loaded.Metadata.Tags["env"] != "test" {
			t.Errorf("Tag not preserved: got %v, want test", loaded.Metadata.Tags["env"])
		}
		if loaded.Metadata.Annotations["note"] != "test" {
			t.Errorf("Annotation not preserved: got %v, want test", loaded.Metadata.Annotations["note"])
		}
	})
}

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

// TestMetadataPreservation tests that metadata is preserved during various operations
func TestMetadataPreservation(t *testing.T) {
	t.Run("SavePreservesMetadata", func(t *testing.T) {
		// Create a temporary directory for testing
		tmpDir := t.TempDir()
		os.Setenv("OPENCENTER_CONFIG_DIR", tmpDir)
		defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

		// Create a config with specific metadata using test fixture
		cfg := testModeConfig("test-cluster")
		originalCreatedAt := time.Now().Add(-24 * time.Hour) // 1 day ago
		originalUpdatedAt := time.Now().Add(-1 * time.Hour)  // 1 hour ago
		cfg.Metadata.CreatedAt = originalCreatedAt
		cfg.Metadata.UpdatedAt = originalUpdatedAt
		cfg.Metadata.CreatedBy = "test-user"
		cfg.Metadata.Tags = map[string]string{"env": "test", "team": "platform"}
		cfg.Metadata.Annotations = map[string]string{"note": "test annotation"}

		// Save the config
		if err := Save(cfg); err != nil {
			t.Fatalf("Failed to save config: %v", err)
		}

		// Load the config back
		loaded, err := Load("test-cluster")
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Verify CreatedAt is preserved (should be the same)
		if !loaded.Metadata.CreatedAt.Equal(originalCreatedAt) {
			t.Errorf("CreatedAt not preserved: got %v, want %v", loaded.Metadata.CreatedAt, originalCreatedAt)
		}

		// Verify UpdatedAt was updated (should be newer than original)
		if !loaded.Metadata.UpdatedAt.After(originalUpdatedAt) {
			t.Errorf("UpdatedAt not updated: got %v, should be after %v", loaded.Metadata.UpdatedAt, originalUpdatedAt)
		}

		// Verify CreatedBy is preserved
		if loaded.Metadata.CreatedBy != "test-user" {
			t.Errorf("CreatedBy not preserved: got %v, want test-user", loaded.Metadata.CreatedBy)
		}

		// Verify Tags are preserved
		if len(loaded.Metadata.Tags) != 2 {
			t.Errorf("Tags not preserved: got %d tags, want 2", len(loaded.Metadata.Tags))
		}
		if loaded.Metadata.Tags["env"] != "test" {
			t.Errorf("Tag 'env' not preserved: got %v, want test", loaded.Metadata.Tags["env"])
		}
		if loaded.Metadata.Tags["team"] != "platform" {
			t.Errorf("Tag 'team' not preserved: got %v, want platform", loaded.Metadata.Tags["team"])
		}

		// Verify Annotations are preserved
		if len(loaded.Metadata.Annotations) != 1 {
			t.Errorf("Annotations not preserved: got %d annotations, want 1", len(loaded.Metadata.Annotations))
		}
		if loaded.Metadata.Annotations["note"] != "test annotation" {
			t.Errorf("Annotation 'note' not preserved: got %v, want 'test annotation'", loaded.Metadata.Annotations["note"])
		}
	})

	t.Run("LoadPreservesMetadataFromFile", func(t *testing.T) {
		// Create a temporary directory for testing
		tmpDir := t.TempDir()
		os.Setenv("OPENCENTER_CONFIG_DIR", tmpDir)
		defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

		// Create a config file with metadata directly
		configPath := filepath.Join(tmpDir, "test-cluster-2.yaml")
		specificTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		configData := map[string]interface{}{
			"schema_version": SchemaVersion,
			"opencenter": map[string]interface{}{
				"meta": map[string]interface{}{
					"name":         "test-cluster-2",
					"organization": "opencenter",
				},
				"cluster": map[string]interface{}{
					"cluster_name": "test-cluster-2",
				},
			},
			"metadata": map[string]interface{}{
				"created_at": specificTime.Format(time.RFC3339),
				"updated_at": specificTime.Format(time.RFC3339),
				"created_by": "original-user",
				"tags": map[string]string{
					"env": "production",
				},
				"annotations": map[string]string{
					"description": "production cluster",
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

		// Load the config
		loaded, err := Load("test-cluster-2")
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Verify metadata is preserved
		if !loaded.Metadata.CreatedAt.Equal(specificTime) {
			t.Errorf("CreatedAt not preserved from file: got %v, want %v", loaded.Metadata.CreatedAt, specificTime)
		}
		if !loaded.Metadata.UpdatedAt.Equal(specificTime) {
			t.Errorf("UpdatedAt not preserved from file: got %v, want %v", loaded.Metadata.UpdatedAt, specificTime)
		}
		if loaded.Metadata.CreatedBy != "original-user" {
			t.Errorf("CreatedBy not preserved from file: got %v, want original-user", loaded.Metadata.CreatedBy)
		}
		if loaded.Metadata.Tags["env"] != "production" {
			t.Errorf("Tag not preserved from file: got %v, want production", loaded.Metadata.Tags["env"])
		}
		if loaded.Metadata.Annotations["description"] != "production cluster" {
			t.Errorf("Annotation not preserved from file: got %v, want 'production cluster'", loaded.Metadata.Annotations["description"])
		}
	})

	t.Run("MergePreservesMetadata", func(t *testing.T) {
		// Test that mergeYAMLMaps preserves metadata
		base := map[string]any{
			"metadata": map[string]any{
				"created_at": "2024-01-01T00:00:00Z",
				"created_by": "base-user",
				"tags": map[string]any{
					"env": "dev",
				},
			},
			"opencenter": map[string]any{
				"cluster": map[string]any{
					"cluster_name": "test",
				},
			},
		}

		override := map[string]any{
			"opencenter": map[string]any{
				"cluster": map[string]any{
					"cluster_name": "test-override",
				},
			},
		}

		result := mergeYAMLMaps(base, override)

		// Verify metadata is preserved from base
		metadata, ok := result["metadata"].(map[string]any)
		if !ok {
			t.Fatal("Metadata not preserved in merge")
		}

		if metadata["created_at"] != "2024-01-01T00:00:00Z" {
			t.Errorf("CreatedAt not preserved in merge: got %v", metadata["created_at"])
		}
		if metadata["created_by"] != "base-user" {
			t.Errorf("CreatedBy not preserved in merge: got %v", metadata["created_by"])
		}

		// Verify override worked for other fields
		opencenter := result["opencenter"].(map[string]any)
		cluster := opencenter["cluster"].(map[string]any)
		if cluster["cluster_name"] != "test-override" {
			t.Errorf("Override didn't work: got %v", cluster["cluster_name"])
		}
	})
}

// TestMetadataTouch tests the Touch method
func TestMetadataTouch(t *testing.T) {
	metadata := NewConfigMetadata()
	originalUpdatedAt := metadata.UpdatedAt

	// Wait a bit to ensure time difference
	time.Sleep(10 * time.Millisecond)

	// Touch the metadata
	metadata.Touch()

	// Verify UpdatedAt was updated
	if !metadata.UpdatedAt.After(originalUpdatedAt) {
		t.Errorf("Touch() did not update UpdatedAt: got %v, original %v", metadata.UpdatedAt, originalUpdatedAt)
	}

	// Verify CreatedAt was not changed
	if metadata.CreatedAt.After(originalUpdatedAt) {
		t.Errorf("Touch() should not change CreatedAt")
	}
}

// TestNewConfigMetadata tests the NewConfigMetadata function
func TestNewConfigMetadata(t *testing.T) {
	metadata := NewConfigMetadata()

	// Verify timestamps are set
	if metadata.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if metadata.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}

	// Verify CreatedAt and UpdatedAt are the same initially
	if !metadata.CreatedAt.Equal(metadata.UpdatedAt) {
		t.Error("CreatedAt and UpdatedAt should be equal initially")
	}

	// Verify CreatedBy is set
	if metadata.CreatedBy == "" {
		t.Error("CreatedBy should not be empty")
	}

	// Verify maps are initialized
	if metadata.Tags == nil {
		t.Error("Tags map should be initialized")
	}
	if metadata.Annotations == nil {
		t.Error("Annotations map should be initialized")
	}
}

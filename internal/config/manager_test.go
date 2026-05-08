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
	"context"
	"os"
	"path/filepath"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// TestNewConfigurationManager verifies manager creation
func TestNewConfigurationManager(t *testing.T) {
	manager, err := NewConfigurationManager()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if manager == nil {
		t.Fatal("Manager is nil")
	}

	if manager.loader == nil {
		t.Error("Loader is nil")
	}

	if manager.validator == nil {
		t.Error("Validator is nil")
	}

	if manager.cache == nil {
		t.Error("Cache is nil")
	}

	if manager.pathResolver == nil {
		t.Error("PathResolver is nil")
	}

	if manager.fileSystem == nil {
		t.Error("FileSystem is nil")
	}
}

// TestConfigurationManager_LoadNonExistent verifies error handling for non-existent configs
func TestConfigurationManager_LoadNonExistent(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create manager with test directory
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	pathResolver := paths.NewPathResolver(tmpDir)
	validator := validation.NewValidationEngine()
	cache := NewConfigCache()
	loader := NewConfigIOHandler(fileSystem)

	manager := NewConfigurationManagerWithDeps(loader, validator, cache, pathResolver, fileSystem)

	ctx := context.Background()

	// Try to load non-existent cluster
	_, err := manager.Load(ctx, "non-existent-cluster")
	if err == nil {
		t.Fatal("Expected error for non-existent cluster, got nil")
	}

	// Verify it's a path error
	if structuredErr, ok := err.(*errors.StructuredError); ok {
		if structuredErr.Type != errors.PathError && structuredErr.Type != errors.FileError {
			t.Errorf("Expected PathError or FileError, got %v", structuredErr.Type)
		}
	}
}

// TestConfigurationManager_ValidateNil verifies nil config validation
func TestConfigurationManager_ValidateNil(t *testing.T) {
	manager, err := NewConfigurationManager()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()

	err = manager.Validate(ctx, nil)
	if err == nil {
		t.Fatal("Expected error for nil config, got nil")
	}

	if structuredErr, ok := err.(*errors.StructuredError); ok {
		if structuredErr.Type != errors.ValidationError {
			t.Errorf("Expected ValidationError, got %v", structuredErr.Type)
		}
	}
}

// TestConfigurationManager_SaveNil verifies nil config save handling
func TestConfigurationManager_SaveNil(t *testing.T) {
	manager, err := NewConfigurationManager()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()

	err = manager.Save(ctx, nil)
	if err == nil {
		t.Fatal("Expected error for nil config, got nil")
	}
}

// TestConfigurationManager_DeleteNonExistent verifies delete error handling
func TestConfigurationManager_DeleteNonExistent(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create manager with test directory
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	pathResolver := paths.NewPathResolver(tmpDir)
	validator := validation.NewValidationEngine()
	cache := NewConfigCache()
	loader := NewConfigIOHandler(fileSystem)

	manager := NewConfigurationManagerWithDeps(loader, validator, cache, pathResolver, fileSystem)

	ctx := context.Background()

	// Try to delete non-existent cluster
	err := manager.Delete(ctx, "non-existent-cluster")
	if err == nil {
		t.Fatal("Expected error for non-existent cluster, got nil")
	}
}

// TestConfigurationManager_ListEmpty verifies empty list handling
func TestConfigurationManager_ListEmpty(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create manager with test directory
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	pathResolver := paths.NewPathResolver(tmpDir)
	validator := validation.NewValidationEngine()
	cache := NewConfigCache()
	loader := NewConfigIOHandler(fileSystem)

	manager := NewConfigurationManagerWithDeps(loader, validator, cache, pathResolver, fileSystem)

	ctx := context.Background()

	// List clusters in empty directory
	clusters, err := manager.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list clusters: %v", err)
	}

	if len(clusters) != 0 {
		t.Errorf("Expected empty list, got %d clusters", len(clusters))
	}
}

// TestConfigurationManager_CacheOperations verifies cache operations
func TestConfigurationManager_CacheOperations(t *testing.T) {
	manager, err := NewConfigurationManager()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()

	// Clear cache
	err = manager.ClearCache(ctx)
	if err != nil {
		t.Errorf("ClearCache failed: %v", err)
	}

	// Invalidate specific cluster
	err = manager.InvalidateCluster(ctx, "test-cluster")
	if err != nil {
		t.Errorf("InvalidateCluster failed: %v", err)
	}
}

// TestConfigurationManager_ListWithOrganization verifies organization filtering
func TestConfigurationManager_ListWithOrganization(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	createSecureConfigTestCluster(t, tmpDir, "test-org", "test-cluster")

	// Create manager with test directory
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	pathResolver := paths.NewPathResolver(tmpDir)
	validator := validation.NewValidationEngine()
	cache := NewConfigCache()
	loader := NewConfigIOHandler(fileSystem)

	manager := NewConfigurationManagerWithDeps(loader, validator, cache, pathResolver, fileSystem)

	ctx := context.Background()

	// List all clusters
	clusters, err := manager.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list clusters: %v", err)
	}

	if len(clusters) != 1 {
		t.Errorf("Expected 1 cluster, got %d", len(clusters))
	}

	if len(clusters) > 0 && clusters[0] != "test-org/test-cluster" {
		t.Errorf("Expected cluster name 'test-org/test-cluster', got %s", clusters[0])
	}

	// List with organization filter
	clusters, err = manager.ListWithOrganization(ctx, "test-org")
	if err != nil {
		t.Fatalf("Failed to list clusters with org filter: %v", err)
	}

	if len(clusters) != 1 {
		t.Errorf("Expected 1 cluster with org filter, got %d", len(clusters))
	}

	// List with non-existent organization
	clusters, err = manager.ListWithOrganization(ctx, "non-existent-org")
	if err != nil {
		t.Fatalf("Failed to list clusters with non-existent org: %v", err)
	}

	if len(clusters) != 0 {
		t.Errorf("Expected 0 clusters for non-existent org, got %d", len(clusters))
	}
}

func TestConfigurationManager_ListRejectsLegacyMixedLayout(t *testing.T) {
	tmpDir := t.TempDir()
	createSecureConfigTestCluster(t, tmpDir, "test-org", "test-cluster")

	legacyOrgDir := filepath.Join(tmpDir, "legacy-org")
	if err := os.MkdirAll(filepath.Join(legacyOrgDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyOrgDir, ".legacy-cluster-config.yaml"), []byte("schema_version: '2.0'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	pathResolver := paths.NewPathResolver(tmpDir)
	validator := validation.NewValidationEngine()
	cache := NewConfigCache()
	loader := NewConfigIOHandler(fileSystem)
	manager := NewConfigurationManagerWithDeps(loader, validator, cache, pathResolver, fileSystem)

	if _, err := manager.List(context.Background()); err == nil {
		t.Fatal("List() error = nil, want legacy layout error")
	} else if _, ok := err.(*paths.LegacyLayoutError); !ok {
		t.Fatalf("List() error = %T %v, want LegacyLayoutError", err, err)
	}

	if _, err := manager.ListWithOrganization(context.Background(), "legacy-org"); err == nil {
		t.Fatal("ListWithOrganization() error = nil, want legacy layout error")
	} else if _, ok := err.(*paths.LegacyLayoutError); !ok {
		t.Fatalf("ListWithOrganization() error = %T %v, want LegacyLayoutError", err, err)
	}
}

// TestConfigurationManager_ListDiscoversConfigFiles verifies that clusters
// are discovered from config files even when no infrastructure directory exists.
func TestConfigurationManager_ListDiscoversConfigFiles(t *testing.T) {
	tmpDir := t.TempDir()

	configFile := createSecureConfigTestCluster(t, tmpDir, "test-org", "config-only-cluster")
	if err := os.WriteFile(configFile, []byte("schema_version: '2.0'\n"), 0600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	pathResolver := paths.NewPathResolver(tmpDir)
	validator := validation.NewValidationEngine()
	cache := NewConfigCache()
	loader := NewConfigIOHandler(fileSystem)
	manager := NewConfigurationManagerWithDeps(loader, validator, cache, pathResolver, fileSystem)

	ctx := context.Background()

	// List all — should find the config-only cluster
	clusters, err := manager.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d: %v", len(clusters), clusters)
	}
	if clusters[0] != "test-org/config-only-cluster" {
		t.Errorf("expected test-org/config-only-cluster, got %s", clusters[0])
	}

	// List with organization filter
	clusters, err = manager.ListWithOrganization(ctx, "test-org")
	if err != nil {
		t.Fatalf("ListWithOrganization failed: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d: %v", len(clusters), clusters)
	}
	if clusters[0] != "config-only-cluster" {
		t.Errorf("expected config-only-cluster, got %s", clusters[0])
	}
}

// TestConfigurationManager_ListMergesDirectoriesAndConfigFiles verifies that
// clusters from both infrastructure directories and config files are merged
// without duplicates.
func TestConfigurationManager_ListMergesDirectoriesAndConfigFiles(t *testing.T) {
	tmpDir := t.TempDir()

	bothConfig := createSecureConfigTestCluster(t, tmpDir, "my-org", "both-cluster")
	if err := os.WriteFile(bothConfig, []byte("schema_version: '2.0'\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Create a cluster with only a directory (no config file)
	dirOnlyCluster := filepath.Join(tmpDir, "state", "my-org", "dir-only")
	if err := os.MkdirAll(dirOnlyCluster, 0755); err != nil {
		t.Fatalf("create dir-only cluster: %v", err)
	}

	// Create a cluster with only a config file (no directory)
	configOnlyFile := createSecureConfigTestCluster(t, tmpDir, "my-org", "config-only")
	if err := os.WriteFile(configOnlyFile, []byte("schema_version: '2.0'\n"), 0600); err != nil {
		t.Fatalf("write config-only file: %v", err)
	}

	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	pathResolver := paths.NewPathResolver(tmpDir)
	validator := validation.NewValidationEngine()
	cache := NewConfigCache()
	loader := NewConfigIOHandler(fileSystem)
	manager := NewConfigurationManagerWithDeps(loader, validator, cache, pathResolver, fileSystem)

	ctx := context.Background()

	clusters, err := manager.ListWithOrganization(ctx, "my-org")
	if err != nil {
		t.Fatalf("ListWithOrganization failed: %v", err)
	}

	expected := []string{"both-cluster", "config-only", "dir-only"}
	if len(clusters) != len(expected) {
		t.Fatalf("expected %d clusters, got %d: %v", len(expected), len(clusters), clusters)
	}
	for i, name := range expected {
		if clusters[i] != name {
			t.Errorf("clusters[%d] = %q, want %q", i, clusters[i], name)
		}
	}
}

// TestParseConfigFileName verifies the config file name parsing helper.
func TestParseConfigFileName(t *testing.T) {
	tests := []struct {
		filename string
		wantName string
		wantOK   bool
	}{
		{"my-cluster-config.yaml", "my-cluster", true},
		{"talos-test-01-config.yaml", "talos-test-01", true},
		{"a-config.yaml", "a", true},
		{".my-cluster-config.yaml", "", false},       // legacy dotted config
		{"config.yaml", "", false},                   // no cluster prefix
		{".sops.yaml", "", false},                    // unrelated dotfile
		{"README.md", "", false},                     // unrelated file
		{"-config.yaml", "", false},                  // empty cluster name
		{"my-cluster-config.yaml.backup", "", false}, // backup file
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			name, ok := parseConfigFileName(tt.filename)
			if ok != tt.wantOK {
				t.Errorf("parseConfigFileName(%q) ok = %v, want %v", tt.filename, ok, tt.wantOK)
			}
			if name != tt.wantName {
				t.Errorf("parseConfigFileName(%q) name = %q, want %q", tt.filename, name, tt.wantName)
			}
		})
	}
}

// TestConfigurationManager_DeleteWithBackup verifies delete creates backup and invalidates cache
func TestConfigurationManager_DeleteWithBackup(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Create a test config file
	configPath := createSecureConfigTestCluster(t, tmpDir, "test-org", "test-cluster")
	testContent := []byte("test: config\ndata: value")
	if err := os.WriteFile(configPath, testContent, 0600); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Create manager with test directory
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	pathResolver := paths.NewPathResolver(tmpDir)
	validator := validation.NewValidationEngine()
	cache := NewConfigCache()
	loader := NewConfigIOHandler(fileSystem)

	manager := NewConfigurationManagerWithDeps(loader, validator, cache, pathResolver, fileSystem)

	ctx := context.Background()

	// Add to cache to verify invalidation
	testConfig := &v2.Config{}
	cache.Set(ctx, "test-cluster", testConfig)

	// Verify it's in cache
	if _, found := cache.Get(ctx, "test-cluster"); !found {
		t.Fatal("Config should be in cache before delete")
	}

	// Delete the cluster
	err := manager.Delete(ctx, "test-cluster")
	if err != nil {
		t.Fatalf("Failed to delete cluster: %v", err)
	}

	// Verify original file is deleted
	if fileSystem.Exists(configPath) {
		t.Error("Original config file should be deleted")
	}

	// Verify backup was created
	backupPath := configPath + ".deleted"
	if !fileSystem.Exists(backupPath) {
		t.Error("Backup file should exist")
	}

	// Verify backup content matches original
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("Failed to read backup: %v", err)
	}

	if string(backupContent) != string(testContent) {
		t.Errorf("Backup content mismatch. Expected %q, got %q", testContent, backupContent)
	}

	// Verify cache was invalidated
	if _, found := cache.Get(ctx, "test-cluster"); found {
		t.Error("Config should be removed from cache after delete")
	}
}

// TestConfigurationManager_ListMultipleOrganizations verifies listing across multiple organizations
func TestConfigurationManager_ListMultipleOrganizations(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Create multiple organizations with clusters
	orgs := []struct {
		name     string
		clusters []string
	}{
		{"org1", []string{"cluster1", "cluster2"}},
		{"org2", []string{"cluster3"}},
		{"org3", []string{"cluster4", "cluster5", "cluster6"}},
	}

	for _, org := range orgs {
		for _, cluster := range org.clusters {
			createSecureConfigTestCluster(t, tmpDir, org.name, cluster)
		}
	}

	// Create manager with test directory
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	pathResolver := paths.NewPathResolver(tmpDir)
	validator := validation.NewValidationEngine()
	cache := NewConfigCache()
	loader := NewConfigIOHandler(fileSystem)

	manager := NewConfigurationManagerWithDeps(loader, validator, cache, pathResolver, fileSystem)

	ctx := context.Background()

	// List all clusters
	clusters, err := manager.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list clusters: %v", err)
	}

	// Should have 6 total clusters
	if len(clusters) != 6 {
		t.Errorf("Expected 6 clusters, got %d", len(clusters))
	}

	// Verify clusters are in organization/cluster format
	expectedClusters := map[string]bool{
		"org1/cluster1": true,
		"org1/cluster2": true,
		"org2/cluster3": true,
		"org3/cluster4": true,
		"org3/cluster5": true,
		"org3/cluster6": true,
	}

	for _, cluster := range clusters {
		if !expectedClusters[cluster] {
			t.Errorf("Unexpected cluster in list: %s", cluster)
		}
	}

	// List org1 clusters
	org1Clusters, err := manager.ListWithOrganization(ctx, "org1")
	if err != nil {
		t.Fatalf("Failed to list org1 clusters: %v", err)
	}

	if len(org1Clusters) != 2 {
		t.Errorf("Expected 2 clusters in org1, got %d", len(org1Clusters))
	}

	// List org2 clusters
	org2Clusters, err := manager.ListWithOrganization(ctx, "org2")
	if err != nil {
		t.Fatalf("Failed to list org2 clusters: %v", err)
	}

	if len(org2Clusters) != 1 {
		t.Errorf("Expected 1 cluster in org2, got %d", len(org2Clusters))
	}

	// List org3 clusters
	org3Clusters, err := manager.ListWithOrganization(ctx, "org3")
	if err != nil {
		t.Fatalf("Failed to list org3 clusters: %v", err)
	}

	if len(org3Clusters) != 3 {
		t.Errorf("Expected 3 clusters in org3, got %d", len(org3Clusters))
	}
}

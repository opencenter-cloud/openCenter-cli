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

func TestPathResolver_ResolveClusterPaths(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	
	// Create a test configuration manager
	config := DefaultCLIConfig()
	config.Paths.ClustersDir = filepath.Join(tempDir, "clusters")
	
	cm, err := NewConfigManagerWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	pr := NewPathResolver(cm)
	
	tests := []struct {
		name         string
		clusterName  string
		organization string
		wantOrg      string
	}{
		{
			name:         "with organization",
			clusterName:  "test-cluster",
			organization: "rackspace",
			wantOrg:      "rackspace",
		},
		{
			name:         "without organization defaults to opencenter",
			clusterName:  "test-cluster",
			organization: "",
			wantOrg:      "opencenter",
		},
		{
			name:         "with different organization",
			clusterName:  "prod-cluster",
			organization: "aws-dev",
			wantOrg:      "aws-dev",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := pr.ResolveClusterPaths(tt.clusterName, tt.organization)
			
			expectedBase := filepath.Join(tempDir, "clusters", tt.wantOrg)
			
			// Verify organization directory
			if paths.OrganizationDir != expectedBase {
				t.Errorf("OrganizationDir = %v, want %v", paths.OrganizationDir, expectedBase)
			}
			
			// Verify GitOps directory (same as organization)
			if paths.GitOpsDir != expectedBase {
				t.Errorf("GitOpsDir = %v, want %v", paths.GitOpsDir, expectedBase)
			}
			
			// Verify cluster directory
			expectedClusterDir := filepath.Join(expectedBase, "infrastructure", "clusters", tt.clusterName)
			if paths.ClusterDir != expectedClusterDir {
				t.Errorf("ClusterDir = %v, want %v", paths.ClusterDir, expectedClusterDir)
			}
			
			// Verify applications directory
			expectedAppsDir := filepath.Join(expectedBase, "applications", "overlays", tt.clusterName)
			if paths.ApplicationsDir != expectedAppsDir {
				t.Errorf("ApplicationsDir = %v, want %v", paths.ApplicationsDir, expectedAppsDir)
			}
			
			// Verify secrets directory
			expectedSecretsDir := filepath.Join(expectedBase, "secrets")
			if paths.SecretsDir != expectedSecretsDir {
				t.Errorf("SecretsDir = %v, want %v", paths.SecretsDir, expectedSecretsDir)
			}
			
			// Verify SOPS key path
			expectedSOPSKey := filepath.Join(expectedSecretsDir, "age", "keys", tt.clusterName+"-key.txt")
			if paths.SOPSKeyPath != expectedSOPSKey {
				t.Errorf("SOPSKeyPath = %v, want %v", paths.SOPSKeyPath, expectedSOPSKey)
			}
			
			// Verify kubeconfig path
			expectedKubeconfig := filepath.Join(expectedClusterDir, "kubeconfig.yaml")
			if paths.KubeconfigPath != expectedKubeconfig {
				t.Errorf("KubeconfigPath = %v, want %v", paths.KubeconfigPath, expectedKubeconfig)
			}
		})
	}
}

func TestPathResolver_CreateOrganizationStructure(t *testing.T) {
	tempDir := t.TempDir()
	
	config := DefaultCLIConfig()
	config.Paths.ClustersDir = filepath.Join(tempDir, "clusters")
	
	cm, err := NewConfigManagerWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	pr := NewPathResolver(cm)
	
	// Test creating organization structure
	err = pr.CreateOrganizationStructure("test-org")
	if err != nil {
		t.Fatalf("Failed to create organization structure: %v", err)
	}
	
	// Verify directories were created
	expectedDirs := []string{
		filepath.Join(tempDir, "clusters", "test-org"),
		filepath.Join(tempDir, "clusters", "test-org", "applications", "overlays"),
		filepath.Join(tempDir, "clusters", "test-org", "infrastructure", "clusters"),
		filepath.Join(tempDir, "clusters", "test-org", "secrets", "age", "keys"),
	}
	
	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Expected directory %s was not created", dir)
		}
	}
}

func TestPathResolver_CreateClusterDirectories(t *testing.T) {
	tempDir := t.TempDir()
	
	config := DefaultCLIConfig()
	config.Paths.ClustersDir = filepath.Join(tempDir, "clusters")
	
	cm, err := NewConfigManagerWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	pr := NewPathResolver(cm)
	
	// Test creating cluster directories
	err = pr.CreateClusterDirectories("test-cluster", "test-org")
	if err != nil {
		t.Fatalf("Failed to create cluster directories: %v", err)
	}
	
	paths := pr.ResolveClusterPaths("test-cluster", "test-org")
	
	// Verify cluster-specific directories were created
	expectedDirs := []string{
		paths.ClusterDir,
		paths.ApplicationsDir,
		paths.InventoryPath,
		paths.VenvPath,
		paths.BinPath,
		filepath.Dir(paths.SOPSKeyPath),
	}
	
	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Expected directory %s was not created", dir)
		}
	}
}

func TestPathResolver_ValidatePath(t *testing.T) {
	config := DefaultCLIConfig()
	cm, err := NewConfigManagerWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	pr := NewPathResolver(cm)
	
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
		},
		{
			name:    "path with traversal",
			path:    "/tmp/../etc/passwd",
			wantErr: true,
		},
		{
			name:    "valid absolute path",
			path:    "/tmp/test",
			wantErr: false,
		},
		{
			name:    "path with tilde",
			path:    "~/test",
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pr.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPathResolver_ExpandPath(t *testing.T) {
	config := DefaultCLIConfig()
	cm, err := NewConfigManagerWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	pr := NewPathResolver(cm)
	
	// Test environment variable expansion
	os.Setenv("TEST_VAR", "test_value")
	defer os.Unsetenv("TEST_VAR")
	
	result := pr.ExpandPath("${TEST_VAR}/path")
	expected := "test_value/path"
	if result != expected {
		t.Errorf("ExpandPath() = %v, want %v", result, expected)
	}
	
	// Test tilde expansion
	home, _ := os.UserHomeDir()
	result = pr.ExpandPath("~/test")
	expected = filepath.Join(home, "test")
	if result != expected {
		t.Errorf("ExpandPath() = %v, want %v", result, expected)
	}
}

func TestMigrationManager_DetectLegacyStructure(t *testing.T) {
	tempDir := t.TempDir()
	
	config := DefaultCLIConfig()
	config.Paths.ClustersDir = filepath.Join(tempDir, "clusters")
	
	cm, err := NewConfigManagerWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	pr := NewPathResolver(cm)
	mm := NewMigrationManager(pr, cm)
	
	// Create a legacy cluster structure
	legacyClusterDir := filepath.Join(tempDir, "clusters", "legacy-cluster")
	err = os.MkdirAll(legacyClusterDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create legacy cluster directory: %v", err)
	}
	
	// Create legacy config file
	legacyConfigPath := filepath.Join(legacyClusterDir, ".legacy-cluster-config.yaml")
	err = os.WriteFile(legacyConfigPath, []byte("test: config"), 0600)
	if err != nil {
		t.Fatalf("Failed to create legacy config file: %v", err)
	}
	
	// Create an organization-based cluster (should not be detected as legacy)
	orgClusterDir := filepath.Join(tempDir, "clusters", "test-org", "infrastructure", "clusters", "org-cluster")
	err = os.MkdirAll(orgClusterDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create org cluster directory: %v", err)
	}
	
	// Detect legacy clusters
	legacyClusters, err := mm.DetectLegacyStructure()
	if err != nil {
		t.Fatalf("Failed to detect legacy structure: %v", err)
	}
	
	// Should find the legacy cluster but not the organization-based one
	if len(legacyClusters) != 1 {
		t.Errorf("Expected 1 legacy cluster, found %d", len(legacyClusters))
	}
	
	if len(legacyClusters) > 0 && legacyClusters[0] != "legacy-cluster" {
		t.Errorf("Expected legacy cluster 'legacy-cluster', found '%s'", legacyClusters[0])
	}
}

func TestPathResolver_OrganizationAwarePaths(t *testing.T) {
	tempDir := t.TempDir()
	
	config := DefaultCLIConfig()
	config.Paths.ClustersDir = filepath.Join(tempDir, "clusters")
	
	cm, err := NewConfigManagerWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	pr := NewPathResolver(cm)
	
	// Test organization-aware cluster directory path
	clusterName := "test-cluster"
	organization := "test-org"
	
	// Create organization structure first
	err = pr.CreateOrganizationStructure(organization)
	if err != nil {
		t.Fatalf("Failed to create organization structure: %v", err)
	}
	
	// Create cluster directories
	err = pr.CreateClusterDirectories(clusterName, organization)
	if err != nil {
		t.Fatalf("Failed to create cluster directories: %v", err)
	}
	
	// Create a cluster config file to make it detectable
	paths := pr.ResolveClusterPaths(clusterName, organization)
	configPath := filepath.Join(paths.ClusterDir, "."+clusterName+"-config.yaml")
	configContent := `
opencenter:
  meta:
    organization: ` + organization + `
  cluster:
    cluster_name: ` + clusterName + `
  gitops:
    git_dir: ` + paths.GitOpsDir + `
`
	err = os.WriteFile(configPath, []byte(configContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create cluster config: %v", err)
	}
	
	// Test organization-aware path resolution
	resolvedPath, err := pr.OrganizationAwareClusterDirectoryPath(clusterName)
	if err != nil {
		t.Fatalf("Failed to resolve organization-aware path: %v", err)
	}
	
	expectedPath := filepath.Join(tempDir, "clusters", organization, "infrastructure", "clusters", clusterName)
	if resolvedPath != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, resolvedPath)
	}
	
	// Test organization-aware config path
	configPathResolved, err := pr.OrganizationAwareConfigPath(clusterName)
	if err != nil {
		t.Fatalf("Failed to resolve organization-aware config path: %v", err)
	}
	
	expectedConfigPath := filepath.Join(expectedPath, "."+clusterName+"-config.yaml")
	if configPathResolved != expectedConfigPath {
		t.Errorf("Expected config path %s, got %s", expectedConfigPath, configPathResolved)
	}
	
	// Test organization-aware secrets path
	secretsPath, err := pr.OrganizationAwareSecretsPath(clusterName)
	if err != nil {
		t.Fatalf("Failed to resolve organization-aware secrets path: %v", err)
	}
	
	expectedSecretsPath := filepath.Join(tempDir, "clusters", organization, "secrets", "age", "keys")
	if secretsPath != expectedSecretsPath {
		t.Errorf("Expected secrets path %s, got %s", expectedSecretsPath, secretsPath)
	}
}

func TestPathResolver_LegacyFallback(t *testing.T) {
	tempDir := t.TempDir()
	
	// Set environment variable to use our temp directory
	os.Setenv("OPENCENTER_CONFIG_DIR", tempDir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")
	
	config := DefaultCLIConfig()
	config.Paths.ClustersDir = filepath.Join(tempDir, "clusters")
	
	cm, err := NewConfigManagerWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	pr := NewPathResolver(cm)
	
	// Create a legacy cluster structure
	clusterName := "legacy-cluster"
	legacyClusterDir := filepath.Join(tempDir, "clusters", clusterName)
	err = os.MkdirAll(legacyClusterDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create legacy cluster directory: %v", err)
	}
	
	// Create legacy config file
	legacyConfigPath := filepath.Join(legacyClusterDir, "."+clusterName+"-config.yaml")
	err = os.WriteFile(legacyConfigPath, []byte("test: config"), 0600)
	if err != nil {
		t.Fatalf("Failed to create legacy config file: %v", err)
	}
	
	// Test that organization-aware path resolution falls back to legacy
	resolvedPath, err := pr.OrganizationAwareClusterDirectoryPath(clusterName)
	if err != nil {
		t.Fatalf("Failed to resolve path for legacy cluster: %v", err)
	}
	
	expectedPath := legacyClusterDir
	if resolvedPath != expectedPath {
		t.Errorf("Expected legacy path %s, got %s", expectedPath, resolvedPath)
	}
}

func TestMigrationManager_MigrateClusterToOrganization(t *testing.T) {
	tempDir := t.TempDir()
	
	config := DefaultCLIConfig()
	config.Paths.ClustersDir = filepath.Join(tempDir, "clusters")
	
	cm, err := NewConfigManagerWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	pr := NewPathResolver(cm)
	mm := NewMigrationManager(pr, cm)
	
	// Create a legacy cluster structure with files
	clusterName := "legacy-cluster"
	organization := "migrated-org"
	legacyClusterDir := filepath.Join(tempDir, "clusters", clusterName)
	
	// Create legacy directories and files
	dirs := []string{
		legacyClusterDir,
		filepath.Join(legacyClusterDir, "inventory"),
		filepath.Join(legacyClusterDir, "venv"),
		filepath.Join(legacyClusterDir, ".bin"),
		filepath.Join(legacyClusterDir, "secrets", "age", "keys"),
	}
	
	for _, dir := range dirs {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}
	
	// Create legacy files
	files := map[string]string{
		filepath.Join(legacyClusterDir, "."+clusterName+"-config.yaml"): `
opencenter:
  cluster:
    cluster_name: ` + clusterName + `
  gitops:
    git_dir: /old/path
`,
		filepath.Join(legacyClusterDir, "kubeconfig.yaml"):                     "kubeconfig content",
		filepath.Join(legacyClusterDir, "main.tf"):                            "terraform content",
		filepath.Join(legacyClusterDir, "inventory", "hosts"):                 "inventory content",
		filepath.Join(legacyClusterDir, "secrets", "age", "keys", "key.txt"):  "age key content",
		filepath.Join(legacyClusterDir, "secrets", ".sops.yaml"):              "sops config",
	}
	
	for filePath, content := range files {
		err = os.WriteFile(filePath, []byte(content), 0600)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", filePath, err)
		}
	}
	
	// Perform migration
	err = mm.MigrateClusterToOrganization(clusterName, organization)
	if err != nil {
		t.Fatalf("Failed to migrate cluster: %v", err)
	}
	
	// Verify migration results
	newPaths := pr.ResolveClusterPaths(clusterName, organization)
	
	// Check that new directories exist
	expectedDirs := []string{
		newPaths.OrganizationDir,
		newPaths.ClusterDir,
		newPaths.ApplicationsDir,
		newPaths.SecretsDir,
	}
	
	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Expected directory %s was not created after migration", dir)
		}
	}
	
	// Check that files were migrated
	expectedFiles := []string{
		filepath.Join(newPaths.ClusterDir, "."+clusterName+"-config.yaml"),
		newPaths.KubeconfigPath,
		filepath.Join(newPaths.ClusterDir, "main.tf"),
		filepath.Join(newPaths.InventoryPath, "hosts"),
		filepath.Join(newPaths.SecretsDir, "age", "keys", "key.txt"),
		filepath.Join(newPaths.SecretsDir, ".sops.yaml"),
	}
	
	for _, filePath := range expectedFiles {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected file %s was not migrated", filePath)
		}
	}
	
	// Verify that legacy directory is gone (or at least empty)
	if entries, err := os.ReadDir(legacyClusterDir); err == nil && len(entries) > 0 {
		// Check if only empty directories remain
		hasFiles := false
		for _, entry := range entries {
			if !entry.IsDir() {
				hasFiles = true
				break
			}
		}
		if hasFiles {
			t.Errorf("Legacy cluster directory %s should not contain files after migration", legacyClusterDir)
		}
	}
	
	// Validate post-migration
	err = mm.ValidatePostMigration(clusterName, organization)
	if err != nil {
		t.Errorf("Post-migration validation failed: %v", err)
	}
}

func TestPathResolver_EnvironmentVariableExpansion(t *testing.T) {
	// Set up test environment variables
	os.Setenv("TEST_CLUSTERS_BASE", "/tmp/test-clusters")
	os.Setenv("TEST_ORG", "env-org")
	defer func() {
		os.Unsetenv("TEST_CLUSTERS_BASE")
		os.Unsetenv("TEST_ORG")
	}()
	
	config := DefaultCLIConfig()
	config.Paths.ClustersDir = "${TEST_CLUSTERS_BASE}"
	
	cm, err := NewConfigManagerWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	pr := NewPathResolver(cm)
	
	// Test environment variable expansion in paths
	testPath := "${TEST_CLUSTERS_BASE}/${TEST_ORG}/test"
	expandedPath := pr.ExpandPath(testPath)
	expectedPath := "/tmp/test-clusters/env-org/test"
	
	if expandedPath != expectedPath {
		t.Errorf("Expected expanded path %s, got %s", expectedPath, expandedPath)
	}
	
	// Test cluster paths with environment variables
	clusterName := "test-cluster"
	organization := os.ExpandEnv("${TEST_ORG}")
	
	paths := pr.ResolveClusterPaths(clusterName, organization)
	
	expectedOrgDir := "/tmp/test-clusters/env-org"
	if paths.OrganizationDir != expectedOrgDir {
		t.Errorf("Expected organization dir %s, got %s", expectedOrgDir, paths.OrganizationDir)
	}
}

func TestPathResolver_PathValidationSecurity(t *testing.T) {
	config := DefaultCLIConfig()
	cm, err := NewConfigManagerWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	pr := NewPathResolver(cm)
	
	tests := []struct {
		name        string
		path        string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "path traversal attack",
			path:        "/tmp/../../../etc/passwd",
			expectError: true,
			errorMsg:    "directory traversal",
		},
		{
			name:        "relative path traversal",
			path:        "../../etc/passwd",
			expectError: true,
			errorMsg:    "directory traversal",
		},
		{
			name:        "empty path",
			path:        "",
			expectError: true,
			errorMsg:    "cannot be empty",
		},
		{
			name:        "valid absolute path",
			path:        "/tmp/valid/path",
			expectError: false,
		},
		{
			name:        "valid path with tilde",
			path:        "~/valid/path",
			expectError: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pr.ValidatePath(tt.path)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for path '%s', but got none", tt.path)
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for path '%s', but got: %v", tt.path, err)
				}
			}
		})
	}
}

func TestPathResolver_ComplexOrganizationStructure(t *testing.T) {
	tempDir := t.TempDir()
	
	config := DefaultCLIConfig()
	config.Paths.ClustersDir = filepath.Join(tempDir, "clusters")
	
	cm, err := NewConfigManagerWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	pr := NewPathResolver(cm)
	
	// Test multiple organizations and clusters
	organizations := []string{"org1", "org2", "opencenter"}
	clusters := []string{"cluster1", "cluster2", "cluster3"}
	
	// Create structures for all combinations
	for _, org := range organizations {
		err = pr.CreateOrganizationStructure(org)
		if err != nil {
			t.Fatalf("Failed to create organization structure for %s: %v", org, err)
		}
		
		for _, cluster := range clusters {
			err = pr.CreateClusterDirectories(cluster, org)
			if err != nil {
				t.Fatalf("Failed to create cluster directories for %s/%s: %v", org, cluster, err)
			}
			
			// Verify paths are correctly resolved
			paths := pr.ResolveClusterPaths(cluster, org)
			
			expectedOrgDir := filepath.Join(tempDir, "clusters", org)
			if paths.OrganizationDir != expectedOrgDir {
				t.Errorf("Expected org dir %s, got %s", expectedOrgDir, paths.OrganizationDir)
			}
			
			expectedClusterDir := filepath.Join(expectedOrgDir, "infrastructure", "clusters", cluster)
			if paths.ClusterDir != expectedClusterDir {
				t.Errorf("Expected cluster dir %s, got %s", expectedClusterDir, paths.ClusterDir)
			}
			
			// Verify directories actually exist
			if _, err := os.Stat(paths.ClusterDir); os.IsNotExist(err) {
				t.Errorf("Cluster directory %s was not created", paths.ClusterDir)
			}
		}
	}
	
	// Test that each organization has its own isolated structure
	org1Paths := pr.ResolveClusterPaths("cluster1", "org1")
	org2Paths := pr.ResolveClusterPaths("cluster1", "org2")
	
	if org1Paths.SecretsDir == org2Paths.SecretsDir {
		t.Error("Different organizations should have separate secrets directories")
	}
	
	if org1Paths.GitOpsDir == org2Paths.GitOpsDir {
		t.Error("Different organizations should have separate GitOps directories")
	}
}
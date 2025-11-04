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
			name:         "without organization defaults to default",
			clusterName:  "test-cluster",
			organization: "",
			wantOrg:      "default",
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
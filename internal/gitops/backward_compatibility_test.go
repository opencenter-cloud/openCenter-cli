/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gitops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackwardCompatibility_CopyBaseWorksWithoutModification validates
// that the existing CopyBase function continues to work without any code changes.
func TestBackwardCompatibility_CopyBaseWorksWithoutModification(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test configuration using existing config API
	cfg := config.NewDefault("test-org")
	cfg.OpenCenter.Cluster.ClusterName = "test-cluster"
	cfg.OpenCenter.GitOps.GitDir = tmpDir

	// Call the EXISTING CopyBase function WITHOUT any modifications
	// This is the exact same function signature that existing code uses
	err := CopyBase(cfg, true)
	require.NoError(t, err, "existing CopyBase call should work without modification")

	// Verify expected files were created
	expectedFiles := []string{
		".gitignore",
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(tmpDir, file)
		_, err := os.Stat(filePath)
		assert.NoError(t, err, "expected file %s should exist", file)
	}

	// Verify expected directories exist
	expectedDirs := []string{
		"applications",
		"infrastructure",
	}

	for _, dir := range expectedDirs {
		dirPath := filepath.Join(tmpDir, dir)
		_, err := os.Stat(dirPath)
		assert.NoError(t, err, "expected directory %s should exist", dir)
	}
}

// TestBackwardCompatibility_RenderClusterAppsWorksWithoutModification validates
// that the existing RenderClusterApps function continues to work without any code changes.
func TestBackwardCompatibility_RenderClusterAppsWorksWithoutModification(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test configuration
	cfg := config.NewDefault("test-org")
	cfg.OpenCenter.Cluster.ClusterName = "test-cluster"
	cfg.OpenCenter.GitOps.GitDir = tmpDir

	// Call the EXISTING RenderClusterApps function WITHOUT any modifications
	err := RenderClusterApps(cfg)
	require.NoError(t, err, "existing RenderClusterApps call should work without modification")

	// Verify expected directory structure was created
	appsDir := filepath.Join(tmpDir, "applications", "overlays", "test-cluster")
	_, err = os.Stat(appsDir)
	assert.NoError(t, err, "applications overlay directory should exist")
}

// TestBackwardCompatibility_RenderInfrastructureClusterWorksWithoutModification validates
// that the existing RenderInfrastructureCluster function continues to work without any code changes.
func TestBackwardCompatibility_RenderInfrastructureClusterWorksWithoutModification(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test configuration
	cfg := config.NewDefault("test-org")
	cfg.OpenCenter.Cluster.ClusterName = "test-cluster"
	cfg.OpenCenter.GitOps.GitDir = tmpDir
	cfg.OpenCenter.Infrastructure.Provider = "openstack"

	// Call the EXISTING RenderInfrastructureCluster function WITHOUT any modifications
	err := RenderInfrastructureCluster(cfg)
	require.NoError(t, err, "existing RenderInfrastructureCluster call should work without modification")

	// Verify expected directory structure was created
	infraDir := filepath.Join(tmpDir, "infrastructure", "clusters", "test-cluster")
	_, err = os.Stat(infraDir)
	assert.NoError(t, err, "infrastructure cluster directory should exist")

	// Verify main.tf was created
	mainTfPath := filepath.Join(infraDir, "main.tf")
	_, err = os.Stat(mainTfPath)
	assert.NoError(t, err, "main.tf should exist")
}

// TestBackwardCompatibility_CompleteWorkflow validates that a complete
// GitOps generation workflow using existing functions works without modification.
func TestBackwardCompatibility_CompleteWorkflow(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test configuration
	cfg := config.NewDefault("test-org")
	cfg.OpenCenter.Cluster.ClusterName = "test-cluster"
	cfg.OpenCenter.GitOps.GitDir = tmpDir
	cfg.OpenCenter.Infrastructure.Provider = "openstack"

	// Execute the complete workflow using EXISTING functions WITHOUT modifications
	// This is the exact pattern used in existing code

	// Step 1: Copy base GitOps structure
	err := CopyBase(cfg, true)
	require.NoError(t, err, "CopyBase should work without modification")

	// Step 2: Render cluster applications
	err = RenderClusterApps(cfg)
	require.NoError(t, err, "RenderClusterApps should work without modification")

	// Step 3: Render infrastructure cluster
	err = RenderInfrastructureCluster(cfg)
	require.NoError(t, err, "RenderInfrastructureCluster should work without modification")

	// Verify the complete structure was created
	expectedDirs := []string{
		"applications/overlays/test-cluster",
		"infrastructure/clusters/test-cluster",
	}

	for _, dir := range expectedDirs {
		dirPath := filepath.Join(tmpDir, dir)
		_, err := os.Stat(dirPath)
		assert.NoError(t, err, "expected directory %s should exist", dir)
	}

	// Verify key files exist
	expectedFiles := []string{
		".gitignore",
		"infrastructure/clusters/test-cluster/main.tf",
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(tmpDir, file)
		_, err := os.Stat(filePath)
		assert.NoError(t, err, "expected file %s should exist", file)
	}
}

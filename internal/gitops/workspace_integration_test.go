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

package gitops

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// TestWorkspaceIsolatedEnvironment verifies that workspace provides an isolated
// environment for generation operations.
//
// This test validates the acceptance criterion:
// "Workspace provides isolated environment for generation"
//
// The test demonstrates:
// 1. Multiple workspaces can coexist without interfering with each other
// 2. Each workspace has its own directory structure
// 3. File operations in one workspace don't affect other workspaces
// 4. Workspaces can be cleaned up independently
func TestWorkspaceIsolatedEnvironment(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	// Create multiple workspaces for different clusters
	cfg1 := config.NewDefault("production-cluster")
	workspace1, err := manager.CreateWorkspace(ctx, cfg1)
	if err != nil {
		t.Fatalf("Failed to create workspace1: %v", err)
	}

	cfg2 := config.NewDefault("staging-cluster")
	workspace2, err := manager.CreateWorkspace(ctx, cfg2)
	if err != nil {
		t.Fatalf("Failed to create workspace2: %v", err)
	}

	cfg3 := config.NewDefault("development-cluster")
	workspace3, err := manager.CreateWorkspace(ctx, cfg3)
	if err != nil {
		t.Fatalf("Failed to create workspace3: %v", err)
	}

	// Verify each workspace has unique ID and directory
	workspaces := []*GitOpsWorkspace{workspace1, workspace2, workspace3}
	ids := make(map[string]bool)
	dirs := make(map[string]bool)

	for _, ws := range workspaces {
		if ids[ws.ID] {
			t.Errorf("Duplicate workspace ID: %s", ws.ID)
		}
		ids[ws.ID] = true

		if dirs[ws.RootDir] {
			t.Errorf("Duplicate workspace directory: %s", ws.RootDir)
		}
		dirs[ws.RootDir] = true

		// Verify directory exists
		if _, err := os.Stat(ws.RootDir); os.IsNotExist(err) {
			t.Errorf("Workspace directory does not exist: %s", ws.RootDir)
		}

		// Verify temp directory exists
		if _, err := os.Stat(ws.TempDir); os.IsNotExist(err) {
			t.Errorf("Workspace temp directory does not exist: %s", ws.TempDir)
		}
	}

	// Perform isolated operations in each workspace
	// Workspace 1: Create infrastructure files
	writer1 := NewAtomicWriter(workspace1)
	if err := writer1.WriteFileString("infrastructure/main.tf", "# Production infrastructure", 0o644); err != nil {
		t.Fatalf("Failed to write to workspace1: %v", err)
	}
	if err := writer1.WriteFileString("infrastructure/variables.tf", "# Production variables", 0o644); err != nil {
		t.Fatalf("Failed to write to workspace1: %v", err)
	}

	// Workspace 2: Create application files
	writer2 := NewAtomicWriter(workspace2)
	if err := writer2.WriteFileString("applications/app1.yaml", "# Staging app1", 0o644); err != nil {
		t.Fatalf("Failed to write to workspace2: %v", err)
	}
	if err := writer2.WriteFileString("applications/app2.yaml", "# Staging app2", 0o644); err != nil {
		t.Fatalf("Failed to write to workspace2: %v", err)
	}

	// Workspace 3: Create configuration files
	writer3 := NewAtomicWriter(workspace3)
	if err := writer3.WriteFileString("config/cluster.yaml", "# Development config", 0o644); err != nil {
		t.Fatalf("Failed to write to workspace3: %v", err)
	}

	// Verify isolation: each workspace only has its own files
	// Workspace 1 should have infrastructure files
	if !workspace1.Exists("infrastructure/main.tf") {
		t.Error("Workspace1 should have infrastructure/main.tf")
	}
	if workspace1.Exists("applications/app1.yaml") {
		t.Error("Workspace1 should not have applications/app1.yaml")
	}
	if workspace1.Exists("config/cluster.yaml") {
		t.Error("Workspace1 should not have config/cluster.yaml")
	}

	// Workspace 2 should have application files
	if workspace2.Exists("infrastructure/main.tf") {
		t.Error("Workspace2 should not have infrastructure/main.tf")
	}
	if !workspace2.Exists("applications/app1.yaml") {
		t.Error("Workspace2 should have applications/app1.yaml")
	}
	if workspace2.Exists("config/cluster.yaml") {
		t.Error("Workspace2 should not have config/cluster.yaml")
	}

	// Workspace 3 should have configuration files
	if workspace3.Exists("infrastructure/main.tf") {
		t.Error("Workspace3 should not have infrastructure/main.tf")
	}
	if workspace3.Exists("applications/app1.yaml") {
		t.Error("Workspace3 should not have applications/app1.yaml")
	}
	if !workspace3.Exists("config/cluster.yaml") {
		t.Error("Workspace3 should have config/cluster.yaml")
	}

	// Verify file contents are isolated
	content1, _ := os.ReadFile(workspace1.GetPath("infrastructure/main.tf"))
	if string(content1) != "# Production infrastructure" {
		t.Errorf("Workspace1 content mismatch: %s", string(content1))
	}

	content2, _ := os.ReadFile(workspace2.GetPath("applications/app1.yaml"))
	if string(content2) != "# Staging app1" {
		t.Errorf("Workspace2 content mismatch: %s", string(content2))
	}

	content3, _ := os.ReadFile(workspace3.GetPath("config/cluster.yaml"))
	if string(content3) != "# Development config" {
		t.Errorf("Workspace3 content mismatch: %s", string(content3))
	}

	// Test metadata isolation
	workspace1.SetMetadata("environment", "production")
	workspace2.SetMetadata("environment", "staging")
	workspace3.SetMetadata("environment", "development")

	env1, _ := workspace1.GetMetadata("environment")
	if env1 != "production" {
		t.Errorf("Workspace1 metadata mismatch: %v", env1)
	}

	env2, _ := workspace2.GetMetadata("environment")
	if env2 != "staging" {
		t.Errorf("Workspace2 metadata mismatch: %v", env2)
	}

	env3, _ := workspace3.GetMetadata("environment")
	if env3 != "development" {
		t.Errorf("Workspace3 metadata mismatch: %v", env3)
	}

	// Test checkpoint isolation
	checkpoint1, err := workspace1.CreateCheckpoint("pre-deploy")
	if err != nil {
		t.Fatalf("Failed to create checkpoint in workspace1: %v", err)
	}

	checkpoint2, err := workspace2.CreateCheckpoint("pre-deploy")
	if err != nil {
		t.Fatalf("Failed to create checkpoint in workspace2: %v", err)
	}

	// Verify checkpoints are isolated
	if len(workspace1.ListCheckpoints()) != 1 {
		t.Error("Workspace1 should have 1 checkpoint")
	}
	if len(workspace2.ListCheckpoints()) != 1 {
		t.Error("Workspace2 should have 1 checkpoint")
	}
	if len(workspace3.ListCheckpoints()) != 0 {
		t.Error("Workspace3 should have 0 checkpoints")
	}

	// Verify checkpoint IDs are the same but checkpoints are independent
	if checkpoint1.ID != checkpoint2.ID {
		t.Error("Checkpoint IDs should be the same")
	}

	// Modify workspace1 and restore
	writer1.WriteFileString("infrastructure/main.tf", "# Modified", 0o644)
	workspace1.RestoreCheckpoint("pre-deploy")

	// Verify workspace1 is restored but workspace2 is unchanged
	content1, _ = os.ReadFile(workspace1.GetPath("infrastructure/main.tf"))
	if string(content1) != "# Production infrastructure" {
		t.Error("Workspace1 should be restored")
	}

	content2, _ = os.ReadFile(workspace2.GetPath("applications/app1.yaml"))
	if string(content2) != "# Staging app1" {
		t.Error("Workspace2 should be unchanged")
	}

	// Test independent cleanup
	// Cleanup workspace2
	if err := manager.CleanupWorkspace(ctx, workspace2); err != nil {
		t.Fatalf("Failed to cleanup workspace2: %v", err)
	}

	// Verify workspace2 is removed but others remain
	if _, err := os.Stat(workspace2.RootDir); !os.IsNotExist(err) {
		t.Error("Workspace2 directory should be removed")
	}

	if _, err := os.Stat(workspace1.RootDir); os.IsNotExist(err) {
		t.Error("Workspace1 directory should still exist")
	}

	if _, err := os.Stat(workspace3.RootDir); os.IsNotExist(err) {
		t.Error("Workspace3 directory should still exist")
	}

	// Cleanup remaining workspaces
	manager.CleanupWorkspace(ctx, workspace1)
	manager.CleanupWorkspace(ctx, workspace3)
}

// TestWorkspaceGenerationScenario simulates a realistic GitOps generation scenario
// to demonstrate the isolated environment in action.
func TestWorkspaceGenerationScenario(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	// Create workspace for cluster generation
	cfg := config.NewDefault("test-cluster")
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	// Simulate GitOps generation stages
	writer := NewAtomicWriter(workspace)

	// Stage 1: Create base directory structure
	dirs := []string{
		"infrastructure/clusters/test-cluster",
		"applications/base",
		"applications/overlays/test-cluster",
		"managed-services",
	}

	for _, dir := range dirs {
		if err := writer.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create checkpoint after base structure
	_, err = workspace.CreateCheckpoint("base-structure")
	if err != nil {
		t.Fatalf("Failed to create checkpoint: %v", err)
	}

	// Stage 2: Generate infrastructure files
	infraFiles := map[string]string{
		"infrastructure/clusters/test-cluster/main.tf":      "# Infrastructure main",
		"infrastructure/clusters/test-cluster/variables.tf": "# Infrastructure variables",
		"infrastructure/clusters/test-cluster/outputs.tf":   "# Infrastructure outputs",
	}

	for path, content := range infraFiles {
		if err := writer.WriteFileString(path, content, 0o644); err != nil {
			t.Fatalf("Failed to write %s: %v", path, err)
		}
	}

	// Create checkpoint after infrastructure
	_, err = workspace.CreateCheckpoint("infrastructure-complete")
	if err != nil {
		t.Fatalf("Failed to create checkpoint: %v", err)
	}

	// Stage 3: Generate application manifests
	appFiles := map[string]string{
		"applications/base/kustomization.yaml":                  "# Base kustomization",
		"applications/overlays/test-cluster/kustomization.yaml": "# Cluster kustomization",
		"applications/overlays/test-cluster/namespace.yaml":     "# Namespace definition",
		"applications/overlays/test-cluster/services/app1.yaml": "# Application 1",
		"applications/overlays/test-cluster/services/app2.yaml": "# Application 2",
	}

	for path, content := range appFiles {
		if err := writer.WriteFileString(path, content, 0o644); err != nil {
			t.Fatalf("Failed to write %s: %v", path, err)
		}
	}

	// Verify all files exist in isolated workspace
	allFiles := make(map[string]string)
	for k, v := range infraFiles {
		allFiles[k] = v
	}
	for k, v := range appFiles {
		allFiles[k] = v
	}

	for path, expectedContent := range allFiles {
		if !workspace.Exists(path) {
			t.Errorf("File should exist: %s", path)
			continue
		}

		content, err := os.ReadFile(workspace.GetPath(path))
		if err != nil {
			t.Errorf("Failed to read %s: %v", path, err)
			continue
		}

		if string(content) != expectedContent {
			t.Errorf("Content mismatch for %s: expected '%s', got '%s'", path, expectedContent, string(content))
		}
	}

	// Verify workspace directory structure
	expectedDirs := []string{
		"infrastructure",
		"infrastructure/clusters",
		"infrastructure/clusters/test-cluster",
		"applications",
		"applications/base",
		"applications/overlays",
		"applications/overlays/test-cluster",
		"applications/overlays/test-cluster/services",
		"managed-services",
	}

	for _, dir := range expectedDirs {
		dirPath := workspace.GetPath(dir)
		info, err := os.Stat(dirPath)
		if err != nil {
			t.Errorf("Directory should exist: %s", dir)
			continue
		}

		if !info.IsDir() {
			t.Errorf("Path should be a directory: %s", dir)
		}
	}

	// Verify checkpoints can restore to previous states
	// Restore to infrastructure-complete (before applications)
	if err := workspace.RestoreCheckpoint("infrastructure-complete"); err != nil {
		t.Fatalf("Failed to restore checkpoint: %v", err)
	}

	// Verify infrastructure files still exist
	for path := range infraFiles {
		if !workspace.Exists(path) {
			t.Errorf("Infrastructure file should exist after restore: %s", path)
		}
	}

	// Verify application files are removed
	for path := range appFiles {
		if workspace.Exists(path) {
			t.Errorf("Application file should not exist after restore: %s", path)
		}
	}

	// Verify workspace provides isolation from host filesystem
	// Files should only exist in workspace, not in temp directory root
	for path := range allFiles {
		hostPath := filepath.Join(tempDir, path)
		if _, err := os.Stat(hostPath); !os.IsNotExist(err) {
			t.Errorf("File should not exist in host filesystem: %s", hostPath)
		}
	}
}

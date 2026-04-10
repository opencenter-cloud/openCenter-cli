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
	"time"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// TestWorkspaceCleanupCheckpoints tests that cleanup removes all checkpoints.
func TestWorkspaceCleanupCheckpoints(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	cfg := config.NewDefault("test-cluster")
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	// Create some files and checkpoints
	writer := NewAtomicWriter(workspace)
	writer.WriteFileString("file1.txt", "content1", 0o644)
	writer.WriteFileString("file2.txt", "content2", 0o644)

	_, err = workspace.CreateCheckpoint("checkpoint1")
	if err != nil {
		t.Fatalf("Failed to create checkpoint1: %v", err)
	}

	writer.WriteFileString("file3.txt", "content3", 0o644)

	_, err = workspace.CreateCheckpoint("checkpoint2")
	if err != nil {
		t.Fatalf("Failed to create checkpoint2: %v", err)
	}

	// Verify checkpoints exist
	if len(workspace.ListCheckpoints()) != 2 {
		t.Errorf("Expected 2 checkpoints, got %d", len(workspace.ListCheckpoints()))
	}

	// Store checkpoint directories for verification
	checkpoint1Dir := filepath.Join(workspace.TempDir, "checkpoints", "checkpoint1")
	checkpoint2Dir := filepath.Join(workspace.TempDir, "checkpoints", "checkpoint2")

	// Verify checkpoint directories exist
	if _, err := os.Stat(checkpoint1Dir); os.IsNotExist(err) {
		t.Error("Checkpoint1 directory should exist")
	}
	if _, err := os.Stat(checkpoint2Dir); os.IsNotExist(err) {
		t.Error("Checkpoint2 directory should exist")
	}

	// Cleanup workspace
	if err := manager.CleanupWorkspace(ctx, workspace); err != nil {
		t.Fatalf("Failed to cleanup workspace: %v", err)
	}

	// Verify workspace directory is removed
	if _, err := os.Stat(workspace.RootDir); !os.IsNotExist(err) {
		t.Error("Workspace directory should be removed")
	}

	// Verify checkpoint directories are removed
	if _, err := os.Stat(checkpoint1Dir); !os.IsNotExist(err) {
		t.Error("Checkpoint1 directory should be removed")
	}
	if _, err := os.Stat(checkpoint2Dir); !os.IsNotExist(err) {
		t.Error("Checkpoint2 directory should be removed")
	}
}

// TestStaleWorkspaceCleanup tests automatic cleanup of stale workspaces.
func TestStaleWorkspaceCleanup(t *testing.T) {
	tempDir := t.TempDir()

	// Create manager with short cleanup intervals for testing
	maxAge := 100 * time.Millisecond
	cleanupInterval := 50 * time.Millisecond
	manager := NewWorkspaceManagerWithOptions(tempDir, maxAge, cleanupInterval)
	defer manager.(*DefaultWorkspaceManager).Shutdown(context.Background())

	ctx := context.Background()

	// Create workspace
	cfg := config.NewDefault("test-cluster")
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	// Create some files
	writer := NewAtomicWriter(workspace)
	writer.WriteFileString("file1.txt", "content1", 0o644)

	// Store workspace directory for verification
	workspaceDir := workspace.RootDir

	// Verify workspace exists
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		t.Error("Workspace directory should exist")
	}

	// Verify workspace is tracked
	if manager.(*DefaultWorkspaceManager).GetActiveWorkspaceCount() != 1 {
		t.Errorf("Expected 1 active workspace, got %d", manager.(*DefaultWorkspaceManager).GetActiveWorkspaceCount())
	}

	// Wait for workspace to become stale and be cleaned up
	// We need to wait for: maxAge + cleanupInterval + some buffer
	time.Sleep(maxAge + cleanupInterval + 100*time.Millisecond)

	// Verify workspace was cleaned up
	if _, err := os.Stat(workspaceDir); !os.IsNotExist(err) {
		t.Error("Stale workspace directory should be removed")
	}

	// Verify workspace is no longer tracked
	if manager.(*DefaultWorkspaceManager).GetActiveWorkspaceCount() != 0 {
		t.Errorf("Expected 0 active workspaces after cleanup, got %d", manager.(*DefaultWorkspaceManager).GetActiveWorkspaceCount())
	}
}

// TestWorkspaceNotStaleWithActivity tests that active workspaces are not cleaned up.
func TestWorkspaceNotStaleWithActivity(t *testing.T) {
	tempDir := t.TempDir()

	// Create manager with short cleanup intervals for testing
	maxAge := 200 * time.Millisecond
	cleanupInterval := 50 * time.Millisecond
	manager := NewWorkspaceManagerWithOptions(tempDir, maxAge, cleanupInterval)
	defer manager.(*DefaultWorkspaceManager).Shutdown(context.Background())

	ctx := context.Background()

	// Create workspace
	cfg := config.NewDefault("test-cluster")
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	// Store workspace directory for verification
	workspaceDir := workspace.RootDir

	// Keep workspace active by updating it periodically
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				workspace.UpdateModifiedTime()
			case <-done:
				return
			}
		}
	}()

	// Wait longer than maxAge
	time.Sleep(maxAge + cleanupInterval + 100*time.Millisecond)

	// Stop activity
	close(done)

	// Verify workspace still exists (not cleaned up because it was active)
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		t.Error("Active workspace should not be removed")
	}

	// Verify workspace is still tracked
	if manager.(*DefaultWorkspaceManager).GetActiveWorkspaceCount() != 1 {
		t.Errorf("Expected 1 active workspace, got %d", manager.(*DefaultWorkspaceManager).GetActiveWorkspaceCount())
	}

	// Clean up manually
	manager.CleanupWorkspace(ctx, workspace)
}

// TestManagerShutdown tests that shutdown cleans up all workspaces.
func TestManagerShutdown(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	// Create multiple workspaces
	cfg1 := config.NewDefault("cluster1")
	workspace1, err := manager.CreateWorkspace(ctx, cfg1)
	if err != nil {
		t.Fatalf("Failed to create workspace1: %v", err)
	}

	cfg2 := config.NewDefault("cluster2")
	workspace2, err := manager.CreateWorkspace(ctx, cfg2)
	if err != nil {
		t.Fatalf("Failed to create workspace2: %v", err)
	}

	cfg3 := config.NewDefault("cluster3")
	workspace3, err := manager.CreateWorkspace(ctx, cfg3)
	if err != nil {
		t.Fatalf("Failed to create workspace3: %v", err)
	}

	// Store workspace directories
	dir1 := workspace1.RootDir
	dir2 := workspace2.RootDir
	dir3 := workspace3.RootDir

	// Verify all workspaces exist
	if manager.(*DefaultWorkspaceManager).GetActiveWorkspaceCount() != 3 {
		t.Errorf("Expected 3 active workspaces, got %d", manager.(*DefaultWorkspaceManager).GetActiveWorkspaceCount())
	}

	// Shutdown manager
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := manager.(*DefaultWorkspaceManager).Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Failed to shutdown manager: %v", err)
	}

	// Verify all workspaces are cleaned up
	if _, err := os.Stat(dir1); !os.IsNotExist(err) {
		t.Error("Workspace1 directory should be removed after shutdown")
	}
	if _, err := os.Stat(dir2); !os.IsNotExist(err) {
		t.Error("Workspace2 directory should be removed after shutdown")
	}
	if _, err := os.Stat(dir3); !os.IsNotExist(err) {
		t.Error("Workspace3 directory should be removed after shutdown")
	}

	// Verify no workspaces are tracked
	if manager.(*DefaultWorkspaceManager).GetActiveWorkspaceCount() != 0 {
		t.Errorf("Expected 0 active workspaces after shutdown, got %d", manager.(*DefaultWorkspaceManager).GetActiveWorkspaceCount())
	}
}

// TestOrphanedWorkspaceCleanup tests cleanup of orphaned workspace directories.
func TestOrphanedWorkspaceCleanup(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	// Create workspace directory structure manually (simulating orphaned workspace)
	workspaceBaseDir := filepath.Join(tempDir, "gitops-workspaces")
	orphanedWorkspaceDir := filepath.Join(workspaceBaseDir, "orphaned-workspace-123")
	if err := os.MkdirAll(orphanedWorkspaceDir, 0o755); err != nil {
		t.Fatalf("Failed to create orphaned workspace directory: %v", err)
	}

	// Create some files in orphaned workspace
	orphanedFile := filepath.Join(orphanedWorkspaceDir, "orphaned-file.txt")
	if err := os.WriteFile(orphanedFile, []byte("orphaned content"), 0o644); err != nil {
		t.Fatalf("Failed to create orphaned file: %v", err)
	}

	// Make the orphaned workspace old enough to be cleaned up
	oldTime := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(orphanedWorkspaceDir, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set old time on orphaned workspace: %v", err)
	}

	// Create a tracked workspace (should not be cleaned up)
	cfg := config.NewDefault("tracked-cluster")
	trackedWorkspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create tracked workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, trackedWorkspace)

	trackedDir := trackedWorkspace.RootDir

	// Verify both directories exist before cleanup
	if _, err := os.Stat(orphanedWorkspaceDir); os.IsNotExist(err) {
		t.Error("Orphaned workspace directory should exist before cleanup")
	}
	if _, err := os.Stat(trackedDir); os.IsNotExist(err) {
		t.Error("Tracked workspace directory should exist before cleanup")
	}

	// Run orphaned workspace cleanup
	if err := manager.(*DefaultWorkspaceManager).CleanupOrphanedWorkspaces(ctx); err != nil {
		t.Fatalf("Failed to cleanup orphaned workspaces: %v", err)
	}

	// Verify orphaned workspace is removed
	if _, err := os.Stat(orphanedWorkspaceDir); !os.IsNotExist(err) {
		t.Error("Orphaned workspace directory should be removed")
	}

	// Verify tracked workspace still exists
	if _, err := os.Stat(trackedDir); os.IsNotExist(err) {
		t.Error("Tracked workspace directory should still exist")
	}
}

// TestRecentOrphanedWorkspaceNotCleaned tests that recent orphaned workspaces are not cleaned.
func TestRecentOrphanedWorkspaceNotCleaned(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	// Create workspace directory structure manually (simulating recent orphaned workspace)
	workspaceBaseDir := filepath.Join(tempDir, "gitops-workspaces")
	recentOrphanedDir := filepath.Join(workspaceBaseDir, "recent-orphaned-workspace-456")
	if err := os.MkdirAll(recentOrphanedDir, 0o755); err != nil {
		t.Fatalf("Failed to create recent orphaned workspace directory: %v", err)
	}

	// Create some files in orphaned workspace
	orphanedFile := filepath.Join(recentOrphanedDir, "recent-file.txt")
	if err := os.WriteFile(orphanedFile, []byte("recent content"), 0o644); err != nil {
		t.Fatalf("Failed to create recent file: %v", err)
	}

	// Don't modify the time - it should be recent

	// Run orphaned workspace cleanup
	if err := manager.(*DefaultWorkspaceManager).CleanupOrphanedWorkspaces(ctx); err != nil {
		t.Fatalf("Failed to cleanup orphaned workspaces: %v", err)
	}

	// Verify recent orphaned workspace still exists (not cleaned up)
	if _, err := os.Stat(recentOrphanedDir); os.IsNotExist(err) {
		t.Error("Recent orphaned workspace should not be removed")
	}
}

// TestGetActiveWorkspaceCount tests counting active workspaces.
func TestGetActiveWorkspaceCount(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	// Initially should have 0 workspaces
	if count := manager.(*DefaultWorkspaceManager).GetActiveWorkspaceCount(); count != 0 {
		t.Errorf("Expected 0 active workspaces initially, got %d", count)
	}

	// Create workspaces
	cfg1 := config.NewDefault("cluster1")
	workspace1, _ := manager.CreateWorkspace(ctx, cfg1)

	if count := manager.(*DefaultWorkspaceManager).GetActiveWorkspaceCount(); count != 1 {
		t.Errorf("Expected 1 active workspace, got %d", count)
	}

	cfg2 := config.NewDefault("cluster2")
	workspace2, _ := manager.CreateWorkspace(ctx, cfg2)

	if count := manager.(*DefaultWorkspaceManager).GetActiveWorkspaceCount(); count != 2 {
		t.Errorf("Expected 2 active workspaces, got %d", count)
	}

	// Cleanup one workspace
	manager.CleanupWorkspace(ctx, workspace1)

	if count := manager.(*DefaultWorkspaceManager).GetActiveWorkspaceCount(); count != 1 {
		t.Errorf("Expected 1 active workspace after cleanup, got %d", count)
	}

	// Cleanup remaining workspace
	manager.CleanupWorkspace(ctx, workspace2)

	if count := manager.(*DefaultWorkspaceManager).GetActiveWorkspaceCount(); count != 0 {
		t.Errorf("Expected 0 active workspaces after all cleanup, got %d", count)
	}
}

// TestListActiveWorkspaces tests listing active workspace IDs.
func TestListActiveWorkspaces(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	// Initially should have empty list
	if workspaces := manager.(*DefaultWorkspaceManager).ListActiveWorkspaces(); len(workspaces) != 0 {
		t.Errorf("Expected empty workspace list initially, got %d workspaces", len(workspaces))
	}

	// Create workspaces
	cfg1 := config.NewDefault("cluster1")
	workspace1, _ := manager.CreateWorkspace(ctx, cfg1)

	cfg2 := config.NewDefault("cluster2")
	workspace2, _ := manager.CreateWorkspace(ctx, cfg2)

	cfg3 := config.NewDefault("cluster3")
	workspace3, _ := manager.CreateWorkspace(ctx, cfg3)

	// Get list of active workspaces
	workspaces := manager.(*DefaultWorkspaceManager).ListActiveWorkspaces()

	if len(workspaces) != 3 {
		t.Errorf("Expected 3 active workspaces, got %d", len(workspaces))
	}

	// Verify all workspace IDs are in the list
	workspaceMap := make(map[string]bool)
	for _, id := range workspaces {
		workspaceMap[id] = true
	}

	if !workspaceMap[workspace1.ID] {
		t.Error("Workspace1 ID should be in active list")
	}
	if !workspaceMap[workspace2.ID] {
		t.Error("Workspace2 ID should be in active list")
	}
	if !workspaceMap[workspace3.ID] {
		t.Error("Workspace3 ID should be in active list")
	}

	// Cleanup
	manager.CleanupWorkspace(ctx, workspace1)
	manager.CleanupWorkspace(ctx, workspace2)
	manager.CleanupWorkspace(ctx, workspace3)
}

// TestConcurrentWorkspaceCleanup tests that concurrent cleanup operations are safe.
func TestConcurrentWorkspaceCleanup(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	// Create multiple workspaces
	workspaces := make([]*GitOpsWorkspace, 10)
	for i := 0; i < 10; i++ {
		cfg := config.NewDefault("cluster-" + string(rune('0'+i)))
		workspace, err := manager.CreateWorkspace(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create workspace %d: %v", i, err)
		}
		workspaces[i] = workspace
	}

	// Cleanup all workspaces concurrently
	done := make(chan error, 10)
	for _, workspace := range workspaces {
		go func(ws *GitOpsWorkspace) {
			done <- manager.CleanupWorkspace(ctx, ws)
		}(workspace)
	}

	// Wait for all cleanups to complete
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("Concurrent cleanup failed: %v", err)
		}
	}

	// Verify all workspaces are cleaned up
	if count := manager.(*DefaultWorkspaceManager).GetActiveWorkspaceCount(); count != 0 {
		t.Errorf("Expected 0 active workspaces after concurrent cleanup, got %d", count)
	}
}

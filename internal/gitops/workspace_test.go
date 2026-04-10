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

// TestWorkspaceCreation tests that a workspace can be created successfully.
func TestWorkspaceCreation(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager
	manager := NewWorkspaceManager(tempDir)

	// Create test configuration
	cfg := config.NewDefault("test-cluster")

	// Create workspace
	ctx := context.Background()
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	// Verify workspace properties
	if workspace.ID == "" {
		t.Error("Workspace ID should not be empty")
	}

	if workspace.RootDir == "" {
		t.Error("Workspace RootDir should not be empty")
	}

	if workspace.TempDir == "" {
		t.Error("Workspace TempDir should not be empty")
	}

	// Verify directories exist
	if _, err := os.Stat(workspace.RootDir); os.IsNotExist(err) {
		t.Errorf("Workspace root directory does not exist: %s", workspace.RootDir)
	}

	if _, err := os.Stat(workspace.TempDir); os.IsNotExist(err) {
		t.Errorf("Workspace temp directory does not exist: %s", workspace.TempDir)
	}

	// Verify configuration is stored
	if workspace.Config.ClusterName() != "test-cluster" {
		t.Errorf("Expected cluster name 'test-cluster', got '%s'", workspace.Config.ClusterName())
	}

	// Cleanup
	if err := manager.CleanupWorkspace(ctx, workspace); err != nil {
		t.Errorf("Failed to cleanup workspace: %v", err)
	}
}

// TestWorkspaceIsolation tests that workspaces are isolated from each other.
func TestWorkspaceIsolation(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	// Create first workspace
	cfg1 := config.NewDefault("cluster1")
	workspace1, err := manager.CreateWorkspace(ctx, cfg1)
	if err != nil {
		t.Fatalf("Failed to create workspace1: %v", err)
	}

	// Create second workspace
	cfg2 := config.NewDefault("cluster2")
	workspace2, err := manager.CreateWorkspace(ctx, cfg2)
	if err != nil {
		t.Fatalf("Failed to create workspace2: %v", err)
	}

	// Verify workspaces have different IDs
	if workspace1.ID == workspace2.ID {
		t.Error("Workspaces should have different IDs")
	}

	// Verify workspaces have different root directories
	if workspace1.RootDir == workspace2.RootDir {
		t.Error("Workspaces should have different root directories")
	}

	// Write file to workspace1
	writer1 := NewAtomicWriter(workspace1)
	if err := writer1.WriteFileString("test.txt", "workspace1 content", 0o644); err != nil {
		t.Fatalf("Failed to write file to workspace1: %v", err)
	}

	// Write file to workspace2
	writer2 := NewAtomicWriter(workspace2)
	if err := writer2.WriteFileString("test.txt", "workspace2 content", 0o644); err != nil {
		t.Fatalf("Failed to write file to workspace2: %v", err)
	}

	// Verify files are isolated
	content1, err := os.ReadFile(workspace1.GetPath("test.txt"))
	if err != nil {
		t.Fatalf("Failed to read file from workspace1: %v", err)
	}
	if string(content1) != "workspace1 content" {
		t.Errorf("Expected 'workspace1 content', got '%s'", string(content1))
	}

	content2, err := os.ReadFile(workspace2.GetPath("test.txt"))
	if err != nil {
		t.Fatalf("Failed to read file from workspace2: %v", err)
	}
	if string(content2) != "workspace2 content" {
		t.Errorf("Expected 'workspace2 content', got '%s'", string(content2))
	}

	// Cleanup
	manager.CleanupWorkspace(ctx, workspace1)
	manager.CleanupWorkspace(ctx, workspace2)
}

// TestWorkspaceMetadata tests workspace metadata operations.
func TestWorkspaceMetadata(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	cfg := config.NewDefault("test-cluster")
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	// Set metadata
	workspace.SetMetadata("key1", "value1")
	workspace.SetMetadata("key2", 42)
	workspace.SetMetadata("key3", true)

	// Get metadata
	val1, exists := workspace.GetMetadata("key1")
	if !exists {
		t.Error("Metadata key1 should exist")
	}
	if val1 != "value1" {
		t.Errorf("Expected 'value1', got '%v'", val1)
	}

	val2, exists := workspace.GetMetadata("key2")
	if !exists {
		t.Error("Metadata key2 should exist")
	}
	if val2 != 42 {
		t.Errorf("Expected 42, got %v", val2)
	}

	val3, exists := workspace.GetMetadata("key3")
	if !exists {
		t.Error("Metadata key3 should exist")
	}
	if val3 != true {
		t.Errorf("Expected true, got %v", val3)
	}

	// Get non-existent metadata
	_, exists = workspace.GetMetadata("nonexistent")
	if exists {
		t.Error("Non-existent metadata should not exist")
	}
}

// TestWorkspacePathOperations tests workspace path operations.
func TestWorkspacePathOperations(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	cfg := config.NewDefault("test-cluster")
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	// Test GetPath
	path := workspace.GetPath("subdir/file.txt")
	expectedPath := filepath.Join(workspace.RootDir, "subdir", "file.txt")
	if path != expectedPath {
		t.Errorf("Expected path '%s', got '%s'", expectedPath, path)
	}

	// Test GetTempPath
	tempPath := workspace.GetTempPath("temp-file.txt")
	expectedTempPath := filepath.Join(workspace.TempDir, "temp-file.txt")
	if tempPath != expectedTempPath {
		t.Errorf("Expected temp path '%s', got '%s'", expectedTempPath, tempPath)
	}

	// Test Exists (file doesn't exist yet)
	if workspace.Exists("test.txt") {
		t.Error("File should not exist yet")
	}

	// Create file
	writer := NewAtomicWriter(workspace)
	if err := writer.WriteFileString("test.txt", "content", 0o644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Test Exists (file now exists)
	if !workspace.Exists("test.txt") {
		t.Error("File should exist now")
	}
}

// TestWorkspaceCleanup tests that workspace cleanup removes all resources.
func TestWorkspaceCleanup(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	cfg := config.NewDefault("test-cluster")
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	// Create some files
	writer := NewAtomicWriter(workspace)
	writer.WriteFileString("file1.txt", "content1", 0o644)
	writer.WriteFileString("subdir/file2.txt", "content2", 0o644)

	// Store root directory path
	rootDir := workspace.RootDir

	// Cleanup workspace
	if err := manager.CleanupWorkspace(ctx, workspace); err != nil {
		t.Fatalf("Failed to cleanup workspace: %v", err)
	}

	// Verify directory is removed
	if _, err := os.Stat(rootDir); !os.IsNotExist(err) {
		t.Error("Workspace directory should be removed after cleanup")
	}

	// Verify workspace is unregistered
	_, err = manager.GetWorkspace(ctx, workspace.ID)
	if err == nil {
		t.Error("Workspace should not be retrievable after cleanup")
	}
}

// TestWorkspaceTimestamps tests workspace timestamp tracking.
func TestWorkspaceTimestamps(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	cfg := config.NewDefault("test-cluster")
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	// Check creation time
	createdAt := workspace.CreatedAt()
	if createdAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	// Check last modified time
	lastModified := workspace.LastModified()
	if lastModified.IsZero() {
		t.Error("LastModified should not be zero")
	}

	// Initially, created and modified times should be close
	if lastModified.Before(createdAt) {
		t.Error("LastModified should not be before CreatedAt")
	}

	// Update modified time
	workspace.UpdateModifiedTime()
	newLastModified := workspace.LastModified()

	// Verify modified time changed
	if !newLastModified.After(lastModified) {
		t.Error("LastModified should be updated")
	}

	// Verify created time didn't change
	if workspace.CreatedAt() != createdAt {
		t.Error("CreatedAt should not change")
	}
}

// TestCheckpointCreation tests creating a checkpoint of workspace state.
func TestCheckpointCreation(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	cfg := config.NewDefault("test-cluster")
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	// Create some files
	writer := NewAtomicWriter(workspace)
	writer.WriteFileString("file1.txt", "content1", 0o644)
	writer.WriteFileString("subdir/file2.txt", "content2", 0o644)

	// Create checkpoint
	checkpoint, err := workspace.CreateCheckpoint("checkpoint1")
	if err != nil {
		t.Fatalf("Failed to create checkpoint: %v", err)
	}

	// Verify checkpoint properties
	if checkpoint.ID != "checkpoint1" {
		t.Errorf("Expected checkpoint ID 'checkpoint1', got '%s'", checkpoint.ID)
	}

	if checkpoint.Timestamp.IsZero() {
		t.Error("Checkpoint timestamp should not be zero")
	}

	if len(checkpoint.Files) != 2 {
		t.Errorf("Expected 2 files in checkpoint, got %d", len(checkpoint.Files))
	}

	// Verify checkpoint is stored
	checkpoints := workspace.ListCheckpoints()
	if len(checkpoints) != 1 {
		t.Errorf("Expected 1 checkpoint, got %d", len(checkpoints))
	}
}

// TestCheckpointRestore tests restoring workspace from a checkpoint.
func TestCheckpointRestore(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	cfg := config.NewDefault("test-cluster")
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	// Create initial files
	writer := NewAtomicWriter(workspace)
	writer.WriteFileString("file1.txt", "original content", 0o644)
	writer.WriteFileString("file2.txt", "original content 2", 0o644)

	// Create checkpoint
	_, err = workspace.CreateCheckpoint("checkpoint1")
	if err != nil {
		t.Fatalf("Failed to create checkpoint: %v", err)
	}

	// Modify files
	writer.WriteFileString("file1.txt", "modified content", 0o644)
	writer.RemoveFile("file2.txt")
	writer.WriteFileString("file3.txt", "new file", 0o644)

	// Verify modifications
	content, _ := os.ReadFile(workspace.GetPath("file1.txt"))
	if string(content) != "modified content" {
		t.Error("File1 should be modified")
	}

	if workspace.Exists("file2.txt") {
		t.Error("File2 should be removed")
	}

	if !workspace.Exists("file3.txt") {
		t.Error("File3 should exist")
	}

	// Restore checkpoint
	if err := workspace.RestoreCheckpoint("checkpoint1"); err != nil {
		t.Fatalf("Failed to restore checkpoint: %v", err)
	}

	// Verify restoration
	content, _ = os.ReadFile(workspace.GetPath("file1.txt"))
	if string(content) != "original content" {
		t.Errorf("File1 should be restored to original content, got '%s'", string(content))
	}

	if !workspace.Exists("file2.txt") {
		t.Error("File2 should be restored")
	}

	if workspace.Exists("file3.txt") {
		t.Error("File3 should not exist after restore")
	}
}

// TestCheckpointDeletion tests deleting a checkpoint.
func TestCheckpointDeletion(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	cfg := config.NewDefault("test-cluster")
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	// Create file and checkpoint
	writer := NewAtomicWriter(workspace)
	writer.WriteFileString("file1.txt", "content", 0o644)

	_, err = workspace.CreateCheckpoint("checkpoint1")
	if err != nil {
		t.Fatalf("Failed to create checkpoint: %v", err)
	}

	// Verify checkpoint exists
	if len(workspace.ListCheckpoints()) != 1 {
		t.Error("Checkpoint should exist")
	}

	// Delete checkpoint
	if err := workspace.DeleteCheckpoint("checkpoint1"); err != nil {
		t.Fatalf("Failed to delete checkpoint: %v", err)
	}

	// Verify checkpoint is deleted
	if len(workspace.ListCheckpoints()) != 0 {
		t.Error("Checkpoint should be deleted")
	}

	// Verify cannot restore deleted checkpoint
	err = workspace.RestoreCheckpoint("checkpoint1")
	if err == nil {
		t.Error("Should not be able to restore deleted checkpoint")
	}
}

// TestAtomicWrite tests atomic file write operations.
func TestAtomicWrite(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	cfg := config.NewDefault("test-cluster")
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	// Create atomic writer
	writer := NewAtomicWriter(workspace)

	// Write file
	content := []byte("test content")
	if err := writer.WriteFile("test.txt", content, 0o644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Verify file exists and has correct content
	readContent, err := os.ReadFile(workspace.GetPath("test.txt"))
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(readContent) != string(content) {
		t.Errorf("Expected content '%s', got '%s'", string(content), string(readContent))
	}

	// Verify file permissions
	info, err := os.Stat(workspace.GetPath("test.txt"))
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	if info.Mode().Perm() != 0o644 {
		t.Errorf("Expected permissions 0644, got %o", info.Mode().Perm())
	}
}

// TestAtomicWriteString tests atomic string write operations.
func TestAtomicWriteString(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	cfg := config.NewDefault("test-cluster")
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	writer := NewAtomicWriter(workspace)

	// Write string
	content := "test string content"
	if err := writer.WriteFileString("test.txt", content, 0o644); err != nil {
		t.Fatalf("Failed to write string: %v", err)
	}

	// Verify content
	readContent, err := os.ReadFile(workspace.GetPath("test.txt"))
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(readContent) != content {
		t.Errorf("Expected content '%s', got '%s'", content, string(readContent))
	}
}

// TestAtomicCopyFile tests atomic file copy operations.
func TestAtomicCopyFile(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	cfg := config.NewDefault("test-cluster")
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	// Create source file
	srcPath := filepath.Join(tempDir, "source.txt")
	srcContent := []byte("source content")
	if err := os.WriteFile(srcPath, srcContent, 0o644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Copy file
	writer := NewAtomicWriter(workspace)
	if err := writer.CopyFile(srcPath, "dest.txt", 0o644); err != nil {
		t.Fatalf("Failed to copy file: %v", err)
	}

	// Verify copied content
	destContent, err := os.ReadFile(workspace.GetPath("dest.txt"))
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(destContent) != string(srcContent) {
		t.Errorf("Expected content '%s', got '%s'", string(srcContent), string(destContent))
	}
}

// TestTransaction tests transactional file operations.
func TestTransaction(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	cfg := config.NewDefault("test-cluster")
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	// Create transaction
	tx := NewTransaction(workspace)

	// Add operations
	tx.WriteFile("file1.txt", []byte("content1"), 0o644)
	tx.WriteFile("file2.txt", []byte("content2"), 0o644)
	tx.MkdirAll("subdir", 0o755)
	tx.WriteFile("subdir/file3.txt", []byte("content3"), 0o644)

	// Commit transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Verify all files were created
	if !workspace.Exists("file1.txt") {
		t.Error("file1.txt should exist")
	}

	if !workspace.Exists("file2.txt") {
		t.Error("file2.txt should exist")
	}

	if !workspace.Exists("subdir/file3.txt") {
		t.Error("subdir/file3.txt should exist")
	}

	// Verify content
	content1, _ := os.ReadFile(workspace.GetPath("file1.txt"))
	if string(content1) != "content1" {
		t.Errorf("Expected 'content1', got '%s'", string(content1))
	}
}

// TestTransactionRollback tests transaction rollback on failure.
func TestTransactionRollback(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	cfg := config.NewDefault("test-cluster")
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	// Create initial file
	writer := NewAtomicWriter(workspace)
	writer.WriteFileString("existing.txt", "original", 0o644)

	// Create transaction that will fail
	tx := NewTransaction(workspace)
	tx.WriteFile("file1.txt", []byte("content1"), 0o644)
	tx.WriteFile("file2.txt", []byte("content2"), 0o644)
	// This will fail because we're trying to write to a directory that doesn't exist
	// and the path is invalid (contains null byte)
	tx.WriteFile("invalid\x00path.txt", []byte("content"), 0o644)

	// Commit transaction (should fail and rollback)
	err = tx.Commit()
	if err == nil {
		t.Error("Transaction should fail with invalid path")
	}

	// Verify files were not created (rolled back)
	if workspace.Exists("file1.txt") {
		t.Error("file1.txt should not exist after rollback")
	}

	if workspace.Exists("file2.txt") {
		t.Error("file2.txt should not exist after rollback")
	}

	// Verify existing file is unchanged
	content, _ := os.ReadFile(workspace.GetPath("existing.txt"))
	if string(content) != "original" {
		t.Error("Existing file should be unchanged after rollback")
	}
}

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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WorkspaceCheckpoint represents a snapshot of workspace state at a specific point in time.
// Checkpoints enable rollback capabilities for GitOps generation operations.
type WorkspaceCheckpoint struct {
	// ID is a unique identifier for this checkpoint
	ID string

	// Timestamp is when the checkpoint was created
	Timestamp time.Time

	// Files is a list of files that were present at checkpoint time
	Files []string

	// Metadata stores arbitrary key-value pairs for checkpoint context
	Metadata map[string]interface{}

	// checkpointDir is the directory where checkpoint data is stored
	checkpointDir string
}

// CreateCheckpoint creates a new checkpoint of the current workspace state.
// The checkpoint captures all files in the workspace and stores them for potential rollback.
func (w *GitOpsWorkspace) CreateCheckpoint(checkpointID string) (*WorkspaceCheckpoint, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if checkpoint already exists
	if _, exists := w.Checkpoints[checkpointID]; exists {
		return nil, fmt.Errorf("checkpoint already exists: %s", checkpointID)
	}

	// Create checkpoint directory
	checkpointDir := filepath.Join(w.TempDir, "checkpoints", checkpointID)
	if err := os.MkdirAll(checkpointDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create checkpoint directory: %w", err)
	}

	// Collect all files in workspace
	var files []string
	err := filepath.Walk(w.RootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip temp directory and its contents
		if isPathWithin(w.TempDir, path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip directories, only track files
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(w.RootDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		files = append(files, relPath)

		// Copy file to checkpoint directory
		destPath := filepath.Join(checkpointDir, relPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return fmt.Errorf("failed to create checkpoint subdirectory: %w", err)
		}

		if err := copyFileContent(path, destPath); err != nil {
			return fmt.Errorf("failed to copy file to checkpoint: %w", err)
		}

		return nil
	})

	if err != nil {
		// Clean up partial checkpoint
		os.RemoveAll(checkpointDir)
		return nil, fmt.Errorf("failed to create checkpoint: %w", err)
	}

	// Create checkpoint object
	checkpoint := WorkspaceCheckpoint{
		ID:            checkpointID,
		Timestamp:     time.Now(),
		Files:         files,
		Metadata:      make(map[string]interface{}),
		checkpointDir: checkpointDir,
	}

	// Store checkpoint
	w.Checkpoints[checkpointID] = checkpoint

	return &checkpoint, nil
}

// RestoreCheckpoint restores the workspace to a previous checkpoint state.
// This operation removes all current files and restores files from the checkpoint.
func (w *GitOpsWorkspace) RestoreCheckpoint(checkpointID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Get checkpoint
	checkpoint, exists := w.Checkpoints[checkpointID]
	if !exists {
		return fmt.Errorf("checkpoint not found: %s", checkpointID)
	}

	// Collect all files and directories to remove (except temp directory)
	var toRemove []string
	err := filepath.Walk(w.RootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Ignore errors for files that don't exist (may have been removed already)
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		// Skip temp directory
		if isPathWithin(w.TempDir, path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip root directory itself
		if path == w.RootDir {
			return nil
		}

		// Add to removal list
		toRemove = append(toRemove, path)

		// Skip subdirectories since we'll remove the parent
		if info.IsDir() {
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to scan workspace for restore: %w", err)
	}

	// Remove collected items
	for _, path := range toRemove {
		if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s: %w", path, err)
		}
	}

	// Restore files from checkpoint
	for _, relPath := range checkpoint.Files {
		srcPath := filepath.Join(checkpoint.checkpointDir, relPath)
		destPath := filepath.Join(w.RootDir, relPath)

		// Create destination directory
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return fmt.Errorf("failed to create directory for restore: %w", err)
		}

		// Copy file from checkpoint
		if err := copyFileContent(srcPath, destPath); err != nil {
			return fmt.Errorf("failed to restore file %s: %w", relPath, err)
		}
	}

	w.lastModified = time.Now()

	return nil
}

// DeleteCheckpoint removes a checkpoint and frees its storage.
func (w *GitOpsWorkspace) DeleteCheckpoint(checkpointID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Get checkpoint
	checkpoint, exists := w.Checkpoints[checkpointID]
	if !exists {
		return fmt.Errorf("checkpoint not found: %s", checkpointID)
	}

	// Remove checkpoint directory
	if err := os.RemoveAll(checkpoint.checkpointDir); err != nil {
		return fmt.Errorf("failed to remove checkpoint directory: %w", err)
	}

	// Remove from map
	delete(w.Checkpoints, checkpointID)

	return nil
}

// ListCheckpoints returns a list of all checkpoint IDs in the workspace.
func (w *GitOpsWorkspace) ListCheckpoints() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	checkpoints := make([]string, 0, len(w.Checkpoints))
	for id := range w.Checkpoints {
		checkpoints = append(checkpoints, id)
	}

	return checkpoints
}

// GetCheckpoint retrieves a checkpoint by ID.
func (w *GitOpsWorkspace) GetCheckpoint(checkpointID string) (*WorkspaceCheckpoint, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	checkpoint, exists := w.Checkpoints[checkpointID]
	if !exists {
		return nil, fmt.Errorf("checkpoint not found: %s", checkpointID)
	}

	return &checkpoint, nil
}

// isPathWithin reports whether target is base or one of base's descendants.
// filepath.Rel keeps similarly prefixed sibling paths outside the boundary.
func isPathWithin(base, target string) bool {
	relPath, err := filepath.Rel(base, target)
	if err != nil || filepath.IsAbs(relPath) {
		return false
	}

	return relPath != ".." && !strings.HasPrefix(relPath, ".."+string(filepath.Separator))
}

// copyFileContent copies the contents of a file from src to dst.
// This is a helper function used by checkpoint operations.
func copyFileContent(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	return nil
}

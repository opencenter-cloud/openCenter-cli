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
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// DryRunWorkspace is a workspace that tracks operations without making filesystem changes.
// It provides an accurate preview of what would be generated while ensuring no actual
// files or directories are created.
type DryRunWorkspace struct {
	// ID is a unique identifier for this workspace
	ID string

	// RootDir is the simulated root directory (not actually created)
	RootDir string

	// TempDir is the simulated temporary directory (not actually created)
	TempDir string

	// Config is the cluster configuration associated with this workspace
	Config v2.Config

	// Metadata stores arbitrary key-value pairs for workspace context
	Metadata map[string]interface{}

	// Checkpoints stores workspace checkpoints (simulated)
	Checkpoints map[string]WorkspaceCheckpoint

	// operations tracks all operations that would be performed
	operations []DryRunOperation

	// files tracks files that would be created
	files map[string]DryRunFile

	// directories tracks directories that would be created
	directories map[string]bool

	// mu protects concurrent access to workspace state
	mu sync.RWMutex

	// createdAt tracks when the workspace was created
	createdAt time.Time

	// lastModified tracks the last modification time
	lastModified time.Time
}

// DryRunOperation represents a single operation that would be performed.
type DryRunOperation struct {
	Type      OperationType
	Path      string
	Content   string
	Mode      os.FileMode
	Timestamp time.Time
	Stage     string
}

// OperationType represents the type of operation.
type OperationType string

const (
	OpCreateDir  OperationType = "create_dir"
	OpWriteFile  OperationType = "write_file"
	OpRemoveFile OperationType = "remove_file"
	OpRemoveDir  OperationType = "remove_dir"
)

// DryRunFile represents a file that would be created.
type DryRunFile struct {
	Path      string
	Content   string
	Mode      os.FileMode
	CreatedBy string // Stage name
	Size      int64
}

// NewDryRunWorkspace creates a new dry-run workspace that simulates operations.
func NewDryRunWorkspace(cfg v2.Config) *DryRunWorkspace {
	workspaceID := fmt.Sprintf("dryrun-%s-%d", cfg.ClusterName(), time.Now().UnixNano())

	return &DryRunWorkspace{
		ID:           workspaceID,
		RootDir:      filepath.Join("/tmp/gitops-dryrun", workspaceID),
		TempDir:      filepath.Join("/tmp/gitops-dryrun", workspaceID, ".tmp"),
		Config:       cfg,
		Metadata:     make(map[string]interface{}),
		Checkpoints:  make(map[string]WorkspaceCheckpoint),
		operations:   make([]DryRunOperation, 0),
		files:        make(map[string]DryRunFile, 0),
		directories:  make(map[string]bool),
		createdAt:    time.Now(),
		lastModified: time.Now(),
	}
}

// GetMetadata retrieves a metadata value from the workspace.
func (w *DryRunWorkspace) GetMetadata(key string) (interface{}, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	value, exists := w.Metadata[key]
	return value, exists
}

// SetMetadata sets a metadata value in the workspace.
func (w *DryRunWorkspace) SetMetadata(key string, value interface{}) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.Metadata[key] = value
	w.lastModified = time.Now()
}

// GetPath returns the simulated absolute path for a relative path within the workspace.
func (w *DryRunWorkspace) GetPath(relativePath string) string {
	return filepath.Join(w.RootDir, relativePath)
}

// GetTempPath returns the simulated absolute path for a relative path within the temp directory.
func (w *DryRunWorkspace) GetTempPath(relativePath string) string {
	return filepath.Join(w.TempDir, relativePath)
}

// Exists checks if a file or directory would exist in the simulated workspace.
func (w *DryRunWorkspace) Exists(relativePath string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Check if it's a file
	if _, exists := w.files[relativePath]; exists {
		return true
	}

	// Check if it's a directory
	if w.directories[relativePath] {
		return true
	}

	// Check if any file or directory has this as a parent
	for path := range w.files {
		if filepath.Dir(path) == relativePath || isParentDir(relativePath, path) {
			return true
		}
	}

	for dir := range w.directories {
		if filepath.Dir(dir) == relativePath || isParentDir(relativePath, dir) {
			return true
		}
	}

	return false
}

// CreatedAt returns the workspace creation time.
func (w *DryRunWorkspace) CreatedAt() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.createdAt
}

// LastModified returns the last modification time.
func (w *DryRunWorkspace) LastModified() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastModified
}

// UpdateModifiedTime updates the last modified timestamp.
func (w *DryRunWorkspace) UpdateModifiedTime() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastModified = time.Now()
}

// CreateCheckpoint creates a simulated checkpoint.
func (w *DryRunWorkspace) CreateCheckpoint(checkpointID string) (WorkspaceCheckpoint, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Create a snapshot of current files
	files := make([]string, 0, len(w.files))
	for path := range w.files {
		files = append(files, path)
	}

	checkpoint := WorkspaceCheckpoint{
		ID:        checkpointID,
		Timestamp: time.Now(),
		Files:     files,
		Metadata:  make(map[string]interface{}),
	}

	w.Checkpoints[checkpointID] = checkpoint
	return checkpoint, nil
}

// RestoreCheckpoint simulates restoring to a checkpoint.
func (w *DryRunWorkspace) RestoreCheckpoint(checkpointID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	checkpoint, exists := w.Checkpoints[checkpointID]
	if !exists {
		return fmt.Errorf("checkpoint not found: %s", checkpointID)
	}

	// In dry-run mode, we just record that a restore would happen
	w.operations = append(w.operations, DryRunOperation{
		Type:      "restore_checkpoint",
		Path:      checkpointID,
		Timestamp: time.Now(),
	})

	// Simulate restoring files
	restoredFiles := make(map[string]DryRunFile)
	for _, path := range checkpoint.Files {
		if file, exists := w.files[path]; exists {
			restoredFiles[path] = file
		}
	}
	w.files = restoredFiles

	return nil
}

// DeleteCheckpoint removes a simulated checkpoint.
func (w *DryRunWorkspace) DeleteCheckpoint(checkpointID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.Checkpoints[checkpointID]; !exists {
		return fmt.Errorf("checkpoint not found: %s", checkpointID)
	}

	delete(w.Checkpoints, checkpointID)
	return nil
}

// RecordOperation records an operation that would be performed.
func (w *DryRunWorkspace) RecordOperation(op DryRunOperation) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.operations = append(w.operations, op)
	w.lastModified = time.Now()

	// Update internal state based on operation
	switch op.Type {
	case OpCreateDir:
		w.directories[op.Path] = true
	case OpWriteFile:
		w.files[op.Path] = DryRunFile{
			Path:      op.Path,
			Content:   op.Content,
			Mode:      op.Mode,
			CreatedBy: op.Stage,
			Size:      int64(len(op.Content)),
		}
	case OpRemoveFile:
		delete(w.files, op.Path)
	case OpRemoveDir:
		delete(w.directories, op.Path)
	}
}

// GetOperations returns all recorded operations.
func (w *DryRunWorkspace) GetOperations() []DryRunOperation {
	w.mu.RLock()
	defer w.mu.RUnlock()

	ops := make([]DryRunOperation, len(w.operations))
	copy(ops, w.operations)
	return ops
}

// GetFiles returns all files that would be created.
func (w *DryRunWorkspace) GetFiles() map[string]DryRunFile {
	w.mu.RLock()
	defer w.mu.RUnlock()

	files := make(map[string]DryRunFile, len(w.files))
	for k, v := range w.files {
		files[k] = v
	}
	return files
}

// GetDirectories returns all directories that would be created.
func (w *DryRunWorkspace) GetDirectories() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	dirs := make([]string, 0, len(w.directories))
	for dir := range w.directories {
		dirs = append(dirs, dir)
	}
	return dirs
}

// GetOperationCount returns the total number of operations recorded.
func (w *DryRunWorkspace) GetOperationCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.operations)
}

// GetFileCount returns the number of files that would be created.
func (w *DryRunWorkspace) GetFileCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.files)
}

// GetDirectoryCount returns the number of directories that would be created.
func (w *DryRunWorkspace) GetDirectoryCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.directories)
}

// GetTotalSize returns the total size of all files that would be created.
func (w *DryRunWorkspace) GetTotalSize() int64 {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var total int64
	for _, file := range w.files {
		total += file.Size
	}
	return total
}

// GenerateSummary creates a summary of what would be generated.
func (w *DryRunWorkspace) GenerateSummary() DryRunSummary {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return DryRunSummary{
		WorkspaceID:        w.ID,
		TotalOperations:    len(w.operations),
		FilesCreated:       len(w.files),
		DirectoriesCreated: len(w.directories),
		TotalSize:          w.GetTotalSize(),
		Operations:         w.GetOperations(),
		Files:              w.GetFiles(),
		Directories:        w.GetDirectories(),
	}
}

// DryRunSummary provides a summary of dry-run operations.
type DryRunSummary struct {
	WorkspaceID        string
	TotalOperations    int
	FilesCreated       int
	DirectoriesCreated int
	TotalSize          int64
	Operations         []DryRunOperation
	Files              map[string]DryRunFile
	Directories        []string
}

// isParentDir checks if parent is a parent directory of child.
func isParentDir(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)

	for {
		child = filepath.Dir(child)
		if child == "." || child == "/" {
			return false
		}
		if child == parent {
			return true
		}
	}
}

// DryRunWorkspaceManager manages dry-run workspaces.
type DryRunWorkspaceManager struct {
	workspaces map[string]*DryRunWorkspace
	mu         sync.RWMutex
}

// NewDryRunWorkspaceManager creates a new dry-run workspace manager.
func NewDryRunWorkspaceManager() *DryRunWorkspaceManager {
	return &DryRunWorkspaceManager{
		workspaces: make(map[string]*DryRunWorkspace),
	}
}

// CreateWorkspace creates a new dry-run workspace.
func (m *DryRunWorkspaceManager) CreateWorkspace(ctx context.Context, cfg v2.Config) (*GitOpsWorkspace, error) {
	dryRunWS := NewDryRunWorkspace(cfg)

	m.mu.Lock()
	m.workspaces[dryRunWS.ID] = dryRunWS
	m.mu.Unlock()

	// Return as GitOpsWorkspace interface
	// Note: This requires DryRunWorkspace to implement the same interface as GitOpsWorkspace
	// For now, we'll wrap it
	return &GitOpsWorkspace{
		ID:           dryRunWS.ID,
		RootDir:      dryRunWS.RootDir,
		TempDir:      dryRunWS.TempDir,
		Config:       dryRunWS.Config,
		Metadata:     dryRunWS.Metadata,
		Checkpoints:  dryRunWS.Checkpoints,
		createdAt:    dryRunWS.createdAt,
		lastModified: dryRunWS.lastModified,
	}, nil
}

// CleanupWorkspace cleans up a dry-run workspace (no-op since nothing was created).
func (m *DryRunWorkspaceManager) CleanupWorkspace(ctx context.Context, workspace *GitOpsWorkspace) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.workspaces, workspace.ID)
	return nil
}

// GetWorkspace retrieves a dry-run workspace by ID.
func (m *DryRunWorkspaceManager) GetWorkspace(ctx context.Context, workspaceID string) (*GitOpsWorkspace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dryRunWS, exists := m.workspaces[workspaceID]
	if !exists {
		return nil, fmt.Errorf("workspace not found: %s", workspaceID)
	}

	return &GitOpsWorkspace{
		ID:           dryRunWS.ID,
		RootDir:      dryRunWS.RootDir,
		TempDir:      dryRunWS.TempDir,
		Config:       dryRunWS.Config,
		Metadata:     dryRunWS.Metadata,
		Checkpoints:  dryRunWS.Checkpoints,
		createdAt:    dryRunWS.createdAt,
		lastModified: dryRunWS.lastModified,
	}, nil
}

// GetDryRunWorkspace retrieves the actual dry-run workspace for inspection.
func (m *DryRunWorkspaceManager) GetDryRunWorkspace(workspaceID string) (*DryRunWorkspace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	workspace, exists := m.workspaces[workspaceID]
	if !exists {
		return nil, fmt.Errorf("workspace not found: %s", workspaceID)
	}

	return workspace, nil
}

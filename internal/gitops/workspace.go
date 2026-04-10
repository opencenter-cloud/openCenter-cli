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

// WorkspaceManager manages GitOps workspace lifecycle including creation,
// cleanup, and resource management.
type WorkspaceManager interface {
	// CreateWorkspace creates a new isolated workspace for GitOps generation
	CreateWorkspace(ctx context.Context, cfg v2.Config) (*GitOpsWorkspace, error)

	// CleanupWorkspace removes a workspace and all its resources
	CleanupWorkspace(ctx context.Context, workspace *GitOpsWorkspace) error

	// GetWorkspace retrieves an existing workspace by ID
	GetWorkspace(ctx context.Context, workspaceID string) (*GitOpsWorkspace, error)
}

// GitOpsWorkspace represents an isolated environment for GitOps repository generation.
// It provides a safe space for file operations with support for checkpointing and rollback.
type GitOpsWorkspace struct {
	// ID is a unique identifier for this workspace
	ID string

	// RootDir is the root directory of the workspace where all files are generated
	RootDir string

	// TempDir is a temporary directory for intermediate operations
	TempDir string

	// Config is the cluster configuration associated with this workspace
	Config v2.Config

	// Metadata stores arbitrary key-value pairs for workspace context
	Metadata map[string]interface{}

	// Checkpoints stores workspace checkpoints for rollback capability
	Checkpoints map[string]WorkspaceCheckpoint

	// mu protects concurrent access to workspace state
	mu sync.RWMutex

	// createdAt tracks when the workspace was created
	createdAt time.Time

	// lastModified tracks the last modification time
	lastModified time.Time
}

// DefaultWorkspaceManager implements WorkspaceManager with standard filesystem operations.
type DefaultWorkspaceManager struct {
	// baseDir is the base directory where workspaces are created
	baseDir string

	// workspaces tracks active workspaces
	workspaces map[string]*GitOpsWorkspace

	// mu protects concurrent access to the workspace registry
	mu sync.RWMutex

	// maxWorkspaceAge is the maximum age for a workspace before it's considered stale
	maxWorkspaceAge time.Duration

	// cleanupInterval is how often to run automatic cleanup
	cleanupInterval time.Duration

	// stopCleanup signals the cleanup goroutine to stop
	stopCleanup chan struct{}

	// cleanupDone signals when cleanup goroutine has finished
	cleanupDone chan struct{}
}

// NewWorkspaceManager creates a new workspace manager with the specified base directory.
// If baseDir is empty, it uses the system temporary directory.
// The manager starts an automatic cleanup goroutine to prevent workspace leaks.
func NewWorkspaceManager(baseDir string) WorkspaceManager {
	if baseDir == "" {
		baseDir = os.TempDir()
	}

	manager := &DefaultWorkspaceManager{
		baseDir:         baseDir,
		workspaces:      make(map[string]*GitOpsWorkspace),
		maxWorkspaceAge: 24 * time.Hour, // Default: clean up workspaces older than 24 hours
		cleanupInterval: 1 * time.Hour,  // Default: run cleanup every hour
		stopCleanup:     make(chan struct{}),
		cleanupDone:     make(chan struct{}),
	}

	// Start automatic cleanup goroutine
	go manager.runCleanupLoop()

	return manager
}

// NewWorkspaceManagerWithOptions creates a workspace manager with custom options.
func NewWorkspaceManagerWithOptions(baseDir string, maxAge, cleanupInterval time.Duration) WorkspaceManager {
	if baseDir == "" {
		baseDir = os.TempDir()
	}

	manager := &DefaultWorkspaceManager{
		baseDir:         baseDir,
		workspaces:      make(map[string]*GitOpsWorkspace),
		maxWorkspaceAge: maxAge,
		cleanupInterval: cleanupInterval,
		stopCleanup:     make(chan struct{}),
		cleanupDone:     make(chan struct{}),
	}

	// Start automatic cleanup goroutine
	go manager.runCleanupLoop()

	return manager
}

// CreateWorkspace creates a new isolated workspace for GitOps generation.
// The workspace provides an isolated environment with its own directory structure.
func (m *DefaultWorkspaceManager) CreateWorkspace(ctx context.Context, cfg v2.Config) (*GitOpsWorkspace, error) {
	// Generate unique workspace ID
	workspaceID := fmt.Sprintf("workspace-%s-%d", cfg.ClusterName(), time.Now().UnixNano())

	// Create workspace root directory
	rootDir := filepath.Join(m.baseDir, "gitops-workspaces", workspaceID)
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create workspace root directory: %w", err)
	}

	// Create temporary directory for intermediate operations
	tempDir := filepath.Join(rootDir, ".tmp")
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create workspace temp directory: %w", err)
	}

	// Initialize workspace
	workspace := &GitOpsWorkspace{
		ID:           workspaceID,
		RootDir:      rootDir,
		TempDir:      tempDir,
		Config:       cfg,
		Metadata:     make(map[string]interface{}),
		Checkpoints:  make(map[string]WorkspaceCheckpoint),
		createdAt:    time.Now(),
		lastModified: time.Now(),
	}

	// Register workspace
	m.mu.Lock()
	m.workspaces[workspaceID] = workspace
	m.mu.Unlock()

	return workspace, nil
}

// CleanupWorkspace removes a workspace and all its resources.
// This operation is irreversible and should be called when generation is complete or failed.
func (m *DefaultWorkspaceManager) CleanupWorkspace(ctx context.Context, workspace *GitOpsWorkspace) error {
	if workspace == nil {
		return fmt.Errorf("workspace is nil")
	}

	// Clean up all checkpoints first
	workspace.mu.Lock()
	checkpointIDs := make([]string, 0, len(workspace.Checkpoints))
	for id := range workspace.Checkpoints {
		checkpointIDs = append(checkpointIDs, id)
	}
	workspace.mu.Unlock()

	// Delete all checkpoints
	for _, id := range checkpointIDs {
		if err := workspace.DeleteCheckpoint(id); err != nil {
			// Log error but continue cleanup
			fmt.Fprintf(os.Stderr, "Warning: failed to delete checkpoint %s: %v\n", id, err)
		}
	}

	// Unregister workspace
	m.mu.Lock()
	delete(m.workspaces, workspace.ID)
	m.mu.Unlock()

	// Remove workspace directory
	if err := os.RemoveAll(workspace.RootDir); err != nil {
		return fmt.Errorf("failed to remove workspace directory: %w", err)
	}

	return nil
}

// GetWorkspace retrieves an existing workspace by ID.
func (m *DefaultWorkspaceManager) GetWorkspace(ctx context.Context, workspaceID string) (*GitOpsWorkspace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	workspace, exists := m.workspaces[workspaceID]
	if !exists {
		return nil, fmt.Errorf("workspace not found: %s", workspaceID)
	}

	return workspace, nil
}

// GetMetadata retrieves a metadata value from the workspace.
func (w *GitOpsWorkspace) GetMetadata(key string) (interface{}, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	value, exists := w.Metadata[key]
	return value, exists
}

// SetMetadata sets a metadata value in the workspace.
func (w *GitOpsWorkspace) SetMetadata(key string, value interface{}) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.Metadata[key] = value
	w.lastModified = time.Now()
}

// GetPath returns the absolute path for a relative path within the workspace.
func (w *GitOpsWorkspace) GetPath(relativePath string) string {
	return filepath.Join(w.RootDir, relativePath)
}

// GetTempPath returns the absolute path for a relative path within the temp directory.
func (w *GitOpsWorkspace) GetTempPath(relativePath string) string {
	return filepath.Join(w.TempDir, relativePath)
}

// Exists checks if a file or directory exists in the workspace.
func (w *GitOpsWorkspace) Exists(relativePath string) bool {
	path := w.GetPath(relativePath)
	_, err := os.Stat(path)
	return err == nil
}

// CreatedAt returns the workspace creation time.
func (w *GitOpsWorkspace) CreatedAt() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.createdAt
}

// LastModified returns the last modification time.
func (w *GitOpsWorkspace) LastModified() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastModified
}

// UpdateModifiedTime updates the last modified timestamp.
func (w *GitOpsWorkspace) UpdateModifiedTime() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastModified = time.Now()
}

// GetFileCount returns the number of files in the workspace.
// It recursively counts all files in the workspace root directory.
func (w *GitOpsWorkspace) GetFileCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()

	count := 0
	filepath.Walk(w.RootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})
	return count
}

// runCleanupLoop runs a periodic cleanup of stale workspaces.
// This prevents workspace leaks from abandoned or crashed operations.
func (m *DefaultWorkspaceManager) runCleanupLoop() {
	defer close(m.cleanupDone)

	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupStaleWorkspaces()
		case <-m.stopCleanup:
			return
		}
	}
}

// cleanupStaleWorkspaces removes workspaces that haven't been modified recently.
// This helps prevent resource leaks from abandoned workspaces.
func (m *DefaultWorkspaceManager) cleanupStaleWorkspaces() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var toCleanup []*GitOpsWorkspace

	// Find stale workspaces
	for _, workspace := range m.workspaces {
		workspace.mu.RLock()
		age := now.Sub(workspace.lastModified)
		workspace.mu.RUnlock()

		if age > m.maxWorkspaceAge {
			toCleanup = append(toCleanup, workspace)
		}
	}

	// Clean up stale workspaces
	for _, workspace := range toCleanup {
		// Remove from registry first
		delete(m.workspaces, workspace.ID)

		// Clean up checkpoints
		workspace.mu.Lock()
		checkpointIDs := make([]string, 0, len(workspace.Checkpoints))
		for id := range workspace.Checkpoints {
			checkpointIDs = append(checkpointIDs, id)
		}
		workspace.mu.Unlock()

		for _, id := range checkpointIDs {
			if err := workspace.DeleteCheckpoint(id); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to delete checkpoint %s during cleanup: %v\n", id, err)
			}
		}

		// Remove workspace directory
		if err := os.RemoveAll(workspace.RootDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove stale workspace %s: %v\n", workspace.ID, err)
		}
	}
}

// Shutdown stops the workspace manager and cleans up all resources.
// This should be called when the application is shutting down.
func (m *DefaultWorkspaceManager) Shutdown(ctx context.Context) error {
	// Stop cleanup goroutine
	close(m.stopCleanup)

	// Wait for cleanup to finish with timeout
	select {
	case <-m.cleanupDone:
		// Cleanup goroutine finished
	case <-ctx.Done():
		return fmt.Errorf("shutdown timeout waiting for cleanup goroutine")
	case <-time.After(5 * time.Second):
		return fmt.Errorf("shutdown timeout after 5 seconds")
	}

	// Clean up all remaining workspaces
	m.mu.Lock()
	workspaces := make([]*GitOpsWorkspace, 0, len(m.workspaces))
	for _, workspace := range m.workspaces {
		workspaces = append(workspaces, workspace)
	}
	m.mu.Unlock()

	for _, workspace := range workspaces {
		if err := m.CleanupWorkspace(ctx, workspace); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to cleanup workspace %s during shutdown: %v\n", workspace.ID, err)
		}
	}

	return nil
}

// CleanupOrphanedWorkspaces scans the workspace directory and removes any
// workspace directories that are not tracked in the manager's registry.
// This helps recover from crashes or improper shutdowns.
func (m *DefaultWorkspaceManager) CleanupOrphanedWorkspaces(ctx context.Context) error {
	workspaceBaseDir := filepath.Join(m.baseDir, "gitops-workspaces")

	// Check if workspace directory exists
	if _, err := os.Stat(workspaceBaseDir); os.IsNotExist(err) {
		return nil // Nothing to clean up
	}

	// Read workspace directories
	entries, err := os.ReadDir(workspaceBaseDir)
	if err != nil {
		return fmt.Errorf("failed to read workspace directory: %w", err)
	}

	m.mu.RLock()
	trackedWorkspaces := make(map[string]bool)
	for id := range m.workspaces {
		trackedWorkspaces[id] = true
	}
	m.mu.RUnlock()

	// Remove orphaned workspace directories
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		workspaceID := entry.Name()

		// Skip if workspace is tracked
		if trackedWorkspaces[workspaceID] {
			continue
		}

		// Check if workspace is recent (within grace period)
		workspaceDir := filepath.Join(workspaceBaseDir, workspaceID)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		age := time.Since(info.ModTime())
		if age < m.maxWorkspaceAge {
			// Give it more time before cleaning up
			continue
		}

		// Remove orphaned workspace
		if err := os.RemoveAll(workspaceDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove orphaned workspace %s: %v\n", workspaceID, err)
		}
	}

	return nil
}

// GetActiveWorkspaceCount returns the number of currently active workspaces.
func (m *DefaultWorkspaceManager) GetActiveWorkspaceCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.workspaces)
}

// ListActiveWorkspaces returns a list of all active workspace IDs.
func (m *DefaultWorkspaceManager) ListActiveWorkspaces() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	workspaceIDs := make([]string, 0, len(m.workspaces))
	for id := range m.workspaces {
		workspaceIDs = append(workspaceIDs, id)
	}

	return workspaceIDs
}

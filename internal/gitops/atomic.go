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
	"os"
	"path/filepath"

	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// AtomicWriter provides atomic file write operations to prevent partial writes.
// Files are written to a temporary location and then atomically moved to the final destination.
// In dry-run mode, operations are recorded without making filesystem changes.
type AtomicWriter struct {
	workspace    *GitOpsWorkspace
	dryRunWriter *DryRunAtomicWriter // nil if not in dry-run mode
	fileSystem   fs.FileSystem
}

// NewAtomicWriter creates a new atomic writer for the given workspace.
// If the workspace is in dry-run mode, it returns a dry-run writer that records
// operations without making filesystem changes.
func NewAtomicWriter(workspace *GitOpsWorkspace) *AtomicWriter {
	// Create FileSystem instance
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)

	// Check if workspace is in dry-run mode
	if isDryRun, ok := workspace.GetMetadata("is_dryrun"); ok && isDryRun.(bool) {
		// Get the dry-run workspace from metadata
		if dryRunWS, ok := workspace.GetMetadata("dryrun_workspace"); ok {
			if drw, ok := dryRunWS.(*DryRunWorkspace); ok {
				// Return a wrapper that implements AtomicWriter interface
				// but delegates to dry-run operations
				return &AtomicWriter{
					workspace:    workspace,
					dryRunWriter: NewDryRunAtomicWriter(drw, ""),
					fileSystem:   fileSystem,
				}
			}
		}
	}

	return &AtomicWriter{
		workspace:  workspace,
		fileSystem: fileSystem,
	}
}

// WriteFile writes data to a file atomically within the workspace.
// The file is first written to a temporary location and then moved to the final path.
// This ensures that the file is either fully written or not present at all.
// In dry-run mode, the operation is recorded without making filesystem changes.
func (aw *AtomicWriter) WriteFile(relativePath string, data []byte, perm os.FileMode) error {
	// If in dry-run mode, record the operation instead of writing
	if aw.dryRunWriter != nil {
		return aw.dryRunWriter.WriteFile(relativePath, data, perm)
	}

	// Get final destination path
	destPath := aw.workspace.GetPath(relativePath)

	// Create destination directory if it doesn't exist
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Create temporary file in workspace temp directory
	tempFile, err := os.CreateTemp(aw.workspace.TempDir, "atomic-write-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tempPath := tempFile.Name()

	// Ensure temp file is cleaned up on error
	defer func() {
		if tempFile != nil {
			_ = tempFile.Close()
			_ = os.Remove(tempPath)
		}
	}()

	// Write data to temporary file
	if _, err := tempFile.Write(data); err != nil {
		return fmt.Errorf("failed to write to temporary file: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}

	// Close temporary file
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}
	tempFile = nil // Prevent cleanup in defer

	// Set file permissions
	if err := os.Chmod(tempPath, perm); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Atomically move temporary file to destination
	if err := os.Rename(tempPath, destPath); err != nil {
		return fmt.Errorf("failed to move file to destination: %w", err)
	}

	// Update workspace modified time
	aw.workspace.UpdateModifiedTime()

	return nil
}

// WriteFileString writes a string to a file atomically within the workspace.
// In dry-run mode, the operation is recorded without making filesystem changes.
func (aw *AtomicWriter) WriteFileString(relativePath string, content string, perm os.FileMode) error {
	// If in dry-run mode, record the operation instead of writing
	if aw.dryRunWriter != nil {
		return aw.dryRunWriter.WriteFileString(relativePath, content, perm)
	}

	return aw.WriteFile(relativePath, []byte(content), perm)
}

// CopyFile copies a file atomically within the workspace.
// The source file is read and then written atomically to the destination.
func (aw *AtomicWriter) CopyFile(srcPath, destRelativePath string, perm os.FileMode) error {
	// Read source file using FileSystem wrapper
	data, err := aw.fileSystem.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	// Write atomically to destination
	return aw.WriteFile(destRelativePath, data, perm)
}

// RemoveFile removes a file from the workspace.
// This operation is not atomic but is included for completeness.
// In dry-run mode, the operation is recorded without making filesystem changes.
func (aw *AtomicWriter) RemoveFile(relativePath string) error {
	// If in dry-run mode, record the operation instead of removing
	if aw.dryRunWriter != nil {
		return aw.dryRunWriter.Remove(relativePath)
	}

	path := aw.workspace.GetPath(relativePath)

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove file: %w", err)
	}

	// Update workspace modified time
	aw.workspace.UpdateModifiedTime()

	return nil
}

// MkdirAll creates a directory and all necessary parent directories.
// In dry-run mode, the operation is recorded without making filesystem changes.
func (aw *AtomicWriter) MkdirAll(relativePath string, perm os.FileMode) error {
	// If in dry-run mode, record the operation instead of creating directories
	if aw.dryRunWriter != nil {
		return aw.dryRunWriter.MkdirAll(relativePath, perm)
	}

	path := aw.workspace.GetPath(relativePath)

	if err := os.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Update workspace modified time
	aw.workspace.UpdateModifiedTime()

	return nil
}

// Transaction represents a set of file operations that can be committed or rolled back atomically.
type Transaction struct {
	workspace  *GitOpsWorkspace
	operations []transactionOp
	committed  bool
}

// transactionOp represents a single operation in a transaction.
type transactionOp struct {
	opType       string // "write", "remove", "mkdir"
	relativePath string
	data         []byte
	perm         os.FileMode
}

// NewTransaction creates a new transaction for atomic multi-file operations.
func NewTransaction(workspace *GitOpsWorkspace) *Transaction {
	return &Transaction{
		workspace:  workspace,
		operations: make([]transactionOp, 0),
		committed:  false,
	}
}

// WriteFile adds a file write operation to the transaction.
func (t *Transaction) WriteFile(relativePath string, data []byte, perm os.FileMode) {
	t.operations = append(t.operations, transactionOp{
		opType:       "write",
		relativePath: relativePath,
		data:         data,
		perm:         perm,
	})
}

// RemoveFile adds a file removal operation to the transaction.
func (t *Transaction) RemoveFile(relativePath string) {
	t.operations = append(t.operations, transactionOp{
		opType:       "remove",
		relativePath: relativePath,
	})
}

// MkdirAll adds a directory creation operation to the transaction.
func (t *Transaction) MkdirAll(relativePath string, perm os.FileMode) {
	t.operations = append(t.operations, transactionOp{
		opType:       "mkdir",
		relativePath: relativePath,
		perm:         perm,
	})
}

// Commit executes all operations in the transaction atomically.
// If any operation fails, all previous operations are rolled back.
func (t *Transaction) Commit() error {
	if t.committed {
		return fmt.Errorf("transaction already committed")
	}

	// Create a checkpoint before committing
	checkpointID := fmt.Sprintf("transaction-%d", t.workspace.LastModified().UnixNano())
	checkpoint, err := t.workspace.CreateCheckpoint(checkpointID)
	if err != nil {
		return fmt.Errorf("failed to create transaction checkpoint: %w", err)
	}

	// Ensure checkpoint is cleaned up
	defer func() {
		if checkpoint != nil {
			_ = t.workspace.DeleteCheckpoint(checkpointID)
		}
	}()

	// Execute all operations
	writer := NewAtomicWriter(t.workspace)
	for i, op := range t.operations {
		var opErr error

		switch op.opType {
		case "write":
			opErr = writer.WriteFile(op.relativePath, op.data, op.perm)
		case "remove":
			opErr = writer.RemoveFile(op.relativePath)
		case "mkdir":
			opErr = writer.MkdirAll(op.relativePath, op.perm)
		default:
			opErr = fmt.Errorf("unknown operation type: %s", op.opType)
		}

		if opErr != nil {
			// Rollback to checkpoint
			if rollbackErr := t.workspace.RestoreCheckpoint(checkpointID); rollbackErr != nil {
				return fmt.Errorf("operation %d failed and rollback failed: %w (original error: %v)", i, rollbackErr, opErr)
			}
			return fmt.Errorf("operation %d failed and was rolled back: %w", i, opErr)
		}
	}

	t.committed = true
	checkpoint = nil // Prevent cleanup

	return nil
}

// Rollback discards all operations in the transaction without executing them.
func (t *Transaction) Rollback() {
	t.operations = nil
	t.committed = true
}

// SetStage sets the current stage name for dry-run operation tracking.
// This is used to associate operations with the stage that performed them.
func (aw *AtomicWriter) SetStage(stageName string) {
	if aw.dryRunWriter != nil {
		aw.dryRunWriter.SetStage(stageName)
	}
}

// IsDryRun returns true if this writer is in dry-run mode.
func (aw *AtomicWriter) IsDryRun() bool {
	return aw.dryRunWriter != nil
}

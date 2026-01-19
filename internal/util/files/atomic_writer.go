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

package files

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rackerlabs/openCenter-cli/internal/resilience"
)

// DefaultAtomicFileWriter implements AtomicFileWriter interface
type DefaultAtomicFileWriter struct {
	validator    FileValidator
	retryHandler resilience.RetryHandler
}

// NewDefaultAtomicFileWriter creates a new default atomic file writer
func NewDefaultAtomicFileWriter() *DefaultAtomicFileWriter {
	return &DefaultAtomicFileWriter{
		validator:    NewDefaultFileValidator(),
		retryHandler: resilience.NewRetryHandler(resilience.FileOperationConfig),
	}
}

// WriteAtomic writes data to a file atomically
func (w *DefaultAtomicFileWriter) WriteAtomic(filename string, data []byte, perm os.FileMode) error {
	ctx := context.Background()
	return w.retryHandler.Do(ctx, func() error {
		// Create temporary file in the same directory as the target file
		dir := filepath.Dir(filename)
		tempFile, err := w.CreateTempFile(dir, ".tmp-"+filepath.Base(filename)+"-")
		if err != nil {
			return fmt.Errorf("failed to create temporary file: %w", err)
		}

		tempPath := tempFile.Name()

		// Ensure cleanup on failure
		defer func() {
			tempFile.Close()
			os.Remove(tempPath)
		}()

		// Write data to temporary file
		if _, err := tempFile.Write(data); err != nil {
			return fmt.Errorf("failed to write to temporary file: %w", err)
		}

		// Set proper permissions
		if err := tempFile.Chmod(perm); err != nil {
			return fmt.Errorf("failed to set file permissions: %w", err)
		}

		// Sync to ensure data is written to disk
		if err := tempFile.Sync(); err != nil {
			return fmt.Errorf("failed to sync temporary file: %w", err)
		}

		// Close the temporary file
		if err := tempFile.Close(); err != nil {
			return fmt.Errorf("failed to close temporary file: %w", err)
		}

		// Atomically move temporary file to final location
		if err := os.Rename(tempPath, filename); err != nil {
			return fmt.Errorf("failed to move temporary file to final location: %w", err)
		}

		// Don't remove tempPath in defer since rename succeeded
		tempPath = ""
		return nil
	})
}

// WriteAtomicWithBackup writes data to a file atomically with backup
func (w *DefaultAtomicFileWriter) WriteAtomicWithBackup(filename string, data []byte, perm os.FileMode) error {
	var backupPath string

	// Create backup if file exists
	if _, err := os.Stat(filename); err == nil {
		backupManager := NewDefaultFileBackupManager()
		backupPath, err = backupManager.CreateBackup(filename)
		if err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Attempt atomic write
	if err := w.WriteAtomic(filename, data, perm); err != nil {
		// Restore backup if write failed and backup exists
		if backupPath != "" {
			backupManager := NewDefaultFileBackupManager()
			if restoreErr := backupManager.RestoreBackup(backupPath, filename); restoreErr != nil {
				return fmt.Errorf("write failed and backup restore failed: write error: %w, restore error: %v", err, restoreErr)
			}
		}
		return fmt.Errorf("atomic write failed: %w", err)
	}

	// Clean up backup on successful write
	if backupPath != "" {
		if err := os.Remove(backupPath); err != nil {
			// Log warning but don't fail the operation
			fmt.Printf("Warning: failed to remove backup file %s: %v\n", backupPath, err)
		}
	}

	return nil
}

// CreateTempFile creates a temporary file in the specified directory
func (w *DefaultAtomicFileWriter) CreateTempFile(dir, pattern string) (*os.File, error) {
	if dir == "" {
		dir = os.TempDir()
	}

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	tempFile, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}

	return tempFile, nil
}

// CommitTempFile commits a temporary file to its final location
func (w *DefaultAtomicFileWriter) CommitTempFile(tempFile *os.File, finalPath string) error {
	if tempFile == nil {
		return fmt.Errorf("temporary file is nil")
	}

	tempPath := tempFile.Name()

	// Close the temporary file
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Ensure parent directory of final path exists
	finalDir := filepath.Dir(finalPath)
	if err := os.MkdirAll(finalDir, 0755); err != nil {
		return fmt.Errorf("failed to create final directory %s: %w", finalDir, err)
	}

	// Atomically move temporary file to final location
	if err := os.Rename(tempPath, finalPath); err != nil {
		// Clean up temporary file on failure
		os.Remove(tempPath)
		return fmt.Errorf("failed to move temporary file to final location: %w", err)
	}

	return nil
}

// WriteFileAtomic is a convenience function for atomic file writing
func WriteFileAtomic(filename string, data []byte, perm os.FileMode) error {
	writer := NewDefaultAtomicFileWriter()
	return writer.WriteAtomic(filename, data, perm)
}

// AtomicFileOperation represents an atomic file operation
type AtomicFileOperation struct {
	tempFile  *os.File
	finalPath string
	writer    *DefaultAtomicFileWriter
	committed bool
}

// NewAtomicFileOperation creates a new atomic file operation
func NewAtomicFileOperation(finalPath string) (*AtomicFileOperation, error) {
	writer := NewDefaultAtomicFileWriter()

	// Create temporary file in the same directory as final path
	dir := filepath.Dir(finalPath)
	tempFile, err := writer.CreateTempFile(dir, ".tmp-"+filepath.Base(finalPath)+"-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}

	return &AtomicFileOperation{
		tempFile:  tempFile,
		finalPath: finalPath,
		writer:    writer,
		committed: false,
	}, nil
}

// Write writes data to the temporary file
func (op *AtomicFileOperation) Write(data []byte) (int, error) {
	if op.committed {
		return 0, fmt.Errorf("operation already committed")
	}

	return op.tempFile.Write(data)
}

// WriteString writes a string to the temporary file
func (op *AtomicFileOperation) WriteString(s string) (int, error) {
	return op.Write([]byte(s))
}

// SetPermissions sets the permissions for the final file
func (op *AtomicFileOperation) SetPermissions(perm os.FileMode) error {
	if op.committed {
		return fmt.Errorf("operation already committed")
	}

	return op.tempFile.Chmod(perm)
}

// Commit commits the operation by moving the temporary file to the final location
func (op *AtomicFileOperation) Commit() error {
	if op.committed {
		return fmt.Errorf("operation already committed")
	}

	// Sync before committing
	if err := op.tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}

	if err := op.writer.CommitTempFile(op.tempFile, op.finalPath); err != nil {
		return fmt.Errorf("failed to commit operation: %w", err)
	}

	op.committed = true
	return nil
}

// Abort aborts the operation by removing the temporary file
func (op *AtomicFileOperation) Abort() error {
	if op.committed {
		return nil // Already committed, nothing to abort
	}

	tempPath := op.tempFile.Name()
	op.tempFile.Close()

	if err := os.Remove(tempPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove temporary file: %w", err)
	}

	return nil
}

// GetTempPath returns the path of the temporary file
func (op *AtomicFileOperation) GetTempPath() string {
	return op.tempFile.Name()
}

// GetFinalPath returns the final path where the file will be committed
func (op *AtomicFileOperation) GetFinalPath() string {
	return op.finalPath
}

// IsCommitted returns true if the operation has been committed
func (op *AtomicFileOperation) IsCommitted() bool {
	return op.committed
}

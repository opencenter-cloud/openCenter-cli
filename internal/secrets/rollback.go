/*
Copyright 2025.

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

package secrets

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
)

// RollbackManager handles atomic operations with rollback support.
// It backs up files before modification and can restore them on failure,
// ensuring that operations either complete fully or leave no partial changes.
//
// Validates: Requirement 3.9 - Rollback on encryption failure
type RollbackManager struct {
	backups map[string][]byte
	logger  *slog.Logger
}

// NewRollbackManager creates a new rollback manager.
//
// Parameters:
//   - logger: Logger for operation tracking (uses default if nil)
//
// Returns:
//   - *RollbackManager: A new rollback manager instance
func NewRollbackManager(logger *slog.Logger) *RollbackManager {
	if logger == nil {
		logger = slog.Default()
	}

	return &RollbackManager{
		backups: make(map[string][]byte),
		logger:  logger,
	}
}

// Backup creates a backup of the file at the specified path.
// If the file doesn't exist, it records nil to indicate the file should be
// deleted during rollback.
//
// Parameters:
//   - path: Path to the file to backup
//
// Returns:
//   - error: Error if backup fails (file read error)
func (r *RollbackManager) Backup(path string) error {
	r.logger.Debug("Backing up file", "path", path)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, record nil to indicate it should be deleted on rollback
			r.backups[path] = nil
			r.logger.Debug("File does not exist, will be deleted on rollback", "path", path)
			return nil
		}
		return fmt.Errorf("failed to read file for backup: %w", err)
	}

	r.backups[path] = data
	r.logger.Debug("File backed up", "path", path, "size", len(data))
	return nil
}

// Rollback restores all backed up files to their original state.
// Files that didn't exist before are deleted, and files that existed are
// restored to their original content.
//
// Returns:
//   - error: Combined error if any restore operations fail
func (r *RollbackManager) Rollback() error {
	r.logger.Info("Rolling back changes", "file_count", len(r.backups))

	var errs []error
	for path, data := range r.backups {
		if data == nil {
			// File didn't exist before, remove it
			r.logger.Debug("Removing file that didn't exist before", "path", path)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				errs = append(errs, fmt.Errorf("failed to remove %s: %w", path, err))
			}
		} else {
			// Restore original content
			r.logger.Debug("Restoring file to original content", "path", path, "size", len(data))
			if err := os.WriteFile(path, data, 0o600); err != nil {
				errs = append(errs, fmt.Errorf("failed to restore %s: %w", path, err))
			}
		}
	}

	if len(errs) > 0 {
		r.logger.Error("Rollback completed with errors", "error_count", len(errs))
		return errors.Join(errs...)
	}

	r.logger.Info("Rollback completed successfully")
	return nil
}

// Clear removes all backup data without performing a rollback.
// This should be called after a successful operation to free memory.
func (r *RollbackManager) Clear() {
	r.logger.Debug("Clearing backup data", "file_count", len(r.backups))
	r.backups = make(map[string][]byte)
}

// HasBackups returns true if there are any backed up files.
func (r *RollbackManager) HasBackups() bool {
	return len(r.backups) > 0
}

// BackupCount returns the number of backed up files.
func (r *RollbackManager) BackupCount() int {
	return len(r.backups)
}

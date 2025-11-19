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

package security

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DefaultBackupManager implements BackupManager interface
type DefaultBackupManager struct {
	backupDir   string
	backups     map[string]BackupInfo
	mu          sync.Mutex
	auditLogger AuditLogger
}

// NewDefaultBackupManager creates a new backup manager
func NewDefaultBackupManager(backupDir string, auditLogger AuditLogger) (*DefaultBackupManager, error) {
	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	return &DefaultBackupManager{
		backupDir:   backupDir,
		backups:     make(map[string]BackupInfo),
		auditLogger: auditLogger,
	}, nil
}

// CreateBackup creates a backup of a resource
func (m *DefaultBackupManager) CreateBackup(ctx context.Context, resource string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if resource exists
	info, err := os.Stat(resource)
	if err != nil {
		return "", fmt.Errorf("failed to stat resource: %w", err)
	}

	// Generate backup ID
	backupID := uuid.New().String()

	// Create backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("%s-%s-%s", filepath.Base(resource), timestamp, backupID[:8])
	backupPath := filepath.Join(m.backupDir, backupName)

	// Copy resource to backup location
	if info.IsDir() {
		if err := copyDir(resource, backupPath); err != nil {
			return "", fmt.Errorf("failed to backup directory: %w", err)
		}
	} else {
		if err := copyFile(resource, backupPath); err != nil {
			return "", fmt.Errorf("failed to backup file: %w", err)
		}
	}

	// Store backup info
	backupInfo := BackupInfo{
		ID:        backupID,
		Resource:  resource,
		CreatedAt: time.Now(),
		Size:      info.Size(),
		Path:      backupPath,
		Metadata: map[string]interface{}{
			"original_path": resource,
			"is_directory":  info.IsDir(),
		},
	}

	m.backups[backupID] = backupInfo

	// Log backup creation
	if m.auditLogger != nil {
		m.auditLogger.LogSecurityEvent(ctx, SecurityEvent{
			Timestamp: time.Now(),
			EventType: "backup_created",
			Operation: "create_backup",
			Resource:  resource,
			Success:   true,
			Severity:  "low",
			Details: map[string]interface{}{
				"backup_id":   backupID,
				"backup_path": backupPath,
				"size":        info.Size(),
			},
		})
	}

	return backupID, nil
}

// RestoreBackup restores a backup
func (m *DefaultBackupManager) RestoreBackup(ctx context.Context, backupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get backup info
	backupInfo, exists := m.backups[backupID]
	if !exists {
		return fmt.Errorf("backup %s not found", backupID)
	}

	// Check if backup file exists
	if _, err := os.Stat(backupInfo.Path); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	// Determine if it's a directory or file
	isDir := false
	if metadata, ok := backupInfo.Metadata["is_directory"].(bool); ok {
		isDir = metadata
	}

	// Restore the backup
	if isDir {
		// Remove existing directory if it exists
		if err := os.RemoveAll(backupInfo.Resource); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove existing directory: %w", err)
		}

		// Copy backup directory to original location
		if err := copyDir(backupInfo.Path, backupInfo.Resource); err != nil {
			return fmt.Errorf("failed to restore directory: %w", err)
		}
	} else {
		// Copy backup file to original location
		if err := copyFile(backupInfo.Path, backupInfo.Resource); err != nil {
			return fmt.Errorf("failed to restore file: %w", err)
		}
	}

	// Log backup restoration
	if m.auditLogger != nil {
		m.auditLogger.LogSecurityEvent(ctx, SecurityEvent{
			Timestamp: time.Now(),
			EventType: "backup_restored",
			Operation: "restore_backup",
			Resource:  backupInfo.Resource,
			Success:   true,
			Severity:  "medium",
			Details: map[string]interface{}{
				"backup_id":   backupID,
				"backup_path": backupInfo.Path,
			},
		})
	}

	return nil
}

// DeleteBackup deletes a backup
func (m *DefaultBackupManager) DeleteBackup(ctx context.Context, backupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get backup info
	backupInfo, exists := m.backups[backupID]
	if !exists {
		return fmt.Errorf("backup %s not found", backupID)
	}

	// Remove backup file/directory
	if err := os.RemoveAll(backupInfo.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	// Remove from tracking
	delete(m.backups, backupID)

	// Log backup deletion
	if m.auditLogger != nil {
		m.auditLogger.LogSecurityEvent(ctx, SecurityEvent{
			Timestamp: time.Now(),
			EventType: "backup_deleted",
			Operation: "delete_backup",
			Resource:  backupInfo.Resource,
			Success:   true,
			Severity:  "low",
			Details: map[string]interface{}{
				"backup_id":   backupID,
				"backup_path": backupInfo.Path,
			},
		})
	}

	return nil
}

// ListBackups lists all backups for a resource
func (m *DefaultBackupManager) ListBackups(ctx context.Context, resource string) ([]BackupInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var backups []BackupInfo

	for _, backup := range m.backups {
		if backup.Resource == resource {
			backups = append(backups, backup)
		}
	}

	return backups, nil
}

// CleanupOldBackups removes backups older than maxAge
func (m *DefaultBackupManager) CleanupOldBackups(ctx context.Context, maxAge time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoffTime := time.Now().Add(-maxAge)
	var errors []error
	deletedCount := 0

	for backupID, backup := range m.backups {
		if backup.CreatedAt.Before(cutoffTime) {
			// Remove backup file/directory
			if err := os.RemoveAll(backup.Path); err != nil && !os.IsNotExist(err) {
				errors = append(errors, fmt.Errorf("failed to delete backup %s: %w", backupID, err))
				continue
			}

			// Remove from tracking
			delete(m.backups, backupID)
			deletedCount++
		}
	}

	// Log cleanup
	if m.auditLogger != nil {
		m.auditLogger.LogSecurityEvent(ctx, SecurityEvent{
			Timestamp: time.Now(),
			EventType: "backup_cleanup",
			Operation: "cleanup_old_backups",
			Success:   len(errors) == 0,
			Severity:  "low",
			Details: map[string]interface{}{
				"deleted_count": deletedCount,
				"max_age":       maxAge.String(),
				"errors_count":  len(errors),
			},
		})
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup completed with errors: %v", errors)
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Read source file
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	// Get source file permissions
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	// Ensure destination directory exists
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0700); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Write to destination file with same permissions
	if err := os.WriteFile(dst, data, info.Mode()); err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	return nil
}

// copyDir recursively copies a directory from src to dst
func copyDir(src, dst string) error {
	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source directory: %w", err)
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Read source directory entries
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	// Copy each entry
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetDefaultBackupDir returns the default backup directory
func GetDefaultBackupDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/opencenter-backups"
	}

	return filepath.Join(homeDir, ".config", "openCenter", "backups")
}

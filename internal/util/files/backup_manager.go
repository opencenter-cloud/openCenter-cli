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
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DefaultFileBackupManager implements FileBackupManager interface
type DefaultFileBackupManager struct {
	fileOperator FileOperator
	validator    FileValidator
}

// NewDefaultFileBackupManager creates a new default file backup manager
func NewDefaultFileBackupManager() *DefaultFileBackupManager {
	return &DefaultFileBackupManager{
		fileOperator: NewDefaultFileOperator(),
		validator:    NewDefaultFileValidator(),
	}
}

// CreateBackup creates a backup of a file with timestamp
func (b *DefaultFileBackupManager) CreateBackup(filename string) (string, error) {
	if err := b.validator.ValidateFileExists(filename); err != nil {
		return "", fmt.Errorf("source file validation failed: %w", err)
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.backup.%s", filename, timestamp)

	// Copy file to backup location
	if err := b.fileOperator.CopyFile(filename, backupPath); err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	return backupPath, nil
}

// RestoreBackup restores a file from backup
func (b *DefaultFileBackupManager) RestoreBackup(backupPath, originalPath string) error {
	if err := b.validator.ValidateFileExists(backupPath); err != nil {
		return fmt.Errorf("backup file validation failed: %w", err)
	}

	// Copy backup to original location
	if err := b.fileOperator.CopyFile(backupPath, originalPath); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	return nil
}

// CleanupBackups removes old backup files based on age
func (b *DefaultFileBackupManager) CleanupBackups(pattern string, maxAgeSeconds int64) error {
	backups, err := b.ListBackups(pattern)
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	currentTime := time.Now().Unix()
	var cleanupErrors []error

	for _, backup := range backups {
		info, err := os.Stat(backup)
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to get info for backup %s: %w", backup, err))
			continue
		}

		fileAge := currentTime - info.ModTime().Unix()
		if fileAge > maxAgeSeconds {
			if err := b.fileOperator.DeleteFile(backup); err != nil {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to delete old backup %s: %w", backup, err))
			}
		}
	}

	if len(cleanupErrors) > 0 {
		return fmt.Errorf("cleanup completed with %d errors: %v", len(cleanupErrors), cleanupErrors)
	}

	return nil
}

// ListBackups lists all backup files matching a pattern
func (b *DefaultFileBackupManager) ListBackups(pattern string) ([]string, error) {
	// If pattern doesn't contain backup suffix, add it
	if !strings.Contains(pattern, ".backup.") {
		pattern = pattern + ".backup.*"
	}

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid backup pattern %s: %w", pattern, err)
	}

	// Filter to ensure they are actually backup files
	var backups []string
	for _, match := range matches {
		if strings.Contains(match, ".backup.") {
			backups = append(backups, match)
		}
	}

	// Sort by modification time (newest first)
	sort.Slice(backups, func(i, j int) bool {
		infoI, errI := os.Stat(backups[i])
		infoJ, errJ := os.Stat(backups[j])

		if errI != nil || errJ != nil {
			return false
		}

		return infoI.ModTime().After(infoJ.ModTime())
	})

	return backups, nil
}

// CreateBackupWithSuffix creates a backup with a custom suffix
func (b *DefaultFileBackupManager) CreateBackupWithSuffix(filename, suffix string) (string, error) {
	if err := b.validator.ValidateFileExists(filename); err != nil {
		return "", fmt.Errorf("source file validation failed: %w", err)
	}

	backupPath := fmt.Sprintf("%s.%s", filename, suffix)

	// Copy file to backup location
	if err := b.fileOperator.CopyFile(filename, backupPath); err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	return backupPath, nil
}

// CreateIncrementalBackup creates an incremental backup (numbered)
func (b *DefaultFileBackupManager) CreateIncrementalBackup(filename string) (string, error) {
	if err := b.validator.ValidateFileExists(filename); err != nil {
		return "", fmt.Errorf("source file validation failed: %w", err)
	}

	// Find the next available backup number
	backupNum := 1
	for {
		backupPath := fmt.Sprintf("%s.backup.%03d", filename, backupNum)
		if !b.fileOperator.FileExists(backupPath) {
			// Copy file to backup location
			if err := b.fileOperator.CopyFile(filename, backupPath); err != nil {
				return "", fmt.Errorf("failed to create incremental backup: %w", err)
			}
			return backupPath, nil
		}
		backupNum++

		// Prevent infinite loop
		if backupNum > 999 {
			return "", fmt.Errorf("too many backup files exist for %s", filename)
		}
	}
}

// GetLatestBackup returns the most recent backup for a file
func (b *DefaultFileBackupManager) GetLatestBackup(filename string) (string, error) {
	pattern := filename + ".backup.*"
	backups, err := b.ListBackups(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to list backups: %w", err)
	}

	if len(backups) == 0 {
		return "", fmt.Errorf("no backups found for file: %s", filename)
	}

	// ListBackups returns sorted by newest first
	return backups[0], nil
}

// RestoreFromLatestBackup restores a file from its most recent backup
func (b *DefaultFileBackupManager) RestoreFromLatestBackup(filename string) error {
	latestBackup, err := b.GetLatestBackup(filename)
	if err != nil {
		return fmt.Errorf("failed to find latest backup: %w", err)
	}

	return b.RestoreBackup(latestBackup, filename)
}

// BackupDirectory creates backups of all files in a directory
func (b *DefaultFileBackupManager) BackupDirectory(dirname string) ([]string, error) {
	if err := b.validator.ValidateFileIsDirectory(dirname); err != nil {
		return nil, fmt.Errorf("directory validation failed: %w", err)
	}

	entries, err := os.ReadDir(dirname)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var backupPaths []string
	var backupErrors []error

	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip subdirectories
		}

		filePath := filepath.Join(dirname, entry.Name())
		backupPath, err := b.CreateBackup(filePath)
		if err != nil {
			backupErrors = append(backupErrors, fmt.Errorf("failed to backup %s: %w", filePath, err))
			continue
		}

		backupPaths = append(backupPaths, backupPath)
	}

	if len(backupErrors) > 0 {
		return backupPaths, fmt.Errorf("directory backup completed with %d errors: %v", len(backupErrors), backupErrors)
	}

	return backupPaths, nil
}

// CleanupOldBackups removes backups older than specified number of days
func (b *DefaultFileBackupManager) CleanupOldBackups(pattern string, maxAgeDays int) error {
	maxAgeSeconds := int64(maxAgeDays * 24 * 60 * 60)
	return b.CleanupBackups(pattern, maxAgeSeconds)
}

// GetBackupInfo returns information about a backup file
func (b *DefaultFileBackupManager) GetBackupInfo(backupPath string) (*BackupInfo, error) {
	if err := b.validator.ValidateFileExists(backupPath); err != nil {
		return nil, fmt.Errorf("backup file validation failed: %w", err)
	}

	info, err := os.Stat(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup file info: %w", err)
	}

	// Extract original filename from backup path
	originalFile := strings.Split(backupPath, ".backup.")[0]

	backupInfo := &BackupInfo{
		BackupPath:   backupPath,
		OriginalFile: originalFile,
		Size:         info.Size(),
		CreatedAt:    info.ModTime(),
		IsValid:      true,
	}

	return backupInfo, nil
}

// BackupInfo represents information about a backup file
type BackupInfo struct {
	BackupPath   string    `json:"backup_path"`
	OriginalFile string    `json:"original_file"`
	Size         int64     `json:"size"`
	CreatedAt    time.Time `json:"created_at"`
	IsValid      bool      `json:"is_valid"`
}

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

package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultDirectoryManager implements DirectoryManager interface
type DefaultDirectoryManager struct {
	validator PathValidator
}

// NewDefaultDirectoryManager creates a new default directory manager
func NewDefaultDirectoryManager() *DefaultDirectoryManager {
	return &DefaultDirectoryManager{
		validator: NewDefaultPathValidator(),
	}
}

// CreateDirectory creates a single directory with the specified permissions
func (d *DefaultDirectoryManager) CreateDirectory(path string, mode uint32) error {
	if err := d.validator.ValidatePath(path); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	if err := os.MkdirAll(path, os.FileMode(mode)); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	// Verify the directory was created and is accessible
	if err := d.validator.ValidatePathIsDirectory(path); err != nil {
		return fmt.Errorf("failed to verify directory creation: %w", err)
	}

	return nil
}

// CreateDirectoryStructure creates multiple directories with the specified permissions
func (d *DefaultDirectoryManager) CreateDirectoryStructure(paths []string, mode uint32) error {
	for _, path := range paths {
		if err := d.CreateDirectory(path, mode); err != nil {
			return fmt.Errorf("failed to create directory structure at %s: %w", path, err)
		}
	}
	return nil
}

// EnsureDirectoryExists creates a directory if it doesn't exist
func (d *DefaultDirectoryManager) EnsureDirectoryExists(path string, mode uint32) error {
	if err := d.validator.ValidatePath(path); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check if directory already exists
	if stat, err := os.Stat(path); err == nil {
		if stat.IsDir() {
			return nil // Directory already exists
		}
		return fmt.Errorf("path exists but is not a directory: %s", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check if directory exists: %w", err)
	}

	// Directory doesn't exist, create it
	return d.CreateDirectory(path, mode)
}

// RemoveDirectoryIfEmpty removes a directory if it's empty or only contains empty directories
func (d *DefaultDirectoryManager) RemoveDirectoryIfEmpty(path string) error {
	// Check if directory exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // Already removed
	}

	// Check if directory is empty or only contains empty directories
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	// Remove any empty subdirectories first
	for _, entry := range entries {
		if entry.IsDir() {
			subPath := filepath.Join(path, entry.Name())
			if err := d.RemoveDirectoryIfEmpty(subPath); err != nil {
				return fmt.Errorf("failed to remove subdirectory: %w", err)
			}
		}
	}

	// Check again if directory is now empty
	entries, err = os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to re-read directory: %w", err)
	}

	if len(entries) == 0 {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to remove empty directory: %w", err)
		}
	}

	return nil // Directory is not empty, leave it
}

// CopyDirectory recursively copies a directory from src to dst
func (d *DefaultDirectoryManager) CopyDirectory(src, dst string) error {
	if err := d.validator.ValidatePathIsDirectory(src); err != nil {
		return fmt.Errorf("source validation failed: %w", err)
	}

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walk error: %w", err)
		}

		// Calculate destination path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("failed to calculate relative path: %w", err)
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return d.CreateDirectory(dstPath, uint32(info.Mode()))
		}

		// Copy file
		return d.copyFile(path, dstPath, info.Mode())
	})
}

// MoveDirectory moves a directory from src to dst
func (d *DefaultDirectoryManager) MoveDirectory(src, dst string) error {
	if err := d.validator.ValidatePathIsDirectory(src); err != nil {
		return fmt.Errorf("source validation failed: %w", err)
	}

	// Try to use os.Rename first (atomic operation if on same filesystem)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// If rename fails, fall back to copy and remove
	if err := d.CopyDirectory(src, dst); err != nil {
		return fmt.Errorf("failed to copy directory: %w", err)
	}

	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("failed to remove source directory after copy: %w", err)
	}

	return nil
}

// copyFile copies a single file from src to dst with the given mode
func (d *DefaultDirectoryManager) copyFile(src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Ensure destination directory exists
	dstDir := filepath.Dir(dst)
	if err := d.EnsureDirectoryExists(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy file contents
	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		n, err := srcFile.Read(buf)
		if n > 0 {
			if _, writeErr := dstFile.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("failed to write to destination file: %w", writeErr)
			}
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return fmt.Errorf("failed to read from source file: %w", err)
		}
	}

	return nil
}

// GetDirectorySize calculates the total size of a directory
func (d *DefaultDirectoryManager) GetDirectorySize(path string) (int64, error) {
	if err := d.validator.ValidatePathIsDirectory(path); err != nil {
		return 0, fmt.Errorf("directory validation failed: %w", err)
	}

	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to calculate directory size: %w", err)
	}

	return size, nil
}

// ListDirectoryContents lists the contents of a directory
func (d *DefaultDirectoryManager) ListDirectoryContents(path string) ([]os.DirEntry, error) {
	if err := d.validator.ValidatePathIsDirectory(path); err != nil {
		return nil, fmt.Errorf("directory validation failed: %w", err)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory contents: %w", err)
	}

	return entries, nil
}

// IsDirectoryEmpty checks if a directory is empty
func (d *DefaultDirectoryManager) IsDirectoryEmpty(path string) (bool, error) {
	entries, err := d.ListDirectoryContents(path)
	if err != nil {
		return false, err
	}

	return len(entries) == 0, nil
}

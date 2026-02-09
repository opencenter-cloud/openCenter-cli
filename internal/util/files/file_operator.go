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
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
	"github.com/rackerlabs/opencenter-cli/internal/util/fs"
)

// DefaultFileOperator implements FileOperator interface
type DefaultFileOperator struct {
	validator  FileValidator
	fileSystem fs.FileSystem
}

// NewDefaultFileOperator creates a new default file operator
func NewDefaultFileOperator() *DefaultFileOperator {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	return &DefaultFileOperator{
		validator:  NewDefaultFileValidator(),
		fileSystem: fileSystem,
	}
}

// ReadFile reads the contents of a file
func (f *DefaultFileOperator) ReadFile(filename string) ([]byte, error) {
	if err := f.validator.ValidateFileExists(filename); err != nil {
		return nil, fmt.Errorf("file validation failed: %w", err)
	}

	if err := f.validator.ValidateFileReadable(filename); err != nil {
		return nil, fmt.Errorf("file not readable: %w", err)
	}

	data, err := f.fileSystem.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	return data, nil
}

// WriteFile writes data to a file
func (f *DefaultFileOperator) WriteFile(filename string, data []byte, perm os.FileMode) error {
	// Ensure parent directory exists
	if err := f.ensureParentDir(filename); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	if err := f.fileSystem.WriteFile(filename, data, perm); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filename, err)
	}

	return nil
}

// WriteFileAtomic writes data to a file atomically
func (f *DefaultFileOperator) WriteFileAtomic(filename string, data []byte, perm os.FileMode) error {
	atomicWriter := NewDefaultAtomicFileWriter()
	return atomicWriter.WriteAtomic(filename, data, perm)
}

// AppendToFile appends data to a file
func (f *DefaultFileOperator) AppendToFile(filename string, data []byte) error {
	// Check if file exists and is writable
	if f.FileExists(filename) {
		if err := f.validator.ValidateFileWritable(filename); err != nil {
			return fmt.Errorf("file not writable: %w", err)
		}
	} else {
		// Ensure parent directory exists
		if err := f.ensureParentDir(filename); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}
	}

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for append %s: %w", filename, err)
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to append to file %s: %w", filename, err)
	}

	return nil
}

// CopyFile copies a file from src to dst
func (f *DefaultFileOperator) CopyFile(src, dst string) error {
	if err := f.validator.ValidateFileExists(src); err != nil {
		return fmt.Errorf("source file validation failed: %w", err)
	}

	if err := f.validator.ValidateFileReadable(src); err != nil {
		return fmt.Errorf("source file not readable: %w", err)
	}

	// Get source file info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to get source file info: %w", err)
	}

	// Ensure destination parent directory exists
	if err := f.ensureParentDir(dst); err != nil {
		return fmt.Errorf("failed to create destination parent directory: %w", err)
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy file contents
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Sync to ensure data is written
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	return nil
}

// MoveFile moves a file from src to dst
func (f *DefaultFileOperator) MoveFile(src, dst string) error {
	if err := f.validator.ValidateFileExists(src); err != nil {
		return fmt.Errorf("source file validation failed: %w", err)
	}

	// Ensure destination parent directory exists
	if err := f.ensureParentDir(dst); err != nil {
		return fmt.Errorf("failed to create destination parent directory: %w", err)
	}

	// Try to rename first (atomic operation if on same filesystem)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// If rename fails, fall back to copy and delete
	if err := f.CopyFile(src, dst); err != nil {
		return fmt.Errorf("failed to copy file during move: %w", err)
	}

	if err := f.DeleteFile(src); err != nil {
		return fmt.Errorf("failed to delete source file after copy: %w", err)
	}

	return nil
}

// DeleteFile deletes a file
func (f *DefaultFileOperator) DeleteFile(filename string) error {
	if !f.FileExists(filename) {
		return nil // File doesn't exist, nothing to delete
	}

	if err := os.Remove(filename); err != nil {
		return fmt.Errorf("failed to delete file %s: %w", filename, err)
	}

	return nil
}

// FileExists checks if a file exists
func (f *DefaultFileOperator) FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

// GetFileInfo returns file information
func (f *DefaultFileOperator) GetFileInfo(filename string) (os.FileInfo, error) {
	info, err := os.Stat(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info for %s: %w", filename, err)
	}

	return info, nil
}

// ensureParentDir ensures the parent directory of a file exists
func (f *DefaultFileOperator) ensureParentDir(filename string) error {
	parentDir := filepath.Dir(filename)
	if parentDir == "." || parentDir == "/" {
		return nil
	}

	if err := f.fileSystem.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory %s: %w", parentDir, err)
	}

	return nil
}

// GetFileSize returns the size of a file
func (f *DefaultFileOperator) GetFileSize(filename string) (int64, error) {
	info, err := f.GetFileInfo(filename)
	if err != nil {
		return 0, err
	}

	return info.Size(), nil
}

// IsDirectory checks if a path is a directory
func (f *DefaultFileOperator) IsDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

// IsRegularFile checks if a path is a regular file
func (f *DefaultFileOperator) IsRegularFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.Mode().IsRegular()
}

// GetFileMetadata returns comprehensive file metadata
func (f *DefaultFileOperator) GetFileMetadata(filename string) (*FileMetadata, error) {
	info, err := f.GetFileInfo(filename)
	if err != nil {
		return nil, err
	}

	metadata := &FileMetadata{
		Path:        filename,
		Size:        info.Size(),
		Mode:        info.Mode(),
		ModTime:     info.ModTime().Unix(),
		IsDir:       info.IsDir(),
		Permissions: info.Mode().String(),
	}

	return metadata, nil
}

// CreateEmptyFile creates an empty file with specified permissions
func (f *DefaultFileOperator) CreateEmptyFile(filename string, perm os.FileMode) error {
	return f.WriteFile(filename, []byte{}, perm)
}

// TouchFile creates a file if it doesn't exist or updates its modification time
func (f *DefaultFileOperator) TouchFile(filename string) error {
	if f.FileExists(filename) {
		// Update modification time
		currentTime := time.Now()
		if err := os.Chtimes(filename, currentTime, currentTime); err != nil {
			return fmt.Errorf("failed to update file modification time: %w", err)
		}
		return nil
	}

	// Create empty file
	return f.CreateEmptyFile(filename, 0644)
}

// TruncateFile truncates a file to a specified size
func (f *DefaultFileOperator) TruncateFile(filename string, size int64) error {
	if err := f.validator.ValidateFileExists(filename); err != nil {
		return fmt.Errorf("file validation failed: %w", err)
	}

	if err := f.validator.ValidateFileWritable(filename); err != nil {
		return fmt.Errorf("file not writable: %w", err)
	}

	if err := os.Truncate(filename, size); err != nil {
		return fmt.Errorf("failed to truncate file %s: %w", filename, err)
	}

	return nil
}

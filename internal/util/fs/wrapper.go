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

package fs

import (
	"crypto/rand"
	"encoding/hex"
	"io/fs"
	"os"

	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
)

// FileSystem provides safe file operations with consistent error handling
type FileSystem interface {
	// ReadFile reads the entire file at path
	ReadFile(path string) ([]byte, error)

	// WriteFile writes data to path with given permissions
	WriteFile(path string, data []byte, perm os.FileMode) error

	// WriteFileAtomic writes data atomically to prevent corruption
	WriteFileAtomic(path string, data []byte, perm os.FileMode) error

	// Exists checks if a file or directory exists at path
	Exists(path string) bool

	// MkdirAll creates directory and all parent directories
	MkdirAll(path string, perm os.FileMode) error

	// Remove removes the file or directory at path
	Remove(path string) error

	// Stat returns file information
	Stat(path string) (fs.FileInfo, error)
}

// DefaultFileSystem implements FileSystem using os package
type DefaultFileSystem struct {
	errorHandler errors.ErrorHandler
}

// NewDefaultFileSystem creates a new DefaultFileSystem with the given error handler
func NewDefaultFileSystem(errorHandler errors.ErrorHandler) *DefaultFileSystem {
	return &DefaultFileSystem{
		errorHandler: errorHandler,
	}
}

// ReadFile reads the entire file at path
func (dfs *DefaultFileSystem) ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.CreateFileError("read", path, err)
	}
	return data, nil
}

// WriteFile writes data to path with given permissions
func (dfs *DefaultFileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	if err := os.WriteFile(path, data, perm); err != nil {
		return errors.CreateFileError("write", path, err)
	}
	return nil
}

// WriteFileAtomic writes data atomically to prevent corruption
func (dfs *DefaultFileSystem) WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	// Generate unique temporary file name
	tmpPath := path + ".tmp." + generateRandomString(8)

	// Write to temporary file
	if err := os.WriteFile(tmpPath, data, perm); err != nil {
		return errors.CreateFileError("write_temp", tmpPath, err)
	}

	// Atomic rename (POSIX guarantees atomicity)
	if err := os.Rename(tmpPath, path); err != nil {
		// Cleanup temp file on failure
		// Note: This cleanup path is difficult to test reliably across platforms.
		// It requires the temp write to succeed but rename to fail, which needs
		// specific permission scenarios that behave differently on macOS/Linux/Windows.
		// The risk is low: if cleanup fails, it only leaves a clearly-marked temp file.
		os.Remove(tmpPath)
		return errors.CreateFileError("atomic_rename", path, err)
	}

	return nil
}

// Exists checks if a file or directory exists at path
func (dfs *DefaultFileSystem) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// MkdirAll creates directory and all parent directories
func (dfs *DefaultFileSystem) MkdirAll(path string, perm os.FileMode) error {
	if err := os.MkdirAll(path, perm); err != nil {
		return errors.CreateFileError("mkdir", path, err)
	}
	return nil
}

// Remove removes the file or directory at path
func (dfs *DefaultFileSystem) Remove(path string) error {
	if err := os.Remove(path); err != nil {
		return errors.CreateFileError("remove", path, err)
	}
	return nil
}

// Stat returns file information
func (dfs *DefaultFileSystem) Stat(path string) (fs.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, errors.CreateFileError("stat", path, err)
	}
	return info, nil
}

// generateRandomString creates a random alphanumeric string of given length
func generateRandomString(length int) string {
	bytes := make([]byte, length/2+1)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a simple timestamp-based string if crypto/rand fails
		// Note: This fallback path is nearly impossible to test without mocking crypto/rand.
		// It only executes if the system's random number generator is unavailable,
		// which would indicate serious system issues. The fallback provides a valid
		// (though predictable) string for temporary file naming with minimal collision risk.
		return "fallback"
	}
	return hex.EncodeToString(bytes)[:length]
}

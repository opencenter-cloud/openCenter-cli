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
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DefaultSecureTempFileManager implements SecureTempFileManager interface
type DefaultSecureTempFileManager struct {
	tempFiles map[string]*SecureTempFile
	tempDirs  map[string]time.Time
	mu        sync.Mutex
}

// NewDefaultSecureTempFileManager creates a new secure temp file manager
func NewDefaultSecureTempFileManager() *DefaultSecureTempFileManager {
	return &DefaultSecureTempFileManager{
		tempFiles: make(map[string]*SecureTempFile),
		tempDirs:  make(map[string]time.Time),
	}
}

// CreateSecureTempFile creates a secure temporary file with restricted permissions
func (m *DefaultSecureTempFileManager) CreateSecureTempFile(pattern string) (*SecureTempFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create temporary file with secure permissions (0600 - owner read/write only)
	tempFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to create secure temporary file: %w", err)
	}

	// Set restrictive permissions immediately
	if err := tempFile.Chmod(0600); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return nil, fmt.Errorf("failed to set secure permissions: %w", err)
	}

	secureTempFile := &SecureTempFile{
		File:        tempFile,
		Path:        tempFile.Name(),
		Permissions: 0600,
		CreatedAt:   time.Now(),
	}

	// Track the temp file for cleanup
	m.tempFiles[secureTempFile.Path] = secureTempFile

	return secureTempFile, nil
}

// CreateSecureTempDir creates a secure temporary directory with restricted permissions
func (m *DefaultSecureTempFileManager) CreateSecureTempDir(pattern string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create temporary directory with secure permissions (0700 - owner full access only)
	tempDir, err := os.MkdirTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("failed to create secure temporary directory: %w", err)
	}

	// Set restrictive permissions
	if err := os.Chmod(tempDir, 0700); err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to set secure permissions: %w", err)
	}

	// Track the temp directory for cleanup
	m.tempDirs[tempDir] = time.Now()

	return tempDir, nil
}

// CleanupTempFile securely removes a temporary file
func (m *DefaultSecureTempFileManager) CleanupTempFile(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close file if it's still open
	if tempFile, exists := m.tempFiles[path]; exists {
		if tempFile.File != nil {
			tempFile.File.Close()
		}
		delete(m.tempFiles, path)
	}

	// Securely remove the file
	if err := secureRemoveFile(path); err != nil {
		return fmt.Errorf("failed to cleanup temp file: %w", err)
	}

	return nil
}

// CleanupTempDir securely removes a temporary directory
func (m *DefaultSecureTempFileManager) CleanupTempDir(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from tracking
	delete(m.tempDirs, path)

	// Securely remove the directory and all contents
	if err := secureRemoveDir(path); err != nil {
		return fmt.Errorf("failed to cleanup temp directory: %w", err)
	}

	return nil
}

// CleanupAll removes all tracked temporary files and directories
func (m *DefaultSecureTempFileManager) CleanupAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error

	// Cleanup all temp files
	for path, tempFile := range m.tempFiles {
		if tempFile.File != nil {
			tempFile.File.Close()
		}
		if err := secureRemoveFile(path); err != nil {
			errors = append(errors, fmt.Errorf("failed to cleanup %s: %w", path, err))
		}
	}
	m.tempFiles = make(map[string]*SecureTempFile)

	// Cleanup all temp directories
	for path := range m.tempDirs {
		if err := secureRemoveDir(path); err != nil {
			errors = append(errors, fmt.Errorf("failed to cleanup %s: %w", path, err))
		}
	}
	m.tempDirs = make(map[string]time.Time)

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}

	return nil
}

// secureRemoveFile securely removes a file by overwriting before deletion
func secureRemoveFile(path string) error {
	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already removed
		}
		return err
	}

	// Only overwrite regular files (not directories or special files)
	if info.Mode().IsRegular() {
		// Overwrite file with zeros before deletion (basic secure deletion)
		file, err := os.OpenFile(path, os.O_WRONLY, 0600)
		if err == nil {
			size := info.Size()
			zeros := make([]byte, 4096)
			for written := int64(0); written < size; {
				toWrite := size - written
				if toWrite > 4096 {
					toWrite = 4096
				}
				file.Write(zeros[:toWrite])
				written += toWrite
			}
			file.Sync()
			file.Close()
		}
	}

	// Remove the file
	return os.Remove(path)
}

// secureRemoveDir securely removes a directory and all its contents
func secureRemoveDir(path string) error {
	// Walk the directory tree and securely remove all files
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory itself for now
		if filePath == path {
			return nil
		}

		// Securely remove files
		if !info.IsDir() {
			if err := secureRemoveFile(filePath); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Remove the directory structure
	return os.RemoveAll(path)
}

// CreateSecureTempFileWithContent creates a secure temp file with initial content
func CreateSecureTempFileWithContent(pattern string, content []byte) (*SecureTempFile, error) {
	manager := NewDefaultSecureTempFileManager()

	tempFile, err := manager.CreateSecureTempFile(pattern)
	if err != nil {
		return nil, err
	}

	if _, err := tempFile.Write(content); err != nil {
		tempFile.Close()
		manager.CleanupTempFile(tempFile.Path)
		return nil, fmt.Errorf("failed to write content to temp file: %w", err)
	}

	// Sync to ensure data is written
	if err := tempFile.File.Sync(); err != nil {
		tempFile.Close()
		manager.CleanupTempFile(tempFile.Path)
		return nil, fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Seek back to beginning for reading
	if _, err := tempFile.File.Seek(0, 0); err != nil {
		tempFile.Close()
		manager.CleanupTempFile(tempFile.Path)
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	return tempFile, nil
}

// SecureFileWriter wraps a file with automatic cleanup
type SecureFileWriter struct {
	tempFile *SecureTempFile
	manager  *DefaultSecureTempFileManager
}

// NewSecureFileWriter creates a new secure file writer
func NewSecureFileWriter(pattern string) (*SecureFileWriter, error) {
	manager := NewDefaultSecureTempFileManager()
	tempFile, err := manager.CreateSecureTempFile(pattern)
	if err != nil {
		return nil, err
	}

	return &SecureFileWriter{
		tempFile: tempFile,
		manager:  manager,
	}, nil
}

// Write writes data to the secure file
func (w *SecureFileWriter) Write(data []byte) (int, error) {
	return w.tempFile.Write(data)
}

// Close closes and cleans up the secure file
func (w *SecureFileWriter) Close() error {
	if err := w.tempFile.Close(); err != nil {
		return err
	}
	return w.manager.CleanupTempFile(w.tempFile.Path)
}

// GetPath returns the path of the temporary file
func (w *SecureFileWriter) GetPath() string {
	return w.tempFile.Path
}

// Sync syncs the file to disk
func (w *SecureFileWriter) Sync() error {
	return w.tempFile.File.Sync()
}

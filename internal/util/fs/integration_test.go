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
	"os"
	"path/filepath"
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
)

// TestFileSystemIntegration tests the FileSystem wrapper with ErrorHandler integration
func TestFileSystemIntegration(t *testing.T) {
	// Create FileSystem with ErrorHandler
	errorHandler := errors.NewDefaultErrorHandler()
	fs := NewDefaultFileSystem(errorHandler)

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "integration-test.txt")
	testData := []byte("integration test content")

	// Test complete workflow: write, read, verify, remove
	t.Run("complete_workflow", func(t *testing.T) {
		// Write file atomically
		if err := fs.WriteFileAtomic(testFile, testData, 0644); err != nil {
			t.Fatalf("WriteFileAtomic failed: %v", err)
		}

		// Verify file exists
		if !fs.Exists(testFile) {
			t.Fatal("file should exist after atomic write")
		}

		// Read file
		data, err := fs.ReadFile(testFile)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		if string(data) != string(testData) {
			t.Errorf("data mismatch: got %q, want %q", data, testData)
		}

		// Get file info
		info, err := fs.Stat(testFile)
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}

		if info.Size() != int64(len(testData)) {
			t.Errorf("size mismatch: got %d, want %d", info.Size(), len(testData))
		}

		// Remove file
		if err := fs.Remove(testFile); err != nil {
			t.Fatalf("Remove failed: %v", err)
		}

		// Verify file no longer exists
		if fs.Exists(testFile) {
			t.Fatal("file should not exist after removal")
		}
	})

	t.Run("error_handling", func(t *testing.T) {
		nonExistentFile := filepath.Join(tmpDir, "nonexistent.txt")

		// Test reading non-existent file returns structured error
		_, err := fs.ReadFile(nonExistentFile)
		if err == nil {
			t.Fatal("ReadFile should fail for non-existent file")
		}

		structuredErr, ok := err.(*errors.StructuredError)
		if !ok {
			t.Fatalf("error should be StructuredError, got %T", err)
		}

		if structuredErr.Type != errors.FileError {
			t.Errorf("error type should be FileError, got %s", structuredErr.Type)
		}

		if structuredErr.Operation != "read" {
			t.Errorf("operation should be 'read', got %s", structuredErr.Operation)
		}

		// Verify error has context
		if structuredErr.Context == nil {
			t.Fatal("error should have context")
		}

		if path, ok := structuredErr.Context["path"].(string); !ok || path != nonExistentFile {
			t.Errorf("error context should contain path %s, got %v", nonExistentFile, structuredErr.Context["path"])
		}

		// Verify error has suggestions
		if len(structuredErr.Suggestions) == 0 {
			t.Error("error should have suggestions")
		}
	})

	t.Run("atomic_write_with_existing_file", func(t *testing.T) {
		existingFile := filepath.Join(tmpDir, "existing.txt")
		originalData := []byte("original content")
		newData := []byte("new content")

		// Create original file
		if err := os.WriteFile(existingFile, originalData, 0644); err != nil {
			t.Fatalf("failed to create original file: %v", err)
		}

		// Overwrite with atomic write
		if err := fs.WriteFileAtomic(existingFile, newData, 0644); err != nil {
			t.Fatalf("WriteFileAtomic failed: %v", err)
		}

		// Verify new content
		data, err := fs.ReadFile(existingFile)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		if string(data) != string(newData) {
			t.Errorf("data should be updated: got %q, want %q", data, newData)
		}

		// Verify no temp files remain
		tmpFiles, err := filepath.Glob(existingFile + ".tmp.*")
		if err != nil {
			t.Fatalf("failed to check for temp files: %v", err)
		}

		if len(tmpFiles) > 0 {
			t.Errorf("temp files should not remain: %v", tmpFiles)
		}
	})

	t.Run("nested_directory_creation", func(t *testing.T) {
		nestedDir := filepath.Join(tmpDir, "a", "b", "c", "d")
		nestedFile := filepath.Join(nestedDir, "nested.txt")
		nestedData := []byte("nested content")

		// Create nested directories
		if err := fs.MkdirAll(nestedDir, 0755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}

		// Write file in nested directory
		if err := fs.WriteFileAtomic(nestedFile, nestedData, 0644); err != nil {
			t.Fatalf("WriteFileAtomic failed: %v", err)
		}

		// Verify file exists and has correct content
		data, err := fs.ReadFile(nestedFile)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		if string(data) != string(nestedData) {
			t.Errorf("data mismatch: got %q, want %q", data, nestedData)
		}
	})
}

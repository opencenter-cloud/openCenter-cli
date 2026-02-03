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

func TestDefaultFileSystem_ReadFile(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandler())
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testData := []byte("test content")

	// Write test file
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test reading
	data, err := fs.ReadFile(testFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if string(data) != string(testData) {
		t.Errorf("ReadFile returned wrong data: got %q, want %q", data, testData)
	}
}

func TestDefaultFileSystem_WriteFile(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandler())
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testData := []byte("test content")

	// Test writing
	if err := fs.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Verify file was written
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	if string(data) != string(testData) {
		t.Errorf("WriteFile wrote wrong data: got %q, want %q", data, testData)
	}
}

func TestDefaultFileSystem_WriteFileAtomic(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandler())
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testData := []byte("test content")

	// Test atomic writing
	if err := fs.WriteFileAtomic(testFile, testData, 0644); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}

	// Verify file was written
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	if string(data) != string(testData) {
		t.Errorf("WriteFileAtomic wrote wrong data: got %q, want %q", data, testData)
	}

	// Verify no temp files remain
	tmpFiles, err := filepath.Glob(testFile + ".tmp.*")
	if err != nil {
		t.Fatalf("failed to check for temp files: %v", err)
	}

	if len(tmpFiles) > 0 {
		t.Errorf("temp files remain after successful write: %v", tmpFiles)
	}
}

func TestDefaultFileSystem_Exists(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandler())
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Test non-existent file
	if fs.Exists(testFile) {
		t.Error("Exists returned true for non-existent file")
	}

	// Create file
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test existing file
	if !fs.Exists(testFile) {
		t.Error("Exists returned false for existing file")
	}
}

func TestDefaultFileSystem_MkdirAll(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandler())
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "a", "b", "c")

	// Test creating nested directories
	if err := fs.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	// Verify directory was created
	info, err := os.Stat(testDir)
	if err != nil {
		t.Fatalf("failed to stat directory: %v", err)
	}

	if !info.IsDir() {
		t.Error("MkdirAll did not create a directory")
	}
}

func TestDefaultFileSystem_Remove(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandler())
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create test file
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test removing
	if err := fs.Remove(testFile); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify file was removed
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("Remove did not remove the file")
	}
}

func TestDefaultFileSystem_Stat(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandler())
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testData := []byte("test content")

	// Create test file
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test stat
	info, err := fs.Stat(testFile)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	if info.Size() != int64(len(testData)) {
		t.Errorf("Stat returned wrong size: got %d, want %d", info.Size(), len(testData))
	}

	if info.IsDir() {
		t.Error("Stat reported file as directory")
	}
}

func TestDefaultFileSystem_ReadFile_Error(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandler())
	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "nonexistent.txt")

	// Test reading non-existent file
	_, err := fs.ReadFile(nonExistentFile)
	if err == nil {
		t.Fatal("ReadFile should have failed for non-existent file")
	}

	// Verify it's a structured error
	if _, ok := err.(*errors.StructuredError); !ok {
		t.Errorf("ReadFile should return StructuredError, got %T", err)
	}
}

func TestDefaultFileSystem_WriteFileAtomic_CleanupOnFailure(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandler())
	tmpDir := t.TempDir()
	
	// Create a read-only directory to force rename failure
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.Mkdir(readOnlyDir, 0555); err != nil {
		t.Fatalf("failed to create read-only directory: %v", err)
	}
	
	testFile := filepath.Join(readOnlyDir, "test.txt")
	testData := []byte("test content")

	// Test atomic writing to read-only directory (should fail)
	err := fs.WriteFileAtomic(testFile, testData, 0644)
	if err == nil {
		t.Fatal("WriteFileAtomic should have failed for read-only directory")
	}

	// Verify no temp files remain in parent directory
	tmpFiles, err := filepath.Glob(filepath.Join(tmpDir, "*.tmp.*"))
	if err != nil {
		t.Fatalf("failed to check for temp files: %v", err)
	}

	if len(tmpFiles) > 0 {
		t.Errorf("temp files remain after failed write: %v", tmpFiles)
	}
}

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
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
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
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
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
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
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
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
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
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
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
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
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
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
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
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
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
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
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

// TestDefaultFileSystem_WriteFile_Error tests error handling in WriteFile
func TestDefaultFileSystem_WriteFile_Error(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	
	// Test writing to invalid path (directory doesn't exist)
	invalidPath := "/nonexistent/directory/test.txt"
	err := fs.WriteFile(invalidPath, []byte("test"), 0644)
	if err == nil {
		t.Fatal("WriteFile should have failed for invalid path")
	}
	
	// Verify it's a structured error
	if _, ok := err.(*errors.StructuredError); !ok {
		t.Errorf("WriteFile should return StructuredError, got %T", err)
	}
}

// TestDefaultFileSystem_WriteFileAtomic_TempWriteError tests temp file write failure
func TestDefaultFileSystem_WriteFileAtomic_TempWriteError(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	
	// Test writing to invalid path (directory doesn't exist)
	invalidPath := "/nonexistent/directory/test.txt"
	err := fs.WriteFileAtomic(invalidPath, []byte("test"), 0644)
	if err == nil {
		t.Fatal("WriteFileAtomic should have failed for invalid path")
	}
	
	// Verify it's a structured error with write_temp operation
	structuredErr, ok := err.(*errors.StructuredError)
	if !ok {
		t.Fatalf("WriteFileAtomic should return StructuredError, got %T", err)
	}
	
	if structuredErr.Operation != "write_temp" {
		t.Errorf("Expected operation 'write_temp', got %q", structuredErr.Operation)
	}
}

// TestDefaultFileSystem_WriteFileAtomic_RenameError tests rename failure and cleanup
func TestDefaultFileSystem_WriteFileAtomic_RenameError(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := t.TempDir()
	
	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}
	
	// Create a file in the subdirectory
	testFile := filepath.Join(subDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("original"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	
	// Make the subdirectory read-only to prevent rename
	if err := os.Chmod(subDir, 0555); err != nil {
		t.Fatalf("failed to make directory read-only: %v", err)
	}
	
	// Restore permissions after test
	defer os.Chmod(subDir, 0755)
	
	// Test atomic writing (should fail on rename or temp write)
	err := fs.WriteFileAtomic(testFile, []byte("new content"), 0644)
	if err == nil {
		t.Fatal("WriteFileAtomic should have failed when directory is read-only")
	}
	
	// Verify it's a structured error (operation could be write_temp or atomic_rename depending on OS)
	structuredErr, ok := err.(*errors.StructuredError)
	if !ok {
		t.Fatalf("WriteFileAtomic should return StructuredError, got %T", err)
	}
	
	// Accept either write_temp or atomic_rename as valid failure operations
	if structuredErr.Operation != "write_temp" && structuredErr.Operation != "atomic_rename" {
		t.Errorf("Expected operation 'write_temp' or 'atomic_rename', got %q", structuredErr.Operation)
	}
}

// TestDefaultFileSystem_MkdirAll_Error tests error handling in MkdirAll
func TestDefaultFileSystem_MkdirAll_Error(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := t.TempDir()
	
	// Create a file
	testFile := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	
	// Try to create a directory with the same name as the file
	err := fs.MkdirAll(testFile, 0755)
	if err == nil {
		t.Fatal("MkdirAll should have failed when path is an existing file")
	}
	
	// Verify it's a structured error
	if _, ok := err.(*errors.StructuredError); !ok {
		t.Errorf("MkdirAll should return StructuredError, got %T", err)
	}
}

// TestDefaultFileSystem_Remove_Error tests error handling in Remove
func TestDefaultFileSystem_Remove_Error(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := t.TempDir()
	
	// Test removing non-existent file
	nonExistentFile := filepath.Join(tmpDir, "nonexistent.txt")
	err := fs.Remove(nonExistentFile)
	if err == nil {
		t.Fatal("Remove should have failed for non-existent file")
	}
	
	// Verify it's a structured error
	if _, ok := err.(*errors.StructuredError); !ok {
		t.Errorf("Remove should return StructuredError, got %T", err)
	}
}

// TestDefaultFileSystem_Stat_Error tests error handling in Stat
func TestDefaultFileSystem_Stat_Error(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := t.TempDir()
	
	// Test stat on non-existent file
	nonExistentFile := filepath.Join(tmpDir, "nonexistent.txt")
	_, err := fs.Stat(nonExistentFile)
	if err == nil {
		t.Fatal("Stat should have failed for non-existent file")
	}
	
	// Verify it's a structured error
	if _, ok := err.(*errors.StructuredError); !ok {
		t.Errorf("Stat should return StructuredError, got %T", err)
	}
}

// TestDefaultFileSystem_Exists_Directory tests Exists with directory
func TestDefaultFileSystem_Exists_Directory(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "testdir")
	
	// Test non-existent directory
	if fs.Exists(testDir) {
		t.Error("Exists returned true for non-existent directory")
	}
	
	// Create directory
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
	
	// Test existing directory
	if !fs.Exists(testDir) {
		t.Error("Exists returned false for existing directory")
	}
}

// TestDefaultFileSystem_Stat_Directory tests Stat with directory
func TestDefaultFileSystem_Stat_Directory(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "testdir")
	
	// Create directory
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
	
	// Test stat on directory
	info, err := fs.Stat(testDir)
	if err != nil {
		t.Fatalf("Stat failed on directory: %v", err)
	}
	
	if !info.IsDir() {
		t.Error("Stat should report directory as IsDir")
	}
}

// TestDefaultFileSystem_WriteFileAtomic_Overwrite tests overwriting existing file atomically
func TestDefaultFileSystem_WriteFileAtomic_Overwrite(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	
	// Create initial file
	initialData := []byte("initial content")
	if err := os.WriteFile(testFile, initialData, 0644); err != nil {
		t.Fatalf("failed to create initial file: %v", err)
	}
	
	// Overwrite with atomic write
	newData := []byte("new content")
	if err := fs.WriteFileAtomic(testFile, newData, 0644); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}
	
	// Verify file was overwritten
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	
	if string(data) != string(newData) {
		t.Errorf("WriteFileAtomic did not overwrite file: got %q, want %q", data, newData)
	}
}

// TestDefaultFileSystem_MkdirAll_ExistingDirectory tests MkdirAll with existing directory
func TestDefaultFileSystem_MkdirAll_ExistingDirectory(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "testdir")
	
	// Create directory
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	
	// Call MkdirAll on existing directory (should succeed)
	if err := fs.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed on existing directory: %v", err)
	}
}

// TestGenerateRandomString tests the random string generator
func TestGenerateRandomString(t *testing.T) {
	// Test normal case
	str1 := generateRandomString(8)
	if len(str1) != 8 {
		t.Errorf("generateRandomString(8) returned string of length %d, want 8", len(str1))
	}
	
	// Test that it generates different strings
	str2 := generateRandomString(8)
	if str1 == str2 {
		t.Error("generateRandomString should generate different strings")
	}
	
	// Test different lengths
	str3 := generateRandomString(16)
	if len(str3) != 16 {
		t.Errorf("generateRandomString(16) returned string of length %d, want 16", len(str3))
	}
	
	// Test that strings are alphanumeric
	for _, c := range str1 {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("generateRandomString returned non-hex character: %c", c)
		}
	}
}

// TestDefaultFileSystem_ReadFile_EmptyFile tests reading an empty file
func TestDefaultFileSystem_ReadFile_EmptyFile(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")
	
	// Create empty file
	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}
	
	// Test reading empty file
	data, err := fs.ReadFile(testFile)
	if err != nil {
		t.Fatalf("ReadFile failed on empty file: %v", err)
	}
	
	if len(data) != 0 {
		t.Errorf("ReadFile returned non-empty data for empty file: %v", data)
	}
}

// TestDefaultFileSystem_WriteFile_EmptyData tests writing empty data
func TestDefaultFileSystem_WriteFile_EmptyData(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")
	
	// Test writing empty data
	if err := fs.WriteFile(testFile, []byte{}, 0644); err != nil {
		t.Fatalf("WriteFile failed with empty data: %v", err)
	}
	
	// Verify file was created and is empty
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	
	if len(data) != 0 {
		t.Errorf("WriteFile wrote non-empty data: %v", data)
	}
}

// TestDefaultFileSystem_WriteFileAtomic_EmptyData tests atomic write with empty data
func TestDefaultFileSystem_WriteFileAtomic_EmptyData(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")
	
	// Test atomic writing empty data
	if err := fs.WriteFileAtomic(testFile, []byte{}, 0644); err != nil {
		t.Fatalf("WriteFileAtomic failed with empty data: %v", err)
	}
	
	// Verify file was created and is empty
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	
	if len(data) != 0 {
		t.Errorf("WriteFileAtomic wrote non-empty data: %v", data)
	}
}

// TestDefaultFileSystem_Remove_Directory tests removing a directory
func TestDefaultFileSystem_Remove_Directory(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "testdir")
	
	// Create empty directory
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	
	// Test removing empty directory
	if err := fs.Remove(testDir); err != nil {
		t.Fatalf("Remove failed on empty directory: %v", err)
	}
	
	// Verify directory was removed
	if _, err := os.Stat(testDir); !os.IsNotExist(err) {
		t.Error("Remove did not remove the directory")
	}
}

// TestDefaultFileSystem_WriteFileAtomic_MultipleWrites tests multiple atomic writes
func TestDefaultFileSystem_WriteFileAtomic_MultipleWrites(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	
	// Perform multiple atomic writes to ensure temp file cleanup works
	for i := 0; i < 5; i++ {
		data := []byte("content " + string(rune('0'+i)))
		if err := fs.WriteFileAtomic(testFile, data, 0644); err != nil {
			t.Fatalf("WriteFileAtomic failed on iteration %d: %v", i, err)
		}
		
		// Verify no temp files remain after each write
		tmpFiles, err := filepath.Glob(testFile + ".tmp.*")
		if err != nil {
			t.Fatalf("failed to check for temp files: %v", err)
		}
		
		if len(tmpFiles) > 0 {
			t.Errorf("temp files remain after iteration %d: %v", i, tmpFiles)
		}
	}
}

// TestDefaultFileSystem_WriteFileAtomic_LargeFile tests atomic write with large file
func TestDefaultFileSystem_WriteFileAtomic_LargeFile(t *testing.T) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")
	
	// Create large data (1MB)
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	
	// Test atomic writing large file
	if err := fs.WriteFileAtomic(testFile, largeData, 0644); err != nil {
		t.Fatalf("WriteFileAtomic failed with large file: %v", err)
	}
	
	// Verify file was written correctly
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read large file: %v", err)
	}
	
	if len(data) != len(largeData) {
		t.Errorf("WriteFileAtomic wrote wrong size: got %d, want %d", len(data), len(largeData))
	}
	
	// Verify no temp files remain
	tmpFiles, err := filepath.Glob(testFile + ".tmp.*")
	if err != nil {
		t.Fatalf("failed to check for temp files: %v", err)
	}
	
	if len(tmpFiles) > 0 {
		t.Errorf("temp files remain after large file write: %v", tmpFiles)
	}
}

// TestGenerateRandomString_Lengths tests various lengths
func TestGenerateRandomString_Lengths(t *testing.T) {
	lengths := []int{1, 2, 4, 8, 16, 32, 64}
	
	for _, length := range lengths {
		str := generateRandomString(length)
		if len(str) != length {
			t.Errorf("generateRandomString(%d) returned string of length %d", length, len(str))
		}
	}
}

// TestGenerateRandomString_Uniqueness tests that strings are unique
func TestGenerateRandomString_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	iterations := 100
	
	for i := 0; i < iterations; i++ {
		str := generateRandomString(16)
		if seen[str] {
			t.Errorf("generateRandomString generated duplicate string: %s", str)
		}
		seen[str] = true
	}
	
	if len(seen) != iterations {
		t.Errorf("Expected %d unique strings, got %d", iterations, len(seen))
	}
}

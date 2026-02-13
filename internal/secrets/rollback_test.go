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
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRollbackManager(t *testing.T) {
	t.Run("creates manager with provided logger", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		manager := NewRollbackManager(logger)

		assert.NotNil(t, manager)
		assert.Equal(t, logger, manager.logger)
		assert.NotNil(t, manager.backups)
		assert.Equal(t, 0, len(manager.backups))
	})

	t.Run("creates manager with default logger when nil", func(t *testing.T) {
		manager := NewRollbackManager(nil)

		assert.NotNil(t, manager)
		assert.NotNil(t, manager.logger)
		assert.NotNil(t, manager.backups)
	})
}

func TestBackup(t *testing.T) {
	t.Run("backs up existing file", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "rollback-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		testFile := filepath.Join(tmpDir, "test.txt")
		testContent := []byte("original content")
		err = os.WriteFile(testFile, testContent, 0o600)
		require.NoError(t, err)

		manager := NewRollbackManager(nil)
		err = manager.Backup(testFile)
		require.NoError(t, err)

		assert.Equal(t, 1, manager.BackupCount())
		assert.True(t, manager.HasBackups())
		assert.Equal(t, testContent, manager.backups[testFile])
	})

	t.Run("backs up non-existent file as nil", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "rollback-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		nonExistentFile := filepath.Join(tmpDir, "nonexistent.txt")

		manager := NewRollbackManager(nil)
		err = manager.Backup(nonExistentFile)
		require.NoError(t, err)

		assert.Equal(t, 1, manager.BackupCount())
		assert.True(t, manager.HasBackups())
		assert.Nil(t, manager.backups[nonExistentFile])
	})

	t.Run("backs up multiple files", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "rollback-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		file1 := filepath.Join(tmpDir, "file1.txt")
		file2 := filepath.Join(tmpDir, "file2.txt")
		content1 := []byte("content 1")
		content2 := []byte("content 2")

		err = os.WriteFile(file1, content1, 0o600)
		require.NoError(t, err)
		err = os.WriteFile(file2, content2, 0o600)
		require.NoError(t, err)

		manager := NewRollbackManager(nil)
		err = manager.Backup(file1)
		require.NoError(t, err)
		err = manager.Backup(file2)
		require.NoError(t, err)

		assert.Equal(t, 2, manager.BackupCount())
		assert.Equal(t, content1, manager.backups[file1])
		assert.Equal(t, content2, manager.backups[file2])
	})

	t.Run("returns error for unreadable file", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "rollback-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// Create a directory instead of a file to cause read error
		dirPath := filepath.Join(tmpDir, "directory")
		err = os.Mkdir(dirPath, 0o700)
		require.NoError(t, err)

		manager := NewRollbackManager(nil)
		err = manager.Backup(dirPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read file for backup")
	})
}

func TestRollback(t *testing.T) {
	t.Run("restores backed up file to original content", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "rollback-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		testFile := filepath.Join(tmpDir, "test.txt")
		originalContent := []byte("original content")
		modifiedContent := []byte("modified content")

		// Create original file
		err = os.WriteFile(testFile, originalContent, 0o600)
		require.NoError(t, err)

		// Backup the file
		manager := NewRollbackManager(nil)
		err = manager.Backup(testFile)
		require.NoError(t, err)

		// Modify the file
		err = os.WriteFile(testFile, modifiedContent, 0o600)
		require.NoError(t, err)

		// Verify file was modified
		data, err := os.ReadFile(testFile)
		require.NoError(t, err)
		assert.Equal(t, modifiedContent, data)

		// Rollback
		err = manager.Rollback()
		require.NoError(t, err)

		// Verify file was restored
		data, err = os.ReadFile(testFile)
		require.NoError(t, err)
		assert.Equal(t, originalContent, data)
	})

	t.Run("removes file that didn't exist before", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "rollback-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		newFile := filepath.Join(tmpDir, "new.txt")

		// Backup non-existent file
		manager := NewRollbackManager(nil)
		err = manager.Backup(newFile)
		require.NoError(t, err)

		// Create the file
		err = os.WriteFile(newFile, []byte("new content"), 0o600)
		require.NoError(t, err)

		// Verify file exists
		_, err = os.Stat(newFile)
		require.NoError(t, err)

		// Rollback
		err = manager.Rollback()
		require.NoError(t, err)

		// Verify file was removed
		_, err = os.Stat(newFile)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("restores multiple files", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "rollback-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		file1 := filepath.Join(tmpDir, "file1.txt")
		file2 := filepath.Join(tmpDir, "file2.txt")
		original1 := []byte("original 1")
		original2 := []byte("original 2")
		modified1 := []byte("modified 1")
		modified2 := []byte("modified 2")

		// Create original files
		err = os.WriteFile(file1, original1, 0o600)
		require.NoError(t, err)
		err = os.WriteFile(file2, original2, 0o600)
		require.NoError(t, err)

		// Backup files
		manager := NewRollbackManager(nil)
		err = manager.Backup(file1)
		require.NoError(t, err)
		err = manager.Backup(file2)
		require.NoError(t, err)

		// Modify files
		err = os.WriteFile(file1, modified1, 0o600)
		require.NoError(t, err)
		err = os.WriteFile(file2, modified2, 0o600)
		require.NoError(t, err)

		// Rollback
		err = manager.Rollback()
		require.NoError(t, err)

		// Verify files were restored
		data1, err := os.ReadFile(file1)
		require.NoError(t, err)
		assert.Equal(t, original1, data1)

		data2, err := os.ReadFile(file2)
		require.NoError(t, err)
		assert.Equal(t, original2, data2)
	})

	t.Run("handles mixed scenario: restore and remove", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "rollback-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		existingFile := filepath.Join(tmpDir, "existing.txt")
		newFile := filepath.Join(tmpDir, "new.txt")
		originalContent := []byte("original")

		// Create existing file
		err = os.WriteFile(existingFile, originalContent, 0o600)
		require.NoError(t, err)

		// Backup both files
		manager := NewRollbackManager(nil)
		err = manager.Backup(existingFile)
		require.NoError(t, err)
		err = manager.Backup(newFile)
		require.NoError(t, err)

		// Modify existing file and create new file
		err = os.WriteFile(existingFile, []byte("modified"), 0o600)
		require.NoError(t, err)
		err = os.WriteFile(newFile, []byte("new content"), 0o600)
		require.NoError(t, err)

		// Rollback
		err = manager.Rollback()
		require.NoError(t, err)

		// Verify existing file was restored
		data, err := os.ReadFile(existingFile)
		require.NoError(t, err)
		assert.Equal(t, originalContent, data)

		// Verify new file was removed
		_, err = os.Stat(newFile)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("returns error when restore fails", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "rollback-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		testFile := filepath.Join(tmpDir, "test.txt")
		originalContent := []byte("original")

		// Create and backup file
		err = os.WriteFile(testFile, originalContent, 0o600)
		require.NoError(t, err)

		manager := NewRollbackManager(nil)
		err = manager.Backup(testFile)
		require.NoError(t, err)

		// Delete the file and make the directory read-only to cause write failure
		err = os.Remove(testFile)
		require.NoError(t, err)
		
		err = os.Chmod(tmpDir, 0o500)
		require.NoError(t, err)
		defer os.Chmod(tmpDir, 0o700) // Restore permissions for cleanup

		// Rollback should fail because we can't write to read-only directory
		err = manager.Rollback()
		if err != nil {
			// On some systems, this might succeed, so we only check if error occurs
			assert.Contains(t, err.Error(), "failed to restore")
		} else {
			// If it succeeds on this system, skip the test
			t.Skip("System allows writing to read-only directory")
		}
	})

	t.Run("succeeds with no backups", func(t *testing.T) {
		manager := NewRollbackManager(nil)
		err := manager.Rollback()
		assert.NoError(t, err)
	})
}

func TestClear(t *testing.T) {
	t.Run("clears all backup data", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "rollback-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("content"), 0o600)
		require.NoError(t, err)

		manager := NewRollbackManager(nil)
		err = manager.Backup(testFile)
		require.NoError(t, err)

		assert.Equal(t, 1, manager.BackupCount())
		assert.True(t, manager.HasBackups())

		manager.Clear()

		assert.Equal(t, 0, manager.BackupCount())
		assert.False(t, manager.HasBackups())
	})

	t.Run("can be called multiple times", func(t *testing.T) {
		manager := NewRollbackManager(nil)
		manager.Clear()
		manager.Clear()
		assert.Equal(t, 0, manager.BackupCount())
	})
}

func TestHasBackups(t *testing.T) {
	t.Run("returns false when no backups", func(t *testing.T) {
		manager := NewRollbackManager(nil)
		assert.False(t, manager.HasBackups())
	})

	t.Run("returns true when backups exist", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "rollback-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("content"), 0o600)
		require.NoError(t, err)

		manager := NewRollbackManager(nil)
		err = manager.Backup(testFile)
		require.NoError(t, err)

		assert.True(t, manager.HasBackups())
	})
}

func TestBackupCount(t *testing.T) {
	t.Run("returns zero when no backups", func(t *testing.T) {
		manager := NewRollbackManager(nil)
		assert.Equal(t, 0, manager.BackupCount())
	})

	t.Run("returns correct count", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "rollback-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		manager := NewRollbackManager(nil)

		for i := 0; i < 5; i++ {
			testFile := filepath.Join(tmpDir, "test"+string(rune('0'+i))+".txt")
			err = os.WriteFile(testFile, []byte("content"), 0o600)
			require.NoError(t, err)
			err = manager.Backup(testFile)
			require.NoError(t, err)
		}

		assert.Equal(t, 5, manager.BackupCount())
	})
}

func TestRollbackIntegration(t *testing.T) {
	t.Run("complete workflow: backup, modify, rollback", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "rollback-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// Setup: Create multiple files with different scenarios
		existingFile1 := filepath.Join(tmpDir, "existing1.txt")
		existingFile2 := filepath.Join(tmpDir, "existing2.txt")
		newFile1 := filepath.Join(tmpDir, "new1.txt")
		newFile2 := filepath.Join(tmpDir, "new2.txt")

		original1 := []byte("original content 1")
		original2 := []byte("original content 2")

		err = os.WriteFile(existingFile1, original1, 0o600)
		require.NoError(t, err)
		err = os.WriteFile(existingFile2, original2, 0o600)
		require.NoError(t, err)

		// Create rollback manager and backup all files
		manager := NewRollbackManager(nil)
		err = manager.Backup(existingFile1)
		require.NoError(t, err)
		err = manager.Backup(existingFile2)
		require.NoError(t, err)
		err = manager.Backup(newFile1)
		require.NoError(t, err)
		err = manager.Backup(newFile2)
		require.NoError(t, err)

		assert.Equal(t, 4, manager.BackupCount())

		// Simulate operation: modify existing files and create new files
		err = os.WriteFile(existingFile1, []byte("modified 1"), 0o600)
		require.NoError(t, err)
		err = os.WriteFile(existingFile2, []byte("modified 2"), 0o600)
		require.NoError(t, err)
		err = os.WriteFile(newFile1, []byte("new content 1"), 0o600)
		require.NoError(t, err)
		err = os.WriteFile(newFile2, []byte("new content 2"), 0o600)
		require.NoError(t, err)

		// Verify modifications
		data, _ := os.ReadFile(existingFile1)
		assert.Equal(t, []byte("modified 1"), data)
		_, err = os.Stat(newFile1)
		assert.NoError(t, err)

		// Rollback
		err = manager.Rollback()
		require.NoError(t, err)

		// Verify rollback: existing files restored, new files removed
		data, err = os.ReadFile(existingFile1)
		require.NoError(t, err)
		assert.Equal(t, original1, data)

		data, err = os.ReadFile(existingFile2)
		require.NoError(t, err)
		assert.Equal(t, original2, data)

		_, err = os.Stat(newFile1)
		assert.True(t, os.IsNotExist(err))

		_, err = os.Stat(newFile2)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("successful operation followed by clear", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "rollback-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		testFile := filepath.Join(tmpDir, "test.txt")
		originalContent := []byte("original")
		newContent := []byte("new content")

		err = os.WriteFile(testFile, originalContent, 0o600)
		require.NoError(t, err)

		manager := NewRollbackManager(nil)
		err = manager.Backup(testFile)
		require.NoError(t, err)

		// Successful operation
		err = os.WriteFile(testFile, newContent, 0o600)
		require.NoError(t, err)

		// Clear backups (operation succeeded)
		manager.Clear()
		assert.Equal(t, 0, manager.BackupCount())

		// File should still have new content
		data, err := os.ReadFile(testFile)
		require.NoError(t, err)
		assert.Equal(t, newContent, data)
	})
}

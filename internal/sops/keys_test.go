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

package sops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewKeyManager(t *testing.T) {
	tests := []struct {
		name     string
		keyDir   string
		expected string
	}{
		{
			name:     "with custom key directory",
			keyDir:   "/custom/path",
			expected: "/custom/path",
		},
		{
			name:     "with empty key directory uses default",
			keyDir:   "",
			expected: filepath.Join(os.Getenv("HOME"), ".config", "sops", "age"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewKeyManager(tt.keyDir)
			assert.Equal(t, tt.expected, manager.keyDir)
		})
	}
}

func TestKeyManager_GenerateAgeKey(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	keyPair, err := manager.GenerateAgeKey()
	require.NoError(t, err)

	// Verify key pair structure
	assert.NotEmpty(t, keyPair.PublicKey)
	assert.NotEmpty(t, keyPair.PrivateKey)
	assert.NotEmpty(t, keyPair.Recipient)

	// Verify public key format
	assert.True(t, strings.HasPrefix(keyPair.PublicKey, "age1"))
	assert.Equal(t, 62, len(keyPair.PublicKey))

	// Verify private key format
	assert.True(t, strings.HasPrefix(keyPair.PrivateKey, "AGE-SECRET-KEY-"))

	// Verify recipient matches public key
	assert.Equal(t, keyPair.PublicKey, keyPair.Recipient)
}

func TestKeyManager_SaveAndLoadAgeKey(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	// Generate key pair
	keyPair, err := manager.GenerateAgeKey()
	require.NoError(t, err)

	keyName := "test-key"

	// Save key pair
	err = manager.SaveAgeKey(keyPair, keyName)
	require.NoError(t, err)

	// Verify files were created
	privateKeyPath := filepath.Join(tempDir, keyName+".txt")
	publicKeyPath := filepath.Join(tempDir, keyName+".pub")

	assert.FileExists(t, privateKeyPath)
	assert.FileExists(t, publicKeyPath)

	// Check file permissions
	info, err := os.Stat(privateKeyPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	info, err = os.Stat(publicKeyPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())

	// Load key pair
	loaded, err := manager.LoadAgeKey(keyName)
	require.NoError(t, err)

	// Verify loaded key matches original
	assert.Equal(t, keyPair.PrivateKey, loaded.PrivateKey)
	assert.Equal(t, keyPair.PublicKey, loaded.PublicKey)
	assert.Equal(t, keyPair.Recipient, loaded.Recipient)
}

func TestKeyManager_LoadAgeKeyNonExistent(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	_, err := manager.LoadAgeKey("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read private key")
}

func TestKeyManager_ListAgeKeys(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	// Initially empty
	keys, err := manager.ListAgeKeys()
	require.NoError(t, err)
	assert.Empty(t, keys)

	// Generate and save some keys
	keyNames := []string{"key1", "key2", "key3"}
	for _, keyName := range keyNames {
		keyPair, err := manager.GenerateAgeKey()
		require.NoError(t, err)

		err = manager.SaveAgeKey(keyPair, keyName)
		require.NoError(t, err)
	}

	// List keys
	keys, err = manager.ListAgeKeys()
	require.NoError(t, err)
	assert.Len(t, keys, 3)

	// Verify all key names are present
	for _, expectedKey := range keyNames {
		assert.Contains(t, keys, expectedKey)
	}
}

func TestKeyManager_ValidateAgeKey(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	tests := []struct {
		name        string
		key         string
		expectError bool
	}{
		{
			name:        "valid age public key",
			key:         "age1ql3vwyqfpvucvyz6x04chxtsm9l3f24tqq5pkq9jz4mk7ccvqxrqsqg5z5",
			expectError: false,
		},
		{
			name:        "valid age private key format",
			key:         "AGE-SECRET-KEY-1GFPYYSJZGFPYYSJZGFPYYSJZGFPYYSJZGFPYYSJZGFPYYSJZGFPYYS",
			expectError: true, // This is not a real valid key, just format check
		},
		{
			name:        "invalid key format",
			key:         "invalid-key-format",
			expectError: true,
		},
		{
			name:        "empty key",
			key:         "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.ValidateAgeKey(tt.key)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKeyManager_ValidatePGPKey(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	tests := []struct {
		name        string
		key         string
		expectError bool
	}{
		{
			name:        "valid 40-char PGP key",
			key:         "1234567890ABCDEF1234567890ABCDEF12345678",
			expectError: false,
		},
		{
			name:        "valid 16-char PGP key",
			key:         "1234567890ABCDEF",
			expectError: false,
		},
		{
			name:        "valid 8-char PGP key",
			key:         "12345678",
			expectError: false,
		},
		{
			name:        "lowercase PGP key",
			key:         "abcdef1234567890",
			expectError: false,
		},
		{
			name:        "invalid PGP key - too short",
			key:         "1234567",
			expectError: true,
		},
		{
			name:        "invalid PGP key - non-hex",
			key:         "GHIJKLMNOPQRSTUV",
			expectError: true,
		},
		{
			name:        "empty key",
			key:         "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.ValidatePGPKey(tt.key)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKeyManager_SetupAgeEnvironment(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	// Generate and save a key
	keyPair, err := manager.GenerateAgeKey()
	require.NoError(t, err)

	keyName := "test-env-key"
	err = manager.SaveAgeKey(keyPair, keyName)
	require.NoError(t, err)

	// Setup environment
	err = manager.SetupAgeEnvironment(keyName)
	require.NoError(t, err)

	// Verify environment variable is set
	expectedPath := filepath.Join(tempDir, keyName+".txt")
	assert.Equal(t, expectedPath, os.Getenv("SOPS_AGE_KEY_FILE"))
}

func TestKeyManager_SetupAgeEnvironmentNonExistent(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	err := manager.SetupAgeEnvironment("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "age key file not found")
}

func TestKeyManager_GenerateKeyForCluster(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	clusterName := "test-cluster"

	keyPair, err := manager.GenerateKeyForCluster(clusterName)
	require.NoError(t, err)

	// Verify key pair was generated
	assert.NotEmpty(t, keyPair.PublicKey)
	assert.NotEmpty(t, keyPair.PrivateKey)

	// Verify key was saved
	assert.FileExists(t, filepath.Join(tempDir, clusterName+".txt"))
	assert.FileExists(t, filepath.Join(tempDir, clusterName+".pub"))

	// Verify environment was set up
	expectedPath := filepath.Join(tempDir, clusterName+".txt")
	assert.Equal(t, expectedPath, os.Getenv("SOPS_AGE_KEY_FILE"))
}

func TestKeyManager_ImportAgeKey(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	// Generate a key to import
	originalKeyPair, err := manager.GenerateAgeKey()
	require.NoError(t, err)

	keyName := "imported-key"

	// Import the key
	importedKeyPair, err := manager.ImportAgeKey(keyName, originalKeyPair.PrivateKey)
	require.NoError(t, err)

	// Verify imported key matches original
	assert.Equal(t, originalKeyPair.PrivateKey, importedKeyPair.PrivateKey)
	assert.Equal(t, originalKeyPair.PublicKey, importedKeyPair.PublicKey)

	// Verify key was saved
	assert.FileExists(t, filepath.Join(tempDir, keyName+".txt"))
	assert.FileExists(t, filepath.Join(tempDir, keyName+".pub"))
}

func TestKeyManager_ImportAgeKeyInvalid(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	_, err := manager.ImportAgeKey("test", "invalid-private-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid private key")
}

func TestKeyManager_ExportAgeKey(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	// Generate and save a key
	originalKeyPair, err := manager.GenerateAgeKey()
	require.NoError(t, err)

	keyName := "export-test"
	err = manager.SaveAgeKey(originalKeyPair, keyName)
	require.NoError(t, err)

	// Export the key
	exportedKeyPair, err := manager.ExportAgeKey(keyName)
	require.NoError(t, err)

	// Verify exported key matches original
	assert.Equal(t, originalKeyPair.PrivateKey, exportedKeyPair.PrivateKey)
	assert.Equal(t, originalKeyPair.PublicKey, exportedKeyPair.PublicKey)
}

func TestKeyManager_DeleteAgeKey(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	// Generate and save a key
	keyPair, err := manager.GenerateAgeKey()
	require.NoError(t, err)

	keyName := "delete-test"
	err = manager.SaveAgeKey(keyPair, keyName)
	require.NoError(t, err)

	// Verify files exist
	privateKeyPath := filepath.Join(tempDir, keyName+".txt")
	publicKeyPath := filepath.Join(tempDir, keyName+".pub")
	assert.FileExists(t, privateKeyPath)
	assert.FileExists(t, publicKeyPath)

	// Delete the key
	err = manager.DeleteAgeKey(keyName)
	require.NoError(t, err)

	// Verify files are deleted
	assert.NoFileExists(t, privateKeyPath)
	assert.NoFileExists(t, publicKeyPath)
}

func TestKeyManager_DeleteAgeKeyNonExistent(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	// Should not error when deleting non-existent key
	err := manager.DeleteAgeKey("nonexistent")
	assert.NoError(t, err)
}

func TestKeyManager_CheckAgeInstallation(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	// This will likely fail in test environment since age might not be installed
	err := manager.CheckAgeInstallation(context.Background())
	if err != nil {
		assert.Contains(t, err.Error(), "age is not installed or not in PATH")
	}
}

func TestKeyManager_GenerateRandomPassword(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	tests := []struct {
		name           string
		length         int
		expectedLength int
	}{
		{
			name:           "default length",
			length:         0,
			expectedLength: 32,
		},
		{
			name:           "custom length",
			length:         16,
			expectedLength: 16,
		},
		{
			name:           "long password",
			length:         64,
			expectedLength: 64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			password, err := manager.GenerateRandomPassword(tt.length)
			require.NoError(t, err)

			assert.Len(t, password, tt.expectedLength)
			assert.NotEmpty(t, password)

			// Verify password contains expected character types
			hasLower := strings.ContainsAny(password, "abcdefghijklmnopqrstuvwxyz")
			hasUpper := strings.ContainsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
			hasDigit := strings.ContainsAny(password, "0123456789")
			hasSpecial := strings.ContainsAny(password, "!@#$%^&*")

			// At least one of these should be true for a random password
			assert.True(t, hasLower || hasUpper || hasDigit || hasSpecial)
		})
	}
}

func TestKeyManager_BackupAndRestoreKeys(t *testing.T) {
	tempDir := t.TempDir()
	backupDir := t.TempDir()

	manager := NewKeyManager(tempDir)

	// Generate and save some keys
	keyNames := []string{"backup-key1", "backup-key2"}
	originalKeys := make(map[string]*AgeKeyPair)

	for _, keyName := range keyNames {
		keyPair, err := manager.GenerateAgeKey()
		require.NoError(t, err)

		err = manager.SaveAgeKey(keyPair, keyName)
		require.NoError(t, err)

		originalKeys[keyName] = keyPair
	}

	// Backup keys
	err := manager.BackupKeys(backupDir)
	require.NoError(t, err)

	// Verify backup files exist
	for _, keyName := range keyNames {
		assert.FileExists(t, filepath.Join(backupDir, keyName+".txt"))
		assert.FileExists(t, filepath.Join(backupDir, keyName+".pub"))
	}

	// Clear original keys
	for _, keyName := range keyNames {
		err = manager.DeleteAgeKey(keyName)
		require.NoError(t, err)
	}

	// Verify keys are deleted
	keys, err := manager.ListAgeKeys()
	require.NoError(t, err)
	assert.Empty(t, keys)

	// Restore keys
	err = manager.RestoreKeys(backupDir)
	require.NoError(t, err)

	// Verify keys are restored
	keys, err = manager.ListAgeKeys()
	require.NoError(t, err)
	assert.Len(t, keys, len(keyNames))

	// Verify restored keys match originals
	for _, keyName := range keyNames {
		restoredKey, err := manager.LoadAgeKey(keyName)
		require.NoError(t, err)

		originalKey := originalKeys[keyName]
		assert.Equal(t, originalKey.PrivateKey, restoredKey.PrivateKey)
		assert.Equal(t, originalKey.PublicKey, restoredKey.PublicKey)
	}
}

func TestKeyManager_GetKeyInfo(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	// Generate and save a key
	keyPair, err := manager.GenerateAgeKey()
	require.NoError(t, err)

	keyName := "info-test"
	err = manager.SaveAgeKey(keyPair, keyName)
	require.NoError(t, err)

	// Get key info
	info, err := manager.GetKeyInfo(keyName)
	require.NoError(t, err)

	// Verify key info
	assert.Equal(t, keyName, info.Name)
	assert.Equal(t, keyPair.PublicKey, info.PublicKey)
	assert.Equal(t, "age", info.KeyType)
	assert.Equal(t, filepath.Join(tempDir, keyName+".txt"), info.FilePath)
	assert.WithinDuration(t, time.Now(), info.CreatedAt, time.Minute)
}

func TestKeyManager_GetKeyInfoNonExistent(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	_, err := manager.GetKeyInfo("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load key")
}

func TestKeyManager_ValidateKeyAccess(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	// Generate and save a key
	keyPair, err := manager.GenerateAgeKey()
	require.NoError(t, err)

	keyName := "access-test"
	err = manager.SaveAgeKey(keyPair, keyName)
	require.NoError(t, err)

	// This test will likely fail without SOPS installed, but we can test the key loading part
	err = manager.ValidateKeyAccess(keyName)
	if err != nil {
		// Should fail on SOPS operations, not on key loading
		assert.Contains(t, err.Error(), "key validation failed")
	}
}

func TestKeyManager_ValidateKeyAccessNonExistent(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	err := manager.ValidateKeyAccess("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load key")
}

func TestKeyManager_EdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	// Test with empty key directory
	emptyManager := NewKeyManager("/nonexistent/path")
	keys, err := emptyManager.ListAgeKeys()
	assert.NoError(t, err)
	assert.Empty(t, keys)

	// Test key validation with various formats
	tests := []struct {
		name        string
		key         string
		expectError bool
	}{
		{
			name:        "valid age public key",
			key:         "age1ql3vwyqfpvucvyz6x04chxtsm9l3f24tqq5pkq9jz4mk7ccvqxrqsqg5z5",
			expectError: false,
		},
		{
			name:        "invalid age private key format",
			key:         "AGE-SECRET-KEY-1GFPYYSJZGFPYYSJZGFPYYSJZGFPYYSJZGFPYYSJZGFPYYSJZGFPYYS",
			expectError: true, // This is not a real valid key, just format check
		},
		{
			name:        "invalid key - too short",
			key:         "age1short",
			expectError: true,
		},
		{
			name:        "invalid key - wrong prefix",
			key:         "wrong1ql3vwyqfpvucvyz6x04chxtsm9l3f24tqq5pkq9jz4mk7ccvqxrqsqg5z5",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.ValidateAgeKey(tt.key)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKeyManager_GenerateRandomPasswordEdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	// Test with zero length (should default to 32)
	password, err := manager.GenerateRandomPassword(0)
	require.NoError(t, err)
	assert.Len(t, password, 32)

	// Test with negative length (should default to 32)
	password, err = manager.GenerateRandomPassword(-5)
	require.NoError(t, err)
	assert.Len(t, password, 32)

	// Test multiple generations are different
	password1, err := manager.GenerateRandomPassword(16)
	require.NoError(t, err)

	password2, err := manager.GenerateRandomPassword(16)
	require.NoError(t, err)

	assert.NotEqual(t, password1, password2, "Generated passwords should be different")
}

func TestKeyManager_BackupRestoreEdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	backupDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	// Test backup with no keys
	err := manager.BackupKeys(backupDir)
	assert.NoError(t, err)

	// Test restore with no backup keys
	err = manager.RestoreKeys(backupDir)
	assert.NoError(t, err)

	// Test backup to non-existent directory (should create it)
	nonExistentBackup := filepath.Join(tempDir, "new-backup")
	err = manager.BackupKeys(nonExistentBackup)
	assert.NoError(t, err)
	assert.DirExists(t, nonExistentBackup)
}

func TestKeyManager_ImportExportRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewKeyManager(tempDir)

	// Generate a key
	originalKeyPair, err := manager.GenerateAgeKey()
	require.NoError(t, err)

	keyName := "roundtrip-test"

	// Save the key
	err = manager.SaveAgeKey(originalKeyPair, keyName)
	require.NoError(t, err)

	// Export the key
	exportedKeyPair, err := manager.ExportAgeKey(keyName)
	require.NoError(t, err)

	// Verify exported key matches original
	assert.Equal(t, originalKeyPair.PrivateKey, exportedKeyPair.PrivateKey)
	assert.Equal(t, originalKeyPair.PublicKey, exportedKeyPair.PublicKey)

	// Delete the key
	err = manager.DeleteAgeKey(keyName)
	require.NoError(t, err)

	// Import the key back
	importedKeyPair, err := manager.ImportAgeKey(keyName, exportedKeyPair.PrivateKey)
	require.NoError(t, err)

	// Verify imported key matches original
	assert.Equal(t, originalKeyPair.PrivateKey, importedKeyPair.PrivateKey)
	assert.Equal(t, originalKeyPair.PublicKey, importedKeyPair.PublicKey)
}

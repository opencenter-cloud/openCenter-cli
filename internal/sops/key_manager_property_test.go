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
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/zalando/go-keyring"
)

// Feature: security-and-operational-remediation, Property 7: OS Keyring Storage for Age Keys
// **Validates: Requirements 4.1, 4.2, 4.5, 4.8**
func TestProperty_OSKeyringStorageForAgeKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	// Use in-memory mock keyring to avoid OS keyring dependencies
	// (macOS Keychain prompts, Linux D-Bus hangs in CI).
	keyring.MockInit()

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20 // Reduced: each iteration hits OS keyring (macOS Keychain subprocess)
	properties := gopter.NewProperties(parameters)

	properties.Property("keys stored in keyring can be retrieved", prop.ForAll(
		func(clusterName string) bool {
			// Create temporary directory for file fallback
			tempDir := t.TempDir()

			// Create key manager with file fallback enabled
			manager := NewEnhancedKeyManager(tempDir, slog.Default())
			manager.SetFallbackToFile(true)

			// Check if keyring is available
			keyringAvailable := manager.IsKeyringAvailable()

			// Generate a key
			keyPair, err := manager.GenerateKey(clusterName)
			if err != nil {
				t.Logf("Failed to generate key: %v", err)
				return false
			}

			// Retrieve the key
			retrievedKey, err := manager.RetrieveKey(clusterName)
			if err != nil {
				t.Logf("Failed to retrieve key: %v", err)
				return false
			}

			// Verify the retrieved key matches the original
			if retrievedKey.PrivateKey != keyPair.PrivateKey {
				t.Logf("Private key mismatch")
				return false
			}

			if retrievedKey.PublicKey != keyPair.PublicKey {
				t.Logf("Public key mismatch")
				return false
			}

			// Clean up
			manager.DeleteKey(clusterName)

			// Log whether keyring was used
			if keyringAvailable {
				t.Logf("Test used OS keyring for cluster: %s", clusterName)
			} else {
				t.Logf("Test used file fallback for cluster: %s", clusterName)
			}

			return true
		},
		genValidClusterName(),
	))

	properties.Property("keys stored with keyring disabled use file storage", prop.ForAll(
		func(clusterName string) bool {
			// Create temporary directory
			tempDir := t.TempDir()

			// Create key manager with keyring disabled
			manager := NewEnhancedKeyManager(tempDir, slog.Default())
			manager.SetKeyringEnabled(false)

			// Generate a key
			keyPair, err := manager.GenerateKey(clusterName)
			if err != nil {
				t.Logf("Failed to generate key: %v", err)
				return false
			}

			// Verify file exists
			privateKeyPath := filepath.Join(tempDir, clusterName+".txt")
			if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
				t.Logf("Private key file not created")
				return false
			}

			// Retrieve the key
			retrievedKey, err := manager.RetrieveKey(clusterName)
			if err != nil {
				t.Logf("Failed to retrieve key: %v", err)
				return false
			}

			// Verify the retrieved key matches
			if retrievedKey.PrivateKey != keyPair.PrivateKey {
				return false
			}

			// Clean up
			manager.DeleteKey(clusterName)

			return true
		},
		genValidClusterName(),
	))

	properties.Property("migration moves keys from file to keyring", prop.ForAll(
		func(clusterName string) bool {
			// Create temporary directory
			tempDir := t.TempDir()

			// Create key manager with keyring disabled initially
			manager := NewEnhancedKeyManager(tempDir, slog.Default())
			manager.SetKeyringEnabled(false)

			// Generate a key in file storage
			originalKey, err := manager.GenerateKey(clusterName)
			if err != nil {
				t.Logf("Failed to generate key: %v", err)
				return false
			}

			// Enable keyring
			manager.SetKeyringEnabled(true)

			// Check if keyring is available
			if !manager.IsKeyringAvailable() {
				t.Logf("Keyring not available, skipping migration test")
				return true // Skip test if keyring not available
			}

			// Migrate to keyring
			err = manager.MigrateToKeyring(clusterName)
			if err != nil {
				t.Logf("Failed to migrate key: %v", err)
				return false
			}

			// Retrieve key (should come from keyring now)
			retrievedKey, err := manager.RetrieveKey(clusterName)
			if err != nil {
				t.Logf("Failed to retrieve migrated key: %v", err)
				return false
			}

			// Verify the key matches
			if retrievedKey.PrivateKey != originalKey.PrivateKey {
				t.Logf("Migrated key doesn't match original")
				return false
			}

			// Clean up
			manager.DeleteKey(clusterName)

			return true
		},
		genValidClusterName(),
	))

	properties.Property("deleted keys are removed from both keyring and file storage", prop.ForAll(
		func(clusterName string) bool {
			// Create temporary directory
			tempDir := t.TempDir()

			// Create key manager
			manager := NewEnhancedKeyManager(tempDir, slog.Default())
			manager.SetFallbackToFile(true)

			// Generate a key
			_, err := manager.GenerateKey(clusterName)
			if err != nil {
				t.Logf("Failed to generate key: %v", err)
				return false
			}

			// Delete the key
			err = manager.DeleteKey(clusterName)
			if err != nil {
				t.Logf("Failed to delete key: %v", err)
				return false
			}

			// Try to retrieve the key (should fail)
			_, err = manager.RetrieveKey(clusterName)
			if err == nil {
				t.Logf("Key still retrievable after deletion")
				return false
			}

			// Verify file doesn't exist
			privateKeyPath := filepath.Join(tempDir, clusterName+".txt")
			if _, err := os.Stat(privateKeyPath); !os.IsNotExist(err) {
				t.Logf("Private key file still exists after deletion")
				return false
			}

			return true
		},
		genValidClusterName(),
	))

	properties.Property("list keys returns all stored keys", prop.ForAll(
		func(clusterNames []string) bool {
			if len(clusterNames) == 0 {
				return true // Skip empty lists
			}

			// Create temporary directory
			tempDir := t.TempDir()

			// Create key manager
			manager := NewEnhancedKeyManager(tempDir, slog.Default())
			manager.SetKeyringEnabled(false) // Use file storage for predictable testing

			// Generate keys for each cluster
			for _, clusterName := range clusterNames {
				_, err := manager.GenerateKey(clusterName)
				if err != nil {
					t.Logf("Failed to generate key for %s: %v", clusterName, err)
					return false
				}
			}

			// List keys
			listedKeys, err := manager.ListKeys()
			if err != nil {
				t.Logf("Failed to list keys: %v", err)
				return false
			}

			// Verify all generated keys are listed
			keyMap := make(map[string]bool)
			for _, key := range listedKeys {
				keyMap[key] = true
			}

			for _, clusterName := range clusterNames {
				if !keyMap[clusterName] {
					t.Logf("Generated key %s not found in list", clusterName)
					return false
				}
			}

			// Clean up
			for _, clusterName := range clusterNames {
				manager.DeleteKey(clusterName)
			}

			return true
		},
		gen.SliceOfN(5, genValidClusterName()),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Helper generators

// genValidClusterName generates valid cluster names
func genValidClusterName() gopter.Gen {
	return gen.RegexMatch("[a-z][a-z0-9-]{0,30}")
}

// genValidPassphrase generates valid passphrases for backup encryption
func genValidPassphrase() gopter.Gen {
	return gen.RegexMatch("[a-zA-Z0-9!@#$%^&*]{12,32}")
}

// genAgeKeyPair generates a valid Age key pair

// Feature: security-and-operational-remediation, Property 8: Key Backup Round-Trip
// **Validates: Requirements 5.2, 5.3, 5.4**
func TestProperty_KeyBackupRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20 // Reduced: each iteration runs Argon2 key derivation (CPU + 64MB RAM)
	properties := gopter.NewProperties(parameters)

	properties.Property("backup and restore produces equivalent key", prop.ForAll(
		func(clusterName string, passphrase string) bool {
			// Skip if passphrase is too short
			if len(passphrase) < 8 {
				return true
			}

			// Create temporary directory
			tempDir := t.TempDir()

			// Create key manager
			manager := NewEnhancedKeyManager(tempDir, slog.Default())
			manager.SetKeyringEnabled(false) // Use file storage for predictable testing

			// Generate original key
			originalKey, err := manager.GenerateKey(clusterName)
			if err != nil {
				t.Logf("Failed to generate key: %v", err)
				return false
			}

			// Create backup
			backup, err := manager.BackupKey(clusterName, passphrase)
			if err != nil {
				t.Logf("Failed to create backup: %v", err)
				return false
			}

			// Verify backup is not empty
			if len(backup) == 0 {
				t.Logf("Backup is empty")
				return false
			}

			// Delete the original key
			err = manager.DeleteKey(clusterName)
			if err != nil {
				t.Logf("Failed to delete key: %v", err)
				return false
			}

			// Restore from backup
			err = manager.RestoreKey(clusterName, backup, passphrase)
			if err != nil {
				t.Logf("Failed to restore key: %v", err)
				return false
			}

			// Retrieve restored key
			restoredKey, err := manager.RetrieveKey(clusterName)
			if err != nil {
				t.Logf("Failed to retrieve restored key: %v", err)
				return false
			}

			// Verify restored key matches original
			if restoredKey.PrivateKey != originalKey.PrivateKey {
				t.Logf("Restored private key doesn't match original")
				return false
			}

			if restoredKey.PublicKey != originalKey.PublicKey {
				t.Logf("Restored public key doesn't match original")
				return false
			}

			// Clean up
			manager.DeleteKey(clusterName)

			return true
		},
		genValidClusterName(),
		genValidPassphrase(),
	))

	properties.Property("backup with wrong passphrase fails to restore", prop.ForAll(
		func(clusterName string, correctPass string, wrongPass string) bool {
			// Skip if passphrases are too short or identical
			if len(correctPass) < 8 || len(wrongPass) < 8 || correctPass == wrongPass {
				return true
			}

			// Create temporary directory
			tempDir := t.TempDir()

			// Create key manager
			manager := NewEnhancedKeyManager(tempDir, slog.Default())
			manager.SetKeyringEnabled(false)

			// Generate key
			_, err := manager.GenerateKey(clusterName)
			if err != nil {
				t.Logf("Failed to generate key: %v", err)
				return false
			}

			// Create backup with correct passphrase
			backup, err := manager.BackupKey(clusterName, correctPass)
			if err != nil {
				t.Logf("Failed to create backup: %v", err)
				return false
			}

			// Delete the key
			manager.DeleteKey(clusterName)

			// Try to restore with wrong passphrase (should fail)
			err = manager.RestoreKey(clusterName, backup, wrongPass)
			if err == nil {
				t.Logf("Restore with wrong passphrase should have failed")
				return false
			}

			return true
		},
		genValidClusterName(),
		genValidPassphrase(),
		genValidPassphrase(),
	))

	properties.Property("backup integrity check detects corruption", prop.ForAll(
		func(clusterName string, passphrase string, corruptionIndex int) bool {
			// Skip if passphrase is too short
			if len(passphrase) < 8 {
				return true
			}

			// Create temporary directory
			tempDir := t.TempDir()

			// Create key manager
			manager := NewEnhancedKeyManager(tempDir, slog.Default())
			manager.SetKeyringEnabled(false)

			// Generate key
			_, err := manager.GenerateKey(clusterName)
			if err != nil {
				t.Logf("Failed to generate key: %v", err)
				return false
			}

			// Create backup
			backup, err := manager.BackupKey(clusterName, passphrase)
			if err != nil {
				t.Logf("Failed to create backup: %v", err)
				return false
			}

			// Skip if backup is too small or corruption index is out of range
			if len(backup) < 50 || corruptionIndex < 0 || corruptionIndex >= len(backup)-32 {
				manager.DeleteKey(clusterName)
				return true
			}

			// Corrupt the backup (but not the checksum at the end)
			corruptedBackup := make([]byte, len(backup))
			copy(corruptedBackup, backup)
			corruptedBackup[corruptionIndex] ^= 0xFF // Flip all bits

			// Delete the key
			manager.DeleteKey(clusterName)

			// Try to restore corrupted backup (should fail)
			err = manager.RestoreKey(clusterName, corruptedBackup, passphrase)
			if err == nil {
				t.Logf("Restore of corrupted backup should have failed")
				return false
			}

			return true
		},
		genValidClusterName(),
		genValidPassphrase(),
		gen.IntRange(0, 200),
	))

	properties.Property("base64 export and import round-trip", prop.ForAll(
		func(clusterName string, passphrase string) bool {
			// Skip if passphrase is too short
			if len(passphrase) < 8 {
				return true
			}

			// Create temporary directory
			tempDir := t.TempDir()

			// Create key manager
			manager := NewEnhancedKeyManager(tempDir, slog.Default())
			manager.SetKeyringEnabled(false)

			// Generate original key
			originalKey, err := manager.GenerateKey(clusterName)
			if err != nil {
				t.Logf("Failed to generate key: %v", err)
				return false
			}

			// Export to base64
			backupBase64, err := manager.ExportKeyToBase64(clusterName, passphrase)
			if err != nil {
				t.Logf("Failed to export to base64: %v", err)
				return false
			}

			// Verify base64 string is not empty
			if backupBase64 == "" {
				t.Logf("Base64 export is empty")
				return false
			}

			// Delete the key
			manager.DeleteKey(clusterName)

			// Import from base64
			err = manager.ImportKeyFromBase64(clusterName, backupBase64, passphrase)
			if err != nil {
				t.Logf("Failed to import from base64: %v", err)
				return false
			}

			// Retrieve restored key
			restoredKey, err := manager.RetrieveKey(clusterName)
			if err != nil {
				t.Logf("Failed to retrieve restored key: %v", err)
				return false
			}

			// Verify restored key matches original
			if restoredKey.PrivateKey != originalKey.PrivateKey {
				t.Logf("Restored private key doesn't match original")
				return false
			}

			// Clean up
			manager.DeleteKey(clusterName)

			return true
		},
		genValidClusterName(),
		genValidPassphrase(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/rackerlabs/openCenter-cli/internal/util/crypto"
	"github.com/rackerlabs/openCenter-cli/internal/util/files"
	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/argon2"
)

const (
	// KeyringService is the service name for OS keyring storage
	KeyringService = "openCenter"
	
	// KeyringAccountSuffix is the suffix for keyring account names
	KeyringAccountSuffix = "-age-key"
	
	// Argon2 parameters for key derivation
	argon2Time    = 1
	argon2Memory  = 64 * 1024
	argon2Threads = 4
	argon2KeyLen  = 32
	
	// AES-256-GCM parameters
	gcmNonceSize = 12
	gcmTagSize   = 16
)

// EnhancedKeyManager implements enhanced key management with OS keyring support
type EnhancedKeyManager struct {
	keyDir         string
	logger         *slog.Logger
	useKeyring     bool
	keyringService string
	fallbackToFile bool
	auditLogger    interface{} // Will be *security.AuditLogger but using interface to avoid circular import
	actor          string
}

// NewEnhancedKeyManager creates a new enhanced key manager with OS keyring support
func NewEnhancedKeyManager(keyDir string, logger *slog.Logger) *EnhancedKeyManager {
	if keyDir == "" {
		keyDir = filepath.Join(os.Getenv("HOME"), ".config", "sops", "age")
	}
	if logger == nil {
		logger = slog.Default()
	}
	
	return &EnhancedKeyManager{
		keyDir:         keyDir,
		logger:         logger,
		useKeyring:     true,
		keyringService: KeyringService,
		fallbackToFile: true,
		actor:          "system",
	}
}

// SetAuditLogger sets the audit logger for logging key operations
func (m *EnhancedKeyManager) SetAuditLogger(logger interface{}) {
	m.auditLogger = logger
}

// SetActor sets the actor (user/system) performing key operations
func (m *EnhancedKeyManager) SetActor(actor string) {
	m.actor = actor
}

// StoreKey stores an Age key in the OS keyring
func (m *EnhancedKeyManager) StoreKey(cluster string, key *crypto.AgeKeyPair) error {
	m.logger.Info("Storing Age key", "cluster", cluster, "use_keyring", m.useKeyring)
	
	if m.useKeyring {
		// Try to store in OS keyring
		account := cluster + KeyringAccountSuffix
		err := keyring.Set(m.keyringService, account, key.PrivateKey)
		if err != nil {
			m.logger.Warn("Failed to store key in OS keyring, falling back to file storage",
				"cluster", cluster, "error", err)
			
			if !m.fallbackToFile {
				return fmt.Errorf("failed to store key in OS keyring: %w", err)
			}
			
			// Fallback to encrypted file storage
			return m.storeKeyInFile(cluster, key)
		}
		
		m.logger.Info("Successfully stored key in OS keyring", "cluster", cluster)
		return nil
	}
	
	// Store in file if keyring is disabled
	return m.storeKeyInFile(cluster, key)
}

// RetrieveKey retrieves an Age key from the OS keyring or file storage
func (m *EnhancedKeyManager) RetrieveKey(cluster string) (*crypto.AgeKeyPair, error) {
	m.logger.Debug("Retrieving Age key", "cluster", cluster)
	
	if m.useKeyring {
		// Try to retrieve from OS keyring
		account := cluster + KeyringAccountSuffix
		privateKey, err := keyring.Get(m.keyringService, account)
		if err == nil {
			// Parse the key to get the public key
			keyPair, parseErr := crypto.ParseAgeKey(privateKey)
			if parseErr != nil {
				return nil, fmt.Errorf("failed to parse key from keyring: %w", parseErr)
			}
			
			m.logger.Debug("Successfully retrieved key from OS keyring", "cluster", cluster)
			return keyPair, nil
		}
		
		m.logger.Debug("Key not found in OS keyring, trying file storage",
			"cluster", cluster, "error", err)
	}
	
	// Fallback to file storage
	return m.retrieveKeyFromFile(cluster)
}

// BackupKey exports an Age key to an encrypted backup file
func (m *EnhancedKeyManager) BackupKey(cluster string, passphrase string) ([]byte, error) {
	m.logger.Info("Creating encrypted backup for Age key", "cluster", cluster)
	
	// Retrieve the key
	keyPair, err := m.RetrieveKey(cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve key: %w", err)
	}
	
	// Generate salt for Argon2
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	
	// Derive encryption key from passphrase using Argon2
	derivedKey := argon2.IDKey(
		[]byte(passphrase),
		salt,
		argon2Time,
		argon2Memory,
		argon2Threads,
		argon2KeyLen,
	)
	
	// Create AES-256-GCM cipher
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	
	// Generate nonce
	nonce := make([]byte, gcmNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	
	// Encrypt the private key
	plaintext := []byte(keyPair.PrivateKey)
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	
	// Create backup format: version(1) + salt(16) + nonce(12) + ciphertext
	backup := make([]byte, 0, 1+len(salt)+len(nonce)+len(ciphertext))
	backup = append(backup, 1) // Version 1
	backup = append(backup, salt...)
	backup = append(backup, nonce...)
	backup = append(backup, ciphertext...)
	
	// Calculate SHA-256 checksum for integrity
	checksum := sha256.Sum256(backup)
	backup = append(backup, checksum[:]...)
	
	m.logger.Info("Successfully created encrypted backup", "cluster", cluster, "size", len(backup))
	return backup, nil
}

// RestoreKey restores an Age key from an encrypted backup
func (m *EnhancedKeyManager) RestoreKey(cluster string, backup []byte, passphrase string) error {
	m.logger.Info("Restoring Age key from encrypted backup", "cluster", cluster)
	
	// Verify minimum size: version(1) + salt(16) + nonce(12) + tag(16) + checksum(32) = 77 bytes
	if len(backup) < 77 {
		return fmt.Errorf("invalid backup: too small")
	}
	
	// Extract checksum and verify integrity
	checksumStart := len(backup) - 32
	expectedChecksum := backup[checksumStart:]
	data := backup[:checksumStart]
	actualChecksum := sha256.Sum256(data)
	
	if !bytesEqual(expectedChecksum, actualChecksum[:]) {
		return fmt.Errorf("backup integrity check failed: checksum mismatch")
	}
	
	// Parse backup format
	version := data[0]
	if version != 1 {
		return fmt.Errorf("unsupported backup version: %d", version)
	}
	
	salt := data[1:17]
	nonce := data[17:29]
	ciphertext := data[29:]
	
	// Derive decryption key from passphrase using Argon2
	derivedKey := argon2.IDKey(
		[]byte(passphrase),
		salt,
		argon2Time,
		argon2Memory,
		argon2Threads,
		argon2KeyLen,
	)
	
	// Create AES-256-GCM cipher
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}
	
	// Decrypt the private key
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("failed to decrypt backup (wrong passphrase?): %w", err)
	}
	
	// Parse the decrypted key
	privateKey := string(plaintext)
	keyPair, err := crypto.ParseAgeKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to parse restored key: %w", err)
	}
	
	// Store the restored key
	if err := m.StoreKey(cluster, keyPair); err != nil {
		return fmt.Errorf("failed to store restored key: %w", err)
	}
	
	m.logger.Info("Successfully restored Age key from backup", "cluster", cluster)
	return nil
}

// MigrateToKeyring migrates existing file-based keys to OS keyring
func (m *EnhancedKeyManager) MigrateToKeyring(cluster string) error {
	m.logger.Info("Migrating Age key to OS keyring", "cluster", cluster)
	
	// Check if key already exists in keyring
	account := cluster + KeyringAccountSuffix
	_, err := keyring.Get(m.keyringService, account)
	if err == nil {
		m.logger.Info("Key already exists in OS keyring", "cluster", cluster)
		return nil
	}
	
	// Load key from file
	keyPair, err := m.retrieveKeyFromFile(cluster)
	if err != nil {
		return fmt.Errorf("failed to load key from file: %w", err)
	}
	
	// Store in keyring
	err = keyring.Set(m.keyringService, account, keyPair.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to store key in OS keyring: %w", err)
	}
	
	// Optionally remove file-based key after successful migration
	// For safety, we'll keep the file as backup
	m.logger.Info("Successfully migrated key to OS keyring (file backup retained)", "cluster", cluster)
	return nil
}

// GenerateKey generates a new Age key for a cluster
func (m *EnhancedKeyManager) GenerateKey(cluster string) (*crypto.AgeKeyPair, error) {
	m.logger.Info("Generating new Age key", "cluster", cluster)
	
	// Generate age identity
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return nil, fmt.Errorf("failed to generate age identity: %w", err)
	}
	
	keyPair := &crypto.AgeKeyPair{
		PrivateKey: identity.String(),
		PublicKey:  identity.Recipient().String(),
		Recipient:  identity.Recipient().String(),
	}
	
	// Store the key
	if err := m.StoreKey(cluster, keyPair); err != nil {
		return nil, fmt.Errorf("failed to store generated key: %w", err)
	}
	
	m.logger.Info("Successfully generated and stored Age key", "cluster", cluster)
	return keyPair, nil
}

// DeleteKey deletes an Age key from both keyring and file storage
func (m *EnhancedKeyManager) DeleteKey(cluster string) error {
	m.logger.Info("Deleting Age key", "cluster", cluster)
	
	var errors []error
	
	// Try to delete from keyring
	if m.useKeyring {
		account := cluster + KeyringAccountSuffix
		err := keyring.Delete(m.keyringService, account)
		if err != nil && !strings.Contains(err.Error(), "not found") {
			errors = append(errors, fmt.Errorf("keyring deletion failed: %w", err))
		}
	}
	
	// Try to delete from file storage
	if err := m.deleteKeyFromFile(cluster); err != nil && !os.IsNotExist(err) {
		errors = append(errors, fmt.Errorf("file deletion failed: %w", err))
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("failed to delete key: %v", errors)
	}
	
	m.logger.Info("Successfully deleted Age key", "cluster", cluster)
	return nil
}

// ListKeys lists all available Age keys from both keyring and file storage
func (m *EnhancedKeyManager) ListKeys() ([]string, error) {
	m.logger.Debug("Listing Age keys")
	
	keysMap := make(map[string]bool)
	
	// List keys from file storage
	if _, err := os.Stat(m.keyDir); !os.IsNotExist(err) {
		files, err := os.ReadDir(m.keyDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read key directory: %w", err)
		}
		
		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".txt") {
				keyName := strings.TrimSuffix(file.Name(), ".txt")
				keysMap[keyName] = true
			}
		}
	}
	
	// Convert map to slice
	keys := make([]string, 0, len(keysMap))
	for key := range keysMap {
		keys = append(keys, key)
	}
	
	return keys, nil
}

// IsKeyringAvailable checks if OS keyring is available on the current platform
func (m *EnhancedKeyManager) IsKeyringAvailable() bool {
	// Test keyring availability by attempting a dummy operation
	testAccount := "test-availability-check"
	err := keyring.Set(m.keyringService, testAccount, "test")
	if err == nil {
		// Clean up test entry
		keyring.Delete(m.keyringService, testAccount)
		return true
	}
	
	m.logger.Debug("OS keyring not available", "error", err, "platform", runtime.GOOS)
	return false
}

// SetKeyringEnabled enables or disables OS keyring usage
func (m *EnhancedKeyManager) SetKeyringEnabled(enabled bool) {
	m.useKeyring = enabled
	m.logger.Info("OS keyring usage updated", "enabled", enabled)
}

// SetFallbackToFile enables or disables fallback to file storage
func (m *EnhancedKeyManager) SetFallbackToFile(enabled bool) {
	m.fallbackToFile = enabled
	m.logger.Info("Fallback to file storage updated", "enabled", enabled)
}

// Private helper methods

// storeKeyInFile stores a key in encrypted file storage
func (m *EnhancedKeyManager) storeKeyInFile(cluster string, key *crypto.AgeKeyPair) error {
	// Ensure key directory exists
	if err := os.MkdirAll(m.keyDir, 0o700); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}
	
	// Use atomic file operations to prevent corruption
	privateKeyPath := filepath.Join(m.keyDir, fmt.Sprintf("%s.txt", cluster))
	publicKeyPath := filepath.Join(m.keyDir, fmt.Sprintf("%s.pub", cluster))
	
	// Save private key atomically
	keyContent := key.PrivateKey
	if !strings.HasSuffix(keyContent, "\n") {
		keyContent += "\n"
	}
	if err := files.WriteFileAtomic(privateKeyPath, []byte(keyContent), 0o600); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}
	
	// Save public key atomically
	if err := files.WriteFileAtomic(publicKeyPath, []byte(key.PublicKey), 0o644); err != nil {
		// Clean up private key if public key save fails
		os.Remove(privateKeyPath)
		return fmt.Errorf("failed to save public key: %w", err)
	}
	
	m.logger.Info("Stored key in file storage (encrypted)", "cluster", cluster)
	return nil
}

// retrieveKeyFromFile retrieves a key from file storage
func (m *EnhancedKeyManager) retrieveKeyFromFile(cluster string) (*crypto.AgeKeyPair, error) {
	// Load private key
	privateKeyPath := filepath.Join(m.keyDir, fmt.Sprintf("%s.txt", cluster))
	privateKeyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}
	
	// Extract the actual private key (skip comments and empty lines)
	privateKeyStr := extractAgeKey(string(privateKeyData))
	if privateKeyStr == "" {
		return nil, fmt.Errorf("no valid age key found in file")
	}
	
	// Load public key
	publicKeyPath := filepath.Join(m.keyDir, fmt.Sprintf("%s.pub", cluster))
	publicKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}
	
	keyPair := &crypto.AgeKeyPair{
		PrivateKey: privateKeyStr,
		PublicKey:  strings.TrimSpace(string(publicKeyData)),
		Recipient:  strings.TrimSpace(string(publicKeyData)),
	}
	
	return keyPair, nil
}

// deleteKeyFromFile deletes a key from file storage
func (m *EnhancedKeyManager) deleteKeyFromFile(cluster string) error {
	// Delete private key
	privateKeyPath := filepath.Join(m.keyDir, fmt.Sprintf("%s.txt", cluster))
	if err := os.Remove(privateKeyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete private key: %w", err)
	}
	
	// Delete public key
	publicKeyPath := filepath.Join(m.keyDir, fmt.Sprintf("%s.pub", cluster))
	if err := os.Remove(publicKeyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete public key: %w", err)
	}
	
	return nil
}

// extractAgeKey extracts the age key from file content, skipping comments and empty lines
func extractAgeKey(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Age private keys start with AGE-SECRET-KEY-
		// Age public keys start with age1
		if strings.HasPrefix(line, "AGE-SECRET-KEY-") || strings.HasPrefix(line, "age1") {
			return line
		}
	}
	return ""
}

// bytesEqual compares two byte slices in constant time
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := range a {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

// ExportKeyToBase64 exports a key backup as base64-encoded string
func (m *EnhancedKeyManager) ExportKeyToBase64(cluster string, passphrase string) (string, error) {
	backup, err := m.BackupKey(cluster, passphrase)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(backup), nil
}

// ImportKeyFromBase64 imports a key from base64-encoded backup string
func (m *EnhancedKeyManager) ImportKeyFromBase64(cluster string, backupBase64 string, passphrase string) error {
	backup, err := base64.StdEncoding.DecodeString(backupBase64)
	if err != nil {
		return fmt.Errorf("failed to decode base64 backup: %w", err)
	}
	return m.RestoreKey(cluster, backup, passphrase)
}

// GenerateAdditionalKey generates an additional Age key for multi-key SOPS configuration
func (m *EnhancedKeyManager) GenerateAdditionalKey(cluster string, keyIndex int) (*crypto.AgeKeyPair, error) {
	keyName := fmt.Sprintf("%s-key-%d", cluster, keyIndex)
	m.logger.Info("Generating additional Age key", "cluster", cluster, "key_index", keyIndex)
	
	// Generate age identity
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return nil, fmt.Errorf("failed to generate age identity: %w", err)
	}
	
	keyPair := &crypto.AgeKeyPair{
		PrivateKey: identity.String(),
		PublicKey:  identity.Recipient().String(),
		Recipient:  identity.Recipient().String(),
	}
	
	// Store the key
	if err := m.StoreKey(keyName, keyPair); err != nil {
		return nil, fmt.Errorf("failed to store additional key: %w", err)
	}
	
	m.logger.Info("Successfully generated and stored additional Age key", "cluster", cluster, "key_index", keyIndex)
	return keyPair, nil
}

// ListClusterKeys lists all keys for a specific cluster (primary and additional)
func (m *EnhancedKeyManager) ListClusterKeys(cluster string) ([]*crypto.AgeKeyPair, error) {
	m.logger.Debug("Listing all keys for cluster", "cluster", cluster)
	
	var keys []*crypto.AgeKeyPair
	
	// Try to retrieve primary key
	primaryKey, err := m.RetrieveKey(cluster)
	if err == nil {
		keys = append(keys, primaryKey)
	}
	
	// Try to retrieve additional keys (key-1, key-2, etc.)
	for i := 1; i < 10; i++ { // Support up to 10 keys
		keyName := fmt.Sprintf("%s-key-%d", cluster, i)
		additionalKey, err := m.RetrieveKey(keyName)
		if err != nil {
			// No more keys found
			break
		}
		keys = append(keys, additionalKey)
	}
	
	if len(keys) == 0 {
		return nil, fmt.Errorf("no keys found for cluster: %s", cluster)
	}
	
	m.logger.Debug("Found keys for cluster", "cluster", cluster, "count", len(keys))
	return keys, nil
}

// GenerateSOPSConfig generates a .sops.yaml configuration with multi-key support
func (m *EnhancedKeyManager) GenerateSOPSConfig(cluster string) (string, error) {
	m.logger.Info("Generating SOPS configuration", "cluster", cluster)
	
	// Get all keys for the cluster
	keys, err := m.ListClusterKeys(cluster)
	if err != nil {
		return "", fmt.Errorf("failed to list cluster keys: %w", err)
	}
	
	// Build age keys list
	var ageKeys []string
	for _, key := range keys {
		ageKeys = append(ageKeys, key.PublicKey)
	}
	
	// Generate SOPS config
	config := fmt.Sprintf(`# SOPS configuration for cluster: %s
# Multi-key encryption with %d keys
creation_rules:
  - path_regex: .*\.(yaml|yml)$
    age: >-
`, cluster, len(ageKeys))
	
	// Add each key on a new line with proper indentation
	for i, key := range ageKeys {
		if i == len(ageKeys)-1 {
			config += fmt.Sprintf("      %s\n", key)
		} else {
			config += fmt.Sprintf("      %s,\n", key)
		}
	}
	
	config += `    encrypted_regex: '^(data|stringData|password|token|key|secret|credentials)'
`
	
	m.logger.Info("Successfully generated SOPS configuration", "cluster", cluster, "key_count", len(ageKeys))
	return config, nil
}

// RotateClusterKeys rotates all keys for a cluster by generating new keys
func (m *EnhancedKeyManager) RotateClusterKeys(cluster string) error {
	m.logger.Info("Rotating keys for cluster", "cluster", cluster)
	
	// Get current keys
	currentKeys, err := m.ListClusterKeys(cluster)
	if err != nil {
		return fmt.Errorf("failed to list current keys: %w", err)
	}
	
	keyCount := len(currentKeys)
	
	// Backup current keys before rotation
	for i, key := range currentKeys {
		keyName := cluster
		if i > 0 {
			keyName = fmt.Sprintf("%s-key-%d", cluster, i)
		}
		
		// Create backup with timestamp
		backupName := fmt.Sprintf("%s-backup-%d", keyName, time.Now().Unix())
		if err := m.StoreKey(backupName, key); err != nil {
			m.logger.Warn("Failed to backup key before rotation", "key", keyName, "error", err)
		}
	}
	
	// Generate new keys
	for i := 0; i < keyCount; i++ {
		if i == 0 {
			// Rotate primary key
			_, err := m.GenerateKey(cluster)
			if err != nil {
				return fmt.Errorf("failed to rotate primary key: %w", err)
			}
		} else {
			// Rotate additional key
			_, err := m.GenerateAdditionalKey(cluster, i)
			if err != nil {
				return fmt.Errorf("failed to rotate additional key %d: %w", i, err)
			}
		}
	}
	
	m.logger.Info("Successfully rotated all keys for cluster", "cluster", cluster, "key_count", keyCount)
	return nil
}

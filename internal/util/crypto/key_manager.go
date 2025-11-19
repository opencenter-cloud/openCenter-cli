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

package crypto

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rackerlabs/openCenter-cli/internal/util/files"
)

// DefaultKeyManager implements KeyManager interface
type DefaultKeyManager struct {
	keyDir    string
	generator KeyGenerator
	validator KeyValidator
}

// NewDefaultKeyManager creates a new default key manager
func NewDefaultKeyManager(keyDir string) *DefaultKeyManager {
	if keyDir == "" {
		keyDir = filepath.Join(os.Getenv("HOME"), ".config", "sops", "age")
	}
	return &DefaultKeyManager{
		keyDir:    keyDir,
		generator: NewAgeKeyGenerator(),
		validator: NewAgeKeyValidator(),
	}
}

// GenerateAgeKey generates a new age key pair with validation
func (m *DefaultKeyManager) GenerateAgeKey() (*AgeKeyPair, error) {
	return m.generator.GenerateAgeKey()
}

// GenerateRandomPassword generates a random password for key encryption
func (m *DefaultKeyManager) GenerateRandomPassword(length int) (string, error) {
	return m.generator.GenerateRandomPassword(length)
}

// GenerateFallbackKey generates a fallback age key when no key is available
func (m *DefaultKeyManager) GenerateFallbackKey() (*AgeKeyPair, error) {
	// Generate a new key pair
	keyPair, err := m.generator.GenerateFallbackKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate fallback key: %w", err)
	}

	// Save the fallback key with a default name
	fallbackKeyName := "fallback-" + fmt.Sprintf("%d", time.Now().Unix())
	if err := m.SaveAgeKey(keyPair, fallbackKeyName); err != nil {
		return nil, fmt.Errorf("failed to save fallback key: %w", err)
	}

	return keyPair, nil
}

// ValidateAgeKey validates an age key format
func (m *DefaultKeyManager) ValidateAgeKey(key string) error {
	return m.validator.ValidateAgeKey(key)
}

// ValidatePGPKey validates a PGP key format
func (m *DefaultKeyManager) ValidatePGPKey(key string) error {
	return m.validator.ValidatePGPKey(key)
}

// ValidateKeyForProduction validates that a key is not a placeholder
func (m *DefaultKeyManager) ValidateKeyForProduction(key string) error {
	return m.validator.ValidateKeyForProduction(key)
}

// SaveAgeKey saves an age key pair to disk with atomic operations
func (m *DefaultKeyManager) SaveAgeKey(keyPair *AgeKeyPair, keyName string) error {
	// Validate key format before saving
	if err := m.ValidateAgeKey(keyPair.PrivateKey); err != nil {
		return fmt.Errorf("invalid private key format: %w", err)
	}
	if err := m.ValidateAgeKey(keyPair.PublicKey); err != nil {
		return fmt.Errorf("invalid public key format: %w", err)
	}

	// Ensure key directory exists
	if err := os.MkdirAll(m.keyDir, 0o700); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// Use atomic file operations to prevent corruption
	privateKeyPath := filepath.Join(m.keyDir, fmt.Sprintf("%s.txt", keyName))
	publicKeyPath := filepath.Join(m.keyDir, fmt.Sprintf("%s.pub", keyName))

	// Save private key atomically
	keyContent := keyPair.PrivateKey
	if !strings.HasSuffix(keyContent, "\n") {
		keyContent += "\n"
	}
	if err := files.WriteFileAtomic(privateKeyPath, []byte(keyContent), 0o600); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}

	// Save public key atomically
	if err := files.WriteFileAtomic(publicKeyPath, []byte(keyPair.PublicKey), 0o644); err != nil {
		// Clean up private key if public key save fails
		os.Remove(privateKeyPath)
		return fmt.Errorf("failed to save public key: %w", err)
	}

	return nil
}

// LoadAgeKey loads an age key pair from disk
func (m *DefaultKeyManager) LoadAgeKey(keyName string) (*AgeKeyPair, error) {
	// Load private key
	privateKeyPath := filepath.Join(m.keyDir, fmt.Sprintf("%s.txt", keyName))
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
	publicKeyPath := filepath.Join(m.keyDir, fmt.Sprintf("%s.pub", keyName))
	publicKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	keyPair := &AgeKeyPair{
		PrivateKey: privateKeyStr,
		PublicKey:  strings.TrimSpace(string(publicKeyData)),
		Recipient:  strings.TrimSpace(string(publicKeyData)),
	}

	return keyPair, nil
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

// ListAgeKeys lists all available age keys
func (m *DefaultKeyManager) ListAgeKeys() ([]string, error) {
	if _, err := os.Stat(m.keyDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	files, err := os.ReadDir(m.keyDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read key directory: %w", err)
	}

	var keyNames []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".txt") {
			keyName := strings.TrimSuffix(file.Name(), ".txt")
			keyNames = append(keyNames, keyName)
		}
	}

	return keyNames, nil
}

// DeleteAgeKey deletes an age key pair
func (m *DefaultKeyManager) DeleteAgeKey(keyName string) error {
	// Delete private key
	privateKeyPath := filepath.Join(m.keyDir, fmt.Sprintf("%s.txt", keyName))
	if err := os.Remove(privateKeyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete private key: %w", err)
	}

	// Delete public key
	publicKeyPath := filepath.Join(m.keyDir, fmt.Sprintf("%s.pub", keyName))
	if err := os.Remove(publicKeyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete public key: %w", err)
	}

	return nil
}

// ImportAgeKey imports an existing age key
func (m *DefaultKeyManager) ImportAgeKey(keyName, privateKey string) (*AgeKeyPair, error) {
	// Validate private key format
	if err := m.ValidateAgeKey(privateKey); err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	// Parse private key to get public key
	keyPair, err := ParseAgeKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse age identity: %w", err)
	}

	// Save key pair
	if err := m.SaveAgeKey(keyPair, keyName); err != nil {
		return nil, fmt.Errorf("failed to save imported key: %w", err)
	}

	return keyPair, nil
}

// ExportAgeKey exports an age key pair
func (m *DefaultKeyManager) ExportAgeKey(keyName string) (*AgeKeyPair, error) {
	return m.LoadAgeKey(keyName)
}

// BackupKeys creates a backup of all age keys
func (m *DefaultKeyManager) BackupKeys(backupPath string) error {
	// Ensure backup directory exists
	if err := os.MkdirAll(backupPath, 0o700); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Get list of keys
	keyNames, err := m.ListAgeKeys()
	if err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}

	// Backup each key
	for _, keyName := range keyNames {
		keyPair, err := m.LoadAgeKey(keyName)
		if err != nil {
			return fmt.Errorf("failed to load key %s: %w", keyName, err)
		}

		// Save to backup location
		backupKeyManager := NewDefaultKeyManager(backupPath)
		if err := backupKeyManager.SaveAgeKey(keyPair, keyName); err != nil {
			return fmt.Errorf("failed to backup key %s: %w", keyName, err)
		}
	}

	return nil
}

// RestoreKeys restores age keys from a backup
func (m *DefaultKeyManager) RestoreKeys(backupPath string) error {
	backupKeyManager := NewDefaultKeyManager(backupPath)

	// Get list of backup keys
	keyNames, err := backupKeyManager.ListAgeKeys()
	if err != nil {
		return fmt.Errorf("failed to list backup keys: %w", err)
	}

	// Restore each key
	for _, keyName := range keyNames {
		keyPair, err := backupKeyManager.LoadAgeKey(keyName)
		if err != nil {
			return fmt.Errorf("failed to load backup key %s: %w", keyName, err)
		}

		// Save to current location
		if err := m.SaveAgeKey(keyPair, keyName); err != nil {
			return fmt.Errorf("failed to restore key %s: %w", keyName, err)
		}
	}

	return nil
}

// GetKeyInfo returns information about a key
func (m *DefaultKeyManager) GetKeyInfo(keyName string) (*KeyInfo, error) {
	keyPair, err := m.LoadAgeKey(keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to load key: %w", err)
	}

	// Get file stats
	privateKeyPath := filepath.Join(m.keyDir, fmt.Sprintf("%s.txt", keyName))
	stat, err := os.Stat(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get key file stats: %w", err)
	}

	info := &KeyInfo{
		Name:      keyName,
		PublicKey: keyPair.PublicKey,
		CreatedAt: stat.ModTime(),
		KeyType:   "age",
		FilePath:  privateKeyPath,
	}

	return info, nil
}

// SetupAgeEnvironment sets up the age environment for SOPS
func (m *DefaultKeyManager) SetupAgeEnvironment(keyName string) error {
	keyPath := filepath.Join(m.keyDir, fmt.Sprintf("%s.txt", keyName))

	// Check if key file exists
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return fmt.Errorf("age key file not found: %s", keyPath)
	}

	// Set SOPS_AGE_KEY_FILE environment variable
	if err := os.Setenv("SOPS_AGE_KEY_FILE", keyPath); err != nil {
		return fmt.Errorf("failed to set SOPS_AGE_KEY_FILE: %w", err)
	}

	return nil
}

// GenerateKeyForCluster generates and saves an age key for a specific cluster
func (m *DefaultKeyManager) GenerateKeyForCluster(clusterName string) (*AgeKeyPair, error) {
	// Generate new key pair
	keyPair, err := m.GenerateAgeKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Save key pair with cluster name
	if err := m.SaveAgeKey(keyPair, clusterName); err != nil {
		return nil, fmt.Errorf("failed to save key pair: %w", err)
	}

	// Set up environment
	if err := m.SetupAgeEnvironment(clusterName); err != nil {
		return nil, fmt.Errorf("failed to setup age environment: %w", err)
	}

	return keyPair, nil
}

// CheckAgeInstallation checks if age is properly installed
func (m *DefaultKeyManager) CheckAgeInstallation(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "age", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("age is not installed or not in PATH: %w", err)
	}
	return nil
}

// ValidateKeyAccess validates that a key can be used for encryption/decryption
func (m *DefaultKeyManager) ValidateKeyAccess(keyName string) error {
	// Load the key
	keyPair, err := m.LoadAgeKey(keyName)
	if err != nil {
		return fmt.Errorf("failed to load key: %w", err)
	}

	// Validate key format
	if err := m.ValidateAgeKey(keyPair.PrivateKey); err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	if err := m.ValidateAgeKey(keyPair.PublicKey); err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	// Test basic key parsing
	if _, err := ParseAgeKey(keyPair.PrivateKey); err != nil {
		return fmt.Errorf("key validation failed - unable to parse private key: %w", err)
	}

	return nil
}

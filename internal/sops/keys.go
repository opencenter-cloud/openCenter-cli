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
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"filippo.io/age"
)

// KeyManager handles SOPS key management operations
type KeyManager struct {
	keyDir string
}

// NewKeyManager creates a new key manager
func NewKeyManager(keyDir string) *KeyManager {
	if keyDir == "" {
		keyDir = filepath.Join(os.Getenv("HOME"), ".config", "sops", "age")
	}
	return &KeyManager{
		keyDir: keyDir,
	}
}

// AgeKeyPair represents an age key pair
type AgeKeyPair struct {
	PublicKey  string
	PrivateKey string
	Recipient  string
}

// GenerateAgeKey generates a new age key pair with validation
func (k *KeyManager) GenerateAgeKey() (*AgeKeyPair, error) {
	// Generate age identity
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return nil, fmt.Errorf("failed to generate age identity: %w", err)
	}

	keyPair := &AgeKeyPair{
		PrivateKey: identity.String(),
		PublicKey:  identity.Recipient().String(),
		Recipient:  identity.Recipient().String(),
	}

	// Validate generated key pair
	if err := k.ValidateAgeKey(keyPair.PrivateKey); err != nil {
		return nil, fmt.Errorf("generated private key validation failed: %w", err)
	}
	if err := k.ValidateAgeKey(keyPair.PublicKey); err != nil {
		return nil, fmt.Errorf("generated public key validation failed: %w", err)
	}

	return keyPair, nil
}

// generateFallbackKey generates a fallback age key when no key is available
func (k *KeyManager) generateFallbackKey() (*AgeKeyPair, error) {
	// Generate a new key pair
	keyPair, err := k.GenerateAgeKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate fallback key: %w", err)
	}

	// Save the fallback key with a default name
	fallbackKeyName := "fallback-" + fmt.Sprintf("%d", time.Now().Unix())
	if err := k.SaveAgeKey(keyPair, fallbackKeyName); err != nil {
		return nil, fmt.Errorf("failed to save fallback key: %w", err)
	}

	return keyPair, nil
}

// writeFileAtomic writes data to a file atomically by writing to a temporary file first
func (k *KeyManager) writeFileAtomic(filename string, data []byte, perm os.FileMode) error {
	// Create temporary file in the same directory
	dir := filepath.Dir(filename)
	tmpFile, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(filename)+"-")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}

	tmpPath := tmpFile.Name()
	
	// Ensure cleanup on failure
	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath)
	}()

	// Write data to temporary file
	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("failed to write to temporary file: %w", err)
	}

	// Set proper permissions
	if err := tmpFile.Chmod(perm); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}

	// Close the temporary file
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Atomically move temporary file to final location
	if err := os.Rename(tmpPath, filename); err != nil {
		return fmt.Errorf("failed to move temporary file to final location: %w", err)
	}

	// Don't remove tmpPath in defer since rename succeeded
	tmpPath = ""
	return nil
}

// SaveAgeKey saves an age key pair to disk with atomic operations
func (k *KeyManager) SaveAgeKey(keyPair *AgeKeyPair, keyName string) error {
	// Validate key format before saving
	if err := k.ValidateAgeKey(keyPair.PrivateKey); err != nil {
		return fmt.Errorf("invalid private key format: %w", err)
	}
	if err := k.ValidateAgeKey(keyPair.PublicKey); err != nil {
		return fmt.Errorf("invalid public key format: %w", err)
	}

	// Ensure key directory exists
	if err := os.MkdirAll(k.keyDir, 0o700); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// Use atomic file operations to prevent corruption
	privateKeyPath := filepath.Join(k.keyDir, fmt.Sprintf("%s.txt", keyName))
	publicKeyPath := filepath.Join(k.keyDir, fmt.Sprintf("%s.pub", keyName))

	// Save private key atomically
	if err := k.writeFileAtomic(privateKeyPath, []byte(keyPair.PrivateKey), 0o600); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}

	// Save public key atomically
	if err := k.writeFileAtomic(publicKeyPath, []byte(keyPair.PublicKey), 0o644); err != nil {
		// Clean up private key if public key save fails
		os.Remove(privateKeyPath)
		return fmt.Errorf("failed to save public key: %w", err)
	}

	return nil
}

// LoadAgeKey loads an age key pair from disk
func (k *KeyManager) LoadAgeKey(keyName string) (*AgeKeyPair, error) {
	// Load private key
	privateKeyPath := filepath.Join(k.keyDir, fmt.Sprintf("%s.txt", keyName))
	privateKeyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	// Load public key
	publicKeyPath := filepath.Join(k.keyDir, fmt.Sprintf("%s.pub", keyName))
	publicKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	keyPair := &AgeKeyPair{
		PrivateKey: strings.TrimSpace(string(privateKeyData)),
		PublicKey:  strings.TrimSpace(string(publicKeyData)),
		Recipient:  strings.TrimSpace(string(publicKeyData)),
	}

	return keyPair, nil
}

// ListAgeKeys lists all available age keys
func (k *KeyManager) ListAgeKeys() ([]string, error) {
	if _, err := os.Stat(k.keyDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	files, err := os.ReadDir(k.keyDir)
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

// ValidateAgeKey validates an age key format
func (k *KeyManager) ValidateAgeKey(key string) error {
	// Age public keys start with "age1" and are base64-encoded
	agePublicKeyRegex := regexp.MustCompile(`^age1[a-z0-9]{58}$`)
	if agePublicKeyRegex.MatchString(key) {
		return nil
	}

	// Age private keys start with "AGE-SECRET-KEY-1"
	agePrivateKeyRegex := regexp.MustCompile(`^AGE-SECRET-KEY-1[A-Z0-9]{58}$`)
	if agePrivateKeyRegex.MatchString(key) {
		return nil
	}

	return fmt.Errorf("invalid age key format: %s", key)
}

// ValidatePGPKey validates a PGP key format
func (k *KeyManager) ValidatePGPKey(key string) error {
	// PGP keys are typically 40-character hex strings (fingerprints)
	pgpKeyRegex := regexp.MustCompile(`^[A-F0-9]{40}$`)
	if pgpKeyRegex.MatchString(strings.ToUpper(key)) {
		return nil
	}

	// Also accept shorter key IDs
	shortPGPKeyRegex := regexp.MustCompile(`^[A-F0-9]{8,16}$`)
	if shortPGPKeyRegex.MatchString(strings.ToUpper(key)) {
		return nil
	}

	return fmt.Errorf("invalid PGP key format: %s", key)
}

// SetupAgeEnvironment sets up the age environment for SOPS
func (k *KeyManager) SetupAgeEnvironment(keyName string) error {
	keyPath := filepath.Join(k.keyDir, fmt.Sprintf("%s.txt", keyName))

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
func (k *KeyManager) GenerateKeyForCluster(clusterName string) (*AgeKeyPair, error) {
	// Generate new key pair
	keyPair, err := k.GenerateAgeKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Save key pair with cluster name
	if err := k.SaveAgeKey(keyPair, clusterName); err != nil {
		return nil, fmt.Errorf("failed to save key pair: %w", err)
	}

	// Set up environment
	if err := k.SetupAgeEnvironment(clusterName); err != nil {
		return nil, fmt.Errorf("failed to setup age environment: %w", err)
	}

	return keyPair, nil
}

// ImportAgeKey imports an existing age key
func (k *KeyManager) ImportAgeKey(keyName, privateKey string) (*AgeKeyPair, error) {
	// Validate private key format
	if err := k.ValidateAgeKey(privateKey); err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	// Parse private key to get public key
	identity, err := age.ParseX25519Identity(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse age identity: %w", err)
	}

	keyPair := &AgeKeyPair{
		PrivateKey: privateKey,
		PublicKey:  identity.Recipient().String(),
		Recipient:  identity.Recipient().String(),
	}

	// Save key pair
	if err := k.SaveAgeKey(keyPair, keyName); err != nil {
		return nil, fmt.Errorf("failed to save imported key: %w", err)
	}

	return keyPair, nil
}

// ExportAgeKey exports an age key pair
func (k *KeyManager) ExportAgeKey(keyName string) (*AgeKeyPair, error) {
	return k.LoadAgeKey(keyName)
}

// DeleteAgeKey deletes an age key pair
func (k *KeyManager) DeleteAgeKey(keyName string) error {
	// Delete private key
	privateKeyPath := filepath.Join(k.keyDir, fmt.Sprintf("%s.txt", keyName))
	if err := os.Remove(privateKeyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete private key: %w", err)
	}

	// Delete public key
	publicKeyPath := filepath.Join(k.keyDir, fmt.Sprintf("%s.pub", keyName))
	if err := os.Remove(publicKeyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete public key: %w", err)
	}

	return nil
}

// CheckAgeInstallation checks if age is properly installed
func (k *KeyManager) CheckAgeInstallation(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "age", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("age is not installed or not in PATH: %w", err)
	}
	return nil
}

// GenerateRandomPassword generates a random password for key encryption
func (k *KeyManager) GenerateRandomPassword(length int) (string, error) {
	if length <= 0 {
		length = 32
	}

	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	password := make([]byte, length)

	for i := range password {
		randomIndex := make([]byte, 1)
		if _, err := rand.Read(randomIndex); err != nil {
			return "", fmt.Errorf("failed to generate random bytes: %w", err)
		}
		password[i] = charset[randomIndex[0]%byte(len(charset))]
	}

	return string(password), nil
}

// BackupKeys creates a backup of all age keys
func (k *KeyManager) BackupKeys(backupPath string) error {
	// Ensure backup directory exists
	if err := os.MkdirAll(backupPath, 0o700); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Get list of keys
	keyNames, err := k.ListAgeKeys()
	if err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}

	// Backup each key
	for _, keyName := range keyNames {
		keyPair, err := k.LoadAgeKey(keyName)
		if err != nil {
			return fmt.Errorf("failed to load key %s: %w", keyName, err)
		}

		// Save to backup location
		backupKeyManager := NewKeyManager(backupPath)
		if err := backupKeyManager.SaveAgeKey(keyPair, keyName); err != nil {
			return fmt.Errorf("failed to backup key %s: %w", keyName, err)
		}
	}

	return nil
}

// RestoreKeys restores age keys from a backup
func (k *KeyManager) RestoreKeys(backupPath string) error {
	backupKeyManager := NewKeyManager(backupPath)

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
		if err := k.SaveAgeKey(keyPair, keyName); err != nil {
			return fmt.Errorf("failed to restore key %s: %w", keyName, err)
		}
	}

	return nil
}

// GetKeyInfo returns information about a key
func (k *KeyManager) GetKeyInfo(keyName string) (*KeyInfo, error) {
	keyPair, err := k.LoadAgeKey(keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to load key: %w", err)
	}

	// Get file stats
	privateKeyPath := filepath.Join(k.keyDir, fmt.Sprintf("%s.txt", keyName))
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

// KeyInfo represents information about a key
type KeyInfo struct {
	Name      string    `json:"name"`
	PublicKey string    `json:"publicKey"`
	CreatedAt time.Time `json:"createdAt"`
	KeyType   string    `json:"keyType"`
	FilePath  string    `json:"filePath"`
}

// ValidateKeyAccess validates that a key can be used for encryption/decryption
func (k *KeyManager) ValidateKeyAccess(keyName string) error {
	// Load the key
	keyPair, err := k.LoadAgeKey(keyName)
	if err != nil {
		return fmt.Errorf("failed to load key: %w", err)
	}

	// Validate key format
	if err := k.ValidateAgeKey(keyPair.PrivateKey); err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	if err := k.ValidateAgeKey(keyPair.PublicKey); err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	// Test encryption/decryption
	testData := "test-encryption-data"

	// Create temporary files for testing
	tmpDir, err := os.MkdirTemp("", "sops-key-test-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(testFile, []byte(testData), 0o644); err != nil {
		return fmt.Errorf("failed to write test file: %w", err)
	}

	// Test encryption
	encryptor := NewEncryptor([]string{keyPair.PublicKey}, nil)
	encryptConfig := EncryptionConfig{
		AgeKeys: []string{keyPair.PublicKey},
		InPlace: true,
	}

	if err := encryptor.EncryptFile(context.Background(), testFile, encryptConfig); err != nil {
		return fmt.Errorf("key validation failed - encryption test: %w", err)
	}

	// Test decryption
	if err := encryptor.DecryptFile(context.Background(), testFile, ""); err != nil {
		return fmt.Errorf("key validation failed - decryption test: %w", err)
	}

	return nil
}

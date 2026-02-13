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
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultAgeExpirationDays is the default expiration period for Age keys (90 days)
	DefaultAgeExpirationDays = 90

	// DefaultSSHExpirationDays is the default expiration period for SSH keys (180 days)
	DefaultSSHExpirationDays = 180

	// RegistryFileName is the name of the key registry file
	RegistryFileName = "key-registry.yaml"
)

// DefaultKeyRegistry implements the KeyRegistry interface with SOPS encryption support.
type DefaultKeyRegistry struct {
	registryPath string
	encryptor    SOPSEncryptor
	logger       *slog.Logger
	mu           sync.RWMutex // Protects concurrent access to registry file
}

// SOPSEncryptor defines the interface for SOPS encryption/decryption operations.
type SOPSEncryptor interface {
	// EncryptFile encrypts a file using SOPS
	EncryptFile(ctx context.Context, filePath string) error

	// DecryptFile decrypts a SOPS-encrypted file and returns the content
	DecryptFile(ctx context.Context, filePath string) ([]byte, error)
}

// registryData represents the structure of the key registry file.
type registryData struct {
	Version            string                  `yaml:"version"`
	DefaultExpiration  defaultExpirationPolicy `yaml:"default_expiration"`
	Keys               []KeyEntry              `yaml:"keys"`
}

// defaultExpirationPolicy defines default expiration periods for different key types.
type defaultExpirationPolicy struct {
	AgeDays int `yaml:"age_days"`
	SSHDays int `yaml:"ssh_days"`
}

// NewDefaultKeyRegistry creates a new key registry with SOPS encryption.
// The registry file is stored at <registryPath>/key-registry.yaml and is encrypted with SOPS.
func NewDefaultKeyRegistry(registryPath string, encryptor SOPSEncryptor, logger *slog.Logger) *DefaultKeyRegistry {
	if logger == nil {
		logger = slog.Default()
	}

	return &DefaultKeyRegistry{
		registryPath: filepath.Join(registryPath, RegistryFileName),
		encryptor:    encryptor,
		logger:       logger,
	}
}

// RegisterKey adds a new key to the registry.
// Returns an error if a key with the same cluster and type already exists with active status.
// Validates: Requirements 4.7, 9.2 - Record creation timestamp and expiration date.
func (r *DefaultKeyRegistry) RegisterKey(ctx context.Context, entry KeyEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Info("Registering key", "cluster", entry.Cluster, "type", entry.KeyType, "fingerprint", entry.Fingerprint)

	// Load existing registry
	data, err := r.loadRegistry(ctx)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Check for duplicate active keys
	for _, existing := range data.Keys {
		if existing.Cluster == entry.Cluster &&
			existing.KeyType == entry.KeyType &&
			existing.Status == KeyStatusActive {
			return fmt.Errorf("active %s key already exists for cluster %s", entry.KeyType, entry.Cluster)
		}
	}

	// Set creation timestamp if not provided
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	// Calculate expiration date if not provided
	if entry.ExpiresAt.IsZero() {
		entry.ExpiresAt = r.calculateExpiration(entry.CreatedAt, entry.KeyType, data.DefaultExpiration)
	}

	// Set default status if not provided
	if entry.Status == "" {
		entry.Status = KeyStatusActive
	}

	// Add the new key
	data.Keys = append(data.Keys, entry)

	// Save the registry
	if err := r.saveRegistry(ctx, data); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	r.logger.Info("Successfully registered key",
		"cluster", entry.Cluster,
		"type", entry.KeyType,
		"fingerprint", entry.Fingerprint,
		"expires_at", entry.ExpiresAt.Format(time.RFC3339))

	return nil
}

// GetKey retrieves key metadata by cluster and type.
// Returns the active key for the specified cluster and type.
// Returns ErrKeyNotFound if no matching active key exists.
func (r *DefaultKeyRegistry) GetKey(ctx context.Context, cluster string, keyType KeyType) (*KeyEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.logger.Debug("Retrieving key", "cluster", cluster, "type", keyType)

	data, err := r.loadRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load registry: %w", err)
	}

	// Find the active key for the cluster and type
	for _, entry := range data.Keys {
		if entry.Cluster == cluster &&
			entry.KeyType == keyType &&
			entry.Status == KeyStatusActive {
			r.logger.Debug("Found key", "cluster", cluster, "type", keyType, "fingerprint", entry.Fingerprint)
			return &entry, nil
		}
	}

	return nil, NewKeyNotFoundError(cluster, keyType, nil)
}

// UpdateKeyStatus updates the status of a key.
// Returns ErrKeyNotFound if no matching key exists.
func (r *DefaultKeyRegistry) UpdateKeyStatus(ctx context.Context, cluster string, keyType KeyType, status KeyStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Info("Updating key status", "cluster", cluster, "type", keyType, "status", status)

	data, err := r.loadRegistry(ctx)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Find and update the key
	found := false
	for i := range data.Keys {
		if data.Keys[i].Cluster == cluster &&
			data.Keys[i].KeyType == keyType &&
			data.Keys[i].Status == KeyStatusActive {
			data.Keys[i].Status = status

			// Set revocation timestamp if revoking
			if status == KeyStatusRevoked && data.Keys[i].RevokedAt.IsZero() {
				data.Keys[i].RevokedAt = time.Now()
			}

			found = true
			break
		}
	}

	if !found {
		return NewKeyNotFoundError(cluster, keyType, nil)
	}

	// Save the registry
	if err := r.saveRegistry(ctx, data); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	r.logger.Info("Successfully updated key status", "cluster", cluster, "type", keyType, "status", status)
	return nil
}

// ListKeys returns all keys, optionally filtered by cluster.
// If cluster is empty, returns keys for all clusters.
func (r *DefaultKeyRegistry) ListKeys(ctx context.Context, cluster string) ([]KeyEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.logger.Debug("Listing keys", "cluster", cluster)

	data, err := r.loadRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load registry: %w", err)
	}

	// Filter by cluster if specified
	if cluster == "" {
		return data.Keys, nil
	}

	var filtered []KeyEntry
	for _, entry := range data.Keys {
		if entry.Cluster == cluster {
			filtered = append(filtered, entry)
		}
	}

	r.logger.Debug("Listed keys", "cluster", cluster, "count", len(filtered))
	return filtered, nil
}

// CheckExpiration returns keys that are expired or expiring soon.
// The warnDays parameter specifies the warning threshold in days.
// Validates: Requirements 4.1, 4.2, 4.3, 4.4 - Check expiration status and warn within 14 days.
func (r *DefaultKeyRegistry) CheckExpiration(ctx context.Context, warnDays int) (*ExpirationReport, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.logger.Debug("Checking key expiration", "warn_days", warnDays)

	data, err := r.loadRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load registry: %w", err)
	}

	report := &ExpirationReport{
		Expired: []KeyExpirationInfo{},
		Warning: []KeyExpirationInfo{},
		Valid:   []KeyExpirationInfo{},
	}

	now := time.Now()
	warnThreshold := now.AddDate(0, 0, warnDays)

	for _, entry := range data.Keys {
		// Only check active keys
		if entry.Status != KeyStatusActive {
			continue
		}

		daysRemaining := int(entry.ExpiresAt.Sub(now).Hours() / 24)

		info := KeyExpirationInfo{
			Cluster:       entry.Cluster,
			KeyType:       entry.KeyType,
			Fingerprint:   entry.Fingerprint,
			DaysRemaining: daysRemaining,
			ExpiresAt:     entry.ExpiresAt,
		}

		if entry.ExpiresAt.Before(now) {
			// Key has expired
			report.Expired = append(report.Expired, info)
		} else if entry.ExpiresAt.Before(warnThreshold) {
			// Key is expiring soon
			report.Warning = append(report.Warning, info)
		} else {
			// Key is valid
			report.Valid = append(report.Valid, info)
		}
	}

	r.logger.Info("Expiration check complete",
		"expired", len(report.Expired),
		"warning", len(report.Warning),
		"valid", len(report.Valid))

	return report, nil
}

// RebuildFromFiles reconstructs the registry from existing key files.
// This is useful when the registry is corrupted or missing.
// Validates: Requirement 9.8 - Rebuild registry from key files.
func (r *DefaultKeyRegistry) RebuildFromFiles(ctx context.Context, keysDir string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Info("Rebuilding registry from key files", "keys_dir", keysDir)

	// Create new registry data with default expiration policies
	data := &registryData{
		Version: "1.0",
		DefaultExpiration: defaultExpirationPolicy{
			AgeDays: DefaultAgeExpirationDays,
			SSHDays: DefaultSSHExpirationDays,
		},
		Keys: []KeyEntry{},
	}

	// Scan Age keys directory
	ageKeysDir := filepath.Join(keysDir, "age")
	if _, err := os.Stat(ageKeysDir); err == nil {
		if err := r.scanAgeKeys(ageKeysDir, data); err != nil {
			r.logger.Warn("Failed to scan Age keys", "error", err)
		}
	}

	// Scan SSH keys directory
	sshKeysDir := filepath.Join(keysDir, "ssh")
	if _, err := os.Stat(sshKeysDir); err == nil {
		if err := r.scanSSHKeys(sshKeysDir, data); err != nil {
			r.logger.Warn("Failed to scan SSH keys", "error", err)
		}
	}

	// Save the rebuilt registry
	if err := r.saveRegistry(ctx, data); err != nil {
		return fmt.Errorf("failed to save rebuilt registry: %w", err)
	}

	r.logger.Info("Successfully rebuilt registry", "key_count", len(data.Keys))
	return nil
}

// Private helper methods

// loadRegistry loads and decrypts the registry file.
func (r *DefaultKeyRegistry) loadRegistry(ctx context.Context) (*registryData, error) {
	// Check if registry file exists
	if _, err := os.Stat(r.registryPath); os.IsNotExist(err) {
		// Create new registry with default values
		r.logger.Info("Registry file does not exist, creating new registry")
		return &registryData{
			Version: "1.0",
			DefaultExpiration: defaultExpirationPolicy{
				AgeDays: DefaultAgeExpirationDays,
				SSHDays: DefaultSSHExpirationDays,
			},
			Keys: []KeyEntry{},
		}, nil
	}

	// Decrypt the registry file
	content, err := r.encryptor.DecryptFile(ctx, r.registryPath)
	if err != nil {
		return nil, NewRegistryCorruptedError(r.registryPath, fmt.Errorf("failed to decrypt: %w", err))
	}

	// Parse YAML
	var data registryData
	if err := yaml.Unmarshal(content, &data); err != nil {
		return nil, NewRegistryCorruptedError(r.registryPath, fmt.Errorf("failed to parse YAML: %w", err))
	}

	// Validate version
	if data.Version != "1.0" {
		return nil, NewRegistryCorruptedError(r.registryPath, fmt.Errorf("unsupported registry version: %s", data.Version))
	}

	return &data, nil
}

// saveRegistry encrypts and saves the registry file.
func (r *DefaultKeyRegistry) saveRegistry(ctx context.Context, data *registryData) error {
	// Ensure directory exists
	dir := filepath.Dir(r.registryPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create registry directory: %w", err)
	}

	// Marshal to YAML
	content, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	// Write to temporary file first
	tempPath := r.registryPath + ".tmp"
	if err := os.WriteFile(tempPath, content, 0o600); err != nil {
		return fmt.Errorf("failed to write temporary registry: %w", err)
	}

	// Encrypt the file
	if err := r.encryptor.EncryptFile(ctx, tempPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to encrypt registry: %w", err)
	}

	// Atomically replace the registry file
	if err := os.Rename(tempPath, r.registryPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to save registry: %w", err)
	}

	return nil
}

// calculateExpiration calculates the expiration date based on creation date and key type.
func (r *DefaultKeyRegistry) calculateExpiration(createdAt time.Time, keyType KeyType, policy defaultExpirationPolicy) time.Time {
	switch keyType {
	case KeyTypeAge:
		return createdAt.AddDate(0, 0, policy.AgeDays)
	case KeyTypeSSH:
		return createdAt.AddDate(0, 0, policy.SSHDays)
	default:
		// Default to Age expiration
		return createdAt.AddDate(0, 0, policy.AgeDays)
	}
}

// scanAgeKeys scans the Age keys directory and adds entries to the registry.
func (r *DefaultKeyRegistry) scanAgeKeys(dir string, data *registryData) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read Age keys directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Look for .pub files (public keys)
		if filepath.Ext(entry.Name()) != ".pub" {
			continue
		}

		// Extract cluster name from filename
		cluster := entry.Name()[:len(entry.Name())-4] // Remove .pub extension

		// Read public key
		pubKeyPath := filepath.Join(dir, entry.Name())
		pubKeyData, err := os.ReadFile(pubKeyPath)
		if err != nil {
			r.logger.Warn("Failed to read Age public key", "file", entry.Name(), "error", err)
			continue
		}

		publicKey := string(pubKeyData)

		// Get file info for creation time
		info, err := entry.Info()
		if err != nil {
			r.logger.Warn("Failed to get file info", "file", entry.Name(), "error", err)
			continue
		}

		createdAt := info.ModTime()
		expiresAt := r.calculateExpiration(createdAt, KeyTypeAge, data.DefaultExpiration)

		// Create key entry
		keyEntry := KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: publicKey, // Use public key as fingerprint for Age keys
			PublicKey:   publicKey,
			CreatedAt:   createdAt,
			ExpiresAt:   expiresAt,
			Status:      KeyStatusActive,
		}

		data.Keys = append(data.Keys, keyEntry)
		r.logger.Debug("Added Age key from file", "cluster", cluster, "created_at", createdAt)
	}

	return nil
}

// scanSSHKeys scans the SSH keys directory and adds entries to the registry.
func (r *DefaultKeyRegistry) scanSSHKeys(dir string, data *registryData) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read SSH keys directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Look for .pub files (public keys)
		if filepath.Ext(entry.Name()) != ".pub" {
			continue
		}

		// Extract cluster name from filename
		cluster := entry.Name()[:len(entry.Name())-4] // Remove .pub extension

		// Read public key
		pubKeyPath := filepath.Join(dir, entry.Name())
		pubKeyData, err := os.ReadFile(pubKeyPath)
		if err != nil {
			r.logger.Warn("Failed to read SSH public key", "file", entry.Name(), "error", err)
			continue
		}

		publicKey := string(pubKeyData)

		// Get file info for creation time
		info, err := entry.Info()
		if err != nil {
			r.logger.Warn("Failed to get file info", "file", entry.Name(), "error", err)
			continue
		}

		createdAt := info.ModTime()
		expiresAt := r.calculateExpiration(createdAt, KeyTypeSSH, data.DefaultExpiration)

		// Create key entry
		keyEntry := KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeSSH,
			Fingerprint: publicKey, // Use public key as fingerprint for SSH keys
			PublicKey:   publicKey,
			CreatedAt:   createdAt,
			ExpiresAt:   expiresAt,
			Status:      KeyStatusActive,
		}

		data.Keys = append(data.Keys, keyEntry)
		r.logger.Debug("Added SSH key from file", "cluster", cluster, "created_at", createdAt)
	}

	return nil
}

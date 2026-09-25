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
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
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
	// EncryptFile encrypts a file using explicit SOPS configuration.
	EncryptFile(ctx context.Context, filePath string, config sops.EncryptionConfig) error

	// DecryptFile decrypts a SOPS-encrypted file and returns the content
	DecryptFile(ctx context.Context, filePath string) ([]byte, error)

	// GetEncryptedContent returns the ciphertext and SOPS metadata.
	GetEncryptedContent(filePath string) (string, error)
}

// registryData represents the structure of the key registry file.
type registryData struct {
	Version              string                  `yaml:"version"`
	DefaultExpiration    defaultExpirationPolicy `yaml:"default_expiration"`
	Keys                 []KeyEntry              `yaml:"keys"`
	encryptionRecipients []string
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
//
// Uniqueness is enforced per (cluster, key type, fingerprint) across all
// statuses: the same key may not be registered twice, and a revoked or
// archived fingerprint may not be silently reinstated.
//
// Multiple ACTIVE keys per cluster and type are valid and expected. SOPS
// encrypts a file to many age recipients simultaneously, and dual-key rotation
// (see DefaultKeyRotator.RotateAgeKey and GetRotationStatus) requires the old
// and new keys to be active at the same time. Revocation likewise requires at
// least two active recipients so that revoking one cannot lock the cluster out.
//
// Validates: Requirements 4.7, 9.2 - Record creation timestamp and expiration date.
func (r *DefaultKeyRegistry) RegisterKey(ctx context.Context, entry KeyEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	registryLock, err := r.acquireRegistryLock(ctx)
	if err != nil {
		return err
	}
	defer r.releaseRegistryLock(registryLock)

	r.logger.Info("Registering key", "cluster", entry.Cluster, "type", entry.KeyType, "fingerprint", entry.Fingerprint)

	// Load existing registry
	data, err := r.loadRegistry(ctx)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Reject re-registering the same fingerprint, regardless of its status.
	for _, existing := range data.Keys {
		if existing.Cluster == entry.Cluster &&
			existing.KeyType == entry.KeyType &&
			existing.Fingerprint == entry.Fingerprint {
			return fmt.Errorf("%s key %s already exists for cluster %s with status %s",
				entry.KeyType, entry.Fingerprint, entry.Cluster, existing.Status)
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

	if entry.Primary && entry.Status != KeyStatusActive {
		return fmt.Errorf("cannot register inactive %s key %s as primary", entry.KeyType, entry.Fingerprint)
	}
	if entry.Primary {
		for _, existing := range data.Keys {
			if existing.Cluster == entry.Cluster && existing.KeyType == entry.KeyType && existing.Status == KeyStatusActive && existing.Primary {
				return fmt.Errorf("active primary %s key %s already exists for cluster %s", entry.KeyType, existing.Fingerprint, entry.Cluster)
			}
		}
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
// It delegates to the active primary key when one exists, falling back to the earliest-registered active key when no primary is set (legacy registries).
// Returns ErrKeyNotFound if no matching key exists.
func (r *DefaultKeyRegistry) GetKey(ctx context.Context, cluster string, keyType KeyType) (*KeyEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.logger.Debug("Retrieving key", "cluster", cluster, "type", keyType)

	data, err := r.loadRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load registry: %w", err)
	}

	return selectKey(data.Keys, cluster, keyType, false)
}

// GetPrimaryKey retrieves the active primary key for a cluster and type.
// Returns ErrKeyNotFound if no active primary exists.
func (r *DefaultKeyRegistry) GetPrimaryKey(ctx context.Context, cluster string, keyType KeyType) (*KeyEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data, err := r.loadRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load registry: %w", err)
	}

	return selectKey(data.Keys, cluster, keyType, true)
}

// SetPrimaryKey selects an exact existing active key and repairs the complete primary group in one transaction.
func (r *DefaultKeyRegistry) SetPrimaryKey(ctx context.Context, cluster string, keyType KeyType, fingerprint string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	registryLock, err := r.acquireRegistryLock(ctx)
	if err != nil {
		return err
	}
	defer r.releaseRegistryLock(registryLock)

	data, err := r.loadRegistry(ctx)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}
	candidate := -1
	for i := range data.Keys {
		entry := data.Keys[i]
		if entry.Cluster == cluster && entry.KeyType == keyType && entry.Fingerprint == fingerprint {
			if entry.Status != KeyStatusActive {
				return fmt.Errorf("cannot set inactive %s key %s as primary", keyType, fingerprint)
			}
			candidate = i
			break
		}
	}
	if candidate == -1 {
		return NewKeyNotFoundError(cluster, keyType, nil)
	}
	changed := false
	for i := range data.Keys {
		if data.Keys[i].Cluster != cluster || data.Keys[i].KeyType != keyType {
			continue
		}
		want := i == candidate
		if data.Keys[i].Primary != want {
			data.Keys[i].Primary = want
			changed = true
		}
	}
	if !changed {
		return nil
	}
	if err := r.saveRegistry(ctx, data); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}
	return nil
}

// ReplacePrimary atomically registers a new primary and clears the predecessor's primary role.
// The predecessor remains active so Age rotations retain dual-key decryption.
func (r *DefaultKeyRegistry) ReplacePrimary(ctx context.Context, oldFingerprint string, newEntry KeyEntry) error {
	return r.replacePrimary(ctx, oldFingerprint, newEntry, false)
}

// ReplacePrimaryAndArchive atomically registers a new primary and archives
// the predecessor. It is used for immediate SSH replacement, where retaining
// the old key as an active recipient would be incorrect.
func (r *DefaultKeyRegistry) ReplacePrimaryAndArchive(ctx context.Context, oldFingerprint string, newEntry KeyEntry) error {
	return r.replacePrimary(ctx, oldFingerprint, newEntry, true)
}

func (r *DefaultKeyRegistry) replacePrimary(ctx context.Context, oldFingerprint string, newEntry KeyEntry, archiveOld bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	registryLock, err := r.acquireRegistryLock(ctx)
	if err != nil {
		return err
	}
	defer r.releaseRegistryLock(registryLock)

	data, err := r.loadRegistry(ctx)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	for _, existing := range data.Keys {
		if existing.Cluster == newEntry.Cluster && existing.KeyType == newEntry.KeyType && existing.Fingerprint == newEntry.Fingerprint {
			return fmt.Errorf("%s key %s already exists for cluster %s with status %s", newEntry.KeyType, newEntry.Fingerprint, newEntry.Cluster, existing.Status)
		}
	}

	oldIndex := -1
	for i := range data.Keys {
		if data.Keys[i].Cluster == newEntry.Cluster && data.Keys[i].KeyType == newEntry.KeyType && data.Keys[i].Fingerprint == oldFingerprint {
			oldIndex = i
			break
		}
	}
	if oldIndex == -1 {
		return NewKeyNotFoundError(newEntry.Cluster, newEntry.KeyType, nil)
	}

	old := data.Keys[oldIndex]
	if old.Status != KeyStatusActive || !old.Primary {
		return fmt.Errorf("old %s key %s is not the current active primary for cluster %s", newEntry.KeyType, oldFingerprint, newEntry.Cluster)
	}
	if newEntry.Status == "" {
		newEntry.Status = KeyStatusActive
	}
	if newEntry.Status != KeyStatusActive {
		return fmt.Errorf("replacement %s key %s must be active", newEntry.KeyType, newEntry.Fingerprint)
	}

	if newEntry.CreatedAt.IsZero() {
		newEntry.CreatedAt = time.Now()
	}
	if newEntry.ExpiresAt.IsZero() {
		newEntry.ExpiresAt = r.calculateExpiration(newEntry.CreatedAt, newEntry.KeyType, data.DefaultExpiration)
	}
	newEntry.Primary = true
	for i := range data.Keys {
		if data.Keys[i].Cluster == newEntry.Cluster && data.Keys[i].KeyType == newEntry.KeyType {
			data.Keys[i].Primary = false
		}
	}
	if archiveOld {
		data.Keys[oldIndex].Status = KeyStatusArchived
		data.Keys[oldIndex].Primary = false
	}
	data.Keys = append(data.Keys, newEntry)

	if err := r.saveRegistry(ctx, data); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}
	return nil
}

// UpdateKeyStatus updates the status of a key.
// Returns ErrKeyNotFound if no matching key exists.
func (r *DefaultKeyRegistry) UpdateKeyStatus(ctx context.Context, cluster string, keyType KeyType, status KeyStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	registryLock, err := r.acquireRegistryLock(ctx)
	if err != nil {
		return err
	}
	defer r.releaseRegistryLock(registryLock)

	r.logger.Info("Updating key status", "cluster", cluster, "type", keyType, "status", status)

	data, err := r.loadRegistry(ctx)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// The legacy API has no fingerprint argument. It is safe only for an
	// unambiguous active group; callers must use UpdateKey for multiple keys.
	matches := make([]int, 0, 1)
	for i := range data.Keys {
		if data.Keys[i].Cluster == cluster && data.Keys[i].KeyType == keyType && data.Keys[i].Status == KeyStatusActive {
			matches = append(matches, i)
		}
	}
	if len(matches) == 0 {
		return NewKeyNotFoundError(cluster, keyType, nil)
	}
	if len(matches) > 1 {
		return fmt.Errorf("multiple active %s keys exist for cluster %s; use fingerprint-targeted UpdateKey", keyType, cluster)
	}
	index := matches[0]
	data.Keys[index].Status = status
	if status != KeyStatusActive {
		data.Keys[index].Primary = false
	}
	if status == KeyStatusRevoked && data.Keys[index].RevokedAt.IsZero() {
		data.Keys[index].RevokedAt = time.Now()
	}

	// Save the registry
	if err := r.saveRegistry(ctx, data); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	r.logger.Info("Successfully updated key status", "cluster", cluster, "type", keyType, "status", status)
	return nil
}

// UpdateKey updates an existing key entry.
// Matching is performed by cluster, key type, and fingerprint when available.
func (r *DefaultKeyRegistry) UpdateKey(ctx context.Context, entry KeyEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	registryLock, err := r.acquireRegistryLock(ctx)
	if err != nil {
		return err
	}
	defer r.releaseRegistryLock(registryLock)

	r.logger.Info("Updating key entry", "cluster", entry.Cluster, "type", entry.KeyType, "fingerprint", entry.Fingerprint)

	data, err := r.loadRegistry(ctx)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	matchIndex := -1
	for i := range data.Keys {
		if data.Keys[i].Cluster != entry.Cluster || data.Keys[i].KeyType != entry.KeyType {
			continue
		}
		if entry.Fingerprint != "" && data.Keys[i].Fingerprint == entry.Fingerprint {
			matchIndex = i
			break
		}
	}

	if matchIndex == -1 {
		return NewKeyNotFoundError(entry.Cluster, entry.KeyType, nil)
	}

	updated := mergeKeyEntry(data.Keys[matchIndex], entry)
	if updated.Primary && updated.Status != KeyStatusActive {
		if entry.Primary {
			return fmt.Errorf("cannot set inactive %s key %s as primary", entry.KeyType, entry.Fingerprint)
		}
		updated.Primary = false
	}
	data.Keys[matchIndex] = updated
	if data.Keys[matchIndex].Primary && data.Keys[matchIndex].Status == KeyStatusActive {
		for i, existing := range data.Keys {
			if i != matchIndex && existing.Cluster == entry.Cluster && existing.KeyType == entry.KeyType && existing.Status == KeyStatusActive && existing.Primary {
				return fmt.Errorf("active primary %s key %s already exists for cluster %s", entry.KeyType, existing.Fingerprint, entry.Cluster)
			}
		}
	}

	if err := r.saveRegistry(ctx, data); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	r.logger.Info("Successfully updated key entry", "cluster", entry.Cluster, "type", entry.KeyType, "fingerprint", entry.Fingerprint)
	return nil
}

// UpdateKeys applies multiple existing key updates in one locked transaction.
// All entries are validated and applied in memory before a single registry save,
// so a failed match or validation leaves the persisted registry unchanged.
func (r *DefaultKeyRegistry) UpdateKeys(ctx context.Context, entries []KeyEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	registryLock, err := r.acquireRegistryLock(ctx)
	if err != nil {
		return err
	}
	defer r.releaseRegistryLock(registryLock)

	if len(entries) == 0 {
		return nil
	}
	data, err := r.loadRegistry(ctx)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	updated := append([]KeyEntry(nil), data.Keys...)
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		identity := entry.Cluster + "\x00" + string(entry.KeyType) + "\x00" + entry.Fingerprint
		if _, duplicate := seen[identity]; duplicate {
			return fmt.Errorf("duplicate key update for %s key %s in cluster %s", entry.KeyType, entry.Fingerprint, entry.Cluster)
		}
		seen[identity] = struct{}{}

		matchIndex := -1
		for i := range updated {
			if updated[i].Cluster != entry.Cluster || updated[i].KeyType != entry.KeyType {
				continue
			}
			if entry.Fingerprint == "" || updated[i].Fingerprint == entry.Fingerprint {
				matchIndex = i
				break
			}
		}
		if matchIndex == -1 {
			return NewKeyNotFoundError(entry.Cluster, entry.KeyType, nil)
		}

		candidate := mergeKeyEntry(updated[matchIndex], entry)
		if candidate.Primary && candidate.Status != KeyStatusActive {
			return fmt.Errorf("cannot set inactive %s key %s as primary", candidate.KeyType, candidate.Fingerprint)
		}
		if candidate.Status == KeyStatusRevoked && candidate.RevokedAt.IsZero() {
			candidate.RevokedAt = time.Now()
		}
		updated[matchIndex] = candidate
	}

	for i, entry := range updated {
		if entry.Status != KeyStatusActive || !entry.Primary {
			continue
		}
		for j := i + 1; j < len(updated); j++ {
			other := updated[j]
			if other.Cluster == entry.Cluster && other.KeyType == entry.KeyType && other.Status == KeyStatusActive && other.Primary {
				return fmt.Errorf("active primary %s keys already exist for cluster %s", entry.KeyType, entry.Cluster)
			}
		}
	}

	data.Keys = updated
	if err := r.saveRegistry(ctx, data); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}
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
	registryLock, err := r.acquireRegistryLock(ctx)
	if err != nil {
		return err
	}
	defer r.releaseRegistryLock(registryLock)

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

// acquireRegistryLock obtains an adjacent OS-level lock while a mutating
// operation performs its complete load/mutate/save transaction.
func (r *DefaultKeyRegistry) acquireRegistryLock(ctx context.Context) (*os.File, error) {
	dir := filepath.Dir(r.registryPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create registry directory: %w", err)
	}
	lockPath := r.registryPath + ".lock"
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("failed to open registry lock: %w", err)
	}
	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return file, nil
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			file.Close()
			return nil, fmt.Errorf("failed to lock registry: %w", err)
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			file.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func (r *DefaultKeyRegistry) releaseRegistryLock(file *os.File) {
	if file == nil {
		return
	}
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	_ = file.Close()
}
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

	// Recover the exact recipients that currently protect the registry. This
	// keeps later saves independent of the caller's cwd or .sops.yaml.
	encryptedContent, err := r.encryptor.GetEncryptedContent(r.registryPath)
	if err != nil {
		return nil, NewRegistryCorruptedError(r.registryPath, fmt.Errorf("failed to read encrypted registry: %w", err))
	}
	var recipients []string
	if strings.Contains(encryptedContent, "sops:") {
		recipients, err = extractSOPSAgeRecipients([]byte(encryptedContent))
		if err != nil {
			return nil, NewRegistryCorruptedError(r.registryPath, fmt.Errorf("failed to read SOPS recipients: %w", err))
		}
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
	data.encryptionRecipients = recipients

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

	// Write to a unique temporary file first.
	tempFile, err := os.CreateTemp(dir, "."+filepath.Base(r.registryPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary registry: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	if err := tempFile.Chmod(0o600); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to set temporary registry permissions: %w", err)
	}
	if _, err := tempFile.Write(content); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write temporary registry: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary registry: %w", err)
	}

	// Always derive recipients from the post-mutation registry state. Envelope
	// recipients may include revoked or archived keys and must never be carried
	// forward. An empty active set is rejected before the old registry is
	// replaced, which is especially important during rebuilds.
	recipients, err := activeAgeRecipients(data.Keys)
	if err != nil {
		return err
	}
	if len(recipients) == 0 {
		return fmt.Errorf("cannot encrypt registry: no active Age recipients")
	}
	if err := r.encryptor.EncryptFile(ctx, tempPath, sops.EncryptionConfig{
		AgeKeys: recipients,
		InPlace: true,
	}); err != nil {
		return fmt.Errorf("failed to encrypt registry: %w", err)
	}

	// Atomically replace the registry file
	if err := os.Rename(tempPath, r.registryPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to save registry: %w", err)
	}

	return nil
}

func extractSOPSAgeRecipients(data []byte) ([]string, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if len(document.Content) == 0 {
		return nil, nil
	}
	root := document.Content[0]
	var sopsNode *yaml.Node
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("SOPS document is not a mapping")
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "sops" {
			sopsNode = root.Content[i+1]
			break
		}
	}
	if sopsNode == nil || sopsNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("SOPS metadata is missing")
	}
	var ageNode *yaml.Node
	for i := 0; i+1 < len(sopsNode.Content); i += 2 {
		if sopsNode.Content[i].Value == "age" {
			ageNode = sopsNode.Content[i+1]
			break
		}
	}
	if ageNode == nil || ageNode.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("SOPS age metadata is missing")
	}
	result := make([]string, 0, len(ageNode.Content))
	seen := make(map[string]struct{})
	for _, item := range ageNode.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(item.Content); i += 2 {
			if item.Content[i].Value != "recipient" {
				continue
			}
			recipient := strings.TrimSpace(item.Content[i+1].Value)
			if recipient == "" {
				return nil, fmt.Errorf("SOPS age recipient is empty")
			}
			if _, ok := seen[recipient]; !ok {
				seen[recipient] = struct{}{}
				result = append(result, recipient)
			}
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("SOPS age metadata has no recipients")
	}
	return result, nil
}

func canonicalAgeRecipient(entry KeyEntry) (string, error) {
	recipient := strings.TrimSpace(entry.PublicKey)
	if recipient == "" {
		recipient = strings.TrimSpace(entry.Fingerprint)
	}
	if recipient == "" {
		return "", fmt.Errorf("age key %q has no public key or fingerprint", entry.Fingerprint)
	}
	return recipient, nil
}

func activeAgeRecipients(keys []KeyEntry) ([]string, error) {
	result := make([]string, 0)
	seen := make(map[string]struct{})
	for _, key := range keys {
		if key.KeyType != KeyTypeAge || key.Status != KeyStatusActive {
			continue
		}
		recipient, err := canonicalAgeRecipient(key)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[recipient]; !ok {
			seen[recipient] = struct{}{}
			result = append(result, recipient)
		}
	}
	return result, nil
}

func selectKey(keys []KeyEntry, cluster string, keyType KeyType, primaryOnly bool) (*KeyEntry, error) {
	var primaries []KeyEntry
	for _, entry := range keys {
		if entry.Cluster == cluster && entry.KeyType == keyType && entry.Status == KeyStatusActive && entry.Primary {
			primaries = append(primaries, entry)
		}
	}
	if len(primaries) > 1 {
		fingerprints := make([]string, len(primaries))
		for i, entry := range primaries {
			fingerprints[i] = entry.Fingerprint
		}
		sort.Strings(fingerprints)
		return nil, fmt.Errorf("multiple active primary %s keys for cluster %s: %s", keyType, cluster, strings.Join(fingerprints, ", "))
	}
	if len(primaries) == 1 {
		entry := primaries[0]
		return &entry, nil
	}
	if primaryOnly {
		return nil, NewKeyNotFoundError(cluster, keyType, nil)
	}
	for _, entry := range keys {
		if entry.Cluster == cluster && entry.KeyType == keyType && entry.Status == KeyStatusActive {
			returnEntry := entry
			return &returnEntry, nil
		}
	}
	return nil, NewKeyNotFoundError(cluster, keyType, nil)
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

func mergeKeyEntry(existing, incoming KeyEntry) KeyEntry {
	if incoming.Cluster != "" {
		existing.Cluster = incoming.Cluster
	}
	if incoming.KeyType != "" {
		existing.KeyType = incoming.KeyType
	}
	if incoming.Fingerprint != "" {
		existing.Fingerprint = incoming.Fingerprint
	}
	if incoming.PublicKey != "" {
		existing.PublicKey = incoming.PublicKey
	}
	if !incoming.CreatedAt.IsZero() {
		existing.CreatedAt = incoming.CreatedAt
	}
	if !incoming.ExpiresAt.IsZero() {
		existing.ExpiresAt = incoming.ExpiresAt
	}
	if incoming.Status != "" {
		existing.Status = incoming.Status
	}
	if incoming.RotatedFrom != "" {
		existing.RotatedFrom = incoming.RotatedFrom
	}
	if !incoming.RevokedAt.IsZero() {
		existing.RevokedAt = incoming.RevokedAt
	}
	if incoming.RevokedBy != "" {
		existing.RevokedBy = incoming.RevokedBy
	}
	if incoming.RevokedReason != "" {
		existing.RevokedReason = incoming.RevokedReason
	}
	if incoming.UsedBy != nil {
		existing.UsedBy = incoming.UsedBy
	}
	if incoming.UserEmail != "" {
		existing.UserEmail = incoming.UserEmail
	}
	existing.Primary = incoming.Primary

	return existing
}

func (r *DefaultKeyRegistry) rebuiltKeyExists(data *registryData, candidate KeyEntry) bool {
	for _, existing := range data.Keys {
		if existing.Cluster == candidate.Cluster &&
			existing.KeyType == candidate.KeyType &&
			existing.Fingerprint == candidate.Fingerprint {
			return true
		}
	}
	return false
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

		publicKey := strings.TrimSpace(string(pubKeyData))
		if publicKey == "" {
			r.logger.Warn("Skipping empty Age public key", "file", entry.Name())
			continue
		}

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

		if r.rebuiltKeyExists(data, keyEntry) {
			r.logger.Warn("Skipping duplicate Age key during registry rebuild", "cluster", cluster, "fingerprint", publicKey, "file", entry.Name())
			continue
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

		publicKey := strings.TrimSpace(string(pubKeyData))
		if publicKey == "" {
			r.logger.Warn("Skipping empty SSH public key", "file", entry.Name())
			continue
		}

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

		if r.rebuiltKeyExists(data, keyEntry) {
			r.logger.Warn("Skipping duplicate SSH key during registry rebuild", "cluster", cluster, "fingerprint", publicKey, "file", entry.Name())
			continue
		}
		data.Keys = append(data.Keys, keyEntry)
		r.logger.Debug("Added SSH key from file", "cluster", cluster, "created_at", createdAt)
	}

	return nil
}

func canonicalAgeRecipientValues(publicKey, fingerprint string) (string, error) {
	return canonicalAgeRecipient(KeyEntry{PublicKey: publicKey, Fingerprint: fingerprint})
}

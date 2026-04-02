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
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/security"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"gopkg.in/yaml.v3"
)

// DefaultKeyRotator implements the KeyRotator interface.
// It handles key rotation operations with dual-key transition support,
// allowing for gradual migration from old to new keys.
type DefaultKeyRotator struct {
	registry       KeyRegistry
	secretsManager SecretsManager
	auditLogger    AuditLogger
	logger         *slog.Logger
}

// NewDefaultKeyRotator creates a new key rotator with the given dependencies.
//
// Parameters:
//   - registry: Key registry for tracking key metadata
//   - secretsManager: Secrets manager for re-encrypting manifests
//   - auditLogger: Logger for audit events (can be nil to disable audit logging)
//   - logger: Logger for operation tracking
//
// Returns:
//   - *DefaultKeyRotator: A new key rotator instance
func NewDefaultKeyRotator(
	registry KeyRegistry,
	secretsManager SecretsManager,
	auditLogger AuditLogger,
	logger *slog.Logger,
) *DefaultKeyRotator {
	if logger == nil {
		logger = slog.Default()
	}

	return &DefaultKeyRotator{
		registry:       registry,
		secretsManager: secretsManager,
		auditLogger:    auditLogger,
		logger:         logger,
	}
}

// RotateAgeKey generates a new Age key and re-encrypts secrets.
// The new key is added alongside the old key in dual-key mode to allow
// for gradual migration. Call CompleteRotation to finalize the rotation.
//
// Validates: Requirements 3.1, 3.2, 3.3, 3.7
// - Generate new Age key pair
// - Add new public key to .sops.yaml alongside old key
// - Re-encrypt all manifests with both keys
// - Archive old key with timestamp
func (r *DefaultKeyRotator) RotateAgeKey(ctx context.Context, opts RotateOptions) (*RotationResult, error) {
	r.logger.Info("Starting Age key rotation", "cluster", opts.Cluster, "dry_run", opts.DryRun)

	// Validate options
	if opts.KeyType != KeyTypeAge {
		return nil, fmt.Errorf("invalid key type for RotateAgeKey: %s", opts.KeyType)
	}

	// Get current key from registry
	oldKey, err := r.registry.GetKey(ctx, opts.Cluster, KeyTypeAge)
	if err != nil {
		return nil, fmt.Errorf("failed to get current Age key: %w", err)
	}

	// Check if rotation is already in progress
	status, err := r.GetRotationStatus(ctx, opts.Cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to check rotation status: %w", err)
	}

	if status.InProgress && !opts.Complete {
		return nil, &ErrRotationInProgress{
			Cluster: opts.Cluster,
			KeyType: KeyTypeAge,
		}
	}

	// If completing rotation, finalize it
	if opts.Complete {
		if !status.InProgress {
			return nil, fmt.Errorf("no rotation in progress for cluster %s", opts.Cluster)
		}
		return r.completeAgeKeyRotation(ctx, opts, status)
	}

	// Generate new Age key pair
	newPublicKey, _, err := r.generateAgeKey(ctx, opts.Cluster, opts.DryRun)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new Age key: %w", err)
	}

	result := &RotationResult{
		OldFingerprint:   oldKey.Fingerprint,
		NewFingerprint:   newPublicKey,
		ReencryptedFiles: []string{},
		DualKeyActive:    true,
	}

	// In dry-run mode, don't make changes
	if opts.DryRun {
		r.logger.Info("Would rotate Age key (dry-run)",
			"cluster", opts.Cluster,
			"old_fingerprint", oldKey.Fingerprint,
			"new_fingerprint", newPublicKey)
		return result, nil
	}

	// Create rollback manager for atomic operations
	rollbackMgr := NewRollbackManager(r.logger)

	// Get .sops.yaml path for backup
	cfg, configPath, err := r.secretsManager.(*DefaultSecretsManager).loadClusterConfig(ctx, opts.Cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to load cluster config: %w", err)
	}

	overlayPath, err := r.secretsManager.(*DefaultSecretsManager).getOverlayPath(configPath, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get overlay path: %w", err)
	}

	sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")

	// Backup .sops.yaml before modification
	if err := rollbackMgr.Backup(sopsConfigPath); err != nil {
		return nil, fmt.Errorf("failed to backup .sops.yaml: %w", err)
	}

	// Find and backup all manifest files before re-encryption
	manifestFiles, err := r.secretsManager.(*DefaultSecretsManager).findManifestFiles(overlayPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find manifest files: %w", err)
	}

	for _, manifestPath := range manifestFiles {
		if err := rollbackMgr.Backup(manifestPath); err != nil {
			r.logger.Warn("Failed to backup manifest", "path", manifestPath, "error", err)
			// Continue with other files
		}
	}

	// Update .sops.yaml with dual-key configuration
	if err := r.updateSOPSConfigDualKey(ctx, opts.Cluster, oldKey.PublicKey, newPublicKey); err != nil {
		rollbackMgr.Rollback()
		return nil, fmt.Errorf("failed to update .sops.yaml: %w", err)
	}

	// Re-encrypt all manifests with both keys
	reencryptedFiles, err := r.reencryptManifests(ctx, opts.Cluster, []string{oldKey.PublicKey, newPublicKey})
	if err != nil {
		// Rollback all changes
		r.logger.Error("Failed to re-encrypt manifests, rolling back all changes", "error", err)
		if rollbackErr := rollbackMgr.Rollback(); rollbackErr != nil {
			r.logger.Error("Rollback failed", "error", rollbackErr)
			return nil, fmt.Errorf("failed to re-encrypt manifests and rollback failed: %w (rollback error: %v)", err, rollbackErr)
		}
		return nil, fmt.Errorf("failed to re-encrypt manifests (changes rolled back): %w", err)
	}

	// Operation succeeded, clear backups
	rollbackMgr.Clear()

	result.ReencryptedFiles = reencryptedFiles

	// Archive old key
	archivedPath, err := r.archiveKey(ctx, opts.Cluster, KeyTypeAge, oldKey.Fingerprint)
	if err != nil {
		r.logger.Warn("Failed to archive old key", "error", err)
		// Don't fail the rotation if archiving fails
	} else {
		result.ArchivedKeyPath = archivedPath
	}

	// Register new key in registry
	newKeyEntry := KeyEntry{
		Cluster:     opts.Cluster,
		KeyType:     KeyTypeAge,
		Fingerprint: newPublicKey,
		PublicKey:   newPublicKey,
		CreatedAt:   time.Now(),
		Status:      KeyStatusActive,
		RotatedFrom: oldKey.Fingerprint,
	}

	if err := r.registry.RegisterKey(ctx, newKeyEntry); err != nil {
		r.logger.Warn("Failed to register new key in registry", "error", err)
		// Don't fail the rotation if registry update fails
	}

	// Update old key status to archived (but keep it active for dual-key mode)
	// We'll mark it as archived when rotation is completed

	r.logger.Info("Age key rotation initiated (dual-key mode active)",
		"cluster", opts.Cluster,
		"old_fingerprint", oldKey.Fingerprint,
		"new_fingerprint", newPublicKey,
		"reencrypted_files", len(reencryptedFiles))

	// Log audit event
	if r.auditLogger != nil {
		actor := r.getActor(ctx)
		if err := r.auditLogger.LogKeyRotated(ctx, actor, string(KeyTypeAge), opts.Cluster); err != nil {
			r.logger.Warn("Failed to log audit event", "error", err)
		}
	}

	return result, nil
}

// RotateSSHKey generates a new SSH key pair.
// Updates the config file with the new key paths and archives the old key.
//
// Validates: Requirements 3.5, 3.6
// - Generate new SSH key pair
// - Update config file with new key paths
// - Archive old SSH key
func (r *DefaultKeyRotator) RotateSSHKey(ctx context.Context, opts RotateOptions) (*RotationResult, error) {
	r.logger.Info("Starting SSH key rotation", "cluster", opts.Cluster, "dry_run", opts.DryRun)

	// Validate options
	if opts.KeyType != KeyTypeSSH {
		return nil, fmt.Errorf("invalid key type for RotateSSHKey: %s", opts.KeyType)
	}

	// Get current key from registry
	oldKey, err := r.registry.GetKey(ctx, opts.Cluster, KeyTypeSSH)
	if err != nil {
		return nil, fmt.Errorf("failed to get current SSH key: %w", err)
	}

	// Generate new SSH key pair
	newPublicKey, newPrivateKeyPath, err := r.generateSSHKey(ctx, opts.Cluster, opts.DryRun)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new SSH key: %w", err)
	}

	result := &RotationResult{
		OldFingerprint:   oldKey.Fingerprint,
		NewFingerprint:   newPublicKey,
		ReencryptedFiles: []string{}, // SSH keys don't require re-encryption
		DualKeyActive:    false,      // SSH rotation is immediate, no dual-key mode
	}

	// In dry-run mode, don't make changes
	if opts.DryRun {
		r.logger.Info("Would rotate SSH key (dry-run)",
			"cluster", opts.Cluster,
			"old_fingerprint", oldKey.Fingerprint,
			"new_fingerprint", newPublicKey)
		return result, nil
	}

	// Update config file with new SSH key paths
	if err := r.updateConfigSSHKey(ctx, opts.Cluster, newPrivateKeyPath); err != nil {
		return nil, fmt.Errorf("failed to update config file: %w", err)
	}

	// Archive old key
	archivedPath, err := r.archiveKey(ctx, opts.Cluster, KeyTypeSSH, oldKey.Fingerprint)
	if err != nil {
		r.logger.Warn("Failed to archive old SSH key", "error", err)
		// Don't fail the rotation if archiving fails
	} else {
		result.ArchivedKeyPath = archivedPath
	}

	// Register new key in registry
	newKeyEntry := KeyEntry{
		Cluster:     opts.Cluster,
		KeyType:     KeyTypeSSH,
		Fingerprint: newPublicKey,
		PublicKey:   newPublicKey,
		CreatedAt:   time.Now(),
		Status:      KeyStatusActive,
		RotatedFrom: oldKey.Fingerprint,
	}

	if err := r.registry.RegisterKey(ctx, newKeyEntry); err != nil {
		r.logger.Warn("Failed to register new SSH key in registry", "error", err)
		// Don't fail the rotation if registry update fails
	}

	// Update old key status to archived
	if err := r.registry.UpdateKeyStatus(ctx, opts.Cluster, KeyTypeSSH, KeyStatusArchived); err != nil {
		r.logger.Warn("Failed to update old SSH key status", "error", err)
	}

	r.logger.Info("SSH key rotation completed",
		"cluster", opts.Cluster,
		"old_fingerprint", oldKey.Fingerprint,
		"new_fingerprint", newPublicKey)

	// Log audit event
	if r.auditLogger != nil {
		actor := r.getActor(ctx)
		if err := r.auditLogger.LogKeyRotated(ctx, actor, string(KeyTypeSSH), opts.Cluster); err != nil {
			r.logger.Warn("Failed to log audit event", "error", err)
		}
	}

	return result, nil
}

// CompleteRotation removes the old key after dual-key period.
// Re-encrypts all manifests with only the new key and marks the old key as archived.
//
// Validates: Requirements 3.4
// - Remove old key from .sops.yaml
// - Re-encrypt manifests with new key only
// - Update registry with archived status for old key
func (r *DefaultKeyRotator) CompleteRotation(ctx context.Context, cluster string, keyType KeyType) error {
	r.logger.Info("Completing key rotation", "cluster", cluster, "key_type", keyType)

	// Only Age keys support dual-key rotation
	if keyType != KeyTypeAge {
		return fmt.Errorf("CompleteRotation only supports Age keys, got: %s", keyType)
	}

	// Check rotation status
	status, err := r.GetRotationStatus(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to check rotation status: %w", err)
	}

	if !status.InProgress {
		return fmt.Errorf("no rotation in progress for cluster %s", cluster)
	}

	if status.NewKey == nil {
		return fmt.Errorf("no new key found in rotation status")
	}

	// Create rollback manager for atomic operations
	rollbackMgr := NewRollbackManager(r.logger)

	// Get .sops.yaml path for backup
	cfg, configPath, err := r.secretsManager.(*DefaultSecretsManager).loadClusterConfig(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to load cluster config: %w", err)
	}

	overlayPath, err := r.secretsManager.(*DefaultSecretsManager).getOverlayPath(configPath, cfg)
	if err != nil {
		return fmt.Errorf("failed to get overlay path: %w", err)
	}

	sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")

	// Backup .sops.yaml before modification
	if err := rollbackMgr.Backup(sopsConfigPath); err != nil {
		return fmt.Errorf("failed to backup .sops.yaml: %w", err)
	}

	// Find and backup all manifest files before re-encryption
	manifestFiles, err := r.secretsManager.(*DefaultSecretsManager).findManifestFiles(overlayPath)
	if err != nil {
		return fmt.Errorf("failed to find manifest files: %w", err)
	}

	for _, manifestPath := range manifestFiles {
		if err := rollbackMgr.Backup(manifestPath); err != nil {
			r.logger.Warn("Failed to backup manifest", "path", manifestPath, "error", err)
			// Continue with other files
		}
	}

	// Update .sops.yaml to use only the new key
	if err := r.updateSOPSConfigSingleKey(ctx, cluster, status.NewKey.PublicKey); err != nil {
		rollbackMgr.Rollback()
		return fmt.Errorf("failed to update .sops.yaml: %w", err)
	}

	// Re-encrypt all manifests with only the new key
	reencryptedFiles, err := r.reencryptManifests(ctx, cluster, []string{status.NewKey.PublicKey})
	if err != nil {
		// Rollback all changes
		r.logger.Error("Failed to re-encrypt manifests, rolling back all changes", "error", err)
		if rollbackErr := rollbackMgr.Rollback(); rollbackErr != nil {
			r.logger.Error("Rollback failed", "error", rollbackErr)
			return fmt.Errorf("failed to re-encrypt manifests and rollback failed: %w (rollback error: %v)", err, rollbackErr)
		}
		return fmt.Errorf("failed to re-encrypt manifests (changes rolled back): %w", err)
	}

	// Operation succeeded, clear backups
	rollbackMgr.Clear()

	// Update old key status to archived
	if status.OldKey != nil {
		if err := r.registry.UpdateKeyStatus(ctx, cluster, KeyTypeAge, KeyStatusArchived); err != nil {
			r.logger.Warn("Failed to update old key status", "error", err)
		}
	}

	r.logger.Info("Key rotation completed",
		"cluster", cluster,
		"key_type", keyType,
		"reencrypted_files", len(reencryptedFiles))

	// Log audit event
	if r.auditLogger != nil {
		actor := r.getActor(ctx)
		if err := r.auditLogger.LogKeyRotated(ctx, actor, string(keyType), cluster); err != nil {
			r.logger.Warn("Failed to log audit event", "error", err)
		}
	}

	return nil
}

// GetRotationStatus returns the current rotation state.
// Checks if a dual-key rotation is in progress and returns details about
// the old and new keys.
func (r *DefaultKeyRotator) GetRotationStatus(ctx context.Context, cluster string) (*RotationStatus, error) {
	r.logger.Debug("Checking rotation status", "cluster", cluster)

	// List all Age keys for the cluster
	keys, err := r.registry.ListKeys(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	// Filter for active Age keys
	var activeAgeKeys []KeyEntry
	for _, key := range keys {
		if key.KeyType == KeyTypeAge && key.Status == KeyStatusActive {
			activeAgeKeys = append(activeAgeKeys, key)
		}
	}

	status := &RotationStatus{
		InProgress:    false,
		DualKeyActive: false,
		OldKey:        nil,
		NewKey:        nil,
		PendingFiles:  []string{},
	}

	// If we have exactly 2 active Age keys, rotation is in progress
	if len(activeAgeKeys) == 2 {
		status.InProgress = true
		status.DualKeyActive = true

		// Determine which is old and which is new based on creation time
		if activeAgeKeys[0].CreatedAt.Before(activeAgeKeys[1].CreatedAt) {
			status.OldKey = &activeAgeKeys[0]
			status.NewKey = &activeAgeKeys[1]
		} else {
			status.OldKey = &activeAgeKeys[1]
			status.NewKey = &activeAgeKeys[0]
		}

		r.logger.Debug("Rotation in progress (dual-key mode)",
			"cluster", cluster,
			"old_key", status.OldKey.Fingerprint,
			"new_key", status.NewKey.Fingerprint)
	} else if len(activeAgeKeys) == 1 {
		// Single active key, no rotation in progress
		status.NewKey = &activeAgeKeys[0]
		r.logger.Debug("No rotation in progress", "cluster", cluster)
	} else if len(activeAgeKeys) > 2 {
		// Unexpected state: more than 2 active keys
		r.logger.Warn("Unexpected number of active Age keys", "cluster", cluster, "count", len(activeAgeKeys))
	}

	return status, nil
}

// Private helper methods

// completeAgeKeyRotation finalizes an Age key rotation.
func (r *DefaultKeyRotator) completeAgeKeyRotation(ctx context.Context, opts RotateOptions, status *RotationStatus) (*RotationResult, error) {
	if status.NewKey == nil {
		return nil, fmt.Errorf("no new key found in rotation status")
	}

	result := &RotationResult{
		OldFingerprint:   "",
		NewFingerprint:   status.NewKey.Fingerprint,
		ReencryptedFiles: []string{},
		DualKeyActive:    false,
	}

	if status.OldKey != nil {
		result.OldFingerprint = status.OldKey.Fingerprint
	}

	// In dry-run mode, don't make changes
	if opts.DryRun {
		r.logger.Info("Would complete Age key rotation (dry-run)",
			"cluster", opts.Cluster,
			"new_fingerprint", status.NewKey.Fingerprint)
		return result, nil
	}

	// Complete the rotation
	if err := r.CompleteRotation(ctx, opts.Cluster, KeyTypeAge); err != nil {
		return nil, err
	}

	return result, nil
}

// generateAgeKey generates a new Age key pair.
// Returns the public key and the path to the private key file.
func (r *DefaultKeyRotator) generateAgeKey(ctx context.Context, cluster string, dryRun bool) (string, string, error) {
	r.logger.Debug("Generating new Age key", "cluster", cluster)

	if dryRun {
		return "age1placeholder...", "/path/to/new/key", nil
	}

	// Use the existing key manager to generate a new Age key
	// The key manager handles generation and storage
	sopsManager := r.secretsManager.(*DefaultSecretsManager).sopsManager
	keyManager := sopsManager.GetKeyManager()

	// Generate a new key for the cluster
	keyPair, err := keyManager.GenerateKeyForCluster(cluster)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate Age key: %w", err)
	}

	// Get the key file path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	keyPath := filepath.Join(homeDir, ".config", "opencenter", "clusters", cluster, "secrets", "age", fmt.Sprintf("%s_keys.txt", cluster))

	r.logger.Info("Generated new Age key", "cluster", cluster, "public_key", keyPair.PublicKey)
	return keyPair.PublicKey, keyPath, nil
}

// generateSSHKey generates a new SSH key pair.
// Returns the public key and the path to the private key file.
func (r *DefaultKeyRotator) generateSSHKey(ctx context.Context, cluster string, dryRun bool) (string, string, error) {
	r.logger.Debug("Generating new SSH key", "cluster", cluster)

	if dryRun {
		return "ssh-ed25519 AAAA...", "/path/to/new/ssh/key", nil
	}

	// Determine SSH key path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	sshDir := filepath.Join(homeDir, ".config", "opencenter", "clusters", cluster, "secrets", "ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return "", "", fmt.Errorf("failed to create SSH directory: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	keyPath := filepath.Join(sshDir, fmt.Sprintf("%s-ssh-%s", cluster, timestamp))

	// Generate SSH key using ssh-keygen
	cmd, err := security.GetDefaultCommandRunner().PrepareCommandContext(ctx, "ssh-keygen",
		"-t", "ed25519",
		"-f", keyPath,
		"-N", "", // No passphrase
		"-C", fmt.Sprintf("%s-cluster-key", cluster),
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to prepare ssh-keygen command: %w", err)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate SSH key: %w (output: %s)", err, string(output))
	}

	// Read the public key
	pubKeyData, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		return "", "", fmt.Errorf("failed to read public key: %w", err)
	}

	publicKey := strings.TrimSpace(string(pubKeyData))

	r.logger.Info("Generated new SSH key", "cluster", cluster, "key_path", keyPath)
	return publicKey, keyPath, nil
}

// updateSOPSConfigDualKey updates .sops.yaml with both old and new keys.
func (r *DefaultKeyRotator) updateSOPSConfigDualKey(ctx context.Context, cluster string, oldKey string, newKey string) error {
	r.logger.Debug("Updating .sops.yaml for dual-key mode", "cluster", cluster)

	// Get the .sops.yaml path from the cluster config
	cfg, configPath, err := r.secretsManager.(*DefaultSecretsManager).loadClusterConfig(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to load cluster config: %w", err)
	}

	// Get the overlay path where .sops.yaml is located
	overlayPath, err := r.secretsManager.(*DefaultSecretsManager).getOverlayPath(configPath, cfg)
	if err != nil {
		return fmt.Errorf("failed to get overlay path: %w", err)
	}

	sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")

	// Read existing .sops.yaml
	data, err := os.ReadFile(sopsConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read .sops.yaml: %w", err)
	}

	// Parse the SOPS config
	var sopsConfig struct {
		CreationRules []struct {
			PathRegex      string `yaml:"path_regex,omitempty"`
			EncryptedRegex string `yaml:"encrypted_regex,omitempty"`
			Age            string `yaml:"age,omitempty"`
		} `yaml:"creation_rules"`
	}

	if err := yaml.Unmarshal(data, &sopsConfig); err != nil {
		return fmt.Errorf("failed to parse .sops.yaml: %w", err)
	}

	// Update all Age keys to use both old and new keys (comma-separated)
	dualKey := fmt.Sprintf("%s,%s", oldKey, newKey)
	for i := range sopsConfig.CreationRules {
		if sopsConfig.CreationRules[i].Age != "" {
			sopsConfig.CreationRules[i].Age = dualKey
		}
	}

	// Write back the updated config
	updatedData, err := yaml.Marshal(&sopsConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal .sops.yaml: %w", err)
	}

	if err := os.WriteFile(sopsConfigPath, updatedData, 0o644); err != nil {
		return fmt.Errorf("failed to write .sops.yaml: %w", err)
	}

	r.logger.Info("Updated .sops.yaml for dual-key mode", "cluster", cluster, "path", sopsConfigPath)
	return nil
}

// updateSOPSConfigSingleKey updates .sops.yaml with only the specified key.
func (r *DefaultKeyRotator) updateSOPSConfigSingleKey(ctx context.Context, cluster string, key string) error {
	r.logger.Debug("Updating .sops.yaml for single-key mode", "cluster", cluster)

	// Get the .sops.yaml path from the cluster config
	cfg, configPath, err := r.secretsManager.(*DefaultSecretsManager).loadClusterConfig(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to load cluster config: %w", err)
	}

	// Get the overlay path where .sops.yaml is located
	overlayPath, err := r.secretsManager.(*DefaultSecretsManager).getOverlayPath(configPath, cfg)
	if err != nil {
		return fmt.Errorf("failed to get overlay path: %w", err)
	}

	sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")

	// Read existing .sops.yaml
	data, err := os.ReadFile(sopsConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read .sops.yaml: %w", err)
	}

	// Parse the SOPS config
	var sopsConfig struct {
		CreationRules []struct {
			PathRegex      string `yaml:"path_regex,omitempty"`
			EncryptedRegex string `yaml:"encrypted_regex,omitempty"`
			Age            string `yaml:"age,omitempty"`
		} `yaml:"creation_rules"`
	}

	if err := yaml.Unmarshal(data, &sopsConfig); err != nil {
		return fmt.Errorf("failed to parse .sops.yaml: %w", err)
	}

	// Update all Age keys to use only the specified key
	for i := range sopsConfig.CreationRules {
		if sopsConfig.CreationRules[i].Age != "" {
			sopsConfig.CreationRules[i].Age = key
		}
	}

	// Write back the updated config
	updatedData, err := yaml.Marshal(&sopsConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal .sops.yaml: %w", err)
	}

	if err := os.WriteFile(sopsConfigPath, updatedData, 0o644); err != nil {
		return fmt.Errorf("failed to write .sops.yaml: %w", err)
	}

	r.logger.Info("Updated .sops.yaml for single-key mode", "cluster", cluster, "path", sopsConfigPath)
	return nil
}

// reencryptManifests re-encrypts all manifests with the specified keys.
func (r *DefaultKeyRotator) reencryptManifests(ctx context.Context, cluster string, keys []string) ([]string, error) {
	r.logger.Debug("Re-encrypting manifests", "cluster", cluster, "key_count", len(keys))

	// Get the cluster config to find manifest files
	cfg, configPath, err := r.secretsManager.(*DefaultSecretsManager).loadClusterConfig(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to load cluster config: %w", err)
	}

	// Get the overlay path where manifests are located
	overlayPath, err := r.secretsManager.(*DefaultSecretsManager).getOverlayPath(configPath, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get overlay path: %w", err)
	}

	// Find all manifest files
	manifestFiles, err := r.secretsManager.(*DefaultSecretsManager).findManifestFiles(overlayPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find manifest files: %w", err)
	}

	r.logger.Info("Found manifests to re-encrypt", "count", len(manifestFiles))

	// Get the SOPS encryptor
	encryptor := r.secretsManager.(*DefaultSecretsManager).sopsManager.GetEncryptor()

	var reencryptedFiles []string
	var reencryptErrors []error

	// Re-encrypt each manifest file
	for _, manifestPath := range manifestFiles {
		r.logger.Debug("Re-encrypting manifest", "path", manifestPath)

		// Check if the file is already encrypted
		isEncrypted, err := r.secretsManager.(*DefaultSecretsManager).isManifestEncrypted(manifestPath)
		if err != nil {
			r.logger.Warn("Failed to check if manifest is encrypted", "path", manifestPath, "error", err)
			continue
		}

		if !isEncrypted {
			r.logger.Debug("Skipping non-encrypted manifest", "path", manifestPath)
			continue
		}

		// Create a temporary file for decryption
		tmpDecrypted := manifestPath + ".tmp.decrypted"
		defer os.Remove(tmpDecrypted)

		// Decrypt the file
		if err := encryptor.DecryptFile(ctx, manifestPath, tmpDecrypted); err != nil {
			reencryptErrors = append(reencryptErrors, fmt.Errorf("failed to decrypt %s: %w", manifestPath, err))
			continue
		}

		// Re-encrypt with the new keys
		encryptConfig := sops.EncryptionConfig{
			AgeKeys: keys,
			InPlace: false,
		}

		// Encrypt back to the original file
		if err := encryptor.EncryptFile(ctx, tmpDecrypted, encryptConfig); err != nil {
			reencryptErrors = append(reencryptErrors, fmt.Errorf("failed to re-encrypt %s: %w", manifestPath, err))
			continue
		}

		// Move the encrypted file back
		if err := os.Rename(tmpDecrypted+".enc", manifestPath); err != nil {
			reencryptErrors = append(reencryptErrors, fmt.Errorf("failed to move re-encrypted file %s: %w", manifestPath, err))
			continue
		}

		reencryptedFiles = append(reencryptedFiles, manifestPath)
		r.logger.Debug("Successfully re-encrypted manifest", "path", manifestPath)
	}

	if len(reencryptErrors) > 0 {
		return reencryptedFiles, fmt.Errorf("failed to re-encrypt %d files: %v", len(reencryptErrors), reencryptErrors[0])
	}

	r.logger.Info("Successfully re-encrypted manifests", "count", len(reencryptedFiles))
	return reencryptedFiles, nil
}

// updateConfigSSHKey updates the cluster config file with the new SSH key path.
func (r *DefaultKeyRotator) updateConfigSSHKey(ctx context.Context, cluster string, newKeyPath string) error {
	r.logger.Debug("Updating config with new SSH key", "cluster", cluster, "key_path", newKeyPath)

	// Get the cluster config path
	configPath, err := r.secretsManager.(*DefaultSecretsManager).getConfigPath(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	// Read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse the config
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Update the SSH key path in the config
	// The SSH key path is typically in secrets.ssh_private_key_file
	if secrets, ok := cfg["secrets"].(map[string]interface{}); ok {
		secrets["ssh_private_key_file"] = newKeyPath
		secrets["ssh_public_key_file"] = newKeyPath + ".pub"
	} else {
		// Create secrets section if it doesn't exist
		cfg["secrets"] = map[string]interface{}{
			"ssh_private_key_file": newKeyPath,
			"ssh_public_key_file":  newKeyPath + ".pub",
		}
	}

	// Write back the updated config
	updatedData, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config file: %w", err)
	}

	if err := os.WriteFile(configPath, updatedData, 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	r.logger.Info("Updated config with new SSH key", "cluster", cluster, "config_path", configPath)
	return nil
}

// archiveKey archives an old key with a timestamp.
func (r *DefaultKeyRotator) archiveKey(ctx context.Context, cluster string, keyType KeyType, fingerprint string) (string, error) {
	r.logger.Debug("Archiving key", "cluster", cluster, "key_type", keyType, "fingerprint", fingerprint)

	// Determine archive directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	archiveDir := filepath.Join(homeDir, ".config", "opencenter", "secrets", "archive")
	if err := os.MkdirAll(archiveDir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Generate archive filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	archiveFilename := fmt.Sprintf("%s-%s-%s.key", cluster, keyType, timestamp)
	archivePath := filepath.Join(archiveDir, archiveFilename)

	// Determine the source key file path based on key type
	var sourceKeyPath string
	if keyType == KeyTypeAge {
		sourceKeyPath = filepath.Join(homeDir, ".config", "opencenter", "clusters", cluster, "secrets", "age", fmt.Sprintf("%s_keys.txt", cluster))
	} else if keyType == KeyTypeSSH {
		// For SSH keys, we need to find the current key file
		// This is more complex as there might be multiple SSH keys
		// For now, we'll use a placeholder approach
		sshDir := filepath.Join(homeDir, ".config", "opencenter", "clusters", cluster, "secrets", "ssh")

		// Find SSH key files in the directory
		files, err := os.ReadDir(sshDir)
		if err != nil {
			return "", fmt.Errorf("failed to read SSH directory: %w", err)
		}

		// Find the most recent SSH key file (not .pub)
		var latestFile string
		var latestTime time.Time
		for _, file := range files {
			if !file.IsDir() && !strings.HasSuffix(file.Name(), ".pub") {
				info, err := file.Info()
				if err != nil {
					continue
				}
				if latestFile == "" || info.ModTime().After(latestTime) {
					latestFile = file.Name()
					latestTime = info.ModTime()
				}
			}
		}

		if latestFile == "" {
			return "", fmt.Errorf("no SSH key file found to archive")
		}

		sourceKeyPath = filepath.Join(sshDir, latestFile)
	} else {
		return "", fmt.Errorf("unsupported key type: %s", keyType)
	}

	// Check if source key exists
	if _, err := os.Stat(sourceKeyPath); os.IsNotExist(err) {
		r.logger.Warn("Source key file does not exist, skipping archive", "path", sourceKeyPath)
		return "", nil
	}

	// Copy the key file to the archive location
	sourceData, err := os.ReadFile(sourceKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read source key file: %w", err)
	}

	if err := os.WriteFile(archivePath, sourceData, 0o600); err != nil {
		return "", fmt.Errorf("failed to write archive file: %w", err)
	}

	// Also archive the public key if it exists (for SSH)
	if keyType == KeyTypeSSH {
		pubKeyPath := sourceKeyPath + ".pub"
		if _, err := os.Stat(pubKeyPath); err == nil {
			pubArchivePath := archivePath + ".pub"
			pubData, err := os.ReadFile(pubKeyPath)
			if err == nil {
				os.WriteFile(pubArchivePath, pubData, 0o644)
			}
		}
	}

	r.logger.Info("Key archived", "cluster", cluster, "key_type", keyType, "archive_path", archivePath)
	return archivePath, nil
}

// getActor retrieves the actor (user) from context or returns a default value.
func (r *DefaultKeyRotator) getActor(ctx context.Context) string {
	if actor, ok := ctx.Value("actor").(string); ok && actor != "" {
		return actor
	}
	// Try to get current user
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return "system"
}

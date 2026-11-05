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

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
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

	// Get the current primary key from the registry.
	oldKey, err := r.registry.GetPrimaryKey(ctx, opts.Cluster, KeyTypeAge)
	if err != nil {
		if IsKeyNotFoundError(err) {
			return nil, fmt.Errorf("no active primary Age key found for cluster %q; select one explicitly with 'opencenter secrets keys set-primary --cluster %s --type age --fingerprint <fingerprint>': %w", opts.Cluster, opts.Cluster, err)
		}
		return nil, fmt.Errorf("failed to get current Age primary key: %w", err)
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

	oldRecipient, err := canonicalAgeRecipient(*oldKey)
	if err != nil {
		return nil, fmt.Errorf("invalid current Age key: %w", err)
	}
	newRecipient, err := canonicalAgeRecipientValues(newPublicKey, newPublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid generated Age key: %w", err)
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
			if rollbackErr := rollbackMgr.Rollback(); rollbackErr != nil {
				return nil, fmt.Errorf("failed to backup manifest %s and rollback failed: %w (rollback error: %v)", manifestPath, err, rollbackErr)
			}
			return nil, fmt.Errorf("failed to backup manifest %s: %w", manifestPath, err)
		}
	}

	// Update .sops.yaml with every active recipient and the new key.
	if err := r.updateSOPSConfigDualKey(ctx, opts.Cluster, oldRecipient, newRecipient); err != nil {
		rollbackErr := rollbackMgr.Rollback()
		if rollbackErr != nil {
			return nil, fmt.Errorf("failed to update .sops.yaml: %w (rollback error: %v)", err, rollbackErr)
		}
		return nil, fmt.Errorf("failed to update .sops.yaml: %w", err)
	}

	// Re-encrypt all manifests with the complete recipient set.
	recipientKeys, err := r.activeRecipientSet(ctx, opts.Cluster, []string{oldRecipient, newRecipient}, nil)
	if err != nil {
		rollbackErr := rollbackMgr.Rollback()
		if rollbackErr != nil {
			return nil, fmt.Errorf("failed to compute SOPS recipients: %w (rollback error: %v)", err, rollbackErr)
		}
		return nil, fmt.Errorf("failed to compute SOPS recipients: %w", err)
	}
	reencryptedFiles, err := r.reencryptManifests(ctx, opts.Cluster, recipientKeys)
	if err != nil {
		r.logger.Error("Failed to re-encrypt manifests, rolling back all changes", "error", err)
		if rollbackErr := rollbackMgr.Rollback(); rollbackErr != nil {
			r.logger.Error("Rollback failed", "error", rollbackErr)
			return nil, fmt.Errorf("failed to re-encrypt manifests and rollback failed: %w (rollback error: %v)", err, rollbackErr)
		}
		return nil, fmt.Errorf("failed to re-encrypt manifests (changes rolled back): %w", err)
	}

	newKeyEntry := KeyEntry{
		Cluster:     opts.Cluster,
		KeyType:     KeyTypeAge,
		Fingerprint: newPublicKey,
		PublicKey:   newPublicKey,
		CreatedAt:   time.Now(),
		Status:      KeyStatusActive,
		RotatedFrom: oldKey.Fingerprint,
		Primary:     true,
	}
	if err := r.registry.ReplacePrimary(ctx, oldKey.Fingerprint, newKeyEntry); err != nil {
		if rollbackErr := rollbackMgr.Rollback(); rollbackErr != nil {
			return nil, fmt.Errorf("failed to update key registry: %w (rollback error: %v)", err, rollbackErr)
		}
		return nil, fmt.Errorf("failed to update key registry (changes rolled back): %w", err)
	}

	// Archiving copies a convenience backup only; registry and encrypted files are authoritative.
	archivedPath, err := r.archiveKey(ctx, opts.Cluster, KeyTypeAge, oldKey.Fingerprint)
	if err != nil {
		r.logger.Warn("Failed to archive old key", "error", err)
	} else {
		result.ArchivedKeyPath = archivedPath
	}

	// Clear backups only after filesystem and registry state are consistent.
	rollbackMgr.Clear()
	result.ReencryptedFiles = reencryptedFiles

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
			// Audit logging is observability-only; it must not invalidate a committed state.
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

	// Get the current primary key from the registry.
	oldKey, err := r.registry.GetPrimaryKey(ctx, opts.Cluster, KeyTypeSSH)
	if err != nil {
		if IsKeyNotFoundError(err) {
			return nil, fmt.Errorf("no active primary SSH key found for cluster %q; select one explicitly with 'opencenter secrets keys set-primary --cluster %s --type ssh --fingerprint <fingerprint>': %w", opts.Cluster, opts.Cluster, err)
		}
		return nil, fmt.Errorf("failed to get current SSH primary key: %w", err)
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

	rollbackMgr := NewRollbackManager(r.logger)
	configPath, err := r.secretsManager.(*DefaultSecretsManager).getConfigPath(ctx, opts.Cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}
	if err := rollbackMgr.Backup(configPath); err != nil {
		return nil, fmt.Errorf("failed to backup config file: %w", err)
	}

	// Update config file with new SSH key paths.
	if err := r.updateConfigSSHKey(ctx, opts.Cluster, newPrivateKeyPath); err != nil {
		return nil, fmt.Errorf("failed to update config file: %w", err)
	}

	newKeyEntry := KeyEntry{
		Cluster:     opts.Cluster,
		KeyType:     KeyTypeSSH,
		Fingerprint: newPublicKey,
		PublicKey:   newPublicKey,
		CreatedAt:   time.Now(),
		Status:      KeyStatusActive,
		RotatedFrom: oldKey.Fingerprint,
		Primary:     true,
	}
	if err := r.registry.ReplacePrimaryAndArchive(ctx, oldKey.Fingerprint, newKeyEntry); err != nil {
		if rollbackErr := rollbackMgr.Rollback(); rollbackErr != nil {
			return nil, fmt.Errorf("failed to atomically replace SSH key registry and rollback filesystem failed (registry unchanged): %w (rollback error: %v)", err, rollbackErr)
		}
		return nil, fmt.Errorf("failed to atomically replace SSH key registry (filesystem changes rolled back; registry unchanged): %w", err)
	}

	// Archiving copies a convenience backup only; registry and config are authoritative.
	archivedPath, err := r.archiveKey(ctx, opts.Cluster, KeyTypeSSH, oldKey.Fingerprint)
	if err != nil {
		r.logger.Warn("Failed to archive old SSH key", "error", err)
	} else {
		result.ArchivedKeyPath = archivedPath
	}

	rollbackMgr.Clear()

	r.logger.Info("SSH key rotation completed",
		"cluster", opts.Cluster,
		"old_fingerprint", oldKey.Fingerprint,
		"new_fingerprint", newPublicKey)

	// Log audit event
	if r.auditLogger != nil {
		actor := r.getActor(ctx)
		if err := r.auditLogger.LogKeyRotated(ctx, actor, string(KeyTypeSSH), opts.Cluster); err != nil {
			// Audit logging is observability-only; it must not invalidate a committed state.
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

	if status.NewKey == nil || status.OldKey == nil {
		return fmt.Errorf("rotation status is missing old or new key")
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
			if rollbackErr := rollbackMgr.Rollback(); rollbackErr != nil {
				return fmt.Errorf("failed to backup manifest %s and rollback failed: %w (rollback error: %v)", manifestPath, err, rollbackErr)
			}
			return fmt.Errorf("failed to backup manifest %s: %w", manifestPath, err)
		}
	}

	newRecipient, err := canonicalAgeRecipient(*status.NewKey)
	if err != nil {
		_ = rollbackMgr.Rollback()
		return fmt.Errorf("invalid successor Age key: %w", err)
	}
	oldRecipient, err := canonicalAgeRecipient(*status.OldKey)
	if err != nil {
		_ = rollbackMgr.Rollback()
		return fmt.Errorf("invalid predecessor Age key: %w", err)
	}
	// Update .sops.yaml while retaining unrelated active recipients.
	if err := r.updateSOPSConfigSingleKey(ctx, cluster, newRecipient, oldRecipient); err != nil {
		rollbackErr := rollbackMgr.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("failed to update .sops.yaml: %w (rollback error: %v)", err, rollbackErr)
		}
		return fmt.Errorf("failed to update .sops.yaml: %w", err)
	}

	recipientKeys, err := r.activeRecipientSet(ctx, cluster, []string{newRecipient}, map[string]struct{}{oldRecipient: {}})
	if err != nil {
		rollbackErr := rollbackMgr.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("failed to compute SOPS recipients: %w (rollback error: %v)", err, rollbackErr)
		}
		return fmt.Errorf("failed to compute SOPS recipients: %w", err)
	}
	// Re-encrypt all manifests with the complete post-rotation recipient set.
	reencryptedFiles, err := r.reencryptManifests(ctx, cluster, recipientKeys)
	if err != nil {
		r.logger.Error("Failed to re-encrypt manifests, rolling back all changes", "error", err)
		if rollbackErr := rollbackMgr.Rollback(); rollbackErr != nil {
			r.logger.Error("Rollback failed", "error", rollbackErr)
			return fmt.Errorf("failed to re-encrypt manifests and rollback failed: %w (rollback error: %v)", err, rollbackErr)
		}
		return fmt.Errorf("failed to re-encrypt manifests (changes rolled back): %w", err)
	}

	status.OldKey.Status = KeyStatusArchived
	status.OldKey.Primary = false
	if err := r.registry.UpdateKey(ctx, *status.OldKey); err != nil {
		if rollbackErr := rollbackMgr.Rollback(); rollbackErr != nil {
			return fmt.Errorf("failed to archive predecessor in key registry: %w (rollback error: %v)", err, rollbackErr)
		}
		return fmt.Errorf("failed to archive predecessor in key registry (changes rolled back): %w", err)
	}

	// Clear backups only after filesystem and registry state are consistent.
	rollbackMgr.Clear()

	r.logger.Info("Key rotation completed",
		"cluster", cluster,
		"key_type", keyType,
		"reencrypted_files", len(reencryptedFiles))

	// Log audit event
	if r.auditLogger != nil {
		actor := r.getActor(ctx)
		if err := r.auditLogger.LogKeyRotated(ctx, actor, string(keyType), cluster); err != nil {
			// Audit logging is observability-only; it must not invalidate a committed state.
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

	activeByFingerprint := make(map[string]KeyEntry, len(activeAgeKeys))
	for _, key := range activeAgeKeys {
		activeByFingerprint[key.Fingerprint] = key
	}

	// Rotation state is represented by an active successor pointing to an active predecessor.
	var candidates []KeyEntry
	for _, key := range activeAgeKeys {
		if key.RotatedFrom == "" {
			continue
		}
		if _, ok := activeByFingerprint[key.RotatedFrom]; ok {
			candidates = append(candidates, key)
		}
	}

	if len(candidates) > 0 {
		candidate := candidates[0]
		if len(candidates) > 1 {
			primaryCount := 0
			for _, key := range candidates {
				if key.Primary {
					candidate = key
					primaryCount++
				}
			}
			if primaryCount == 0 || primaryCount > 1 {
				conflicts := make([]string, 0, len(candidates))
				for _, key := range candidates {
					conflicts = append(conflicts, fmt.Sprintf("%s<- %s", key.Fingerprint, key.RotatedFrom))
				}
				return nil, fmt.Errorf("multiple active Age rotation chains for cluster %s: %s", cluster, strings.Join(conflicts, ", "))
			}
		}
		oldKey := activeByFingerprint[candidate.RotatedFrom]
		status.InProgress = true
		status.DualKeyActive = true
		status.OldKey = &oldKey
		status.NewKey = &candidate
		r.logger.Debug("Rotation in progress (dual-key mode)",
			"cluster", cluster,
			"old_key", oldKey.Fingerprint,
			"new_key", candidate.Fingerprint)
		return status, nil
	}

	// Unrelated active recipients are normal. Only expose the primary as NewKey.
	primary, err := r.registry.GetPrimaryKey(ctx, cluster, KeyTypeAge)
	if err != nil {
		if !IsKeyNotFoundError(err) {
			return nil, fmt.Errorf("failed to get primary Age key: %w", err)
		}
	} else {
		status.NewKey = primary
	}
	r.logger.Debug("No rotation in progress", "cluster", cluster)
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

// activeRecipientSet returns the full set of Age recipients that must be able to
// decrypt after the operation: the requested additions first, then every active
// recipient recorded in the registry, minus any explicit removals.
//
// Callers pass this to SOPS as the complete --age list. Ordering here only
// affects the command line; the on-disk .sops.yaml ordering is preserved
// separately by rewriteSOPSAgeValues, which reorders against the existing file.
func (r *DefaultKeyRotator) activeRecipientSet(ctx context.Context, cluster string, additions []string, removals map[string]struct{}) ([]string, error) {
	keys, err := r.registry.ListKeys(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}
	active := make(map[string]struct{})
	var ordered []string
	for _, key := range keys {
		if key.KeyType != KeyTypeAge || key.Status != KeyStatusActive {
			continue
		}
		recipient, err := canonicalAgeRecipient(key)
		if err != nil {
			return nil, fmt.Errorf("invalid active Age key: %w", err)
		}
		active[recipient] = struct{}{}
	}
	for _, recipient := range additions {
		if recipient != "" {
			active[recipient] = struct{}{}
		}
	}
	for _, recipient := range additions {
		if recipient == "" {
			continue
		}
		if _, removed := removals[recipient]; removed {
			continue
		}
		if _, exists := active[recipient]; exists {
			ordered = appendUniqueRecipient(ordered, recipient)
		}
	}
	for _, key := range keys {
		if key.KeyType != KeyTypeAge || key.Status != KeyStatusActive {
			continue
		}
		recipient, err := canonicalAgeRecipient(key)
		if err != nil {
			return nil, fmt.Errorf("invalid active Age key: %w", err)
		}
		if _, removed := removals[recipient]; removed {
			continue
		}
		ordered = appendUniqueRecipient(ordered, recipient)
	}
	return ordered, nil
}

func appendUniqueRecipient(recipients []string, recipient string) []string {
	for _, existing := range recipients {
		if existing == recipient {
			return recipients
		}
	}
	return append(recipients, recipient)
}

// updateSOPSConfigDualKey adds the successor only to creation rules containing the old primary recipient.
func (r *DefaultKeyRotator) updateSOPSConfigDualKey(ctx context.Context, cluster string, oldKey string, newKey string) error {
	var err error
	oldKey, err = canonicalAgeRecipientValues(oldKey, oldKey)
	if err != nil {
		return err
	}
	newKey, err = canonicalAgeRecipientValues(newKey, newKey)
	if err != nil {
		return err
	}
	r.logger.Debug("Updating .sops.yaml for dual-key mode", "cluster", cluster)
	cfg, configPath, err := r.secretsManager.(*DefaultSecretsManager).loadClusterConfig(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to load cluster config: %w", err)
	}
	overlayPath, err := r.secretsManager.(*DefaultSecretsManager).getOverlayPath(configPath, cfg)
	if err != nil {
		return fmt.Errorf("failed to get overlay path: %w", err)
	}
	sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")
	data, err := os.ReadFile(sopsConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read .sops.yaml: %w", err)
	}
	updatedData, err := rewriteSOPSAgeValues(data, func(existing []string) ([]string, error) {
		oldPresent := false
		for _, recipient := range existing {
			if recipient == oldKey {
				oldPresent = true
				break
			}
		}
		ordered := make([]string, 0, len(existing)+1)
		for _, recipient := range existing {
			ordered = appendUniqueRecipient(ordered, recipient)
		}
		if oldPresent {
			ordered = appendUniqueRecipient(ordered, newKey)
		}
		return ordered, nil
	})
	if err != nil {
		return fmt.Errorf("failed to update SOPS YAML: %w", err)
	}
	if err := os.WriteFile(sopsConfigPath, updatedData, 0o644); err != nil {
		return fmt.Errorf("failed to write .sops.yaml: %w", err)
	}
	r.logger.Info("Updated .sops.yaml for dual-key mode", "cluster", cluster, "path", sopsConfigPath)
	return nil
}

// updateSOPSConfigSingleKey updates .sops.yaml with all active recipients except the retired key.
// The optional retired argument preserves compatibility with direct callers; when omitted,
// RotatedFrom on the new registry entry identifies the predecessor.
func (r *DefaultKeyRotator) updateSOPSConfigSingleKey(ctx context.Context, cluster string, key string, retired ...string) error {
	var err error
	key, err = canonicalAgeRecipientValues(key, key)
	if err != nil {
		return err
	}
	r.logger.Debug("Updating .sops.yaml for single-key mode", "cluster", cluster)
	cfg, configPath, err := r.secretsManager.(*DefaultSecretsManager).loadClusterConfig(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to load cluster config: %w", err)
	}
	overlayPath, err := r.secretsManager.(*DefaultSecretsManager).getOverlayPath(configPath, cfg)
	if err != nil {
		return fmt.Errorf("failed to get overlay path: %w", err)
	}
	sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")
	data, err := os.ReadFile(sopsConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read .sops.yaml: %w", err)
	}
	keys, err := r.registry.ListKeys(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}
	retiredKey := ""
	if len(retired) > 0 {
		retiredKey = retired[0]
	} else {
		for _, entry := range keys {
			if entry.KeyType != KeyTypeAge || entry.Status != KeyStatusActive {
				continue
			}
			recipient, recipientErr := canonicalAgeRecipient(entry)
			if recipientErr != nil {
				return recipientErr
			}
			if recipient == key {
				retiredKey = entry.RotatedFrom
				break
			}
		}
	}
	if retiredKey != "" {
		retiredKey, err = canonicalAgeRecipientValues(retiredKey, retiredKey)
		if err != nil {
			return err
		}
	}
	updatedData, err := rewriteSOPSAgeValues(data, func(existing []string) ([]string, error) {
		retiredPresent := false
		ordered := make([]string, 0, len(existing)+1)
		for _, recipient := range existing {
			if recipient == retiredKey {
				retiredPresent = true
				continue
			}
			ordered = appendUniqueRecipient(ordered, recipient)
		}
		if retiredPresent {
			ordered = appendUniqueRecipient(ordered, key)
		}
		return ordered, nil
	})
	if err != nil {
		return fmt.Errorf("failed to update SOPS YAML: %w", err)
	}
	if err := os.WriteFile(sopsConfigPath, updatedData, 0o644); err != nil {
		return fmt.Errorf("failed to write .sops.yaml: %w", err)
	}
	r.logger.Info("Updated .sops.yaml for single-key mode", "cluster", cluster, "path", sopsConfigPath)
	return nil
}

// reencryptManifests re-encrypts all manifests with the specified keys.
func (r *DefaultKeyRotator) reencryptManifests(ctx context.Context, cluster string, keys []string) ([]string, error) {
	// Recipients are selected by each post-mutation creation rule, not by a
	// cluster-wide recipient set.
	_ = keys
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

		if err := reencryptManifestUsingCreationRule(ctx, encryptor, manifestPath, filepath.Join(overlayPath, ".sops.yaml")); err != nil {
			reencryptErrors = append(reencryptErrors, err)
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
	updatedData, err = v2.SanitizePublicYAML(updatedData)
	if err != nil {
		return fmt.Errorf("failed to sanitize config file: %w", err)
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
				_ = os.WriteFile(pubArchivePath, pubData, 0o644)
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

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
	"time"

	"github.com/rackerlabs/opencenter-cli/internal/sops"
	"gopkg.in/yaml.v3"
)

// DefaultKeyRevoker implements the KeyRevoker interface.
// It handles key revocation operations for users and compromised keys,
// with support for emergency revocation that immediately generates a new primary key.
type DefaultKeyRevoker struct {
	registry       KeyRegistry
	rotator        KeyRotator
	secretsManager SecretsManager
	auditLogger    AuditLogger
	logger         *slog.Logger
}

// NewDefaultKeyRevoker creates a new key revoker with the given dependencies.
//
// Parameters:
//   - registry: Key registry for tracking key metadata
//   - rotator: Key rotator for generating new keys during emergency revocation
//   - secretsManager: Secrets manager for re-encrypting manifests
//   - auditLogger: Logger for audit events (can be nil to disable audit logging)
//   - logger: Logger for operation tracking
//
// Returns:
//   - *DefaultKeyRevoker: A new key revoker instance
func NewDefaultKeyRevoker(
	registry KeyRegistry,
	rotator KeyRotator,
	secretsManager SecretsManager,
	auditLogger AuditLogger,
	logger *slog.Logger,
) *DefaultKeyRevoker {
	if logger == nil {
		logger = slog.Default()
	}

	return &DefaultKeyRevoker{
		registry:       registry,
		rotator:        rotator,
		secretsManager: secretsManager,
		auditLogger:    auditLogger,
		logger:         logger,
	}
}

// RevokeByUser removes all keys associated with a user.
// Re-encrypts all manifests without the revoked keys.
//
// Validates: Requirements 6.1, 6.2, 6.3, 6.4
// - Identify all keys associated with the user
// - Remove user's Age key public key from .sops.yaml
// - Re-encrypt all manifests without the revoked key
// - Log revocation event to audit log
//
// Returns ErrSingleKeyRevocation if this would remove the only key.
func (r *DefaultKeyRevoker) RevokeByUser(ctx context.Context, opts RevokeOptions) (*RevocationResult, error) {
	r.logger.Info("Starting key revocation by user", "cluster", opts.Cluster, "user", opts.User, "dry_run", opts.DryRun)

	// Validate options
	if opts.User == "" {
		return nil, fmt.Errorf("user email is required for RevokeByUser")
	}

	// List all keys for the cluster
	keys, err := r.registry.ListKeys(ctx, opts.Cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	// Find all keys associated with the user
	var userKeys []KeyEntry
	for _, key := range keys {
		if key.KeyType == KeyTypeAge && key.Status == KeyStatusActive {
			// Check if this key is associated with the user
			// For now, we'll use a simple check based on key metadata
			// In a real implementation, this would check against a user-key mapping
			if r.isKeyOwnedByUser(key, opts.User) {
				userKeys = append(userKeys, key)
			}
		}
	}

	if len(userKeys) == 0 {
		return nil, fmt.Errorf("no active keys found for user %s", opts.User)
	}

	r.logger.Debug("Found keys for user", "user", opts.User, "count", len(userKeys))

	// Check if revoking these keys would leave no active keys
	activeKeyCount := 0
	for _, key := range keys {
		if key.KeyType == KeyTypeAge && key.Status == KeyStatusActive {
			activeKeyCount++
		}
	}

	if activeKeyCount-len(userKeys) == 0 {
		return nil, &ErrSingleKeyRevocation{
			Cluster: opts.Cluster,
			KeyType: KeyTypeAge,
		}
	}

	result := &RevocationResult{
		RevokedKeys:      []string{},
		ReencryptedFiles: []string{},
		NewPrimaryKey:    "",
	}

	// In dry-run mode, don't make changes
	if opts.DryRun {
		for _, key := range userKeys {
			result.RevokedKeys = append(result.RevokedKeys, key.Fingerprint)
		}
		r.logger.Info("Would revoke keys for user (dry-run)",
			"cluster", opts.Cluster,
			"user", opts.User,
			"key_count", len(userKeys))
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

	// Remove user's keys from .sops.yaml
	remainingKeys, err := r.removeKeysFromSOPSConfig(ctx, opts.Cluster, userKeys)
	if err != nil {
		rollbackMgr.Rollback()
		return nil, fmt.Errorf("failed to update .sops.yaml: %w", err)
	}

	// Re-encrypt all manifests without the revoked keys
	reencryptedFiles, err := r.reencryptManifestsWithKeys(ctx, opts.Cluster, remainingKeys)
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

	// Update key status in registry
	for _, key := range userKeys {
		result.RevokedKeys = append(result.RevokedKeys, key.Fingerprint)

		// Update key entry with revocation details
		key.Status = KeyStatusRevoked
		key.RevokedAt = time.Now()
		key.RevokedBy = opts.User

		if err := r.registry.UpdateKeyStatus(ctx, opts.Cluster, KeyTypeAge, KeyStatusRevoked); err != nil {
			r.logger.Warn("Failed to update key status in registry", "fingerprint", key.Fingerprint, "error", err)
			// Don't fail the revocation if registry update fails
		}
	}

	r.logger.Info("Key revocation completed",
		"cluster", opts.Cluster,
		"user", opts.User,
		"revoked_keys", len(result.RevokedKeys),
		"reencrypted_files", len(result.ReencryptedFiles))

	// Log audit event
	if r.auditLogger != nil {
		actor := r.getActor(ctx)
		for _, fingerprint := range result.RevokedKeys {
			if err := r.auditLogger.LogKeyRevoked(ctx, actor, opts.Cluster, fingerprint, opts.User, len(result.ReencryptedFiles)); err != nil {
				r.logger.Warn("Failed to log audit event", "error", err)
			}
		}
	}

	return result, nil
}

// RevokeByFingerprint removes a specific key.
// Re-encrypts all manifests without the revoked key.
//
// Validates: Requirements 6.5, 6.2, 6.3
// - Revoke specific key by fingerprint
// - Re-encrypt all manifests without the revoked key
//
// Returns ErrKeyNotFound if the fingerprint doesn't exist.
// Returns ErrSingleKeyRevocation if this would remove the only key.
func (r *DefaultKeyRevoker) RevokeByFingerprint(ctx context.Context, opts RevokeOptions) (*RevocationResult, error) {
	r.logger.Info("Starting key revocation by fingerprint", "cluster", opts.Cluster, "fingerprint", opts.Fingerprint, "dry_run", opts.DryRun)

	// Validate options
	if opts.Fingerprint == "" {
		return nil, fmt.Errorf("fingerprint is required for RevokeByFingerprint")
	}

	// List all keys for the cluster
	keys, err := r.registry.ListKeys(ctx, opts.Cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	// Find the key with the specified fingerprint
	var targetKey *KeyEntry
	for i, key := range keys {
		if key.Fingerprint == opts.Fingerprint && key.KeyType == KeyTypeAge && key.Status == KeyStatusActive {
			targetKey = &keys[i]
			break
		}
	}

	if targetKey == nil {
		return nil, &ErrKeyNotFound{
			Cluster: opts.Cluster,
			KeyType: KeyTypeAge,
		}
	}

	// Check if revoking this key would leave no active keys
	activeKeyCount := 0
	for _, key := range keys {
		if key.KeyType == KeyTypeAge && key.Status == KeyStatusActive {
			activeKeyCount++
		}
	}

	if activeKeyCount == 1 {
		return nil, &ErrSingleKeyRevocation{
			Cluster: opts.Cluster,
			KeyType: KeyTypeAge,
		}
	}

	result := &RevocationResult{
		RevokedKeys:      []string{opts.Fingerprint},
		ReencryptedFiles: []string{},
		NewPrimaryKey:    "",
	}

	// In dry-run mode, don't make changes
	if opts.DryRun {
		r.logger.Info("Would revoke key (dry-run)",
			"cluster", opts.Cluster,
			"fingerprint", opts.Fingerprint)
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

	// Remove key from .sops.yaml
	remainingKeys, err := r.removeKeysFromSOPSConfig(ctx, opts.Cluster, []KeyEntry{*targetKey})
	if err != nil {
		rollbackMgr.Rollback()
		return nil, fmt.Errorf("failed to update .sops.yaml: %w", err)
	}

	// Re-encrypt all manifests without the revoked key
	reencryptedFiles, err := r.reencryptManifestsWithKeys(ctx, opts.Cluster, remainingKeys)
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

	// Update key status in registry
	targetKey.Status = KeyStatusRevoked
	targetKey.RevokedAt = time.Now()
	targetKey.RevokedBy = "admin" // TODO: Get actual user from context

	if err := r.registry.UpdateKeyStatus(ctx, opts.Cluster, KeyTypeAge, KeyStatusRevoked); err != nil {
		r.logger.Warn("Failed to update key status in registry", "fingerprint", opts.Fingerprint, "error", err)
		// Don't fail the revocation if registry update fails
	}

	r.logger.Info("Key revocation completed",
		"cluster", opts.Cluster,
		"fingerprint", opts.Fingerprint,
		"reencrypted_files", len(result.ReencryptedFiles))

	// Log audit event
	if r.auditLogger != nil {
		actor := r.getActor(ctx)
		if err := r.auditLogger.LogKeyRevoked(ctx, actor, opts.Cluster, opts.Fingerprint, "", len(result.ReencryptedFiles)); err != nil {
			r.logger.Warn("Failed to log audit event", "error", err)
		}
	}

	return result, nil
}

// EmergencyRevoke immediately revokes a key and generates a new primary key.
// This is used when a key is compromised and immediate action is needed.
//
// Validates: Requirements 6.6
// - Perform immediate revocation
// - Generate new primary key
func (r *DefaultKeyRevoker) EmergencyRevoke(ctx context.Context, cluster string, fingerprint string) (*RevocationResult, error) {
	r.logger.Warn("Starting emergency key revocation", "cluster", cluster, "fingerprint", fingerprint)

	// First, generate a new primary key
	r.logger.Info("Generating new primary key for emergency revocation")
	rotateOpts := RotateOptions{
		Cluster:  cluster,
		KeyType:  KeyTypeAge,
		DryRun:   false,
		Complete: false,
	}

	rotationResult, err := r.rotator.RotateAgeKey(ctx, rotateOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new primary key: %w", err)
	}

	r.logger.Info("New primary key generated", "fingerprint", rotationResult.NewFingerprint)

	// Now revoke the compromised key
	revokeOpts := RevokeOptions{
		Cluster:     cluster,
		Fingerprint: fingerprint,
		DryRun:      false,
		Emergency:   true,
		Reason:      "Emergency revocation - key compromised",
	}

	// Since we just generated a new key, we should have at least 2 active keys now
	// So the revocation should succeed
	revocationResult, err := r.RevokeByFingerprint(ctx, revokeOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to revoke compromised key: %w", err)
	}

	// Set the new primary key in the result
	revocationResult.NewPrimaryKey = rotationResult.NewFingerprint

	r.logger.Warn("Emergency key revocation completed",
		"cluster", cluster,
		"revoked_fingerprint", fingerprint,
		"new_primary_key", rotationResult.NewFingerprint,
		"reencrypted_files", len(revocationResult.ReencryptedFiles))

	// Log audit event
	if r.auditLogger != nil {
		actor := r.getActor(ctx)
		if err := r.auditLogger.LogKeyRevoked(ctx, actor, cluster, fingerprint, "", len(revocationResult.ReencryptedFiles)); err != nil {
			r.logger.Warn("Failed to log audit event", "error", err)
		}
	}

	return revocationResult, nil
}

// Private helper methods

// isKeyOwnedByUser checks if a key is owned by the specified user.
// This is a placeholder implementation that should be replaced with
// actual user-key mapping logic.
func (r *DefaultKeyRevoker) isKeyOwnedByUser(key KeyEntry, userEmail string) bool {
	// TODO: Implement actual user-key mapping
	// For now, we'll check if the user email is in the UsedBy list
	for _, user := range key.UsedBy {
		if user == userEmail {
			return true
		}
	}
	return false
}

// removeKeysFromSOPSConfig removes the specified keys from .sops.yaml.
// Returns the list of remaining active keys.
func (r *DefaultKeyRevoker) removeKeysFromSOPSConfig(ctx context.Context, cluster string, keysToRemove []KeyEntry) ([]string, error) {
	r.logger.Debug("Removing keys from .sops.yaml", "cluster", cluster, "key_count", len(keysToRemove))

	// Get the .sops.yaml path from the cluster config
	cfg, configPath, err := r.secretsManager.(*DefaultSecretsManager).loadClusterConfig(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to load cluster config: %w", err)
	}

	// Get the overlay path where .sops.yaml is located
	overlayPath, err := r.secretsManager.(*DefaultSecretsManager).getOverlayPath(configPath, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get overlay path: %w", err)
	}

	sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")

	// Read existing .sops.yaml
	data, err := os.ReadFile(sopsConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read .sops.yaml: %w", err)
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
		return nil, fmt.Errorf("failed to parse .sops.yaml: %w", err)
	}

	// Create a set of keys to remove for quick lookup
	keysToRemoveSet := make(map[string]bool)
	for _, key := range keysToRemove {
		keysToRemoveSet[key.PublicKey] = true
	}

	// Get all active keys for the cluster
	allKeys, err := r.registry.ListKeys(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	// Build list of remaining active keys
	var remainingKeys []string
	for _, key := range allKeys {
		if key.KeyType == KeyTypeAge && key.Status == KeyStatusActive && !keysToRemoveSet[key.PublicKey] {
			remainingKeys = append(remainingKeys, key.PublicKey)
		}
	}

	if len(remainingKeys) == 0 {
		return nil, fmt.Errorf("no active keys would remain after revocation")
	}

	// Update all Age keys in .sops.yaml to use only remaining keys
	remainingKeysStr := ""
	for i, key := range remainingKeys {
		if i > 0 {
			remainingKeysStr += ","
		}
		remainingKeysStr += key
	}

	for i := range sopsConfig.CreationRules {
		if sopsConfig.CreationRules[i].Age != "" {
			sopsConfig.CreationRules[i].Age = remainingKeysStr
		}
	}

	// Write back the updated config
	updatedData, err := yaml.Marshal(&sopsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal .sops.yaml: %w", err)
	}

	if err := os.WriteFile(sopsConfigPath, updatedData, 0o644); err != nil {
		return nil, fmt.Errorf("failed to write .sops.yaml: %w", err)
	}

	r.logger.Info("Updated .sops.yaml with remaining keys",
		"cluster", cluster,
		"remaining_keys", len(remainingKeys),
		"path", sopsConfigPath)

	return remainingKeys, nil
}

// reencryptManifestsWithKeys re-encrypts all manifests with the specified keys.
func (r *DefaultKeyRevoker) reencryptManifestsWithKeys(ctx context.Context, cluster string, keys []string) ([]string, error) {
	r.logger.Debug("Re-encrypting manifests with remaining keys", "cluster", cluster, "key_count", len(keys))

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

		// Re-encrypt with the remaining keys
		// Note: The SOPS config has already been updated with the remaining keys,
		// so we can use in-place encryption
		if err := encryptor.EncryptFile(ctx, tmpDecrypted, sops.EncryptionConfig{
			AgeKeys: keys,
			InPlace: false,
		}); err != nil {
			reencryptErrors = append(reencryptErrors, fmt.Errorf("failed to re-encrypt %s: %w", manifestPath, err))
			continue
		}

		// Move the encrypted file back
		encryptedPath := tmpDecrypted + ".enc"
		if err := os.Rename(encryptedPath, manifestPath); err != nil {
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

// getActor retrieves the actor (user) from context or returns a default value.
func (r *DefaultKeyRevoker) getActor(ctx context.Context) string {
	if actor, ok := ctx.Value("actor").(string); ok && actor != "" {
		return actor
	}
	// Try to get current user
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return "system"
}

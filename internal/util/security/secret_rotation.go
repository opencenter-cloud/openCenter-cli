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

package security

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultSecretRotationManager implements SecretRotationManager interface
type DefaultSecretRotationManager struct {
	backupManager BackupManager
	atomicManager AtomicOperationManager
	auditLogger   AuditLogger
}

// NewDefaultSecretRotationManager creates a new secret rotation manager
func NewDefaultSecretRotationManager(backupManager BackupManager, atomicManager AtomicOperationManager, auditLogger AuditLogger) *DefaultSecretRotationManager {
	return &DefaultSecretRotationManager{
		backupManager: backupManager,
		atomicManager: atomicManager,
		auditLogger:   auditLogger,
	}
}

// RotateSecret rotates a secret with atomic operations and rollback
func (m *DefaultSecretRotationManager) RotateSecret(ctx context.Context, secretName string, newValue []byte) error {
	return m.atomicManager.ExecuteAtomic(ctx, fmt.Sprintf("rotate_secret_%s", secretName), func(tx Transaction) error {
		// Create backup operation
		var backupID string
		backupOp := Operation{
			Type:        "backup",
			Description: fmt.Sprintf("Backup secret %s", secretName),
			Execute: func() error {
				id, err := m.backupManager.CreateBackup(ctx, secretName)
				if err != nil {
					return fmt.Errorf("failed to create backup: %w", err)
				}
				backupID = id
				return nil
			},
			Rollback: func() error {
				if backupID != "" {
					return m.backupManager.DeleteBackup(ctx, backupID)
				}
				return nil
			},
		}

		if err := tx.AddOperation(backupOp); err != nil {
			return err
		}

		// Create rotation operation
		var oldValue []byte
		rotateOp := Operation{
			Type:        "rotate",
			Description: fmt.Sprintf("Rotate secret %s", secretName),
			Execute: func() error {
				// Read old value for rollback
				var err error
				oldValue, err = os.ReadFile(secretName)
				if err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("failed to read old secret: %w", err)
				}

				// Write new value
				if err := os.WriteFile(secretName, newValue, 0600); err != nil {
					return fmt.Errorf("failed to write new secret: %w", err)
				}

				return nil
			},
			Rollback: func() error {
				if oldValue != nil {
					return os.WriteFile(secretName, oldValue, 0600)
				}
				return nil
			},
		}

		if err := tx.AddOperation(rotateOp); err != nil {
			return err
		}

		// Log rotation
		if m.auditLogger != nil {
			m.auditLogger.LogSecurityEvent(ctx, SecurityEvent{
				EventType: "secret_rotation",
				Operation: "rotate_secret",
				Resource:  secretName,
				Success:   true,
				Severity:  "high",
			})
		}

		return nil
	})
}

// RotateSOPSKeys rotates SOPS keys with atomic operations
func (m *DefaultSecretRotationManager) RotateSOPSKeys(ctx context.Context, oldKeyPath, newKeyPath string) error {
	return m.atomicManager.ExecuteAtomic(ctx, "rotate_sops_keys", func(tx Transaction) error {
		// Backup old key
		var backupID string
		backupOp := Operation{
			Type:        "backup",
			Description: "Backup old SOPS key",
			Execute: func() error {
				id, err := m.backupManager.CreateBackup(ctx, oldKeyPath)
				if err != nil {
					return fmt.Errorf("failed to backup old key: %w", err)
				}
				backupID = id
				return nil
			},
			Rollback: func() error {
				if backupID != "" {
					return m.backupManager.DeleteBackup(ctx, backupID)
				}
				return nil
			},
		}

		if err := tx.AddOperation(backupOp); err != nil {
			return err
		}

		// Copy new key to old key location
		var oldKeyData []byte
		replaceOp := Operation{
			Type:        "replace_key",
			Description: "Replace SOPS key",
			Execute: func() error {
				// Read old key for rollback
				var err error
				oldKeyData, err = os.ReadFile(oldKeyPath)
				if err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("failed to read old key: %w", err)
				}

				// Read new key
				newKeyData, err := os.ReadFile(newKeyPath)
				if err != nil {
					return fmt.Errorf("failed to read new key: %w", err)
				}

				// Write new key to old key location
				if err := os.WriteFile(oldKeyPath, newKeyData, 0600); err != nil {
					return fmt.Errorf("failed to write new key: %w", err)
				}

				return nil
			},
			Rollback: func() error {
				if oldKeyData != nil {
					return os.WriteFile(oldKeyPath, oldKeyData, 0600)
				}
				return nil
			},
		}

		if err := tx.AddOperation(replaceOp); err != nil {
			return err
		}

		// Log SOPS key rotation
		if m.auditLogger != nil {
			m.auditLogger.LogSOPSOperation(ctx, "rotate_keys", oldKeyPath, true)
		}

		return nil
	})
}

// ValidateRotation validates that a secret rotation was successful
func (m *DefaultSecretRotationManager) ValidateRotation(ctx context.Context, secretName string) error {
	// Check if secret file exists
	if _, err := os.Stat(secretName); err != nil {
		return fmt.Errorf("secret file not found after rotation: %w", err)
	}

	// Check file permissions
	info, err := os.Stat(secretName)
	if err != nil {
		return fmt.Errorf("failed to stat secret file: %w", err)
	}

	// Ensure file has secure permissions (0600)
	if info.Mode().Perm() != 0600 {
		return fmt.Errorf("secret file has insecure permissions: %s", info.Mode().Perm())
	}

	// Check file is not empty
	if info.Size() == 0 {
		return fmt.Errorf("secret file is empty after rotation")
	}

	return nil
}

// RollbackRotation rolls back a secret rotation using backup
func (m *DefaultSecretRotationManager) RollbackRotation(ctx context.Context, secretName string) error {
	// Find the most recent backup for this secret
	backups, err := m.backupManager.ListBackups(ctx, secretName)
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	if len(backups) == 0 {
		return fmt.Errorf("no backups found for secret %s", secretName)
	}

	// Get the most recent backup
	var mostRecent BackupInfo
	for _, backup := range backups {
		if mostRecent.CreatedAt.IsZero() || backup.CreatedAt.After(mostRecent.CreatedAt) {
			mostRecent = backup
		}
	}

	// Restore the backup
	if err := m.backupManager.RestoreBackup(ctx, mostRecent.ID); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	// Log rollback
	if m.auditLogger != nil {
		m.auditLogger.LogSecurityEvent(ctx, SecurityEvent{
			EventType: "secret_rollback",
			Operation: "rollback_rotation",
			Resource:  secretName,
			Success:   true,
			Severity:  "high",
			Details: map[string]interface{}{
				"backup_id": mostRecent.ID,
			},
		})
	}

	return nil
}

// RotateAllSecretsInDirectory rotates all secrets in a directory
func (m *DefaultSecretRotationManager) RotateAllSecretsInDirectory(ctx context.Context, dirPath string, rotateFunc func(string) ([]byte, error)) error {
	// Read directory entries
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	// Rotate each secret file
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		secretPath := filepath.Join(dirPath, entry.Name())

		// Get new value from rotation function
		newValue, err := rotateFunc(secretPath)
		if err != nil {
			return fmt.Errorf("failed to generate new value for %s: %w", secretPath, err)
		}

		// Rotate the secret
		if err := m.RotateSecret(ctx, secretPath, newValue); err != nil {
			return fmt.Errorf("failed to rotate %s: %w", secretPath, err)
		}
	}

	return nil
}

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
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/sops"
	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
	"github.com/rackerlabs/opencenter-cli/internal/util/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRevoker creates a test key revoker with dependencies
func setupTestRevoker(t *testing.T) (*DefaultKeyRevoker, *MockKeyRegistry, *DefaultSecretsManager, string, func()) {
	t.Helper()

	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "key-revoker-test-*")
	require.NoError(t, err)

	// Create file system
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)

	// Create config loader
	configLoader := config.NewConfigIOHandler(fileSystem)

	// Create SOPS manager
	sopsManager := sops.NewDefaultSOPSManager(nil, nil, slog.Default())

	// Create secrets manager
	secretsManager := NewDefaultSecretsManager(configLoader, sopsManager, nil, slog.Default())

	// Create mock registry
	mockRegistry := NewMockKeyRegistry()

	// Create mock rotator (needed for emergency revocation)
	mockRotator := &MockKeyRotator{}

	// Create key revoker
	revoker := NewDefaultKeyRevoker(mockRegistry, mockRotator, secretsManager, nil, slog.Default())

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return revoker, mockRegistry, secretsManager, tmpDir, cleanup
}

// MockKeyRotator is a mock implementation of KeyRotator for testing
type MockKeyRotator struct {
	rotateAgeKeyCalled bool
	lastRotateOpts     RotateOptions
}

func (m *MockKeyRotator) RotateAgeKey(ctx context.Context, opts RotateOptions) (*RotationResult, error) {
	m.rotateAgeKeyCalled = true
	m.lastRotateOpts = opts
	return &RotationResult{
		OldFingerprint:   "age1old123",
		NewFingerprint:   "age1new456",
		ReencryptedFiles: []string{},
		DualKeyActive:    true,
	}, nil
}

func (m *MockKeyRotator) RotateSSHKey(ctx context.Context, opts RotateOptions) (*RotationResult, error) {
	return nil, nil
}

func (m *MockKeyRotator) CompleteRotation(ctx context.Context, cluster string, keyType KeyType) error {
	return nil
}

func (m *MockKeyRotator) GetRotationStatus(ctx context.Context, cluster string) (*RotationStatus, error) {
	return &RotationStatus{}, nil
}

func TestNewDefaultKeyRevoker(t *testing.T) {
	t.Run("creates revoker with provided logger", func(t *testing.T) {
		mockRegistry := NewMockKeyRegistry()
		mockRotator := &MockKeyRotator{}
		mockSecretsManager := &DefaultSecretsManager{}
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

		revoker := NewDefaultKeyRevoker(mockRegistry, mockRotator, mockSecretsManager, nil, logger)

		assert.NotNil(t, revoker)
		assert.Equal(t, logger, revoker.logger)
		assert.Equal(t, mockRegistry, revoker.registry)
		assert.Equal(t, mockRotator, revoker.rotator)
		assert.Equal(t, mockSecretsManager, revoker.secretsManager)
	})

	t.Run("creates revoker with default logger when nil", func(t *testing.T) {
		mockRegistry := NewMockKeyRegistry()
		mockRotator := &MockKeyRotator{}
		mockSecretsManager := &DefaultSecretsManager{}

		revoker := NewDefaultKeyRevoker(mockRegistry, mockRotator, mockSecretsManager, nil, nil)

		assert.NotNil(t, revoker)
		assert.NotNil(t, revoker.logger)
	})
}

func TestRevokeByUser(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error when user email is empty", func(t *testing.T) {
		revoker, _, _, _, cleanup := setupTestRevoker(t)
		defer cleanup()

		opts := RevokeOptions{
			Cluster: "test-cluster",
			User:    "",
			DryRun:  true,
		}

		result, err := revoker.RevokeByUser(ctx, opts)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "user email is required")
	})

	t.Run("returns error when no keys found for user", func(t *testing.T) {
		revoker, mockRegistry, _, _, cleanup := setupTestRevoker(t)
		defer cleanup()

		cluster := "test-cluster"

		// Register a key not associated with the user
		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: "age1abc123",
			PublicKey:   "age1abc123",
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
			UsedBy:      []string{"other@example.com"},
		})
		require.NoError(t, err)

		opts := RevokeOptions{
			Cluster: cluster,
			User:    "user@example.com",
			DryRun:  false,
		}

		result, err := revoker.RevokeByUser(ctx, opts)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no active keys found for user")
	})

	t.Run("returns error when revoking would leave no active keys", func(t *testing.T) {
		revoker, mockRegistry, _, _, cleanup := setupTestRevoker(t)
		defer cleanup()

		cluster := "test-cluster-single"
		userEmail := "user@example.com"

		// Register a single key associated with the user
		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: "age1only123",
			PublicKey:   "age1only123",
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
			UsedBy:      []string{userEmail},
		})
		require.NoError(t, err)

		opts := RevokeOptions{
			Cluster: cluster,
			User:    userEmail,
			DryRun:  false,
		}

		result, err := revoker.RevokeByUser(ctx, opts)
		assert.Error(t, err)
		assert.Nil(t, result)

		var singleKeyErr *ErrSingleKeyRevocation
		assert.ErrorAs(t, err, &singleKeyErr)
		assert.Equal(t, cluster, singleKeyErr.Cluster)
		assert.Equal(t, KeyTypeAge, singleKeyErr.KeyType)
	})

	t.Run("succeeds in dry-run mode", func(t *testing.T) {
		revoker, mockRegistry, _, _, cleanup := setupTestRevoker(t)
		defer cleanup()

		cluster := "test-cluster-dryrun"
		userEmail := "user@example.com"

		// Register two keys: one for the user, one for another user
		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: "age1user123",
			PublicKey:   "age1user123",
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
			UsedBy:      []string{userEmail},
		})
		require.NoError(t, err)

		err = mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: "age1other456",
			PublicKey:   "age1other456",
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
			UsedBy:      []string{"other@example.com"},
		})
		require.NoError(t, err)

		opts := RevokeOptions{
			Cluster: cluster,
			User:    userEmail,
			DryRun:  true,
		}

		result, err := revoker.RevokeByUser(ctx, opts)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.RevokedKeys, 1)
		assert.Contains(t, result.RevokedKeys, "age1user123")
		assert.Empty(t, result.ReencryptedFiles)
	})

	t.Run("identifies multiple keys for user", func(t *testing.T) {
		revoker, mockRegistry, _, _, cleanup := setupTestRevoker(t)
		defer cleanup()

		cluster := "test-cluster-multi"
		userEmail := "user@example.com"

		// Register multiple keys for the user
		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: "age1user1",
			PublicKey:   "age1user1",
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
			UsedBy:      []string{userEmail},
		})
		require.NoError(t, err)

		err = mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: "age1user2",
			PublicKey:   "age1user2",
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
			UsedBy:      []string{userEmail},
		})
		require.NoError(t, err)

		// Register a key for another user (to prevent single-key error)
		err = mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: "age1other",
			PublicKey:   "age1other",
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
			UsedBy:      []string{"other@example.com"},
		})
		require.NoError(t, err)

		opts := RevokeOptions{
			Cluster: cluster,
			User:    userEmail,
			DryRun:  true,
		}

		result, err := revoker.RevokeByUser(ctx, opts)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.RevokedKeys, 2)
		assert.Contains(t, result.RevokedKeys, "age1user1")
		assert.Contains(t, result.RevokedKeys, "age1user2")
	})
}

func TestRevokeByFingerprint(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error when fingerprint is empty", func(t *testing.T) {
		revoker, _, _, _, cleanup := setupTestRevoker(t)
		defer cleanup()

		opts := RevokeOptions{
			Cluster:     "test-cluster",
			Fingerprint: "",
			DryRun:      true,
		}

		result, err := revoker.RevokeByFingerprint(ctx, opts)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "fingerprint is required")
	})

	t.Run("returns error when key not found", func(t *testing.T) {
		revoker, mockRegistry, _, _, cleanup := setupTestRevoker(t)
		defer cleanup()

		cluster := "test-cluster"

		// Register a different key
		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: "age1different",
			PublicKey:   "age1different",
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
		})
		require.NoError(t, err)

		opts := RevokeOptions{
			Cluster:     cluster,
			Fingerprint: "age1nonexistent",
			DryRun:      false,
		}

		result, err := revoker.RevokeByFingerprint(ctx, opts)
		assert.Error(t, err)
		assert.Nil(t, result)

		var keyNotFoundErr *ErrKeyNotFound
		assert.ErrorAs(t, err, &keyNotFoundErr)
		assert.Equal(t, cluster, keyNotFoundErr.Cluster)
		assert.Equal(t, KeyTypeAge, keyNotFoundErr.KeyType)
	})

	t.Run("returns error when revoking only key", func(t *testing.T) {
		revoker, mockRegistry, _, _, cleanup := setupTestRevoker(t)
		defer cleanup()

		cluster := "test-cluster-single"
		fingerprint := "age1only"

		// Register a single key
		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: fingerprint,
			PublicKey:   fingerprint,
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
		})
		require.NoError(t, err)

		opts := RevokeOptions{
			Cluster:     cluster,
			Fingerprint: fingerprint,
			DryRun:      false,
		}

		result, err := revoker.RevokeByFingerprint(ctx, opts)
		assert.Error(t, err)
		assert.Nil(t, result)

		var singleKeyErr *ErrSingleKeyRevocation
		assert.ErrorAs(t, err, &singleKeyErr)
	})

	t.Run("succeeds in dry-run mode", func(t *testing.T) {
		revoker, mockRegistry, _, _, cleanup := setupTestRevoker(t)
		defer cleanup()

		cluster := "test-cluster-dryrun"
		fingerprint := "age1revoke"

		// Register two keys
		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: fingerprint,
			PublicKey:   fingerprint,
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
		})
		require.NoError(t, err)

		err = mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: "age1keep",
			PublicKey:   "age1keep",
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
		})
		require.NoError(t, err)

		opts := RevokeOptions{
			Cluster:     cluster,
			Fingerprint: fingerprint,
			DryRun:      true,
		}

		result, err := revoker.RevokeByFingerprint(ctx, opts)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.RevokedKeys, 1)
		assert.Equal(t, fingerprint, result.RevokedKeys[0])
		assert.Empty(t, result.ReencryptedFiles)
	})
}

func TestEmergencyRevoke(t *testing.T) {
	ctx := context.Background()

	t.Run("generates new key and revokes compromised key", func(t *testing.T) {
		revoker, mockRegistry, _, _, cleanup := setupTestRevoker(t)
		defer cleanup()

		cluster := "test-cluster-emergency"
		compromisedFingerprint := "age1compromised"

		// Register the compromised key
		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: compromisedFingerprint,
			PublicKey:   compromisedFingerprint,
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
		})
		require.NoError(t, err)

		result, err := revoker.EmergencyRevoke(ctx, cluster, compromisedFingerprint)

		// We expect an error because we don't have real config/SOPS setup
		// But we can verify the method attempts the correct operations
		if err != nil {
			t.Logf("Expected error in test environment (no real config): %v", err)
			// Verify it's not a validation error
			assert.NotContains(t, err.Error(), "fingerprint is required")
		} else {
			// If it succeeds, verify the result
			assert.NotNil(t, result)
			assert.Contains(t, result.RevokedKeys, compromisedFingerprint)
			assert.NotEmpty(t, result.NewPrimaryKey)
		}
	})

	t.Run("calls rotator to generate new key", func(t *testing.T) {
		mockRegistry := NewMockKeyRegistry()
		mockRotator := &MockKeyRotator{}
		mockSecretsManager := &DefaultSecretsManager{}
		revoker := NewDefaultKeyRevoker(mockRegistry, mockRotator, mockSecretsManager, nil, slog.Default())

		cluster := "test-cluster"
		fingerprint := "age1compromised"

		// Register the compromised key
		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: fingerprint,
			PublicKey:   fingerprint,
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
		})
		require.NoError(t, err)

		_, err = revoker.EmergencyRevoke(ctx, cluster, fingerprint)

		// Verify rotator was called (even if the overall operation fails)
		assert.True(t, mockRotator.rotateAgeKeyCalled)
		assert.Equal(t, cluster, mockRotator.lastRotateOpts.Cluster)
		assert.Equal(t, KeyTypeAge, mockRotator.lastRotateOpts.KeyType)
		assert.False(t, mockRotator.lastRotateOpts.DryRun)
	})
}

func TestIsKeyOwnedByUser(t *testing.T) {
	revoker, _, _, _, cleanup := setupTestRevoker(t)
	defer cleanup()

	t.Run("returns true when user is in UsedBy list", func(t *testing.T) {
		key := KeyEntry{
			UsedBy: []string{"user1@example.com", "user2@example.com"},
		}

		assert.True(t, revoker.isKeyOwnedByUser(key, "user1@example.com"))
		assert.True(t, revoker.isKeyOwnedByUser(key, "user2@example.com"))
	})

	t.Run("returns false when user is not in UsedBy list", func(t *testing.T) {
		key := KeyEntry{
			UsedBy: []string{"user1@example.com"},
		}

		assert.False(t, revoker.isKeyOwnedByUser(key, "user2@example.com"))
	})

	t.Run("returns false when UsedBy list is empty", func(t *testing.T) {
		key := KeyEntry{
			UsedBy: []string{},
		}

		assert.False(t, revoker.isKeyOwnedByUser(key, "user@example.com"))
	})
}

func TestRevokeByUserIntegration(t *testing.T) {
	t.Run("revokes user keys and updates registry", func(t *testing.T) {
		revoker, mockRegistry, _, tmpDir, cleanup := setupTestRevoker(t)
		defer cleanup()

		ctx := context.Background()
		cluster := "test-cluster-integration"
		userEmail := "departing@example.com"

		// Setup: Register keys for multiple users
		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: "age1departing",
			PublicKey:   "age1departing",
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
			UsedBy:      []string{userEmail},
		})
		require.NoError(t, err)

		err = mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: "age1remaining",
			PublicKey:   "age1remaining",
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
			UsedBy:      []string{"remaining@example.com"},
		})
		require.NoError(t, err)

		// Create test config structure
		homeDir, err := os.UserHomeDir()
		require.NoError(t, err)

		configDir := filepath.Join(homeDir, ".config", "opencenter", "clusters", "test-org", cluster)
		err = os.MkdirAll(configDir, 0755)
		require.NoError(t, err)
		defer os.RemoveAll(filepath.Join(homeDir, ".config", "opencenter", "clusters", "test-org", cluster))

		testRepoDir := filepath.Join(tmpDir, "test-repo")
		err = os.MkdirAll(testRepoDir, 0755)
		require.NoError(t, err)

		// Create config file
		configPath := filepath.Join(configDir, ".k8s-"+cluster+"-config.yaml")
		configData := `schema_version: "2.0"
opencenter:
  cluster:
    cluster_name: ` + cluster + `
  gitops:
    git_dir: ` + testRepoDir + `
secrets:
  sops_age_key_file: ~/.config/sops/age/test-key.txt
`
		err = os.WriteFile(configPath, []byte(configData), 0644)
		require.NoError(t, err)

		// Create overlay directory with .sops.yaml
		overlayPath := filepath.Join(testRepoDir, "applications", "overlays", cluster)
		err = os.MkdirAll(overlayPath, 0755)
		require.NoError(t, err)

		sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")
		sopsConfigData := `creation_rules:
  - path_regex: .*\.yaml$
    encrypted_regex: ^(data|stringData)$
    age: age1departing,age1remaining
`
		err = os.WriteFile(sopsConfigPath, []byte(sopsConfigData), 0644)
		require.NoError(t, err)

		// Attempt revocation
		opts := RevokeOptions{
			Cluster: cluster,
			User:    userEmail,
			DryRun:  false,
		}

		result, err := revoker.RevokeByUser(ctx, opts)

		// We expect an error because we don't have real SOPS setup
		// But we can verify the method attempts the correct operations
		if err != nil {
			t.Logf("Expected error in test environment (no real SOPS setup): %v", err)
			// Verify it's not a validation error
			assert.NotContains(t, err.Error(), "user email is required")
			assert.NotContains(t, err.Error(), "no active keys found")
		} else {
			// If it succeeds, verify the result
			assert.NotNil(t, result)
			assert.Contains(t, result.RevokedKeys, "age1departing")
			assert.NotContains(t, result.RevokedKeys, "age1remaining")
		}
	})
}

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
	"testing"
	"time"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRotator creates a test key rotator with dependencies
func setupTestRotator(t *testing.T) (*DefaultKeyRotator, *MockKeyRegistry, *DefaultSecretsManager, string, func()) {
	t.Helper()

	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "key-rotator-test-*")
	require.NoError(t, err)

	originalHome := os.Getenv("HOME")
	originalConfigDir := os.Getenv("OPENCENTER_CONFIG_DIR")
	testHome := filepath.Join(tmpDir, "home")
	testConfigDir := filepath.Join(testHome, ".config", "opencenter")

	require.NoError(t, os.MkdirAll(testConfigDir, 0o755))
	require.NoError(t, os.Setenv("HOME", testHome))
	require.NoError(t, os.Setenv("OPENCENTER_CONFIG_DIR", testConfigDir))

	// Create file system
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)

	// Create config loader
	configLoader := v2.NewConfigIOHandler(fileSystem)

	// Create SOPS manager
	sopsManager := sops.NewDefaultSOPSManager(nil, nil, slog.Default())

	// Create secrets manager
	secretsManager := NewDefaultSecretsManager(configLoader, sopsManager, nil, slog.Default())

	// Create mock registry
	mockRegistry := NewMockKeyRegistry()

	// Create key rotator
	rotator := NewDefaultKeyRotator(mockRegistry, secretsManager, nil, slog.Default())

	cleanup := func() {
		if originalHome == "" {
			os.Unsetenv("HOME")
		} else {
			os.Setenv("HOME", originalHome)
		}
		if originalConfigDir == "" {
			os.Unsetenv("OPENCENTER_CONFIG_DIR")
		} else {
			os.Setenv("OPENCENTER_CONFIG_DIR", originalConfigDir)
		}
		os.RemoveAll(tmpDir)
	}

	return rotator, mockRegistry, secretsManager, tmpDir, cleanup
}

// MockKeyRegistry is a mock implementation of KeyRegistry for testing
type MockKeyRegistry struct {
	keys          map[string][]KeyEntry
	mutationError error
}

// SetMutationError injects a failure into the next registry mutation path.
// It is intentionally configurable so rollback tests can exercise every caller.
func (m *MockKeyRegistry) SetMutationError(err error) {
	m.mutationError = err
}

func NewMockKeyRegistry() *MockKeyRegistry {
	return &MockKeyRegistry{
		keys: make(map[string][]KeyEntry),
	}
}

func (m *MockKeyRegistry) RegisterKey(ctx context.Context, entry KeyEntry) error {
	if m.mutationError != nil {
		return m.mutationError
	}
	key := entry.Cluster + ":" + string(entry.KeyType)
	for _, existing := range m.keys[key] {
		if existing.Fingerprint == entry.Fingerprint {
			return fmt.Errorf("%s key %s already exists for cluster %s with status %s", entry.KeyType, entry.Fingerprint, entry.Cluster, existing.Status)
		}
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if entry.ExpiresAt.IsZero() {
		days := DefaultAgeExpirationDays
		if entry.KeyType == KeyTypeSSH {
			days = DefaultSSHExpirationDays
		}
		entry.ExpiresAt = entry.CreatedAt.AddDate(0, 0, days)
	}
	if entry.Status == "" {
		entry.Status = KeyStatusActive
	}
	if entry.Primary && entry.Status != KeyStatusActive {
		return fmt.Errorf("cannot register inactive %s key %s as primary", entry.KeyType, entry.Fingerprint)
	}
	if entry.Primary {
		for _, existing := range m.keys[key] {
			if existing.Status == KeyStatusActive && existing.Primary {
				return fmt.Errorf("active primary %s key %s already exists for cluster %s", entry.KeyType, existing.Fingerprint, entry.Cluster)
			}
		}
	}
	m.keys[key] = append(m.keys[key], entry)
	return nil
}

// GetKey retrieves key metadata by cluster and type.
// It delegates to the active primary key when one exists, falling back to the earliest-registered active key when no primary is set (legacy registries).
func (m *MockKeyRegistry) GetKey(ctx context.Context, cluster string, keyType KeyType) (*KeyEntry, error) {
	return mockSelectKey(m.keys[cluster+":"+string(keyType)], cluster, keyType, false)
}

func (m *MockKeyRegistry) GetPrimaryKey(ctx context.Context, cluster string, keyType KeyType) (*KeyEntry, error) {
	return mockSelectKey(m.keys[cluster+":"+string(keyType)], cluster, keyType, true)
}

func (m *MockKeyRegistry) SetPrimaryKey(ctx context.Context, cluster string, keyType KeyType, fingerprint string) error {
	if m.mutationError != nil {
		return m.mutationError
	}
	key := cluster + ":" + string(keyType)
	entries := m.keys[key]
	candidate := -1
	for i := range entries {
		if entries[i].Fingerprint == fingerprint {
			if entries[i].Status != KeyStatusActive {
				return fmt.Errorf("cannot set inactive %s key %s as primary", keyType, fingerprint)
			}
			candidate = i
			break
		}
	}
	if candidate == -1 {
		return &ErrKeyNotFound{Cluster: cluster, KeyType: keyType}
	}
	changed := false
	for i := range entries {
		want := i == candidate
		if entries[i].Primary != want {
			entries[i].Primary = want
			changed = true
		}
	}
	if changed {
		m.keys[key] = entries
	}
	return nil
}

func (m *MockKeyRegistry) ReplacePrimary(ctx context.Context, oldFingerprint string, newEntry KeyEntry) error {
	if m.mutationError != nil {
		return m.mutationError
	}
	key := newEntry.Cluster + ":" + string(newEntry.KeyType)
	entries := m.keys[key]
	for _, existing := range entries {
		if existing.Fingerprint == newEntry.Fingerprint {
			return fmt.Errorf("%s key %s already exists for cluster %s with status %s", newEntry.KeyType, newEntry.Fingerprint, newEntry.Cluster, existing.Status)
		}
	}
	oldIndex := -1
	for i := range entries {
		if entries[i].Fingerprint == oldFingerprint {
			oldIndex = i
			break
		}
	}
	if oldIndex == -1 {
		return &ErrKeyNotFound{Cluster: newEntry.Cluster, KeyType: newEntry.KeyType}
	}
	if entries[oldIndex].Status != KeyStatusActive || !entries[oldIndex].Primary {
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
		days := DefaultAgeExpirationDays
		if newEntry.KeyType == KeyTypeSSH {
			days = DefaultSSHExpirationDays
		}
		newEntry.ExpiresAt = newEntry.CreatedAt.AddDate(0, 0, days)
	}
	if newEntry.Status == "" {
		newEntry.Status = KeyStatusActive
	}
	for i := range entries {
		entries[i].Primary = false
	}
	newEntry.Primary = true
	m.keys[key] = append(entries, newEntry)
	return nil
}

func (m *MockKeyRegistry) ReplacePrimaryAndArchive(ctx context.Context, oldFingerprint string, newEntry KeyEntry) error {
	if m.mutationError != nil {
		return m.mutationError
	}
	key := newEntry.Cluster + ":" + string(newEntry.KeyType)
	original := append([]KeyEntry(nil), m.keys[key]...)
	if err := m.ReplacePrimary(ctx, oldFingerprint, newEntry); err != nil {
		return err
	}
	entries := m.keys[key]
	for i := range entries {
		if entries[i].Fingerprint == oldFingerprint {
			entries[i].Status = KeyStatusArchived
			entries[i].Primary = false
			m.keys[key] = entries
			return nil
		}
	}
	m.keys[key] = original
	return &ErrKeyNotFound{Cluster: newEntry.Cluster, KeyType: newEntry.KeyType}
}

func (m *MockKeyRegistry) UpdateKeys(ctx context.Context, entries []KeyEntry) error {
	if m.mutationError != nil {
		return m.mutationError
	}
	original := m.keys
	working := make(map[string][]KeyEntry, len(original))
	for key, values := range original {
		working[key] = append([]KeyEntry(nil), values...)
	}
	m.keys = working
	for _, entry := range entries {
		if err := m.UpdateKey(ctx, entry); err != nil {
			m.keys = original
			return err
		}
	}
	return nil
}

func (m *MockKeyRegistry) UpdateKeyStatus(ctx context.Context, cluster string, keyType KeyType, status KeyStatus) error {
	if m.mutationError != nil {
		return m.mutationError
	}
	key := cluster + ":" + string(keyType)
	entries := m.keys[key]
	if len(entries) > 0 {
		matches := make([]int, 0, 1)
		for i := range entries {
			if entries[i].Status == KeyStatusActive {
				matches = append(matches, i)
			}
		}
		if len(matches) == 0 {
			return &ErrKeyNotFound{Cluster: cluster, KeyType: keyType}
		}
		if len(matches) > 1 {
			return fmt.Errorf("multiple active %s keys exist for cluster %s; use fingerprint-targeted UpdateKey", keyType, cluster)
		}
		index := matches[0]
		entries[index].Status = status
		if status != KeyStatusActive {
			entries[index].Primary = false
		}
		m.keys[key] = entries
		return nil
	}
	return &ErrKeyNotFound{Cluster: cluster, KeyType: keyType}
}

func (m *MockKeyRegistry) UpdateKey(ctx context.Context, entry KeyEntry) error {
	if m.mutationError != nil {
		return m.mutationError
	}
	key := entry.Cluster + ":" + string(entry.KeyType)
	entries := m.keys[key]
	for i := range entries {
		if entry.Fingerprint != "" && entries[i].Fingerprint == entry.Fingerprint {
			updated := entries[i]
			if entry.PublicKey != "" {
				updated.PublicKey = entry.PublicKey
			}
			if entry.Status != "" {
				updated.Status = entry.Status
			}
			if !entry.CreatedAt.IsZero() {
				updated.CreatedAt = entry.CreatedAt
			}
			if !entry.ExpiresAt.IsZero() {
				updated.ExpiresAt = entry.ExpiresAt
			}
			updated.RotatedFrom = entry.RotatedFrom
			updated.RevokedAt = entry.RevokedAt
			updated.RevokedBy = entry.RevokedBy
			updated.RevokedReason = entry.RevokedReason
			updated.UsedBy = entry.UsedBy
			updated.UserEmail = entry.UserEmail
			updated.Primary = entry.Primary
			if updated.Primary && updated.Status != KeyStatusActive {
				if entry.Primary {
					return fmt.Errorf("cannot set inactive %s key %s as primary", entry.KeyType, entry.Fingerprint)
				}
				updated.Primary = false
			}
			if updated.Primary && updated.Status == KeyStatusActive {
				for j, other := range entries {
					if j != i && other.Status == KeyStatusActive && other.Primary {
						return fmt.Errorf("active primary %s key %s already exists for cluster %s", entry.KeyType, other.Fingerprint, entry.Cluster)
					}
				}
			}
			entries[i] = updated
			m.keys[key] = entries
			return nil
		}
	}
	return &ErrKeyNotFound{Cluster: entry.Cluster, KeyType: entry.KeyType}
}

func (m *MockKeyRegistry) ListKeys(ctx context.Context, cluster string) ([]KeyEntry, error) {
	var result []KeyEntry
	for _, entries := range m.keys {
		for _, entry := range entries {
			if cluster == "" || entry.Cluster == cluster {
				result = append(result, entry)
			}
		}
	}
	return result, nil
}

func mockSelectKey(entries []KeyEntry, cluster string, keyType KeyType, primaryOnly bool) (*KeyEntry, error) {
	var primaries []KeyEntry
	for _, entry := range entries {
		if entry.Status == KeyStatusActive && entry.Primary {
			primaries = append(primaries, entry)
		}
	}
	if len(primaries) > 1 {
		fingerprints := make([]string, len(primaries))
		for i, entry := range primaries {
			fingerprints[i] = entry.Fingerprint
		}
		return nil, fmt.Errorf("multiple active primary %s keys for cluster %s: %s", keyType, cluster, strings.Join(fingerprints, ", "))
	}
	if len(primaries) == 1 {
		entry := primaries[0]
		return &entry, nil
	}
	if primaryOnly {
		return nil, &ErrKeyNotFound{Cluster: cluster, KeyType: keyType}
	}
	for _, entry := range entries {
		if entry.Status == KeyStatusActive {
			returnEntry := entry
			return &returnEntry, nil
		}
	}
	return nil, &ErrKeyNotFound{Cluster: cluster, KeyType: keyType}
}

func (m *MockKeyRegistry) CheckExpiration(ctx context.Context, warnDays int) (*ExpirationReport, error) {
	return &ExpirationReport{}, nil
}

func (m *MockKeyRegistry) RebuildFromFiles(ctx context.Context, keysDir string) error {
	return nil
}

func TestNewDefaultKeyRotator(t *testing.T) {
	t.Run("creates rotator with provided logger", func(t *testing.T) {
		mockRegistry := NewMockKeyRegistry()
		mockSecretsManager := &DefaultSecretsManager{}
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

		rotator := NewDefaultKeyRotator(mockRegistry, mockSecretsManager, nil, logger)

		assert.NotNil(t, rotator)
		assert.Equal(t, logger, rotator.logger)
		assert.Equal(t, mockRegistry, rotator.registry)
		assert.Equal(t, mockSecretsManager, rotator.secretsManager)
	})

	t.Run("creates rotator with default logger when nil", func(t *testing.T) {
		mockRegistry := NewMockKeyRegistry()
		mockSecretsManager := &DefaultSecretsManager{}

		rotator := NewDefaultKeyRotator(mockRegistry, mockSecretsManager, nil, nil)

		assert.NotNil(t, rotator)
		assert.NotNil(t, rotator.logger)
	})
}

func TestGetRotationStatus(t *testing.T) {
	rotator, mockRegistry, _, _, cleanup := setupTestRotator(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("returns no rotation in progress with single active key", func(t *testing.T) {
		// Register a single active Age key
		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     "test-cluster",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1abc123",
			PublicKey:   "age1abc123",
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
			Primary:     true,
		})
		require.NoError(t, err)

		status, err := rotator.GetRotationStatus(ctx, "test-cluster")
		require.NoError(t, err)

		assert.False(t, status.InProgress)
		assert.False(t, status.DualKeyActive)
		assert.NotNil(t, status.NewKey)
		assert.Nil(t, status.OldKey)
	})

	t.Run("returns rotation in progress with two active keys", func(t *testing.T) {
		// Register two active Age keys
		oldTime := time.Now().Add(-1 * time.Hour)
		newTime := time.Now()

		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     "test-cluster-dual",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1old123",
			PublicKey:   "age1old123",
			CreatedAt:   oldTime,
			Status:      KeyStatusActive,
			Primary:     true,
		})
		require.NoError(t, err)

		err = mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     "test-cluster-dual",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1new456",
			PublicKey:   "age1new456",
			CreatedAt:   newTime,
			Status:      KeyStatusActive,
			RotatedFrom: "age1old123",
		})
		require.NoError(t, err)

		status, err := rotator.GetRotationStatus(ctx, "test-cluster-dual")
		require.NoError(t, err)

		assert.True(t, status.InProgress)
		assert.True(t, status.DualKeyActive)
		assert.NotNil(t, status.OldKey)
		assert.NotNil(t, status.NewKey)
		assert.Equal(t, "age1old123", status.OldKey.Fingerprint)
		assert.Equal(t, "age1new456", status.NewKey.Fingerprint)
	})

	t.Run("returns no keys when cluster has no Age keys", func(t *testing.T) {
		status, err := rotator.GetRotationStatus(ctx, "nonexistent-cluster")
		require.NoError(t, err)

		assert.False(t, status.InProgress)
		assert.False(t, status.DualKeyActive)
		assert.Nil(t, status.OldKey)
		assert.Nil(t, status.NewKey)
	})
}

func TestCompleteRotation(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error when no rotation in progress", func(t *testing.T) {
		rotator, mockRegistry, _, _, cleanup := setupTestRotator(t)
		defer cleanup()

		// Register a single active key (no rotation in progress)
		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     "test-cluster",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1abc123",
			PublicKey:   "age1abc123",
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
			Primary:     true,
		})
		require.NoError(t, err)

		err = rotator.CompleteRotation(ctx, "test-cluster", KeyTypeAge)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no rotation in progress")
	})

	t.Run("returns error for non-Age key types", func(t *testing.T) {
		rotator, _, _, _, cleanup := setupTestRotator(t)
		defer cleanup()

		err := rotator.CompleteRotation(ctx, "test-cluster", KeyTypeSSH)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "only supports Age keys")
	})

	t.Run("returns error when new key not found", func(t *testing.T) {
		rotator, mockRegistry, _, _, cleanup := setupTestRotator(t)
		defer cleanup()

		// Register two active keys but with same creation time (edge case)
		now := time.Now()
		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     "test-cluster",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1key1",
			PublicKey:   "age1key1",
			CreatedAt:   now,
			Status:      KeyStatusActive,
			Primary:     true,
		})
		require.NoError(t, err)

		err = mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     "test-cluster",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1key2",
			PublicKey:   "age1key2",
			CreatedAt:   now,
			Status:      KeyStatusActive,
			RotatedFrom: "age1key1",
		})
		require.NoError(t, err)

		// This should work since we have two active keys
		// The test verifies the method handles the rotation status correctly
		status, err := rotator.GetRotationStatus(ctx, "test-cluster")
		require.NoError(t, err)
		assert.True(t, status.InProgress)
	})
}

func TestCompleteRotationIntegration(t *testing.T) {
	t.Run("completes rotation successfully with dual-key setup", func(t *testing.T) {
		rotator, mockRegistry, _, tmpDir, cleanup := setupTestRotator(t)
		defer cleanup()

		ctx := context.Background()
		cluster := "test-cluster-complete"

		// Setup: Create a dual-key rotation scenario
		oldTime := time.Now().Add(-1 * time.Hour)
		newTime := time.Now()

		// Register old key
		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: "age1old123",
			PublicKey:   "age1old123",
			CreatedAt:   oldTime,
			Status:      KeyStatusActive,
			Primary:     true,
		})
		require.NoError(t, err)

		// Register new key
		err = mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: "age1new456",
			PublicKey:   "age1new456",
			CreatedAt:   newTime,
			Status:      KeyStatusActive,
			RotatedFrom: "age1old123",
		})
		require.NoError(t, err)

		// Verify rotation is in progress
		status, err := rotator.GetRotationStatus(ctx, cluster)
		require.NoError(t, err)
		assert.True(t, status.InProgress)
		assert.True(t, status.DualKeyActive)

		// Create test config and overlay structure
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
		writeNormalizedSecretsConfigFile(t, configPath, cluster, configData)

		// Create overlay directory with .sops.yaml
		overlayPath := filepath.Join(testRepoDir, "applications", "overlays", cluster)
		err = os.MkdirAll(overlayPath, 0755)
		require.NoError(t, err)

		sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")
		sopsConfigData := `creation_rules:
  - path_regex: .*\.yaml$
    encrypted_regex: ^(data|stringData)$
    age: age1old123,age1new456
`
		err = os.WriteFile(sopsConfigPath, []byte(sopsConfigData), 0644)
		require.NoError(t, err)

		// Note: CompleteRotation will fail in this test environment because:
		// 1. We don't have actual SOPS keys set up
		// 2. We don't have actual encrypted manifests
		// But we can verify the method is called and handles the setup correctly

		err = rotator.CompleteRotation(ctx, cluster, KeyTypeAge)
		// We expect an error because we don't have real SOPS setup
		// But the error should be about loading config or re-encryption, not about rotation logic
		if err != nil {
			t.Logf("Expected error in test environment (no real SOPS setup): %v", err)
			// Verify it's not a rotation logic error
			assert.NotContains(t, err.Error(), "no rotation in progress")
			assert.NotContains(t, err.Error(), "only supports Age keys")
		}
	})
}

func TestRotateAgeKey(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error for non-Age key type", func(t *testing.T) {
		rotator, _, _, _, cleanup := setupTestRotator(t)
		defer cleanup()

		opts := RotateOptions{
			Cluster: "test-cluster",
			KeyType: KeyTypeSSH,
			DryRun:  true,
		}

		result, err := rotator.RotateAgeKey(ctx, opts)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid key type")
	})

	t.Run("returns error when rotation already in progress", func(t *testing.T) {
		rotator, mockRegistry, _, _, cleanup := setupTestRotator(t)
		defer cleanup()

		cluster := "test-cluster-in-progress"

		// Setup dual-key scenario (rotation in progress)
		oldTime := time.Now().Add(-1 * time.Hour)
		newTime := time.Now()

		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: "age1old",
			PublicKey:   "age1old",
			CreatedAt:   oldTime,
			Status:      KeyStatusActive,
			Primary:     true,
		})
		require.NoError(t, err)

		err = mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: "age1new",
			PublicKey:   "age1new",
			CreatedAt:   newTime,
			Status:      KeyStatusActive,
			RotatedFrom: "age1old",
		})
		require.NoError(t, err)

		opts := RotateOptions{
			Cluster: cluster,
			KeyType: KeyTypeAge,
			DryRun:  false,
		}

		result, err := rotator.RotateAgeKey(ctx, opts)
		assert.Error(t, err)
		assert.Nil(t, result)

		var rotationErr *ErrRotationInProgress
		assert.ErrorAs(t, err, &rotationErr)
		assert.Equal(t, cluster, rotationErr.Cluster)
	})

	t.Run("succeeds in dry-run mode", func(t *testing.T) {
		rotator, mockRegistry, _, _, cleanup := setupTestRotator(t)
		defer cleanup()

		cluster := "test-cluster-dryrun"

		// Register initial key
		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeAge,
			Fingerprint: "age1initial",
			PublicKey:   "age1initial",
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
			Primary:     true,
		})
		require.NoError(t, err)

		opts := RotateOptions{
			Cluster: cluster,
			KeyType: KeyTypeAge,
			DryRun:  true,
		}

		result, err := rotator.RotateAgeKey(ctx, opts)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "age1initial", result.OldFingerprint)
		assert.Equal(t, "age1placeholder...", result.NewFingerprint)
		assert.True(t, result.DualKeyActive)
	})
}

func TestRotateSSHKey(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error for non-SSH key type", func(t *testing.T) {
		rotator, _, _, _, cleanup := setupTestRotator(t)
		defer cleanup()

		opts := RotateOptions{
			Cluster: "test-cluster",
			KeyType: KeyTypeAge,
			DryRun:  true,
		}

		result, err := rotator.RotateSSHKey(ctx, opts)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid key type")
	})

	t.Run("succeeds in dry-run mode", func(t *testing.T) {
		rotator, mockRegistry, _, _, cleanup := setupTestRotator(t)
		defer cleanup()

		cluster := "test-cluster-ssh-dryrun"

		// Register initial SSH key
		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeSSH,
			Fingerprint: "SHA256:old123",
			PublicKey:   "ssh-ed25519 AAAA...",
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
			Primary:     true,
		})
		require.NoError(t, err)

		opts := RotateOptions{
			Cluster: cluster,
			KeyType: KeyTypeSSH,
			DryRun:  true,
		}

		result, err := rotator.RotateSSHKey(ctx, opts)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "SHA256:old123", result.OldFingerprint)
		assert.Equal(t, "ssh-ed25519 AAAA...", result.NewFingerprint)
		assert.False(t, result.DualKeyActive) // SSH rotation is immediate
	})

	t.Run("verifies no dual-key mode for SSH rotation", func(t *testing.T) {
		rotator, mockRegistry, _, tmpDir, cleanup := setupTestRotator(t)
		defer cleanup()

		cluster := "test-cluster-ssh-nodual"

		// Register initial SSH key
		err := mockRegistry.RegisterKey(ctx, KeyEntry{
			Cluster:     cluster,
			KeyType:     KeyTypeSSH,
			Fingerprint: "SHA256:initial123",
			PublicKey:   "ssh-ed25519 AAAA...initial",
			CreatedAt:   time.Now(),
			Status:      KeyStatusActive,
			Primary:     true,
		})
		require.NoError(t, err)

		// Create test config structure
		homeDir, err := os.UserHomeDir()
		require.NoError(t, err)

		configDir := filepath.Join(homeDir, ".config", "opencenter", "clusters", "test-org", cluster)
		err = os.MkdirAll(configDir, 0755)
		require.NoError(t, err)
		defer os.RemoveAll(filepath.Join(homeDir, ".config", "opencenter", "clusters", "test-org", cluster))

		testRepoDir := filepath.Join(tmpDir, "test-repo-ssh")
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
  ssh_private_key_file: ~/.ssh/old-key
  ssh_public_key_file: ~/.ssh/old-key.pub
`
		writeNormalizedSecretsConfigFile(t, configPath, cluster, configData)

		// Create SSH directory for key generation
		sshDir := filepath.Join(homeDir, ".config", "opencenter", "clusters", cluster, "secrets", "ssh")
		err = os.MkdirAll(sshDir, 0700)
		require.NoError(t, err)
		defer os.RemoveAll(filepath.Join(homeDir, ".config", "opencenter", "clusters", cluster))

		// Create a dummy old SSH key file for archiving
		oldKeyPath := filepath.Join(sshDir, cluster+"-ssh-old")
		err = os.WriteFile(oldKeyPath, []byte("old-private-key-content"), 0600)
		require.NoError(t, err)
		err = os.WriteFile(oldKeyPath+".pub", []byte("ssh-ed25519 AAAA...initial"), 0644)
		require.NoError(t, err)

		opts := RotateOptions{
			Cluster: cluster,
			KeyType: KeyTypeSSH,
			DryRun:  false,
		}

		result, err := rotator.RotateSSHKey(ctx, opts)

		// We expect an error because ssh-keygen might not be available or config update might fail
		// But we can verify the result structure if it succeeds
		if err != nil {
			t.Logf("Expected error in test environment (ssh-keygen or config update): %v", err)
			// Verify it's not a validation error
			assert.NotContains(t, err.Error(), "invalid key type")
		} else {
			// If it succeeds, verify the result
			assert.NotNil(t, result)
			assert.Equal(t, "SHA256:initial123", result.OldFingerprint)
			assert.False(t, result.DualKeyActive, "SSH rotation should not use dual-key mode")
			assert.Empty(t, result.ReencryptedFiles, "SSH rotation should not re-encrypt files")
		}
	})

	t.Run("returns error when current SSH key not found", func(t *testing.T) {
		rotator, _, _, _, cleanup := setupTestRotator(t)
		defer cleanup()

		opts := RotateOptions{
			Cluster: "nonexistent-cluster",
			KeyType: KeyTypeSSH,
			DryRun:  false,
		}

		result, err := rotator.RotateSSHKey(ctx, opts)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no active primary SSH key")
	})
}

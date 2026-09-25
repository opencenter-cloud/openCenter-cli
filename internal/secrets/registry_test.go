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
	"sync"
	"testing"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSOPSEncryptor is a mock implementation of SOPSEncryptor for testing.
type mockSOPSEncryptor struct {
	mu                   sync.Mutex
	encryptedFiles       map[string][]byte
	encryptionRecipients []string
	encryptError         error
	decryptError         error
}

func newMockSOPSEncryptor() *mockSOPSEncryptor {
	return &mockSOPSEncryptor{
		encryptedFiles: make(map[string][]byte),
	}
}

// cacheLocked stores content for filePath. It lazily initializes the cache so a
// zero-value mockSOPSEncryptor is usable without newMockSOPSEncryptor.
func (m *mockSOPSEncryptor) cacheLocked(filePath string, content []byte) {
	if m.encryptedFiles == nil {
		m.encryptedFiles = make(map[string][]byte)
	}
	m.encryptedFiles[filePath] = content
}

func (m *mockSOPSEncryptor) EncryptFile(ctx context.Context, filePath string, config sops.EncryptionConfig) error {
	if m.encryptError != nil {
		return m.encryptError
	}

	// Read the file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Store encrypted content (in real SOPS, this would be encrypted)
	// For testing, we just store the plaintext and expose metadata separately.
	// Always update the cache to handle file renames. Recipient state is kept
	// on the mock rather than keyed by filePath because the registry encrypts a
	// temporary file before renaming it to its final path.
	m.mu.Lock()
	m.cacheLocked(filePath, content)
	m.encryptionRecipients = append([]string(nil), config.AgeKeys...)
	m.mu.Unlock()

	return nil
}

func (m *mockSOPSEncryptor) GetEncryptedContent(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		m.mu.Lock()
		cached, ok := m.encryptedFiles[filePath]
		m.mu.Unlock()
		if !ok {
			return "", err
		}
		content = cached
	}

	m.mu.Lock()
	recipients := append([]string(nil), m.encryptionRecipients...)
	m.mu.Unlock()
	if len(recipients) == 0 {
		return string(content), nil
	}

	var metadata strings.Builder
	metadata.WriteString("sops:\n  age:\n")
	for _, recipient := range recipients {
		fmt.Fprintf(&metadata, "    - recipient: %q\n", recipient)
	}
	return metadata.String(), nil
}
func (m *mockSOPSEncryptor) DecryptFile(ctx context.Context, filePath string) ([]byte, error) {
	if m.decryptError != nil {
		return nil, m.decryptError
	}

	// Always try reading from disk first to handle file renames
	content, err := os.ReadFile(filePath)
	if err == nil {
		// Update cache
		m.mu.Lock()
		m.cacheLocked(filePath, content)
		m.mu.Unlock()
		return content, nil
	}

	// Fallback to cached content if file doesn't exist
	m.mu.Lock()
	cachedContent, ok := m.encryptedFiles[filePath]
	m.mu.Unlock()

	if ok {
		return cachedContent, nil
	}

	// File not found
	return nil, err
}

func TestNewDefaultKeyRegistry(t *testing.T) {
	tempDir := t.TempDir()
	encryptor := newMockSOPSEncryptor()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	registry := NewDefaultKeyRegistry(tempDir, encryptor, logger)

	assert.NotNil(t, registry)
	assert.Equal(t, filepath.Join(tempDir, RegistryFileName), registry.registryPath)
	assert.NotNil(t, registry.encryptor)
	assert.NotNil(t, registry.logger)
}

func TestRegisterKey(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	encryptor := newMockSOPSEncryptor()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	registry := NewDefaultKeyRegistry(tempDir, encryptor, logger)

	t.Run("register new key", func(t *testing.T) {
		entry := KeyEntry{
			Cluster:     "test-cluster",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1test123",
			PublicKey:   "age1test123",
		}

		err := registry.RegisterKey(ctx, entry)
		require.NoError(t, err)

		// Verify key was registered
		retrieved, err := registry.GetKey(ctx, "test-cluster", KeyTypeAge)
		require.NoError(t, err)
		assert.Equal(t, "test-cluster", retrieved.Cluster)
		assert.Equal(t, KeyTypeAge, retrieved.KeyType)
		assert.Equal(t, "age1test123", retrieved.Fingerprint)
		assert.Equal(t, KeyStatusActive, retrieved.Status)
		assert.False(t, retrieved.CreatedAt.IsZero())
		assert.False(t, retrieved.ExpiresAt.IsZero())
	})

	t.Run("register key with explicit timestamps", func(t *testing.T) {
		createdAt := time.Now().Add(-30 * 24 * time.Hour)
		expiresAt := time.Now().Add(60 * 24 * time.Hour)

		entry := KeyEntry{
			Cluster:     "test-cluster-2",
			KeyType:     KeyTypeSSH,
			Fingerprint: "ssh-test123",
			PublicKey:   "ssh-rsa AAAAB3...",
			CreatedAt:   createdAt,
			ExpiresAt:   expiresAt,
			Status:      KeyStatusActive,
		}

		err := registry.RegisterKey(ctx, entry)
		require.NoError(t, err)

		// Verify timestamps were preserved
		retrieved, err := registry.GetKey(ctx, "test-cluster-2", KeyTypeSSH)
		require.NoError(t, err)
		assert.Equal(t, createdAt.Unix(), retrieved.CreatedAt.Unix())
		assert.Equal(t, expiresAt.Unix(), retrieved.ExpiresAt.Unix())
	})

	t.Run("register duplicate fingerprint fails", func(t *testing.T) {
		entry := KeyEntry{
			Cluster:     "test-cluster",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1test123",
			PublicKey:   "age1test123",
		}

		err := registry.RegisterKey(ctx, entry)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("register second active key for same cluster succeeds", func(t *testing.T) {
		// SOPS encrypts to multiple age recipients at once, and dual-key
		// rotation needs the old and new keys active at the same time.
		entry := KeyEntry{
			Cluster:     "test-cluster",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1second",
			PublicKey:   "age1second",
		}

		err := registry.RegisterKey(ctx, entry)
		require.NoError(t, err)

		keys, err := registry.ListKeys(ctx, "test-cluster")
		require.NoError(t, err)

		activeAge := 0
		for _, key := range keys {
			if key.KeyType == KeyTypeAge && key.Status == KeyStatusActive {
				activeAge++
			}
		}
		assert.Equal(t, 2, activeAge)

		// GetKey stays deterministic: the earliest-registered active key.
		retrieved, err := registry.GetKey(ctx, "test-cluster", KeyTypeAge)
		require.NoError(t, err)
		assert.Equal(t, "age1test123", retrieved.Fingerprint)
	})

	t.Run("register archived fingerprint again fails", func(t *testing.T) {
		entry := KeyEntry{
			Cluster:     "archive-cluster",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1archived",
			PublicKey:   "age1archived",
		}
		require.NoError(t, registry.RegisterKey(ctx, entry))
		require.NoError(t, registry.UpdateKeyStatus(ctx, "archive-cluster", KeyTypeAge, KeyStatusArchived))

		err := registry.RegisterKey(ctx, entry)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("register key with default expiration", func(t *testing.T) {
		entry := KeyEntry{
			Cluster:     "test-cluster-3",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1expiration",
			PublicKey:   "age1expiration",
		}

		err := registry.RegisterKey(ctx, entry)
		require.NoError(t, err)

		retrieved, err := registry.GetKey(ctx, "test-cluster-3", KeyTypeAge)
		require.NoError(t, err)

		// Verify expiration is set to default (90 days for Age)
		expectedExpiration := retrieved.CreatedAt.AddDate(0, 0, DefaultAgeExpirationDays)
		assert.Equal(t, expectedExpiration.Unix(), retrieved.ExpiresAt.Unix())
	})
}

func TestGetKey(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	encryptor := newMockSOPSEncryptor()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	registry := NewDefaultKeyRegistry(tempDir, encryptor, logger)

	// Register a test key
	entry := KeyEntry{
		Cluster:     "test-cluster",
		KeyType:     KeyTypeAge,
		Fingerprint: "age1test123",
		PublicKey:   "age1test123",
	}
	err := registry.RegisterKey(ctx, entry)
	require.NoError(t, err)

	t.Run("get existing key", func(t *testing.T) {
		retrieved, err := registry.GetKey(ctx, "test-cluster", KeyTypeAge)
		require.NoError(t, err)
		assert.Equal(t, "test-cluster", retrieved.Cluster)
		assert.Equal(t, KeyTypeAge, retrieved.KeyType)
		assert.Equal(t, "age1test123", retrieved.Fingerprint)
	})

	t.Run("get non-existent key", func(t *testing.T) {
		_, err := registry.GetKey(ctx, "non-existent", KeyTypeAge)
		assert.Error(t, err)
		assert.True(t, IsKeyNotFoundError(err))
	})

	t.Run("get wrong key type", func(t *testing.T) {
		_, err := registry.GetKey(ctx, "test-cluster", KeyTypeSSH)
		assert.Error(t, err)
		assert.True(t, IsKeyNotFoundError(err))
	})
}

func TestUpdateKeyStatus(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	encryptor := newMockSOPSEncryptor()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	registry := NewDefaultKeyRegistry(tempDir, encryptor, logger)

	// Register a test key
	entry := KeyEntry{
		Cluster:     "test-cluster",
		KeyType:     KeyTypeAge,
		Fingerprint: "age1test123",
		PublicKey:   "age1test123",
	}
	err := registry.RegisterKey(ctx, entry)
	require.NoError(t, err)

	// Keep an independent active Age recipient so lifecycle tests can verify
	// that archived/revoked entries are removed from the encryption envelope.
	require.NoError(t, registry.RegisterKey(ctx, KeyEntry{
		Cluster:     "envelope-keeper",
		KeyType:     KeyTypeAge,
		Fingerprint: "age1keeper",
		PublicKey:   "age1keeper",
	}))

	t.Run("update to archived", func(t *testing.T) {
		err := registry.UpdateKeyStatus(ctx, "test-cluster", KeyTypeAge, KeyStatusArchived)
		require.NoError(t, err)

		// Verify status was updated
		keys, err := registry.ListKeys(ctx, "test-cluster")
		require.NoError(t, err)
		assert.Equal(t, 1, len(keys))
		assert.Equal(t, KeyStatusArchived, keys[0].Status)
	})

	t.Run("update to revoked sets timestamp", func(t *testing.T) {
		// Register another key
		entry2 := KeyEntry{
			Cluster:     "test-cluster-2",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1test456",
			PublicKey:   "age1test456",
		}
		err := registry.RegisterKey(ctx, entry2)
		require.NoError(t, err)

		// Update to revoked
		err = registry.UpdateKeyStatus(ctx, "test-cluster-2", KeyTypeAge, KeyStatusRevoked)
		require.NoError(t, err)

		// Verify revocation timestamp was set
		keys, err := registry.ListKeys(ctx, "test-cluster-2")
		require.NoError(t, err)
		assert.Equal(t, 1, len(keys))
		assert.Equal(t, KeyStatusRevoked, keys[0].Status)
		assert.False(t, keys[0].RevokedAt.IsZero())
	})

	t.Run("update non-existent key fails", func(t *testing.T) {
		err := registry.UpdateKeyStatus(ctx, "non-existent", KeyTypeAge, KeyStatusArchived)
		assert.Error(t, err)
		assert.True(t, IsKeyNotFoundError(err))
	})
}

func TestUpdateKey(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	encryptor := newMockSOPSEncryptor()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	registry := NewDefaultKeyRegistry(tempDir, encryptor, logger)

	entry := KeyEntry{
		Cluster:     "test-cluster",
		KeyType:     KeyTypeAge,
		Fingerprint: "age1test123",
		PublicKey:   "age1test123",
		Status:      KeyStatusActive,
		UserEmail:   "alice@example.com",
	}
	require.NoError(t, registry.RegisterKey(ctx, KeyEntry{
		Cluster:     "envelope-keeper",
		KeyType:     KeyTypeAge,
		Fingerprint: "age1keeper",
		PublicKey:   "age1keeper",
	}))
	err := registry.RegisterKey(ctx, entry)
	require.NoError(t, err)

	entry.Status = KeyStatusRevoked
	entry.RevokedBy = "bob@example.com"
	entry.RevokedReason = "compromised"
	entry.RevokedAt = time.Now().UTC().Truncate(time.Second)

	err = registry.UpdateKey(ctx, entry)
	require.NoError(t, err)

	keys, err := registry.ListKeys(ctx, "test-cluster")
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, KeyStatusRevoked, keys[0].Status)
	assert.Equal(t, "bob@example.com", keys[0].RevokedBy)
	assert.Equal(t, "compromised", keys[0].RevokedReason)
	assert.Equal(t, "alice@example.com", keys[0].UserEmail)
	assert.False(t, keys[0].RevokedAt.IsZero())
}

func TestListKeys(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	encryptor := newMockSOPSEncryptor()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	registry := NewDefaultKeyRegistry(tempDir, encryptor, logger)

	// Register multiple keys
	keys := []KeyEntry{
		{
			Cluster:     "cluster-1",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1cluster1",
			PublicKey:   "age1cluster1",
		},
		{
			Cluster:     "cluster-1",
			KeyType:     KeyTypeSSH,
			Fingerprint: "ssh-cluster1",
			PublicKey:   "ssh-rsa AAAAB3...",
		},
		{
			Cluster:     "cluster-2",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1cluster2",
			PublicKey:   "age1cluster2",
		},
	}

	for _, key := range keys {
		err := registry.RegisterKey(ctx, key)
		require.NoError(t, err)
	}

	t.Run("list all keys", func(t *testing.T) {
		allKeys, err := registry.ListKeys(ctx, "")
		require.NoError(t, err)
		assert.Equal(t, 3, len(allKeys))
	})

	t.Run("list keys for specific cluster", func(t *testing.T) {
		clusterKeys, err := registry.ListKeys(ctx, "cluster-1")
		require.NoError(t, err)
		assert.Equal(t, 2, len(clusterKeys))

		for _, key := range clusterKeys {
			assert.Equal(t, "cluster-1", key.Cluster)
		}
	})

	t.Run("list keys for cluster with no keys", func(t *testing.T) {
		clusterKeys, err := registry.ListKeys(ctx, "non-existent")
		require.NoError(t, err)
		assert.Equal(t, 0, len(clusterKeys))
	})
}

func TestCheckExpiration(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	encryptor := newMockSOPSEncryptor()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	registry := NewDefaultKeyRegistry(tempDir, encryptor, logger)

	now := time.Now()

	// Register keys with different expiration states
	keys := []KeyEntry{
		{
			Cluster:     "expired-cluster",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1expired",
			PublicKey:   "age1expired",
			CreatedAt:   now.AddDate(0, 0, -100),
			ExpiresAt:   now.AddDate(0, 0, -10), // Expired 10 days ago
			Status:      KeyStatusActive,
		},
		{
			Cluster:     "warning-cluster",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1warning",
			PublicKey:   "age1warning",
			CreatedAt:   now.AddDate(0, 0, -80),
			ExpiresAt:   now.AddDate(0, 0, 10), // Expires in 10 days
			Status:      KeyStatusActive,
		},
		{
			Cluster:     "valid-cluster",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1valid",
			PublicKey:   "age1valid",
			CreatedAt:   now,
			ExpiresAt:   now.AddDate(0, 0, 90), // Expires in 90 days
			Status:      KeyStatusActive,
		},
		{
			Cluster:     "archived-cluster",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1archived",
			PublicKey:   "age1archived",
			CreatedAt:   now.AddDate(0, 0, -200),
			ExpiresAt:   now.AddDate(0, 0, -100), // Expired but archived
			Status:      KeyStatusArchived,
		},
	}

	for _, key := range keys {
		err := registry.RegisterKey(ctx, key)
		require.NoError(t, err)
	}

	t.Run("check expiration with 14 day warning", func(t *testing.T) {
		report, err := registry.CheckExpiration(ctx, 14)
		require.NoError(t, err)

		assert.Equal(t, 1, len(report.Expired), "Should have 1 expired key")
		assert.Equal(t, 1, len(report.Warning), "Should have 1 warning key")
		assert.Equal(t, 1, len(report.Valid), "Should have 1 valid key")

		// Verify expired key
		assert.Equal(t, "expired-cluster", report.Expired[0].Cluster)
		assert.True(t, report.Expired[0].DaysRemaining < 0)

		// Verify warning key
		assert.Equal(t, "warning-cluster", report.Warning[0].Cluster)
		assert.True(t, report.Warning[0].DaysRemaining > 0)
		assert.True(t, report.Warning[0].DaysRemaining <= 14)

		// Verify valid key
		assert.Equal(t, "valid-cluster", report.Valid[0].Cluster)
		assert.True(t, report.Valid[0].DaysRemaining > 14)
	})

	t.Run("check expiration with 30 day warning", func(t *testing.T) {
		report, err := registry.CheckExpiration(ctx, 30)
		require.NoError(t, err)

		assert.Equal(t, 1, len(report.Expired))
		assert.Equal(t, 1, len(report.Warning))
		assert.Equal(t, 1, len(report.Valid))
	})
}

func TestRebuildFromFiles(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	encryptor := newMockSOPSEncryptor()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	registry := NewDefaultKeyRegistry(tempDir, encryptor, logger)

	// Create test key files
	keysDir := filepath.Join(tempDir, "keys")
	ageKeysDir := filepath.Join(keysDir, "age")
	sshKeysDir := filepath.Join(keysDir, "ssh")

	err := os.MkdirAll(ageKeysDir, 0o700)
	require.NoError(t, err)
	err = os.MkdirAll(sshKeysDir, 0o700)
	require.NoError(t, err)

	// Create Age key files
	err = os.WriteFile(filepath.Join(ageKeysDir, "cluster-1.pub"), []byte("age1test123"), 0o644)
	require.NoError(t, err)

	// Create SSH key files
	err = os.WriteFile(filepath.Join(sshKeysDir, "cluster-2.pub"), []byte("ssh-rsa AAAAB3..."), 0o644)
	require.NoError(t, err)

	t.Run("rebuild from files", func(t *testing.T) {
		err := registry.RebuildFromFiles(ctx, keysDir)
		require.NoError(t, err)

		// Verify keys were added
		keys, err := registry.ListKeys(ctx, "")
		require.NoError(t, err)
		assert.Equal(t, 2, len(keys))

		// Verify Age key
		ageKey, err := registry.GetKey(ctx, "cluster-1", KeyTypeAge)
		require.NoError(t, err)
		assert.Equal(t, "age1test123", ageKey.PublicKey)
		assert.Equal(t, KeyStatusActive, ageKey.Status)

		// Verify SSH key
		sshKey, err := registry.GetKey(ctx, "cluster-2", KeyTypeSSH)
		require.NoError(t, err)
		assert.Equal(t, "ssh-rsa AAAAB3...", sshKey.PublicKey)
		assert.Equal(t, KeyStatusActive, sshKey.Status)
	})

	t.Run("rebuild with non-existent directory preserves the existing registry", func(t *testing.T) {
		before, err := os.ReadFile(registry.registryPath)
		require.NoError(t, err)

		err = registry.RebuildFromFiles(ctx, filepath.Join(tempDir, "non-existent"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no active Age recipients")

		after, err := os.ReadFile(registry.registryPath)
		require.NoError(t, err)
		assert.Equal(t, before, after)

		encryptor.mu.Lock()
		assert.Equal(t, []string{"age1test123"}, encryptor.encryptionRecipients)
		encryptor.mu.Unlock()
	})

	t.Run("rebuild with multiple keys per cluster type", func(t *testing.T) {
		// Create multiple Age keys
		err := os.WriteFile(filepath.Join(ageKeysDir, "cluster-3.pub"), []byte("age1test456"), 0o644)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(ageKeysDir, "cluster-4.pub"), []byte("age1test789"), 0o644)
		require.NoError(t, err)

		err = registry.RebuildFromFiles(ctx, keysDir)
		require.NoError(t, err)

		// Verify all keys were added
		keys, err := registry.ListKeys(ctx, "")
		require.NoError(t, err)
		assert.Equal(t, 4, len(keys))
	})

	t.Run("rebuild ignores non-pub files", func(t *testing.T) {
		// Create non-.pub files that should be ignored
		err := os.WriteFile(filepath.Join(ageKeysDir, "cluster-5.txt"), []byte("age1test999"), 0o644)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(sshKeysDir, "cluster-6.key"), []byte("ssh-rsa AAAAB3..."), 0o644)
		require.NoError(t, err)

		err = registry.RebuildFromFiles(ctx, keysDir)
		require.NoError(t, err)

		// Verify non-.pub files were ignored
		keys, err := registry.ListKeys(ctx, "")
		require.NoError(t, err)
		assert.Equal(t, 4, len(keys)) // Should still be 4 from previous test
	})

	t.Run("rebuild sets correct expiration dates", func(t *testing.T) {
		err := registry.RebuildFromFiles(ctx, keysDir)
		require.NoError(t, err)

		// Verify Age key has correct expiration (90 days from creation)
		ageKey, err := registry.GetKey(ctx, "cluster-1", KeyTypeAge)
		require.NoError(t, err)
		expectedAgeExpiration := ageKey.CreatedAt.AddDate(0, 0, DefaultAgeExpirationDays)
		assert.Equal(t, expectedAgeExpiration.Unix(), ageKey.ExpiresAt.Unix())

		// Verify SSH key has correct expiration (180 days from creation)
		sshKey, err := registry.GetKey(ctx, "cluster-2", KeyTypeSSH)
		require.NoError(t, err)
		expectedSSHExpiration := sshKey.CreatedAt.AddDate(0, 0, DefaultSSHExpirationDays)
		assert.Equal(t, expectedSSHExpiration.Unix(), sshKey.ExpiresAt.Unix())
	})

	t.Run("rebuild replaces existing registry", func(t *testing.T) {
		// First rebuild
		err := registry.RebuildFromFiles(ctx, keysDir)
		require.NoError(t, err)

		keys1, err := registry.ListKeys(ctx, "")
		require.NoError(t, err)
		initialCount := len(keys1)

		// Add a new key file
		err = os.WriteFile(filepath.Join(ageKeysDir, "cluster-new.pub"), []byte("age1testnew"), 0o644)
		require.NoError(t, err)

		// Rebuild again
		err = registry.RebuildFromFiles(ctx, keysDir)
		require.NoError(t, err)

		// Verify new key was added
		keys2, err := registry.ListKeys(ctx, "")
		require.NoError(t, err)
		assert.Equal(t, initialCount+1, len(keys2))

		// Verify new key exists
		newKey, err := registry.GetKey(ctx, "cluster-new", KeyTypeAge)
		require.NoError(t, err)
		assert.Equal(t, "age1testnew", newKey.PublicKey)
	})
}

func TestRegistryPersistence(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	encryptor := newMockSOPSEncryptor()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("registry persists across instances", func(t *testing.T) {
		// Create first registry instance and add a key
		registry1 := NewDefaultKeyRegistry(tempDir, encryptor, logger)
		entry := KeyEntry{
			Cluster:     "test-cluster",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1test123",
			PublicKey:   "age1test123",
		}
		err := registry1.RegisterKey(ctx, entry)
		require.NoError(t, err)

		// Create second registry instance and verify key exists
		registry2 := NewDefaultKeyRegistry(tempDir, encryptor, logger)
		retrieved, err := registry2.GetKey(ctx, "test-cluster", KeyTypeAge)
		require.NoError(t, err)
		assert.Equal(t, "age1test123", retrieved.Fingerprint)
	})
}

func TestRegistryThreadSafety(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	encryptor := newMockSOPSEncryptor()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	registry := NewDefaultKeyRegistry(tempDir, encryptor, logger)

	t.Run("concurrent operations", func(t *testing.T) {
		// Register initial key
		entry := KeyEntry{
			Cluster:     "test-cluster",
			KeyType:     KeyTypeAge,
			Fingerprint: "age1test123",
			PublicKey:   "age1test123",
		}
		err := registry.RegisterKey(ctx, entry)
		require.NoError(t, err)

		// Perform concurrent reads and writes
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func(id int) {
				// Read operation
				_, _ = registry.GetKey(ctx, "test-cluster", KeyTypeAge)

				// List operation
				_, _ = registry.ListKeys(ctx, "")

				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func TestDefaultKeyRegistryConcurrentInstances(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	first := NewDefaultKeyRegistry(dir, newMockSOPSEncryptor(), logger)
	second := NewDefaultKeyRegistry(dir, newMockSOPSEncryptor(), logger)
	ctx := context.Background()
	errs := make(chan error, 2)
	go func() {
		errs <- first.RegisterKey(ctx, KeyEntry{Cluster: "cluster", KeyType: KeyTypeAge, Fingerprint: "age-one", PublicKey: "age-one"})
	}()
	go func() {
		errs <- second.RegisterKey(ctx, KeyEntry{Cluster: "cluster", KeyType: KeyTypeAge, Fingerprint: "age-two", PublicKey: "age-two"})
	}()
	for i := 0; i < 2; i++ {
		require.NoError(t, <-errs)
	}
	keys, err := first.ListKeys(ctx, "cluster")
	require.NoError(t, err)
	require.Len(t, keys, 2)
}

func TestRegistryUsesOnlyActiveAgeRecipients(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	encryptor := newMockSOPSEncryptor()
	registry := NewDefaultKeyRegistry(dir, encryptor, slog.Default())

	require.NoError(t, registry.RegisterKey(ctx, KeyEntry{Cluster: "cluster", KeyType: KeyTypeAge, Fingerprint: "age-old", PublicKey: "age-old"}))
	require.NoError(t, registry.RegisterKey(ctx, KeyEntry{Cluster: "cluster", KeyType: KeyTypeAge, Fingerprint: "age-new", PublicKey: "age-new"}))
	encryptor.mu.Lock()
	recipients := append([]string(nil), encryptor.encryptionRecipients...)
	encryptor.mu.Unlock()
	assert.Equal(t, []string{"age-old", "age-new"}, recipients)

	require.NoError(t, registry.UpdateKey(ctx, KeyEntry{
		Cluster: "cluster", KeyType: KeyTypeAge, Fingerprint: "age-old", Status: KeyStatusArchived,
	}))
	encryptor.mu.Lock()
	recipients = append([]string(nil), encryptor.encryptionRecipients...)
	encryptor.mu.Unlock()
	assert.Equal(t, []string{"age-new"}, recipients)

	keys, err := registry.ListKeys(ctx, "cluster")
	require.NoError(t, err)
	for _, key := range keys {
		if key.Status != KeyStatusActive {
			assert.NotContains(t, recipients, key.PublicKey)
		}
	}
}

func TestUpdateKeysIsAllOrNothing(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	registry := NewDefaultKeyRegistry(dir, newMockSOPSEncryptor(), slog.Default())
	for _, fingerprint := range []string{"age-a", "age-b"} {
		require.NoError(t, registry.RegisterKey(ctx, KeyEntry{
			Cluster: "cluster", KeyType: KeyTypeAge, Fingerprint: fingerprint, PublicKey: fingerprint,
		}))
	}
	before, err := os.ReadFile(registry.registryPath)
	require.NoError(t, err)

	err = registry.UpdateKeys(ctx, []KeyEntry{
		{Cluster: "cluster", KeyType: KeyTypeAge, Fingerprint: "age-a", Status: KeyStatusRevoked},
		{Cluster: "cluster", KeyType: KeyTypeAge, Fingerprint: "missing", Status: KeyStatusRevoked},
	})
	require.Error(t, err)
	after, err := os.ReadFile(registry.registryPath)
	require.NoError(t, err)
	assert.Equal(t, before, after)

	keys, err := registry.ListKeys(ctx, "cluster")
	require.NoError(t, err)
	for _, key := range keys {
		assert.Equal(t, KeyStatusActive, key.Status)
	}
}

func TestRegistryPrimaryReplacementTransactions(t *testing.T) {
	ctx := context.Background()
	registry := NewDefaultKeyRegistry(t.TempDir(), newMockSOPSEncryptor(), slog.Default())

	require.NoError(t, registry.RegisterKey(ctx, KeyEntry{
		Cluster: "age-cluster", KeyType: KeyTypeAge, Fingerprint: "age-old", PublicKey: "age-old", Primary: true,
	}))
	require.NoError(t, registry.ReplacePrimary(ctx, "age-old", KeyEntry{
		Cluster: "age-cluster", KeyType: KeyTypeAge, Fingerprint: "age-new", PublicKey: "age-new",
	}))
	ageKeys, err := registry.ListKeys(ctx, "age-cluster")
	require.NoError(t, err)
	for _, key := range ageKeys {
		if key.Fingerprint == "age-old" {
			assert.Equal(t, KeyStatusActive, key.Status)
			assert.False(t, key.Primary)
		}
		if key.Fingerprint == "age-new" {
			assert.Equal(t, KeyStatusActive, key.Status)
			assert.True(t, key.Primary)
		}
	}

	require.NoError(t, registry.RegisterKey(ctx, KeyEntry{
		Cluster: "ssh-cluster", KeyType: KeyTypeSSH, Fingerprint: "ssh-old", PublicKey: "ssh-old", Primary: true,
	}))
	require.NoError(t, registry.ReplacePrimaryAndArchive(ctx, "ssh-old", KeyEntry{
		Cluster: "ssh-cluster", KeyType: KeyTypeSSH, Fingerprint: "ssh-new", PublicKey: "ssh-new",
	}))
	sshKeys, err := registry.ListKeys(ctx, "ssh-cluster")
	require.NoError(t, err)
	for _, key := range sshKeys {
		if key.Fingerprint == "ssh-old" {
			assert.Equal(t, KeyStatusArchived, key.Status)
			assert.False(t, key.Primary)
		}
		if key.Fingerprint == "ssh-new" {
			assert.Equal(t, KeyStatusActive, key.Status)
			assert.True(t, key.Primary)
		}
	}
}

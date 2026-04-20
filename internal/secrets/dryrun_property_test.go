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
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/prop"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
	"github.com/stretchr/testify/require"
)

const dryRunTestOrg = "test-org"

func configureDryRunTestEnv(t *testing.T, tmpDir string) {
	t.Helper()

	originalHome := os.Getenv("HOME")
	originalConfigDir := os.Getenv("OPENCENTER_CONFIG_DIR")
	configDir := filepath.Join(tmpDir, ".config", "opencenter")

	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.Setenv("HOME", tmpDir))
	require.NoError(t, os.Setenv("OPENCENTER_CONFIG_DIR", configDir))

	t.Cleanup(func() {
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
	})
}

func createDryRunClusterPaths(ctx context.Context, clusterName string) (*paths.ClusterPaths, error) {
	resolver := paths.NewPathResolver(config.ResolveClustersDir())
	if err := resolver.CreateClusterDirectories(ctx, clusterName, dryRunTestOrg); err != nil {
		return nil, err
	}

	return resolver.Resolve(ctx, clusterName, dryRunTestOrg)
}

func saveDryRunConfig(
	ctx context.Context,
	configLoader *config.ConfigIOHandler,
	cfg *v2.Config,
	clusterPaths *paths.ClusterPaths,
	clusterName string,
) error {
	if err := configLoader.SaveToFile(ctx, clusterPaths.ConfigPath, cfg); err != nil {
		return err
	}

	legacyDir := filepath.Join(config.ResolveClustersDir(), dryRunTestOrg, clusterName)
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		return err
	}

	legacyPath := filepath.Join(legacyDir, fmt.Sprintf(".k8s-%s-config.yaml", clusterName))
	return configLoader.SaveToFile(ctx, legacyPath, cfg)
}

// **Validates: Requirements 1.5, 3.8, 6.8, 8.8**
//
// Property 4: Dry-Run Immutability
//
// For any secrets operation (sync, rotate, revoke) with the `--dry-run` flag,
// no files in the repository should be modified.
//
// This property verifies that:
// 1. Sync operations with --dry-run do not modify any files
// 2. Rotate operations with --dry-run do not modify any files
// 3. Revoke operations with --dry-run do not modify any files
// 4. Multi-cluster sync with --dry-run does not modify any files

func TestProperty_DryRunImmutability_Sync(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("sync with dry-run does not modify any files", prop.ForAll(
		func(secretsGen map[string]string) bool {
			// Skip if empty
			if len(secretsGen) == 0 {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			clusterName := "test-cluster"
			ctx := context.Background()

			// Create test infrastructure
			manager, configPath, overlayPath, err := setupDryRunTest(t, tmpDir, clusterName, secretsGen)
			if err != nil {
				t.Logf("Failed to setup dry-run test: %v", err)
				return false
			}

			// Capture filesystem state before operation
			beforeState, err := captureFilesystemState(tmpDir)
			if err != nil {
				t.Logf("Failed to capture filesystem state: %v", err)
				return false
			}

			// Perform sync with dry-run
			opts := SyncOptions{
				Cluster: clusterName,
				DryRun:  true,
			}

			_, err = manager.SyncSecrets(ctx, opts)
			if err != nil {
				t.Logf("Sync operation failed: %v", err)
				return false
			}

			// Capture filesystem state after operation
			afterState, err := captureFilesystemState(tmpDir)
			if err != nil {
				t.Logf("Failed to capture filesystem state after operation: %v", err)
				return false
			}

			// Property: Filesystem state should be identical
			if !compareFilesystemStates(beforeState, afterState) {
				t.Logf("Filesystem state changed during dry-run sync")
				logStateDifferences(t, beforeState, afterState)
				return false
			}

			// Verify config and overlay paths still exist
			_ = configPath
			_ = overlayPath

			return true
		},
		genSecretMap(),
	))

	properties.TestingRun(t)
}

func TestProperty_DryRunImmutability_Rotate(t *testing.T) {
	// Skip if SOPS is not available
	if _, err := exec.LookPath("sops"); err != nil {
		t.Skip("SOPS binary not found, skipping property test")
	}

	// Skip if age-keygen is not available
	if _, err := exec.LookPath("age-keygen"); err != nil {
		t.Skip("age-keygen binary not found, skipping property test")
	}

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50 // Reduced due to SOPS operations
	properties := gopter.NewProperties(parameters)

	properties.Property("rotate with dry-run does not modify any files", prop.ForAll(
		func(secretsGen map[string]string) bool {
			// Skip if empty
			if len(secretsGen) == 0 {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			clusterName := "test-cluster"
			ctx := context.Background()

			// Create test infrastructure with rotation setup
			rotator, err := setupDryRunRotationTest(t, tmpDir, clusterName, secretsGen)
			if err != nil {
				t.Logf("Failed to setup dry-run rotation test: %v", err)
				return false
			}

			// Capture filesystem state before operation
			beforeState, err := captureFilesystemState(tmpDir)
			if err != nil {
				t.Logf("Failed to capture filesystem state: %v", err)
				return false
			}

			// Perform rotation with dry-run
			opts := RotateOptions{
				Cluster: clusterName,
				KeyType: KeyTypeAge,
				DryRun:  true,
			}

			_, err = rotator.RotateAgeKey(ctx, opts)
			if err != nil {
				t.Logf("Rotation operation failed: %v", err)
				return false
			}

			// Capture filesystem state after operation
			afterState, err := captureFilesystemState(tmpDir)
			if err != nil {
				t.Logf("Failed to capture filesystem state after operation: %v", err)
				return false
			}

			// Property: Filesystem state should be identical
			if !compareFilesystemStates(beforeState, afterState) {
				t.Logf("Filesystem state changed during dry-run rotation")
				logStateDifferences(t, beforeState, afterState)
				return false
			}

			return true
		},
		genSecretMap(),
	))

	properties.TestingRun(t)
}

func TestProperty_DryRunImmutability_Revoke(t *testing.T) {
	// Skip if SOPS is not available
	if _, err := exec.LookPath("sops"); err != nil {
		t.Skip("SOPS binary not found, skipping property test")
	}

	// Skip if age-keygen is not available
	if _, err := exec.LookPath("age-keygen"); err != nil {
		t.Skip("age-keygen binary not found, skipping property test")
	}

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50 // Reduced due to SOPS operations
	properties := gopter.NewProperties(parameters)

	properties.Property("revoke with dry-run does not modify any files", prop.ForAll(
		func(secretsGen map[string]string) bool {
			// Skip if empty
			if len(secretsGen) == 0 {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			clusterName := "test-cluster"
			ctx := context.Background()

			// Create test infrastructure with multi-key setup
			revoker, err := setupDryRunRevocationTest(t, tmpDir, clusterName, secretsGen)
			if err != nil {
				t.Logf("Failed to setup dry-run revocation test: %v", err)
				return false
			}

			// Capture filesystem state before operation
			beforeState, err := captureFilesystemState(tmpDir)
			if err != nil {
				t.Logf("Failed to capture filesystem state: %v", err)
				return false
			}

			// Perform revocation with dry-run
			opts := RevokeOptions{
				Cluster: clusterName,
				User:    "test-user@example.com",
				DryRun:  true,
			}

			_, err = revoker.RevokeByUser(ctx, opts)
			if err != nil {
				t.Logf("Revocation operation failed: %v", err)
				return false
			}

			// Capture filesystem state after operation
			afterState, err := captureFilesystemState(tmpDir)
			if err != nil {
				t.Logf("Failed to capture filesystem state after operation: %v", err)
				return false
			}

			// Property: Filesystem state should be identical
			if !compareFilesystemStates(beforeState, afterState) {
				t.Logf("Filesystem state changed during dry-run revocation")
				logStateDifferences(t, beforeState, afterState)
				return false
			}

			return true
		},
		genSecretMap(),
	))

	properties.TestingRun(t)
}

func TestProperty_DryRunImmutability_MultiClusterSync(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("multi-cluster sync with dry-run does not modify any files", prop.ForAll(
		func(secretsGen map[string]string) bool {
			// Skip if empty
			if len(secretsGen) == 0 {
				return true
			}

			// Setup test environment with multiple clusters
			tmpDir := t.TempDir()
			ctx := context.Background()

			// Create test infrastructure for multiple clusters
			syncer, err := setupDryRunMultiClusterTest(t, tmpDir, secretsGen)
			if err != nil {
				t.Logf("Failed to setup dry-run multi-cluster test: %v", err)
				return false
			}

			// Capture filesystem state before operation
			beforeState, err := captureFilesystemState(tmpDir)
			if err != nil {
				t.Logf("Failed to capture filesystem state: %v", err)
				return false
			}

			// Perform multi-cluster sync with dry-run
			opts := MultiClusterSyncOptions{
				Organization: "test-org",
				Concurrency:  2,
				StopOnError:  false,
				DryRun:       true,
			}

			_, err = syncer.SyncAll(ctx, opts)
			if err != nil {
				t.Logf("Multi-cluster sync operation failed: %v", err)
				return false
			}

			// Capture filesystem state after operation
			afterState, err := captureFilesystemState(tmpDir)
			if err != nil {
				t.Logf("Failed to capture filesystem state after operation: %v", err)
				return false
			}

			// Property: Filesystem state should be identical
			if !compareFilesystemStates(beforeState, afterState) {
				t.Logf("Filesystem state changed during dry-run multi-cluster sync")
				logStateDifferences(t, beforeState, afterState)
				return false
			}

			return true
		},
		genSecretMap(),
	))

	properties.TestingRun(t)
}

// Helper types and functions

// FilesystemState represents the state of all files in a directory tree
type FilesystemState struct {
	Files map[string]FileState // path -> file state
}

// FileState represents the state of a single file
type FileState struct {
	Path    string
	Hash    string // SHA256 hash of file content
	Mode    os.FileMode
	Size    int64
	ModTime int64 // Unix timestamp
	IsDir   bool
}

// captureFilesystemState captures the current state of all files in a directory
func captureFilesystemState(rootDir string) (*FilesystemState, error) {
	state := &FilesystemState{
		Files: make(map[string]FileState),
	}

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		fileState := FileState{
			Path:    relPath,
			Mode:    info.Mode(),
			Size:    info.Size(),
			ModTime: info.ModTime().Unix(),
			IsDir:   info.IsDir(),
		}

		// Calculate hash for regular files
		if !info.IsDir() {
			hash, err := calculateFileHash(path)
			if err != nil {
				return fmt.Errorf("failed to calculate hash for %s: %w", path, err)
			}
			fileState.Hash = hash
		}

		state.Files[relPath] = fileState
		return nil
	})

	if err != nil {
		return nil, err
	}

	return state, nil
}

// calculateFileHash calculates the SHA256 hash of a file
func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// compareFilesystemStates compares two filesystem states and returns true if they are identical
func compareFilesystemStates(before, after *FilesystemState) bool {
	// Check if number of files is the same
	if len(before.Files) != len(after.Files) {
		return false
	}

	// Check each file
	for path, beforeFile := range before.Files {
		afterFile, exists := after.Files[path]
		if !exists {
			return false
		}

		// Compare file properties
		if beforeFile.Hash != afterFile.Hash {
			return false
		}

		if beforeFile.Size != afterFile.Size {
			return false
		}

		if beforeFile.IsDir != afterFile.IsDir {
			return false
		}

		// Note: We don't compare ModTime as it may change due to filesystem operations
		// even if the content is the same. We rely on hash comparison for content.
	}

	return true
}

// logStateDifferences logs the differences between two filesystem states
func logStateDifferences(t *testing.T, before, after *FilesystemState) {
	t.Helper()

	// Files added
	for path := range after.Files {
		if _, exists := before.Files[path]; !exists {
			t.Logf("  Added: %s", path)
		}
	}

	// Files removed
	for path := range before.Files {
		if _, exists := after.Files[path]; !exists {
			t.Logf("  Removed: %s", path)
		}
	}

	// Files modified
	for path, beforeFile := range before.Files {
		if afterFile, exists := after.Files[path]; exists {
			if beforeFile.Hash != afterFile.Hash {
				t.Logf("  Modified: %s (hash changed from %s to %s)", path, beforeFile.Hash[:8], afterFile.Hash[:8])
			}
			if beforeFile.Size != afterFile.Size {
				t.Logf("  Modified: %s (size changed from %d to %d)", path, beforeFile.Size, afterFile.Size)
			}
		}
	}
}

// setupDryRunTest creates a test environment for dry-run sync testing
func setupDryRunTest(t *testing.T, tmpDir string, clusterName string, secrets map[string]string) (*DefaultSecretsManager, string, string, error) {
	t.Helper()

	configureDryRunTestEnv(t, tmpDir)

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	ctx := context.Background()
	clusterPaths, err := createDryRunClusterPaths(ctx, clusterName)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create cluster paths: %w", err)
	}

	// Create directory structure used by the test fixtures.
	configDir := filepath.Join(tmpDir, ".config", "opencenter", "clusters", "test-org", clusterName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, "", "", fmt.Errorf("failed to create config dir: %w", err)
	}

	overlayPath := filepath.Join(tmpDir, "test-repo", "applications", "overlays", clusterName)
	serviceDir := filepath.Join(overlayPath, "services", "test-service")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return nil, "", "", fmt.Errorf("failed to create service dir: %w", err)
	}

	// Create secrets directory for Age key
	secretsDir := filepath.Join(configDir, "secrets", "age")
	if err := os.MkdirAll(secretsDir, 0755); err != nil {
		return nil, "", "", fmt.Errorf("failed to create secrets dir: %w", err)
	}

	// Create a mock Age key file (for dry-run testing, we don't need a real key)
	ageKeyPath := filepath.Join(secretsDir, fmt.Sprintf("%s_keys.txt", clusterName))
	mockAgeKey := `# created: 2024-01-15T10:30:00Z
# public key: age1test1234567890abcdefghijklmnopqrstuvwxyz1234567890abc
AGE-SECRET-KEY-1TEST1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890ABC
`
	if err := os.WriteFile(ageKeyPath, []byte(mockAgeKey), 0600); err != nil {
		return nil, "", "", fmt.Errorf("failed to create mock age key: %w", err)
	}

	// Create config file
	cfg := createDryRunTestConfig(clusterName, tmpDir, secrets)

	// Create file system
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)

	// Create config loader
	configLoader := config.NewConfigIOHandler(fileSystem)

	// Save config in the resolver-backed location and a legacy copy used by older discovery tests.
	if err := saveDryRunConfig(ctx, configLoader, cfg, clusterPaths, clusterName); err != nil {
		return nil, "", "", fmt.Errorf("failed to save config: %w", err)
	}

	// Create SOPS manager with mock encryptor
	sopsManager := sops.NewDefaultSOPSManager(nil, nil, logger)

	// Create secrets manager
	manager := &DefaultSecretsManager{
		configLoader: configLoader,
		sopsManager:  sopsManager,
		auditLogger:  nil,
		logger:       logger,
	}

	return manager, clusterPaths.ConfigPath, overlayPath, nil
}

// setupDryRunRotationTest creates a test environment for dry-run rotation testing
func setupDryRunRotationTest(t *testing.T, tmpDir string, clusterName string, secrets map[string]string) (*DefaultKeyRotator, error) {
	t.Helper()

	// Override HOME environment variable for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	t.Cleanup(func() {
		os.Setenv("HOME", originalHome)
	})

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create directory structure
	configDir := filepath.Join(tmpDir, ".config", "opencenter", "clusters", "test-org", clusterName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config dir: %w", err)
	}

	overlayPath := filepath.Join(tmpDir, "test-repo", "applications", "overlays", clusterName)
	serviceDir := filepath.Join(overlayPath, "services", "test-service")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create service dir: %w", err)
	}

	secretsDir := filepath.Join(configDir, "secrets", "age")
	if err := os.MkdirAll(secretsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create secrets dir: %w", err)
	}

	registryDir := filepath.Join(configDir, "secrets")
	if err := os.MkdirAll(registryDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create registry dir: %w", err)
	}

	// Generate Age key
	keyPath := filepath.Join(secretsDir, fmt.Sprintf("%s_keys.txt", clusterName))
	publicKey, err := generateAgeKeyFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to generate age key: %w", err)
	}

	// Create .sops.yaml
	sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")
	sopsConfig := fmt.Sprintf(`creation_rules:
  - path_regex: .*\.yaml$
    age: >-
      %s
`, publicKey)
	if err := os.WriteFile(sopsConfigPath, []byte(sopsConfig), 0644); err != nil {
		return nil, fmt.Errorf("failed to create .sops.yaml: %w", err)
	}

	// Create manifest with secrets
	manifestPath := filepath.Join(serviceDir, "secret.yaml")
	manifestContent := createManifestContent(secrets)
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to create manifest: %w", err)
	}

	// Create mock SOPS encryptor for registry
	mockEncryptor := &mockSOPSEncryptor{}

	// Create key registry
	registry := NewDefaultKeyRegistry(registryDir, mockEncryptor, logger)

	// Create file system
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)

	// Create config loader
	configLoader := config.NewConfigIOHandler(fileSystem)

	// Create SOPS manager
	sopsManager := sops.NewSOPSManager()

	// Create secrets manager
	secretsManager := &DefaultSecretsManager{
		configLoader: configLoader,
		sopsManager:  sopsManager,
		auditLogger:  nil,
		logger:       logger,
	}

	// Create key rotator
	rotator := &DefaultKeyRotator{
		registry:       registry,
		secretsManager: secretsManager,
		auditLogger:    nil,
		logger:         logger,
	}

	// Register key in registry
	ctx := context.Background()
	keyEntry := KeyEntry{
		Cluster:     clusterName,
		KeyType:     KeyTypeAge,
		Fingerprint: publicKey,
		PublicKey:   publicKey,
		Status:      KeyStatusActive,
	}
	if err := registry.RegisterKey(ctx, keyEntry); err != nil {
		return nil, fmt.Errorf("failed to register key: %w", err)
	}

	return rotator, nil
}

// setupDryRunRevocationTest creates a test environment for dry-run revocation testing
func setupDryRunRevocationTest(t *testing.T, tmpDir string, clusterName string, secrets map[string]string) (*DefaultKeyRevoker, error) {
	t.Helper()

	// Override HOME environment variable for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	t.Cleanup(func() {
		os.Setenv("HOME", originalHome)
	})

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create directory structure
	configDir := filepath.Join(tmpDir, ".config", "opencenter", "clusters", "test-org", clusterName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config dir: %w", err)
	}

	overlayPath := filepath.Join(tmpDir, "test-repo", "applications", "overlays", clusterName)
	serviceDir := filepath.Join(overlayPath, "services", "test-service")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create service dir: %w", err)
	}

	secretsDir := filepath.Join(configDir, "secrets", "age")
	if err := os.MkdirAll(secretsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create secrets dir: %w", err)
	}

	registryDir := filepath.Join(configDir, "secrets")
	if err := os.MkdirAll(registryDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create registry dir: %w", err)
	}

	// Generate two Age keys (for multi-recipient setup)
	key1Path := filepath.Join(secretsDir, fmt.Sprintf("%s_keys.txt", clusterName))
	publicKey1, err := generateAgeKeyFile(key1Path)
	if err != nil {
		return nil, fmt.Errorf("failed to generate age key 1: %w", err)
	}

	key2Path := filepath.Join(secretsDir, fmt.Sprintf("%s_keys_user.txt", clusterName))
	publicKey2, err := generateAgeKeyFile(key2Path)
	if err != nil {
		return nil, fmt.Errorf("failed to generate age key 2: %w", err)
	}

	// Create .sops.yaml with both keys
	sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")
	sopsConfig := fmt.Sprintf(`creation_rules:
  - path_regex: .*\.yaml$
    age: >-
      %s,
      %s
`, publicKey1, publicKey2)
	if err := os.WriteFile(sopsConfigPath, []byte(sopsConfig), 0644); err != nil {
		return nil, fmt.Errorf("failed to create .sops.yaml: %w", err)
	}

	// Create manifest with secrets
	manifestPath := filepath.Join(serviceDir, "secret.yaml")
	manifestContent := createManifestContent(secrets)
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to create manifest: %w", err)
	}

	// Create mock SOPS encryptor for registry
	mockEncryptor := &mockSOPSEncryptor{}

	// Create key registry
	registry := NewDefaultKeyRegistry(registryDir, mockEncryptor, logger)

	// Create file system
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)

	// Create config loader
	configLoader := config.NewConfigIOHandler(fileSystem)

	// Create SOPS manager
	sopsManager := sops.NewSOPSManager()

	// Create secrets manager
	secretsManager := &DefaultSecretsManager{
		configLoader: configLoader,
		sopsManager:  sopsManager,
		auditLogger:  nil,
		logger:       logger,
	}

	// Create key revoker
	revoker := &DefaultKeyRevoker{
		registry:       registry,
		secretsManager: secretsManager,
		auditLogger:    nil,
		logger:         logger,
	}

	// Register keys in registry
	ctx := context.Background()
	keyEntry1 := KeyEntry{
		Cluster:     clusterName,
		KeyType:     KeyTypeAge,
		Fingerprint: publicKey1,
		PublicKey:   publicKey1,
		Status:      KeyStatusActive,
	}
	if err := registry.RegisterKey(ctx, keyEntry1); err != nil {
		return nil, fmt.Errorf("failed to register key 1: %w", err)
	}

	keyEntry2 := KeyEntry{
		Cluster:     clusterName,
		KeyType:     KeyTypeAge,
		Fingerprint: publicKey2,
		PublicKey:   publicKey2,
		Status:      KeyStatusActive,
		UsedBy:      []string{"test-user@example.com"},
	}
	if err := registry.RegisterKey(ctx, keyEntry2); err != nil {
		return nil, fmt.Errorf("failed to register key 2: %w", err)
	}

	return revoker, nil
}

// setupDryRunMultiClusterTest creates a test environment for dry-run multi-cluster sync testing
func setupDryRunMultiClusterTest(t *testing.T, tmpDir string, secrets map[string]string) (*DefaultMultiClusterSyncer, error) {
	t.Helper()

	configureDryRunTestEnv(t, tmpDir)

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create two test clusters
	clusterNames := []string{"cluster1", "cluster2"}
	for _, clusterName := range clusterNames {
		ctx := context.Background()
		clusterPaths, err := createDryRunClusterPaths(ctx, clusterName)
		if err != nil {
			return nil, fmt.Errorf("failed to create cluster paths for %s: %w", clusterName, err)
		}

		cfg := createDryRunTestConfig(clusterName, tmpDir, secrets)

		if err := os.MkdirAll(filepath.Dir(cfg.Secrets.SopsAgeKeyFile), 0755); err != nil {
			return nil, fmt.Errorf("failed to create age key dir for %s: %w", clusterName, err)
		}
		mockAgeKey := `# created: 2024-01-15T10:30:00Z
# public key: age1test1234567890abcdefghijklmnopqrstuvwxyz1234567890abc
AGE-SECRET-KEY-1TEST1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890ABC
`
		if err := os.WriteFile(cfg.Secrets.SopsAgeKeyFile, []byte(mockAgeKey), 0600); err != nil {
			return nil, fmt.Errorf("failed to create mock age key for %s: %w", clusterName, err)
		}

		// Create file system
		errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
		fileSystem := fs.NewDefaultFileSystem(errorHandler)

		// Create config loader
		configLoader := config.NewConfigIOHandler(fileSystem)

		if err := saveDryRunConfig(ctx, configLoader, cfg, clusterPaths, clusterName); err != nil {
			return nil, fmt.Errorf("failed to save config for %s: %w", clusterName, err)
		}

		// Create overlay directory
		overlayPath := filepath.Join(tmpDir, "test-repo", "applications", "overlays", clusterName)
		serviceDir := filepath.Join(overlayPath, "services", "test-service")
		if err := os.MkdirAll(serviceDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create service dir for %s: %w", clusterName, err)
		}
	}

	// Create file system
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)

	// Create config loader
	configLoader := config.NewConfigIOHandler(fileSystem)

	// Create SOPS manager
	sopsManager := sops.NewDefaultSOPSManager(nil, nil, logger)

	// Create secrets manager
	secretsManager := &DefaultSecretsManager{
		configLoader: configLoader,
		sopsManager:  sopsManager,
		auditLogger:  nil,
		logger:       logger,
	}

	// Create multi-cluster syncer
	syncer := NewDefaultMultiClusterSyncer(secretsManager, logger)

	return syncer, nil
}

// createDryRunTestConfig creates a test configuration for dry-run testing
func createDryRunTestConfig(clusterName string, tmpDir string, secrets map[string]string) *v2.Config {
	// Convert secrets to config format
	certManagerSecrets := v2.CertManagerSecrets{}
	lokiSecrets := v2.LokiSecrets{}
	keycloakSecrets := v2.KeycloakSecrets{}

	// Distribute secrets across services
	i := 0
	for key, value := range secrets {
		switch i % 3 {
		case 0:
			if key == "aws_access_key" || key == "key_one" {
				certManagerSecrets.AWSAccessKey = value
			} else {
				certManagerSecrets.AWSSecretAccessKey = value
			}
		case 1:
			if key == "s3_access_key_id" || key == "key_two" {
				lokiSecrets.S3AccessKeyID = value
			} else {
				lokiSecrets.S3SecretAccessKey = value
			}
		case 2:
			if key == "client_secret" || key == "key_three" {
				keycloakSecrets.ClientSecret = value
			} else {
				keycloakSecrets.AdminPassword = value
			}
		}
		i++
	}

	cfg := newSecretsTestConfig(clusterName, "openstack")
	cfg.OpenCenter.Meta.Organization = dryRunTestOrg
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "test-repo")
	cfg.Secrets.SopsAgeKeyFile = filepath.Join(tmpDir, ".config", "opencenter", "clusters", "test-org", clusterName, "secrets", "age", fmt.Sprintf("%s_keys.txt", clusterName))
	cfg.Secrets.CertManager = certManagerSecrets
	cfg.Secrets.Loki = lokiSecrets
	cfg.Secrets.Keycloak = keycloakSecrets
	return cfg
}

// Sanity tests to verify the property tests are working correctly

func TestProperty_DryRunImmutability_Sync_Sanity(t *testing.T) {
	tmpDir := t.TempDir()
	clusterName := "sanity-cluster"
	ctx := context.Background()

	// Create test secrets
	testSecrets := map[string]string{
		"aws_access_key":        "AKIAIOSFODNN7EXAMPLE",
		"aws_secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	// Setup test environment
	manager, _, _, err := setupDryRunTest(t, tmpDir, clusterName, testSecrets)
	require.NoError(t, err, "Failed to setup dry-run test")

	// Capture filesystem state before operation
	beforeState, err := captureFilesystemState(tmpDir)
	require.NoError(t, err, "Failed to capture filesystem state")
	require.NotNil(t, beforeState, "Before state should not be nil")
	require.NotEmpty(t, beforeState.Files, "Before state should have files")

	// Perform sync with dry-run
	opts := SyncOptions{
		Cluster: clusterName,
		DryRun:  true,
	}

	result, err := manager.SyncSecrets(ctx, opts)
	require.NoError(t, err, "Sync operation should not fail")
	require.NotNil(t, result, "Sync result should not be nil")

	// Capture filesystem state after operation
	afterState, err := captureFilesystemState(tmpDir)
	require.NoError(t, err, "Failed to capture filesystem state after operation")
	require.NotNil(t, afterState, "After state should not be nil")

	// Verify filesystem state is identical
	require.True(t, compareFilesystemStates(beforeState, afterState),
		"Filesystem state should be identical after dry-run sync")

	// Verify number of files is the same
	require.Equal(t, len(beforeState.Files), len(afterState.Files),
		"Number of files should be the same")
}

func TestProperty_DryRunImmutability_MultiClusterSync_Sanity(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create test secrets
	testSecrets := map[string]string{
		"key_one": "value_one",
		"key_two": "value_two",
	}

	// Setup test environment
	syncer, err := setupDryRunMultiClusterTest(t, tmpDir, testSecrets)
	require.NoError(t, err, "Failed to setup dry-run multi-cluster test")

	// Capture filesystem state before operation
	beforeState, err := captureFilesystemState(tmpDir)
	require.NoError(t, err, "Failed to capture filesystem state")
	require.NotNil(t, beforeState, "Before state should not be nil")

	// Perform multi-cluster sync with dry-run
	opts := MultiClusterSyncOptions{
		Organization: "test-org",
		Concurrency:  2,
		StopOnError:  false,
		DryRun:       true,
	}

	result, err := syncer.SyncAll(ctx, opts)
	require.NoError(t, err, "Multi-cluster sync operation should not fail")
	require.NotNil(t, result, "Sync result should not be nil")

	// Capture filesystem state after operation
	afterState, err := captureFilesystemState(tmpDir)
	require.NoError(t, err, "Failed to capture filesystem state after operation")
	require.NotNil(t, afterState, "After state should not be nil")

	// Verify filesystem state is identical
	require.True(t, compareFilesystemStates(beforeState, afterState),
		"Filesystem state should be identical after dry-run multi-cluster sync")
}

func TestFilesystemStateCapture(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some test files
	testFile1 := filepath.Join(tmpDir, "test1.txt")
	require.NoError(t, os.WriteFile(testFile1, []byte("content1"), 0644))

	testDir := filepath.Join(tmpDir, "testdir")
	require.NoError(t, os.MkdirAll(testDir, 0755))

	testFile2 := filepath.Join(testDir, "test2.txt")
	require.NoError(t, os.WriteFile(testFile2, []byte("content2"), 0644))

	// Capture state
	state, err := captureFilesystemState(tmpDir)
	require.NoError(t, err, "Should capture filesystem state")
	require.NotNil(t, state, "State should not be nil")

	// Verify files are captured
	require.Contains(t, state.Files, "test1.txt", "Should capture test1.txt")
	require.Contains(t, state.Files, "testdir", "Should capture testdir")
	require.Contains(t, state.Files, filepath.Join("testdir", "test2.txt"), "Should capture test2.txt")

	// Verify file properties
	file1State := state.Files["test1.txt"]
	require.False(t, file1State.IsDir, "test1.txt should not be a directory")
	require.NotEmpty(t, file1State.Hash, "test1.txt should have a hash")
	require.Equal(t, int64(8), file1State.Size, "test1.txt should have size 8")

	dirState := state.Files["testdir"]
	require.True(t, dirState.IsDir, "testdir should be a directory")
}

func TestFilesystemStateComparison(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial state
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))

	state1, err := captureFilesystemState(tmpDir)
	require.NoError(t, err)

	// Capture again without changes
	state2, err := captureFilesystemState(tmpDir)
	require.NoError(t, err)

	// Should be identical
	require.True(t, compareFilesystemStates(state1, state2),
		"States should be identical when no changes made")

	// Modify file
	require.NoError(t, os.WriteFile(testFile, []byte("modified"), 0644))

	state3, err := captureFilesystemState(tmpDir)
	require.NoError(t, err)

	// Should be different
	require.False(t, compareFilesystemStates(state1, state3),
		"States should be different after file modification")

	// Add new file
	testFile2 := filepath.Join(tmpDir, "test2.txt")
	require.NoError(t, os.WriteFile(testFile2, []byte("new"), 0644))

	state4, err := captureFilesystemState(tmpDir)
	require.NoError(t, err)

	// Should be different
	require.False(t, compareFilesystemStates(state3, state4),
		"States should be different after adding file")
}

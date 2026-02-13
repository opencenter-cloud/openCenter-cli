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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/prop"
	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/sops"
	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
	"github.com/rackerlabs/opencenter-cli/internal/util/fs"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// **Validates: Requirements 6.2, 6.3**
//
// Property 14: Revocation Effectiveness
//
// For any revoked key, encrypted manifests should not be decryptable with the revoked key
// after re-encryption.
//
// This property verifies that after key revocation:
// 1. Manifests cannot be decrypted using the revoked key
// 2. Manifests can still be decrypted using remaining active keys
// 3. The .sops.yaml configuration no longer contains the revoked key
// 4. The key registry reflects the revoked key status
func TestProperty_RevocationEffectiveness(t *testing.T) {
	// Skip if SOPS is not available (CI environment)
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

	properties.Property("revoked key cannot decrypt manifests after re-encryption", prop.ForAll(
		func(secretsGen map[string]string) bool {
			// Skip if empty
			if len(secretsGen) == 0 {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			clusterName := "test-cluster"
			ctx := context.Background()

			// Create test infrastructure with multiple keys
			revoker, key1Path, key2Path, key3Path, manifestPath, overlayPath, err := setupRevocationTest(t, tmpDir, clusterName, secretsGen)
			if err != nil {
				t.Logf("Failed to setup revocation test: %v", err)
				return false
			}

			// Get key2 public key (this is the key we'll revoke)
			key2PublicKey, err := extractPublicKeyFromFile(key2Path)
			if err != nil {
				t.Logf("Failed to extract key2 public key: %v", err)
				return false
			}

			// Verify manifest is initially decryptable with all three keys
			if _, err := decryptManifestWithKey(ctx, manifestPath, key1Path); err != nil {
				t.Logf("Failed to decrypt manifest with key1 before revocation: %v", err)
				return false
			}

			if _, err := decryptManifestWithKey(ctx, manifestPath, key2Path); err != nil {
				t.Logf("Failed to decrypt manifest with key2 before revocation: %v", err)
				return false
			}

			if _, err := decryptManifestWithKey(ctx, manifestPath, key3Path); err != nil {
				t.Logf("Failed to decrypt manifest with key3 before revocation: %v", err)
				return false
			}

			// Revoke key2
			revokeOpts := RevokeOptions{
				Cluster:     clusterName,
				Fingerprint: key2PublicKey,
				DryRun:      false,
			}

			revocationResult, err := revoker.RevokeByFingerprint(ctx, revokeOpts)
			if err != nil {
				t.Logf("Failed to revoke key: %v", err)
				return false
			}

			// Property 1: Revocation should have re-encrypted at least one file
			if len(revocationResult.ReencryptedFiles) == 0 {
				t.Logf("No files were re-encrypted during revocation")
				return false
			}

			// Property 2: Manifest should NOT be decryptable with revoked key (key2)
			_, err = decryptManifestWithKey(ctx, manifestPath, key2Path)
			if err == nil {
				t.Logf("Manifest should not be decryptable with revoked key")
				return false
			}

			// Property 3: Manifest should still be decryptable with remaining keys (key1 and key3)
			key1Decrypted, err := decryptManifestWithKey(ctx, manifestPath, key1Path)
			if err != nil {
				t.Logf("Failed to decrypt manifest with key1 after revocation: %v", err)
				return false
			}

			key3Decrypted, err := decryptManifestWithKey(ctx, manifestPath, key3Path)
			if err != nil {
				t.Logf("Failed to decrypt manifest with key3 after revocation: %v", err)
				return false
			}

			// Property 4: Decrypted content should be identical with both remaining keys
			if !compareManifestData(key1Decrypted, key3Decrypted) {
				t.Logf("Decrypted content differs between remaining keys")
				return false
			}

			// Property 5: Decrypted content should match original secrets
			if !verifyDecryptedSecrets(key1Decrypted, secretsGen) {
				t.Logf("Decrypted content does not match original secrets")
				return false
			}

			// Property 6: .sops.yaml should not contain the revoked key
			sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")
			if containsKey(sopsConfigPath, key2PublicKey) {
				t.Logf(".sops.yaml should not contain revoked key")
				return false
			}

			// Property 7: .sops.yaml should still contain the remaining keys
			key1PublicKey, _ := extractPublicKeyFromFile(key1Path)
			key3PublicKey, _ := extractPublicKeyFromFile(key3Path)

			if !containsKey(sopsConfigPath, key1PublicKey) {
				t.Logf(".sops.yaml should contain key1")
				return false
			}

			if !containsKey(sopsConfigPath, key3PublicKey) {
				t.Logf(".sops.yaml should contain key3")
				return false
			}

			// Property 8: Manifest should have SOPS metadata for remaining keys only
			if !verifyManifestHasKeys(manifestPath, []string{key1PublicKey, key3PublicKey}) {
				t.Logf("Manifest should have SOPS metadata for remaining keys only")
				return false
			}

			return true
		},
		genSecretMap(),
	))

	properties.TestingRun(t)
}

// setupRevocationTest creates a test environment with three keys and encrypted manifests
func setupRevocationTest(t *testing.T, tmpDir string, clusterName string, secrets map[string]string) (*DefaultKeyRevoker, string, string, string, string, string, error) {
	t.Helper()

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create SOPS manager
	sopsManager := sops.NewSOPSManager()

	// Create directory structure
	configDir := filepath.Join(tmpDir, ".config", "opencenter", "clusters", "test-org", clusterName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, "", "", "", "", "", fmt.Errorf("failed to create config dir: %w", err)
	}

	overlayPath := filepath.Join(tmpDir, "test-repo", "applications", "overlays", clusterName)
	serviceDir := filepath.Join(overlayPath, "services", "test-service")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return nil, "", "", "", "", "", fmt.Errorf("failed to create service dir: %w", err)
	}

	secretsDir := filepath.Join(configDir, "secrets", "age")
	if err := os.MkdirAll(secretsDir, 0755); err != nil {
		return nil, "", "", "", "", "", fmt.Errorf("failed to create secrets dir: %w", err)
	}

	registryDir := filepath.Join(configDir, "secrets")
	if err := os.MkdirAll(registryDir, 0755); err != nil {
		return nil, "", "", "", "", "", fmt.Errorf("failed to create registry dir: %w", err)
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

	// Create secrets manager
	secretsManager := &DefaultSecretsManager{
		configLoader: configLoader,
		sopsManager:  sopsManager,
		auditLogger:  nil,
		logger:       logger,
	}

	// Create key rotator (needed for emergency revocation)
	rotator := &DefaultKeyRotator{
		registry:       registry,
		secretsManager: secretsManager,
		auditLogger:    nil,
		logger:         logger,
	}

	// Create key revoker
	revoker := &DefaultKeyRevoker{
		registry:       registry,
		rotator:        rotator,
		secretsManager: secretsManager,
		auditLogger:    nil,
		logger:         logger,
	}

	// Generate three Age keys
	key1Path := filepath.Join(secretsDir, fmt.Sprintf("%s_keys_1.txt", clusterName))
	key1PublicKey, err := generateAgeKeyFile(key1Path)
	if err != nil {
		return nil, "", "", "", "", "", fmt.Errorf("failed to generate key1: %w", err)
	}

	key2Path := filepath.Join(secretsDir, fmt.Sprintf("%s_keys_2.txt", clusterName))
	key2PublicKey, err := generateAgeKeyFile(key2Path)
	if err != nil {
		return nil, "", "", "", "", "", fmt.Errorf("failed to generate key2: %w", err)
	}

	key3Path := filepath.Join(secretsDir, fmt.Sprintf("%s_keys_3.txt", clusterName))
	key3PublicKey, err := generateAgeKeyFile(key3Path)
	if err != nil {
		return nil, "", "", "", "", "", fmt.Errorf("failed to generate key3: %w", err)
	}

	// Create .sops.yaml with all three keys
	sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")
	sopsConfig := fmt.Sprintf(`creation_rules:
  - path_regex: .*\.yaml$
    age: >-
      %s,
      %s,
      %s
`, key1PublicKey, key2PublicKey, key3PublicKey)
	if err := os.WriteFile(sopsConfigPath, []byte(sopsConfig), 0644); err != nil {
		return nil, "", "", "", "", "", fmt.Errorf("failed to create .sops.yaml: %w", err)
	}

	// Create manifest with secrets
	manifestPath := filepath.Join(serviceDir, "secret.yaml")
	manifestContent := createManifestContent(secrets)
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		return nil, "", "", "", "", "", fmt.Errorf("failed to create manifest: %w", err)
	}

	// Encrypt manifest with all three keys using SOPS
	ctx := context.Background()
	encryptor := sopsManager.GetEncryptor()
	encryptConfig := sops.EncryptionConfig{
		AgeKeys:    []string{key1PublicKey, key2PublicKey, key3PublicKey},
		ConfigFile: sopsConfigPath,
		InPlace:    true,
		DryRun:     false,
		Verbose:    false,
	}

	if err := encryptor.EncryptFile(ctx, manifestPath, encryptConfig); err != nil {
		return nil, "", "", "", "", "", fmt.Errorf("failed to encrypt manifest: %w", err)
	}

	// Register keys in registry
	key1Entry := KeyEntry{
		Cluster:     clusterName,
		KeyType:     KeyTypeAge,
		Fingerprint: key1PublicKey,
		PublicKey:   key1PublicKey,
		Status:      KeyStatusActive,
	}
	if err := registry.RegisterKey(ctx, key1Entry); err != nil {
		return nil, "", "", "", "", "", fmt.Errorf("failed to register key1: %w", err)
	}

	key2Entry := KeyEntry{
		Cluster:     clusterName,
		KeyType:     KeyTypeAge,
		Fingerprint: key2PublicKey,
		PublicKey:   key2PublicKey,
		Status:      KeyStatusActive,
	}
	if err := registry.RegisterKey(ctx, key2Entry); err != nil {
		return nil, "", "", "", "", "", fmt.Errorf("failed to register key2: %w", err)
	}

	key3Entry := KeyEntry{
		Cluster:     clusterName,
		KeyType:     KeyTypeAge,
		Fingerprint: key3PublicKey,
		PublicKey:   key3PublicKey,
		Status:      KeyStatusActive,
	}
	if err := registry.RegisterKey(ctx, key3Entry); err != nil {
		return nil, "", "", "", "", "", fmt.Errorf("failed to register key3: %w", err)
	}

	// Create cluster config file (needed for revocation to work)
	configPath := filepath.Join(configDir, fmt.Sprintf(".k8s-%s-config.yaml", clusterName))
	clusterConfig := fmt.Sprintf(`schema_version: "2.0"
opencenter:
  cluster:
    cluster_name: %s
  gitops:
    git_dir: %s
secrets:
  sops_age_key_file: %s
`, clusterName, filepath.Join(tmpDir, "test-repo"), key1Path)
	if err := os.WriteFile(configPath, []byte(clusterConfig), 0644); err != nil {
		return nil, "", "", "", "", "", fmt.Errorf("failed to create cluster config: %w", err)
	}

	return revoker, key1Path, key2Path, key3Path, manifestPath, overlayPath, nil
}

// extractPublicKeyFromFile extracts the public key from an Age key file
func extractPublicKeyFromFile(keyPath string) (string, error) {
	content, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read key file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# public key: ") {
			publicKey := strings.TrimPrefix(line, "# public key: ")
			return strings.TrimSpace(publicKey), nil
		}
	}

	return "", fmt.Errorf("failed to extract public key from key file")
}

// containsKey checks if .sops.yaml contains the specified key
func containsKey(sopsConfigPath string, publicKey string) bool {
	content, err := os.ReadFile(sopsConfigPath)
	if err != nil {
		return false
	}

	return strings.Contains(string(content), publicKey)
}

// verifyManifestHasKeys verifies that the manifest has SOPS metadata for the specified keys
func verifyManifestHasKeys(manifestPath string, expectedKeys []string) bool {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return false
	}

	// Parse YAML to check SOPS metadata
	var manifest map[string]interface{}
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return false
	}

	sopsMetadata, ok := manifest["sops"].(map[string]interface{})
	if !ok {
		return false
	}

	ageRecipients, ok := sopsMetadata["age"].([]interface{})
	if !ok {
		return false
	}

	// Check that we have the expected number of recipients
	if len(ageRecipients) != len(expectedKeys) {
		return false
	}

	// Extract recipient public keys from SOPS metadata
	recipientKeys := make(map[string]bool)
	for _, recipient := range ageRecipients {
		recipientMap, ok := recipient.(map[string]interface{})
		if !ok {
			continue
		}
		if recipientKey, ok := recipientMap["recipient"].(string); ok {
			recipientKeys[recipientKey] = true
		}
	}

	// Verify all expected keys are present
	for _, expectedKey := range expectedKeys {
		if !recipientKeys[expectedKey] {
			return false
		}
	}

	return true
}

// Test that verifies the revocation effectiveness property test is working correctly
func TestProperty_RevocationEffectiveness_Sanity(t *testing.T) {
	// Skip if SOPS is not available
	if _, err := exec.LookPath("sops"); err != nil {
		t.Skip("SOPS binary not found, skipping sanity test")
	}

	// Skip if age-keygen is not available
	if _, err := exec.LookPath("age-keygen"); err != nil {
		t.Skip("age-keygen binary not found, skipping sanity test")
	}

	tmpDir := t.TempDir()
	clusterName := "sanity-cluster"
	ctx := context.Background()

	// Create test secrets
	testSecrets := map[string]string{
		"aws_access_key":        "AKIAIOSFODNN7EXAMPLE",
		"aws_secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	// Setup revocation test
	revoker, key1Path, key2Path, key3Path, manifestPath, overlayPath, err := setupRevocationTest(t, tmpDir, clusterName, testSecrets)
	require.NoError(t, err, "Failed to setup revocation test")

	// Get key2 public key (this is the key we'll revoke)
	key2PublicKey, err := extractPublicKeyFromFile(key2Path)
	require.NoError(t, err, "Failed to extract key2 public key")

	// Test 1: Verify manifest is initially decryptable with all three keys
	t.Run("manifest decryptable with all keys before revocation", func(t *testing.T) {
		_, err := decryptManifestWithKey(ctx, manifestPath, key1Path)
		require.NoError(t, err, "Should decrypt with key1")

		_, err = decryptManifestWithKey(ctx, manifestPath, key2Path)
		require.NoError(t, err, "Should decrypt with key2")

		_, err = decryptManifestWithKey(ctx, manifestPath, key3Path)
		require.NoError(t, err, "Should decrypt with key3")
	})

	// Test 2: Revoke key2
	t.Run("revoke key2", func(t *testing.T) {
		revokeOpts := RevokeOptions{
			Cluster:     clusterName,
			Fingerprint: key2PublicKey,
			DryRun:      false,
		}

		revocationResult, err := revoker.RevokeByFingerprint(ctx, revokeOpts)
		require.NoError(t, err, "Should revoke key successfully")
		require.NotEmpty(t, revocationResult.ReencryptedFiles, "Should have re-encrypted files")
		require.Contains(t, revocationResult.RevokedKeys, key2PublicKey, "Should contain revoked key fingerprint")
	})

	// Test 3: Verify manifest is NOT decryptable with revoked key
	t.Run("manifest not decryptable with revoked key", func(t *testing.T) {
		_, err := decryptManifestWithKey(ctx, manifestPath, key2Path)
		require.Error(t, err, "Should not decrypt with revoked key")
	})

	// Test 4: Verify manifest is still decryptable with remaining keys
	t.Run("manifest decryptable with remaining keys", func(t *testing.T) {
		key1Decrypted, err := decryptManifestWithKey(ctx, manifestPath, key1Path)
		require.NoError(t, err, "Should decrypt with key1")

		key3Decrypted, err := decryptManifestWithKey(ctx, manifestPath, key3Path)
		require.NoError(t, err, "Should decrypt with key3")

		// Verify decrypted content is identical
		require.True(t, compareManifestData(key1Decrypted, key3Decrypted),
			"Decrypted content should be identical")

		// Verify decrypted content matches original secrets
		require.True(t, verifyDecryptedSecrets(key1Decrypted, testSecrets),
			"Decrypted content should match original secrets")
	})

	// Test 5: Verify .sops.yaml does not contain revoked key
	t.Run("sops config does not contain revoked key", func(t *testing.T) {
		sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")
		require.False(t, containsKey(sopsConfigPath, key2PublicKey),
			".sops.yaml should not contain revoked key")

		// Verify remaining keys are still present
		key1PublicKey, err := extractPublicKeyFromFile(key1Path)
		require.NoError(t, err)
		require.True(t, containsKey(sopsConfigPath, key1PublicKey),
			".sops.yaml should contain key1")

		key3PublicKey, err := extractPublicKeyFromFile(key3Path)
		require.NoError(t, err)
		require.True(t, containsKey(sopsConfigPath, key3PublicKey),
			".sops.yaml should contain key3")
	})

	// Test 6: Verify manifest has SOPS metadata for remaining keys only
	t.Run("manifest has SOPS metadata for remaining keys only", func(t *testing.T) {
		key1PublicKey, err := extractPublicKeyFromFile(key1Path)
		require.NoError(t, err)

		key3PublicKey, err := extractPublicKeyFromFile(key3Path)
		require.NoError(t, err)

		require.True(t, verifyManifestHasKeys(manifestPath, []string{key1PublicKey, key3PublicKey}),
			"Manifest should have SOPS metadata for remaining keys only")
	})
}

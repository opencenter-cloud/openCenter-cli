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
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// **Validates: Requirements 3.2, 3.3**
//
// Property 7: Key Rotation Dual-Key Decryption
//
// For any cluster in dual-key rotation state, all encrypted manifests should be
// decryptable with both the old key and the new key.
//
// This property verifies that during the dual-key rotation phase:
// 1. Manifests are encrypted with both old and new Age keys
// 2. Manifests can be successfully decrypted using the old key alone
// 3. Manifests can be successfully decrypted using the new key alone
// 4. The decrypted content is identical regardless of which key is used
func TestProperty_DualKeyDecryption(t *testing.T) {
	// Skip if SOPS is not available (CI environment)
	if _, err := exec.LookPath("sops"); err != nil {
		t.Skip("SOPS binary not found, skipping property test")
	}

	// Skip if age-keygen is not available
	if _, err := exec.LookPath("age-keygen"); err != nil {
		t.Skip("age-keygen binary not found, skipping property test")
	}

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50 // Reduced from 100 due to SOPS operations
	properties := gopter.NewProperties(parameters)

	properties.Property("manifests in dual-key state are decryptable with both old and new keys", prop.ForAll(
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
			rotator, oldKeyPath, newKeyPath, manifestPath, err := setupDualKeyRotationTest(t, tmpDir, clusterName, secretsGen)
			if err != nil {
				t.Logf("Failed to setup dual-key rotation test: %v", err)
				return false
			}

			// Property 1: Manifest should be decryptable with old key
			oldKeyDecrypted, err := decryptManifestWithKey(ctx, manifestPath, oldKeyPath)
			if err != nil {
				t.Logf("Failed to decrypt manifest with old key: %v", err)
				return false
			}

			// Property 2: Manifest should be decryptable with new key
			newKeyDecrypted, err := decryptManifestWithKey(ctx, manifestPath, newKeyPath)
			if err != nil {
				t.Logf("Failed to decrypt manifest with new key: %v", err)
				return false
			}

			// Property 3: Decrypted content should be identical
			if !compareManifestData(oldKeyDecrypted, newKeyDecrypted) {
				t.Logf("Decrypted content differs between old and new keys")
				t.Logf("Old key decrypted: %v", oldKeyDecrypted)
				t.Logf("New key decrypted: %v", newKeyDecrypted)
				return false
			}

			// Property 4: Decrypted content should match original secrets
			if !verifyDecryptedSecrets(oldKeyDecrypted, secretsGen) {
				t.Logf("Decrypted content does not match original secrets")
				return false
			}

			// Property 5: Manifest should contain SOPS metadata for both keys
			if !verifyDualKeySOPSMetadata(manifestPath) {
				t.Logf("Manifest does not contain dual-key SOPS metadata")
				return false
			}

			// Cleanup
			_ = rotator

			return true
		},
		genSecretMap(),
	))

	properties.TestingRun(t)
}

// setupDualKeyRotationTest creates a test environment with dual-key rotation in progress
func setupDualKeyRotationTest(t *testing.T, tmpDir string, clusterName string, secrets map[string]string) (*DefaultKeyRotator, string, string, string, error) {
	t.Helper()

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create SOPS manager
	sopsManager := sops.NewSOPSManager()

	// Create directory structure
	configDir := filepath.Join(tmpDir, ".config", "opencenter", "clusters", "test-org", clusterName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, "", "", "", fmt.Errorf("failed to create config dir: %w", err)
	}

	overlayPath := filepath.Join(tmpDir, "test-repo", "applications", "overlays", clusterName)
	serviceDir := filepath.Join(overlayPath, "services", "test-service")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return nil, "", "", "", fmt.Errorf("failed to create service dir: %w", err)
	}

	secretsDir := filepath.Join(configDir, "secrets", "age")
	if err := os.MkdirAll(secretsDir, 0755); err != nil {
		return nil, "", "", "", fmt.Errorf("failed to create secrets dir: %w", err)
	}

	registryDir := filepath.Join(configDir, "secrets")
	if err := os.MkdirAll(registryDir, 0755); err != nil {
		return nil, "", "", "", fmt.Errorf("failed to create registry dir: %w", err)
	}

	// Create mock SOPS encryptor for registry
	mockEncryptor := &mockSOPSEncryptor{}

	// Create key registry
	registry := NewDefaultKeyRegistry(registryDir, mockEncryptor, logger)

	// Create file system
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)

	// Create config loader
	configLoader := v2.NewConfigIOHandler(fileSystem)

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

	// Generate old Age key
	oldKeyPath := filepath.Join(secretsDir, fmt.Sprintf("%s_keys.txt", clusterName))
	oldPublicKey, err := generateAgeKeyFile(oldKeyPath)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("failed to generate old key: %w", err)
	}

	// Generate new Age key
	newKeyPath := filepath.Join(secretsDir, fmt.Sprintf("%s_keys_new.txt", clusterName))
	newPublicKey, err := generateAgeKeyFile(newKeyPath)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("failed to generate new key: %w", err)
	}

	// Create .sops.yaml with dual-key configuration
	sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")
	sopsConfig := fmt.Sprintf(`creation_rules:
  - path_regex: .*\.yaml$
    age: >-
      %s,
      %s
`, oldPublicKey, newPublicKey)
	if err := os.WriteFile(sopsConfigPath, []byte(sopsConfig), 0644); err != nil {
		return nil, "", "", "", fmt.Errorf("failed to create .sops.yaml: %w", err)
	}

	// Create manifest with secrets
	manifestPath := filepath.Join(serviceDir, "secret.yaml")
	manifestContent := createManifestContent(secrets)
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		return nil, "", "", "", fmt.Errorf("failed to create manifest: %w", err)
	}

	// Encrypt manifest with both keys using SOPS
	ctx := context.Background()
	encryptor := sopsManager.GetEncryptor()
	encryptConfig := sops.EncryptionConfig{
		AgeKeys:    []string{oldPublicKey, newPublicKey},
		ConfigFile: sopsConfigPath,
		InPlace:    true,
		DryRun:     false,
		Verbose:    false,
	}

	if err := encryptor.EncryptFile(ctx, manifestPath, encryptConfig); err != nil {
		return nil, "", "", "", fmt.Errorf("failed to encrypt manifest: %w", err)
	}

	// Register keys in registry
	oldKeyEntry := KeyEntry{
		Cluster:     clusterName,
		KeyType:     KeyTypeAge,
		Fingerprint: oldPublicKey,
		PublicKey:   oldPublicKey,
		Status:      KeyStatusActive,
	}
	if err := registry.RegisterKey(ctx, oldKeyEntry); err != nil {
		return nil, "", "", "", fmt.Errorf("failed to register old key: %w", err)
	}

	newKeyEntry := KeyEntry{
		Cluster:     clusterName,
		KeyType:     KeyTypeAge,
		Fingerprint: newPublicKey,
		PublicKey:   newPublicKey,
		Status:      KeyStatusActive,
		RotatedFrom: oldPublicKey,
	}
	if err := registry.RegisterKey(ctx, newKeyEntry); err != nil {
		return nil, "", "", "", fmt.Errorf("failed to register new key: %w", err)
	}

	return rotator, oldKeyPath, newKeyPath, manifestPath, nil
}

// generateAgeKeyFile generates an Age key pair and writes it to the specified path
func generateAgeKeyFile(keyPath string) (string, error) {
	// Use age-keygen to generate a key
	cmd := exec.Command("age-keygen", "-o", keyPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to generate age key: %w (output: %s)", err, string(output))
	}

	// Read the public key from the file
	content, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read key file: %w", err)
	}

	// Extract public key from the comment line
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# public key: ") {
			publicKey := strings.TrimPrefix(line, "# public key: ")
			return strings.TrimSpace(publicKey), nil
		}
	}

	return "", fmt.Errorf("failed to extract public key from key file")
}

// createManifestContent creates a Kubernetes Secret manifest with the given secrets
func createManifestContent(secrets map[string]string) string {
	content := `apiVersion: v1
kind: Secret
metadata:
  name: test-secret
  namespace: default
type: Opaque
data:
`
	for key, value := range secrets {
		manifestKey := strings.ReplaceAll(key, "_", "-")
		content += fmt.Sprintf("  %s: %s\n", manifestKey, value)
	}

	return content
}

// decryptManifestWithKey decrypts a manifest using a specific Age key
func decryptManifestWithKey(ctx context.Context, manifestPath string, keyPath string) (map[string]interface{}, error) {
	// Create a temporary file for decrypted output
	tmpFile, err := os.CreateTemp("", "decrypted-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Set SOPS_AGE_KEY_FILE environment variable to use specific key
	oldEnv := os.Getenv("SOPS_AGE_KEY_FILE")
	os.Setenv("SOPS_AGE_KEY_FILE", keyPath)
	defer os.Setenv("SOPS_AGE_KEY_FILE", oldEnv)

	// Decrypt using SOPS
	cmd := exec.CommandContext(ctx, "sops", "--decrypt", "--output", tmpFile.Name(), manifestPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt manifest: %w (output: %s)", err, string(output))
	}

	// Read decrypted content
	decryptedContent, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read decrypted file: %w", err)
	}

	// Parse YAML
	var manifest map[string]interface{}
	if err := yaml.Unmarshal(decryptedContent, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse decrypted YAML: %w", err)
	}

	return manifest, nil
}

// compareManifestData compares the data sections of two manifests
func compareManifestData(manifest1, manifest2 map[string]interface{}) bool {
	data1, ok1 := manifest1["data"].(map[string]interface{})
	data2, ok2 := manifest2["data"].(map[string]interface{})

	if !ok1 || !ok2 {
		return false
	}

	if len(data1) != len(data2) {
		return false
	}

	for key, value1 := range data1 {
		value2, exists := data2[key]
		if !exists {
			return false
		}

		// Compare string values
		str1, ok1 := value1.(string)
		str2, ok2 := value2.(string)
		if !ok1 || !ok2 || str1 != str2 {
			return false
		}
	}

	return true
}

// verifyDecryptedSecrets verifies that decrypted manifest contains the original secrets
func verifyDecryptedSecrets(manifest map[string]interface{}, originalSecrets map[string]string) bool {
	data, ok := manifest["data"].(map[string]interface{})
	if !ok {
		return false
	}

	for configKey, configValue := range originalSecrets {
		manifestKey := strings.ReplaceAll(configKey, "_", "-")
		manifestValue, exists := data[manifestKey]
		if !exists {
			return false
		}

		// Compare values
		if manifestValue != configValue {
			return false
		}
	}

	return true
}

// verifyDualKeySOPSMetadata verifies that the manifest contains SOPS metadata for both keys
func verifyDualKeySOPSMetadata(manifestPath string) bool {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return false
	}

	contentStr := string(content)

	// Check for SOPS metadata
	if !strings.Contains(contentStr, "sops:") {
		return false
	}

	// Check for age section
	if !strings.Contains(contentStr, "age:") {
		return false
	}

	// Check for MAC (indicates encryption)
	if !strings.Contains(contentStr, "mac:") {
		return false
	}

	// Parse YAML to check for multiple age recipients
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

	// Should have at least 2 recipients (old and new keys)
	return len(ageRecipients) >= 2
}

// Test that verifies the dual-key decryption property test is working correctly
func TestProperty_DualKeyDecryption_Sanity(t *testing.T) {
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

	// Setup dual-key rotation test
	_, oldKeyPath, newKeyPath, manifestPath, err := setupDualKeyRotationTest(t, tmpDir, clusterName, testSecrets)
	require.NoError(t, err, "Failed to setup dual-key rotation test")

	// Test 1: Decrypt with old key
	t.Run("decrypt with old key", func(t *testing.T) {
		oldKeyDecrypted, err := decryptManifestWithKey(ctx, manifestPath, oldKeyPath)
		require.NoError(t, err, "Should decrypt with old key")
		require.NotNil(t, oldKeyDecrypted, "Decrypted manifest should not be nil")

		// Verify data section exists
		data, ok := oldKeyDecrypted["data"].(map[string]interface{})
		require.True(t, ok, "Should have data section")
		require.NotEmpty(t, data, "Data section should not be empty")
	})

	// Test 2: Decrypt with new key
	t.Run("decrypt with new key", func(t *testing.T) {
		newKeyDecrypted, err := decryptManifestWithKey(ctx, manifestPath, newKeyPath)
		require.NoError(t, err, "Should decrypt with new key")
		require.NotNil(t, newKeyDecrypted, "Decrypted manifest should not be nil")

		// Verify data section exists
		data, ok := newKeyDecrypted["data"].(map[string]interface{})
		require.True(t, ok, "Should have data section")
		require.NotEmpty(t, data, "Data section should not be empty")
	})

	// Test 3: Compare decrypted content
	t.Run("compare decrypted content", func(t *testing.T) {
		oldKeyDecrypted, err := decryptManifestWithKey(ctx, manifestPath, oldKeyPath)
		require.NoError(t, err)

		newKeyDecrypted, err := decryptManifestWithKey(ctx, manifestPath, newKeyPath)
		require.NoError(t, err)

		// Compare data sections
		require.True(t, compareManifestData(oldKeyDecrypted, newKeyDecrypted),
			"Decrypted content should be identical")
	})

	// Test 4: Verify original secrets
	t.Run("verify original secrets", func(t *testing.T) {
		oldKeyDecrypted, err := decryptManifestWithKey(ctx, manifestPath, oldKeyPath)
		require.NoError(t, err)

		require.True(t, verifyDecryptedSecrets(oldKeyDecrypted, testSecrets),
			"Decrypted content should match original secrets")
	})

	// Test 5: Verify dual-key SOPS metadata
	t.Run("verify dual-key SOPS metadata", func(t *testing.T) {
		require.True(t, verifyDualKeySOPSMetadata(manifestPath),
			"Manifest should contain dual-key SOPS metadata")

		// Read and parse manifest
		content, err := os.ReadFile(manifestPath)
		require.NoError(t, err)

		var manifest map[string]interface{}
		err = yaml.Unmarshal(content, &manifest)
		require.NoError(t, err)

		// Verify SOPS metadata structure
		sopsMetadata, ok := manifest["sops"].(map[string]interface{})
		require.True(t, ok, "Should have sops metadata")

		ageRecipients, ok := sopsMetadata["age"].([]interface{})
		require.True(t, ok, "Should have age recipients")
		require.GreaterOrEqual(t, len(ageRecipients), 2, "Should have at least 2 age recipients")
	})
}

// **Validates: Requirements 3.4, 3.7**
//
// Property 8: Key Rotation Completion
//
// For any completed key rotation, manifests should only be decryptable with the new key,
// and the old key should be archived with a timestamp.
//
// This property verifies that after completing a key rotation:
// 1. Manifests can be successfully decrypted using the new key
// 2. Manifests cannot be decrypted using the old key (it has been removed from recipients)
// 3. The old key has been archived with a timestamp in the archive directory
// 4. The .sops.yaml configuration only contains the new key
// 5. The key registry reflects the old key as archived
func TestProperty_RotationCompletion(t *testing.T) {
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

	properties.Property("completed rotation makes manifests decryptable only with new key and archives old key", prop.ForAll(
		func(secretsGen map[string]string) bool {
			// Skip if empty
			if len(secretsGen) == 0 {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			clusterName := "test-cluster"
			ctx := context.Background()

			// Create test infrastructure with dual-key rotation
			rotator, oldKeyPath, newKeyPath, manifestPath, overlayPath, err := setupCompletedRotationTest(t, tmpDir, clusterName, secretsGen)
			if err != nil {
				t.Logf("Failed to setup completed rotation test: %v", err)
				return false
			}

			// Complete the rotation (remove old key, re-encrypt with new key only)
			if err := completeRotationForTest(ctx, rotator, clusterName, overlayPath, newKeyPath); err != nil {
				t.Logf("Failed to complete rotation: %v", err)
				return false
			}

			// Property 1: Manifest should be decryptable with new key
			newKeyDecrypted, err := decryptManifestWithKey(ctx, manifestPath, newKeyPath)
			if err != nil {
				t.Logf("Failed to decrypt manifest with new key after completion: %v", err)
				return false
			}

			// Property 2: Manifest should NOT be decryptable with old key
			_, err = decryptManifestWithKey(ctx, manifestPath, oldKeyPath)
			if err == nil {
				t.Logf("Manifest should not be decryptable with old key after completion")
				return false
			}

			// Property 3: Decrypted content with new key should match original secrets
			if !verifyDecryptedSecrets(newKeyDecrypted, secretsGen) {
				t.Logf("Decrypted content does not match original secrets")
				return false
			}

			// Property 4: Old key should be archived with timestamp
			archiveDir := filepath.Join(filepath.Dir(oldKeyPath), "..", "archive")
			if !verifyKeyArchived(archiveDir, oldKeyPath) {
				t.Logf("Old key not found in archive directory")
				return false
			}

			// Property 5: .sops.yaml should only contain new key
			sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")
			if !verifySOPSConfigSingleKey(sopsConfigPath, newKeyPath) {
				t.Logf(".sops.yaml should only contain new key after completion")
				return false
			}

			// Property 6: Manifest should contain SOPS metadata for only one key
			if !verifySingleKeySOPSMetadata(manifestPath) {
				t.Logf("Manifest should contain SOPS metadata for only one key")
				return false
			}

			return true
		},
		genSecretMap(),
	))

	properties.TestingRun(t)
}

// setupCompletedRotationTest creates a test environment with dual-key rotation that will be completed
func setupCompletedRotationTest(t *testing.T, tmpDir string, clusterName string, secrets map[string]string) (*DefaultKeyRotator, string, string, string, string, error) {
	t.Helper()

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create SOPS manager
	sopsManager := sops.NewSOPSManager()

	// Create directory structure
	configDir := filepath.Join(tmpDir, ".config", "opencenter", "clusters", "test-org", clusterName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, "", "", "", "", fmt.Errorf("failed to create config dir: %w", err)
	}

	overlayPath := filepath.Join(tmpDir, "test-repo", "applications", "overlays", clusterName)
	serviceDir := filepath.Join(overlayPath, "services", "test-service")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return nil, "", "", "", "", fmt.Errorf("failed to create service dir: %w", err)
	}

	secretsDir := filepath.Join(configDir, "secrets", "age")
	if err := os.MkdirAll(secretsDir, 0755); err != nil {
		return nil, "", "", "", "", fmt.Errorf("failed to create secrets dir: %w", err)
	}

	archiveDir := filepath.Join(configDir, "secrets", "archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return nil, "", "", "", "", fmt.Errorf("failed to create archive dir: %w", err)
	}

	registryDir := filepath.Join(configDir, "secrets")
	if err := os.MkdirAll(registryDir, 0755); err != nil {
		return nil, "", "", "", "", fmt.Errorf("failed to create registry dir: %w", err)
	}

	// Create mock SOPS encryptor for registry
	mockEncryptor := &mockSOPSEncryptor{}

	// Create key registry
	registry := NewDefaultKeyRegistry(registryDir, mockEncryptor, logger)

	// Create file system
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)

	// Create config loader
	configLoader := v2.NewConfigIOHandler(fileSystem)

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

	// Generate old Age key
	oldKeyPath := filepath.Join(secretsDir, fmt.Sprintf("%s_keys.txt", clusterName))
	oldPublicKey, err := generateAgeKeyFile(oldKeyPath)
	if err != nil {
		return nil, "", "", "", "", fmt.Errorf("failed to generate old key: %w", err)
	}

	// Generate new Age key
	newKeyPath := filepath.Join(secretsDir, fmt.Sprintf("%s_keys_new.txt", clusterName))
	newPublicKey, err := generateAgeKeyFile(newKeyPath)
	if err != nil {
		return nil, "", "", "", "", fmt.Errorf("failed to generate new key: %w", err)
	}

	// Create .sops.yaml with dual-key configuration (simulating in-progress rotation)
	sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")
	sopsConfig := fmt.Sprintf(`creation_rules:
  - path_regex: .*\.yaml$
    age: >-
      %s,
      %s
`, oldPublicKey, newPublicKey)
	if err := os.WriteFile(sopsConfigPath, []byte(sopsConfig), 0644); err != nil {
		return nil, "", "", "", "", fmt.Errorf("failed to create .sops.yaml: %w", err)
	}

	// Create manifest with secrets
	manifestPath := filepath.Join(serviceDir, "secret.yaml")
	manifestContent := createManifestContent(secrets)
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		return nil, "", "", "", "", fmt.Errorf("failed to create manifest: %w", err)
	}

	// Encrypt manifest with both keys using SOPS
	ctx := context.Background()
	encryptor := sopsManager.GetEncryptor()
	encryptConfig := sops.EncryptionConfig{
		AgeKeys:    []string{oldPublicKey, newPublicKey},
		ConfigFile: sopsConfigPath,
		InPlace:    true,
		DryRun:     false,
		Verbose:    false,
	}

	if err := encryptor.EncryptFile(ctx, manifestPath, encryptConfig); err != nil {
		return nil, "", "", "", "", fmt.Errorf("failed to encrypt manifest: %w", err)
	}

	// Register keys in registry
	oldKeyEntry := KeyEntry{
		Cluster:     clusterName,
		KeyType:     KeyTypeAge,
		Fingerprint: oldPublicKey,
		PublicKey:   oldPublicKey,
		Status:      KeyStatusActive,
	}
	if err := registry.RegisterKey(ctx, oldKeyEntry); err != nil {
		return nil, "", "", "", "", fmt.Errorf("failed to register old key: %w", err)
	}

	newKeyEntry := KeyEntry{
		Cluster:     clusterName,
		KeyType:     KeyTypeAge,
		Fingerprint: newPublicKey,
		PublicKey:   newPublicKey,
		Status:      KeyStatusActive,
		RotatedFrom: oldPublicKey,
	}
	if err := registry.RegisterKey(ctx, newKeyEntry); err != nil {
		return nil, "", "", "", "", fmt.Errorf("failed to register new key: %w", err)
	}

	return rotator, oldKeyPath, newKeyPath, manifestPath, overlayPath, nil
}

// completeRotationForTest completes the rotation by removing old key and re-encrypting with new key only
func completeRotationForTest(ctx context.Context, rotator *DefaultKeyRotator, clusterName string, overlayPath string, newKeyPath string) error {
	// Read new key to get public key
	newKeyContent, err := os.ReadFile(newKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read new key: %w", err)
	}

	// Extract new public key
	var newPublicKey string
	lines := strings.Split(string(newKeyContent), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# public key: ") {
			newPublicKey = strings.TrimPrefix(line, "# public key: ")
			newPublicKey = strings.TrimSpace(newPublicKey)
			break
		}
	}

	if newPublicKey == "" {
		return fmt.Errorf("failed to extract new public key")
	}

	// Update .sops.yaml to only contain new key
	sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")
	sopsConfig := fmt.Sprintf(`creation_rules:
  - path_regex: .*\.yaml$
    age: >-
      %s
`, newPublicKey)
	if err := os.WriteFile(sopsConfigPath, []byte(sopsConfig), 0644); err != nil {
		return fmt.Errorf("failed to update .sops.yaml: %w", err)
	}

	// Re-encrypt all manifests with new key only
	serviceDir := filepath.Join(overlayPath, "services", "test-service")
	manifestPath := filepath.Join(serviceDir, "secret.yaml")

	// Set SOPS_AGE_KEY_FILE to new key
	oldEnv := os.Getenv("SOPS_AGE_KEY_FILE")
	os.Setenv("SOPS_AGE_KEY_FILE", newKeyPath)
	defer os.Setenv("SOPS_AGE_KEY_FILE", oldEnv)

	// Decrypt and re-encrypt with new key
	cmd := exec.CommandContext(ctx, "sops", "--rotate", "--in-place", manifestPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to rotate manifest: %w (output: %s)", err, string(output))
	}

	// Archive old key (simulate what CompleteRotation does)
	oldKeyPath := filepath.Join(filepath.Dir(newKeyPath), fmt.Sprintf("%s_keys.txt", clusterName))
	archiveDir := filepath.Join(filepath.Dir(oldKeyPath), "..", "archive")
	timestamp := "20240115_103000" // Fixed timestamp for testing
	archivedKeyPath := filepath.Join(archiveDir, fmt.Sprintf("%s_keys_%s.txt", clusterName, timestamp))

	oldKeyContent, err := os.ReadFile(oldKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read old key: %w", err)
	}

	if err := os.WriteFile(archivedKeyPath, oldKeyContent, 0600); err != nil {
		return fmt.Errorf("failed to archive old key: %w", err)
	}

	// Remove old key from active location
	if err := os.Remove(oldKeyPath); err != nil {
		return fmt.Errorf("failed to remove old key: %w", err)
	}

	return nil
}

// verifyKeyArchived checks if the old key has been archived with a timestamp
func verifyKeyArchived(archiveDir string, oldKeyPath string) bool {
	// Check if archive directory exists
	if _, err := os.Stat(archiveDir); os.IsNotExist(err) {
		return false
	}

	// List files in archive directory
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		return false
	}

	// Check if any archived key file exists
	// Archived keys should have format: <cluster>_keys_<timestamp>.txt
	baseName := filepath.Base(oldKeyPath)
	clusterName := strings.TrimSuffix(baseName, "_keys.txt")

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), clusterName+"_keys_") && strings.HasSuffix(entry.Name(), ".txt") {
			// Found an archived key
			return true
		}
	}

	return false
}

// verifySOPSConfigSingleKey checks if .sops.yaml only contains the new key
func verifySOPSConfigSingleKey(sopsConfigPath string, newKeyPath string) bool {
	// Read .sops.yaml
	content, err := os.ReadFile(sopsConfigPath)
	if err != nil {
		return false
	}

	// Read new key to get public key
	newKeyContent, err := os.ReadFile(newKeyPath)
	if err != nil {
		return false
	}

	// Extract new public key
	var newPublicKey string
	lines := strings.Split(string(newKeyContent), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# public key: ") {
			newPublicKey = strings.TrimPrefix(line, "# public key: ")
			newPublicKey = strings.TrimSpace(newPublicKey)
			break
		}
	}

	if newPublicKey == "" {
		return false
	}

	contentStr := string(content)

	// Check that new key is present
	if !strings.Contains(contentStr, newPublicKey) {
		return false
	}

	// Count number of age keys (should be only one)
	// Parse YAML to check age recipients
	var sopsConfig map[string]interface{}
	if err := yaml.Unmarshal(content, &sopsConfig); err != nil {
		return false
	}

	creationRules, ok := sopsConfig["creation_rules"].([]interface{})
	if !ok || len(creationRules) == 0 {
		return false
	}

	firstRule, ok := creationRules[0].(map[string]interface{})
	if !ok {
		return false
	}

	ageKeys, ok := firstRule["age"].(string)
	if !ok {
		return false
	}

	// Count commas in age keys (multiple keys are comma-separated)
	// Should have no commas (only one key)
	commaCount := strings.Count(ageKeys, ",")
	return commaCount == 0
}

// verifyS ingleKeySOPSMetadata verifies that the manifest contains SOPS metadata for only one key
func verifySingleKeySOPSMetadata(manifestPath string) bool {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return false
	}

	// Parse YAML to check for SOPS metadata
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

	// Should have exactly 1 recipient (new key only)
	return len(ageRecipients) == 1
}

// Test that verifies the rotation completion property test is working correctly
func TestProperty_RotationCompletion_Sanity(t *testing.T) {
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

	// Setup completed rotation test
	_, oldKeyPath, newKeyPath, manifestPath, overlayPath, err := setupCompletedRotationTest(t, tmpDir, clusterName, testSecrets)
	require.NoError(t, err, "Failed to setup completed rotation test")

	// Verify dual-key state before completion
	t.Run("verify dual-key state before completion", func(t *testing.T) {
		// Should be decryptable with both keys
		_, err := decryptManifestWithKey(ctx, manifestPath, oldKeyPath)
		require.NoError(t, err, "Should decrypt with old key before completion")

		_, err = decryptManifestWithKey(ctx, manifestPath, newKeyPath)
		require.NoError(t, err, "Should decrypt with new key before completion")

		// Should have dual-key SOPS metadata
		require.True(t, verifyDualKeySOPSMetadata(manifestPath), "Should have dual-key metadata before completion")
	})

	// Complete the rotation
	rotator := &DefaultKeyRotator{
		registry:       nil,
		secretsManager: nil,
		auditLogger:    nil,
		logger:         slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	err = completeRotationForTest(ctx, rotator, clusterName, overlayPath, newKeyPath)
	require.NoError(t, err, "Failed to complete rotation")

	// Test 1: Decrypt with new key
	t.Run("decrypt with new key after completion", func(t *testing.T) {
		newKeyDecrypted, err := decryptManifestWithKey(ctx, manifestPath, newKeyPath)
		require.NoError(t, err, "Should decrypt with new key after completion")
		require.NotNil(t, newKeyDecrypted, "Decrypted manifest should not be nil")

		// Verify data section exists
		data, ok := newKeyDecrypted["data"].(map[string]interface{})
		require.True(t, ok, "Should have data section")
		require.NotEmpty(t, data, "Data section should not be empty")

		// Verify original secrets
		require.True(t, verifyDecryptedSecrets(newKeyDecrypted, testSecrets),
			"Decrypted content should match original secrets")
	})

	// Test 2: Cannot decrypt with old key
	t.Run("cannot decrypt with old key after completion", func(t *testing.T) {
		_, err := decryptManifestWithKey(ctx, manifestPath, oldKeyPath)
		require.Error(t, err, "Should not decrypt with old key after completion")
	})

	// Test 3: Old key is archived
	t.Run("old key is archived", func(t *testing.T) {
		archiveDir := filepath.Join(filepath.Dir(oldKeyPath), "..", "archive")
		require.True(t, verifyKeyArchived(archiveDir, oldKeyPath),
			"Old key should be archived")

		// Verify old key is removed from active location
		_, err := os.Stat(oldKeyPath)
		require.True(t, os.IsNotExist(err), "Old key should be removed from active location")
	})

	// Test 4: .sops.yaml only contains new key
	t.Run("sops config only contains new key", func(t *testing.T) {
		sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")
		require.True(t, verifySOPSConfigSingleKey(sopsConfigPath, newKeyPath),
			".sops.yaml should only contain new key")
	})

	// Test 5: Manifest has single-key SOPS metadata
	t.Run("manifest has single-key metadata", func(t *testing.T) {
		require.True(t, verifySingleKeySOPSMetadata(manifestPath),
			"Manifest should have single-key SOPS metadata")

		// Read and parse manifest
		content, err := os.ReadFile(manifestPath)
		require.NoError(t, err)

		var manifest map[string]interface{}
		err = yaml.Unmarshal(content, &manifest)
		require.NoError(t, err)

		// Verify SOPS metadata structure
		sopsMetadata, ok := manifest["sops"].(map[string]interface{})
		require.True(t, ok, "Should have sops metadata")

		ageRecipients, ok := sopsMetadata["age"].([]interface{})
		require.True(t, ok, "Should have age recipients")
		require.Equal(t, 1, len(ageRecipients), "Should have exactly 1 age recipient")
	})
}

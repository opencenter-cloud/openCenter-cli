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
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/require"
)

// **Validates: Requirements 7.6, 7.7**
//
// Property 17: Pre-Commit Plaintext Key Detection
//
// For any staged file containing a plaintext Age private key or SSH private key,
// the pre-commit hook should detect and block the commit.
//
// This property verifies that:
// 1. Age private key files in secrets/age/ directory are detected
// 2. SSH private key files in secrets/ssh/ directory are detected
// 3. The hook result indicates the commit should be blocked (Passed = false)
// 4. The detected key files are listed in PlaintextKeys field
// 5. Non-key files are not incorrectly flagged as plaintext keys
func TestProperty_PlaintextKeyDetection(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("Age private key files are detected in staged changes", prop.ForAll(
		func(clusterName string, keyFileName string) bool {
			// Skip invalid inputs
			if clusterName == "" || keyFileName == "" {
				return true
			}

			// Setup
			tmpDir := t.TempDir()
			ctx := context.Background()

			// Create hook manager
			hookManager := setupHookManager(t, tmpDir)

			// Create Age key file path
			ageKeyPath := filepath.Join("secrets", "age", fmt.Sprintf("%s_%s.txt", clusterName, keyFileName))

			// Create the file in temp directory
			fullPath := filepath.Join(tmpDir, ageKeyPath)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				t.Logf("Failed to create directory: %v", err)
				return false
			}

			// Write a mock Age private key
			ageKeyContent := `# created: 2024-01-15T10:30:00Z
# public key: age1abcdefghijklmnopqrstuvwxyz1234567890abcdefghijklmnopqr
AGE-SECRET-KEY-1ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890ABCDEFGHIJKLMNOPQR`

			if err := os.WriteFile(fullPath, []byte(ageKeyContent), 0600); err != nil {
				t.Logf("Failed to write Age key file: %v", err)
				return false
			}

			// Run validation with the staged file
			result, err := hookManager.ValidatePreCommit(ctx, []string{ageKeyPath})
			if err != nil {
				t.Logf("ValidatePreCommit failed: %v", err)
				return false
			}

			// Property 1: Hook should not pass (commit should be blocked)
			if result.Passed {
				t.Logf("Hook should not pass when Age key file is staged")
				return false
			}

			// Property 2: PlaintextKeys should contain the Age key file
			if len(result.PlaintextKeys) == 0 {
				t.Logf("PlaintextKeys should contain the Age key file")
				return false
			}

			found := false
			for _, key := range result.PlaintextKeys {
				if key == ageKeyPath {
					found = true
					break
				}
			}

			if !found {
				t.Logf("Age key file not found in PlaintextKeys: %v", result.PlaintextKeys)
				return false
			}

			return true
		},
		genHookClusterName(),
		genHookKeyFileName(),
	))

	properties.Property("SSH private key files are detected in staged changes", prop.ForAll(
		func(clusterName string, keyType string) bool {
			// Skip invalid inputs
			if clusterName == "" || keyType == "" {
				return true
			}

			// Setup
			tmpDir := t.TempDir()
			ctx := context.Background()

			// Create hook manager
			hookManager := setupHookManager(t, tmpDir)

			// Create SSH key file path (private key without .pub extension)
			sshKeyPath := filepath.Join("secrets", "ssh", fmt.Sprintf("%s_%s", clusterName, keyType))

			// Create the file in temp directory
			fullPath := filepath.Join(tmpDir, sshKeyPath)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				t.Logf("Failed to create directory: %v", err)
				return false
			}

			// Write a mock SSH private key
			sshKeyContent := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACAbcdefghijklmnopqrstuvwxyz1234567890ABCDEFGHIJKLMNOPQR
-----END OPENSSH PRIVATE KEY-----`

			if err := os.WriteFile(fullPath, []byte(sshKeyContent), 0600); err != nil {
				t.Logf("Failed to write SSH key file: %v", err)
				return false
			}

			// Run validation with the staged file
			result, err := hookManager.ValidatePreCommit(ctx, []string{sshKeyPath})
			if err != nil {
				t.Logf("ValidatePreCommit failed: %v", err)
				return false
			}

			// Property 1: Hook should not pass (commit should be blocked)
			if result.Passed {
				t.Logf("Hook should not pass when SSH key file is staged")
				return false
			}

			// Property 2: PlaintextKeys should contain the SSH key file
			if len(result.PlaintextKeys) == 0 {
				t.Logf("PlaintextKeys should contain the SSH key file")
				return false
			}

			found := false
			for _, key := range result.PlaintextKeys {
				if key == sshKeyPath {
					found = true
					break
				}
			}

			if !found {
				t.Logf("SSH key file not found in PlaintextKeys: %v", result.PlaintextKeys)
				return false
			}

			return true
		},
		genHookClusterName(),
		genHookSSHKeyType(),
	))

	properties.Property("non-key files are not flagged as plaintext keys", prop.ForAll(
		func(fileName string) bool {
			// Skip invalid inputs
			if fileName == "" {
				return true
			}

			// Setup
			tmpDir := t.TempDir()
			ctx := context.Background()

			// Create hook manager
			hookManager := setupHookManager(t, tmpDir)

			// Create a non-key file path (e.g., in applications directory)
			nonKeyPath := filepath.Join("applications", "overlays", "test-cluster", "services", "test-service", fileName)

			// Create the file in temp directory
			fullPath := filepath.Join(tmpDir, nonKeyPath)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				t.Logf("Failed to create directory: %v", err)
				return false
			}

			// Write some non-key content
			content := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value`

			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				t.Logf("Failed to write file: %v", err)
				return false
			}

			// Run validation with the staged file
			result, err := hookManager.ValidatePreCommit(ctx, []string{nonKeyPath})
			if err != nil {
				t.Logf("ValidatePreCommit failed: %v", err)
				return false
			}

			// Property: PlaintextKeys should be empty for non-key files
			if len(result.PlaintextKeys) > 0 {
				t.Logf("Non-key file incorrectly flagged as plaintext key: %v", result.PlaintextKeys)
				return false
			}

			return true
		},
		genHookNonKeyFileName(),
	))

	properties.Property("SSH public key files (.pub) are not flagged as plaintext keys", prop.ForAll(
		func(clusterName string, keyType string) bool {
			// Skip invalid inputs
			if clusterName == "" || keyType == "" {
				return true
			}

			// Setup
			tmpDir := t.TempDir()
			ctx := context.Background()

			// Create hook manager
			hookManager := setupHookManager(t, tmpDir)

			// Create SSH public key file path (with .pub extension)
			sshPubKeyPath := filepath.Join("secrets", "ssh", fmt.Sprintf("%s_%s.pub", clusterName, keyType))

			// Create the file in temp directory
			fullPath := filepath.Join(tmpDir, sshPubKeyPath)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				t.Logf("Failed to create directory: %v", err)
				return false
			}

			// Write a mock SSH public key
			sshPubKeyContent := `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBtx3efghijklmnopqrstuvwxyz1234567890ABCDEFGHIJKLMNOPQR user@host`

			if err := os.WriteFile(fullPath, []byte(sshPubKeyContent), 0644); err != nil {
				t.Logf("Failed to write SSH public key file: %v", err)
				return false
			}

			// Run validation with the staged file
			result, err := hookManager.ValidatePreCommit(ctx, []string{sshPubKeyPath})
			if err != nil {
				t.Logf("ValidatePreCommit failed: %v", err)
				return false
			}

			// Property: PlaintextKeys should be empty for public key files
			if len(result.PlaintextKeys) > 0 {
				t.Logf("SSH public key file incorrectly flagged as plaintext key: %v", result.PlaintextKeys)
				return false
			}

			return true
		},
		genHookClusterName(),
		genHookSSHKeyType(),
	))

	properties.Property("multiple plaintext keys are all detected", prop.ForAll(
		func(clusterName string) bool {
			// Skip invalid inputs
			if clusterName == "" {
				return true
			}

			// Setup
			tmpDir := t.TempDir()
			ctx := context.Background()

			// Create hook manager
			hookManager := setupHookManager(t, tmpDir)

			// Create multiple key files
			ageKeyPath := filepath.Join("secrets", "age", fmt.Sprintf("%s_keys.txt", clusterName))
			sshKeyPath := filepath.Join("secrets", "ssh", fmt.Sprintf("%s_rsa", clusterName))

			// Create Age key file
			ageFullPath := filepath.Join(tmpDir, ageKeyPath)
			if err := os.MkdirAll(filepath.Dir(ageFullPath), 0755); err != nil {
				t.Logf("Failed to create Age directory: %v", err)
				return false
			}

			ageKeyContent := `# created: 2024-01-15T10:30:00Z
# public key: age1abcdefghijklmnopqrstuvwxyz1234567890abcdefghijklmnopqr
AGE-SECRET-KEY-1ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890ABCDEFGHIJKLMNOPQR`

			if err := os.WriteFile(ageFullPath, []byte(ageKeyContent), 0600); err != nil {
				t.Logf("Failed to write Age key file: %v", err)
				return false
			}

			// Create SSH key file
			sshFullPath := filepath.Join(tmpDir, sshKeyPath)
			if err := os.MkdirAll(filepath.Dir(sshFullPath), 0755); err != nil {
				t.Logf("Failed to create SSH directory: %v", err)
				return false
			}

			sshKeyContent := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACAbcdefghijklmnopqrstuvwxyz1234567890ABCDEFGHIJKLMNOPQR
-----END OPENSSH PRIVATE KEY-----`

			if err := os.WriteFile(sshFullPath, []byte(sshKeyContent), 0600); err != nil {
				t.Logf("Failed to write SSH key file: %v", err)
				return false
			}

			// Run validation with both staged files
			result, err := hookManager.ValidatePreCommit(ctx, []string{ageKeyPath, sshKeyPath})
			if err != nil {
				t.Logf("ValidatePreCommit failed: %v", err)
				return false
			}

			// Property 1: Hook should not pass
			if result.Passed {
				t.Logf("Hook should not pass when multiple key files are staged")
				return false
			}

			// Property 2: PlaintextKeys should contain both key files
			if len(result.PlaintextKeys) != 2 {
				t.Logf("PlaintextKeys should contain both key files, got %d", len(result.PlaintextKeys))
				return false
			}

			// Verify both keys are present
			foundAge := false
			foundSSH := false
			for _, key := range result.PlaintextKeys {
				if key == ageKeyPath {
					foundAge = true
				}
				if key == sshKeyPath {
					foundSSH = true
				}
			}

			if !foundAge || !foundSSH {
				t.Logf("Not all key files found in PlaintextKeys: %v", result.PlaintextKeys)
				return false
			}

			return true
		},
		genHookClusterName(),
	))

	properties.TestingRun(t)
}

// setupHookManager creates a hook manager for testing
func setupHookManager(t *testing.T, tmpDir string) *DefaultHookManager {
	t.Helper()

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create hook manager (secrets manager can be nil for these tests)
	hookManager := NewDefaultHookManager(nil, logger)

	return hookManager
}

// Generators for property-based testing

func genHookClusterName() gopter.Gen {
	return gen.OneConstOf(
		"test-cluster",
		"k8s-dev",
		"k8s-prod",
		"k8s-staging",
		"metro-bank",
	)
}

func genHookKeyFileName() gopter.Gen {
	return gen.OneConstOf(
		"keys",
		"keys_new",
		"keys_backup",
		"primary",
	)
}

func genHookSSHKeyType() gopter.Gen {
	return gen.OneConstOf(
		"rsa",
		"ed25519",
		"ecdsa",
		"dsa",
	)
}

func genHookNonKeyFileName() gopter.Gen {
	return gen.OneConstOf(
		"config.yaml",
		"deployment.yaml",
		"service.yaml",
		"configmap.yaml",
		"README.md",
		"kustomization.yaml",
	)
}

// Test that verifies the plaintext key detection property test is working correctly
func TestProperty_PlaintextKeyDetection_Sanity(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create hook manager
	hookManager := setupHookManager(t, tmpDir)

	// Test 1: Age private key detection
	t.Run("detect Age private key", func(t *testing.T) {
		ageKeyPath := filepath.Join("secrets", "age", "test-cluster_keys.txt")
		fullPath := filepath.Join(tmpDir, ageKeyPath)

		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)

		ageKeyContent := `# created: 2024-01-15T10:30:00Z
# public key: age1abcdefghijklmnopqrstuvwxyz1234567890abcdefghijklmnopqr
AGE-SECRET-KEY-1ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890ABCDEFGHIJKLMNOPQR`

		err = os.WriteFile(fullPath, []byte(ageKeyContent), 0600)
		require.NoError(t, err)

		result, err := hookManager.ValidatePreCommit(ctx, []string{ageKeyPath})
		require.NoError(t, err)
		require.False(t, result.Passed, "Hook should not pass with Age key file")
		require.NotEmpty(t, result.PlaintextKeys, "PlaintextKeys should contain Age key file")
		require.Contains(t, result.PlaintextKeys, ageKeyPath)
	})

	// Test 2: SSH private key detection
	t.Run("detect SSH private key", func(t *testing.T) {
		sshKeyPath := filepath.Join("secrets", "ssh", "test-cluster_rsa")
		fullPath := filepath.Join(tmpDir, sshKeyPath)

		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)

		sshKeyContent := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACAbcdefghijklmnopqrstuvwxyz1234567890ABCDEFGHIJKLMNOPQR
-----END OPENSSH PRIVATE KEY-----`

		err = os.WriteFile(fullPath, []byte(sshKeyContent), 0600)
		require.NoError(t, err)

		result, err := hookManager.ValidatePreCommit(ctx, []string{sshKeyPath})
		require.NoError(t, err)
		require.False(t, result.Passed, "Hook should not pass with SSH key file")
		require.NotEmpty(t, result.PlaintextKeys, "PlaintextKeys should contain SSH key file")
		require.Contains(t, result.PlaintextKeys, sshKeyPath)
	})

	// Test 3: Non-key file should not be flagged
	t.Run("non-key file not flagged", func(t *testing.T) {
		configPath := filepath.Join("applications", "overlays", "test-cluster", "services", "test-service", "config.yaml")
		fullPath := filepath.Join(tmpDir, configPath)

		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)

		content := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value`

		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)

		result, err := hookManager.ValidatePreCommit(ctx, []string{configPath})
		require.NoError(t, err)
		require.Empty(t, result.PlaintextKeys, "PlaintextKeys should be empty for non-key file")
	})

	// Test 4: SSH public key should not be flagged
	t.Run("SSH public key not flagged", func(t *testing.T) {
		sshPubKeyPath := filepath.Join("secrets", "ssh", "test-cluster_rsa.pub")
		fullPath := filepath.Join(tmpDir, sshPubKeyPath)

		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)

		sshPubKeyContent := `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBtx3efghijklmnopqrstuvwxyz1234567890ABCDEFGHIJKLMNOPQR user@host`

		err = os.WriteFile(fullPath, []byte(sshPubKeyContent), 0644)
		require.NoError(t, err)

		result, err := hookManager.ValidatePreCommit(ctx, []string{sshPubKeyPath})
		require.NoError(t, err)
		require.Empty(t, result.PlaintextKeys, "PlaintextKeys should be empty for public key file")
	})

	// Test 5: Multiple plaintext keys detected
	t.Run("multiple plaintext keys detected", func(t *testing.T) {
		ageKeyPath := filepath.Join("secrets", "age", "multi-cluster_keys.txt")
		sshKeyPath := filepath.Join("secrets", "ssh", "multi-cluster_ed25519")

		// Create Age key
		ageFullPath := filepath.Join(tmpDir, ageKeyPath)
		err := os.MkdirAll(filepath.Dir(ageFullPath), 0755)
		require.NoError(t, err)

		ageKeyContent := `# created: 2024-01-15T10:30:00Z
# public key: age1abcdefghijklmnopqrstuvwxyz1234567890abcdefghijklmnopqr
AGE-SECRET-KEY-1ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890ABCDEFGHIJKLMNOPQR`

		err = os.WriteFile(ageFullPath, []byte(ageKeyContent), 0600)
		require.NoError(t, err)

		// Create SSH key
		sshFullPath := filepath.Join(tmpDir, sshKeyPath)
		err = os.MkdirAll(filepath.Dir(sshFullPath), 0755)
		require.NoError(t, err)

		sshKeyContent := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACAbcdefghijklmnopqrstuvwxyz1234567890ABCDEFGHIJKLMNOPQR
-----END OPENSSH PRIVATE KEY-----`

		err = os.WriteFile(sshFullPath, []byte(sshKeyContent), 0600)
		require.NoError(t, err)

		result, err := hookManager.ValidatePreCommit(ctx, []string{ageKeyPath, sshKeyPath})
		require.NoError(t, err)
		require.False(t, result.Passed, "Hook should not pass with multiple key files")
		require.Len(t, result.PlaintextKeys, 2, "PlaintextKeys should contain both key files")
		require.Contains(t, result.PlaintextKeys, ageKeyPath)
		require.Contains(t, result.PlaintextKeys, sshKeyPath)
	})
}

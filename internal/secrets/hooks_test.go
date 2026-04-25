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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestHookManager creates a test hook manager with temporary directories
func setupTestHookManager(t *testing.T) (*DefaultHookManager, string, func()) {
	t.Helper()

	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "hook-manager-test-*")
	require.NoError(t, err)

	// Create hook manager (with nil secrets manager for unit tests)
	manager := NewDefaultHookManager(nil, slog.Default())

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return manager, tmpDir, cleanup
}

func TestNewDefaultHookManager(t *testing.T) {
	manager := NewDefaultHookManager(nil, slog.Default())
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.logger)
}

func TestNewDefaultHookManager_NilLogger(t *testing.T) {
	manager := NewDefaultHookManager(nil, nil)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.logger, "should use default logger when nil is provided")
}

func TestInstallHooks(t *testing.T) {
	manager, tmpDir, cleanup := setupTestHookManager(t)
	defer cleanup()

	// Create a mock git repository
	repoPath := filepath.Join(tmpDir, "test-repo")
	gitDir := filepath.Join(repoPath, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))

	ctx := context.Background()
	err := manager.InstallHooks(ctx, repoPath, "test-cluster")
	require.NoError(t, err)

	// Verify hook file was created
	hookPath := filepath.Join(gitDir, "hooks", "pre-commit")
	assert.FileExists(t, hookPath)

	// Verify hook file is executable
	info, err := os.Stat(hookPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm())

	// Verify hook content contains cluster name
	content, err := os.ReadFile(hookPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "test-cluster")
	assert.Contains(t, string(content), "#!/bin/bash")
	assert.Contains(t, string(content), "OPENCENTER_SKIP_HOOKS")
}

func TestInstallHooks_NotGitRepo(t *testing.T) {
	manager, tmpDir, cleanup := setupTestHookManager(t)
	defer cleanup()

	// Create a directory without .git
	repoPath := filepath.Join(tmpDir, "not-a-repo")
	require.NoError(t, os.MkdirAll(repoPath, 0755))

	ctx := context.Background()
	err := manager.InstallHooks(ctx, repoPath, "test-cluster")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
}

func TestUninstallHooks(t *testing.T) {
	manager, tmpDir, cleanup := setupTestHookManager(t)
	defer cleanup()

	// Create a mock git repository with hooks
	repoPath := filepath.Join(tmpDir, "test-repo")
	hooksDir := filepath.Join(repoPath, ".git", "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// Create a pre-commit hook
	hookPath := filepath.Join(hooksDir, "pre-commit")
	require.NoError(t, os.WriteFile(hookPath, []byte("#!/bin/bash\necho test"), 0755))

	ctx := context.Background()
	err := manager.UninstallHooks(ctx, repoPath)
	require.NoError(t, err)

	// Verify hook file was removed
	assert.NoFileExists(t, hookPath)
}

func TestUninstallHooks_NoHook(t *testing.T) {
	manager, tmpDir, cleanup := setupTestHookManager(t)
	defer cleanup()

	// Create a mock git repository without hooks
	repoPath := filepath.Join(tmpDir, "test-repo")
	gitDir := filepath.Join(repoPath, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))

	ctx := context.Background()
	err := manager.UninstallHooks(ctx, repoPath)
	require.NoError(t, err) // Should not error if hook doesn't exist
}

func TestIsPlaintextKeyFile(t *testing.T) {
	manager := NewDefaultHookManager(nil, slog.Default())

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{
			name:     "Age private key",
			filePath: "secrets/age/test-cluster_keys.txt",
			want:     true,
		},
		{
			name:     "SSH RSA private key",
			filePath: "secrets/ssh/test-cluster_rsa",
			want:     true,
		},
		{
			name:     "SSH ed25519 private key",
			filePath: "secrets/ssh/test-cluster_ed25519",
			want:     true,
		},
		{
			name:     "SSH public key",
			filePath: "secrets/ssh/test-cluster_rsa.pub",
			want:     false,
		},
		{
			name:     "Regular file",
			filePath: "config/test.yaml",
			want:     false,
		},
		{
			name:     "Age public key",
			filePath: "secrets/age/test-cluster.pub",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := manager.isPlaintextKeyFile(tt.filePath)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsManifestFile(t *testing.T) {
	manager := NewDefaultHookManager(nil, slog.Default())

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{
			name:     "Valid manifest file",
			filePath: "applications/overlays/test-cluster/services/cert-manager/secret.yaml",
			want:     true,
		},
		{
			name:     "Valid manifest file with different service",
			filePath: "applications/overlays/prod-cluster/services/loki/secret.yaml",
			want:     true,
		},
		{
			name:     "Not a secret.yaml file",
			filePath: "applications/overlays/test-cluster/services/cert-manager/config.yaml",
			want:     false,
		},
		{
			name:     "Wrong directory structure",
			filePath: "config/secret.yaml",
			want:     false,
		},
		{
			name:     "Missing services directory",
			filePath: "applications/overlays/test-cluster/cert-manager/secret.yaml",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := manager.isManifestFile(tt.filePath)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCheckFileEncryption(t *testing.T) {
	manager, tmpDir, cleanup := setupTestHookManager(t)
	defer cleanup()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name: "SOPS encrypted file",
			content: `apiVersion: v1
kind: Secret
metadata:
  name: test-secret
data:
  password: ENC[AES256_GCM,data:abc123,iv:def456,tag:ghi789,type:str]
sops:
  kms: []
  gcp_kms: []
  azure_kv: []
  hc_vault: []
  age:
    - recipient: age1234567890
      enc: |
        -----BEGIN AGE ENCRYPTED FILE-----
        test
        -----END AGE ENCRYPTED FILE-----
  lastmodified: "2024-01-15T10:30:00Z"
  mac: ENC[AES256_GCM,data:abc123,iv:def456,tag:ghi789,type:str]
  pgp: []
  version: 3.8.1`,
			want: true,
		},
		{
			name: "Unencrypted file",
			content: `apiVersion: v1
kind: Secret
metadata:
  name: test-secret
data:
  password: plaintext-password`,
			want: false,
		},
		{
			name: "File with sops but no mac",
			content: `apiVersion: v1
kind: Secret
metadata:
  name: test-secret
data:
  password: test
sops:
  version: 3.8.1`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			testFile := filepath.Join(tmpDir, "test-secret.yaml")
			require.NoError(t, os.WriteFile(testFile, []byte(tt.content), 0644))

			got, err := manager.checkFileEncryption(testFile)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidatePreCommit(t *testing.T) {
	manager, tmpDir, cleanup := setupTestHookManager(t)
	defer cleanup()

	unencryptedContent := `apiVersion: v1
kind: Secret
data:
  password: plaintext`

	encryptedContent := `apiVersion: v1
kind: Secret
data:
  password: ENC[AES256_GCM,data:abc123]
sops:
  mac: test
  lastmodified: "2024-01-15T10:30:00Z"`

	tests := []struct {
		name        string
		stagedFiles []string
		encrypted   bool // whether the manifest should be encrypted
		wantPassed  bool
		wantUnenc   int
		wantKeys    int
	}{
		{
			name: "All files valid",
			stagedFiles: []string{
				"applications/overlays/test/services/cert-manager/secret.yaml",
			},
			encrypted:  true,
			wantPassed: true,
			wantUnenc:  0,
			wantKeys:   0,
		},
		{
			name: "Plaintext key detected",
			stagedFiles: []string{
				"secrets/age/test-cluster_keys.txt",
			},
			encrypted:  false,
			wantPassed: false,
			wantUnenc:  0,
			wantKeys:   1,
		},
		{
			name: "Unencrypted manifest detected",
			stagedFiles: []string{
				"applications/overlays/test/services/loki/secret.yaml",
			},
			encrypted:  false,
			wantPassed: false,
			wantUnenc:  1,
			wantKeys:   0,
		},
		{
			name: "Multiple issues",
			stagedFiles: []string{
				"secrets/age/test-cluster_keys.txt",
				"applications/overlays/test/services/loki/secret.yaml",
			},
			encrypted:  false,
			wantPassed: false,
			wantUnenc:  1,
			wantKeys:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create actual files for manifest checks
			// We'll pass the relative paths to ValidatePreCommit but create files with absolute paths
			for _, file := range tt.stagedFiles {
				if manager.isManifestFile(file) {
					fullPath := filepath.Join(tmpDir, file)
					require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
					// Use unencrypted content for testing
					require.NoError(t, os.WriteFile(fullPath, []byte(unencryptedContent), 0644))
				}
			}

			// For the test, we need to pass paths that can be checked
			// In a real scenario, these would be relative to the repo root
			// For testing, we'll create a temporary "repo" structure
			testRepoDir := filepath.Join(tmpDir, "test-repo")
			require.NoError(t, os.MkdirAll(testRepoDir, 0755))

			// Create files in the test repo with the correct structure
			testStagedFiles := make([]string, len(tt.stagedFiles))
			for i, file := range tt.stagedFiles {
				fullPath := filepath.Join(testRepoDir, file)
				testStagedFiles[i] = fullPath

				if manager.isManifestFile(file) {
					require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
					content := unencryptedContent
					if tt.encrypted {
						content = encryptedContent
					}
					require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
				}
			}

			ctx := context.Background()
			result, err := manager.ValidatePreCommit(ctx, testStagedFiles)
			require.NoError(t, err)

			assert.Equal(t, tt.wantPassed, result.Passed, "Passed status mismatch")
			assert.Len(t, result.UnencryptedFiles, tt.wantUnenc, "Unencrypted files count mismatch")
			assert.Len(t, result.PlaintextKeys, tt.wantKeys, "Plaintext keys count mismatch")
		})
	}
}

func TestGenerateHookScript(t *testing.T) {
	manager := NewDefaultHookManager(nil, slog.Default())

	script := manager.generateHookScript("test-cluster")

	// Verify script contains required elements
	assert.Contains(t, script, "#!/bin/bash")
	assert.Contains(t, script, "test-cluster")
	assert.Contains(t, script, "OPENCENTER_SKIP_HOOKS")
	assert.Contains(t, script, "secrets/age/")
	assert.Contains(t, script, "secrets/ssh/")
	assert.Contains(t, script, "applications/overlays/")
	assert.Contains(t, script, "sops:")
	assert.Contains(t, script, "opencenter secrets sync")

	// Verify drift detection is included
	assert.Contains(t, script, "opencenter secrets validate")
	assert.Contains(t, script, "Configuration drift detected")
	assert.Contains(t, script, "command -v opencenter")
}

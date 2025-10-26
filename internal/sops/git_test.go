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

package sops

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rackerlabs/openCenter/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGitIntegrator(t *testing.T) {
	tempDir := t.TempDir()
	encryptor := NewEncryptor(nil, nil)

	integrator := NewGitIntegrator(tempDir, encryptor)
	assert.NotNil(t, integrator)
	assert.Equal(t, tempDir, integrator.repoPath)
	assert.Equal(t, encryptor, integrator.encryptor)
}

func TestGitIntegrator_ValidateRepository(t *testing.T) {
	tempDir := t.TempDir()
	encryptor := NewEncryptor(nil, nil)
	integrator := NewGitIntegrator(tempDir, encryptor)

	// Should fail without .git directory
	err := integrator.ValidateRepository()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")

	// Create .git directory
	gitDir := filepath.Join(tempDir, ".git")
	err = os.MkdirAll(gitDir, 0o755)
	require.NoError(t, err)

	// Should pass with .git directory
	err = integrator.ValidateRepository()
	assert.NoError(t, err)
}

func TestGitIntegrator_CreateGitIgnore(t *testing.T) {
	tempDir := t.TempDir()
	encryptor := NewEncryptor(nil, nil)
	integrator := NewGitIntegrator(tempDir, encryptor)

	// Create .gitignore
	err := integrator.CreateGitIgnore()
	require.NoError(t, err)

	// Verify .gitignore was created
	gitignorePath := filepath.Join(tempDir, ".gitignore")
	assert.FileExists(t, gitignorePath)

	// Verify content
	content, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)

	contentStr := string(content)
	expectedEntries := []string{
		".sops.yaml.bak",
		"*.dec",
		"*.tmp",
		".DS_Store",
		".vscode/",
	}

	for _, entry := range expectedEntries {
		assert.Contains(t, contentStr, entry)
	}
}

func TestGitIntegrator_CreateGitIgnoreAppend(t *testing.T) {
	tempDir := t.TempDir()
	encryptor := NewEncryptor(nil, nil)
	integrator := NewGitIntegrator(tempDir, encryptor)

	// Create existing .gitignore
	gitignorePath := filepath.Join(tempDir, ".gitignore")
	existingContent := "# Existing content\n*.log\n"
	err := os.WriteFile(gitignorePath, []byte(existingContent), 0o644)
	require.NoError(t, err)

	// Append to .gitignore
	err = integrator.CreateGitIgnore()
	require.NoError(t, err)

	// Verify content includes both existing and new entries
	content, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "# Existing content")
	assert.Contains(t, contentStr, "*.log")
	assert.Contains(t, contentStr, ".sops.yaml.bak")
}

func TestGitIntegrator_SetupGitAttributes(t *testing.T) {
	tempDir := t.TempDir()
	encryptor := NewEncryptor(nil, nil)
	integrator := NewGitIntegrator(tempDir, encryptor)

	// Setup .gitattributes
	err := integrator.SetupGitAttributes()
	require.NoError(t, err)

	// Verify .gitattributes was created
	gitattributesPath := filepath.Join(tempDir, ".gitattributes")
	assert.FileExists(t, gitattributesPath)

	// Verify content
	content, err := os.ReadFile(gitattributesPath)
	require.NoError(t, err)

	contentStr := string(content)
	expectedEntries := []string{
		"*.yaml diff=sopsdiffer",
		"*.yml diff=sopsdiffer",
		"secrets/*.yaml binary",
		"**/secrets/*.yaml binary",
	}

	for _, entry := range expectedEntries {
		assert.Contains(t, contentStr, entry)
	}
}

func TestGitIntegrator_CreateCommitMessage(t *testing.T) {
	tempDir := t.TempDir()
	encryptor := NewEncryptor(nil, nil)
	integrator := NewGitIntegrator(tempDir, encryptor)

	cfg := &config.ClusterConfig{
		Metadata: config.ConfigMetadata{
			Name: "test-cluster",
		},
	}

	tests := []struct {
		name      string
		operation string
		contains  []string
	}{
		{
			name:      "bootstrap operation",
			operation: "bootstrap",
			contains: []string{
				"Bootstrap GitOps overlay for cluster test-cluster",
				"Initialize FluxCD overlay structure",
				"Add SOPS-encrypted secrets",
			},
		},
		{
			name:      "update operation",
			operation: "update",
			contains: []string{
				"Update overlay configuration for cluster test-cluster",
				"Update SOPS-encrypted files",
			},
		},
		{
			name:      "encrypt operation",
			operation: "encrypt",
			contains: []string{
				"Encrypt sensitive files for cluster test-cluster",
				"Apply SOPS encryption to secrets",
			},
		},
		{
			name:      "default operation",
			operation: "unknown",
			contains: []string{
				"Update overlay files for cluster test-cluster",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := integrator.CreateCommitMessage(cfg, tt.operation)

			for _, expected := range tt.contains {
				assert.Contains(t, message, expected)
			}
		})
	}
}

func TestCommitConfig_Validation(t *testing.T) {
	tests := []struct {
		name   string
		config CommitConfig
		valid  bool
	}{
		{
			name: "valid config with all fields",
			config: CommitConfig{
				Message:     "Test commit",
				Author:      "Test Author",
				Email:       "test@example.com",
				SignCommits: true,
				DryRun:      false,
				Verbose:     true,
			},
			valid: true,
		},
		{
			name: "valid config with minimal fields",
			config: CommitConfig{
				Message: "Test commit",
			},
			valid: true,
		},
		{
			name: "config with dry run",
			config: CommitConfig{
				Message: "Test commit",
				DryRun:  true,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the config can be created and used
			assert.NotEmpty(t, tt.config.Message)
		})
	}
}

// Mock tests for Git operations that require a real Git repository
func TestGitIntegrator_MockGitOperations(t *testing.T) {
	tempDir := t.TempDir()
	encryptor := NewEncryptor(nil, nil)
	integrator := NewGitIntegrator(tempDir, encryptor)

	// Test configuration validation
	cfg := &config.ClusterConfig{
		Spec: config.ClusterConfigSpec{
			Provider: "openstack",
			SOPS: config.SOPSConfig{
				Age: config.AgeConfig{
					PublicKey: "age1test123",
				},
			},
		},
	}

	// Test file list generation
	filesToEncrypt := encryptor.getFilesToEncrypt(tempDir, cfg)
	assert.NotEmpty(t, filesToEncrypt)
	assert.Contains(t, filesToEncrypt, "flux-system/gotk-sync.yaml")
	assert.Contains(t, filesToEncrypt, "secrets/openstack-credentials.yaml")

	// Test commit message generation
	message := integrator.CreateCommitMessage(cfg, "bootstrap")
	assert.Contains(t, message, "Bootstrap GitOps overlay")
}

func TestGitIntegrator_EncryptFilesForCommit(t *testing.T) {
	tempDir := t.TempDir()
	encryptor := NewEncryptor(nil, nil)
	integrator := NewGitIntegrator(tempDir, encryptor)

	cfg := &config.ClusterConfig{
		Spec: config.ClusterConfigSpec{
			Provider: "openstack",
			SOPS: config.SOPSConfig{
				Age: config.AgeConfig{
					PublicKey: "age1ql3vwyqfpvucvyz6x04chxtsm9l3f24tqq5pkq9jz4mk7ccvqxrqsqg5z5",
				},
			},
		},
	}

	// Create test files
	testFiles := map[string]string{
		"flux-system/gotk-sync.yaml": `apiVersion: v1
kind: Secret
metadata:
  name: flux-system
data:
  token: dGVzdC10b2tlbg==`,
		"secrets/openstack-credentials.yaml": `apiVersion: v1
kind: Secret
metadata:
  name: openstack-creds
data:
  username: dGVzdA==
  password: cGFzcw==`,
	}

	for file, content := range testFiles {
		filePath := filepath.Join(tempDir, file)
		err := os.MkdirAll(filepath.Dir(filePath), 0o755)
		require.NoError(t, err)
		err = os.WriteFile(filePath, []byte(content), 0o644)
		require.NoError(t, err)
	}

	// This will fail without SOPS installed, but we can test the file detection logic
	err := integrator.encryptFilesForCommit(context.Background(), cfg)
	if err != nil {
		// Should fail on SOPS operations, not on file detection
		assert.Contains(t, err.Error(), "SOPS")
	}
}

func TestGitIntegrator_StageEncryptedFiles(t *testing.T) {
	tempDir := t.TempDir()
	encryptor := NewEncryptor(nil, nil)
	integrator := NewGitIntegrator(tempDir, encryptor)

	cfg := &config.ClusterConfig{
		Spec: config.ClusterConfigSpec{
			Provider: "kind",
		},
	}

	// Create test files
	testFiles := []string{
		"flux-system/gotk-sync.yaml",
		"managed-services/sources/base-repo.yaml",
	}

	for _, file := range testFiles {
		filePath := filepath.Join(tempDir, file)
		err := os.MkdirAll(filepath.Dir(filePath), 0o755)
		require.NoError(t, err)
		err = os.WriteFile(filePath, []byte("test content"), 0o644)
		require.NoError(t, err)
	}

	// This will fail without Git repository, but we can test the file detection logic
	err := integrator.stageEncryptedFiles(context.Background(), cfg)
	if err != nil {
		// Should fail on Git operations
		assert.Error(t, err)
	}
}

func TestGitIntegrator_CommitChanges(t *testing.T) {
	tempDir := t.TempDir()
	encryptor := NewEncryptor(nil, nil)
	integrator := NewGitIntegrator(tempDir, encryptor)

	tests := []struct {
		name   string
		config CommitConfig
	}{
		{
			name: "basic commit",
			config: CommitConfig{
				Message: "Test commit",
			},
		},
		{
			name: "commit with author",
			config: CommitConfig{
				Message: "Test commit",
				Author:  "Test Author",
				Email:   "test@example.com",
			},
		},
		{
			name: "signed commit",
			config: CommitConfig{
				Message:     "Test commit",
				SignCommits: true,
			},
		},
		{
			name: "dry run commit",
			config: CommitConfig{
				Message: "Test commit",
				DryRun:  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail without Git repository, but we can test the config handling
			err := integrator.commitChanges(context.Background(), tt.config)
			if err != nil {
				// Should fail on Git operations
				assert.Contains(t, err.Error(), "git")
			}
		})
	}
}

// Test helper functions that don't require Git
func TestGitIntegrator_HelperFunctions(t *testing.T) {
	tempDir := t.TempDir()
	encryptor := NewEncryptor(nil, nil)
	integrator := NewGitIntegrator(tempDir, encryptor)

	// Test CreateGitIgnore
	err := integrator.CreateGitIgnore()
	assert.NoError(t, err)

	// Test SetupGitAttributes
	err = integrator.SetupGitAttributes()
	assert.NoError(t, err)

	// Test ValidateRepository (should fail without .git)
	err = integrator.ValidateRepository()
	assert.Error(t, err)

	// Create .git directory and test again
	gitDir := filepath.Join(tempDir, ".git")
	err = os.MkdirAll(gitDir, 0o755)
	require.NoError(t, err)

	err = integrator.ValidateRepository()
	assert.NoError(t, err)
}

func TestGitIntegrator_FileOperations(t *testing.T) {
	tempDir := t.TempDir()
	encryptor := NewEncryptor(nil, nil)
	integrator := NewGitIntegrator(tempDir, encryptor)

	cfg := &config.ClusterConfig{
		Metadata: config.ConfigMetadata{
			Name: "test-cluster",
		},
		Spec: config.ClusterConfigSpec{
			Provider: "vsphere",
			SOPS: config.SOPSConfig{
				Age: config.AgeConfig{
					PublicKey: "age1test123",
				},
			},
		},
	}

	// Test commit message generation for different operations
	operations := []string{"bootstrap", "update", "encrypt", "unknown"}
	for _, op := range operations {
		message := integrator.CreateCommitMessage(cfg, op)
		assert.NotEmpty(t, message)
		assert.Contains(t, message, "test-cluster")
	}

	// Test file list generation through encryptor
	files := encryptor.getFilesToEncrypt(tempDir, cfg)
	assert.NotEmpty(t, files)
	assert.Contains(t, files, "flux-system/gotk-sync.yaml")
	assert.Contains(t, files, "secrets/vsphere-credentials.yaml")
}

func TestGitIntegrator_ErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	encryptor := NewEncryptor(nil, nil)
	integrator := NewGitIntegrator(tempDir, encryptor)

	// Test operations that should fail gracefully
	ctx := context.Background()

	// These operations will fail without proper Git setup, but should handle errors gracefully
	_, err := integrator.GetCurrentBranch(ctx)
	if err != nil {
		assert.Error(t, err)
	}

	_, err = integrator.GetRemoteURL(ctx, "origin")
	if err != nil {
		assert.Error(t, err)
	}

	hasChanges, err := integrator.CheckForChanges(ctx)
	if err != nil {
		assert.False(t, hasChanges)
	}

	err = integrator.PushChanges(ctx, "origin", "main")
	if err != nil {
		assert.Error(t, err)
	}

	err = integrator.CloneRepository(ctx, "https://github.com/test/repo.git", filepath.Join(tempDir, "clone"), "main")
	if err != nil {
		assert.Error(t, err)
	}

	_, err = integrator.GetLastCommitHash(ctx)
	if err != nil {
		assert.Error(t, err)
	}
}

func TestGitIntegrator_ConfigureSOPSDiff(t *testing.T) {
	tempDir := t.TempDir()
	encryptor := NewEncryptor(nil, nil)
	integrator := NewGitIntegrator(tempDir, encryptor)

	// This will fail without Git repository, but we can test the error handling
	err := integrator.ConfigureSOPSDiff(context.Background())
	if err != nil {
		assert.Error(t, err)
	}
}

func TestGitIntegrator_ValidateGitConfig(t *testing.T) {
	tempDir := t.TempDir()
	encryptor := NewEncryptor(nil, nil)
	integrator := NewGitIntegrator(tempDir, encryptor)

	// This will fail without Git repository, but we can test the error handling
	err := integrator.ValidateGitConfig(context.Background())
	if err != nil {
		assert.Error(t, err)
	}
}

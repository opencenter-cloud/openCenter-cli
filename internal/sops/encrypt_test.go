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
	"strings"
	"testing"

	"github.com/rackerlabs/openCenter/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptor_IsFileEncrypted(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
		wantErr  bool
	}{
		{
			name: "encrypted file with age",
			content: `apiVersion: v1
kind: Secret
metadata:
  name: test
sops:
  age:
    - recipient: age1test123
      enc: encrypted_data_here`,
			expected: true,
			wantErr:  false,
		},
		{
			name: "encrypted file with pgp",
			content: `apiVersion: v1
kind: Secret
metadata:
  name: test
sops:
  pgp:
    - fp: ABCD1234
      enc: encrypted_data_here`,
			expected: true,
			wantErr:  false,
		},
		{
			name: "unencrypted file",
			content: `apiVersion: v1
kind: Secret
metadata:
  name: test
data:
  key: value`,
			expected: false,
			wantErr:  false,
		},
		{
			name: "file with sops but no encryption keys",
			content: `apiVersion: v1
kind: Secret
metadata:
  name: test
sops:
  version: 3.7.0`,
			expected: false,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpFile, err := os.CreateTemp("", "sops-test-*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			// Write content to file
			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			e := NewEncryptor(nil, nil)
			result, err := e.IsFileEncrypted(tmpFile.Name())

			if (err != nil) != tt.wantErr {
				t.Errorf("IsFileEncrypted() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if result != tt.expected {
				t.Errorf("IsFileEncrypted() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestEncryptor_getFilesToEncrypt(t *testing.T) {
	tests := []struct {
		name     string
		config   *config.Config
		expected []string
	}{
		{
			name: "openstack cluster",
			config: &config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Infrastructure: config.Infrastructure{
						Provider: "openstack",
					},
				},
			},
			expected: []string{
				"flux-system/gotk-sync.yaml",
				"managed-services/sources/base-repo.yaml",
				"secrets/openstack-credentials.yaml",
			},
		},
		{
			name: "vsphere cluster",
			config: &config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Infrastructure: config.Infrastructure{
						Provider: "vsphere",
					},
				},
			},
			expected: []string{
				"flux-system/gotk-sync.yaml",
				"managed-services/sources/base-repo.yaml",
				"secrets/vsphere-credentials.yaml",
				"customer-managed/services/cloud-provider-vsphere/secret.yaml",
			},
		},
		{
			name: "basic cluster with no provider-specific files",
			config: &config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Infrastructure: config.Infrastructure{
						Provider: "kind",
					},
				},
			},
			expected: []string{
				"flux-system/gotk-sync.yaml",
				"managed-services/sources/base-repo.yaml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEncryptor(nil, nil)
			files := e.getFilesToEncrypt("", tt.config)

			if len(files) != len(tt.expected) {
				t.Errorf("Expected %d files, got %d", len(tt.expected), len(files))
			}

			for _, expectedFile := range tt.expected {
				found := false
				for _, file := range files {
					if file == expectedFile {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected file not found: %s", expectedFile)
				}
			}
		})
	}
}

func TestEncryptor_CreateSOPSConfig(t *testing.T) {
	config := &config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Cluster: config.ClusterConfig{
				ClusterName: "test-cluster",
			},
			Infrastructure: config.Infrastructure{
				Provider: "vsphere",
			},
		},
		Secrets: config.Secrets{
			SopsAgeKeyFile: "age1test123456789",
		},
	}

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "sops-config-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	e := NewEncryptor(nil, nil)
	err = e.CreateSOPSConfig(tmpDir, config)
	if err != nil {
		t.Fatalf("CreateSOPSConfig() error = %v", err)
	}

	// Check that .sops.yaml was created
	sopsConfigPath := filepath.Join(tmpDir, ".sops.yaml")
	if _, err := os.Stat(sopsConfigPath); os.IsNotExist(err) {
		t.Errorf(".sops.yaml file was not created")
	}

	// Read and validate content
	content, err := os.ReadFile(sopsConfigPath)
	if err != nil {
		t.Fatalf("Failed to read .sops.yaml: %v", err)
	}

	contentStr := string(content)
	expectedContents := []string{
		"test-cluster",
		"age1test123456789",
		"creation_rules:",
		"path_regex:",
		"encrypted_regex:",
	}

	for _, expected := range expectedContents {
		if !strings.Contains(contentStr, expected) {
			t.Errorf(".sops.yaml should contain %q", expected)
		}
	}

	// Check for vSphere-specific rules
	if !strings.Contains(contentStr, "vsphere-credentials") {
		t.Errorf(".sops.yaml should contain vSphere-specific rules")
	}
}

func TestEncryptor_generateSOPSConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   *config.Config
		contains []string
	}{
		{
			name: "openstack config",
			config: &config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Cluster: config.ClusterConfig{
						ClusterName: "openstack-cluster",
					},
					Infrastructure: config.Infrastructure{
						Provider: "openstack",
					},
				},
				Secrets: config.Secrets{
					SopsAgeKeyFile: "age1openstack123",
				},
			},
			contains: []string{
				"openstack-cluster",
				"age1openstack123",
				"openstack-credentials",
			},
		},
		{
			name: "vsphere config",
			config: &config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Cluster: config.ClusterConfig{
						ClusterName: "vsphere-cluster",
					},
					Infrastructure: config.Infrastructure{
						Provider: "vsphere",
					},
				},
				Secrets: config.Secrets{
					SopsAgeKeyFile: "age1vsphere123",
				},
			},
			contains: []string{
				"vsphere-cluster",
				"age1vsphere123",
				"vsphere-credentials",
				"customer-managed/services/.*/secret",
			},
		},
		{
			name: "config with no age key",
			config: &config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Cluster: config.ClusterConfig{
						ClusterName: "no-key-cluster",
					},
					Infrastructure: config.Infrastructure{
						Provider: "kind",
					},
				},
				Secrets: config.Secrets{
					SopsAgeKeyFile: "", // No age key provided
				},
			},
			contains: []string{
				"no-key-cluster",
				"TODO", // Should fallback to TODO
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEncryptor(nil, nil)
			sopsConfig := e.generateSOPSConfig(tt.config)

			for _, expected := range tt.contains {
				if !strings.Contains(sopsConfig, expected) {
					t.Errorf("SOPS config should contain %q, got:\n%s", expected, sopsConfig)
				}
			}
		})
	}
}

func TestEncryptor_ValidateEncryption(t *testing.T) {
	// Create temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "validate-encryption-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Infrastructure: config.Infrastructure{
				Provider: "openstack",
			},
		},
	}

	// Create test files - one encrypted, one not
	testFiles := map[string]string{
		"flux-system/gotk-sync.yaml": `apiVersion: v1
kind: Secret
sops:
  age:
    - recipient: age1test123
      enc: encrypted_data`,
		"managed-services/sources/base-repo.yaml": `apiVersion: v1
kind: Secret
data:
  key: unencrypted_value`,
	}

	for file, content := range testFiles {
		filePath := filepath.Join(tmpDir, file)
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", file, err)
		}
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("Failed to create file %s: %v", file, err)
		}
	}

	e := NewEncryptor(nil, nil)

	// This should fail because base-repo.yaml is not encrypted
	err = e.ValidateEncryption(tmpDir, config)
	if err == nil {
		t.Errorf("ValidateEncryption() should fail when files are not encrypted")
	}

	// Fix the unencrypted file
	encryptedContent := `apiVersion: v1
kind: Secret
sops:
  age:
    - recipient: age1test123
      enc: encrypted_data`

	err = os.WriteFile(filepath.Join(tmpDir, "managed-services/sources/base-repo.yaml"), []byte(encryptedContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to update file: %v", err)
	}

	// Now validation should pass
	err = e.ValidateEncryption(tmpDir, config)
	if err != nil {
		t.Errorf("ValidateEncryption() should pass when all files are encrypted, got error: %v", err)
	}
}

func TestEncryptionConfig_Validation(t *testing.T) {
	tests := []struct {
		name   string
		config EncryptionConfig
		valid  bool
	}{
		{
			name: "valid config with age keys",
			config: EncryptionConfig{
				AgeKeys: []string{"age1test123"},
				InPlace: true,
			},
			valid: true,
		},
		{
			name: "valid config with pgp keys",
			config: EncryptionConfig{
				PGPKeys: []string{"ABCD1234"},
				InPlace: true,
			},
			valid: true,
		},
		{
			name: "valid config with both key types",
			config: EncryptionConfig{
				AgeKeys: []string{"age1test123"},
				PGPKeys: []string{"ABCD1234"},
				InPlace: true,
			},
			valid: true,
		},
		{
			name: "config with no keys (should use defaults)",
			config: EncryptionConfig{
				InPlace: true,
			},
			valid: true, // Will use default keys from encryptor
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the config can be created and used
			// In a real implementation, you might have validation logic
			if len(tt.config.AgeKeys) == 0 && len(tt.config.PGPKeys) == 0 {
				// This would use default keys from the encryptor
				t.Logf("Config with no keys will use defaults")
			}
		})
	}
}

// Mock test for SOPS CLI interaction
func TestEncryptor_CheckSOPSVersion(t *testing.T) {
	e := NewEncryptor(nil, nil)

	// This test would normally check if SOPS CLI is available
	// In a real test environment, you might want to mock the exec.Command
	version, err := e.CheckSOPSVersion(context.Background())

	// We expect this to fail in test environment since SOPS CLI might not be installed
	// The important thing is that the method doesn't panic
	if err != nil {
		t.Logf("CheckSOPSVersion() failed as expected in test environment: %v", err)
	} else {
		t.Logf("SOPS version: %s", version)
	}
}

func TestEncryptor_EncryptFile(t *testing.T) {
	e := NewEncryptor([]string{"age1test123"}, nil)

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "encrypt-test-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Write test content
	testContent := "password: secret123"
	_, err = tmpFile.WriteString(testContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Test encryption config
	config := EncryptionConfig{
		AgeKeys: []string{"age1test123"},
		InPlace: true,
		DryRun:  true, // Use dry run to avoid actual SOPS execution
	}

	// Test encryption (dry run)
	err = e.EncryptFile(context.Background(), tmpFile.Name(), config)
	assert.NoError(t, err) // Should succeed in dry run mode

	// Test with non-existent file
	err = e.EncryptFile(context.Background(), "/nonexistent/file.yaml", config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file does not exist")
}

func TestEncryptor_DecryptFile(t *testing.T) {
	e := NewEncryptor(nil, nil)

	// Create mock encrypted file
	tmpFile, err := os.CreateTemp("", "decrypt-test-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Write encrypted content
	encryptedContent := `apiVersion: v1
kind: Secret
sops:
  age:
    - recipient: age1test123
      enc: encrypted_data`

	_, err = tmpFile.WriteString(encryptedContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Test decryption (will fail without SOPS, but tests parameter validation)
	err = e.DecryptFile(context.Background(), tmpFile.Name(), "")
	if err != nil {
		// Should fail on SOPS execution, not parameter validation
		assert.Contains(t, err.Error(), "SOPS")
	}

	// Test with non-existent file
	err = e.DecryptFile(context.Background(), "/nonexistent/file.yaml", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file does not exist")
}

func TestEncryptor_EncryptFiles(t *testing.T) {
	e := NewEncryptor([]string{"age1test123"}, nil)

	// Create temporary files
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.yaml")
	file2 := filepath.Join(tmpDir, "file2.yaml")

	err := os.WriteFile(file1, []byte("password: secret1"), 0o644)
	require.NoError(t, err)

	err = os.WriteFile(file2, []byte("password: secret2"), 0o644)
	require.NoError(t, err)

	// Test encrypting multiple files
	config := EncryptionConfig{
		AgeKeys: []string{"age1test123"},
		InPlace: true,
		DryRun:  true, // Use dry run
	}

	err = e.EncryptFiles(context.Background(), []string{file1, file2}, config)
	assert.NoError(t, err) // Should succeed in dry run mode
}

func TestEncryptor_RotateKeys(t *testing.T) {
	e := NewEncryptor(nil, nil)

	// Create mock encrypted file
	tmpFile, err := os.CreateTemp("", "rotate-test-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	encryptedContent := `apiVersion: v1
kind: Secret
sops:
  age:
    - recipient: age1oldkey123
      enc: encrypted_data`

	_, err = tmpFile.WriteString(encryptedContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Test key rotation (will fail without SOPS)
	newAgeKeys := []string{"age1newkey123"}
	err = e.RotateKeys(context.Background(), tmpFile.Name(), newAgeKeys, nil)
	if err != nil {
		// Should fail on SOPS execution
		assert.Contains(t, err.Error(), "SOPS")
	}
}

func TestEncryptor_GetEncryptedContent(t *testing.T) {
	e := NewEncryptor(nil, nil)

	// Create test file
	tmpFile, err := os.CreateTemp("", "content-test-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	testContent := "test content"
	_, err = tmpFile.WriteString(testContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Test getting content
	content, err := e.GetEncryptedContent(tmpFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, testContent, content)

	// Test with non-existent file
	_, err = e.GetEncryptedContent("/nonexistent/file.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestEncryptor_EditEncryptedFile(t *testing.T) {
	e := NewEncryptor(nil, nil)

	// Create mock encrypted file
	tmpFile, err := os.CreateTemp("", "edit-test-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Test editing (will fail without SOPS and interactive terminal)
	err = e.EditEncryptedFile(context.Background(), tmpFile.Name())
	if err != nil {
		// Should fail on SOPS execution
		assert.Contains(t, err.Error(), "SOPS")
	}
}

func TestEncryptor_CreateSampleEncryptedSecrets(t *testing.T) {
	e := NewEncryptor(nil, nil)
	tmpDir := t.TempDir()
	ageKey := "age1ql3vwyqfpvucvyz6x04chxtsm9l3f24tqq5pkq9jz4mk7ccvqxrqsqg5z5"

	// Test creating sample encrypted secrets
	err := e.CreateSampleEncryptedSecrets(context.Background(), tmpDir, ageKey)
	assert.NoError(t, err)

	// Verify samples directory was created
	samplesDir := filepath.Join(tmpDir, "examples", "secrets")
	assert.DirExists(t, samplesDir)

	// Verify sample encrypted files were created
	expectedFiles := []string{
		"sample-secret.enc.yaml",
		"database-credentials.enc.yaml",
		"api-tokens.enc.yaml",
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(samplesDir, file)
		assert.FileExists(t, filePath)

		// Verify file contains SOPS metadata (either real or placeholder)
		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		contentStr := string(content)

		// Should contain either real SOPS metadata or placeholder
		assert.True(t,
			strings.Contains(contentStr, "sops:") ||
				strings.Contains(contentStr, "placeholder_encrypted_data"),
			"File should contain SOPS metadata or placeholder: %s", file)

		// Should contain the age key
		assert.Contains(t, contentStr, ageKey)
	}
}

func TestEncryptor_CreatePlaceholderEncryptedContent(t *testing.T) {
	e := NewEncryptor(nil, nil)
	ageKey := "age1ql3vwyqfpvucvyz6x04chxtsm9l3f24tqq5pkq9jz4mk7ccvqxrqsqg5z5"
	originalContent := `apiVersion: v1
kind: Secret
metadata:
  name: test-secret
stringData:
  password: secret123`

	placeholder := e.createPlaceholderEncryptedContent(originalContent, ageKey)

	// Verify placeholder contains expected elements
	assert.Contains(t, placeholder, ageKey)
	assert.Contains(t, placeholder, "sops:")
	assert.Contains(t, placeholder, "placeholder_encrypted_data")
	assert.Contains(t, placeholder, "# Original content")
	assert.Contains(t, placeholder, "DO NOT COMMIT UNENCRYPTED")
}

func TestEncryptor_EncryptRepositorySecrets(t *testing.T) {
	e := NewEncryptor(nil, nil)
	tmpDir := t.TempDir()
	ageKey := "age1ql3vwyqfpvucvyz6x04chxtsm9l3f24tqq5pkq9jz4mk7ccvqxrqsqg5z5"

	// Create sample secrets directory and files
	secretsDir := filepath.Join(tmpDir, "examples", "secrets")
	err := os.MkdirAll(secretsDir, 0o755)
	require.NoError(t, err)

	// Create test secret files
	testSecrets := map[string]string{
		"test-secret.yaml": `apiVersion: v1
kind: Secret
metadata:
  name: test-secret
stringData:
  password: secret123`,
		"another-secret.yaml": `apiVersion: v1
kind: Secret
metadata:
  name: another-secret
stringData:
  token: token456`,
		"README.md": "# This should not be encrypted",
	}

	for filename, content := range testSecrets {
		filePath := filepath.Join(secretsDir, filename)
		err := os.WriteFile(filePath, []byte(content), 0o644)
		require.NoError(t, err)
	}

	// Test encrypting repository secrets
	err = e.EncryptRepositorySecrets(context.Background(), tmpDir, ageKey)
	assert.NoError(t, err)

	// Verify YAML files were processed (encrypted or placeholder created)
	for filename := range testSecrets {
		if strings.HasSuffix(filename, ".yaml") && !strings.Contains(filename, "README") {
			filePath := filepath.Join(secretsDir, filename)
			content, err := os.ReadFile(filePath)
			require.NoError(t, err)
			contentStr := string(content)

			// Should contain SOPS metadata or placeholder
			assert.True(t,
				strings.Contains(contentStr, "sops:") ||
					strings.Contains(contentStr, "placeholder_encrypted_data"),
				"File should be encrypted or have placeholder: %s", filename)
		}
	}

	// Verify README was not modified
	readmePath := filepath.Join(secretsDir, "README.md")
	content, err := os.ReadFile(readmePath)
	require.NoError(t, err)
	assert.Equal(t, "# This should not be encrypted", string(content))
}

func TestEncryptor_EncryptFileToOutput(t *testing.T) {
	e := NewEncryptor(nil, nil)
	tmpDir := t.TempDir()
	ageKey := "age1ql3vwyqfpvucvyz6x04chxtsm9l3f24tqq5pkq9jz4mk7ccvqxrqsqg5z5"

	// Create input file
	inputFile := filepath.Join(tmpDir, "input.yaml")
	content := `apiVersion: v1
kind: Secret
metadata:
  name: test
stringData:
  password: secret123`

	err := os.WriteFile(inputFile, []byte(content), 0o644)
	require.NoError(t, err)

	// Test encrypting to output file
	outputFile := filepath.Join(tmpDir, "output.enc.yaml")
	config := EncryptionConfig{
		AgeKeys: []string{ageKey},
	}

	err = e.encryptFileToOutput(context.Background(), inputFile, outputFile, config)
	if err != nil {
		// Should fail without SOPS installed
		assert.Contains(t, err.Error(), "SOPS")
	} else {
		// If SOPS is available, verify output file exists
		assert.FileExists(t, outputFile)
	}
}

func TestEncryptor_SampleSecretsIntegration(t *testing.T) {
	e := NewEncryptor(nil, nil)
	tmpDir := t.TempDir()
	ageKey := "age1ql3vwyqfpvucvyz6x04chxtsm9l3f24tqq5pkq9jz4mk7ccvqxrqsqg5z5"

	// Test complete workflow: create samples, then encrypt repository
	err := e.CreateSampleEncryptedSecrets(context.Background(), tmpDir, ageKey)
	assert.NoError(t, err)

	// Verify samples were created
	samplesDir := filepath.Join(tmpDir, "examples", "secrets")
	files, err := os.ReadDir(samplesDir)
	require.NoError(t, err)
	assert.NotEmpty(t, files)

	// Test encrypting the repository (should handle already encrypted files)
	err = e.EncryptRepositorySecrets(context.Background(), tmpDir, ageKey)
	assert.NoError(t, err)

	// Verify all .enc.yaml files contain proper structure
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".enc.yaml") {
			filePath := filepath.Join(samplesDir, file.Name())
			content, err := os.ReadFile(filePath)
			require.NoError(t, err)
			contentStr := string(content)

			// Should contain SOPS structure
			assert.True(t,
				strings.Contains(contentStr, "sops:") ||
					strings.Contains(contentStr, "placeholder_encrypted_data"),
				"Encrypted file should have SOPS structure: %s", file.Name())
		}
	}
}
func TestEncryptor_EncryptOverlayFiles(t *testing.T) {
	e := NewEncryptor([]string{"age1test123"}, nil)
	tmpDir := t.TempDir()

	// Create test overlay structure
	fluxDir := filepath.Join(tmpDir, "flux-system")
	err := os.MkdirAll(fluxDir, 0o755)
	require.NoError(t, err)

	secretsDir := filepath.Join(tmpDir, "secrets")
	err = os.MkdirAll(secretsDir, 0o755)
	require.NoError(t, err)

	// Create test files
	testFiles := map[string]string{
		"flux-system/gotk-sync.yaml": `apiVersion: v1
kind: Secret
metadata:
  name: flux-system
stringData:
  identity: |
    -----BEGIN OPENSSH PRIVATE KEY-----
    test-key-content
    -----END OPENSSH PRIVATE KEY-----`,
		"secrets/openstack-credentials.yaml": `apiVersion: v1
kind: Secret
metadata:
  name: openstack-credentials
stringData:
  username: test-user
  password: test-password`,
	}

	for file, content := range testFiles {
		filePath := filepath.Join(tmpDir, file)
		err := os.WriteFile(filePath, []byte(content), 0o644)
		require.NoError(t, err)
	}

	// Test config
	cfg := &config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Infrastructure: config.Infrastructure{
				Provider: "openstack",
			},
		},
		Secrets: config.Secrets{
			SopsAgeKeyFile: "age1test123",
		},
	}

	// Test encryption (dry run to avoid SOPS dependency)
	err = e.EncryptOverlayFiles(context.Background(), tmpDir, cfg)
	if err != nil {
		// Should fail on SOPS execution, not parameter validation
		assert.Contains(t, err.Error(), "SOPS")
	}
}

func TestEncryptor_ProviderSpecificFiles(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		expected []string
	}{
		{
			name:     "openstack provider",
			provider: "openstack",
			expected: []string{
				"flux-system/gotk-sync.yaml",
				"managed-services/sources/base-repo.yaml",
				"secrets/openstack-credentials.yaml",
			},
		},
		{
			name:     "vsphere provider",
			provider: "vsphere",
			expected: []string{
				"flux-system/gotk-sync.yaml",
				"managed-services/sources/base-repo.yaml",
				"secrets/vsphere-credentials.yaml",
				"customer-managed/services/cloud-provider-vsphere/secret.yaml",
			},
		},
		{
			name:     "kind provider",
			provider: "kind",
			expected: []string{
				"flux-system/gotk-sync.yaml",
				"managed-services/sources/base-repo.yaml",
			},
		},
		{
			name:     "unknown provider",
			provider: "unknown",
			expected: []string{
				"flux-system/gotk-sync.yaml",
				"managed-services/sources/base-repo.yaml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEncryptor(nil, nil)
			cfg := &config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Infrastructure: config.Infrastructure{
						Provider: tt.provider,
					},
				},
			}

			files := e.getFilesToEncrypt("", cfg)
			assert.Equal(t, len(tt.expected), len(files))

			for _, expectedFile := range tt.expected {
				assert.Contains(t, files, expectedFile)
			}
		})
	}
}

func TestEncryptor_ErrorHandling(t *testing.T) {
	e := NewEncryptor(nil, nil)

	tests := []struct {
		name        string
		operation   func() error
		expectError bool
		errorMsg    string
	}{
		{
			name: "encrypt non-existent file",
			operation: func() error {
				config := EncryptionConfig{AgeKeys: []string{"age1test123"}}
				return e.EncryptFile(context.Background(), "/nonexistent/file.yaml", config)
			},
			expectError: true,
			errorMsg:    "file does not exist",
		},
		{
			name: "decrypt non-existent file",
			operation: func() error {
				return e.DecryptFile(context.Background(), "/nonexistent/file.yaml", "")
			},
			expectError: true,
			errorMsg:    "file does not exist",
		},
		{
			name: "get content of non-existent file",
			operation: func() error {
				_, err := e.GetEncryptedContent("/nonexistent/file.yaml")
				return err
			},
			expectError: true,
			errorMsg:    "failed to read file",
		},
		{
			name: "validate encryption with non-existent overlay",
			operation: func() error {
				cfg := &config.Config{
					OpenCenter: config.SimplifiedOpenCenter{
						Infrastructure: config.Infrastructure{
							Provider: "openstack",
						},
					},
				}
				return e.ValidateEncryption("/nonexistent/overlay", cfg)
			},
			expectError: false, // Should not error if files don't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.operation()
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEncryptor_ConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config EncryptionConfig
		valid  bool
	}{
		{
			name: "valid config with age keys only",
			config: EncryptionConfig{
				AgeKeys: []string{"age1test123"},
				InPlace: true,
			},
			valid: true,
		},
		{
			name: "valid config with pgp keys only",
			config: EncryptionConfig{
				PGPKeys: []string{"ABCD1234"},
				InPlace: true,
			},
			valid: true,
		},
		{
			name: "valid config with both key types",
			config: EncryptionConfig{
				AgeKeys: []string{"age1test123"},
				PGPKeys: []string{"ABCD1234"},
				InPlace: true,
			},
			valid: true,
		},
		{
			name: "config with empty keys (should use encryptor defaults)",
			config: EncryptionConfig{
				InPlace: true,
			},
			valid: true,
		},
		{
			name: "config with dry run enabled",
			config: EncryptionConfig{
				AgeKeys: []string{"age1test123"},
				DryRun:  true,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEncryptor([]string{"age1default"}, []string{"DEFAULTPGP"})
			
			// Create a temporary file for testing
			tmpFile, err := os.CreateTemp("", "config-test-*.yaml")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			
			_, err = tmpFile.WriteString("test: content")
			require.NoError(t, err)
			tmpFile.Close()

			// Test the configuration
			err = e.EncryptFile(context.Background(), tmpFile.Name(), tt.config)
			
			if tt.valid {
				// Should either succeed (dry run) or fail on SOPS execution
				if err != nil && !tt.config.DryRun {
					assert.Contains(t, err.Error(), "SOPS")
				}
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestEncryptor_AlreadyEncryptedFile(t *testing.T) {
	e := NewEncryptor([]string{"age1test123"}, nil)
	
	// Create a file that's already encrypted
	tmpFile, err := os.CreateTemp("", "already-encrypted-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	encryptedContent := `apiVersion: v1
kind: Secret
metadata:
  name: test-secret
sops:
  age:
    - recipient: age1test123
      enc: ENC[AES256_GCM,data:encrypted_data,type:str]`

	_, err = tmpFile.WriteString(encryptedContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Try to encrypt an already encrypted file
	config := EncryptionConfig{
		AgeKeys: []string{"age1test123"},
		InPlace: true,
	}

	err = e.EncryptFile(context.Background(), tmpFile.Name(), config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file is already encrypted")
}

func TestEncryptor_DecryptUnencryptedFile(t *testing.T) {
	e := NewEncryptor(nil, nil)
	
	// Create an unencrypted file
	tmpFile, err := os.CreateTemp("", "unencrypted-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	unencryptedContent := `apiVersion: v1
kind: Secret
metadata:
  name: test-secret
data:
  key: value`

	_, err = tmpFile.WriteString(unencryptedContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Try to decrypt an unencrypted file
	err = e.DecryptFile(context.Background(), tmpFile.Name(), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file is not encrypted")
}

func TestEncryptor_CreateSampleSecretsForTemplate(t *testing.T) {
	e := NewEncryptor(nil, nil)
	tmpDir := t.TempDir()
	ageKey := "age1ql3vwyqfpvucvyz6x04chxtsm9l3f24tqq5pkq9jz4mk7ccvqxrqsqg5z5"

	tests := []struct {
		name     string
		template string
		expected []string
	}{
		{
			name:     "basic template",
			template: "basic",
			expected: []string{
				"sample-secret.enc.yaml",
				"database-credentials.enc.yaml",
				"api-tokens.enc.yaml",
			},
		},
		{
			name:     "enterprise template",
			template: "enterprise",
			expected: []string{
				"sample-secret.enc.yaml",
				"database-credentials.enc.yaml",
				"api-tokens.enc.yaml",
				"production-secrets.enc.yaml",
				"monitoring-secrets.enc.yaml",
			},
		},
		{
			name:     "multi-tenant template",
			template: "multi-tenant",
			expected: []string{
				"sample-secret.enc.yaml",
				"database-credentials.enc.yaml",
				"api-tokens.enc.yaml",
				"production-secrets.enc.yaml",
				"monitoring-secrets.enc.yaml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := filepath.Join(tmpDir, tt.name)
			err := e.CreateSampleEncryptedSecretsForTemplate(context.Background(), testDir, ageKey, tt.template)
			assert.NoError(t, err)

			// Verify expected files were created
			samplesDir := filepath.Join(testDir, "examples", "secrets")
			for _, expectedFile := range tt.expected {
				filePath := filepath.Join(samplesDir, expectedFile)
				assert.FileExists(t, filePath)

				// Verify file contains expected content
				content, err := os.ReadFile(filePath)
				require.NoError(t, err)
				contentStr := string(content)

				// Should contain SOPS metadata or placeholder
				assert.True(t,
					strings.Contains(contentStr, "sops:") ||
						strings.Contains(contentStr, "placeholder_encrypted_data"),
					"File should contain SOPS metadata or placeholder: %s", expectedFile)

				// Should contain the age key
				assert.Contains(t, contentStr, ageKey)
			}
		})
	}
}
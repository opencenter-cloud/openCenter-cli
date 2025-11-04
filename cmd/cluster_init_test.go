package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config"
)

func TestGenerateDefaultSOPSKey(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	tests := []struct {
		name        string
		clusterName string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid cluster name",
			clusterName: "test-cluster",
			expectError: false,
		},
		{
			name:        "cluster name with underscores",
			clusterName: "test_cluster_01",
			expectError: false,
		},
		{
			name:        "cluster name with dots",
			clusterName: "test.cluster.dev",
			expectError: false,
		},
		{
			name:        "invalid cluster name with slash",
			clusterName: "test/cluster",
			expectError: true,
			errorMsg:    "failed to get cluster secrets path",
		},
		{
			name:        "empty cluster name",
			clusterName: "",
			expectError: true,
			errorMsg:    "failed to get cluster secrets path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test config
			cfg := config.NewDefault(tt.clusterName)
			cfg.OpenCenter.GitOps.GitDir = "test-dir"

			// Call generateDefaultSOPSKey
			err := generateDefaultSOPSKey(tt.clusterName, &cfg)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for cluster name %q, but got none", tt.clusterName)
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("expected no error for cluster name %q, but got: %v", tt.clusterName, err)
				return
			}

			// Verify the SOPS key file was created
			expectedSecretsDir := filepath.Join(dir, "clusters", tt.clusterName, "secrets", "age", "keys")
			expectedKeyFile := filepath.Join(expectedSecretsDir, tt.clusterName+"-key.txt")

			// Check that secrets directory was created
			if _, err := os.Stat(expectedSecretsDir); os.IsNotExist(err) {
				t.Errorf("secrets directory was not created: %s", expectedSecretsDir)
				return
			}

			// Check directory permissions
			info, err := os.Stat(expectedSecretsDir)
			if err != nil {
				t.Errorf("failed to stat secrets directory: %v", err)
				return
			}
			if info.Mode().Perm() != 0o755 {
				t.Errorf("expected secrets directory permissions 0755, got %o", info.Mode().Perm())
			}

			// Check that key file was created
			if _, err := os.Stat(expectedKeyFile); os.IsNotExist(err) {
				t.Errorf("SOPS key file was not created: %s", expectedKeyFile)
				return
			}

			// Check file permissions
			info, err = os.Stat(expectedKeyFile)
			if err != nil {
				t.Errorf("failed to stat key file: %v", err)
				return
			}
			if info.Mode().Perm() != 0o600 {
				t.Errorf("expected key file permissions 0600, got %o", info.Mode().Perm())
			}

			// Verify key file content
			keyContent, err := os.ReadFile(expectedKeyFile)
			if err != nil {
				t.Errorf("failed to read key file: %v", err)
				return
			}

			keyStr := string(keyContent)
			if !strings.HasPrefix(keyStr, "AGE-SECRET-KEY-1") {
				t.Errorf("expected key to start with 'AGE-SECRET-KEY-1', got: %s", keyStr[:20])
			}

			if !strings.HasSuffix(keyStr, "\n") {
				t.Error("expected key to end with newline")
			}

			// Verify key length (AGE-SECRET-KEY-1 + 64 hex chars + newline = 81 chars)
			if len(keyStr) != 81 {
				t.Errorf("expected key length 81, got %d", len(keyStr))
			}

			// Verify config was updated
			if cfg.Secrets.SopsAgeKeyFile != expectedKeyFile {
				t.Errorf("expected config SopsAgeKeyFile to be %s, got %s", expectedKeyFile, cfg.Secrets.SopsAgeKeyFile)
			}
		})
	}
}

func TestGenerateDefaultSOPSKeyDirectoryCreation(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	clusterName := "test-cluster"
	cfg := config.NewDefault(clusterName)
	cfg.OpenCenter.GitOps.GitDir = "test-dir"

	// Verify that the secrets directory doesn't exist initially
	secretsDir := filepath.Join(dir, "clusters", clusterName, "secrets", "age", "keys")
	if _, err := os.Stat(secretsDir); !os.IsNotExist(err) {
		t.Errorf("secrets directory should not exist initially")
	}

	// Call generateDefaultSOPSKey
	err := generateDefaultSOPSKey(clusterName, &cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify that the entire directory structure was created
	if _, err := os.Stat(secretsDir); os.IsNotExist(err) {
		t.Errorf("secrets directory was not created: %s", secretsDir)
	}

	// Verify intermediate directories exist
	clusterDir := filepath.Join(dir, "clusters", clusterName)
	if _, err := os.Stat(clusterDir); os.IsNotExist(err) {
		t.Errorf("cluster directory was not created: %s", clusterDir)
	}

	secretsBaseDir := filepath.Join(clusterDir, "secrets")
	if _, err := os.Stat(secretsBaseDir); os.IsNotExist(err) {
		t.Errorf("secrets base directory was not created: %s", secretsBaseDir)
	}

	ageDir := filepath.Join(secretsBaseDir, "age")
	if _, err := os.Stat(ageDir); os.IsNotExist(err) {
		t.Errorf("age directory was not created: %s", ageDir)
	}

	keysDir := filepath.Join(ageDir, "keys")
	if _, err := os.Stat(keysDir); os.IsNotExist(err) {
		t.Errorf("keys directory was not created: %s", keysDir)
	}
}

func TestGenerateDefaultSOPSKeyMultipleCalls(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	clusterName := "test-cluster"
	cfg := config.NewDefault(clusterName)
	cfg.OpenCenter.GitOps.GitDir = "test-dir"

	// Call generateDefaultSOPSKey first time
	err := generateDefaultSOPSKey(clusterName, &cfg)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}

	// Read the first key
	keyFile := filepath.Join(dir, "clusters", clusterName, "secrets", "age", "keys", clusterName+"-key.txt")
	firstKey, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("failed to read first key: %v", err)
	}

	// Call generateDefaultSOPSKey second time (should overwrite)
	cfg2 := config.NewDefault(clusterName)
	cfg2.OpenCenter.GitOps.GitDir = "test-dir"
	err = generateDefaultSOPSKey(clusterName, &cfg2)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}

	// Read the second key
	secondKey, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("failed to read second key: %v", err)
	}

	// Keys should be different (random generation)
	if string(firstKey) == string(secondKey) {
		t.Error("expected different keys on multiple calls, but got the same key")
	}

	// Both keys should be valid
	firstKeyStr := string(firstKey)
	secondKeyStr := string(secondKey)

	if !strings.HasPrefix(firstKeyStr, "AGE-SECRET-KEY-1") {
		t.Error("first key should start with 'AGE-SECRET-KEY-1'")
	}

	if !strings.HasPrefix(secondKeyStr, "AGE-SECRET-KEY-1") {
		t.Error("second key should start with 'AGE-SECRET-KEY-1'")
	}
}

func TestGenerateDefaultSOPSKeyExistingDirectory(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	clusterName := "test-cluster"
	cfg := config.NewDefault(clusterName)
	cfg.OpenCenter.GitOps.GitDir = "test-dir"

	// Pre-create the secrets directory
	secretsDir := filepath.Join(dir, "clusters", clusterName, "secrets", "age", "keys")
	err := os.MkdirAll(secretsDir, 0o755)
	if err != nil {
		t.Fatalf("failed to create secrets directory: %v", err)
	}

	// Call generateDefaultSOPSKey (should work with existing directory)
	err = generateDefaultSOPSKey(clusterName, &cfg)
	if err != nil {
		t.Fatalf("unexpected error with existing directory: %v", err)
	}

	// Verify key file was created
	keyFile := filepath.Join(secretsDir, clusterName+"-key.txt")
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		t.Errorf("SOPS key file was not created: %s", keyFile)
	}
}
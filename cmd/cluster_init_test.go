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
			errorMsg:    "invalid cluster name",
		},
		{
			name:        "empty cluster name",
			clusterName: "",
			expectError: true,
			errorMsg:    "invalid cluster name",
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

			// Verify key length (AGE-SECRET-KEY-1 + 58 base64 chars + newline = 75 chars)
			if len(keyStr) != 75 {
				t.Errorf("expected key length 75, got %d", len(keyStr))
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

func TestGenerateOrganizationSOPSKey(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	tests := []struct {
		name         string
		clusterName  string
		organization string
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "valid cluster and organization",
			clusterName:  "test-cluster",
			organization: "dev-team",
			expectError:  false,
		},
		{
			name:         "cluster with opencenter organization",
			clusterName:  "prod-cluster",
			organization: "opencenter",
			expectError:  false,
		},
		{
			name:         "cluster with empty organization (should use default)",
			clusterName:  "staging-cluster",
			organization: "",
			expectError:  false,
		},
		{
			name:         "cluster name with special characters",
			clusterName:  "test-cluster-01.dev",
			organization: "qa-team",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize configuration manager and path resolver
			configManager, err := config.NewConfigManager("")
			if err != nil {
				t.Fatalf("failed to create config manager: %v", err)
			}

			pathResolver := config.NewPathResolver(configManager)

			// Create a test config
			cfg := config.NewDefault(tt.clusterName)
			cfg.OpenCenter.GitOps.GitDir = "test-dir"

			// Set expected organization (opencenter if empty)
			expectedOrg := tt.organization
			if expectedOrg == "" {
				expectedOrg = "opencenter"
			}

			// Call generateOrganizationSOPSKey
			err = generateOrganizationSOPSKey(tt.clusterName, tt.organization, &cfg, pathResolver)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for cluster %q and organization %q, but got none", tt.clusterName, tt.organization)
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("expected no error for cluster %q and organization %q, but got: %v", tt.clusterName, tt.organization, err)
				return
			}

			// Resolve expected paths
			clusterPaths := pathResolver.ResolveClusterPaths(tt.clusterName, expectedOrg)

			// Verify the organization secrets directory was created
			expectedSecretsDir := filepath.Dir(clusterPaths.SOPSKeyPath)
			if _, err := os.Stat(expectedSecretsDir); os.IsNotExist(err) {
				t.Errorf("organization secrets directory was not created: %s", expectedSecretsDir)
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
			if _, err := os.Stat(clusterPaths.SOPSKeyPath); os.IsNotExist(err) {
				t.Errorf("SOPS key file was not created: %s", clusterPaths.SOPSKeyPath)
				return
			}

			// Check file permissions
			info, err = os.Stat(clusterPaths.SOPSKeyPath)
			if err != nil {
				t.Errorf("failed to stat key file: %v", err)
				return
			}
			if info.Mode().Perm() != 0o600 {
				t.Errorf("expected key file permissions 0600, got %o", info.Mode().Perm())
			}

			// Verify key file content
			keyContent, err := os.ReadFile(clusterPaths.SOPSKeyPath)
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

			// Verify key length (AGE-SECRET-KEY-1 + 58 base64 chars + newline = 75 chars)
			if len(keyStr) != 75 {
				t.Errorf("expected key length 75, got %d", len(keyStr))
			}

			// Verify config was updated with organization-based path
			if cfg.Secrets.SopsAgeKeyFile != clusterPaths.SOPSKeyPath {
				t.Errorf("expected config SopsAgeKeyFile to be %s, got %s", clusterPaths.SOPSKeyPath, cfg.Secrets.SopsAgeKeyFile)
			}

			// Verify SOPS config file was created
			if _, err := os.Stat(clusterPaths.SOPSConfigPath); os.IsNotExist(err) {
				t.Errorf("SOPS config file was not created: %s", clusterPaths.SOPSConfigPath)
				return
			}

			// Check SOPS config file permissions
			info, err = os.Stat(clusterPaths.SOPSConfigPath)
			if err != nil {
				t.Errorf("failed to stat SOPS config file: %v", err)
				return
			}
			if info.Mode().Perm() != 0o600 {
				t.Errorf("expected SOPS config file permissions 0600, got %o", info.Mode().Perm())
			}

			// Verify SOPS config file content
			sopsContent, err := os.ReadFile(clusterPaths.SOPSConfigPath)
			if err != nil {
				t.Errorf("failed to read SOPS config file: %v", err)
				return
			}

			sopsStr := string(sopsContent)
			if !strings.Contains(sopsStr, "creation_rules:") {
				t.Error("expected SOPS config to contain 'creation_rules:'")
			}
			if !strings.Contains(sopsStr, "path_regex:") {
				t.Error("expected SOPS config to contain 'path_regex:'")
			}
			if !strings.Contains(sopsStr, "age:") {
				t.Error("expected SOPS config to contain 'age:'")
			}
		})
	}
}

func TestOrganizationBasedClusterInit(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	tests := []struct {
		name         string
		clusterName  string
		organization string
		expectError  bool
	}{
		{
			name:         "init cluster in dev organization",
			clusterName:  "web-app",
			organization: "dev-team",
			expectError:  false,
		},
		{
			name:         "init cluster in opencenter organization",
			clusterName:  "legacy-app",
			organization: "opencenter",
			expectError:  false,
		},
		{
			name:         "init cluster with empty organization",
			clusterName:  "test-app",
			organization: "",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize configuration manager and path resolver
			configManager, err := config.NewConfigManager("")
			if err != nil {
				t.Fatalf("failed to create config manager: %v", err)
			}

			pathResolver := config.NewPathResolver(configManager)

			// Set expected organization (opencenter if empty)
			expectedOrg := tt.organization
			if expectedOrg == "" {
				expectedOrg = "opencenter"
			}

			// Create organization structure
			err = pathResolver.CreateOrganizationStructure(expectedOrg)
			if err != nil {
				t.Fatalf("failed to create organization structure: %v", err)
			}

			// Create cluster directories
			err = pathResolver.CreateClusterDirectories(tt.clusterName, expectedOrg)
			if err != nil {
				t.Fatalf("failed to create cluster directories: %v", err)
			}

			// Resolve cluster paths
			clusterPaths := pathResolver.ResolveClusterPaths(tt.clusterName, expectedOrg)

			// Verify organization directory structure was created
			expectedDirs := []string{
				clusterPaths.OrganizationDir,
				clusterPaths.ClusterDir,
				clusterPaths.ApplicationsDir,
				clusterPaths.InventoryPath,
				clusterPaths.VenvPath,
				clusterPaths.BinPath,
				filepath.Dir(clusterPaths.SOPSKeyPath), // age/keys directory
			}

			for _, expectedDir := range expectedDirs {
				if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
					t.Errorf("expected directory was not created: %s", expectedDir)
				}
			}

			// Verify GitOps structure
			gitopsStructureDirs := []string{
				filepath.Join(clusterPaths.GitOpsDir, "applications", "overlays"),
				filepath.Join(clusterPaths.GitOpsDir, "infrastructure", "clusters"),
				filepath.Join(clusterPaths.SecretsDir, "age", "keys"),
			}

			for _, gitopsDir := range gitopsStructureDirs {
				if _, err := os.Stat(gitopsDir); os.IsNotExist(err) {
					t.Errorf("expected GitOps directory was not created: %s", gitopsDir)
				}
			}

			// Test that paths are correctly resolved
			if !strings.Contains(clusterPaths.ClusterDir, expectedOrg) {
				t.Errorf("cluster directory should contain organization name %q, got: %s", expectedOrg, clusterPaths.ClusterDir)
			}

			if !strings.Contains(clusterPaths.ApplicationsDir, expectedOrg) {
				t.Errorf("applications directory should contain organization name %q, got: %s", expectedOrg, clusterPaths.ApplicationsDir)
			}

			if !strings.Contains(clusterPaths.SecretsDir, expectedOrg) {
				t.Errorf("secrets directory should contain organization name %q, got: %s", expectedOrg, clusterPaths.SecretsDir)
			}

			// Verify that GitOps directory points to organization root
			if clusterPaths.GitOpsDir != clusterPaths.OrganizationDir {
				t.Errorf("GitOps directory should point to organization root, got: %s, expected: %s", clusterPaths.GitOpsDir, clusterPaths.OrganizationDir)
			}
		})
	}
}

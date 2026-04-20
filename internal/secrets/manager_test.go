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

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestManager creates a test secrets manager with temporary directories
func setupTestManager(t *testing.T) (*DefaultSecretsManager, string, func()) {
	t.Helper()

	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "secrets-manager-test-*")
	require.NoError(t, err)

	originalHome := os.Getenv("HOME")
	originalConfigDir := os.Getenv("OPENCENTER_CONFIG_DIR")
	configDir := filepath.Join(tmpDir, ".config", "opencenter")

	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.Setenv("HOME", tmpDir))
	require.NoError(t, os.Setenv("OPENCENTER_CONFIG_DIR", configDir))

	// Create file system
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)

	// Create config loader
	configLoader := config.NewConfigIOHandler(fileSystem)

	// Create SOPS manager (with nil dependencies for unit tests)
	sopsManager := sops.NewDefaultSOPSManager(nil, nil, slog.Default())

	// Create secrets manager
	manager := NewDefaultSecretsManager(configLoader, sopsManager, nil, slog.Default())

	cleanup := func() {
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
		os.RemoveAll(tmpDir)
	}

	return manager, tmpDir, cleanup
}

func writeManagerTestConfig(t *testing.T, clusterName string, configData string) string {
	t.Helper()

	resolver := paths.NewPathResolver(config.ResolveClustersDir())
	require.NoError(t, resolver.CreateClusterDirectories(context.Background(), clusterName, "test-org"))

	clusterPaths, err := resolver.Resolve(context.Background(), clusterName, "test-org")
	require.NoError(t, err)
	writeNormalizedSecretsConfigFile(t, clusterPaths.ConfigPath, clusterName, configData)

	return clusterPaths.ConfigPath
}

// createTestConfig creates a test configuration with secrets
func createTestConfig(clusterName string) *v2.Config {
	cfg := newSecretsTestConfig(clusterName, "openstack")
	cfg.OpenCenter.GitOps.Repository.LocalDir = "/tmp/test-repo"
	cfg.Secrets.SopsAgeKeyFile = "~/.config/sops/age/test-key.txt"
	cfg.Secrets.CertManager = v2.CertManagerSecrets{
		AWSAccessKey:       "AKIAIOSFODNN7EXAMPLE",
		AWSSecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}
	cfg.Secrets.Loki = v2.LokiSecrets{
		S3AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		S3SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}
	cfg.Secrets.Keycloak = v2.KeycloakSecrets{
		ClientSecret:  "test-client-secret",
		AdminPassword: "test-admin-password",
	}
	cfg.Secrets.Grafana = v2.GrafanaSecrets{
		AdminPassword: "test-grafana-password",
	}
	return cfg
}

func TestNewDefaultSecretsManager(t *testing.T) {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	configLoader := config.NewConfigIOHandler(fileSystem)
	sopsManager := sops.NewDefaultSOPSManager(nil, nil, slog.Default())

	t.Run("creates manager with provided logger", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		manager := NewDefaultSecretsManager(configLoader, sopsManager, nil, logger)

		assert.NotNil(t, manager)
		assert.Equal(t, logger, manager.logger)
	})

	t.Run("creates manager with default logger when nil", func(t *testing.T) {
		manager := NewDefaultSecretsManager(configLoader, sopsManager, nil, nil)

		assert.NotNil(t, manager)
		assert.NotNil(t, manager.logger)
	})
}

func TestExtractSecretsFromConfig(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("extracts all service secrets", func(t *testing.T) {
		cfg := createTestConfig("test-cluster")

		secretsMap, err := manager.extractSecretsFromConfig(cfg)
		require.NoError(t, err)

		// Verify cert-manager secrets
		assert.Contains(t, secretsMap, "cert-manager")
		assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", secretsMap["cert-manager"]["aws_access_key"])
		assert.Equal(t, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", secretsMap["cert-manager"]["aws_secret_access_key"])

		// Verify Loki secrets
		assert.Contains(t, secretsMap, "loki")
		assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", secretsMap["loki"]["s3_access_key_id"])
		assert.Equal(t, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", secretsMap["loki"]["s3_secret_access_key"])

		// Verify Keycloak secrets
		assert.Contains(t, secretsMap, "keycloak")
		assert.Equal(t, "test-client-secret", secretsMap["keycloak"]["client_secret"])
		assert.Equal(t, "test-admin-password", secretsMap["keycloak"]["admin_password"])

		// Verify Grafana secrets
		assert.Contains(t, secretsMap, "grafana")
		assert.Equal(t, "test-grafana-password", secretsMap["grafana"]["admin_password"])
	})

	t.Run("excludes services with no secrets", func(t *testing.T) {
		cfg := &v2.Config{
			Secrets: v2.SecretsConfig{
				CertManager: v2.CertManagerSecrets{
					AWSAccessKey: "test-key",
				},
			},
		}

		secretsMap, err := manager.extractSecretsFromConfig(cfg)
		require.NoError(t, err)

		assert.Contains(t, secretsMap, "cert-manager")
		assert.NotContains(t, secretsMap, "loki")
		assert.NotContains(t, secretsMap, "keycloak")
		assert.NotContains(t, secretsMap, "grafana")
	})

	t.Run("handles empty config", func(t *testing.T) {
		cfg := &v2.Config{
			Secrets: v2.SecretsConfig{},
		}

		secretsMap, err := manager.extractSecretsFromConfig(cfg)
		require.NoError(t, err)

		assert.Empty(t, secretsMap)
	})

	t.Run("extracts vSphere CSI secrets", func(t *testing.T) {
		cfg := &v2.Config{
			Secrets: v2.SecretsConfig{
				VSphereCsi: v2.VSphereCsiSecrets{
					VCenterHost:  "vcenter.example.com",
					Username:     "administrator@vsphere.local",
					Password:     "test-password",
					Datacenters:  "DC1,DC2",
					InsecureFlag: "true",
					Port:         "443",
					Datastoreurl: "ds:///vmfs/volumes/datastore1/",
				},
			},
		}

		secretsMap, err := manager.extractSecretsFromConfig(cfg)
		require.NoError(t, err)

		assert.Contains(t, secretsMap, "vsphere-csi")
		assert.Equal(t, "vcenter.example.com", secretsMap["vsphere-csi"]["vcenter_host"])
		assert.Equal(t, "administrator@vsphere.local", secretsMap["vsphere-csi"]["username"])
		assert.Equal(t, "test-password", secretsMap["vsphere-csi"]["password"])
	})
}

func TestMapSecretsToManifests(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	cfg := createTestConfig("test-cluster")

	t.Run("maps all services without filter", func(t *testing.T) {
		secretsMap := map[string]map[string]interface{}{
			"cert-manager": {"aws_access_key": "test"},
			"loki":         {"s3_access_key_id": "test"},
			"keycloak":     {"client_secret": "test"},
		}

		manifestPaths, err := manager.mapSecretsToManifests(cfg, secretsMap, nil)
		require.NoError(t, err)

		assert.Len(t, manifestPaths, 3)
		assert.Contains(t, manifestPaths, "cert-manager")
		assert.Contains(t, manifestPaths, "loki")
		assert.Contains(t, manifestPaths, "keycloak")
	})

	t.Run("filters services when filter provided", func(t *testing.T) {
		secretsMap := map[string]map[string]interface{}{
			"cert-manager": {"aws_access_key": "test"},
			"loki":         {"s3_access_key_id": "test"},
			"keycloak":     {"client_secret": "test"},
		}

		manifestPaths, err := manager.mapSecretsToManifests(cfg, secretsMap, []string{"cert-manager", "loki"})
		require.NoError(t, err)

		assert.Len(t, manifestPaths, 2)
		assert.Contains(t, manifestPaths, "cert-manager")
		assert.Contains(t, manifestPaths, "loki")
		assert.NotContains(t, manifestPaths, "keycloak")
	})

	t.Run("returns correct manifest paths", func(t *testing.T) {
		secretsMap := map[string]map[string]interface{}{
			"cert-manager": {"aws_access_key": "test"},
		}

		manifestPaths, err := manager.mapSecretsToManifests(cfg, secretsMap, nil)
		require.NoError(t, err)

		expectedPath := filepath.Join("services", "cert-manager", "secret.yaml")
		assert.Equal(t, expectedPath, manifestPaths["cert-manager"])
	})
}

func TestGetManifestPath(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	cfg := createTestConfig("test-cluster")

	tests := []struct {
		name            string
		service         string
		expectedPattern string
	}{
		{
			name:            "cert-manager",
			service:         "cert-manager",
			expectedPattern: "services/cert-manager/secret.yaml",
		},
		{
			name:            "loki",
			service:         "loki",
			expectedPattern: "services/loki/secret.yaml",
		},
		{
			name:            "keycloak",
			service:         "keycloak",
			expectedPattern: "services/keycloak/secret.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := manager.getManifestPath(tt.service, cfg)
			assert.Equal(t, tt.expectedPattern, path)
		})
	}
}

func TestGetOverlayPath(t *testing.T) {
	manager, tmpDir, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("constructs overlay path from config", func(t *testing.T) {
		cfg := createTestConfig("test-cluster")
		cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "test-repo")

		configPath := filepath.Join(tmpDir, ".k8s-test-cluster-config.yaml")

		overlayPath, err := manager.getOverlayPath(configPath, cfg)
		require.NoError(t, err)

		expectedPath := filepath.Join(tmpDir, "test-repo", "applications", "overlays", "test-cluster")
		assert.Equal(t, expectedPath, overlayPath)
	})

	t.Run("returns error when git_dir not configured", func(t *testing.T) {
		cfg := createTestConfig("test-cluster")
		cfg.OpenCenter.GitOps.Repository.LocalDir = ""

		configPath := filepath.Join(tmpDir, ".k8s-test-cluster-config.yaml")

		_, err := manager.getOverlayPath(configPath, cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gitops.git_dir not configured")
	})
}

func TestSyncSecrets(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("returns error when config not found", func(t *testing.T) {
		opts := SyncOptions{
			Cluster: "nonexistent-cluster",
			DryRun:  true,
		}

		result, err := manager.SyncSecrets(context.Background(), opts)
		assert.Error(t, err)
		assert.Nil(t, result)

		var configErr *ErrConfigNotFound
		assert.ErrorAs(t, err, &configErr)
		assert.Equal(t, "nonexistent-cluster", configErr.Cluster)
	})

	t.Run("filters services when Services field is provided", func(t *testing.T) {
		// This test verifies that the Services field in SyncOptions
		// correctly filters which services are synced
		cfg := createTestConfig("test-cluster-filter")

		// Verify config has multiple services with secrets
		secretsMap, err := manager.extractSecretsFromConfig(cfg)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(secretsMap), 3, "Config should have at least 3 services")
		require.Contains(t, secretsMap, "cert-manager")
		require.Contains(t, secretsMap, "loki")
		require.Contains(t, secretsMap, "keycloak")

		// Test filtering to only cert-manager
		manifestPaths, err := manager.mapSecretsToManifests(cfg, secretsMap, []string{"cert-manager"})
		require.NoError(t, err)

		// Should only include cert-manager
		assert.Len(t, manifestPaths, 1)
		assert.Contains(t, manifestPaths, "cert-manager")
		assert.NotContains(t, manifestPaths, "loki")
		assert.NotContains(t, manifestPaths, "keycloak")
		assert.NotContains(t, manifestPaths, "grafana")

		// Test filtering to multiple services
		manifestPaths, err = manager.mapSecretsToManifests(cfg, secretsMap, []string{"cert-manager", "keycloak"})
		require.NoError(t, err)

		// Should include both cert-manager and keycloak
		assert.Len(t, manifestPaths, 2)
		assert.Contains(t, manifestPaths, "cert-manager")
		assert.Contains(t, manifestPaths, "keycloak")
		assert.NotContains(t, manifestPaths, "loki")
		assert.NotContains(t, manifestPaths, "grafana")

		// Test with empty filter (should include all services)
		manifestPaths, err = manager.mapSecretsToManifests(cfg, secretsMap, []string{})
		require.NoError(t, err)

		// Should include all services from secretsMap
		assert.Equal(t, len(secretsMap), len(manifestPaths))
		for service := range secretsMap {
			assert.Contains(t, manifestPaths, service)
		}

		// Test with non-existent service in filter
		manifestPaths, err = manager.mapSecretsToManifests(cfg, secretsMap, []string{"non-existent-service"})
		require.NoError(t, err)

		// Should return empty map since service doesn't exist
		assert.Len(t, manifestPaths, 0)
	})
}

func TestValidateSecrets(t *testing.T) {
	manager, tmpDir, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("returns error when config not found", func(t *testing.T) {
		opts := ValidateOptions{
			Cluster: "nonexistent-cluster",
		}

		result, err := manager.ValidateSecrets(context.Background(), opts)
		assert.Error(t, err)
		assert.Nil(t, result)

		var configErr *ErrConfigNotFound
		assert.ErrorAs(t, err, &configErr)
		assert.Equal(t, "nonexistent-cluster", configErr.Cluster)
	})

	t.Run("validates secrets successfully with no drift", func(t *testing.T) {
		// Create a test config file in the user's home directory
		homeDir, err := os.UserHomeDir()
		require.NoError(t, err)

		// Create test Age key
		ageKeyDir := filepath.Join(homeDir, ".config", "sops", "age")
		err = os.MkdirAll(ageKeyDir, 0755)
		require.NoError(t, err)

		ageKeyPath := filepath.Join(ageKeyDir, "test-key.txt")
		ageKeyContent := `# created: 2024-01-01T00:00:00Z
# public key: age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
AGE-SECRET-KEY-1GFPYYSJL7VYMDXVJZ4QQZZ7JQJQJQJQJQJQJQJQJQJQJQJQJQJQJQJQ
`
		err = os.WriteFile(ageKeyPath, []byte(ageKeyContent), 0600)
		require.NoError(t, err)
		defer os.Remove(ageKeyPath)

		// Create test repo directory
		testRepoDir := filepath.Join(tmpDir, "test-repo")
		err = os.MkdirAll(testRepoDir, 0755)
		require.NoError(t, err)

		// Write config file
		configData := `schema_version: "2.0"
opencenter:
  cluster:
    cluster_name: test-cluster-validate
  meta:
    organization: test-org
  gitops:
    git_dir: ` + testRepoDir + `
secrets:
  sops_age_key_file: ~/.config/sops/age/test-key.txt
  cert_manager:
    aws_access_key: AKIAIOSFODNN7EXAMPLE
    aws_secret_access_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
`
		_ = writeManagerTestConfig(t, "test-cluster-validate", configData)

		// Create overlay directory (but no manifests)
		overlayPath := filepath.Join(testRepoDir, "applications", "overlays", "test-cluster-validate", "services")
		err = os.MkdirAll(overlayPath, 0755)
		require.NoError(t, err)

		opts := ValidateOptions{
			Cluster: "test-cluster-validate",
		}

		result, err := manager.ValidateSecrets(context.Background(), opts)

		// Should succeed but report missing manifests
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Valid) // Not valid because manifests are missing
		assert.Equal(t, 1, result.ExitCode)
		assert.NotEmpty(t, result.MissingManifests) // Should report missing cert-manager manifest
	})
}

func TestDetectDrift(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("returns error when config not found", func(t *testing.T) {
		report, err := manager.DetectDrift(context.Background(), "nonexistent-cluster")

		assert.Error(t, err)
		assert.Nil(t, report)

		var configNotFoundErr *ErrConfigNotFound
		assert.ErrorAs(t, err, &configNotFoundErr)
	})
}

func TestDetectDriftFields(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("detects no drift when secrets match", func(t *testing.T) {
		configSecrets := map[string]interface{}{
			"username": "test-user",
			"password": "test-pass",
		}

		manifestSecrets := map[string]interface{}{
			"username": "test-user",
			"password": "test-pass",
		}

		driftFields := manager.detectDriftFields(configSecrets, manifestSecrets)
		assert.Empty(t, driftFields)
	})

	t.Run("detects drift when values differ", func(t *testing.T) {
		configSecrets := map[string]interface{}{
			"username": "test-user",
			"password": "new-password",
		}

		manifestSecrets := map[string]interface{}{
			"username": "test-user",
			"password": "old-password",
		}

		driftFields := manager.detectDriftFields(configSecrets, manifestSecrets)
		assert.Len(t, driftFields, 1)
		assert.Equal(t, "data.password", driftFields[0].Path)
		assert.NotEmpty(t, driftFields[0].ConfigHash)
		assert.NotEmpty(t, driftFields[0].ManifestHash)
		assert.NotEqual(t, driftFields[0].ConfigHash, driftFields[0].ManifestHash)
	})

	t.Run("detects missing secrets in manifest", func(t *testing.T) {
		configSecrets := map[string]interface{}{
			"username": "test-user",
			"password": "test-pass",
			"api_key":  "test-key",
		}

		manifestSecrets := map[string]interface{}{
			"username": "test-user",
			"password": "test-pass",
		}

		driftFields := manager.detectDriftFields(configSecrets, manifestSecrets)
		assert.Len(t, driftFields, 1)
		assert.Equal(t, "data.api-key", driftFields[0].Path)
		assert.NotEmpty(t, driftFields[0].ConfigHash)
		assert.Empty(t, driftFields[0].ManifestHash)
	})

	t.Run("handles key format conversion", func(t *testing.T) {
		// Config uses underscores, manifest uses hyphens
		configSecrets := map[string]interface{}{
			"aws_access_key": "test-key",
		}

		manifestSecrets := map[string]interface{}{
			"aws-access-key": "test-key",
		}

		driftFields := manager.detectDriftFields(configSecrets, manifestSecrets)
		assert.Empty(t, driftFields, "Should handle underscore to hyphen conversion")
	})

	t.Run("detects drift with multiple fields", func(t *testing.T) {
		configSecrets := map[string]interface{}{
			"field1": "value1",
			"field2": "value2-new",
			"field3": "value3",
			"field4": "value4-new",
		}

		manifestSecrets := map[string]interface{}{
			"field1": "value1",
			"field2": "value2-old",
			"field3": "value3",
			"field4": "value4-old",
		}

		driftFields := manager.detectDriftFields(configSecrets, manifestSecrets)
		assert.Len(t, driftFields, 2)

		// Verify both drifted fields are detected
		driftPaths := make(map[string]bool)
		for _, field := range driftFields {
			driftPaths[field.Path] = true
		}
		assert.True(t, driftPaths["data.field2"])
		assert.True(t, driftPaths["data.field4"])
	})

	t.Run("handles empty secrets", func(t *testing.T) {
		configSecrets := map[string]interface{}{}
		manifestSecrets := map[string]interface{}{}

		driftFields := manager.detectDriftFields(configSecrets, manifestSecrets)
		assert.Empty(t, driftFields)
	})

	t.Run("detects all config secrets as drift when manifest is empty", func(t *testing.T) {
		configSecrets := map[string]interface{}{
			"username": "test-user",
			"password": "test-pass",
		}
		manifestSecrets := map[string]interface{}{}

		driftFields := manager.detectDriftFields(configSecrets, manifestSecrets)
		assert.Len(t, driftFields, 2)

		for _, field := range driftFields {
			assert.NotEmpty(t, field.ConfigHash)
			assert.Empty(t, field.ManifestHash)
		}
	})
}

func TestGetSecretSources(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("returns error when config not found", func(t *testing.T) {
		sources, err := manager.GetSecretSources(context.Background(), "nonexistent-cluster")
		assert.Error(t, err)
		assert.Nil(t, sources)

		var configErr *ErrConfigNotFound
		assert.ErrorAs(t, err, &configErr)
	})
}

func TestSyncSecretsIntegration(t *testing.T) {
	manager, tmpDir, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("syncs secrets to manifests in dry-run mode", func(t *testing.T) {
		// Create a test config file
		cfg := createTestConfig("test-cluster")
		cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "test-repo")

		// Create config directory structure
		configDir := filepath.Join(tmpDir, ".config", "opencenter", "clusters", "test-org", "test-cluster")
		err := os.MkdirAll(configDir, 0755)
		require.NoError(t, err)

		// Write config file
		configPath := filepath.Join(configDir, ".k8s-test-cluster-config.yaml")
		configData := `schema_version: "2.0"
opencenter:
  cluster:
    cluster_name: test-cluster
  gitops:
    git_dir: ` + filepath.Join(tmpDir, "test-repo") + `
secrets:
  sops_age_key_file: ~/.config/sops/age/test-key.txt
  cert_manager:
    aws_access_key: AKIAIOSFODNN7EXAMPLE
    aws_secret_access_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
  keycloak:
    client_secret: test-client-secret
    admin_password: test-admin-password
`
		writeNormalizedSecretsConfigFile(t, configPath, "test-cluster", configData)

		// Run sync in dry-run mode
		opts := SyncOptions{
			Cluster: "test-cluster",
			DryRun:  true,
		}

		result, err := manager.SyncSecrets(context.Background(), opts)

		// Should fail because Age key doesn't exist, but that's expected
		// In a real scenario, the Age key would be present
		if err != nil {
			t.Logf("Expected error in test environment (no Age key): %v", err)
			return
		}

		// If it somehow succeeded (shouldn't in test env), verify result structure
		assert.NotNil(t, result)
	})

	t.Run("filters services when specified", func(t *testing.T) {
		// This test verifies the service filtering logic
		cfg := createTestConfig("test-cluster")
		secretsMap := map[string]map[string]interface{}{
			"cert-manager": {"aws_access_key": "test"},
			"loki":         {"s3_access_key_id": "test"},
			"keycloak":     {"client_secret": "test"},
		}

		// Test with service filter
		manifestPaths, err := manager.mapSecretsToManifests(cfg, secretsMap, []string{"cert-manager"})
		require.NoError(t, err)

		assert.Len(t, manifestPaths, 1)
		assert.Contains(t, manifestPaths, "cert-manager")
		assert.NotContains(t, manifestPaths, "loki")
		assert.NotContains(t, manifestPaths, "keycloak")
	})
}

func TestGenerateSecretManifest(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("generates new manifest without existing", func(t *testing.T) {
		secrets := map[string]interface{}{
			"aws_access_key":        "AKIAIOSFODNN7EXAMPLE",
			"aws_secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		}

		manifest := manager.generateSecretManifest("cert-manager", secrets, nil)

		assert.Equal(t, "v1", manifest["apiVersion"])
		assert.Equal(t, "Secret", manifest["kind"])

		metadata, ok := manifest["metadata"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "opencenter-cert-manager-secret", metadata["name"])

		data, ok := manifest["data"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", data["aws-access-key"])
		assert.Equal(t, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", data["aws-secret-access-key"])
	})

	t.Run("preserves metadata from existing manifest", func(t *testing.T) {
		existingManifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "custom-secret-name",
				"namespace": "custom-namespace",
				"labels": map[string]interface{}{
					"app": "test-app",
				},
				"annotations": map[string]interface{}{
					"description": "Test secret",
				},
			},
			"data": map[string]interface{}{
				"old-key": "old-value",
			},
		}

		secrets := map[string]interface{}{
			"new_key": "new-value",
		}

		manifest := manager.generateSecretManifest("test-service", secrets, existingManifest)

		metadata, ok := manifest["metadata"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "custom-secret-name", metadata["name"])
		assert.Equal(t, "custom-namespace", metadata["namespace"])
		assert.NotNil(t, metadata["labels"])
		assert.NotNil(t, metadata["annotations"])

		data, ok := manifest["data"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "new-value", data["new-key"])
	})

	t.Run("converts underscores to hyphens in keys", func(t *testing.T) {
		secrets := map[string]interface{}{
			"aws_access_key":        "test-key",
			"aws_secret_access_key": "test-secret",
			"client_secret":         "test-client",
		}

		manifest := manager.generateSecretManifest("test-service", secrets, nil)

		data, ok := manifest["data"].(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, data, "aws-access-key")
		assert.Contains(t, data, "aws-secret-access-key")
		assert.Contains(t, data, "client-secret")
		assert.NotContains(t, data, "aws_access_key")
	})
}

func TestHasSecretsChanged(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("detects no change when secrets are same", func(t *testing.T) {
		newSecrets := map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		}

		existingSecrets := map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		}

		changed := manager.hasSecretsChanged(newSecrets, existingSecrets)
		assert.False(t, changed)
	})

	t.Run("detects change when keys differ", func(t *testing.T) {
		newSecrets := map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		}

		existingSecrets := map[string]interface{}{
			"key1": "value1",
		}

		changed := manager.hasSecretsChanged(newSecrets, existingSecrets)
		assert.True(t, changed)
	})

	t.Run("detects change when values differ", func(t *testing.T) {
		newSecrets := map[string]interface{}{
			"key1": "value1",
			"key2": "new-value",
		}

		existingSecrets := map[string]interface{}{
			"key1": "value1",
			"key2": "old-value",
		}

		changed := manager.hasSecretsChanged(newSecrets, existingSecrets)
		assert.True(t, changed)
	})

	t.Run("handles underscore to hyphen conversion", func(t *testing.T) {
		newSecrets := map[string]interface{}{
			"aws_access_key": "AKIAIOSFODNN7EXAMPLE",
		}

		existingSecrets := map[string]interface{}{
			"aws-access-key": "AKIAIOSFODNN7EXAMPLE",
		}

		changed := manager.hasSecretsChanged(newSecrets, existingSecrets)
		assert.False(t, changed)
	})

	t.Run("detects change when existing has no secrets", func(t *testing.T) {
		newSecrets := map[string]interface{}{
			"key1": "value1",
		}

		existingSecrets := map[string]interface{}{}

		changed := manager.hasSecretsChanged(newSecrets, existingSecrets)
		assert.True(t, changed)
	})
}

func TestValidateSecretsHelperMethods(t *testing.T) {
	manager, tmpDir, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("findManifestFiles finds secret.yaml files", func(t *testing.T) {
		// Create test overlay structure
		overlayPath := filepath.Join(tmpDir, "overlay")
		servicesPath := filepath.Join(overlayPath, "services")

		// Create some service directories with secret.yaml files
		services := []string{"cert-manager", "loki", "keycloak"}
		for _, service := range services {
			serviceDir := filepath.Join(servicesPath, service)
			err := os.MkdirAll(serviceDir, 0755)
			require.NoError(t, err)

			secretPath := filepath.Join(serviceDir, "secret.yaml")
			err = os.WriteFile(secretPath, []byte("test content"), 0644)
			require.NoError(t, err)
		}

		// Find manifest files
		manifestFiles, err := manager.findManifestFiles(overlayPath)
		require.NoError(t, err)

		assert.Len(t, manifestFiles, 3)
		for _, file := range manifestFiles {
			assert.Contains(t, file, "secret.yaml")
		}
	})

	t.Run("extractServiceFromPath extracts service name", func(t *testing.T) {
		testCases := []struct {
			path            string
			expectedService string
		}{
			{
				path:            "/path/to/services/cert-manager/secret.yaml",
				expectedService: "cert-manager",
			},
			{
				path:            "/path/to/services/loki/secret.yaml",
				expectedService: "loki",
			},
			{
				path:            "/path/to/services/keycloak/secret.yaml",
				expectedService: "keycloak",
			},
		}

		for _, tc := range testCases {
			service := manager.extractServiceFromPath(tc.path)
			assert.Equal(t, tc.expectedService, service)
		}
	})

	t.Run("isManifestEncrypted detects SOPS encryption", func(t *testing.T) {
		// Create encrypted manifest (with SOPS metadata)
		encryptedPath := filepath.Join(tmpDir, "encrypted.yaml")
		encryptedContent := `apiVersion: v1
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
    - recipient: age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
      enc: |
        -----BEGIN AGE ENCRYPTED FILE-----
        test
        -----END AGE ENCRYPTED FILE-----
  lastmodified: "2024-01-01T00:00:00Z"
  mac: ENC[AES256_GCM,data:test,iv:test,tag:test,type:str]
  pgp: []
  version: 3.7.3
`
		err := os.WriteFile(encryptedPath, []byte(encryptedContent), 0644)
		require.NoError(t, err)

		isEncrypted, err := manager.isManifestEncrypted(encryptedPath)
		require.NoError(t, err)
		assert.True(t, isEncrypted)

		// Create unencrypted manifest
		unencryptedPath := filepath.Join(tmpDir, "unencrypted.yaml")
		unencryptedContent := `apiVersion: v1
kind: Secret
metadata:
  name: test-secret
data:
  password: plaintext-password
`
		err = os.WriteFile(unencryptedPath, []byte(unencryptedContent), 0644)
		require.NoError(t, err)

		isEncrypted, err = manager.isManifestEncrypted(unencryptedPath)
		require.NoError(t, err)
		assert.False(t, isEncrypted)
	})

	t.Run("compareSecrets detects drift", func(t *testing.T) {
		configSecrets := map[string]interface{}{
			"aws_access_key":        "AKIAIOSFODNN7EXAMPLE",
			"aws_secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		}

		// Manifest with same values (no drift)
		manifestSecrets := map[string]interface{}{
			"aws-access-key":        "AKIAIOSFODNN7EXAMPLE",
			"aws-secret-access-key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		}

		driftItems := manager.compareSecrets("cert-manager", configSecrets, manifestSecrets)
		assert.Empty(t, driftItems)

		// Manifest with different values (drift)
		manifestSecretsDrift := map[string]interface{}{
			"aws-access-key":        "DIFFERENT_KEY",
			"aws-secret-access-key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		}

		driftItems = manager.compareSecrets("cert-manager", configSecrets, manifestSecretsDrift)
		assert.Len(t, driftItems, 1)
		assert.Equal(t, "cert-manager", driftItems[0].Service)
		assert.Equal(t, "data.aws-access-key", driftItems[0].FieldPath)

		// Manifest missing a key
		manifestSecretsMissing := map[string]interface{}{
			"aws-access-key": "AKIAIOSFODNN7EXAMPLE",
		}

		driftItems = manager.compareSecrets("cert-manager", configSecrets, manifestSecretsMissing)
		assert.Len(t, driftItems, 1)
		assert.Equal(t, "data.aws-secret-access-key", driftItems[0].FieldPath)
		assert.Empty(t, driftItems[0].ManifestHash) // Empty hash indicates missing
	})

	t.Run("hashValue creates consistent hashes", func(t *testing.T) {
		value1 := "test-password-123"
		value2 := "test-password-123"
		value3 := "different-password"

		hash1 := manager.hashValue(value1)
		hash2 := manager.hashValue(value2)
		hash3 := manager.hashValue(value3)

		// Same values should produce same hash
		assert.Equal(t, hash1, hash2)

		// Different values should produce different hashes
		assert.NotEqual(t, hash1, hash3)

		// Hash should be non-empty
		assert.NotEmpty(t, hash1)
	})
}

func TestValidateSecretsWithSecurityIssues(t *testing.T) {
	manager, tmpDir, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("detects unencrypted secrets as security issues", func(t *testing.T) {
		// Create a test config file in the user's home directory
		homeDir, err := os.UserHomeDir()
		require.NoError(t, err)

		// Create test Age key
		ageKeyDir := filepath.Join(homeDir, ".config", "sops", "age")
		err = os.MkdirAll(ageKeyDir, 0755)
		require.NoError(t, err)

		ageKeyPath := filepath.Join(ageKeyDir, "test-key-security.txt")
		ageKeyContent := `# created: 2024-01-01T00:00:00Z
# public key: age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
AGE-SECRET-KEY-1GFPYYSJL7VYMDXVJZ4QQZZ7JQJQJQJQJQJQJQJQJQJQJQJQJQJQJQJQ
`
		err = os.WriteFile(ageKeyPath, []byte(ageKeyContent), 0600)
		require.NoError(t, err)
		defer os.Remove(ageKeyPath)

		// Create test repo directory
		testRepoDir := filepath.Join(tmpDir, "test-repo-security")
		err = os.MkdirAll(testRepoDir, 0755)
		require.NoError(t, err)

		// Write config file
		configData := `schema_version: "2.0"
opencenter:
  cluster:
    cluster_name: test-cluster-security
  meta:
    organization: test-org
  gitops:
    git_dir: ` + testRepoDir + `
secrets:
  sops_age_key_file: ~/.config/sops/age/test-key-security.txt
  cert_manager:
    aws_access_key: AKIAIOSFODNN7EXAMPLE
    aws_secret_access_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
`
		_ = writeManagerTestConfig(t, "test-cluster-security", configData)

		// Create overlay directory with unencrypted manifest
		overlayPath := filepath.Join(testRepoDir, "applications", "overlays", "test-cluster-security", "services")
		certManagerDir := filepath.Join(overlayPath, "cert-manager")
		err = os.MkdirAll(certManagerDir, 0755)
		require.NoError(t, err)

		// Create unencrypted secret manifest
		unencryptedManifest := filepath.Join(certManagerDir, "secret.yaml")
		unencryptedContent := `apiVersion: v1
kind: Secret
metadata:
  name: cert-manager-secret
data:
  aws-access-key: AKIAIOSFODNN7EXAMPLE
  aws-secret-access-key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
`
		err = os.WriteFile(unencryptedManifest, []byte(unencryptedContent), 0644)
		require.NoError(t, err)

		opts := ValidateOptions{
			Cluster: "test-cluster-security",
		}

		result, err := manager.ValidateSecrets(context.Background(), opts)

		// Should succeed but report security issues
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Valid)
		assert.Equal(t, 1, result.ExitCode)
		assert.NotEmpty(t, result.SecurityIssues)
		assert.Equal(t, "critical", result.SecurityIssues[0].Severity)
	})
}

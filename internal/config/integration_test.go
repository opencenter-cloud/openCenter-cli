// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// addDefaultSecrets adds all required secrets for services that are enabled by default.
// This helper function ensures tests don't fail due to missing secrets for default services.
func addDefaultSecrets(config *Config) {
	config.Secrets.CertManager.AWSAccessKey = "test-key"
	config.Secrets.CertManager.AWSSecretAccessKey = "test-secret"
	config.Secrets.Keycloak.AdminPassword = "test-password"
	config.Secrets.Grafana.AdminPassword = "test-password"
	config.Secrets.WeaveGitOps.PasswordHash = "test-hash"
	config.Secrets.Headlamp.OIDCClientSecret = "test-headlamp-secret"
	config.Secrets.AlertProxy.CoreDeviceId = "test-device-id"
	config.Secrets.AlertProxy.AccountServiceToken = "test-service-token"
	config.Secrets.AlertProxy.CoreAccountNumber = "test-account-number"
}

func TestConfigurationManagerIntegration(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "openCenter-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set up environment
	os.Setenv("OPENCENTER_CONFIG_DIR", tempDir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	// Create factory
	factory, err := GetDefaultFactory()
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}

	// Create configuration manager
	configManager := factory.CreateTestConfigurationManager()

	ctx := context.Background()
	clusterName := "test-cluster"

	// Test loading default configuration
	t.Run("LoadDefaultConfig", func(t *testing.T) {
		config, err := configManager.loader.LoadDefault(ctx, clusterName)
		if err != nil {
			t.Fatalf("Failed to load default config: %v", err)
		}

		if config.ClusterName() != clusterName {
			t.Errorf("Expected cluster name %s, got %s", clusterName, config.ClusterName())
		}
	})

	// Test configuration validation
	t.Run("ValidateConfig", func(t *testing.T) {
		config, err := configManager.loader.LoadDefault(ctx, clusterName)
		if err != nil {
			t.Fatalf("Failed to load default config: %v", err)
		}

		// Add required secrets for enabled services
		addDefaultSecrets(config)

		result := configManager.ValidateConfig(ctx, config)
		if result == nil {
			t.Fatal("Validation result should not be nil")
		}

		// Default config should be valid with secrets
		if !result.Valid {
			t.Errorf("Default configuration should be valid, but got errors: %v", result.Errors)
		}
	})

	// Test path resolution
	t.Run("PathResolution", func(t *testing.T) {
		paths, err := configManager.GetClusterPaths(ctx, clusterName)
		if err != nil {
			t.Fatalf("Failed to get cluster paths: %v", err)
		}

		if paths.ClusterDir == "" {
			t.Error("Cluster directory should not be empty")
		}

		if paths.SecretsDir == "" {
			t.Error("Secrets directory should not be empty")
		}
	})

	// Test directory creation
	t.Run("DirectoryCreation", func(t *testing.T) {
		organization := "test-org"
		err := configManager.CreateClusterDirectories(ctx, clusterName, organization)
		if err != nil {
			t.Fatalf("Failed to create cluster directories: %v", err)
		}

		// Verify directories were created
		paths, err := configManager.pathResolver.ResolveClusterPaths(ctx, clusterName, organization)
		if err != nil {
			t.Fatalf("Failed to resolve paths: %v", err)
		}

		if _, err := os.Stat(paths.ClusterDir); os.IsNotExist(err) {
			t.Error("Cluster directory was not created")
		}

		if _, err := os.Stat(paths.SecretsDir); os.IsNotExist(err) {
			t.Error("Secrets directory was not created")
		}
	})

	// Test configuration saving and loading
	t.Run("SaveAndLoadConfig", func(t *testing.T) {
		// Create a test configuration
		config, err := configManager.loader.LoadDefault(ctx, clusterName)
		if err != nil {
			t.Fatalf("Failed to load default config: %v", err)
		}

		// Modify the configuration
		config.OpenCenter.Meta.Organization = "test-org"

		// Add required secrets for enabled services
		addDefaultSecrets(config)

		// Save the configuration
		err = configManager.SaveConfig(ctx, config)
		if err != nil {
			t.Fatalf("Failed to save config: %v", err)
		}

		// Load the configuration back
		loadedConfig, err := configManager.LoadConfig(ctx, clusterName)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Verify the modification was saved
		if loadedConfig.OpenCenter.Meta.Organization != "test-org" {
			t.Errorf("Expected organization 'test-org', got '%s'", loadedConfig.OpenCenter.Meta.Organization)
		}
	})
}

func TestConfigCacheIntegration(t *testing.T) {
	cache := NewInMemoryConfigCache(100*time.Millisecond, 10)
	ctx := context.Background()

	// Test basic cache operations
	t.Run("BasicCacheOperations", func(t *testing.T) {
		config := NewDefault("test-cluster")
		key := "test-key"

		// Test Set and Get
		err := cache.Set(ctx, key, &config)
		if err != nil {
			t.Fatalf("Failed to set cache: %v", err)
		}

		cachedConfig, found := cache.Get(ctx, key)
		if !found {
			t.Error("Config should be found in cache")
		}

		if cachedConfig.ClusterName() != config.ClusterName() {
			t.Error("Cached config does not match original")
		}

		// Test Delete
		err = cache.Delete(ctx, key)
		if err != nil {
			t.Fatalf("Failed to delete from cache: %v", err)
		}

		_, found = cache.Get(ctx, key)
		if found {
			t.Error("Config should not be found after deletion")
		}
	})

	// Test cache expiration
	t.Run("CacheExpiration", func(t *testing.T) {
		config := NewDefault("test-cluster")
		key := "expiry-test"

		err := cache.Set(ctx, key, &config)
		if err != nil {
			t.Fatalf("Failed to set cache: %v", err)
		}

		// Wait for expiration
		time.Sleep(150 * time.Millisecond)

		_, found := cache.Get(ctx, key)
		if found {
			t.Error("Config should have expired")
		}
	})
}

func TestPathResolverIntegration(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "openCenter-path-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set up environment
	os.Setenv("OPENCENTER_CONFIG_DIR", tempDir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	// Create CLI config manager
	cliConfigManager, err := NewConfigManager("")
	if err != nil {
		t.Fatalf("Failed to create CLI config manager: %v", err)
	}

	pathResolver := NewPathResolverImpl(cliConfigManager)
	ctx := context.Background()

	t.Run("OrganizationPathResolution", func(t *testing.T) {
		clusterName := "test-cluster"
		organization := "test-org"

		paths, err := pathResolver.ResolveClusterPaths(ctx, clusterName, organization)
		if err != nil {
			t.Fatalf("Failed to resolve paths: %v", err)
		}

		// Verify path structure
		expectedClusterDir := filepath.Join(tempDir, "clusters", organization, "infrastructure", "clusters", clusterName)
		if paths.ClusterDir != expectedClusterDir {
			t.Errorf("Expected cluster dir %s, got %s", expectedClusterDir, paths.ClusterDir)
		}

		expectedSecretsDir := filepath.Join(tempDir, "clusters", organization, "secrets")
		if paths.SecretsDir != expectedSecretsDir {
			t.Errorf("Expected secrets dir %s, got %s", expectedSecretsDir, paths.SecretsDir)
		}
	})

	t.Run("DirectoryCreation", func(t *testing.T) {
		clusterName := "test-cluster-2"
		organization := "test-org-2"

		err := pathResolver.CreateOrganizationStructure(ctx, organization)
		if err != nil {
			t.Fatalf("Failed to create organization structure: %v", err)
		}

		err = pathResolver.CreateClusterDirectories(ctx, clusterName, organization)
		if err != nil {
			t.Fatalf("Failed to create cluster directories: %v", err)
		}

		// Verify directories exist
		paths, err := pathResolver.ResolveClusterPaths(ctx, clusterName, organization)
		if err != nil {
			t.Fatalf("Failed to resolve paths: %v", err)
		}

		if _, err := os.Stat(paths.ClusterDir); os.IsNotExist(err) {
			t.Error("Cluster directory should exist")
		}

		if _, err := os.Stat(paths.SecretsDir); os.IsNotExist(err) {
			t.Error("Secrets directory should exist")
		}
	})
}

func TestFullClusterRendering(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "openCenter-render-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set up environment
	os.Setenv("OPENCENTER_CONFIG_DIR", tempDir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	ctx := context.Background()

	t.Run("LoadIntegrationTestConfig", func(t *testing.T) {
		// Load the integration test configuration
		configPath := filepath.Join("../../testdata/config/clusters/test-integration/cluster.yaml")

		// Check if file exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Skipf("Integration test config not found at %s", configPath)
		}

		// Create factory and loader
		factory, err := GetDefaultFactory()
		if err != nil {
			t.Fatalf("Failed to create factory: %v", err)
		}
		loader := factory.CreateConfigLoader()

		config, err := loader.LoadFromFile(ctx, configPath)
		if err != nil {
			t.Fatalf("Failed to load integration test config: %v", err)
		}

		// Verify cluster configuration fields
		if config.OpenCenter.Cluster.ClusterName != "test-integration" {
			t.Errorf("Expected cluster name 'test-integration', got '%s'", config.OpenCenter.Cluster.ClusterName)
		}

		if config.OpenCenter.Cluster.BaseDomain != "k8s.test.example.com" {
			t.Errorf("Expected base domain 'k8s.test.example.com', got '%s'", config.OpenCenter.Cluster.BaseDomain)
		}

		if config.OpenCenter.Cluster.ClusterFQDN != "test-integration.us-east-1.k8s.test.example.com" {
			t.Errorf("Expected cluster FQDN 'test-integration.us-east-1.k8s.test.example.com', got '%s'", config.OpenCenter.Cluster.ClusterFQDN)
		}

		if config.OpenCenter.Cluster.AdminEmail != "admin@test.example.com" {
			t.Errorf("Expected admin email 'admin@test.example.com', got '%s'", config.OpenCenter.Cluster.AdminEmail)
		}

		// Verify storage configuration
		if config.OpenCenter.Storage.DefaultStorageClass != "csi-cinder-sc-delete" {
			t.Errorf("Expected default storage class 'csi-cinder-sc-delete', got '%s'", config.OpenCenter.Storage.DefaultStorageClass)
		}

		// Verify GitOps configuration
		if config.OpenCenter.GitOps.GitOpsBaseRepo != "ssh://git@github.com/rackerlabs/openCenter-gitops-base.git" {
			t.Errorf("Expected GitOps base repo 'ssh://git@github.com/rackerlabs/openCenter-gitops-base.git', got '%s'", config.OpenCenter.GitOps.GitOpsBaseRepo)
		}

		if config.OpenCenter.GitOps.GitOpsBaseRelease != "v0.2.0" {
			t.Errorf("Expected GitOps base release 'v0.2.0', got '%s'", config.OpenCenter.GitOps.GitOpsBaseRelease)
		}

		if config.OpenCenter.GitOps.GitOpsBranch != "main" {
			t.Errorf("Expected GitOps branch 'main', got '%s'", config.OpenCenter.GitOps.GitOpsBranch)
		}
	})

	t.Run("VerifyServiceConfiguration", func(t *testing.T) {
		configPath := filepath.Join("../../testdata/config/clusters/test-integration/cluster.yaml")

		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Skipf("Integration test config not found at %s", configPath)
		}

		factory, err := GetDefaultFactory()
		if err != nil {
			t.Fatalf("Failed to create factory: %v", err)
		}
		loader := factory.CreateConfigLoader()

		config, err := loader.LoadFromFile(ctx, configPath)
		if err != nil {
			t.Fatalf("Failed to load integration test config: %v", err)
		}

		// Verify cert-manager configuration
		certManager := config.OpenCenter.Services["cert-manager"]
		if !certManager.Enabled {
			t.Error("cert-manager should be enabled")
		}
		if certManager.LetsEncryptServer != "https://acme-staging-v02.api.letsencrypt.org/directory" {
			t.Errorf("Expected staging LetsEncrypt server, got '%s'", certManager.LetsEncryptServer)
		}
		if certManager.Region != "us-east-1" {
			t.Errorf("Expected region 'us-east-1', got '%s'", certManager.Region)
		}

		// Verify Loki configuration
		loki := config.OpenCenter.Services["loki"]
		if !loki.Enabled {
			t.Error("loki should be enabled")
		}
		if loki.LokiBucketName != "test-integration-loki" {
			t.Errorf("Expected bucket name 'test-integration-loki', got '%s'", loki.LokiBucketName)
		}
		if loki.LokiVolumeSize != 50 {
			t.Errorf("Expected volume size 50, got %d", loki.LokiVolumeSize)
		}
		if loki.LokiStorageClass != "csi-cinder-sc-delete" {
			t.Errorf("Expected storage class 'csi-cinder-sc-delete', got '%s'", loki.LokiStorageClass)
		}
		if loki.SwiftAuthURL != "https://keystone.api.test.example.com/v3/" {
			t.Errorf("Expected Swift auth URL 'https://keystone.api.test.example.com/v3/', got '%s'", loki.SwiftAuthURL)
		}

		// Verify Velero configuration
		velero := config.OpenCenter.Services["velero"]
		if !velero.Enabled {
			t.Error("velero should be enabled")
		}
		if velero.VeleroBackupBucket != "test-integration-backups" {
			t.Errorf("Expected backup bucket 'test-integration-backups', got '%s'", velero.VeleroBackupBucket)
		}
		if velero.VeleroRegion != "us-east-1" {
			t.Errorf("Expected region 'us-east-1', got '%s'", velero.VeleroRegion)
		}

		// Verify Keycloak configuration
		keycloak := config.OpenCenter.Services["keycloak"]
		if !keycloak.Enabled {
			t.Error("keycloak should be enabled")
		}
		if keycloak.KeycloakRealm != "opencenter" {
			t.Errorf("Expected realm 'opencenter', got '%s'", keycloak.KeycloakRealm)
		}
		if keycloak.KeycloakClientID != "opencenter" {
			t.Errorf("Expected client ID 'opencenter', got '%s'", keycloak.KeycloakClientID)
		}
		if keycloak.Hostname != "auth.test-integration.us-east-1.k8s.test.example.com" {
			t.Errorf("Expected hostname 'auth.test-integration.us-east-1.k8s.test.example.com', got '%s'", keycloak.Hostname)
		}

		// Verify Headlamp configuration
		headlamp := config.OpenCenter.Services["headlamp"]
		if !headlamp.Enabled {
			t.Error("headlamp should be enabled")
		}
		if headlamp.HeadlampOIDCClientID != "opencenter" {
			t.Errorf("Expected OIDC client ID 'opencenter', got '%s'", headlamp.HeadlampOIDCClientID)
		}
		if headlamp.Hostname != "headlamp.test-integration.us-east-1.k8s.test.example.com" {
			t.Errorf("Expected hostname 'headlamp.test-integration.us-east-1.k8s.test.example.com', got '%s'", headlamp.Hostname)
		}

		// Verify kube-prometheus-stack configuration
		prometheus := config.OpenCenter.Services["kube-prometheus-stack"]
		if !prometheus.Enabled {
			t.Error("kube-prometheus-stack should be enabled")
		}
		if prometheus.GrafanaVolumeSize != 20 {
			t.Errorf("Expected Grafana volume size 20, got %d", prometheus.GrafanaVolumeSize)
		}
		if prometheus.PrometheusVolumeSize != 100 {
			t.Errorf("Expected Prometheus volume size 100, got %d", prometheus.PrometheusVolumeSize)
		}
		if prometheus.AlertmanagerVolumeSize != 10 {
			t.Errorf("Expected Alertmanager volume size 10, got %d", prometheus.AlertmanagerVolumeSize)
		}

		// Verify Calico configuration
		calico := config.OpenCenter.Services["calico"]
		if !calico.Enabled {
			t.Error("calico should be enabled")
		}
		if calico.CalicoKubeAPIServer != "https://test-integration-api.us-east-1.k8s.test.example.com:6443" {
			t.Errorf("Expected API server 'https://test-integration-api.us-east-1.k8s.test.example.com:6443', got '%s'", calico.CalicoKubeAPIServer)
		}
	})

	t.Run("VerifySecretsConfiguration", func(t *testing.T) {
		configPath := filepath.Join("../../testdata/config/clusters/test-integration/cluster.yaml")

		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Skipf("Integration test config not found at %s", configPath)
		}

		factory, err := GetDefaultFactory()
		if err != nil {
			t.Fatalf("Failed to create factory: %v", err)
		}
		loader := factory.CreateConfigLoader()

		config, err := loader.LoadFromFile(ctx, configPath)
		if err != nil {
			t.Fatalf("Failed to load integration test config: %v", err)
		}

		// Verify cert-manager secrets
		if config.Secrets.CertManager.AWSAccessKey != "AKIATEST123456789ABC" {
			t.Errorf("Expected AWS access key 'AKIATEST123456789ABC', got '%s'", config.Secrets.CertManager.AWSAccessKey)
		}
		if config.Secrets.CertManager.AWSSecretAccessKey != "wJalrXUtnFEMI/K7MDENG/bPxRfiCYTESTKEY123" {
			t.Errorf("Expected AWS secret key 'wJalrXUtnFEMI/K7MDENG/bPxRfiCYTESTKEY123', got '%s'", config.Secrets.CertManager.AWSSecretAccessKey)
		}

		// Verify Loki secrets
		if config.Secrets.Loki.SwiftPassword != "test-swift-password-secure-123" {
			t.Errorf("Expected Swift password 'test-swift-password-secure-123', got '%s'", config.Secrets.Loki.SwiftPassword)
		}

		// Verify Keycloak secrets
		if config.Secrets.Keycloak.ClientSecret != "f8V0we25ajxjm9OMpFz9BsYObGTYKM4Y" {
			t.Errorf("Expected client secret 'f8V0we25ajxjm9OMpFz9BsYObGTYKM4Y', got '%s'", config.Secrets.Keycloak.ClientSecret)
		}
		if config.Secrets.Keycloak.AdminPassword != "SecureKeycloakAdminPassword123!" {
			t.Errorf("Expected admin password 'SecureKeycloakAdminPassword123!', got '%s'", config.Secrets.Keycloak.AdminPassword)
		}

		// Verify Headlamp secrets
		if config.Secrets.Headlamp.OIDCClientSecret != "headlamp-oidc-secret-abc123xyz" {
			t.Errorf("Expected OIDC client secret 'headlamp-oidc-secret-abc123xyz', got '%s'", config.Secrets.Headlamp.OIDCClientSecret)
		}

		// Verify Weave GitOps secrets
		if config.Secrets.WeaveGitOps.Password != "WeaveGitOpsPassword123!" {
			t.Errorf("Expected password 'WeaveGitOpsPassword123!', got '%s'", config.Secrets.WeaveGitOps.Password)
		}
		if config.Secrets.WeaveGitOps.PasswordHash != "$2a$10$abcdefghijklmnopqrstuvwxyz1234567890ABCDEFGHIJKLMNOP" {
			t.Errorf("Expected password hash '$2a$10$abcdefghijklmnopqrstuvwxyz1234567890ABCDEFGHIJKLMNOP', got '%s'", config.Secrets.WeaveGitOps.PasswordHash)
		}

		// Verify Grafana secrets
		if config.Secrets.Grafana.AdminPassword != "GrafanaAdminPassword123!" {
			t.Errorf("Expected admin password 'GrafanaAdminPassword123!', got '%s'", config.Secrets.Grafana.AdminPassword)
		}

		// Verify alert-proxy secrets
		if config.Secrets.AlertProxy.CoreDeviceId != "device-test-integration-12345" {
			t.Errorf("Expected device ID 'device-test-integration-12345', got '%s'", config.Secrets.AlertProxy.CoreDeviceId)
		}
		if config.Secrets.AlertProxy.AccountServiceToken != "token-test-integration-67890" {
			t.Errorf("Expected service token 'token-test-integration-67890', got '%s'", config.Secrets.AlertProxy.AccountServiceToken)
		}
		if config.Secrets.AlertProxy.CoreAccountNumber != "account-test-integration-11111" {
			t.Errorf("Expected account number 'account-test-integration-11111', got '%s'", config.Secrets.AlertProxy.CoreAccountNumber)
		}
	})

	t.Run("VerifyManagedServiceConfiguration", func(t *testing.T) {
		configPath := filepath.Join("../../testdata/config/clusters/test-integration/cluster.yaml")

		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Skipf("Integration test config not found at %s", configPath)
		}

		factory, err := GetDefaultFactory()
		if err != nil {
			t.Fatalf("Failed to create factory: %v", err)
		}
		loader := factory.CreateConfigLoader()

		config, err := loader.LoadFromFile(ctx, configPath)
		if err != nil {
			t.Fatalf("Failed to load integration test config: %v", err)
		}

		// Verify alert-proxy configuration
		alertProxy := config.OpenCenter.ManagedService["alert-proxy"]
		if !alertProxy.Enabled {
			t.Error("alert-proxy should be enabled")
		}
		if alertProxy.AlertManagerBaseUrl != "https://alertmanager.test-integration.us-east-1.k8s.test.example.com" {
			t.Errorf("Expected AlertManager URL 'https://alertmanager.test-integration.us-east-1.k8s.test.example.com', got '%s'", alertProxy.AlertManagerBaseUrl)
		}
		if alertProxy.HTTPRouteFQDN != "alerts.test-integration.us-east-1.k8s.test.example.com" {
			t.Errorf("Expected HTTPRoute FQDN 'alerts.test-integration.us-east-1.k8s.test.example.com', got '%s'", alertProxy.HTTPRouteFQDN)
		}
		if alertProxy.ImageTag != "v1.2.3" {
			t.Errorf("Expected image tag 'v1.2.3', got '%s'", alertProxy.ImageTag)
		}
	})

	t.Run("VerifyNoHardcodedValues", func(t *testing.T) {
		configPath := filepath.Join("../../testdata/config/clusters/test-integration/cluster.yaml")

		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Skipf("Integration test config not found at %s", configPath)
		}

		factory, err := GetDefaultFactory()
		if err != nil {
			t.Fatalf("Failed to create factory: %v", err)
		}
		loader := factory.CreateConfigLoader()

		config, err := loader.LoadFromFile(ctx, configPath)
		if err != nil {
			t.Fatalf("Failed to load integration test config: %v", err)
		}

		// Verify that all critical fields are populated from config, not hardcoded
		// This is a sanity check that the configuration is complete

		// Check cluster fields
		if config.OpenCenter.Cluster.ClusterName == "" {
			t.Error("ClusterName should not be empty")
		}
		if config.OpenCenter.Cluster.BaseDomain == "" {
			t.Error("BaseDomain should not be empty")
		}
		if config.OpenCenter.Cluster.ClusterFQDN == "" {
			t.Error("ClusterFQDN should not be empty")
		}
		if config.OpenCenter.Cluster.AdminEmail == "" {
			t.Error("AdminEmail should not be empty")
		}

		// Check storage fields
		if config.OpenCenter.Storage.DefaultStorageClass == "" {
			t.Error("DefaultStorageClass should not be empty")
		}

		// Check GitOps fields
		if config.OpenCenter.GitOps.GitOpsBaseRepo == "" {
			t.Error("GitOpsBaseRepo should not be empty")
		}
		if config.OpenCenter.GitOps.GitOpsBaseRelease == "" {
			t.Error("GitOpsBaseRelease should not be empty")
		}

		// Check that secrets are present for enabled services
		if config.OpenCenter.Services["cert-manager"].Enabled {
			if config.Secrets.CertManager.AWSAccessKey == "" {
				t.Error("CertManager AWS access key should not be empty when cert-manager is enabled")
			}
			if config.Secrets.CertManager.AWSSecretAccessKey == "" {
				t.Error("CertManager AWS secret key should not be empty when cert-manager is enabled")
			}
		}

		if config.OpenCenter.Services["loki"].Enabled {
			if config.Secrets.Loki.SwiftPassword == "" {
				t.Error("Loki Swift password should not be empty when loki is enabled")
			}
		}

		if config.OpenCenter.Services["keycloak"].Enabled {
			if config.Secrets.Keycloak.AdminPassword == "" {
				t.Error("Keycloak admin password should not be empty when keycloak is enabled")
			}
		}

		if config.OpenCenter.Services["kube-prometheus-stack"].Enabled {
			if config.Secrets.Grafana.AdminPassword == "" {
				t.Error("Grafana admin password should not be empty when kube-prometheus-stack is enabled")
			}
		}

		if config.OpenCenter.Services["weave-gitops"].Enabled {
			if config.Secrets.WeaveGitOps.PasswordHash == "" {
				t.Error("Weave GitOps password hash should not be empty when weave-gitops is enabled")
			}
		}
	})
}

func TestConfigurationValidation(t *testing.T) {
	ctx := context.Background()

	t.Run("ValidateCompleteConfiguration", func(t *testing.T) {
		configPath := filepath.Join("../../testdata/config/clusters/test-integration/cluster.yaml")

		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Skipf("Integration test config not found at %s", configPath)
		}

		factory, err := GetDefaultFactory()
		if err != nil {
			t.Fatalf("Failed to create factory: %v", err)
		}
		loader := factory.CreateConfigLoader()

		config, err := loader.LoadFromFile(ctx, configPath)
		if err != nil {
			t.Fatalf("Failed to load integration test config: %v", err)
		}

		// Validate the configuration
		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, config)

		if result == nil {
			t.Fatal("Validation result should not be nil")
		}

		if !result.Valid {
			t.Errorf("Complete configuration should be valid, but got errors: %v", result.Errors)
		}

		if len(result.Errors) > 0 {
			t.Errorf("Expected no validation errors, but got: %v", result.Errors)
		}
	})

	t.Run("DetectMissingClusterName", func(t *testing.T) {
		config := NewDefault("")
		config.OpenCenter.Cluster.ClusterName = ""

		// Add default secrets so we only test cluster name validation
		addDefaultSecrets(&config)

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Configuration with empty cluster name should be invalid")
		}

		foundError := false
		for _, err := range result.Errors {
			if err.Field == "opencenter.cluster.cluster_name" || err.Message == "cluster name must be set" {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about missing cluster_name, but got: %v", result.Errors)
		}
	})

	t.Run("DetectInvalidEmail", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Cluster.AdminEmail = "invalid-email"

		// Add required secrets
		addDefaultSecrets(&config)

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Configuration with invalid email should be invalid")
		}

		foundError := false
		for _, err := range result.Errors {
			if err.Field == "opencenter.cluster.admin_email" {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about invalid admin_email, but got: %v", result.Errors)
		}
	})

	t.Run("DetectInvalidDomain", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Cluster.ClusterFQDN = "invalid domain with spaces"

		// Add required secrets
		addDefaultSecrets(&config)

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Configuration with invalid domain should be invalid")
		}

		foundError := false
		for _, err := range result.Errors {
			if err.Field == "opencenter.cluster.cluster_fqdn" {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about invalid cluster_fqdn, but got: %v", result.Errors)
		}
	})

	t.Run("DetectMissingCertManagerSecrets", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Services["cert-manager"] = ServiceCfg{
			Enabled: true,
		}
		// Don't set cert-manager secrets

		// Add other required secrets
		addDefaultSecrets(&config)
		// Clear cert-manager secrets to test validation
		config.Secrets.CertManager.AWSAccessKey = ""
		config.Secrets.CertManager.AWSSecretAccessKey = ""

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Configuration with missing cert-manager secrets should be invalid")
		}

		foundError := false
		for _, err := range result.Errors {
			if err.Field == "secrets.cert_manager.aws_access_key" || err.Field == "secrets.cert_manager.aws_secret_access_key" {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about missing cert-manager secrets, but got: %v", result.Errors)
		}
	})

	t.Run("DetectMissingLokiSecrets", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Services["loki"] = ServiceCfg{
			Enabled: true,
		}
		// Don't set loki secrets

		// Add other required secrets
		addDefaultSecrets(&config)

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Configuration with missing loki secrets should be invalid")
		}

		foundError := false
		for _, err := range result.Errors {
			if err.Field == "secrets.loki.swift_password" {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about missing loki secrets, but got: %v", result.Errors)
		}
	})

	t.Run("DetectLokiMixedStorageBackends", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Services["loki"] = ServiceCfg{
			Enabled:                      true,
			LokiStorageType:              "swift",
			LokiBucketName:               "loki-logs",
			SwiftAuthURL:                 "https://auth.cloud.ovh.net/v3",
			SwiftRegion:                  "GRA9",
			SwiftApplicationCredentialID: "test-app-cred-id",
			// Also configure S3 (should fail)
			LokiS3Region:   "us-east-1",
			LokiS3Endpoint: "https://s3.amazonaws.com",
		}
		config.Secrets.Loki.SwiftApplicationCredentialSecret = "test-secret"

		// Add other required secrets
		addDefaultSecrets(&config)

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Configuration with both S3 and Swift backends should be invalid")
		}

		foundError := false
		for _, err := range result.Errors {
			if strings.Contains(err.Message, "Cannot configure both S3 and Swift") {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about mixed storage backends, but got: %v", result.Errors)
		}
	})

	t.Run("DetectLokiStorageTypeMismatch", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Services["loki"] = ServiceCfg{
			Enabled:         true,
			LokiStorageType: "swift", // Set to swift
			LokiBucketName:  "loki-logs",
			// But only configure S3
			LokiS3Region:   "us-east-1",
			LokiS3Endpoint: "https://s3.amazonaws.com",
		}
		config.Secrets.Loki.S3AccessKeyID = "AKIA..."
		config.Secrets.Loki.S3SecretAccessKey = "secret"

		// Add other required secrets
		addDefaultSecrets(&config)

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Configuration with storage type mismatch should be invalid")
		}

		foundError := false
		for _, err := range result.Errors {
			if strings.Contains(err.Message, "only S3 configuration is present") {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about storage type mismatch, but got: %v", result.Errors)
		}
	})

	t.Run("DetectMissingKeycloakSecrets", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Services["keycloak"] = ServiceCfg{
			Enabled: true,
		}
		// Don't set keycloak secrets

		// Add other required secrets
		addDefaultSecrets(&config)
		// Clear keycloak secrets to test validation
		config.Secrets.Keycloak.AdminPassword = ""

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Configuration with missing keycloak secrets should be invalid")
		}

		foundError := false
		for _, err := range result.Errors {
			if err.Field == "secrets.keycloak.admin_password" {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about missing keycloak secrets, but got: %v", result.Errors)
		}
	})

	t.Run("DetectMissingGrafanaSecrets", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Services["kube-prometheus-stack"] = ServiceCfg{
			Enabled: true,
		}
		// Don't set grafana secrets

		// Add other required secrets
		addDefaultSecrets(&config)
		// Clear grafana secrets to test validation
		config.Secrets.Grafana.AdminPassword = ""

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Configuration with missing grafana secrets should be invalid")
		}

		foundError := false
		for _, err := range result.Errors {
			if err.Field == "secrets.grafana.admin_password" {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about missing grafana secrets, but got: %v", result.Errors)
		}
	})

	t.Run("DetectMissingWeaveGitOpsSecrets", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Services["weave-gitops"] = ServiceCfg{
			Enabled: true,
		}
		// Don't set weave-gitops secrets

		// Add other required secrets
		addDefaultSecrets(&config)
		// Clear weave-gitops secrets to test validation
		config.Secrets.WeaveGitOps.PasswordHash = ""

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Configuration with missing weave-gitops secrets should be invalid")
		}

		foundError := false
		for _, err := range result.Errors {
			if err.Field == "secrets.weave_gitops.password_hash" {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about missing weave-gitops secrets, but got: %v", result.Errors)
		}
	})

	t.Run("DetectMissingHeadlampSecrets", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Services["headlamp"] = ServiceCfg{
			Enabled: true,
		}
		// Don't set headlamp secrets

		// Add other required secrets
		addDefaultSecrets(&config)
		// Clear headlamp secrets to test validation
		config.Secrets.Headlamp.OIDCClientSecret = ""

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Configuration with missing headlamp secrets should be invalid")
		}

		foundError := false
		for _, err := range result.Errors {
			if err.Field == "secrets.headlamp.oidc_client_secret" {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about missing headlamp secrets, but got: %v", result.Errors)
		}
	})

	t.Run("DetectMissingAlertProxySecrets", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.ManagedService["alert-proxy"] = ServiceCfg{
			Enabled: true,
		}
		// Don't set alert-proxy secrets

		// Add other required secrets
		addDefaultSecrets(&config)
		// Clear alert-proxy secrets to test validation
		config.Secrets.AlertProxy.CoreDeviceId = ""
		config.Secrets.AlertProxy.AccountServiceToken = ""
		config.Secrets.AlertProxy.CoreAccountNumber = ""

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Configuration with missing alert-proxy secrets should be invalid")
		}

		foundError := false
		for _, err := range result.Errors {
			if err.Field == "secrets.alert_proxy.core_device_id" ||
				err.Field == "secrets.alert_proxy.account_service_token" ||
				err.Field == "secrets.alert_proxy.core_account_number" {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about missing alert-proxy secrets, but got: %v", result.Errors)
		}
	})

	t.Run("AcceptValidConfiguration", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Cluster.AdminEmail = "admin@example.com"
		config.OpenCenter.Cluster.ClusterFQDN = "test.example.com"
		config.OpenCenter.Cluster.BaseDomain = "example.com"

		// Add all required secrets
		addDefaultSecrets(&config)

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if !result.Valid {
			t.Errorf("Valid configuration should pass validation, but got errors: %v", result.Errors)
		}

		if len(result.Errors) > 0 {
			t.Errorf("Expected no validation errors, but got: %v", result.Errors)
		}
	})
}

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
	"testing"
	"time"
)

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
		config.Secrets.CertManager.AWSAccessKey = "test-access-key"
		config.Secrets.CertManager.AWSSecretAccessKey = "test-secret-key"
		config.Secrets.Keycloak.AdminPassword = "test-admin-password"
		config.Secrets.Grafana.AdminPassword = "test-grafana-password"
		config.Secrets.WeaveGitOps.PasswordHash = "test-password-hash"

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
		config.Secrets.CertManager.AWSAccessKey = "test-access-key"
		config.Secrets.CertManager.AWSSecretAccessKey = "test-secret-key"
		config.Secrets.Keycloak.AdminPassword = "test-admin-password"
		config.Secrets.Grafana.AdminPassword = "test-grafana-password"
		config.Secrets.WeaveGitOps.PasswordHash = "test-password-hash"

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
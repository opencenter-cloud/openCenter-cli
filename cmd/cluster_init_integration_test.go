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

package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/cluster"
	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation/validators"
	"github.com/rackerlabs/opencenter-cli/internal/di"
	"gopkg.in/yaml.v3"
)

// TestClusterInitIntegration tests the full cluster init workflow
func TestClusterInitIntegration(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	tests := []struct {
		name         string
		clusterName  string
		organization string
		provider     string
		noKeyGen     bool
		expectError  bool
	}{
		{
			name:         "basic cluster init",
			clusterName:  "test-cluster",
			organization: "opencenter",
			provider:     "openstack",
			noKeyGen:     false,
			expectError:  false,
		},
		{
			name:         "cluster init with custom organization",
			clusterName:  "dev-cluster",
			organization: "dev-team",
			provider:     "aws",
			noKeyGen:     false,
			expectError:  false,
		},
		{
			name:         "cluster init without key generation",
			clusterName:  "no-keys-cluster",
			organization: "opencenter",
			provider:     "openstack",
			noKeyGen:     true,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create command
			cmd := newClusterInitCmd()

			// Set up command output buffers
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)

			// Set command arguments
			args := []string{tt.clusterName}
			if tt.organization != "" {
				args = append(args, "--org", tt.organization)
			}
			if tt.provider != "" {
				args = append(args, "--type", tt.provider)
			}
			if tt.noKeyGen {
				args = append(args, "--no-keygen")
			}

			cmd.SetArgs(args)

			// Execute command
			err := cmd.Execute()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v\nstderr: %s", err, stderr.String())
				return
			}

			// Verify output
			output := stdout.String()
			if !strings.Contains(output, "Created cluster configuration") {
				t.Errorf("expected success message in output, got: %s", output)
			}

			// Verify directory structure was created
			expectedOrg := tt.organization
			if expectedOrg == "" {
				expectedOrg = "opencenter"
			}

			orgDir := filepath.Join(dir, "clusters", expectedOrg)
			if _, err := os.Stat(orgDir); os.IsNotExist(err) {
				t.Errorf("organization directory not created: %s", orgDir)
			}

			clusterDir := filepath.Join(orgDir, tt.clusterName)
			if _, err := os.Stat(clusterDir); os.IsNotExist(err) {
				t.Errorf("cluster directory not created: %s", clusterDir)
			}

			// Verify config file was created
			configPath := filepath.Join(orgDir, "."+tt.clusterName+"-config.yaml")
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				t.Errorf("config file not created: %s", configPath)
			}

			// Verify config content
			data, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("failed to read config file: %v", err)
			}

			var cfg config.Config
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatalf("failed to parse config file: %v", err)
			}

			if cfg.OpenCenter.Cluster.ClusterName != tt.clusterName {
				t.Errorf("expected cluster name %q, got %q", tt.clusterName, cfg.OpenCenter.Cluster.ClusterName)
			}

			if cfg.OpenCenter.Meta.Organization != expectedOrg {
				t.Errorf("expected organization %q, got %q", expectedOrg, cfg.OpenCenter.Meta.Organization)
			}

			if tt.provider != "" && cfg.OpenCenter.Infrastructure.Provider != tt.provider {
				t.Errorf("expected provider %q, got %q", tt.provider, cfg.OpenCenter.Infrastructure.Provider)
			}

			// Verify keys were generated (or not)
			if !tt.noKeyGen {
				secretsDir := filepath.Join(orgDir, "secrets")
				sopsKeyPath := filepath.Join(secretsDir, "age", "keys", tt.clusterName+"-key.txt")
				if _, err := os.Stat(sopsKeyPath); os.IsNotExist(err) {
					t.Errorf("SOPS key not created: %s", sopsKeyPath)
				}

				// SSH keys are in the format: <cluster> (PathResolver uses simple cluster name)
				// The old format was <cluster>-<env>-<region> but PathResolver simplified this
				sshKeyPath := filepath.Join(secretsDir, "ssh", tt.clusterName)
				if _, err := os.Stat(sshKeyPath); os.IsNotExist(err) {
					t.Errorf("SSH key not created: %s", sshKeyPath)
				}
			}
		})
	}
}

// TestClusterInitWithDIContainer tests that the DI container is properly set up
func TestClusterInitWithDIContainer(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	// Create DI container
	container := di.NewContainer()
	if err := setupContainer(container); err != nil {
		t.Fatalf("failed to setup container: %v", err)
	}

	// Verify all services can be resolved
	var pathResolver *paths.PathResolver
	if err := container.ResolveAs("path-resolver", &pathResolver); err != nil {
		t.Errorf("failed to resolve path-resolver: %v", err)
	}
	if pathResolver == nil {
		t.Error("path-resolver is nil")
	}

	var configManager *config.ConfigManager
	if err := container.ResolveAs("config-manager", &configManager); err != nil {
		t.Errorf("failed to resolve config-manager: %v", err)
	}
	if configManager == nil {
		t.Error("config-manager is nil")
	}

	var validationEngine *validation.ValidationEngine
	if err := container.ResolveAs("validation-engine", &validationEngine); err != nil {
		t.Errorf("failed to resolve validation-engine: %v", err)
	}
	if validationEngine == nil {
		t.Error("validation-engine is nil")
	}

	var initService *cluster.InitService
	if err := container.ResolveAs("init-service", &initService); err != nil {
		t.Errorf("failed to resolve init-service: %v", err)
	}
	if initService == nil {
		t.Error("init-service is nil")
	}
}

// TestClusterInitServiceIntegration tests the InitService directly
func TestClusterInitServiceIntegration(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	// Create dependencies
	pathResolver := paths.NewPathResolver(dir)
	validationEngine := validation.NewValidationEngine()

	// Register validators
	if err := validationEngine.Register(validators.NewClusterNameValidator()); err != nil {
		t.Fatalf("failed to register cluster name validator: %v", err)
	}

	configManager, err := config.NewConfigManager("")
	if err != nil {
		t.Fatalf("failed to create config manager: %v", err)
	}

	// Create InitService
	initService := cluster.NewInitService(pathResolver, validationEngine, configManager)

	// Test initialization
	opts := cluster.InitOptions{
		ClusterName:  "test-cluster",
		Organization: "test-org",
		Provider:     "openstack",
		NoKeyGen:     true, // Skip key generation for faster test
		NoGitInit:    true, // Skip git init for faster test
	}

	result, err := initService.Initialize(context.Background(), opts)
	if err != nil {
		t.Fatalf("initialization failed: %v", err)
	}

	// Verify result
	if result.Config == nil {
		t.Error("result config is nil")
	}
	if result.ClusterPaths == nil {
		t.Error("result cluster paths is nil")
	}
	if result.ConfigPath == "" {
		t.Error("result config path is empty")
	}

	// Verify config values
	if result.Config.OpenCenter.Cluster.ClusterName != opts.ClusterName {
		t.Errorf("expected cluster name %q, got %q", opts.ClusterName, result.Config.OpenCenter.Cluster.ClusterName)
	}
	if result.Config.OpenCenter.Meta.Organization != opts.Organization {
		t.Errorf("expected organization %q, got %q", opts.Organization, result.Config.OpenCenter.Meta.Organization)
	}
	if result.Config.OpenCenter.Infrastructure.Provider != opts.Provider {
		t.Errorf("expected provider %q, got %q", opts.Provider, result.Config.OpenCenter.Infrastructure.Provider)
	}

	// Verify directories were created
	if _, err := os.Stat(result.ClusterPaths.ClusterDir); os.IsNotExist(err) {
		t.Errorf("cluster directory not created: %s", result.ClusterPaths.ClusterDir)
	}
	if _, err := os.Stat(result.ClusterPaths.SecretsDir); os.IsNotExist(err) {
		t.Errorf("secrets directory not created: %s", result.ClusterPaths.SecretsDir)
	}
}

// TestClusterInitForceOverwrite tests the --force flag
func TestClusterInitForceOverwrite(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	clusterName := "test-cluster"
	organization := "opencenter"

	// Create command
	cmd := newClusterInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	// First initialization
	cmd.SetArgs([]string{clusterName, "--org", organization, "--no-keygen"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("first init failed: %v", err)
	}

	// Reset buffers
	stdout.Reset()
	stderr.Reset()

	// Try to init again without --force (should fail)
	cmd = newClusterInitCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{clusterName, "--org", organization, "--no-keygen"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when initializing existing cluster without --force")
	}

	// Reset buffers
	stdout.Reset()
	stderr.Reset()

	// Try to init again with --force (should succeed)
	cmd = newClusterInitCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{clusterName, "--org", organization, "--no-keygen", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("init with --force failed: %v", err)
	}
}

// TestClusterInitStrictValidation tests the --strict flag
func TestClusterInitStrictValidation(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	// Create command
	cmd := newClusterInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	// Initialize with --strict flag
	cmd.SetArgs([]string{"test-cluster", "--strict", "--no-keygen"})
	err := cmd.Execute()

	// With strict validation, the command should validate the config
	// The result depends on whether the default config passes validation
	// For now, we just verify the command runs
	if err != nil {
		// If there's an error, it should be a validation error
		if !strings.Contains(err.Error(), "validation") {
			t.Errorf("expected validation error with --strict, got: %v", err)
		}
	}
}

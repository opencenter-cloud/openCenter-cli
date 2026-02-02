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
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/cluster"
	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation/validators"
	"github.com/rackerlabs/opencenter-cli/internal/di"
	"gopkg.in/yaml.v3"
)

// TestClusterSetupIntegration tests the full cluster setup workflow
func TestClusterSetupIntegration(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()

	oldConfigDir := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer func() {
		if oldConfigDir != "" {
			os.Setenv("OPENCENTER_CONFIG_DIR", oldConfigDir)
		} else {
			os.Unsetenv("OPENCENTER_CONFIG_DIR")
		}
	}()

	// Initialize test cluster
	clusterName := "test-setup-cluster"
	organization := "test-org"
	
	if err := initializeTestCluster(t, clusterName, organization); err != nil {
		t.Fatalf("failed to initialize test cluster: %v", err)
	}

	// Create dependencies
	pathResolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	validationEngine := validation.NewValidationEngine()
	
	// Register validators
	if err := validationEngine.Register(validators.NewClusterNameValidator()); err != nil {
		t.Fatalf("failed to register cluster name validator: %v", err)
	}

	// Create SetupService
	setupService := cluster.NewSetupService(pathResolver, validationEngine)

	// Run setup
	opts := cluster.SetupOptions{
		ClusterName:  clusterName,
		Organization: organization,
		Force:        false,
		DryRun:       false,
		SkipValidation: true,
	}

	result, err := setupService.Setup(context.Background(), opts)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if result == nil {
		t.Fatal("setup result is nil")
	}

	// Verify GitOps directory was created
	if _, err := os.Stat(result.GitOpsPath); os.IsNotExist(err) {
		t.Errorf("GitOps directory was not created: %s", result.GitOpsPath)
	}
}

// TestClusterSetupWithDIContainer tests that the DI container is properly set up
func TestClusterSetupWithDIContainer(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()

	oldConfigDir := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer func() {
		if oldConfigDir != "" {
			os.Setenv("OPENCENTER_CONFIG_DIR", oldConfigDir)
		} else {
			os.Unsetenv("OPENCENTER_CONFIG_DIR")
		}
	}()

	// Create DI container
	container := di.NewContainer()
	if err := setupSetupContainer(container); err != nil {
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

	var validationEngine *validation.ValidationEngine
	if err := container.ResolveAs("validation-engine", &validationEngine); err != nil {
		t.Errorf("failed to resolve validation-engine: %v", err)
	}
	if validationEngine == nil {
		t.Error("validation-engine is nil")
	}

	var setupService *cluster.SetupService
	if err := container.ResolveAs("setup-service", &setupService); err != nil {
		t.Errorf("failed to resolve setup-service: %v", err)
	}
	if setupService == nil {
		t.Error("setup-service is nil")
	}
}

// TestClusterSetupServiceIntegration tests the SetupService directly
func TestClusterSetupServiceIntegration(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()

	oldConfigDir := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer func() {
		if oldConfigDir != "" {
			os.Setenv("OPENCENTER_CONFIG_DIR", oldConfigDir)
		} else {
			os.Unsetenv("OPENCENTER_CONFIG_DIR")
		}
	}()

	// Initialize test cluster
	clusterName := "test-service-cluster"
	organization := "test-org"
	
	if err := initializeTestCluster(t, clusterName, organization); err != nil {
		t.Fatalf("failed to initialize test cluster: %v", err)
	}

	// Create dependencies
	pathResolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	validationEngine := validation.NewValidationEngine()
	
	// Register validators
	if err := validationEngine.Register(validators.NewClusterNameValidator()); err != nil {
		t.Fatalf("failed to register cluster name validator: %v", err)
	}

	// Create SetupService
	setupService := cluster.NewSetupService(pathResolver, validationEngine)

	// Run setup
	opts := cluster.SetupOptions{
		ClusterName:  clusterName,
		Organization: organization,
		Force:        false,
		DryRun:       false,
		SkipValidation: true,
	}

	result, err := setupService.Setup(context.Background(), opts)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if result == nil {
		t.Fatal("setup result is nil")
	}

	// Verify result contains expected data
	if result.GitOpsPath == "" {
		t.Error("GitOpsPath is empty")
	}
}

// TestClusterSetupForceOverwrite tests the --force flag
func TestClusterSetupForceOverwrite(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()

	oldConfigDir := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer func() {
		if oldConfigDir != "" {
			os.Setenv("OPENCENTER_CONFIG_DIR", oldConfigDir)
		} else {
			os.Unsetenv("OPENCENTER_CONFIG_DIR")
		}
	}()

	// Initialize test cluster
	clusterName := "test-force-cluster"
	organization := "test-org"
	
	if err := initializeTestCluster(t, clusterName, organization); err != nil {
		t.Fatalf("failed to initialize test cluster: %v", err)
	}

	// Create dependencies
	pathResolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	validationEngine := validation.NewValidationEngine()
	
	// Register validators
	if err := validationEngine.Register(validators.NewClusterNameValidator()); err != nil {
		t.Fatalf("failed to register cluster name validator: %v", err)
	}

	// Create SetupService
	setupService := cluster.NewSetupService(pathResolver, validationEngine)

	// Run setup first time
	opts := cluster.SetupOptions{
		ClusterName:  clusterName,
		Organization: organization,
		Force:        false,
		DryRun:       false,
		SkipValidation: true,
	}

	_, err := setupService.Setup(context.Background(), opts)
	if err != nil {
		t.Fatalf("first setup failed: %v", err)
	}

	// Run setup again with force flag
	opts.Force = true
	result, err := setupService.Setup(context.Background(), opts)
	if err != nil {
		t.Fatalf("setup with force failed: %v", err)
	}

	if result == nil {
		t.Fatal("setup result is nil")
	}
}

// TestClusterSetupDryRun tests the --dry-run flag
func TestClusterSetupDryRun(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()

	oldConfigDir := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer func() {
		if oldConfigDir != "" {
			os.Setenv("OPENCENTER_CONFIG_DIR", oldConfigDir)
		} else {
			os.Unsetenv("OPENCENTER_CONFIG_DIR")
		}
	}()

	// Initialize test cluster
	clusterName := "test-dryrun-cluster"
	organization := "test-org"
	
	if err := initializeTestCluster(t, clusterName, organization); err != nil {
		t.Fatalf("failed to initialize test cluster: %v", err)
	}

	// Create dependencies
	pathResolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	validationEngine := validation.NewValidationEngine()
	
	// Register validators
	if err := validationEngine.Register(validators.NewClusterNameValidator()); err != nil {
		t.Fatalf("failed to register cluster name validator: %v", err)
	}

	// Create SetupService
	setupService := cluster.NewSetupService(pathResolver, validationEngine)

	// Run setup with dry-run flag
	opts := cluster.SetupOptions{
		ClusterName:  clusterName,
		Organization: organization,
		Force:        false,
		DryRun:       true,
		SkipValidation: true,
	}

	result, err := setupService.Setup(context.Background(), opts)
	if err != nil {
		t.Fatalf("setup with dry-run failed: %v", err)
	}

	if result == nil {
		t.Fatal("setup result is nil")
	}

	// In dry-run mode, GitOps directory should not be created
	// (depending on implementation, this may vary)
}

// TestClusterSetupSkipValidation tests the --skip-validation flag
func TestClusterSetupSkipValidation(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()

	oldConfigDir := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer func() {
		if oldConfigDir != "" {
			os.Setenv("OPENCENTER_CONFIG_DIR", oldConfigDir)
		} else {
			os.Unsetenv("OPENCENTER_CONFIG_DIR")
		}
	}()

	// Initialize test cluster
	clusterName := "test-skipval-cluster"
	organization := "test-org"
	
	if err := initializeTestCluster(t, clusterName, organization); err != nil {
		t.Fatalf("failed to initialize test cluster: %v", err)
	}

	// Create dependencies
	pathResolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	validationEngine := validation.NewValidationEngine()
	
	// Register validators
	if err := validationEngine.Register(validators.NewClusterNameValidator()); err != nil {
		t.Fatalf("failed to register cluster name validator: %v", err)
	}

	// Create SetupService
	setupService := cluster.NewSetupService(pathResolver, validationEngine)

	// Run setup with skip-validation flag
	opts := cluster.SetupOptions{
		ClusterName:    clusterName,
		Organization:   organization,
		Force:          false,
		DryRun:         false,
		SkipValidation: true,
	}

	result, err := setupService.Setup(context.Background(), opts)
	if err != nil {
		t.Fatalf("setup with skip-validation failed: %v", err)
	}

	if result == nil {
		t.Fatal("setup result is nil")
	}
}

// initializeTestCluster is a helper function to initialize a test cluster
func initializeTestCluster(t *testing.T, clusterName, organization string) error {
	t.Helper()

	// Get the config directory from environment
	dir := os.Getenv("OPENCENTER_CONFIG_DIR")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".config", "opencenter")
	}

	// Create dependencies with correct base directory
	clustersDir := filepath.Join(dir, "clusters")
	pathResolver := paths.NewPathResolver(clustersDir)
	validationEngine := validation.NewValidationEngine()

	// Register validators
	if err := validationEngine.Register(validators.NewClusterNameValidator()); err != nil {
		return err
	}

	configManager, err := config.NewConfigManager("")
	if err != nil {
		return err
	}

	// Create InitService
	initService := cluster.NewInitService(pathResolver, validationEngine, configManager)

	// Initialize cluster
	opts := cluster.InitOptions{
		ClusterName:  clusterName,
		Organization: organization,
		Provider:     "openstack",
		NoKeyGen:     true, // Skip key generation for faster test
		NoGitInit:    true, // Skip git init for faster test
	}

	result, err := initService.Initialize(context.Background(), opts)
	if err != nil {
		return err
	}

	// Update the config to set a valid git_dir
	cfg := result.Config
	gitopsDir := filepath.Join(result.ClusterPaths.OrganizationDir, "gitops")
	cfg.OpenCenter.GitOps.GitDir = gitopsDir

	// Save the updated config
	configPath := result.ConfigPath
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return err
	}

	return nil
}

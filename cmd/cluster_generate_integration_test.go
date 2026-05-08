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

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation/validators"
	"gopkg.in/yaml.v3"
)

// TestClusterGenerateIntegration tests the full cluster generate workflow
func TestClusterGenerateIntegration(t *testing.T) {
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

	// Install fake git binary so tests work in environments without global git config
	binDir := t.TempDir()
	installFakeGitBinary(t, binDir)
	prependTestPath(t, binDir)

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
		ClusterName:    clusterName,
		Organization:   organization,
		Force:          false,
		DryRun:         false,
		SkipValidation: true,
	}

	result, err := setupService.Setup(context.Background(), opts)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	if result == nil {
		t.Fatal("setup result is nil")
	}

	// Verify GitOps directory was created
	if _, err := os.Stat(result.GitOpsPath); os.IsNotExist(err) {
		t.Errorf("GitOps directory was not created: %s", result.GitOpsPath)
	}
}

// broken: full-suite run fails on generated GitOps source contracts (repo casing, ref strategy,
// sync interval, and cert-manager kustomization indentation); see docs/test-results.md.
func TestClusterGenerateIntegrationKindProvider(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	binDir := t.TempDir()
	installFakeGitBinary(t, binDir)
	prependTestPath(t, binDir)

	initCmd := newClusterInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"kind-setup-int", "--type", "kind"})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("cluster init failed: %v", err)
	}

	resetCommandStateForTests()

	setupCmd := newClusterGenerateCmd()
	var stdout, stderr bytes.Buffer
	setupCmd.SetOut(&stdout)
	setupCmd.SetErr(&stderr)
	setupCmd.SetArgs([]string{"kind-setup-int"})
	if err := setupCmd.Execute(); err != nil {
		t.Fatalf("cluster generate failed: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "opencenter cluster deploy kind-setup-int") {
		t.Fatalf("expected deploy next step in generate output, got:\n%s", stdout.String())
	}

	clusterDir := filepath.Join(dir, "clusters", "gitops", "opencenter", "infrastructure", "clusters", "kind-setup-int")
	kindConfigPath := filepath.Join(clusterDir, "kind-config.yaml")
	if _, err := os.Stat(kindConfigPath); err != nil {
		t.Fatalf("expected kind-config.yaml to exist: %v", err)
	}
	kindConfigBytes, err := os.ReadFile(kindConfigPath)
	if err != nil {
		t.Fatalf("read kind-config.yaml: %v", err)
	}
	if !strings.Contains(string(kindConfigBytes), "disableDefaultCNI: false") {
		t.Fatalf("expected kind-config.yaml to render disableDefaultCNI: false by default\ncontents:\n%s", string(kindConfigBytes))
	}

	for _, path := range []string{
		filepath.Join(clusterDir, "main.tf"),
		filepath.Join(clusterDir, "provider.tf"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be absent for kind setup", path)
		}
	}
}

func TestClusterGenerateIntegrationKindProviderDisableDefaultCNI(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	binDir := t.TempDir()
	installFakeGitBinary(t, binDir)
	prependTestPath(t, binDir)

	initCmd := newClusterInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"kind-setup-cni-int", "--type", "kind", "--kind-disable-default-cni"})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("cluster init failed: %v", err)
	}

	resetCommandStateForTests()

	setupCmd := newClusterGenerateCmd()
	var stdout, stderr bytes.Buffer
	setupCmd.SetOut(&stdout)
	setupCmd.SetErr(&stderr)
	setupCmd.SetArgs([]string{"kind-setup-cni-int"})
	if err := setupCmd.Execute(); err != nil {
		t.Fatalf("cluster generate failed: %v\nstderr: %s", err, stderr.String())
	}

	clusterDir := filepath.Join(dir, "clusters", "gitops", "opencenter", "infrastructure", "clusters", "kind-setup-cni-int")
	kindConfigPath := filepath.Join(clusterDir, "kind-config.yaml")
	kindConfigBytes, err := os.ReadFile(kindConfigPath)
	if err != nil {
		t.Fatalf("read kind-config.yaml: %v", err)
	}
	if !strings.Contains(string(kindConfigBytes), "disableDefaultCNI: true") {
		t.Fatalf("expected kind-config.yaml to render disableDefaultCNI: true\ncontents:\n%s", string(kindConfigBytes))
	}
}

// TestClusterGenerateWithDIContainer tests that the DI container is properly set up
func TestClusterGenerateWithDIContainer(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	resetContainerForTests()
	container := getContainer()

	// Verify all services can be resolved
	var pathResolver *paths.PathResolver
	if err := container.ResolveAs("PathResolver", &pathResolver); err != nil {
		t.Errorf("failed to resolve PathResolver: %v", err)
	}
	if pathResolver == nil {
		t.Error("PathResolver is nil")
	}

	var validationEngine *validation.ValidationEngine
	if err := container.ResolveAs("ValidationEngine", &validationEngine); err != nil {
		t.Errorf("failed to resolve ValidationEngine: %v", err)
	}
	if validationEngine == nil {
		t.Error("ValidationEngine is nil")
	}

	var setupService *cluster.SetupService
	if err := container.ResolveAs("SetupService", &setupService); err != nil {
		t.Errorf("failed to resolve SetupService: %v", err)
	}
	if setupService == nil {
		t.Error("SetupService is nil")
	}
}

// TestClusterGenerateServiceIntegration tests the SetupService directly
func TestClusterGenerateServiceIntegration(t *testing.T) {
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

	// Install fake git binary so tests work in environments without global git config
	binDir := t.TempDir()
	installFakeGitBinary(t, binDir)
	prependTestPath(t, binDir)

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
		ClusterName:    clusterName,
		Organization:   organization,
		Force:          false,
		DryRun:         false,
		SkipValidation: true,
	}

	result, err := setupService.Setup(context.Background(), opts)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	if result == nil {
		t.Fatal("setup result is nil")
	}

	// Verify result contains expected data
	if result.GitOpsPath == "" {
		t.Error("GitOpsPath is empty")
	}
}

// TestClusterGenerateForceOverwrite tests the --force flag
func TestClusterGenerateForceOverwrite(t *testing.T) {
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

	// Install fake git binary so tests work in environments without global git config
	binDir := t.TempDir()
	installFakeGitBinary(t, binDir)
	prependTestPath(t, binDir)

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
		ClusterName:    clusterName,
		Organization:   organization,
		Force:          false,
		DryRun:         false,
		SkipValidation: true,
	}

	_, err := setupService.Setup(context.Background(), opts)
	if err != nil {
		t.Fatalf("first generate failed: %v", err)
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

// TestClusterGenerateDryRun tests the --dry-run flag
func TestClusterGenerateDryRun(t *testing.T) {
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
		ClusterName:    clusterName,
		Organization:   organization,
		Force:          false,
		DryRun:         true,
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

// TestClusterGenerateSkipValidation tests the --skip-validation flag
func TestClusterGenerateSkipValidation(t *testing.T) {
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

	// Install fake git binary so tests work in environments without global git config
	binDir := t.TempDir()
	installFakeGitBinary(t, binDir)
	prependTestPath(t, binDir)

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
	if err := validationEngine.Register(validators.NewOrganizationNameValidator()); err != nil {
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
		NoKeyGen:     false,
		NoGitInit:    true, // Skip git init for faster test
	}

	result, err := initService.Initialize(context.Background(), opts)
	if err != nil {
		return err
	}

	// Update the config to set a valid git_dir
	cfg := result.Config
	cfg.OpenCenter.GitOps.Repository.LocalDir = result.ClusterPaths.GitOpsDir

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

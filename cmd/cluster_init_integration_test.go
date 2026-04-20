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
	"github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation/validators"
	"github.com/opencenter-cloud/opencenter-cli/internal/di"
)

// TestClusterInitIntegration tests the full cluster init workflow
func TestClusterInitIntegration(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

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
			provider:     "kind",
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

			clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", tt.clusterName)
			if _, err := os.Stat(clusterDir); os.IsNotExist(err) {
				t.Errorf("cluster directory not created: %s", clusterDir)
			}

			// Verify config file was created
			configPath := filepath.Join(orgDir, "."+tt.clusterName+"-config.yaml")
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				t.Errorf("config file not created: %s", configPath)
			}

			// Verify config content
			cfg := loadV2ConfigForTest(t, configPath)

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

				sshKeyPath := filepath.Join(secretsDir, "ssh", tt.clusterName)
				if _, err := os.Stat(sshKeyPath); os.IsNotExist(err) {
					t.Errorf("SSH key not created: %s", sshKeyPath)
				}
			}
		})
	}
}

func TestClusterInitIntegrationKindProvider(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cmd := newClusterInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"kind-int", "--type", "kind", "--org", "opencenter", "--no-keygen"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster init failed: %v\nstderr: %s", err, stderr.String())
	}

	configPath := filepath.Join(dir, "clusters", "opencenter", ".kind-int-config.yaml")
	cfg := loadV2ConfigForTest(t, configPath)

	if cfg.OpenCenter.Infrastructure.Provider != "kind" {
		t.Fatalf("expected provider kind, got %q", cfg.OpenCenter.Infrastructure.Provider)
	}
	if cfg.OpenTofu.Enabled {
		t.Fatal("expected opentofu to be disabled for kind")
	}
	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack != nil {
		t.Fatalf("expected openstack cloud config to be cleared, got %#v", cfg.OpenCenter.Infrastructure.Cloud.OpenStack)
	}
	if cfg.OpenCenter.Infrastructure.Cloud.AWS != nil {
		t.Fatalf("expected aws cloud config to be cleared, got %#v", cfg.OpenCenter.Infrastructure.Cloud.AWS)
	}
	if cfg.OpenCenter.Infrastructure.Cloud.VMware != nil {
		t.Fatalf("expected vmware cloud config to be cleared, got %#v", cfg.OpenCenter.Infrastructure.Cloud.VMware)
	}

	canonicalCfg, err := loadCanonicalConfig("kind-int")
	if err != nil {
		t.Fatalf("load canonical config: %v", err)
	}
	if canonicalCfg.OpenCenter.Infrastructure.Kind == nil {
		t.Fatal("expected kind infrastructure config to be present")
	}
	if canonicalCfg.OpenCenter.Infrastructure.Kind.DisableDefaultCNI {
		t.Fatal("expected disable_default_cni to default to false for kind")
	}
	if cfg.OpenCenter.Meta.Stage != config.StageInit || cfg.OpenCenter.Meta.Status != config.StatusSuccess {
		t.Fatalf("unexpected lifecycle state: %s/%s", cfg.OpenCenter.Meta.Stage, cfg.OpenCenter.Meta.Status)
	}
}

func TestClusterInitKindDisableDefaultCNIFlag(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cmd := newClusterInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"kind-cni-int", "--type", "kind", "--org", "opencenter", "--no-keygen", "--kind-disable-default-cni"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster init failed: %v\nstderr: %s", err, stderr.String())
	}

	configPath := filepath.Join(dir, "clusters", "opencenter", ".kind-cni-int-config.yaml")
	v2Cfg := loadV2ConfigForTest(t, configPath)
	if v2Cfg.OpenCenter.Infrastructure.Kind == nil {
		t.Fatal("expected native v2 kind compatibility config to be present")
	}
	if !v2Cfg.OpenCenter.Infrastructure.Kind.DisableDefaultCNI {
		t.Fatal("expected native v2 config to persist disable_default_cni=true")
	}

	cfg, err := loadCanonicalConfig("kind-cni-int")
	if err != nil {
		t.Fatalf("load canonical config: %v", err)
	}

	if cfg.OpenCenter.Infrastructure.Kind == nil {
		t.Fatal("expected kind infrastructure config to be present")
	}
	if !cfg.OpenCenter.Infrastructure.Kind.DisableDefaultCNI {
		t.Fatal("expected disable_default_cni to be true when --kind-disable-default-cni is set")
	}
}

func TestClusterInitRejectsKindDisableDefaultCNIForNonKind(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cmd := newClusterInitCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"openstack-cni-int", "--type", "openstack", "--no-keygen", "--kind-disable-default-cni"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected cluster init to reject --kind-disable-default-cni for non-kind provider")
	}
	if !strings.Contains(err.Error(), "--kind-disable-default-cni is only valid for kind clusters") {
		t.Fatalf("expected kind-only error, got: %v", err)
	}
}

func TestClusterInitSupportsDottedOverrides(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cmd := newClusterInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"flag-init",
		"--opencenter.meta.organization=legacy-org",
		"--opencenter.gitops.repository.local_dir=/opt/opencenter/flag-init",
		"--opencenter.infrastructure.compute.master_count=5",
		"--no-keygen",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster init failed: %v\nstderr: %s", err, stderr.String())
	}

	configPath := filepath.Join(dir, "clusters", "legacy-org", ".flag-init-config.yaml")
	cfg := loadV2ConfigForTest(t, configPath)

	if cfg.OpenCenter.Meta.Organization != "legacy-org" {
		t.Fatalf("expected deprecated organization alias to set legacy-org, got %q", cfg.OpenCenter.Meta.Organization)
	}
	if cfg.OpenCenter.GitOps.Repository.LocalDir != "/opt/opencenter/flag-init" {
		t.Fatalf("expected explicit local_dir to be preserved, got %q", cfg.OpenCenter.GitOps.Repository.LocalDir)
	}
	if cfg.OpenCenter.Infrastructure.Compute.MasterCount != 5 {
		t.Fatalf("expected master_count 5, got %d", cfg.OpenCenter.Infrastructure.Compute.MasterCount)
	}
	if !strings.Contains(stdout.String(), "/opt/opencenter/flag-init") {
		t.Fatalf("expected result message to mention explicit local_dir, got %q", stdout.String())
	}
}

func TestClusterInitRejectsLegacyDottedOverrides(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cmd := newClusterInitCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"legacy-override",
		"--opencenter.cluster.kubernetes.master_count=5",
		"--no-keygen",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected legacy dotted override to fail")
	}
	if !strings.Contains(err.Error(), "field not found") {
		t.Fatalf("expected clear legacy override failure, got: %v", err)
	}
}

func TestClusterInitOrgFlagOverridesDeprecatedAlias(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cmd := newClusterInitCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"org-precedence",
		"--org", "flag-org",
		"--opencenter.meta.organization=legacy-org",
		"--no-keygen",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster init failed: %v\nstderr: %s", err, stderr.String())
	}

	configPath := filepath.Join(dir, "clusters", "flag-org", ".org-precedence-config.yaml")
	cfg := loadV2ConfigForTest(t, configPath)

	if cfg.OpenCenter.Meta.Organization != "flag-org" {
		t.Fatalf("expected --org to win over deprecated alias, got %q", cfg.OpenCenter.Meta.Organization)
	}
}

func TestClusterInitFullSchemaProducesValidV2Template(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cmd := newClusterInitCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"full-one", "--full-schema", "--no-keygen"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster init failed: %v\nstderr: %s", err, stderr.String())
	}

	configPath := filepath.Join(dir, "clusters", "opencenter", ".full-one-config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	if strings.Contains(string(data), "local.") {
		t.Fatalf("expected full-schema config to avoid legacy local examples, got:\n%s", string(data))
	}
	if strings.Contains(string(data), "\niac:") {
		t.Fatalf("expected full-schema config to avoid legacy iac section, got:\n%s", string(data))
	}
	if !strings.Contains(string(data), "--opencenter.infrastructure.compute.master_count=5") {
		t.Fatalf("expected full-schema header to document native v2 dotted paths, got:\n%s", string(data))
	}

	_ = loadV2ConfigForTest(t, configPath)
}

func TestClusterInitNoSOPSKeygenLeavesSOPSPathEmpty(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cmd := newClusterInitCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"no-sops", "--no-sops-keygen"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster init failed: %v\nstderr: %s", err, stderr.String())
	}

	configPath := filepath.Join(dir, "clusters", "opencenter", ".no-sops-config.yaml")
	cfg := loadV2ConfigForTest(t, configPath)

	if cfg.Secrets.SopsAgeKeyFile != "" {
		t.Fatalf("expected empty SOPS key path when key generation is disabled, got %q", cfg.Secrets.SopsAgeKeyFile)
	}
	if cfg.Secrets.SOPSConfig.Enabled {
		t.Fatalf("expected SOPS config to be disabled when key generation is disabled")
	}

	sopsKeyPath := filepath.Join(dir, "clusters", "opencenter", "secrets", "age", "keys", "no-sops-key.txt")
	if _, err := os.Stat(sopsKeyPath); !os.IsNotExist(err) {
		t.Fatalf("expected no SOPS key to be generated at %s", sopsKeyPath)
	}
}

func TestClusterInitThenValidateSucceeds(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	initCmd := newClusterInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"validate-me", "--no-keygen"})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("cluster init failed: %v", err)
	}

	configPath := filepath.Join(dir, "clusters", "opencenter", ".validate-me-config.yaml")

	validateCmd := newClusterValidateCmd()
	var stdout, stderr bytes.Buffer
	validateCmd.SetOut(&stdout)
	validateCmd.SetErr(&stderr)
	validateCmd.SetArgs([]string{"--config", configPath})
	if err := validateCmd.Execute(); err != nil {
		t.Fatalf("cluster validate failed: %v\nstderr: %s", err, stderr.String())
	}
}

// TestClusterInitWithDIContainer tests that the DI container is properly set up
func TestClusterInitWithDIContainer(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

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

func loadV2ConfigForTest(t *testing.T, configPath string) *v2.Config {
	t.Helper()

	loader := v2.NewConfigLoader(defaults.NewRegistry())
	cfg, err := loader.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("load v2 config %s: %v", configPath, err)
	}

	return cfg
}

// TestClusterInitServiceIntegration tests the InitService directly
func TestClusterInitServiceIntegration(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	// Create dependencies
	pathResolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	validationEngine := validation.NewValidationEngine()

	// Register validators
	if err := validationEngine.Register(validators.NewClusterNameValidator()); err != nil {
		t.Fatalf("failed to register cluster name validator: %v", err)
	}
	if err := validationEngine.Register(validators.NewOrganizationNameValidator()); err != nil {
		t.Fatalf("failed to register organization name validator: %v", err)
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
	prepareCommandTestEnv(t, dir)

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
	prepareCommandTestEnv(t, dir)

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

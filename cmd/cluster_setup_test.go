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
	configdefaults "github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation/validators"
)

// TestClusterSetupCommandRegistration verifies that NewClusterCmd() includes the "setup" subcommand.
// Requirements: 2.1
func TestClusterSetupCommandRegistration(t *testing.T) {
	cmd := NewClusterCmd()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "setup" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected 'setup' subcommand to be registered under 'cluster'")
	}
}

// TestClusterSetupCommandStructure verifies the setup command's Use, flags, and args.
// Requirements: 2.1, 2.2
func TestClusterSetupCommandStructure(t *testing.T) {
	cmd := newClusterSetupCmd()

	if cmd.Use != "setup [name]" {
		t.Errorf("expected Use='setup [name]', got %q", cmd.Use)
	}

	// Verify flags exist
	flags := []string{"force", "dry-run", "skip-validation"}
	for _, name := range flags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("expected flag %q to be registered", name)
		}
	}
}

// TestClusterSetupMissingConfig verifies that running setup for a non-existent cluster returns an error.
// Requirements: 2.3
func TestClusterSetupMissingConfig(t *testing.T) {
	cfgDir := t.TempDir()

	oldEnv := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", cfgDir)
	defer func() {
		if oldEnv != "" {
			os.Setenv("OPENCENTER_CONFIG_DIR", oldEnv)
		} else {
			os.Unsetenv("OPENCENTER_CONFIG_DIR")
		}
	}()

	cmd := newClusterSetupCmd()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"nonexistent-cluster"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when cluster config does not exist")
	}
}

// TestClusterSetupNoClusterArg verifies that running setup without an argument and no active cluster returns an error.
// Requirements: 2.2
// broken: full-suite run resolves a load-test cluster path instead of reporting no active cluster;
// see docs/test-results.md.
func TestClusterSetupNoClusterArg(t *testing.T) {
	cfgDir := t.TempDir()
	prepareCommandTestEnv(t, cfgDir)

	cmd := newClusterSetupCmd()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no cluster name and no active cluster")
	}

	if !strings.Contains(err.Error(), "no active cluster") {
		t.Errorf("expected error about no active cluster, got: %v", err)
	}
}

// TestClusterSetupDryRunSkipsCommit verifies that --dry-run produces "Dry run complete" output
// and does not produce a commit hash.
// Requirements: 2.2, 2.3
func TestClusterSetupDryRunSkipsCommit(t *testing.T) {
	cfgDir := t.TempDir()

	oldEnv := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", cfgDir)
	defer func() {
		if oldEnv != "" {
			os.Setenv("OPENCENTER_CONFIG_DIR", oldEnv)
		} else {
			os.Unsetenv("OPENCENTER_CONFIG_DIR")
		}
	}()

	clusterName := "test-dryrun-setup"
	organization := "test-org"
	clustersDir := filepath.Join(cfgDir, "clusters")

	// Create the org-based directory structure expected by PathResolver
	orgDir := filepath.Join(clustersDir, organization)
	infraDir := filepath.Join(orgDir, "infrastructure", "clusters", clusterName)
	if err := os.MkdirAll(infraDir, 0o755); err != nil {
		t.Fatalf("failed to create infrastructure directory: %v", err)
	}

	gitopsDir := filepath.Join(cfgDir, "gitops", clusterName)
	if err := os.MkdirAll(gitopsDir, 0o755); err != nil {
		t.Fatalf("failed to create gitops directory: %v", err)
	}

	// Write config file directly at the expected path
	cfgPtr, err := v2.NewV2Default(clusterName, "openstack")
	if err != nil {
		t.Fatalf("failed to create native v2 config: %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = gitopsDir

	configPath := filepath.Join(orgDir, "."+clusterName+"-config.yaml")
	loader := v2.NewConfigLoader(configdefaults.NewRegistry())
	if err := loader.SaveToFile(&cfg, configPath); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Run SetupService directly with DryRun to verify behavior,
	// since the Cobra command depends on the global DI container.
	pathResolver := paths.NewPathResolver(clustersDir)
	validationEngine := validation.NewValidationEngine()
	if err := validationEngine.Register(validators.NewClusterNameValidator()); err != nil {
		t.Fatalf("failed to register validator: %v", err)
	}

	setupService := cluster.NewSetupService(pathResolver, validationEngine)

	opts := cluster.SetupOptions{
		ClusterName:    clusterName,
		Organization:   organization,
		DryRun:         true,
		SkipValidation: true,
	}

	result, err := setupService.Setup(context.Background(), opts)
	if err != nil {
		t.Fatalf("dry-run setup failed: %v", err)
	}

	// In dry-run mode, CommitHash should be empty (commit is skipped)
	if result.CommitHash != "" {
		t.Errorf("expected empty CommitHash in dry-run mode, got %q", result.CommitHash)
	}

	// ManifestsCreated should be non-zero (estimated count)
	if result.ManifestsCreated == 0 {
		t.Error("expected non-zero ManifestsCreated in dry-run mode")
	}
}

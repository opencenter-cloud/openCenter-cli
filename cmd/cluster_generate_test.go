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
	"github.com/spf13/cobra"
)

// TestClusterGenerateCommandRegistration verifies that NewClusterCmd() includes the "generate" subcommand.
// Requirements: 2.1
func TestClusterGenerateCommandRegistration(t *testing.T) {
	cmd := NewClusterCmd()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "generate" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected 'generate' subcommand to be registered under 'cluster'")
	}
}

// TestClusterGenerateCommandStructure verifies the generate command's Use, flags, and args.
// Requirements: 2.1, 2.2
func TestClusterGenerateCommandStructure(t *testing.T) {
	cmd := newClusterGenerateCmd()

	if cmd.Use != "generate [name]" {
		t.Errorf("expected Use='generate [name]', got %q", cmd.Use)
	}

	// Verify flags exist
	flags := []string{"force", "skip-validation", "render-only"}
	for _, name := range flags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("expected flag %q to be registered", name)
		}
	}
	if cmd.Flags().Lookup("dry-run") != nil {
		t.Error("expected dry-run to be global, not command-local")
	}
}

// TestClusterGenerateMissingConfig verifies that running generate for a non-existent cluster returns an error.
// Requirements: 2.3
func TestClusterGenerateMissingConfig(t *testing.T) {
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

	cmd := newClusterGenerateCmd()
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

// TestClusterGenerateNoClusterArg verifies that running generate without an argument and no active cluster returns an error.
// Requirements: 2.2
// broken: full-suite run resolves a load-test cluster path instead of reporting no active cluster;
// see docs/test-results.md.
func TestClusterGenerateNoClusterArg(t *testing.T) {
	cfgDir := t.TempDir()
	prepareCommandTestEnv(t, cfgDir)

	cmd := newClusterGenerateCmd()
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

func TestClusterMissingActiveClusterErrorShape(t *testing.T) {
	cfgDir := t.TempDir()
	prepareCommandTestEnv(t, cfgDir)

	expected := `no active cluster is set

Fix:
  opencenter cluster list
  opencenter cluster use <org/name>

Or pass a cluster explicitly:
  opencenter cluster validate <org/name>`

	for name, resolve := range map[string]func() (string, error){
		"args": func() (string, error) {
			return resolveClusterName(nil, true)
		},
		"flag": func() (string, error) {
			return resolveClusterNameFromFlag("", true)
		},
	} {
		t.Run(name, func(t *testing.T) {
			gotName, err := resolve()
			if err == nil {
				t.Fatal("expected missing active cluster error")
			}
			if gotName != "" {
				t.Fatalf("expected no resolved cluster name, got %q", gotName)
			}
			if err.Error() != expected {
				t.Fatalf("unexpected missing-active error:\n%s", err.Error())
			}
		})
	}
}

func TestMissingActiveClusterHintUsesInvokedCommand(t *testing.T) {
	cfgDir := t.TempDir()
	prepareCommandTestEnv(t, cfgDir)

	root := &cobra.Command{
		Use:           "opencenter",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return applyGlobalOptions(cmd, args)
		},
	}
	addGlobalFlags(root)
	root.AddCommand(NewClusterCmd())
	root.SetArgs([]string{"cluster", "export"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected missing active cluster error")
	}
	if !strings.Contains(err.Error(), "opencenter cluster export <org/name>") {
		t.Fatalf("expected export-specific missing-active hint, got:\n%s", err.Error())
	}
	if strings.Contains(err.Error(), "opencenter cluster validate <org/name>") {
		t.Fatalf("missing-active hint should not point at validate for export:\n%s", err.Error())
	}
}

// TestClusterGenerateDryRunSkipsCommit verifies that dry-run generation
// and does not produce a commit hash.
// Requirements: 2.2, 2.3
func TestClusterGenerateDryRunSkipsCommit(t *testing.T) {
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

	clusterName := "test-dryrun-generate"
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
		t.Fatalf("dry-run generate failed: %v", err)
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

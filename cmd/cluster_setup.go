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
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rackerlabs/opencenter-cli/internal/cluster"
	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
	"github.com/rackerlabs/opencenter-cli/internal/di"
	"github.com/rackerlabs/opencenter-cli/internal/resilience"
	"github.com/spf13/cobra"
)

// newClusterSetupCmd creates the command for setting up the GitOps repository.
//
// This command initializes the GitOps repository structure for a cluster by:
// - Validating the cluster configuration
// - Checking if the repository is already initialized (unless --force is used)
// - Rendering templates into the GitOps directory
// - Provisioning OpenTofu configuration files
//
// Returns:
//   - *cobra.Command: A pointer to the configured `setup` command.
func newClusterSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup [name]",
		Short: "Set up GitOps repository for a cluster",
		Long: `Set up the GitOps repository structure for a cluster.

This command initializes the GitOps repository by rendering templates and
creating the necessary directory structure. It is idempotent by default,
meaning it will skip setup if the repository is already initialized.

Only v2 configurations (schema_version: "2.0") are supported.
v1 configurations will be rejected with migration instructions.

Use --force to overwrite an existing repository.

The setup process:
1. Validates the cluster configuration
2. Checks if the GitOps directory is already initialized
3. Renders base GitOps structure
4. Renders cluster-specific templates
5. Provisions OpenTofu configuration

Configuration must have opencenter.gitops.git_dir set.`,
		Example: `  # Set up GitOps repository for active cluster
  opencenter cluster setup

  # Set up for a specific cluster
  opencenter cluster setup my-cluster

  # Force overwrite existing repository
  opencenter cluster setup my-cluster --force`,
		Args: cobra.MaximumNArgs(1),
		RunE: runClusterSetup,
	}

	cmd.Flags().Bool("force", false, "Force overwrite existing GitOps repository")
	cmd.Flags().Bool("skip-validation", false, "Skip configuration validation")
	cmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	cmd.Flags().String("org", "", "organization name (defaults to 'opencenter')")

	return cmd
}

// runClusterSetup executes the cluster setup command
func runClusterSetup(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize DI container
	container := di.NewContainer()
	if err := setupSetupContainer(container); err != nil {
		return fmt.Errorf("setting up DI container: %w", err)
	}

	// Resolve SetupService from container
	var setupService *cluster.SetupService
	if err := container.ResolveAs("setup-service", &setupService); err != nil {
		return fmt.Errorf("resolving setup service: %w", err)
	}

	// Parse command-line options
	opts, err := parseSetupOptions(cmd, args)
	if err != nil {
		return err
	}

	// Acquire lock for setup operation
	lockMgr, err := resilience.NewLockManager(resilience.DefaultLockConfig)
	if err != nil {
		return fmt.Errorf("failed to create lock manager: %w", err)
	}

	lock, err := lockMgr.AcquireWithMetadata(ctx, opts.ClusterName, 1*time.Hour, map[string]string{
		"operation": "setup",
		"command":   "cluster setup",
	})
	if err != nil {
		return fmt.Errorf("[E6003] failed to acquire lock: %w", err)
	}
	defer lockMgr.Release(lock)

	// Execute setup
	result, err := setupService.Setup(ctx, opts)
	if err != nil {
		return err
	}

	// Display results
	if opts.DryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Dry run: would create %d manifests at: %s\n", result.ManifestsCreated, result.GitOpsPath)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Created GitOps repo at: %s\n", result.GitOpsPath)
		fmt.Fprintf(cmd.OutOrStdout(), "Generated %d manifests\n", result.ManifestsCreated)
		if result.CommitHash != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Committed changes: %s\n", result.CommitHash)
		}
	}

	return nil
}

// setupSetupContainer initializes the DI container with required services
func setupSetupContainer(container di.Container) error {
	// Get base directory from environment or use default
	baseDir := os.Getenv("OPENCENTER_CONFIG_DIR")
	if baseDir == "" {
		// Use default config directory
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("getting home directory: %w", err)
		}
		baseDir = filepath.Join(home, ".config", "opencenter")
	}
	
	pathResolver, err := di.ProvidePathResolver(baseDir)
	if err != nil {
		return err
	}
	if err := container.Singleton("path-resolver", func() (*paths.PathResolver, error) {
		return pathResolver, nil
	}); err != nil {
		return err
	}
	if err := container.Singleton("validation-engine", di.ProvideValidationEngine); err != nil {
		return err
	}
	if err := container.Singleton("setup-service", di.ProvideSetupService); err != nil {
		return err
	}
	return container.Initialize()
}

// parseSetupOptions parses command-line flags into SetupOptions
func parseSetupOptions(cmd *cobra.Command, args []string) (cluster.SetupOptions, error) {
	opts := cluster.SetupOptions{}

	// Resolve cluster name from args or active cluster
	name, err := resolveClusterName(args, true)
	if err != nil {
		return opts, err
	}
	opts.ClusterName = name

	// Parse flags
	opts.Organization, _ = cmd.Flags().GetString("org")
	opts.Force, _ = cmd.Flags().GetBool("force")
	opts.SkipValidation, _ = cmd.Flags().GetBool("skip-validation")
	opts.DryRun, _ = cmd.Flags().GetBool("dry-run")

	return opts, nil
}

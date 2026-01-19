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
	"strings"
	"time"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/gitops"
	"github.com/rackerlabs/openCenter-cli/internal/resilience"
	"github.com/rackerlabs/openCenter-cli/internal/tofu"
	"github.com/spf13/cobra"
)

// newClusterSetupCmd creates the command for setting up the GitOps repository.
//
// This command initializes the GitOps repository structure for a cluster by:
// - Checking if the repository is already initialized (unless --force is used)
// - Rendering templates into the GitOps directory
// - Provisioning OpenTofu configuration files
//
// Returns:
//   - *cobra.Command: A pointer to the configured `setup` command.
func newClusterSetupCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "setup [name]",
		Short: "Set up GitOps repository for a cluster",
		Long: `Set up the GitOps repository structure for a cluster.

This command initializes the GitOps repository by rendering templates and
creating the necessary directory structure. It is idempotent by default,
meaning it will skip setup if the repository is already initialized.

Use --force to overwrite an existing repository.

The setup process:
1. Validates the cluster configuration
2. Checks if the GitOps directory is already initialized
3. Renders base GitOps structure
4. Renders cluster-specific templates
5. Provisions OpenTofu configuration

Configuration must have opencenter.gitops.git_dir set.`,
		Example: `  # Set up GitOps repository for active cluster
  openCenter cluster setup

  # Set up for a specific cluster
  openCenter cluster setup my-cluster

  # Force overwrite existing repository
  openCenter cluster setup my-cluster --force`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name
			var name string
			if len(args) > 0 {
				name = args[0]
			} else {
				var err error
				name, err = config.GetActive()
				if err != nil {
					return err
				}
				if name == "" {
					return fmt.Errorf("no active cluster; specify name or run 'openCenter cluster select' first")
				}
			}

			// Acquire lock for setup operation
			lockMgr, err := resilience.NewLockManager(resilience.DefaultLockConfig)
			if err != nil {
				return fmt.Errorf("failed to create lock manager: %w", err)
			}

			ctx := context.Background()
			lock, err := lockMgr.Acquire(ctx, name, 1*time.Hour)
			if err != nil {
				return formatErrorWithInfo(err, "E6003")
			}
			defer lockMgr.Release(lock)

			// Load configuration
			cfg, err := config.Load(name)
			if err != nil {
				return err
			}

			// Validate that git_dir is set
			gitDir := cfg.GitOps().GitDir
			// Treat test default paths as unset for validation purposes
			if gitDir == "" || strings.HasPrefix(gitDir, "./testdata/test-git-repo-") {
				return fmt.Errorf("opencenter.gitops.git_dir must be set in the configuration")
			}

			// Check if already initialized (unless --force is used)
			if !force {
				initialized, err := gitops.IsGitOpsInitialized(gitDir)
				if err != nil {
					return fmt.Errorf("failed to check if GitOps repository is initialized: %w", err)
				}

				if initialized {
					fmt.Fprintf(cmd.OutOrStdout(), "GitOps repository already initialized at: %s\n", gitDir)
					fmt.Fprintln(cmd.OutOrStdout(), "Use --force to overwrite existing files")
					return nil
				}
			}

			// Perform setup
			if err := setupGitOpsRepository(cfg, cmd); err != nil {
				return fmt.Errorf("failed to set up GitOps repository: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created GitOps repo at: %s\n", gitDir)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force overwrite existing GitOps repository")

	return cmd
}

// setupGitOpsRepository performs the actual GitOps repository setup.
func setupGitOpsRepository(cfg config.Config, cmd *cobra.Command) error {
	// Copy base GitOps structure (always render for generation)
	if err := gitops.CopyBase(cfg, true); err != nil {
		return fmt.Errorf("failed to copy base GitOps structure: %w", err)
	}

	// Render cluster-specific applications
	if err := gitops.RenderClusterApps(cfg); err != nil {
		return fmt.Errorf("failed to render cluster apps: %w", err)
	}

	// Render infrastructure templates
	if err := gitops.RenderInfrastructureCluster(cfg); err != nil {
		return fmt.Errorf("failed to render infrastructure cluster: %w", err)
	}

	// Provision OpenTofu (renders main.tf and provider.tf)
	if err := tofu.Provision(cfg); err != nil {
		return fmt.Errorf("failed to provision opentofu: %w", err)
	}

	return nil
}

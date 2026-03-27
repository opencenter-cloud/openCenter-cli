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

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/spf13/cobra"
)

// newClusterSetupCmd creates the command for setting up a cluster's GitOps repository.
func newClusterSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup [name]",
		Short: "Generate the customer GitOps repository structure",
		Long: `Generate the customer GitOps repository structure for a cluster.

This command invokes the SetupService to generate infrastructure templates,
FluxCD manifests, and application overlays based on the cluster configuration.
The generated repository follows the openCenter GitOps pattern with
infrastructure/, applications/, and secrets/ directories.

Only v2 configurations (schema_version: "2.0") are supported.
Configurations with any other schema version are invalid.

If no cluster name is provided, the currently active cluster is used.`,
		Example: `  # Set up the active cluster
  opencenter cluster setup

  # Set up a specific cluster
  opencenter cluster setup my-cluster

  # Preview what would be generated
  opencenter cluster setup my-cluster --dry-run

  # Force overwrite existing GitOps repository
  opencenter cluster setup my-cluster --force

  # Skip configuration validation
  opencenter cluster setup my-cluster --skip-validation`,
		Args: cobra.MaximumNArgs(1),
		RunE: runClusterSetup,
	}

	cmd.Flags().Bool("force", false, "overwrite existing GitOps repository")
	cmd.Flags().Bool("dry-run", false, "show what would be generated without writing files")
	cmd.Flags().Bool("skip-validation", false, "skip configuration validation before setup")

	return cmd
}

func runClusterSetup(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Resolve cluster name from args or active cluster
	name, err := resolveClusterName(args, true)
	if err != nil {
		return err
	}

	organization := ""

	// Reject planned providers that are not yet available
	cfg, err := loadCanonicalConfig(name)
	if err == nil {
		organization = cfg.OpenCenter.Meta.Organization
		if err := checkProviderAvailability(cfg.OpenCenter.Infrastructure.Provider); err != nil {
			return err
		}
	}

	// Resolve SetupService from global DI container
	container := getContainer()
	var setupService *cluster.SetupService
	if err := container.ResolveAs("SetupService", &setupService); err != nil {
		return fmt.Errorf("resolving setup service: %w", err)
	}

	// Parse flags into SetupOptions
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	skipValidation, _ := cmd.Flags().GetBool("skip-validation")

	opts := cluster.SetupOptions{
		ClusterName:    name,
		Organization:   organization,
		DryRun:         dryRun,
		SkipValidation: skipValidation,
		Force:          force,
	}

	if !dryRun {
		if err := config.UpdateStatus(name, config.StageSetup, config.StatusRunning); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", err)
		}
	}

	// Execute setup
	result, err := setupService.Setup(ctx, opts)
	if err != nil {
		if !dryRun {
			if statusErr := config.UpdateStatus(name, config.StageSetup, config.StatusFailed); statusErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", statusErr)
			}
		}
		return err
	}

	// Print result summary
	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Dry run complete\n")
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Setup complete\n")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "GitOps path:     %s\n", result.GitOpsPath)
	fmt.Fprintf(cmd.OutOrStdout(), "Manifests created: %d\n", result.ManifestsCreated)
	if result.CommitHash != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Commit:          %s\n", result.CommitHash)
	}

	if !dryRun {
		if err := config.UpdateStatus(name, config.StageSetup, config.StatusSuccess); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", err)
		}
	}

	return nil
}

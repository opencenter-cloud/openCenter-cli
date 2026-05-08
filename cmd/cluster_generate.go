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

// newClusterGenerateCmd creates the command for generating a cluster's GitOps repository.
func newClusterGenerateCmd() *cobra.Command {
	var renderOnly bool

	cmd := &cobra.Command{
		Use:   "generate [name]",
		Short: "Generate the GitOps repository and rendered manifests",
		Long: `Generate the customer GitOps repository and rendered manifests for a cluster.

This command creates or updates the repository structure, infrastructure templates,
Flux manifests, and application overlays based on the cluster configuration.

Use --render-only to render templates without running the full repository setup flow.`,
		Example: `  # Generate assets for the active cluster
  opencenter cluster generate

  # Generate assets for a specific cluster
  opencenter cluster generate my-cluster

  # Preview what would be generated
  opencenter cluster generate my-cluster --dry-run

  # Render templates only
  opencenter cluster generate my-cluster --render-only`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if renderOnly {
				return runClusterGenerateRenderOnly(cmd, args)
			}
			return runClusterGenerate(cmd, args)
		},
	}

	cmd.Flags().Bool("force", false, "overwrite existing GitOps repository")
	cmd.Flags().Bool("skip-validation", false, "skip configuration validation before generation")
	cmd.Flags().BoolVar(&renderOnly, "render-only", false, "render templates without running repository setup")

	return cmd
}

func runClusterGenerate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Resolve cluster name from args or active cluster
	name, err := resolveClusterNameForCommand(cmd, args, true)
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

	// Extract just the cluster name (without organization prefix) for path resolution
	actualClusterName := extractClusterName(name)

	app, err := GetApp(cmd.Context())
	if err != nil {
		return err
	}
	setupService := app.SetupService

	// Parse flags into SetupOptions
	force, _ := cmd.Flags().GetBool("force")
	dryRun := getGlobalOptions(cmd).DryRun
	skipValidation, _ := cmd.Flags().GetBool("skip-validation")

	opts := cluster.SetupOptions{
		ClusterName:    actualClusterName,
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
		fmt.Fprintf(cmd.OutOrStdout(), "Generate dry-run complete\n")
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Generate complete\n")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "GitOps path:       %s\n", result.GitOpsPath)
	fmt.Fprintf(cmd.OutOrStdout(), "Manifests created: %d\n", result.ManifestsCreated)

	if !dryRun {
		if err := config.UpdateStatus(name, config.StageSetup, config.StatusSuccess); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\nNext steps:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  opencenter secrets sync %s       # encrypt secrets\n", name)
		fmt.Fprintf(cmd.OutOrStdout(), "  opencenter cluster validate %s   # verify readiness\n", name)
		fmt.Fprintf(cmd.OutOrStdout(), "  opencenter cluster deploy %s     # deploy cluster\n", name)
	}

	return nil
}

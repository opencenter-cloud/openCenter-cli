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
	"fmt"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/gitops"
	"github.com/rackerlabs/openCenter-cli/internal/tofu"
	"github.com/spf13/cobra"
)

// newClusterRenderCmd creates the command for rendering GitOps templates.
//
// This command handles template rendering with full organization-based structure support.
// It always renders templates (no skip logic) making it ideal for iterative development.
// Unlike `setup`, it does not perform Git operations or initialization checks.
//
// Returns:
//   - *cobra.Command: A pointer to the configured `render` command.
func newClusterRenderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "render [name]",
		Short: "Render templates into the GitOps directory (always overwrites)",
		Long: `Render cluster templates into the GitOps directory structure.

This command always renders templates without any initialization checks,
making it perfect for iterative development and testing configuration changes.
It handles organization-based directory structures and overwrites existing files.

Unlike 'cluster setup', this command:
- Always renders templates (no skip logic)
- Does not perform Git operations
- Does not check if directory already exists
- Ideal for development and testing`,
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
					return fmt.Errorf("no active cluster; specify name")
				}
			}

			// Load configuration
			cfg, err := config.Load(name)
			if err != nil {
				return err
			}

			// Get organization from cluster metadata
			organization := cfg.OpenCenter.Meta.Organization
			if organization == "" {
				organization = "opencenter"
			}

			// Render templates with organization-based structure
			if err := renderClusterTemplates(cfg, organization, cmd); err != nil {
				return fmt.Errorf("failed to render cluster templates: %w", err)
			}

			// Update stage and status
			if err := config.UpdateStatus(name, config.StageRender, config.StatusSuccess); err != nil {
				// Don't fail the command if status update fails, just warn
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Render complete.")
			return nil
		},
	}
	return cmd
}

// renderClusterTemplates renders all cluster templates with organization-based structure support.
// This function handles the same organization-based GitOps structure as cluster setup,
// but without Git initialization or skip logic.
func renderClusterTemplates(cfg config.Config, organization string, cmd *cobra.Command) error {
	// Get CLI configuration manager for path resolution
	cliConfigManager, err := config.NewConfigManager("")
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	// Create path resolver for organization-based paths
	pathResolver := config.NewPathResolver(cliConfigManager)

	// Get organization-based paths
	clusterName := cfg.ClusterName()
	paths := pathResolver.ResolveClusterPaths(clusterName, organization)

	// Update configuration to use organization-based GitOps directory
	updatedCfg := cfg
	originalGitDir := cfg.GitOps().GitDir

	// If user specified a custom git_dir, use it; otherwise use organization-based path
	if originalGitDir != "" && originalGitDir != paths.GitOpsDir {
		// User has specified a custom git_dir, use it instead of organization path
		updatedCfg.OpenCenter.GitOps.GitDir = originalGitDir
	} else {
		// Use organization-based path
		updatedCfg.OpenCenter.GitOps.GitDir = paths.GitOpsDir
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Rendering templates to: %s\n", updatedCfg.GitOps().GitDir)

	// Create organization directory structure
	if err := pathResolver.CreateOrganizationStructure(organization); err != nil {
		return fmt.Errorf("failed to create organization structure: %w", err)
	}

	// Create cluster-specific directories
	if err := pathResolver.CreateClusterDirectories(clusterName, organization); err != nil {
		return fmt.Errorf("failed to create cluster directories: %w", err)
	}

	// Render GitOps base structure at organization level
	if err := gitops.CopyBase(updatedCfg, true); err != nil {
		return fmt.Errorf("failed to render base templates: %w", err)
	}

	// Render cluster-specific templates to organization structure
	if err := gitops.RenderClusterApps(updatedCfg); err != nil {
		return fmt.Errorf("failed to render cluster apps templates: %w", err)
	}

	if err := gitops.RenderInfrastructureCluster(updatedCfg); err != nil {
		return fmt.Errorf("failed to render infrastructure cluster templates: %w", err)
	}

	// Provision OpenTofu (renders main.tf and provider.tf)
	if err := tofu.Provision(updatedCfg); err != nil {
		return fmt.Errorf("failed to provision opentofu: %w", err)
	}

	return nil
}

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
	"path/filepath"

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

			// Render templates (organization handling is done by the internal packages)
			if err := renderClusterTemplates(cfg, "", cmd); err != nil {
				return fmt.Errorf("failed to render cluster templates: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Render complete.")
			return nil
		},
	}
	return cmd
}

// renderClusterTemplates renders all cluster templates using the GitOps package.
func renderClusterTemplates(cfg config.Config, organization string, cmd *cobra.Command) error {
	// Parse cluster identifier to get organization and cluster name
	clusterName := cfg.OpenCenter.Cluster.ClusterName
	org := cfg.OpenCenter.Meta.Organization
	if org == "" {
		org = organization
	}

	// Get the infrastructure cluster path
	gitOpsDir := cfg.GitOps().GitDir
	infrastructurePath := filepath.Join(gitOpsDir, "infrastructure", "clusters", clusterName)

	// Display informative output
	fmt.Fprintf(cmd.OutOrStdout(), "Rendering templates for cluster: %s\n", clusterName)
	if org != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Organization: %s\n", org)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Rendering templates to: %s\n", infrastructurePath)

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

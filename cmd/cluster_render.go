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
// This command specifically handles the template rendering part of the setup process.
// It populates the GitOps directory by processing all `.tmpl` files, but unlike
// the `setup` command, it does not perform any Git operations. This is useful
// for inspecting the output of the templates without committing them to a repository.
//
// Returns:
//   - *cobra.Command: A pointer to the configured `render` command.
func newClusterRenderCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "render [name]",
		Short: "Render templates into the GitOps directory without initializing git",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			cfg, err := config.Load(name)
			if err != nil {
				return err
			}
			if err := gitops.CopyBase(cfg, true); err != nil {
				return fmt.Errorf("failed to render base templates: %w", err)
			}
			if err := gitops.RenderClusterApps(cfg); err != nil {
				return fmt.Errorf("failed to render cluster apps templates: %w", err)
			}
			if err := gitops.RenderInfrastructureCluster(cfg); err != nil {
				return fmt.Errorf("failed to render infrastructure cluster templates: %w", err)
			}
			// Provision OpenTofu (renders provider.tf)
			if err := tofu.Provision(cfg); err != nil {
				return fmt.Errorf("failed to provision opentofu: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Render complete.")
			return nil
		},
	}
}

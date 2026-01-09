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
	"github.com/spf13/cobra"
)

// NewClusterCmd creates the top-level "cluster" command. It has
// several subcommands defined in separate files. Running "openCenter
// cluster" without subcommand prints help.
func NewClusterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Manage cluster configurations",
		Long: `Manage Kubernetes cluster configurations throughout their lifecycle.

The cluster command provides subcommands for initializing, validating, updating,
and managing cluster configurations. It supports organization-based multi-tenancy
and integrates with GitOps workflows.

Common Workflow:
  1. Initialize a new cluster configuration
  2. Validate the configuration
  3. Run preflight checks
  4. Set up infrastructure and GitOps repository
  5. Bootstrap the cluster with Flux

Configuration files are stored in organization-based directories:
  ~/.config/openCenter/clusters/<organization>/<cluster>/`,
		Example: `  # Initialize a new cluster
  openCenter cluster init my-cluster

  # Initialize with organization
  openCenter cluster init my-cluster --opencenter.meta.organization=myorg

  # Validate configuration
  openCenter cluster validate my-cluster

  # List all clusters
  openCenter cluster list

  # Select active cluster
  openCenter cluster select my-cluster

  # Show current cluster
  openCenter cluster current`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	// Add subcommands
	cmd.AddCommand(newClusterListCmd())
	cmd.AddCommand(newClusterSelectCmd())
	cmd.AddCommand(newClusterCurrentCmd())
	cmd.AddCommand(newClusterInfoCmd())
	cmd.AddCommand(newClusterInitCmd())
	cmd.AddCommand(newClusterEditCmd())
	cmd.AddCommand(newClusterValidateCmd())
	cmd.AddCommand(newClusterPreflightCmd())

	cmd.AddCommand(newClusterRenderCmd())
	cmd.AddCommand(newClusterBootstrapCmd())
	cmd.AddCommand(newClusterSchemaCmd())
	cmd.AddCommand(newClusterDestroyCmd())
	cmd.AddCommand(newClusterUpdateCmd())
	cmd.AddCommand(newClusterConfigUpdateCmd())
	cmd.AddCommand(newClusterServiceCmd())
	cmd.AddCommand(newClusterCredentialsCmd())
	return cmd
}

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
	"strings"

	"github.com/spf13/cobra"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/credentials"
)

// newClusterDeactivateCmd creates the "cluster deactivate" command.
func newClusterDeactivateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deactivate <organization/cluster-name>",
		Short: "Deactivate a cluster environment by unsetting all environment variables",
		Long: `Deactivate a cluster environment by unsetting all environment variables.

This command generates shell unset statements to remove all environment variables
that were set by the 'cluster activate' command. This includes:

1. Cloud provider credentials (AWS and OpenStack)
2. Local binary path (BIN)
3. Kubernetes configuration (KUBECONFIG)

Note: The PATH variable is not automatically restored to its previous value.
You may need to restart your shell or manually restore PATH if needed.

The command outputs shell unset statements that should be evaluated in the
current shell to deactivate the cluster environment:

  eval $(openCenter cluster deactivate myorg/my-cluster)

This is equivalent to running:
  eval $(openCenter cluster credentials unset --provider all)
  unset BIN
  unset KUBECONFIG

The deactivated environment removes cluster-specific variables, allowing you
to switch to a different cluster or work in a clean environment.`,
		Example: `  # Deactivate cluster environment
  eval $(openCenter cluster deactivate myorg/my-cluster)

  # Deactivate current cluster if one is selected
  eval $(openCenter cluster deactivate)

  # Preview deactivation commands without executing
  openCenter cluster deactivate myorg/my-cluster`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name from args or active cluster for validation
			if len(args) == 0 {
				// No name provided, try to use active cluster
				activeName, err := config.GetActive()
				if err != nil {
					return fmt.Errorf("failed to get active cluster: %w", err)
				}
				if activeName == "" {
					return fmt.Errorf("no cluster name provided and no active cluster set; specify a cluster name or use 'openCenter cluster select <name>' to set an active cluster")
				}
			}

			var output strings.Builder

			// Unset AWS credentials
			awsVars := credentials.GetAWSEnvVars()
			for _, envVar := range awsVars {
				output.WriteString(fmt.Sprintf("unset %s\n", envVar))
			}

			// Unset OpenStack credentials
			osVars := credentials.GetOpenStackEnvVars()
			for _, envVar := range osVars {
				output.WriteString(fmt.Sprintf("unset %s\n", envVar))
			}

			// Unset cluster-specific environment variables
			output.WriteString("unset BIN\n")
			output.WriteString("unset KUBECONFIG\n")

			fmt.Fprint(cmd.OutOrStdout(), output.String())
			return nil
		},
	}

	return cmd
}

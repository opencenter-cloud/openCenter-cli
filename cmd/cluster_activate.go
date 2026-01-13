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

// newClusterActivateCmd creates the "cluster activate" command.
func newClusterActivateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "activate <organization/cluster-name>",
		Short: "Activate a cluster environment by setting all required environment variables",
		Long: `Activate a cluster environment by setting all required environment variables.

This command combines credential export with additional environment setup needed
for cluster operations. It sets up:

1. Cloud provider credentials (from cluster configuration)
2. Cluster binary path (BIN=<cluster-dir>/<organization>/infrastructure/clusters/<cluster>/.bin)
3. Extended PATH to include cluster binaries
4. Kubernetes configuration (KUBECONFIG=<cluster-dir>/<organization>/infrastructure/clusters/<cluster>/kubeconfig.yaml)

The command outputs shell export statements that should be evaluated in the
current shell to activate the cluster environment:

  eval $(openCenter cluster activate myorg/my-cluster)

This is equivalent to running:
  eval $(openCenter cluster credentials export --provider all)
  export BIN=<cluster-dir>/<organization>/infrastructure/clusters/<cluster>/.bin
  export PATH=${BIN}:${PATH}
  export KUBECONFIG=<cluster-dir>/<organization>/infrastructure/clusters/<cluster>/kubeconfig.yaml

The activated environment provides everything needed to work with the cluster
using kubectl, cloud CLI tools, and local binaries downloaded by openCenter.`,
		Example: `  # Activate cluster environment
  eval $(openCenter cluster activate myorg/my-cluster)

  # Activate current cluster if one is selected
  eval $(openCenter cluster activate)

  # Preview activation commands without executing
  openCenter cluster activate myorg/my-cluster`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name from args or active cluster
			var name string
			if len(args) > 0 {
				name = args[0]
			} else {
				// No name provided, try to use active cluster
				activeName, err := config.GetActive()
				if err != nil {
					return fmt.Errorf("failed to get active cluster: %w", err)
				}
				if activeName == "" {
					return fmt.Errorf("no cluster name provided and no active cluster set; specify a cluster name or use 'openCenter cluster select <name>' to set an active cluster")
				}
				name = activeName
			}

			// Load cluster configuration
			cfg, err := config.Load(name)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration: %w", err)
			}

			// Get cluster paths
			configManager, err := config.NewConfigManager("")
			if err != nil {
				return fmt.Errorf("failed to create config manager: %w", err)
			}
			pathResolver := config.NewPathResolver(configManager)
			
			// Parse cluster identifier to get organization and cluster name
			organization, actualClusterName, err := config.ParseClusterIdentifier(name)
			if err != nil {
				return fmt.Errorf("invalid cluster identifier: %w", err)
			}

			// Use organization from config if available
			if cfg.OpenCenter.Meta.Organization != "" {
				organization = cfg.OpenCenter.Meta.Organization
			}

			// Resolve cluster paths
			paths := pathResolver.ResolveClusterPaths(actualClusterName, organization)

			// Create credentials extractor
			extractor := credentials.NewExtractor(cfg)

			var output strings.Builder

			// Export cloud provider credentials
			awsCreds, awsErr := extractor.ExtractAWS()
			osCreds, osErr := extractor.ExtractOpenStack()

			hasAWS := awsErr == nil && !awsCreds.IsEmpty()
			hasOS := osErr == nil && !osCreds.IsEmpty()

			if hasAWS {
				output.WriteString(awsCreds.ToEnvVars())
			}
			if hasOS {
				if hasAWS {
					output.WriteString("\n")
				}
				output.WriteString(osCreds.ToEnvVars())
			}

			// Add cluster-specific environment variables
			if hasAWS || hasOS {
				output.WriteString("\n")
			}

			// Set BIN directory to full cluster path
			output.WriteString(fmt.Sprintf("export BIN=%s\n", paths.BinPath))

			// Extend PATH to include BIN directory
			output.WriteString("export PATH=${BIN}:${PATH}\n")

			// Set KUBECONFIG to full cluster path
			output.WriteString(fmt.Sprintf("export KUBECONFIG=%s\n", paths.KubeconfigPath))

			fmt.Fprint(cmd.OutOrStdout(), output.String())
			return nil
		},
	}

	return cmd
}

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

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/spf13/cobra"
)

// newClusterListCmd creates the command for listing all configured clusters.
//
// This command retrieves the names of all clusters from the configuration
// directory and prints them to standard output, one per line.
//
// Returns:
//   - *cobra.Command: A pointer to the configured `list` command.
func newClusterListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List configured clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			config.Debug("cluster list: starting cluster list operation")

			names, err := listClusters(ctx)
			if err != nil {
				config.Debugf("cluster list: failed to list clusters: %v", err)
				return failf("failed to list clusters: %v", err)
			}

			config.Debugf("cluster list: found %d cluster(s)", len(names))
			for i, name := range names {
				config.Debugf("cluster list: [%d] %s", i, name)
			}

			// Get active cluster to show indicator
			activeCluster, err := getActiveCluster()
			if err != nil {
				config.Debugf("cluster list: failed to get active cluster: %v", err)
				// Continue without active indicator if we can't get it
				activeCluster = ""
			} else {
				config.Debugf("cluster list: active cluster: %s", activeCluster)
				activeCluster = normalizeClusterDisplayName(activeCluster)
			}

			opts := getGlobalOptions(cmd)
			if opts.Output == OutputJSON || opts.Output == OutputYAML {
				return writeStructuredOutput(cmd, opts.Output, names)
			}

			config.Debug("cluster list: outputting plain text format")
			for _, n := range names {
				// Show active indicator with asterisk
				if activeCluster != "" && n == activeCluster {
					fmt.Fprintf(cmd.OutOrStdout(), "* %s\n", n)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), n)
				}
			}
			config.Debug("cluster list: operation completed successfully")
			return nil
		},
	}

	markReadOnlyCommand(cmd)

	return cmd
}

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
    "github.com/spf13/cobra"
)

import (
	"encoding/json"
)

// newClusterListCmd creates the command for listing all configured clusters.
//
// This command retrieves the names of all clusters from the configuration
// directory and prints them to standard output, one per line. It also supports
// a `--json` flag, which formats the output as a JSON array for easier
// consumption by scripts and other tools.
//
// Returns:
//   - *cobra.Command: A pointer to the configured `list` command.
func newClusterListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List configured clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := config.List()
			if err != nil {
				return failf("failed to list clusters: %v", err)
			}

			jsonOutput, _ := cmd.Flags().GetBool("json")
			if jsonOutput {
				b, err := json.Marshal(names)
				if err != nil {
					return failf("failed to marshal cluster names to JSON: %v", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}

			for _, n := range names {
				fmt.Fprintln(cmd.OutOrStdout(), n)
			}
			return nil
		},
	}

	cmd.Flags().Bool("json", false, "Machine-readable JSON output for supported commands")

	return cmd
}

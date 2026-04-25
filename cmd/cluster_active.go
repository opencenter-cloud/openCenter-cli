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
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newClusterActiveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "active",
		Short: "Show the active cluster",
		Long: `Show the current active cluster with its selection source.

The active cluster follows this precedence:
  1. OPENCENTER_CLUSTER environment variable (session-scoped)
  2. Session file (if shell integration is active)
  3. Persistent selection from marker file

Use --quiet to output only the cluster name without source information.`,
		Example: `  # Show current cluster with source
  opencenter cluster active

  # Show only cluster name (for scripting)
  opencenter cluster active --quiet`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := getActiveCluster()
			if err != nil {
				return err
			}
			if name == "" {
				// Nothing to show
				return nil
			}

			q, _ := cmd.Flags().GetBool("quiet")
			if q {
				fmt.Fprint(cmd.OutOrStdout(), strings.TrimSpace(name))
			} else {
				// Determine source of the active cluster
				source := "persistent"
				if os.Getenv("OPENCENTER_CLUSTER") != "" {
					source = "environment"
				} else if sessionFile := os.Getenv("OPENCENTER_SESSION_FILE"); sessionFile != "" {
					if _, err := os.Stat(sessionFile); err == nil {
						source = "session"
					}
				}

				fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\n", strings.TrimSpace(name), source)
			}
			return nil
		},
	}
	cmd.Flags().BoolP("quiet", "q", false, "quiet output (just the name)")
	return cmd
}

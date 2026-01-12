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
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// newClusterActiveFastCmd creates a fast command for shell integration.
// This command bypasses the full configuration system for maximum performance.
func newClusterActiveFastCmd() *cobra.Command {
	var short bool
	var prompt bool

	cmd := &cobra.Command{
		Use:   "active-fast",
		Short: "Fast active cluster lookup for shell integration",
		Long: `Fast active cluster lookup optimized for shell integration.

This command bypasses the full configuration loading system and directly
reads the active cluster file for maximum performance. It's designed to
be used in shell prompts and scripts where speed is critical.

The command outputs nothing if no cluster is active, making it safe
to use in shell prompts without error messages.`,
		Example: `  # Get active cluster name
  openCenter cluster active-fast

  # Get short name (without organization)
  openCenter cluster active-fast --short

  # Get formatted for prompt
  openCenter cluster active-fast --prompt

  # Use in shell prompt
  PS1="$(openCenter cluster active-fast --prompt)$PS1"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Fast path: directly read the active file
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil // Silent fail for shell integration
			}

			activeFile := filepath.Join(homeDir, ".config", "openCenter", ".active")
			data, err := os.ReadFile(activeFile)
			if err != nil {
				return nil // Silent fail - no active cluster
			}

			clusterName := strings.TrimSpace(string(data))
			if clusterName == "" {
				return nil // Silent fail - empty active cluster
			}

			// Format output based on flags
			if short {
				// Extract just the cluster name (after last /)
				parts := strings.Split(clusterName, "/")
				clusterName = parts[len(parts)-1]
			}

			if prompt {
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] ", clusterName)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", clusterName)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&short, "short", false, "show only cluster name (without organization)")
	cmd.Flags().BoolVar(&prompt, "prompt", false, "format for shell prompt with brackets and trailing space")

	return cmd
}
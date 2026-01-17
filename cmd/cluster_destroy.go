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

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/spf13/cobra"
)

func newClusterDestroyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy [name]",
		Short: "Destroy a cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, err := config.Load(name)
			if err != nil {
				return err
			}

			// Remove gitops directory if it exists
			gitopsDir := cfg.GitOps().GitDir
			if gitopsDir != "" {
				if err := os.RemoveAll(gitopsDir); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("failed to remove gitops directory: %w", err)
				}
			}

			// Get the config file path
			configPath, err := config.ConfigPath(name)
			if err != nil {
				return fmt.Errorf("failed to resolve config path: %w", err)
			}

			// Delete the config file
			if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to delete config file: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Cluster %q destroyed successfully.\n", name)
			return nil
		},
	}
	return cmd
}

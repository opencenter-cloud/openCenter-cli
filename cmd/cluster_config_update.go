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
	"time"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/spf13/cobra"
)

// newClusterConfigUpdateCmd creates the command for updating a cluster configuration
// by reloading it with current defaults and saving it back with empty fields omitted.
//
// This command loads the current cluster configuration and creates a new cluster
// configuration using the values at load time. This is useful for:
// - Applying new default values from schema updates
// - Normalizing configuration format
// - Migrating configurations to new structure
// - Cleaning up empty/unused fields from the configuration
//
// Returns:
//   - *cobra.Command: A pointer to the configured `config-update` command.
func newClusterConfigUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config-update [name]",
		Short: "Update cluster configuration with current defaults",
		Long: `Update a cluster configuration by reloading it with current defaults and saving it back.

This command loads the current cluster configuration and creates a new cluster
configuration using the values at load time. This is useful for:
- Applying new default values from schema updates
- Normalizing configuration format
- Migrating configurations to new structure
- Cleaning up empty/unused fields from the configuration

The command will:
1. Load the existing cluster configuration
2. Merge it with current schema defaults
3. Remove any empty fields (omitempty behavior)
4. Save the updated configuration back to the same location

If no cluster name is provided, the currently selected cluster is used.

Examples:
  # Update the currently selected cluster
  openCenter cluster config-update

  # Update a specific cluster
  openCenter cluster config-update my-cluster

  # Update a cluster in a specific organization
  openCenter cluster config-update myorg/my-cluster`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var clusterName string

			// Determine cluster name
			if len(args) > 0 {
				clusterName = args[0]
			} else {
				// Use currently selected cluster
				active, err := config.GetActive()
				if err != nil {
					return fmt.Errorf("failed to get active cluster: %w", err)
				}
				if active == "" {
					return fmt.Errorf("no cluster selected. Use 'openCenter cluster select' to select a cluster or provide a cluster name")
				}
				clusterName = active
			}

			// Load the current configuration
			fmt.Fprintf(cmd.OutOrStdout(), "Loading configuration for cluster '%s'...\n", clusterName)
			cfg, err := config.Load(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration: %w", err)
			}

			// Validate the configuration
			fmt.Fprintf(cmd.OutOrStdout(), "Validating configuration...\n")
			if errs := config.Validate(cfg); len(errs) > 0 {
				fmt.Fprintf(cmd.OutOrStderr(), "Warning: Configuration has validation errors:\n")
				for _, e := range errs {
					fmt.Fprintf(cmd.OutOrStderr(), "  - %s\n", e)
				}
				fmt.Fprintf(cmd.OutOrStderr(), "\nProceeding with update anyway...\n")
			}

			// Create backup of existing configuration before updating
			configPath, pathErr := config.ConfigPath(clusterName)
			if pathErr != nil {
				return fmt.Errorf("failed to get config path: %w", pathErr)
			}

			// Create backup with timestamp
			timestamp := time.Now().Format("20060102-150405")
			backupPath := fmt.Sprintf("%s.%s.backup", configPath, timestamp)

			fmt.Fprintf(cmd.OutOrStdout(), "Creating backup at: %s\n", backupPath)
			if err := copyFile(configPath, backupPath); err != nil {
				return fmt.Errorf("failed to create backup: %w", err)
			}

			// Save the configuration with omitempty (this will apply current defaults and remove empty fields)
			fmt.Fprintf(cmd.OutOrStdout(), "Saving updated configuration...\n")
			if err := config.SaveWithOmitEmpty(cfg); err != nil {
				return fmt.Errorf("failed to save updated configuration: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully updated configuration for cluster '%s'\n", clusterName)
			fmt.Fprintf(cmd.OutOrStdout(), "Configuration saved to: %s\n", configPath)

			return nil
		},
	}

	return cmd
}

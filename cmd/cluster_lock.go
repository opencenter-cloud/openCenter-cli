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
	"gopkg.in/yaml.v3"
)

// newClusterLockCmd creates the command for locking a cluster.
//
// This command locks a cluster to prevent accidental modifications.
// A locked cluster requires an explicit unlock before it can be modified.
//
// Returns:
//   - *cobra.Command: A pointer to the configured `lock` command.
func newClusterLockCmd() *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "lock [name]",
		Short: "Lock a cluster to prevent modifications",
		Long: `Lock a cluster to prevent accidental modifications.

A locked cluster cannot be modified until it is explicitly unlocked.
This is useful for protecting production clusters or clusters undergoing maintenance.

Examples:
  # Lock the currently selected cluster
  openCenter cluster lock --reason "Production cluster - do not modify"

  # Lock a specific cluster
  openCenter cluster lock my-cluster --reason "Under maintenance"

  # Lock a cluster in a specific organization
  openCenter cluster lock myorg/my-cluster --reason "Critical infrastructure"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine cluster name
			var clusterName string
			if len(args) > 0 {
				clusterName = args[0]
			} else {
				active, err := config.GetActive()
				if err != nil {
					return fmt.Errorf("failed to get active cluster: %w", err)
				}
				if active == "" {
					return fmt.Errorf("no cluster selected. Use 'openCenter cluster select' to select a cluster or provide a cluster name")
				}
				clusterName = active
			}

			// Require a reason for locking
			if reason == "" {
				return fmt.Errorf("lock reason is required. Use --reason flag to specify why the cluster is being locked")
			}

			// Load the cluster configuration
			cfg, err := config.Load(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration: %w", err)
			}

			// Check if already locked
			if cfg.OpenCenter.Meta.Locked {
				return fmt.Errorf("cluster '%s' is already locked. Reason: %s", clusterName, cfg.OpenCenter.Meta.LockReason)
			}

			// Lock the cluster
			cfg.OpenCenter.Meta.Locked = true
			cfg.OpenCenter.Meta.LockReason = reason

			// Save the configuration
			configPath, err := config.ConfigPath(clusterName)
			if err != nil {
				return fmt.Errorf("failed to get config path: %w", err)
			}

			data, err := yaml.Marshal(&cfg)
			if err != nil {
				return fmt.Errorf("failed to marshal configuration: %w", err)
			}

			if err := os.WriteFile(configPath, data, 0600); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Cluster '%s' has been locked\n", clusterName)
			fmt.Fprintf(cmd.OutOrStdout(), "Reason: %s\n", reason)
			fmt.Fprintf(cmd.OutOrStdout(), "\nTo unlock this cluster, run: openCenter cluster unlock %s --reason \"<unlock reason>\"\n", clusterName)

			return nil
		},
	}

	cmd.Flags().StringVarP(&reason, "reason", "r", "", "Reason for locking the cluster (required)")

	return cmd
}

// newClusterUnlockCmd creates the command for unlocking a cluster.
//
// This command unlocks a previously locked cluster, allowing modifications.
//
// Returns:
//   - *cobra.Command: A pointer to the configured `unlock` command.
func newClusterUnlockCmd() *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "unlock [name]",
		Short: "Unlock a cluster to allow modifications",
		Long: `Unlock a previously locked cluster to allow modifications.

This command removes the lock from a cluster, allowing it to be modified again.
A reason must be provided to document why the cluster is being unlocked.

Examples:
  # Unlock the currently selected cluster
  openCenter cluster unlock --reason "Maintenance completed"

  # Unlock a specific cluster
  openCenter cluster unlock my-cluster --reason "Emergency fix applied"

  # Unlock a cluster in a specific organization
  openCenter cluster unlock myorg/my-cluster --reason "Approved by ops team"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine cluster name
			var clusterName string
			if len(args) > 0 {
				clusterName = args[0]
			} else {
				active, err := config.GetActive()
				if err != nil {
					return fmt.Errorf("failed to get active cluster: %w", err)
				}
				if active == "" {
					return fmt.Errorf("no cluster selected. Use 'openCenter cluster select' to select a cluster or provide a cluster name")
				}
				clusterName = active
			}

			// Require a reason for unlocking
			if reason == "" {
				return fmt.Errorf("unlock reason is required. Use --reason flag to specify why the cluster is being unlocked")
			}

			// Load the cluster configuration
			cfg, err := config.Load(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration: %w", err)
			}

			// Check if not locked
			if !cfg.OpenCenter.Meta.Locked {
				return fmt.Errorf("cluster '%s' is not locked", clusterName)
			}

			// Store the previous lock reason for logging
			previousReason := cfg.OpenCenter.Meta.LockReason

			// Unlock the cluster
			cfg.OpenCenter.Meta.Locked = false
			cfg.OpenCenter.Meta.LockReason = ""

			// Save the configuration
			configPath, err := config.ConfigPath(clusterName)
			if err != nil {
				return fmt.Errorf("failed to get config path: %w", err)
			}

			data, err := yaml.Marshal(&cfg)
			if err != nil {
				return fmt.Errorf("failed to marshal configuration: %w", err)
			}

			if err := os.WriteFile(configPath, data, 0600); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Cluster '%s' has been unlocked\n", clusterName)
			fmt.Fprintf(cmd.OutOrStdout(), "Previous lock reason: %s\n", previousReason)
			fmt.Fprintf(cmd.OutOrStdout(), "Unlock reason: %s\n", reason)

			return nil
		},
	}

	cmd.Flags().StringVarP(&reason, "reason", "r", "", "Reason for unlocking the cluster (required)")

	return cmd
}

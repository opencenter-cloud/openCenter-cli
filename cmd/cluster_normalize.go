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
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// newClusterNormalizeCmd creates the "cluster normalize" command.
// This command adds missing default fields to an existing cluster configuration.
func newClusterNormalizeCmd() *cobra.Command {
	var noBackup bool

	cmd := &cobra.Command{
		Use:   "normalize [name]",
		Short: "Add missing default fields to a cluster configuration",
		Long: `Add missing default fields to an existing cluster configuration.

This command loads the current cluster configuration, merges it with the default
configuration template to add any missing keys, and writes the updated configuration
back to the file.

To set specific configuration values, use 'opencenter cluster set' with native v2
dot notation instead (e.g., opencenter.gitops.repository.url=...).

A timestamped backup is automatically created before modification:
  <config-file>.backup.<timestamp>

The backup allows you to review changes and revert if needed. Delete the backup
once you're satisfied with the updated configuration.

Missing keys are added with their default values based on:
  • Provider-specific defaults (if provider is configured)
  • Global schema defaults
  • Empty/zero values for required fields

Existing values are preserved - only missing keys are added.

If no cluster name is provided, normalizes the currently active cluster.`,
		Example: `  # Normalize active cluster configuration
  opencenter cluster normalize

  # Normalize specific cluster
  opencenter cluster normalize my-cluster

  # Normalize with organization prefix
  opencenter cluster normalize myorg/my-cluster

  # Dry run to preview changes
  opencenter cluster normalize my-cluster --dry-run

  # Update without creating backup (not recommended)
  opencenter cluster normalize my-cluster --no-backup`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name from args or active cluster
			name, err := resolveClusterName(args, true)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// Load config to get organization
			cfg, err := loadConfig(ctx, name)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Extract just the cluster name (without organization prefix)
			actualClusterName := extractClusterName(name)

			// Get configuration file path
			configPath, err := getConfigPath(ctx, actualClusterName, cfg.OpenCenter.Meta.Organization)
			if err != nil {
				return fmt.Errorf("failed to resolve configuration path: %w", err)
			}

			// Check if configuration file exists
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				return fmt.Errorf("configuration file does not exist: %s", configPath)
			}

			// Read existing configuration as raw YAML
			existingData, err := os.ReadFile(configPath)
			if err != nil {
				return fmt.Errorf("failed to read existing configuration: %w", err)
			}

			// Load configuration using ConfigurationManager (this will merge with defaults)
			manager, err := getConfigManager()
			if err != nil {
				return fmt.Errorf("failed to get configuration manager: %w", err)
			}

			completeCfg, err := manager.Load(ctx, name)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Marshal the complete configuration to YAML
			completeData, err := yaml.Marshal(completeCfg)
			if err != nil {
				return fmt.Errorf("failed to marshal configuration: %w", err)
			}

			// Check if there are any changes
			if bytes.Equal(existingData, completeData) {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ Configuration is already up to date\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  No missing keys found\n")
				return nil
			}

			// Count added bytes
			existingSize := len(existingData)
			completeSize := len(completeData)
			addedBytes := completeSize - existingSize

			dryRun := getGlobalOptions(cmd).DryRun
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Dry run - no changes will be made\n\n")
				fmt.Fprintf(cmd.OutOrStdout(), "Would normalize configuration:\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  File: %s\n", configPath)
				fmt.Fprintf(cmd.OutOrStdout(), "  Current size: %d bytes\n", existingSize)
				fmt.Fprintf(cmd.OutOrStdout(), "  Normalized size: %d bytes\n", completeSize)
				fmt.Fprintf(cmd.OutOrStdout(), "  Added: ~%d bytes\n", addedBytes)
				if !noBackup {
					backupPath := generateBackupPath(configPath)
					fmt.Fprintf(cmd.OutOrStdout(), "  Backup would be created: %s\n", backupPath)
				}
				return nil
			}

			// Create backup unless disabled
			var backupPath string
			if !noBackup {
				backupPath = generateBackupPath(configPath)
				if err := os.WriteFile(backupPath, existingData, 0600); err != nil {
					return fmt.Errorf("failed to create backup: %w", err)
				}
			}

			// Write updated configuration
			if err := os.WriteFile(configPath, completeData, 0600); err != nil {
				return fmt.Errorf("failed to write updated configuration: %w", err)
			}

			// Success output
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Configuration normalized successfully\n\n")
			fmt.Fprintf(cmd.OutOrStdout(), "Normalized configuration:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  File: %s\n", configPath)
			fmt.Fprintf(cmd.OutOrStdout(), "  Added: ~%d bytes of missing keys\n", addedBytes)

			if !noBackup {
				fmt.Fprintf(cmd.OutOrStdout(), "\nBackup created:\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", backupPath)
				fmt.Fprintf(cmd.OutOrStdout(), "\nReview the changes and delete the backup if satisfied:\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  rm %s\n", backupPath)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&noBackup, "no-backup", false, "skip creating a backup before normalizing")

	return cmd
}

// generateBackupPath generates a timestamped backup file path.
func generateBackupPath(originalPath string) string {
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s.backup.%s", originalPath, timestamp)
}

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

	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation/validators"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
	"github.com/spf13/cobra"
)

// newClusterEditCmd creates the command for editing a cluster configuration.
//
// This command opens the cluster configuration file in the user's preferred editor.
// If no cluster name is provided, it uses the currently selected cluster.
// The editor is determined by checking the following environment variables in order:
// 1. EDITOR
// 2. VISUAL
// 3. Falls back to "vi" if neither is set
//
// Returns:
//   - *cobra.Command: A pointer to the configured `edit` command.
func newClusterEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit [name]",
		Short: "Edit a cluster configuration in your preferred editor",
		Long: `Edit a cluster configuration file in your preferred editor.

If no cluster name is provided, the currently selected cluster is edited.
The editor is determined by the EDITOR or VISUAL environment variables,
falling back to 'vi' if neither is set.

Examples:
  # Edit the currently selected cluster
  opencenter cluster edit

  # Edit a specific cluster
  opencenter cluster edit my-cluster

  # Edit a cluster in a specific organization
  opencenter cluster edit myorg/my-cluster`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Initialize security components
			clusterValidator := validators.NewClusterNameValidator()
			pathValidator := security.NewDefaultInputValidator()
			sanitizer := security.NewDefaultCommandSanitizer()
			runner := security.GetDefaultCommandRunner()

			var clusterName string

			// Determine cluster name
			if len(args) > 0 {
				clusterName = args[0]

				// Validate cluster name - allow organization/cluster format
				// Split and validate each part separately
				parts := strings.Split(clusterName, "/")
				if len(parts) > 2 {
					return fmt.Errorf("invalid cluster identifier format: use 'cluster' or 'organization/cluster'")
				}
				for _, part := range parts {
					result, err := clusterValidator.Validate(ctx, part)
					if err != nil {
						return fmt.Errorf("validation error: %w", err)
					}
					if !result.Valid {
						return fmt.Errorf("invalid cluster name: %s", result.Errors[0].Message)
					}
				}
			} else {
				// Use currently selected cluster
				active, err := getActiveCluster()
				if err != nil {
					return fmt.Errorf("failed to get active cluster: %w", err)
				}
				if active == "" {
					return fmt.Errorf("no cluster selected. Use 'opencenter cluster select' to select a cluster or provide a cluster name")
				}
				clusterName = active

				// Active cluster is already validated when set, no need to re-validate
			}

			// Get the configuration file path
			// Load config to get organization (this handles organization/cluster format)
			cfg, err := loadConfig(ctx, clusterName)
			if err != nil {
				return fmt.Errorf("failed to load configuration for cluster '%s': %w", clusterName, err)
			}

			// Extract just the cluster name (without organization prefix)
			actualClusterName := extractClusterName(clusterName)

			configPath, err := getConfigPath(ctx, actualClusterName, cfg.OpenCenter.Meta.Organization)
			if err != nil {
				return fmt.Errorf("failed to get config path for cluster '%s': %w", clusterName, err)
			}

			// Validate the config path (Requirements: 1.8)
			if err := pathValidator.ValidatePath(configPath); err != nil {
				return fmt.Errorf("invalid config path: %w", err)
			}

			// Check if the configuration file exists
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				return fmt.Errorf("cluster configuration file '%s' not found. Use 'opencenter cluster list' to see available clusters", clusterName)
			}

			// Check if dry-run mode is enabled
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would open %s for editing\n", configPath)
				return nil
			}

			// Determine the editor to use
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = os.Getenv("VISUAL")
			}
			if editor == "" {
				editor = "vi" // Default fallback
			}

			// Validate EDITOR environment variable (Requirements: 1.1, 1.2)
			if err := sanitizer.ValidateEditor(editor); err != nil {
				return fmt.Errorf("invalid EDITOR environment variable: %w", err)
			}

			// Open the editor
			fmt.Fprintf(cmd.OutOrStdout(), "Opening %s in %s...\n", configPath, editor)

			// Use sanitized command execution (Requirements: 1.3, 1.4)
			editorCmd, err := runner.PrepareCommand(editor, configPath)
			if err != nil {
				return fmt.Errorf("failed to prepare editor command: %w", err)
			}

			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr

			if err := editorCmd.Run(); err != nil {
				return fmt.Errorf("failed to open editor: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Configuration file saved.\n")
			return nil
		},
	}

	return cmd
}

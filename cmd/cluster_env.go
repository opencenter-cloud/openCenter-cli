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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/credentials"
	"github.com/spf13/cobra"
)

func newClusterEnvCmd() *cobra.Command {
	var shellOverride string

	cmd := &cobra.Command{
		Use:   "env [cluster-name]",
		Short: "Export cluster environment variables",
		Long: `Export environment variables for the specified cluster or current active cluster.

This command generates shell commands to set up the cluster environment including:
- Cloud provider credentials (AWS, OpenStack)
- KUBECONFIG path
- ANSIBLE_INVENTORY path
- Cluster-specific binary paths
- Virtual environment activation

If no cluster name is provided, uses the current active cluster.

The output is designed to be evaluated by your shell:
  eval "$(opencenter cluster env)"
  eval "$(opencenter cluster env my-cluster)"

This is useful for:
- Re-exporting environment variables after they've changed
- Setting up environment in a new terminal session
- Refreshing credentials that may have been updated`,
		Example: `  # Export current cluster environment
  eval "$(opencenter cluster env)"

  # Export specific cluster environment
  eval "$(opencenter cluster env prod-cluster)"

  # Export with organization
  eval "$(opencenter cluster env myorg/prod-cluster)"

  # Override shell detection
  opencenter cluster env --shell fish | source`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name
			clusterName, err := resolveClusterName(args, true)
			if err != nil {
				return err
			}

			// Load cluster configuration
			cfg, err := loadConfig(cmd.Context(), clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration: %w", err)
			}

			// Get cluster paths
			configManager, err := config.NewConfigManager("")
			if err != nil {
				return fmt.Errorf("failed to create config manager: %w", err)
			}

			// Get base directory for clusters
			baseDir := configManager.GetConfig().Paths.ClustersDir
			if baseDir == "" {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to get home directory: %w", err)
				}
				baseDir = filepath.Join(homeDir, ".config", "opencenter", "clusters")
			}

			pathResolver := paths.NewPathResolver(baseDir)

			// Parse cluster identifier to get organization and cluster name
			organization, actualClusterName, err := config.ParseClusterIdentifier(clusterName)
			if err != nil {
				return fmt.Errorf("invalid cluster identifier: %w", err)
			}

			// Use organization from config if available
			if cfg.OpenCenter.Meta.Organization != "" {
				organization = cfg.OpenCenter.Meta.Organization
			}

			// Resolve cluster paths
			ctx := context.Background()
			clusterPaths, err := pathResolver.Resolve(ctx, actualClusterName, organization)
			if err != nil {
				return fmt.Errorf("failed to resolve cluster paths: %w", err)
			}

			// Detect shell (or use override)
			shell := shellOverride
			if shell == "" {
				shell = detectShell()
			}

			// Validate shell override if provided
			if shellOverride != "" {
				validShells := map[string]bool{"bash": true, "zsh": true, "fish": true, "powershell": true}
				if !validShells[shell] {
					return fmt.Errorf("invalid shell: %s (valid options: bash, zsh, fish, powershell)", shell)
				}
			}

			nativeCfg, _, _, _, err := loadNativeV2ConfigWithIdentifier(cmd.Context(), clusterName)
			if err != nil {
				return fmt.Errorf("failed to load native cluster configuration: %w", err)
			}

			// Create credentials extractor
			extractor := credentials.NewExtractor(*nativeCfg)

			var output strings.Builder

			// Export cloud provider credentials with shell-specific syntax
			awsCreds, awsErr := extractor.ExtractAWS()
			osCreds, osErr := extractor.ExtractOpenStack()

			hasAWS := awsErr == nil && !awsCreds.IsEmpty()
			hasOS := osErr == nil && !osCreds.IsEmpty()

			if hasAWS {
				output.WriteString(awsCreds.ToEnvVarsForShell(shell))
			}
			if hasOS {
				if hasAWS {
					output.WriteString("\n")
				}
				output.WriteString(osCreds.ToEnvVarsForShell(shell))
			}

			// Add cluster-specific environment variables with shell-aware syntax
			if hasAWS || hasOS {
				output.WriteString("\n")
			}

			// Generate shell-specific export commands
			switch shell {
			case "fish":
				output.WriteString(fmt.Sprintf("set -gx OPENCENTER_CLUSTER %s\n", clusterName))
				if _, err := os.Stat(clusterPaths.KubeconfigPath); err == nil {
					output.WriteString(fmt.Sprintf("set -gx KUBECONFIG %s\n", clusterPaths.KubeconfigPath))
				}
				if _, err := os.Stat(clusterPaths.InventoryPath); err == nil {
					output.WriteString(fmt.Sprintf("set -gx ANSIBLE_INVENTORY %s\n", clusterPaths.InventoryPath))
				}
				if _, err := os.Stat(clusterPaths.BinPath); err == nil {
					output.WriteString(fmt.Sprintf("set -gx PATH %s $PATH\n", clusterPaths.BinPath))
				}
				if _, err := os.Stat(clusterPaths.VenvPath); err == nil {
					activateScript := fmt.Sprintf("%s/bin/activate.fish", clusterPaths.VenvPath)
					if _, err := os.Stat(activateScript); err == nil {
						output.WriteString(fmt.Sprintf("source %s\n", activateScript))
					}
				}

			case "powershell":
				output.WriteString(fmt.Sprintf("$env:OPENCENTER_CLUSTER = '%s'\n", clusterName))
				if _, err := os.Stat(clusterPaths.KubeconfigPath); err == nil {
					output.WriteString(fmt.Sprintf("$env:KUBECONFIG = '%s'\n", clusterPaths.KubeconfigPath))
				}
				if _, err := os.Stat(clusterPaths.InventoryPath); err == nil {
					output.WriteString(fmt.Sprintf("$env:ANSIBLE_INVENTORY = '%s'\n", clusterPaths.InventoryPath))
				}
				if _, err := os.Stat(clusterPaths.BinPath); err == nil {
					output.WriteString(fmt.Sprintf("$env:PATH = '%s;' + $env:PATH\n", clusterPaths.BinPath))
				}
				if _, err := os.Stat(clusterPaths.VenvPath); err == nil {
					activateScript := fmt.Sprintf("%s\\Scripts\\Activate.ps1", clusterPaths.VenvPath)
					if _, err := os.Stat(activateScript); err == nil {
						output.WriteString(fmt.Sprintf(". %s\n", activateScript))
					}
				}

			default:
				// Bash/Zsh syntax
				output.WriteString(fmt.Sprintf("export OPENCENTER_CLUSTER=%s\n", clusterName))
				if _, err := os.Stat(clusterPaths.KubeconfigPath); err == nil {
					output.WriteString(fmt.Sprintf("export KUBECONFIG=%s\n", clusterPaths.KubeconfigPath))
				}
				if _, err := os.Stat(clusterPaths.InventoryPath); err == nil {
					output.WriteString(fmt.Sprintf("export ANSIBLE_INVENTORY=%s\n", clusterPaths.InventoryPath))
				}
				if _, err := os.Stat(clusterPaths.BinPath); err == nil {
					output.WriteString(fmt.Sprintf("export PATH=%s:$PATH\n", clusterPaths.BinPath))
				}
				if _, err := os.Stat(clusterPaths.VenvPath); err == nil {
					activateScript := fmt.Sprintf("%s/bin/activate", clusterPaths.VenvPath)
					if _, err := os.Stat(activateScript); err == nil {
						output.WriteString(fmt.Sprintf("source %s\n", activateScript))
					}
				}
			}

			// Output the commands
			fmt.Fprint(cmd.OutOrStdout(), output.String())

			return nil
		},
	}

	// Add flag to override shell detection
	cmd.Flags().StringVar(&shellOverride, "shell", "", "Override shell detection (bash, zsh, fish, powershell)")

	return cmd
}

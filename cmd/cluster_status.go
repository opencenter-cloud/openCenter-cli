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

	"github.com/spf13/cobra"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
)

// newClusterStatusCmd creates the "cluster status" command.
func newClusterStatusCmd() *cobra.Command {
	var showPaths bool
	var quiet bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the current active cluster and its status",
		Long: `Show the current active cluster and its status information.

This command displays:
- The currently active cluster (if any)
- Basic cluster metadata (environment, region, organization)
- Cluster status (initialized, validated, deployed, etc.)
- Key file paths (with --paths flag)

If no cluster is active, it will show available clusters and suggest
using 'opencenter cluster select' to set one.`,
		Example: `  # Show active cluster status
  opencenter cluster status

  # Show active cluster with file paths
  opencenter cluster status --paths

  # Quiet output (just the cluster name)
  opencenter cluster status --quiet`,
		RunE: func(cmd *cobra.Command, args []string) error {
			activeCluster, err := config.GetActive()
			if err != nil {
				return fmt.Errorf("failed to get active cluster: %w", err)
			}

			if activeCluster == "" {
				if quiet {
					// In quiet mode, output nothing when no active cluster
					return nil
				}

				fmt.Fprintf(cmd.OutOrStdout(), "No active cluster set\n\n")

				// Show available clusters
				clusters, listErr := config.List()
				if listErr == nil && len(clusters) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Available clusters:\n")
					for _, cluster := range clusters {
						fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", cluster)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "\nUse 'opencenter cluster select <name>' to set an active cluster\n")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "No clusters found. Use 'opencenter cluster init <name>' to create one.\n")
				}
				return nil
			}

			// Quiet mode: just output the cluster name
			if quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", activeCluster)
				return nil
			}

			// Load cluster configuration to get detailed information
			cfg, err := config.Load(activeCluster)
			if err != nil {
				// If we can't load the config, still show basic info
				fmt.Fprintf(cmd.OutOrStdout(), "Active cluster: %s\n", activeCluster)
				fmt.Fprintf(cmd.OutOrStdout(), "Status: Configuration not found or invalid\n")
				return nil
			}

			// Display cluster information
			fmt.Fprintf(cmd.OutOrStdout(), "Active Cluster: %s\n", activeCluster)
			fmt.Fprintf(cmd.OutOrStdout(), "  Name:         %s\n", cfg.OpenCenter.Meta.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "  Environment:  %s\n", cfg.OpenCenter.Meta.Env)
			fmt.Fprintf(cmd.OutOrStdout(), "  Region:       %s\n", cfg.OpenCenter.Meta.Region)
			fmt.Fprintf(cmd.OutOrStdout(), "  Status:       %s\n", cfg.OpenCenter.Meta.Status)
			fmt.Fprintf(cmd.OutOrStdout(), "  Organization: %s\n", cfg.OpenCenter.Meta.Organization)
			fmt.Fprintf(cmd.OutOrStdout(), "  Provider:     %s\n", cfg.OpenCenter.Infrastructure.Provider)

			// Show paths if requested
			if showPaths {
				fmt.Fprintf(cmd.OutOrStdout(), "\nCluster Paths:\n")

				// Get configuration manager and path resolver
				configManager, err := config.NewConfigManager("")
				if err == nil {
					// Get base directory for clusters
					baseDir := configManager.GetConfig().Paths.ClustersDir
					if baseDir == "" {
						homeDir, err := os.UserHomeDir()
						if err == nil {
							baseDir = filepath.Join(homeDir, ".config", "opencenter", "clusters")
						}
					}

					if baseDir != "" {
						pathResolver := paths.NewPathResolver(baseDir)

						// Parse cluster identifier to get organization
						organization, clusterName, parseErr := config.ParseClusterIdentifier(activeCluster)
						if parseErr == nil {
							// Use organization from config if available, otherwise use parsed
							if cfg.OpenCenter.Meta.Organization != "" {
								organization = cfg.OpenCenter.Meta.Organization
							}

							// Resolve cluster paths
							ctx := context.Background()
							clusterPaths, err := pathResolver.Resolve(ctx, clusterName, organization)
							if err == nil {
								fmt.Fprintf(cmd.OutOrStdout(), "  Config Directory:  %s\n", clusterPaths.ClusterDir)
								fmt.Fprintf(cmd.OutOrStdout(), "  SOPS Key:          %s\n", clusterPaths.SOPSKeyPath)
								fmt.Fprintf(cmd.OutOrStdout(), "  GitOps Directory:  %s\n", clusterPaths.GitOpsDir)

								// Check if key files exist
								if _, err := os.Stat(clusterPaths.SOPSKeyPath); err == nil {
									fmt.Fprintf(cmd.OutOrStdout(), "  SOPS Key Status:   ✓ Present\n")
								} else {
									fmt.Fprintf(cmd.OutOrStdout(), "  SOPS Key Status:   ✗ Missing\n")
								}

								// Check if GitOps directory exists
								if _, err := os.Stat(clusterPaths.GitOpsDir); err == nil {
									fmt.Fprintf(cmd.OutOrStdout(), "  GitOps Status:     ✓ Initialized\n")
								} else {
									fmt.Fprintf(cmd.OutOrStdout(), "  GitOps Status:     ✗ Not initialized\n")
								}

								// Check for kubeconfig
								if _, err := os.Stat(clusterPaths.KubeconfigPath); err == nil {
									fmt.Fprintf(cmd.OutOrStdout(), "  Kubeconfig:        ✓ Present\n")
								} else {
									fmt.Fprintf(cmd.OutOrStdout(), "  Kubeconfig:        ✗ Missing\n")
								}
							}
						}
					}
				}
			}

			// Show next steps based on status
			status := strings.ToLower(cfg.OpenCenter.Meta.Status)
			fmt.Fprintf(cmd.OutOrStdout(), "\nNext Steps:\n")
			switch status {
			case "initialized", "":
				fmt.Fprintf(cmd.OutOrStdout(), "  - Run 'opencenter cluster validate %s' to validate configuration\n", activeCluster)
				fmt.Fprintf(cmd.OutOrStdout(), "  - Run 'opencenter cluster setup %s' to generate GitOps repository\n", activeCluster)
			case "validated":
				fmt.Fprintf(cmd.OutOrStdout(), "  - Run 'opencenter cluster setup %s' to generate GitOps repository\n", activeCluster)
			case "setup", "ready":
				fmt.Fprintf(cmd.OutOrStdout(), "  - Run 'opencenter cluster bootstrap %s' to deploy the cluster\n", activeCluster)
			case "deployed":
				fmt.Fprintf(cmd.OutOrStdout(), "  - Run 'eval $(opencenter cluster activate)' to configure your environment\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  - Use 'kubectl' to interact with the cluster\n")
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "  - Check cluster documentation for next steps\n")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&showPaths, "paths", false, "show cluster file paths and their status")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "quiet output (just the cluster name)")

	return cmd
}

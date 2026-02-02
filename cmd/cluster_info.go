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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/resilience"
	"github.com/spf13/cobra"
)

func newClusterInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info [name]",
		Short: "Show configuration for a cluster",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name from args or active cluster
			name, err := resolveClusterName(args, false)
			if err != nil {
				return err
			}

			var isActiveCluster bool
			if len(args) == 0 {
				isActiveCluster = true
			}

			cfg, err := config.Load(name)
			if err != nil {
				return err
			}

			// Handle --export-only flag
			exportOnly, _ := cmd.Flags().GetBool("export-only")
			if exportOnly {
				shellOverride, _ := cmd.Flags().GetString("shell")
				return handleExportOnly(cmd, name, shellOverride)
			}

			// Handle --validate flag
			validate, _ := cmd.Flags().GetBool("validate")
			if validate {
				errs := config.Validate(cfg)
				if len(errs) > 0 {
					for _, e := range errs {
						fmt.Fprintln(cmd.ErrOrStderr(), e)
					}
					return fmt.Errorf("validation failed")
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Validation successful.")
				return nil
			}

			// Get the full path to the config file
			configPath, err := config.ConfigPath(name)
			if err != nil {
				return fmt.Errorf("failed to resolve config path: %w", err)
			}

			// Check if we're in the git directory to show "Active cluster" prefix
			isInGitDir := false
			if cfg.OpenCenter.GitOps.GitDir != "" {
				cwd, err := os.Getwd()
				if err == nil {
					gitDir := cfg.OpenCenter.GitOps.GitDir
					// Expand tilde and environment variables
					if strings.HasPrefix(gitDir, "~/") {
						if home, err := os.UserHomeDir(); err == nil {
							gitDir = filepath.Join(home, gitDir[2:])
						}
					}
					gitDir = os.ExpandEnv(gitDir)

					if absGitDir, err := filepath.Abs(gitDir); err == nil {
						if absCwd, err := filepath.Abs(cwd); err == nil {
							isInGitDir = (absCwd == absGitDir)
						}
					}
				}
			}

			// Output format
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				// Print full config in JSON format including cluster_name
				output := map[string]any{
					"config_path":  configPath,
					"cluster_name": cfg.OpenCenter.Cluster.ClusterName,
					"organization": cfg.OpenCenter.Meta.Organization,
					"provider":     cfg.OpenCenter.Infrastructure.Provider,
					"metadata":     cfg.OpenCenter.Meta,
					"git_dir":      cfg.OpenCenter.GitOps.GitDir,
					"git_url":      cfg.OpenCenter.GitOps.GitURL,
				}
				b, err := json.MarshalIndent(output, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}

			// Print metadata and config path in human-readable format
			// Show "Active cluster:" if this is the active cluster or we're in the git directory
			if isActiveCluster || isInGitDir {
				fmt.Fprintf(cmd.OutOrStdout(), "Active cluster: %s\n", name)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Cluster: %s\n", name)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Config Path: %s\n\n", configPath)

			// Print GitOps configuration
			if cfg.OpenCenter.GitOps.GitDir != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "git_dir: %s\n", cfg.OpenCenter.GitOps.GitDir)
			}
			if cfg.OpenCenter.GitOps.GitURL != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "git_url: %s\n", cfg.OpenCenter.GitOps.GitURL)
			}
			fmt.Fprintln(cmd.OutOrStdout())

			fmt.Fprintln(cmd.OutOrStdout(), "Metadata:")

			// Create a combined metadata output that includes both Meta and cluster_name
			metadataOutput := map[string]any{
				"name":         cfg.OpenCenter.Meta.Name,
				"cluster_name": cfg.OpenCenter.Cluster.ClusterName,
				"organization": cfg.OpenCenter.Meta.Organization,
				"provider":     cfg.OpenCenter.Infrastructure.Provider,
				"env":          cfg.OpenCenter.Meta.Env,
				"region":       cfg.OpenCenter.Meta.Region,
				"status":       cfg.OpenCenter.Meta.Status,
			}

			data, err := yaml.Marshal(metadataOutput)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), string(data))

			// Check lock status with detailed information
			lockMgr, err := resilience.NewLockManager(resilience.DefaultLockConfig)
			if err == nil {
				lockInfo, err := lockMgr.GetLockInfo(name)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "\nWarning: Failed to read lock info: %v\n", err)
				} else if lockInfo != nil {
					// Lock exists - show detailed information
					fmt.Fprintln(cmd.OutOrStdout(), "\nLock Status:")
					fmt.Fprintln(cmd.OutOrStdout(), "  status: locked")

					// Show operation if available
					if operation, ok := lockInfo.Metadata["operation"]; ok {
						fmt.Fprintf(cmd.OutOrStdout(), "  operation: %s\n", operation)
					}
					if command, ok := lockInfo.Metadata["command"]; ok {
						fmt.Fprintf(cmd.OutOrStdout(), "  command: %s\n", command)
					}

					// Parse owner to extract hostname and PID
					owner := lockInfo.Owner
					hostname := owner
					pid := ""
					if colonIdx := -1; colonIdx < len(owner) {
						for i := 0; i < len(owner); i++ {
							if owner[i] == ':' {
								colonIdx = i
								break
							}
						}
						if colonIdx > 0 {
							hostname = owner[:colonIdx]
							pid = owner[colonIdx+1:]
						}
					}

					fmt.Fprintf(cmd.OutOrStdout(), "  owner: %s\n", owner)
					if hostname != owner {
						fmt.Fprintf(cmd.OutOrStdout(), "  hostname: %s\n", hostname)
						fmt.Fprintf(cmd.OutOrStdout(), "  pid: %s\n", pid)
					}

					// Show timestamps
					if !lockInfo.AcquiredAt.IsZero() {
						fmt.Fprintf(cmd.OutOrStdout(), "  acquired_at: %s\n", lockInfo.AcquiredAt.Format(time.RFC3339))
						fmt.Fprintf(cmd.OutOrStdout(), "  acquired_ago: %s\n", time.Since(lockInfo.AcquiredAt).Round(time.Second))
					}

					if !lockInfo.ExpiresAt.IsZero() {
						fmt.Fprintf(cmd.OutOrStdout(), "  expires_at: %s\n", lockInfo.ExpiresAt.Format(time.RFC3339))
						if time.Now().Before(lockInfo.ExpiresAt) {
							fmt.Fprintf(cmd.OutOrStdout(), "  expires_in: %s\n", time.Until(lockInfo.ExpiresAt).Round(time.Second))
						} else {
							fmt.Fprintln(cmd.OutOrStdout(), "  expires_in: expired (stale lock)")
						}
					}

					if lockInfo.TTL > 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "  ttl: %s\n", lockInfo.TTL)
					}

					// Add helpful message
					if operation, ok := lockInfo.Metadata["operation"]; ok {
						fmt.Fprintf(cmd.OutOrStdout(), "  message: %s operation is in progress on this cluster\n", operation)
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), "  message: Another operation is in progress on this cluster")
					}

					// Check if process is still running (on Unix-like systems)
					if pid != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "  hint: Check if process %s is still running, or remove stale lock file\n", pid)
					}
				} else {
					// No lock exists
					fmt.Fprintln(cmd.OutOrStdout(), "\nLock Status:")
					fmt.Fprintln(cmd.OutOrStdout(), "  status: available")
				}
			}

			return nil
		},
	}
	cmd.Flags().Bool("validate", false, "validate cluster configuration invariants")
	cmd.Flags().Bool("json", false, "output JSON instead of YAML")
	cmd.Flags().Bool("export-only", false, "only output export commands for shell evaluation")
	cmd.Flags().String("shell", "", "override shell detection (bash, zsh, fish, powershell)")
	return cmd
}

// handleExportOnly handles the --export-only flag for cluster info command.
func handleExportOnly(cmd *cobra.Command, clusterName string, shellOverride string) error {
	// Generate cluster select output to get export commands
	output, err := generateClusterSelectOutput(clusterName, shellOverride)
	if err != nil {
		return err
	}

	// Only output export commands
	for _, command := range output.ExportCommands {
		fmt.Fprintln(cmd.OutOrStdout(), command)
	}

	return nil
}

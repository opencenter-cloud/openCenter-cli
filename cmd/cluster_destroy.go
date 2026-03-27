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
	"runtime"
	"strings"
	"time"

	kindprovider "github.com/opencenter-cloud/opencenter-cli/internal/cloud/kind"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/resilience"
	"github.com/opencenter-cloud/opencenter-cli/internal/ui"
	"github.com/spf13/cobra"
)

func newClusterDestroyCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "destroy [name]",
		Short: "Destroy a cluster",
		Long: `Destroy a cluster and remove its configuration.

This command removes the cluster configuration and optionally its GitOps directory.
The cluster name can be specified as 'cluster' or 'organization/cluster'.

If no cluster name is provided, the active cluster will be destroyed.`,
		Example: `  # Destroy a specific cluster
  opencenter cluster destroy my-cluster

  # Destroy cluster in specific organization
  opencenter cluster destroy myorg/my-cluster

  # Destroy without confirmation
  opencenter cluster destroy my-cluster --force

  # Destroy active cluster
  opencenter cluster destroy`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name from args or active cluster
			name, err := resolveClusterName(args, true)
			if err != nil {
				return err
			}

			// Acquire lock for destroy operation
			lockMgr, err := resilience.NewLockManager(resilience.DefaultLockConfig)
			if err != nil {
				return fmt.Errorf("failed to create lock manager: %w", err)
			}

			ctx := cmd.Context()
			lock, err := lockMgr.AcquireWithMetadata(ctx, name, 1*time.Hour, map[string]string{
				"operation": "destroy",
				"command":   "cluster destroy",
			})
			if err != nil {
				return fmt.Errorf("failed to acquire lock for cluster %q: %w\nAnother operation may be in progress. Wait for it to complete or use 'opencenter cluster info %s' to check lock status", name, err, name)
			}
			defer lockMgr.Release(lock)

			// Load cluster configuration
			cfg, err := loadCanonicalConfig(name)
			if err != nil {
				return err
			}

			// Get cluster name and organization
			clusterName := cfg.ClusterName()
			organization := cfg.OpenCenter.Meta.Organization
			if organization == "" {
				organization = "opencenter"
			}

			// Confirmation prompt unless --force is used
			if !force {
				// Create appropriate prompter based on environment
				testMode := os.Getenv("OPENCENTER_TEST_MODE") != ""
				prompter := ui.GetPrompter(os.Stdin, cmd.OutOrStdout(), testMode)

				// Build confirmation message
				message := fmt.Sprintf("WARNING: This will permanently destroy cluster %q", clusterName)
				if organization != "" && organization != "opencenter" {
					message += fmt.Sprintf(" in organization %q", organization)
				}
				message += ". Are you sure?"

				// Prompt for confirmation
				confirmed, err := prompter.Confirm(ctx, message)
				if err != nil {
					return fmt.Errorf("confirmation prompt failed: %w", err)
				}
				if !confirmed {
					fmt.Fprintf(cmd.OutOrStdout(), "Destroy operation cancelled.\n")
					return nil
				}
			}

			if strings.EqualFold(cfg.OpenCenter.Infrastructure.Provider, "kind") {
				if err := destroyKindCluster(ctx, cfg); err != nil {
					return err
				}
			}

			// Update cluster status to "destroyed" before removal (skip for flat configs)
			// Get default config directory
			configDir := os.Getenv("OPENCENTER_CONFIG_DIR")
			if configDir == "" {
				if runtime.GOOS == "windows" {
					base := os.Getenv("APPDATA")
					if base == "" {
						base = os.Getenv("LOCALAPPDATA")
					}
					if base == "" {
						base = os.Getenv("USERPROFILE")
					}
					configDir = filepath.Join(base, "opencenter")
				} else {
					if home, err := os.UserHomeDir(); err == nil {
						configDir = filepath.Join(home, ".config", "opencenter")
					}
				}
			}

			// Extract just the cluster name (without organization prefix)
			actualClusterName := extractClusterName(name)
			configPath, err := getConfigPath(ctx, actualClusterName, cfg.OpenCenter.Meta.Organization)
			if err != nil {
				return fmt.Errorf("failed to resolve config path: %w", err)
			}

			resolvedConfigDir := os.Getenv("OPENCENTER_CONFIG_DIR")
			if resolvedConfigDir == "" {
				if runtime.GOOS == "windows" {
					base := os.Getenv("APPDATA")
					if base == "" {
						base = os.Getenv("LOCALAPPDATA")
					}
					if base == "" {
						base = os.Getenv("USERPROFILE")
					}
					resolvedConfigDir = filepath.Join(base, "opencenter")
				} else {
					home, err := os.UserHomeDir()
					if err != nil {
						return fmt.Errorf("failed to get home directory: %w", err)
					}
					resolvedConfigDir = filepath.Join(home, ".config", "opencenter")
				}
			}

			// Check if this is a flat config file (not in clusters directory)
			isFlatConfig := filepath.Dir(configPath) == resolvedConfigDir

			var clusterPaths *paths.ClusterPaths
			if !isFlatConfig {
				pathResolver := paths.NewPathResolver(config.ResolveClustersDir())
				clusterPaths, _ = pathResolver.Resolve(context.Background(), clusterName, organization)
			}

			if configDir != "" {
				if filepath.Dir(configPath) != configDir {
					// Not a flat config, safe to update status
					cfg.OpenCenter.Meta.Status = "destroyed"
					if err := saveConfig(ctx, cfg); err != nil {
						// Log warning but continue with destroy
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", err)
					}
				}
			}

			// Remove GitOps directory if specified
			gitopsDir := cfg.GitOps().GitDir
			if gitopsDir != "" {
				if err := os.RemoveAll(gitopsDir); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("failed to remove gitops directory: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Removed GitOps directory: %s\n", gitopsDir)
			}

			if !isFlatConfig {
				if clusterPaths != nil {
					// Organization-based structure found
					// Remove cluster directory: clusters/<org>/infrastructure/clusters/<cluster>/
					if err := os.RemoveAll(clusterPaths.ClusterDir); err != nil && !os.IsNotExist(err) {
						return fmt.Errorf("failed to remove cluster directory: %w", err)
					}
					if _, statErr := os.Stat(clusterPaths.ClusterDir); os.IsNotExist(statErr) {
						fmt.Fprintf(cmd.OutOrStdout(), "Removed cluster directory: %s\n", clusterPaths.ClusterDir)
					}

					// Remove applications directory: clusters/<org>/applications/overlays/<cluster>/
					if err := os.RemoveAll(clusterPaths.ApplicationsDir); err != nil && !os.IsNotExist(err) {
						return fmt.Errorf("failed to remove applications directory: %w", err)
					}
					if _, statErr := os.Stat(clusterPaths.ApplicationsDir); os.IsNotExist(statErr) {
						fmt.Fprintf(cmd.OutOrStdout(), "Removed applications directory: %s\n", clusterPaths.ApplicationsDir)
					}
				}
			}
			// For flat config files, we only remove the config file itself (no cluster directory)

			// Remove the config file
			if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to delete config file: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed config file: %s\n", configPath)

			// Remove from active cluster if it was active
			activeCluster, err := getActiveCluster()
			if err == nil && activeCluster == name {
				if err := setActiveCluster(""); err != nil {
					// Log warning but don't fail
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to clear active cluster: %v\n", err)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Cleared active cluster marker\n")
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Cluster %q destroyed successfully.\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")

	return cmd
}

func destroyKindCluster(ctx context.Context, cfg config.Config) error {
	runtime := ""
	if cfg.OpenCenter.Infrastructure.Kind != nil {
		runtime = cfg.OpenCenter.Infrastructure.Kind.Runtime
	}

	clusterName := cfg.ClusterName()
	if cfg.OpenCenter.Infrastructure.Kind != nil {
		if override := strings.TrimSpace(cfg.OpenCenter.Infrastructure.Kind.ClusterNameOverride); override != "" {
			clusterName = override
		}
	}

	if err := kindprovider.NewProvider().DeleteCluster(ctx, clusterName, kindprovider.BuildEnvironment(runtime)); err != nil {
		return fmt.Errorf("failed to destroy kind cluster %q: %w", clusterName, err)
	}

	return nil
}

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
	"github.com/opencenter-cloud/opencenter-cli/internal/cluster"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/ui"
	"github.com/spf13/cobra"
)

func newClusterDestroyCmd() *cobra.Command {
	var (
		force              bool
		removeFiles        bool
		skipInfrastructure bool
	)

	cmd := &cobra.Command{
		Use:   "destroy [name]",
		Short: "Destroy a cluster",
		Long: `Destroy a cluster's infrastructure and optionally remove its configuration files.

This command first destroys the cloud infrastructure (via OpenTofu for supported providers),
then optionally removes local configuration files and GitOps directories.

By default, local files are preserved after infrastructure destruction to allow for
inspection, debugging, or recovery. Use --remove-files to also delete local files.

The cluster name can be specified as 'cluster' or 'organization/cluster'.
If no cluster name is provided, the active cluster will be destroyed.

If an existing lock is found, you will be prompted to break it. Use --break-lock
to automatically break any existing lock without prompting.`,
		Example: `  # Destroy infrastructure only (keep local files for inspection)
  opencenter cluster destroy my-cluster --force

  # Destroy infrastructure AND remove all local files
  opencenter cluster destroy my-cluster --force --remove-files

  # Skip infrastructure destruction (just remove local files)
  opencenter cluster destroy my-cluster --force --skip-infrastructure --remove-files

  # Destroy cluster in specific organization
  opencenter cluster destroy myorg/my-cluster --force

  # Destroy and break any existing lock without prompting
  opencenter cluster destroy my-cluster --force --break-lock

  # Destroy active cluster
  opencenter cluster destroy --force`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name from args or active cluster
			name, err := resolveClusterNameForCommand(cmd, args, true)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// Acquire lock for destroy operation (with prompt if lock exists)
			lockResult, err := AcquireLockWithPrompt(ctx, cmd, name, "destroy", 1*time.Hour, map[string]string{
				"operation": "destroy",
				"command":   "cluster destroy",
			})
			if err != nil {
				return err
			}
			// Ensure lock is released on success and lock file is removed
			defer func() {
				if lockResult.Lock != nil {
					lockResult.LockManager.Release(lockResult.Lock)
					// Force break to ensure lock file is removed after successful destroy
					_ = lockResult.LockManager.ForceBreak(name)
				}
			}()

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

			// Destroy infrastructure (unless skipped)
			if !skipInfrastructure {
				if err := destroyClusterInfrastructure(ctx, cmd, cfg); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Skipping infrastructure destruction (--skip-infrastructure)\n")
			}

			// If --remove-files is not set, we're done
			if !removeFiles {
				fmt.Fprintf(cmd.OutOrStdout(), "\nInfrastructure destroyed. Local files preserved.\n")
				fmt.Fprintf(cmd.OutOrStdout(), "Use --remove-files to also delete local configuration and GitOps files.\n")
				return nil
			}

			// Remove local files
			return removeClusterFiles(ctx, cmd, cfg, name, clusterName, organization)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&removeFiles, "remove-files", false, "Remove local configuration and GitOps files after infrastructure destruction")
	cmd.Flags().BoolVar(&skipInfrastructure, "skip-infrastructure", false, "Skip infrastructure destruction (only remove local files when combined with --remove-files)")

	return cmd
}

// destroyClusterInfrastructure handles infrastructure destruction based on provider type.
func destroyClusterInfrastructure(ctx context.Context, cmd *cobra.Command, cfg v2.Config) error {
	provider := strings.ToLower(cfg.Provider())

	// Handle Kind clusters
	if provider == "kind" {
		fmt.Fprintf(cmd.OutOrStdout(), "Destroying Kind cluster...\n")
		if err := destroyKindCluster(ctx, cfg); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Kind cluster destroyed.\n")
		return nil
	}

	// Handle OpenTofu-based providers
	destroyService := cluster.NewDestroyService(cmd.OutOrStdout())
	if destroyService.SupportsInfraDestroy(&cfg) {
		fmt.Fprintf(cmd.OutOrStdout(), "Destroying infrastructure via OpenTofu...\n")
		result, err := destroyService.DestroyInfrastructure(ctx, &cfg, &cluster.DestroyOptions{
			AutoApprove: true,
		})
		if err != nil {
			return fmt.Errorf("infrastructure destruction failed: %w", err)
		}
		if result.InfraDestroyed {
			fmt.Fprintf(cmd.OutOrStdout(), "Infrastructure destroyed successfully.\n")
		}
		return nil
	}

	// Provider doesn't have infrastructure to destroy
	fmt.Fprintf(cmd.OutOrStdout(), "Provider %q does not have infrastructure to destroy.\n", provider)
	return nil
}

// removeClusterFiles removes local configuration and GitOps files.
func removeClusterFiles(ctx context.Context, cmd *cobra.Command, cfg v2.Config, name, clusterName, organization string) error {
	// Update cluster status to "destroyed" before removal (skip for flat configs)
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
		pathResolver := config.NewPathResolverFromConfig()
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
	gitopsDir := cfg.GitDir()
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
}

func destroyKindCluster(ctx context.Context, cfg v2.Config) error {
	clusterName := cfg.ClusterName()
	if err := kindprovider.NewProvider().DeleteCluster(ctx, clusterName, kindprovider.BuildEnvironment("")); err != nil {
		return fmt.Errorf("failed to destroy kind cluster %q: %w", clusterName, err)
	}

	return nil
}

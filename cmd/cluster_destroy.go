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
	"github.com/opencenter-cloud/opencenter-cli/internal/logging"
	"github.com/opencenter-cloud/opencenter-cli/internal/ui"
	"github.com/spf13/cobra"
)

func newClusterDestroyCmd() *cobra.Command {
	var (
		force                bool
		removeFiles          bool
		skipInfrastructure   bool
		deleteVolumes        bool
		strict               bool
		requireClusterAccess bool
		skipClusterAccess    bool
		allowCleanupFailures bool
		destroyRetries       int
		destroyParallelism   int
	)

	cmd := &cobra.Command{
		Use:   "destroy [name]",
		Short: "Destroy a cluster",
		Long: `Destroy a cluster's infrastructure and optionally remove its configuration files.

This command first destroys the cloud infrastructure (via OpenTofu for supported providers),
then optionally removes this cluster's local configuration files and its
trees inside the GitOps worktree.

By default, local files are preserved after infrastructure destruction to allow for
inspection, debugging, or recovery. Use --remove-files to also delete local files.
The GitOps worktree (gitops.repository.local_dir) is shared by every cluster in
the organization: --remove-files deletes only the designated cluster's
directories and never the shared worktree or another cluster's files.

The cluster name can be specified as 'cluster' or 'organization/cluster'.
If no cluster name is provided, the active cluster will be destroyed.

If an existing lock is found, you will be prompted to break it. Use --break-lock
to automatically break any existing lock without prompting.

When the cluster API endpoint is unreachable (decommissioned cluster, VPN down),
use --skip-cluster-access to omit the kubectl CSI volume-capture step entirely,
so destroy never blocks on the dead endpoint. Orphaned Cinder volumes then need
manual cleanup.`,
		Example: `  # Destroy infrastructure only (keep local files for inspection)
  opencenter cluster destroy my-cluster --force

  # Destroy infrastructure AND remove all local files
  opencenter cluster destroy my-cluster --force --remove-files

  # Skip infrastructure destruction (just remove local files)
  opencenter cluster destroy my-cluster --force --skip-infrastructure --remove-files

  # Destroy with an unreachable cluster API endpoint (skip kubectl capture)
  opencenter cluster destroy my-cluster --force --skip-cluster-access --remove-files

  # Destroy cluster in specific organization
  opencenter cluster destroy myorg/my-cluster --force

  # Destroy and break any existing lock without prompting
  opencenter cluster destroy my-cluster --force --break-lock

  # Destroy active cluster
  opencenter cluster destroy --force`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Apply log-level flag
			if logLevel, _ := cmd.Flags().GetString("log-level"); logLevel != "" {
				_ = logging.SetLogLevel(logLevel)
			}

			// --skip-cluster-access and --require-cluster-access are exact
			// opposites; reject before touching the lock or the cluster.
			if skipClusterAccess && requireClusterAccess {
				return fmt.Errorf("--skip-cluster-access and --require-cluster-access are mutually exclusive: one skips the cluster API, the other requires it")
			}

			// Resolve cluster name from args or active cluster
			name, err := resolveClusterNameForCommand(cmd, args, true)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

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

			// Set up destroy logging
			customLogPath, _ := cmd.Flags().GetString("log")
			destroyLogPath, err := cluster.ResolveDestroyLogPath(&cfg, customLogPath, time.Now())
			if err != nil {
				return fmt.Errorf("resolving destroy log path: %w", err)
			}

			logFile, err := cluster.OpenActionLogFile(destroyLogPath)
			if err != nil {
				return fmt.Errorf("opening destroy log file: %w", err)
			}
			if logFile != nil {
				defer logFile.Close()
				ctx = logging.WithActionLogWriter(ctx, logFile)
			}

			logging.LogActionMessage(ctx, "destroy started for %s/%s", organization, clusterName)
			logging.LogActionMessage(ctx, "options: force=%v, remove_files=%v, skip_infrastructure=%v, delete_volumes=%v, require_cluster_access=%v, skip_cluster_access=%v, allow_cleanup_failures=%v, strict=%v, destroy_retries=%d, destroy_parallelism=%d",
				force, removeFiles, skipInfrastructure, deleteVolumes, requireClusterAccess, skipClusterAccess, allowCleanupFailures, strict, destroyRetries, destroyParallelism)
			logging.Debugf("destroy: log file at %s", destroyLogPath)

			// Destroy infrastructure (unless skipped)
			var cleanupWarnings []error
			if !skipInfrastructure {
				warnings, err := destroyClusterInfrastructure(ctx, cmd, cfg, destroyOptions{
					deleteVolumes:        deleteVolumes,
					requireClusterAccess: requireClusterAccess,
					skipClusterAccess:    skipClusterAccess,
					allowCleanupFailures: allowCleanupFailures,
					retries:              destroyRetries,
					parallelism:          destroyParallelism,
				})
				if err != nil {
					logging.LogActionMessage(ctx, "destroy failed during infrastructure destruction: %v", err)
					if destroyLogPath != "" {
						fmt.Fprintf(cmd.ErrOrStderr(), "Destroy log: %s\n", destroyLogPath)
					}
					return err
				}
				cleanupWarnings = warnings
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Skipping infrastructure destruction (--skip-infrastructure)\n")
				logging.LogActionMessage(ctx, "skipped infrastructure destruction (--skip-infrastructure)")
			}

			// Surface non-fatal cleanup degradations (leaked LBs, orphaned
			// volumes, skipped CSI capture) so they are never silent.
			if err := reportDestroyCleanupWarnings(cmd, ctx, cleanupWarnings, strict); err != nil {
				if destroyLogPath != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "Destroy log: %s\n", destroyLogPath)
				}
				return err
			}

			// If --remove-files is not set, we're done
			if !removeFiles {
				logging.LogActionMessage(ctx, "infrastructure destroyed; local files preserved")
				fmt.Fprintf(cmd.OutOrStdout(), "\nInfrastructure destroyed. Local files preserved.\n")
				fmt.Fprintf(cmd.OutOrStdout(), "Use --remove-files to also delete local configuration and GitOps files.\n")
				if destroyLogPath != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "Destroy log: %s\n", destroyLogPath)
				}
				return nil
			}

			// Remove local files
			if err := removeClusterFiles(ctx, cmd, cfg, name, clusterName, organization); err != nil {
				logging.LogActionMessage(ctx, "destroy failed during file removal: %v", err)
				if destroyLogPath != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "Destroy log: %s\n", destroyLogPath)
				}
				return err
			}

			logging.LogActionMessage(ctx, "destroy complete for %s/%s", organization, clusterName)
			if destroyLogPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Destroy log: %s\n", destroyLogPath)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&removeFiles, "remove-files", false, "Remove this cluster's local configuration and GitOps files after infrastructure destruction (the shared organization GitOps worktree and other clusters' files are preserved)")
	cmd.Flags().BoolVar(&skipInfrastructure, "skip-infrastructure", false, "Skip infrastructure destruction (only remove local files when combined with --remove-files)")
	cmd.Flags().BoolVar(&deleteVolumes, "delete-volumes", false, "Delete CSI-provisioned Cinder volumes left orphaned after infrastructure destruction (OpenStack only)")
	cmd.Flags().BoolVar(&strict, "strict", false, "Exit with an error when post-destroy cleanup reported warnings (leaked load balancers, orphaned volumes)")
	cmd.Flags().BoolVar(&requireClusterAccess, "require-cluster-access", false, "Abort before destroying infrastructure if the cluster kubeconfig is missing or unreachable (CSI volume capture cannot be skipped)")
	cmd.Flags().BoolVar(&skipClusterAccess, "skip-cluster-access", false, "Never contact the cluster API during destroy (skips the kubectl CSI volume capture); use when the kube endpoint is unreachable (incompatible with --require-cluster-access)")
	cmd.Flags().BoolVar(&allowCleanupFailures, "allow-cleanup-failures", false, "Continue when pre-destroy cleanup steps (load balancer sweep) fail; leaked resources are reported as warnings")
	cmd.Flags().IntVar(&destroyRetries, "destroy-retries", 1, "Additional attempts for the OpenTofu destroy step after a transient failure (0 disables retries)")
	cmd.Flags().IntVar(&destroyParallelism, "destroy-parallelism", 0, "OpenTofu destroy -parallelism value (0 keeps the OpenTofu default)")
	cmd.Flags().String("log", "", "log file path (defaults to <state_dir>/logs/destroy/<org>/<name>/destroy-YYYYMMDDTHHMMSSZ.log)")

	return cmd
}

// destroyOptions carries the infrastructure-destruction knobs from the CLI
// flags into destroyClusterInfrastructure.
type destroyOptions struct {
	deleteVolumes        bool
	requireClusterAccess bool
	skipClusterAccess    bool
	allowCleanupFailures bool
	retries              int
	parallelism          int
}

// destroyClusterInfrastructure handles infrastructure destruction based on
// provider type. It returns the non-fatal cleanup warnings collected during
// destruction (leaked load balancers, orphaned volumes, skipped CSI capture)
// alongside the fatal error, if any.
func destroyClusterInfrastructure(ctx context.Context, cmd *cobra.Command, cfg v2.Config, opts destroyOptions) ([]error, error) {
	provider := strings.ToLower(cfg.Provider())

	// Handle Kind clusters
	if provider == "kind" {
		if opts.skipClusterAccess {
			// The flag contract is "never contact the cluster API". A Kind
			// cluster is deleted through the container runtime, but the
			// endpoint belongs to that same control plane: refuse rather
			// than silently violating the contract.
			return nil, fmt.Errorf("--skip-cluster-access cannot destroy a Kind cluster: the Kind control plane is the cluster API endpoint; omit the flag or delete the cluster with 'kind delete cluster --name %s'", cfg.ClusterName())
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Destroying Kind cluster...\n")
		logging.LogActionMessage(ctx, "destroying Kind cluster %s", cfg.ClusterName())
		if err := destroyKindCluster(ctx, cfg); err != nil {
			logging.LogActionMessage(ctx, "failed to destroy Kind cluster %s: %v", cfg.ClusterName(), err)
			return nil, err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Kind cluster destroyed.\n")
		logging.LogActionMessage(ctx, "Kind cluster %s destroyed successfully", cfg.ClusterName())
		return nil, nil
	}

	// Handle OpenTofu-based providers
	destroyService := cluster.NewDestroyService(cmd.OutOrStdout())
	if destroyService.SupportsInfraDestroy(&cfg) {
		if provider == "magnum" {
			fmt.Fprintf(cmd.OutOrStdout(), "Destroying Magnum cluster...\n")
			logging.LogActionMessage(ctx, "destroying Magnum cluster %s", cfg.ClusterName())
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Destroying infrastructure via OpenTofu...\n")
			logging.LogActionMessage(ctx, "destroying infrastructure via OpenTofu for %s", cfg.ClusterName())
		}
		result, err := destroyService.DestroyInfrastructure(ctx, &cfg, &cluster.DestroyOptions{
			AutoApprove:          true,
			DeleteVolumes:        opts.deleteVolumes,
			RequireClusterAccess: opts.requireClusterAccess,
			SkipClusterAccess:    opts.skipClusterAccess,
			AllowCleanupFailures: opts.allowCleanupFailures,
			DestroyRetries:       opts.retries,
			DestroyParallelism:   opts.parallelism,
		})
		if err != nil {
			logging.LogActionMessage(ctx, "infrastructure destruction failed: %v", err)
			return nil, fmt.Errorf("infrastructure destruction failed: %w", err)
		}
		if result.InfraDestroyed {
			fmt.Fprintf(cmd.OutOrStdout(), "Infrastructure destroyed successfully.\n")
			logging.LogActionMessage(ctx, "infrastructure destroyed successfully")
		}
		return result.CleanupWarnings, nil
	}

	// Provider doesn't have infrastructure to destroy
	fmt.Fprintf(cmd.OutOrStdout(), "Provider %q does not have infrastructure to destroy.\n", provider)
	logging.LogActionMessage(ctx, "provider %q does not have infrastructure to destroy", provider)
	return nil, nil
}

// reportDestroyCleanupWarnings prints and logs every non-fatal cleanup
// warning so leaked resources are visible in both the terminal and the
// destroy log. With strict mode the warnings become a command error, giving
// automation a non-zero exit code to react to.
func reportDestroyCleanupWarnings(cmd *cobra.Command, ctx context.Context, warnings []error, strict bool) error {
	if len(warnings) == 0 {
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nWarnings (%d): post-destroy cleanup was incomplete:\n", len(warnings))
	for i, w := range warnings {
		fmt.Fprintf(cmd.OutOrStdout(), "  %d. %v\n", i+1, w)
		logging.LogActionMessage(ctx, "destroy cleanup warning %d: %v", i+1, w)
	}
	if strict {
		return fmt.Errorf("destroy completed with %d cleanup warning(s); review the list above and the destroy log", len(warnings))
	}
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
				logging.LogActionMessage(ctx, "warning: failed to update cluster status: %v", err)
			} else {
				logging.LogActionMessage(ctx, "updated cluster status to destroyed")
			}
		}
	}

	// The config's GitOps directory is the shared organization worktree (e.g.
	// clusters/gitops/<org>): every cluster in the organization renders into it
	// (infrastructure/clusters/<cluster>/, applications/overlays/<cluster>/,
	// clusters/<cluster>/). Removing the whole worktree destroyed every other
	// cluster's files, so --remove-files only removes this cluster's own trees
	// below.
	if !isFlatConfig {
		if clusterPaths != nil {
			// Organization-based structure found
			// Remove cluster directory: clusters/<org>/infrastructure/clusters/<cluster>/
			if err := os.RemoveAll(clusterPaths.ClusterDir); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove cluster directory: %w", err)
			}
			if _, statErr := os.Stat(clusterPaths.ClusterDir); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.OutOrStdout(), "Removed cluster directory: %s\n", clusterPaths.ClusterDir)
				logging.LogActionMessage(ctx, "removed cluster directory: %s", clusterPaths.ClusterDir)
			}

			// Remove applications directory: clusters/<org>/applications/overlays/<cluster>/
			if err := os.RemoveAll(clusterPaths.ApplicationsDir); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove applications directory: %w", err)
			}
			if _, statErr := os.Stat(clusterPaths.ApplicationsDir); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.OutOrStdout(), "Removed applications directory: %s\n", clusterPaths.ApplicationsDir)
				logging.LogActionMessage(ctx, "removed applications directory: %s", clusterPaths.ApplicationsDir)
			}

			// Remove this cluster's GitOps bridge directory: <gitops>/clusters/<cluster>/
			// (the services.yaml bridge written by the generator; the flux
			// bootstrap content under the same directory dies with the cluster).
			if gitopsDir := strings.TrimSpace(cfg.GitDir()); gitopsDir != "" {
				bridgeDir := filepath.Join(gitopsDir, "clusters", clusterName)
				if err := os.RemoveAll(bridgeDir); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("failed to remove cluster bridge directory: %w", err)
				}
				if _, statErr := os.Stat(bridgeDir); os.IsNotExist(statErr) {
					fmt.Fprintf(cmd.OutOrStdout(), "Removed cluster bridge directory: %s\n", bridgeDir)
					logging.LogActionMessage(ctx, "removed cluster bridge directory: %s", bridgeDir)
				}
			}
		}
	}
	// For flat config files, we only remove the config file itself (no cluster directory)

	// Remove the config file
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete config file: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Removed config file: %s\n", configPath)
	logging.LogActionMessage(ctx, "removed config file: %s", configPath)

	// Remove the blueprint directory itself. saveConfig leaves a
	// <cluster>-config.yaml.backup next to the config file, and other sidecar
	// files (e.g. .harbor.storage-recovery.json) outlive the config file too.
	// Deleting only the config file left a bare blueprint directory behind:
	// a phantom that listing no longer shows but that litters the blueprints
	// zone forever.
	if !isFlatConfig && clusterPaths != nil {
		blueprintDir := filepath.Dir(clusterPaths.ConfigPath)
		if err := os.RemoveAll(blueprintDir); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove blueprint directory: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Removed blueprint directory: %s\n", blueprintDir)
		logging.LogActionMessage(ctx, "removed blueprint directory: %s", blueprintDir)
	}

	// Remove from active cluster if it was active
	activeCluster, err := getActiveCluster()
	if err == nil && activeCluster == name {
		if err := setActiveCluster(""); err != nil {
			// Log warning but don't fail
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to clear active cluster: %v\n", err)
			logging.LogActionMessage(ctx, "warning: failed to clear active cluster: %v", err)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Cleared active cluster marker\n")
			logging.LogActionMessage(ctx, "cleared active cluster marker")
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Cluster %q destroyed successfully.\n", name)
	logging.LogActionMessage(ctx, "cluster %q destroyed successfully", name)
	return nil
}

func destroyKindCluster(ctx context.Context, cfg v2.Config) error {
	clusterName := cfg.ClusterName()
	if err := kindprovider.NewProvider().DeleteCluster(ctx, clusterName, kindprovider.BuildEnvironment("")); err != nil {
		return fmt.Errorf("failed to destroy kind cluster %q: %w", clusterName, err)
	}

	return nil
}

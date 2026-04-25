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
	"sync"

	"github.com/opencenter-cloud/opencenter-cli/internal/secrets"
	"github.com/spf13/cobra"
)

// newSecretsSyncCmd creates the command for synchronizing secrets across clusters.
func newSecretsSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync [cluster]",
		Short: "Synchronize secrets from config to encrypted manifests",
		Long: `Synchronize secrets from the cluster configuration file to encrypted manifests.

This command reads secrets from the cluster's config file (.k8s-<cluster>-config.yaml)
and generates corresponding SOPS-encrypted manifests for each service. It ensures that
deployed secrets match the source of truth in the configuration.

The sync operation:
  • Reads secrets from the config file
  • Generates encrypted manifests for each service
  • Preserves non-secret fields in existing manifests
  • Uses the cluster's Age key for encryption
  • Reports created, updated, and unchanged files

If no cluster name is provided, uses the currently active cluster.

Multi-cluster mode (--all flag) processes all clusters in parallel with configurable
concurrency. Use --org to filter to a specific organization.`,
		Example: `  # Sync secrets for active cluster
  opencenter secrets sync

  # Sync secrets for specific cluster
  opencenter secrets sync my-cluster

  # Sync only specific services
  opencenter secrets sync my-cluster --services=cert-manager,loki

  # Preview changes without applying (dry-run)
  opencenter secrets sync my-cluster --dry-run

  # Force sync even if no drift detected
  opencenter secrets sync my-cluster --force

  # Sync all clusters in organization
  opencenter secrets sync --all --org myorg

  # Sync all clusters with custom concurrency
  opencenter secrets sync --all --concurrency=8

  # Stop on first error
  opencenter secrets sync --all --stop-on-error`,
		Args: cobra.MaximumNArgs(1),
		RunE: runClusterSyncSecrets,
	}

	cmd.Flags().String("cluster", "", "Cluster name (uses active cluster if not specified)")
	cmd.Flags().StringSlice("services", []string{}, "Comma-separated list of services to sync (e.g., cert-manager,loki)")
	cmd.Flags().Bool("dry-run", false, "Preview changes without applying them")
	cmd.Flags().Bool("force", false, "Overwrite manifests even if no drift detected")
	cmd.Flags().Bool("all", false, "Sync secrets for all clusters in organization")
	cmd.Flags().String("org", "", "Filter to specific organization (used with --all)")
	cmd.Flags().String("organization", "", "Filter to specific organization (used with --all)")
	_ = cmd.Flags().MarkHidden("organization")
	cmd.Flags().Int("concurrency", 4, "Maximum number of parallel cluster syncs (used with --all)")
	cmd.Flags().Bool("stop-on-error", false, "Stop processing on first failure (used with --all)")

	return cmd
}

func runClusterSyncSecrets(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get flags
	clusterFlag, _ := cmd.Flags().GetString("cluster")
	services, _ := cmd.Flags().GetStringSlice("services")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	force, _ := cmd.Flags().GetBool("force")
	all, _ := cmd.Flags().GetBool("all")
	organization := getSecretsSyncOrganizationFlag(cmd)
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	stopOnError, _ := cmd.Flags().GetBool("stop-on-error")

	// Initialize secrets manager
	secretsManager, err := initializeSecretsManager()
	if err != nil {
		return fmt.Errorf("failed to initialize secrets manager: %w", err)
	}

	// Handle multi-cluster sync
	if all {
		return runMultiClusterSync(ctx, cmd, secretsManager, multiClusterSyncParams{
			organization: organization,
			concurrency:  concurrency,
			stopOnError:  stopOnError,
			dryRun:       dryRun,
			services:     services,
			force:        force,
		})
	}

	// Single cluster sync - resolve cluster name from flag or args
	var clusterName string
	if clusterFlag != "" {
		clusterName = clusterFlag
	} else if len(args) > 0 {
		clusterName = args[0]
	} else {
		clusterName, err = resolveClusterNameForCommand(cmd, args, true)
		if err != nil {
			return err
		}
	}

	// Build sync options
	opts := secrets.SyncOptions{
		Cluster:  clusterName,
		Services: services,
		DryRun:   dryRun,
		Force:    force,
	}

	// Execute sync
	result, err := secretsManager.SyncSecrets(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to sync secrets: %w", err)
	}

	// Display results
	displaySyncResult(cmd, clusterName, result, dryRun)

	// Return error if there were any sync errors
	if len(result.Errors) > 0 {
		return fmt.Errorf("sync completed with %d errors", len(result.Errors))
	}

	return nil
}

func getSecretsSyncOrganizationFlag(cmd *cobra.Command) string {
	if organization, _ := cmd.Flags().GetString("org"); organization != "" {
		return organization
	}
	organization, _ := cmd.Flags().GetString("organization")
	return organization
}

// multiClusterSyncParams holds parameters for multi-cluster sync
type multiClusterSyncParams struct {
	organization string
	concurrency  int
	stopOnError  bool
	dryRun       bool
	services     []string
	force        bool
}

// runMultiClusterSync handles synchronization across multiple clusters
func runMultiClusterSync(
	ctx context.Context,
	cmd *cobra.Command,
	secretsManager secrets.SecretsManager,
	params multiClusterSyncParams,
) error {
	// Discover clusters
	clusters, err := discoverClusters(params.organization)
	if err != nil {
		return fmt.Errorf("failed to discover clusters: %w", err)
	}

	if len(clusters) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No clusters found")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Syncing secrets for %d clusters (concurrency: %d)\n\n", len(clusters), params.concurrency)

	// Create semaphore for concurrency control
	sem := make(chan struct{}, params.concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Track results
	results := make(map[string]*secrets.SyncResult)
	failures := make(map[string]error)
	stopFlag := false

	// Process each cluster
	for _, cluster := range clusters {
		// Check stop flag
		mu.Lock()
		if stopFlag {
			mu.Unlock()
			break
		}
		mu.Unlock()

		wg.Add(1)
		go func(clusterName string) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Check stop flag again
			mu.Lock()
			if stopFlag {
				mu.Unlock()
				return
			}
			mu.Unlock()

			// Build sync options
			opts := secrets.SyncOptions{
				Cluster:  clusterName,
				Services: params.services,
				DryRun:   params.dryRun,
				Force:    params.force,
			}

			// Execute sync
			result, err := secretsManager.SyncSecrets(ctx, opts)

			// Store result
			mu.Lock()
			if err != nil {
				failures[clusterName] = err
				fmt.Fprintf(cmd.ErrOrStderr(), "✗ %s: %v\n", clusterName, err)

				// Set stop flag if stop-on-error is enabled
				if params.stopOnError {
					stopFlag = true
				}
			} else {
				results[clusterName] = result
				fmt.Fprintf(cmd.OutOrStdout(), "✓ %s: %d created, %d updated, %d unchanged, %d errors\n",
					clusterName,
					len(result.Created),
					len(result.Updated),
					len(result.Unchanged),
					len(result.Errors))
			}
			mu.Unlock()
		}(cluster)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Display summary
	fmt.Fprintf(cmd.OutOrStdout(), "\nMulti-cluster sync summary:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Total clusters: %d\n", len(clusters))
	fmt.Fprintf(cmd.OutOrStdout(), "  Successful: %d\n", len(results))
	fmt.Fprintf(cmd.OutOrStdout(), "  Failed: %d\n", len(failures))

	if len(failures) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\nFailed clusters:\n")
		for cluster, err := range failures {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %v\n", cluster, err)
		}
		return fmt.Errorf("multi-cluster sync completed with %d failures", len(failures))
	}

	return nil
}

// discoverClusters finds all clusters, optionally filtered by organization
func discoverClusters(organization string) ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	clustersDir := filepath.Join(homeDir, ".config", "opencenter", "clusters")

	// Check if clusters directory exists
	if _, err := os.Stat(clustersDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	var clusters []string

	// Walk the clusters directory
	err = filepath.Walk(clustersDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip directories with errors
		}

		// Look for config files matching pattern .k8s-*-config.yaml
		if !info.IsDir() && strings.HasPrefix(info.Name(), ".k8s-") && strings.HasSuffix(info.Name(), "-config.yaml") {
			// Extract cluster name from filename
			clusterName := strings.TrimPrefix(info.Name(), ".k8s-")
			clusterName = strings.TrimSuffix(clusterName, "-config.yaml")

			// Get organization from path
			relPath, err := filepath.Rel(clustersDir, filepath.Dir(path))
			if err != nil {
				return nil
			}

			// Organization is the first directory component
			parts := strings.Split(relPath, string(filepath.Separator))
			if len(parts) > 0 {
				org := parts[0]

				// Filter by organization if specified
				if organization != "" && org != organization {
					return nil
				}
			}

			clusters = append(clusters, clusterName)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk clusters directory: %w", err)
	}

	return clusters, nil
}

// displaySyncResult formats and displays the sync result
func displaySyncResult(cmd *cobra.Command, clusterName string, result *secrets.SyncResult, dryRun bool) {
	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Secrets sync plan for cluster %s (dry-run):\n\n", clusterName)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Secrets sync completed for cluster %s:\n\n", clusterName)
	}

	// Display created files
	if len(result.Created) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Created (%d):\n", len(result.Created))
		for _, path := range result.Created {
			fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", path)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Display updated files
	if len(result.Updated) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Updated (%d):\n", len(result.Updated))
		for _, path := range result.Updated {
			fmt.Fprintf(cmd.OutOrStdout(), "  ~ %s\n", path)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Display unchanged files
	if len(result.Unchanged) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Unchanged (%d):\n", len(result.Unchanged))
		for _, path := range result.Unchanged {
			fmt.Fprintf(cmd.OutOrStdout(), "  = %s\n", path)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Display errors
	if len(result.Errors) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "Errors (%d):\n", len(result.Errors))
		for _, syncErr := range result.Errors {
			fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s (%s): %v\n", syncErr.FilePath, syncErr.Service, syncErr.Error)
		}
		fmt.Fprintln(cmd.ErrOrStderr())
	}

	// Display summary
	fmt.Fprintf(cmd.OutOrStdout(), "Summary:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Created: %d\n", len(result.Created))
	fmt.Fprintf(cmd.OutOrStdout(), "  Updated: %d\n", len(result.Updated))
	fmt.Fprintf(cmd.OutOrStdout(), "  Unchanged: %d\n", len(result.Unchanged))
	fmt.Fprintf(cmd.OutOrStdout(), "  Errors: %d\n", len(result.Errors))
}

// initializeSecretsManager creates and configures a secrets manager instance
func initializeSecretsManager() (secrets.SecretsManager, error) {
	logger := createSecretsLogger()
	configLoader := createConfigLoader()
	sopsManager := createSOPSManager(logger)
	auditLogger, err := createAuditLogger()
	if err != nil {
		return nil, err
	}

	secretsManager := secrets.NewDefaultSecretsManager(configLoader, sopsManager, auditLogger, logger)
	return secretsManager, nil
}

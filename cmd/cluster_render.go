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
	"path/filepath"
	"strings"
	"time"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/gitops"
	"github.com/rackerlabs/opencenter-cli/internal/tofu"
	"github.com/spf13/cobra"
)

// newClusterRenderCmd creates the command for rendering GitOps templates.
//
// This command handles template rendering with full organization-based structure support.
// It always renders templates (no skip logic) making it ideal for iterative development.
// Unlike `setup`, it does not perform Git operations or initialization checks.
//
// Returns:
//   - *cobra.Command: A pointer to the configured `render` command.
func newClusterRenderCmd() *cobra.Command {
	var (
		force       bool
		all         bool
		infra       bool
		services    bool
		serviceName string
	)

	cmd := &cobra.Command{
		Use:   "render [name] [service]",
		Short: "Render templates into the GitOps directory",
		Long: `Render cluster templates into the GitOps directory structure.

This command renders templates with safety checks to prevent accidental overwrites.
It handles organization-based directory structures and creates backups before overwriting.

Modes:
- No args: Checks if services already rendered, exits with instructions
- --all: Renders all services and infrastructure (requires --force if already rendered)
- --services: Renders all services only, no infrastructure (requires --force if already rendered)
- --infra: Renders infrastructure templates only (creates backups)
- <service>: Renders specific service (requires --force if already rendered)

Unlike 'cluster setup', this command:
- Performs safety checks before rendering
- Creates timestamped backups before overwriting
- Does not perform Git operations
- Ideal for iterative development and updates

Global Flags:
- --log-level: Set log level (debug, info, warn, error)
- --dry-run: Preview operations without making changes`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse global flags for logging and dry-run
			globalFlags, err := parseGlobalFlags(cmd)
			if err != nil {
				return fmt.Errorf("failed to parse global flags: %w", err)
			}

			// Apply log level override
			if globalFlags.LogLevel != "" {
				if err := config.SetLogLevel(globalFlags.LogLevel); err != nil {
					return fmt.Errorf("failed to set log level: %w", err)
				}
			}

			// Log dry-run mode if enabled
			if globalFlags.DryRun {
				config.Info("🧪 DRY RUN MODE: No files will be modified")
			}

			// Resolve cluster name from args or active cluster
			name, err := resolveClusterName(args, true)
			if err != nil {
				return err
			}

			// Check if service name provided as second arg
			if len(args) > 1 {
				serviceName = args[1]
			}

			// Load configuration
			cfg, err := loadConfigV2Only(name)
			if err != nil {
				return err
			}

			// Handle different render modes
			if infra {
				return renderInfrastructureOnly(cfg, globalFlags.DryRun, cmd)
			}

			if services {
				return renderServicesOnly(cfg, force, globalFlags.DryRun, cmd)
			}

			if serviceName != "" {
				return renderSingleService(cfg, serviceName, force, globalFlags.DryRun, cmd)
			}

			if all {
				return renderAllServices(cfg, force, globalFlags.DryRun, cmd)
			}

			// Default: check if already rendered
			return checkRenderStatus(cfg, cmd)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force overwrite existing files (creates backups)")
	cmd.Flags().BoolVar(&all, "all", false, "Render all services and infrastructure")
	cmd.Flags().BoolVar(&services, "services", false, "Render all services only (no infrastructure)")
	cmd.Flags().BoolVar(&infra, "infra", false, "Render infrastructure templates only")

	return cmd
}

// checkRenderStatus checks if services have already been rendered
func checkRenderStatus(cfg config.Config, cmd *cobra.Command) error {
	clusterName := cfg.ClusterName()
	gitOpsDir := cfg.GitOps().GitDir
	kustomizationPath := filepath.Join(gitOpsDir, "applications", "overlays", clusterName, "kustomization.yaml")

	if _, err := os.Stat(kustomizationPath); err == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Services have already been rendered for cluster '%s'.\n\n", clusterName)
		fmt.Fprintf(cmd.OutOrStdout(), "To render all services and infrastructure (with backups), use:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  opencenter cluster render %s --all --force\n\n", clusterName)
		fmt.Fprintf(cmd.OutOrStdout(), "To render all services only (with backups), use:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  opencenter cluster render %s --services --force\n\n", clusterName)
		fmt.Fprintf(cmd.OutOrStdout(), "To render a specific service, use:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  opencenter cluster render %s <service-name> --force\n\n", clusterName)
		fmt.Fprintf(cmd.OutOrStdout(), "To render infrastructure only, use:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  opencenter cluster render %s --infra\n", clusterName)
		return nil
	}

	// Not rendered yet, proceed with initial render (not dry-run)
	return renderAllServices(cfg, false, false, cmd)
}

// renderAllServices renders all cluster services and infrastructure
func renderAllServices(cfg config.Config, force bool, dryRun bool, cmd *cobra.Command) error {
	clusterName := cfg.ClusterName()
	gitOpsDir := cfg.GitOps().GitDir
	kustomizationPath := filepath.Join(gitOpsDir, "applications", "overlays", clusterName, "kustomization.yaml")

	// Check if already rendered and force not specified
	if _, err := os.Stat(kustomizationPath); err == nil && !force {
		return fmt.Errorf("services already rendered for cluster '%s', use --force to overwrite (creates backups)", clusterName)
	}

	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "🧪 DRY RUN: Would render all services and infrastructure for cluster: %s\n", clusterName)
		fmt.Fprintf(cmd.OutOrStdout(), "  - Copy base GitOps structure\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  - Render cluster-specific applications\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  - Render infrastructure templates\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  - Provision OpenTofu configuration\n")
		if force {
			fmt.Fprintf(cmd.OutOrStdout(), "  - Create timestamped backups before overwriting\n")
		}
		return nil
	}

	// Create backups if force is specified and files exist
	if force {
		if err := backupApplicationsDirectory(cfg, cmd); err != nil {
			return fmt.Errorf("failed to create backups: %w", err)
		}

		// Also backup infrastructure if it exists
		infraPath := filepath.Join(gitOpsDir, "infrastructure", "clusters", clusterName)
		if _, err := os.Stat(infraPath); err == nil {
			if err := backupInfrastructureDirectory(infraPath, clusterName, cmd); err != nil {
				return fmt.Errorf("failed to create infrastructure backups: %w", err)
			}
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Rendering all services and infrastructure for cluster: %s\n", clusterName)

	// Copy base GitOps structure
	if err := gitops.CopyBase(cfg, true); err != nil {
		return fmt.Errorf("failed to copy base GitOps structure: %w", err)
	}

	// Render cluster-specific applications
	if err := gitops.RenderClusterApps(cfg); err != nil {
		return fmt.Errorf("failed to render cluster apps: %w", err)
	}

	// Render infrastructure templates
	if err := gitops.RenderInfrastructureCluster(cfg); err != nil {
		return fmt.Errorf("failed to render infrastructure cluster: %w", err)
	}

	// Provision OpenTofu (renders main.tf and provider.tf)
	if err := tofu.Provision(cfg); err != nil {
		return fmt.Errorf("failed to provision opentofu: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "✓ All services and infrastructure rendered successfully")
	return nil
}

// renderServicesOnly renders all cluster services without infrastructure
func renderServicesOnly(cfg config.Config, force bool, dryRun bool, cmd *cobra.Command) error {
	clusterName := cfg.ClusterName()
	gitOpsDir := cfg.GitOps().GitDir
	kustomizationPath := filepath.Join(gitOpsDir, "applications", "overlays", clusterName, "kustomization.yaml")

	// Check if already rendered and force not specified
	if _, err := os.Stat(kustomizationPath); err == nil && !force {
		return fmt.Errorf("services already rendered for cluster '%s', use --force to overwrite (creates backups)", clusterName)
	}

	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "🧪 DRY RUN: Would render all services (no infrastructure) for cluster: %s\n", clusterName)
		fmt.Fprintf(cmd.OutOrStdout(), "  - Copy base GitOps structure\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  - Render cluster-specific applications\n")
		if force {
			fmt.Fprintf(cmd.OutOrStdout(), "  - Create timestamped backups before overwriting\n")
		}
		return nil
	}

	// Create backups if force is specified and files exist
	if force {
		if err := backupApplicationsDirectory(cfg, cmd); err != nil {
			return fmt.Errorf("failed to create backups: %w", err)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Rendering all services (no infrastructure) for cluster: %s\n", clusterName)

	// Copy base GitOps structure
	if err := gitops.CopyBase(cfg, true); err != nil {
		return fmt.Errorf("failed to copy base GitOps structure: %w", err)
	}

	// Render cluster-specific applications
	if err := gitops.RenderClusterApps(cfg); err != nil {
		return fmt.Errorf("failed to render cluster apps: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "✓ All services rendered successfully (infrastructure skipped)")
	return nil
}

// renderSingleService renders a specific service
func renderSingleService(cfg config.Config, serviceName string, force bool, dryRun bool, cmd *cobra.Command) error {
	clusterName := cfg.ClusterName()

	// Check if service exists in configuration
	serviceConfig, exists := cfg.OpenCenter.Services[serviceName]
	if !exists {
		return fmt.Errorf("service '%s' not found in cluster configuration", serviceName)
	}

	// Check if service is enabled
	if gitops.IsServiceDisabled(serviceConfig) {
		return fmt.Errorf("service '%s' is disabled in cluster configuration", serviceName)
	}

	// Check if service files already exist
	gitOpsDir := cfg.GitOps().GitDir
	serviceDir := filepath.Join(gitOpsDir, "applications", "overlays", clusterName, "services", serviceName)

	if _, err := os.Stat(serviceDir); err == nil && !force {
		return fmt.Errorf("service '%s' is enabled but files already exist, use --force to overwrite (creates backup)", serviceName)
	}

	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "🧪 DRY RUN: Would render service '%s' for cluster: %s\n", serviceName, clusterName)
		fmt.Fprintf(cmd.OutOrStdout(), "  - Service directory: %s\n", serviceDir)
		if force {
			if _, err := os.Stat(serviceDir); err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  - Create timestamped backup before overwriting\n")
			}
		}
		return nil
	}

	// Create backup if force is specified and files exist
	if force {
		if _, err := os.Stat(serviceDir); err == nil {
			if err := backupServiceDirectory(serviceDir, serviceName, cmd); err != nil {
				return fmt.Errorf("failed to create backup: %w", err)
			}
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Rendering service '%s' for cluster: %s\n", serviceName, clusterName)

	// Determine if this is a managed service
	isManaged := false
	managedServiceDir := filepath.Join(gitOpsDir, "applications", "overlays", clusterName, "managed-services", serviceName)
	if _, err := os.Stat(managedServiceDir); err == nil {
		isManaged = true
	}

	// Render the single service
	if err := gitops.RenderSingleService(cfg, serviceName, isManaged); err != nil {
		return fmt.Errorf("failed to render service '%s': %w", serviceName, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Service '%s' rendered successfully\n", serviceName)
	return nil
}

// renderInfrastructureOnly renders infrastructure templates only
func renderInfrastructureOnly(cfg config.Config, dryRun bool, cmd *cobra.Command) error {
	clusterName := cfg.ClusterName()
	gitOpsDir := cfg.GitOps().GitDir
	infraPath := filepath.Join(gitOpsDir, "infrastructure", "clusters", clusterName)

	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "🧪 DRY RUN: Would render infrastructure templates for cluster: %s\n", clusterName)
		fmt.Fprintf(cmd.OutOrStdout(), "  - Render infrastructure cluster templates\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  - Provision OpenTofu configuration\n")
		if _, err := os.Stat(infraPath); err == nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  - Create timestamped backups before overwriting\n")
		}
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Rendering infrastructure templates for cluster: %s\n", clusterName)

	// Create backups of existing infrastructure files
	if _, err := os.Stat(infraPath); err == nil {
		if err := backupInfrastructureDirectory(infraPath, clusterName, cmd); err != nil {
			return fmt.Errorf("failed to create backups: %w", err)
		}
	}

	// Render infrastructure templates
	if err := gitops.RenderInfrastructureCluster(cfg); err != nil {
		return fmt.Errorf("failed to render infrastructure cluster: %w", err)
	}

	// Provision OpenTofu (renders main.tf and provider.tf)
	if err := tofu.Provision(cfg); err != nil {
		return fmt.Errorf("failed to provision opentofu: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "✓ Infrastructure templates rendered successfully")
	return nil
}

// backupApplicationsDirectory creates backups of all files in the applications overlay directory
func backupApplicationsDirectory(cfg config.Config, cmd *cobra.Command) error {
	clusterName := cfg.ClusterName()
	gitOpsDir := cfg.GitOps().GitDir
	appsPath := filepath.Join(gitOpsDir, "applications", "overlays", clusterName)

	if _, err := os.Stat(appsPath); os.IsNotExist(err) {
		return nil // Nothing to backup
	}

	timestamp := time.Now().Format("20060102-150405")
	fmt.Fprintf(cmd.OutOrStdout(), "Creating backups with timestamp: %s\n", timestamp)

	return filepath.Walk(appsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Skip files that are already backups (contain .bak- in the filename)
		if strings.Contains(filepath.Base(path), ".bak-") {
			return nil
		}

		backupPath := fmt.Sprintf("%s.bak-%s", path, timestamp)
		if err := copyFile(path, backupPath); err != nil {
			return fmt.Errorf("failed to backup %s: %w", path, err)
		}
		return nil
	})
}

// backupServiceDirectory creates backups of all files in a service directory
func backupServiceDirectory(serviceDir, serviceName string, cmd *cobra.Command) error {
	timestamp := time.Now().Format("20060102-150405")
	fmt.Fprintf(cmd.OutOrStdout(), "Creating backup of service '%s' with timestamp: %s\n", serviceName, timestamp)

	return filepath.Walk(serviceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Skip files that are already backups (contain .bak- in the filename)
		if strings.Contains(filepath.Base(path), ".bak-") {
			return nil
		}

		backupPath := fmt.Sprintf("%s.bak-%s", path, timestamp)
		if err := copyFile(path, backupPath); err != nil {
			return fmt.Errorf("failed to backup %s: %w", path, err)
		}
		return nil
	})
}

// backupInfrastructureDirectory creates backups of all files in the infrastructure directory
func backupInfrastructureDirectory(infraPath, clusterName string, cmd *cobra.Command) error {
	timestamp := time.Now().Format("20060102-150405")
	fmt.Fprintf(cmd.OutOrStdout(), "Creating backup of infrastructure files with timestamp: %s\n", timestamp)

	return filepath.Walk(infraPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Skip files that are already backups (contain .bak- in the filename)
		if strings.Contains(filepath.Base(path), ".bak-") {
			return nil
		}

		backupPath := fmt.Sprintf("%s.bak-%s", path, timestamp)
		if err := copyFile(path, backupPath); err != nil {
			return fmt.Errorf("failed to backup %s: %w", path, err)
		}
		return nil
	})
}

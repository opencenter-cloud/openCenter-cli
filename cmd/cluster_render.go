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

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
	"github.com/opencenter-cloud/opencenter-cli/internal/tofu"
	"github.com/spf13/cobra"
)

func runClusterGenerateRenderOnly(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	dryRun := getGlobalOptions(cmd).DryRun

	name, err := resolveClusterName(args, true)
	if err != nil {
		return err
	}

	cfg, _, _, _, err := loadNativeV2ConfigWithIdentifier(cmd.Context(), name)
	if err != nil {
		return err
	}

	return renderAllServices(cfg, force, dryRun, cmd)
}

// checkRenderStatus checks if services have already been rendered
func checkRenderStatus(cfg *v2.Config, cmd *cobra.Command) error {
	clusterName := cfg.ClusterName()
	gitOpsDir := cfg.GitDir()
	kustomizationPath := filepath.Join(gitOpsDir, "applications", "overlays", clusterName, "kustomization.yaml")

	if _, err := os.Stat(kustomizationPath); err == nil {
		fmt.Fprintln(cmd.OutOrStdout(), "Render complete")
		fmt.Fprintf(cmd.OutOrStdout(), "Services have already been rendered for cluster '%s'.\n\n", clusterName)
		fmt.Fprintf(cmd.OutOrStdout(), "To re-render generated assets with backups, use:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  opencenter cluster generate %s --render-only --force\n", clusterName)
		return nil
	}

	// Not rendered yet, proceed with initial render (not dry-run)
	return renderAllServices(cfg, false, false, cmd)
}

// renderAllServices renders all cluster services and infrastructure
func renderAllServices(cfg *v2.Config, force bool, dryRun bool, cmd *cobra.Command) error {
	clusterName := cfg.ClusterName()
	gitOpsDir := cfg.GitDir()
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
	if err := gitops.CopyBase(*cfg, true); err != nil {
		return fmt.Errorf("failed to copy base GitOps structure: %w", err)
	}

	// Render cluster-specific applications
	if err := gitops.RenderClusterApps(*cfg); err != nil {
		return fmt.Errorf("failed to render cluster apps: %w", err)
	}

	// Render infrastructure templates
	if err := gitops.RenderInfrastructureCluster(*cfg); err != nil {
		return fmt.Errorf("failed to render infrastructure cluster: %w", err)
	}

	// Provision OpenTofu (renders main.tf and provider.tf)
	if err := tofu.Provision(*cfg); err != nil {
		return fmt.Errorf("failed to provision opentofu: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "✓ All services and infrastructure rendered successfully")
	fmt.Fprintln(cmd.OutOrStdout(), "Render complete")
	return nil
}

// renderServicesOnly renders all cluster services without infrastructure
func renderServicesOnly(cfg *v2.Config, force bool, dryRun bool, cmd *cobra.Command) error {
	clusterName := cfg.ClusterName()
	gitOpsDir := cfg.GitDir()
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
	if err := gitops.CopyBase(*cfg, true); err != nil {
		return fmt.Errorf("failed to copy base GitOps structure: %w", err)
	}

	// Render cluster-specific applications
	if err := gitops.RenderClusterApps(*cfg); err != nil {
		return fmt.Errorf("failed to render cluster apps: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "✓ All services rendered successfully (infrastructure skipped)")
	fmt.Fprintln(cmd.OutOrStdout(), "Render complete")
	return nil
}

// renderSingleService renders a specific service
func renderSingleService(cfg *v2.Config, serviceName string, force bool, dryRun bool, cmd *cobra.Command) error {
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
	gitOpsDir := cfg.GitDir()
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
	if err := gitops.RenderSingleService(*cfg, serviceName, isManaged); err != nil {
		return fmt.Errorf("failed to render service '%s': %w", serviceName, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Service '%s' rendered successfully\n", serviceName)
	fmt.Fprintln(cmd.OutOrStdout(), "Render complete")
	return nil
}

// renderInfrastructureOnly renders infrastructure templates only
func renderInfrastructureOnly(cfg *v2.Config, dryRun bool, cmd *cobra.Command) error {
	clusterName := cfg.ClusterName()
	gitOpsDir := cfg.GitDir()
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
	if err := gitops.RenderInfrastructureCluster(*cfg); err != nil {
		return fmt.Errorf("failed to render infrastructure cluster: %w", err)
	}

	// Provision OpenTofu (renders main.tf and provider.tf)
	if err := tofu.Provision(*cfg); err != nil {
		return fmt.Errorf("failed to provision opentofu: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "✓ Infrastructure templates rendered successfully")
	fmt.Fprintln(cmd.OutOrStdout(), "Render complete")
	return nil
}

// backupApplicationsDirectory creates backups of all files in the applications overlay directory
func backupApplicationsDirectory(cfg *v2.Config, cmd *cobra.Command) error {
	clusterName := cfg.ClusterName()
	gitOpsDir := cfg.GitDir()
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

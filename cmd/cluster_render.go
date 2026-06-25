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

	name, err := resolveClusterNameForCommand(cmd, args, true)
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

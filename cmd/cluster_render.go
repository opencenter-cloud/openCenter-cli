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
	"time"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/opencenter-cloud/opencenter-cli/internal/tofu"
	"github.com/spf13/cobra"
)

var encryptRenderedServiceOverrides = func(ctx context.Context, overlayPath string, cfg *v2.Config) error {
	return sops.NewSOPSManager().EncryptServiceOverrideValues(ctx, overlayPath, cfg)
}

func renderClusterAppsEncryptedResult(ctx context.Context, cfg v2.Config, prune, adoptGenerated bool) (*gitops.PromoteResult, error) {
	return gitops.RenderClusterAppsWithEncryptionResult(ctx, cfg, encryptRenderedServiceOverrides, gitops.PromoteOptions{
		Prune:          &prune,
		AdoptGenerated: adoptGenerated,
	})
}

func renderSingleServiceEncryptedResult(ctx context.Context, cfg v2.Config, serviceName string, isManaged, prune, adoptGenerated bool) (*gitops.PromoteResult, error) {
	return renderSingleServiceEncryptedResultWithBeforePromote(ctx, cfg, serviceName, isManaged, prune, adoptGenerated, nil)
}

func renderSingleServiceEncryptedResultWithBeforePromote(ctx context.Context, cfg v2.Config, serviceName string, isManaged, prune, adoptGenerated bool, beforePromote func() error) (*gitops.PromoteResult, error) {
	return gitops.RenderSingleServiceWithEncryptionResult(ctx, cfg, serviceName, isManaged, encryptRenderedServiceOverrides, gitops.PromoteOptions{
		Prune:          &prune,
		AdoptGenerated: adoptGenerated,
		BeforePromote:  beforePromote,
	})
}

func runClusterGenerateRenderOnly(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	prune, _ := cmd.Flags().GetBool("prune")
	adoptGenerated, _ := cmd.Flags().GetBool("adopt-generated")
	dryRun := getGlobalOptions(cmd).DryRun

	// Resolve gitops auth method early so we fail fast on invalid values.
	gitopsAuth, err := resolveGitopsAuthMethod(cmd)
	if err != nil {
		return err
	}

	name, err := resolveClusterName(args, true)
	if err != nil {
		return err
	}

	cfg, _, _, _, err := loadNativeV2ConfigWithIdentifier(cmd.Context(), name)
	if err != nil {
		return err
	}

	// Apply gitops auth override to BaseRepo.URL
	applyGitopsAuthOverride(cfg, gitopsAuth)

	skipValidation, _ := cmd.Flags().GetBool("skip-validation")
	manifestValidator, err := renderOnlyManifestValidator(cfg, skipValidation)
	if err != nil {
		return err
	}

	return renderAllServicesWithOptionsAndValidator(cfg, force, dryRun, prune, adoptGenerated, manifestValidator, cmd)
}

func renderOnlyManifestValidator(cfg *v2.Config, skipValidation bool) (func(string) error, error) {
	if skipValidation {
		return nil, nil
	}
	if err := v2.ValidateForGeneration(cfg); err != nil {
		return nil, fmt.Errorf("validating configuration for generation: %w", err)
	}
	return func(gitDir string) error {
		return gitops.NewManifestValidator(gitDir).Validate()
	}, nil
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

func renderAllServices(cfg *v2.Config, force bool, dryRun bool, cmd *cobra.Command) error {
	return renderAllServicesWithOptions(cfg, force, dryRun, true, false, cmd)
}

func renderAllServicesWithOptions(cfg *v2.Config, force bool, dryRun, prune, adoptGenerated bool, cmd *cobra.Command) error {
	return renderAllServicesWithOptionsAndValidator(cfg, force, dryRun, prune, adoptGenerated, nil, cmd)
}

func renderAllServicesWithOptionsAndValidator(cfg *v2.Config, force bool, dryRun, prune, adoptGenerated bool, manifestValidator func(string) error, cmd *cobra.Command) error {
	clusterName := cfg.ClusterName()
	gitOpsDir := cfg.GitDir()
	kustomizationPath := filepath.Join(gitOpsDir, "applications", "overlays", clusterName, "kustomization.yaml")

	alreadyRendered := false
	if _, err := os.Stat(kustomizationPath); err == nil {
		alreadyRendered = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect existing render: %w", err)
	}
	if alreadyRendered && !force && manifestValidator == nil {
		return fmt.Errorf("services already rendered for cluster '%s', use --force to overwrite (creates backups)", clusterName)
	}

	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: Would render all services and infrastructure for cluster: %s\n", clusterName)
		promotion, _, err := gitops.GenerateClusterTree(cmd.Context(), *cfg, gitops.StagedGenerationOptions{
			Materialize:           stagedTofuMaterializer(*cfg),
			Encrypt:               encryptRenderedServiceOverrides,
			ValidateManifest:      manifestValidator,
			IncludeInfrastructure: true,
			IncludeFluxBridge:     true,
			Promote: gitops.PromoteOptions{
				Prune:          &prune,
				DryRun:         true,
				AdoptGenerated: adoptGenerated,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to plan staged generation: %w", err)
		}
		printPromotionSummary(cmd, promotion)
		if alreadyRendered && !force {
			return fmt.Errorf("services already rendered for cluster '%s', use --force to overwrite (creates backups)", clusterName)
		}
		if force {
			fmt.Fprintf(cmd.OutOrStdout(), "  force overwrite backups: enabled\n")
		}
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Rendering all services and infrastructure for cluster: %s\n", clusterName)
	promotion, _, err := gitops.GenerateClusterTree(cmd.Context(), *cfg, gitops.StagedGenerationOptions{
		Materialize:           stagedTofuMaterializer(*cfg),
		Encrypt:               encryptRenderedServiceOverrides,
		ValidateManifest:      manifestValidator,
		IncludeInfrastructure: true,
		IncludeFluxBridge:     true,
		BeforePromote: func() error {
			if !force {
				return nil
			}
			if err := backupApplicationsDirectory(cfg, cmd); err != nil {
				return err
			}
			infraPath := filepath.Join(gitOpsDir, "infrastructure", "clusters", clusterName)
			if _, err := os.Stat(infraPath); err == nil {
				if err := backupInfrastructureDirectory(infraPath, clusterName, cmd); err != nil {
					return err
				}
			} else if !os.IsNotExist(err) {
				return err
			}
			return nil
		},
		Promote: gitops.PromoteOptions{
			Prune:          &prune,
			DryRun:         alreadyRendered && !force,
			AdoptGenerated: adoptGenerated,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to generate staged tree: %w", err)
	}
	printPromotionSummary(cmd, promotion)
	if alreadyRendered && !force {
		return fmt.Errorf("services already rendered for cluster '%s', use --force to overwrite (creates backups)", clusterName)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "✓ All services and infrastructure rendered successfully")
	fmt.Fprintln(cmd.OutOrStdout(), "Render complete")
	return nil
}

// renderServicesOnly renders all cluster services without infrastructure
func renderServicesOnly(cfg *v2.Config, force bool, dryRun bool, cmd *cobra.Command) error {
	return renderServicesOnlyWithOptions(cfg, force, dryRun, true, false, cmd)
}

func stagedTofuMaterializer(cfg v2.Config) func(string) error {
	if strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider)) == "kind" {
		return nil
	}
	return func(root string) error {
		return tofu.ProvisionAt(cfg, root)
	}
}

func renderServicesOnlyWithOptions(cfg *v2.Config, force bool, dryRun, prune, adoptGenerated bool, cmd *cobra.Command) error {
	clusterName := cfg.ClusterName()
	gitOpsDir := cfg.GitDir()
	kustomizationPath := filepath.Join(gitOpsDir, "applications", "overlays", clusterName, "kustomization.yaml")

	alreadyRendered := false
	if _, err := os.Stat(kustomizationPath); err == nil {
		alreadyRendered = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect existing render: %w", err)
	}
	if alreadyRendered && !force {
		return fmt.Errorf("services already rendered for cluster '%s', use --force to overwrite (creates backups)", clusterName)
	}

	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: Would render all services (no infrastructure) for cluster: %s\n", clusterName)
		if strings.TrimSpace(cfg.OpenCenter.GitOps.Repository.URL) == "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  - Copy base GitOps structure\n  - Render cluster-specific applications\n")
			if force {
				fmt.Fprintf(cmd.OutOrStdout(), "  - Create timestamped backups before overwriting\n")
			}
			return nil
		}
		promotion, _, err := gitops.GenerateClusterTree(cmd.Context(), *cfg, gitops.StagedGenerationOptions{
			Encrypt:           encryptRenderedServiceOverrides,
			IncludeFluxBridge: true,
			Promote: gitops.PromoteOptions{
				Prune:          &prune,
				DryRun:         true,
				AdoptGenerated: adoptGenerated,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to plan staged services: %w", err)
		}
		printPromotionSummary(cmd, promotion)
		if alreadyRendered && !force {
			return fmt.Errorf("services already rendered for cluster '%s', use --force to overwrite (creates backups)", clusterName)
		}
		if force {
			fmt.Fprintf(cmd.OutOrStdout(), "  force overwrite backups: enabled\n")
		}
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Rendering all services (no infrastructure) for cluster: %s\n", clusterName)
	promotion, _, err := gitops.GenerateClusterTree(cmd.Context(), *cfg, gitops.StagedGenerationOptions{
		Encrypt:           encryptRenderedServiceOverrides,
		IncludeFluxBridge: true,
		BeforePromote: func() error {
			if !force {
				return nil
			}
			return backupApplicationsDirectory(cfg, cmd)
		},
		Promote: gitops.PromoteOptions{
			Prune:          &prune,
			DryRun:         alreadyRendered && !force,
			AdoptGenerated: adoptGenerated,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to generate staged services: %w", err)
	}
	printPromotionSummary(cmd, promotion)
	if alreadyRendered && !force {
		return fmt.Errorf("services already rendered for cluster '%s', use --force to overwrite (creates backups)", clusterName)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "✓ All services rendered successfully (infrastructure skipped)")
	fmt.Fprintln(cmd.OutOrStdout(), "Render complete")
	return nil
}

// renderSingleService renders a specific service
func renderSingleService(cfg *v2.Config, serviceName string, force bool, dryRun bool, cmd *cobra.Command) error {
	return renderSingleServiceWithOptions(cfg, serviceName, force, dryRun, true, false, cmd)
}

func renderSingleServiceWithOptions(cfg *v2.Config, serviceName string, force bool, dryRun, prune, adoptGenerated bool, cmd *cobra.Command) error {
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

	fmt.Fprintf(cmd.OutOrStdout(), "Rendering service '%s' for cluster: %s\n", serviceName, clusterName)

	// Determine if this is a managed service
	isManaged := false
	managedServiceDir := filepath.Join(gitOpsDir, "applications", "overlays", clusterName, "managed-services", serviceName)
	if _, err := os.Stat(managedServiceDir); err == nil {
		isManaged = true
	}

	// Render the single service. Force backups are deferred until rendering,
	// encryption, materialized-secret validation, and ownership preflight pass.
	beforePromote := func() error {
		if !force {
			return nil
		}
		if _, err := os.Stat(serviceDir); os.IsNotExist(err) {
			return nil
		} else if err != nil {
			return err
		}
		return backupServiceDirectory(serviceDir, serviceName, cmd)
	}
	promotion, err := renderSingleServiceEncryptedResultWithBeforePromote(cmd.Context(), *cfg, serviceName, isManaged, prune, adoptGenerated, beforePromote)
	if err != nil {
		return fmt.Errorf("failed to render service '%s': %w", serviceName, err)
	}
	printPromotionSummary(cmd, promotion)

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

func printPromotionSummary(cmd *cobra.Command, result *gitops.PromoteResult) {
	if result == nil {
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Promotion summary: added=%d updated=%d unchanged=%d seeded=%d pruned=%d prune-candidates=%d renamed=%d adopted=%d\n", len(result.Added), len(result.Updated), len(result.Unchanged), len(result.Seeded), len(result.Pruned), len(result.PruneCandidates), len(result.Renamed), len(result.Adopted))
	for _, item := range result.Pruned {
		fmt.Fprintf(cmd.OutOrStdout(), "  pruned: %s\n", item)
	}
	for _, item := range result.PruneCandidates {
		fmt.Fprintf(cmd.OutOrStdout(), "  prune candidate (retained): %s\n", item)
	}
	for _, item := range result.Renamed {
		fmt.Fprintf(cmd.OutOrStdout(), "  renamed: %s\n", item)
	}
	for _, item := range result.Adopted {
		fmt.Fprintf(cmd.OutOrStdout(), "  adopted: %s\n", item)
	}
	for _, item := range result.BackupPaths {
		fmt.Fprintf(cmd.OutOrStdout(), "  backup: %s\n", item)
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(cmd.OutOrStdout(), "  warning: %s\n", warning)
	}
}

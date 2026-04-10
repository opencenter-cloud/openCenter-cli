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

package stages

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
)

// ValidationStage validates that the generated GitOps repository structure
// meets all requirements before completion.
// This stage should typically be the last stage in the generation pipeline.
type ValidationStage struct {
	BaseStage
}

// NewValidationStage creates a new validation stage.
// This stage depends on all other generation stages completing first.
func NewValidationStage(dependencies []string) *ValidationStage {
	return &ValidationStage{
		BaseStage: NewBaseStage(
			"validation",
			"Validate generated repository structure",
			dependencies,
		),
	}
}

// Execute validates the generated repository structure.
// This performs comprehensive checks to ensure all required components exist.
func (vs *ValidationStage) Execute(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
	// Validate base directory structure
	if err := vs.validateBaseStructure(workspace); err != nil {
		return fmt.Errorf("base structure validation failed: %w", err)
	}

	// Validate required files
	if err := vs.validateRequiredFiles(workspace); err != nil {
		return fmt.Errorf("required files validation failed: %w", err)
	}

	// Validate kustomization files
	if err := vs.validateKustomizationFiles(workspace); err != nil {
		return fmt.Errorf("kustomization files validation failed: %w", err)
	}

	// Validate Flux system
	if err := vs.validateFluxSystem(workspace); err != nil {
		return fmt.Errorf("flux system validation failed: %w", err)
	}

	// Validate cluster-specific structure
	if err := vs.validateClusterStructure(workspace); err != nil {
		return fmt.Errorf("cluster structure validation failed: %w", err)
	}

	// Validate organization structure
	if err := vs.validateOrganizationStructure(workspace); err != nil {
		return fmt.Errorf("organization structure validation failed: %w", err)
	}

	return nil
}

// validateBaseStructure checks that the base directory structure exists.
func (vs *ValidationStage) validateBaseStructure(workspace *gitops.GitOpsWorkspace) error {
	requiredDirs := []string{
		"applications",
		"applications/base",
		"applications/overlays",
		"infrastructure",
		"infrastructure/base",
		"infrastructure/clusters",
	}

	var missing []string
	for _, dir := range requiredDirs {
		if !workspace.Exists(dir) {
			missing = append(missing, dir)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required directories: %s", strings.Join(missing, ", "))
	}

	return nil
}

// validateRequiredFiles checks that required files exist.
func (vs *ValidationStage) validateRequiredFiles(workspace *gitops.GitOpsWorkspace) error {
	requiredFiles := []string{
		".gitignore",
		"README.md",
	}

	var missing []string
	for _, file := range requiredFiles {
		if !workspace.Exists(file) {
			missing = append(missing, file)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required files: %s", strings.Join(missing, ", "))
	}

	return nil
}

// validateKustomizationFiles checks that required kustomization files exist and are valid.
func (vs *ValidationStage) validateKustomizationFiles(workspace *gitops.GitOpsWorkspace) error {
	clusterName := workspace.Config.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}

	// Try to use PathResolver to get cluster paths
	var clusterInfraPath, clusterAppsPath string
	resolver := paths.NewPathResolver(workspace.RootDir)
	clusterPaths, err := resolver.ResolveWithFallback(context.Background(), clusterName)
	if err == nil {
		// Successfully resolved paths - get relative paths from workspace root
		clusterInfraPath, _ = filepath.Rel(workspace.RootDir, filepath.Join(clusterPaths.ClusterDir, "kustomization.yaml"))
		clusterAppsPath, _ = filepath.Rel(workspace.RootDir, filepath.Join(clusterPaths.ApplicationsDir, "kustomization.yaml"))
	} else {
		// Fallback to standard paths for test environments
		clusterInfraPath = filepath.Join("infrastructure", "clusters", clusterName, "kustomization.yaml")
		clusterAppsPath = filepath.Join("applications", "overlays", clusterName, "kustomization.yaml")
	}

	// Required kustomization files
	requiredKustomizations := []string{
		"infrastructure/kustomization.yaml",
		"applications/kustomization.yaml",
		clusterInfraPath,
		clusterAppsPath,
	}

	var missing []string
	for _, file := range requiredKustomizations {
		if !workspace.Exists(file) {
			missing = append(missing, file)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required kustomization files: %s", strings.Join(missing, ", "))
	}

	return nil
}

// validateFluxSystem checks that Flux system directory exists if configured.
func (vs *ValidationStage) validateFluxSystem(workspace *gitops.GitOpsWorkspace) error {
	// Check if Flux system directory exists
	if workspace.Exists(".flux-system") {
		// If it exists, validate it has a kustomization file
		if !workspace.Exists(".flux-system/kustomization.yaml") {
			return fmt.Errorf("flux system directory exists but missing kustomization.yaml")
		}
	}

	return nil
}

// validateClusterStructure checks that cluster-specific directories exist.
func (vs *ValidationStage) validateClusterStructure(workspace *gitops.GitOpsWorkspace) error {
	clusterName := workspace.Config.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}

	// Try to use PathResolver to get cluster paths
	var appsDir, clusterDir string
	resolver := paths.NewPathResolver(workspace.RootDir)
	clusterPaths, err := resolver.ResolveWithFallback(context.Background(), clusterName)
	if err == nil {
		// Successfully resolved paths - get relative paths from workspace root
		appsDir, _ = filepath.Rel(workspace.RootDir, clusterPaths.ApplicationsDir)
		clusterDir, _ = filepath.Rel(workspace.RootDir, clusterPaths.ClusterDir)
	} else {
		// Fallback to standard paths for test environments
		appsDir = filepath.Join("applications/overlays", clusterName)
		clusterDir = filepath.Join("infrastructure/clusters", clusterName)
	}

	// Check cluster-specific directories
	clusterDirs := []string{appsDir, clusterDir}

	var missing []string
	for _, dir := range clusterDirs {
		if !workspace.Exists(dir) {
			missing = append(missing, dir)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing cluster-specific directories: %s", strings.Join(missing, ", "))
	}

	return nil
}

// validateOrganizationStructure checks organization-level structure.
func (vs *ValidationStage) validateOrganizationStructure(workspace *gitops.GitOpsWorkspace) error {
	// Check for SOPS configuration if secrets backend is configured
	// Secrets are considered enabled if a backend is specified
	if workspace.Config.OpenCenter.Secrets.Backend != "" {
		if !workspace.Exists(".sops.yaml") {
			return fmt.Errorf("SOPS configuration file (.sops.yaml) is missing but secrets backend is configured")
		}
	}

	// Check for .opencenter marker file
	if !workspace.Exists(".opencenter") {
		// This is a warning, not an error - marker file may be created later
		// Just log it for now
	}

	return nil
}

// Rollback is a no-op for validation stage since it doesn't modify anything.
func (vs *ValidationStage) Rollback(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
	// Validation doesn't modify anything, so no rollback needed
	return nil
}

// Validate checks that validation completed successfully.
// For the validation stage itself, this is a no-op since Execute already validates.
func (vs *ValidationStage) Validate(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
	// The Execute method already performs validation
	// This method is called after Execute, so if we got here, validation passed
	return nil
}

// DryRun returns a plan for the validation stage.
func (vs *ValidationStage) DryRun(ctx context.Context, cfg v2.Config) (*gitops.StagePlan, error) {
	return &gitops.StagePlan{
		Name:         vs.Name(),
		Description:  vs.Description(),
		Files:        []string{}, // Validation doesn't create files
		Directories:  []string{}, // Validation doesn't create directories
		Dependencies: vs.Dependencies(),
	}, nil
}

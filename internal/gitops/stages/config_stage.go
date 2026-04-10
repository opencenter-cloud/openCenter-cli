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
	"os"
	"path/filepath"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
	"github.com/opencenter-cloud/opencenter-cli/internal/template"
)

// ConfigStage generates cluster-specific configuration files.
// This stage creates configuration files like kustomization.yaml, cluster overlays,
// and other cluster-specific settings that tie together the infrastructure and services.
type ConfigStage struct {
	BaseStage
	templateEngine   template.TemplateEngine
	templateRegistry template.TemplateRegistry
}

// NewConfigStage creates a new configuration generation stage.
func NewConfigStage(engine template.TemplateEngine, registry template.TemplateRegistry) *ConfigStage {
	return &ConfigStage{
		BaseStage: NewBaseStage(
			"config",
			"Generate cluster-specific configuration files",
			[]string{"service"}, // Depends on service stage
		),
		templateEngine:   engine,
		templateRegistry: registry,
	}
}

// Execute generates the cluster-specific configuration files.
func (cs *ConfigStage) Execute(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
	clusterName := workspace.Config.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name not specified in configuration")
	}

	// Always create default configs first
	if err := cs.createDefaultConfigs(ctx, workspace); err != nil {
		return fmt.Errorf("failed to create default configs: %w", err)
	}

	// Get configuration templates
	templates := cs.templateRegistry.GetTemplatesForType(template.TemplateTypeConfig)
	if len(templates) == 0 {
		// No additional templates to render
		return nil
	}

	// Resolve template dependencies
	templateNames := make([]string, len(templates))
	for i, tmpl := range templates {
		templateNames[i] = tmpl.Name
	}

	resolvedTemplates, err := cs.templateRegistry.ResolveTemplateDependencies(templateNames)
	if err != nil {
		return fmt.Errorf("failed to resolve template dependencies: %w", err)
	}

	// Create atomic writer for this stage
	writer := gitops.NewAtomicWriter(workspace)
	writer.SetStage(cs.Name())

	// Render each template
	for _, tmpl := range resolvedTemplates {
		// Check if conditions are met for this template
		if !cs.evaluateConditions(tmpl.Conditions, workspace.Config) {
			continue
		}

		// Render the template
		rendered, err := cs.templateEngine.Render(ctx, tmpl.Path, workspace.Config)
		if err != nil {
			return fmt.Errorf("failed to render template %s: %w", tmpl.Name, err)
		}

		// Determine output path
		outputPath := cs.getOutputPath(tmpl, workspace.Config)

		// Write the rendered template
		if err := writer.WriteFile(outputPath, rendered, 0o644); err != nil {
			return fmt.Errorf("failed to write template %s to %s: %w", tmpl.Name, outputPath, err)
		}
	}

	return nil
}

// createDefaultConfigs creates default configuration files when no templates are available.
func (cs *ConfigStage) createDefaultConfigs(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
	clusterName := workspace.Config.ClusterName()
	writer := gitops.NewAtomicWriter(workspace)
	writer.SetStage(cs.Name())

	// Try to use PathResolver to get cluster paths
	var clusterInfraPath, clusterAppsPath string
	resolver := paths.NewPathResolver(workspace.RootDir)
	clusterPaths, err := resolver.ResolveWithFallback(ctx, clusterName)
	if err == nil {
		// Successfully resolved paths - get relative paths from workspace root
		clusterInfraPath, _ = filepath.Rel(workspace.RootDir, filepath.Join(clusterPaths.ClusterDir, "kustomization.yaml"))
		clusterAppsPath, _ = filepath.Rel(workspace.RootDir, filepath.Join(clusterPaths.ApplicationsDir, "kustomization.yaml"))
	} else {
		// Fallback to standard paths for test environments
		clusterInfraPath = filepath.Join("infrastructure", "clusters", clusterName, "kustomization.yaml")
		clusterAppsPath = filepath.Join("applications", "overlays", clusterName, "kustomization.yaml")
	}

	// Create infrastructure kustomization.yaml
	infraKustomization := fmt.Sprintf(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - clusters/%s
`, clusterName)

	if err := writer.WriteFileString("infrastructure/kustomization.yaml", infraKustomization, 0o644); err != nil {
		return fmt.Errorf("failed to write infrastructure kustomization: %w", err)
	}

	// Create cluster-specific infrastructure kustomization
	clusterInfraKustomization := fmt.Sprintf(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
metadata:
  name: %s-infrastructure
  namespace: flux-system
`, clusterName)

	if err := writer.WriteFileString(clusterInfraPath, clusterInfraKustomization, 0o644); err != nil {
		return fmt.Errorf("failed to write cluster infrastructure kustomization: %w", err)
	}

	// Create applications kustomization.yaml
	appsKustomization := fmt.Sprintf(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - base
  - overlays/%s
`, clusterName)

	if err := writer.WriteFileString("applications/kustomization.yaml", appsKustomization, 0o644); err != nil {
		return fmt.Errorf("failed to write applications kustomization: %w", err)
	}

	// Create cluster-specific application overlay kustomization
	clusterAppsKustomization := fmt.Sprintf(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
metadata:
  name: %s-applications
  namespace: flux-system
resources:
  - ../../base
`, clusterName)

	if err := writer.WriteFileString(clusterAppsPath, clusterAppsKustomization, 0o644); err != nil {
		return fmt.Errorf("failed to write cluster applications kustomization: %w", err)
	}

	// Create Flux system kustomization
	fluxKustomization := fmt.Sprintf(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
metadata:
  name: flux-system
  namespace: flux-system
resources:
  - gotk-components.yaml
  - gotk-sync.yaml
`)

	if err := writer.WriteFileString(".flux-system/kustomization.yaml", fluxKustomization, 0o644); err != nil {
		return fmt.Errorf("failed to write flux system kustomization: %w", err)
	}

	return nil
}

// Rollback removes the configuration files created by this stage.
func (cs *ConfigStage) Rollback(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
	clusterName := workspace.Config.ClusterName()
	if clusterName == "" {
		return nil // Nothing to rollback
	}

	// Use PathResolver to get cluster paths
	resolver := paths.NewPathResolver(workspace.RootDir)
	clusterPaths, err := resolver.ResolveWithFallback(ctx, clusterName)
	if err != nil {
		// If we can't resolve paths, just continue with default paths
		clusterPaths = nil
	}

	// Get configuration templates
	templates := cs.templateRegistry.GetTemplatesForType(template.TemplateTypeConfig)

	// Remove files created by templates
	for _, tmpl := range templates {
		outputPath := cs.getOutputPath(tmpl, workspace.Config)
		fullPath := workspace.GetPath(outputPath)

		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s during rollback: %w", outputPath, err)
		}
	}

	// Remove default config files
	defaultFiles := []string{
		"infrastructure/kustomization.yaml",
		"applications/kustomization.yaml",
		".flux-system/kustomization.yaml",
	}

	// Add cluster-specific paths if we have them
	if clusterPaths != nil {
		clusterInfraPath, _ := filepath.Rel(workspace.RootDir, filepath.Join(clusterPaths.ClusterDir, "kustomization.yaml"))
		clusterAppsPath, _ := filepath.Rel(workspace.RootDir, filepath.Join(clusterPaths.ApplicationsDir, "kustomization.yaml"))
		defaultFiles = append(defaultFiles, clusterInfraPath, clusterAppsPath)
	}

	for _, file := range defaultFiles {
		fullPath := workspace.GetPath(file)
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s during rollback: %w", file, err)
		}
	}

	return nil
}

// Validate checks that the configuration files were generated correctly.
func (cs *ConfigStage) Validate(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
	clusterName := workspace.Config.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name not specified in configuration")
	}

	// Try to use PathResolver to get cluster paths
	var clusterInfraPath, clusterAppsPath string
	resolver := paths.NewPathResolver(workspace.RootDir)
	clusterPaths, err := resolver.ResolveWithFallback(ctx, clusterName)
	if err == nil {
		// Successfully resolved paths - get relative paths from workspace root
		clusterInfraPath, _ = filepath.Rel(workspace.RootDir, filepath.Join(clusterPaths.ClusterDir, "kustomization.yaml"))
		clusterAppsPath, _ = filepath.Rel(workspace.RootDir, filepath.Join(clusterPaths.ApplicationsDir, "kustomization.yaml"))
	} else {
		// Fallback to standard paths for test environments
		clusterInfraPath = filepath.Join("infrastructure", "clusters", clusterName, "kustomization.yaml")
		clusterAppsPath = filepath.Join("applications", "overlays", clusterName, "kustomization.yaml")
	}

	// Check for required configuration files
	requiredFiles := []string{
		"infrastructure/kustomization.yaml",
		"applications/kustomization.yaml",
		clusterInfraPath,
		clusterAppsPath,
	}

	for _, file := range requiredFiles {
		if !workspace.Exists(file) {
			return fmt.Errorf("required configuration file not found: %s", file)
		}
	}

	// Validate that kustomization files are valid YAML
	// (In a real implementation, this would parse and validate the YAML structure)

	return nil
}

// DryRun returns a plan of what this stage would create.
func (cs *ConfigStage) DryRun(ctx context.Context, cfg v2.Config) (*gitops.StagePlan, error) {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name not specified in configuration")
	}

	// Try to use PathResolver to get cluster paths
	var clusterInfraPath, clusterAppsPath string
	resolver := paths.NewPathResolver(cfg.GitOps().GitDir)
	clusterPaths, err := resolver.ResolveWithFallback(ctx, clusterName)
	if err == nil {
		// Successfully resolved paths - get relative paths from git dir
		clusterInfraPath, _ = filepath.Rel(cfg.GitOps().GitDir, filepath.Join(clusterPaths.ClusterDir, "kustomization.yaml"))
		clusterAppsPath, _ = filepath.Rel(cfg.GitOps().GitDir, filepath.Join(clusterPaths.ApplicationsDir, "kustomization.yaml"))
	} else {
		// Fallback to standard paths for test environments
		clusterInfraPath = filepath.Join("infrastructure", "clusters", clusterName, "kustomization.yaml")
		clusterAppsPath = filepath.Join("applications", "overlays", clusterName, "kustomization.yaml")
	}

	// Get configuration templates
	templates := cs.templateRegistry.GetTemplatesForType(template.TemplateTypeConfig)

	// Build list of files that would be created
	files := make([]string, 0)
	directories := make(map[string]bool)

	// Add template-based files
	for _, tmpl := range templates {
		// Skip templates whose conditions aren't met
		if !cs.evaluateConditions(tmpl.Conditions, cfg) {
			continue
		}

		outputPath := cs.getOutputPath(tmpl, cfg)
		files = append(files, outputPath)

		// Track directories
		dir := filepath.Dir(outputPath)
		for dir != "." && dir != "/" {
			directories[dir] = true
			dir = filepath.Dir(dir)
		}
	}

	// Add default configuration files
	defaultFiles := []string{
		"infrastructure/kustomization.yaml",
		"applications/kustomization.yaml",
		".flux-system/kustomization.yaml",
		clusterInfraPath,
		clusterAppsPath,
	}

	for _, file := range defaultFiles {
		files = append(files, file)

		// Track directories
		dir := filepath.Dir(file)
		for dir != "." && dir != "/" {
			directories[dir] = true
			dir = filepath.Dir(dir)
		}
	}

	// Convert directories map to slice
	dirs := make([]string, 0, len(directories))
	for dir := range directories {
		dirs = append(dirs, dir)
	}

	return &gitops.StagePlan{
		Name:         cs.Name(),
		Description:  fmt.Sprintf("%s (cluster: %s)", cs.Description(), clusterName),
		Files:        files,
		Directories:  dirs,
		Dependencies: cs.Dependencies(),
	}, nil
}

// getOutputPath determines the output path for a configuration template.
func (cs *ConfigStage) getOutputPath(tmpl template.TemplateDefinition, cfg v2.Config) string {
	clusterName := cfg.ClusterName()

	// If template has a custom output path in metadata tags, use the first tag
	if len(tmpl.Metadata.Tags) > 0 && tmpl.Metadata.Tags[0] != "" {
		outputPath := tmpl.Metadata.Tags[0]
		// Replace placeholders in the path
		outputPath = filepath.Join(outputPath)
		return outputPath
	}

	// Default: use template name as filename in cluster config directory
	filename := tmpl.Name
	if !hasExtension(filename) {
		filename += ".yaml"
	}

	// Use PathResolver to get the cluster directory
	resolver := paths.NewPathResolver(cfg.GitOps().GitDir)
	clusterPaths, err := resolver.ResolveWithFallback(context.Background(), clusterName)
	if err != nil {
		// Fallback to old path construction if resolver fails
		return filepath.Join("infrastructure", "clusters", clusterName, filename)
	}

	// Get relative path from git dir
	relPath, err := filepath.Rel(cfg.GitOps().GitDir, filepath.Join(clusterPaths.ClusterDir, filename))
	if err != nil {
		// Fallback to old path construction if relative path fails
		return filepath.Join("infrastructure", "clusters", clusterName, filename)
	}

	return relPath
}

// evaluateConditions checks if all conditions for a template are met.
func (cs *ConfigStage) evaluateConditions(conditions []template.RenderCondition, cfg v2.Config) bool {
	// If no conditions, template should be rendered
	if len(conditions) == 0 {
		return true
	}

	// All conditions must be met
	for _, condition := range conditions {
		if !cs.evaluateCondition(condition, cfg) {
			return false
		}
	}

	return true
}

// evaluateCondition checks if a single condition is met.
func (cs *ConfigStage) evaluateCondition(condition template.RenderCondition, cfg v2.Config) bool {
	// Get the field value from configuration
	fieldValue := cs.getFieldValue(condition.Field, cfg)

	switch condition.Type {
	case template.ConditionTypeEquals:
		return fieldValue == condition.Value

	case template.ConditionTypeNotEquals:
		return fieldValue != condition.Value

	case template.ConditionTypeContains:
		// Check if fieldValue (as string) contains the condition value
		if strValue, ok := fieldValue.(string); ok {
			if strCondition, ok := condition.Value.(string); ok {
				return contains(strValue, strCondition)
			}
		}
		return false

	case template.ConditionTypeExists:
		return fieldValue != nil && fieldValue != ""

	case template.ConditionTypeGreaterThan:
		return compareValues(fieldValue, condition.Value) > 0

	case template.ConditionTypeLessThan:
		return compareValues(fieldValue, condition.Value) < 0

	default:
		return false
	}
}

// getFieldValue extracts a field value from the configuration using dot notation.
func (cs *ConfigStage) getFieldValue(field string, cfg v2.Config) interface{} {
	// Simple field extraction - in a real implementation, this would use reflection
	// or a more sophisticated path resolution mechanism
	switch field {
	case "opencenter.infrastructure.provider", "provider":
		return cfg.OpenCenter.Infrastructure.Provider
	case "opencenter.cluster_name", "cluster_name":
		return cfg.ClusterName()
	case "opencenter.meta.organization", "organization":
		return cfg.OpenCenter.Meta.Organization
	default:
		return nil
	}
}

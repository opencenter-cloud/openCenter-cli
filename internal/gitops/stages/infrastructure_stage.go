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

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
	"github.com/rackerlabs/opencenter-cli/internal/gitops"
	"github.com/rackerlabs/opencenter-cli/internal/template"
)

// InfrastructureStage generates provider-specific infrastructure templates.
// This stage creates the infrastructure configuration files based on the
// cluster's provider (OpenStack, AWS, baremetal, etc.).
type InfrastructureStage struct {
	BaseStage
	templateEngine   template.TemplateEngine
	templateRegistry template.TemplateRegistry
}

// NewInfrastructureStage creates a new infrastructure generation stage.
func NewInfrastructureStage(engine template.TemplateEngine, registry template.TemplateRegistry) *InfrastructureStage {
	return &InfrastructureStage{
		BaseStage: NewBaseStage(
			"infrastructure",
			"Generate provider-specific infrastructure templates",
			[]string{"init"}, // Depends on init stage
		),
		templateEngine:   engine,
		templateRegistry: registry,
	}
}

// Execute generates the infrastructure templates for the configured provider.
func (is *InfrastructureStage) Execute(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
	// Get provider from configuration
	provider := workspace.Config.OpenCenter.Infrastructure.Provider
	if provider == "" {
		return fmt.Errorf("provider not specified in configuration")
	}

	// Get infrastructure templates for this provider
	templates := is.templateRegistry.GetTemplatesForProvider(provider)
	if len(templates) == 0 {
		return fmt.Errorf("no infrastructure templates found for provider: %s", provider)
	}

	// Filter for infrastructure-type templates only
	infraTemplates := make([]template.TemplateDefinition, 0)
	for _, tmpl := range templates {
		if tmpl.Type == template.TemplateTypeInfrastructure {
			infraTemplates = append(infraTemplates, tmpl)
		}
	}

	if len(infraTemplates) == 0 {
		return fmt.Errorf("no infrastructure templates found for provider: %s", provider)
	}

	// Resolve template dependencies
	templateNames := make([]string, len(infraTemplates))
	for i, tmpl := range infraTemplates {
		templateNames[i] = tmpl.Name
	}

	resolvedTemplates, err := is.templateRegistry.ResolveTemplateDependencies(templateNames)
	if err != nil {
		return fmt.Errorf("failed to resolve template dependencies: %w", err)
	}

	// Create atomic writer for this stage
	writer := gitops.NewAtomicWriter(workspace)
	writer.SetStage(is.Name())

	// Render each template
	for _, tmpl := range resolvedTemplates {
		// Check if conditions are met for this template
		if !is.evaluateConditions(tmpl.Conditions, workspace.Config) {
			continue
		}

		// Render the template
		rendered, err := is.templateEngine.Render(ctx, tmpl.Path, workspace.Config)
		if err != nil {
			return fmt.Errorf("failed to render template %s: %w", tmpl.Name, err)
		}

		// Determine output path
		outputPath := is.getOutputPath(tmpl, workspace.Config)

		// Write the rendered template
		if err := writer.WriteFile(outputPath, rendered, 0o644); err != nil {
			return fmt.Errorf("failed to write template %s to %s: %w", tmpl.Name, outputPath, err)
		}
	}

	return nil
}

// Rollback removes the infrastructure files created by this stage.
func (is *InfrastructureStage) Rollback(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
	// Get provider from configuration
	provider := workspace.Config.OpenCenter.Infrastructure.Provider
	if provider == "" {
		return nil // Nothing to rollback
	}

	// Get infrastructure templates for this provider
	templates := is.templateRegistry.GetTemplatesForProvider(provider)

	// Remove files created by this stage
	for _, tmpl := range templates {
		if tmpl.Type != template.TemplateTypeInfrastructure {
			continue
		}

		outputPath := is.getOutputPath(tmpl, workspace.Config)
		fullPath := workspace.GetPath(outputPath)

		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s during rollback: %w", outputPath, err)
		}
	}

	return nil
}

// Validate checks that the infrastructure templates were generated correctly.
func (is *InfrastructureStage) Validate(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
	// Get provider from configuration
	provider := workspace.Config.OpenCenter.Infrastructure.Provider
	if provider == "" {
		return fmt.Errorf("provider not specified in configuration")
	}

	// Get infrastructure templates for this provider
	templates := is.templateRegistry.GetTemplatesForProvider(provider)

	// Check that at least one infrastructure file was created
	foundFiles := 0
	for _, tmpl := range templates {
		if tmpl.Type != template.TemplateTypeInfrastructure {
			continue
		}

		// Skip templates whose conditions aren't met
		if !is.evaluateConditions(tmpl.Conditions, workspace.Config) {
			continue
		}

		outputPath := is.getOutputPath(tmpl, workspace.Config)
		if !workspace.Exists(outputPath) {
			return fmt.Errorf("expected infrastructure file not found: %s", outputPath)
		}

		foundFiles++
	}

	if foundFiles == 0 {
		return fmt.Errorf("no infrastructure files were generated for provider: %s", provider)
	}

	return nil
}

// DryRun returns a plan of what this stage would create.
func (is *InfrastructureStage) DryRun(ctx context.Context, cfg config.Config) (*gitops.StagePlan, error) {
	provider := cfg.OpenCenter.Infrastructure.Provider
	if provider == "" {
		return nil, fmt.Errorf("provider not specified in configuration")
	}

	// Get infrastructure templates for this provider
	templates := is.templateRegistry.GetTemplatesForProvider(provider)

	// Build list of files that would be created
	files := make([]string, 0)
	directories := make(map[string]bool)

	for _, tmpl := range templates {
		if tmpl.Type != template.TemplateTypeInfrastructure {
			continue
		}

		// Skip templates whose conditions aren't met
		if !is.evaluateConditions(tmpl.Conditions, cfg) {
			continue
		}

		outputPath := is.getOutputPath(tmpl, cfg)
		files = append(files, outputPath)

		// Track directories
		dir := filepath.Dir(outputPath)
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
		Name:         is.Name(),
		Description:  fmt.Sprintf("%s (provider: %s)", is.Description(), provider),
		Files:        files,
		Directories:  dirs,
		Dependencies: is.Dependencies(),
	}, nil
}

// getOutputPath determines the output path for a template.
// It uses the template metadata to determine the appropriate location.
func (is *InfrastructureStage) getOutputPath(tmpl template.TemplateDefinition, cfg config.Config) string {
	// Default path structure: infrastructure/clusters/<cluster-name>/<template-name>
	clusterName := cfg.ClusterName()

	// If template has a custom output path in metadata tags, use the first tag
	if len(tmpl.Metadata.Tags) > 0 && tmpl.Metadata.Tags[0] != "" {
		outputPath := tmpl.Metadata.Tags[0]

		// Use PathResolver to get the cluster directory
		resolver := paths.NewPathResolver(cfg.GitOps().GitDir)
		clusterPaths, err := resolver.ResolveWithFallback(context.Background(), clusterName)
		if err != nil {
			// Fallback to old path construction if resolver fails
			return filepath.Join("infrastructure", "clusters", clusterName, outputPath)
		}

		// Get relative path from git dir
		relPath, err := filepath.Rel(cfg.GitOps().GitDir, filepath.Join(clusterPaths.ClusterDir, outputPath))
		if err != nil {
			// Fallback to old path construction if relative path fails
			return filepath.Join("infrastructure", "clusters", clusterName, outputPath)
		}
		return relPath
	}

	// Default: use template name as filename
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
func (is *InfrastructureStage) evaluateConditions(conditions []template.RenderCondition, cfg config.Config) bool {
	// If no conditions, template should be rendered
	if len(conditions) == 0 {
		return true
	}

	// All conditions must be met
	for _, condition := range conditions {
		if !is.evaluateCondition(condition, cfg) {
			return false
		}
	}

	return true
}

// evaluateCondition checks if a single condition is met.
func (is *InfrastructureStage) evaluateCondition(condition template.RenderCondition, cfg config.Config) bool {
	// Get the field value from configuration
	fieldValue := is.getFieldValue(condition.Field, cfg)

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
// For example: "opencenter.infrastructure.provider" returns the provider value.
func (is *InfrastructureStage) getFieldValue(field string, cfg config.Config) interface{} {
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

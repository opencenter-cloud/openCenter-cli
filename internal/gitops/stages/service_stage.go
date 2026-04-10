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
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
	"github.com/opencenter-cloud/opencenter-cli/internal/services"
	"github.com/opencenter-cloud/opencenter-cli/internal/template"
)

// ServiceStage generates service-specific templates for enabled services.
// This stage creates the service configuration files based on which services
// are enabled in the cluster configuration.
type ServiceStage struct {
	BaseStage
	templateEngine   template.TemplateEngine
	templateRegistry template.TemplateRegistry
	serviceRegistry  services.ServiceRegistry
}

// NewServiceStage creates a new service generation stage.
func NewServiceStage(
	engine template.TemplateEngine,
	templateRegistry template.TemplateRegistry,
	serviceRegistry services.ServiceRegistry,
) *ServiceStage {
	return &ServiceStage{
		BaseStage: NewBaseStage(
			"service",
			"Generate enabled service configurations",
			[]string{"infrastructure"}, // Depends on infrastructure stage
		),
		templateEngine:   engine,
		templateRegistry: templateRegistry,
		serviceRegistry:  serviceRegistry,
	}
}

// Execute generates the service templates for enabled services.
func (ss *ServiceStage) Execute(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
	// Get enabled services from configuration
	enabledServices := ss.getEnabledServices(workspace.Config)
	if len(enabledServices) == 0 {
		// No services enabled, nothing to do
		return nil
	}

	// Resolve service dependencies to get correct order
	resolvedServices, err := ss.serviceRegistry.ResolveDependencies(enabledServices)
	if err != nil {
		return fmt.Errorf("failed to resolve service dependencies: %w", err)
	}

	// Get templates for enabled services
	templates := ss.templateRegistry.GetTemplatesForEnabledServices(enabledServices)
	if len(templates) == 0 {
		// No templates found for enabled services, this might be okay
		return nil
	}

	// Filter for service-type templates only
	serviceTemplates := make([]template.TemplateDefinition, 0)
	for _, tmpl := range templates {
		if tmpl.Type == template.TemplateTypeService {
			serviceTemplates = append(serviceTemplates, tmpl)
		}
	}

	// Resolve template dependencies
	templateNames := make([]string, len(serviceTemplates))
	for i, tmpl := range serviceTemplates {
		templateNames[i] = tmpl.Name
	}

	resolvedTemplates, err := ss.templateRegistry.ResolveTemplateDependencies(templateNames)
	if err != nil {
		return fmt.Errorf("failed to resolve template dependencies: %w", err)
	}

	// Create atomic writer for this stage
	writer := gitops.NewAtomicWriter(workspace)
	writer.SetStage(ss.Name())

	// Execute PreInstall lifecycle hooks for services
	for _, service := range resolvedServices {
		if err := service.ExecuteLifecycleHook(ctx, "PreInstall", workspace.Config); err != nil {
			return fmt.Errorf("PreInstall hook failed for service %s: %w", service.Name, err)
		}
	}

	// Render each template
	for _, tmpl := range resolvedTemplates {
		// Check if conditions are met for this template
		if !ss.evaluateConditions(tmpl.Conditions, workspace.Config) {
			continue
		}

		// Render the template
		rendered, err := ss.templateEngine.Render(ctx, tmpl.Path, workspace.Config)
		if err != nil {
			return fmt.Errorf("failed to render template %s: %w", tmpl.Name, err)
		}

		// Determine output path
		outputPath := ss.getOutputPath(tmpl, workspace.Config)

		// Write the rendered template
		if err := writer.WriteFile(outputPath, rendered, 0o644); err != nil {
			return fmt.Errorf("failed to write template %s to %s: %w", tmpl.Name, outputPath, err)
		}
	}

	// Execute PostInstall lifecycle hooks for services
	for _, service := range resolvedServices {
		if err := service.ExecuteLifecycleHook(ctx, "PostInstall", workspace.Config); err != nil {
			return fmt.Errorf("PostInstall hook failed for service %s: %w", service.Name, err)
		}
	}

	return nil
}

// Rollback removes the service files created by this stage.
func (ss *ServiceStage) Rollback(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
	// Get enabled services from configuration
	enabledServices := ss.getEnabledServices(workspace.Config)
	if len(enabledServices) == 0 {
		return nil // Nothing to rollback
	}

	// Get templates for enabled services
	templates := ss.templateRegistry.GetTemplatesForEnabledServices(enabledServices)

	// Remove files created by this stage
	for _, tmpl := range templates {
		if tmpl.Type != template.TemplateTypeService {
			continue
		}

		outputPath := ss.getOutputPath(tmpl, workspace.Config)
		fullPath := workspace.GetPath(outputPath)

		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s during rollback: %w", outputPath, err)
		}
	}

	return nil
}

// Validate checks that the service templates were generated correctly.
func (ss *ServiceStage) Validate(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
	// Get enabled services from configuration
	enabledServices := ss.getEnabledServices(workspace.Config)
	if len(enabledServices) == 0 {
		// No services enabled, validation passes
		return nil
	}

	// Get templates for enabled services
	templates := ss.templateRegistry.GetTemplatesForEnabledServices(enabledServices)

	// Check that at least one service file was created
	foundFiles := 0
	for _, tmpl := range templates {
		if tmpl.Type != template.TemplateTypeService {
			continue
		}

		// Skip templates whose conditions aren't met
		if !ss.evaluateConditions(tmpl.Conditions, workspace.Config) {
			continue
		}

		outputPath := ss.getOutputPath(tmpl, workspace.Config)
		if !workspace.Exists(outputPath) {
			return fmt.Errorf("expected service file not found: %s", outputPath)
		}

		foundFiles++
	}

	if foundFiles == 0 && len(enabledServices) > 0 {
		return fmt.Errorf("no service files were generated despite having enabled services")
	}

	return nil
}

// DryRun returns a plan of what this stage would create.
func (ss *ServiceStage) DryRun(ctx context.Context, cfg v2.Config) (*gitops.StagePlan, error) {
	// Get enabled services from configuration
	enabledServices := ss.getEnabledServices(cfg)
	if len(enabledServices) == 0 {
		return &gitops.StagePlan{
			Name:         ss.Name(),
			Description:  ss.Description() + " (no services enabled)",
			Files:        []string{},
			Directories:  []string{},
			Dependencies: ss.Dependencies(),
		}, nil
	}

	// Get templates for enabled services
	templates := ss.templateRegistry.GetTemplatesForEnabledServices(enabledServices)

	// Build list of files that would be created
	files := make([]string, 0)
	directories := make(map[string]bool)

	for _, tmpl := range templates {
		if tmpl.Type != template.TemplateTypeService {
			continue
		}

		// Skip templates whose conditions aren't met
		if !ss.evaluateConditions(tmpl.Conditions, cfg) {
			continue
		}

		outputPath := ss.getOutputPath(tmpl, cfg)
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

	description := fmt.Sprintf("%s (%d services enabled)", ss.Description(), len(enabledServices))

	return &gitops.StagePlan{
		Name:         ss.Name(),
		Description:  description,
		Files:        files,
		Directories:  dirs,
		Dependencies: ss.Dependencies(),
	}, nil
}

// getEnabledServices extracts the list of enabled service names from the configuration.
func (ss *ServiceStage) getEnabledServices(cfg v2.Config) []string {
	enabled := make([]string, 0)

	managedServices := cfg.OpenCenter.ManagedServices
	if len(managedServices) == 0 {
		managedServices = cfg.OpenCenter.LegacyManaged
	}

	// Check both Services and ManagedServices maps
	for name, serviceAny := range cfg.OpenCenter.Services {
		if !gitops.IsServiceDisabled(serviceAny) {
			enabled = append(enabled, name)
		}
	}

	for name, serviceAny := range managedServices {
		if !gitops.IsServiceDisabled(serviceAny) {
			enabled = append(enabled, name)
		}
	}

	return enabled
}

// getOutputPath determines the output path for a service template.
// It uses the template metadata to determine the appropriate location.
func (ss *ServiceStage) getOutputPath(tmpl template.TemplateDefinition, cfg v2.Config) string {
	clusterName := cfg.ClusterName()

	// If template has a custom output path in metadata tags, use the first tag
	if len(tmpl.Metadata.Tags) > 0 && tmpl.Metadata.Tags[0] != "" {
		outputPath := tmpl.Metadata.Tags[0]
		// Replace placeholders in the path
		outputPath = filepath.Join("applications", "overlays", clusterName, outputPath)
		return outputPath
	}

	// Default: use template name as filename in services directory
	filename := tmpl.Name
	if !hasExtension(filename) {
		filename += ".yaml"
	}

	// Determine service name from template services list
	serviceName := "common"
	if len(tmpl.Services) > 0 {
		serviceName = tmpl.Services[0]
	}

	return filepath.Join("applications", "overlays", clusterName, serviceName, filename)
}

// evaluateConditions checks if all conditions for a template are met.
func (ss *ServiceStage) evaluateConditions(conditions []template.RenderCondition, cfg v2.Config) bool {
	// If no conditions, template should be rendered
	if len(conditions) == 0 {
		return true
	}

	// All conditions must be met
	for _, condition := range conditions {
		if !ss.evaluateCondition(condition, cfg) {
			return false
		}
	}

	return true
}

// evaluateCondition checks if a single condition is met.
func (ss *ServiceStage) evaluateCondition(condition template.RenderCondition, cfg v2.Config) bool {
	// Get the field value from configuration
	fieldValue := ss.getFieldValue(condition.Field, cfg)

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
func (ss *ServiceStage) getFieldValue(field string, cfg v2.Config) interface{} {
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
		// Check if it's a service enablement field
		if len(field) > 8 && field[:8] == "service." {
			serviceName := field[8:]
			managedServices := cfg.OpenCenter.ManagedServices
			if len(managedServices) == 0 {
				managedServices = cfg.OpenCenter.LegacyManaged
			}
			if serviceAny, ok := cfg.OpenCenter.Services[serviceName]; ok {
				return !gitops.IsServiceDisabled(serviceAny)
			}
			if serviceAny, ok := managedServices[serviceName]; ok {
				return !gitops.IsServiceDisabled(serviceAny)
			}
		}
		return nil
	}
}

/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package template

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// EmbeddedTemplateRegistrar handles registration of embedded templates into the registry
type EmbeddedTemplateRegistrar struct {
	registry TemplateRegistry
}

// NewEmbeddedTemplateRegistrar creates a new registrar for embedded templates
func NewEmbeddedTemplateRegistrar(registry TemplateRegistry) *EmbeddedTemplateRegistrar {
	return &EmbeddedTemplateRegistrar{
		registry: registry,
	}
}

// RegisterFromFS scans an embedded filesystem and registers all templates found
func (r *EmbeddedTemplateRegistrar) RegisterFromFS(fsys fs.FS, basePath string, opts RegistrationOptions) error {
	return fs.WalkDir(fsys, basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Check if file is a template
		if !isTemplateFile(path) {
			return nil
		}

		// Create template definition
		def := r.createTemplateDefinition(path, basePath, opts)

		// Register the template
		if err := r.registry.RegisterTemplate(def); err != nil {
			return fmt.Errorf("failed to register template %s: %w", path, err)
		}

		return nil
	})
}

// RegistrationOptions provides configuration for template registration
type RegistrationOptions struct {
	// Provider specifies the cloud provider this template is for (openstack, aws, baremetal, etc.)
	Provider string

	// Services lists the services this template is associated with
	Services []string

	// Type specifies the template type (infrastructure, service, base, overlay)
	Type TemplateType

	// Priority sets the rendering priority (higher values render first)
	Priority int

	// Description provides a human-readable description
	Description string

	// Version specifies the template version
	Version string

	// Tags provides additional categorization
	Tags []string
}

// createTemplateDefinition creates a TemplateDefinition from a file path and options
func (r *EmbeddedTemplateRegistrar) createTemplateDefinition(path, basePath string, opts RegistrationOptions) TemplateDefinition {
	// Generate a unique name from the path
	name := generateTemplateName(path, basePath)

	// Infer template type from path if not specified
	templateType := opts.Type
	if templateType == "" {
		templateType = inferTemplateType(path)
	}

	// Infer provider from path if not specified
	provider := opts.Provider
	if provider == "" {
		provider = inferProvider(path)
	}

	// Infer services from path if not specified
	services := opts.Services
	if len(services) == 0 {
		services = inferServices(path)
	}

	return TemplateDefinition{
		Name:         name,
		Path:         path,
		Type:         templateType,
		Provider:     provider,
		Services:     services,
		Dependencies: []string{}, // Dependencies can be added later if needed
		Conditions:   []RenderCondition{},
		Metadata: TemplateMetadata{
			Description: opts.Description,
			Version:     opts.Version,
			Tags:        opts.Tags,
			Priority:    opts.Priority,
		},
	}
}

// isTemplateFile checks if a file is a template based on its extension
func isTemplateFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".tpl" || ext == ".tmpl" || ext == ".yaml" || ext == ".yml"
}

// generateTemplateName creates a unique template name from the file path
func generateTemplateName(path, basePath string) string {
	// Remove base path prefix
	relPath := strings.TrimPrefix(path, basePath)
	relPath = strings.TrimPrefix(relPath, "/")

	// Replace path separators with dots and remove extension
	name := strings.ReplaceAll(relPath, "/", ".")
	name = strings.TrimSuffix(name, filepath.Ext(name))

	return name
}

// inferTemplateType attempts to determine the template type from the path
func inferTemplateType(path string) TemplateType {
	lowerPath := strings.ToLower(path)

	if strings.Contains(lowerPath, "infrastructure") {
		return TemplateTypeInfrastructure
	}

	if strings.Contains(lowerPath, "service") || strings.Contains(lowerPath, "managed-services") {
		return TemplateTypeService
	}

	if strings.Contains(lowerPath, "overlay") {
		return TemplateTypeOverlay
	}

	if strings.Contains(lowerPath, "base") {
		return TemplateTypeBase
	}

	// Default to base type
	return TemplateTypeBase
}

// inferProvider attempts to determine the provider from the path or filename
func inferProvider(path string) string {
	lowerPath := strings.ToLower(path)
	filename := strings.ToLower(filepath.Base(path))

	// Check for provider-specific indicators
	if strings.Contains(filename, "baremetal") || strings.Contains(lowerPath, "baremetal") {
		return "baremetal"
	}

	if strings.Contains(filename, "openstack") || strings.Contains(lowerPath, "openstack") {
		return "openstack"
	}

	if strings.Contains(filename, "aws") || strings.Contains(lowerPath, "aws") {
		return "aws"
	}

	if strings.Contains(filename, "vsphere") || strings.Contains(lowerPath, "vsphere") {
		return "vsphere"
	}

	if strings.Contains(filename, "kind") || strings.Contains(lowerPath, "kind") {
		return "kind"
	}

	// Empty string means universal (works with all providers)
	return ""
}

// inferServices attempts to determine which services a template is associated with
func inferServices(path string) []string {
	lowerPath := strings.ToLower(path)
	services := []string{}

	// Common service names to look for
	serviceNames := []string{
		"alert-proxy",
		"loki",
		"prometheus",
		"grafana",
		"cert-manager",
		"calico",
		"weave-gitops",
		"velero",
		"keycloak",
		"headlamp",
		"etcd-backup",
		"vsphere-csi",
		"fluxcd",
	}

	for _, service := range serviceNames {
		if strings.Contains(lowerPath, service) {
			services = append(services, service)
		}
	}

	return services
}

// RegisterGitOpsTemplates registers all templates from the gitops embedded filesystem
func RegisterGitOpsTemplates(registry TemplateRegistry, fsys fs.FS) error {
	registrar := NewEmbeddedTemplateRegistrar(registry)

	// Register infrastructure templates
	infraOpts := RegistrationOptions{
		Type:        TemplateTypeInfrastructure,
		Description: "Infrastructure cluster templates",
		Version:     "1.0.0",
		Priority:    100,
		Tags:        []string{"infrastructure", "terraform", "opentofu"},
	}

	if err := registrar.RegisterFromFS(fsys, "templates/infrastructure-cluster-template", infraOpts); err != nil {
		return fmt.Errorf("failed to register infrastructure templates: %w", err)
	}

	// Register service templates
	serviceOpts := RegistrationOptions{
		Type:        TemplateTypeService,
		Description: "Cluster service templates",
		Version:     "1.0.0",
		Priority:    50,
		Tags:        []string{"services", "kubernetes", "helm"},
	}

	if err := registrar.RegisterFromFS(fsys, "templates/cluster-apps-base", serviceOpts); err != nil {
		return fmt.Errorf("failed to register service templates: %w", err)
	}

	return nil
}

// RegisterProvisionTemplates registers all templates from the provision embedded filesystem
func RegisterProvisionTemplates(registry TemplateRegistry, fsys fs.FS) error {
	registrar := NewEmbeddedTemplateRegistrar(registry)

	opts := RegistrationOptions{
		Type:        TemplateTypeInfrastructure,
		Description: "Provisioning templates for Terraform/OpenTofu",
		Version:     "1.0.0",
		Priority:    100,
		Tags:        []string{"provision", "terraform", "opentofu", "ansible"},
	}

	if err := registrar.RegisterFromFS(fsys, "templates", opts); err != nil {
		return fmt.Errorf("failed to register provision templates: %w", err)
	}

	return nil
}

// RegisterGitOpsBaseTemplates registers all templates from the gitops-base-dir embedded filesystem
func RegisterGitOpsBaseTemplates(registry TemplateRegistry, fsys fs.FS) error {
	registrar := NewEmbeddedTemplateRegistrar(registry)

	// Register base directory structure templates
	baseOpts := RegistrationOptions{
		Type:        TemplateTypeBase,
		Description: "GitOps base directory structure templates",
		Version:     "1.0.0",
		Priority:    200, // Higher priority than other templates
		Tags:        []string{"gitops", "base", "structure"},
	}

	if err := registrar.RegisterFromFS(fsys, "gitops-base-dir", baseOpts); err != nil {
		return fmt.Errorf("failed to register gitops base templates: %w", err)
	}

	return nil
}

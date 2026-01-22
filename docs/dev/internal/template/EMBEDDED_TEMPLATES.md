# Embedded Template Registry


## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Usage](#usage)
- [Template Metadata](#template-metadata)
- [Template Inference](#template-inference)
- [Registering Custom Templates](#registering-custom-templates)
- [Testing](#testing)
- [Migration from Legacy System](#migration-from-legacy-system)
- [Statistics](#statistics)
- [Future Enhancements](#future-enhancements)
- [See Also](#see-also)
This document describes the embedded template registration system that automatically catalogs all templates embedded in the openCenter CLI binary.

## Overview

The embedded template registry provides a centralized way to discover, filter, and access all templates that are embedded in the binary using Go's `//go:embed` directive. This system replaces ad-hoc template discovery with a structured registry that includes metadata about each template.

## Architecture

### Components

1. **TemplateRegistry**: Interface for managing template definitions
2. **EmbeddedTemplateRegistrar**: Scans embedded filesystems and registers templates
3. **Global Registry**: Singleton instance initialized at startup with all embedded templates

### Template Sources

Currently, templates are embedded from two main sources:

1. **GitOps Templates** (`internal/gitops/Files`):
   - Infrastructure cluster templates (Terraform/OpenTofu)
   - Service templates (Kubernetes manifests, Helm values)
   - Managed services (FluxCD, monitoring, etc.)

2. **Provision Templates** (`internal/provision/templatesFS`):
   - Terraform/OpenTofu provisioning templates
   - Ansible inventory templates
   - Provider-specific configurations

## Usage

### Getting the Global Registry

```go
import "github.com/rackerlabs/openCenter-cli/internal/template"

// Get the global registry (initialized once, thread-safe)
registry, err := template.GetGlobalRegistry()
if err != nil {
    return fmt.Errorf("failed to get template registry: %w", err)
}

// List all registered templates
templates := registry.ListTemplates()
fmt.Printf("Found %d templates\n", len(templates))
```

### Filtering Templates

#### By Provider

```go
// Get templates for a specific provider
baremetalTemplates := registry.GetTemplatesForProvider("baremetal")
openstackTemplates := registry.GetTemplatesForProvider("openstack")

// Get universal templates (work with all providers)
universalTemplates := registry.GetTemplatesForProvider("")
```

#### By Service

```go
// Get templates associated with a specific service
lokiTemplates := registry.GetTemplatesForService("loki")
prometheusTemplates := registry.GetTemplatesForService("prometheus")
```

#### By Enabled Services

```go
// Get templates for enabled services only
enabledServices := []string{"loki", "prometheus", "cert-manager"}
templates := registry.GetTemplatesForEnabledServices(enabledServices)

// This returns:
// - Templates with no service association (universal)
// - Templates associated with at least one enabled service
```

### Accessing Template Metadata

```go
template, err := registry.GetTemplate("main.tf")
if err != nil {
    return err
}

fmt.Printf("Name: %s\n", template.Name)
fmt.Printf("Path: %s\n", template.Path)
fmt.Printf("Type: %s\n", template.Type)
fmt.Printf("Provider: %s\n", template.Provider)
fmt.Printf("Services: %v\n", template.Services)
fmt.Printf("Priority: %d\n", template.Metadata.Priority)
fmt.Printf("Version: %s\n", template.Metadata.Version)
```

## Template Metadata

Each registered template includes:

### Core Fields

- **Name**: Unique identifier (e.g., "services.loki.helm-values.override-values")
- **Path**: Embedded filesystem path (e.g., "templates/services/loki/helm-values/override-values.yaml")
- **Type**: Template category (infrastructure, service, base, overlay)
- **Provider**: Cloud provider (openstack, aws, baremetal, vsphere, kind, or "" for universal)
- **Services**: Associated services (e.g., ["loki"], ["prometheus", "grafana"])

### Metadata Fields

- **Description**: Human-readable description
- **Version**: Template version
- **Priority**: Rendering priority (higher values render first)
- **Tags**: Additional categorization tags

## Template Inference

The registration system automatically infers metadata from template paths and filenames:

### Type Inference

- Paths containing "infrastructure" → `TemplateTypeInfrastructure`
- Paths containing "service" or "managed-services" → `TemplateTypeService`
- Paths containing "overlay" → `TemplateTypeOverlay`
- Paths containing "base" → `TemplateTypeBase`
- Default → `TemplateTypeBase`

### Provider Inference

- Filenames/paths containing "baremetal" → `"baremetal"`
- Filenames/paths containing "openstack" → `"openstack"`
- Filenames/paths containing "aws" → `"aws"`
- Filenames/paths containing "vsphere" → `"vsphere"`
- Filenames/paths containing "kind" → `"kind"`
- Default → `""` (universal)

### Service Inference

The system scans paths for known service names:
- alert-proxy, loki, prometheus, grafana, cert-manager
- calico, weave-gitops, velero, keycloak, headlamp
- etcd-backup, vsphere-csi, fluxcd

## Registering Custom Templates

### From Embedded Filesystem

```go
import "embed"

//go:embed my-templates/*
var myTemplates embed.FS

// Register templates with custom options
registry := template.NewInMemoryTemplateRegistry()
registrar := template.NewEmbeddedTemplateRegistrar(registry)

opts := template.RegistrationOptions{
    Type:        template.TemplateTypeService,
    Provider:    "aws",
    Services:    []string{"my-service"},
    Priority:    100,
    Description: "My custom templates",
    Version:     "1.0.0",
    Tags:        []string{"custom"},
}

err := registrar.RegisterFromFS(myTemplates, "my-templates", opts)
if err != nil {
    return fmt.Errorf("failed to register templates: %w", err)
}
```

### Manual Registration

```go
registry := template.NewInMemoryTemplateRegistry()

def := template.TemplateDefinition{
    Name:     "my-custom-template",
    Path:     "templates/custom.yaml",
    Type:     template.TemplateTypeService,
    Provider: "openstack",
    Services: []string{"my-service"},
    Metadata: template.TemplateMetadata{
        Description: "Custom template",
        Version:     "1.0.0",
        Priority:    50,
        Tags:        []string{"custom"},
    },
}

err := registry.RegisterTemplate(def)
if err != nil {
    return fmt.Errorf("failed to register template: %w", err)
}
```

## Testing

### Unit Tests

```go
func TestMyTemplateRegistration(t *testing.T) {
    registry := template.NewInMemoryTemplateRegistry()
    
    // Register your templates
    err := registerMyTemplates(registry)
    require.NoError(t, err)
    
    // Verify registration
    templates := registry.ListTemplates()
    assert.Greater(t, len(templates), 0)
    
    // Test filtering
    myTemplates := registry.GetTemplatesForService("my-service")
    assert.Len(t, myTemplates, expectedCount)
}
```

### Integration Tests

```go
func TestGlobalRegistryIntegration(t *testing.T) {
    // Reset for clean test
    template.ResetGlobalRegistry()
    
    // Get global registry
    registry, err := template.GetGlobalRegistry()
    require.NoError(t, err)
    
    // Verify templates are registered
    templates := registry.ListTemplates()
    assert.Greater(t, len(templates), 100)
}
```

## Migration from Legacy System

The embedded template registry is part of the configuration system refactor (Task 3.4). It provides:

1. **Centralized Discovery**: All templates in one registry
2. **Rich Metadata**: Provider, service, and type information
3. **Flexible Filtering**: Query templates by multiple criteria
4. **Automatic Inference**: Metadata derived from paths/filenames
5. **Thread-Safe Access**: Singleton pattern with sync.Once

### Compatibility

The legacy template system (`internal/template/legacy.go`) continues to work alongside the new registry. The registry provides discovery and metadata, while the legacy system handles rendering.

## Statistics

As of the current implementation:

- **Total Templates**: 133 (from gitops.Files)
- **Infrastructure Templates**: ~4
- **Service Templates**: ~129
- **Providers Supported**: baremetal, openstack, aws, vsphere, kind
- **Services Covered**: 15+ (loki, prometheus, grafana, cert-manager, etc.)

## Future Enhancements

1. **Dependency Resolution**: Resolve template dependencies automatically
2. **Conditional Rendering**: Support render conditions based on configuration
3. **Template Composition**: Combine base templates with overlays
4. **Validation**: Validate template syntax at registration time
5. **Caching**: Cache rendered templates for performance
6. **Hot Reload**: Support dynamic template registration

## See Also

- [Template Registry](./registry.go) - Core registry interface
- [Embedded Registrar](./embedded_registry.go) - Registration implementation
- [Global Registry](./global_registry.go) - Singleton instance
- [Legacy Compatibility](./legacy.go) - Backward compatibility layer

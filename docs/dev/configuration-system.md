# Configuration System Developer Guide

## Overview

This guide explains how to extend and work with openCenter's refactored configuration system. The system is built on modular, extensible components with clean interfaces that enable independent development and testing.

**Target Audience:** Developers who want to:
- Add new cloud providers
- Create custom services
- Extend template functionality
- Add validation rules
- Contribute to the configuration system

**Prerequisites:**
- Familiarity with Go programming
- Understanding of openCenter's architecture (see [Architecture Documentation](../explanation/architecture.md))
- Basic knowledge of Kubernetes and GitOps concepts

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Working with the Template Engine](#working-with-the-template-engine)
3. [Building Configurations](#building-configurations)
4. [Creating Service Plugins](#creating-service-plugins)
5. [Extending the Template Registry](#extending-the-template-registry)
6. [Adding Custom Validators](#adding-custom-validators)
7. [Working with GitOps Generation](#working-with-gitops-generation)
8. [Testing Your Extensions](#testing-your-extensions)
9. [Best Practices](#best-practices)
10. [Common Patterns](#common-patterns)

## Architecture Overview

The refactored configuration system consists of several key components:

```
┌─────────────────────────────────────────────────────────┐
│                    CLI Layer (cmd/)                     │
│  User commands, flag parsing, command orchestration     │
└────────────────────┬────────────────────────────────────┘
                     │
        ┌────────────┼────────────┐
        │            │            │
        ▼            ▼            ▼
┌──────────┐  ┌──────────┐  ┌──────────┐
│ Template │  │  Config  │  │ Service  │
│  Engine  │  │ Builder  │  │ Registry │
└──────────┘  └──────────┘  └──────────┘
        │            │            │
        └────────────┼────────────┘
                     │
                     ▼
           ┌──────────────────┐
           │ GitOps Generator │
           │   (Pipeline)     │
           └──────────────────┘
```

### Key Components

| Component | Location | Purpose |
|-----------|----------|---------|
| **Template Engine** | `internal/template/` | Renders Go templates with caching and validation |
| **Configuration Builder** | `internal/config/builder.go` | Type-safe fluent API for building configurations |
| **Service Registry** | `internal/services/` | Manages service plugins and dependencies |
| **Template Registry** | `internal/template/registry.go` | Catalogs templates with metadata |
| **GitOps Generator** | `internal/gitops/` | Pipeline-based repository generation |
| **Error Handling** | `internal/util/errors/` | Structured error handling with context |

### Design Principles

1. **Interface-Based Design**: Components interact through well-defined interfaces
2. **Dependency Injection**: Pass dependencies explicitly, avoid global state
3. **Error Aggregation**: Collect all errors before failing
4. **Testability**: Every component is independently testable
5. **Extensibility**: Plugin architecture for adding functionality
6. **Backward Compatibility**: Legacy compatibility layers during migration

## Working with the Template Engine

The template engine provides a clean abstraction for rendering Go templates with caching, validation, and comprehensive error reporting.

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "github.com/rackerlabs/openCenter-cli/internal/template"
)

func main() {
    // Create a new template engine
    engine := template.NewGoTemplateEngine()
    
    // Enable caching for better performance
    engine.SetCacheEnabled(true)
    
    // Render a template
    data := map[string]interface{}{
        "ClusterName": "my-cluster",
        "Provider":    "openstack",
    }
    
    result, err := engine.Render(context.Background(), "cluster.yaml.tmpl", data)
    if err != nil {
        fmt.Printf("Error rendering template: %v\n", err)
        return
    }
    
    fmt.Println(string(result))
}
```

### Registering Custom Functions

Add custom functions to extend template capabilities:

```go
// Register a single function
engine.RegisterFunction("toUpper", strings.ToUpper)

// Register multiple functions
engine.RegisterFunctions(template.FuncMap{
    "formatDate": func(t time.Time) string {
        return t.Format("2006-01-02")
    },
    "generatePassword": func(length int) string {
        // Password generation logic
        return "secure-password"
    },
})

// Use in templates
// {{ .ClusterName | toUpper }}
// {{ formatDate .CreatedAt }}
```

### Loading Templates from Embedded Filesystem

```go
import "embed"

//go:embed templates/*.tmpl
var templatesFS embed.FS

func main() {
    engine := template.NewGoTemplateEngine()
    
    // Load all templates matching pattern
    err := engine.LoadFromFS(templatesFS, "templates/*.tmpl")
    if err != nil {
        log.Fatal(err)
    }
    
    // Execute a named template
    result, err := engine.ExecuteTemplate("cluster.yaml.tmpl", data)
    if err != nil {
        log.Fatal(err)
    }
}
```

### Error Handling

The template engine provides detailed error messages with line numbers and context:

```go
result, err := engine.Render(ctx, "template.tmpl", data)
if err != nil {
    // Error includes:
    // - Template path
    // - Line number
    // - Column number (if available)
    // - Source context around error
    // - Actionable suggestions
    fmt.Printf("Template error: %v\n", err)
}
```

### Template Engine Interface

Implement the `TemplateEngine` interface to add support for other template formats:

```go
type TemplateEngine interface {
    Render(ctx context.Context, templatePath string, data interface{}) ([]byte, error)
    RenderString(ctx context.Context, templateName, templateContent string, data interface{}) ([]byte, error)
    RenderToWriter(ctx context.Context, templatePath string, data interface{}, w io.Writer) error
    ValidateTemplate(templatePath string) error
    RegisterFunction(name string, fn interface{})
    RegisterFunctions(funcs template.FuncMap)
    SetCacheEnabled(enabled bool)
    ClearCache()
}

// Example: Jinja2 template engine
type Jinja2TemplateEngine struct {
    // Implementation details
}

func (e *Jinja2TemplateEngine) Render(ctx context.Context, templatePath string, data interface{}) ([]byte, error) {
    // Jinja2 rendering logic
    return nil, nil
}
```

## Building Configurations

The configuration builder provides a type-safe, fluent API for constructing cluster configurations.

### Basic Configuration Building

```go
package main

import (
    "fmt"
    "github.com/rackerlabs/openCenter-cli/internal/config"
)

func main() {
    // Create a new builder
    builder := config.NewConfigBuilder("my-cluster")
    
    // Build configuration using fluent API
    cfg, err := builder.
        WithProvider("openstack").
        WithOrganization("acme-corp").
        WithEnvironment("production").
        WithRegion("us-east-1").
        WithKubernetesVersion("1.28.0").
        WithNodeCounts(3, 5).  // 3 masters, 5 workers
        WithServices("cert-manager", "monitoring").
        Build()
    
    if err != nil {
        fmt.Printf("Configuration build failed: %v\n", err)
        return
    }
    
    fmt.Printf("Built configuration for cluster: %s\n", cfg.OpenCenter.Meta.Name)
}
```

### Conditional Configuration

Use conditional methods for provider-specific configuration:

```go
cfg, err := builder.
    WithProvider("openstack").
    WithOrganization("acme-corp").
    WithClusterName("prod-cluster").
    // Apply OpenStack-specific configuration
    WhenProvider("openstack", func(b config.ConfigBuilder) config.ConfigBuilder {
        return b.WithOpenStackConfig(config.SimplifiedOpenStackCloud{
            AuthURL:    "https://openstack.example.com:5000/v3",
            ProjectID:  "project-123",
            NetworkID:  "network-456",
        })
    }).
    // Apply AWS-specific configuration
    WhenProvider("aws", func(b config.ConfigBuilder) config.ConfigBuilder {
        return b.WithAWSConfig(config.SimplifiedAWSCloud{
            Region: "us-east-1",
            VPC:    "vpc-123456",
        })
    }).
    // Apply to multiple providers
    WhenProviderIn([]string{"openstack", "aws"}, func(b config.ConfigBuilder) config.ConfigBuilder {
        return b.WithK8sHardening(true).WithOSHardening(true)
    }).
    // Exclude specific providers
    WhenNotProvider("kind", func(b config.ConfigBuilder) config.ConfigBuilder {
        return b.WithServices("monitoring", "logging")
    }).
    Build()
```

### Type-Safe Path Overrides

Use type-safe paths for compile-time validation:

```go
// Type-safe path overrides (compile-time validation)
cfg, err := builder.
    WithPath(config.TypedConfigPaths.ClusterName, "my-cluster").
    WithPathInt(config.TypedConfigPaths.MasterCount, 3).
    WithPathBool(config.TypedConfigPaths.K8sHardening, true).
    WithPathStringSlice(config.TypedConfigPaths.DNSNameservers, []string{"8.8.8.8", "8.8.4.4"}).
    Build()

// Runtime path overrides (runtime validation)
cfg, err := builder.
    WithOverride("opencenter.cluster.cluster_name", "my-cluster").
    WithOverride("opencenter.cluster.kubernetes.master_count", 3).
    Build()
```

### Custom Validators

Add custom validation logic to the builder:

```go
// Define a custom validator
type MyCustomValidator struct{}

func (v *MyCustomValidator) Validate(cfg config.Config) []config.ValidationError {
    var errors []config.ValidationError
    
    // Custom validation logic
    if cfg.OpenCenter.Cluster.Kubernetes.MasterCount > 7 {
        errors = append(errors, config.ValidationError{
            Field:   "opencenter.cluster.kubernetes.master_count",
            Message: "master count should not exceed 7 for optimal performance",
            Suggestions: []string{
                "Recommended: 3-5 masters for most deployments",
                "Large clusters may experience etcd performance issues with >7 masters",
            },
        })
    }
    
    return errors
}

// Use the custom validator
builder := config.NewConfigBuilder("my-cluster")
builder.AddValidator(&MyCustomValidator{})

cfg, err := builder.
    WithProvider("openstack").
    WithMasterCount(9).  // Will trigger custom validation
    Build()
```

### Validation Reports

Get detailed validation reports with suggestions:

```go
builder := config.NewConfigBuilder("my-cluster")

// Build configuration (may fail validation)
cfg, err := builder.
    WithProvider("openstack").
    // Missing required fields...
    Build()

if err != nil {
    // Get detailed validation report
    report := builder.GetValidationReport()
    
    fmt.Printf("Validation failed with %d errors:\n", len(report.Errors))
    for _, err := range report.Errors {
        fmt.Printf("  Field: %s\n", err.Field)
        fmt.Printf("  Error: %s\n", err.Message)
        fmt.Printf("  Suggestions:\n")
        for _, suggestion := range err.Suggestions {
            fmt.Printf("    - %s\n", suggestion)
        }
    }
}
```

## Creating Service Plugins

Service plugins enable modular service management with automatic dependency resolution.

### Service Plugin Interface

Implement the `ServicePlugin` interface to create a new service:

```go
package myplugin

import (
    "context"
    "github.com/rackerlabs/openCenter-cli/internal/services"
)

type MyServicePlugin struct {
    name string
}

func NewMyServicePlugin() *MyServicePlugin {
    return &MyServicePlugin{
        name: "my-service",
    }
}

func (p *MyServicePlugin) Name() string {
    return p.name
}

func (p *MyServicePlugin) Type() services.ServiceType {
    return services.ServiceTypeCustom
}

func (p *MyServicePlugin) Validate(config interface{}) error {
    // Validate service-specific configuration
    // Return error if configuration is invalid
    return nil
}

func (p *MyServicePlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
    // Render service templates to workspace
    // Generate Kubernetes manifests, Helm charts, etc.
    return nil
}

func (p *MyServicePlugin) Status(config interface{}) services.ServiceStatus {
    // Return current service status
    return services.ServiceStatus{
        State:   "running",
        Message: "Service is healthy",
        Details: map[string]interface{}{
            "version": "1.0.0",
        },
    }
}
```

### Service Plugin Manifest

Create a manifest file to describe your service plugin:

```yaml
# internal/services/plugins/my-service/manifest.yaml
name: my-service
version: 1.0.0
type: custom
description: My custom service for cluster management

dependencies:
  - cert-manager  # This service requires cert-manager
  - monitoring    # And monitoring

templates:
  - name: deployment
    path: templates/deployment.yaml.tmpl
  - name: service
    path: templates/service.yaml.tmpl
  - name: ingress
    path: templates/ingress.yaml.tmpl
    condition:
      provider: openstack  # Only render for OpenStack

config:
  schema:
    replicas:
      type: integer
      default: 3
    image:
      type: string
      required: true
  defaults:
    replicas: 3
    image: "my-service:latest"
  required:
    - image
  validation:
    - field: replicas
      type: range
      operator: between
      value: "1-10"
      message: "Replicas must be between 1 and 10"

metadata:
  author: "Your Name"
  license: "Apache-2.0"
  homepage: "https://github.com/example/my-service"
```

### Registering Service Plugins

Register your service plugin with the service registry:

```go
package main

import (
    "github.com/rackerlabs/openCenter-cli/internal/services"
    "github.com/rackerlabs/openCenter-cli/internal/services/plugins/myplugin"
)

func main() {
    // Create service registry
    registry := services.NewServiceRegistry()
    
    // Load manifest
    manifest, err := services.LoadManifest("internal/services/plugins/my-service/manifest.yaml")
    if err != nil {
        log.Fatal(err)
    }
    
    // Create plugin instance
    plugin := myplugin.NewMyServicePlugin()
    
    // Register service
    err = registry.RegisterService(services.ServiceDefinition{
        Name:         manifest.Name,
        Type:         manifest.Type,
        Dependencies: manifest.Dependencies,
        Templates:    manifest.Templates,
        Plugin:       plugin,
        Metadata: services.ServiceMetadata{
            Version:     manifest.Version,
            Description: manifest.Description,
        },
    })
    
    if err != nil {
        log.Fatal(err)
    }
}
```

### Service Lifecycle Hooks

Add lifecycle hooks for pre/post operations:

```go
lifecycle := services.ServiceLifecycle{
    PreInstall: func(ctx context.Context, config interface{}) error {
        // Run before service installation
        fmt.Println("Preparing to install my-service...")
        return nil
    },
    PostInstall: func(ctx context.Context, config interface{}) error {
        // Run after service installation
        fmt.Println("my-service installed successfully")
        return nil
    },
    PreUpdate: func(ctx context.Context, config interface{}) error {
        // Run before service update
        fmt.Println("Preparing to update my-service...")
        return nil
    },
    PostUpdate: func(ctx context.Context, config interface{}) error {
        // Run after service update
        fmt.Println("my-service updated successfully")
        return nil
    },
    PreRemove: func(ctx context.Context, config interface{}) error {
        // Run before service removal
        fmt.Println("Preparing to remove my-service...")
        return nil
    },
    PostRemove: func(ctx context.Context, config interface{}) error {
        // Run after service removal
        fmt.Println("my-service removed successfully")
        return nil
    },
}

// Register service with lifecycle hooks
err = registry.RegisterService(services.ServiceDefinition{
    Name:       "my-service",
    Plugin:     plugin,
    Lifecycle:  lifecycle,
})
```

### Dependency Resolution

The service registry automatically resolves dependencies:

```go
// Get enabled services with dependencies resolved
enabledServices, err := registry.GetEnabledServices(config)
if err != nil {
    log.Fatal(err)
}

// Services are returned in dependency order
for _, svc := range enabledServices {
    fmt.Printf("Service: %s (depends on: %v)\n", svc.Name, svc.Dependencies)
}

// Validate dependencies before enabling
err = registry.ValidateDependencies([]string{"my-service"})
if err != nil {
    // Error if dependencies are missing or circular
    log.Fatal(err)
}
```

## Extending the Template Registry

The template registry manages templates with metadata and dependency resolution.

### Registering Templates

```go
package main

import (
    "github.com/rackerlabs/openCenter-cli/internal/template"
)

func main() {
    registry := template.NewTemplateRegistry()
    
    // Register a template
    err := registry.RegisterTemplate(template.TemplateDefinition{
        Name:     "cluster-base",
        Path:     "templates/cluster-base.yaml.tmpl",
        Type:     template.TemplateTypeBase,
        Provider: "openstack",
        Services: []string{},  // No service dependencies
        Dependencies: []string{},  // No template dependencies
        Conditions: []template.RenderCondition{
            {
                Type:     template.ConditionTypeProvider,
                Field:    "provider",
                Operator: "equals",
                Value:    "openstack",
            },
        },
        Metadata: template.TemplateMetadata{
            Version:     "1.0.0",
            Description: "Base cluster template for OpenStack",
            Author:      "Your Name",
        },
    })
    
    if err != nil {
        log.Fatal(err)
    }
}
```

### Template Types

The registry supports different template types:

```go
const (
    TemplateTypeInfrastructure TemplateType = "infrastructure"  // Provider-specific infrastructure
    TemplateTypeService        TemplateType = "service"         // Service-specific configuration
    TemplateTypeBase          TemplateType = "base"            // Foundation templates
    TemplateTypeOverlay       TemplateType = "overlay"         // Patches and extensions
)
```

### Filtering Templates

```go
// Get templates for specific provider
openstackTemplates := registry.GetTemplatesForProvider("openstack")

// Get templates for specific service
certManagerTemplates := registry.GetTemplatesForService("cert-manager")

// Resolve template dependencies
templates := []string{"cluster-base", "networking-overlay", "storage-overlay"}
resolved, err := registry.ResolveTemplateDependencies(templates)
if err != nil {
    log.Fatal(err)
}

// Templates are returned in dependency order
for _, tmpl := range resolved {
    fmt.Printf("Template: %s (depends on: %v)\n", tmpl.Name, tmpl.Dependencies)
}
```

### Render Conditions

Control when templates are rendered using conditions:

```go
// Provider-based condition
conditions := []template.RenderCondition{
    {
        Type:     template.ConditionTypeProvider,
        Field:    "provider",
        Operator: "equals",
        Value:    "openstack",
    },
}

// Service-based condition
conditions := []template.RenderCondition{
    {
        Type:     template.ConditionTypeService,
        Field:    "services.monitoring.enabled",
        Operator: "equals",
        Value:    true,
    },
}

// Configuration value condition
conditions := []template.RenderCondition{
    {
        Type:     template.ConditionTypeConfig,
        Field:    "opencenter.cluster.kubernetes.master_count",
        Operator: "greater_than",
        Value:    1,
    },
}

// Multiple conditions (AND logic)
conditions := []template.RenderCondition{
    {
        Type:     template.ConditionTypeProvider,
        Field:    "provider",
        Operator: "equals",
        Value:    "openstack",
    },
    {
        Type:     template.ConditionTypeService,
        Field:    "services.cert-manager.enabled",
        Operator: "equals",
        Value:    true,
    },
}
```

## Adding Custom Validators

Create custom validators for configuration validation.

### Validator Interface

```go
package myvalidator

import (
    "github.com/rackerlabs/openCenter-cli/internal/config"
)

type MyCustomValidator struct {
    // Validator state
}

func NewMyCustomValidator() *MyCustomValidator {
    return &MyCustomValidator{}
}

func (v *MyCustomValidator) Validate(cfg config.Config) []config.ValidationError {
    var errors []config.ValidationError
    
    // Custom validation logic
    if cfg.OpenCenter.Cluster.BaseDomain == "" {
        errors = append(errors, config.ValidationError{
            Field:   "opencenter.cluster.base_domain",
            Message: "base domain is required for production clusters",
            Suggestions: []string{
                "Set base domain with: builder.WithBaseDomain(\"example.com\")",
                "Base domain is used for cluster DNS and ingress",
                "Example: \"k8s.example.com\"",
            },
            Context: map[string]interface{}{
                "environment": cfg.OpenCenter.Meta.Env,
            },
        })
    }
    
    return errors
}
```

### Provider-Specific Validators

Create validators for specific cloud providers:

```go
package validators

import (
    "fmt"
    "github.com/rackerlabs/openCenter-cli/internal/config"
)

type OpenStackValidator struct{}

func (v *OpenStackValidator) Validate(cfg config.Config) []config.ValidationError {
    var errors []config.ValidationError
    
    // Only validate if provider is OpenStack
    if cfg.OpenCenter.Infrastructure.Provider != "openstack" {
        return errors
    }
    
    osConfig := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
    
    // Validate auth URL
    if osConfig.AuthURL == "" {
        errors = append(errors, config.ValidationError{
            Field:   "opencenter.infrastructure.cloud.openstack.auth_url",
            Message: "OpenStack auth URL is required",
            Suggestions: []string{
                "Get auth URL from OpenStack dashboard",
                "Example: https://openstack.example.com:5000/v3",
                "Verify connectivity: curl -k <auth_url>",
            },
        })
    }
    
    // Validate project ID
    if osConfig.ProjectID == "" {
        errors = append(errors, config.ValidationError{
            Field:   "opencenter.infrastructure.cloud.openstack.project_id",
            Message: "OpenStack project ID is required",
            Suggestions: []string{
                "Get project ID from OpenStack dashboard",
                "Or use CLI: openstack project list",
            },
        })
    }
    
    // Validate network ID
    if osConfig.NetworkID == "" {
        errors = append(errors, config.ValidationError{
            Field:   "opencenter.infrastructure.cloud.openstack.network_id",
            Message: "OpenStack network ID is required",
            Suggestions: []string{
                "Get network ID from OpenStack dashboard",
                "Or use CLI: openstack network list",
                "Ensure network has DHCP enabled",
            },
        })
    }
    
    return errors
}
```

### Cross-Field Validators

Validate relationships between multiple fields:

```go
type CrossFieldValidator struct{}

func (v *CrossFieldValidator) Validate(cfg config.Config) []config.ValidationError {
    var errors []config.ValidationError
    
    // Validate Windows workers require Windows support
    if cfg.OpenCenter.Cluster.Kubernetes.WorkerCountWindows > 0 {
        // Check if Windows support is enabled
        if !cfg.OpenCenter.Cluster.Kubernetes.WindowsSupport {
            errors = append(errors, config.ValidationError{
                Field:   "opencenter.cluster.kubernetes.worker_count_windows",
                Message: "Windows workers require Windows support to be enabled",
                Suggestions: []string{
                    "Enable Windows support: builder.WithWindowsSupport(true)",
                    "Or set Windows worker count to 0",
                },
                Context: map[string]interface{}{
                    "windows_workers": cfg.OpenCenter.Cluster.Kubernetes.WorkerCountWindows,
                    "windows_support": cfg.OpenCenter.Cluster.Kubernetes.WindowsSupport,
                },
            })
        }
    }
    
    // Validate subnet overlap
    if cfg.Networking.SubnetNodes == cfg.Networking.SubnetPods {
        errors = append(errors, config.ValidationError{
            Field:   "networking.subnet_pods",
            Message: "pod subnet cannot overlap with node subnet",
            Suggestions: []string{
                "Use different CIDR ranges for nodes and pods",
                "Example: nodes=10.0.0.0/24, pods=10.244.0.0/16",
            },
            Context: map[string]interface{}{
                "subnet_nodes": cfg.Networking.SubnetNodes,
                "subnet_pods":  cfg.Networking.SubnetPods,
            },
        })
    }
    
    return errors
}
```

## Working with GitOps Generation

The GitOps generator uses a pipeline-based approach with staged execution and rollback.

### Creating Generation Stages

Implement the `GenerationStage` interface to create custom stages:

```go
package mystage

import (
    "context"
    "github.com/rackerlabs/openCenter-cli/internal/gitops"
)

type MyCustomStage struct {
    name string
}

func NewMyCustomStage() *MyCustomStage {
    return &MyCustomStage{
        name: "my-custom-stage",
    }
}

func (s *MyCustomStage) Name() string {
    return s.name
}

func (s *MyCustomStage) Execute(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
    // Stage execution logic
    // Generate files, render templates, etc.
    
    // Example: Create a custom directory
    customDir := filepath.Join(workspace.RootDir, "custom")
    if err := os.MkdirAll(customDir, 0755); err != nil {
        return fmt.Errorf("failed to create custom directory: %w", err)
    }
    
    // Example: Render a template
    templateEngine := template.NewGoTemplateEngine()
    result, err := templateEngine.Render(ctx, "custom-template.yaml.tmpl", workspace.Config)
    if err != nil {
        return fmt.Errorf("failed to render template: %w", err)
    }
    
    // Write rendered content
    outputPath := filepath.Join(customDir, "custom-config.yaml")
    if err := os.WriteFile(outputPath, result, 0644); err != nil {
        return fmt.Errorf("failed to write file: %w", err)
    }
    
    return nil
}

func (s *MyCustomStage) Rollback(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
    // Rollback logic - undo changes made in Execute
    customDir := filepath.Join(workspace.RootDir, "custom")
    return os.RemoveAll(customDir)
}

func (s *MyCustomStage) Validate(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
    // Validation logic - verify stage completed successfully
    customDir := filepath.Join(workspace.RootDir, "custom")
    if _, err := os.Stat(customDir); os.IsNotExist(err) {
        return fmt.Errorf("custom directory was not created")
    }
    return nil
}
```

### Using the Pipeline Generator

```go
package main

import (
    "context"
    "github.com/rackerlabs/openCenter-cli/internal/gitops"
    "github.com/rackerlabs/openCenter-cli/internal/config"
)

func main() {
    // Create pipeline generator
    generator := gitops.NewPipelineGenerator()
    
    // Add custom stage
    generator.AddStage(mystage.NewMyCustomStage())
    
    // Load configuration
    cfg, err := config.LoadConfig("cluster-config.yaml")
    if err != nil {
        log.Fatal(err)
    }
    
    // Generate GitOps repository
    err = generator.Generate(context.Background(), cfg)
    if err != nil {
        // Automatic rollback occurred
        log.Fatalf("Generation failed: %v", err)
    }
    
    fmt.Println("GitOps repository generated successfully")
}
```

### Dry-Run Mode

Preview changes without modifying the filesystem:

```go
// Generate dry-run plan
plan, err := generator.GenerateDryRun(context.Background(), cfg)
if err != nil {
    log.Fatal(err)
}

// Review planned changes
fmt.Printf("Dry-run plan:\n")
fmt.Printf("  Stages: %d\n", len(plan.Stages))
fmt.Printf("  Files to create: %d\n", len(plan.FilesToCreate))
fmt.Printf("  Files to modify: %d\n", len(plan.FilesToModify))

for _, file := range plan.FilesToCreate {
    fmt.Printf("  + %s\n", file)
}
```

### Workspace Checkpointing

Create checkpoints for rollback:

```go
// Create workspace
workspace := gitops.NewGitOpsWorkspace("/path/to/gitops-repo")

// Create checkpoint before risky operation
checkpoint := workspace.CreateCheckpoint("before-custom-stage")

// Perform operation
err := myCustomStage.Execute(ctx, workspace)
if err != nil {
    // Rollback to checkpoint
    workspace.RestoreCheckpoint(checkpoint.ID)
    log.Fatal(err)
}

// Operation successful, create another checkpoint
workspace.CreateCheckpoint("after-custom-stage")
```

## Testing Your Extensions

Comprehensive testing is essential for maintaining system reliability.

### Unit Testing

Test individual components in isolation:

```go
package myvalidator_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/rackerlabs/openCenter-cli/internal/config"
    "mypackage/myvalidator"
)

func TestMyCustomValidator(t *testing.T) {
    validator := myvalidator.NewMyCustomValidator()
    
    t.Run("valid configuration", func(t *testing.T) {
        cfg := config.Config{
            OpenCenter: config.SimplifiedOpenCenter{
                Cluster: config.ClusterConfig{
                    BaseDomain: "example.com",
                },
            },
        }
        
        errors := validator.Validate(cfg)
        assert.Empty(t, errors, "should not have validation errors")
    })
    
    t.Run("missing base domain", func(t *testing.T) {
        cfg := config.Config{
            OpenCenter: config.SimplifiedOpenCenter{
                Cluster: config.ClusterConfig{
                    BaseDomain: "",
                },
            },
        }
        
        errors := validator.Validate(cfg)
        assert.NotEmpty(t, errors, "should have validation errors")
        assert.Contains(t, errors[0].Field, "base_domain")
    })
}
```

### Property-Based Testing

Use property-based testing for universal properties:

```go
package myvalidator_test

import (
    "testing"
    "github.com/leanovate/gopter"
    "github.com/leanovate/gopter/gen"
    "github.com/leanovate/gopter/prop"
    "github.com/rackerlabs/openCenter-cli/internal/config"
)

func TestConfigBuilderIdempotency(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    // Property: Building a config twice should yield identical results
    properties.Property("config builder idempotency", prop.ForAll(
        func(provider, org, cluster string) bool {
            builder := config.NewConfigBuilder(cluster)
            
            cfg1, err1 := builder.
                WithProvider(provider).
                WithOrganization(org).
                Build()
            
            cfg2, err2 := builder.
                WithProvider(provider).
                WithOrganization(org).
                Build()
            
            // Both should succeed or both should fail
            if (err1 == nil) != (err2 == nil) {
                return false
            }
            
            // If both succeeded, configs should be identical
            if err1 == nil {
                return cfg1.OpenCenter.Meta.Name == cfg2.OpenCenter.Meta.Name &&
                       cfg1.OpenCenter.Meta.Organization == cfg2.OpenCenter.Meta.Organization &&
                       cfg1.OpenCenter.Infrastructure.Provider == cfg2.OpenCenter.Infrastructure.Provider
            }
            
            return true
        },
        gen.OneConstOf("openstack", "aws", "kind"),
        gen.AlphaString(),
        gen.AlphaString(),
    ))
    
    properties.TestingRun(t, gopter.ConsoleReporter(false))
}
```

### Integration Testing

Test complete workflows end-to-end:

```go
package integration_test

import (
    "context"
    "os"
    "path/filepath"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/rackerlabs/openCenter-cli/internal/config"
    "github.com/rackerlabs/openCenter-cli/internal/gitops"
)

func TestCompleteGitOpsGeneration(t *testing.T) {
    // Create temporary directory for test
    tmpDir := t.TempDir()
    
    // Build configuration
    cfg, err := config.NewConfigBuilder("test-cluster").
        WithProvider("openstack").
        WithOrganization("test-org").
        WithKubernetesVersion("1.28.0").
        WithNodeCounts(3, 5).
        Build()
    require.NoError(t, err)
    
    // Create generator
    generator := gitops.NewPipelineGenerator()
    
    // Generate GitOps repository
    err = generator.Generate(context.Background(), cfg)
    require.NoError(t, err)
    
    // Verify generated structure
    expectedDirs := []string{
        "infrastructure/clusters/test-cluster",
        "apps/test-cluster",
    }
    
    for _, dir := range expectedDirs {
        path := filepath.Join(tmpDir, dir)
        assert.DirExists(t, path, "directory should exist: %s", dir)
    }
    
    // Verify generated files
    expectedFiles := []string{
        "infrastructure/clusters/test-cluster/flux-system/gotk-components.yaml",
        "infrastructure/clusters/test-cluster/kustomization.yaml",
    }
    
    for _, file := range expectedFiles {
        path := filepath.Join(tmpDir, file)
        assert.FileExists(t, path, "file should exist: %s", file)
    }
}
```

### Testing with Feature Flags

Test both legacy and new implementations:

```go
func TestWithFeatureFlags(t *testing.T) {
    testCases := []struct {
        name        string
        featureFlag string
        enabled     bool
    }{
        {
            name:        "legacy template engine",
            featureFlag: "OPENCENTER_USE_NEW_TEMPLATE_ENGINE",
            enabled:     false,
        },
        {
            name:        "new template engine",
            featureFlag: "OPENCENTER_USE_NEW_TEMPLATE_ENGINE",
            enabled:     true,
        },
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Set feature flag
            if tc.enabled {
                os.Setenv(tc.featureFlag, "true")
            } else {
                os.Unsetenv(tc.featureFlag)
            }
            defer os.Unsetenv(tc.featureFlag)
            
            // Run test with feature flag
            engine := template.NewGoTemplateEngine()
            result, err := engine.Render(context.Background(), "test.tmpl", data)
            
            assert.NoError(t, err)
            assert.NotEmpty(t, result)
        })
    }
}
```

## Best Practices

### 1. Use Interfaces for Extensibility

Define interfaces for all major components to enable testing and extensibility:

```go
// Good: Interface-based design
type TemplateEngine interface {
    Render(ctx context.Context, path string, data interface{}) ([]byte, error)
}

// Bad: Concrete implementation only
type GoTemplateEngine struct {
    // ...
}
```

### 2. Inject Dependencies

Pass dependencies explicitly rather than using global state:

```go
// Good: Dependency injection
func NewMyService(engine TemplateEngine, registry ServiceRegistry) *MyService {
    return &MyService{
        engine:   engine,
        registry: registry,
    }
}

// Bad: Global state
var globalEngine TemplateEngine

func NewMyService() *MyService {
    return &MyService{
        engine: globalEngine,  // Tight coupling
    }
}
```

### 3. Aggregate Errors

Collect all errors before failing to provide complete feedback:

```go
// Good: Error aggregation
func Validate(cfg Config) []ValidationError {
    var errors []ValidationError
    
    if cfg.Name == "" {
        errors = append(errors, ValidationError{...})
    }
    if cfg.Provider == "" {
        errors = append(errors, ValidationError{...})
    }
    
    return errors  // Return all errors
}

// Bad: Fail on first error
func Validate(cfg Config) error {
    if cfg.Name == "" {
        return fmt.Errorf("name is required")  // User must fix and retry
    }
    if cfg.Provider == "" {
        return fmt.Errorf("provider is required")
    }
    return nil
}
```

### 4. Provide Actionable Error Messages

Include suggestions and context in error messages:

```go
// Good: Actionable error with suggestions
return ValidationError{
    Field:   "opencenter.cluster.base_domain",
    Message: "base domain is required for production clusters",
    Suggestions: []string{
        "Set base domain with: builder.WithBaseDomain(\"example.com\")",
        "Base domain is used for cluster DNS and ingress",
        "Example: \"k8s.example.com\"",
    },
    Context: map[string]interface{}{
        "environment": cfg.Environment,
    },
}

// Bad: Vague error
return fmt.Errorf("invalid configuration")
```

### 5. Use Context for Cancellation

Support context cancellation for long-running operations:

```go
// Good: Context support
func (e *Engine) Render(ctx context.Context, path string, data interface{}) ([]byte, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    
    // Rendering logic...
}

// Bad: No cancellation support
func (e *Engine) Render(path string, data interface{}) ([]byte, error) {
    // Long-running operation with no way to cancel
}
```

### 6. Write Comprehensive Tests

Test all code paths including error cases:

```go
func TestMyFunction(t *testing.T) {
    t.Run("success case", func(t *testing.T) {
        // Test happy path
    })
    
    t.Run("error case - invalid input", func(t *testing.T) {
        // Test error handling
    })
    
    t.Run("error case - timeout", func(t *testing.T) {
        // Test timeout handling
    })
    
    t.Run("edge case - empty input", func(t *testing.T) {
        // Test edge cases
    })
}
```

### 7. Use Feature Flags for Gradual Rollout

Enable gradual adoption of new features:

```go
// Check feature flag
if config.UseNewTemplateEngine() {
    // Use new implementation
    engine = template.NewGoTemplateEngine()
} else {
    // Use legacy implementation
    engine = template.NewLegacyEngine()
}
```

### 8. Document Public APIs

Add comprehensive documentation to all public APIs:

```go
// MyService provides cluster management functionality.
// It handles service registration, dependency resolution, and lifecycle management.
//
// Example usage:
//
//	service := NewMyService(engine, registry)
//	err := service.Deploy(ctx, config)
//	if err != nil {
//	    log.Fatal(err)
//	}
type MyService struct {
    engine   TemplateEngine
    registry ServiceRegistry
}

// Deploy deploys the service to the cluster.
// It validates the configuration, resolves dependencies, and renders templates.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - config: Service configuration
//
// Returns:
//   - error: nil on success, error with context on failure
func (s *MyService) Deploy(ctx context.Context, config ServiceConfig) error {
    // Implementation...
}
```

## Common Patterns

### Builder Pattern

Use the builder pattern for complex object construction:

```go
type ServiceBuilder struct {
    name         string
    version      string
    dependencies []string
    templates    []TemplateRef
}

func NewServiceBuilder(name string) *ServiceBuilder {
    return &ServiceBuilder{
        name:         name,
        dependencies: []string{},
        templates:    []TemplateRef{},
    }
}

func (b *ServiceBuilder) WithVersion(version string) *ServiceBuilder {
    b.version = version
    return b
}

func (b *ServiceBuilder) WithDependency(dep string) *ServiceBuilder {
    b.dependencies = append(b.dependencies, dep)
    return b
}

func (b *ServiceBuilder) WithTemplate(tmpl TemplateRef) *ServiceBuilder {
    b.templates = append(b.templates, tmpl)
    return b
}

func (b *ServiceBuilder) Build() (ServiceDefinition, error) {
    // Validation
    if b.name == "" {
        return ServiceDefinition{}, fmt.Errorf("name is required")
    }
    
    return ServiceDefinition{
        Name:         b.name,
        Version:      b.version,
        Dependencies: b.dependencies,
        Templates:    b.templates,
    }, nil
}

// Usage
service, err := NewServiceBuilder("my-service").
    WithVersion("1.0.0").
    WithDependency("cert-manager").
    WithTemplate(templateRef).
    Build()
```

### Registry Pattern

Use the registry pattern for managing collections of plugins:

```go
type ServiceRegistry struct {
    services map[string]ServiceDefinition
    mu       sync.RWMutex
}

func NewServiceRegistry() *ServiceRegistry {
    return &ServiceRegistry{
        services: make(map[string]ServiceDefinition),
    }
}

func (r *ServiceRegistry) Register(service ServiceDefinition) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if _, exists := r.services[service.Name]; exists {
        return fmt.Errorf("service %s already registered", service.Name)
    }
    
    r.services[service.Name] = service
    return nil
}

func (r *ServiceRegistry) Get(name string) (ServiceDefinition, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    service, exists := r.services[name]
    if !exists {
        return ServiceDefinition{}, fmt.Errorf("service %s not found", name)
    }
    
    return service, nil
}

func (r *ServiceRegistry) List() []ServiceDefinition {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    services := make([]ServiceDefinition, 0, len(r.services))
    for _, service := range r.services {
        services = append(services, service)
    }
    
    return services
}
```

### Strategy Pattern

Use the strategy pattern for pluggable algorithms:

```go
// Strategy interface
type ValidationStrategy interface {
    Validate(config Config) []ValidationError
}

// Concrete strategies
type SchemaValidationStrategy struct{}

func (s *SchemaValidationStrategy) Validate(config Config) []ValidationError {
    // Schema validation logic
    return []ValidationError{}
}

type BusinessRuleValidationStrategy struct{}

func (s *BusinessRuleValidationStrategy) Validate(config Config) []ValidationError {
    // Business rule validation logic
    return []ValidationError{}
}

// Context that uses strategies
type Validator struct {
    strategies []ValidationStrategy
}

func NewValidator() *Validator {
    return &Validator{
        strategies: []ValidationStrategy{
            &SchemaValidationStrategy{},
            &BusinessRuleValidationStrategy{},
        },
    }
}

func (v *Validator) Validate(config Config) []ValidationError {
    var allErrors []ValidationError
    
    for _, strategy := range v.strategies {
        errors := strategy.Validate(config)
        allErrors = append(allErrors, errors...)
    }
    
    return allErrors
}

// Add custom strategy
func (v *Validator) AddStrategy(strategy ValidationStrategy) {
    v.strategies = append(v.strategies, strategy)
}
```

### Template Method Pattern

Use the template method pattern for algorithms with fixed structure:

```go
// Abstract base with template method
type BaseStage struct {
    name string
}

func (s *BaseStage) Execute(ctx context.Context, workspace *GitOpsWorkspace) error {
    // Template method with fixed algorithm structure
    
    // 1. Pre-execution hook
    if err := s.preExecute(ctx, workspace); err != nil {
        return err
    }
    
    // 2. Main execution (implemented by subclasses)
    if err := s.doExecute(ctx, workspace); err != nil {
        return err
    }
    
    // 3. Post-execution hook
    if err := s.postExecute(ctx, workspace); err != nil {
        return err
    }
    
    return nil
}

func (s *BaseStage) preExecute(ctx context.Context, workspace *GitOpsWorkspace) error {
    // Common pre-execution logic
    fmt.Printf("Starting stage: %s\n", s.name)
    return nil
}

func (s *BaseStage) doExecute(ctx context.Context, workspace *GitOpsWorkspace) error {
    // To be implemented by subclasses
    return nil
}

func (s *BaseStage) postExecute(ctx context.Context, workspace *GitOpsWorkspace) error {
    // Common post-execution logic
    fmt.Printf("Completed stage: %s\n", s.name)
    return nil
}

// Concrete implementation
type InfrastructureStage struct {
    BaseStage
}

func (s *InfrastructureStage) doExecute(ctx context.Context, workspace *GitOpsWorkspace) error {
    // Infrastructure-specific execution logic
    return nil
}
```

### Observer Pattern

Use the observer pattern for event notification:

```go
// Event types
type EventType string

const (
    EventServiceRegistered EventType = "service_registered"
    EventServiceEnabled    EventType = "service_enabled"
    EventServiceDisabled   EventType = "service_disabled"
)

// Event
type Event struct {
    Type      EventType
    Timestamp time.Time
    Data      interface{}
}

// Observer interface
type Observer interface {
    OnEvent(event Event)
}

// Subject (observable)
type ServiceRegistry struct {
    services  map[string]ServiceDefinition
    observers []Observer
    mu        sync.RWMutex
}

func (r *ServiceRegistry) AddObserver(observer Observer) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.observers = append(r.observers, observer)
}

func (r *ServiceRegistry) notifyObservers(event Event) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    for _, observer := range r.observers {
        observer.OnEvent(event)
    }
}

func (r *ServiceRegistry) Register(service ServiceDefinition) error {
    r.mu.Lock()
    r.services[service.Name] = service
    r.mu.Unlock()
    
    // Notify observers
    r.notifyObservers(Event{
        Type:      EventServiceRegistered,
        Timestamp: time.Now(),
        Data:      service,
    })
    
    return nil
}

// Concrete observer
type LoggingObserver struct{}

func (o *LoggingObserver) OnEvent(event Event) {
    fmt.Printf("[%s] Event: %s\n", event.Timestamp.Format(time.RFC3339), event.Type)
}

// Usage
registry := NewServiceRegistry()
registry.AddObserver(&LoggingObserver{})
registry.Register(service)  // Triggers notification
```

## Example: Complete Service Plugin

Here's a complete example of a service plugin with all components:

```go
// File: internal/services/plugins/myservice/plugin.go
package myservice

import (
    "context"
    "fmt"
    "path/filepath"
    
    "github.com/rackerlabs/openCenter-cli/internal/services"
    "github.com/rackerlabs/openCenter-cli/internal/template"
)

// MyServicePlugin implements the ServicePlugin interface
type MyServicePlugin struct {
    name         string
    templateEngine template.TemplateEngine
}

func NewMyServicePlugin() *MyServicePlugin {
    return &MyServicePlugin{
        name:         "my-service",
        templateEngine: template.NewGoTemplateEngine(),
    }
}

func (p *MyServicePlugin) Name() string {
    return p.name
}

func (p *MyServicePlugin) Type() services.ServiceType {
    return services.ServiceTypeCustom
}

func (p *MyServicePlugin) Validate(config interface{}) error {
    // Type assert to service config
    cfg, ok := config.(*MyServiceConfig)
    if !ok {
        return fmt.Errorf("invalid config type")
    }
    
    // Validate required fields
    if cfg.Image == "" {
        return fmt.Errorf("image is required")
    }
    
    if cfg.Replicas < 1 || cfg.Replicas > 10 {
        return fmt.Errorf("replicas must be between 1 and 10")
    }
    
    return nil
}

func (p *MyServicePlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
    cfg, ok := config.(*MyServiceConfig)
    if !ok {
        return fmt.Errorf("invalid config type")
    }
    
    ws, ok := workspace.(*GitOpsWorkspace)
    if !ok {
        return fmt.Errorf("invalid workspace type")
    }
    
    // Create service directory
    serviceDir := filepath.Join(ws.RootDir, "apps", ws.Config.ClusterName, "my-service")
    if err := os.MkdirAll(serviceDir, 0755); err != nil {
        return fmt.Errorf("failed to create service directory: %w", err)
    }
    
    // Render deployment template
    deploymentData := map[string]interface{}{
        "Name":      p.name,
        "Namespace": cfg.Namespace,
        "Image":     cfg.Image,
        "Replicas":  cfg.Replicas,
    }
    
    deployment, err := p.templateEngine.Render(ctx, "templates/deployment.yaml.tmpl", deploymentData)
    if err != nil {
        return fmt.Errorf("failed to render deployment: %w", err)
    }
    
    // Write deployment file
    deploymentPath := filepath.Join(serviceDir, "deployment.yaml")
    if err := os.WriteFile(deploymentPath, deployment, 0644); err != nil {
        return fmt.Errorf("failed to write deployment: %w", err)
    }
    
    // Render service template
    service, err := p.templateEngine.Render(ctx, "templates/service.yaml.tmpl", deploymentData)
    if err != nil {
        return fmt.Errorf("failed to render service: %w", err)
    }
    
    // Write service file
    servicePath := filepath.Join(serviceDir, "service.yaml")
    if err := os.WriteFile(servicePath, service, 0644); err != nil {
        return fmt.Errorf("failed to write service: %w", err)
    }
    
    return nil
}

func (p *MyServicePlugin) Status(config interface{}) services.ServiceStatus {
    return services.ServiceStatus{
        State:   "running",
        Message: "Service is healthy",
        Details: map[string]interface{}{
            "version": "1.0.0",
        },
    }
}

// MyServiceConfig defines the configuration for this service
type MyServiceConfig struct {
    Enabled   bool   `yaml:"enabled" json:"enabled"`
    Namespace string `yaml:"namespace" json:"namespace"`
    Image     string `yaml:"image" json:"image"`
    Replicas  int    `yaml:"replicas" json:"replicas"`
}
```

## Additional Resources

### Documentation

- **Architecture Documentation**: [docs/explanation/architecture.md](../explanation/architecture.md) - Complete system architecture
- **Migration Guide**: [docs/migration/configuration-system-refactor.md](../migration/configuration-system-refactor.md) - Migration from legacy to refactored system
- **Requirements**: [.kiro/specs/configuration-system-refactor/requirements.md](../../.kiro/specs/configuration-system-refactor/requirements.md) - System requirements
- **Design**: [.kiro/specs/configuration-system-refactor/design.md](../../.kiro/specs/configuration-system-refactor/design.md) - Detailed design document
- **Tasks**: [.kiro/specs/configuration-system-refactor/tasks.md](../../.kiro/specs/configuration-system-refactor/tasks.md) - Implementation tasks

### Code Examples

- **Template Engine**: `internal/template/engine.go` - Template engine implementation
- **Configuration Builder**: `internal/config/builder.go` - Configuration builder implementation
- **Service Plugin**: `internal/services/plugin.go` - Service plugin interface
- **Service Registry**: `internal/services/registry.go` - Service registry implementation
- **GitOps Generator**: `internal/gitops/generator.go` - GitOps generation pipeline

### Testing Examples

- **Unit Tests**: `internal/config/builder_test.go` - Configuration builder tests
- **Property Tests**: `internal/config/builder_property_test.go` - Property-based tests
- **Integration Tests**: `internal/gitops/generator_test.go` - GitOps generation tests

### Development Tools

- **Mise**: Build system and task automation (see `.mise.toml`)
- **Go**: Primary programming language (version 1.25.2)
- **Gopter**: Property-based testing library
- **Testify**: Testing assertions and utilities

### Getting Help

If you need help extending the configuration system:

1. **Review existing code**: Look at similar implementations for patterns
2. **Check documentation**: Refer to architecture and design documents
3. **Run tests**: Use `mise run test` to validate your changes
4. **Ask questions**: Open an issue or discussion on GitHub

## Conclusion

The refactored configuration system provides a solid foundation for extending openCenter with new functionality. By following the patterns and best practices outlined in this guide, you can:

- Add new cloud providers with minimal code changes
- Create custom services that integrate seamlessly
- Extend template functionality with custom functions
- Add validation rules for specific use cases
- Contribute to the configuration system with confidence

**Key Takeaways:**

1. **Use interfaces** for extensibility and testability
2. **Inject dependencies** to avoid tight coupling
3. **Aggregate errors** to provide complete feedback
4. **Write comprehensive tests** including property-based tests
5. **Follow established patterns** for consistency
6. **Document your code** for future maintainers

For questions or contributions, refer to the [Contributing Guide](../contributing.md) and open an issue or pull request on GitHub.

Happy coding! 🚀

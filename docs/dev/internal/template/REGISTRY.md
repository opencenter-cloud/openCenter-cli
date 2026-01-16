# Template Registry

The Template Registry is a centralized system for managing template definitions with rich metadata, dependency resolution, and filtering capabilities.

## Overview

The Template Registry provides:

- **Template Management**: Register, retrieve, and unregister templates
- **Metadata Storage**: Store comprehensive metadata for each template
- **Dependency Resolution**: Resolve template dependencies in correct order
- **Filtering**: Filter templates by provider, service, or tags
- **Thread Safety**: Concurrent access support with read-write locks

## Core Types

### TemplateRegistry Interface

```go
type TemplateRegistry interface {
    RegisterTemplate(template TemplateDefinition) error
    GetTemplate(name string) (TemplateDefinition, error)
    GetTemplatesForProvider(provider string) []TemplateDefinition
    GetTemplatesForService(service string) []TemplateDefinition
    ResolveTemplateDependencies(templates []string) ([]TemplateDefinition, error)
    ListTemplates() []TemplateDefinition
    UnregisterTemplate(name string) error
}
```

### TemplateDefinition

```go
type TemplateDefinition struct {
    Name         string              // Unique template identifier
    Path         string              // Path to template file
    Type         TemplateType        // Template category
    Provider     string              // Target provider (empty = universal)
    Services     []string            // Associated services
    Dependencies []string            // Template dependencies
    Conditions   []RenderCondition   // Rendering conditions
    Metadata     TemplateMetadata    // Additional metadata
}
```

### TemplateMetadata

```go
type TemplateMetadata struct {
    Description string   // Human-readable description
    Version     string   // Template version
    Author      string   // Template author
    Tags        []string // Searchable tags
    Priority    int      // Rendering priority (higher = first)
}
```

## Usage Examples

### Basic Registration

```go
registry := NewInMemoryTemplateRegistry()

template := TemplateDefinition{
    Name:     "openstack-base",
    Path:     "/templates/openstack/base.yaml",
    Type:     TemplateTypeInfrastructure,
    Provider: "openstack",
    Metadata: TemplateMetadata{
        Description: "Base OpenStack infrastructure template",
        Version:     "1.0.0",
        Priority:    10,
    },
}

err := registry.RegisterTemplate(template)
if err != nil {
    log.Fatal(err)
}
```

### Retrieving Templates

```go
// By name
template, err := registry.GetTemplate("openstack-base")

// By provider
openstackTemplates := registry.GetTemplatesForProvider("openstack")

// By service
prometheusTemplates := registry.GetTemplatesForService("prometheus")

// All templates
allTemplates := registry.ListTemplates()
```

### Dependency Resolution

```go
// Register templates with dependencies
registry.RegisterTemplate(TemplateDefinition{
    Name: "base",
    Path: "/templates/base.yaml",
    Type: TemplateTypeBase,
})

registry.RegisterTemplate(TemplateDefinition{
    Name:         "app",
    Path:         "/templates/app.yaml",
    Type:         TemplateTypeService,
    Dependencies: []string{"base"},
})

// Resolve dependencies (returns templates in dependency order)
templates, err := registry.ResolveTemplateDependencies([]string{"app"})
// Returns: [base, app]
```

### Service-Based Templates

```go
template := TemplateDefinition{
    Name:     "monitoring-stack",
    Path:     "/templates/monitoring.yaml",
    Type:     TemplateTypeService,
    Services: []string{"prometheus", "grafana", "alertmanager"},
    Metadata: TemplateMetadata{
        Description: "Complete monitoring stack",
        Tags:        []string{"monitoring", "observability"},
    },
}

registry.RegisterTemplate(template)

// Find all templates for prometheus
prometheusTemplates := registry.GetTemplatesForService("prometheus")
```

### Conditional Templates

```go
template := TemplateDefinition{
    Name: "production-config",
    Path: "/templates/prod.yaml",
    Type: TemplateTypeOverlay,
    Conditions: []RenderCondition{
        {
            Type:  ConditionTypeEquals,
            Field: "environment",
            Value: "production",
        },
    },
}

registry.RegisterTemplate(template)
```

## Template Types

- **TemplateTypeInfrastructure**: Infrastructure provisioning templates
- **TemplateTypeService**: Service deployment templates
- **TemplateTypeBase**: Base templates for composition
- **TemplateTypeOverlay**: Overlay templates for customization

## Condition Types

- **ConditionTypeEquals**: Field equals value
- **ConditionTypeNotEquals**: Field does not equal value
- **ConditionTypeContains**: Field contains value
- **ConditionTypeExists**: Field exists
- **ConditionTypeGreaterThan**: Field is greater than value
- **ConditionTypeLessThan**: Field is less than value

## Filtering and Sorting

### Provider Filtering

Templates are filtered by provider with the following rules:
- Templates with empty provider are universal (match all providers)
- Templates with matching provider are included
- Results are sorted by priority (descending) then name (ascending)

### Service Filtering

Templates are filtered by service:
- Templates with the service in their Services list are included
- Results are sorted by priority (descending) then name (ascending)

## Dependency Resolution

The registry provides automatic dependency resolution:

1. **Topological Sort**: Dependencies are resolved in correct order
2. **Circular Detection**: Circular dependencies are detected and rejected
3. **Transitive Resolution**: All transitive dependencies are included
4. **Validation**: Missing dependencies cause errors

### Example Dependency Chain

```
base
  ├── middleware (depends on base)
  └── app (depends on middleware, base)
```

Resolving "app" returns: `[base, middleware, app]`

## Thread Safety

The `InMemoryTemplateRegistry` is thread-safe:
- Read operations use read locks (concurrent reads allowed)
- Write operations use write locks (exclusive access)
- Safe for concurrent use across goroutines

## Validation

Templates are validated on registration:
- Name must not be empty
- Path must not be empty
- Type must be valid
- Self-dependencies are rejected
- Condition types must be valid

## Performance Characteristics

- **Registration**: O(1) average case
- **Retrieval by name**: O(1) average case
- **Provider filtering**: O(n) where n = total templates
- **Service filtering**: O(n * m) where m = services per template
- **Dependency resolution**: O(n + e) where e = edges in dependency graph
- **List all**: O(n log n) due to sorting

## Best Practices

1. **Use Descriptive Names**: Template names should be clear and unique
2. **Set Priorities**: Use priority to control template ordering
3. **Tag Appropriately**: Use tags for flexible filtering
4. **Document Dependencies**: Clearly document why dependencies exist
5. **Version Templates**: Use semantic versioning for templates
6. **Universal Templates**: Use empty provider for universal templates
7. **Validate Early**: Register templates at startup to catch errors early

## Error Handling

Common errors and their meanings:

- `"template name cannot be empty"`: Name is required
- `"template path cannot be empty"`: Path is required
- `"template X not found"`: Template doesn't exist in registry
- `"circular dependency detected"`: Dependency cycle exists
- `"cannot depend on itself"`: Self-dependency not allowed
- `"depends on it"`: Cannot unregister template with dependents

## Integration with Template Engine

The registry works with the template engine:

```go
registry := NewInMemoryTemplateRegistry()
engine := NewGoTemplateEngine()

// Register templates
registry.RegisterTemplate(template)

// Resolve dependencies
templates, _ := registry.ResolveTemplateDependencies([]string{"app"})

// Render in order
for _, tmpl := range templates {
    result, err := engine.Render(ctx, tmpl.Path, data)
    // Process result
}
```

## Testing

The Template Registry has comprehensive test coverage including:

### Unit Tests

- Template registration and validation
- Retrieval by name, provider, and service
- Dependency resolution and circular dependency detection
- Concurrent access safety
- Metadata management

### Property-Based Tests

Property-based tests validate universal properties across many generated inputs:

#### Property 6: Template Metadata Completeness
*For any registered template, it should have complete metadata and be retrievable by name, provider, or service*

#### Property 8: Provider-Specific Template Filtering
*For any provider filter, only templates compatible with that provider should be returned*

This property validates:
- Universal templates (empty provider) are included for all providers
- Provider-specific templates are only returned for matching providers
- Incompatible provider templates are excluded
- Results are sorted by priority (descending) then name (ascending)

#### Property 10: Template Dependency Ordering
*For any set of templates with dependencies, they should always be returned in dependency-first order*

Additional dependency properties:
- Circular dependencies are always detected
- Transitive dependencies are included in resolution
- Templates with no dependencies resolve to themselves
- Shared dependencies appear only once
- Dependency resolution is idempotent

All property-based tests run 100 iterations with randomly generated template graphs to ensure correctness across a wide range of scenarios.

## Future Enhancements

Potential future improvements:
- Persistent storage backend
- Template versioning and rollback
- Template inheritance
- Dynamic template loading
- Template validation hooks
- Caching of resolved dependencies
- Template usage analytics

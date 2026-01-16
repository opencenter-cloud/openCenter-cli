# Template Composition System

## Overview

The template composition system allows base templates to be extended with overlays and patches, enabling flexible and reusable template configurations.

## Implementation

### Core Components

1. **TemplateComposition** - Defines a composition with base template, overlays, and patches
2. **TemplateOverlay** - Represents an overlay with priority ordering and conditions
3. **TemplatePatch** - Targeted modifications (add, remove, replace operations)
4. **TemplateComposer** - Interface for composing templates
5. **DefaultTemplateComposer** - Default implementation with validation and condition evaluation

### Key Features

#### Base Template Extension
- Base templates can be extended with multiple overlays
- Overlays are applied in deterministic priority order (highest priority first)
- When priorities are equal, overlays are sorted by name for consistency

#### Conditional Overlays
- Overlays can have conditions that determine if they should be applied
- Supported condition types:
  - `equals` - Field equals value
  - `not_equals` - Field does not equal value
  - `contains` - Field contains value
  - `exists` - Field exists
  - `greater_than` - Field is greater than value
  - `less_than` - Field is less than value

#### Patch System
- Three patch operations supported:
  - `add` - Add content at specified path
  - `remove` - Remove content at specified path
  - `replace` - Replace content at specified path
- Patches can have conditions for conditional application

#### Validation
- Comprehensive validation of compositions before rendering
- Validates base template, overlays, patches, and conditions
- Supports both registry-based templates and file paths

## Usage Examples

### Basic Overlay Composition

```go
engine := NewGoTemplateEngine()
registry := NewInMemoryTemplateRegistry()
composer := NewDefaultTemplateComposer(engine, registry)

composition := TemplateComposition{
    BaseTemplate: "base-cluster.yaml.tmpl",
    Overlays: []TemplateOverlay{
        {
            Name:     "networking",
            Path:     "overlay-networking.yaml.tmpl",
            Priority: 2,
        },
        {
            Name:     "storage",
            Path:     "overlay-storage.yaml.tmpl",
            Priority: 1,
        },
    },
}

data := map[string]interface{}{
    "ClusterName": "my-cluster",
    "Provider":    "openstack",
}

result, err := composer.Compose(context.Background(), composition, data)
```

### Conditional Overlay

```go
composition := TemplateComposition{
    BaseTemplate: "base.tmpl",
    Overlays: []TemplateOverlay{
        {
            Name:     "production-overlay",
            Path:     "prod-overlay.tmpl",
            Priority: 1,
            Conditions: []RenderCondition{
                {
                    Type:  ConditionTypeEquals,
                    Field: "environment",
                    Value: "production",
                },
            },
        },
    },
}
```

### Using Patches

```go
composition := TemplateComposition{
    BaseTemplate: "base.tmpl",
    Patches: []TemplatePatch{
        {
            Operation: "replace",
            Path:      "version",
            Value:     "2.0.0",
        },
        {
            Operation: "add",
            Path:      "feature",
            Value:     "new-feature: enabled",
        },
    },
}
```

## Testing

Comprehensive test coverage includes:
- Basic overlay application
- Multiple overlays with priority ordering
- Conditional overlays
- Patch operations (add, remove, replace)
- Condition evaluation
- Validation

Run tests:
```bash
go test ./internal/template -run TestTemplateComposition -v
```

## Properties Validated

This implementation validates the following correctness properties from the design document:

- **Property 28**: Overlay Application Correctness - Base templates can be extended with overlays correctly
- **Property 29**: Deterministic Overlay Ordering - Overlays are applied in consistent, deterministic order
- **Property 30**: Overlay Compatibility Validation - Incompatible overlays are rejected during composition

## Files

- `composition.go` - Core composition implementation
- `composition_test.go` - Comprehensive test suite
- `testdata/composition/` - Test templates and examples

## Future Enhancements

Potential improvements for future iterations:
1. YAML/JSON-aware merging for structured content
2. JSONPath support for more precise patch targeting
3. Template inheritance with override mechanisms
4. Composition caching for performance
5. Dry-run mode for composition preview

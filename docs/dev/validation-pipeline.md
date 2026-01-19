# Validation Pipeline Architecture

## Overview

The openCenter CLI uses a composable validation pipeline architecture to validate cluster configurations. This design separates validation concerns into focused, testable components that can be composed together.

## Architecture

### Validation Pipeline

The `ValidationPipeline` is the core abstraction that composes multiple validators:

```go
type ValidationPipeline interface {
    AddValidator(v Validator) ValidationPipeline
    Validate(ctx context.Context, cfg *config.Config) *ValidationResult
    ValidateAsync(ctx context.Context, cfg *config.Config) <-chan *ValidationResult
}
```

### Validator Interface

Each validator implements a simple interface:

```go
type Validator interface {
    Name() string
    Validate(ctx context.Context, cfg *config.Config, result *ValidationResult)
}
```

## Validator Components

The validation pipeline is composed of four focused validators:

### 1. Structural Validator

**Location**: `internal/config/validation/structural.go`

**Responsibility**: Validates basic structure and required fields

**Max Size**: 200 lines

**Validates**:
- Required fields (cluster name, GitOps directory)
- Field formats (email, domain, FQDN, Kubernetes version)
- Node counts (master, worker, Windows worker)

**Example**:
```go
validator := validation.NewStructuralValidator()
```

### 2. Semantic Validator

**Location**: `internal/config/validation/semantic.go`

**Responsibility**: Validates business rules and field relationships

**Max Size**: 250 lines

**Validates**:
- OpenTofu/Terraform configuration
- Windows workers configuration
- SSH authorized keys
- VRRP configuration
- Service-specific configuration
- Managed services configuration

**Example**:
```go
validator := validation.NewSemanticValidator()
```

### 3. Network Validator

**Location**: `internal/config/validation/network.go`

**Responsibility**: Validates network plugin configuration

**Max Size**: 200 lines

**Validates**:
- Network plugin selection (exactly one enabled)
- Plugin-specific configuration (Calico, Cilium, Kube-OVN)
- Subnet configuration (pods, services)

**Example**:
```go
validator := validation.NewNetworkValidator()
```

### 4. Provider Validator

**Location**: `internal/config/validation/provider.go`

**Responsibility**: Delegates to provider-specific validators

**Max Size**: 150 lines

**Validates**:
- Provider selection
- OpenStack configuration and credentials
- AWS configuration and credentials
- vSphere configuration and credentials

**Example**:
```go
validator := validation.NewProviderValidator()
```

## Usage

### Creating a Validation Pipeline

```go
pipeline := validation.NewPipeline().
    AddValidator(validation.NewStructuralValidator()).
    AddValidator(validation.NewSemanticValidator()).
    AddValidator(validation.NewNetworkValidator()).
    AddValidator(validation.NewProviderValidator())

result := pipeline.Validate(ctx, cfg)
if !result.Valid {
    for _, err := range result.Errors {
        fmt.Println(errorFormatter.Format(err))
    }
}
```

### Synchronous Validation

```go
result := pipeline.Validate(ctx, cfg)
```

Runs all validators sequentially and returns a single result.

### Asynchronous Validation

```go
resultChan := pipeline.ValidateAsync(ctx, cfg)
result := <-resultChan
```

Runs all validators concurrently for faster validation of large configurations.

### Partial Validation

You can create pipelines with only specific validators:

```go
// Validate only structure
structuralPipeline := validation.NewPipeline().
    AddValidator(validation.NewStructuralValidator())

// Validate only networking
networkPipeline := validation.NewPipeline().
    AddValidator(validation.NewNetworkValidator())
```

## Adding New Validators

### Step 1: Create Validator

Create a new file in `internal/config/validation/`:

```go
package validation

import (
    "context"
    "github.com/rackerlabs/openCenter-cli/internal/config"
)

type MyValidator struct{}

func NewMyValidator() Validator {
    return &MyValidator{}
}

func (v *MyValidator) Name() string {
    return "my-validator"
}

func (v *MyValidator) Validate(ctx context.Context, cfg *config.Config, result *ValidationResult) {
    // Add validation logic
    if someCondition {
        result.AddError(&ValidationError{
            Type:    "validation",
            Field:   "field.path",
            Message: "error message",
            Validator: v.Name(),
            Suggestions: []string{
                "Suggestion 1",
                "Suggestion 2",
            },
        })
    }
}
```

### Step 2: Keep It Focused

- Single responsibility: validate one aspect of configuration
- Maximum 300 lines of code
- No dependencies on other validators
- Clear, descriptive error messages

### Step 3: Add to Pipeline

```go
pipeline := validation.NewPipeline().
    AddValidator(validation.NewStructuralValidator()).
    AddValidator(validation.NewSemanticValidator()).
    AddValidator(validation.NewNetworkValidator()).
    AddValidator(validation.NewProviderValidator()).
    AddValidator(validation.NewMyValidator()) // Add your validator
```

### Step 4: Write Tests

Create `my_validator_test.go`:

```go
func TestMyValidator(t *testing.T) {
    validator := NewMyValidator()
    cfg := &config.Config{
        // Test configuration
    }
    result := &ValidationResult{
        Valid:    true,
        Errors:   make([]*ValidationError, 0),
        Warnings: make([]*ValidationWarning, 0),
    }

    validator.Validate(context.Background(), cfg, result)

    if result.Valid {
        t.Error("Expected validation to fail")
    }
}
```

## Validator Composition Patterns

### Sequential Validation

Validators run in the order they are added:

```go
pipeline := validation.NewPipeline().
    AddValidator(validation.NewStructuralValidator()).  // 1st
    AddValidator(validation.NewSemanticValidator()).    // 2nd
    AddValidator(validation.NewNetworkValidator()).     // 3rd
    AddValidator(validation.NewProviderValidator())     // 4th
```

### Conditional Validation

Skip validators based on configuration:

```go
pipeline := validation.NewPipeline().
    AddValidator(validation.NewStructuralValidator())

if cfg.OpenCenter.Infrastructure.Provider == "openstack" {
    pipeline.AddValidator(validation.NewProviderValidator())
}
```

### Custom Validation Order

Change the order based on requirements:

```go
// Validate provider first for early failure
pipeline := validation.NewPipeline().
    AddValidator(validation.NewProviderValidator()).
    AddValidator(validation.NewStructuralValidator()).
    AddValidator(validation.NewSemanticValidator()).
    AddValidator(validation.NewNetworkValidator())
```

## Error Handling

### Validation Errors

Errors indicate configuration issues that must be fixed:

```go
result.AddError(&ValidationError{
    Type:        "validation",
    Field:       "opencenter.cluster.cluster_name",
    Value:       cfg.ClusterName(),
    Message:     "cluster name must be set",
    Validator:   v.Name(),
    Suggestions: []string{
        "Set opencenter.cluster.cluster_name to a valid cluster name",
    },
})
```

### Validation Warnings

Warnings indicate potential issues that don't block validation:

```go
result.AddWarning(&ValidationWarning{
    Type:        "validation",
    Field:       "opencenter.cluster.ssh_authorized_keys",
    Message:     "no SSH authorized keys configured",
    Validator:   v.Name(),
    Suggestions: []string{
        "Add SSH public keys for cluster access",
    },
})
```

### Context Cancellation

Validators should respect context cancellation:

```go
func (v *MyValidator) Validate(ctx context.Context, cfg *config.Config, result *ValidationResult) {
    select {
    case <-ctx.Done():
        result.AddError(&ValidationError{
            Type:    "validation",
            Message: "validation cancelled",
        })
        return
    default:
    }

    // Validation logic
}
```

## Testing

### Unit Tests

Test individual validators:

```go
func TestStructuralValidator_ValidateClusterName(t *testing.T) {
    validator := NewStructuralValidator()
    cfg := &config.Config{}
    result := &ValidationResult{
        Valid:  true,
        Errors: make([]*ValidationError, 0),
    }

    validator.Validate(context.Background(), cfg, result)

    if result.Valid {
        t.Error("Expected validation to fail for empty cluster name")
    }
}
```

### Integration Tests

Test the complete pipeline:

```go
func TestValidationPipeline_Complete(t *testing.T) {
    pipeline := NewPipeline().
        AddValidator(NewStructuralValidator()).
        AddValidator(NewSemanticValidator()).
        AddValidator(NewNetworkValidator()).
        AddValidator(NewProviderValidator())

    cfg := loadTestConfig()
    result := pipeline.Validate(context.Background(), cfg)

    if !result.Valid {
        t.Errorf("Validation failed: %v", result.Errors)
    }
}
```

### Size Limit Tests

Ensure validators stay under size limits:

```go
func TestValidatorSizeLimits(t *testing.T) {
    tests := []struct {
        name     string
        filename string
        maxLines int
    }{
        {"structural", "structural.go", 200},
        {"semantic", "semantic.go", 250},
        {"network", "network.go", 200},
        {"provider", "provider.go", 150},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            lineCount := countLines(tt.filename)
            if lineCount > tt.maxLines {
                t.Errorf("%s has %d lines, exceeds limit of %d",
                    tt.filename, lineCount, tt.maxLines)
            }
        })
    }
}
```

## Best Practices

1. **Single Responsibility**: Each validator should validate one aspect of configuration
2. **Size Limits**: Keep validators under their size limits (150-250 lines)
3. **Clear Errors**: Provide actionable error messages with suggestions
4. **No Side Effects**: Validators should only read configuration, not modify it
5. **Context Aware**: Respect context cancellation for long-running validation
6. **Testable**: Write unit tests for each validator
7. **Composable**: Design validators to work independently and together
8. **Performance**: Use async validation for large configurations

## Migration from Old Validator

The old `ClusterConfigValidator` has been refactored to use the pipeline architecture:

### Before

```go
validator := config.NewConfigValidator(false)
result := validator.Validate(ctx, cfg)
```

### After

```go
validator := config.NewConfigValidator(false)
result := validator.Validate(ctx, cfg) // Same interface, new implementation
```

The public API remains the same for backward compatibility, but internally uses the validation pipeline.

## Performance Considerations

### Synchronous vs Asynchronous

- **Synchronous**: Simpler, easier to debug, predictable order
- **Asynchronous**: Faster for large configurations, concurrent execution

### Optimization Tips

1. Order validators by likelihood of failure (fail fast)
2. Use async validation for independent validators
3. Cache expensive validation results
4. Skip validators when not applicable

## Troubleshooting

### Validation Takes Too Long

- Use async validation: `pipeline.ValidateAsync(ctx, cfg)`
- Check for expensive operations in validators
- Add timeouts to context

### Validation Errors Are Unclear

- Add more specific error messages
- Include field paths in errors
- Provide actionable suggestions

### Validators Are Too Large

- Split into multiple validators
- Extract helper functions
- Remove duplicate code

## References

- [Design Document](../../.kiro/specs/security-and-operational-remediation/design.md)
- [Requirements Document](../../.kiro/specs/security-and-operational-remediation/requirements.md)
- [Validator Interface](../../internal/config/validation/pipeline.go)

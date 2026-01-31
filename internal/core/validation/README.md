# Validation Engine

A unified validation system with pluggable validators, automatic suggestion generation, and thread-safe operations.

## Overview

The validation package provides a flexible validation engine that supports:
- Pluggable validators through the Validator interface
- Thread-safe validator registration and lookup
- Context-aware validation with cancellation support
- Automatic suggestion generation for validation errors
- Parallel validation for independent validators

## Package Structure

```
internal/core/validation/
├── doc.go              # Package documentation
├── engine.go           # ValidationEngine implementation
├── types.go            # Core types (ValidationResult, Validator interface)
├── registry.go         # Validator registration and lookup
├── suggestions.go      # Suggestion engine with typo detection
├── engine_test.go      # Engine tests
├── types_test.go       # Types tests
├── registry_test.go    # Registry tests
├── suggestions_test.go # Suggestions tests
├── example_test.go     # Usage examples
└── README.md           # This file
```

## Quick Start

### Basic Usage

```go
// Create a validation engine
engine := validation.NewValidationEngine()

// Register a validator
validator := validation.NewValidatorFunc("cluster-name", func(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
    name, ok := value.(string)
    if !ok {
        result := &validation.ValidationResult{Valid: false}
        result.AddError("cluster.name", "value must be a string")
        return result, nil
    }

    result := &validation.ValidationResult{Valid: true}
    if len(name) == 0 {
        result.AddError("cluster.name", "cluster name cannot be empty")
    }
    return result, nil
})

engine.Register(validator)

// Validate a value
result, err := engine.Validate(context.Background(), "cluster-name", "my-cluster")
if err != nil {
    log.Fatal(err)
}

if !result.Valid {
    for _, issue := range result.Errors {
        fmt.Printf("Error: %s - %s\n", issue.Field, issue.Message)
        for _, suggestion := range issue.Suggestions {
            fmt.Printf("  Suggestion: %s\n", suggestion)
        }
    }
}
```

### Multiple Validators

```go
// Validate sequentially
result, err := engine.ValidateAll(ctx, []string{"validator1", "validator2"}, value)

// Validate in parallel
result, err := engine.ValidateParallel(ctx, []string{"validator1", "validator2"}, value)
```

### Validation Options

```go
opts := &validation.ValidationOptions{
    StopOnFirstError: true,
    IncludeWarnings:  true,
    Context: map[string]interface{}{
        "valid_values": []string{"dev", "staging", "prod"},
    },
}

result, err := engine.ValidateWithOptions(ctx, "environment", "production", opts)
```

## Core Components

### ValidationEngine

The main validation orchestrator that manages validators and coordinates validation operations.

**Key Methods:**
- `Register(validator)` - Register a validator
- `Validate(ctx, name, value)` - Validate using a single validator
- `ValidateAll(ctx, names, value)` - Validate using multiple validators sequentially
- `ValidateParallel(ctx, names, value)` - Validate using multiple validators in parallel

### Registry

Thread-safe validator registration and lookup system.

**Key Methods:**
- `Register(validator)` - Register a validator
- `Get(name)` - Retrieve a validator by name
- `Has(name)` - Check if a validator exists
- `List()` - Get all validator names

### SuggestionEngine

Automatic suggestion generation for validation errors using:
- Typo detection with Levenshtein distance
- Context-aware suggestions based on field names
- Custom suggestion rules

**Key Methods:**
- `EnhanceResult(result, context)` - Add suggestions to validation result
- `AddRule(rule)` - Add custom suggestion rule

### Validator Interface

```go
type Validator interface {
    Name() string
    Validate(ctx context.Context, value interface{}) (*ValidationResult, error)
}
```

## Features

### Thread Safety

All operations are thread-safe:
- Validator registration uses RWMutex
- Parallel validation uses goroutines safely
- Registry operations are protected

### Performance

Target performance characteristics:
- Validator lookup: <100μs
- Single validation: <100μs (validator-dependent)
- Parallel validation: ~1/N of sequential (N validators)
- Suggestion generation: <10μs per issue

### Test Coverage

- 85.7% code coverage
- Comprehensive unit tests for all components
- Example tests demonstrating usage patterns

## Integration with Existing Code

The validation engine is designed to work alongside existing validation code:

1. **Gradual Migration**: Existing validators can be wrapped in the Validator interface
2. **Backward Compatibility**: Does not break existing validation logic
3. **Incremental Adoption**: Can be used for new validators while keeping old ones

## Best Practices

1. Register validators during initialization
2. Use parallel validation for independent validators
3. Provide context for better suggestions
4. Use StopOnFirstError for fail-fast validation
5. Implement Validator interface for complex validation logic
6. Use ValidatorFunc for simple validation functions

## Future Enhancements

Planned improvements:
1. Built-in validators for common patterns (email, URL, CIDR, etc.)
2. Validator composition and chaining
3. Validation result caching
4. Metrics and observability
5. Integration with existing config validators

## Related Documentation

- [Design Document](../../../.kiro/specs/architectural-refactoring/design.md)
- [Requirements](../../../.kiro/specs/architectural-refactoring/requirements.md)
- [Tasks](../../../.kiro/specs/architectural-refactoring/tasks.md)

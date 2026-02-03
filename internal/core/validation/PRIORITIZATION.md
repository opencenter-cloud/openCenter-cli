# Validator Prioritization

## Overview

The ValidationEngine supports validator prioritization to optimize validation performance. Validators with lower priority values execute first, allowing fast validators (format checks, simple rules) to run before slow validators (network checks, file I/O).

## Priority Levels

The validation system defines three standard priority levels:

```go
const (
    // PriorityHigh (50): Fast validators that should run first
    // Examples: format checks, simple pattern matching, length validation
    PriorityHigh = 50

    // PriorityNormal (100): Standard validators with moderate complexity
    // Examples: business logic validation, cross-field validation
    PriorityNormal = 100

    // PriorityLow (200): Slow validators that should run last
    // Examples: network connectivity checks, file I/O, external API calls
    PriorityLow = 200
)
```

## Validator Priority Assignment

### High Priority (Fast Validators)

Validators that perform quick, in-memory checks:

- **ClusterNameValidator**: Format and length checks (regex matching)
- **OrganizationNameValidator**: Format and length checks
- **ConfigStructureValidator**: YAML structure validation
- **SecurityValidator**: Pattern matching for security issues

### Normal Priority (Standard Validators)

Validators with moderate complexity:

- **NetworkValidator**: CIDR parsing and overlap detection
- **ConfigValidator**: Business logic validation
- **ServiceValidator**: Service configuration validation
- **ProviderValidator**: Provider-specific validation

### Low Priority (Slow Validators)

Validators that perform I/O operations:

- **SOPSKeyValidator**: File I/O for key validation
- **FileValidator**: File existence and permission checks
- **GitOpsValidator**: Repository structure validation

## Implementation

### Adding Priority to a Validator

All validators must implement the `Priority()` method:

```go
type MyValidator struct{}

func (v *MyValidator) Name() string {
    return "my-validator"
}

func (v *MyValidator) Priority() int {
    return validation.PriorityHigh // Fast validator
}

func (v *MyValidator) Validate(ctx context.Context, value interface{}) (*ValidationResult, error) {
    // Validation logic...
}
```

### Using ValidatorFunc with Priority

When creating validators using `ValidatorFunc`, use the priority-aware constructor:

```go
// Default priority (PriorityNormal)
validator := validation.NewValidatorFunc("my-validator", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
    // Validation logic...
})

// Custom priority
validator := validation.NewValidatorFuncWithPriority("my-validator", validation.PriorityHigh, func(ctx context.Context, value interface{}) (*ValidationResult, error) {
    // Validation logic...
})
```

## Execution Order

### ValidateAll

When using `ValidateAll`, validators are sorted by priority before execution:

```go
engine := validation.NewValidationEngine()
engine.Register(slowValidator)    // Priority: 200
engine.Register(fastValidator)    // Priority: 50
engine.Register(normalValidator)  // Priority: 100

// Execution order: fastValidator -> normalValidator -> slowValidator
result, err := engine.ValidateAll(ctx, []string{"slow", "fast", "normal"}, value)
```

### ValidateParallel

When using `ValidateParallel`, validators are sorted by priority before parallel execution. While they run concurrently, the sorting ensures validators are launched in priority order:

```go
// Validators are sorted by priority, then launched in parallel
result, err := engine.ValidateParallel(ctx, []string{"slow", "fast", "normal"}, value)
```

### Security Validators

Security validators always run first, regardless of priority. They execute sequentially before any other validators to ensure security checks cannot be bypassed.

## Performance Benefits

Prioritization provides several performance benefits:

1. **Early Failure Detection**: Fast validators catch common errors quickly, avoiding expensive slow validators
2. **Resource Optimization**: Slow validators only run if fast validators pass
3. **Better User Experience**: Users get feedback faster for simple validation errors

### Example Performance Impact

```go
// Without prioritization:
// 1. File I/O validator (200ms)
// 2. Network validator (100ms)
// 3. Format validator (1ms) - FAILS
// Total time: 301ms to detect format error

// With prioritization:
// 1. Format validator (1ms) - FAILS
// Total time: 1ms to detect format error
```

## Custom Priority Values

You can use custom priority values for fine-grained control:

```go
const (
    PriorityCritical = 10   // Critical validators (must run first)
    PriorityHigh     = 50   // Fast validators
    PriorityNormal   = 100  // Standard validators
    PriorityLow      = 200  // Slow validators
    PriorityDeferred = 300  // Deferred validators (run last)
)
```

## Best Practices

1. **Assign Appropriate Priorities**: Consider the actual execution time of your validator
2. **Use Standard Levels**: Stick to PriorityHigh, PriorityNormal, PriorityLow unless you need fine-grained control
3. **Test Performance**: Benchmark your validators to verify priority assignments
4. **Document Priority Choices**: Explain why a validator has a specific priority

## Testing

The validation system includes comprehensive tests for prioritization:

- `TestValidatorPrioritization`: Verifies basic priority ordering
- `TestValidatorPrioritization_CustomPriorities`: Tests custom priority values
- `TestValidatorPrioritization_SamePriority`: Tests validators with identical priorities
- `TestSortValidatorsByPriority`: Tests the sorting algorithm
- `TestValidatorPrioritization_FastValidatorsFirst`: Verifies fast validators run first

## Migration Guide

If you have existing validators without the `Priority()` method:

1. Add the `Priority()` method to your validator:

```go
func (v *MyValidator) Priority() int {
    return validation.PriorityNormal // Choose appropriate priority
}
```

2. Consider the validator's characteristics:
   - Does it perform I/O? → PriorityLow
   - Is it a simple format check? → PriorityHigh
   - Is it business logic? → PriorityNormal

3. Update tests if they depend on execution order

## See Also

- [Validator Guide](VALIDATOR_GUIDE.md): Complete guide to creating validators
- [Performance Testing](performance_target_test.go): Performance benchmarks and targets
- [Engine Documentation](engine.go): ValidationEngine implementation details

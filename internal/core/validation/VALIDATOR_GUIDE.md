# Validator Implementation Guide

Guide for implementing custom validators in the opencenter-cli validation system.

## Table of Contents

- [Overview](#overview)
- [Validator Interface](#validator-interface)
- [Implementation Steps](#implementation-steps)
- [Best Practices](#best-practices)
- [Validation Result Structure](#validation-result-structure)
- [Error Codes and Messages](#error-codes-and-messages)
- [Suggestion Generation](#suggestion-generation)
- [Context Usage](#context-usage)
- [Testing Validators](#testing-validators)
- [Common Patterns](#common-patterns)
- [Examples](#examples)
- [Troubleshooting](#troubleshooting)

## Overview

Validators are pluggable components that implement specific validation logic. Each validator:

- Implements the `Validator` interface
- Has a unique name for registration
- Returns structured validation results
- Can provide suggestions for fixing errors
- Respects context cancellation

## Validator Interface

All validators must implement this interface:

```go
type Validator interface {
    // Name returns the unique identifier for this validator
    Name() string
    
    // Validate performs validation on the given value
    Validate(ctx context.Context, value interface{}) ValidationResult
}
```

## Implementation Steps

### Step 1: Define the Validator Struct

```go
package validators

import (
    "context"
    "github.com/rackerlabs/opencenter-cli/internal/core/validation"
)

type MyValidator struct {
    // Optional: configuration fields
    maxLength int
    allowList []string
}
```

### Step 2: Implement Constructor

```go
func NewMyValidator() *MyValidator {
    return &MyValidator{
        maxLength: 100,
        allowList: []string{"allowed1", "allowed2"},
    }
}

// Optional: Constructor with options
func NewMyValidatorWithOptions(maxLength int, allowList []string) *MyValidator {
    return &MyValidator{
        maxLength: maxLength,
        allowList: allowList,
    }
}
```

### Step 3: Implement Name Method

```go
func (v *MyValidator) Name() string {
    return "my-validator"
}
```

**Naming conventions**:
- Use lowercase with hyphens: `cluster-name`, `file-path`
- Be descriptive: `security-input` not `sec`
- Avoid generic names: `config-validator` not `validator`

### Step 4: Implement Validate Method

```go
func (v *MyValidator) Validate(ctx context.Context, value interface{}) validation.ValidationResult {
    // Check context cancellation
    select {
    case <-ctx.Done():
        return validation.ValidationResult{
            Valid: false,
            Errors: []validation.ValidationError{
                {Message: "validation cancelled"},
            },
        }
    default:
    }
    
    // Type assertion
    str, ok := value.(string)
    if !ok {
        return validation.ValidationResult{
            Valid: false,
            Errors: []validation.ValidationError{
                {
                    Field:   "value",
                    Message: "expected string value",
                    Code:    "INVALID_TYPE",
                },
            },
        }
    }
    
    // Validation logic
    if len(str) > v.maxLength {
        return validation.ValidationResult{
            Valid: false,
            Errors: []validation.ValidationError{
                {
                    Field:      "value",
                    Message:    fmt.Sprintf("value exceeds maximum length of %d", v.maxLength),
                    Code:       "VALUE_TOO_LONG",
                    Suggestion: fmt.Sprintf("shorten to %d characters or less", v.maxLength),
                },
            },
        }
    }
    
    // Success
    return validation.ValidationResult{Valid: true}
}
```

### Step 5: Register the Validator

```go
// In your initialization code
engine := validation.NewValidationEngine()
engine.Register(NewMyValidator())
```

## Best Practices

### 1. Single Responsibility

Each validator should validate one specific aspect:

```go
// Good: Focused validator
type ClusterNameValidator struct{}

// Bad: Too broad
type ClusterValidator struct{} // Validates name, config, resources, etc.
```

### 2. Type Safety

Always perform type assertions with checks:

```go
// Good
str, ok := value.(string)
if !ok {
    return validation.ValidationResult{
        Valid: false,
        Errors: []validation.ValidationError{
            {Message: "expected string value"},
        },
    }
}

// Bad: Panic risk
str := value.(string) // May panic
```

### 3. Context Respect

Check context cancellation for long-running validations:

```go
func (v *MyValidator) Validate(ctx context.Context, value interface{}) validation.ValidationResult {
    // For quick validations (<1ms), context check is optional
    
    // For longer validations, check periodically
    select {
    case <-ctx.Done():
        return validation.ValidationResult{
            Valid: false,
            Errors: []validation.ValidationError{
                {Message: "validation cancelled"},
            },
        }
    default:
    }
    
    // Validation logic...
}
```

### 4. Clear Error Messages

Provide actionable error messages:

```go
// Good: Specific and actionable
"cluster name must contain only lowercase letters, numbers, and hyphens"

// Bad: Vague
"invalid name"
```

### 5. Helpful Suggestions

Include suggestions when possible:

```go
validation.ValidationError{
    Message:    "cluster name must not start with a hyphen",
    Suggestion: "try 'my-cluster' instead of '-my-cluster'",
}
```

### 6. Consistent Error Codes

Use uppercase with underscores:

```go
// Good
Code: "INVALID_CLUSTER_NAME"
Code: "VALUE_TOO_LONG"
Code: "MISSING_REQUIRED_FIELD"

// Bad
Code: "error1"
Code: "InvalidName"
```

## Validation Result Structure

```go
type ValidationResult struct {
    Valid    bool                // Overall validation status
    Errors   []ValidationError   // List of validation errors
    Warnings []ValidationWarning // Optional warnings
}

type ValidationError struct {
    Field      string // Field that failed (optional)
    Message    string // Human-readable error message
    Code       string // Machine-readable error code
    Suggestion string // Optional suggestion for fixing
}

type ValidationWarning struct {
    Field   string // Field with warning
    Message string // Warning message
}
```

### Multiple Errors

Return all errors, not just the first:

```go
func (v *MyValidator) Validate(ctx context.Context, value interface{}) validation.ValidationResult {
    var errors []validation.ValidationError
    
    // Check multiple conditions
    if condition1 {
        errors = append(errors, validation.ValidationError{
            Message: "error 1",
        })
    }
    
    if condition2 {
        errors = append(errors, validation.ValidationError{
            Message: "error 2",
        })
    }
    
    return validation.ValidationResult{
        Valid:  len(errors) == 0,
        Errors: errors,
    }
}
```

## Error Codes and Messages

### Standard Error Codes

Use these standard codes when applicable:

- `INVALID_TYPE`: Value is wrong type
- `INVALID_FORMAT`: Value format is incorrect
- `VALUE_TOO_LONG`: Value exceeds maximum length
- `VALUE_TOO_SHORT`: Value below minimum length
- `MISSING_REQUIRED_FIELD`: Required field is missing
- `INVALID_CHARACTERS`: Contains invalid characters
- `OUT_OF_RANGE`: Value outside valid range
- `DUPLICATE_VALUE`: Value already exists
- `NOT_FOUND`: Referenced value doesn't exist

### Custom Error Codes

For domain-specific errors, use descriptive codes:

```go
// Cluster validation
Code: "INVALID_CLUSTER_NAME"
Code: "CLUSTER_ALREADY_EXISTS"

// File validation
Code: "FILE_NOT_FOUND"
Code: "INVALID_FILE_PERMISSIONS"

// Security validation
Code: "PATH_TRAVERSAL_DETECTED"
Code: "COMMAND_INJECTION_DETECTED"
```

## Suggestion Generation

### Automatic Suggestions

The suggestion engine automatically adds suggestions for:

- Typos (Levenshtein distance)
- Common mistakes
- Context-aware recommendations

### Manual Suggestions

Provide explicit suggestions in your validator:

```go
// Format correction
validation.ValidationError{
    Message:    "cluster name must use hyphens, not underscores",
    Suggestion: fmt.Sprintf("try '%s'", strings.ReplaceAll(name, "_", "-")),
}

// Value correction
validation.ValidationError{
    Message:    "invalid provider",
    Suggestion: "valid providers: openstack, aws, vsphere",
}

// Action suggestion
validation.ValidationError{
    Message:    "configuration file not found",
    Suggestion: "run 'opencenter cluster init' to create configuration",
}
```

## Context Usage

### Timeout Example

```go
func (v *MyValidator) Validate(ctx context.Context, value interface{}) validation.ValidationResult {
    // Create timeout for external call
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    // Make external call with timeout
    if err := v.checkExternal(ctx, value); err != nil {
        return validation.ValidationResult{
            Valid: false,
            Errors: []validation.ValidationError{
                {Message: err.Error()},
            },
        }
    }
    
    return validation.ValidationResult{Valid: true}
}
```

### Cancellation Example

```go
func (v *MyValidator) Validate(ctx context.Context, value interface{}) validation.ValidationResult {
    // Long-running validation
    for i := 0; i < 1000; i++ {
        // Check cancellation periodically
        if i%100 == 0 {
            select {
            case <-ctx.Done():
                return validation.ValidationResult{
                    Valid: false,
                    Errors: []validation.ValidationError{
                        {Message: "validation cancelled"},
                    },
                }
            default:
            }
        }
        
        // Validation work...
    }
    
    return validation.ValidationResult{Valid: true}
}
```

## Testing Validators

### Unit Test Structure

```go
func TestMyValidator_Validate(t *testing.T) {
    tests := []struct {
        name    string
        value   interface{}
        want    bool
        wantErr string
    }{
        {
            name:  "valid value",
            value: "valid-input",
            want:  true,
        },
        {
            name:    "invalid type",
            value:   123,
            want:    false,
            wantErr: "expected string value",
        },
        {
            name:    "too long",
            value:   strings.Repeat("a", 101),
            want:    false,
            wantErr: "exceeds maximum length",
        },
    }
    
    validator := NewMyValidator()
    ctx := context.Background()
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := validator.Validate(ctx, tt.value)
            
            if result.Valid != tt.want {
                t.Errorf("Valid = %v, want %v", result.Valid, tt.want)
            }
            
            if !result.Valid && tt.wantErr != "" {
                found := false
                for _, err := range result.Errors {
                    if strings.Contains(err.Message, tt.wantErr) {
                        found = true
                        break
                    }
                }
                if !found {
                    t.Errorf("Expected error containing %q, got %v", tt.wantErr, result.Errors)
                }
            }
        })
    }
}
```

### Test Context Cancellation

```go
func TestMyValidator_ContextCancellation(t *testing.T) {
    validator := NewMyValidator()
    
    ctx, cancel := context.WithCancel(context.Background())
    cancel() // Cancel immediately
    
    result := validator.Validate(ctx, "test-value")
    
    if result.Valid {
        t.Error("Expected validation to fail on cancelled context")
    }
}
```

### Benchmark Tests

```go
func BenchmarkMyValidator_Validate(b *testing.B) {
    validator := NewMyValidator()
    ctx := context.Background()
    value := "test-value"
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        validator.Validate(ctx, value)
    }
}
```

## Common Patterns

### Pattern 1: String Validation

```go
func (v *StringValidator) Validate(ctx context.Context, value interface{}) validation.ValidationResult {
    str, ok := value.(string)
    if !ok {
        return validation.ValidationResult{
            Valid: false,
            Errors: []validation.ValidationError{
                {Message: "expected string value"},
            },
        }
    }
    
    // Length check
    if len(str) < v.minLength {
        return validation.ValidationResult{
            Valid: false,
            Errors: []validation.ValidationError{
                {
                    Message: fmt.Sprintf("minimum length is %d", v.minLength),
                    Code:    "VALUE_TOO_SHORT",
                },
            },
        }
    }
    
    // Pattern check
    if !v.pattern.MatchString(str) {
        return validation.ValidationResult{
            Valid: false,
            Errors: []validation.ValidationError{
                {
                    Message: "invalid format",
                    Code:    "INVALID_FORMAT",
                },
            },
        }
    }
    
    return validation.ValidationResult{Valid: true}
}
```

### Pattern 2: Struct Validation

```go
func (v *ConfigValidator) Validate(ctx context.Context, value interface{}) validation.ValidationResult {
    cfg, ok := value.(Config)
    if !ok {
        return validation.ValidationResult{
            Valid: false,
            Errors: []validation.ValidationError{
                {Message: "expected Config struct"},
            },
        }
    }
    
    var errors []validation.ValidationError
    
    // Validate required fields
    if cfg.Name == "" {
        errors = append(errors, validation.ValidationError{
            Field:   "name",
            Message: "name is required",
            Code:    "MISSING_REQUIRED_FIELD",
        })
    }
    
    if cfg.Provider == "" {
        errors = append(errors, validation.ValidationError{
            Field:   "provider",
            Message: "provider is required",
            Code:    "MISSING_REQUIRED_FIELD",
        })
    }
    
    return validation.ValidationResult{
        Valid:  len(errors) == 0,
        Errors: errors,
    }
}
```

### Pattern 3: External Validation

```go
func (v *ExternalValidator) Validate(ctx context.Context, value interface{}) validation.ValidationResult {
    // Add timeout for external call
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    // Make external call
    exists, err := v.client.CheckExists(ctx, value)
    if err != nil {
        return validation.ValidationResult{
            Valid: false,
            Errors: []validation.ValidationError{
                {
                    Message: fmt.Sprintf("validation failed: %v", err),
                    Code:    "EXTERNAL_CHECK_FAILED",
                },
            },
        }
    }
    
    if !exists {
        return validation.ValidationResult{
            Valid: false,
            Errors: []validation.ValidationError{
                {
                    Message: "resource not found",
                    Code:    "NOT_FOUND",
                },
            },
        }
    }
    
    return validation.ValidationResult{Valid: true}
}
```

### Pattern 4: Composite Validation

```go
func (v *CompositeValidator) Validate(ctx context.Context, value interface{}) validation.ValidationResult {
    var errors []validation.ValidationError
    
    // Run multiple sub-validations
    for _, subValidator := range v.validators {
        result := subValidator.Validate(ctx, value)
        if !result.Valid {
            errors = append(errors, result.Errors...)
        }
    }
    
    return validation.ValidationResult{
        Valid:  len(errors) == 0,
        Errors: errors,
    }
}
```

## Examples

### Example 1: Email Validator

```go
type EmailValidator struct {
    pattern *regexp.Regexp
}

func NewEmailValidator() *EmailValidator {
    return &EmailValidator{
        pattern: regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`),
    }
}

func (v *EmailValidator) Name() string {
    return "email"
}

func (v *EmailValidator) Validate(ctx context.Context, value interface{}) validation.ValidationResult {
    email, ok := value.(string)
    if !ok {
        return validation.ValidationResult{
            Valid: false,
            Errors: []validation.ValidationError{
                {Message: "expected string value"},
            },
        }
    }
    
    if !v.pattern.MatchString(email) {
        return validation.ValidationResult{
            Valid: false,
            Errors: []validation.ValidationError{
                {
                    Field:      "email",
                    Message:    "invalid email format",
                    Code:       "INVALID_EMAIL",
                    Suggestion: "email must be in format: user@example.com",
                },
            },
        }
    }
    
    return validation.ValidationResult{Valid: true}
}
```

### Example 2: Range Validator

```go
type RangeValidator struct {
    min int
    max int
}

func NewRangeValidator(min, max int) *RangeValidator {
    return &RangeValidator{min: min, max: max}
}

func (v *RangeValidator) Name() string {
    return "range"
}

func (v *RangeValidator) Validate(ctx context.Context, value interface{}) validation.ValidationResult {
    num, ok := value.(int)
    if !ok {
        return validation.ValidationResult{
            Valid: false,
            Errors: []validation.ValidationError{
                {Message: "expected integer value"},
            },
        }
    }
    
    if num < v.min || num > v.max {
        return validation.ValidationResult{
            Valid: false,
            Errors: []validation.ValidationError{
                {
                    Message:    fmt.Sprintf("value must be between %d and %d", v.min, v.max),
                    Code:       "OUT_OF_RANGE",
                    Suggestion: fmt.Sprintf("use a value between %d and %d", v.min, v.max),
                },
            },
        }
    }
    
    return validation.ValidationResult{Valid: true}
}
```

### Example 3: Enum Validator

```go
type EnumValidator struct {
    allowedValues []string
}

func NewEnumValidator(allowedValues []string) *EnumValidator {
    return &EnumValidator{allowedValues: allowedValues}
}

func (v *EnumValidator) Name() string {
    return "enum"
}

func (v *EnumValidator) Validate(ctx context.Context, value interface{}) validation.ValidationResult {
    str, ok := value.(string)
    if !ok {
        return validation.ValidationResult{
            Valid: false,
            Errors: []validation.ValidationError{
                {Message: "expected string value"},
            },
        }
    }
    
    for _, allowed := range v.allowedValues {
        if str == allowed {
            return validation.ValidationResult{Valid: true}
        }
    }
    
    return validation.ValidationResult{
        Valid: false,
        Errors: []validation.ValidationError{
            {
                Message:    fmt.Sprintf("invalid value: %s", str),
                Code:       "INVALID_VALUE",
                Suggestion: fmt.Sprintf("allowed values: %s", strings.Join(v.allowedValues, ", ")),
            },
        },
    }
}
```

## Troubleshooting

### Validator Not Found

**Problem**: `engine.Validate()` returns "validator not found" error

**Solution**: Ensure validator is registered:

```go
engine := validation.NewValidationEngine()
engine.Register(NewMyValidator())

// Verify registration
result, err := engine.Validate(ctx, "my-validator", value)
```

### Type Assertion Panics

**Problem**: Validator panics on type assertion

**Solution**: Always use safe type assertions:

```go
// Bad: May panic
str := value.(string)

// Good: Safe
str, ok := value.(string)
if !ok {
    return validation.ValidationResult{
        Valid: false,
        Errors: []validation.ValidationError{
            {Message: "expected string value"},
        },
    }
}
```

### Context Not Respected

**Problem**: Validation doesn't stop when context is cancelled

**Solution**: Check context in long-running validations:

```go
select {
case <-ctx.Done():
    return validation.ValidationResult{
        Valid: false,
        Errors: []validation.ValidationError{
            {Message: "validation cancelled"},
        },
    }
default:
}
```

### Suggestions Not Appearing

**Problem**: Validation errors don't include suggestions

**Solution**: Add suggestions explicitly or ensure suggestion engine is enabled:

```go
validation.ValidationError{
    Message:    "invalid format",
    Suggestion: "try using lowercase letters only",
}
```

### Performance Issues

**Problem**: Validation is too slow

**Solutions**:

1. Profile the validator:
```go
func BenchmarkMyValidator(b *testing.B) {
    validator := NewMyValidator()
    ctx := context.Background()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        validator.Validate(ctx, "test-value")
    }
}
```

2. Optimize expensive operations:
   - Cache compiled regexes
   - Avoid repeated allocations
   - Use string builders for concatenation
   - Minimize external calls

3. Add caching if appropriate:
```go
type CachedValidator struct {
    cache map[string]validation.ValidationResult
    mu    sync.RWMutex
}
```

## Related Documentation

- [Package Documentation](doc.go)
- [Usage Examples](example_test.go)
- [Built-in Validators](validators/)
- [Migration Guide](../paths/MIGRATION.md)

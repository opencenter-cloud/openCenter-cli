# Error Handling Guide

**doc_type**: reference

This document provides comprehensive guidance on error handling in openCenter CLI, including error classification, structured error usage, error codes, and common patterns.

## Overview

openCenter uses a structured error handling system that provides:
- **Type classification**: Errors are categorized by type (validation, network, security, etc.)
- **Error codes**: Unique codes (E1001-E6999) for documentation and troubleshooting
- **Credential masking**: Automatic masking of sensitive information in error messages
- **Actionable suggestions**: Fix commands and hints for common errors
- **Middleware support**: Consistent error handling across all commands

## Error Types

All errors in openCenter are classified into one of the following types:

| Type | Description | Retryable | Example |
|------|-------------|-----------|---------|
| `ValidationError` | Configuration or input validation failures | No | Invalid cluster name, missing required field |
| `PathError` | File or directory path issues | No | File not found, invalid path |
| `PermissionError` | Access control and permission issues | No | Permission denied, insufficient privileges |
| `TemplateError` | Template rendering failures | No | Invalid template syntax, missing variable |
| `SOPSError` | SOPS encryption/decryption failures | No | Missing Age key, decryption failed |
| `ConfigError` | Configuration loading or parsing errors | No | Invalid YAML, schema mismatch |
| `NetworkError` | Network connectivity issues | Yes | Timeout, connection refused |
| `FileError` | File system operations | Sometimes | Disk full, file locked |
| `SystemError` | System-level errors | Sometimes | Out of memory, panic recovered |
| `UserError` | User-facing errors | No | Invalid command usage |
| `CloudError` | Cloud provider API errors | Yes | OpenStack API error, AWS throttling |
| `CredentialError` | Authentication and credential issues | No | Invalid credentials, expired token |
| `ServiceError` | Service configuration or deployment errors | No | Invalid service config |
| `GenerationError` | GitOps generation failures | No | Template copy failed |

## Error Codes

Error codes follow the pattern `E<category><number>`:

### E1xxx: Validation Errors
- `E1001`: OpenStack region not configured
- `E1002`: SOPS key not found
- `E1003`: Invalid cluster name
- `E1004`: Configuration validation failed
- `E1005`: Required field missing

### E2xxx: Security Errors
- `E2001`: Command injection attempt detected
- `E2002`: Template injection attempt detected
- `E2003`: Path traversal attempt detected
- `E2004`: Invalid EDITOR environment variable

### E3xxx: Network Errors
- `E3001`: Network timeout
- `E3002`: Connection refused

### E4xxx: File System Errors
- `E4001`: File not found
- `E4002`: Permission denied
- `E4003`: Disk space exhausted

### E5xxx: Provider Errors
- `E5001`: OpenStack API error
- `E5002`: AWS API error
- `E5003`: Provider authentication failed

### E6xxx: Operational Errors
- `E6001`: Drift detection failed
- `E6002`: Backup creation failed
- `E6003`: Lock acquisition failed
- `E6004`: Retry budget exhausted

## Creating Structured Errors

### Basic Structured Error

```go
import "github.com/rackerlabs/openCenter-cli/internal/util/errors"

err := &errors.StructuredError{
    Type:    errors.ValidationError,
    Message: "cluster name is required",
    Field:   "cluster_name",
}
```

### Error with Suggestions

```go
err := &errors.StructuredError{
    Type:    errors.ConfigError,
    Message: "configuration file not found",
    Suggestions: []string{
        "Initialize configuration: openCenter cluster init",
        "Check file path: ls -la ~/.config/openCenter/",
    },
}
```

### Error with Context

```go
err := &errors.StructuredError{
    Type:    errors.CloudError,
    Message: "failed to create instance",
    Context: map[string]interface{}{
        "provider": "openstack",
        "region":   "RegionOne",
        "flavor":   "m1.large",
    },
    Retryable: true,
}
```

### Error with File Context

```go
err := &errors.StructuredError{
    Type:         errors.TemplateError,
    Message:      "undefined variable 'cluster_name'",
    FilePath:     "templates/cluster.yaml",
    LineNumber:   42,
    ColumnNumber: 15,
}
```

## Using Error Helper Functions

The `internal/util/errors` package provides helper functions for common error scenarios:

### Validation Errors

```go
err := errors.CreateValidationError(
    "cluster_name",
    "cluster name must start with a letter",
    "Use a valid name: openCenter cluster init my-cluster",
)
```

### Path Errors

```go
err := errors.CreatePathError(
    "/invalid/path",
    "path does not exist",
    originalErr,
)
```

### Permission Errors

```go
err := errors.CreatePermissionError(
    "/path/to/file",
    "write",
    originalErr,
)
```

### SOPS Errors

```go
err := errors.CreateSOPSError(
    "decrypt",
    "age key not found",
    originalErr,
)
```

### Configuration Errors

```go
err := errors.CreateConfigError(
    "opencenter.infrastructure.provider",
    "provider is required",
    originalErr,
)
```

### Cloud Provider Errors

```go
err := errors.CreateCloudError(
    "OpenStack",
    "create_instance",
    "quota exceeded",
    originalErr,
)
```

### Template Errors

```go
err := errors.CreateTemplateError(
    "templates/cluster.yaml",
    42,
    "undefined variable",
    originalErr,
)
```

## Error Wrapping

Always wrap errors with context using `fmt.Errorf` with `%w`:

```go
if err != nil {
    return fmt.Errorf("failed to load configuration: %w", err)
}
```

For structured errors, use the error wrapper:

```go
wrapper := errors.NewDefaultErrorWrapper()
wrappedErr := wrapper.WrapError(err, "failed to initialize cluster")
```

Wrap with type:

```go
wrappedErr := wrapper.WrapErrorWithType(
    err,
    errors.NetworkError,
    "network operation failed",
)
```

## Error Middleware

Use error middleware in commands for consistent error handling:

```go
import (
    "context"
    "github.com/rackerlabs/openCenter-cli/internal/util/errors"
)

func runCommand(ctx context.Context) error {
    // Create middleware with logger
    middleware := errors.NewErrorMiddleware(logger)
    
    // Wrap command execution
    return middleware.Handle(ctx, "cluster_init", func() error {
        // Command logic here
        return doWork()
    })
}
```

The middleware provides:
- **Panic recovery**: Catches panics and converts to errors
- **Credential masking**: Automatically masks sensitive data
- **Logging**: Logs errors with correlation IDs
- **Context propagation**: Adds correlation IDs and operation context

### Wrapping Cobra Commands

```go
func newClusterInitCmd(middleware *errors.ErrorMiddleware) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "init",
        Short: "Initialize cluster configuration",
        RunE: middleware.WrapCommandWithArgs("cluster_init", func(ctx context.Context, args []string) error {
            // Command implementation
            return initCluster(ctx, args)
        }),
    }
    return cmd
}
```

## Error Formatting

Use the error formatter for user-friendly output:

```go
import "github.com/rackerlabs/openCenter-cli/internal/ui"

formatter := ui.NewDefaultErrorFormatter()

// Basic formatting
formatted := formatter.Format(err)
fmt.Println(formatted)

// With error code
formatted := formatter.FormatWithCode(err, "E1001")
fmt.Println(formatted)

// With error info
info, ok := formatter.GetErrorInfo("E1001")
if ok {
    formatted := formatter.FormatWithErrorInfo(err, info)
    fmt.Println(formatted)
}

// Multiple errors with limit
formatted := formatter.FormatMultipleWithLimit(errs, 5, false)
fmt.Println(formatted)
```

## Common Patterns

### Pattern 1: Validation with Multiple Errors

```go
func validateConfig(cfg *config.Config) error {
    aggregator := errors.NewErrorAggregator()
    
    if cfg.ClusterName == "" {
        aggregator.AddErrorWithContext(
            "cluster_name",
            errors.CreateValidationError("cluster_name", "cluster name is required"),
        )
    }
    
    if cfg.Organization == "" {
        aggregator.AddErrorWithContext(
            "organization",
            errors.CreateValidationError("organization", "organization is required"),
        )
    }
    
    if aggregator.HasErrors() {
        return aggregator.ToError()
    }
    
    return nil
}
```

### Pattern 2: Retryable Operations

```go
func callAPI(ctx context.Context) error {
    err := doAPICall()
    if err != nil {
        // Create retryable error
        return &errors.StructuredError{
            Type:      errors.NetworkError,
            Message:   "API call failed",
            Cause:     err,
            Retryable: true,
        }
    }
    return nil
}
```

### Pattern 3: Error Recovery

```go
func processWithRecovery(ctx context.Context) (err error) {
    middleware := errors.NewErrorMiddleware(logger)
    
    defer func() {
        if recovered := middleware.RecoverPanic(ctx, "process"); recovered != nil {
            err = recovered
        }
    }()
    
    // Potentially panicking code
    return doWork()
}
```

### Pattern 4: Contextual Error Information

```go
func deployCluster(ctx context.Context, cluster string) error {
    err := deploy(cluster)
    if err != nil {
        return errors.WrapWithContext(err, map[string]interface{}{
            "cluster":   cluster,
            "operation": "deploy",
            "timestamp": time.Now(),
        })
    }
    return nil
}
```

### Pattern 5: File Operation Errors

```go
func readConfigFile(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, errors.CreatePathError(path, "configuration file not found", err)
        }
        if os.IsPermission(err) {
            return nil, errors.CreatePermissionError(path, "read", err)
        }
        return nil, fmt.Errorf("failed to read config file: %w", err)
    }
    
    // Parse config...
    return cfg, nil
}
```

## Error Handling Checklist

When implementing error handling:

- [ ] Use structured errors from `internal/util/errors`
- [ ] Assign appropriate error type
- [ ] Wrap errors with `%w` to preserve error chain
- [ ] Add field context for validation errors
- [ ] Include file path and line number for file-related errors
- [ ] Mark network/cloud errors as retryable
- [ ] Provide actionable suggestions
- [ ] Use error middleware in commands
- [ ] Never expose credentials in error messages
- [ ] Log errors with correlation IDs
- [ ] Test error paths in unit tests
- [ ] Document error codes in error registry

## Testing Error Handling

### Unit Test Example

```go
func TestValidateClusterName(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
        errType errors.ErrorType
    }{
        {
            name:    "valid name",
            input:   "my-cluster",
            wantErr: false,
        },
        {
            name:    "empty name",
            input:   "",
            wantErr: true,
            errType: errors.ValidationError,
        },
        {
            name:    "invalid characters",
            input:   "cluster;rm",
            wantErr: true,
            errType: errors.ValidationError,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateClusterName(tt.input)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("validateClusterName() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            
            if err != nil {
                structuredErr, ok := err.(*errors.StructuredError)
                if !ok {
                    t.Errorf("expected StructuredError, got %T", err)
                    return
                }
                
                if structuredErr.Type != tt.errType {
                    t.Errorf("expected error type %v, got %v", tt.errType, structuredErr.Type)
                }
            }
        })
    }
}
```

### Property Test Example

```go
func TestProperty_ErrorsAreStructured(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    properties.Property("all errors are structured", prop.ForAll(
        func(message string) bool {
            if message == "" {
                return true
            }
            
            handler := errors.NewDefaultErrorHandler()
            err := fmt.Errorf("%s", message)
            
            structured := handler.HandleError(err)
            
            // Verify it's structured
            return structured != nil && structured.Type != ""
        },
        gen.AnyString().SuchThat(func(s string) bool { return s != "" }),
    ))
    
    properties.TestingRun(t, gopter.ConsoleReporter(false))
}
```

## Migration Guide

### Migrating from Plain Errors

**Before:**
```go
if err != nil {
    return fmt.Errorf("failed to load config")
}
```

**After:**
```go
if err != nil {
    return errors.CreateConfigError(
        "config",
        "failed to load configuration",
        err,
    )
}
```

### Migrating from errors.New

**Before:**
```go
return errors.New("cluster name is required")
```

**After:**
```go
return errors.CreateValidationError(
    "cluster_name",
    "cluster name is required",
)
```

### Adding Error Codes

**Before:**
```go
return fmt.Errorf("OpenStack region not configured")
```

**After:**
```go
info, _ := formatter.GetErrorInfo("E1001")
return formatter.FormatWithErrorInfo(
    fmt.Errorf("OpenStack region not configured"),
    info,
)
```

## Best Practices

1. **Always use structured errors** for application errors
2. **Wrap errors with context** using `%w` format verb
3. **Classify errors correctly** by type
4. **Mark retryable errors** appropriately
5. **Provide actionable suggestions** for common errors
6. **Use error middleware** in all commands
7. **Never expose credentials** in error messages
8. **Test error paths** thoroughly
9. **Document error codes** in the registry
10. **Log errors with correlation IDs** for traceability

## References

- [Error Handling in Go](https://go.dev/blog/error-handling-and-go)
- [Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors)
- [Structured Error Design Document](../../.kiro/specs/security-and-operational-remediation/design.md)
- [Error Formatter Implementation](../../internal/ui/error_formatter.go)
- [Error Middleware Implementation](../../internal/util/errors/middleware.go)

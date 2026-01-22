# Logging Migration Guide


## Table of Contents

- [Overview](#overview)
- [Why Migrate?](#why-migrate)
- [Migration Steps](#migration-steps)
- [Migration Checklist](#migration-checklist)
- [Common Patterns](#common-patterns)
- [Testing](#testing)
- [Backward Compatibility](#backward-compatibility)
- [Performance Considerations](#performance-considerations)
- [Troubleshooting](#troubleshooting)
- [References](#references)
## Overview

This guide explains how to migrate from logrus to the new structured logging system in `internal/observability`.

## Why Migrate?

The new logging system provides:

1. **Credential Masking**: Automatic masking of sensitive data (API keys, passwords, tokens)
2. **Structured Logging**: JSON format with consistent fields
3. **Correlation IDs**: Track operations across multiple log entries
4. **Log Shipping**: Send logs to external systems (Syslog, Loki)
5. **Context Integration**: Extract logging context from Go contexts

## Migration Steps

### Step 1: Import the New Logger

Replace:
```go
import "github.com/sirupsen/logrus"
```

With:
```go
import "github.com/rackerlabs/openCenter-cli/internal/observability"
```

### Step 2: Update Logger Initialization

**Old (logrus):**
```go
logger := logrus.New()
logger.SetLevel(logrus.InfoLevel)
logger.SetFormatter(&logrus.JSONFormatter{})
```

**New:**
```go
logger := observability.NewDefaultLogger(observability.LoggerConfig{
    Level:  observability.InfoLevel,
    Format: observability.JSONFormat,
    Output: os.Stdout,
})
```

### Step 3: Update Logging Calls

**Old (logrus):**
```go
logrus.Info("Processing cluster")
logrus.WithFields(logrus.Fields{
    "cluster": "prod",
    "operation": "bootstrap",
}).Info("Starting operation")
```

**New:**
```go
observability.Info("Processing cluster")
observability.WithFields(map[string]interface{}{
    "cluster": "prod",
    "operation": "bootstrap",
}).Info("Starting operation")
```

Or using the Logger interface:
```go
logger.Info("Processing cluster")
logger.WithFields(
    observability.Field{Key: "cluster", Value: "prod"},
    observability.Field{Key: "operation", Value: "bootstrap"},
).Info("Starting operation")
```

### Step 4: Add Correlation IDs

**New feature - not available in logrus:**
```go
// Generate correlation ID (e.g., using UUID)
correlationID := uuid.New().String()

// Create logger with correlation ID
logger := observability.GetGlobalLogger().WithCorrelationID(correlationID)

// All logs from this logger will include the correlation ID
logger.Info("Operation started")
logger.Info("Operation completed")
```

### Step 5: Use Context-Based Logging

**New feature - extract logging context from Go context:**
```go
// Add correlation ID to context
ctx := context.WithValue(ctx, observability.CorrelationIDKey, correlationID)
ctx = context.WithValue(ctx, observability.ClusterKey, "prod")

// Create logger from context
logger := observability.FromContext(ctx)

// Logger automatically includes correlation ID and cluster name
logger.Info("Processing request")
```

### Step 6: Configure Log Shipping

**New feature - ship logs to external systems:**
```go
// Configure Syslog shipping
syslogShipper, err := observability.NewSyslogShipper("tcp", "localhost:514")
if err != nil {
    return err
}

// Configure Loki shipping
lokiShipper := observability.NewLokiShipper("http://localhost:3100/loki/api/v1/push")

// Use multiple shippers
multiShipper := observability.NewMultiShipper(syslogShipper, lokiShipper)

// Set shipper on global logger
observability.SetGlobalLogShipper(multiShipper)
```

## Migration Checklist

- [ ] Replace logrus imports with observability imports
- [ ] Update logger initialization
- [ ] Update logging calls (Info, Debug, Warn, Error)
- [ ] Update WithFields calls
- [ ] Add correlation IDs to operations
- [ ] Use context-based logging where appropriate
- [ ] Configure log shipping if needed
- [ ] Test that credentials are masked in logs
- [ ] Verify JSON format output
- [ ] Update tests to use new logger

## Common Patterns

### Pattern 1: Component Logger

**Old:**
```go
func Logger() *logrus.Entry {
    return config.GetGlobalLogger().WithField("component", "talos")
}
```

**New:**
```go
func Logger() observability.Logger {
    return observability.GetGlobalLogger().WithFields(
        observability.Field{Key: "component", Value: "talos"},
    )
}
```

### Pattern 2: Operation Logging

**Old:**
```go
logrus.WithFields(logrus.Fields{
    "cluster": cluster,
    "operation": "bootstrap",
}).Info("Starting bootstrap")
```

**New:**
```go
logger := observability.GetGlobalLogger().WithFields(
    observability.Field{Key: "cluster", Value: cluster},
    observability.Field{Key: "operation", Value: "bootstrap"},
)
logger.Info("Starting bootstrap")
```

### Pattern 3: Error Logging

**Old:**
```go
logrus.WithError(err).Error("Operation failed")
```

**New:**
```go
observability.Error("Operation failed", 
    observability.Field{Key: "error", Value: err},
)
```

## Testing

### Test Credential Masking

```go
func TestCredentialMasking(t *testing.T) {
    var buf bytes.Buffer
    logger := observability.NewDefaultLogger(observability.LoggerConfig{
        Level:  observability.InfoLevel,
        Format: observability.JSONFormat,
        Output: &buf,
    })

    // Log a message with a credential
    logger.Info("AWS key: AKIAIOSFODNN7EXAMPLE")

    // Verify credential is masked
    output := buf.String()
    if strings.Contains(output, "AKIAIOSFODNN7EXAMPLE") {
        t.Error("Credential was not masked")
    }
    if !strings.Contains(output, "****") {
        t.Error("Masked indicator not found")
    }
}
```

### Test Correlation ID Propagation

```go
func TestCorrelationID(t *testing.T) {
    var buf bytes.Buffer
    logger := observability.NewDefaultLogger(observability.LoggerConfig{
        Level:  observability.InfoLevel,
        Format: observability.JSONFormat,
        Output: &buf,
    })

    correlationID := "test-123"
    logger = logger.WithCorrelationID(correlationID)

    logger.Info("Test message")

    var entry observability.LogEntry
    if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
        t.Fatal(err)
    }

    if entry.CorrelationID != correlationID {
        t.Errorf("Expected correlation ID %s, got %s", correlationID, entry.CorrelationID)
    }
}
```

## Backward Compatibility

The migration helper provides backward compatibility functions:

- `observability.WithField()` - Compatible with `logrus.WithField()`
- `observability.WithFields()` - Compatible with `logrus.WithFields()`
- `observability.Info()`, `Debug()`, `Warn()`, `Error()` - Compatible with logrus global functions

This allows gradual migration without breaking existing code.

## Performance Considerations

- Log shipping is asynchronous to avoid blocking
- Credential masking uses compiled regex patterns (cached)
- JSON marshaling is only done once per log entry
- Mutex locks are held for minimal time

## Troubleshooting

### Logs Not Appearing

Check that the log level is set correctly:
```go
observability.SetGlobalLogLevel(observability.DebugLevel)
```

### Credentials Not Masked

Verify the credential pattern is registered:
```go
logger := observability.NewDefaultLogger(config)
// Default patterns are registered automatically
```

### Log Shipping Failures

Check shipper configuration and network connectivity:
```go
shipper, err := observability.NewSyslogShipper("tcp", "localhost:514")
if err != nil {
    log.Printf("Failed to create shipper: %v", err)
}
```

## References

- [Structured Logging Design](../specs/security-and-operational-remediation/design.md#structured-logger)
- [Requirements](../specs/security-and-operational-remediation/requirements.md#requirement-12-centralized-logging)
- [Property Tests](../../internal/observability/logger_property_test.go)

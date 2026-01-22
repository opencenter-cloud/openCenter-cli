# Feature Flag Structured Logging


## Table of Contents

- [Overview](#overview)
- [Logging Framework](#logging-framework)
- [Log Events](#log-events)
- [Configuration](#configuration)
- [Use Cases](#use-cases)
- [Integration with Monitoring Systems](#integration-with-monitoring-systems)
- [Testing](#testing)
- [Example Code](#example-code)
- [Related Documentation](#related-documentation)
- [Requirements](#requirements)
## Overview

The openCenter CLI uses structured logging to track feature flag evaluation and usage. This provides visibility into which systems are active during execution and helps with monitoring, debugging, and troubleshooting during the migration from legacy to new implementations.

## Logging Framework

Feature flag logging uses [logrus](https://github.com/sirupsen/logrus) for structured logging with the following benefits:

- **Structured Data**: All log entries include structured fields for easy parsing and analysis
- **Multiple Formats**: Supports JSON, text, and YAML output formats
- **Log Levels**: Configurable log levels (debug, info, warn, error)
- **Integration**: Integrates with existing openCenter logging infrastructure

## Log Events

### Initialization Event

When the feature flag system initializes, it logs the state of all feature flags:

```json
{
  "component": "feature_flags",
  "operation": "initialization",
  "new_template_engine": true,
  "pipeline_generator": false,
  "new_config_builder": false,
  "service_registry": false,
  "all_new_features": false,
  "debug_enabled": false,
  "level": "info",
  "msg": "Feature flags initialized",
  "time": "2026-01-16T00:15:13.637-06:00"
}
```

A summary event is also logged with the count of active features:

```json
{
  "component": "feature_flags",
  "active_features": 1,
  "total_features": 4,
  "level": "info",
  "msg": "Feature flag summary",
  "time": "2026-01-16T00:15:13.637-06:00"
}
```

### Evaluation Event

Each time a feature flag is evaluated (first access only, subsequent accesses use cache), a log entry is created:

```json
{
  "component": "feature_flags",
  "operation": "evaluation",
  "feature_name": "new template engine",
  "env_var": "OPENCENTER_USE_NEW_TEMPLATE_ENGINE",
  "enabled": true,
  "source": "environment",
  "level": "debug",
  "msg": "Feature flag evaluated",
  "time": "2026-01-16T00:15:13.638-06:00"
}
```

**Fields:**
- `component`: Always "feature_flags"
- `operation`: Always "evaluation"
- `feature_name`: Human-readable name of the feature
- `env_var`: Environment variable name
- `enabled`: Boolean indicating if the feature is enabled
- `source`: Where the value came from (see below)

**Source Values:**
- `environment`: Value explicitly set via environment variable
- `all_new_features`: Value inherited from `OPENCENTER_ENABLE_ALL_NEW_FEATURES`
- `default`: Default value (false) when no environment variable is set

### Cache Clear Event

When the feature flag cache is cleared (typically during testing or configuration reload):

```json
{
  "component": "feature_flags",
  "operation": "cache_clear",
  "all_new_features_before": false,
  "all_new_features_after": true,
  "debug_enabled_before": false,
  "debug_enabled_after": true,
  "level": "info",
  "msg": "Feature flag cache cleared and reloaded",
  "time": "2026-01-16T00:15:13.639-06:00"
}
```

## Configuration

### Log Level

Feature flag evaluation logs at the **debug** level by default. To see these logs, set the log level to debug:

```bash
# Via CLI config
opencenter config ide --log-level debug

# Via environment variable
export OPENCENTER_LOG_LEVEL=debug
```

### Debug Mode

Enable debug mode for additional stderr output alongside structured logs:

```bash
export OPENCENTER_FEATURE_FLAG_DEBUG=true
```

When debug mode is enabled:
1. Evaluation logs are promoted to **info** level
2. Human-readable output is written to stderr:
   ```
   [FEATURE FLAG] new template engine is enabled (OPENCENTER_USE_NEW_TEMPLATE_ENGINE, source: environment)
   ```

### Log Format

Configure the log format in your CLI configuration:

```yaml
logging:
  level: debug
  format: json  # or "text" or "yaml"
  output: stderr  # or "stdout" or a file path
```

## Use Cases

### Monitoring Active Features

Query logs to see which features are active in production:

```bash
# Using jq to filter feature flag logs
cat app.log | jq 'select(.component == "feature_flags" and .operation == "initialization")'
```

### Tracking Feature Adoption

Monitor which features are being used over time:

```bash
# Count evaluations by feature
cat app.log | jq -r 'select(.operation == "evaluation") | .feature_name' | sort | uniq -c
```

### Debugging Configuration Issues

When troubleshooting why a feature isn't working as expected:

```bash
# Enable debug mode
export OPENCENTER_FEATURE_FLAG_DEBUG=true
export OPENCENTER_LOG_LEVEL=debug

# Run command and observe feature flag evaluation
opencenter cluster render
```

### Auditing Feature Flag Sources

Determine where feature flag values are coming from:

```bash
# Show source of each feature flag
cat app.log | jq 'select(.operation == "evaluation") | {feature: .feature_name, enabled: .enabled, source: .source}'
```

## Integration with Monitoring Systems

### Prometheus Metrics

The structured logs can be parsed by log aggregators to generate metrics:

- `opencenter_feature_flag_enabled{feature="template_engine"}` - Gauge (0 or 1)
- `opencenter_feature_flag_evaluations_total{feature="template_engine"}` - Counter

### Log Aggregation

Common log aggregation tools can parse the JSON logs:

- **ELK Stack**: Logstash can parse JSON logs directly
- **Splunk**: JSON format is natively supported
- **Datadog**: Structured logs enable rich filtering and dashboards
- **CloudWatch Logs**: JSON logs can be queried with CloudWatch Insights

### Example Queries

**Splunk:**
```
index=opencenter component="feature_flags" operation="evaluation"
| stats count by feature_name, enabled
```

**CloudWatch Insights:**
```
fields @timestamp, feature_name, enabled, source
| filter component = "feature_flags" and operation = "evaluation"
| stats count() by feature_name, enabled
```

## Testing

The feature flag logging system includes comprehensive tests:

```bash
# Run all feature flag tests
go test ./internal/config -run FeatureFlag

# Run specific logging tests
go test ./internal/config -run TestFeatureFlagStructuredLogging
go test ./internal/config -run TestFeatureFlagLoggingFields
go test ./internal/config -run TestFeatureFlagActiveCount
```

## Example Code

See `internal/config/feature_flags_example_test.go` for working examples of:

- Basic structured logging
- Debug mode usage
- Getting feature flag status

## Related Documentation

- [Feature Flags](../../.kiro/specs/configuration-system-refactor/design.md#feature-flags) - Design document
- [Migration Guide](../../.kiro/specs/configuration-system-refactor/tasks.md#migration-guide) - Migration timeline
- [Logging Configuration](../reference/configuration.md#logging) - CLI logging configuration

## Requirements

This implementation satisfies:

- **Requirement 8.6**: "THE System SHALL log all configuration operations with structured logging for analysis"
- Provides visibility into which systems are active
- Tracks feature flag evaluation with source attribution
- Enables monitoring and debugging during migration

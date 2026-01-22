# Template Engine Feature Flag


## Table of Contents

- [Overview](#overview)
- [Purpose](#purpose)
- [Usage](#usage)
- [Migration Strategy](#migration-strategy)
- [Affected Functions](#affected-functions)
- [Testing](#testing)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)
- [Future Plans](#future-plans)
- [References](#references)
## Overview

The `OPENCENTER_USE_NEW_TEMPLATE_ENGINE` environment variable provides a feature flag for controlling which template engine implementation is used during the migration from the legacy text/template system to the new GoTemplateEngine.

## Purpose

This feature flag enables:

1. **Gradual Migration**: Deploy the new engine without immediately switching all workloads
2. **Safe Rollback**: Instantly revert to legacy system if issues are detected
3. **A/B Testing**: Compare performance and behavior between engines
4. **Risk Mitigation**: Test in development/staging before production rollout
5. **Zero Downtime**: Switch engines without code changes or redeployment

## Usage

### Environment Variable

```bash
# Enable new template engine
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true

# Disable (use legacy system)
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=false

# Or simply unset to use default (legacy)
unset OPENCENTER_USE_NEW_TEMPLATE_ENGINE
```

### Valid Values

**Enable new engine** (case-insensitive):
- `true`
- `1`
- `yes`
- `on`

**Use legacy engine** (default):
- Unset
- `false`
- `0`
- `no`
- `off`
- Any other value

### Programmatic Check

```go
import "github.com/rackerlabs/openCenter-cli/internal/template"

if template.UseNewTemplateEngine() {
    // New engine is enabled
} else {
    // Legacy engine is active (default)
}
```

## Migration Strategy

### Phase 1: Development Testing

```bash
# Enable in development environment
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true

# Run tests
go test ./...

# Test cluster operations
opencenter cluster init ...
opencenter cluster validate ...
```

### Phase 2: Staging Validation

```bash
# Enable in staging environment
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true

# Deploy and monitor
# - Check logs for errors
# - Verify output matches legacy system
# - Monitor performance metrics
```

### Phase 3: Production Rollout

```bash
# Gradual rollout strategy:
# 1. Enable for 10% of workloads
# 2. Monitor for 24-48 hours
# 3. Increase to 50% if stable
# 4. Monitor for another 24-48 hours
# 5. Enable for 100% if no issues

export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true
```

### Phase 4: Rollback (If Needed)

```bash
# Immediate rollback - no code changes needed
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=false

# Or unset
unset OPENCENTER_USE_NEW_TEMPLATE_ENGINE

# System automatically reverts to legacy implementation
```

## Affected Functions

The following functions respect the feature flag:

### `RenderTemplateToFile`

```go
// Automatically uses feature flag
err := template.RenderTemplateToFile(fsys, "template.yaml", outputPath, data)
```

- **Flag enabled**: Uses `GoTemplateEngine` with caching
- **Flag disabled**: Uses legacy `text/template` directly

### `RenderTemplateToWriter`

```go
// Automatically uses feature flag
err := template.RenderTemplateToWriter(fsys, "template.yaml", data, writer)
```

- **Flag enabled**: Uses `GoTemplateEngine`
- **Flag disabled**: Uses legacy implementation

### Direct Engine Usage (No Flag)

```go
// These functions bypass the feature flag and always use new engine
engine := template.NewGoTemplateEngine()
err := template.RenderWithEngine(engine, fsys, "template.yaml", outputPath, data)
```

## Testing

### Unit Tests

The feature flag is thoroughly tested in:

- `internal/template/legacy_test.go` - Flag detection logic
- `internal/template/migration_test.go` - Output identity verification

Run tests:

```bash
# Test flag detection
go test -v ./internal/template -run TestUseNewTemplateEngine

# Test feature flag behavior
go test -v ./internal/template -run TestFeatureFlag

# Test output identity
go test -v ./internal/template -run TestFeatureFlagOutputIdentity
```

### Integration Testing

Test with actual cluster operations:

```bash
# Test with legacy engine (default)
unset OPENCENTER_USE_NEW_TEMPLATE_ENGINE
opencenter cluster init test-org test-cluster openstack

# Test with new engine
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true
opencenter cluster init test-org test-cluster openstack

# Compare outputs
diff -r ~/.config/openCenter/clusters/test-org/test-cluster-legacy \
        ~/.config/openCenter/clusters/test-org/test-cluster-new
```

## Monitoring

### Logging

The template engine logs which implementation is being used:

```go
if template.UseNewTemplateEngine() {
    log.Debug("Using new GoTemplateEngine")
} else {
    log.Debug("Using legacy text/template")
}
```

### Metrics

Monitor these metrics when testing the new engine:

- **Template rendering time**: Should be faster with caching
- **Memory usage**: Should be similar or better
- **Error rates**: Should be equal or lower
- **Output correctness**: Must be identical to legacy

## Troubleshooting

### Issue: Templates render differently

**Solution**: This should never happen. If it does:

1. Disable the feature flag immediately
2. Report the issue with template content and data
3. Run comparison tests to identify differences

```bash
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=false
```

### Issue: Performance degradation

**Solution**: 

1. Check if caching is working correctly
2. Profile the template rendering
3. Compare with legacy system performance
4. Report findings for optimization

### Issue: Errors with new engine

**Solution**:

1. Check error messages for details
2. Verify template syntax is valid
3. Test with legacy engine to confirm template is correct
4. Report if error handling differs from legacy

```bash
# Test with legacy
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=false
opencenter cluster validate ...

# Test with new
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true
opencenter cluster validate ...
```

## Future Plans

### Short Term (Current)

- Feature flag enables gradual migration
- Both engines coexist
- Default is legacy system

### Medium Term (Next Release)

- After validation period, switch default to new engine
- Feature flag allows opting back to legacy if needed
- Deprecation notice for legacy system

### Long Term (Future Release)

- Remove feature flag
- Remove legacy implementation
- New engine becomes the only option

## References

- [Template Engine Migration Guide](../../docs/migration/template-engine.md)
- [Quick Reference](../../docs/migration/template-engine-quick-reference.md)
- [Design Document](../../.kiro/specs/configuration-system-refactor/design.md)
- [Requirements](../../.kiro/specs/configuration-system-refactor/requirements.md)

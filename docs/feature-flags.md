# Feature Flags

## Overview

openCenter CLI uses feature flags to enable gradual migration from legacy systems to new implementations. This allows testing new features in production while maintaining the ability to quickly rollback if issues are discovered.

## Available Feature Flags

### Template Engine
- **Environment Variable**: `OPENCENTER_USE_NEW_TEMPLATE_ENGINE`
- **Default**: `false` (use legacy text/template)
- **New System**: Enhanced template engine with caching and better error messages
- **Status**: Available for testing

### GitOps Generator
- **Environment Variable**: `OPENCENTER_USE_PIPELINE_GENERATOR`
- **Default**: `false` (use legacy generation functions)
- **New System**: Pipeline-based generation with rollback and progress reporting
- **Status**: In development (Tasks 4.1-4.3)

### Configuration Builder
- **Environment Variable**: `OPENCENTER_USE_NEW_CONFIG_BUILDER`
- **Default**: `false` (use legacy reflection-based approach)
- **New System**: Type-safe fluent builder with compile-time validation
- **Status**: Planned (Tasks 2.1-2.4)

### Service Registry
- **Environment Variable**: `OPENCENTER_USE_SERVICE_REGISTRY`
- **Default**: `false` (use legacy hardcoded services)
- **New System**: Plugin-based service registry with dependency resolution
- **Status**: Planned (Tasks 3.3, 5.1-5.4)

### Global Flag
- **Environment Variable**: `OPENCENTER_ENABLE_ALL_NEW_FEATURES`
- **Default**: `false`
- **Effect**: Enables all new features at once (individual flags override this)

### Debug Logging
- **Environment Variable**: `OPENCENTER_FEATURE_FLAG_DEBUG`
- **Default**: `false`
- **Effect**: Prints feature flag evaluation to stderr for troubleshooting

## Usage

### Viewing Feature Flag Status

```bash
# View current status in table format
openCenter config features

# View status in JSON format
openCenter config features -o json

# View status as environment variable exports
openCenter config features -o env
```

### Enabling Features

```bash
# Enable a single feature
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true
openCenter cluster render

# Enable all new features
export OPENCENTER_ENABLE_ALL_NEW_FEATURES=true
openCenter cluster init my-cluster

# Enable with debug logging
export OPENCENTER_FEATURE_FLAG_DEBUG=true
export OPENCENTER_USE_PIPELINE_GENERATOR=true
openCenter cluster render
```

### Disabling Features

```bash
# Disable a specific feature when all are enabled
export OPENCENTER_ENABLE_ALL_NEW_FEATURES=true
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=false
openCenter cluster render

# Disable all features (use legacy systems)
unset OPENCENTER_ENABLE_ALL_NEW_FEATURES
unset OPENCENTER_USE_NEW_TEMPLATE_ENGINE
unset OPENCENTER_USE_PIPELINE_GENERATOR
```

## Valid Values

To enable a feature flag, use any of these values (case-insensitive):
- `true`
- `1`
- `yes`
- `on`

Any other value or unset means the feature is disabled.

## Migration Timeline

### Phase 1: Current (All Legacy)
- All flags default to `false`
- Legacy systems are active
- New systems available for testing

### Phase 2: Testing
- Flags can be enabled for testing
- Both systems available
- Gradual rollout to production

### Phase 3: Transition
- Flags default to `true`
- New systems become default
- Legacy systems still available for rollback

### Phase 4: Cleanup
- Legacy systems removed
- Flags no longer needed
- New systems are the only option

## Rollback Procedure

If you encounter issues with a new feature:

1. **Immediate Rollback**: Disable the feature flag
   ```bash
   export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=false
   ```

2. **Verify Rollback**: Check that the legacy system is active
   ```bash
   openCenter config features
   ```

3. **Report Issue**: Report the issue with debug logs
   ```bash
   export OPENCENTER_FEATURE_FLAG_DEBUG=true
   openCenter cluster render > output.log 2>&1
   ```

4. **Continue Operations**: Use the legacy system while the issue is resolved

## Best Practices

### Development
- Enable individual features for testing
- Use debug logging to understand which systems are active
- Test with both legacy and new systems to ensure compatibility

### Staging
- Enable all new features to test complete system
- Monitor for issues and performance regressions
- Validate output matches legacy system

### Production
- Enable features gradually (one at a time)
- Monitor closely for issues
- Keep rollback procedure ready
- Document which features are enabled

## Troubleshooting

### Feature Flag Not Taking Effect

1. Check environment variable is set:
   ```bash
   echo $OPENCENTER_USE_NEW_TEMPLATE_ENGINE
   ```

2. Verify feature flag status:
   ```bash
   openCenter config features
   ```

3. Enable debug logging:
   ```bash
   export OPENCENTER_FEATURE_FLAG_DEBUG=true
   openCenter cluster render
   ```

### Unexpected Behavior

1. Disable the feature flag immediately
2. Verify rollback to legacy system
3. Collect debug logs with feature flag debug enabled
4. Report issue with logs and reproduction steps

### Performance Issues

1. Compare performance with legacy system
2. Check if caching is enabled (for template engine)
3. Monitor resource usage
4. Consider disabling feature if performance is worse

## Implementation Details

Feature flags are implemented in `internal/config/feature_flags.go` with:
- Thread-safe singleton pattern
- Caching for performance
- Debug logging for troubleshooting
- Centralized management

For more information, see:
- Design document: `.kiro/specs/configuration-system-refactor/design.md`
- Tasks document: `.kiro/specs/configuration-system-refactor/tasks.md`
- Implementation: `internal/config/feature_flags.go`

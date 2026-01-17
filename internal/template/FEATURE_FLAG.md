# Template Engine Feature Flag

## Overview

The `OPENCENTER_USE_NEW_TEMPLATE_ENGINE` feature flag controls whether the new template engine or the legacy text/template implementation is used for rendering templates throughout the openCenter CLI.

## Purpose

This feature flag enables:
- **Gradual Migration**: Test the new template engine in production without full commitment
- **Risk Mitigation**: Quick rollback capability if issues are discovered
- **Validation**: Verify output identity between legacy and new systems
- **Performance Testing**: Compare performance characteristics in real-world scenarios

## Usage

### Enable New Template Engine

```bash
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true
openCenter cluster setup my-cluster
```

### Disable (Use Legacy System)

```bash
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=false
openCenter cluster setup my-cluster
```

### Check Current Status

```bash
export OPENCENTER_FEATURE_FLAG_DEBUG=true
openCenter cluster setup my-cluster
```

This will print feature flag evaluation to stderr, showing which system is active.

## Valid Values

**Enable the new engine:**
- `true`
- `1`
- `yes`
- `on`

All values are case-insensitive. Any other value or unset means disabled (use legacy system).

## Global Feature Flag

The `OPENCENTER_ENABLE_ALL_NEW_FEATURES` flag can enable all new systems at once:

```bash
export OPENCENTER_ENABLE_ALL_NEW_FEATURES=true
```

**Important:** Individual feature flags take precedence over the global flag. To disable a specific feature when all are enabled:

```bash
export OPENCENTER_ENABLE_ALL_NEW_FEATURES=true
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=false  # Override for this specific feature
```

## Migration Path

### Phase 1: Testing (Current)
- Flag defaults to `false` (legacy system)
- New system available for opt-in testing
- Both systems produce identical output

### Phase 2: Gradual Rollout
1. Enable in development/staging environments
2. Monitor for issues and performance
3. Validate output identity with legacy system
4. Enable in production with monitoring

### Phase 3: Default Transition
- Flag defaults to `true` (new system)
- Legacy system still available for rollback
- Deprecation warnings added

### Phase 4: Legacy Removal
- Legacy system removed from codebase
- Feature flag no longer needed
- New system is the only implementation

## Benefits of New Template Engine

1. **Performance**: Template caching reduces repeated parsing overhead
2. **Error Messages**: Better error reporting with context and suggestions
3. **Validation**: Template validation before rendering
4. **Registry**: Centralized template management and discovery
5. **Testing**: Improved testability with mock templates

## Output Identity Guarantee

The new template engine is designed to produce **byte-for-byte identical output** to the legacy system. This guarantee is validated by comprehensive test suites:

- `TestLegacyCompatibility`: Validates identical output for common patterns
- `TestLegacySystemOutputIdentity`: Tests real-world template patterns
- `TestFeatureFlagOutputIdentity`: Verifies identity when toggling the flag
- `TestMigrationWithRealWorldTemplates`: Tests actual openCenter templates

## Rollback Procedure

If issues are discovered with the new template engine:

1. **Immediate Rollback**: Disable the feature flag
   ```bash
   export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=false
   ```

2. **No Code Changes Required**: The legacy system is still present and functional

3. **No Redeployment Needed**: Environment variable change takes effect immediately

4. **Report Issue**: File a bug report with:
   - Template content that failed
   - Error message from new engine
   - Expected vs actual output
   - Debug logs (`OPENCENTER_FEATURE_FLAG_DEBUG=true`)

## Testing

### Unit Tests

Run template engine tests:
```bash
go test ./internal/template -v
```

### Feature Flag Tests

Test feature flag behavior:
```bash
go test ./internal/template -v -run TestFeatureFlag
```

### Migration Tests

Test migration path and output identity:
```bash
go test ./internal/template -v -run TestMigration
```

### Integration Tests

Test with real cluster configurations:
```bash
# With new engine
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true
openCenter cluster setup test-cluster

# With legacy engine
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=false
openCenter cluster setup test-cluster

# Compare outputs
diff -r gitops-new/ gitops-legacy/
```

## Common Issues

### Issue: Templates fail with new engine but work with legacy

**Solution**: This indicates a compatibility issue. Please:
1. Rollback to legacy system
2. Report the issue with template content and error message
3. The template may use unsupported syntax or edge cases

### Issue: Performance is worse with new engine

**Solution**: The new engine should be faster due to caching. If not:
1. Check if templates are being cached (enable debug logging)
2. Verify template registry is working correctly
3. Report performance metrics for investigation

### Issue: Output differs between engines

**Solution**: This is a critical bug. Please:
1. Rollback to legacy system immediately
2. Report with both outputs and template content
3. This violates the output identity guarantee

## Code Examples

### Check if New Engine is Enabled

```go
import "github.com/rackerlabs/openCenter-cli/internal/template"

if template.UseNewTemplateEngine() {
    // New engine is active
} else {
    // Legacy system is active
}
```

### Render with Explicit Engine Choice

```go
// Always use new engine (ignores feature flag)
engine := template.NewGoTemplateEngine()
result, err := engine.RenderString(ctx, "template.yaml", content, data)

// Use feature flag to choose engine
err := template.RenderTemplateToFile(fsys, "template.yaml", output, data)
```

### Test Both Engines

```go
func TestBothEngines(t *testing.T) {
    // Test with legacy
    t.Setenv("OPENCENTER_USE_NEW_TEMPLATE_ENGINE", "false")
    legacyOutput := renderTemplate(...)
    
    // Test with new
    t.Setenv("OPENCENTER_USE_NEW_TEMPLATE_ENGINE", "true")
    newOutput := renderTemplate(...)
    
    // Verify identity
    assert.Equal(t, legacyOutput, newOutput)
}
```

## Related Documentation

- [Template Engine Migration Guide](../../docs/migration/template-engine.md)
- [Template Engine Quick Reference](../../docs/migration/template-engine-quick-reference.md)
- [Migration Path Validation](../../docs/migration/MIGRATION_PATH_VALIDATION.md)
- [Feature Flag System](../../internal/config/feature_flags.go)

## Support

For questions or issues:
1. Check the migration documentation
2. Enable debug logging for troubleshooting
3. File an issue with debug logs and reproduction steps
4. Use rollback procedure if blocking production

## Timeline

- **2024-Q4**: Feature flag introduced, new engine available for testing
- **2025-Q1**: Gradual rollout to production environments
- **2025-Q2**: New engine becomes default (flag defaults to true)
- **2025-Q3**: Legacy system deprecated with warnings
- **2025-Q4**: Legacy system removed, feature flag retired

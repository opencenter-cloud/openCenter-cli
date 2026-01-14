# Template Engine Migration Quick Reference

This is a quick reference guide for migrating from the legacy template system to the new template engine. For comprehensive details, see [template-engine.md](./template-engine.md).

## Quick Decision Tree

```
Do you need to change existing code?
├─ NO → You're done! Legacy compatibility layer handles it automatically
└─ YES → Are you writing new code or refactoring?
    ├─ New code → Use Path 2 (Helper Functions) or Path 3 (Full API)
    └─ Refactoring → Use Path 2 (Helper Functions) with feature flag
```

## Migration Paths Summary

### Path 1: No Changes (Recommended for Most Code)
**When**: Existing code using legacy compatibility layer  
**Effort**: Zero  
**Risk**: None

```go
// This continues to work unchanged
err := renderTemplate(fsys, "template.yaml.tmpl", outputPath, data)
```

**Feature Flag**: Control which engine is used via `OPENCENTER_USE_NEW_TEMPLATE_ENGINE` environment variable.

### Path 2: Helper Functions (Recommended for New Code)
**When**: Writing new code or refactoring  
**Effort**: Minimal  
**Risk**: Low

```go
engine := template.NewGoTemplateEngine()
err := template.RenderWithEngine(engine, fsys, "template.yaml", outputPath, data)
```

**Benefits**: Caching, better errors, validation support

### Path 3: Full API (For Advanced Use Cases)
**When**: Need maximum control and features  
**Effort**: Moderate  
**Risk**: Low

```go
engine := template.NewGoTemplateEngine()
result, err := engine.RenderString(ctx, "template", content, data)
```

**Benefits**: Context support, pre-validation, all engine features

## Feature Flag Control

The `OPENCENTER_USE_NEW_TEMPLATE_ENGINE` environment variable controls template engine selection:

```bash
# Use new engine (with caching and enhanced features)
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true

# Use legacy engine (default)
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=false
# or unset
unset OPENCENTER_USE_NEW_TEMPLATE_ENGINE
```

**Valid values for enabling**: `true`, `1`, `yes`, `on` (case-insensitive)  
**Default**: Legacy engine (when unset or any other value)

**Quick rollback**: Simply disable the flag - no code changes needed!

### Path 4: Template Registry (For Embedded Templates)
**When**: Working with embedded template system  
**Effort**: Moderate  
**Risk**: Low

```go
registry := template.GetGlobalRegistry()
tmplDef, err := registry.GetTemplate("cluster-base")
// Use template definition with engine
```

**Benefits**: Metadata, dependencies, provider filtering

## Common Patterns

### Pattern 1: Simple Template Rendering
```go
// Old
tmpl, _ := template.ParseFiles("template.yaml")
tmpl.Execute(file, data)

// New
engine := template.NewGoTemplateEngine()
template.RenderWithEngine(engine, os.DirFS("."), "template.yaml", "output.yaml", data)
```

### Pattern 2: Multiple Templates with Caching
```go
// Create engine once, reuse for caching
engine := template.NewGoTemplateEngine()

for _, tmpl := range templates {
    template.RenderWithEngine(engine, fsys, tmpl.Path, tmpl.Output, data)
}
```

### Pattern 3: Template with Validation
```go
engine := template.NewGoTemplateEngine()

// Validate first (optional but recommended)
if err := engine.ValidateTemplate("template.yaml"); err != nil {
    return fmt.Errorf("invalid template: %w", err)
}

// Then render
template.RenderWithEngine(engine, fsys, "template.yaml", "output.yaml", data)
```

### Pattern 4: Feature Flag Migration
```go
func renderTemplate(fsys fs.FS, tmplPath, outputPath string, data interface{}) error {
    if os.Getenv("USE_NEW_ENGINE") == "true" {
        engine := template.NewGoTemplateEngine()
        return template.RenderWithEngine(engine, fsys, tmplPath, outputPath, data)
    }
    return renderTemplateLegacy(fsys, tmplPath, outputPath, data)
}
```

## Testing Checklist

Before deploying migrated code:

- [ ] Output is byte-for-byte identical to legacy system
- [ ] All unit tests pass
- [ ] Integration tests pass
- [ ] Performance is equal or better
- [ ] Error handling works correctly
- [ ] Feature flag is in place (if applicable)
- [ ] Rollback plan is documented

## Common Issues and Solutions

### Issue: Template syntax conflicts
**Solution**: Use the same escaping as legacy system (handled automatically by compatibility layer)

### Issue: Missing Sprig functions
**Solution**: Already included in new engine, no action needed

### Issue: Different error messages
**Solution**: Expected and beneficial - update error handling tests if needed

### Issue: Performance differences
**Solution**: New system is faster due to caching - update benchmarks

## Performance Tips

1. **Reuse engine instances** for caching benefits
2. **Validate templates at startup**, not at render time
3. **Use context** for long-running operations
4. **Clear cache** periodically if memory is a concern

## Getting Help

- **Documentation**: `internal/template/README.md`
- **Test Examples**: `internal/template/migration_test.go`
- **Full Guide**: `docs/migration/template-engine.md`

## Key Differences

| Aspect | Legacy System | New System |
|--------|--------------|------------|
| Caching | No | Yes (automatic) |
| Validation | Runtime only | Pre-render + runtime |
| Error Messages | Basic | Detailed with context |
| Performance | Baseline | 1.5-2x faster |
| Extensibility | Limited | High (interfaces) |
| Testing | Manual | Comprehensive suite |

## Migration Timeline

- **Phase 1** (Current): Coexistence - both systems work
- **Phase 2** (In Progress): Gradual migration with feature flags
- **Phase 3** (Future): Complete migration, legacy deprecation

## Quick Commands

```bash
# Run migration tests
go test -v ./internal/template -run TestMigration

# Run all template tests
go test -v ./internal/template

# Run performance comparison
go test -v ./internal/template -run TestMigrationPerformanceComparison

# Run with short mode (skip performance tests)
go test -v -short ./internal/template
```

## Example Migration

**Before** (legacy):
```go
func generateManifest(config Config) error {
    tmpl, err := template.ParseFiles("manifest.yaml.tmpl")
    if err != nil {
        return err
    }
    
    f, err := os.Create("output/manifest.yaml")
    if err != nil {
        return err
    }
    defer f.Close()
    
    return tmpl.Execute(f, config)
}
```

**After** (new engine):
```go
func generateManifest(config Config) error {
    engine := template.NewGoTemplateEngine()
    fsys := os.DirFS(".")
    return template.RenderWithEngine(
        engine,
        fsys,
        "manifest.yaml.tmpl",
        "output/manifest.yaml",
        config,
    )
}
```

**Benefits of migration**:
- Automatic caching (faster on repeated renders)
- Better error messages with line numbers
- Validation support
- Consistent with new architecture
- Easier to test

## Next Steps

1. Review full migration guide: `docs/migration/template-engine.md`
2. Check test examples: `internal/template/migration_test.go`
3. Start with Path 1 (no changes) for existing code
4. Use Path 2 (helper functions) for new code
5. Implement feature flags for critical paths
6. Monitor and validate output identity
7. Gradually expand migration scope

## Support

For questions or issues:
- Review test examples in `internal/template/migration_test.go`
- Consult full migration guide in `docs/migration/template-engine.md`
- Check existing migrations in codebase
- Reach out to team members who have completed migrations

# Template Engine Migration Guide

## Overview

This guide provides a comprehensive migration path from the legacy template system to the new template engine introduced in the configuration system refactor. The new template engine provides better abstraction, caching, validation, and extensibility while maintaining 100% backward compatibility with existing code.

## Why Migrate?

The new template engine offers several advantages over the legacy system:

- **Better Abstraction**: Clean interface separating template concerns from business logic
- **Performance**: Built-in caching reduces template parsing overhead
- **Validation**: Pre-rendering validation catches errors early
- **Extensibility**: Easy to add new template formats (Helm, Jinja2, etc.)
- **Error Handling**: Detailed error messages with line numbers and context
- **Testing**: Comprehensive test coverage with property-based tests
- **Registry System**: Centralized template management with metadata

## Migration Strategy

The migration follows a phased approach to minimize risk and allow gradual adoption:

### Phase 1: Coexistence (Current)
- New template engine is available alongside legacy system
- New code can use new engine
- Existing code continues to work unchanged
- Legacy compatibility layer ensures identical output

### Phase 2: Gradual Migration (Recommended)
- Migrate code module by module
- Use feature flags to control rollout
- Validate output identity at each step
- Maintain backward compatibility

### Phase 3: Complete Migration (Future)
- All code uses new template engine
- Legacy compatibility layer can be removed
- Deprecate old template utilities

## Migration Paths

### Path 1: No Changes Required (Recommended for Most Code)

If your code uses the legacy compatibility layer, **no changes are required**. The compatibility layer automatically uses the new template engine under the hood while maintaining identical behavior.

**Example**: Code in `internal/gitops/copy.go` that uses `renderTemplate` continues to work without modification.

```go
// This code continues to work unchanged
err := renderTemplate(fsys, "template.yaml.tmpl", outputPath, data)
```

### Path 2: Migrate to Template Engine with Helper Functions

For new code or when refactoring, use the helper functions that wrap the template engine:

**Before** (legacy):
```go
import "text/template"

func renderMyTemplate(data interface{}) error {
    tmpl, err := template.ParseFiles("template.yaml")
    if err != nil {
        return err
    }
    
    f, err := os.Create("output.yaml")
    if err != nil {
        return err
    }
    defer f.Close()
    
    return tmpl.Execute(f, data)
}
```

**After** (with helper):
```go
import "github.com/rackerlabs/openCenter-cli/internal/template"

func renderMyTemplate(data interface{}) error {
    engine := template.NewGoTemplateEngine()
    fsys := os.DirFS(".")
    return template.RenderWithEngine(engine, fsys, "template.yaml", "output.yaml", data)
}
```

**Benefits**:
- Automatic caching
- Better error messages
- Validation support
- Consistent with new architecture

### Path 3: Full Template Engine API

For maximum control and features, use the template engine API directly:

**Before** (legacy):
```go
import (
    "text/template"
    "github.com/Masterminds/sprig/v3"
)

func renderMyTemplate(data interface{}) ([]byte, error) {
    tmpl, err := template.New("mytemplate").Funcs(sprig.TxtFuncMap()).Parse(templateContent)
    if err != nil {
        return nil, err
    }
    
    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, data); err != nil {
        return nil, err
    }
    
    return buf.Bytes(), nil
}
```

**After** (full API):
```go
import (
    "context"
    "github.com/rackerlabs/openCenter-cli/internal/template"
)

func renderMyTemplate(data interface{}) ([]byte, error) {
    engine := template.NewGoTemplateEngine()
    
    // Optional: Validate before rendering
    if err := engine.ValidateString("mytemplate", templateContent); err != nil {
        return nil, err
    }
    
    return engine.RenderString(context.Background(), "mytemplate", templateContent, data)
}
```

**Benefits**:
- Pre-rendering validation
- Context support for cancellation
- Access to all engine features
- Better testability

### Path 4: Using Template Registry

For templates that are part of the embedded template system:

**Before** (direct file access):
```go
import "embed"

//go:embed templates/*.yaml
var templatesFS embed.FS

func renderClusterTemplate(config Config) error {
    return renderTemplate(templatesFS, "templates/cluster.yaml.tmpl", "output.yaml", config)
}
```

**After** (using registry):
```go
import "github.com/rackerlabs/openCenter-cli/internal/template"

func renderClusterTemplate(config Config) error {
    registry := template.GetGlobalRegistry()
    
    // Get template definition with metadata
    tmplDef, err := registry.GetTemplate("cluster-base")
    if err != nil {
        return err
    }
    
    // Render using the engine
    engine := template.NewGoTemplateEngine()
    return template.RenderWithEngine(engine, templatesFS, tmplDef.Path, "output.yaml", config)
}
```

**Benefits**:
- Template metadata and dependencies
- Provider and service filtering
- Centralized template management
- Better discoverability

## Step-by-Step Migration Examples

### Example 1: Migrating GitOps Template Rendering

**Current Code** (`internal/gitops/copy.go`):
```go
func copyAndRenderTemplates(srcFS fs.FS, dstDir string, config Config) error {
    return fs.WalkDir(srcFS, ".", func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }
        
        if d.IsDir() {
            return nil
        }
        
        dstPath := filepath.Join(dstDir, path)
        
        if strings.HasSuffix(path, ".tmpl") {
            // Render template
            return renderTemplate(srcFS, path, strings.TrimSuffix(dstPath, ".tmpl"), config)
        }
        
        // Copy non-template files
        return copyFile(srcFS, path, dstPath)
    })
}
```

**Migrated Code** (using template engine):
```go
func copyAndRenderTemplates(srcFS fs.FS, dstDir string, config Config) error {
    engine := template.NewGoTemplateEngine()
    
    return fs.WalkDir(srcFS, ".", func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }
        
        if d.IsDir() {
            return nil
        }
        
        dstPath := filepath.Join(dstDir, path)
        
        if strings.HasSuffix(path, ".tmpl") {
            // Render template using new engine
            return template.RenderWithEngine(engine, srcFS, path, strings.TrimSuffix(dstPath, ".tmpl"), config)
        }
        
        // Copy non-template files
        return copyFile(srcFS, path, dstPath)
    })
}
```

**Migration Steps**:
1. Create template engine instance (reuse across multiple renders for caching)
2. Replace `renderTemplate` calls with `template.RenderWithEngine`
3. Test output identity with legacy system
4. Deploy with feature flag (optional)
5. Monitor for issues
6. Remove feature flag after validation

### Example 2: Migrating Service Template Generation

**Current Code**:
```go
func generateServiceManifest(service string, config Config) error {
    tmplPath := fmt.Sprintf("services/%s.yaml.tmpl", service)
    outputPath := fmt.Sprintf("output/services/%s.yaml", service)
    
    tmpl, err := template.ParseFiles(tmplPath)
    if err != nil {
        return err
    }
    
    f, err := os.Create(outputPath)
    if err != nil {
        return err
    }
    defer f.Close()
    
    return tmpl.Execute(f, config)
}
```

**Migrated Code** (with validation):
```go
func generateServiceManifest(service string, config Config) error {
    engine := template.NewGoTemplateEngine()
    tmplPath := fmt.Sprintf("services/%s.yaml.tmpl", service)
    outputPath := fmt.Sprintf("output/services/%s.yaml", service)
    
    // Validate template before rendering
    if err := engine.ValidateFile(tmplPath); err != nil {
        return fmt.Errorf("template validation failed: %w", err)
    }
    
    // Render template
    fsys := os.DirFS(".")
    return template.RenderWithEngine(engine, fsys, tmplPath, outputPath, config)
}
```

**Migration Steps**:
1. Add template validation step
2. Use `RenderWithEngine` helper
3. Add error context
4. Test with existing service templates
5. Verify output matches legacy system

### Example 3: Migrating with Feature Flag

For critical code paths, use the built-in feature flag to control the migration:

```go
import (
    "github.com/rackerlabs/openCenter-cli/internal/template"
)

func renderTemplate(fsys fs.FS, tmplPath, outputPath string, data interface{}) error {
    // The RenderTemplateToFile function automatically respects the feature flag
    // No code changes needed - just set the environment variable
    return template.RenderTemplateToFile(fsys, tmplPath, outputPath, data)
}
```

**Feature Flag Usage**:

The `OPENCENTER_USE_NEW_TEMPLATE_ENGINE` environment variable controls which template engine is used:

- **Unset or `false`** (default): Uses legacy text/template implementation
- **`true`, `1`, `yes`, `on`**: Uses new GoTemplateEngine with caching and enhanced features

**Enabling the Feature Flag**:

```bash
# Enable for current session
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true

# Enable for single command
OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true opencenter cluster init ...

# Disable (use legacy system)
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=false
# or simply unset it
unset OPENCENTER_USE_NEW_TEMPLATE_ENGINE
```

**Feature Flag Strategy**:
1. **Phase 1**: Deploy with flag disabled (default to legacy) - no changes needed
2. **Phase 2**: Enable for development/staging environments
   ```bash
   export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true
   ```
3. **Phase 3**: Monitor for issues and performance improvements
4. **Phase 4**: Gradually enable for production workloads
5. **Phase 5**: After validation, make new engine the default (future release)
6. **Phase 6**: Remove flag and legacy code after deprecation period

**Rollback Procedure**:

If issues are detected with the new engine:

```bash
# Immediate rollback - disable the feature flag
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=false

# Or unset it to use default (legacy)
unset OPENCENTER_USE_NEW_TEMPLATE_ENGINE
```

No code changes or redeployment needed - the system automatically falls back to the legacy implementation.

**Checking Current Engine**:

```go
import "github.com/rackerlabs/openCenter-cli/internal/template"

if template.UseNewTemplateEngine() {
    fmt.Println("Using new template engine")
} else {
    fmt.Println("Using legacy template system")
}
```

## Testing Migration

### Verify Output Identity

The most critical test is verifying that the new system produces **identical** output to the legacy system:

```go
func TestMigrationOutputIdentity(t *testing.T) {
    fsys := fstest.MapFS{
        "template.yaml.tmpl": &fstest.MapFile{
            Data: []byte("cluster: {{.ClusterName}}\nregion: {{.Region}}"),
        },
    }
    
    data := map[string]string{
        "ClusterName": "test-cluster",
        "Region":      "us-east-1",
    }
    
    tmpDir := t.TempDir()
    
    // Render with legacy system
    legacyOutput := filepath.Join(tmpDir, "legacy.yaml")
    err := renderTemplateLegacy(fsys, "template.yaml.tmpl", legacyOutput, data)
    require.NoError(t, err)
    
    // Render with new system
    newOutput := filepath.Join(tmpDir, "new.yaml")
    engine := template.NewGoTemplateEngine()
    err = template.RenderWithEngine(engine, fsys, "template.yaml.tmpl", newOutput, data)
    require.NoError(t, err)
    
    // Verify byte-for-byte identity
    legacyContent, _ := os.ReadFile(legacyOutput)
    newContent, _ := os.ReadFile(newOutput)
    assert.Equal(t, string(legacyContent), string(newContent))
}
```

### Integration Testing

Test complete workflows with the new system:

```go
func TestGitOpsGenerationWithNewEngine(t *testing.T) {
    config := loadTestConfig(t)
    tmpDir := t.TempDir()
    
    // Generate GitOps repository using new engine
    err := generateGitOpsRepo(config, tmpDir)
    require.NoError(t, err)
    
    // Verify repository structure
    assertDirectoryExists(t, filepath.Join(tmpDir, "infrastructure"))
    assertDirectoryExists(t, filepath.Join(tmpDir, "applications"))
    
    // Verify generated manifests are valid YAML
    manifests, err := filepath.Glob(filepath.Join(tmpDir, "**/*.yaml"))
    require.NoError(t, err)
    
    for _, manifest := range manifests {
        assertValidYAML(t, manifest)
    }
}
```

## Common Migration Issues

### Issue 1: Template Syntax Conflicts

**Problem**: Templates with special syntax (e.g., Helm templates in Makefiles) may conflict with Go template parsing.

**Solution**: The legacy compatibility layer handles this automatically. For new code, use the same escaping:

```go
// For Makefile.tpl with Helm syntax
content := strings.ReplaceAll(content, `--template="{{.Version}}"`, `--template="{{"{{"}}.Version{{"}}"}}"`)
```

### Issue 2: Missing Sprig Functions

**Problem**: Legacy code relies on Sprig functions that aren't registered.

**Solution**: The new engine automatically includes Sprig functions. No changes needed.

### Issue 3: Different Error Messages

**Problem**: Error messages differ between legacy and new systems.

**Solution**: This is expected and beneficial. The new system provides better error context. Update error handling tests if needed.

### Issue 4: Performance Differences

**Problem**: Performance characteristics differ due to caching.

**Solution**: The new system is generally faster due to caching. Update performance benchmarks to reflect improvements.

## Rollback Strategy

If issues arise during migration:

### Immediate Rollback
1. Disable feature flag (if using)
2. Revert code changes
3. Deploy previous version
4. Investigate issues

### Gradual Rollback
1. Identify problematic code paths
2. Revert specific modules
3. Keep working migrations
4. Fix issues incrementally

## Validation Checklist

Before completing migration:

- [ ] All templates render successfully with new engine
- [ ] Output is byte-for-byte identical to legacy system
- [ ] All tests pass (unit, integration, property-based)
- [ ] Performance is equal or better than legacy
- [ ] Error handling works correctly
- [ ] Documentation is updated
- [ ] Team is trained on new system
- [ ] Monitoring is in place
- [ ] Rollback plan is documented

## Performance Considerations

### Caching Benefits

The new engine caches parsed templates, providing significant performance improvements:

```go
// First render: parses template
engine := template.NewGoTemplateEngine()
engine.RenderString(ctx, "template", content, data) // ~10ms

// Subsequent renders: uses cache
engine.RenderString(ctx, "template", content, data) // ~1ms
```

### Memory Usage

Caching increases memory usage slightly. For large-scale operations:

```go
// Clear cache periodically if needed
engine.ClearCache()

// Or disable caching for one-off renders
engine.SetCacheEnabled(false)
```

## Best Practices

### 1. Reuse Engine Instances

Create one engine instance and reuse it for multiple renders:

```go
// Good: Reuse engine for caching benefits
engine := template.NewGoTemplateEngine()
for _, tmpl := range templates {
    engine.RenderString(ctx, tmpl.Name, tmpl.Content, data)
}

// Bad: Create new engine each time (no caching)
for _, tmpl := range templates {
    engine := template.NewGoTemplateEngine()
    engine.RenderString(ctx, tmpl.Name, tmpl.Content, data)
}
```

### 2. Validate Templates Early

Validate templates during initialization, not at render time:

```go
func init() {
    engine := template.NewGoTemplateEngine()
    for _, tmpl := range embeddedTemplates {
        if err := engine.ValidateString(tmpl.Name, tmpl.Content); err != nil {
            log.Fatalf("Invalid template %s: %v", tmpl.Name, err)
        }
    }
}
```

### 3. Use Context for Cancellation

Pass context for long-running operations:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := engine.RenderString(ctx, "template", content, data)
```

### 4. Handle Errors Gracefully

The new engine provides detailed error context:

```go
result, err := engine.RenderString(ctx, "template", content, data)
if err != nil {
    // Error includes line numbers and context
    log.Errorf("Template rendering failed: %v", err)
    // Implement fallback or recovery logic
}
```

## Support and Resources

### Documentation
- Template Engine API: `internal/template/README.md`
- Embedded Templates: `internal/template/EMBEDDED_TEMPLATES.md`
- Implementation Details: `internal/template/IMPLEMENTATION_SUMMARY.md`

### Tests
- Migration Tests: `internal/template/migration_test.go`
- Integration Tests: `internal/template/embedded_integration_test.go`
- Property Tests: `internal/template/registry_property_test.go`

### Getting Help
- Review test examples in `migration_test.go`
- Check existing migrations in codebase
- Consult with team members who have completed migrations
- File issues for unexpected behavior

## Timeline and Milestones

### Completed
- ✅ New template engine implementation
- ✅ Legacy compatibility layer
- ✅ Output identity validation
- ✅ Comprehensive test coverage
- ✅ Migration documentation

### In Progress
- 🔄 Feature flag implementation
- 🔄 Gradual code migration
- 🔄 Performance monitoring

### Future
- ⏳ Complete migration of all code
- ⏳ Legacy system deprecation
- ⏳ Legacy code removal

## Conclusion

The new template engine provides significant improvements while maintaining full backward compatibility. The migration can be done gradually with minimal risk. Most code requires no changes, and the new system provides better performance, validation, and error handling.

For questions or issues during migration, refer to the test examples in `internal/template/migration_test.go` or consult the team.

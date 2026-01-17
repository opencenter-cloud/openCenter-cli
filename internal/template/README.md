# Template Package

The `internal/template` package provides a comprehensive template rendering system for the openCenter CLI, supporting both legacy compatibility and a new enhanced template engine with advanced features.

## Overview

This package manages all template rendering operations in openCenter, including:
- GitOps repository generation
- Kubernetes manifest templating
- Configuration file generation
- Infrastructure-as-code templates

## Architecture

### Core Components

1. **Template Engine** (`engine.go`)
   - Main template rendering engine with Sprig function support
   - Context-aware rendering with cancellation support
   - Template validation and error handling
   - Performance optimizations through caching

2. **Template Registry** (`registry.go`, `global_registry.go`)
   - Centralized template management
   - Template discovery and registration
   - Metadata tracking (version, author, tags)
   - Embedded template support

3. **Template Cache** (`cache.go`)
   - LRU-based template caching
   - Configurable cache size and TTL
   - Thread-safe concurrent access
   - Cache statistics and monitoring

4. **Legacy Compatibility** (`legacy.go`)
   - Backward compatibility layer for existing code
   - Feature flag support for gradual migration
   - Output identity guarantee with legacy system

5. **Template Composition** (`composition.go`)
   - Template inheritance and composition
   - Partial template support
   - Template dependencies and resolution

## Feature Flag System

The package uses feature flags to enable gradual migration from the legacy template system to the new enhanced engine.

### Environment Variables

- `OPENCENTER_USE_NEW_TEMPLATE_ENGINE`: Enable new template engine (default: false)
- `OPENCENTER_ENABLE_ALL_NEW_FEATURES`: Enable all new features globally
- `OPENCENTER_FEATURE_FLAG_DEBUG`: Enable debug logging for feature flags

See [FEATURE_FLAG.md](./FEATURE_FLAG.md) for detailed documentation.

## Usage

### Basic Template Rendering

```go
import "github.com/rackerlabs/openCenter-cli/internal/template"

// Using legacy compatibility layer (respects feature flag)
err := template.RenderTemplateToFile(fsys, "template.yaml", outputPath, data)

// Using new engine directly
engine := template.NewGoTemplateEngine()
result, err := engine.RenderString(ctx, "template", content, data)
```

### Template Registry

```go
// Get global registry
registry, err := template.GetGlobalRegistry()

// Register a template
err = registry.Register("my-template", content, metadata)

// Render from registry
result, err := registry.Render(ctx, "my-template", data)

// List available templates
templates := registry.ListTemplates()
```

### Template Caching

```go
// Create engine with custom cache
cache := template.NewTemplateCache(100, 1*time.Hour)
engine := template.NewGoTemplateEngineWithCache(cache)

// Cache is automatically used for repeated renders
result1, _ := engine.RenderString(ctx, "template", content, data)
result2, _ := engine.RenderString(ctx, "template", content, data) // Uses cache

// Get cache statistics
stats := cache.Stats()
fmt.Printf("Hit rate: %.2f%%\n", stats.HitRate())
```

### Template Composition

```go
// Define base template
baseTemplate := `
{{define "base"}}
<html>
  <head>{{template "head" .}}</head>
  <body>{{template "body" .}}</body>
</html>
{{end}}
`

// Define child template
childTemplate := `
{{define "head"}}<title>{{.Title}}</title>{{end}}
{{define "body"}}<h1>{{.Content}}</h1>{{end}}
`

// Compose and render
composer := template.NewComposer()
composer.AddTemplate("base", baseTemplate)
composer.AddTemplate("child", childTemplate)
result, err := composer.Render(ctx, "base", data)
```

## Migration Path

The package supports gradual migration from legacy to new template engine:

### Path 1: No Changes Required
Existing code continues to work unchanged through the legacy compatibility layer.

```go
// This code works with both legacy and new engines
err := template.RenderTemplateToFile(fsys, "template.yaml", output, data)
```

### Path 2: Use Helper Functions
Migrate to helper functions that provide access to new features while maintaining compatibility.

```go
engine := template.NewGoTemplateEngine()
err := template.RenderWithEngine(engine, fsys, "template.yaml", output, data)
```

### Path 3: Full Template Engine API
Use the complete template engine API for maximum control and features.

```go
engine := template.NewGoTemplateEngine()
result, err := engine.RenderString(ctx, "template", content, data)
```

### Path 4: Template Registry
Use the template registry for centralized template management.

```go
registry, _ := template.GetGlobalRegistry()
result, err := registry.Render(ctx, "template-name", data)
```

## Testing

### Unit Tests

```bash
# Run all template tests
go test ./internal/template -v

# Run specific test suites
go test ./internal/template -v -run TestEngine
go test ./internal/template -v -run TestRegistry
go test ./internal/template -v -run TestCache
go test ./internal/template -v -run TestMigration
```

### Feature Flag Tests

```bash
# Test feature flag behavior
go test ./internal/template -v -run TestFeatureFlag

# Test migration path
go test ./internal/template -v -run TestMigrationPath
```

### Performance Tests

```bash
# Run cache performance tests
go test ./internal/template -v -run TestCachePerformance

# Run benchmarks
go test ./internal/template -bench=. -benchmem
```

### Property-Based Tests

```bash
# Run property-based tests
go test ./internal/template -v -run TestRegistryProperty
```

## Performance Characteristics

### Template Caching

The new template engine includes an LRU cache that significantly improves performance for repeated template renders:

- **First render**: ~1-2ms (parse + execute)
- **Cached render**: ~0.1-0.2ms (execute only)
- **Cache hit rate**: Typically >90% in production workloads

### Memory Usage

- **Engine**: ~1KB per instance
- **Cache**: ~10KB per cached template
- **Registry**: ~5KB + template storage

### Concurrency

All components are thread-safe and support concurrent access:
- Engine: Multiple goroutines can render simultaneously
- Cache: Lock-free reads, synchronized writes
- Registry: Read-write lock for optimal concurrency

## Error Handling

The package provides detailed error messages with context:

```go
result, err := engine.RenderString(ctx, "template", content, data)
if err != nil {
    // Error includes:
    // - Template name
    // - Line number (if parse error)
    // - Context around error
    // - Suggestions for common issues
    fmt.Printf("Template error: %v\n", err)
}
```

## Best Practices

### 1. Use Context for Cancellation

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := engine.RenderString(ctx, "template", content, data)
```

### 2. Reuse Engine Instances

```go
// Good: Reuse engine to benefit from caching
engine := template.NewGoTemplateEngine()
for _, tmpl := range templates {
    result, _ := engine.RenderString(ctx, tmpl.Name, tmpl.Content, data)
}

// Bad: Creating new engine for each render loses cache benefits
for _, tmpl := range templates {
    engine := template.NewGoTemplateEngine()
    result, _ := engine.RenderString(ctx, tmpl.Name, tmpl.Content, data)
}
```

### 3. Use Registry for Shared Templates

```go
// Register templates once at startup
registry, _ := template.GetGlobalRegistry()
registry.Register("header", headerTemplate, metadata)
registry.Register("footer", footerTemplate, metadata)

// Render from registry throughout application
result, _ := registry.Render(ctx, "header", data)
```

### 4. Validate Templates Early

```go
// Validate template syntax before rendering
engine := template.NewGoTemplateEngine()
if err := engine.Validate("template", content); err != nil {
    return fmt.Errorf("invalid template: %w", err)
}
```

### 5. Monitor Cache Performance

```go
cache := engine.GetCache()
stats := cache.Stats()

if stats.HitRate() < 0.5 {
    // Consider increasing cache size
    log.Warnf("Low cache hit rate: %.2f%%", stats.HitRate()*100)
}
```

## Troubleshooting

### Issue: Templates render slowly

**Diagnosis**: Check cache hit rate
```go
stats := cache.Stats()
fmt.Printf("Hit rate: %.2f%%\n", stats.HitRate())
```

**Solution**: 
- Increase cache size if hit rate is low
- Ensure engine instances are reused
- Check for template variations that prevent caching

### Issue: Template parse errors

**Diagnosis**: Enable debug logging
```bash
export OPENCENTER_FEATURE_FLAG_DEBUG=true
```

**Solution**:
- Check template syntax
- Verify Sprig function names
- Ensure data structure matches template expectations

### Issue: Output differs from legacy system

**Diagnosis**: Run migration tests
```bash
go test ./internal/template -v -run TestLegacySystemOutputIdentity
```

**Solution**:
- Report issue with template content and both outputs
- Rollback to legacy system using feature flag
- This is a critical bug that violates output identity guarantee

## Related Documentation

- [Feature Flag Documentation](./FEATURE_FLAG.md)
- [Template Engine Migration Guide](../../docs/migration/template-engine.md)
- [Template Engine Quick Reference](../../docs/migration/template-engine-quick-reference.md)
- [Migration Path Validation](../../docs/migration/MIGRATION_PATH_VALIDATION.md)

## API Reference

### Core Types

```go
// TemplateEngine is the main interface for template rendering
type TemplateEngine interface {
    RenderString(ctx context.Context, name, content string, data interface{}) ([]byte, error)
    Render(ctx context.Context, name string, data interface{}) ([]byte, error)
    Validate(name, content string) error
}

// TemplateRegistry manages template registration and discovery
type TemplateRegistry interface {
    Register(name, content string, metadata TemplateMetadata) error
    Render(ctx context.Context, name string, data interface{}) ([]byte, error)
    Get(name string) (string, TemplateMetadata, error)
    ListTemplates() []string
}

// TemplateCache provides template caching with LRU eviction
type TemplateCache interface {
    Get(key string) (*template.Template, bool)
    Set(key string, tmpl *template.Template)
    Stats() CacheStats
    Clear()
}
```

### Key Functions

```go
// NewGoTemplateEngine creates a new template engine with default cache
func NewGoTemplateEngine() *GoTemplateEngine

// NewGoTemplateEngineWithCache creates an engine with custom cache
func NewGoTemplateEngineWithCache(cache *TemplateCache) *GoTemplateEngine

// GetGlobalRegistry returns the global template registry
func GetGlobalRegistry() (*TemplateRegistry, error)

// RenderTemplateToFile renders a template to a file (legacy compatibility)
func RenderTemplateToFile(fsys fs.FS, path, dst string, data interface{}) error

// RenderWithEngine renders using a specific engine instance
func RenderWithEngine(engine *GoTemplateEngine, fsys fs.FS, path, dst string, data interface{}) error

// UseNewTemplateEngine checks if new engine is enabled via feature flag
func UseNewTemplateEngine() bool
```

## Contributing

When contributing to this package:

1. **Maintain Output Identity**: New features must produce identical output to legacy system
2. **Add Tests**: Include unit tests, integration tests, and migration tests
3. **Update Documentation**: Keep README and FEATURE_FLAG.md current
4. **Performance**: Run benchmarks to ensure no performance regressions
5. **Backward Compatibility**: Ensure legacy compatibility layer continues to work

## Version History

- **v1.0.0**: Initial release with legacy compatibility layer
- **v1.1.0**: Added template registry and caching
- **v1.2.0**: Added template composition support
- **v1.3.0**: Added feature flag system for gradual migration
- **v2.0.0** (planned): Remove legacy system, new engine becomes default

## License

Copyright 2024 Rackspace Technology

Licensed under the Apache License, Version 2.0. See LICENSE file for details.

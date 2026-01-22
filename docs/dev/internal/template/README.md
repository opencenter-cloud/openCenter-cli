# Template Engine


## Table of Contents

- [Overview](#overview)
- [Core Components](#core-components)
- [Usage Examples](#usage-examples)
- [Design Decisions](#design-decisions)
- [Performance Considerations](#performance-considerations)
- [Testing](#testing)
- [Future Enhancements](#future-enhancements)
- [Migration from Legacy System](#migration-from-legacy-system)
This package provides a clean, extensible template engine abstraction for rendering Go templates with caching, validation, and comprehensive error reporting.

## Overview

The template engine is designed with the following principles:

- **Clean Interface**: Simple, focused interface for template operations
- **Extensibility**: Easy to add new template formats and rendering logic
- **Performance**: Built-in caching with TTL and size limits
- **Type Safety**: Compile-time type checking where possible
- **Error Reporting**: Detailed error messages with context and suggestions
- **Concurrency**: Thread-safe operations for concurrent rendering

## Core Components

### TemplateEngine Interface

The main interface for template operations:

```go
type TemplateEngine interface {
    // Render renders a template with the given data
    Render(ctx context.Context, templatePath string, data interface{}) ([]byte, error)
    
    // ValidateTemplate validates template syntax before execution
    ValidateTemplate(templatePath string) error
    
    // RegisterFunction registers a custom function for use in templates
    RegisterFunction(name string, fn interface{})
    
    // SetCacheEnabled enables or disables template caching
    SetCacheEnabled(enabled bool)
    
    // ClearCache clears all cached templates
    ClearCache()
}
```

### GoTemplateEngine

The default implementation using Go's `text/template` package:

```go
engine := NewGoTemplateEngine()

// Register custom functions
engine.RegisterFunction("upper", strings.ToUpper)

// Render a template
result, err := engine.Render(ctx, "template.tmpl", data)
```

### TemplateCache

Thread-safe caching with TTL and size limits:

```go
// Create cache with 5-minute TTL and max 100 entries
cache := NewInMemoryTemplateCache(5*time.Minute, 100)

// Cache operations
cache.Set("key", template)
tmpl, ok := cache.Get("key")
cache.Delete("key")
cache.Clear()
```

### RenderContext

Comprehensive context for template rendering:

```go
ctx := NewRenderContext(data).
    WithMetadata("version", "1.0").
    WithFunction("custom", customFunc).
    WithStrictMode(true).
    WithValidation(true)

// Or use the builder
ctx, err := NewContextBuilder().
    WithData(data).
    WithMetadata("key", "value").
    WithStrictMode(true).
    Build()
```

## Usage Examples

### Named Template Execution

When multiple templates are loaded together, you can execute them by name:

```go
engine := NewGoTemplateEngine()

// Load multiple templates from embedded filesystem
if err := engine.LoadFromFS(templatesFS, "templates/*.tmpl"); err != nil {
    log.Fatal(err)
}

// Execute a specific template by name
data := map[string]interface{}{"Count": 3}
result, err := engine.ExecuteTemplate("inventory.tmpl", data)
if err != nil {
    log.Fatal(err)
}

// Or write directly to a file
file, _ := os.Create("output.txt")
defer file.Close()
err = engine.ExecuteTemplateToWriter("inventory.tmpl", data, file)
```

### Custom Function Registration

Register custom functions for use in templates:

```go
engine := NewGoTemplateEngine()

// Register a single function
engine.RegisterFunction("hcl", func(v interface{}) string {
    // Custom HCL rendering logic
    return fmt.Sprintf("hcl(%v)", v)
})

// Register multiple functions at once
funcs := template.FuncMap{
    "sortedKeys": func(m map[string]interface{}) []string {
        keys := make([]string, 0, len(m))
        for k := range m {
            keys = append(keys, k)
        }
        sort.Strings(keys)
        return keys
    },
}
engine.RegisterFunctions(funcs)

// Use in templates
template := "{{hcl .Value}}"
result, _ := engine.RenderString(ctx, "test", template, data)
```

### Loading Templates from Files

Load individual template files or collections:

```go
engine := NewGoTemplateEngine()

// Load a single template file
if err := engine.LoadFromFile("templates/config.tmpl"); err != nil {
    log.Fatal(err)
}

// Load multiple files
files := []string{
    "templates/ansible.cfg.tmpl",
    "templates/inventory.tmpl",
}
for _, file := range files {
    if err := engine.LoadFromFile(file); err != nil {
        log.Fatal(err)
    }
}

// Execute by file path
result, err := engine.ExecuteTemplate("templates/config.tmpl", data)
```

### Accessing Loaded Templates

Get direct access to parsed templates:

```go
engine := NewGoTemplateEngine()
engine.LoadFromFS(templatesFS, "templates/*.tmpl")

// Get a specific template
tmpl, err := engine.GetTemplate("config.tmpl")
if err != nil {
    log.Fatal(err)
}

// Use the template directly
var buf bytes.Buffer
tmpl.Execute(&buf, data)
```

### Basic Template Rendering

```go
engine := NewGoTemplateEngine()

data := map[string]interface{}{
    "Name": "OpenCenter",
    "Version": "1.0",
}

result, err := engine.Render(context.Background(), "config.tmpl", data)
if err != nil {
    log.Fatal(err)
}

fmt.Println(string(result))
```

### Custom Functions

```go
engine := NewGoTemplateEngine()

// Register custom functions
engine.RegisterFunction("upper", strings.ToUpper)
engine.RegisterFunction("lower", strings.ToLower)
engine.RegisterFunction("join", strings.Join)

// Use in templates: {{ upper .Name }}
```

### Template Validation

```go
engine := NewGoTemplateEngine()

// Validate template syntax
if err := engine.ValidateTemplate("config.tmpl"); err != nil {
    log.Printf("Template validation failed: %v", err)
}
```

### Caching Control

```go
engine := NewGoTemplateEngine()

// Disable caching for development
engine.SetCacheEnabled(false)

// Enable caching for production
engine.SetCacheEnabled(true)

// Clear cache after template updates
engine.ClearCache()
```

### Context Cancellation

```go
engine := NewGoTemplateEngine()

// Create context with timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

// Render with timeout
result, err := engine.Render(ctx, "template.tmpl", data)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        log.Println("Template rendering timed out")
    }
}
```

### Advanced Context Usage

```go
// Create render context with all options
ctx := NewRenderContext(data).
    WithMetadata("cluster", "prod-01").
    WithMetadata("region", "us-east-1").
    WithFunction("formatDate", formatDateFunc).
    WithStrictMode(true).
    WithValidation(true)

// Convert to template data
templateData := ctx.ToTemplateData()

// Render with context
result, err := engine.Render(context.Background(), "template.tmpl", templateData)
```

## Design Decisions

### Why a Clean Interface?

The `TemplateEngine` interface is intentionally minimal to:
- Make it easy to implement alternative template engines
- Reduce coupling between template processing and other components
- Enable testing with mock implementations
- Support future template formats (Helm, Jinja2, etc.)

### Why Separate Cache Implementation?

The cache is separate from the engine to:
- Allow different caching strategies (in-memory, Redis, etc.)
- Enable cache sharing across multiple engine instances
- Simplify testing and benchmarking
- Support custom eviction policies

### Why Context-Based Rendering?

Using `context.Context` for rendering enables:
- Timeout and cancellation support
- Request-scoped values and tracing
- Graceful shutdown of long-running renders
- Better integration with Go's concurrency patterns

### Why RenderContext?

The `RenderContext` type provides:
- Consistent structure for template data
- Metadata support for debugging and auditing
- Custom function registration per-render
- Validation and strict mode options
- Fluent API for easy configuration

## Performance Considerations

### Caching

Template caching significantly improves performance:
- First render: ~1ms (parse + execute)
- Cached render: ~0.1ms (execute only)
- 10x performance improvement for cached templates

### Concurrency

The engine is designed for concurrent use:
- Read-write locks for cache access
- Lock-free reads for cached templates
- Safe concurrent function registration
- No global state or shared mutable data

### Memory Usage

Cache memory usage is controlled by:
- Maximum size limit (evicts LRU entries)
- TTL-based expiration (removes stale entries)
- Manual cache clearing when needed
- Efficient template storage

## Testing

The package includes comprehensive tests:

```bash
# Run all tests
go test ./internal/template/...

# Run with coverage
go test ./internal/template/... -cover

# Run with race detection
go test ./internal/template/... -race

# Run benchmarks
go test ./internal/template/... -bench=.
```

## Future Enhancements

Planned improvements:

1. **Template Registry**: Centralized template management with metadata
2. **Template Composition**: Base templates with overlays and patches
3. **Multiple Formats**: Support for Helm, Jinja2, and other formats
4. **Validation Framework**: Comprehensive template and data validation
5. **Error Recovery**: Graceful degradation and partial rendering
6. **Metrics**: Performance metrics and monitoring integration

## Migration from Legacy System

The new template engine is designed to coexist with the existing `internal/util/template` package:

1. **Phase 1**: New code uses new engine
2. **Phase 2**: Gradual migration of existing code
3. **Phase 3**: Deprecate and remove legacy package

### Migration Documentation

- **Full Migration Guide**: `docs/migration/template-engine.md` - Comprehensive guide with examples, strategies, and best practices
- **Quick Reference**: `docs/migration/template-engine-quick-reference.md` - Quick decision tree and common patterns
- **Test Examples**: `internal/template/migration_test.go` - Working code examples and test cases

### Quick Start

Most existing code requires **no changes** - the legacy compatibility layer automatically uses the new engine. For new code:

```go
// Simple migration - use helper function
engine := template.NewGoTemplateEngine()
err := template.RenderWithEngine(engine, fsys, "template.yaml", "output.yaml", data)
```

See the migration documentation for detailed guidance on different migration paths and strategies.

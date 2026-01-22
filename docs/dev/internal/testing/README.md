# Testing Framework


## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Components](#components)
- [Best Practices](#best-practices)
- [Examples](#examples)
- [Performance](#performance)
- [Contributing](#contributing)
This package provides a consistent testing environment for openCenter tests, including test data generators, mock implementations, and test utilities.

## Overview

The testing framework provides:

- **TestFramework**: A consistent testing environment with temporary directories, template engine, and data generators
- **Data Generators**: Realistic test data generators for configurations, templates, services, and GitOps
- **Test Utilities**: Helper functions for file assertions and test setup

## Quick Start

### Basic Usage

```go
func TestMyFeature(t *testing.T) {
    // Create a test framework
    fw := testing.NewTestFramework(t)
    
    // Generate a test configuration
    cfg := fw.CreateTestConfig("openstack")
    
    // Use the configuration in your tests
    assert.Equal(t, "openstack", cfg.OpenCenter.Infrastructure.Provider)
    
    // Write and render templates
    templatePath := fw.WriteTemplate(t, "test.tmpl", "Hello {{ .Name }}!")
    data := map[string]interface{}{"Name": "World"}
    result, err := fw.TemplateEngine.Render(context.Background(), templatePath, data)
    require.NoError(t, err)
    assert.Equal(t, "Hello World!", string(result))
}
```

### Deterministic Testing

For reproducible tests, use a specific seed:

```go
func TestDeterministicBehavior(t *testing.T) {
    // Create framework with specific seed
    fw := testing.NewTestFrameworkWithSeed(t, 12345)
    
    // Generate deterministic test data
    cfg := fw.CreateTestConfig("openstack")
    
    // The same seed will always produce the same data
    assert.Equal(t, "expected-cluster-name", cfg.OpenCenter.Meta.Name)
}
```

## Components

### TestFramework

The main testing framework that provides:

- **TempDir**: Root temporary directory for test artifacts
- **ConfigDir**: Directory for test configuration files
- **TemplateDir**: Directory for test template files
- **TemplateEngine**: Template engine instance for rendering tests
- **Generators**: Data generators for various test scenarios

#### Methods

- `NewTestFramework(t *testing.T)`: Create a new test framework with default seed
- `NewTestFrameworkWithSeed(t *testing.T, seed int64)`: Create a framework with custom seed
- `WriteTemplate(t, filename, content)`: Write a template file
- `WriteFile(t, filename, content)`: Write arbitrary file
- `CreateTestConfig(provider)`: Generate a test configuration
- `CreateTestTemplateData()`: Generate template test data
- `CreateTestServiceDefinition()`: Generate service test data
- `CreateTestGitOpsConfig()`: Generate GitOps test data
- `AssertFileExists(t, path)`: Assert file exists
- `AssertFileNotExists(t, path)`: Assert file doesn't exist
- `AssertDirExists(t, path)`: Assert directory exists
- `AssertDirNotExists(t, path)`: Assert directory doesn't exist

### Data Generators

#### ConfigGenerator

Generates realistic cluster configurations:

```go
gen := testing.NewConfigGenerator(42)
cfg := gen.GenerateConfig("openstack")
```

Supports providers:
- `openstack`: OpenStack cloud configurations
- `aws`: AWS cloud configurations
- `baremetal`: Bare metal configurations

#### TemplateDataGenerator

Generates realistic template rendering data:

```go
gen := testing.NewTemplateDataGenerator(42)
data := gen.GenerateTemplateData()
// data contains: ClusterName, Namespace, Version, Replicas, Image, Port, etc.
```

#### ServiceDataGenerator

Generates realistic service definitions:

```go
gen := testing.NewServiceDataGenerator(42)
service := gen.GenerateServiceDefinition()
// service contains: name, type, enabled, version, dependencies, config
```

#### GitOpsDataGenerator

Generates realistic GitOps configurations:

```go
gen := testing.NewGitOpsDataGenerator(42)
gitops := gen.GenerateGitOpsConfig()
// gitops contains: enabled, repository, branch, path, sync settings
```

## Best Practices

### 1. Use TestFramework for Consistency

Always use `NewTestFramework(t)` to ensure consistent test environments:

```go
func TestFeature(t *testing.T) {
    fw := testing.NewTestFramework(t)
    // Test code here
}
```

### 2. Use Deterministic Seeds for Reproducibility

When you need reproducible test data, use a specific seed:

```go
func TestReproducible(t *testing.T) {
    fw := testing.NewTestFrameworkWithSeed(t, 12345)
    // Same seed = same data every time
}
```

### 3. Leverage Generators for Realistic Data

Use the generators to create realistic test data instead of hardcoding values:

```go
// Good: Realistic, varied test data
cfg := fw.CreateTestConfig("openstack")

// Avoid: Hardcoded, unrealistic test data
cfg := config.Config{
    OpenCenter: config.SimplifiedOpenCenter{
        Meta: config.ClusterMeta{Name: "test"},
    },
}
```

### 4. Use Assertion Helpers

Use the framework's assertion helpers for cleaner tests:

```go
// Good: Clear intent
fw.AssertFileExists(t, path)

// Avoid: Manual checks
if _, err := os.Stat(path); os.IsNotExist(err) {
    t.Errorf("file does not exist: %s", path)
}
```

### 5. Clean Up Automatically

The framework uses `t.TempDir()` for automatic cleanup:

```go
func TestWithCleanup(t *testing.T) {
    fw := testing.NewTestFramework(t)
    // No manual cleanup needed - t.TempDir() handles it
}
```

## Examples

### Testing Template Rendering

```go
func TestTemplateRendering(t *testing.T) {
    fw := testing.NewTestFramework(t)
    
    // Write a template
    tmpl := fw.WriteTemplate(t, "deployment.yaml.tmpl", `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Name }}
spec:
  replicas: {{ .Replicas }}
`)
    
    // Generate test data
    data := fw.CreateTestTemplateData()
    
    // Render the template
    result, err := fw.TemplateEngine.Render(context.Background(), tmpl, data)
    require.NoError(t, err)
    
    // Verify the result
    assert.Contains(t, string(result), "kind: Deployment")
}
```

### Testing Configuration Generation

```go
func TestConfigGeneration(t *testing.T) {
    fw := testing.NewTestFramework(t)
    
    // Generate a configuration
    cfg := fw.CreateTestConfig("openstack")
    
    // Verify configuration properties
    assert.Equal(t, "openstack", cfg.OpenCenter.Infrastructure.Provider)
    assert.NotEmpty(t, cfg.OpenCenter.Meta.Name)
    assert.NotEmpty(t, cfg.OpenCenter.Meta.Organization)
    
    // Verify OpenStack-specific fields
    assert.NotNil(t, cfg.OpenCenter.Infrastructure.Cloud.OpenStack)
    assert.NotEmpty(t, cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL)
}
```

### Property-Based Testing

```go
func TestConfigGeneratorProperties(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    properties.Property("all generated configs are valid", prop.ForAll(
        func(seed int64) bool {
            fw := testing.NewTestFrameworkWithSeed(t, seed)
            cfg := fw.CreateTestConfig("openstack")
            
            // Verify invariants
            return cfg.OpenCenter.Meta.Name != "" &&
                   cfg.OpenCenter.Meta.Organization != "" &&
                   cfg.OpenCenter.Infrastructure.Provider == "openstack"
        },
        gen.Int64(),
    ))
    
    properties.TestingRun(t)
}
```

## Performance

The framework is designed for fast test execution:

- Template caching is enabled by default
- Generators use efficient random number generation
- Temporary directories are cleaned up automatically

Benchmark results (on typical hardware):

```
BenchmarkConfigGenerator-8              50000    25000 ns/op
BenchmarkTemplateDataGenerator-8       100000    15000 ns/op
BenchmarkServiceDataGenerator-8        100000    12000 ns/op
BenchmarkGitOpsDataGenerator-8         100000    10000 ns/op
```

## Contributing

When adding new test utilities:

1. Add methods to `TestFramework` for common operations
2. Create new generators for new data types
3. Add tests for new functionality
4. Update this README with examples
5. Follow the existing patterns for consistency

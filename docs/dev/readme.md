# Developer Guide

**doc_type: explanation**

This guide covers the technical implementation details of opencenter-cli for developers contributing to the project. It explains the architecture, coding standards, and development workflows.

## Table of Contents

- [Getting Started](#getting-started)
- [Architecture Overview](#architecture-overview)
- [Core Abstractions](#core-abstractions)
- [Development Workflow](#development-workflow)
- [Coding Standards](#coding-standards)
- [Testing Guidelines](#testing-guidelines)
- [Common Development Tasks](#common-development-tasks)
- [Debugging and Troubleshooting](#debugging-and-troubleshooting)
- [Performance Optimization](#performance-optimization)
- [Contributing](#contributing)

## Getting Started

### Prerequisites

- Go 1.25.2 or later
- Mise for tool version management
- Git for version control
- Basic understanding of Kubernetes and GitOps

### Initial Setup

```bash
# Clone repository
git clone https://github.com/rackerlabs/opencenter-cli.git
cd opencenter-cli

# Install tools via mise
mise install

# Build binary
mise run build

# Run tests
mise run test

# Run BDD tests
mise run godog
```

### Project Structure

```
opencenter-cli/
├── cmd/                    # CLI commands (Cobra)
├── internal/               # Internal packages
│   ├── core/              # Core abstractions
│   │   ├── paths/         # Path resolution
│   │   ├── config/        # Configuration management
│   │   └── validation/    # Validation engine
│   ├── cluster/           # Domain services
│   ├── di/                # Dependency injection
│   ├── gitops/            # GitOps generation
│   ├── sops/              # Secrets management
│   └── cloud/             # Provider logic
├── docs/                   # Documentation
├── tests/                  # BDD test scenarios
└── .mise.toml             # Build tasks
```

## Architecture Overview

opencenter-cli follows clean architecture principles with clear separation of concerns:

### Layer Architecture

```
CLI Layer (cmd/)
    ↓
Domain Services (internal/cluster/)
    ↓
Core Infrastructure (internal/core/)
    ↓
Supporting Services (internal/*)
```

**Key Principles**:
1. **Single Responsibility**: Each module has one clear purpose
2. **Dependency Inversion**: High-level modules depend on abstractions
3. **DRY**: Eliminate duplication through centralization
4. **Interface Segregation**: Small, focused interfaces
5. **Open/Closed**: Open for extension, closed for modification

For detailed architecture documentation, see [Architecture](architecture.md).

## Core Abstractions

### PathResolver (internal/core/paths/)

Centralized path resolution with organization support.

**Usage**:
```go
import "github.com/rackerlabs/opencenter-cli/internal/core/paths"

resolver := paths.NewPathResolver(baseDir)
clusterPaths, err := resolver.Resolve("my-cluster", "my-org")
if err != nil {
    return err
}

// Access paths
configFile := clusterPaths.ConfigFile
secretsDir := clusterPaths.SecretsDir
gitopsDir := clusterPaths.GitOpsDir
```

**Features**:
- Organization-aware path resolution
- Caching (<100μs cached, <1ms uncached)
- Thread-safe operations
- Fallback strategies for backward compatibility

### ValidationEngine (internal/core/validation/)

Unified validation system with pluggable validators.

**Usage**:
```go
import "github.com/rackerlabs/opencenter-cli/internal/core/validation"

engine := validation.NewEngine()

// Register validators
engine.Register("cluster-name", &validators.ClusterNameValidator{})

// Validate
result, err := engine.Validate(ctx, "cluster-name", "my-cluster")
if !result.Valid {
    for _, err := range result.Errors {
        fmt.Println(err.Message)
        for _, suggestion := range err.Suggestions {
            fmt.Printf("  Suggestion: %s\n", suggestion)
        }
    }
}
```

**Features**:
- Pluggable validator architecture
- Suggestion engine for common mistakes
- Context-aware validation
- <100μs per validation

### ConfigManager (internal/core/config/)

Unified configuration loading with version handling.

**Usage**:
```go
import "github.com/rackerlabs/opencenter-cli/internal/core/config"

manager := config.NewManager()

// Load config (auto-detects version)
cfg, err := manager.Load(configPath, config.LoadOptions{
    Validate:    true,
    AutoMigrate: true,
})
if err != nil {
    return err
}

// Save config
err = manager.Save(configPath, cfg)
```

**Features**:
- Strategy pattern for v1, v2, legacy loaders
- Auto-detection of version
- Integrated migration pipeline
- Caching with invalidation

### Domain Services (internal/cluster/)

Business logic separated from CLI.

**Usage**:
```go
import "github.com/rackerlabs/opencenter-cli/internal/cluster"

// Get service from DI container
initService := container.Get(cluster.InitService)

// Invoke service
result, err := initService.Initialize(ctx, cluster.InitOptions{
    ClusterName:  "my-cluster",
    Organization: "my-org",
    Provider:     "openstack",
})
```

**Services**:
- **InitService**: Cluster initialization
- **ValidateService**: Configuration validation
- **SetupService**: GitOps repository setup
- **BootstrapService**: Infrastructure provisioning

## Development Workflow

### Building

```bash
# Build for current platform
mise run build

# Build for Linux
mise run build-linux

# Build for all platforms
mise run build-all

# Clean build artifacts
mise run clean
```

### Testing

```bash
# Run unit tests
mise run test

# Run unit tests with coverage
go test -cover ./internal/...

# Run BDD tests
mise run godog

# Run WIP scenarios only
mise run godog-wip

# Run specific test
go test -run TestPathResolver ./internal/core/paths/

# Run benchmarks
go test -bench=. ./internal/core/paths/
```

### Code Quality

```bash
# Format code
mise run fmt

# Tidy dependencies
mise run tidy

# Run linters
golangci-lint run

# Run static analysis
go vet ./...
staticcheck ./...
```

### Schema Management

```bash
# Generate JSON schema
mise run schema

# Verify schema
mise run schema-verify

# Validate configuration
mise run validate
```

## Coding Standards

### Go Conventions

- **Formatting**: Use `gofmt` (run `mise run fmt`)
- **Naming**: `CamelCase` for exported, `mixedCase` for locals
- **Imports**: Standard library, external deps, internal packages
- **Error Handling**: Always check errors, wrap with context

**Example**:
```go
import (
    "context"
    "fmt"
    
    "github.com/spf13/cobra"
    
    "github.com/rackerlabs/opencenter-cli/internal/config"
)

func processCluster(ctx context.Context, name string) error {
    cfg, err := loadConfig(name)
    if err != nil {
        return fmt.Errorf("loading config: %w", err)
    }
    
    // Process config
    return nil
}
```

### File Organization

- **Test files**: `*_test.go` (unit), `*_property_test.go` (property-based)
- **Test functions**: `TestXxx` naming convention
- **Benchmarks**: `BenchmarkXxx` naming convention
- **Examples**: `ExampleXxx` naming convention

### Documentation

- **Package docs**: `doc.go` in each package
- **Function docs**: Godoc comments for exported functions
- **Examples**: Runnable examples in `example_test.go`

**Example**:
```go
// Package paths provides centralized path resolution for cluster resources.
//
// The PathResolver handles all path construction with support for
// organization-based directory structures and intelligent caching.
//
// Example usage:
//
//	resolver := paths.NewPathResolver("/base")
//	paths, err := resolver.Resolve("my-cluster", "my-org")
//	if err != nil {
//	    return err
//	}
//	fmt.Println(paths.ConfigFile)
package paths
```

### Error Handling

- **Always check errors**: Never ignore error returns
- **Wrap errors**: Add context with `fmt.Errorf("context: %w", err)`
- **Sentinel errors**: Use `errors.Is()` for comparison
- **Error types**: Define custom error types for specific cases

**Example**:
```go
var ErrClusterNotFound = errors.New("cluster not found")

func loadCluster(name string) (*Cluster, error) {
    path, err := resolvePath(name)
    if err != nil {
        return nil, fmt.Errorf("resolving path for %s: %w", name, err)
    }
    
    if !fileExists(path) {
        return nil, ErrClusterNotFound
    }
    
    // Load cluster
    return cluster, nil
}

// Usage
cluster, err := loadCluster("my-cluster")
if errors.Is(err, ErrClusterNotFound) {
    // Handle not found
}
```

## Testing Guidelines

### Unit Tests

**Focus**: Test individual functions and methods in isolation.

**Structure**:
```go
func TestPathResolver_Resolve(t *testing.T) {
    // Setup
    resolver := paths.NewPathResolver("/base")
    
    // Execute
    paths, err := resolver.Resolve("test-cluster", "test-org")
    
    // Verify
    require.NoError(t, err)
    assert.Equal(t, "/base/test-org/test-cluster", paths.ClusterDir)
}
```

**Best Practices**:
- Use table-driven tests for multiple cases
- Use `testify/require` for fatal assertions
- Use `testify/assert` for non-fatal assertions
- Test error cases explicitly

### Integration Tests

**Focus**: Test complete workflows end-to-end.

**Structure**:
```go
func TestClusterInit_Integration(t *testing.T) {
    // Setup
    tmpDir := t.TempDir()
    container := setupTestContainer(tmpDir)
    
    // Execute
    svc := container.Get(InitService)
    result, err := svc.Initialize(context.Background(), InitOptions{
        ClusterName:  "test-cluster",
        Organization: "test-org",
    })
    
    // Verify
    require.NoError(t, err)
    assert.FileExists(t, result.ConfigPath)
    
    // Verify config content
    cfg, err := loadConfig(result.ConfigPath)
    require.NoError(t, err)
    assert.Equal(t, "test-cluster", cfg.Cluster.Name)
}
```

### Benchmark Tests

**Focus**: Measure performance of critical operations.

**Structure**:
```go
func BenchmarkPathResolver_Resolve(b *testing.B) {
    resolver := paths.NewPathResolver("/base")
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        resolver.Resolve("test-cluster", "test-org")
    }
}
```

**Run benchmarks**:
```bash
go test -bench=. -benchmem ./internal/core/paths/
```

### Property-Based Tests

**Focus**: Test invariants across many generated inputs.

**Structure**:
```go
func TestPathResolver_Properties(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    properties.Property("Resolve is deterministic", prop.ForAll(
        func(name, org string) bool {
            resolver := paths.NewPathResolver("/base")
            paths1, _ := resolver.Resolve(name, org)
            paths2, _ := resolver.Resolve(name, org)
            return reflect.DeepEqual(paths1, paths2)
        },
        gen.Identifier(),
        gen.Identifier(),
    ))
    
    properties.TestingRun(t)
}
```

### BDD Tests

**Focus**: Test user-facing behavior with Gherkin scenarios.

**Location**: `tests/features/*.feature`

**Example**:
```gherkin
Feature: Cluster Initialization
  
  Scenario: Initialize new cluster
    Given I have opencenter installed
    When I run "opencenter cluster init test-cluster"
    Then the command should succeed
    And a config file should exist at "~/.config/opencenter/clusters/opencenter/.test-cluster-config.yaml"
```

## Common Development Tasks

### Adding a New Command

1. Create command file in `cmd/`:
```go
// cmd/cluster_mycommand.go
func newClusterMyCommandCmd(container *di.Container) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "mycommand <cluster-name>",
        Short: "Description of my command",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Parse flags
            opts := parseMyCommandFlags(cmd, args)
            
            // Get service
            svc := container.Get(MyCommandService)
            
            // Invoke service
            result, err := svc.Execute(cmd.Context(), opts)
            
            // Display result
            return displayMyCommandResult(result, err)
        },
    }
    return cmd
}
```

2. Create service in `internal/cluster/`:
```go
// internal/cluster/mycommand_service.go
type MyCommandService struct {
    pathResolver  paths.PathResolver
    configManager config.ConfigManager
}

func (s *MyCommandService) Execute(ctx context.Context, opts MyCommandOptions) (*MyCommandResult, error) {
    // Business logic here
    return &MyCommandResult{}, nil
}
```

3. Register service in DI container:
```go
// internal/di/providers.go
func ProvideMyCommandService(
    pathResolver paths.PathResolver,
    configManager config.ConfigManager,
) *cluster.MyCommandService {
    return &cluster.MyCommandService{
        pathResolver:  pathResolver,
        configManager: configManager,
    }
}
```

4. Add tests:
```go
// internal/cluster/mycommand_service_test.go
func TestMyCommandService_Execute(t *testing.T) {
    // Test implementation
}
```

### Adding a New Validator

1. Create validator in `internal/core/validation/validators/`:
```go
// internal/core/validation/validators/myvalidator.go
type MyValidator struct{}

func (v *MyValidator) Validate(ctx context.Context, value interface{}) validation.ValidationResult {
    result := validation.ValidationResult{Valid: true}
    
    // Validation logic
    if !isValid(value) {
        result.Valid = false
        result.Errors = append(result.Errors, validation.ValidationError{
            Field:   "field-name",
            Message: "validation failed",
            Suggestions: []string{"try this instead"},
        })
    }
    
    return result
}
```

2. Register validator:
```go
// In init() or setup code
engine.Register("my-validator", &validators.MyValidator{})
```

3. Add tests:
```go
// internal/core/validation/validators/myvalidator_test.go
func TestMyValidator_Validate(t *testing.T) {
    validator := &MyValidator{}
    
    result := validator.Validate(context.Background(), "test-value")
    
    assert.True(t, result.Valid)
}
```

### Adding a New Config Version

1. Create strategy in `internal/core/config/strategies/`:
```go
// internal/core/config/strategies/v3.go
type V3Strategy struct{}

func (s *V3Strategy) CanLoad(data []byte) bool {
    return bytes.Contains(data, []byte("schema_version: v3"))
}

func (s *V3Strategy) Load(data []byte) (*config.Config, error) {
    var cfg config.Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
```

2. Create migration in `internal/core/config/migration/`:
```go
// internal/core/config/migration/v2_to_v3.go
func MigrateV2ToV3(v2Config *config.Config) (*config.Config, error) {
    v3Config := *v2Config
    // Migration logic
    return &v3Config, nil
}
```

3. Register strategy in ConfigManager:
```go
// internal/core/config/manager.go
func NewManager() *Manager {
    return &Manager{
        strategies: []LoadStrategy{
            &strategies.V3Strategy{},
            &strategies.V2Strategy{},
            &strategies.V1Strategy{},
            &strategies.LegacyStrategy{},
        },
    }
}
```

## Debugging and Troubleshooting

### Debugging Tips

**Enable verbose logging**:
```bash
export OPENCENTER_LOG_LEVEL=debug
./bin/opencenter cluster init test-cluster
```

**Use delve debugger**:
```bash
dlv debug ./cmd/opencenter -- cluster init test-cluster
```

**Print debug info**:
```go
import "github.com/davecgh/go-spew/spew"

spew.Dump(config)
```

### Common Issues

**Issue**: Path resolution fails
```bash
# Check base directory
echo $OPENCENTER_CONFIG_DIR

# Verify organization structure
ls -la ~/.config/opencenter/clusters/
```

**Issue**: Config loading fails
```bash
# Validate YAML syntax
yamllint ~/.config/opencenter/clusters/opencenter/.test-cluster-config.yaml

# Check schema version
grep schema_version ~/.config/opencenter/clusters/opencenter/.test-cluster-config.yaml
```

**Issue**: Tests fail
```bash
# Run specific test with verbose output
go test -v -run TestPathResolver ./internal/core/paths/

# Check test fixtures
ls -la internal/core/paths/testdata/
```

## Performance Optimization

### Profiling

**CPU profiling**:
```bash
go test -cpuprofile=cpu.prof -bench=. ./internal/core/paths/
go tool pprof cpu.prof
```

**Memory profiling**:
```bash
go test -memprofile=mem.prof -bench=. ./internal/core/paths/
go tool pprof mem.prof
```

**Trace analysis**:
```bash
go test -trace=trace.out ./internal/core/paths/
go tool trace trace.out
```

### Performance Targets

- **Path Resolution**: <1ms uncached, <100μs cached
- **Config Loading**: <100ms
- **Validation**: <100μs per validator
- **Memory Usage**: <100MB peak

### Optimization Techniques

1. **Caching**: Cache expensive operations (path resolution, config loading)
2. **Pooling**: Reuse buffers for YAML parsing
3. **Lazy Loading**: Load config only when needed
4. **Parallel Validation**: Run independent validators concurrently

## Contributing

### Contribution Workflow

1. **Fork repository**: Create personal fork on GitHub
2. **Create branch**: `git checkout -b feature/my-feature`
3. **Make changes**: Follow coding standards
4. **Add tests**: Ensure >90% coverage
5. **Run checks**: `mise run fmt && mise run test && mise run godog`
6. **Commit**: Use conventional commits (`feat:`, `fix:`, `docs:`)
7. **Push**: `git push origin feature/my-feature`
8. **Create PR**: Submit pull request with description

### PR Requirements

- [ ] All tests pass
- [ ] Code formatted with `gofmt`
- [ ] New code has >90% test coverage
- [ ] Documentation updated
- [ ] Conventional commit messages
- [ ] No breaking changes (or documented)

### Code Review Process

1. **Automated checks**: CI runs tests, linters, benchmarks
2. **Peer review**: At least one approval required
3. **Maintainer review**: Final approval from maintainer
4. **Merge**: Squash and merge to main

## References

- [Architecture Documentation](architecture.md)
- [Path Resolution Deprecation](path-resolution-deprecation.md)
- [Deprecation Timeline](deprecation-timeline.md)
- [Contributing Guide](../contributing.md)
- [Architectural Refactoring Spec](../../.kiro/specs/architectural-refactoring/README.md)

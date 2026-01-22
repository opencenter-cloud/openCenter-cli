---
doc_type: reference
---

# Coding Standards


## Table of Contents

- [Who this is for](#who-this-is-for)
- [Go Language Standards](#go-language-standards)
- [Code Organization](#code-organization)
- [Error Handling](#error-handling)
- [Testing Standards](#testing-standards)
- [Documentation Standards](#documentation-standards)
- [Dependency Management](#dependency-management)
- [Concurrency](#concurrency)
- [Security Practices](#security-practices)
- [Performance Considerations](#performance-considerations)
- [Common Patterns](#common-patterns)
- [Code Review Checklist](#code-review-checklist)
- [See Also](#see-also)
This document defines the code style, naming conventions, and best practices for openCenter development. These standards are derived from actual codebase patterns and Go community conventions.

## Who this is for

Developers contributing to openCenter who need to understand expected code style, naming conventions, testing patterns, and architectural principles.

## Go Language Standards

### Formatting

All Go code must be formatted with `gofmt`:

```bash
mise run fmt
```

**Rules**:
- Use tabs for indentation (gofmt default)
- No trailing whitespace
- One blank line between top-level declarations
- Group imports by standard library, external, and internal

**Import Organization**:

```go
import (
    // Standard library
    "context"
    "fmt"
    "os"
    
    // External dependencies
    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"
    
    // Internal packages
    "github.com/rackerlabs/openCenter-cli/internal/config"
    "github.com/rackerlabs/openCenter-cli/internal/security"
)
```

### Naming Conventions

#### Packages

- Use lowercase, single-word names when possible
- Avoid underscores or mixed caps
- Use descriptive names that indicate purpose

```go
// Good
package config
package gitops
package security

// Avoid
package configManager
package git_ops
package sec
```

#### Files

**Command Files**: `<noun>_<verb>.go`

```
cmd/cluster_init.go
cmd/cluster_validate.go
cmd/cluster_setup.go
cmd/sops_encrypt.go
```

**Test Files**: `<name>_test.go`, `<name>_property_test.go`, `<name>_integration_test.go`

```
config_test.go              # Unit tests
builder_property_test.go    # Property-based tests
gitops_integration_test.go  # Integration tests
```

**Documentation Files**: `doc.go` in each package

```go
/*
Package config provides functionality for managing cluster configurations.

This package defines the data structures for the cluster configuration, as well
as functions for loading, saving, and validating configurations.

# When to use

This package is used internally by openCenter to manage cluster configurations.
*/
package config
```

#### Functions and Methods

**Exported Functions**: Use `CamelCase`

```go
func NewConfigManager(path string) (*ConfigManager, error)
func ValidateClusterName(name string) error
func GenerateDefaultFromSchema(clusterName string) ([]byte, error)
```

**Unexported Functions**: Use `mixedCase`

```go
func setField(obj any, path string, value string) error
func convertStringValue(value string) any
func validateOrganizationName(org string) error
```

**Constructor Pattern**: `New<Type>` or `New<Type>With<Options>`

```go
func NewConfigManager(path string) (*ConfigManager, error)
func NewConfigManagerWithConfig(cfg *CLIConfig) (*ConfigManager, error)
func NewDefaultInputValidator() *InputValidator
```

**Command Constructors**: `new<Command><Action>Cmd()`

```go
func newClusterInitCmd() *cobra.Command
func newClusterValidateCmd() *cobra.Command
func newSOPSEncryptCmd() *cobra.Command
```

#### Variables and Constants

**Exported Constants**: Use `CamelCase`

```go
const (
    StageInit      = "init"
    StageValidate  = "validate"
    StatusSuccess  = "success"
    StatusFailed   = "failed"
)
```

**Unexported Constants**: Use `mixedCase`

```go
const (
    defaultTimeout = 30 * time.Second
    maxRetries     = 3
)
```

**Error Variables**: Prefix with `Err`

```go
var (
    ErrClusterNotFound = errors.New("cluster not found")
    ErrInvalidConfig   = errors.New("invalid configuration")
)
```

#### Types

**Structs**: Use `CamelCase` for exported, `mixedCase` for unexported

```go
// Exported
type Config struct {
    OpenCenter OpenCenterConfig `yaml:"opencenter"`
    Secrets    SecretsConfig    `yaml:"secrets"`
}

// Unexported
type retryHandler struct {
    config RetryConfig
    mu     sync.Mutex
}
```

**Interfaces**: Use `CamelCase`, often ending in `-er` or `Interface`

```go
type ConfigLoader interface {
    Load(path string) (*Config, error)
}

type ConfigManagerInterface interface {
    LoadConfig(ctx context.Context, name string) (*Config, error)
    SaveConfig(ctx context.Context, config *Config) error
}
```

**Error Types**: Suffix with `Error`

```go
type TalosError struct {
    Code     string
    Message  string
    Category ErrorCategory
}

type TemplateError struct {
    Type     TemplateErrorType
    Template string
    Message  string
}
```

## Code Organization

### Package Structure

Each package should have a single, well-defined responsibility:

```
internal/config/
├── doc.go                  # Package documentation
├── interfaces.go           # Interface definitions
├── config.go               # Main types
├── types_*.go              # Domain-specific types
├── loader.go               # Loading logic
├── validator.go            # Validation logic
├── manager.go              # Lifecycle management
└── *_test.go               # Tests
```

### File Organization

Within a file, organize code in this order:

1. Package declaration and documentation
2. Imports (grouped by standard/external/internal)
3. Constants
4. Variables
5. Type definitions
6. Constructor functions
7. Methods (grouped by receiver)
8. Helper functions

Example:

```go
// Copyright header

package config

import (
    // imports
)

const (
    // constants
)

var (
    // package-level variables
)

type Config struct {
    // fields
}

func NewConfig(name string) *Config {
    // constructor
}

func (c *Config) Validate() error {
    // method
}

func validateField(value string) error {
    // helper
}
```

## Error Handling

### Error Wrapping

Always wrap errors with context using `fmt.Errorf` with `%w`:

```go
if err := loader.Load(path); err != nil {
    return fmt.Errorf("failed to load config from %s: %w", path, err)
}
```

**Don't** lose error context:

```go
// Bad
if err := loader.Load(path); err != nil {
    return errors.New("failed to load config")
}

// Good
if err := loader.Load(path); err != nil {
    return fmt.Errorf("failed to load config from %s: %w", path, err)
}
```

### Error Types

Use structured error types for domain-specific errors:

```go
type TalosError struct {
    Code        string
    Message     string
    Category    ErrorCategory
    Retryable   bool
    Remediation *RemediationAction
    Err         error
}

func (e *TalosError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("[%s] %s: %s (code: %s)", 
            e.Category, e.Message, e.Err.Error(), e.Code)
    }
    return fmt.Sprintf("[%s] %s (code: %s)", e.Category, e.Message, e.Code)
}

func (e *TalosError) Unwrap() error {
    return e.Err
}
```

### Error Constructors

Provide constructor functions for common error types:

```go
func NewValidationError(code, message string, remediation *RemediationAction) *TalosError {
    return &TalosError{
        Code:        code,
        Message:     message,
        Category:    ErrorCategoryValidation,
        Retryable:   false,
        Remediation: remediation,
    }
}
```

### Error Checking

Check errors immediately after function calls:

```go
// Good
cfg, err := config.Load(name)
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}

// Avoid deferring error checks
cfg, err := config.Load(name)
// ... other code ...
if err != nil {
    return err
}
```

## Testing Standards

### Unit Tests

**File Naming**: `*_test.go`

**Function Naming**: `Test<FunctionName>` or `Test<Type>_<Method>`

```go
func TestConfigValidation(t *testing.T) {
    // test code
}

func TestConfigManager_LoadConfig(t *testing.T) {
    // test code
}
```

**Test Structure**: Use table-driven tests for multiple cases:

```go
func TestValidateClusterName(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {
            name:    "valid cluster name",
            input:   "my-cluster",
            wantErr: false,
        },
        {
            name:    "empty name",
            input:   "",
            wantErr: true,
        },
        {
            name:    "name too long",
            input:   strings.Repeat("a", 100),
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateClusterName(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateClusterName() error = %v, wantErr %v", 
                    err, tt.wantErr)
            }
        })
    }
}
```

**Assertions**: Use `testify/assert` or `testify/require`:

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestConfigLoad(t *testing.T) {
    cfg, err := config.Load("test-cluster")
    require.NoError(t, err)
    assert.Equal(t, "test-cluster", cfg.ClusterName())
}
```

**Temporary Files**: Use `t.TempDir()` for test files:

```go
func TestFileOperations(t *testing.T) {
    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.yaml")
    
    // test code using testFile
}
```

### Property-Based Tests

**File Naming**: `*_property_test.go`

**Function Naming**: `TestProperty_<PropertyName>`

```go
func TestProperty_BuilderMethodChaining(t *testing.T) {
    parameters := gopter.DefaultTestParameters()
    parameters.MinSuccessfulTests = 100
    properties := gopter.NewProperties(parameters)
    
    properties.Property("builder methods support chaining", prop.ForAll(
        func(clusterName, org, provider string) bool {
            builder := NewConfigBuilder(clusterName)
            result := builder.
                WithOrganization(org).
                WithProvider(provider).
                WithEnvironment("test")
            
            _, ok := result.(ConfigBuilder)
            return ok
        },
        genValidClusterName(),
        genOrganization(),
        genProvider(),
    ))
    
    properties.TestingRun(t, gopter.ConsoleReporter(false))
}
```

**Generators**: Define custom generators for domain types:

```go
func genValidClusterName() gopter.Gen {
    return gen.RegexMatch("[a-zA-Z0-9][a-zA-Z0-9._-]{1,18}[a-zA-Z0-9]")
}

func genProvider() gopter.Gen {
    return gen.OneConstOf("openstack", "aws", "baremetal", "kind")
}
```

### BDD Tests

**File Location**: `tests/features/*.feature`

**Feature Files**: Use Gherkin syntax:

```gherkin
Feature: Cluster Initialization
  As a cluster operator
  I want to initialize cluster configurations
  So that I can deploy Kubernetes clusters

  Scenario: Initialize cluster with defaults
    When I run "openCenter cluster init my-cluster"
    Then the command should succeed
    And a cluster configuration "my-cluster" should exist
    And the configuration should be valid
```

**Step Definitions**: `tests/features/steps/*.go`

```go
func TestFeatures(t *testing.T) {
    opts := godog.Options{
        Output: colors.Colored(os.Stdout),
        Format: "pretty",
        Paths:  []string{"../features"},
        Tags:   "~@wip",
    }
    
    status := godog.TestSuite{
        Name:                 "openCenter",
        ScenarioInitializer:  InitializeScenario,
        Options:              &opts,
    }.Run()
    
    if status != 0 {
        t.Fail()
    }
}
```

**WIP Tag**: Use `@wip` for work-in-progress scenarios:

```gherkin
@wip
Scenario: Complex multi-cluster setup
  # scenario under development
```

### Integration Tests

**File Naming**: `*_integration_test.go`

**Build Tag**: Use build tags to separate integration tests:

```go
//go:build integration
// +build integration

package gitops

func TestGitOpsGenerationIntegration(t *testing.T) {
    // integration test code
}
```

Run with: `go test -tags=integration ./...`

## Documentation Standards

### Package Documentation

Every package must have a `doc.go` file:

```go
/*
Package config provides functionality for managing cluster configurations.

This package defines the data structures for the cluster configuration, as well
as functions for loading, saving, and validating configurations. It also includes
functionality for generating a JSON schema for the configuration.

# When to use

This package is used internally by openCenter to manage cluster configurations.
It is not intended for direct use by end-users.

# Configuration structure

The main data structure in this package is the `Config` struct, which represents
the root configuration for a cluster.
*/
package config
```

### Function Documentation

Document exported functions with comments:

```go
// NewConfigManager creates a new configuration manager with the specified path.
// If path is empty, uses the default configuration directory.
//
// The configuration manager handles loading, validation, and lifecycle management
// of cluster configurations.
//
// Returns an error if the configuration directory cannot be accessed or created.
func NewConfigManager(path string) (*ConfigManager, error) {
    // implementation
}
```

**Format**:
- First sentence is a summary (appears in godoc)
- Additional paragraphs provide details
- Document parameters and return values
- Mention error conditions

### Type Documentation

Document exported types:

```go
// Config represents the root configuration for an openCenter cluster.
//
// The configuration is organized into logical sections:
//   - OpenCenter: Core cluster configuration
//   - Secrets: Secrets management configuration
//   - Metadata: Cluster metadata and timestamps
//
// Configuration files are stored in YAML format with organization-based
// directory structure.
type Config struct {
    OpenCenter OpenCenterConfig `yaml:"opencenter"`
    Secrets    SecretsConfig    `yaml:"secrets"`
    Metadata   Metadata         `yaml:"metadata,omitempty"`
}
```

### Example Code

Provide example code in `*_example_test.go` files:

```go
func ExampleConfigBuilder() {
    config := NewConfigBuilder("my-cluster").
        WithOrganization("myorg").
        WithProvider("openstack").
        WithKubernetesVersion("1.28.0").
        Build()
    
    fmt.Println(config.ClusterName())
    // Output: my-cluster
}
```

## Dependency Management

### Dependency Injection

Pass dependencies explicitly, avoid global state:

```go
// Good: Dependencies injected
type ConfigManager struct {
    loader    ConfigLoader
    validator ConfigValidator
    resolver  PathResolver
}

func NewConfigManager(
    loader ConfigLoader,
    validator ConfigValidator,
    resolver PathResolver,
) *ConfigManager {
    return &ConfigManager{
        loader:    loader,
        validator: validator,
        resolver:  resolver,
    }
}

// Avoid: Global state
var globalLoader ConfigLoader

func LoadConfig(name string) (*Config, error) {
    return globalLoader.Load(name)
}
```

### Interface-Based Design

Define interfaces in consumer packages:

```go
// internal/gitops/interfaces.go
type TemplateEngine interface {
    Render(template string, data interface{}) (string, error)
}

// internal/gitops/generator.go
type Generator struct {
    engine TemplateEngine
}

// internal/template/engine.go implements the interface
```

### Functional Options

Use functional options for complex constructors:

```go
type Option func(*ConfigManager) error

func WithLoader(loader ConfigLoader) Option {
    return func(cm *ConfigManager) error {
        cm.loader = loader
        return nil
    }
}

func WithValidator(validator ConfigValidator) Option {
    return func(cm *ConfigManager) error {
        cm.validator = validator
        return nil
    }
}

func NewConfigManager(opts ...Option) (*ConfigManager, error) {
    cm := &ConfigManager{
        loader:    defaultLoader,
        validator: defaultValidator,
    }
    
    for _, opt := range opts {
        if err := opt(cm); err != nil {
            return nil, err
        }
    }
    
    return cm, nil
}
```

## Concurrency

### Goroutine Management

Always provide a way to stop goroutines:

```go
// Good: Context-based cancellation
func (w *Worker) Start(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case work := <-w.workCh:
            w.process(work)
        }
    }
}

// Avoid: No way to stop
func (w *Worker) Start() {
    for {
        work := <-w.workCh
        w.process(work)
    }
}
```

### Mutex Usage

Protect shared state with mutexes:

```go
type Cache struct {
    mu    sync.RWMutex
    items map[string]*Config
}

func (c *Cache) Get(key string) (*Config, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    item, ok := c.items[key]
    return item, ok
}

func (c *Cache) Set(key string, config *Config) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    c.items[key] = config
}
```

### Channel Usage

Use channels for communication, not shared memory:

```go
// Good: Channel-based communication
type Pipeline struct {
    stages []Stage
    dataCh chan Data
}

func (p *Pipeline) Process(ctx context.Context, input Data) error {
    p.dataCh <- input
    
    for _, stage := range p.stages {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case data := <-p.dataCh:
            result, err := stage.Process(data)
            if err != nil {
                return err
            }
            p.dataCh <- result
        }
    }
    
    return nil
}
```

## Security Practices

### Input Validation

Validate all user input:

```go
func ValidateClusterName(name string) error {
    if name == "" {
        return errors.New("cluster name cannot be empty")
    }
    
    if len(name) > 63 {
        return errors.New("cluster name too long (max 63 characters)")
    }
    
    if !clusterNameRegex.MatchString(name) {
        return errors.New("cluster name must match pattern [a-z0-9]([-a-z0-9]*[a-z0-9])?")
    }
    
    return nil
}
```

### Path Validation

Prevent path traversal attacks:

```go
func ValidatePath(path string) error {
    // Resolve to absolute path
    absPath, err := filepath.Abs(path)
    if err != nil {
        return fmt.Errorf("invalid path: %w", err)
    }
    
    // Check for path traversal
    if strings.Contains(absPath, "..") {
        return errors.New("path traversal not allowed")
    }
    
    return nil
}
```

### Credential Masking

Mask credentials in all output:

```go
func (m *CredentialMasker) MaskString(input string) string {
    output := input
    
    for _, pattern := range m.patterns {
        output = pattern.ReplaceAllString(output, "****")
    }
    
    return output
}
```

### File Permissions

Set restrictive permissions on sensitive files:

```go
// Configuration files: 0600 (owner read/write only)
if err := os.WriteFile(configPath, data, 0600); err != nil {
    return fmt.Errorf("failed to write config: %w", err)
}

// SSH private keys: 0600
if err := os.WriteFile(keyPath, privateKey, 0600); err != nil {
    return fmt.Errorf("failed to write key: %w", err)
}

// Directories: 0700 (owner access only)
if err := os.MkdirAll(secretsDir, 0700); err != nil {
    return fmt.Errorf("failed to create directory: %w", err)
}
```

## Performance Considerations

### Avoid Premature Optimization

Write clear code first, optimize when needed:

```go
// Good: Clear and correct
func FindCluster(clusters []Cluster, name string) *Cluster {
    for i := range clusters {
        if clusters[i].Name == name {
            return &clusters[i]
        }
    }
    return nil
}

// Avoid: Premature optimization
func FindCluster(clusters []Cluster, name string) *Cluster {
    // Complex optimization that's hard to understand
    // and may not provide meaningful benefit
}
```

### Use Appropriate Data Structures

Choose data structures based on access patterns:

```go
// Frequent lookups: Use map
type Registry struct {
    services map[string]*Service
}

// Ordered iteration: Use slice
type Pipeline struct {
    stages []Stage
}

// Concurrent access: Use sync.Map
type Cache struct {
    items sync.Map
}
```

### Minimize Allocations

Reuse buffers and objects when appropriate:

```go
// Good: Reuse buffer
var buf bytes.Buffer
for _, item := range items {
    buf.Reset()
    buf.WriteString(item.String())
    process(buf.String())
}

// Avoid: Allocate on each iteration
for _, item := range items {
    s := item.String()
    process(s)
}
```

## Common Patterns

### Builder Pattern

Use for complex object construction:

```go
type ConfigBuilder interface {
    WithOrganization(org string) ConfigBuilder
    WithProvider(provider string) ConfigBuilder
    WithKubernetesVersion(version string) ConfigBuilder
    Build() (*Config, error)
}

type configBuilder struct {
    config Config
}

func NewConfigBuilder(name string) ConfigBuilder {
    return &configBuilder{
        config: Config{
            OpenCenter: OpenCenterConfig{
                Meta: MetaConfig{
                    Name: name,
                },
            },
        },
    }
}

func (b *configBuilder) WithOrganization(org string) ConfigBuilder {
    b.config.OpenCenter.Meta.Organization = org
    return b
}

func (b *configBuilder) Build() (*Config, error) {
    if err := b.validate(); err != nil {
        return nil, err
    }
    return &b.config, nil
}
```

### Factory Pattern

Use for creating configured instances:

```go
func NewConfigManager(opts ...Option) (*ConfigManager, error) {
    cm := &ConfigManager{
        loader:    defaultLoader,
        validator: defaultValidator,
    }
    
    for _, opt := range opts {
        if err := opt(cm); err != nil {
            return nil, err
        }
    }
    
    return cm, nil
}
```

### Registry Pattern

Use for managing collections of related objects:

```go
type ServiceRegistry struct {
    mu       sync.RWMutex
    services map[string]*ServiceDefinition
}

func (r *ServiceRegistry) Register(name string, service *ServiceDefinition) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if _, exists := r.services[name]; exists {
        return fmt.Errorf("service %s already registered", name)
    }
    
    r.services[name] = service
    return nil
}

func (r *ServiceRegistry) Get(name string) (*ServiceDefinition, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    service, ok := r.services[name]
    if !ok {
        return nil, fmt.Errorf("service %s not found", name)
    }
    
    return service, nil
}
```

## Code Review Checklist

Before submitting code for review, verify:

- [ ] Code is formatted with `gofmt`
- [ ] All tests pass (`mise run test && mise run godog`)
- [ ] New code has tests (unit, property, or BDD)
- [ ] Exported functions have documentation
- [ ] Errors are wrapped with context
- [ ] No global state or mutable globals
- [ ] Input validation on all user input
- [ ] Credentials masked in logs
- [ ] File permissions set correctly
- [ ] No hardcoded paths or credentials
- [ ] Interfaces defined in consumer packages
- [ ] Dependencies injected, not global

## See Also

- [Developer Guide](./README.md) - Development setup and workflows
- [Architecture Documentation](./architecture.md) - Codebase architecture
- [Testing Guide](./testing/README.md) - Testing strategies
- [Contributing Guidelines](./contributing.md) - Contribution process

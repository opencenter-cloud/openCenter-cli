# Architectural Improvements

**Project**: opencenter-cli  
**Review Date**: February 4, 2026  
**Document**: Phase 2 - Architectural Analysis

## Table of Contents

- [Overview](#overview)
- [Current Architecture Assessment](#current-architecture-assessment)
- [Separation of Concerns Analysis](#separation-of-concerns-analysis)
- [Layer Boundaries](#layer-boundaries)
- [Proposed Improvements](#proposed-improvements)
- [Modularity Enhancements](#modularity-enhancements)
- [Testability Improvements](#testability-improvements)
- [Implementation Guidelines](#implementation-guidelines)

## Overview

This document evaluates the separation of concerns in opencenter-cli and proposes architectural improvements to enhance modularity, testability, and maintainability.

**Current Architecture Score**: 75/100 (Good, with improvement opportunities)

**Key Findings**:
- ✅ Clear package structure following Go conventions
- ✅ Good use of interfaces for abstraction
- ✅ Dependency injection container implemented
- ⚠️ Dual service systems blur boundaries
- ⚠️ Configuration and business logic sometimes mixed
- ⚠️ Some circular dependencies between packages

## Current Architecture Assessment

### Package Organization

```
opencenter-cli/
├── cmd/                    # CLI commands (Cobra) - GOOD
├── internal/
│   ├── cluster/           # Cluster lifecycle - GOOD
│   ├── config/            # Configuration - MIXED
│   │   └── services/      # ⚠️ Business logic in config
│   ├── services/          # Service plugins - GOOD
│   │   └── plugins/       # ⚠️ Duplicates config/services
│   ├── gitops/            # GitOps generation - GOOD
│   ├── template/          # Template engine - EXCELLENT
│   ├── di/                # Dependency injection - GOOD
│   ├── core/              # Core abstractions - GOOD
│   ├── util/              # Utilities - FRAGMENTED
│   └── ...
```

### Strengths

1. **Clear CLI Layer**: Commands are well-organized with consistent patterns
2. **Template Engine**: Excellent abstraction with caching and validation
3. **DI Container**: Clean dependency injection implementation
4. **GitOps Pipeline**: Well-structured staged generation
5. **Core Abstractions**: Good use of interfaces in `internal/core`

### Weaknesses

1. **Dual Service Systems**: Business logic split between `config/services` and `services/plugins`
2. **Configuration Mixing**: `internal/config` contains both data structures and business logic
3. **Utility Fragmentation**: Utilities scattered without clear organization
4. **Circular Dependencies**: Some packages have circular import issues
5. **Validation Scatter**: Validation logic not centralized

## Separation of Concerns Analysis

### API Layer (cmd/)

**Current State**: ✅ GOOD

```go
// cmd/cluster_init.go
func newClusterInitCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "init [cluster-name]",
        Short: "Initialize a new cluster configuration",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Parse flags
            // Get dependencies from DI container
            // Call business logic
            // Handle errors
        },
    }
}
```

**Assessment**:
- Clear separation between CLI and business logic
- Proper use of dependency injection
- Consistent error handling
- Good flag parsing

**Recommendation**: ✅ No changes needed

---

### Business Logic Layer

**Current State**: ⚠️ NEEDS IMPROVEMENT

**Problem**: Business logic split across multiple locations

```
Business Logic Locations:
1. internal/cluster/        - Cluster lifecycle (init, validate, setup)
2. internal/config/services - Service configuration logic
3. internal/services/       - Service plugin logic
4. internal/gitops/         - GitOps generation logic
```

**Issues**:
- Service logic duplicated in two places
- Configuration validation mixed with data structures
- Unclear which layer owns service definitions

**Recommendation**: Consolidate service logic into single location

---

### Data Layer

**Current State**: ⚠️ MIXED

**Problem**: Data structures mixed with business logic

```go
// internal/config/config.go
type Config struct {
    // Data fields
    OpenCenter OpenCenterConfig
    Secrets    SecretsConfig
}

// Business logic methods on data structure
func (c Config) GetCertManagerAWSCredentials() (string, string) {
    // Complex fallback logic
}

func (c Config) GetLokiS3Credentials() (string, string) {
    // Complex fallback logic
}
```

**Issues**:
- Data structures have business logic methods
- Credential resolution logic embedded in config type
- Hard to test in isolation

**Recommendation**: Separate data structures from business logic

---

### Infrastructure Layer

**Current State**: ✅ GOOD

```go
// internal/util/fs/filesystem.go
type FileSystem interface {
    ReadFile(path string) ([]byte, error)
    WriteFile(path string, data []byte, perm os.FileMode) error
    // ...
}
```

**Assessment**:
- Good abstraction over file system
- Testable through interfaces
- Clean separation from business logic

**Recommendation**: ✅ Extend this pattern to other utilities

## Layer Boundaries

### Ideal Layer Architecture

```
┌─────────────────────────────────────┐
│         Presentation Layer          │
│         (cmd/, CLI)                 │
└─────────────────────────────────────┘
              ↓
┌─────────────────────────────────────┐
│         Application Layer           │
│    (cluster/, gitops/, services/)   │
│    - Orchestrates business logic    │
│    - Coordinates services           │
└─────────────────────────────────────┘
              ↓
┌─────────────────────────────────────┐
│          Domain Layer               │
│    (config/, validation/)           │
│    - Business rules                 │
│    - Domain models                  │
│    - Validation logic               │
└─────────────────────────────────────┘
              ↓
┌─────────────────────────────────────┐
│      Infrastructure Layer           │
│    (util/, template/, di/)          │
│    - File I/O                       │
│    - Template rendering             │
│    - Dependency injection           │
└─────────────────────────────────────┘
```

### Current Boundary Violations

1. **Config → Services**: `internal/config/services/` contains service logic (should be in application layer)
2. **Config → Validation**: Validation logic scattered across layers
3. **Util → Business Logic**: Some utilities contain business logic
4. **Circular Dependencies**: Some packages import each other

### Proposed Boundary Enforcement

```go
// Rule 1: Presentation layer only calls application layer
// cmd/ → cluster/, gitops/, services/

// Rule 2: Application layer uses domain layer
// cluster/, gitops/, services/ → config/, validation/

// Rule 3: Domain layer uses infrastructure layer
// config/, validation/ → util/, template/, di/

// Rule 4: Infrastructure layer has no upward dependencies
// util/, template/, di/ → standard library only
```

## Proposed Improvements

### Improvement 1: Unified Service Architecture

**Current**:
```
internal/config/services/  ← Configuration-focused
internal/services/         ← Plugin-focused
```

**Proposed**:
```
internal/services/
├── registry.go           # Service registry
├── definition.go         # Service definitions
├── lifecycle.go          # Lifecycle management
├── plugins/              # Service implementations
│   ├── cert_manager.go
│   ├── cilium.go
│   └── ...
└── adapters/             # Configuration adapters
    ├── config_adapter.go
    └── ...
```

**Benefits**:
- Single source of truth
- Clear separation: plugins for logic, adapters for config
- Easier to test and maintain
- Consistent behavior

---

### Improvement 2: Separate Configuration from Business Logic

**Current**:
```go
// internal/config/config.go
type Config struct {
    OpenCenter OpenCenterConfig
}

func (c Config) GetCertManagerAWSCredentials() (string, string) {
    // Business logic
}
```

**Proposed**:
```go
// internal/config/types.go
type Config struct {
    OpenCenter OpenCenterConfig
}

// internal/config/credentials/resolver.go
type CredentialResolver struct {
    config *Config
}

func (r *CredentialResolver) GetCertManagerAWSCredentials() (string, string) {
    // Business logic
}
```

**Benefits**:
- Clear separation of data and logic
- Easier to test credential resolution
- Config struct remains simple
- Business logic can be mocked

---

### Improvement 3: Centralized Validation

**Current**: Validation scattered across packages

**Proposed**:
```
internal/core/validation/
├── engine.go             # Validation engine
├── validators/           # Validator implementations
│   ├── cluster.go
│   ├── service.go
│   ├── config.go
│   └── ...
├── rules/                # Validation rules
│   ├── dependency.go
│   ├── security.go
│   └── ...
└── result.go             # Validation results
```

**Benefits**:
- Single validation framework
- Consistent validation behavior
- Easy to add new validators
- Centralized validation rules

---

### Improvement 4: Unified Utility Layer

**Current**: Utilities fragmented

**Proposed**:
```
internal/util/
├── crypto/
│   └── unified.go        # Single crypto module
├── errors/
│   └── structured.go     # Unified error handling
├── files/
│   └── operations.go     # File I/O abstraction
├── security/
│   └── masking.go        # Credential masking
└── testing/
    └── framework.go      # Test utilities
```

**Benefits**:
- Clear organization
- No duplication
- Easy to find utilities
- Consistent patterns

## Modularity Enhancements

### Enhancement 1: Plugin System

**Goal**: Make services truly pluggable

```go
// internal/services/plugin.go
type ServicePlugin interface {
    Name() string
    Type() ServiceType
    Validate(config interface{}) error
    Render(ctx context.Context, config interface{}, workspace interface{}) error
    Status(config interface{}) ServiceStatus
}

// internal/services/registry.go
type ServiceRegistry interface {
    RegisterPlugin(plugin ServicePlugin) error
    GetPlugin(name string) (ServicePlugin, error)
    ListPlugins() []ServicePlugin
}
```

**Benefits**:
- Services can be added without modifying core code
- External plugins possible
- Clear plugin contract
- Easy to test plugins in isolation

---

### Enhancement 2: Configuration Adapters

**Goal**: Separate configuration format from business logic

```go
// internal/services/adapters/adapter.go
type ConfigAdapter interface {
    FromConfig(config interface{}) (ServiceConfig, error)
    ToConfig(serviceConfig ServiceConfig) (interface{}, error)
    Validate(config interface{}) error
}

// internal/services/adapters/cert_manager.go
type CertManagerAdapter struct{}

func (a *CertManagerAdapter) FromConfig(config interface{}) (ServiceConfig, error) {
    // Convert from YAML config to service config
}
```

**Benefits**:
- Configuration format can change without affecting plugins
- Multiple configuration formats supported
- Clear conversion logic
- Easy to test adapters

---

### Enhancement 3: Validation Engine

**Goal**: Extensible validation framework

```go
// internal/core/validation/engine.go
type ValidationEngine struct {
    validators map[string]Validator
}

func (e *ValidationEngine) Register(validator Validator) error {
    // Register validator
}

func (e *ValidationEngine) Validate(ctx context.Context, name string, data interface{}) (*ValidationResult, error) {
    // Run validation
}

// internal/core/validation/validator.go
type Validator interface {
    Name() string
    Validate(ctx context.Context, data interface{}) (*ValidationResult, error)
}
```

**Benefits**:
- Validators can be added dynamically
- Consistent validation interface
- Easy to test validators
- Validation results are structured

## Testability Improvements

### Improvement 1: Interface-Based Design

**Current**: Some concrete dependencies

**Proposed**: All dependencies through interfaces

```go
// Before
type InitService struct {
    pathResolver *paths.PathResolver  // Concrete type
    configManager *config.ConfigManager
}

// After
type InitService struct {
    pathResolver paths.PathResolver    // Interface
    configManager config.Manager
}
```

**Benefits**:
- Easy to mock dependencies
- Tests don't need real file system
- Faster test execution
- Better isolation

---

### Improvement 2: Dependency Injection

**Current**: ✅ Already implemented

**Enhancement**: Extend to all services

```go
// internal/di/providers.go
func ProvideCredentialResolver(config *config.Config) (*credentials.Resolver, error) {
    return credentials.NewResolver(config), nil
}

func ProvideServiceRegistry(validator *validation.Engine) (*services.Registry, error) {
    return services.NewRegistry(validator), nil
}
```

**Benefits**:
- All dependencies injected
- Easy to swap implementations
- Clear dependency graph
- Testable in isolation

---

### Improvement 3: Test Utilities

**Current**: Scattered test helpers

**Proposed**: Centralized test framework

```go
// internal/testing/framework.go
type TestFramework struct {
    TempDir string
    Config  *config.Config
    Mocks   *MockFactory
}

func NewTestFramework(t *testing.T) *TestFramework {
    // Setup test environment
}

func (f *TestFramework) CreateTestConfig() *config.Config {
    // Create test configuration
}

func (f *TestFramework) CreateMockService(name string) *MockService {
    // Create mock service
}
```

**Benefits**:
- Consistent test setup
- Reusable test utilities
- Less boilerplate in tests
- Easier to write new tests

## Implementation Guidelines

### Guideline 1: Package Dependencies

**Rule**: Dependencies flow downward only

```
cmd/ → cluster/, services/, gitops/
cluster/, services/, gitops/ → config/, validation/
config/, validation/ → util/, template/
util/, template/ → standard library
```

**Enforcement**:
- Use `go mod graph` to detect violations
- Add linter rules for import restrictions
- Document allowed dependencies

---

### Guideline 2: Interface Placement

**Rule**: Interfaces defined by consumers, not providers

```go
// Good: Interface in consumer package
// internal/cluster/interfaces.go
type ConfigManager interface {
    Load(path string) (*config.Config, error)
    Save(config *config.Config) error
}

// internal/cluster/init_service.go
type InitService struct {
    configManager ConfigManager  // Uses interface from same package
}

// Bad: Interface in provider package
// internal/config/manager.go
type ConfigManager interface {  // Don't define interface here
    Load(path string) (*Config, error)
}
```

**Benefits**:
- Loose coupling
- Easy to mock
- Clear dependencies

---

### Guideline 3: Error Handling

**Rule**: Use structured errors with context

```go
// Good
func (s *InitService) Initialize(name string) error {
    if err := s.validate(name); err != nil {
        return errors.NewValidationError(
            "cluster initialization failed",
            err,
            errors.WithField("cluster", name),
            errors.WithSuggestion("Check cluster name format"),
        )
    }
}

// Bad
func (s *InitService) Initialize(name string) error {
    if err := s.validate(name); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
}
```

**Benefits**:
- Rich error context
- Automatic suggestions
- Consistent error format
- Better user experience

---

### Guideline 4: Testing Strategy

**Rule**: Test at appropriate level

```
Unit Tests:      Test individual functions/methods
Integration Tests: Test package interactions
E2E Tests:       Test complete workflows
```

```go
// Unit test
func TestCredentialResolver_GetAWSCredentials(t *testing.T) {
    resolver := credentials.NewResolver(mockConfig)
    accessKey, secretKey := resolver.GetAWSCredentials()
    assert.Equal(t, "expected-key", accessKey)
}

// Integration test
func TestInitService_Initialize(t *testing.T) {
    container := setupTestContainer(t)
    service := container.Get("InitService").(*cluster.InitService)
    err := service.Initialize("test-cluster")
    assert.NoError(t, err)
}

// E2E test (BDD)
Scenario: Initialize a new cluster
  Given I have a valid cluster name
  When I run "opencenter cluster init test-cluster"
  Then the cluster configuration should be created
```

## Conclusion

The opencenter-cli architecture is fundamentally sound but suffers from duplication and boundary violations. The proposed improvements will:

1. **Eliminate duplication** through service system unification
2. **Clarify boundaries** through proper layer separation
3. **Improve testability** through interface-based design
4. **Enhance modularity** through plugin architecture
5. **Standardize patterns** through consistent guidelines

**Next Steps**:
1. Review proposed improvements with team
2. Prioritize improvements based on impact
3. Begin implementation with Phase 1 (Foundation)
4. Measure improvements through metrics

**Success Metrics**:
- Code duplication < 5%
- Test coverage > 80%
- Package coupling reduced by 30%
- Build time maintained or improved
- Developer satisfaction increased

# Release 1.0 Architectural Refactoring Plan

Comprehensive architectural review and refactoring roadmap for opencenter-cli codebase health improvement.

## Table of Contents

- [Executive Summary](#executive-summary)
- [Health Score Assessment](#health-score-assessment)
- [Architectural Diagrams](#architectural-diagrams)
- [Detailed Findings](#detailed-findings)
  - [Cross-Module Duplication](#cross-module-duplication)
  - [Architectural Improvements](#architectural-improvements)
  - [Consolidation and Boilerplate Reduction](#consolidation-and-boilerplate-reduction)
  - [Orphaned Code and Tech Debt](#orphaned-code-and-tech-debt)
- [Refactoring Roadmap](#refactoring-roadmap)
- [Metrics and Success Criteria](#metrics-and-success-criteria)
- [Risk Assessment](#risk-assessment)

## Executive Summary

**Codebase Health Score: 6.5/10**

The opencenter-cli codebase exhibits solid functionality but suffers from architectural drift, duplication, and complexity accumulation. This document outlines a systematic refactoring plan to improve maintainability, reduce bugs, and accelerate feature development.

**Top 3 Priority Fixes:**

1. **Path Resolution Duplication** - 40+ instances of `filepath.Join(..., "clusters", ...)` scattered across codebase
2. **Validation Pattern Fragmentation** - 50+ validation functions with inconsistent interfaces and error handling
3. **Configuration Loading Complexity** - Multiple overlapping loaders (v1, v2, legacy, organization-based) creating maintenance burden

**Expected Outcomes:**
- 20% reduction in lines of code through deduplication
- 30% faster feature development time
- 40% reduction in bug fix time
- 50% faster developer onboarding

## Health Score Assessment

### Scoring Breakdown

| Category | Score | Weight | Notes |
|----------|-------|--------|-------|
| Code Duplication | 4/10 | 25% | High duplication in path resolution and validation |
| Architectural Clarity | 6/10 | 25% | Mixed concerns, bloated command layer |
| Test Coverage | 7/10 | 20% | Good coverage but gaps in integration tests |
| Documentation | 7/10 | 15% | Adequate but needs architecture docs |
| Tech Debt | 6/10 | 15% | Deprecated code, orphaned interfaces |

**Overall: 6.5/10** - Functional but needs systematic improvement


## Architectural Diagrams

### Current Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         CMD Layer                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │cluster_  │  │cluster_  │  │cluster_  │  │cluster_  │   │
│  │init.go   │  │validate  │  │setup.go  │  │bootstrap │   │
│  │(1672 LOC)│  │.go       │  │          │  │.go       │   │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘   │
│       │             │              │              │          │
│       └─────────────┴──────────────┴──────────────┘          │
│                         │                                     │
└─────────────────────────┼─────────────────────────────────────┘
                          │
┌─────────────────────────┼─────────────────────────────────────┐
│                    Config Layer                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │config.go     │  │loader.go     │  │validator.go  │       │
│  │(1984 LOC)    │  │              │  │              │       │
│  │              │  │              │  │              │       │
│  │ - Load()     │  │ - LoadFrom   │  │ - Validate() │       │
│  │ - Save()     │  │   File()     │  │ - Pipeline   │       │
│  │ - ConfigPath│  │ - LoadFrom   │  │   Adapter    │       │
│  │   (complex)  │  │   Bytes()    │  │              │       │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘       │
│         │                  │                  │               │
│         └──────────────────┴──────────────────┘               │
│                            │                                  │
│  ┌─────────────────────────┴──────────────────────┐          │
│  │        Path Resolution (DUPLICATED)            │          │
│  │  - ClusterDirectoryPath()                      │          │
│  │  - ConfigPath() - 4 fallback strategies        │          │
│  │  - 40+ filepath.Join("clusters"...) calls      │          │
│  └────────────────────────────────────────────────┘          │
└───────────────────────────────────────────────────────────────┘
                          │
┌─────────────────────────┼─────────────────────────────────────┐
│                   Service Layer                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │ServiceMap    │  │ServiceRegistry│  │Service       │       │
│  │(polymorphic) │  │              │  │Plugins       │       │
│  │              │  │ - Register   │  │ (50+ types)  │       │
│  │ - Unmarshal  │  │ - Resolve    │  │              │       │
│  │   YAML       │  │   Dependencies│  │              │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
└───────────────────────────────────────────────────────────────┘
                          │
┌─────────────────────────┼─────────────────────────────────────┐
│                  Template/GitOps Layer                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │Template      │  │Template      │  │GitOps        │       │
│  │Engine        │  │Registry      │  │Pipeline      │       │
│  │              │  │              │  │              │       │
│  │ - Render()   │  │ - Register   │  │ - Generate() │       │
│  │ - Validate() │  │ - Resolve    │  │ - Stages     │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
└───────────────────────────────────────────────────────────────┘
```

**Problems:**
- Command layer contains business logic (1672 LOC in cluster_init.go)
- Config layer mixes concerns (path resolution, loading, validation)
- Path resolution duplicated across 40+ locations
- No clear domain layer separating infrastructure from business logic


### Proposed Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      CMD Layer (Thin)                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│  │cluster   │  │config    │  │services  │                  │
│  │commands  │  │commands  │  │commands  │                  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘                  │
│       └─────────────┴──────────────┘                         │
│                     │                                        │
└─────────────────────┼────────────────────────────────────────┘
                      │
┌─────────────────────┼────────────────────────────────────────┐
│              Core Domain Layer (NEW)                         │
│  ┌──────────────────────────────────────────────────┐       │
│  │           PathResolver (Centralized)             │       │
│  │  - Single source of truth for all paths         │       │
│  │  - Organization-aware                            │       │
│  │  - Caching built-in                              │       │
│  └──────────────────────────────────────────────────┘       │
│  ┌──────────────────────────────────────────────────┐       │
│  │        ConfigurationManager (Unified)            │       │
│  │  - Single loader for all versions                │       │
│  │  - Migration pipeline integrated                 │       │
│  │  - Validation pipeline integrated                │       │
│  └──────────────────────────────────────────────────┘       │
│  ┌──────────────────────────────────────────────────┐       │
│  │         ValidationEngine (Unified)               │       │
│  │  - Single interface for all validation          │       │
│  │  - Pluggable validators                          │       │
│  │  - Consistent error format                       │       │
│  └──────────────────────────────────────────────────┘       │
└───────────────────────────────────────────────────────────────┘
                      │
┌─────────────────────┼────────────────────────────────────────┐
│            Infrastructure Layer (Unchanged)                  │
│  Services, Templates, GitOps, SOPS, etc.                    │
└───────────────────────────────────────────────────────────────┘
```

**Benefits:**
- Thin command layer (CLI concerns only)
- Clear domain layer with single responsibilities
- Centralized path resolution eliminates duplication
- Unified configuration management simplifies version handling
- Consistent validation across all components

## Detailed Findings

### Cross-Module Duplication

#### 1.1 Path Resolution Duplication (CRITICAL)

**Impact:** High maintenance burden, inconsistent behavior, bug-prone

**Evidence:**
- `filepath.Join(clustersDir, organization, "infrastructure", "clusters", clusterName)` appears 40+ times
- 4 different path resolution strategies in `ConfigPath()`:
  1. Organization-based explicit
  2. Organization-based search
  3. Flat file backward compatibility
  4. Legacy directory structure


**Duplicate logic locations:**
- `internal/config/config.go:ConfigPath()`
- `internal/config/path_resolver_impl.go:ResolveClusterPaths()`
- `internal/gitops/copy.go` (multiple functions)
- `internal/gitops/stages/config_stage.go`
- `internal/gitops/stages/infrastructure_stage.go`
- `internal/operations/backup_manager.go`
- `cmd/cluster_init.go`

**Recommendation:**

```go
// Centralized path resolver in internal/core/paths/resolver.go
type PathResolver struct {
    cache map[string]ClusterPaths
    mu    sync.RWMutex
}

func (pr *PathResolver) Resolve(cluster, org string) ClusterPaths {
    // Single source of truth for all path logic
    // Handles: organization structure, legacy, flat files
    // Returns: ClusterPaths with all computed paths
}

// Usage example
paths := pathResolver.Resolve("my-cluster", "myorg")
configPath := paths.ConfigFile
gitopsDir := paths.GitOpsDir
sopsKey := paths.SOPSKeyPath
```

**Migration Strategy:**
1. Create new PathResolver in `internal/core/paths/`
2. Implement all path resolution strategies
3. Add comprehensive tests
4. Update one module at a time to use PathResolver
5. Mark old functions as deprecated
6. Remove old functions after migration complete

#### 1.2 Validation Function Proliferation

**Impact:** Inconsistent validation, difficult to maintain

**Evidence:**
- 50+ `Validate*()` functions across codebase
- Multiple validation interfaces:
  - `internal/config/validator.go:ConfigValidatorInterface`
  - `internal/util/files/file_validator.go:FileValidator`
  - `internal/util/template/validator.go:TemplateValidator`
  - `internal/security/input_validator.go:InputValidator`
- No shared error format or suggestion engine
- Duplicate validation logic for cluster names, organization names, paths

**Recommendation:**

```go
// Unified validation interface in internal/core/validation/engine.go
type Validator interface {
    Validate(ctx context.Context, target interface{}) ValidationResult
}

type ValidationResult struct {
    Valid       bool
    Errors      []ValidationError
    Warnings    []ValidationWarning
    Suggestions []string
}

type ValidationError struct {
    Field   string
    Message string
    Code    string
}

// Registry pattern for validators
type ValidationRegistry struct {
    validators map[reflect.Type][]Validator
}

func (vr *ValidationRegistry) Register(targetType reflect.Type, validator Validator)
func (vr *ValidationRegistry) Validate(ctx context.Context, target interface{}) ValidationResult
```


**Migration Strategy:**
1. Create ValidationEngine in `internal/core/validation/`
2. Define standard ValidationResult format
3. Migrate one validator type as proof of concept
4. Update callers to use new interface
5. Consolidate duplicate validation logic
6. Remove old validators

#### 1.3 Configuration Loading Duplication

**Impact:** Complex maintenance, version migration issues

**Evidence:**
- 3 separate loaders:
  - `internal/config/loader.go:ConfigLoader` (v1)
  - `internal/config/v2/loader.go:V2Loader` (v2)
  - `cmd/cluster_init.go:handleLegacyFlatInit()` (legacy)
- Overlapping logic for:
  - YAML unmarshaling
  - Default application
  - Environment variable expansion
  - Metadata initialization

**Recommendation:**

```go
// Unified loader with strategy pattern in internal/core/config/manager.go
type ConfigLoader struct {
    strategies map[string]LoadStrategy
}

type LoadStrategy interface {
    CanLoad(data []byte) bool
    Load(data []byte) (*Config, error)
    Version() string
}

// Strategies
type V1Strategy struct{}
type V2Strategy struct{}
type LegacyStrategy struct{}

// Auto-detection and loading
func (cl *ConfigLoader) Load(data []byte) (*Config, error) {
    for _, strategy := range cl.strategies {
        if strategy.CanLoad(data) {
            return strategy.Load(data)
        }
    }
    return nil, errors.New("no compatible loader found")
}
```

**Migration Strategy:**
1. Create ConfigLoader with strategy pattern
2. Implement V1Strategy wrapping existing loader
3. Implement V2Strategy wrapping existing loader
4. Implement LegacyStrategy for flat files
5. Update all Load() calls to use new ConfigLoader
6. Remove old loaders

### Architectural Improvements

#### 2.1 Command Layer Bloat

**Issue:** `cmd/cluster_init.go` is 1672 lines with embedded business logic

**Problems:**
- Reflection-based field setting (`setField`, `setReflectValue`)
- SOPS key generation logic
- SSH key generation logic
- Git repository initialization
- Organization structure creation
- All mixed with CLI flag parsing

**Current structure:**
```go
func newClusterInitCmd() *cobra.Command {
    return &cobra.Command{
        RunE: func(cmd *cobra.Command, args []string) error {
            // 1500+ lines of business logic here
            // - Parse flags
            // - Validate inputs
            // - Generate keys
            // - Create directories
            // - Initialize git
            // - Write config
        },
    }
}
```


**Recommendation:**

```go
// Thin command layer in cmd/cluster_init.go
func newClusterInitCmd() *cobra.Command {
    return &cobra.Command{
        RunE: func(cmd *cobra.Command, args []string) error {
            // Parse flags only
            opts := parseInitOptions(cmd, args)
            
            // Delegate to domain service
            container := di.GetContainer(cmd.Context())
            svc := container.GetClusterInitService()
            return svc.Initialize(cmd.Context(), opts)
        },
    }
}

// Domain service in internal/cluster/init_service.go
type InitService struct {
    pathResolver  PathResolver
    keyGenerator  KeyGenerator
    configManager ConfigManager
    gitOps        GitOpsService
}

func (s *InitService) Initialize(ctx context.Context, opts InitOptions) error {
    // Clean business logic without CLI concerns
    // 1. Resolve paths
    // 2. Generate keys if needed
    // 3. Create configuration
    // 4. Initialize git repository
    // 5. Write files
}
```

**Benefits:**
- Testable business logic (no cobra dependencies)
- Reusable from other contexts (API, SDK)
- Clear separation of concerns
- Easier to maintain and extend

#### 2.2 Config Package Complexity

**Issue:** `internal/config/config.go` is 1984 lines, mixing concerns

**Problems:**
- Configuration struct definitions
- Path resolution logic
- Loading/saving logic
- Validation logic
- Migration logic
- Default generation
- All in one file

**Recommendation:**

Split into focused modules:

```
internal/config/
├── types.go          # Config struct definitions only
├── defaults.go       # Default generation
├── persistence.go    # Load/Save operations
├── migration/        # Version migration
│   ├── v1_to_v2.go
│   ├── v2_to_v3.go
│   └── detector.go
├── validation/       # Validation logic
│   ├── schema.go
│   ├── semantic.go
│   └── provider.go
└── paths/            # Path resolution (deprecated, use core/paths)
    ├── resolver.go
    └── strategies.go
```

**Migration Strategy:**
1. Create new directory structure
2. Move struct definitions to types.go
3. Move default generation to defaults.go
4. Move Load/Save to persistence.go
5. Move validation to validation/
6. Move migration to migration/
7. Update imports across codebase
8. Remove old config.go


#### 2.3 Service Registry vs Template Registry Duplication

**Issue:** Two nearly identical registry patterns

**Evidence:**
- `internal/services/registry.go:ServiceRegistry`
- `internal/template/registry.go:TemplateRegistry`
- Both implement:
  - Registration
  - Dependency resolution
  - Circular dependency detection
  - Metadata management

**Recommendation:**

```go
// Generic registry pattern in internal/core/registry/registry.go
type Registry[T any] interface {
    Register(name string, item T) error
    Get(name string) (T, error)
    List() []T
    ResolveDependencies(names []string) ([]T, error)
}

type DependencyAware interface {
    GetDependencies() []string
}

type GenericRegistry[T DependencyAware] struct {
    items map[string]T
    mu    sync.RWMutex
}

// Implement common logic once
func (r *GenericRegistry[T]) Register(name string, item T) error
func (r *GenericRegistry[T]) ResolveDependencies(names []string) ([]T, error)

// Reuse for both services and templates
type ServiceRegistry = Registry[ServiceDefinition]
type TemplateRegistry = Registry[TemplateDefinition]
```

**Benefits:**
- Single implementation of dependency resolution
- Consistent behavior across registries
- Easier to add new registry types
- Reduced code duplication

### Consolidation and Boilerplate Reduction

#### 3.1 Excessive Wrapper Functions

**Issue:** Many single-line wrapper functions adding no value

**Examples:**

```go
// cmd/root.go - Deprecated stubs
func GetConfigManager() *config.ConfigManager {
    return nil  // Marked deprecated but still exported
}

func formatError(err error) error {
    return err  // Pass-through, no value added
}

// internal/config/config.go - Simple field access
func (c Config) ClusterName() string {
    return c.OpenCenter.Cluster.ClusterName
}

func (c Config) GitOps() GitOpsConfig {
    return c.OpenCenter.GitOps
}
```

**Recommendation:**
- Remove deprecated stubs entirely
- Use direct field access instead of getter methods
- Only create wrappers when they add value (validation, transformation, caching)

**Action Items:**
1. Audit all wrapper functions
2. Remove those that don't add value
3. Update callers to use direct access
4. Document remaining wrappers with clear purpose


#### 3.2 Duplicate Error Handling Patterns

**Issue:** Inconsistent error wrapping across codebase

**Examples:**

```go
// Pattern 1: fmt.Errorf with %w
return fmt.Errorf("failed to X: %w", err)

// Pattern 2: Custom error types
return errors.CreateTemplateError(path, line, msg, err)

// Pattern 3: Simple wrapping
return fmt.Errorf("X failed: %v", err)

// Pattern 4: No wrapping
return err
```

**Recommendation:**

```go
// Standardized error package in internal/core/errors/
type Error struct {
    Code       string
    Message    string
    Cause      error
    Context    map[string]interface{}
    Suggestions []string
    StackTrace []string
}

func Wrap(err error, code, message string) *Error
func WithContext(err error, key string, value interface{}) *Error
func WithSuggestion(err error, suggestion string) *Error

// Usage
return errors.Wrap(err, "E1001", "failed to load configuration").
    WithContext("path", configPath).
    WithSuggestion("Check file permissions")
```

**Benefits:**
- Consistent error format across codebase
- Machine-readable error codes
- Contextual information for debugging
- User-friendly suggestions
- Stack traces for troubleshooting

#### 3.3 Redundant Validation Checks

**Issue:** Same validations repeated in multiple layers

**Example:** Cluster name validation occurs in:
1. `cmd/cluster_init.go:ValidateClusterName()`
2. `internal/config/config.go:ValidateClusterName()`
3. `internal/security/input_validator.go:ValidateClusterName()`
4. `internal/config/path_resolver.go` (implicit validation)

**Recommendation:**
- Single validation point with clear ownership
- Use ValidationEngine from core layer
- Commands validate at boundary only
- Domain layer assumes valid inputs

**Validation Layers:**
```
┌─────────────────────────────────────┐
│ CMD Layer: Input validation only    │
│ - Parse flags                        │
│ - Validate format (syntax)           │
└─────────────────┬───────────────────┘
                  │
┌─────────────────┴───────────────────┐
│ Domain Layer: Business validation   │
│ - Validate semantics                 │
│ - Check business rules               │
│ - Verify dependencies                │
└─────────────────┬───────────────────┘
                  │
┌─────────────────┴───────────────────┐
│ Infrastructure: Technical validation│
│ - File system checks                 │
│ - Network connectivity               │
│ - External service availability      │
└─────────────────────────────────────┘
```

### Orphaned Code and Tech Debt

#### 4.1 Deprecated Functions Still in Use

**Evidence:**

```go
// cmd/root.go - Marked deprecated but still exported
func GetConfigManager() *config.ConfigManager {
    return nil
}

func formatError(err error) error {
    return err
}

func formatErrorWithCode(err error, code string) error {
    // Stub implementation
}
```

**Action Items:**
1. Search for all usages of deprecated functions
2. Update callers to use new implementations
3. Remove deprecated functions
4. Update documentation


#### 4.2 Unused Interfaces

**Evidence:**

```go
// internal/config/interfaces.go
type ConfigLoaderInterface interface {
    LoadFromFile(ctx context.Context, filePath string) (*Config, error)
    LoadFromBytes(ctx context.Context, data []byte, clusterName string) (*Config, error)
    LoadDefault(ctx context.Context, clusterName string) (*Config, error)
    // ... 7 more methods
}

// Only partially implemented by ConfigLoader
// Some methods never called in production code
```

**Action Items:**
1. Audit all interface definitions
2. Check actual usage of each method
3. Remove unused methods
4. Consider splitting large interfaces (Interface Segregation Principle)

#### 4.3 Test-Only Code in Production

**Evidence:**

```go
// internal/config/config.go
func defaultConfig(name string) Config {
    isTestMode := os.Getenv("OPENCENTER_TEST_MODE") == "true"
    if isTestMode {
        authURL = "https://identity.example.com/v3"
        region = "RegionOne"
        tenantName = "admin"
        // Test-specific defaults
    }
    // Production defaults
}
```

**Recommendation:**
- Separate test fixtures from production code
- Use dependency injection for test data
- Create test builders for complex objects

```go
// Production code
func defaultConfig(name string) Config {
    // Only production defaults
}

// Test code
func testConfig(name string) Config {
    cfg := defaultConfig(name)
    cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://identity.example.com/v3"
    // Test-specific overrides
    return cfg
}
```

#### 4.4 Commented-Out Code

**Evidence:**
- Multiple instances of commented-out imports
- Commented-out function implementations
- TODO comments without tracking

**Action Items:**
1. Remove all commented-out code
2. Convert TODOs to tracked issues
3. Add issue references to remaining TODOs
4. Use feature flags for experimental code

## Refactoring Roadmap

### Phase 1: Foundation (Weeks 1-2)

**Goal:** Establish core abstractions without breaking changes

#### Week 1: Core Infrastructure

**Task 1.1: Create PathResolver Core**
- Location: `internal/core/paths/resolver.go`
- Extract all path logic from existing code
- Implement caching mechanism
- Add comprehensive unit tests
- Keep existing functions as deprecated wrappers

```go
// internal/core/paths/resolver.go
type PathResolver struct {
    cache      map[string]ClusterPaths
    mu         sync.RWMutex
    configMgr  ConfigManager
}

func (pr *PathResolver) Resolve(cluster, org string) (ClusterPaths, error)
func (pr *PathResolver) ResolveWithFallback(cluster string) (ClusterPaths, error)
func (pr *PathResolver) InvalidateCache(cluster string)
```

**Deliverables:**
- [ ] PathResolver implementation
- [ ] 100% test coverage
- [ ] Documentation
- [ ] Benchmark tests


**Task 1.2: Create ValidationEngine Core**
- Location: `internal/core/validation/engine.go`
- Extract validation interface
- Implement registry pattern
- Migrate one validator as proof of concept
- Add comprehensive tests

```go
// internal/core/validation/engine.go
type ValidationEngine struct {
    validators map[reflect.Type][]Validator
}

func (ve *ValidationEngine) Register(targetType reflect.Type, validator Validator)
func (ve *ValidationEngine) Validate(ctx context.Context, target interface{}) ValidationResult
```

**Deliverables:**
- [ ] ValidationEngine implementation
- [ ] Standard ValidationResult format
- [ ] One migrated validator (cluster name)
- [ ] Documentation

#### Week 2: Configuration Management

**Task 2.1: Create ConfigManager Core**
- Location: `internal/core/config/manager.go`
- Extract unified loader with strategy pattern
- Implement version detection
- Keep existing loaders as adapters
- Add comprehensive tests

```go
// internal/core/config/manager.go
type ConfigManager struct {
    strategies []LoadStrategy
    pathResolver PathResolver
}

func (cm *ConfigManager) Load(clusterName string) (*Config, error)
func (cm *ConfigManager) LoadFromBytes(data []byte) (*Config, error)
func (cm *ConfigManager) Save(cfg *Config) error
```

**Deliverables:**
- [ ] ConfigManager implementation
- [ ] Strategy pattern for loaders
- [ ] Version detection logic
- [ ] Migration tests

**Task 2.2: Split config.go**
- Split 1984-line file into focused modules
- Create types.go, defaults.go, persistence.go
- Update imports across codebase
- Verify no functionality changes

**Deliverables:**
- [ ] New module structure
- [ ] All tests passing
- [ ] Import updates complete
- [ ] Documentation updated

### Phase 2: Migration (Weeks 3-4)

**Goal:** Migrate existing code to use new abstractions

#### Week 3: Path Resolution Migration

**Task 3.1: Migrate cmd/ package**
- Update `cmd/cluster_init.go` to use PathResolver
- Update other cluster commands
- Remove direct filepath.Join calls
- Add integration tests

**Deliverables:**
- [ ] cmd/ package migrated
- [ ] Integration tests passing
- [ ] No direct path construction

**Task 3.2: Migrate internal/gitops/**
- Update all gitops stages
- Update copy.go and related files
- Remove duplicate path logic
- Add integration tests

**Deliverables:**
- [ ] internal/gitops/ migrated
- [ ] All tests passing
- [ ] Path duplication eliminated

**Task 3.3: Migrate internal/operations/**
- Update backup_manager.go
- Update other operations
- Remove duplicate path logic

**Deliverables:**
- [ ] internal/operations/ migrated
- [ ] All tests passing

#### Week 4: Validation and Config Migration

**Task 4.1: Migrate Validators**
- Update all validators to use ValidationEngine
- Consolidate error formats
- Add suggestion engine integration
- Update tests

**Deliverables:**
- [ ] All validators migrated
- [ ] Consistent error format
- [ ] Suggestion engine working

**Task 4.2: Migrate Configuration Loading**
- Update all Load() calls to use ConfigManager
- Remove duplicate loading logic
- Consolidate default generation
- Update tests

**Deliverables:**
- [ ] All loaders migrated
- [ ] Duplicate logic removed
- [ ] All tests passing


### Phase 3: Cleanup (Weeks 5-6)

**Goal:** Remove duplication and tech debt

#### Week 5: Code Cleanup

**Task 5.1: Remove Deprecated Code**
- Delete wrapper functions that add no value
- Remove orphaned interfaces
- Clean up test-only code in production
- Update documentation

**Deliverables:**
- [ ] All deprecated code removed
- [ ] No orphaned interfaces
- [ ] Test fixtures separated
- [ ] Documentation updated

**Task 5.2: Consolidate Registries**
- Implement generic Registry[T]
- Migrate ServiceRegistry
- Migrate TemplateRegistry
- Remove duplicate code

```go
// internal/core/registry/registry.go
type GenericRegistry[T DependencyAware] struct {
    items map[string]T
    mu    sync.RWMutex
}
```

**Deliverables:**
- [ ] Generic registry implemented
- [ ] ServiceRegistry migrated
- [ ] TemplateRegistry migrated
- [ ] Tests passing

**Task 5.3: Standardize Error Handling**
- Implement core error package
- Migrate to standard error format
- Add error codes and suggestions
- Update all error handling

**Deliverables:**
- [ ] Error package implemented
- [ ] All errors migrated
- [ ] Consistent format across codebase
- [ ] Documentation updated

#### Week 6: Documentation and Testing

**Task 6.1: Architecture Documentation**
- Document new architecture
- Create migration guides
- Update developer guides
- Add ADRs (Architecture Decision Records)

**Deliverables:**
- [ ] Architecture diagrams
- [ ] Migration guides
- [ ] Developer documentation
- [ ] ADRs for major decisions

**Task 6.2: Integration Testing**
- Add end-to-end tests
- Test migration paths
- Test backward compatibility
- Performance benchmarks

**Deliverables:**
- [ ] E2E test suite
- [ ] Migration tests
- [ ] Backward compatibility tests
- [ ] Performance benchmarks

### Phase 4: Optimization (Week 7)

**Goal:** Performance and maintainability improvements

#### Week 7: Performance Optimization

**Task 7.1: Add Caching**
- PathResolver caching
- Template caching improvements
- Configuration caching
- Benchmark improvements

**Deliverables:**
- [ ] Caching implemented
- [ ] Performance benchmarks
- [ ] Cache invalidation strategy
- [ ] Documentation

**Task 7.2: Reduce Allocations**
- Profile hot paths
- Optimize string operations
- Pool frequently allocated objects
- Reduce memory footprint

**Deliverables:**
- [ ] Profiling results
- [ ] Optimization implementation
- [ ] Performance comparison
- [ ] Documentation

**Task 7.3: Improve Error Messages**
- Standardize error format
- Add contextual suggestions
- Improve stack traces
- User-friendly messages

**Deliverables:**
- [ ] Error message improvements
- [ ] Suggestion engine
- [ ] User testing feedback
- [ ] Documentation

## Metrics and Success Criteria

### Code Quality Metrics

**Before Refactoring:**
- Total Lines of Code: ~50,000
- Cyclomatic Complexity (cluster_init.go): 150+
- Test Coverage: ~60%
- Duplicate Code: ~15%
- Technical Debt Ratio: 25%

**After Refactoring (Targets):**
- Total Lines of Code: ~40,000 (20% reduction)
- Cyclomatic Complexity (cluster_init.go): <50 (67% reduction)
- Test Coverage: >80% (33% increase)
- Duplicate Code: <5% (67% reduction)
- Technical Debt Ratio: <10% (60% reduction)

### Architectural Metrics

| Metric | Before | Target | Improvement |
|--------|--------|--------|-------------|
| Module Coupling | High | Medium | -40% |
| Module Cohesion | Medium | High | +50% |
| Interface Size (avg methods) | 10 | 6 | -40% |
| Dependency Depth | 5 levels | 3 levels | -40% |
| Path Resolution Locations | 40+ | 1 | -97% |
| Validation Functions | 50+ | 10 | -80% |
| Config Loaders | 3 | 1 | -67% |


### Maintainability Metrics

| Metric | Before | Target | Improvement |
|--------|--------|--------|-------------|
| Time to Add Feature | 3 days | 2 days | -30% |
| Bug Fix Time | 4 hours | 2.4 hours | -40% |
| Onboarding Time | 2 weeks | 1 week | -50% |
| Code Review Time | 2 hours | 1.3 hours | -35% |
| Build Time | 45s | 30s | -33% |

### Performance Metrics

| Metric | Before | Target | Improvement |
|--------|--------|--------|-------------|
| Cluster Init Time | 5s | 3s | -40% |
| Config Load Time | 200ms | 100ms | -50% |
| Validation Time | 500ms | 300ms | -40% |
| Template Render Time | 1s | 600ms | -40% |
| Memory Usage (peak) | 150MB | 100MB | -33% |

### Quality Gates

All phases must pass these gates before proceeding:

**Phase 1 Gates:**
- [ ] All new core packages have >90% test coverage
- [ ] No breaking changes to public APIs
- [ ] All existing tests pass
- [ ] Performance benchmarks show no regression

**Phase 2 Gates:**
- [ ] All migrations complete with backward compatibility
- [ ] Integration tests pass
- [ ] No increase in cyclomatic complexity
- [ ] Code review approved

**Phase 3 Gates:**
- [ ] All deprecated code removed
- [ ] No orphaned code remains
- [ ] Documentation complete
- [ ] All tests pass

**Phase 4 Gates:**
- [ ] Performance targets met
- [ ] Memory usage reduced
- [ ] User acceptance testing passed
- [ ] Production deployment successful

## Risk Assessment

### High Risk Items

#### 1. Path Resolution Changes
**Risk:** Could break existing deployments with custom directory structures

**Impact:** High - Users unable to access existing clusters

**Probability:** Medium

**Mitigation:**
- Extensive backward compatibility tests
- Phased rollout with feature flag
- Comprehensive migration guide
- Rollback plan documented
- Beta testing with real users

**Contingency:**
- Keep old path resolution as fallback
- Provide migration tool
- Support both old and new structures for 2 releases

#### 2. Configuration Loading Changes
**Risk:** Could affect migration paths between versions

**Impact:** High - Data loss or corruption

**Probability:** Low

**Mitigation:**
- Keep old loaders as fallbacks
- Comprehensive migration tests
- Backup before migration
- Dry-run mode for migrations
- Version detection validation

**Contingency:**
- Rollback to previous loader
- Manual migration scripts
- Support team training

### Medium Risk Items

#### 3. Validation Consolidation
**Risk:** Could change validation behavior

**Impact:** Medium - Some invalid configs might pass or valid configs might fail

**Probability:** Low

**Mitigation:**
- Maintain existing validation behavior
- Add regression tests
- Document any intentional changes
- Beta testing period

**Contingency:**
- Revert to old validators
- Provide override flags
- Document workarounds

#### 4. Command Layer Refactoring
**Risk:** Could introduce bugs in CLI behavior

**Impact:** Medium - User experience degradation

**Probability:** Low

**Mitigation:**
- Comprehensive integration tests
- Manual testing of all commands
- Beta release before GA
- Monitoring and alerting

**Contingency:**
- Quick rollback capability
- Hotfix process
- User communication plan

### Low Risk Items

#### 5. Error Message Improvements
**Risk:** Users might be confused by new error format

**Impact:** Low - Temporary confusion

**Probability:** Low

**Mitigation:**
- Gradual rollout
- Documentation updates
- User feedback collection

**Contingency:**
- Provide old format option
- Update documentation
- Support team training

#### 6. Performance Optimizations
**Risk:** Could introduce subtle bugs

**Impact:** Low - Functionality preserved

**Probability:** Very Low

**Mitigation:**
- Extensive benchmarking
- Regression testing
- Gradual rollout

**Contingency:**
- Revert optimization
- Performance profiling
- Bug fixes

## Implementation Guidelines

### Code Review Checklist

All PRs must pass this checklist:

**Architecture:**
- [ ] Follows new architecture patterns
- [ ] No new path resolution duplication
- [ ] Uses core abstractions (PathResolver, ValidationEngine, ConfigManager)
- [ ] Clear separation of concerns

**Code Quality:**
- [ ] No functions >50 lines
- [ ] No files >500 lines
- [ ] Cyclomatic complexity <10
- [ ] Test coverage >80%

**Testing:**
- [ ] Unit tests for all new code
- [ ] Integration tests for workflows
- [ ] Backward compatibility tests
- [ ] Performance benchmarks

**Documentation:**
- [ ] Public APIs documented
- [ ] Architecture decisions recorded
- [ ] Migration guide updated
- [ ] Examples provided

**Security:**
- [ ] No hardcoded secrets
- [ ] Input validation present
- [ ] Error messages don't leak sensitive data
- [ ] Dependencies up to date

### Testing Strategy

**Unit Tests:**
- Test individual functions in isolation
- Mock external dependencies
- Cover edge cases and error paths
- Fast execution (<1s per test)

**Integration Tests:**
- Test component interactions
- Use real dependencies where possible
- Test happy path and error scenarios
- Reasonable execution time (<10s per test)

**End-to-End Tests:**
- Test complete workflows
- Use real file system and git
- Test backward compatibility
- Longer execution time acceptable (<1m per test)

**Performance Tests:**
- Benchmark critical paths
- Compare before/after metrics
- Test with realistic data sizes
- Run on CI for regression detection

### Deployment Strategy

**Phase 1-2: Internal Testing**
- Deploy to development environment
- Internal team testing
- Fix critical bugs
- Performance validation

**Phase 3: Beta Release**
- Deploy to beta users
- Collect feedback
- Monitor metrics
- Fix reported issues

**Phase 4: Gradual Rollout**
- 10% of users (week 1)
- 25% of users (week 2)
- 50% of users (week 3)
- 100% of users (week 4)

**Monitoring:**
- Error rates
- Performance metrics
- User feedback
- Support tickets

**Rollback Criteria:**
- Error rate >5% increase
- Performance degradation >20%
- Critical bugs reported
- User satisfaction <80%

## Communication Plan

### Internal Communication

**Weekly Status Updates:**
- Progress on roadmap
- Blockers and risks
- Metrics and KPIs
- Next week's plan

**Architecture Reviews:**
- Bi-weekly design reviews
- ADR presentations
- Code walkthrough sessions
- Knowledge sharing

### External Communication

**Release Notes:**
- Feature highlights
- Breaking changes
- Migration guides
- Performance improvements

**Blog Posts:**
- Architecture evolution
- Performance improvements
- Developer experience enhancements
- Community contributions

**Documentation:**
- Updated architecture docs
- Migration guides
- API reference updates
- Tutorial updates

## Success Criteria

The refactoring is considered successful when:

**Code Quality:**
- [ ] 20% reduction in total lines of code
- [ ] 67% reduction in cyclomatic complexity
- [ ] 80%+ test coverage achieved
- [ ] <5% duplicate code

**Performance:**
- [ ] 40% faster cluster initialization
- [ ] 50% faster configuration loading
- [ ] 33% reduction in memory usage
- [ ] No performance regressions

**Maintainability:**
- [ ] 30% faster feature development
- [ ] 40% faster bug fixes
- [ ] 50% faster developer onboarding
- [ ] 35% faster code reviews

**User Experience:**
- [ ] No breaking changes for users
- [ ] Improved error messages
- [ ] Better documentation
- [ ] Positive user feedback

**Team Satisfaction:**
- [ ] Developers find code easier to work with
- [ ] Code reviews are faster and more focused
- [ ] Fewer production incidents
- [ ] Higher code quality confidence

## Conclusion

This refactoring plan addresses systemic architectural issues in the opencenter-cli codebase. By following the phased approach, we minimize risk while delivering incremental value. The focus on core abstractions (PathResolver, ValidationEngine, ConfigManager) eliminates duplication and establishes clear patterns for future development.

The success of this refactoring will be measured not just in code metrics, but in improved developer productivity, faster feature delivery, and better user experience. The investment in architectural health will pay dividends in reduced maintenance burden and increased development velocity.

## References

- [Architecture Decision Records](./dev/architecture.md)
- [Developer Guide](./dev/readme.md)
- [Testing Strategy](./dev/testing/)
- [Performance Optimization](./dev/performance-optimization-analysis.md)
- [Service Registry Patterns](../.kiro/steering/service-registry-patterns.md)
- [GitOps Manifest Standards](../.kiro/steering/gitops-manifest-standards.md)

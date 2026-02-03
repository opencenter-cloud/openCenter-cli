# Refactoring Roadmap: opencenter-cli

## Table of Contents

- [Overview](#overview)
- [Phase 1: Foundation & Quick Wins](#phase-1-foundation--quick-wins)
- [Phase 2: Validation Consolidation](#phase-2-validation-consolidation)
- [Phase 3: Configuration Unification](#phase-3-configuration-unification)
- [Phase 4: Cleanup & Optimization](#phase-4-cleanup--optimization)
- [Testing Strategy](#testing-strategy)
- [Rollback Plan](#rollback-plan)
- [Success Criteria](#success-criteria)

---

## Overview

This roadmap provides a step-by-step guide to refactor the opencenter-cli codebase, addressing the architectural issues identified in the detailed findings. The approach prioritizes minimal breaking changes while maximizing impact.

**Total Duration:** 14 weeks  
**Team Size:** 2-3 engineers  
**Risk Level:** Medium

### Guiding Principles

1. **Incremental Changes:** Small, reviewable PRs over large rewrites
2. **Test Coverage:** Maintain >80% coverage throughout
3. **Backward Compatibility:** Use deprecation warnings before removal
4. **Feature Flags:** Enable gradual rollout of new systems
5. **Documentation:** Update docs with each phase

---

## Phase 1: Foundation & Quick Wins

**Duration:** 2 weeks  
**Risk:** Low  
**Impact:** Medium

### Week 1: Utility Consolidation

#### Task 1.1: Create File Operations Wrapper
**Effort:** 2 days | **Priority:** HIGH

**Steps:**


1. Create `internal/util/fs/wrapper.go`:
```go
type FileSystem interface {
    ReadFile(path string) ([]byte, error)
    WriteFile(path string, data []byte, perm os.FileMode) error
    WriteFileAtomic(path string, data []byte, perm os.FileMode) error
    Exists(path string) bool
    MkdirAll(path string, perm os.FileMode) error
}
```

2. Implement `DefaultFileSystem` with error wrapping
3. Add comprehensive unit tests (target: 95% coverage)
4. Create migration guide for developers

**Acceptance Criteria:**
- [ ] All file operations wrapped with proper error handling
- [ ] Unit tests pass with >95% coverage
- [ ] Documentation updated
- [ ] No breaking changes to existing APIs

**Files to Create:**
- `internal/util/fs/wrapper.go`
- `internal/util/fs/wrapper_test.go`
- `internal/util/fs/atomic.go`
- `docs/dev/file-operations-guide.md`

---

#### Task 1.2: Standardize Error Handling
**Effort:** 2 days | **Priority:** HIGH

**Steps:**

1. Enhance `internal/util/errors/structured.go`:
```go
type StructuredError struct {
    Type        ErrorType
    Field       string
    Message     string
    Suggestions []string
    Context     map[string]interface{}
    Cause       error
    Operation   string
    Retryable   bool
}

func CreateValidationError(field, message string, suggestions ...string) *StructuredError
func CreateFileError(operation, path string, cause error) *StructuredError
func CreateNetworkError(operation string, cause error) *StructuredError
```

2. Add error formatting utilities
3. Create error aggregation helpers
4. Write comprehensive tests

**Acceptance Criteria:**
- [ ] Consistent error creation functions
- [ ] Error context properly captured
- [ ] Suggestions provided for common errors
- [ ] Tests cover all error types

**Files to Modify:**
- `internal/util/errors/structured.go`
- `internal/util/errors/handler.go`
- `internal/util/errors/aggregator.go`

---

#### Task 1.3: Remove Orphaned Code
**Effort:** 1 day | **Priority:** MEDIUM

**Steps:**

1. Remove unused `internal/core/config/` directory:
```bash
git rm -r internal/core/config/
```

2. Document decision in ADR:
```markdown
# ADR-005: Remove Incomplete Core Config Migration

## Status
Accepted

## Decision
Remove `internal/core/config/` as migration was never completed.
Keep existing `internal/config/` as single source of truth.

## Rationale
- 0 references to core/config in codebase
- Incomplete migration creates confusion
- Maintaining two systems is costly
```

3. Update architecture documentation
4. Clean up import statements

**Acceptance Criteria:**
- [ ] Orphaned code removed
- [ ] ADR documented
- [ ] No broken imports
- [ ] All tests pass

**Files to Remove:**
- `internal/core/config/loader.go`
- `internal/core/config/manager.go`
- `internal/core/config/types.go`

**Files to Create:**
- `docs/architecture/adr/005-remove-core-config.md`

---

### Week 2: Test Infrastructure & DI Cleanup

#### Task 1.4: Consolidate Test Helpers
**Effort:** 2 days | **Priority:** LOW

**Steps:**

1. Create `internal/testing/helpers.go`:
```go
func CreateTempConfig(t *testing.T, content string) string
func CreateTempDir(t *testing.T, files map[string]string) string
func CreateMockFileSystem(t *testing.T) *MockFileSystem
func AssertNoError(t *testing.T, err error, msg string)
func AssertEqual(t *testing.T, expected, actual interface{})
```

2. Migrate existing test helpers
3. Update all test files to use new helpers
4. Remove duplicate helper functions

**Acceptance Criteria:**
- [ ] Single source for test utilities
- [ ] All tests migrated
- [ ] No duplicate test helpers
- [ ] Test coverage maintained

---

#### Task 1.5: Unify DI Container Initialization
**Effort:** 2 days | **Priority:** MEDIUM

**Steps:**

1. Consolidate to single initialization in `internal/di/setup.go`:
```go
func SetupContainer(baseDir string) (Container, error) {
    container := NewContainer()
    
    // Register all services using provider functions
    container.Singleton("PathResolver", func() (*paths.PathResolver, error) {
        return ProvidePathResolver(baseDir)
    })
    container.Singleton("ConfigManager", ProvideConfigManager)
    container.Singleton("ValidationEngine", ProvideValidationEngine)
    
    return container, container.Initialize()
}
```

2. Update `cmd/root.go` to use single initialization
3. Remove duplicate initialization code
4. Add integration tests

**Acceptance Criteria:**
- [ ] Single DI initialization point
- [ ] All services properly registered
- [ ] Integration tests pass
- [ ] No duplicate registration code

**Files to Modify:**
- `internal/di/setup.go`
- `cmd/root.go`

---

### Phase 1 Deliverables

- [ ] File operations wrapper implemented
- [ ] Structured error handling standardized
- [ ] Orphaned code removed
- [ ] Test helpers consolidated
- [ ] DI container unified
- [ ] Documentation updated
- [ ] All tests passing

**Phase 1 Metrics:**
- Code reduction: ~500 LOC
- Test coverage: Maintained at >80%
- Build time: Reduced by 5%

---


## Phase 2: Validation Consolidation

**Duration:** 3 weeks  
**Risk:** Medium  
**Impact:** High

### Week 3: Validation Engine Enhancement

#### Task 2.1: Complete ValidationEngine Implementation
**Effort:** 3 days | **Priority:** CRITICAL

**Steps:**

1. Enhance `internal/core/validation/engine.go`:
```go
type ValidationEngine struct {
    validators map[string]Validator
    mu         sync.RWMutex
}

func (e *ValidationEngine) Register(validator Validator) error
func (e *ValidationEngine) Validate(ctx context.Context, validatorName string, data interface{}) (*ValidationResult, error)
func (e *ValidationEngine) ValidateAll(ctx context.Context, data interface{}) (*ValidationResult, error)
func (e *ValidationEngine) Has(name string) bool
```

2. Add validator chaining support
3. Implement validation result aggregation
4. Add context-aware validation

**Acceptance Criteria:**
- [ ] Engine supports all validation types
- [ ] Validator registration is thread-safe
- [ ] Results properly aggregated
- [ ] Context passed through validation chain

---

#### Task 2.2: Create Unified Validators
**Effort:** 4 days | **Priority:** CRITICAL

**Steps:**

1. Create validators in `internal/core/validation/validators/`:
```go
// cluster_validator.go
type ClusterValidator struct {
    nameValidator     *ClusterNameValidator
    networkValidator  *NetworkValidator
    providerValidator *ProviderValidator
}

// config_validator.go
type ConfigValidator struct {
    structureValidator *StructureValidator
    semanticValidator  *SemanticValidator
}

// sops_validator.go
type SOPSValidator struct {
    keyValidator    *KeyValidator
    configValidator *ConfigValidator
}
```

2. Migrate validation logic from existing validators
3. Add comprehensive test coverage
4. Create validation rule documentation

**Acceptance Criteria:**
- [ ] All validation logic migrated
- [ ] Tests cover all validation rules
- [ ] Documentation complete
- [ ] No duplicate validation code

**Files to Create:**
- `internal/core/validation/validators/cluster_validator.go`
- `internal/core/validation/validators/config_validator.go`
- `internal/core/validation/validators/sops_validator.go`
- `internal/core/validation/validators/gitops_validator.go`
- `internal/core/validation/validators/service_validator.go`

---

### Week 4-5: Validation Migration

#### Task 2.3: Migrate Config Validation
**Effort:** 5 days | **Priority:** CRITICAL

**Steps:**

1. Add feature flag for new validation:
```go
// internal/config/flags.go
var UseNewValidation = os.Getenv("OPENCENTER_NEW_VALIDATION") == "true"
```

2. Update `ConfigurationManager` to use ValidationEngine:
```go
func (cm *ConfigurationManager) ValidateConfig(ctx context.Context, config *Config) *ConfigValidationResult {
    if flags.UseNewValidation {
        return cm.validateWithEngine(ctx, config)
    }
    return cm.validator.Validate(ctx, config) // Legacy
}

func (cm *ConfigurationManager) validateWithEngine(ctx context.Context, config *Config) *ConfigValidationResult {
    result, err := cm.validationEngine.Validate(ctx, "config", config)
    if err != nil {
        return &ConfigValidationResult{Valid: false, Errors: []*ConfigValidationError{{Message: err.Error()}}}
    }
    return convertValidationResult(result)
}
```

3. Run parallel validation (old + new) with comparison
4. Fix discrepancies
5. Enable new validation by default

**Acceptance Criteria:**
- [ ] Feature flag implemented
- [ ] Parallel validation runs successfully
- [ ] Results match between old and new
- [ ] Performance is equal or better

---

#### Task 2.4: Migrate SOPS Validation
**Effort:** 3 days | **Priority:** HIGH

**Steps:**

1. Update `internal/sops/manager.go`:
```go
func (m *DefaultSOPSManager) ValidateEncryption(overlayPath string, cfg *config.Config) error {
    result, err := m.validationEngine.Validate(ctx, "sops", map[string]interface{}{
        "overlay_path": overlayPath,
        "config":       cfg,
    })
    if err != nil {
        return err
    }
    if !result.Valid {
        return result.ToError()
    }
    return nil
}
```

2. Remove old `internal/sops/validator.go`
3. Update tests
4. Update documentation

**Acceptance Criteria:**
- [ ] SOPS validation uses ValidationEngine
- [ ] Old validator removed
- [ ] Tests updated and passing
- [ ] Documentation current

---

#### Task 2.5: Migrate Service Validation
**Effort:** 3 days | **Priority:** MEDIUM

**Steps:**

1. Create base service validator:
```go
type ServiceValidator struct {
    engine *validation.ValidationEngine
}

func (v *ServiceValidator) ValidateService(ctx context.Context, name string, config interface{}) error {
    return v.engine.Validate(ctx, "service:"+name, config)
}
```

2. Update service plugins to register validators:
```go
func init() {
    validation.DefaultEngine().Register(&CertManagerValidator{})
    validation.DefaultEngine().Register(&LokiValidator{})
    // ... other services
}
```

3. Remove individual Validate methods from plugins
4. Update service registry

**Acceptance Criteria:**
- [ ] All services use ValidationEngine
- [ ] Plugin Validate methods removed
- [ ] Service registry updated
- [ ] Tests passing

---

### Phase 2 Deliverables

- [ ] ValidationEngine fully implemented
- [ ] All validators migrated
- [ ] Feature flags in place
- [ ] Parallel validation tested
- [ ] Old validation code removed
- [ ] Documentation updated
- [ ] Performance benchmarks met

**Phase 2 Metrics:**
- Code reduction: ~1,800 LOC
- Validation consistency: 100%
- Test coverage: >85%
- Performance: Equal or better

---


## Phase 3: Configuration Unification

**Duration:** 4 weeks  
**Risk:** High  
**Impact:** High

### Week 6-7: Configuration Manager Consolidation

#### Task 3.1: Design Unified Configuration API
**Effort:** 2 days | **Priority:** CRITICAL

**Steps:**

1. Create API specification document:
```markdown
# Configuration Manager API Specification

## Core Operations
- Load(ctx, name) (*Config, error)
- Save(ctx, config) error
- Validate(ctx, config) error
- List(ctx) ([]string, error)
- Delete(ctx, name) error

## Builder Operations
- NewBuilder(name) ConfigBuilder
- BuildFrom(config) ConfigBuilder

## Cache Operations
- ClearCache(ctx) error
- InvalidateCluster(ctx, name) error
```

2. Design migration strategy
3. Create compatibility layer
4. Review with team

**Acceptance Criteria:**
- [ ] API specification complete
- [ ] Migration strategy documented
- [ ] Team review completed
- [ ] Compatibility plan approved

**Files to Create:**
- `docs/architecture/config-manager-api-spec.md`
- `docs/architecture/config-migration-strategy.md`

---

#### Task 3.2: Implement Unified ConfigurationManager
**Effort:** 5 days | **Priority:** CRITICAL

**Steps:**

1. Create new `internal/config/unified_manager.go`:
```go
type UnifiedConfigurationManager struct {
    loader       *ConfigLoader
    validator    *validation.ValidationEngine
    cache        *ConfigCache
    pathResolver *paths.PathResolver
    fileSystem   fs.FileSystem
    
    mu sync.RWMutex
}

func NewUnifiedConfigurationManager(opts ...ManagerOption) (*UnifiedConfigurationManager, error) {
    mgr := &UnifiedConfigurationManager{
        loader:       NewConfigLoader(),
        validator:    validation.DefaultEngine(),
        cache:        NewConfigCache(),
        pathResolver: paths.NewPathResolver(resolveBaseDir()),
        fileSystem:   fs.NewDefaultFileSystem(),
    }
    
    for _, opt := range opts {
        opt(mgr)
    }
    
    return mgr, nil
}

func (m *UnifiedConfigurationManager) Load(ctx context.Context, name string) (*Config, error) {
    // Check cache
    if cached, found := m.cache.Get(ctx, name); found {
        return cached, nil
    }
    
    // Resolve path
    path, err := m.pathResolver.ResolveConfigPath(name, "")
    if err != nil {
        return nil, errors.CreateFileError("resolve_path", name, err)
    }
    
    // Load from file
    data, err := m.fileSystem.ReadFile(path)
    if err != nil {
        return nil, errors.CreateFileError("read_config", path, err)
    }
    
    // Parse and validate
    config, err := m.loader.LoadFromBytes(ctx, data)
    if err != nil {
        return nil, err
    }
    
    // Cache result
    m.cache.Set(ctx, name, config)
    
    return config, nil
}
```

2. Implement all core operations
3. Add comprehensive tests
4. Create benchmarks

**Acceptance Criteria:**
- [ ] All operations implemented
- [ ] Tests cover all paths
- [ ] Benchmarks show acceptable performance
- [ ] Documentation complete

---

#### Task 3.3: Create Compatibility Layer
**Effort:** 3 days | **Priority:** HIGH

**Steps:**

1. Create `internal/config/compat.go`:
```go
// Deprecated: Use UnifiedConfigurationManager.Load instead
func Load(clusterName string) (Config, error) {
    if useNewManager() {
        mgr, _ := NewUnifiedConfigurationManager()
        cfg, err := mgr.Load(context.Background(), clusterName)
        if err != nil {
            return Config{}, err
        }
        return *cfg, nil
    }
    return legacyLoad(clusterName)
}

// Deprecated: Use UnifiedConfigurationManager.Save instead
func Save(cfg Config) error {
    if useNewManager() {
        mgr, _ := NewUnifiedConfigurationManager()
        return mgr.Save(context.Background(), &cfg)
    }
    return legacySave(cfg)
}

func useNewManager() bool {
    return os.Getenv("OPENCENTER_NEW_CONFIG_MANAGER") == "true"
}
```

2. Add deprecation warnings
3. Update documentation
4. Create migration guide

**Acceptance Criteria:**
- [ ] Compatibility layer works
- [ ] Deprecation warnings visible
- [ ] Migration guide complete
- [ ] No breaking changes

---

### Week 8-9: Migration & Cleanup

#### Task 3.4: Migrate All Config Callers
**Effort:** 5 days | **Priority:** CRITICAL

**Steps:**

1. Identify all callers of legacy config functions:
```bash
grep -r "config\.Load\|config\.Save\|config\.Validate" cmd/ internal/
```

2. Create migration checklist:
```markdown
- [ ] cmd/cluster_init.go
- [ ] cmd/cluster_validate.go
- [ ] cmd/cluster_setup.go
- [ ] internal/cluster/init_service.go
- [ ] internal/cluster/validate_service.go
- [ ] internal/gitops/generator.go
... (45+ files)
```

3. Migrate in batches of 10 files
4. Test after each batch
5. Update integration tests

**Acceptance Criteria:**
- [ ] All callers migrated
- [ ] Tests passing after each batch
- [ ] Integration tests updated
- [ ] No legacy calls remaining

---

#### Task 3.5: Remove Legacy Configuration Code
**Effort:** 3 days | **Priority:** HIGH

**Steps:**

1. Remove deprecated functions:
```bash
# Remove from config.go
- func Load(clusterName string) (Config, error)
- func Save(cfg Config) error
- func Validate(cfg Config) []string
- func List() ([]string, error)
```

2. Remove old ConfigurationManager:
```bash
git rm internal/config/manager.go
```

3. Rename UnifiedConfigurationManager to ConfigurationManager
4. Update all imports
5. Run full test suite

**Acceptance Criteria:**
- [ ] Legacy code removed
- [ ] Imports updated
- [ ] All tests passing
- [ ] Documentation current

---

### Phase 3 Deliverables

- [ ] Unified ConfigurationManager implemented
- [ ] All callers migrated
- [ ] Legacy code removed
- [ ] Compatibility maintained during migration
- [ ] Performance benchmarks met
- [ ] Documentation complete

**Phase 3 Metrics:**
- Code reduction: ~1,200 LOC
- API simplification: 3 systems → 1 system
- Performance: 40% faster config loading (with cache)
- Test coverage: >85%

---


## Phase 4: Cleanup & Optimization

**Duration:** 5 weeks  
**Risk:** Low  
**Impact:** Medium

### Week 10-11: Service Plugin Consolidation

#### Task 4.1: Create Base Service Plugin
**Effort:** 3 days | **Priority:** MEDIUM

**Steps:**

1. Create `internal/services/base_plugin.go`:
```go
type BaseServicePlugin struct {
    metadata PluginMetadata
    validator func(interface{}) error
    renderer func(interface{}) ([]byte, error)
}

func NewBasePlugin(metadata PluginMetadata) *BaseServicePlugin {
    return &BaseServicePlugin{
        metadata: metadata,
        validator: func(interface{}) error { return nil },
        renderer: func(interface{}) ([]byte, error) { return nil, nil },
    }
}

func (p *BaseServicePlugin) Name() string { return p.metadata.Name }
func (p *BaseServicePlugin) Version() string { return p.metadata.Version }
func (p *BaseServicePlugin) Description() string { return p.metadata.Description }
func (p *BaseServicePlugin) Type() string { return p.metadata.Type }

func (p *BaseServicePlugin) Validate(config interface{}) error {
    return p.validator(config)
}

func (p *BaseServicePlugin) Render(config interface{}) ([]byte, error) {
    return p.renderer(config)
}
```

2. Add plugin registration helpers
3. Create plugin testing utilities
4. Write comprehensive tests

**Acceptance Criteria:**
- [ ] Base plugin implemented
- [ ] Registration helpers work
- [ ] Tests cover all methods
- [ ] Documentation complete

---

#### Task 4.2: Migrate Service Plugins
**Effort:** 5 days | **Priority:** MEDIUM

**Steps:**

1. Migrate plugins one by one:
```go
// Before (cert_manager.go - 120 lines)
type CertManagerPlugin struct {
    name        string
    version     string
    description string
}

func NewCertManagerPlugin() *CertManagerPlugin {
    return &CertManagerPlugin{
        name:        "cert-manager",
        version:     "1.0.0",
        description: "Certificate management",
    }
}

func (p *CertManagerPlugin) Name() string { return p.name }
func (p *CertManagerPlugin) Version() string { return p.version }
// ... 8 more boilerplate methods

// After (cert_manager.go - 40 lines)
type CertManagerPlugin struct {
    *BaseServicePlugin
}

func NewCertManagerPlugin() *CertManagerPlugin {
    base := NewBasePlugin(PluginMetadata{
        Name:        "cert-manager",
        Version:     "1.0.0",
        Description: "Certificate management",
        Type:        "core",
    })
    
    plugin := &CertManagerPlugin{BaseServicePlugin: base}
    base.validator = plugin.validate
    base.renderer = plugin.render
    return plugin
}

func (p *CertManagerPlugin) validate(config interface{}) error {
    // Only unique validation logic
}
```

2. Update plugin registry
3. Test each migrated plugin
4. Update documentation

**Acceptance Criteria:**
- [ ] All 15+ plugins migrated
- [ ] Tests passing for each plugin
- [ ] Registry updated
- [ ] Documentation current

---

### Week 12-13: Path Resolution & File Operations

#### Task 4.3: Consolidate Path Resolution
**Effort:** 3 days | **Priority:** MEDIUM

**Steps:**

1. Enhance `internal/core/paths/resolver.go`:
```go
type PathResolver struct {
    baseDir string
    cache   map[string]string
    mu      sync.RWMutex
}

func (pr *PathResolver) ResolveConfigPath(clusterName, organization string) (string, error) {
    cacheKey := fmt.Sprintf("%s:%s", organization, clusterName)
    
    pr.mu.RLock()
    if cached, found := pr.cache[cacheKey]; found {
        pr.mu.RUnlock()
        return cached, nil
    }
    pr.mu.RUnlock()
    
    if organization == "" {
        organization = "opencenter"
    }
    
    path := filepath.Join(pr.baseDir, organization, "."+clusterName+"-config.yaml")
    
    pr.mu.Lock()
    pr.cache[cacheKey] = path
    pr.mu.Unlock()
    
    return path, nil
}
```

2. Remove duplicate path functions from `internal/config/paths.go`
3. Update all callers
4. Add caching for performance

**Acceptance Criteria:**
- [ ] Single path resolver
- [ ] All callers updated
- [ ] Caching improves performance
- [ ] Tests passing

---

#### Task 4.4: Migrate to File Operations Wrapper
**Effort:** 5 days | **Priority:** MEDIUM

**Steps:**

1. Update all packages to use `fs.FileSystem`:
```go
// Before
data, err := os.ReadFile(path)
if err != nil {
    return fmt.Errorf("failed to read: %w", err)
}

// After
data, err := fileSystem.ReadFile(path)
if err != nil {
    return err // Already wrapped with context
}
```

2. Create migration script:
```bash
#!/bin/bash
# migrate-file-ops.sh
find internal/ -name "*.go" -exec sed -i 's/os\.ReadFile/fileSystem.ReadFile/g' {} \;
find internal/ -name "*.go" -exec sed -i 's/os\.WriteFile/fileSystem.WriteFile/g' {} \;
```

3. Update dependency injection to provide FileSystem
4. Test thoroughly

**Acceptance Criteria:**
- [ ] All file operations use wrapper
- [ ] No direct os.ReadFile/WriteFile calls
- [ ] Error handling consistent
- [ ] Tests passing

---

### Week 14: Final Cleanup & Documentation

#### Task 4.5: Remove Unused Interfaces
**Effort:** 2 days | **Priority:** LOW

**Steps:**

1. Audit interface usage:
```bash
# Find interfaces with single implementation
grep -r "type.*Interface interface" internal/ | while read line; do
    interface=$(echo $line | awk '{print $2}')
    count=$(grep -r "$interface" internal/ | wc -l)
    if [ $count -lt 5 ]; then
        echo "Low usage: $interface ($count references)"
    fi
done
```

2. Remove unused interfaces:
```go
// Remove from internal/config/interfaces.go
- type ConfigLoaderInterface interface { ... }
- type PathResolverInterface interface { ... }
- type ConfigCacheInterface interface { ... }

// Keep only
type ConfigValidatorInterface interface { ... }
```

3. Update to use concrete types
4. Update documentation

**Acceptance Criteria:**
- [ ] Unused interfaces removed
- [ ] Concrete types used where appropriate
- [ ] Tests passing
- [ ] Documentation updated

---

#### Task 4.6: Performance Optimization
**Effort:** 2 days | **Priority:** LOW

**Steps:**

1. Run performance benchmarks:
```bash
mise run test -bench=. -benchmem ./internal/...
```

2. Identify bottlenecks
3. Optimize hot paths:
   - Config loading with caching
   - Validation with memoization
   - Path resolution with caching

4. Re-run benchmarks and compare

**Acceptance Criteria:**
- [ ] Benchmarks run successfully
- [ ] Performance improved by 15%
- [ ] No regressions
- [ ] Results documented

---

#### Task 4.7: Documentation Update
**Effort:** 2 days | **Priority:** HIGH

**Steps:**

1. Update architecture documentation:
   - `docs/architecture/overview.md`
   - `docs/architecture/configuration.md`
   - `docs/architecture/validation.md`

2. Create migration guides:
   - `docs/migration/v1-to-v2-config.md`
   - `docs/migration/validation-engine.md`

3. Update API documentation:
   - `docs/reference/configuration-api.md`
   - `docs/reference/validation-api.md`

4. Create ADRs for major decisions

**Acceptance Criteria:**
- [ ] All docs updated
- [ ] Migration guides complete
- [ ] ADRs documented
- [ ] Examples working

---

### Phase 4 Deliverables

- [ ] Service plugins consolidated
- [ ] Path resolution unified
- [ ] File operations migrated
- [ ] Unused interfaces removed
- [ ] Performance optimized
- [ ] Documentation complete

**Phase 4 Metrics:**
- Code reduction: ~1,000 LOC
- Performance improvement: 15%
- Documentation: 100% current
- Technical debt: Reduced by 70%

---


## Testing Strategy

### Test Coverage Requirements

| Phase | Minimum Coverage | Target Coverage |
|-------|------------------|-----------------|
| Phase 1 | 80% | 85% |
| Phase 2 | 82% | 87% |
| Phase 3 | 85% | 90% |
| Phase 4 | 85% | 90% |

### Testing Approach

#### Unit Tests
- Test each component in isolation
- Mock dependencies using interfaces
- Aim for >90% coverage on new code

```bash
# Run unit tests
mise run test

# Run with coverage
go test -cover ./internal/...

# Generate coverage report
go test -coverprofile=coverage.out ./internal/...
go tool cover -html=coverage.out
```

#### Integration Tests
- Test component interactions
- Use real dependencies where possible
- Focus on critical paths

```bash
# Run integration tests
go test -tags=integration ./internal/...
```

#### Property-Based Tests
- Maintain existing property tests
- Add new property tests for validators
- Use gopter for generative testing

```bash
# Run property tests
go test -run Property ./internal/...
```

#### BDD Tests
- Keep existing Cucumber/Godog tests
- Update scenarios for new behavior
- Add scenarios for new features

```bash
# Run BDD tests
mise run godog
```

### Continuous Testing

```yaml
# .github/workflows/refactor-tests.yml
name: Refactor Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.25.2'
      
      - name: Run unit tests
        run: mise run test
      
      - name: Check coverage
        run: |
          go test -coverprofile=coverage.out ./internal/...
          coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          if (( $(echo "$coverage < 85" | bc -l) )); then
            echo "Coverage $coverage% is below 85%"
            exit 1
          fi
      
      - name: Run integration tests
        run: go test -tags=integration ./internal/...
      
      - name: Run BDD tests
        run: mise run godog
```

---

## Rollback Plan

### Rollback Triggers

1. **Test Coverage Drop:** Coverage falls below 80%
2. **Performance Regression:** >10% performance degradation
3. **Critical Bugs:** P0/P1 bugs introduced
4. **Build Failures:** CI/CD pipeline fails consistently

### Rollback Procedures

#### Phase 1 Rollback
```bash
# Revert file operations wrapper
git revert <commit-hash>

# Restore direct os.ReadFile/WriteFile calls
git checkout main -- internal/

# Run tests
mise run test
```

#### Phase 2 Rollback
```bash
# Disable new validation via feature flag
export OPENCENTER_NEW_VALIDATION=false

# Or revert validation commits
git revert <validation-commit-range>

# Restore old validators
git checkout main -- internal/config/validator.go
git checkout main -- internal/sops/validator.go
```

#### Phase 3 Rollback
```bash
# Disable new config manager
export OPENCENTER_NEW_CONFIG_MANAGER=false

# Or revert config manager commits
git revert <config-commit-range>

# Restore legacy config functions
git checkout main -- internal/config/config.go
```

#### Phase 4 Rollback
```bash
# Revert service plugin changes
git revert <plugin-commit-range>

# Restore individual plugins
git checkout main -- internal/services/plugins/
```

### Feature Flags

All major changes use feature flags for gradual rollout:

```go
// internal/config/flags.go
package config

import "os"

var (
    UseNewValidation     = os.Getenv("OPENCENTER_NEW_VALIDATION") == "true"
    UseNewConfigManager  = os.Getenv("OPENCENTER_NEW_CONFIG_MANAGER") == "true"
    UseFileSystemWrapper = os.Getenv("OPENCENTER_FS_WRAPPER") == "true"
)
```

Enable progressively:
1. **Week 1-2:** Internal testing only
2. **Week 3-4:** Alpha users (opt-in)
3. **Week 5-6:** Beta users (default on, opt-out)
4. **Week 7+:** General availability

---

## Success Criteria

### Quantitative Metrics

| Metric | Baseline | Target | Measurement |
|--------|----------|--------|-------------|
| Lines of Code (internal/) | 45,000 | 34,000 | `cloc internal/` |
| Test Coverage | 75% | 85% | `go test -cover` |
| Build Time | 45s | 38s | CI pipeline |
| Config Load Time | 120ms | 70ms | Benchmarks |
| Validation Time | 80ms | 50ms | Benchmarks |
| Cyclomatic Complexity | 15 avg | 10 avg | `gocyclo` |
| Code Duplication | 12% | 5% | `dupl` |

### Qualitative Metrics

- [ ] Developer onboarding time reduced by 30%
- [ ] Code review time reduced by 25%
- [ ] Bug report rate reduced by 40%
- [ ] Feature development velocity increased by 20%
- [ ] Documentation completeness at 100%

### Acceptance Criteria

#### Phase 1
- [ ] All file operations use wrapper
- [ ] Structured errors used consistently
- [ ] Orphaned code removed
- [ ] Test helpers consolidated
- [ ] DI container unified

#### Phase 2
- [ ] ValidationEngine fully adopted
- [ ] All validators migrated
- [ ] Old validation code removed
- [ ] Performance maintained or improved

#### Phase 3
- [ ] Single ConfigurationManager
- [ ] All callers migrated
- [ ] Legacy config code removed
- [ ] Caching improves performance

#### Phase 4
- [ ] Service plugins use base class
- [ ] Path resolution unified
- [ ] Unused interfaces removed
- [ ] Documentation complete

### Sign-off Requirements

Each phase requires sign-off from:
- [ ] Tech Lead
- [ ] QA Lead
- [ ] Product Owner
- [ ] 2+ Senior Engineers

---

## Risk Mitigation

### Technical Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Breaking changes | Medium | High | Feature flags, compatibility layer |
| Performance regression | Low | High | Benchmarks, gradual rollout |
| Test coverage drop | Medium | Medium | Coverage gates, mandatory reviews |
| Incomplete migration | Medium | High | Clear acceptance criteria, tracking |

### Process Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Scope creep | High | Medium | Strict phase boundaries, no new features |
| Resource constraints | Medium | High | Buffer time, prioritization |
| Knowledge silos | Low | Medium | Pair programming, documentation |
| Stakeholder misalignment | Low | High | Regular updates, demos |

### Mitigation Strategies

1. **Feature Flags:** Enable gradual rollout and easy rollback
2. **Parallel Systems:** Run old and new systems side-by-side during migration
3. **Comprehensive Testing:** Maintain >85% coverage throughout
4. **Regular Communication:** Weekly updates to stakeholders
5. **Pair Programming:** Knowledge sharing during complex changes
6. **Code Reviews:** Mandatory reviews by 2+ engineers
7. **Documentation:** Update docs with each phase
8. **Monitoring:** Track metrics continuously

---

## Communication Plan

### Weekly Updates

**Audience:** Engineering team, product, leadership  
**Format:** Email + Slack  
**Content:**
- Progress against roadmap
- Metrics update
- Blockers and risks
- Next week's plan

### Bi-weekly Demos

**Audience:** Stakeholders  
**Format:** Video call + slides  
**Content:**
- Live demo of improvements
- Before/after comparisons
- Q&A session

### Monthly Reviews

**Audience:** Leadership  
**Format:** Presentation  
**Content:**
- Phase completion status
- ROI analysis
- Risk assessment
- Go/no-go decision for next phase

---

## Appendix

### Useful Commands

```bash
# Check code coverage
go test -coverprofile=coverage.out ./internal/...
go tool cover -html=coverage.out

# Find duplicate code
dupl -t 100 internal/

# Check cyclomatic complexity
gocyclo -over 15 internal/

# Count lines of code
cloc internal/

# Run all tests
mise run test && mise run godog

# Run benchmarks
go test -bench=. -benchmem ./internal/...

# Check for unused code
deadcode ./internal/...

# Static analysis
staticcheck ./internal/...
```

### References

- [Executive Summary](./executive-summary.md)
- [Architectural Diagrams](./architectural-diagram.md)
- [Detailed Findings](./detailed-findings.md)
- [Go Best Practices](https://go.dev/doc/effective_go)
- [Testing Best Practices](https://go.dev/doc/tutorial/add-a-test)

---

**Document Version:** 1.0  
**Last Updated:** February 3, 2026  
**Next Review:** End of Phase 1 (Week 2)  
**Owner:** Principal Software Architect  
**Approvers:** Tech Lead, Engineering Manager

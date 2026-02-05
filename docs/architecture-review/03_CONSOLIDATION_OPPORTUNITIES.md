# Consolidation & Boilerplate Reduction

**Project**: opencenter-cli  
**Review Date**: February 4, 2026  
**Document**: Phase 3 - Consolidation Analysis

## Table of Contents

- [Overview](#overview)
- [Over-Engineered Areas](#over-engineered-areas)
- [Redundant Interfaces](#redundant-interfaces)
- [Excessive Wrapper Code](#excessive-wrapper-code)
- [Consolidation Opportunities](#consolidation-opportunities)
- [Boilerplate Reduction Strategies](#boilerplate-reduction-strategies)
- [Simplification Recommendations](#simplification-recommendations)

## Overview

This document identifies areas where multiple files/classes can be merged into simpler, unified structures, and where excessive "wrapper" code can be eliminated.

**Key Findings**:
- 🔴 Dual service systems with 11+ duplicate implementations
- 🟡 Triple error handling systems with overlapping functionality
- 🟡 Scattered test utilities with duplicate helpers
- 🟡 Multiple crypto modules with similar functionality
- 🟢 Template engine is well-designed (no consolidation needed)

## Over-Engineered Areas

### Area 1: Service Architecture (CRITICAL)

**Problem**: Two complete service systems with overlapping functionality

**Current Structure**:
```
internal/config/services/
├── interfaces.go              # 10 lines
├── dependency_validator.go    # 140 lines
├── secrets_validator.go       # 200 lines
├── cert_manager.go            # 150 lines
├── cilium.go                  # 120 lines
├── harbor.go                  # 180 lines
├── keycloak.go                # 130 lines
├── loki.go                    # 140 lines
├── metallb.go                 # 110 lines
├── opentelemetry.go           # 100 lines
├── weave_gitops.go            # 90 lines
├── headlamp.go                # 95 lines
└── gateway.go                 # 105 lines
Total: ~1,570 lines

internal/services/
├── plugin.go                  # 250 lines
├── registry.go                # 450 lines
├── base_plugin.go             # 200 lines
├── plugins/
│   ├── cert_manager.go        # 180 lines
│   ├── cilium.go              # 150 lines
│   ├── harbor.go              # 200 lines
│   ├── keycloak.go            # 160 lines
│   ├── loki.go                # 170 lines
│   ├── prometheus_stack.go    # 140 lines
│   ├── tempo.go               # 130 lines
│   ├── velero.go              # 120 lines
│   └── calico.go              # 110 lines
Total: ~2,160 lines
```

**Over-Engineering Issues**:
1. **Duplicate Implementations**: 11+ services implemented twice (~1,500 lines duplicated)
2. **Parallel Validation**: Two separate validation systems
3. **Redundant Interfaces**: Similar interfaces in both systems
4. **Unnecessary Abstraction**: Config services don't need full plugin system

**Consolidation Opportunity**:
```
internal/services/
├── registry.go                # 450 lines (keep)
├── definition.go              # 200 lines (new, unified)
├── lifecycle.go               # 150 lines (new)
├── plugins/                   # 11 implementations (~1,400 lines)
│   ├── cert_manager.go
│   ├── cilium.go
│   └── ...
└── adapters/                  # Config adapters (~300 lines)
    ├── config_adapter.go
    └── service_adapters.go
Total: ~2,500 lines (vs 3,730 lines currently)
```

**Savings**: 1,230 lines (33% reduction)

---

### Area 2: Error Handling (HIGH PRIORITY)

**Problem**: Three separate error handling systems

**Current Structure**:
```
internal/config/errors.go
├── NewFileError()             # 30 lines
├── NewValidationError()       # 35 lines
├── NewPathError()             # 30 lines
├── NewParseError()            # 30 lines
├── WrapFileError()            # 25 lines
├── WrapValidationError()      # 25 lines
├── WrapPathError()            # 25 lines
├── WrapParseError()           # 25 lines
├── IsFileNotFoundError()      # 15 lines
├── IsValidationError()        # 15 lines
├── IsPathError()              # 15 lines
└── IsParseError()             # 15 lines
Total: ~285 lines

internal/util/errors/error_handler.go
├── HandleError()              # 50 lines
├── CreateValidationError()    # 40 lines
├── NewDefaultErrorHandler()   # 30 lines
├── Similar wrapper functions  # 150 lines
Total: ~270 lines

internal/config/flags/errors.go
├── ConfigError struct         # 40 lines
├── ErrorBuilder               # 60 lines
├── ErrorReporter              # 50 lines
├── CreateTemplatedError()     # 45 lines
Total: ~195 lines

Grand Total: ~750 lines
```

**Over-Engineering Issues**:
1. **Duplicate Error Creation**: Same patterns in 3 places
2. **Redundant Wrappers**: Multiple wrapper functions doing same thing
3. **Inconsistent Interfaces**: Each system has different API
4. **Unnecessary Builders**: ErrorBuilder adds complexity without benefit

**Consolidation Opportunity**:
```
internal/util/errors/
├── structured.go              # 200 lines (unified error type)
├── factory.go                 # 150 lines (error creation)
├── handler.go                 # 100 lines (error handling)
└── formatter.go               # 100 lines (error formatting)
Total: ~550 lines (vs 750 lines currently)
```

**Savings**: 200 lines (27% reduction)

---

### Area 3: Test Utilities (MEDIUM PRIORITY)

**Problem**: Test helpers scattered across multiple files

**Current Structure**:
```
internal/testing/helpers.go
├── CreateTempConfig()         # 20 lines
├── CreateTempDir()            # 25 lines
├── AssertNoError()            # 10 lines
├── AssertError()              # 10 lines
├── AssertEqual()              # 10 lines
├── AssertFileExists()         # 15 lines
└── AssertFileNotExists()      # 15 lines
Total: ~105 lines

internal/testing/framework.go
├── TestFramework struct       # 30 lines
├── NewTestFramework()         # 40 lines
├── WriteTemplate()            # 25 lines
├── WriteFile()                # 20 lines
├── CreateTestConfig()         # 30 lines
├── AssertFileExists()         # 15 lines (duplicate!)
├── AssertFileNotExists()      # 15 lines (duplicate!)
├── AssertDirExists()          # 15 lines
└── AssertDirNotExists()       # 15 lines
Total: ~205 lines

Scattered in test files:
├── setupValidationEngine()    # 30 lines × 3 files
├── setupTestCluster()         # 40 lines × 2 files
├── setupBenchmarkManager()    # 35 lines × 2 files
Total: ~225 lines

Grand Total: ~535 lines
```

**Over-Engineering Issues**:
1. **Duplicate Assertions**: File assertions in both helpers and framework
2. **Scattered Setup**: Setup functions repeated in test files
3. **Inconsistent Naming**: Setup vs setup, Assert vs assert
4. **No Centralization**: No single place for test utilities

**Consolidation Opportunity**:
```
internal/testing/
├── framework.go               # 150 lines (unified framework)
├── assertions.go              # 80 lines (all assertions)
├── fixtures.go                # 100 lines (test data)
└── mocks.go                   # 1200 lines (keep as is)
Total: ~1,530 lines (vs 1,735 lines currently)
```

**Savings**: 205 lines (12% reduction)

---

### Area 4: Crypto Utilities (MEDIUM PRIORITY)

**Problem**: Key generation and management split across modules

**Current Structure**:
```
internal/util/crypto/key_generator.go
├── GenerateAgeKey()           # 30 lines
├── GenerateFallbackKey()      # 25 lines
├── GenerateRandomPassword()   # 20 lines
├── ParseAgeKey()              # 25 lines
└── GenerateKeyWithTimestamp() # 20 lines
Total: ~120 lines

internal/util/crypto/key_manager.go
├── GenerateAgeKey()           # 15 lines (delegates)
├── GenerateFallbackKey()      # 15 lines (delegates)
├── GenerateRandomPassword()   # 15 lines (delegates)
├── SaveAgeKey()               # 40 lines
├── LoadAgeKey()               # 35 lines
└── GenerateKeyForCluster()    # 30 lines
Total: ~150 lines

internal/sops/key_manager.go
├── Similar key generation     # 100 lines
├── SOPS-specific logic        # 150 lines
Total: ~250 lines

Grand Total: ~520 lines
```

**Over-Engineering Issues**:
1. **Delegation Pattern**: KeyManager just delegates to KeyGenerator
2. **Duplicate Logic**: Key generation duplicated in SOPS
3. **Unclear Separation**: Not clear when to use which module
4. **Unnecessary Abstraction**: KeyManager interface not needed

**Consolidation Opportunity**:
```
internal/util/crypto/
├── keys.go                    # 200 lines (generation + management)
└── sops.go                    # 150 lines (SOPS-specific)
Total: ~350 lines (vs 520 lines currently)
```

**Savings**: 170 lines (33% reduction)

## Redundant Interfaces

### Redundancy 1: Service Interfaces

**Problem**: Similar interfaces in both service systems

```go
// internal/config/services/interfaces.go
type ServiceConfig interface {
    IsEnabled() bool
    GetStatus() string
}

// internal/services/plugin.go
type ServicePlugin interface {
    Name() string
    Type() ServiceType
    Validate(config interface{}) error
    Render(ctx context.Context, config interface{}, workspace interface{}) error
    Status(config interface{}) ServiceStatus
}
```

**Analysis**:
- `ServiceConfig` is too simple (only 2 methods)
- `ServicePlugin` is more complete
- Both try to represent services
- No clear relationship between them

**Consolidation**:
```go
// internal/services/definition.go
type ServiceDefinition interface {
    Name() string
    Type() ServiceType
    IsEnabled(config interface{}) bool
    Validate(config interface{}) error
    Render(ctx context.Context, config interface{}, workspace interface{}) error
    Status(config interface{}) ServiceStatus
}
```

**Benefits**:
- Single service interface
- Clear contract
- No confusion about which to use

---

### Redundancy 2: Error Interfaces

**Problem**: Multiple error types with overlapping functionality

```go
// internal/config/errors.go
type FileError struct {
    Path       string
    Operation  string
    Err        error
    Suggestion string
}

// internal/util/errors/error_handler.go
type StructuredError struct {
    Type       string
    Message    string
    Cause      error
    Context    map[string]interface{}
    Suggestion string
}

// internal/config/flags/errors.go
type ConfigError struct {
    Field      string
    Value      interface{}
    Message    string
    Cause      error
    Suggestion string
}
```

**Analysis**:
- All three have `Cause error` and `Suggestion string`
- All three wrap underlying errors
- Different field names for similar concepts
- No shared interface

**Consolidation**:
```go
// internal/util/errors/structured.go
type StructuredError struct {
    Type       ErrorType
    Message    string
    Cause      error
    Context    ErrorContext
    Suggestion string
    Metadata   map[string]interface{}
}

type ErrorContext struct {
    File      string
    Line      int
    Column    int
    Field     string
    Value     interface{}
    Operation string
}
```

**Benefits**:
- Single error type
- All context in one place
- Consistent interface
- Easy to extend

---

### Redundancy 3: Validation Interfaces

**Problem**: Multiple validation approaches

```go
// internal/config/services/dependency_validator.go
type DependencyValidator struct {
    config Config
}

func (v *DependencyValidator) ValidateDependencies() []string {
    // Returns error strings
}

// internal/services/registry.go
func (r *DefaultServiceRegistry) ValidateDependencies(services []string) error {
    // Returns error
}

// internal/core/validation/engine.go
type ValidationEngine struct {
    validators map[string]Validator
}

func (e *ValidationEngine) Validate(ctx context.Context, name string, data interface{}) (*ValidationResult, error) {
    // Returns structured result
}
```

**Analysis**:
- Three different validation patterns
- Inconsistent return types ([]string, error, *ValidationResult)
- No shared validation interface
- Duplicate dependency checking logic

**Consolidation**:
```go
// internal/core/validation/validator.go
type Validator interface {
    Name() string
    Validate(ctx context.Context, data interface{}) (*ValidationResult, error)
}

type ValidationResult struct {
    Valid      bool
    Errors     []ValidationError
    Warnings   []ValidationWarning
    Metadata   map[string]interface{}
}

// All validators implement this interface
```

**Benefits**:
- Consistent validation interface
- Structured results
- Easy to add new validators
- Uniform error handling

## Excessive Wrapper Code

### Wrapper 1: Configuration Credential Methods

**Problem**: Too many wrapper methods for credential access

```go
// internal/config/config.go (420 lines total)

// Wrapper methods (12 methods × ~15 lines = 180 lines)
func (c Config) GetCertManagerAWSCredentials() (string, string)
func (c Config) GetLokiS3Credentials() (string, string)
func (c Config) GetTempoS3Credentials() (string, string)
func (c Config) GetS3BackendCredentials() (string, string)
func (c Config) GetAWSApplicationCredentials() (string, string)
func (c Config) GetCertManagerAWSAccessKey() string
func (c Config) GetCertManagerAWSSecretKey() string
func (c Config) GetLokiS3AccessKey() string
func (c Config) GetLokiS3SecretKey() string
func (c Config) GetTempoS3AccessKey() string
func (c Config) GetTempoS3SecretKey() string
func (c Config) GetS3BackendAccessKey() string
func (c Config) GetS3BackendSecretKey() string
```

**Analysis**:
- 13 methods for credential access
- All follow same pattern (service-specific → global fallback)
- Template-friendly single-value methods duplicate tuple methods
- 180 lines of repetitive code

**Consolidation**:
```go
// internal/config/credentials/resolver.go
type CredentialResolver struct {
    config *Config
}

func (r *CredentialResolver) GetCredentials(service string) (accessKey, secretKey string) {
    // Single method with service parameter
    // Handles all fallback logic
}

// Template helper
func (r *CredentialResolver) GetAccessKey(service string) string {
    accessKey, _ := r.GetCredentials(service)
    return accessKey
}

func (r *CredentialResolver) GetSecretKey(service string) string {
    _, secretKey := r.GetCredentials(service)
    return secretKey
}
```

**Savings**: 150 lines (83% reduction in credential methods)

---

### Wrapper 2: Error Wrapping Functions

**Problem**: Too many error wrapping functions

```go
// internal/config/errors.go

func WrapFileError(err error, path, operation string) error { /* 25 lines */ }
func WrapValidationError(err error, field string) error { /* 25 lines */ }
func WrapPathError(err error, path string) error { /* 25 lines */ }
func WrapParseError(err error, line int) error { /* 25 lines */ }
// Total: 100 lines
```

**Analysis**:
- All wrappers follow same pattern
- Only difference is error type
- Could be unified with error type parameter

**Consolidation**:
```go
// internal/util/errors/factory.go

func Wrap(err error, errorType ErrorType, opts ...ErrorOption) error {
    // Single wrapper function
    // Options pattern for flexibility
}

// Usage
errors.Wrap(err, errors.FileError, 
    errors.WithPath(path),
    errors.WithOperation(operation))

errors.Wrap(err, errors.ValidationError,
    errors.WithField(field))
```

**Savings**: 75 lines (75% reduction in wrapper functions)

---

### Wrapper 3: Type Assertion Boilerplate

**Problem**: Identical type assertion pattern in 9+ service plugins

```go
// Repeated in 9+ files
func (p *ServicePlugin) validate(config interface{}) error {
    cfg, ok := config.(*services.ServiceConfig)
    if !ok {
        return fmt.Errorf("invalid config type")
    }
    // validation logic
}
```

**Analysis**:
- Exact same pattern in 9+ files
- 4-5 lines per file = 40-45 lines total
- Could be extracted to helper

**Consolidation**:
```go
// internal/services/helpers.go

func AssertServiceConfig(config interface{}) (*services.ServiceConfig, error) {
    cfg, ok := config.(*services.ServiceConfig)
    if !ok {
        return nil, fmt.Errorf("invalid config type: expected *services.ServiceConfig, got %T", config)
    }
    return cfg, nil
}

// Usage in plugins
func (p *ServicePlugin) validate(config interface{}) error {
    cfg, err := services.AssertServiceConfig(config)
    if err != nil {
        return err
    }
    // validation logic
}
```

**Savings**: 30 lines (67% reduction in type assertion code)

## Consolidation Opportunities

### Opportunity 1: Merge Service Systems

**Current**: 3,730 lines across two systems  
**Target**: 2,500 lines in unified system  
**Savings**: 1,230 lines (33%)

**Approach**:
1. Keep `internal/services/` as primary location
2. Move service definitions from `internal/config/services/`
3. Create config adapters for backward compatibility
4. Migrate validation logic to unified validators
5. Remove duplicate implementations

---

### Opportunity 2: Unify Error Handling

**Current**: 750 lines across three systems  
**Target**: 550 lines in unified system  
**Savings**: 200 lines (27%)

**Approach**:
1. Create unified `StructuredError` type
2. Implement error factory with options pattern
3. Migrate all error creation to factory
4. Remove duplicate wrapper functions
5. Update error checking to use unified types

---

### Opportunity 3: Consolidate Test Utilities

**Current**: 535 lines scattered  
**Target**: 330 lines centralized  
**Savings**: 205 lines (38%)

**Approach**:
1. Merge helpers.go and framework.go
2. Remove duplicate assertions
3. Create centralized setup functions
4. Standardize naming conventions
5. Update all tests to use centralized utilities

---

### Opportunity 4: Merge Crypto Modules

**Current**: 520 lines across three modules  
**Target**: 350 lines in two modules  
**Savings**: 170 lines (33%)

**Approach**:
1. Merge key_generator.go and key_manager.go
2. Keep SOPS-specific logic separate
3. Remove delegation pattern
4. Consolidate key generation logic
5. Update all crypto operations

## Boilerplate Reduction Strategies

### Strategy 1: Options Pattern

**Replace**: Multiple constructor parameters

```go
// Before: Multiple constructors
func NewServiceWithDefaults(name string) *Service
func NewServiceWithConfig(name string, config Config) *Service
func NewServiceWithValidator(name string, validator Validator) *Service
func NewServiceFull(name string, config Config, validator Validator) *Service

// After: Options pattern
type ServiceOption func(*Service)

func WithConfig(config Config) ServiceOption {
    return func(s *Service) { s.config = config }
}

func WithValidator(validator Validator) ServiceOption {
    return func(s *Service) { s.validator = validator }
}

func NewService(name string, opts ...ServiceOption) *Service {
    s := &Service{name: name}
    for _, opt := range opts {
        opt(s)
    }
    return s
}
```

**Benefits**:
- Single constructor
- Flexible configuration
- Easy to add new options
- Backward compatible

---

### Strategy 2: Generic Helpers

**Replace**: Type-specific helpers with generics

```go
// Before: Type-specific
func AssertServiceConfig(config interface{}) (*ServiceConfig, error)
func AssertClusterConfig(config interface{}) (*ClusterConfig, error)
func AssertGitOpsConfig(config interface{}) (*GitOpsConfig, error)

// After: Generic
func AssertType[T any](value interface{}) (*T, error) {
    result, ok := value.(*T)
    if !ok {
        return nil, fmt.Errorf("invalid type: expected *%T, got %T", new(T), value)
    }
    return result, nil
}

// Usage
cfg, err := AssertType[ServiceConfig](config)
```

**Benefits**:
- Single implementation
- Type-safe
- Less code duplication
- Easier to maintain

---

### Strategy 3: Code Generation

**Replace**: Repetitive code with generated code

```go
// Generate credential accessor methods
//go:generate go run gen/credentials.go

// gen/credentials.go generates:
// - GetCertManagerAWSCredentials()
// - GetLokiS3Credentials()
// - GetTempoS3Credentials()
// etc.
```

**Benefits**:
- No manual duplication
- Consistent implementation
- Easy to add new services
- Single source of truth

## Simplification Recommendations

### Recommendation 1: Simplify Service Configuration

**Current**: Complex nested structure

```go
type Config struct {
    OpenCenter struct {
        Services map[string]interface{}
    }
}
```

**Simplified**:

```go
type Config struct {
    Services ServiceRegistry
}

type ServiceRegistry struct {
    services map[string]ServiceConfig
}

func (r *ServiceRegistry) Get(name string) (ServiceConfig, bool)
func (r *ServiceRegistry) Set(name string, config ServiceConfig)
func (r *ServiceRegistry) List() []ServiceConfig
```

**Benefits**:
- Type-safe service access
- Clear API
- No interface{} casting
- Better IDE support

---

### Recommendation 2: Simplify Error Creation

**Current**: Many specialized functions

```go
NewFileError(path, operation, err, suggestion)
NewValidationError(field, value, err, suggestion)
NewPathError(path, err, suggestion)
```

**Simplified**:

```go
errors.New(errors.FileError, "operation failed",
    errors.WithCause(err),
    errors.WithPath(path),
    errors.WithSuggestion(suggestion))
```

**Benefits**:
- Single error creation function
- Flexible options
- Consistent API
- Easy to extend

---

### Recommendation 3: Simplify Validation

**Current**: Multiple validation methods

```go
ValidateDependencies() []string
ValidateSecrets() []string
ValidateConfig() error
ValidateService(name string) error
```

**Simplified**:

```go
engine.Validate(ctx, "dependencies", config)
engine.Validate(ctx, "secrets", config)
engine.Validate(ctx, "config", config)
engine.Validate(ctx, "service:cert-manager", config)
```

**Benefits**:
- Consistent validation interface
- Structured results
- Easy to add validators
- Uniform error handling

## Conclusion

Significant consolidation opportunities exist in opencenter-cli:

**Total Savings**:
- Service systems: 1,230 lines (33%)
- Error handling: 200 lines (27%)
- Test utilities: 205 lines (38%)
- Crypto modules: 170 lines (33%)
- **Total: 1,805 lines (25% of affected code)**

**Priority Order**:
1. 🔴 Service system consolidation (highest impact)
2. 🟡 Error handling unification (high impact)
3. 🟡 Test utility consolidation (medium impact)
4. 🟡 Crypto module merge (medium impact)

**Implementation Strategy**:
- Use options pattern for flexibility
- Apply generics where appropriate
- Consider code generation for repetitive code
- Maintain backward compatibility during migration

**Next Steps**: Proceed with Phase 2 (Service Consolidation) of the refactoring roadmap.

---
doc_type: explanation
---

# Code Architecture


## Table of Contents

- [Who this is for](#who-this-is-for)
- [Architectural Overview](#architectural-overview)
- [Core Principles](#core-principles)
- [Package Organization](#package-organization)
- [Design Patterns](#design-patterns)
- [Data Flow](#data-flow)
- [Dependency Graph](#dependency-graph)
- [Testing Strategy](#testing-strategy)
- [Configuration Storage](#configuration-storage)
- [Plugin System](#plugin-system)
- [Performance Optimizations](#performance-optimizations)
- [Security Architecture](#security-architecture)
- [Extensibility Points](#extensibility-points)
- [Trade-offs and Decisions](#trade-offs-and-decisions)
- [Common Misconceptions](#common-misconceptions)
- [Future Directions](#future-directions)
- [See Also](#see-also)
This document explains the openCenter codebase architecture, design patterns, and organizational principles.

## Who this is for

Developers who need to understand how openCenter is structured internally, why certain design decisions were made, and how components interact.

## Architectural Overview

openCenter follows a layered architecture with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────┐
│                    CLI Layer (cmd/)                      │
│  Cobra commands, flag parsing, user interaction         │
└─────────────────────────────────────────────────────────┘
                          │
┌─────────────────────────────────────────────────────────┐
│              Business Logic (internal/)                  │
│  Configuration, GitOps, Secrets, Validation             │
└─────────────────────────────────────────────────────────┘
                          │
┌─────────────────────────────────────────────────────────┐
│           Provider Integrations (internal/)              │
│  OpenStack, AWS, Terraform, Ansible, Pulumi             │
└─────────────────────────────────────────────────────────┘
```

## Core Principles

### Separation of Concerns

Each package has a single, well-defined responsibility:
- `cmd/`: User interface and command orchestration
- `internal/config/`: Configuration management
- `internal/gitops/`: GitOps repository generation
- `internal/sops/`: Secrets encryption
- `internal/cloud/`: Provider-specific logic

### Dependency Injection

Avoid global state. Pass dependencies explicitly:

```go
// Good: Dependencies injected
func NewConfigManager(loader Loader, validator Validator) *ConfigManager {
    return &ConfigManager{
        loader:    loader,
        validator: validator,
    }
}

// Avoid: Global state
var globalConfig *Config
```

### Interface-Based Design

Define interfaces in consumer packages, not provider packages:

```go
// internal/gitops/interfaces.go
type TemplateEngine interface {
    Render(template string, data interface{}) (string, error)
}

// internal/template/engine.go implements the interface
```

This allows easy testing with mocks and swapping implementations.

### Error Wrapping

Use `fmt.Errorf` with `%w` to preserve error chains:

```go
if err := loader.Load(path); err != nil {
    return fmt.Errorf("failed to load config from %s: %w", path, err)
}
```

## Package Organization

### Command Layer (`cmd/`)

Each command is a separate file following the pattern `cmd/<command>_<subcommand>.go`:

```
cmd/
├── root.go                 # Root command, global flags
├── cluster.go              # Cluster command group
├── cluster_init.go         # cluster init subcommand
├── cluster_validate.go     # cluster validate subcommand
├── cluster_setup.go        # cluster setup subcommand
├── config.go               # Config command group
├── sops.go                 # SOPS command group
└── version.go              # Version command
```

**Naming Convention**: `newCluster<Action>Cmd()` returns `*cobra.Command`

Commands are responsible for:
- Flag parsing and validation
- User interaction (prompts, output)
- Calling business logic
- Error formatting

Commands should be thin - delegate to `internal/` packages.

### Configuration (`internal/config/`)

Configuration management is the heart of openCenter:

```
internal/config/
├── config.go               # Main Config struct
├── types_*.go              # Type definitions by domain
├── schema.go               # JSON schema generation
├── validator.go            # Validation logic
├── loader.go               # YAML loading
├── manager.go              # Configuration lifecycle
├── path_resolver.go        # Organization-based paths
├── migrator.go             # Schema migration
├── builder.go              # Fluent configuration builder
└── defaults/               # Default templates per provider
```

**Key Types**:
- `Config`: Root configuration struct
- `ConfigManager`: Manages configuration lifecycle
- `Validator`: Multi-layered validation
- `PathResolver`: Resolves organization-based paths

**Validation Layers**:
1. JSON schema validation (structure, types)
2. Business rule validation (cross-field checks)
3. Provider-specific validation
4. Connectivity validation (optional)

### GitOps (`internal/gitops/`)

GitOps repository scaffolding:

```
internal/gitops/
├── copy.go                 # Template copying and rendering
├── embed.go                # Embedded template management
├── generator.go            # GitOps generation orchestration
├── workspace.go            # Workspace management
├── pipeline.go             # Generation pipeline
├── gitops-base-dir/        # Base repository structure (embedded)
└── templates/              # Cluster-specific templates (embedded)
```

**Template Processing**:
1. Files with `.tmpl` extension are rendered with Go templates
2. Files without `.tmpl` are copied verbatim
3. Templates have access to full `Config` struct
4. Sprig functions available for advanced templating

**Embedded Resources**:
Templates are embedded in the binary using `//go:embed`:

```go
//go:embed gitops-base-dir
var gitopsBaseFS embed.FS
```

This ensures templates are always available without external dependencies.

### Secrets (`internal/sops/`)

SOPS and Age key management:

```
internal/sops/
├── manager.go              # SOPS manager interface
├── keys.go                 # Age key generation
├── encrypt.go              # Encryption/decryption
├── git.go                  # Git integration
└── validator.go            # SOPS configuration validation
```

**Key Management**:
- Age keys generated per cluster
- Keys stored in organization-based structure
- SOPS configuration generated automatically
- Git hooks prevent committing unencrypted secrets

### Providers (`internal/cloud/`, `internal/provision/`)

Provider-specific implementations:

```
internal/cloud/
└── openstack/
    └── preflight.go        # OpenStack preflight checks

internal/provision/
├── embed.go                # Terraform templates
└── templates/              # Provider-specific templates

internal/ansible/
└── provision.go            # Ansible provisioning (Kubespray)

internal/talos/
├── config.go               # Talos configuration
└── pulumi/                 # Pulumi-based provisioning
```

**Provider Isolation**:
- Each provider in separate package
- Common interfaces defined in consumer packages
- Provider-specific logic isolated
- Easy to add new providers

### Utilities (`internal/util/`)

Shared utility packages:

```
internal/util/
├── crypto/                 # Key generation
├── errors/                 # Error aggregation
├── files/                  # Atomic file operations
├── paths/                  # Path resolution
├── security/               # Credential masking, audit logging
└── template/               # Template utilities
```

### Security (`internal/security/`)

Security components:

```
internal/security/
├── input_validator.go      # Input validation and sanitization
├── command_sanitizer.go    # Command injection prevention
├── credential_masker.go    # Credential masking in logs
└── audit_logger.go         # Audit logging
```

**Security Features**:
- Input validation prevents injection attacks
- Command sanitization for shell execution
- Credential masking in all output
- Audit logging for compliance

### Resilience (`internal/resilience/`)

Operational resilience:

```
internal/resilience/
├── retry.go                # Retry with exponential backoff
├── circuit_breaker.go      # Circuit breaker pattern
└── lock_manager.go         # Distributed locking
```

### Operations (`internal/operations/`)

Operational capabilities:

```
internal/operations/
├── drift_detector.go       # Configuration drift detection
└── backup_manager.go       # Backup and disaster recovery
```

### Template Engine (`internal/template/`)

Template rendering with sandboxing:

```
internal/template/
├── engine.go               # Template engine interface
├── sandbox.go              # Template sandboxing
├── registry.go             # Template registry
├── cache.go                # Template caching
└── composition.go          # Template composition
```

**Sandboxing**:
Templates are sandboxed to prevent code execution:
- No file system access
- No network access
- No arbitrary function calls
- Only safe Sprig functions allowed

### Testing Framework (`internal/testing/`)

Comprehensive testing infrastructure:

```
internal/testing/
├── framework.go            # Test framework
├── generators.go           # Test data generators
├── mocks.go                # Mock implementations
└── benchmarks.go           # Benchmark utilities
```

**Test Framework Features**:
- Temporary directory management
- Mock implementations for all interfaces
- Realistic test data generators
- Property-based testing support

## Design Patterns

### Builder Pattern

Configuration building uses fluent builder pattern:

```go
config := config.NewBuilder().
    WithClusterName("my-cluster").
    WithProvider("openstack").
    WithKubernetesVersion("1.28.0").
    Build()
```

### Factory Pattern

Factories create configured instances:

```go
func NewConfigManager(opts ...Option) (*ConfigManager, error) {
    cm := &ConfigManager{
        loader:    defaultLoader,
        validator: defaultValidator,
    }
    
    for _, opt := range opts {
        opt(cm)
    }
    
    return cm, nil
}
```

### Pipeline Pattern

GitOps generation uses pipeline pattern:

```go
pipeline := gitops.NewPipeline().
    AddStage(gitops.ValidateStage).
    AddStage(gitops.CopyBaseStage).
    AddStage(gitops.RenderTemplatesStage).
    AddStage(gitops.GenerateManifestsStage)

result := pipeline.Execute(ctx, config)
```

### Registry Pattern

Services and templates use registry pattern:

```go
registry := services.NewRegistry()
registry.Register("cert-manager", certManagerService)
registry.Register("prometheus", prometheusService)

service, err := registry.Get("cert-manager")
```

## Data Flow

### Configuration Loading

```
User Input (YAML)
    ↓
Loader (YAML parsing)
    ↓
Config Struct (Go types)
    ↓
Validator (Multi-layer validation)
    ↓
ConfigManager (Lifecycle management)
    ↓
Commands (Business logic)
```

### GitOps Generation

```
Config
    ↓
GitOps Generator
    ↓
Template Engine
    ↓
File System Operations
    ↓
GitOps Repository
```

### Secrets Management

```
Age Key Generation
    ↓
SOPS Configuration
    ↓
Secret Encryption
    ↓
Encrypted Files
    ↓
Git Repository
```

## Dependency Graph

```
cmd/
  ↓
internal/config/
  ↓
internal/gitops/ ← internal/template/
  ↓
internal/sops/
  ↓
internal/cloud/
  ↓
internal/provision/
```

Commands depend on business logic, which depends on utilities. Dependencies flow downward, never upward.

## Testing Strategy

### Unit Tests

Test individual functions and components in isolation:

```go
func TestConfigValidation(t *testing.T) {
    validator := config.NewValidator()
    cfg := config.Config{...}
    
    err := validator.Validate(cfg)
    assert.NoError(t, err)
}
```

### Property-Based Tests

Test properties that should hold for all inputs:

```go
func TestConfigMigrationPreservesData(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    properties.Property("migration preserves data", prop.ForAll(
        func(cfg config.Config) bool {
            migrated := migrator.Migrate(cfg)
            return dataEqual(cfg, migrated)
        },
        generators.Config(),
    ))
    
    properties.TestingRun(t)
}
```

### BDD Tests

Test complete workflows from user perspective:

```gherkin
Feature: Cluster Initialization
  Scenario: Initialize cluster with defaults
    When I run "openCenter cluster init my-cluster"
    Then a cluster configuration "my-cluster" should exist
    And the configuration should be valid
```

### Integration Tests

Test component interactions:

```go
func TestGitOpsGenerationIntegration(t *testing.T) {
    fw := testing.NewTestFramework(t)
    config := fw.CreateTestConfig("openstack")
    
    generator := gitops.NewGenerator(fw.TemplateEngine)
    err := generator.Generate(config, fw.TempDir)
    
    assert.NoError(t, err)
    fw.AssertDirExists(t, filepath.Join(fw.TempDir, "infrastructure"))
}
```

## Configuration Storage

Organization-based directory structure:

```
~/.config/openCenter/clusters/
└── <organization>/
    ├── .<cluster>-config.yaml
    ├── infrastructure/
    │   └── clusters/<cluster>/
    ├── applications/
    │   └── overlays/<cluster>/
    └── secrets/
        ├── age/keys/
        └── ssh/
```

**Why Organization-Based?**
- Multiple teams can manage clusters independently
- Shared GitOps repository per organization
- Isolated secrets per organization
- Clear ownership boundaries

## Plugin System

Plugins extend openCenter with custom commands:

**Discovery**:
1. `OPENCENTER_PLUGINS_DIR` environment variable
2. `<config-dir>/plugins` directory
3. System `PATH`

**Naming**: `openCenter-<plugin-name>`

**Registration**: Plugins are dynamically registered as Cobra subcommands

**Execution**: Plugins are executed as separate processes

## Performance Optimizations

- **Configuration Caching**: Loaded once at startup
- **Template Caching**: Rendered templates cached
- **Plugin Discovery Caching**: Discovered plugins cached
- **Lazy Loading**: Components loaded on demand
- **Embedded Resources**: Templates embedded in binary

## Security Architecture

### Defense in Depth

Multiple security layers:
1. Input validation (prevent injection)
2. Command sanitization (safe shell execution)
3. Credential masking (prevent leaks)
4. Template sandboxing (prevent code execution)
5. SOPS encryption (protect secrets)
6. Audit logging (compliance)

### Threat Model

**Threats Addressed**:
- Command injection via user input
- Path traversal attacks
- Credential leakage in logs
- Arbitrary code execution via templates
- Unencrypted secrets in git

**Mitigations**:
- Input validation on all user input
- Path sanitization and validation
- Credential masking in all output
- Template sandboxing with restricted functions
- SOPS encryption with Age keys
- Git hooks prevent unencrypted commits

## Extensibility Points

### Adding New Providers

1. Create `internal/cloud/<provider>/preflight.go`
2. Add provider config in `internal/config/types_infrastructure.go`
3. Update schema in `internal/config/schema.go`
4. Add validation in `internal/config/<provider>_validator.go`
5. Add provisioning in `internal/provision/<provider>/`

### Adding New Services

1. Create service definition in `internal/services/`
2. Add service templates in `internal/gitops/templates/`
3. Register service in service registry
4. Add service configuration to schema

### Adding New Commands

1. Create `cmd/<command>_<subcommand>.go`
2. Implement command logic
3. Register in parent command
4. Add tests
5. Update documentation

## Trade-offs and Decisions

### Embedded Templates vs External Files

**Decision**: Embed templates in binary

**Rationale**:
- Single binary distribution
- No external dependencies
- Version-locked templates
- Simpler deployment

**Trade-off**: Requires recompilation to update templates

### Organization-Based Structure vs Flat Structure

**Decision**: Organization-based directory structure

**Rationale**:
- Multi-tenancy support
- Clear ownership boundaries
- Shared GitOps repository
- Isolated secrets

**Trade-off**: More complex path resolution

### SOPS vs Other Secret Management

**Decision**: SOPS with Age encryption

**Rationale**:
- Git-friendly (encrypted files in repo)
- Simple key management
- No external service required
- Industry standard

**Trade-off**: Keys must be managed separately

### Mise vs Make

**Decision**: Mise for task automation

**Rationale**:
- Tool version management
- Cross-platform support
- Modern task runner
- Better developer experience

**Trade-off**: Additional tool to install

## Common Misconceptions

### "Configuration is just YAML parsing"

Configuration management includes:
- YAML parsing
- Schema validation
- Business rule validation
- Provider-specific validation
- Migration between versions
- Path resolution
- Environment variable expansion
- Runtime overrides

### "Templates are just string replacement"

Template engine includes:
- Go template rendering
- Sprig function library
- Template composition
- Caching
- Sandboxing
- Error handling

### "Validation is just schema checking"

Validation includes:
- JSON schema validation (structure)
- Business rule validation (cross-field)
- Provider-specific validation
- Connectivity validation
- Semantic validation

## Future Directions

Potential architectural improvements:

- **Plugin API**: Formal plugin API with versioning
- **Event System**: Event-driven architecture for extensibility
- **State Management**: Explicit state management for cluster lifecycle
- **Observability**: Built-in metrics and tracing
- **Multi-Cluster**: Native multi-cluster management

## See Also

- [Developer Guide](./README.md) - Development setup and workflows
- [Testing Guide](./testing/README.md) - Testing strategies
- [Contributing Guidelines](./contributing.md) - How to contribute
- [Configuration Reference](../reference/configuration.md) - Configuration schema

# opencenter-cli Architecture

Architecture documentation for opencenter-cli, a Kubernetes cluster lifecycle management tool with GitOps-first design.

## Table of Contents

- [System Overview](#system-overview)
- [Architecture Layers](#architecture-layers)
- [Core Components](#core-components)
- [Data Flow](#data-flow)
- [Component Interactions](#component-interactions)
- [Design Patterns](#design-patterns)
- [Security Architecture](#security-architecture)
- [Extension Points](#extension-points)

## System Overview

opencenter-cli transforms declarative YAML cluster configurations into production-ready GitOps repositories. The architecture follows a layered design with clear separation of concerns, dependency injection, and pluggable components.

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLI Layer                                │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐       │
│  │  cluster │  │  config  │  │   sops   │  │ plugins  │       │
│  │ commands │  │ commands │  │ commands │  │ commands │       │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘       │
└─────────────────────────────────────────────────────────────────┘
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Dependency Injection                          │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                    DI Container                           │  │
│  │  • ConfigManager  • SOPSManager  • ErrorFormatter        │  │
│  │  • Logger         • GitOpsGenerator                      │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Core Services Layer                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │ Configuration│  │    GitOps    │  │   Secrets    │         │
│  │  Management  │  │  Generation  │  │  Management  │         │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │   Template   │  │   Services   │  │  Validation  │         │
│  │    Engine    │  │   Registry   │  │   Pipeline   │         │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
└─────────────────────────────────────────────────────────────────┘
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Infrastructure Layer                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │  OpenStack   │  │     AWS      │  │   VMware     │         │
│  │   Provider   │  │   Provider   │  │   Provider   │         │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │  Kubespray   │  │    Talos     │  │     Kind     │         │
│  │  Provisioner │  │  Provisioner │  │  Provisioner │         │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
└─────────────────────────────────────────────────────────────────┘
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Storage Layer                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │ Filesystem   │  │  SOPS/Age    │  │   OpenTofu   │         │
│  │   Storage    │  │  Encryption  │  │    State     │         │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
└─────────────────────────────────────────────────────────────────┘
```

## Architecture Layers

### 1. CLI Layer
Entry point for user interactions via Cobra commands.

**Components:**
- `cmd/cluster_*.go` - Cluster lifecycle commands
- `cmd/config_*.go` - Configuration management
- `cmd/sops_*.go` - Secrets operations
- `cmd/plugins_*.go` - Plugin management
- `cmd/root.go` - Root command and global flags

**Responsibilities:**
- Parse command-line arguments
- Validate user input
- Retrieve dependencies from DI container
- Orchestrate business logic
- Format output for users

### 2. Dependency Injection Layer
Manages component lifecycle and dependencies.

**Components:**
- `internal/di/container.go` - DI container implementation
- `internal/di/setup.go` - Component registration

**Responsibilities:**
- Register singleton services
- Resolve dependencies
- Initialize components
- Manage component lifecycle
- Provide thread-safe access

### 3. Core Services Layer
Business logic and domain services.

#### Configuration Management
**Location:** `internal/config/`

**Key Components:**
- `ConfigManager` - Configuration lifecycle
- `ConfigLoader` - YAML parsing
- `ConfigValidator` - Multi-layer validation
- `PathResolver` - Organization-based paths
- `ConfigMigrator` - Schema migrations

**Responsibilities:**
- Load/save cluster configurations
- Validate configuration correctness
- Migrate between schema versions
- Resolve file paths
- Cache configurations

#### GitOps Generation
**Location:** `internal/gitops/`

**Key Components:**
- `PipelineGenerator` - Staged generation pipeline
- `GitOpsWorkspace` - Workspace management
- `GenerationStage` - Individual generation stages
- `AtomicWriter` - Atomic file operations
- `DryRunWorkspace` - Dry-run simulation

**Responsibilities:**
- Generate GitOps repository structure
- Copy and render templates
- Create FluxCD manifests
- Manage generation checkpoints
- Support dry-run mode

#### Secrets Management
**Location:** `internal/sops/`

**Key Components:**
- `SOPSManager` - SOPS orchestration
- `KeyManager` - Age key management
- `Encryptor` - File encryption
- `Validator` - Encryption validation

**Responsibilities:**
- Generate Age encryption keys
- Encrypt sensitive files
- Create SOPS configurations
- Validate encryption status
- Manage key lifecycle

#### Template Engine
**Location:** `internal/template/`

**Key Components:**
- `TemplateEngine` - Template rendering
- `TemplateRegistry` - Template catalog
- `TemplateSandbox` - Secure execution
- `TemplateCache` - Performance optimization

**Responsibilities:**
- Render Go templates
- Provide Sprig functions
- Cache parsed templates
- Sandbox untrusted templates
- Validate template syntax

#### Services Registry
**Location:** `internal/services/`

**Key Components:**
- `ServiceRegistry` - Service catalog
- `ServiceDefinition` - Service metadata
- `ServicePlugin` - Plugin interface
- `ServiceLifecycle` - Lifecycle hooks

**Responsibilities:**
- Register service definitions
- Resolve service dependencies
- Execute lifecycle hooks
- Load service manifests
- Validate service configurations

#### Validation Pipeline
**Location:** `internal/config/`

**Key Components:**
- `ValidationPipeline` - Multi-stage validation
- `EnhancedValidator` - Comprehensive checks
- `SuggestionEngine` - Error suggestions
- `PipelineAdapter` - Validation orchestration

**Responsibilities:**
- Schema validation
- Semantic validation
- Provider-specific validation
- Network configuration validation
- Generate helpful suggestions

### 4. Infrastructure Layer
Cloud provider and provisioner integrations.

**Providers:**
- `internal/cloud/openstack/` - OpenStack integration
- AWS provider (planned)
- VMware provider (planned)

**Provisioners:**
- `internal/ansible/` - Kubespray provisioning
- `internal/talos/` - Talos Linux provisioning
- `internal/provision/` - OpenTofu provisioning

### 5. Storage Layer
Persistent storage and encryption.

**Components:**
- Filesystem operations
- SOPS/Age encryption
- OpenTofu state management
- Git repository storage

## Core Components

### Configuration Manager

```
┌─────────────────────────────────────────────────────────────┐
│                    ConfigurationManager                      │
├─────────────────────────────────────────────────────────────┤
│ + LoadConfig(ctx, clusterName) (*Config, error)            │
│ + SaveConfig(ctx, config) error                            │
│ + ValidateConfig(ctx, config) *ValidationResult            │
│ + ListConfigs(ctx) ([]string, error)                       │
│ + DeleteConfig(ctx, clusterName) error                     │
│ + MigrateClusterToOrganization(ctx, name, org) error       │
├─────────────────────────────────────────────────────────────┤
│ - loader: ConfigLoaderInterface                            │
│ - validator: ConfigValidatorInterface                      │
│ - pathResolver: PathResolverInterface                      │
│ - cache: ConfigCacheInterface                              │
│ - migrator: ConfigMigratorInterface                        │
└─────────────────────────────────────────────────────────────┘
```

**Design Pattern:** Facade + Strategy
**Thread Safety:** Read-write mutex
**Caching:** TTL-based with invalidation

### GitOps Pipeline Generator

```
┌─────────────────────────────────────────────────────────────┐
│                   PipelineGenerator                          │
├─────────────────────────────────────────────────────────────┤
│ + Generate(ctx, config) error                              │
│ + GenerateDryRun(ctx, config) (*GenerationPlan, error)     │
│ + Rollback(ctx, checkpointID) error                        │
│ + SetProgressCallback(callback) void                       │
├─────────────────────────────────────────────────────────────┤
│ - stages: []GenerationStage                                │
│ - workspace: *GitOpsWorkspace                              │
│ - workspaceManager: WorkspaceManager                       │
│ - completedStages: []string                                │
└─────────────────────────────────────────────────────────────┘
```

**Design Pattern:** Pipeline + Command
**Error Handling:** Automatic rollback on failure
**Checkpointing:** Per-stage checkpoints

### SOPS Manager

```
┌─────────────────────────────────────────────────────────────┐
│                    DefaultSOPSManager                        │
├─────────────────────────────────────────────────────────────┤
│ + EncryptOverlayFiles(ctx, path, config) error             │
│ + CreateSOPSConfig(path, config) error                     │
│ + ValidateEncryption(path, config) error                   │
│ + CreateSampleEncryptedSecrets(ctx, path, key) error       │
├─────────────────────────────────────────────────────────────┤
│ - keyManager: crypto.KeyManager                            │
│ - encryptor: Encryptor                                     │
│ - validator: Validator                                     │
│ - logger: *slog.Logger                                     │
└─────────────────────────────────────────────────────────────┘
```

**Design Pattern:** Facade + Dependency Injection
**Encryption:** Age (modern, secure)
**Validation:** Pre-encryption checks

### Template Engine

```
┌─────────────────────────────────────────────────────────────┐
│                   GoTemplateEngine                           │
├─────────────────────────────────────────────────────────────┤
│ + Render(ctx, path, data) ([]byte, error)                  │
│ + RenderString(ctx, name, content, data) ([]byte, error)   │
│ + ValidateTemplate(path) error                             │
│ + RegisterFunction(name, fn) void                          │
│ + EnableSandbox() void                                     │
├─────────────────────────────────────────────────────────────┤
│ - funcMap: template.FuncMap                                │
│ - cache: map[string]*template.Template                     │
│ - sandbox: *DefaultTemplateSandbox                         │
│ - cacheEnabled: bool                                       │
└─────────────────────────────────────────────────────────────┘
```

**Design Pattern:** Strategy + Decorator
**Functions:** Sprig v3 + custom
**Security:** Optional sandboxing

## Data Flow

### Cluster Initialization Flow

```
User Command
    │
    ├─> cluster init <name>
    │
    ▼
Parse Arguments
    │
    ├─> Validate cluster name
    ├─> Parse global flags
    │
    ▼
DI Container
    │
    ├─> Resolve ConfigManager
    ├─> Resolve PathResolver
    │
    ▼
Create Default Config
    │
    ├─> Load CLI defaults
    ├─> Apply provider defaults
    ├─> Generate service configs
    │
    ▼
Path Resolution
    │
    ├─> Determine organization
    ├─> Create directory structure
    │
    ▼
Save Configuration
    │
    ├─> Marshal to YAML
    ├─> Write to file
    ├─> Set permissions
    │
    ▼
Generate Keys (optional)
    │
    ├─> Generate Age key
    ├─> Generate SSH key
    │
    ▼
Output Success
```

### GitOps Generation Flow

```
User Command
    │
    ├─> cluster setup <name>
    │
    ▼
Load Configuration
    │
    ├─> Resolve config path
    ├─> Parse YAML
    ├─> Validate schema
    │
    ▼
Validation Pipeline
    │
    ├─> Schema validation
    ├─> Semantic validation
    ├─> Provider validation
    ├─> Network validation
    │
    ▼
Create Workspace
    │
    ├─> Create temp directory
    ├─> Initialize metadata
    │
    ▼
Pipeline Execution
    │
    ├─> Stage 1: Base Structure
    │   ├─> Create directories
    │   ├─> Copy base templates
    │   └─> Create checkpoint
    │
    ├─> Stage 2: Infrastructure
    │   ├─> Render OpenTofu configs
    │   ├─> Generate provider manifests
    │   └─> Create checkpoint
    │
    ├─> Stage 3: Services
    │   ├─> Resolve dependencies
    │   ├─> Render service manifests
    │   ├─> Apply overlays
    │   └─> Create checkpoint
    │
    ├─> Stage 4: Secrets
    │   ├─> Create SOPS config
    │   ├─> Encrypt sensitive files
    │   └─> Create checkpoint
    │
    └─> Stage 5: Finalization
        ├─> Validate manifests
        ├─> Generate README
        └─> Commit to Git
    │
    ▼
Output Repository
```

### Validation Flow

```
Configuration Input
    │
    ▼
Schema Validation
    │
    ├─> JSON Schema validation
    ├─> Required fields check
    ├─> Type validation
    │
    ▼
Semantic Validation
    │
    ├─> Cross-field validation
    ├─> Business rules
    ├─> Consistency checks
    │
    ▼
Provider Validation
    │
    ├─> OpenStack validation
    │   ├─> Region check
    │   ├─> Network validation
    │   └─> Flavor validation
    │
    ├─> AWS validation
    │   ├─> VPC validation
    │   └─> Subnet validation
    │
    └─> VMware validation
        ├─> Datacenter check
        └─> Datastore validation
    │
    ▼
Network Validation
    │
    ├─> CNI plugin validation
    ├─> CIDR overlap check
    ├─> Subnet validation
    │
    ▼
Service Validation
    │
    ├─> Dependency resolution
    ├─> Configuration validation
    ├─> Secret requirements
    │
    ▼
Validation Result
    │
    ├─> Errors (blocking)
    ├─> Warnings (non-blocking)
    └─> Suggestions (helpful)
```

## Component Interactions

### Configuration Loading Sequence

```
┌──────────┐         ┌──────────────┐         ┌──────────┐
│   CLI    │────────>│ ConfigManager│────────>│  Loader  │
└──────────┘         └──────────────┘         └──────────┘
     │                      │                       │
     │                      │                       ▼
     │                      │                  ┌──────────┐
     │                      │                  │   YAML   │
     │                      │                  │  Parser  │
     │                      │                  └──────────┘
     │                      │                       │
     │                      ▼                       ▼
     │               ┌──────────────┐         ┌──────────┐
     │               │  Validator   │<────────│  Config  │
     │               └──────────────┘         └──────────┘
     │                      │
     │                      ▼
     │               ┌──────────────┐
     │               │    Cache     │
     │               └──────────────┘
     │                      │
     ▼                      ▼
┌──────────┐         ┌──────────┐
│  Output  │<────────│  Config  │
└──────────┘         └──────────┘
```

### GitOps Generation Sequence

```
┌──────────┐         ┌──────────────┐         ┌──────────┐
│   CLI    │────────>│   Pipeline   │────────>│Workspace │
└──────────┘         │  Generator   │         │ Manager  │
                     └──────────────┘         └──────────┘
                            │                       │
                            ▼                       ▼
                     ┌──────────────┐         ┌──────────┐
                     │    Stage 1   │────────>│  Files   │
                     │ Base Structure│         └──────────┘
                     └──────────────┘
                            │
                            ▼
                     ┌──────────────┐         ┌──────────┐
                     │    Stage 2   │────────>│ Template │
                     │Infrastructure│         │  Engine  │
                     └──────────────┘         └──────────┘
                            │
                            ▼
                     ┌──────────────┐         ┌──────────┐
                     │    Stage 3   │────────>│ Services │
                     │   Services   │         │ Registry │
                     └──────────────┘         └──────────┘
                            │
                            ▼
                     ┌──────────────┐         ┌──────────┐
                     │    Stage 4   │────────>│   SOPS   │
                     │   Secrets    │         │ Manager  │
                     └──────────────┘         └──────────┘
                            │
                            ▼
                     ┌──────────────┐
                     │    Stage 5   │
                     │ Finalization │
                     └──────────────┘
                            │
                            ▼
                     ┌──────────────┐
                     │   GitOps     │
                     │  Repository  │
                     └──────────────┘
```

### Service Registration Flow

```
┌──────────────┐         ┌──────────────┐
│   Service    │────────>│   Registry   │
│   Plugin     │         └──────────────┘
└──────────────┘                │
                                ▼
                         ┌──────────────┐
                         │  Validate    │
                         │  Manifest    │
                         └──────────────┘
                                │
                                ▼
                         ┌──────────────┐
                         │   Resolve    │
                         │ Dependencies │
                         └──────────────┘
                                │
                                ▼
                         ┌──────────────┐
                         │   Register   │
                         │  Definition  │
                         └──────────────┘
```

## Design Patterns

### Dependency Injection
**Usage:** Component lifecycle management
**Implementation:** `internal/di/container.go`
**Benefits:**
- Testability
- Loose coupling
- Lifecycle management
- Thread safety

### Pipeline Pattern
**Usage:** GitOps generation
**Implementation:** `internal/gitops/pipeline.go`
**Benefits:**
- Staged execution
- Automatic rollback
- Progress tracking
- Checkpointing

### Strategy Pattern
**Usage:** Provider-specific logic
**Implementation:** Multiple provider packages
**Benefits:**
- Pluggable providers
- Isolated logic
- Easy extension

### Facade Pattern
**Usage:** Complex subsystem access
**Implementation:** ConfigManager, SOPSManager
**Benefits:**
- Simplified interface
- Encapsulation
- Reduced coupling

### Registry Pattern
**Usage:** Service management
**Implementation:** `internal/services/registry.go`
**Benefits:**
- Dynamic registration
- Dependency resolution
- Lifecycle hooks

### Builder Pattern
**Usage:** Configuration construction
**Implementation:** `internal/config/builder.go`
**Benefits:**
- Fluent API
- Validation
- Immutability

## Security Architecture

### Secrets Management

```
┌─────────────────────────────────────────────────────────────┐
│                    Secrets Architecture                      │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐         ┌──────────────┐                 │
│  │  Age Keys    │────────>│     SOPS     │                 │
│  │  Generation  │         │  Encryption  │                 │
│  └──────────────┘         └──────────────┘                 │
│         │                        │                          │
│         ▼                        ▼                          │
│  ┌──────────────┐         ┌──────────────┐                 │
│  │ Key Storage  │         │  Encrypted   │                 │
│  │  ~/.config/  │         │    Files     │                 │
│  └──────────────┘         └──────────────┘                 │
│                                  │                          │
│                                  ▼                          │
│                           ┌──────────────┐                 │
│                           │   GitOps     │                 │
│                           │  Repository  │                 │
│                           └──────────────┘                 │
└─────────────────────────────────────────────────────────────┘
```

**Key Features:**
- Age encryption (modern, secure)
- SOPS integration
- Encrypted at rest
- Key rotation support
- Barbican backend option

### Input Validation

**Layers:**
1. CLI argument validation
2. Schema validation (JSON Schema)
3. Semantic validation (business rules)
4. Provider-specific validation
5. Network configuration validation

**Security Checks:**
- Path traversal prevention
- Cluster name sanitization
- CIDR validation
- Credential masking in logs
- Template sandboxing

### Template Security

**Sandbox Features:**
- Disabled dangerous functions (env, exec, readFile)
- Timeout enforcement
- Resource limits
- Function whitelist
- Validation before execution

## Extension Points

### 1. Service Plugins

**Interface:** `ServicePlugin`
**Location:** `internal/services/plugin.go`

```go
type ServicePlugin interface {
    Name() string
    Type() ServiceType
    Validate(config interface{}) error
    Render(ctx context.Context, config interface{}, workspace interface{}) error
    Status(config interface{}) ServiceStatus
}
```

**Extension Steps:**
1. Implement ServicePlugin interface
2. Create service manifest
3. Register with ServiceRegistry
4. Provide templates
5. Define lifecycle hooks

### 2. Cloud Providers

**Interface:** Provider-specific
**Location:** `internal/cloud/`

**Extension Steps:**
1. Create provider package
2. Implement validation logic
3. Add configuration types
4. Provide OpenTofu modules
5. Register with validator

### 3. Provisioners

**Interface:** Provisioner-specific
**Location:** `internal/provision/`, `internal/ansible/`, `internal/talos/`

**Extension Steps:**
1. Create provisioner package
2. Implement provisioning logic
3. Add configuration types
4. Provide templates
5. Integrate with pipeline

### 4. Validation Rules

**Interface:** `ValidationRule`
**Location:** `internal/config/validator.go`

**Extension Steps:**
1. Implement validation function
2. Register with ValidationPipeline
3. Define error messages
4. Provide suggestions
5. Add tests

### 5. Template Functions

**Interface:** `template.FuncMap`
**Location:** `internal/template/engine.go`

**Extension Steps:**
1. Define function
2. Register with TemplateEngine
3. Document usage
4. Add to sandbox whitelist (if safe)
5. Provide examples

### 6. CLI Commands

**Interface:** `*cobra.Command`
**Location:** `cmd/`

**Extension Steps:**
1. Create command file
2. Define command structure
3. Implement RunE function
4. Add to root command
5. Write tests

## Performance Considerations

### Caching Strategy
- Configuration caching (5-minute TTL)
- Template caching (parse once)
- Service registry caching
- Path resolution caching

### Parallel Operations
- Parallel file encryption
- Concurrent template rendering
- Parallel validation checks

### Resource Management
- Workspace cleanup
- Checkpoint pruning
- Cache invalidation
- Memory-efficient streaming

### Metrics Collection
- Template render duration
- GitOps generation duration
- Validation duration
- File operation counts

## Testing Architecture

### Test Organization
- Unit tests: `*_test.go`
- Property tests: `*_property_test.go`
- Integration tests: `*_integration_test.go`
- BDD tests: `tests/features/*.feature`

### Test Patterns
- Table-driven tests
- Property-based testing (gopter)
- Mock interfaces
- Test fixtures in `testdata/`

### Coverage Goals
- Unit tests: >80%
- Integration tests: Critical paths
- BDD tests: User workflows
- Property tests: Complex logic

## Future Architecture

### Planned Enhancements
1. Metrics exporter (Prometheus)
2. Distributed tracing (OpenTelemetry)
3. Circuit breaker pattern
4. Retry with backoff
5. Drift detection
6. Backup management
7. Audit logging
8. Multi-cluster management

### Scalability Considerations
- Horizontal scaling (multiple clusters)
- Vertical scaling (large configurations)
- Distributed state management
- Event-driven architecture
- Async operations

## References

- [Development Guide](dev/readme.md)
- [Configuration System](explanation/configuration-system.md)
- [GitOps Workflow](explanation/gitops-workflow.md)
- [Plugin System](explanation/plugin-system.md)
- [Security Model](explanation/security-model.md)

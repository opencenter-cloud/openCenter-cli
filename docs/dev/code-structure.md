---
doc_type: explanation
title: "Codebase Organization"
audience: "developers"
---

# Codebase Organization

**Purpose:** For developers, explains how the openCenter-cli codebase is organized and where to find specific functionality.

## Repository Layout

```
openCenter-cli/
├── cmd/                    # CLI commands (Cobra)
├── internal/               # Internal packages
├── docs/                   # Documentation
├── tests/                  # BDD test scenarios
├── schema/                 # JSON schema definitions
├── testdata/               # Test fixtures
├── hack/                   # Scripts and utilities
├── bin/                    # Compiled binaries (gitignored)
├── third-party/            # External dependencies
├── main.go                 # Entry point
├── go.mod                  # Go module definition
└── .mise.toml              # Build tasks and tool versions
```

## Command Layer (cmd/)

Commands follow the pattern `cmd/<command>_<subcommand>.go`:

**Cluster commands** (`cluster_*.go`):
- `cluster_init.go` - Initialize cluster configuration
- `cluster_validate.go` - Validate configuration
- `cluster_setup.go` - Generate GitOps repository
- `cluster_bootstrap.go` - Deploy cluster
- `cluster_edit.go` - Edit configuration interactively
- `cluster_update.go` - Update configuration via flags
- `cluster_list.go` - List clusters
- `cluster_select.go` - Set active cluster
- `cluster_info.go` - Show cluster details
- `cluster_destroy.go` - Destroy cluster
- `cluster_preflight.go` - Run preflight checks
- `cluster_render.go` - Render templates without deploying

**Secrets commands** (`secrets_*.go`, `sops_*.go`):
- `cluster_check_keys.go` - Check SOPS key expiration
- `cluster_rotate_keys.go` - Rotate SOPS keys
- `cluster_validate_secrets.go` - Validate secrets encryption
- `cluster_sync_secrets.go` - Synchronize secrets

**Configuration commands** (`config_*.go`):
- `config_edit.go` - Edit global configuration

**Utility commands**:
- `version.go` - Show version information
- `shell_init.go` - Shell integration setup
- `plugins.go` - Plugin management

**Command naming convention:**
```go
// Function returns *cobra.Command
func newCluster<Action>Cmd() *cobra.Command {
    return &cobra.Command{
        Use:   "action",
        Short: "Brief description",
        RunE:  runClusterAction,
    }
}
```


## Internal Packages (internal/)

### Configuration (internal/config/)

Core configuration management:

- `config.go` - Main Config struct and types
- `schema.go` - JSON schema generation
- `validator.go` - Validation logic (schema + business rules)
- `loader.go` - Configuration loading from YAML
- `manager.go` - Configuration lifecycle management
- `path_resolver.go` - Organization-based path resolution
- `migrator.go` - Schema migration between versions
- `defaults/` - Default configuration templates per provider
- `v2/` - Version 2 configuration structs

**Key types:**
```go
type Config struct {
    OpenCenter OpenCenterConfig
    OpenTofu   OpenTofuConfig
    Deployment DeploymentConfig
    Metadata   MetadataConfig
    Secrets    SecretsConfig
}

type OpenCenterConfig struct {
    Meta           MetaConfig
    Secrets        SecretsBackendConfig
    Infrastructure InfrastructureConfig
    Cluster        ClusterConfig
    GitOps         GitOpsConfig
    Storage        StorageConfig
    Services       ServicesConfig
}
```

### GitOps (internal/gitops/)

GitOps repository scaffolding:

- `copy.go` - Template copying and rendering logic
- `embed.go` - Embedded template management (`//go:embed`)
- `gitops-base-dir/` - Base repository structure (embedded)
- `templates/` - Cluster-specific templates (embedded)

**Template structure:**
```
gitops-base-dir/
├── applications/
│   └── base/
│       └── services/
│           ├── cert-manager/
│           ├── kyverno/
│           └── ...
└── infrastructure/
    └── clusters/
        ├── openstack/
        ├── vmware/
        └── ...
```

### Secrets (internal/sops/)

SOPS and Age key management:

- `manager.go` - SOPS manager interface
- `keys.go` - Age key generation and storage
- `encrypt.go` - Encryption/decryption operations
- `git.go` - Git integration for encrypted files
- `validator.go` - SOPS configuration validation

### Providers (internal/cloud/, internal/provision/)

Cloud provider adapters:

- `internal/cloud/openstack/` - OpenStack preflight checks
- `internal/provision/` - Terraform/OpenTofu provisioning
- `internal/ansible/` - Ansible provisioning (Kubespray)

### Security (internal/security/)

Security utilities:

- `input_validator.go` - Input validation (path traversal, injection)
- `command_sanitizer.go` - Command sanitization
- `credential_masker.go` - Credential masking in logs
- `audit_logger.go` - Audit logging with HMAC signatures

### Utilities (internal/util/)

Shared utility packages:

- `crypto/` - Key generation and management
- `errors/` - Error handling and aggregation
- `files/` - File operations (atomic writes, backups)
- `paths/` - Path resolution and validation
- `template/` - Template engine and validation

### Other Packages

- `internal/cluster/` - Cluster lifecycle management
- `internal/credentials/` - Credential export/unset
- `internal/di/` - Dependency injection
- `internal/observability/` - Metrics and logging
- `internal/operations/` - Backup, drift detection
- `internal/plugins/` - Plugin system
- `internal/resilience/` - Retry, circuit breaker, locking
- `internal/services/` - Service management
- `internal/template/` - Template sandbox
- `internal/testing/` - Test utilities
- `internal/ui/` - TUI components (Bubble Tea)

## Testing (tests/)

BDD tests using Cucumber/Gherkin:

```
tests/
└── features/
    ├── workflow.feature
    ├── cluster_init.feature
    ├── validation.feature
    └── steps/
        ├── cluster_steps.go
        ├── config_steps.go
        └── test_suite.go
```

**Tag convention:**
- `@wip` - Work in progress scenarios
- `@priority1` - High priority tests
- `@priority2` - Medium priority tests

## Configuration Storage

User configurations stored in organization-based structure:

```
~/.config/opencenter/clusters/
└── <organization>/
    ├── <cluster>/
    │   └── .<cluster>-config.yaml
    ├── secrets/
    │   ├── age/
    │   │   └── <cluster>-key.txt
    │   └── ssh/
    │       └── <cluster>-<env>-<region>
    └── gitops/
        ├── applications/
        └── infrastructure/
```

## Code Organization Principles

### Separation of Concerns

Each package has a single, well-defined responsibility:
- `cmd/` - CLI interface and user interaction
- `internal/config/` - Configuration management
- `internal/gitops/` - GitOps repository generation
- `internal/sops/` - Secrets encryption
- `internal/cloud/` - Provider-specific logic

### Dependency Injection

Avoid global state, pass dependencies explicitly:

```go
// Good: Dependencies injected
func NewValidator(schema *jsonschema.Schema) *Validator {
    return &Validator{schema: schema}
}

// Bad: Global state
var globalSchema *jsonschema.Schema
```

### Interface-Based Design

Define interfaces in consumer packages:

```go
// internal/config/interfaces.go
type SecretManager interface {
    Encrypt(data []byte) ([]byte, error)
    Decrypt(data []byte) ([]byte, error)
}

// internal/sops/manager.go implements SecretManager
```

### Embedded Resources

Templates and defaults embedded in binary via `//go:embed`:

```go
//go:embed gitops-base-dir
var gitopsBaseFS embed.FS

//go:embed templates
var templatesFS embed.FS
```

### Error Wrapping

Use `fmt.Errorf` with `%w` for error context:

```go
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}
```

## File Naming Conventions

- Commands: `<noun>_<verb>.go` (e.g., `cluster_init.go`)
- Tests: `<name>_test.go` (unit), `<name>_property_test.go` (property)
- Interfaces: `interfaces.go` in each package
- Documentation: `doc.go` for package documentation

## Finding Functionality

**To add a new command:**
1. Create `cmd/cluster_<action>.go`
2. Implement `newCluster<Action>Cmd()`
3. Register in `cmd/cluster.go`

**To modify configuration:**
1. Update `internal/config/config.go` (structs)
2. Update `internal/config/schema.go` (JSON schema)
3. Update `internal/config/defaults.go` (defaults)

**To add a provider:**
1. Create `internal/cloud/<provider>/preflight.go`
2. Add defaults in `internal/config/defaults.go`
3. Register in `cmd/cluster_preflight.go`

**To add a service:**
1. Add defaults in `internal/config/defaults.go`
2. Create templates in `internal/gitops/gitops-base-dir/`
3. Add validation in `internal/config/service_validator.go`

**To add validation:**
1. Update `internal/config/validator.go` (business rules)
2. Update `internal/config/schema.go` (schema constraints)
3. Add provider-specific validation in `internal/config/<provider>_validator.go`

## Code Metrics

- **Total lines:** 147,952 LOC
- **Go files:** 628 files
- **Test files:** 276 files
- **Internal packages:** 25 packages
- **Commands:** 70+ command files
- **Dependencies:** 19 direct dependencies

## Architecture Patterns

### Command Pattern

Commands encapsulate operations:
```go
type Command interface {
    Execute() error
}
```

### Repository Pattern

Configuration storage abstracted:
```go
type ConfigRepository interface {
    Load(path string) (*Config, error)
    Save(path string, cfg *Config) error
}
```

### Template Method Pattern

Base template with provider-specific overrides:
```go
func GenerateInfrastructure(provider string) error {
    // Common steps
    loadConfig()
    validateConfig()
    
    // Provider-specific
    switch provider {
    case "openstack":
        generateOpenStackTerraform()
    case "aws":
        generateAWSTerraform()
    }
    
    // Common steps
    writeFiles()
}
```

---

## Evidence

This documentation is based on the following repository files:

- Project structure: `.kiro/steering/structure.md:1-128`
- Command layer: `cmd/` directory (70+ files)
- Internal packages: `internal/` directory (25 packages)
- Configuration: `internal/config/` directory
- GitOps: `internal/gitops/` directory
- Testing: `tests/features/` directory
- Code metrics: Session 1 summary (A1)
- Organization principles: `.kiro/steering/structure.md:130-145`

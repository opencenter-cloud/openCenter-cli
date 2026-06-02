---
id: code-structure
title: "Codebase Organization"
sidebar_label: Codebase Organization
description: Map of the openCenter-cli repository: top-level layout, package boundaries, and where to find each subsystem.
doc_type: explanation
audience: "developers"
tags: [contributing]
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

* `cluster_init.go` - Initialize cluster configuration
* `cluster_validate.go` - Validate configuration
* `cluster_generate.go` - Generate GitOps repository
* `cluster_deploy.go` - Deploy cluster (no longer auto-commits)
* `cluster_deploy_plan.go` - Deploy dry-run plan formatting
* `cluster_edit.go` - Edit configuration interactively
* `cluster_set.go` - Update configuration via dot-notation flags
* `cluster_list.go` - List clusters
* `cluster_use.go` - Set active cluster
* `cluster_describe.go` - Show cluster details
* `cluster_status.go` - Show cluster status with inventory
* `cluster_destroy.go` - Destroy cluster
* `cluster_render.go` - Render templates without deploying
* `cluster_drift.go` - Infrastructure drift detection/reconciliation
* `cluster_service.go` - Service enable/disable/status
* `cluster_backup.go` - Backup/restore operations

**Secrets commands** (`secrets_*.go`):

* `secrets.go` - Secrets parent + CRUD subcommands
* `secrets_keys.go` - Key lifecycle subcommands
* `secrets_keys_ops.go` - Key operations (generate, rotate, backup, validate)
* `secrets_sync.go` - Synchronize secrets to encrypted manifests
* `secrets_validate.go` - Validate secrets encryption
* `secrets_sops.go` - Encrypt/decrypt/status commands

**Configuration commands** (`config_*.go`):

* `config.go` - Settings parent command
* `config_edit.go` - Edit global configuration
* `config_ide.go` - IDE schema generation
* `config_explain.go` - Explain config effects

**Utility commands**:

* `version.go` - Show version information
* `shell_init.go` - Shell integration setup
* `plugins.go` - Plugin management

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

CLI settings management (the legacy monolithic config package was removed in May 2026):

* `cli_settings.go` - CLI user preferences, cluster defaults, plugin checksums
* `cli_settings_helpers.go` - Path resolution from CLI config (ResolveClustersDir, GetGitOpsDir, etc.)
* `manager.go` - Global manager singleton
* `persistence.go` - Config/state directory resolution
* `status.go` - Cluster status updates
* `defaults/` - Default configuration templates per provider-region
* `v2/` - Authoritative v2 configuration (loader, validator, manager, cache, errors, io_handler)
* `flags/` - CLI flag parsing and struct mutation via reflection
* `services/` - Typed service configs with dependency/provider validation

**Key types (v2):**

```go
type Config struct {
    Meta           MetaConfig
    OpenCenter     OpenCenterConfig
    Infrastructure InfrastructureConfig
    Kubernetes     KubernetesConfig
    GitOps         GitOpsConfig
    Services       ServiceMap
    Secrets        SecretsConfig
    OpenTofu       OpenTofuConfig
    Deployment     DeploymentConfig
}
```

### GitOps (internal/gitops/)

GitOps repository scaffolding:

* `copy.go` - Template copying and rendering logic
* `embed.go` - Embedded template management (`//go:embed`)
* `gitops-base-dir/` - Base repository structure (embedded)
* `templates/` - Cluster-specific templates (embedded)

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

* `manager.go` - SOPS manager interface
* `keys.go` - Age key generation and storage
* `encrypt.go` - Encryption/decryption operations
* `git.go` - Git integration for encrypted files
* `validator.go` - SOPS configuration validation

### Providers (internal/cloud/, internal/provision/)

Cloud provider adapters:

* `internal/cloud/openstack/` - OpenStack drift detection + discovery
* `internal/cloud/vmware/` - VMware/vSphere drift detection
* `internal/cloud/kind/` - Kind cluster lifecycle
* `internal/cloud/` - Provider factory, drift comparison
* `internal/provision/` - Embedded OpenTofu/Terraform provisioning templates

### Security (internal/security/)

Security utilities:

* `input_validator.go` - Input validation (path traversal, injection)
* `command_sanitizer.go` - Command sanitization
* `credential_masker.go` - Credential masking in logs
* `audit_logger.go` - Audit logging with HMAC signatures

### Utilities (internal/util/)

Shared utility packages:

* `crypto/` - Key generation and management
* `errors/` - Error handling and aggregation
* `files/` - File operations (atomic writes, backups)
* `paths/` - Path resolution and validation
* `template/` - Template engine and validation

### Other Packages

* `internal/ansible/` - Kubespray inventory generation from config
* `internal/barbican/` - OpenStack Key Manager (Barbican) client
* `internal/cluster/` - Cluster lifecycle services (init, validate, setup, bootstrap, destroy)
* `internal/core/` - Shared path resolution (`core/paths`) and validation engine (`core/validation`)
* `internal/credentials/` - Cloud credential extraction from config
* `internal/di/` - Dependency injection container (App struct + reflection-based Container)
* `internal/importer/` - Live cluster import/scan for existing workloads
* `internal/localdev/` - Local dev environment (Kind, Gitea, Flux)
* `internal/logging/` - Structured logging (global logger, level/format reconfiguration)
* `internal/observability/` - Log shipping (Loki, syslog), migration helpers
* `internal/operations/` - Drift detection, backup, disaster recovery
* `internal/plugins/` - External CLI plugin discovery and checksum verification
* `internal/resilience/` - Retry, circuit breaker, distributed locks
* `internal/secrets/` - Multi-cluster secrets management (rotation, registry, hooks, revocation)
* `internal/services/` - Platform service plugin registry with dependency resolution
* `internal/template/` - Template engine with caching, validation, sandboxing
* `internal/testenv/` - Test environment helpers (isolated CLI config/state)
* `internal/testing/` - Shared test utilities (helpers, mocks, generators, benchmarks)
* `internal/tofu/` - OpenTofu/Terraform provisioning execution (falls back to terraform)
* `internal/ui/` - Prompts, error formatting, guided flows

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

* `@wip` - Work in progress scenarios
* `@priority1` - High priority tests
* `@priority2` - Medium priority tests

## Configuration Storage

User configurations stored in organization-based structure:

```
~/.config/opencenter/clusters/
└── <organization>/
    ├── .<cluster>-config.yaml       # Cluster configuration (dot-prefixed)
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

* `cmd/` - CLI interface and user interaction
* `internal/config/` - Configuration management
* `internal/gitops/` - GitOps repository generation
* `internal/sops/` - Secrets encryption
* `internal/cloud/` - Provider-specific logic

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

* Commands: `<noun>_<verb>.go` (e.g., `cluster_init.go`, `secrets_keys_ops.go`)
* Tests: `<name>_test.go` (unit), `<name>_property_test.go` (property-based)
* Integration tests: `<name>_integration_test.go`
* Interfaces: `interfaces.go` in each package
* Documentation: `doc.go` for package documentation

## Finding Functionality

**To add a new command:**

1. Create `cmd/cluster_<action>.go`
2. Implement `newCluster<Action>Cmd()`
3. Register in `cmd/cluster.go`

**To modify configuration:**

1. Update `internal/config/v2/config.go` (structs)
2. Update `internal/config/v2/validator.go` (validation rules)
3. Update `internal/config/v2/defaults.go` (defaults)
4. Run `go generate ./internal/config/v2schema/` to regenerate JSON schema

**To add a provider:**

1. Create `internal/cloud/<provider>/provider.go`
2. Add defaults in `internal/config/defaults/<provider>.go`
3. Add bootstrap steps in `internal/cluster/bootstrap_provider_infra.go`
4. Add template in `internal/gitops/templates/infrastructure-cluster-template/`

**To add a service:**

1. Add service config in `internal/config/services/`
2. Create plugin in `internal/services/plugins/`
3. Add templates in `internal/gitops/gitops-base-dir/`
4. Add descriptor in `internal/services/descriptors/`

**To add validation:**

1. Create validator in `internal/core/validation/validators/`
2. Register in `internal/di/providers.go`

## Code Metrics

* **Total lines:** ~226,000 LOC
* **Go files:** ~710 files
* **Test files:** ~350 files
* **Internal packages:** 30+ packages
* **Commands:** 50+ command files
* **Dependencies:** 19 direct dependencies

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

* Project structure: `.kiro/steering/structure.md:1-128`
* Command layer: `cmd/` directory (70+ files)
* Internal packages: `internal/` directory (25 packages)
* Configuration: `internal/config/` directory
* GitOps: `internal/gitops/` directory
* Testing: `tests/features/` directory
* Code metrics: Session 1 summary (A1)
* Organization principles: `.kiro/steering/structure.md:130-145`
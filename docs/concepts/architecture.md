---
last_updated: 2026-10-22
id: architecture
title: "Architecture: rationale and design"
sidebar_label: Architecture (concepts)
description: System design, core components, and architectural decisions behind openCenter.
doc_type: explanation
audience: "architects, developers"
tags: [architecture, design, components, patterns]
---
# Architecture

**Role:** Rationale-focused conceptual architecture for architects and operators. It explains why openCenter is designed this way, including trade-offs and design principles; it is not the source-grounded contributor package map.

Understanding openCenter’s architecture helps you make informed decisions about deployment, customization, and troubleshooting. This explanation covers the key architectural patterns and design choices.

This page explains *why* the system is built this way, for architects and operators. For the terse, source-grounded package map used by contributors and code-oriented agents, see [Architecture](../architecture.md); for a package-by-package breakdown of each subsystem, see the [CODEMAPS index](../CODEMAPS/INDEX.md).

## System Overview

openCenter follows a **configuration-first, GitOps-native** architecture where a single YAML file drives the entire cluster lifecycle. The system transforms declarative configuration into production infrastructure through multiple layers of abstraction.

```
Cluster Configuration (v2 YAML)
    ↓
Validation Engine (Schema + Business Rules)
    ↓
GitOps Planner and Renderers
    ↓
GitOps Repository (Infrastructure + Applications)
    ↓
Provider-specific lifecycle (OpenTofu/Kubespray, Kind, or Magnum)
    ↓
Production Cluster (Kubernetes + Services)
```

## Core Components

### Configuration Manager

**Purpose:** Load, validate, and manage cluster configurations.

**Design:** The configuration manager uses a six-step loading pipeline (in `internal/config/v2/loader.go`):

1. **Parse YAML** -- Decode raw YAML into intermediate representation
2. **Normalize** -- Canonicalize provider names, resolve aliases
3. **Resolve References** -- Expand `${ref:path}`, `${env:VAR}`, `${file:path}` with dependency graph and cycle detection
4. **Apply Defaults** -- Hydrate empty fields from the provider default map in `internal/config/v2/defaults.go`
5. **Validate** -- schema, business rules, provider/deployment rules, and service checks
6. **Freeze** -- marks the result ready for use (Go does not enforce immutability)

There are two distinct precedence systems. The v2 loader hydrates a loaded
cluster file with provider/region defaults without overwriting explicit fields;
`${env:VAR}` references are resolved only when the configuration explicitly
uses that reference syntax. The `cluster init`/`cluster set` flag merger uses,
from lowest to highest precedence, built-in defaults, the existing file,
template-derived values, and CLI overrides. CLI settings are inputs to initial
default construction and path resolution, not a higher-precedence replacement
for fields in an already loaded cluster config.

**Why this design:** The pipeline ensures every config is fully resolved and validated before use. Reference resolution with topological sort prevents circular dependencies. Hydration fills gaps without overwriting explicit values.

**Evidence:** `internal/config/v2/loader.go`, `internal/config/v2/manager.go`, `internal/config/v2/defaults.go`. The top-level `internal/config` package is now a thin compatibility layer over `internal/config/v2` — for example `internal/config/manager.go` just re-exports `v2.ConfigurationManager` — plus CLI settings (`internal/config/cli_settings.go`). The v2 package is the authoritative implementation; see [CODEMAPS: Config system](../CODEMAPS/config-system.md).

### Validation Engine

**Purpose:** Ensure configuration correctness before deployment.

**Design:** Multi-layered validation with progressive checks:

1. **Schema Validation:** JSON schema compliance (structure, types, formats)
2. **Business Rules:** Cross-field dependencies (e.g., VRRP IP required when Octavia disabled)
3. **Provider Validation:** Provider-specific constraints (image IDs, flavors, networks)
4. **Online checks:** API reachability and provider discovery are optional and
   are run by validation service online mode, not by every v2 load.

**Why this design:** Catch errors early (fail fast) with increasingly specific checks. Schema validation is fast and catches 80% of errors. Business rules catch logical inconsistencies. Provider validation catches deployment-time failures before provisioning.

**Trade-offs:** More validation means slower feedback, but prevents costly deployment failures. Connectivity validation is optional because it requires credentials and network access.

**Evidence:** `internal/config/v2/validator.go`, `internal/config/v2/deployment_validator.go`, `internal/core/validation/`, `cmd/cluster_validate.go`

### Template Engine

**Purpose:** Generate infrastructure and application manifests from configuration.

**Design:** Embedded templates with Go’s `text/template` and Sprig functions:

* Templates embedded in binary (`//go:embed`)
* Configuration values injected via template variables
* Sprig functions for string manipulation, encoding, etc.
* No hardcoded values in templates

**Why this design:** Templates are version-controlled with the CLI, ensuring consistency. Embedding eliminates external dependencies. Sprig provides rich template functions without custom code.

**Trade-offs:** Templates are less flexible than code but more maintainable. Embedded templates are versioned and distributed with the CLI binary, keeping the rendered-resource source aligned with that binary.

**Evidence:** `internal/gitops/copy.go`, `internal/template/`

### GitOps Repository Generator

**Purpose:** Create complete GitOps repository structure.

**Design:** Standardized directory layout with Kustomize overlays:

```
<clusters-dir>/
├── blueprints/<organization>/<cluster>/<cluster>-config.yaml  # local input
├── gitops/<organization>/                                     # Git repository
│   ├── applications/overlays/<cluster>/                       # Flux/application output
│   └── infrastructure/clusters/<cluster>/                     # provider output + gitignored runtime state
└── secrets/<organization>/<cluster>/                          # Age/SSH keys and kubeconfig
```

The GitOps tree contains generated repository content and self-contained runtime build artifacts
(`inventory/`, `venv/`, `kubespray/`, `.bin/`, `kubespray-outputs.json`) ignored by `.gitignore`.
Admin credentials (`kubernetes/kubeconfig.yaml`) and private keys are deliberately isolated in the secrets zone.
is skipped for Kind and Magnum, and lifecycle implementation differs by
provider.

**Why this design:** Separation of infrastructure (Terraform) and applications (Kubernetes manifests) allows different teams to manage different layers. Kustomize overlays enable cluster-specific customization without duplicating base manifests.

**Trade-offs:** More directories and files, but clear separation of concerns. Overlay pattern requires understanding Kustomize, but provides powerful composition.

**Evidence:** `internal/gitops/`, `docs/CODEMAPS/gitops-engine.md`

### Secrets Management

**Purpose:** Manage secrets encryption, rotation, and lifecycle.

**Design:** Two-layer architecture:

1. **`internal/sops/`** -- Low-level SOPS/Age encryption operations (encrypt/decrypt files, key generation, OS keyring integration)
2. **`internal/secrets/`** -- High-level multi-cluster secrets management (sync, drift detection, rotation, revocation, registry, Git hooks)

Encryption strategy:

* **In Git:** SOPS Age encryption (secrets safe to commit)
* **In Cluster:** Kubernetes encryption at rest (etcd encrypted)
* **In Transit:** FluxCD decrypts on-the-fly during reconciliation

Key management features:

* OS keyring integration with file-based fallback
* Multi-recipient Age encryption: multiple active Age keys may decrypt a file simultaneously; there is no fixed maximum
* Primary-based rotation: the active primary key is replaced through a dual-key period while unrelated active recipients remain available
* Key expiration monitoring (Age 90 days, SSH 180 days)
* Git pre-commit hooks preventing plaintext secret commits
* HMAC-signed audit logging for tamper detection
* Reconciliation of `.sops.yaml` recipients with the key registry before destructive operations

**Why this design:** Secrets can be version-controlled safely. FluxCD handles decryption automatically. Age keys are simpler than GPG (no key servers). Primary-based rotation allows gradual re-encryption without removing unrelated recipients, while rollback keeps registry and encrypted-file changes consistent.

**Evidence:** `internal/sops/`, `internal/secrets/`, `internal/security/audit_logger.go`

### Dependency Injection and Runtime Wiring

**Purpose:** Manage service dependencies and lifecycle.

**Design:** Two approaches coexist, with different scope:

1. **`App` struct** (preferred) -- Explicit constructor chaining with typed fields. Built via `di.NewApp(baseDir)`.
2. **`DIContainer`** (legacy/compatibility) -- Reflection-based resolution matching constructor parameter types to registered return types. `SetupContainer` registers only a partial set of components; it is not the complete runtime graph.

Key properties:

* Services registered as factory functions (provider pattern)
* Dependencies resolved by type matching
* Legacy-container singletons are initialized eagerly via `Initialize()`; the typed `App` is built by explicit constructor chaining.
* Circular dependencies detected via topological ordering
* Thread-safe after initialization (`sync.RWMutex`)
* Graceful shutdown calling `Shutdown()` on components

**Why this design:** Testability (mock dependencies), flexibility (swap implementations), and explicit dependencies. The typed `App` struct provides compile-time wiring for the current command path, while the reflection container preserves older callers and tests. This is a partial migration, not a claim that all packages are injected.

**Evidence:** `internal/di/`, `cmd/root.go`

### Cluster Lifecycle Services

**Purpose:** Orchestrate the full cluster lifecycle from initialization to destruction.

**Design:** Domain services separated from CLI layer for testability:

* `InitService` -- Create config, generate SSH/Age keys, create directory structure
* `ConfigureService` -- Interactive guided configuration with provider discovery
* `ValidateService` -- Schema + business + connectivity + provider validation
* `SetupService` -- Generate GitOps repository via pipeline
* `BootstrapService` -- Provision infrastructure + deploy cluster with resume support

**Why this design:** Each service handles one lifecycle stage with clear inputs/outputs. Resume support (JSON state file) allows restarting failed deployments without re-running completed steps.

**Evidence:** `internal/cluster/`, `cmd/cluster*.go`

## Package Map

For a complete architectural map of all packages, see the [CODEMAPS index](../CODEMAPS/INDEX.md). For the terse contributor/agent-oriented package map, see [Architecture](../architecture.md).

Key packages by responsibility:

| Layer | Packages |
| --- | --- |
| CLI | `cmd/` (Cobra commands), `internal/ui` (prompts), `internal/plugins` (external plugins) |
| Domain | `internal/cluster` (lifecycle), `internal/secrets` (secrets mgmt), `internal/operations` (drift, backup) |
| Config | `internal/config/v2` (authoritative typed model, loader, defaults, validation), `internal/config` (CLI settings and compatibility re-exports), `internal/config/services` (typed service configs and enforced service dependencies) |
| GitOps | `internal/gitops` (live rendering, templates, atomic output), `internal/template` (supporting template engine) |
| Infra | `internal/cloud`, `internal/cloud/openstack`, `internal/cloud/vmware`, `internal/cloud/kind`, `internal/cloud/magnum` (providers), `internal/provision` (embedded provisioning templates, including Ansible/Kubespray inventory assets), `internal/tofu` (OpenTofu) |
| Security | `internal/security` (audit, masking, sanitization), `internal/sops` (encryption) |
| Foundation | `internal/di` (DI container), `internal/core` (paths, validation), `internal/util` (shared), `internal/resilience` (locks, retry); `internal/services` is a separate, unwired service-plugin subsystem |

There is no dedicated `internal/ansible` package. Kubespray is invoked as an embedded `local-exec` provisioner inside the generated OpenTofu module (part of the `opentofu-apply` bootstrap step), using inventory templates embedded via `internal/provision` and `internal/gitops/templates/infrastructure-cluster-template/`; see `internal/cluster/bootstrap_provider_infra.go`.

## Architectural Patterns

### Configuration as Code

**Pattern:** All cluster state defined in version-controlled YAML.

**Benefits:**

* Reproducible deployments
* Audit trail (Git history)
* Rollback capability (Git revert)
* Collaboration (pull requests)

**Constraints:**

* Configuration must be complete (no implicit state)
* Changes require validation before apply
* Secrets must be encrypted

**Evidence:** `internal/config/v2/`, `internal/gitops/`

### GitOps Native

**Pattern:** Git is the source for generated cluster/platform state; FluxCD
reconciles the generated GitOps tree. Local blueprints and runtime state remain
outside that repository.

**Benefits:**

* Declarative (describe what, not how)
* Self-healing (FluxCD corrects drift)
* Auditable (all changes in Git)
* Secure (no direct cluster access needed)

**Constraints:**

* Git repository required
* FluxCD must be running
* Changes take time to reconcile (5-15 minutes)

**Evidence:** [GitOps Workflow](gitops-workflow.md), `internal/gitops/`

### Provider Abstraction

**Pattern:** Provider-specific logic isolated in adapters.

**Benefits:**

* Add new providers without changing core
* Test providers independently
* Swap providers without rewriting

**Constraints:**

* Common interface limits provider-specific features
* Abstraction adds complexity
* Not all providers have same capabilities

**Evidence:** `internal/cloud/`, `internal/provision/`, `docs/CODEMAPS/providers.md`

### Layered Validation

**Pattern:** Multiple validation layers with increasing specificity.

**Benefits:**

* Fast feedback (schema validation is instant)
* Specific errors (business rules explain why)
* Prevent deployment failures (provider validation)

**Constraints:**

* More code to maintain
* Validation can be slow (connectivity checks)
* False positives possible (stale provider data)

**Evidence:** `internal/config/v2/validator.go`, `internal/core/validation/`

### Embedded Resources

**Pattern:** Templates and defaults embedded in binary.

**Benefits:**

* No external dependencies
* Version-locked (templates match CLI version)
* Offline capable
* Single binary distribution

**Constraints:**

* Embedded resources are packaged into the binary and versioned with that binary
* Binary size increases
* Cannot customize without forking

**Evidence:** `internal/gitops/embed.go`

## Design Principles

### 1. Declarative Over Imperative

**Principle:** Describe desired state, not steps to achieve it.

**Example:** Configuration specifies "3 control plane nodes" not "create node 1, create node 2, create node 3."

**Rationale:** Declarative is idempotent (safe to re-apply), easier to understand (what not how), and enables automation (reconciliation loops).

**Evidence:** `internal/config/v2/` typed model

### 2. Fail Fast

**Principle:** Catch errors as early as possible.

**Example:** Schema validation before business rules before provider checks before deployment.

**Rationale:** Faster feedback loop, cheaper to fix (no infrastructure provisioned), clearer error messages (specific validation layer).

**Evidence:** `internal/config/v2/validator.go`, `internal/core/validation/`

### 3. Composition Over Inheritance

**Principle:** Build complex behavior from simple components.

**Example:** Kustomize overlays compose base + cluster-specific configuration rather than inheriting from base classes.

**Rationale:** More flexible (mix and match), easier to understand (explicit composition), avoids deep hierarchies.

**Evidence:** [GitOps Workflow](gitops-workflow.md) Kustomize overlay pattern

### 4. Explicit Dependencies

**Principle:** Dependencies injected, not instantiated internally.

**Example:** Validation engine receives validators as parameters, not creating them internally.

**Rationale:** Testability (mock dependencies), flexibility (swap implementations), clarity (dependencies visible in signatures).

**Evidence:** `internal/di/`

### 5. Security First

**Principle:** Secure by default, with secret-bearing generated artifacts
handled by SOPS/Age flows.

**Example:** SOPS/Age encryption is used for generated secret-bearing artifacts
before they are promoted to the GitOps tree.

**Rationale:** Prevent accidental exposure, enforce best practices, compliance requirements.

**Evidence:** `internal/sops/`, `internal/secrets/`

## Data Flow

### Initialization Flow

```
User: opencenter cluster init my-cluster --org my-org
    ↓
CLI: Load defaults from internal/config/v2/defaults.go
    ↓
CLI: Resolve zone paths and apply selected CLI settings/defaults
    ↓
CLI: Generate configuration file
    ↓
CLI: Write to <blueprints-dir>/my-org/my-cluster/my-cluster-config.yaml
    ↓
User: Configuration ready for editing
```

### Validation Flow

```
User: opencenter cluster validate my-cluster
    ↓
Validation Engine: Load configuration
    ↓
Schema Validator: Check JSON schema compliance
    ↓
Business Rules Validator: Check cross-field dependencies
    ↓
Provider Validator: Check provider-specific constraints
    ↓
Online validation (when requested): Check reachability and provider discovery
    ↓
CLI: Report validation results
```

### Setup Flow

```
User: opencenter cluster generate my-cluster
    ↓
Template Engine: Load embedded templates
    ↓
Template Engine: Inject configuration values
    ↓
Template Engine: Render to GitOps repository
    ↓
SOPS Manager: Encrypt secret-bearing generated artifacts
    ↓
GitOps workspace: Promote generated output with ownership checks
    ↓
CLI: Repository ready for the operator to commit and push
```

### Bootstrap Flow

```
User: opencenter cluster deploy
    ↓
Provider lifecycle: provision infrastructure where supported
    ↓
Kubespray/Kind/Magnum: provider-specific Kubernetes bring-up
    ↓
FluxCD: Bootstrap GitOps (install controllers, create sources)
    ↓
FluxCD: Reconcile services (deploy platform services)
    ↓
CLI: Cluster ready
```

## Scalability Considerations

The repository defines configuration, filesystem-layout, and service-rendering
contracts, but does not publish general capacity maxima. Capacity planning
therefore remains dependent on the selected provider, Kubernetes, and the
services enabled in the configuration. See the
[Platform Services Architecture](../reference/platform-services.md) for the
live service model rather than inferring capacity from package names or default
maps.

## Extension Points

### Custom Providers

Provider support is split into independent extension points:

1. Add typed configuration and validation under `internal/config/v2/`.
2. Add guided configuration and lifecycle bootstrap/destroy dispatch under
   `internal/cluster/` when the provider owns those operations.
3. Add an isolated `internal/cloud/<provider>/` client only when the provider
   API needs one.
4. Decide separately whether to implement the `internal/cloud.CloudProvider`
   drift interface; lifecycle support does not imply drift support.

**Evidence:** `internal/cloud/`, `internal/provision/`

### Custom Services

Add new platform services by:

1. Create service configuration in `internal/config/services/<service>.go`
2. Add the service to the default service map in `internal/config/v2/defaults.go`
3. Create service manifests in openCenter-gitops-base
4. Update documentation

See [Platform Services Architecture](../reference/platform-services.md) for the
live service configuration and rendering model, and [Adding Services](../contributing/adding-services.md)
for the contributor contract. The `internal/services` plugin interfaces and
`gitops/stages.ServiceStage` are supporting, unwired APIs rather than the live
service-generation path.

**Evidence:** `internal/config/services/`, `internal/config/v2/defaults.go`

### Custom Validators

Add new validation rules by:

1. Implement validator interface in `internal/core/validation/validators/`
2. Register validator with validation engine
3. Add tests for validator

**Evidence:** `internal/core/validation/`

### Plugins

Extend CLI with external plugins:

1. Create executable named `opencenter-<plugin>`
2. Place in PATH
3. CLI discovers and loads automatically

Production starts with `NewBuiltinRootCmd` and then attaches external plugins.
The documentation generator also starts with `NewBuiltinRootCmd`, so generated
command reference pages exclude external plugins.

**Evidence:** `internal/plugins/`, `cmd/plugins.go`

## Common Misconceptions

### "openCenter is just a wrapper around Terraform"

**Reality:** openCenter orchestrates multiple tools (Terraform, Kubespray, FluxCD) and provides validation, secrets management, and GitOps scaffolding. Terraform is one component.

### "Configuration changes have one universal lifecycle"

**Reality:** The repository provides separate validation, generation, and deployment commands. The applicable workflow depends on the configuration change and selected provider; consult [Configuration Lifecycle](configuration-lifecycle.md) rather than assuming a universal in-place update or rebuild rule.

### "GitOps means no manual changes"

**Reality:** GitOps means Git is the source of truth, but manual changes are possible (and sometimes necessary) for debugging. They’ll be reverted on next reconciliation unless committed to Git.

### "All secrets must be in configuration file"

**Reality:** Secret inputs and generated secret-bearing artifacts follow the
implemented SOPS/Age flows. Do not infer an external-secret-provider
integration from this repository alone.

### "openCenter only works with OpenStack"

**Reality:** OpenStack is the default and most mature provider, and the GA infrastructure surface also includes Magnum (managed OpenStack Kubernetes via cluster templates), VMware, Baremetal, and Kind. AWS-backed service integrations remain available where platform services use them, but AWS is not a GA infrastructure provider. See [CODEMAPS: Providers](../CODEMAPS/providers.md) for the exact capability matrix.

## Further Reading

* [Architecture (contributor map)](../architecture.md) - Terse, source-grounded package boundaries and runtime wiring
* [GitOps Workflow](gitops-workflow.md) - Repository structure and reconciliation
* [Security Model](security-model.md) - Security architecture and controls
* [Configuration Lifecycle](configuration-lifecycle.md) - Configuration management
* [Provider Comparison](provider-comparison.md) - Choosing infrastructure providers

---

For detailed code-level architecture maps, see the [CODEMAPS index](../CODEMAPS/INDEX.md).

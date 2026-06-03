---
id: architecture
title: "Architecture"
sidebar_label: Architecture
description: System design, core components, and architectural decisions behind openCenter.
doc_type: explanation
audience: "architects, developers"
tags: [architecture, design, components, patterns]
---
# Architecture

**Purpose:** For technical users, explains the system design and architectural decisions behind openCenter, covering components through design principles.

Understanding openCenter’s architecture helps you make informed decisions about deployment, customization, and troubleshooting. This explanation covers the key architectural patterns and design choices.

## System Overview

openCenter follows a **configuration-first, GitOps-native** architecture where a single YAML file drives the entire cluster lifecycle. The system transforms declarative configuration into production infrastructure through multiple layers of abstraction.

```
Configuration File (YAML)
    ↓
Validation Engine (Schema + Business Rules)
    ↓
Template Engine (Go templates + Sprig)
    ↓
GitOps Repository (Infrastructure + Applications)
    ↓
Provisioning Layer (Terraform/Kubespray)
    ↓
Production Cluster (Kubernetes + Services)
```

## Core Components

### Configuration Manager

**Purpose:** Load, validate, and manage cluster configurations.

**Design:** The configuration manager uses a **5-stage loading pipeline** (in `internal/config/v2/loader.go`):

1. **Parse YAML** -- Decode raw YAML into intermediate representation
2. **Normalize** -- Canonicalize provider names, resolve aliases
3. **Resolve References** -- Expand `${ref:path}`, `${env:VAR}`, `${file:path}` with dependency graph and cycle detection
4. **Apply Defaults** -- Hydrate empty fields from provider-region defaults registry
5. **Validate** -- Schema + business rules + provider + deployment + services checks

Configuration precedence (highest to lowest):

1. Command-line flags
2. Environment variables
3. Cluster config file
4. CLI settings file (`~/.config/opencenter/config.yaml`)
5. Built-in defaults

**Why this design:** The pipeline ensures every config is fully resolved and validated before use. Reference resolution with topological sort prevents circular dependencies. Hydration fills gaps without overwriting explicit values.

**Evidence:** `internal/config/v2/loader.go`, `internal/config/manager.go`, `internal/config/defaults.go`

### Validation Engine

**Purpose:** Ensure configuration correctness before deployment.

**Design:** Multi-layered validation with progressive checks:

1. **Schema Validation:** JSON schema compliance (structure, types, formats)
2. **Business Rules:** Cross-field dependencies (e.g., VRRP IP required when Octavia disabled)
3. **Provider Validation:** Provider-specific constraints (image IDs, flavors, networks)
4. **Connectivity Validation:** API reachability and credential verification (optional)

**Why this design:** Catch errors early (fail fast) with increasingly specific checks. Schema validation is fast and catches 80% of errors. Business rules catch logical inconsistencies. Provider validation catches deployment-time failures before provisioning.

**Trade-offs:** More validation means slower feedback, but prevents costly deployment failures. Connectivity validation is optional because it requires credentials and network access.

**Evidence:** `internal/config/v2/validator.go`, `internal/core/validation/`, `cmd/cluster_validate.go`

### Template Engine

**Purpose:** Generate infrastructure and application manifests from configuration.

**Design:** Embedded templates with Go’s `text/template` and Sprig functions:

* Templates embedded in binary (`//go:embed`)
* Configuration values injected via template variables
* Sprig functions for string manipulation, encoding, etc.
* No hardcoded values in templates

**Why this design:** Templates are version-controlled with the CLI, ensuring consistency. Embedding eliminates external dependencies. Sprig provides rich template functions without custom code.

**Trade-offs:** Templates are less flexible than code but more maintainable. Changes require CLI rebuild, but this ensures tested combinations.

**Evidence:** `internal/gitops/copy.go`, `internal/template/`, `.kiro/steering/product.md:35`

### GitOps Repository Generator

**Purpose:** Create complete GitOps repository structure.

**Design:** Standardized directory layout with Kustomize overlays:

```
<git_dir>/
├── applications/
│   └── overlays/<cluster>/
│       ├── flux-system/          # FluxCD bootstrap
│       ├── services/              # Platform services
│       └── managed-services/      # Customer applications
└── infrastructure/
    └── clusters/<cluster>/
        ├── main.tf                # Terraform/OpenTofu
        ├── inventory/             # Kubespray Ansible
        └── kubeconfig.yaml        # Cluster access
```

**Why this design:** Separation of infrastructure (Terraform) and applications (Kubernetes manifests) allows different teams to manage different layers. Kustomize overlays enable cluster-specific customization without duplicating base manifests.

**Trade-offs:** More directories and files, but clear separation of concerns. Overlay pattern requires understanding Kustomize, but provides powerful composition.

**Evidence:** `internal/gitops/`, `.kiro/steering/structure.md:118-128`, Ecosystem.md

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
* Dual-key rotation (add new key → re-encrypt → remove old key)
* Key expiration monitoring (Age 90 days, SSH 180 days)
* Git pre-commit hooks preventing plaintext secret commits
* HMAC-signed audit logging for tamper detection

**Why this design:** Secrets can be version-controlled safely. FluxCD handles decryption automatically. Age keys are simpler than GPG (no key servers). Dual-key rotation allows gradual re-encryption without downtime.

**Evidence:** `internal/sops/`, `internal/secrets/`, `internal/security/audit_logger.go`

### Dependency Injection Container

**Purpose:** Manage service dependencies and lifecycle.

**Design:** Two approaches coexist:

1. **`App` struct** (preferred) -- Explicit constructor chaining with typed fields. Built via `di.NewApp(baseDir)`.
2. **`DIContainer`** (legacy) -- Reflection-based resolution matching constructor parameter types to registered return types.

Key properties:

* Services registered as factory functions (provider pattern)
* Dependencies resolved by type matching
* Singletons initialized eagerly via `Initialize()`
* Circular dependencies detected via topological ordering
* Thread-safe after initialization (`sync.RWMutex`)
* Graceful shutdown calling `Shutdown()` on components

**Why this design:** Testability (mock dependencies), flexibility (swap implementations), and explicit dependencies (no global state). The typed `App` struct provides compile-time safety while the reflection container supports dynamic resolution.

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

For a complete architectural map of all packages, see [Codemaps Index](../index.md).

Key packages by responsibility:

| Layer | Packages |
| --- | --- |
| CLI | `cmd/` (Cobra commands), `internal/ui` (prompts), `internal/plugins` (external plugins) |
| Domain | `internal/cluster` (lifecycle), `internal/secrets` (secrets mgmt), `internal/operations` (drift, backup) |
| Config | `internal/config` (types, loader, builder, v2), `internal/config/services` (service registry) |
| GitOps | `internal/gitops` (pipeline, templates, rendering), `internal/template` (engine) |
| Infra | `internal/cloud` (providers), `internal/provision` (templates), `internal/tofu` (OpenTofu), `internal/ansible` (Kubespray) |
| Security | `internal/security` (audit, masking, sanitization), `internal/sops` (encryption) |
| Foundation | `internal/di` (DI container), `internal/core` (paths, validation), `internal/util` (shared), `internal/resilience` (locks, retry) |

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

**Evidence:** `.kiro/steering/product.md:30-35`

### GitOps Native

**Pattern:** Git as single source of truth, FluxCD reconciles desired state.

**Benefits:**

* Declarative (describe what, not how)
* Self-healing (FluxCD corrects drift)
* Auditable (all changes in Git)
* Secure (no direct cluster access needed)

**Constraints:**

* Git repository required
* FluxCD must be running
* Changes take time to reconcile (5-15 minutes)

**Evidence:** Ecosystem.md GitOps flow, `.kiro/steering/product.md:31`

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

**Evidence:** `internal/cloud/`, `internal/provision/`, `.kiro/steering/product.md:34`

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

* Changes require rebuild
* Binary size increases
* Cannot customize without forking

**Evidence:** `internal/gitops/embed.go`, `.kiro/steering/structure.md:95`

## Design Principles

### 1. Declarative Over Imperative

**Principle:** Describe desired state, not steps to achieve it.

**Example:** Configuration specifies "3 control plane nodes" not "create node 1, create node 2, create node 3."

**Rationale:** Declarative is idempotent (safe to re-apply), easier to understand (what not how), and enables automation (reconciliation loops).

**Evidence:** `.kiro/steering/product.md:30`

### 2. Fail Fast

**Principle:** Catch errors as early as possible.

**Example:** Schema validation before business rules before provider checks before deployment.

**Rationale:** Faster feedback loop, cheaper to fix (no infrastructure provisioned), clearer error messages (specific validation layer).

**Evidence:** `internal/config/v2/validator.go`, `internal/core/validation/`

### 3. Composition Over Inheritance

**Principle:** Build complex behavior from simple components.

**Example:** Kustomize overlays compose base + cluster-specific configuration rather than inheriting from base classes.

**Rationale:** More flexible (mix and match), easier to understand (explicit composition), avoids deep hierarchies.

**Evidence:** Ecosystem.md Kustomize overlay pattern, `.kiro/steering/product.md:34`

### 4. Explicit Dependencies

**Principle:** Dependencies injected, not instantiated internally.

**Example:** Validation engine receives validators as parameters, not creating them internally.

**Rationale:** Testability (mock dependencies), flexibility (swap implementations), clarity (dependencies visible in signatures).

**Evidence:** `internal/di/`

### 5. Security First

**Principle:** Secure by default, no plaintext secrets.

**Example:** SOPS encryption required for secrets, no option to disable.

**Rationale:** Prevent accidental exposure, enforce best practices, compliance requirements.

**Evidence:** `internal/sops/`, `internal/secrets/`

## Data Flow

### Initialization Flow

```
User: opencenter cluster init my-cluster
    ↓
CLI: Load defaults from internal/config/defaults.go
    ↓
CLI: Apply CLI defaults from ~/.config/opencenter/config.yaml
    ↓
CLI: Generate configuration file
    ↓
CLI: Write to ~/.config/opencenter/clusters/<org>/.my-cluster-config.yaml
    ↓
User: Configuration ready for editing
```

### Validation Flow

```
User: opencenter cluster validate
    ↓
Validation Engine: Load configuration
    ↓
Schema Validator: Check JSON schema compliance
    ↓
Business Rules Validator: Check cross-field dependencies
    ↓
Provider Validator: Check provider-specific constraints
    ↓
Connectivity Validator: Check API reachability (optional)
    ↓
CLI: Report validation results
```

### Setup Flow

```
User: opencenter cluster generate
    ↓
Template Engine: Load embedded templates
    ↓
Template Engine: Inject configuration values
    ↓
Template Engine: Render to GitOps repository
    ↓
SOPS Manager: Encrypt secrets
    ↓
Git: Initialize repository
    ↓
CLI: Repository ready for commit
```

### Bootstrap Flow

```
User: opencenter cluster deploy
    ↓
Terraform: Provision infrastructure (VMs, networks, storage)
    ↓
Kubespray: Deploy Kubernetes (control plane, workers, CNI)
    ↓
FluxCD: Bootstrap GitOps (install controllers, create sources)
    ↓
FluxCD: Reconcile services (deploy platform services)
    ↓
CLI: Cluster ready
```

## Scalability Considerations

### Configuration Size

**Current:** Single YAML file (typically 500-2000 lines)

**Limits:** No hard limit, but large files are harder to manage

**Mitigation:** Use CLI defaults for common values, reference external files for large data (SSH keys, certificates)

### Cluster Count

**Current:** Organization-based directory structure supports unlimited clusters

**Limits:** Filesystem limits (millions of files)

**Mitigation:** Archive old clusters, use separate organizations for different teams

### Service Count

**Current:** 20+ services enabled by default

**Limits:** Kubernetes resource limits (pods, services, etc.)

**Mitigation:** Disable unnecessary services, use larger nodes, scale horizontally

## Extension Points

### Custom Providers

Add new infrastructure providers by implementing provider interface:

1. Create provider adapter in `internal/cloud/<provider>/`
2. Implement provisioning logic in `internal/provision/<provider>/`
3. Add provider-specific validation
4. Update schema with provider configuration

**Evidence:** `internal/cloud/`, `internal/provision/`

### Custom Services

Add new platform services by:

1. Create service configuration in `internal/config/services/<service>.go`
2. Add service to defaults in `internal/config/defaults.go`
3. Create service manifests in openCenter-gitops-base
4. Update documentation

**Evidence:** `internal/config/services/`,

### Custom Validators

Add new validation rules by:

1. Implement validator interface in `internal/core/validation/validators/`
2. Register validator with validation engine
3. Add tests for validator

**Evidence:** `internal/core/validation/`,

### Plugins

Extend CLI with external plugins:

1. Create executable named `opencenter-<plugin>`
2. Place in PATH
3. CLI discovers and loads automatically

**Evidence:** `internal/plugins/`, `cmd/plugins.go`

## Common Misconceptions

### "openCenter is just a wrapper around Terraform"

**Reality:** openCenter orchestrates multiple tools (Terraform, Kubespray, FluxCD) and provides validation, secrets management, and GitOps scaffolding. Terraform is one component.

### "Configuration changes require cluster rebuild"

**Reality:** Most configuration changes can be applied by updating the configuration file and running `opencenter cluster generate`. Only provider changes (OpenStack → VMware) require rebuild.

### "GitOps means no manual changes"

**Reality:** GitOps means Git is the source of truth, but manual changes are possible (and sometimes necessary) for debugging. They’ll be reverted on next reconciliation unless committed to Git.

### "All secrets must be in configuration file"

**Reality:** Secrets can be in configuration file (encrypted with SOPS) or external secret providers (planned feature). Configuration file is convenient but not required.

### "openCenter only works with OpenStack"

**Reality:** OpenStack is the default and most mature provider, and the GA infrastructure surface also includes VMware, Baremetal, and Kind. AWS-backed service integrations remain available where platform services use them, but AWS is not a GA infrastructure provider.

## Further Reading

* [GitOps Workflow](gitops-workflow.md) - Repository structure and reconciliation
* [Security Model](security-model.md) - Security architecture and controls
* [Configuration Lifecycle](configuration-lifecycle.md) - Configuration management
* [Provider Comparison](provider-comparison.md) - Choosing infrastructure providers

---

---

**Last Updated:** 2026-05-11

For detailed code-level architecture maps, see [Codemaps](../index.md).
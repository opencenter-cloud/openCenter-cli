# System Architecture


## Table of Contents

- [Overview](#overview)
- [The Core Concept](#the-core-concept)
- [Why a Single YAML File?](#why-a-single-yaml-file)
- [Architecture Layers](#architecture-layers)
- [Data Flow: From YAML to Infrastructure](#data-flow-from-yaml-to-infrastructure)
- [Design Rationale: Key Decisions](#design-rationale-key-decisions)
- [Trade-offs and Limitations](#trade-offs-and-limitations)
- [Component Interactions](#component-interactions)
- [Related Concepts](#related-concepts)
- [Conclusion](#conclusion)
**doc_type**: explanation

## Overview

openCenter transforms a single declarative YAML configuration into a production-ready GitOps repository. This document explains the architectural decisions, design patterns, and trade-offs that shape how openCenter works.

## The Core Concept

At its heart, openCenter solves a fundamental problem: **how do you make Kubernetes cluster deployment both simple and production-ready?** The answer lies in a carefully orchestrated transformation pipeline that takes human-friendly configuration and generates battle-tested infrastructure code.

```
User YAML → Validation → Template Rendering → GitOps Repository → Infrastructure
```

This isn't just a code generator—it's an opinionated framework that embeds years of operational knowledge into templates, validation rules, and organizational patterns.

## Why a Single YAML File?

### The Decision

openCenter uses a single configuration file as the source of truth for an entire cluster. This might seem limiting compared to systems that allow multiple configuration files or dynamic composition, but it's a deliberate choice.

### The Rationale

**Cognitive Load**: When troubleshooting a production issue at 3 AM, you want one place to look. A single file means no hunting through directories, no wondering which file takes precedence, no merge conflicts between team members editing different files.

**Auditability**: Git history on a single file tells the complete story of a cluster's evolution. Every change—from initial creation to production scaling—is visible in one file's commit log.

**Validation Simplicity**: A single file can be validated atomically. There's no partial validity, no "this file is correct but conflicts with that file." The configuration is either valid or it isn't.

### The Trade-off

You lose some flexibility. You can't easily compose configurations from multiple sources or share common settings across clusters through file inclusion. But you gain predictability and operational simplicity—a trade-off openCenter makes deliberately in favor of production reliability.

## Architecture Layers

openCenter is structured in distinct layers, each with clear responsibilities and boundaries.

### Layer 1: CLI Interface (cmd/)

```
┌─────────────────────────────────────────┐
│         Cobra Command Layer             │
│  ┌─────────┐  ┌──────────┐  ┌────────┐ │
│  │ cluster │  │  config  │  │  sops  │ │
│  └─────────┘  └──────────┘  └────────┘ │
│       ↓             ↓            ↓      │
│  ┌─────────────────────────────────┐   │
│  │   Dependency Injection (DI)     │   │
│  │   Container Management          │   │
│  └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

**Purpose**: Translate user intent into system operations.

**Design Decision**: We use Cobra for command structure because it provides excellent help generation, flag parsing, and subcommand organization. But more importantly, we inject dependencies through a DI container rather than using global state.

**Why DI?**: Global configuration managers create hidden dependencies and make testing difficult. By injecting dependencies, each command explicitly declares what it needs, making the system more testable and the data flow more transparent.

### Layer 2: Configuration System (internal/config/)

```
┌──────────────────────────────────────────────────┐
│           Configuration Pipeline                  │
│                                                   │
│  Load → Schema Validation → Business Rules →     │
│  Provider Validation → Semantic Checks            │
│                                                   │
│  ┌──────────────┐  ┌─────────────┐              │
│  │ ConfigManager│  │  Validator  │              │
│  │  - Loading   │  │  - Pipeline │              │
│  │  - Defaults  │  │  - Rules    │              │
│  │  - Migration │  │  - Repair   │              │
│  └──────────────┘  └─────────────┘              │
│                                                   │
│  ┌──────────────────────────────────────┐       │
│  │      PathResolver                     │       │
│  │  Organization-based directory layout  │       │
│  │  ~/.config/openCenter/clusters/       │       │
│  │    └── <org>/                         │       │
│  │        ├── infrastructure/             │       │
│  │        ├── applications/               │       │
│  │        └── secrets/                    │       │
│  └──────────────────────────────────────┘       │
└──────────────────────────────────────────────────┘
```

**Purpose**: Ensure configuration correctness before any infrastructure changes.

**The Validation Pipeline**: Validation happens in stages, each catching different classes of errors:

1. **Schema Validation**: Is the YAML structurally correct? Are required fields present?
2. **Business Rules**: Does the configuration make logical sense? (e.g., worker count > 0)
3. **Provider Validation**: Are provider-specific requirements met? (e.g., OpenStack auth URL)
4. **Semantic Checks**: Do the pieces fit together? (e.g., network plugin compatibility)

**Why Staged Validation?**: Early stages catch simple errors quickly. Later stages perform expensive checks (like API calls) only after basic correctness is established. This provides fast feedback for common mistakes while still catching complex issues.

**Organization Structure**: The PathResolver implements an organization-based directory layout that supports multi-tenancy. Each organization gets its own namespace, preventing cluster name collisions and enabling team isolation.

**Why Organizations?**: In enterprise environments, different teams need isolated cluster namespaces. The organization structure provides this isolation while maintaining a consistent layout pattern.

### Layer 3: GitOps Generator (internal/gitops/)

```
┌────────────────────────────────────────────────┐
│          Template Rendering Engine              │
│                                                 │
│  ┌──────────────┐         ┌─────────────────┐ │
│  │  Embedded    │         │  Go Templates   │ │
│  │  Templates   │    →    │  + Sprig        │ │
│  │  (go:embed)  │         │  Functions      │ │
│  └──────────────┘         └─────────────────┘ │
│                                    ↓            │
│                           ┌─────────────────┐  │
│                           │  Atomic Writer  │  │
│                           │  (Workspace)    │  │
│                           └─────────────────┘  │
│                                    ↓            │
│  ┌──────────────────────────────────────────┐ │
│  │         GitOps Repository                 │ │
│  │  ├── applications/                        │ │
│  │  │   └── overlays/<cluster>/              │ │
│  │  ├── infrastructure/                      │ │
│  │  │   └── clusters/<cluster>/              │ │
│  │  └── README.md                            │ │
│  └──────────────────────────────────────────┘ │
└────────────────────────────────────────────────┘
```

**Purpose**: Transform validated configuration into deployable infrastructure code.

**Embedded Templates**: All templates are embedded in the binary using `//go:embed`. This means:
- No external dependencies at runtime
- Version-locked templates (template version matches CLI version)
- Simplified distribution (single binary)

**Why Embed?**: External templates create version skew problems. If templates are separate files, users might mix CLI version 1.0 with templates from version 1.1, causing subtle bugs. Embedding ensures consistency.

**Atomic Operations**: The GitOpsWorkspace and AtomicWriter ensure that file operations either complete fully or fail cleanly. No partial writes, no corrupted repositories.

**Why Atomic?**: GitOps repositories are the source of truth for production infrastructure. A partial write could deploy broken configuration. Atomic operations prevent this entire class of errors.

### Layer 4: Provider Adapters (internal/cloud/, internal/talos/, internal/ansible/)

```
┌─────────────────────────────────────────────┐
│         Provider Abstraction Layer          │
│                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐ │
│  │OpenStack │  │   AWS    │  │  VMware  │ │
│  │          │  │          │  │          │ │
│  │Preflight │  │Preflight │  │Preflight │ │
│  │Validation│  │Validation│  │Validation│ │
│  └──────────┘  └──────────┘  └──────────┘ │
│                                             │
│  ┌──────────────────────────────────────┐  │
│  │    Provisioning Engines              │  │
│  │  ┌──────────┐  ┌─────────────────┐  │  │
│  │  │OpenTofu/ │  │  Ansible/       │  │  │
│  │  │Terraform │  │  Kubespray      │  │  │
│  │  └──────────┘  └─────────────────┘  │  │
│  │  ┌──────────┐                       │  │
│  │  │  Pulumi  │  (Talos)              │  │
│  │  └──────────┘                       │  │
│  └──────────────────────────────────────┘  │
└─────────────────────────────────────────────┘
```

**Purpose**: Isolate provider-specific logic from core system.

**Adapter Pattern**: Each provider implements a common interface for preflight checks and validation. The core system doesn't know about OpenStack APIs or AWS SDKs—it just calls the adapter interface.

**Why Adapters?**: Cloud providers change their APIs, add new features, and deprecate old ones. By isolating provider logic in adapters, we can update OpenStack support without touching AWS code, or add a new provider without modifying the core.

**Multiple Provisioning Engines**: Different providers work best with different tools:
- OpenStack → OpenTofu (declarative infrastructure)
- Talos → Pulumi (programmatic control)
- Kubernetes → Ansible/Kubespray (configuration management)

**Why Multiple Engines?**: Each tool excels in its domain. OpenTofu is perfect for cloud resources, Pulumi provides type-safe infrastructure code, Ansible handles complex configuration. Using the right tool for each job produces better results than forcing everything through one tool.

### Layer 5: Secrets Management (internal/sops/)

```
┌──────────────────────────────────────────────┐
│         Secrets Management Layer             │
│                                              │
│  ┌────────────────────────────────────────┐ │
│  │         SOPS Integration               │ │
│  │  ┌──────────┐      ┌───────────────┐  │ │
│  │  │   Age    │  →   │  Encrypted    │  │ │
│  │  │   Keys   │      │  YAML Files   │  │ │
│  │  └──────────┘      └───────────────┘  │ │
│  │                                        │ │
│  │  ┌──────────────────────────────────┐ │ │
│  │  │  Key Management                  │ │ │
│  │  │  - Generation                    │ │ │
│  │  │  - Storage (age/keys/)           │ │ │
│  │  │  - Rotation                      │ │ │
│  │  └──────────────────────────────────┘ │ │
│  └────────────────────────────────────────┘ │
│                                              │
│  ┌────────────────────────────────────────┐ │
│  │  Validation & Enforcement              │ │
│  │  - No plaintext secrets in Git         │ │
│  │  - Production key validation           │ │
│  │  - Encryption verification             │ │
│  └────────────────────────────────────────┘ │
└──────────────────────────────────────────────┘
```

**Purpose**: Ensure secrets never exist in plaintext in Git repositories.

**SOPS + Age**: We use SOPS (Secrets OPerationS) with Age encryption because:
- Age is simple and secure (modern cryptography, minimal attack surface)
- SOPS integrates with Git workflows (encrypt on commit, decrypt on checkout)
- YAML structure is preserved (you can see what's encrypted, just not the values)

**Why Not Vault/External Secrets?**: Those are runtime secret stores. SOPS solves a different problem: how to store secrets in Git safely. You can use both—SOPS for GitOps repository secrets, Vault for application runtime secrets.

**Key Management**: Keys are stored in the organization's secrets directory, separate from the GitOps repository. This separation means:
- GitOps repo can be public (encrypted secrets are safe)
- Key rotation doesn't require repository changes
- Different teams can have different keys

### Layer 6: Template Engine (internal/template/)

```
┌──────────────────────────────────────────────┐
│         Template Processing Engine           │
│                                              │
│  ┌────────────────────────────────────────┐ │
│  │  Go text/template + Sprig Functions    │ │
│  │                                        │ │
│  │  ┌──────────────┐  ┌───────────────┐ │ │
│  │  │   Caching    │  │  Validation   │ │ │
│  │  │   (parsed    │  │  (syntax      │ │ │
│  │  │   templates) │  │   checking)   │ │ │
│  │  └──────────────┘  └───────────────┘ │ │
│  └────────────────────────────────────────┘ │
│                                              │
│  ┌────────────────────────────────────────┐ │
│  │  Sandbox Mode (Optional)               │ │
│  │  - Disabled dangerous functions        │ │
│  │  - Timeout enforcement                 │ │
│  │  - Safe function whitelist             │ │
│  └────────────────────────────────────────┘ │
│                                              │
│  ┌────────────────────────────────────────┐ │
│  │  Error Reporting                       │ │
│  │  - Line numbers                        │ │
│  │  - Context extraction                  │ │
│  │  - Structured errors                   │ │
│  └────────────────────────────────────────┘ │
└──────────────────────────────────────────────┘
```

**Purpose**: Provide safe, powerful template rendering with excellent error messages.

**Go Templates + Sprig**: Go's `text/template` provides the foundation, Sprig adds 100+ utility functions (string manipulation, date formatting, crypto, etc.). This combination handles complex transformations without custom code.

**Why Go Templates?**: They're:
- Type-safe (compile-time checking)
- Sandboxable (can disable dangerous operations)
- Fast (compiled, not interpreted)
- Familiar (same syntax as Helm, Kubernetes, etc.)

**Caching**: Parsed templates are cached to avoid re-parsing on every render. For a cluster with 50+ template files, this provides a 10x speedup.

**Sandbox Mode**: When enabled, dangerous functions (env, exec, readFile) are disabled. This allows rendering untrusted templates safely—important for future features like user-provided templates.

**Error Reporting**: Template errors include:
- Exact line and column numbers
- Context (lines around the error)
- Suggestions for common mistakes

**Why Detailed Errors?**: Template syntax errors are frustrating. "Error at line 42" is useless without context. Showing the surrounding lines and highlighting the problem makes debugging 10x faster.

## Data Flow: From YAML to Infrastructure

Let's trace a cluster creation through the system:

### Step 1: User Creates Configuration

```bash
openCenter cluster init my-cluster
```

The CLI generates a default configuration with sensible values. The user edits this YAML file to specify their requirements.

### Step 2: Validation Pipeline

```bash
openCenter cluster validate my-cluster
```

The configuration flows through the validation pipeline:

1. **Load**: Read YAML, expand environment variables, merge with defaults
2. **Schema**: Validate against JSON schema (structure, types, required fields)
3. **Business Rules**: Check logical constraints (counts > 0, valid versions, etc.)
4. **Provider**: Call provider adapter for cloud-specific validation
5. **Semantic**: Verify cross-field dependencies (network plugin compatibility, etc.)

If validation fails, the user gets structured errors with suggestions. If it passes, we know the configuration is deployable.

### Step 3: GitOps Repository Generation

```bash
openCenter cluster setup my-cluster
```

The GitOps generator:

1. **Create Workspace**: Initialize atomic workspace for safe file operations
2. **Render Base**: Copy and render base GitOps structure (README, Makefile, etc.)
3. **Render Applications**: Generate Flux manifests for enabled services
4. **Render Infrastructure**: Generate OpenTofu/Terraform for cloud resources
5. **Encrypt Secrets**: Use SOPS to encrypt sensitive files
6. **Commit**: Atomic commit of all changes

The result is a complete GitOps repository ready for `git push`.

### Step 4: Infrastructure Provisioning

```bash
openCenter cluster bootstrap my-cluster
```

The bootstrap process:

1. **Preflight Checks**: Verify cloud credentials, quotas, network connectivity
2. **Provision Infrastructure**: Run OpenTofu to create VMs, networks, load balancers
3. **Configure Nodes**: Run Ansible/Kubespray to install Kubernetes
4. **Deploy GitOps**: Install Flux and point it at the repository
5. **Verify**: Wait for cluster to reach ready state

Each step is idempotent—you can re-run bootstrap safely if it fails partway through.

## Design Rationale: Key Decisions

### Why Embedded Templates?

**Decision**: Templates are embedded in the binary using `//go:embed`.

**Alternatives Considered**:
- External template files (user can customize)
- Template repository (fetch from GitHub)
- Plugin system (load templates dynamically)

**Why Embedded**:
- **Version Consistency**: Template version always matches CLI version
- **Offline Operation**: No network required after installation
- **Simplicity**: Single binary, no installation steps
- **Security**: No template injection attacks

**Trade-off**: Users can't easily customize templates. But customization is available through:
- Configuration overrides (most common needs)
- Kustomize overlays (for advanced customization)
- Forking (for complete control)

### Why Organization-Based Structure?

**Decision**: Clusters are organized by organization in the filesystem.

```
~/.config/openCenter/clusters/
├── acme-corp/
│   ├── prod-cluster/
│   ├── staging-cluster/
│   └── secrets/
└── widgets-inc/
    ├── cluster-1/
    └── secrets/
```

**Alternatives Considered**:
- Flat structure (all clusters in one directory)
- Project-based (group by project, not organization)
- Database (store metadata in SQLite)

**Why Organizations**:
- **Multi-Tenancy**: Different teams don't collide on cluster names
- **Access Control**: Filesystem permissions provide natural isolation
- **Scalability**: Hundreds of clusters don't create a flat directory nightmare
- **Migration Path**: Legacy flat structure is still supported

**Trade-off**: More complex directory structure. But the PathResolver abstracts this complexity—commands work the same regardless of structure.

### Why Single Validation Pipeline?

**Decision**: All validation goes through one pipeline with multiple stages.

**Alternatives Considered**:
- Validation scattered across commands
- Separate validators for each concern
- Lazy validation (validate only when needed)

**Why Pipeline**:
- **Consistency**: Same validation rules everywhere
- **Ordering**: Early stages catch simple errors before expensive checks
- **Composability**: Easy to add new validation stages
- **Testing**: Single pipeline is easier to test comprehensively

**Trade-off**: All validation code must fit the pipeline model. But the staged approach is flexible enough for all current needs.

### Why Atomic File Operations?

**Decision**: GitOps repository changes use atomic operations (all-or-nothing).

**Alternatives Considered**:
- Direct file writes (simpler code)
- Transactional filesystem (requires special FS)
- Backup-and-restore (manual rollback)

**Why Atomic**:
- **Safety**: No partial writes that corrupt the repository
- **Rollback**: Failed operations leave no artifacts
- **Concurrency**: Multiple operations don't interfere
- **Reliability**: Production systems need predictable behavior

**Trade-off**: More complex code (workspace management, temp files). But the reliability gain is worth it—GitOps repositories are too important to risk corruption.

## Trade-offs and Limitations

### Flexibility vs. Simplicity

**Trade-off**: openCenter is opinionated. It makes decisions for you (directory structure, validation rules, template organization).

**Why**: Flexibility creates complexity. Every decision point is a place for errors. By making good default decisions, we reduce cognitive load and operational risk.

**Limitation**: If you need something very different from openCenter's opinions, you might need to fork or use a different tool.

### Validation Strictness vs. Ease of Use

**Trade-off**: openCenter validates aggressively. Some configurations that "might work" are rejected.

**Why**: Production systems need reliability. It's better to reject a questionable configuration than to deploy something that fails at 2 AM.

**Limitation**: You might need to adjust your configuration to pass validation, even if you think it would work.

### Template Embedding vs. Customization

**Trade-off**: Embedded templates can't be easily customized without forking.

**Why**: Version consistency and security are more important than easy customization for most users.

**Limitation**: Advanced customization requires Kustomize overlays or forking. But this is intentional—most users shouldn't need to customize templates.

## Component Interactions

Here's how the major components work together:

```
┌─────────────────────────────────────────────────────────────┐
│                         User                                 │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ↓
┌─────────────────────────────────────────────────────────────┐
│                    CLI Commands (Cobra)                      │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │  init    │  │ validate │  │  setup   │  │bootstrap │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ↓
┌─────────────────────────────────────────────────────────────┐
│              Dependency Injection Container                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │ConfigManager │  │SOPSManager   │  │TemplateEngine│     │
│  └──────────────┘  └──────────────┘  └──────────────┘     │
└────────────────────────┬────────────────────────────────────┘
                         │
         ┌───────────────┼───────────────┐
         ↓               ↓               ↓
┌────────────────┐ ┌────────────┐ ┌────────────────┐
│ Configuration  │ │  GitOps    │ │   Secrets      │
│   Pipeline     │ │ Generator  │ │  Management    │
│                │ │            │ │                │
│ • Load         │ │ • Render   │ │ • Encryption   │
│ • Validate     │ │ • Atomic   │ │ • Key Mgmt     │
│ • Migrate      │ │ • Embed    │ │ • Validation   │
└────────┬───────┘ └─────┬──────┘ └────────┬───────┘
         │               │                  │
         └───────────────┼──────────────────┘
                         ↓
         ┌───────────────────────────────┐
         │    Provider Adapters          │
         │  ┌──────────┐  ┌──────────┐  │
         │  │OpenStack │  │   AWS    │  │
         │  └──────────┘  └──────────┘  │
         └───────────────┬───────────────┘
                         ↓
         ┌───────────────────────────────┐
         │   Provisioning Engines        │
         │  ┌──────────┐  ┌──────────┐  │
         │  │OpenTofu  │  │ Ansible  │  │
         │  └──────────┘  └──────────┘  │
         └───────────────┬───────────────┘
                         ↓
         ┌───────────────────────────────┐
         │    Cloud Infrastructure       │
         │  (VMs, Networks, Storage)     │
         └───────────────────────────────┘
```

**Key Interactions**:

1. **CLI → DI Container**: Commands request dependencies, never access globals
2. **ConfigManager → Validator**: Configuration flows through validation pipeline
3. **GitOps Generator → Template Engine**: Renders templates with validated config
4. **SOPS Manager → Key Manager**: Manages encryption keys separately from secrets
5. **Provider Adapters → Cloud APIs**: Isolated provider-specific logic
6. **All Components → Error System**: Structured errors with context and suggestions

## Related Concepts

- **[GitOps Workflow](../operations/gitops-workflow.md)**: How the generated repository drives deployment
- **[Security Model](../operations/security-model.md)**: How secrets and access control work
- **[Configuration System](../reference/configuration.md)**: Detailed configuration reference
- **[Validation Pipeline](../reference/validation.md)**: How validation stages work
- **[Provider Architecture](../providers/README.md)**: Provider-specific details

## Conclusion

openCenter's architecture reflects a core philosophy: **production reliability through opinionated simplicity**. Every design decision—from embedded templates to atomic operations to staged validation—prioritizes operational safety over flexibility.

This isn't the right architecture for every use case. If you need maximum flexibility or want to support every possible configuration, you'll find openCenter constraining. But if you want a tool that makes good decisions for you and prevents entire classes of operational errors, this architecture delivers.

The key insight is that **constraints enable reliability**. By limiting what you can do, we ensure that what you do works. And in production systems, that's what matters most.

# Design Decisions and Rationale


## Table of Contents

- [Who this is for](#who-this-is-for)
- [Configuration as a single YAML file](#configuration-as-a-single-yaml-file)
- [Embedded templates in the binary](#embedded-templates-in-the-binary)
- [Organization-based directory structure](#organization-based-directory-structure)
- [Staged validation pipeline](#staged-validation-pipeline)
- [SOPS with Age encryption for secrets](#sops-with-age-encryption-for-secrets)
- [Dependency injection container](#dependency-injection-container)
- [Mise instead of Make](#mise-instead-of-make)
- [Atomic file operations for GitOps generation](#atomic-file-operations-for-gitops-generation)
- [Template sandboxing](#template-sandboxing)
- [Why these decisions matter](#why-these-decisions-matter)
- [Evolution and future decisions](#evolution-and-future-decisions)
- [See also](#see-also)
**doc_type**: explanation

## Who this is for

Developers and architects who want to understand why openCenter is built the way it is. This document explains the reasoning behind major design decisions, the alternatives considered, and the trade-offs made.

## Configuration as a single YAML file

### The decision

openCenter uses a single YAML file as the source of truth for an entire cluster configuration.

### Why we chose this

**Cognitive simplicity**: When troubleshooting at 3 AM, you want one place to look. A single file means no hunting through directories, no wondering which file takes precedence, no merge conflicts between team members editing different files.

**Atomic validation**: A single file can be validated atomically. There's no partial validity, no "this file is correct but conflicts with that file." The configuration is either valid or it isn't.

**Clear audit trail**: Git history on a single file tells the complete story of a cluster's evolution. Every change—from initial creation to production scaling—is visible in one file's commit log.

**Operational reliability**: In production, simplicity reduces risk. A single file is easier to backup, easier to restore, easier to version, and easier to understand.

### Alternatives considered

**Multiple configuration files**: Split configuration by concern (networking.yaml, storage.yaml, services.yaml). This provides better organization for large configurations.

**Why we rejected it**: The organizational benefit doesn't outweigh the operational complexity. Which file do you check first when something breaks? How do you ensure consistency across files? How do you handle cross-file dependencies?

**Configuration composition**: Allow importing and merging multiple files. This enables sharing common settings across clusters.

**Why we rejected it**: Composition creates implicit dependencies. Understanding the final configuration requires tracing through multiple files. Debugging becomes harder because the effective configuration isn't visible in one place.

**Database storage**: Store configuration in a database with a schema and query interface.

**Why we rejected it**: Databases add operational complexity (backup, migration, access control). Git provides versioning, branching, and collaboration for free. YAML is human-readable and editable with any text editor.

### Trade-offs accepted

**Limited reusability**: You can't easily share configuration snippets across clusters. Each cluster's configuration is self-contained.

**Mitigation**: Use templates or scripts to generate configurations. The `cluster init` command provides sensible defaults that can be customized.

**Large files**: A fully-configured cluster can result in a 500+ line YAML file.

**Mitigation**: YAML's structure keeps related settings together. Good editor support (folding, search) makes navigation manageable. The alternative—multiple files—would have the same total lines, just spread across files.

## Embedded templates in the binary

### The decision

All GitOps templates are embedded in the openCenter binary using Go's `//go:embed` directive.

### Why we chose this

**Version consistency**: Template version always matches CLI version. You can't accidentally use templates from version 1.1 with CLI version 1.0, which would cause subtle bugs.

**Offline operation**: No network required after installation. No dependency on GitHub being available. No risk of template repository being deleted or modified.

**Single binary distribution**: One file to download, one file to install. No separate template installation step. No template path configuration.

**Security**: No template injection attacks. No risk of malicious templates being fetched from a compromised repository. Templates are verified at build time, not runtime.

### Alternatives considered

**External template files**: Ship templates as separate files that users can customize.

**Why we rejected it**: Template customization is rarely needed—most users want standard deployments. For those who need customization, Kustomize overlays provide a better mechanism. External files create version skew problems and complicate distribution.

**Template repository**: Fetch templates from GitHub at runtime.

**Why we rejected it**: Network dependency makes the tool less reliable. Version management becomes complex (which template version for which CLI version?). Offline operation is impossible.

**Plugin system for templates**: Allow users to provide custom templates via plugins.

**Why we rejected it**: This is planned for the future, but as an addition, not a replacement. Embedded templates provide the default experience; plugins provide customization for advanced users.

### Trade-offs accepted

**Customization requires recompilation**: To change templates, you must fork openCenter and rebuild. This is a high barrier for customization.

**Mitigation**: Most customization needs are met by configuration options. For advanced needs, Kustomize overlays allow modifying generated manifests without changing templates. For complete control, forking is acceptable—it's a one-time cost for organizations with unique requirements.

**Binary size**: Embedding templates increases binary size by ~2MB.

**Mitigation**: 2MB is negligible on modern systems. The benefits (reliability, simplicity) far outweigh the cost.

## Organization-based directory structure

### The decision

Clusters are organized by organization in the filesystem:

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

### Why we chose this

**Multi-tenancy**: Different teams can manage clusters independently without name collisions. "prod" in acme-corp is separate from "prod" in widgets-inc.

**Access control**: Filesystem permissions provide natural isolation. You can grant a team access to their organization directory without exposing other organizations.

**Scalability**: Hundreds of clusters don't create a flat directory nightmare. Organization-level grouping keeps directories manageable.

**Shared resources**: Organizations can share GitOps repositories, secrets infrastructure, and operational tooling while keeping clusters isolated.

### Alternatives considered

**Flat structure**: All clusters in one directory (`~/.config/openCenter/clusters/prod`, `~/.config/openCenter/clusters/staging`).

**Why we rejected it**: Doesn't scale beyond a single team. Name collisions are inevitable. No natural isolation boundary.

**Project-based structure**: Group by project rather than organization.

**Why we rejected it**: Projects are more fluid than organizations. A cluster might belong to multiple projects. Organizations provide a stable, long-term grouping.

**Database with metadata**: Store cluster metadata in a database, files on disk.

**Why we rejected it**: Adds complexity without clear benefit. Filesystem already provides organization, search, and access control.

### Trade-offs accepted

**More complex paths**: Paths are longer and require organization specification. Commands need to parse "org/cluster" identifiers.

**Mitigation**: The PathResolver abstracts path complexity. Commands work the same regardless of structure. Legacy flat structure is still supported for backward compatibility.

**Migration complexity**: Existing users must migrate from flat to organization-based structure.

**Mitigation**: The default organization is "opencenter", so single-team users can continue without changes. Migration is optional and can be done incrementally.

## Staged validation pipeline

### The decision

Configuration validation happens in multiple stages, each checking different aspects:

1. Schema validation (structure, types)
2. Business rule validation (logical constraints)
3. Provider-specific validation (cloud requirements)
4. Connectivity validation (API access, quotas)

### Why we chose this

**Fast feedback**: Early stages catch simple errors quickly (milliseconds). Later stages perform expensive checks (API calls) only after basic correctness is established.

**Clear error messages**: Each stage can provide targeted error messages. Schema errors point to YAML structure. Business rule errors explain logical problems. Provider errors include cloud-specific context.

**Composability**: New validation stages can be added without modifying existing stages. Provider-specific validation is isolated from core validation.

**Testability**: Each stage can be tested independently. Mock providers can test validation logic without real cloud APIs.

### Alternatives considered

**Single validation pass**: Perform all validation in one function.

**Why we rejected it**: Mixing concerns makes code hard to maintain. Schema validation logic intertwined with business rules intertwined with provider checks creates a tangled mess.

**Lazy validation**: Validate only when needed (e.g., validate networking only if deploying).

**Why we rejected it**: Partial validation creates confusion. Is the configuration valid or not? Lazy validation means errors appear late in the process, after users have invested time.

**Validation on save**: Validate automatically when configuration is saved.

**Why we rejected it**: Validation can be slow (API calls). Automatic validation would block saves. Explicit validation (`cluster validate`) gives users control over when to pay the validation cost.

### Trade-offs accepted

**Validation can be slow**: Connectivity validation makes API calls, which can take seconds.

**Mitigation**: Connectivity validation is optional and can be skipped with `--skip-connectivity`. Fast validation stages run first, providing quick feedback for common errors.

**Validation doesn't guarantee success**: A configuration can pass validation but still fail during deployment (e.g., cloud quota exhausted between validation and deployment).

**Mitigation**: Validation catches most errors. For runtime errors, the bootstrap process is idempotent—you can re-run it after fixing the issue.

## SOPS with Age encryption for secrets

### The decision

Use SOPS (Secrets OPerationS) with Age encryption for managing secrets in GitOps repositories.

### Why we chose this

**Git-friendly**: SOPS encrypts values but preserves YAML structure. You can see what's encrypted (keys are visible) without seeing the values. This makes code review and diff viewing practical.

**Simple key management**: Age uses simple key files, not complex PKI infrastructure. Generate a key pair, store the private key securely, commit the public key to the repository.

**No external dependencies**: SOPS and Age are standalone tools. No secret server to run, no API to maintain, no network dependency at runtime.

**Industry standard**: SOPS is widely used in the Kubernetes ecosystem. Many tools integrate with it (Flux, ArgoCD, Helm Secrets).

### Alternatives considered

**Sealed Secrets**: Kubernetes-native secret encryption using a controller.

**Why we rejected it**: Requires a running cluster to decrypt secrets. Doesn't work for bootstrapping (you need secrets to create the cluster). Ties secret management to cluster lifecycle.

**Vault**: HashiCorp Vault for secret storage and management.

**Why we rejected it**: Vault is a runtime secret store, not a GitOps secret solution. You can use both—SOPS for GitOps repository secrets, Vault for application runtime secrets. But Vault alone doesn't solve the "secrets in Git" problem.

**Git-crypt**: Transparent encryption for Git repositories.

**Why we rejected it**: Encrypts entire files, not individual values. You can't review encrypted files in pull requests. No support for multiple keys (everyone needs the same key).

**External Secrets Operator**: Sync secrets from external stores into Kubernetes.

**Why we rejected it**: Requires an external secret store (Vault, AWS Secrets Manager). Adds operational complexity. Doesn't solve the bootstrapping problem (how do you store the credentials to access the secret store?).

### Trade-offs accepted

**Key management burden**: Users must manage Age keys securely. Lost keys mean lost secrets.

**Mitigation**: Documentation emphasizes key backup. Keys are stored in a well-known location (`~/.config/openCenter/clusters/<org>/secrets/age/keys/`). Backup procedures are documented.

**No automatic key rotation**: Rotating keys requires re-encrypting all secrets.

**Mitigation**: Key rotation is rare in practice. When needed, SOPS provides tools for re-encryption. For most users, generate keys once and use them for the cluster's lifetime.

## Dependency injection container

### The decision

Use a dependency injection (DI) container to manage component lifecycle and dependencies.

### Why we chose this

**Testability**: Components receive dependencies as parameters, not globals. Tests can inject mocks easily.

**Explicit dependencies**: Each component declares what it needs. No hidden dependencies on global state.

**Lifecycle management**: The container manages initialization order and cleanup. Components don't need to know about each other's lifecycle.

**Flexibility**: Swap implementations without changing consumers. Use a real ConfigManager in production, a mock in tests.

### Alternatives considered

**Global variables**: Store shared components in package-level variables.

**Why we rejected it**: Global state makes testing hard (tests interfere with each other). Makes code hard to understand (where is this variable set?). Makes refactoring risky (who depends on this global?).

**Manual dependency passing**: Pass dependencies explicitly through function parameters.

**Why we rejected it**: Works for small systems but doesn't scale. Commands would need to accept 5+ parameters. Adding a new dependency requires changing every call site.

**Service locator pattern**: Components request dependencies from a global registry.

**Why we rejected it**: Hides dependencies (you can't tell what a component needs by looking at its signature). Makes testing harder (must set up global registry). The DI container provides the same benefits with explicit dependencies.

### Trade-offs accepted

**Boilerplate**: Setting up the DI container requires registration code for each component.

**Mitigation**: Registration is centralized in `internal/di/setup.go`. Adding a component requires one registration call. The benefit (testability, clarity) outweighs the cost.

**Learning curve**: Developers must understand DI concepts.

**Mitigation**: The pattern is well-documented and widely used in the industry. Once learned, it applies to many projects.

## Mise instead of Make

### The decision

Use Mise for task automation instead of Make.

### Why we chose this

**Tool version management**: Mise manages tool versions (Go, golangci-lint, etc.) in addition to running tasks. Make doesn't handle tool installation.

**Cross-platform**: Mise works identically on Linux, macOS, and Windows. Make requires different syntax for different platforms.

**Modern syntax**: Mise uses TOML configuration, which is more readable than Makefile syntax. No tab vs. space issues.

**Better developer experience**: `mise tasks` shows all available tasks with descriptions. `mise run <task>` provides consistent interface.

### Alternatives considered

**Make**: Traditional build automation tool.

**Why we rejected it**: Make is ubiquitous but has limitations. No tool version management. Platform-specific syntax. Cryptic error messages. Tab sensitivity causes frustration.

**Just**: Modern command runner similar to Make.

**Why we rejected it**: Just is excellent but doesn't handle tool versions. We'd need Just + asdf/mise for tool management. Using Mise for both simplifies the toolchain.

**Task**: Task runner with YAML configuration.

**Why we rejected it**: Similar to Just—good for tasks but not tool management. Mise provides both in one tool.

**Shell scripts**: Write bash scripts for common tasks.

**Why we rejected it**: Scripts work but lack discoverability (how do you find available scripts?). No built-in help. No dependency management between tasks.

### Trade-offs accepted

**Additional tool**: Developers must install Mise.

**Mitigation**: Mise installation is simple (`curl | sh` or package manager). Once installed, it manages all other tools automatically.

**Less familiar**: Make is more widely known than Mise.

**Mitigation**: Mise syntax is simpler than Make. The learning curve is minimal. Documentation includes Mise examples.

## Atomic file operations for GitOps generation

### The decision

GitOps repository generation uses atomic operations—all files are written successfully or none are written.

### Why we chose this

**Reliability**: No partial writes that corrupt the repository. If generation fails, the repository is unchanged.

**Rollback**: Failed operations leave no artifacts. No manual cleanup required.

**Concurrency**: Multiple operations don't interfere. Atomic operations prevent race conditions.

**Production safety**: GitOps repositories are the source of truth for production infrastructure. Corruption is unacceptable.

### Alternatives considered

**Direct file writes**: Write files directly to the target directory.

**Why we rejected it**: If generation fails partway through, you have a partially-written repository. Which files are valid? Which need to be regenerated? Manual cleanup is error-prone.

**Backup and restore**: Backup the repository before generation, restore on failure.

**Why we rejected it**: Requires disk space for backups. Restore is manual. Doesn't prevent corruption during the write itself (e.g., disk full).

**Transactional filesystem**: Use a filesystem with transaction support.

**Why we rejected it**: Not all filesystems support transactions. Requires specific filesystem features. Atomic operations work on any filesystem.

### Trade-offs accepted

**Complexity**: Atomic operations require workspace management, temporary files, and careful error handling.

**Mitigation**: The complexity is encapsulated in `GitOpsWorkspace` and `AtomicWriter`. Commands use a simple interface and don't need to understand the implementation.

**Disk space**: Temporary files require additional disk space during generation.

**Mitigation**: Temporary files are small (a few MB) and cleaned up immediately after generation. The space cost is negligible.

## Template sandboxing

### The decision

The template engine supports a sandbox mode that restricts template capabilities.

### Why we chose this

**Security**: Untrusted templates can't execute arbitrary code, read files, or access environment variables.

**Future-proofing**: Enables future features like user-provided templates or template marketplace.

**Defense in depth**: Even if a template has a vulnerability, sandbox mode limits the damage.

### Alternatives considered

**No sandboxing**: Trust all templates.

**Why we rejected it**: Works for embedded templates (we control them) but prevents future extensibility. Can't safely render user-provided templates.

**Separate template engine**: Use a restricted template language (Mustache, Handlebars).

**Why we rejected it**: Go templates are powerful and widely used in the Kubernetes ecosystem. Switching would lose compatibility with existing templates and reduce functionality.

**Runtime restrictions**: Use OS-level restrictions (containers, VMs) to limit template execution.

**Why we rejected it**: Too heavyweight for template rendering. Adds operational complexity. Sandbox mode provides sufficient protection with minimal overhead.

### Trade-offs accepted

**Reduced functionality**: Sandboxed templates can't use some Sprig functions (env, exec, readFile).

**Mitigation**: Sandbox mode is optional. Embedded templates run without sandbox (we trust them). User-provided templates run with sandbox (we don't trust them).

**Performance overhead**: Sandbox mode adds function call overhead.

**Mitigation**: The overhead is negligible (nanoseconds per function call). Template rendering is dominated by I/O, not function calls.

## Why these decisions matter

These design decisions aren't arbitrary—they reflect hard-won lessons from operating production systems:

**Simplicity reduces operational risk**: A single configuration file, embedded templates, and atomic operations all prioritize reliability over flexibility.

**Explicit is better than implicit**: Dependency injection, staged validation, and organization structure make the system's behavior predictable and understandable.

**Security by default**: SOPS encryption, template sandboxing, and isolated plugins protect against common security issues.

**Developer experience matters**: Mise, clear error messages, and comprehensive documentation make the tool pleasant to use.

The common thread is **production reliability through opinionated simplicity**. openCenter makes decisions for you, and those decisions are informed by operational experience. This isn't the right approach for every tool, but for infrastructure management, it's the right trade-off.

## Evolution and future decisions

Design decisions aren't permanent. As openCenter evolves, some decisions may change:

**Plugin API**: Currently, plugins are separate executables. A future plugin API might allow tighter integration while maintaining isolation.

**Configuration composition**: The single-file approach works well today. If users consistently need composition, we might add it—but only with careful design to preserve simplicity.

**Alternative secret backends**: SOPS works well for GitOps. If users need integration with Vault or cloud secret managers, we might add adapters—but SOPS remains the default.

**Template customization**: Embedded templates are the right default. A future template override mechanism might allow customization without forking—but only if it doesn't compromise version consistency.

The key is that changes must preserve the core philosophy: **production reliability through opinionated simplicity**. New features are welcome if they maintain this principle.

## See also

- **[Architecture](./architecture.md)**: How these decisions fit into the overall system
- **[Plugin System](./plugin-system.md)**: Plugin architecture and extensibility
- **[Security Model](./security-model.md)**: Security design and threat model
- **[Configuration Reference](../reference/configuration.md)**: Configuration structure and options

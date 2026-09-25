---
last_updated: 2026-09-24
id: cli-llm-code-map
title: "Navigate the openCenter CLI Code Map"
sidebar_label: LLM Code Map
description: Maps openCenter CLI entry points, package responsibilities, provider/storage operations, data flows, and safe change boundaries.
doc_type: explanation
audience: "contributors, maintainers, code-oriented agents"
tags: [architecture, code-map, cli, packages, providers]
---
# LLM Code Map

This map is for humans and automated agents changing openCenter CLI. Read it before moving code across packages. For historical evidence, also read `docs/refactor-audit-report.md`, `dead-code-removal-report.md`, `deduplication-report.md`, and `docs/library-extraction-report.md`.

## Main entrypoints

| Path | Role |
| --- | --- |
| `main.go` | Production CLI process entrypoint, build metadata, compatibility bootstrap, exit mapping, and shutdown. |
| `cmd/root.go` | Root Cobra tree, early path selection, typed graph creation, command registration, and external plugin attachment. |
| `cmd/opencenter-local/main.go` | Local-development executable. |
| `cmd/docs/generate.go` | Tools-constrained command-reference generator; output can depend on discovered plugins. |
| `hack/generate_relaypoint_fixture_configs.go` | Fixture configuration generator. |

Command constructors follow `New...Cmd` naming in `cmd/`. Start at the constructor, then follow its service call into `internal`; do not infer a workflow from filenames alone.

## Where application wiring happens

The canonical dependency graph is `internal/di.App`, constructed explicitly by `internal/di.NewApp` in `internal/di/app.go`. Provider constructors are grouped in `internal/di/providers.go`. `cmd/root.go` creates the graph and places it in the command context.

`internal/di.NewAppContainer` is a read-only adapter for callers that still resolve names through `di.Container`. `internal/di.SetupContainer` and `internal/di.NewContainer` retain reflection-driven compatibility behavior. When adding a dependency:

1. Add or reuse a focused constructor in the owning package.
2. Wire it explicitly in `di.NewApp`.
3. Add a typed `App` field only when command wiring needs it.
4. Extend the container adapter only for a real compatibility caller.
5. Do not add new reflection registrations as the default path.

## Important packages and responsibilities

| Area | Start here | Boundary |
| --- | --- | --- |
| Commands | `cmd/root.go` and the relevant `cmd/*` constructor | Parsing, prompts, orchestration, presentation. No reusable business rules. |
| Typed wiring | `internal/di/app.go` | Construction only. Lower layers never import it. |
| Configuration | `internal/config/v2/manager.go`, `loader.go`, `resolver.go`, `validator.go` | Typed schema and processing order. Treat serialized behavior as compatible API. |
| Paths | `internal/core/paths` and focused configuration path code | Filesystem layout and resolution policy. Avoid command-local path rules. |
| Cluster services | `internal/cluster` | Lifecycle use cases, options, and results. |
| Provider orchestration | `internal/cluster/orchestration` | Capability contracts, prompts, reviews, and provider coordination. |
| Provider clients | `internal/cloud` and its provider subpackages | Cloud-specific discovery and mutation. |
| OpenStack provider/storage operations | `internal/cluster/provider/openstack`, `internal/cluster/storage/openstack` | Typed provider planning plus explicit one-service storage provisioning, credential sequencing, and recovery-aware persistence. |
| GitOps | `internal/gitops` and `internal/gitops/stages` | Workspace lifecycle, generation order, transactions, checkpoints, and dry-run state. |
| Templates | `internal/template` | Registry, context, rendering, composition, sandboxing, and dependency resolution. |
| Service metadata | `internal/services` and `internal/services/descriptors` | Service plugin contracts and embedded descriptors. |
| Secrets | `internal/secrets` | High-level secret lifecycle and policy. |
| SOPS | `internal/sops` | Terminal-independent encryption, decryption, key, and Git primitives. |
| Operations | `internal/operations` | Backup, recovery, drift, and scheduling. |
| External plugins | `internal/plugins` | Executable discovery and attachment. |
| Local environment | `internal/localdev` | Flux, Gitea, API, and GitOps service coordination. |
| Provisioning | `internal/provision` and `internal/tofu` | Embedded provisioning assets and OpenTofu execution. |
| UI | `internal/ui` | Terminal rendering and error presentation. |
| Focused legacy primitives | `internal/util/*` | Existing narrow primitives only; not a general extension point. |

## Key types and interfaces

- `di.App` is the typed runtime graph; `di.Container` is the compatibility service-locator contract.
- `config/v2.Config` is the authoritative serialized configuration root. `ConfigurationManager`, `ConfigLoader`, `ReferenceResolver`, and `Validator` coordinate its lifecycle.
- Cluster service types such as `InitService`, `ConfigureService`, `ValidateService`, and `DestroyService` expose use-case operations with explicit option and result values.
- `cluster/orchestration.ProviderOrchestrator` and `CapabilityHandler` define provider workflow boundaries; `PromptRunner` keeps interaction substitutable.
- `cloud.CloudProvider` and `CloudProviderFactory` own the shared provider boundary.
- `gitops.WorkspaceManager`, `AtomicWriter`, and `Transaction` govern workspace mutation and recovery behavior.
- `template.TemplateEngine`, `TemplateRegistry`, `TemplateComposer`, and `TemplateSandbox` separate template selection, execution, composition, and safety.
- `secrets.SecretsManager`, `KeyRegistry`, `KeyRotator`, `KeyRevoker`, and `HookManager` own high-level secret policy.
- `sops.SOPSManager` and `Encryptor` expose encryption operations without terminal behavior.
- `operations.BackupManager`, `DriftDetector`, and its consumer-defined provider/config interfaces own operational workflows.
- `services.ServicePlugin` and `ServiceRegistry` own service metadata and lifecycle extension.

Before adding an interface, check whether the consumer needs substitution. Prefer a small consumer-owned interface over mirroring every method of a concrete provider.

## Common workflows

### Change configuration behavior

1. Start with the relevant type under `internal/config/v2`.
2. Trace loader, normalization, reference, default, validation, and persistence order.
3. Search YAML and JSON tags, schema output, fixtures, property tests, and command consumers.
4. Add compatibility or migration coverage before changing serialized behavior.
5. Validate documentation drift and derived schema handling separately.

### Add a cluster use case

1. Put reusable options, results, and behavior in `internal/cluster` or a specifically named subpackage.
2. Keep provider-specific API calls in the provider package.
3. Wire required dependencies through `di.NewApp` when the root graph owns them.
4. Add a thin Cobra constructor for parsing, interaction, and output.
5. Test the service independently, then test command translation and errors.

### Change GitOps generation

1. Identify the generation stage and its order in `internal/gitops/stages`.
2. Trace template registration and embedded asset names.
3. Preserve dry-run, checkpoint, transaction, and rollback semantics.
4. Test exact file paths, contents, modes, and ordering where observable.

### Change secret encryption

1. Keep prompts and terminal output in `cmd`.
2. Keep encryption, decryption, keys, and Git primitives in `internal/sops`.
3. Keep rotation, registry, revocation, and multi-cluster policy in `internal/secrets`.
4. Characterize key-source precedence, permissions, replacement behavior, and file order before refactoring.
5. Verify no plaintext or key material reaches logs, errors, fixtures, or snapshots.

### Add or change a provider

1. Implement provider behavior under `internal/cloud/<provider>`.
2. Register construction through the provider factory or orchestration registry that owns the use case.
3. Keep credentials and provider API types out of `cmd` where possible.
4. Test discovery separately from mutation and cover transient-error classification.

### Change an external command plugin

1. Start in `internal/plugins` for discovery rules and `cmd/root.go` for attachment.
2. Treat executable names, metadata, environment, checksums, arguments, output, and exit behavior as compatibility surfaces.
3. Test with controlled directories; do not depend on the developer machine's installed plugins.

## Where to add new features

- Add CLI-only parsing, confirmation, and presentation to the relevant file in `cmd`.
- Add stable cluster use cases to `internal/cluster` or a specific cluster subpackage.
- Add provider behavior to its provider package.
- Add schema and configuration lifecycle behavior to `internal/config/v2`.
- Add reusable rendering behavior to `internal/template`; add generation sequencing to `internal/gitops/stages`.
- Add high-level key and secret policy to `internal/secrets`; add terminal-free encryption primitives to `internal/sops`.
- Add operational backup, recovery, or drift behavior to `internal/operations` when it has an independently testable contract.
- Add a new `internal/<concept>` package only when the concept has a specific name, multiple real consumers or a stable integration boundary, a small API, minimal dependencies, independent tests, and clearer call sites.

## Where not to add code

- Do not put domain logic in `main.go`, `cmd/root.go`, Cobra constructors, or DI providers.
- Do not import `cmd` from `internal` packages.
- Do not add new general `utils`, `common`, `helpers`, `misc`, or `shared` packages.
- Do not use `internal/util` as a default home for unrelated reuse; its subpackages are existing focused compatibility areas.
- Do not create `pkg/` unless the Go API is intentionally supported for external modules.
- Do not add reflection registrations when typed constructor wiring is possible.
- Do not create bridge packages to work around a dependency cycle; move ownership to the layer that owns the contract.
- Do not split a package solely to reduce file length.
- Do not add provider-specific policy to shared cloud, configuration, or command code.

## Generated-code and embedded-asset warnings

- `schema/opencenter-v2.schema.json` is derived. Confirm a trustworthy regeneration path before editing it.
- `docs/reference/opencenter/` is generated from `cmd/docs/generate.go`; the result may include host-discovered plugins.
- `hack/generate_relaypoint_fixture_configs.go` owns a fixture generation path.
- Shell integration, service descriptor YAML, provisioning templates, GitOps base content, and GitOps templates are embedded. Renames can break runtime lookup even when Go compilation succeeds.
- There are no marked generated Go files today. Re-check before moving any file that looks generated.
- Some `.mise.toml` generation entries are stale. Do not use a task merely because its name suggests authority; inspect the command and source first.

## Dynamic-reference warnings

Static call graphs do not cover all reachability in this repository. Search these before deleting or renaming code or data:

- Cobra command registration and parent-child attachment.
- `init` functions and package side effects.
- Reflection-based DI names and the adapter's canonical name mapping.
- External plugin executable names and runtime discovery.
- Configuration keys, YAML and JSON tags, reference expressions, environment variables, and filesystem conventions.
- Embedded paths, template names, service descriptor identifiers, and provider names.
- String-based callback, hook, and lifecycle registrations.
- Tools constraints and non-default test tags.

For deletion work, require both static evidence and dynamic-reference searches, then run the relevant integration or property tests.

## Common test commands

Run from the module root:

```sh
go test ./path/to/changed/package
go test ./cmd/...
go test ./...
go test ./internal/... ./cmd/... -count=1 -race
go vet ./...
golangci-lint run
go mod tidy -diff
```

Use the repository-selected Go toolchain. Run formatting on changed Go files before validation. Tools-constrained generators, integration suites, property suites, and vulnerability checks may require separate commands or installed tools; inspect `.mise.toml` and CI workflows before invoking them.

## Safe refactor guidelines for future agents

1. Read the architecture and the three prior refactor reports before choosing a boundary.
2. Record the current tests and lint state so inherited debt is not mistaken for a regression.
3. Preserve CLI behavior, exit codes, serialized configuration, path layout, plugin conventions, and asset names unless an explicit compatibility change is authorized.
4. Characterize ordering, error wrapping, cancellation, filesystem modes, prompts, and dry-run semantics before moving code.
5. Extract one stable concept at a time. Update call sites and run its package tests immediately.
6. Define interfaces at the consumer when substitution is needed; avoid interface layers used only to rename a concrete type.
7. Reject cycles by rethinking ownership, not by adding a shared bridge.
8. Treat generated, embedded, reflected, and string-referenced artifacts as live until proven otherwise.
9. Use `internal` by default. Use `pkg` only with an explicit external support commitment.
10. Prefer no extraction when the API would need policy switches, higher-layer imports, or speculative future consumers.
11. Run formatting, targeted tests, the full suite, vet, lint, module tidy diff, and final diff inspection before claiming completion.

---
last_updated: 2026-09-25
id: di-container-map
title: "Explain the Application Dependency Graph"
sidebar_label: DI Container
description: "Execution map for typed application construction, command context injection, compatibility wiring, and shutdown."
doc_type: explanation
audience: "contributors, maintainers"
tags: [dependency-injection, runtime, services, security, wiring]
---
# Dependency injection and runtime wiring

## Feature → subsystem → symbol

| Feature | Subsystem / package | File → symbol | Dependencies and evidence |
|---|---|---|---|
| Process container | startup / `internal/di` | [`internal/di/setup.go`](../../internal/di/setup.go) → `SetupContainer`; [`internal/di/container.go`](../../internal/di/container.go) → `Container`, `DIContainer.Shutdown` | Outer compatibility container used by `main`; `setup.go` builds and registers it, `container.go` defines the interface and the registry its `Shutdown` drains; [`cmd/root_container_test.go`](../../cmd/root_container_test.go) |
| Typed graph | application graph / `internal/di` | [`internal/di/app.go`](../../internal/di/app.go) → `App`, `NewApp` | Explicit constructors for paths, config, validation, security, lifecycle; [`internal/di/app_test.go`](../../internal/di/app_test.go) |
| Command adapter | compatibility boundary / `internal/di` | [`internal/di/app_container.go`](../../internal/di/app_container.go) → `NewAppContainer` | Exposes typed graph through legacy `Container`; command context uses `AppKey` and `ContainerKey` |
| Context retrieval | command runtime / `cmd` | [`cmd/root.go`](../../cmd/root.go) → `initializeApp`, `GetApp`, `GetContainer`, `ExecuteWithContext` | Cobra handlers retrieve graph values, not global service constructors |
| Lifecycle services | `internal/cluster` | [`internal/di/providers.go`](../../internal/di/providers.go) → `ProvideInitService`, `ProvideConfigureService`, `ProvideValidateService`, `ProvideSetupService`, `ProvideBootstrapService` | Depends on `PathResolver`, `ValidationEngine`, and configuration managers |
| Cross-cutting security | `internal/security` | [`internal/di/providers.go`](../../internal/di/providers.go) → `ProvideAuditLogger`, `ProvideInputValidator`, `ProvideCredentialMasker`, `ProvideCommandSanitizer`, `ProvideCommandRunner` | External process and credential boundaries; [`internal/security/command_runner_test.go`](../../internal/security/command_runner_test.go) |

## Actual production path

```text
main.main
  -> config.ResolveClustersDir
  -> di.SetupContainer(baseDir)
  -> context[ContainerKey]
  -> cmd.ExecuteWithContext
       -> di.NewApp(baseDir)
            -> filesystem -> PathResolver -> logger
            -> ConfigManager -> ValidationEngine
            -> security providers
            -> Init/Configure/Validate/Setup/Bootstrap services
       -> di.NewAppContainer(app)
       -> context[AppKey, ContainerKey]
       -> NewBuiltinRootCmd -> plugin load -> ExecuteContext
```

`NewApp` is the command-critical source of truth. `initializeContainer` in [`cmd/root.go`](../../cmd/root.go) can fall back to `di.SetupContainer` only when typed startup wiring fails; new dependencies should not be added to the reflection-based registry merely to make a typed constructor work.

`cmd/opencenter-local/main.go` is deliberately outside this graph. Its `newRootCmd` constructs the local workflow services directly; it shares security/path/config packages where needed but is not a production subcommand.

## Safe-change boundaries

- Add command-critical services to `App` and explicit `Provide*` constructors first; use `NewAppContainer` only for compatibility callers.
- Preserve both context keys and shutdown ordering. `main` shuts down the outer process container on success and error.
- Keep constructors testable through dependency parameters; do not introduce hidden package globals or command-owned service singletons.
- A graph change affects all command handlers that call `GetApp`; update focused graph and root-container tests with it.

## Related maps

[CLI commands](cli-commands.md) · [Config system](config-system.md) · [Cluster lifecycle](cluster-lifecycle.md) · [Runtime extensions](runtime-extensions-and-local-development.md)

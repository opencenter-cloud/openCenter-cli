---
last_updated: 2026-09-25
id: cli-commands-map
title: "Map the Built-in CLI Commands"
sidebar_label: CLI Commands
description: "Execution map for built-in Cobra registration, command handlers, typed services, and the production-only plugin boundary."
doc_type: explanation
audience: "contributors, maintainers, CLI integrators"
tags: [cli, cobra, commands, plugins, runtime]
---
# CLI commands

## Feature → subsystem → symbol

| Feature | Subsystem / package | File → symbol | Dependencies and evidence |
|---|---|---|---|
| Process execution | process / `cmd` | [`main.go`](../../main.go) → `main`; [`cmd/root.go`](../../cmd/root.go) → `ExecuteWithContext` | `di.SetupContainer`, `di.NewApp`, Cobra; [`cmd/root_container_test.go`](../../cmd/root_container_test.go) |
| Built-in root | command registration / `cmd` | [`cmd/root.go`](../../cmd/root.go) → `NewBuiltinRootCmd`, `addGlobalFlags` | Adds `NewClusterCmd`, `NewSettingsCmd`, `NewSecretsCmd`, `NewPluginsCmd`, `NewVersionCmd`, `NewShellInitCmd`; [`cmd/root_test.go`](../../cmd/root_test.go) |
| Cluster tree | command registration / `cmd` | [`cmd/cluster.go`](../../cmd/cluster.go) → `NewClusterCmd` | Delegates to per-feature constructors; [`cmd/ga_command_surface_test.go`](../../cmd/ga_command_surface_test.go) |
| Lifecycle handlers | command orchestration / `cmd` | [`cmd/cluster_init.go`](../../cmd/cluster_init.go) → `newClusterInitCmd`; [`cmd/cluster_generate.go`](../../cmd/cluster_generate.go) → `newClusterGenerateCmd`; [`cmd/cluster_deploy.go`](../../cmd/cluster_deploy.go) → `newClusterDeployCmd`; [`cmd/cluster_destroy.go`](../../cmd/cluster_destroy.go) → `newClusterDestroyCmd` | Resolves `di.GetApp(ctx)` services; [`cmd/cluster_generate_test.go`](../../cmd/cluster_generate_test.go), [`cmd/cluster_deploy_test.go`](../../cmd/cluster_deploy_test.go) |
| OpenStack provider | explicit operation / `cmd` | [`cmd/cluster_provider_openstack.go`](../../cmd/cluster_provider_openstack.go) → `newClusterProviderOpenStackCmd`, `runClusterProviderOpenStack` | Depends on cloud profile/discovery and typed provider planner; see [OpenStack map](openstack-provider-storage-operations.md) |
| One-service storage | explicit operation / `cmd` | [`cmd/cluster_service_storage.go`](../../cmd/cluster_service_storage.go) → `newClusterServiceStorageCmd`, `runClusterServiceStorage` | Depends on `storage/openstack.Plan` and `Apply`; [`cmd/cluster_service_storage_test.go`](../../cmd/cluster_service_storage_test.go), [`cmd/task16_storage_contract_test.go`](../../cmd/task16_storage_contract_test.go) |
| Secrets | command routing / `cmd` | [`cmd/secrets.go`](../../cmd/secrets.go) → `NewSecretsCmd`; [`cmd/secrets_sync.go`](../../cmd/secrets_sync.go) → `newSecretsSyncCmd`, `runClusterSyncSecrets`; [`cmd/secrets_keys.go`](../../cmd/secrets_keys.go) → `NewSecretsKeysCmd` | Backend CRUD, manifest sync, SOPS files, and key lifecycle remain separate; [`cmd/secrets_router_test.go`](../../cmd/secrets_router_test.go) |
| Import | command orchestration / `cmd` | [`cmd/cluster_import.go`](../../cmd/cluster_import.go) → `newClusterImportCmd` and scan/report/apply constructors | Depends on `internal/importer`; [`cmd/cluster_import_test.go`](../../cmd/cluster_import_test.go) |
| External plugins | extension boundary / `internal/plugins` | [`internal/plugins/loader.go`](../../internal/plugins/loader.go) → `LoadExternalPlugins`, `DiscoverDetailed`, `runExternal` | `security.CommandRunner`, checksum file, PATH/config plugin dirs; [`internal/plugins/loader_test.go`](../../internal/plugins/loader_test.go), [`cmd/plugins_test.go`](../../cmd/plugins_test.go) |

## Actual registration and execution path

```text
main.main
  -> cmd.ExecuteWithContext
       -> pre-parse --config-dir
       -> initializeApp -> di.NewApp
       -> context[AppKey, ContainerKey]
       -> plugins.LoadExternalPlugins(rootCmd)
       -> rootCmd.ExecuteContext
            -> PersistentPreRunE -> applyGlobalOptions
            -> command RunE
            -> typed service or feature package
```

`NewBuiltinRootCmd` is deterministic and is also the input to generated command documentation. Production attaches external executables only inside `ExecuteWithContext`; therefore generated references and tests of the built-in tree must not expect plugins. Built-ins cannot be shadowed by a plugin name. Hidden `cluster`-`template` and `cluster validate-manifests` commands are registered for internal workflows, not the normal GA surface. Both set `Hidden: true` (`cmd/cluster_template.go:32`, `cmd/cluster.go:110-113`), so `IsAvailableCommand` is false and the Cobra `doc.GenMarkdownTreeCustom` recursion in `cmd/docs/generate.go:78` skips them: neither has a page under `docs/reference/opencenter/`. That absence is intentional; do not add pages for them or treat the missing reference as drift.

The registered top-level tree is `cluster`, `settings`, `secrets`, `plugins`, `version`, and `shell-init`; Cobra adds `help` and completion. `NewClusterCmd` owns lifecycle, service/storage, pool, drift, backup, lock, import, layout, and provider subtrees. The exact command-surface assertions live in [`cmd/ga_command_surface_test.go`](../../cmd/ga_command_surface_test.go), not in this prose.

## Safe-change boundaries

- Keep registration in `cmd/`; keep domain behavior in `internal/*`. A handler should resolve a typed service rather than duplicate config, rendering, or provider logic.
- Changes to `NewBuiltinRootCmd` affect generated references; changes to `LoadExternalPlugins` affect only production runtime.
- Preserve global flag pre-parsing: plugin discovery needs `--config-dir` before Cobra hooks and DI initialization.
- Preserve `DisableFlagParsing` and argument forwarding for plugins, checksum refusal, built-in collision protection, and plugin exit behavior.
- Provider plan/apply has no remote mutation; storage apply does. Do not merge either into lifecycle deploy or secrets sync.

## Related maps

[DI container](di-container.md) · [Cluster lifecycle](cluster-lifecycle.md) · [Providers](providers.md) · [Runtime extensions](runtime-extensions-and-local-development.md) · [Secrets management](secrets-management.md)

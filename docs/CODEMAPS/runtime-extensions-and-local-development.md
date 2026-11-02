---
last_updated: 2026-09-25
id: runtime-extensions-and-local-development
title: "Explain Runtime Extensions and Local Development"
sidebar_label: Runtime Extensions
description: "Execution map for production external plugins and the separate opencenter-local Kind, Gitea, Flux, and RustFS workflow."
doc_type: explanation
audience: "contributors, maintainers, plugin authors"
tags: [plugins, local-development, gitea, flux, rustfs, templates]
---
# Runtime extensions and local development

External plugins extend the production root at runtime. `opencenter-local` is a separate executable with its own Cobra root and direct local-development services; it is not a production subcommand.

## Feature → subsystem → symbol

| Feature | Subsystem / package | File → symbol | Dependencies and evidence |
|---|---|---|---|
| Production plugin attach | extension loader / `internal/plugins` | [`internal/plugins/loader.go`](../../internal/plugins/loader.go) → `LoadExternalPlugins`, `DiscoverDetailed`, `runExternal` | Cobra root, config/plugin dirs, PATH, SHA-256 checks, `security.CommandRunner`; [`internal/plugins/loader_test.go`](../../internal/plugins/loader_test.go) |
| Plugin command | production runtime / `cmd` | [`cmd/root.go`](../../cmd/root.go) → `ExecuteWithContext` | Loads plugins after typed graph/context setup and before `ExecuteContext`; [`cmd/plugins_test.go`](../../cmd/plugins_test.go) |
| Local root | separate executable | [`cmd/opencenter-local/main.go`](../../cmd/opencenter-local/main.go) → `main`, `newRootCmd` | Registers Gitea, RustFS, GitOps, Flux; [`cmd/opencenter-local/main_test.go`](../../cmd/opencenter-local/main_test.go) |
| Cluster resolution | localdev / `internal/localdev` | [`internal/localdev/cluster.go`](../../internal/localdev/cluster.go) → `NewClusterResolver`, `ClusterResolver.Resolve` | Shared v2 `ConfigurationManager` and `PathResolver`; [`internal/localdev/layout_test.go`](../../internal/localdev/layout_test.go) |
| Gitea | localdev / `internal/localdev/gitea` | [`internal/localdev/gitea/service.go`](../../internal/localdev/gitea/service.go) → service lifecycle methods | container executor, Gitea API, Kind network; [`internal/localdev/gitea/service_test.go`](../../internal/localdev/gitea/service_test.go) |
| RustFS | localdev / `internal/localdev/rustfs` | [`internal/localdev/rustfs/service.go`](../../internal/localdev/rustfs/service.go) → `Service.Up`, `Status`, `Destroy`, `AttachKindWithKubeconfig` | `localdev.Executor`, runtime state, S3/health probes, kubeconfig; `AttachKind` is a hard-error stub, so attachment is asymmetric with Gitea and only the kubeconfig variant runs; [`internal/localdev/rustfs/service_test.go`](../../internal/localdev/rustfs/service_test.go) |
| GitOps push / Flux bootstrap | localdev / `internal/localdev/gitops`, `flux` | [`internal/localdev/gitops/service.go`](../../internal/localdev/gitops/service.go), [`internal/localdev/flux/service.go`](../../internal/localdev/flux/service.go) | `ClusterResolver`, Gitea and sanitized executor; localdev tests |
| Reusable templates | generic template / `internal/template` | [`internal/template/engine.go`](../../internal/template/engine.go) → `TemplateEngine`; [`internal/template/registry.go`](../../internal/template/registry.go) → registries | Rendering primitives only; GitOps chooses topology; [`internal/template/engine_test.go`](../../internal/template/engine_test.go) |
| Shared command security | security / `internal/security` | [`internal/security/command_sanitizer.go`](../../internal/security/command_sanitizer.go), `command_runner.go`, `input_validator.go`, `credential_masker.go` | No direct unsafe `os/exec` boundary; security tests |

## Actual plugin path

```text
cmd.ExecuteWithContext
  -> pre-parse --config-dir
  -> NewBuiltinRootCmd + typed context
  -> plugins.LoadExternalPlugins
       -> OPENCENTER_PLUGINS_DIR
       -> <config-dir>/plugins
       -> PATH
       -> names beginning opencenter-
       -> checksum status
       -> skip built-in collisions
       -> attach Cobra command
  -> plugin RunE -> security runner -> external process
```

Verified plugins run, unverified plugins warn, checksum mismatches/verification errors refuse. Generated command references call `NewBuiltinRootCmd` directly and therefore never discover plugins.

## Actual local path

```text
opencenter-local newRootCmd
  -> gitea {up,status,destroy,attach-kind}
  -> rustfs {up,status,destroy,attach-kind}
  -> gitops push
  -> flux bootstrap
       -> ClusterResolver.Resolve (where cluster is required)
       -> local service -> Executor -> container/API/kubectl process
```

RustFS `up` waits for S3 and health readiness, `status` avoids credential disclosure, `destroy` removes disposable data/state, and `attach-kind` explicitly connects the service then probes authenticated reachability. `up` does not attach to Kind automatically. Attachment is not symmetric with Gitea: Gitea's `AttachKind` performs the network attach and is also called from `up`, while RustFS `AttachKind` returns `RustFS Kind attachment requires a kubeconfig for the authenticated workload probe` unconditionally, and the `attach-kind` command calls `AttachKindWithKubeconfig` with the resolved cluster kubeconfig.

## Safe-change boundaries

- Do not add local commands to the production root or external plugins to the local executable.
- Preserve plugin prefix, discovery order, collision protection, checksum statuses, transparent args, and exit behavior.
- Pass external commands through the security runner and local executor; preserve input validation and credential masking.
- Keep `internal/template` generic. GitOps owns embedded template selection and output ownership; localdev owns disposable infrastructure state.
- Changes to cluster path/config resolution affect both production and local workflows; update the corresponding localdev tests.

## Related maps

[CLI commands](cli-commands.md) · [DI container](di-container.md) · [Providers](providers.md) · [GitOps engine](gitops-engine.md)

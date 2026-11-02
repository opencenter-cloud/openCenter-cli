---
last_updated: 2026-09-25
id: gitops-engine-map
title: "Explain the GitOps Generation Engine"
sidebar_label: GitOps Engine
description: "Execution map for staged GitOps rendering, template/materialization stages, validation, ownership preflight, and promotion."
doc_type: explanation
audience: "contributors, maintainers"
tags: [gitops, rendering, flux, kustomize, templates]
---
# GitOps engine

## Feature → subsystem → symbol

| Feature | Subsystem / package | File → symbol | Dependencies and evidence |
|---|---|---|---|
| Live generation entry | lifecycle → GitOps | [`internal/cluster/setup_service.go`](../../internal/cluster/setup_service.go) → `SetupService.generateGitOpsManifestsWithPromotion` | v2 config, SOPS encryptor, optional `tofu.ProvisionAt`; [`cmd/cluster_generate_test.go`](../../cmd/cluster_generate_test.go) |
| One staged transaction | workspace pipeline / `internal/gitops` | [`internal/gitops/copy.go`](../../internal/gitops/copy.go) → `StagedGenerationOptions`, `GenerateClusterTree` | `WorkspaceManager`, atomic renderers, ownership promoter; [`internal/gitops/copy_test.go`](../../internal/gitops/copy_test.go) |
| Base and application output | render modules / `internal/gitops` | [`internal/gitops/copy.go`](../../internal/gitops/copy.go) → `CopyBaseAtomic`, `RenderClusterAppsAtomic` | embedded base/templates, descriptor and catalog planners |
| Infrastructure and Flux bridge | render modules / `internal/gitops` | [`internal/gitops/copy.go`](../../internal/gitops/copy.go) → `RenderInfrastructureClusterAtomic`, `RenderClusterFluxBridgeAtomic` | provider config, template registry; [`internal/gitops/generator_test.go`](../../internal/gitops/generator_test.go) |
| Service decisions | descriptors/catalog / `internal/gitops` | [`internal/gitops/descriptor_renderer.go`](../../internal/gitops/descriptor_renderer.go) → `planClusterAppActions`, `validateClusterAppActions`; [`internal/gitops/auto_descriptor.go`](../../internal/gitops/auto_descriptor.go) → `planAutoServiceActions`; [`internal/gitops/render_catalog.go`](../../internal/gitops/render_catalog.go) → `newBuiltInRenderCatalog` | v2 service config, embedded descriptor registry, secret-artifact plan |
| Manifest and secret checks | validation/encryption | [`internal/gitops/validators.go`](../../internal/gitops/validators.go) → `ManifestValidator.Validate`; [`internal/sops/manager.go`](../../internal/sops/manager.go) → `EncryptServiceOverrideValues` | SOPS/Age, YAML/Kustomize validation; [`cmd/cluster_generate_render_integration_test.go`](../../cmd/cluster_generate_render_integration_test.go) |
| Promotion | ownership / `internal/gitops` | [`internal/gitops/ownership.go`](../../internal/gitops/ownership.go) → `promoteGeneratedTree`, `planRepositoryPromotion`, `applyGeneratedTreePlan` | v2 ledgers, hashes/modes, custom and secret-artifact exclusions; [`internal/gitops/ownership_test.go`](../../internal/gitops/ownership_test.go) |
| Atomic file writes | workspace / `internal/gitops` | [`internal/gitops/atomic.go`](../../internal/gitops/atomic.go) → `NewAtomicWriter`, `AtomicWriter.WriteFile`; [`internal/gitops/workspace.go`](../../internal/gitops/workspace.go) → `CreateWorkspace` | temporary workspace and rename semantics; [`internal/gitops/staged_generation_test.go`](../../internal/gitops/staged_generation_test.go) |

## Actual execution path

```text
SetupService.Setup
  -> GenerateClusterTree
       -> CreateWorkspace(os.TempDir)
       -> CopyBaseAtomic
       -> RenderClusterAppsAtomic
       -> RenderInfrastructureClusterAtomic (when included)
       -> RenderClusterFluxBridgeAtomic (when included)
       -> Materialize (OpenTofu for non-Kind/non-Magnum)
       -> Encrypt staged service overrides
       -> ValidateManifest
       -> promoteGeneratedTree(..., DryRun=true) ownership preflight
       -> countWorkspaceFiles
       -> [dry-run return] or promoteGeneratedTree(..., DryRun=false)
       -> cleanup workspace
```

The same private tree is validated, planned, and promoted; the live repository is not written before preflight succeeds. `PipelineGenerator` and stage types remain supporting library/test APIs in [`internal/gitops/pipeline.go`](../../internal/gitops/pipeline.go), not the command's top-level entry.

Promotion records repository-relative SHA-256/mode records in `.opencenter/ownership/clusters/<cluster>.json`, and the same records for the built-in global files in `.opencenter/ownership/global.json`. The global ledger only ever holds paths from the compiled `globalOwnershipFiles` set (`.gitignore`, `README.md`, `applications/overlays/.gitkeep`, `infrastructure/clusters/.gitkeep`; `internal/gitops/ownership.go`), so it is a manifest of promoted files, not a configurable allowlist. Cluster scopes are applications, infrastructure, and cluster bridge paths; Flux bootstrap, existing `custom/`, and hash-verified secret artifacts remain outside generator ownership. Legacy `.opencenter-generated.json` sentinels fail fast.

## Safe-change boundaries

- New output must be staged, normalized, declared/selected by descriptor or immutable catalog, and pass action containment before writing.
- Do not add a mutable renderer registry or let plugins inject renderers; `RenderCatalog` is compiled into the binary.
- Preserve ownership scope, ledger format/version, hash/mode conflict checks, dry-run semantics, `Prune: false`, and `AdoptGenerated` restrictions.
- Keep `SetupService` as the live command caller; changing supporting pipeline APIs does not change the command path unless wired there explicitly.
- Template changes can alter generated ownership and SOPS input; update renderer, ownership, and integration tests together.

## Related maps

[Cluster lifecycle](cluster-lifecycle.md) · [Rendering ownership](rendering-ownership-and-secret-artifacts.md) · [Secrets management](secrets-management.md) · [Config system](config-system.md)

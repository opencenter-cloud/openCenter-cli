---
last_updated: 2026-09-25
id: gitops-engine-map
title: "Explain the GitOps Generation Engine"
sidebar_label: GitOps Engine
description: Explains how validated configuration becomes an atomic Flux/Kustomize and infrastructure workspace through descriptors, render catalogs, templates, and promotion.
doc_type: explanation
audience: "contributors, maintainers"
tags: [gitops, rendering, flux, kustomize, templates]
---
# GitOps engine

`internal/gitops` owns generated repository output. The live command path is `SetupService.generateGitOpsManifests` in `internal/cluster/setup_service.go`, which calls GitOps operations directly. `PipelineGenerator` is a supporting staged-generation API, not the live top-level generate path.

## Live generation flow

```text
validated v2.Config
  -> CopyBase
  -> RenderClusterAppsWithEncryption
       -> descriptor registry and conditions
       -> immutable RenderCatalog
       -> secret artifact plan
       -> owned action plan and path validation
       -> auto-service actions for catalog-owned services
       -> atomic workspace writes
       -> promotion preserving custom/
  -> RenderInfrastructureCluster
  -> RenderClusterFluxBridge
  -> non-Kind tofu.Provision
  -> manifest validation and generated-file count
```

The application render is staged and promoted so the final GitOps target is not partially updated. Promotion is tracked in `internal/gitops/ownership.go` with repository-relative paths and separate version-2 ledgers. `.opencenter/ownership/clusters/<cluster>.json` records SHA-256 and mode records for the active cluster in `applications/overlays/<cluster>/`, `infrastructure/clusters/<cluster>/`, and `clusters/<cluster>/`; `clusters/<cluster>/flux-system/` is reserved for Flux bootstrap and excluded, while generated bridge files outside that directory remain owned. `.opencenter/ownership/global.json` records only the exact global allowlist (`.gitignore`, `README.md`, and the two repository `.gitkeep` files), never a recursive repository scope. Full-tree generation activates all applicable scopes and global files; applications-only and single-service promotion activate only the current application or service scope. Sibling cluster records are retained and neither inspected nor pruned by scoped updates.

Promotion diffs planned output against the applicable ledgers and live tree to classify each path as added, updated, unchanged, seeded, renamed, adopted, or pruned. A path with an on-disk hash or mode that no longer matches its ledger entry is an ownership conflict and blocks promotion rather than being silently overwritten. `custom/` subdirectories are excluded from generator ownership: missing staged defaults can be seeded, but existing custom files are not overwritten or pruned. Hash-verified secret artifacts remain owned by secret synchronization rather than the generator. The old repository-root or current overlay `.opencenter-generated.json` manifest is rejected before mutation; it is not migrated implicitly. Dry-run runs the same preflight without writing, `Prune: false` reports retained candidates, and `AdoptGenerated` backs up and claims only an untracked planned collision. `Force` does not override ownership safety.

## Package ownership

| Area | Files/packages | Responsibility |
|---|---|---|
| Base and workspace | `copy.go`, `workspace.go`, `atomic.go`, `dryrun.go` | Copy base structure, isolate workspaces, write atomically, and support dry runs |
| Descriptors | `descriptor_renderer.go`, `internal/services/descriptors` | Declare file roots, files, conditions, ownership, and render behavior |
| Auto rendering | `auto_descriptor.go` | Generate Flux sources, Kustomizations, namespaces, overrides, and resources for services without explicit descriptors |
| Catalog | `render_catalog.go` and renderer files | Hold immutable built-in service behavior as direct planning/rendering functions |
| Infrastructure | `infrastructure_renderer.go` and embedded templates | Select provider-specific infrastructure assets |
| Validation | `validators.go`, `security_scanner.go`, overlay validation | Validate manifests and scan generated output |
| Diagnostics | `render_diagnostics.go` | Record descriptor decisions and planned actions |
| Templates | `embed.go`, `templates/`, `internal/template` | Embed repository templates and provide reusable template execution primitives |

## Ownership and validation

`descriptor_renderer.go` and `auto_descriptor.go` build actions before writing. The planner validates descriptor coverage, config ownership, output containment, and action ownership. The immutable catalog supplies renderer functions without a mutable renderer registry or configuration-selected renderer names.

A service can be rendered by an explicit descriptor or by an enabled built-in catalog entry. Auto rendering skips disabled or externally managed services. Descriptor and catalog decisions are diagnostic data, not an invitation for a generated document or runtime plugin to inject arbitrary renderers.

## Infrastructure boundary

`RenderInfrastructureCluster` selects templates using the provider configuration. The generated infrastructure workspace is separate from application overlays. `RenderClusterFluxBridge` connects cluster-level Flux reconciliation to generated per-service overlays. Non-Kind generation may invoke OpenTofu after rendering; Kind skips that provisioning call.

## Supporting API versus command path

The staged `PipelineGenerator` abstraction, checkpoints, and stage interfaces remain useful for library consumers and tests. They should not be described as the command's current generation entry point: command execution reaches `SetupService`, which follows the direct render/copy/promotion sequence above.

## Related maps

- [Cluster lifecycle](cluster-lifecycle.md) — caller and post-render flow
- [Rendering ownership and secret artifacts](rendering-ownership-and-secret-artifacts.md) — detailed planning boundary
- [Config system](config-system.md) — validated input contract
- [Secrets management](secrets-management.md) — encrypted overlay values and manifest sync

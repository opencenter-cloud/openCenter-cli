---
last_updated: 2026-09-25
id: rendering-ownership-and-secret-artifacts
title: "Explain Rendering Ownership and Secret Artifacts"
sidebar_label: Rendering Ownership
description: Explains how descriptors, the immutable render catalog, action validation, custom-file preservation, and secret artifact planning divide ownership during GitOps generation.
doc_type: explanation
audience: "contributors, maintainers"
tags: [rendering, ownership, descriptors, catalog, secrets]
---
# Rendering ownership and secret artifacts

The renderer separates three questions: which service is enabled, which code owns an output path, and which logical secret payload becomes a physical manifest. Keeping these decisions explicit prevents cross-service overwrites and unsafe adoption.

## Planning flow

```text
v2.Config
  -> descriptor registry + config view
  -> immutable RenderCatalog
  -> secretartifacts.Plan
  -> descriptor actions + dynamic catalog actions + auto-service actions
  -> validateClusterAppActions
  -> AtomicWriter workspace
  -> promotion to applications/overlays/<cluster>
```

`internal/gitops/descriptor_renderer.go` owns descriptor expansion, coverage checks, action planning, output normalization, and ownership validation. `internal/gitops/auto_descriptor.go` fills the gap for enabled services with no explicit descriptor, but only when a built-in catalog entry exists. `render_catalog.go` supplies direct function references and validates config ownership; it is not a mutable plugin registry.

## Ownership rules

| Decision | Source of truth | Consequence |
|---|---|---|
| Descriptor enabled | Service/managed-service config and descriptor conditions | Disabled or externally managed services produce no generated actions |
| Explicit file ownership | Descriptor roots/files | Every embedded template file must have one owner; duplicates/missing coverage fail planning |
| Dynamic service behavior | Immutable built-in `RenderCatalog` | Renderer behavior is compiled into the CLI and selected by service identity |
| Output safety | `validateClusterAppActions` and normalized paths | Actions cannot escape the target application workspace |
| User customization | `custom/` workspace subtree | Promotion preserves customer-owned custom content |
| Secret target | `secretartifacts.Artifact.TargetService` and `Path` | Physical manifest placement is independent of logical owner name |

## Secret artifact planning

`internal/secretartifacts/planner.go` is intentionally independent of secret backends and GitOps rendering. It:

- reads fixed secret blocks and `service_secrets` entries;
- normalizes service/key names and rejects unsafe service names;
- maps logical owners to a physical target and `secret.yaml` path;
- merges multiple owners deterministically, rejecting conflicting canonical keys;
- records all owners and source payloads; and
- validates that a materialized target is declared and enabled when topology is present.

The planner returns a sorted artifact list. The renderer may use artifact presence to decide whether a generated service should include a secret resource, while `internal/secrets` performs encrypted manifest synchronization and maintains ownership hashes.

## Write and promotion boundary

Application output is written in a temporary workspace with atomic file operations. The renderer can encrypt temporary override values before promotion. `internal/gitops/ownership.go` performs promotion using repository-relative paths and two version-2 ownership ledgers:

```text
.opencenter/ownership/
├── clusters/<cluster>.json   # that cluster's generated paths
└── global.json               # exact repository-wide generated files
```

Each ledger records `version`, scope/identity, and per-file SHA-256 and mode records. The cluster ledger is identified by the active cluster and is limited to these explicit recursive scopes: `applications/overlays/<cluster>/`, `infrastructure/clusters/<cluster>/`, and `clusters/<cluster>/`. Flux bootstrap content under `clusters/<cluster>/flux-system/` is excluded; the generated bridge files directly under `clusters/<cluster>/` remain in the cluster boundary. The global ledger has repository identity and is an exact allowlist for `.gitignore`, `README.md`, `applications/overlays/.gitkeep`, and `infrastructure/clusters/.gitkeep`; it is not a recursive repository-wide scope. Every path loaded from either ledger is validated against these policy boundaries before promotion, so a manifest cannot grant authority over a sibling cluster or an unlisted global path.

The full-tree promoter activates all applicable scopes and the global allowlist. Applications-only promotion activates only the active cluster's application overlay, and single-service promotion activates only that service's generated outputs (plus any explicitly planned companion outputs); these scoped paths do not load, write, or prune `global.json`. Sibling cluster records and out-of-scope files are neither inspected nor pruned by a scoped update. This is what permits a full → apps → service → full sequence without losing unrelated ownership records.

`custom/` subdirectories inside generator-owned roots are excluded from the ledgers and live-tree scan. A staged default may seed a missing custom file, but existing custom content is not overwritten or pruned. Secret artifacts whose hash is recorded and still matches `internal/secretartifacts` state are owned by secret synchronization and are excluded from generator conflicts and pruning. Flux bootstrap files are likewise outside generator ownership.

The pre-v1 breaking change is fail-fast: either the repository-root `.opencenter-generated.json` or the current cluster overlay's legacy `.opencenter-generated.json` causes promotion to stop before mutation. There is no implicit migration. Dry-run performs the same ownership preflight and reports classifications without writing. `Prune: false` reports prune candidates and retains them; `AdoptGenerated` permits adoption only for an untracked planned collision and creates a backup. A modified tracked file or an unknown user-authored file still blocks promotion. `Force` is retained for API compatibility and never overrides ownership safety.

This boundary is important for changes: a new renderer must first declare ownership, then produce a plan, then pass action containment and coverage validation. It must not write directly into the final overlay or infer ownership from whatever files happen to exist there.

## Related maps

- [GitOps engine](gitops-engine.md) — generation entry point and workspace lifecycle
- [Secrets management](secrets-management.md) — transactional encrypted manifest sync
- [Config system](config-system.md) — service and secret input model

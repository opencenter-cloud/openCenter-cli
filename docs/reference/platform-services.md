---
last_updated: 2026-10-02
id: platform-services
title: "Platform Services Architecture"
sidebar_label: Platform Services Architecture
description: How openCenter's service configuration, descriptor, and rendering systems fit together, and how enable/disable and validation actually work.
doc_type: explanation
audience: "contributors, platform engineers"
tags: [services, architecture, gitops, descriptors, rendering]
---

# Platform services architecture

> **Purpose:** For contributors and platform engineers, explains how a service goes from a schema entry in cluster configuration to rendered manifests in the GitOps repository — and which of the two "service" abstractions in this codebase are actually load-bearing.

For a directory of individual services with their configuration fields, see [Services index](services/index.md). This page is about the machinery behind that list, not the list itself.

## The two service systems in this codebase

The repository contains **two independent abstractions that both call themselves "services."** Only one of them is wired into the live `opencenter` binary.

### The live system: `config/services` + `descriptors` + the render catalog

This is what actually runs when you execute `opencenter cluster service enable|disable|status|options` or `opencenter cluster generate`:

1. **`internal/config/services`** defines one Go struct per service with specific configuration (e.g. `CertManagerConfig`, `HarborConfig`), each embedding `BaseConfig` (`internal/config/services/base.go`). `BaseConfig` supplies the fields every service shares: `enabled`, `adoption_mode`, `namespace`, `address_pool`, `source` (`repo`/`branch`/`release`), and `image` (`repository`/`tag`). Runtime status is not stored in this declarative base config. Services without extra fields register `DefaultServiceConfig` (just `BaseConfig`) via `internal/config/services/default_services.go`.
2. **`internal/config/registry`** maps a service name string to its registered Go config type, so `internal/config/v2.ServiceMap` can decode `opencenter.services.<name>` / `opencenter.managed_services.<name>` YAML into the correct typed struct (`internal/config/v2/services.go`).
3. **`internal/config/services/dependency_validator.go`** is the only *enforced* dependency graph. It is a small, explicit list (`serviceDependencyGraph`) checked by `opencenter cluster service enable|disable` (via `validateServiceDependencies` in `cmd/cluster_service.go`) for the non-managed `services` map only:
   - `weave-gitops` requires `fluxcd`
   - `keycloak` requires `olm` and `postgres-operator`
   - `headlamp` requires `keycloak`, but only when Headlamp's OIDC fields are actually set (`ValidateHeadlampOIDC`)
4. **`internal/services/descriptors`** loads a fixed set of embedded YAML files (`internal/services/descriptors/data/*.yaml`) that each own a specific, named set of generated GitOps files for one service (`service: <name>`) or one managed service (`managed_service: <name>`), plus a handful of structural aggregate/root descriptors that are not tied to any single service. Each descriptor can gate whole roots or individual files behind an `enabled_when`/`when` condition evaluated against the typed v2 config (`internal/services/descriptors/loader.go`).
5. **`internal/gitops/render_catalog.go`** defines a Go-only `RenderCatalog` of `RenderSpec` entries for every other enabled service that has no dedicated descriptor file — namespace, GitOps source name, base template path, and Flux Kustomization dependency wiring, driven purely by `BaseConfig` fields.
6. At render time (`internal/gitops/auto_descriptor.go`), every enabled, non-external service in `opencenter.services` must resolve to *either* an explicit descriptor *or* a render catalog entry; if neither exists, generation fails with `"enabled service %q has neither an explicit descriptor nor a built-in render catalog entry"`. `fluxcd` and `sources` are treated as structural and handled by the root/aggregate descriptors rather than by an explicit `service:` descriptor.

The provider-polymorphism helper in this package is an exception to the live-path summary: `ServiceProviderValidator` currently has no production callers in this repository. Its defaults and compatibility checks are implemented and tested, but are not invoked by the live enable/disable or generate paths. See [Service-provider resolution](https://github.com/opencenter-cloud/openCenter-cli/blob/main/internal/config/services/PROVIDER_POLYMORPHISM.md).

### The unwired system: `internal/services` + `internal/services/plugins`

`internal/services` defines a generic `ServicePlugin` interface, a `ServiceRegistry` with its own dependency resolution (`ResolveDependencies`, `ExecuteLifecycleHooks`) and lifecycle hooks, and `internal/services/plugins` implements concrete plugins (`CalicoPlugin`, `CertManagerPlugin`, `KeycloakPlugin`, etc.) plus `RegisterBuiltInServices`, which registers a *different* dependency graph than the one in `config/services` (for example it wires `cert-manager -> harbor` and `kube-prometheus-stack -> alert-proxy`, neither of which is enforced anywhere else).

**This subsystem is self-contained and covered by its own tests, but nothing in `cmd/` or the live `cluster generate` pipeline calls `RegisterBuiltInServices` or constructs a `ServiceRegistry` from it.** `internal/gitops/stages.ServiceStage` is the one place that consumes `services.ServiceRegistry`, but `NewServiceStage` itself is only ever called from its own test file — it is not part of the stage list used by `cluster generate`. Treat any dependency graph, validation rule, or status logic that lives only in `internal/services` or `internal/services/plugins` as documentation of intent, not of enforced behavior. The per-service reference pages in this directory describe configuration and behavior from the live system above, not from this package.

## Enable, disable, and status

`cmd/cluster_service.go` implements the CLI surface:

- `opencenter cluster service enable <name> [--managed] [--param k=v] [--secret k=v] [--force] [--render]` — loads the cluster's canonical config, materializes a typed config struct for the service (via `internal/config/registry`), sets `enabled: true`, applies `--param`/`--secret` overrides by matching each key against the struct's `json` tags, runs service-specific validation, and — for non-managed services — runs the `DependencyValidator` before saving. `--force` allows re-enabling (and re-rendering) an already-enabled service; `--render` renders immediately afterward.
- `opencenter cluster service disable <name> [--managed]` — sets `enabled: false` on the existing entry; for non-managed services it re-runs `DependencyValidator` afterward so disabling a required dependency is rejected.
- `opencenter cluster service status` — lists every entry in `opencenter.services` and `opencenter.managed_services` with its enabled/disabled state and `adoption_mode`.
- `opencenter cluster service options <name> [--managed]` — prints a hardcoded, service-specific set of example parameters/secrets from `getServiceOptions`/`getServiceSecrets` in `cmd/cluster_service.go`; it is a curated subset, not a live reflection of the full schema.

### `adoption_mode`

Every service's `BaseConfig` carries an `adoption_mode` enum (`internal/config/services/base.go`): `managed` (default — Flux fully manages the resource), `external` (exists outside Flux management; `IsExternal()` causes the renderer to skip it entirely), `sync` (Flux renders manifests but does not force changes), `deferred` (Flux renders but suspends the Kustomization), and `takeover` (Flux takes over an existing, externally created resource).

### Validation

Beyond the JSON Schema (`schema/opencenter-v2.schema.json`) and the dependency validator, `cmd/cluster_service.go` runs a small set of hand-written, per-service checks in `validateServiceWithConfig`/`validateServiceLegacy` at enable time: `cert-manager` requires `email`; `keycloak` requires `secrets.keycloak.admin_password`; `harbor` requires its S3 access key and secret to be provided together (or both absent); and `loki`/`tempo` require an S3 endpoint plus matched access/secret keys, or matched Swift application-credential ID/secret, depending on the resolved storage backend.

## Configuration shape

Cluster configuration exposes two parallel maps under `opencenter`, both keyed by service name and both accepting the same set of 32 service keys defined in `schema/opencenter-v2.schema.json` (`opencenter.services`, `opencenter.managed-service`, and its alias `opencenter.managed_services` all share an identical schema):

- **`opencenter.services.<name>`** — services rendered into the cluster's own GitOps overlay.
- **`opencenter.managed_services.<name>`** (aliased as `opencenter.managed-service.<name>` in the schema) — services rendered as managed-service overlays; `--managed` on the CLI targets this map instead.

See [Services index](services/index.md) for the full list of service names, their default enabled state, and links to per-service configuration reference.

## Where to look next

- [Services index](services/index.md) — the directory of every service, its default state, and its dedicated reference page.
- `internal/config/services/base.go` — the shared `BaseConfig`/`AdoptionMode` fields every service inherits.
- `internal/config/services/dependency_validator.go` — the only enforced cross-service dependency graph.
- `internal/services/descriptors/data/*.yaml` — the embedded descriptors for services with dedicated, conditional GitOps output.
- `internal/gitops/render_catalog.go` — the built-in render specs for every other service.
- `cmd/cluster_service.go` — the `enable`/`disable`/`status`/`options` command implementations.

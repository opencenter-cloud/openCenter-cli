---
id: services-rendering-options
title: "Service Rendering: Dynamic Plugin Options"
sidebar_label: Rendering Options
description: Evaluates architectural approaches for reducing service-rendering boilerplate in the openCenter CLI and recommends a hybrid descriptor-driven rendering model that preserves typed config.
doc_type: explanation
audience: "developers, platform engineers"
tags: [plugins, services, rendering, architecture, design]
---

# Service Rendering: Dynamic Plugin Options

**Purpose:** Evaluate architectural approaches for reducing openCenter service-rendering boilerplate, document the real constraints in the current codebase, and recommend a path that removes hardcoded rendering behavior without introducing a second service-configuration system.

## Problem Statement

Adding a new platform service to openCenter still touches multiple unrelated code paths:

1. Typed config registration in `internal/config/services/<service>.go`
2. Default values in `internal/config/defaults.go` or `internal/config/v2/defaults.go`
3. Service plugin registration in `internal/services/plugins/registry.go`
4. Optional validator registration in `internal/services/plugins/validators.go`
5. CLI parameter help in `cmd/cluster_service.go` via `getServiceOptions`
6. CLI secret help in `cmd/cluster_service.go` via `getServiceSecrets`
7. CLI enable-time validation in `cmd/cluster_service.go` via `validateService`
8. GitOps overlay templates under `internal/gitops/templates/cluster-apps-base/...`
9. Source and Flux template discovery via filename conventions
10. Aggregate inclusion via hardcoded or convention-based rendering logic

That cost is too high for standard services whose only real work is:

- register dependencies
- render a known overlay directory
- emit a standard GitRepository source
- emit a standard Flux Kustomization pair

The goal should be narrower and more practical than "make everything YAML-driven":

- remove hardcoded rendering topology
- remove switch-based CLI help where possible
- preserve the existing typed config and validation model until there is a single better replacement

## Evidence From The Current Code

### Hardcoded CLI behavior

`cmd/cluster_service.go` still hardcodes service options, secrets, and enable-time validation:

- `getServiceOptions(serviceName string)` is a switch statement
- `getServiceSecrets(serviceName string)` is a switch statement
- `validateService(serviceName string, serviceCfg any, secretsCfg *config.Secrets)` is a switch statement
- `processSecrets(...)` maintains a manual `serviceName -> secret struct field` map

This is real duplication. The same service metadata is spread across config structs, CLI help text, secrets types, and validators.

### Hardcoded render topology

`internal/gitops/copy.go` still discovers what to render by walking the filesystem and inferring intent from paths and filenames:

- `shouldSkipFile` inspects `services/<name>/...`
- source files are recognized by `opencenter-<service>.yaml`
- `RenderSingleService` locates files by directory and filename convention
- `RenderClusterAppsAtomic` walks every embedded file and filters after the fact

That means service topology is encoded in the embedded directory layout, not in a first-class model.

### Hardcoded template discovery

`internal/template/embedded_registry.go` contains a fixed `serviceNames` list inside `inferServices`. This is another manually maintained registry that can drift from the real service catalog.

### Typed config is already a core runtime contract

The service config pipeline is not incidental. It is part of how the CLI works today:

- `internal/config/registry/registry.go` maps service name to Go type
- `internal/config/service_map.go` uses that registry to unmarshal `services:` into typed structs
- `internal/config/schema.go` generates JSON schema from a hardcoded Go-side model
- `internal/config/services/*.go` already hold field names, defaults, descriptions, and enums in struct tags

Any design that replaces config metadata needs to replace this entire pipeline, not just the renderer.

### Existing manifest support is thin

The repository already has `ServicePluginManifest`, `TemplateRef`, `ValidationRule`, and manifest-loading helpers in `internal/services/plugin.go`. That is useful, but it is not a near-complete replacement for the current runtime.

The current manifest shape only models:

- service identity and dependencies
- template references
- generic `schema/defaults/required/validation`

It does not currently model:

- CLI option groups
- CLI secrets
- secret ownership or fallback chains
- source generation
- Flux generation
- custom plugin mode
- overlay render roots

`DefaultServiceRegistry.LoadManifestsFromDirectory` also wires manifests to `BasicServicePlugin`, which is effectively a stub.

## What The Renderer Must Produce

The renderer contract needs to be stated more precisely:

- One `.<cluster>-config.yaml` renders one cluster overlay tree at `applications/overlays/<cluster>/`.
- Reproducing the RelayPoint fixture means there are **five distinct cluster config files**, one per overlay:
  - `.k8s-dev-config.yaml`
  - `.k8s-dr-config.yaml`
  - `.k8s-prod-config.yaml`
  - `.k8s-qa-config.yaml`
  - `.k8s-uat-config.yaml`
- Each of those five config files is rendered independently to produce its matching overlay tree.

That overlay tree is larger than just `services/`. In the current fixture it may contain:

1. Root overlay files such as `applications/overlays/<cluster>/kustomization.yaml`
2. Optional cluster bootstrap files such as `applications/overlays/<cluster>/flux-system/`
3. Platform service overlays under `applications/overlays/<cluster>/services/`
4. Managed service overlays under `applications/overlays/<cluster>/managed-services/`
5. Optional customer-managed overlays under `applications/overlays/<cluster>/customer-managed/`

Each branch may itself contain:

- `sources/`
- `fluxcd/`
- service or unit-specific overlay directories

Some render units also break the "one source + one Flux pair" assumption. For example:

- Keycloak emits multiple GitRepository sources and multiple Flux Kustomizations
- `customer-managed/` emits cluster-level sources and multiple customer-owned Flux Kustomizations
- Some clusters include `flux-system/` while others do not

Any recommended design has to model the full overlay tree, not just standard services.

## Design Constraints

A viable design for this codebase should satisfy all of these:

1. Preserve `cluster edit`, typed unmarshalling, and schema generation during the migration.
2. Remove render-time filename conventions as the source of truth.
3. Support standard services with little or no Go code beyond configuration where needed.
4. Keep complex validation and lifecycle hooks in Go.
5. Support single-service rendering without scanning unrelated files.
6. Avoid introducing a second service-config DSL unless it clearly replaces the first one everywhere.
7. Support overlay classes beyond `services/`, including `managed-services/`, optional `customer-managed/`, and cluster-level files.
8. Support multiple rendered sources and multiple Flux units for a single logical service or overlay unit.
9. Provide a typed cluster-level config surface for non-service overlay units such as `customer-managed/` and optional bootstrap files.

## Approach A: Inline Kubernetes Resources In Cluster Config

Embed rendered resources or resource fragments directly in the cluster config YAML.

### Pros

- Operators can see the final resource shape in one file.
- No separate template inventory is required.
- Any Kubernetes resource can theoretically be represented.

### Cons

- It turns the cluster config into a Kubernetes manifest store.
- It pushes implementation details back onto operators.
- It makes SOPS and secret handling awkward.
- It does not work well for large or multi-directory services like Keycloak.
- It makes `cluster edit` and merge workflows noisier.
- It does not address the existing typed config pipeline; it competes with it.

### Assessment

This is a poor fit for openCenter. The CLI's value is abstraction over raw Kubernetes resources, not embedding them into cluster config.

## Approach B: Full Service Manifests For Config, CLI, Validation, And Rendering

Use YAML manifests as the new source of truth for:

- service options
- service secrets
- validation rules
- defaults
- source and Flux generation
- overlay discovery
- dependency metadata

This is the broad direction proposed by the earlier version of this document.

### Pros

- In theory, a standard service becomes data-only.
- Rendering topology can become declarative.
- CLI help can be derived from manifests.
- Hardcoded switch statements can go away.

### Cons

- It introduces a second service metadata system immediately.
- It duplicates information already stored in Go config structs and secret types.
- It does not replace the typed config registry, schema generation, or config unmarshalling by itself.
- The existing manifest implementation is much smaller than the required design.
- Secret fallback logic already lives in `internal/config/config.go`; moving it into YAML is a separate migration.
- Complex validation still needs Go, so the model is not truly manifest-only.

### Assessment

This approach would make sense in a greenfield design. In this repository, today, it asks the team to build a new config model before finishing the old one. That is too much architecture churn for the first step.

## Approach C: Keep The Current Model And Only Remove The CLI Switches

Keep rendering exactly as it is and only derive `cluster service options` and secrets from existing structs.

### Pros

- Lowest migration risk.
- Reduces some obvious duplication in `cmd/cluster_service.go`.
- Reuses metadata already present in config and secret structs.

### Cons

- It leaves the renderer convention-driven.
- `RenderSingleService`, `shouldSkipFile`, and `inferServices` stay brittle.
- Source and Flux files are still coupled to directory layout and filename patterns.
- New services still require touching render topology in multiple places.

### Assessment

This is an improvement, but not enough. It treats a symptom while leaving the main rendering problem intact.

## Approach D: Overlay Unit Descriptors With Typed Config (Recommended)

Split responsibilities cleanly:

- **Typed Go config remains the source of truth** for service configuration shape, defaults, and complex validation.
- **Overlay unit descriptors become the source of truth** for rendered topology inside `applications/overlays/<cluster>/`.

This is the narrowest change that solves the real problems first.

### Core Idea

Each renderable overlay unit gets a descriptor that answers only rendering questions.

A unit can represent:

- a platform service in `services/`
- a managed service in `managed-services/`
- a cluster-scoped customer-owned layer in `customer-managed/`
- a cluster-scoped bootstrap unit such as root overlay files or optional `flux-system/`

Each descriptor answers:

- What logical unit is this?
- Which overlay layer does it belong to?
- Is it gated by `services.<name>.enabled`, `managed-service.<name>.enabled`, or by cluster-level config?
- Does it use a custom Go plugin?
- Which GitRepository sources does it emit?
- Which Flux Kustomizations does it emit?
- Where is its overlay root?
- Which files belong to the overlay?

It does **not** redefine:

- the service config schema
- CLI parameter names
- secret structs
- complex validation rules
- secret fallback behavior

### Example Descriptor

```yaml
# internal/services/descriptors/keycloak.yaml
name: keycloak
type: security
layer: services
owner:
  type: service
  name: keycloak
dependencies:
  - cert-manager

plugin:
  mode: custom

render:
  sources:
    - name: opencenter-keycloak
      url: ssh://git@github.com/rackerlabs/openCenter-gitops-base.git
      ref_type: branch
      ref_value: main
    - name: opencenter-keycloak-config
      source: cluster-repo

  flux_units:
    - name: keycloak-postgres
      source_ref: opencenter-keycloak-config
      path: services/keycloak/00-postgres
      namespace: keycloak
      depends_on:
        - sources
        - postgres-operator-base
        - postgres-operator-override
    - name: keycloak-operator
      source_ref: opencenter-keycloak-config
      path: services/keycloak/10-operator
      namespace: keycloak
      depends_on:
        - sources
        - keycloak-postgres
    - name: keycloak-cr
      source_ref: opencenter-keycloak-config
      path: services/keycloak/20-keycloak
      namespace: keycloak
      sops_decryption: true
      depends_on:
        - sources
        - keycloak-postgres
        - keycloak-operator
        - envoy-gateway-api-base
        - envoy-gateway-api-override
        - gateway

  overlay:
    root: services/keycloak
    files:
      - 00-postgres/kustomization.yaml
      - 10-operator/kustomization.yaml
      - 20-keycloak/kustomization.yaml
      - 20-keycloak/keycloak-cr-patch.yaml.tpl
```

This descriptor is intentionally about rendering only. A cluster-scoped descriptor would use the same model with `layer: customer-managed` or `layer: cluster`.

### How It Works

1. Each `.<cluster>-config.yaml` still unmarshals through `internal/config/service_map.go` using registered Go types.
2. The renderer takes one cluster config and produces one overlay tree under `applications/overlays/<cluster>/`.
3. The renderer loads descriptors for enabled service-scoped units and any applicable cluster-scoped units.
4. Shared templates generate GitRepository and Flux resources from descriptor lists (`sources`, `flux_units`) plus cluster config.
5. Aggregate `kustomization.yaml` files are generated for the root overlay and each active branch (`services/fluxcd`, `managed-services/fluxcd`, optional `customer-managed/fluxcd`).
6. CLI help still derives from config structs and secret structs, not from overlay descriptors.
7. Complex validation remains in Go validators.

### Required Cluster-Level Config Surface

Service maps already cover `services:` and `managed-service:`. To make the design fully capable of reproducing the RelayPoint-style overlays from one cluster config, the cluster config also needs a typed section for cluster-scoped overlay units.

One reasonable shape is:

```yaml
opencenter:
  gitops:
    overlay_units:
      flux_system:
        enabled: true
      customer_managed:
        enabled: true
        repository_name: customer-repository-rpl-apps-flux-k8s
        repository_url: ssh://relaypointlogistics@git.relaypointlogistics.com/rpl/apps-flux-k8s.git
        branch: main
        secret_name: customer-repository-rpl-apps-flux-k8s
        secret_ref: customer_managed.rpl_apps_flux_k8s
        kustomizations:
          - name: policies
            path: /policies/qa
          - name: infrastructure
            path: /infrastructure/qa
          - name: apps
            path: /apps/qa
```

That keeps non-service overlay inputs out of the service schema while still making them first-class, typed, and renderable from `.<cluster>-config.yaml`.

Cluster-scoped units that emit Secret manifests also need a matching typed secret surface, for example:

```yaml
secrets:
  customer_managed:
    rpl_apps_flux_k8s:
      identity: ""
      identity_pub: ""
      known_hosts: ""
```

The descriptor then decides whether that secret is rendered and which manifest name it uses. The config remains the source of truth for the secret material or secret backend reference.

### Pros

- Removes filename-convention rendering logic.
- Removes hardcoded service-name lists from render and template discovery.
- Preserves the current typed config pipeline and schema/editor compatibility.
- Keeps complex service behavior in Go where it already works.
- Supports standard services, managed services, customer-managed layers, and cluster-scoped overlay units with one model.
- Allows generic generation of sources, Flux units, and aggregate kustomizations.
- Supports incremental migration service by service and layer by layer.

### Cons

- It is not fully YAML-driven.
- Services with new config fields still need a Go config struct until config metadata is unified.
- CLI secrets still need a better metadata source if the team wants to remove all manual mapping.
- The repository will temporarily have typed config plus overlay descriptors.
- Customer-managed and bootstrap units are not naturally "services", so naming and ownership need to be explicit in the model.

### Assessment

This is the best fit for the current repository. It removes the most brittle rendering behavior first while still covering the actual overlay topology in the RelayPoint fixture.

## Comparison Matrix

| Criterion | A: Inline Resources | B: Full Manifests | C: Only Remove CLI Switches | D: Overlay Unit Descriptors + Typed Config |
|---|---|---|---|---|
| Removes filename-convention rendering | No | Yes | No | Yes |
| Preserves current typed config pipeline | No | Partially | Yes | Yes |
| Introduces second source of truth for config | Yes | Yes | No | No |
| Handles complex validation cleanly | Poorly | Needs Go escape hatch | Yes | Yes |
| Supports generic source and Flux generation | Possible | Yes | Limited | Yes |
| Migration risk | High | High | Low | Medium |
| New render-only service can be data-only | No | Yes | No | Yes |
| New service with new config fields requires Go changes | Yes | No | Yes | Yes |
| Fit for current codebase | Poor | Moderate at best | Moderate | Strong |

## Recommendation

**Recommend Approach D: overlay unit descriptors with typed config.**

The reasoning is straightforward:

1. The biggest current problem is hardcoded rendering topology, not typed config by itself.
2. The typed config registry is already part of config loading, schema generation, and editor UX.
3. The existing manifest support is too small to justify replacing config, CLI metadata, and rendering in one step.
4. Complex services already need Go validation and lifecycle hooks, so a pure data model would still need escape hatches.
5. An overlay descriptor catalog lets the team make the renderer deterministic and discoverable immediately.
6. The same descriptor model can cover `services/`, `managed-services/`, optional `customer-managed/`, and cluster-level overlay units.

## Recommended Implementation Path

### Phase 1: Introduce An Overlay Unit Descriptor Catalog

Add a dedicated model for overlay rendering metadata.

Suggested fields:

- unit name
- unit type
- layer (`services`, `managed-services`, `customer-managed`, `cluster`)
- owner (`service` or `cluster`)
- dependency list
- plugin mode (`default` or `custom`)
- source list
- Flux unit list
- overlay root
- overlay file list
- aggregate target metadata
- conditions or enablement gates

The descriptor can be stored as YAML in an embedded directory or as Go structs first. YAML is acceptable here because the scope is narrow and render-specific.

### Phase 2: Replace Convention-Based Rendering

Refactor these paths to use descriptors instead of filesystem guessing:

- `internal/gitops/copy.go` `shouldSkipFile`
- `internal/gitops/copy.go` `RenderSingleService`
- `internal/gitops/copy.go` `RenderClusterAppsAtomic`
- `internal/template/embedded_registry.go` `inferServices`

After this step:

- a unit is rendered because its descriptor says so
- single-service rendering uses descriptor membership
- root and branch aggregate kustomizations iterate active descriptors
- `services/`, `managed-services/`, and optional `customer-managed/` are all rendered from the same catalog

### Phase 3: Make Source And Flux Generation Generic

Replace per-service and per-layer source and Flux discovery with shared templates driven by descriptor metadata.

That should remove a large amount of repetitive file inventory without changing service config semantics.

The shared model must support:

- multiple sources for a single unit
- multiple Flux units for a single unit
- cluster-level values such as overlay path, intervals, and repo URLs
- branch-specific aggregate `kustomization.yaml` files

### Phase 4: Derive CLI Help From Existing Types

Remove `getServiceOptions` and `getServiceSecrets` switch statements by deriving help text from:

- registered service config structs
- secret structs in `internal/config/types_secrets.go`
- existing struct tags or generated schema metadata

This is a better improvement than duplicating CLI metadata into render descriptors.

### Phase 5: Keep Complex Validation In Go

Retain Go validators for services like:

- Keycloak
- Loki
- Cert-manager

If later the team wants declarative validation for simple cases, add it as a supplement to typed config, not as a second canonical schema.

### Phase 6: Validate Against Fixture Repositories

Add fixture-based verification for parity with the real rendered shape.

At minimum:

- render one cluster config into one overlay tree
- run the renderer across the five distinct RelayPoint config files:
  - `.k8s-dev-config.yaml`
  - `.k8s-dr-config.yaml`
  - `.k8s-prod-config.yaml`
  - `.k8s-qa-config.yaml`
  - `.k8s-uat-config.yaml`
- diff each rendered result against `testdata/relaypoint-logistics-shared/applications/overlays/<cluster>/`
- document any intentionally canonicalized differences

### Phase 7: Revisit Config Metadata Unification Later

Only after render descriptors are stable should the team consider a larger consolidation of:

- schema generation
- CLI options/secrets help
- defaults
- validation metadata

At that point, the team can decide whether to:

- generate all metadata from Go types, or
- move fully to manifests

That should be a separate design decision, not bundled into the rendering refactor.

## Non-Goals For The First Migration

The first migration should **not** try to do these things:

- delete typed service config structs
- delete service config registration
- move all validation into YAML
- move secret fallback chains out of `internal/config/config.go`
- embed full Kubernetes resources into cluster config

Those are larger design changes with broader blast radius than the rendering problem requires.

## Fixture Parity Status

If the bar is "can this design, once implemented, take five distinct `.<cluster>-config.yaml` files and reproduce the RelayPoint overlay trees?", the answer is **not yet**.

The renderer contract must still be:

- one `.<cluster>-config.yaml` renders one `applications/overlays/<cluster>/` tree
- the RelayPoint-style repository therefore requires five distinct config files:
  - `.k8s-dev-config.yaml`
  - `.k8s-dr-config.yaml`
  - `.k8s-prod-config.yaml`
  - `.k8s-qa-config.yaml`
  - `.k8s-uat-config.yaml`

The recommended architecture is still correct, but it is only a necessary base. It does not yet guarantee exact fixture parity without additional requirements.

### Remaining Shortcomings

1. **Descriptors need conditional file membership.**
   The fixture contains services whose rendered file set varies by cluster. A static `overlay.files` list is not enough for cases like `patch-subscription.yaml`, `rbac-manager-users.yaml`, `alertmanager-routes.yaml`, or the cluster-specific cert-manager files.

2. **Cluster-scoped rendered assets still need explicit ownership.**
   The model needs first-class handling for non-service files such as `.sops.yaml`, cluster-level customer-managed source Secrets, and any other root-level rendered artifacts.

3. **`flux-system/` needs a defined lifecycle boundary.**
   All five root `kustomization.yaml` files reference `./flux-system`, but checked-in `flux-system/` content exists only in `k8s-dr`, `k8s-prod`, and `k8s-qa`. The design must explicitly say whether `flux-system/*` is template-rendered, bootstrap-generated, or excluded from parity diffs.

4. **The parity fixtures need complete per-cluster config inputs.**
   The repository currently has no checked-in `.k8s-dev-config.yaml`, `.k8s-dr-config.yaml`, `.k8s-prod-config.yaml`, `.k8s-qa-config.yaml`, or `.k8s-uat-config.yaml`. Some rendered overlay content also appears to have been added manually and is not obviously represented in config today. Exact parity requires authoring complete config fixtures first.

5. **Canonicalization rules need to be explicit.**
   Some differences in the fixture are legacy naming drift rather than required behavior. The implementation needs a documented policy for things like source filenames, cert-manager filenames, and disabled services that currently leave stray files behind.

### What Needs To Be Added

To make the "five configs produce five overlays" claim true, this design needs a few concrete additions:

1. Add conditional render rules to descriptor `overlay.files` entries and any generated source or Flux unit lists.
2. Add typed cluster-scoped config and secret surfaces for non-service units, including customer-managed repo credentials and `.sops.yaml` generation inputs.
3. Define `flux-system/` as a separate lifecycle concern and make the parity tests compare template-rendered output separately from bootstrap-owned files.
4. Create and maintain the five per-cluster config fixtures as first-class test inputs.
5. Document allowed canonicalization diffs so parity tests can distinguish intentional cleanup from regressions.

The detailed problem statement and implementation plan for that work lives in [Service Rendering: Fixture Parity Plan](services-rendering-parity-plan.md).

## Conclusion

The recommendation still stands, but the scope needs to be explicit: this is not just "service rendering." It is "cluster overlay rendering."

The architectural split that holds up is:

- typed config for cluster and service parameters
- overlay unit descriptors for rendered topology

That split is the right foundation for reproducing the RelayPoint-style overlays, but it still needs the parity requirements above before it can reliably turn five distinct `.<cluster>-config.yaml` inputs into the five expected overlay trees.

## Evidence

Code paths referenced in this document:

- `cmd/cluster_service.go`
- `internal/config/config.go`
- `internal/config/registry/registry.go`
- `internal/config/schema.go`
- `internal/config/service_map.go`
- `internal/config/services/`
- `internal/config/types_secrets.go`
- `internal/gitops/copy.go`
- `internal/services/plugin.go`
- `internal/services/plugins/registry.go`
- `internal/services/plugins/validators.go`
- `internal/template/embedded_registry.go`

Rendered output references reviewed from `testdata/relaypoint-logistics-shared/applications/overlays/`:

- `k8s-dev/`, `k8s-dr/`, `k8s-prod/`, `k8s-qa/`, `k8s-uat/` — five separate cluster overlay trees
- required config inputs for fixture parity: `.k8s-dev-config.yaml`, `.k8s-dr-config.yaml`, `.k8s-prod-config.yaml`, `.k8s-qa-config.yaml`, `.k8s-uat-config.yaml`
- `k8s-dev/kustomization.yaml`, `k8s-dr/kustomization.yaml`, `k8s-prod/kustomization.yaml`, `k8s-qa/kustomization.yaml`, `k8s-uat/kustomization.yaml` — root aggregate overlay files, all of which reference `./flux-system`
- checked-in `flux-system/` content exists only under `k8s-dr/`, `k8s-prod/`, and `k8s-qa/`
- `k8s-dev/services/fluxcd/keycloak.yaml` — example of one service emitting multiple Flux Kustomizations
- `k8s-dev/services/sources/opencenter-keycloak.yaml` and `k8s-dev/services/sources/opencenter-keycloak-config.yaml` — example of one service emitting multiple GitRepository sources
- `k8s-dev/managed-services/fluxcd/alert-proxy.yaml` — example of a managed-service overlay unit
- `k8s-uat/customer-managed/sources/customer-repository-ffcb-apps-flux-k8s-secret.yaml` — example of a cluster-scoped rendered Secret that must be driven by per-cluster config or secret references

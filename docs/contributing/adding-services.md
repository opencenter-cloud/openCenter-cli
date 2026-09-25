---
last_updated: 2026-09-25
id: adding-services
title: "Adding New Platform Services"
sidebar_label: Adding New Platform Services
description: How to add new platform services to openCenter-cli using the immutable render catalog and explicit descriptors.
doc_type: how-to
audience: "developers, platform engineers"
tags: [services, rendering, gitops, render-catalog, descriptors]
---
# Adding New Platform Services

**Purpose:** For developers, shows how to add new platform services to openCenter-cli.

## Prerequisites

* Development environment set up (see [Development Setup](development-setup.md))
* Service’s Helm chart already added to `openCenter-gitops-base` under `applications/base/services/<service>/`

## Quick Path: Standard Services (Catalog-Backed Auto-Descriptor)

Most services follow the standard two-stage FluxCD pattern. For these, adding a
service uses the typed config/defaults and an immutable catalog entry that can
reuse the standard planner. It does not require a service-specific renderer,
string-based renderer registration, or renderer fields in `BaseConfig`.

### Step 1: Register the Service Config Type

If the service has no custom fields beyond `BaseConfig`, it’s already registered via `DefaultServiceConfig` in `internal/config/services/default_services.go`. Just add the name:

```go
defaults := []string{
    // ... existing services
    "my-service",
}
```

If the service needs custom fields (storage type, credentials, etc.), create a typed config:

```go
// internal/config/services/my_service.go
package services

type MyServiceConfig struct {
    BaseConfig `yaml:",inline"`
    BucketName string `yaml:"bucket_name,omitempty" json:"bucket_name,omitempty"`
}

func init() {
    registry.RegisterServiceConfig("my-service", MyServiceConfig{})
}
```

### Step 2: Set Defaults

Add the service to `internal/config/v2/defaults.go` in `defaultServiceMap()`:

```go
"my-service": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{
    Enabled:   true,
    Namespace: "my-service",
}},
```

Once the typed config, defaults, and catalog coverage are in place, the
standard planner generates:

* `services/sources/opencenter-my-service.yaml` (GitRepository)
* `services/fluxcd/my-service.yaml` (two-stage Kustomization)
* `services/my-service/kustomization.yaml` (overlay with secretGenerator)
* `services/my-service/helm-values/override-values.yaml` (placeholder)
* `services/my-service/custom/` (user-owned; missing defaults may be seeded once, existing files are never overwritten)
* Entries in aggregate `kustomization.yaml` files
* A repository-relative SHA-256/mode ownership entry per generated file in the version-2 `.opencenter/ownership/clusters/<cluster>.json`

### Step 3: Regenerate Schema

```bash
mise run schema-v2
```

The regeneration test file is created on demand by the mise task, so
`go test -run TestRegenSchema` on its own will not find anything to run.

### Render catalog and contributor contract

`BaseConfig` is part of the typed desired-state model. Keep it limited to
public service inputs such as enablement, namespace, image settings, and other
fields that operators can safely configure. Do not add renderer selectors,
rendering topology, generated-file lists, or raw Helm override content to a
service config type.

Rendering is resolved through an immutable internal render catalog. Each
catalog entry identifies the service and holds direct Go function references for
service-specific rendering behavior. `newBuiltInRenderCatalog()` is private and
is constructed at the planner/rendering call sites that need catalog data (for
example, `planAutoServiceActions`, `buildAutoServiceContextWithArtifacts`, and
descriptor rendering). Each caller receives its own catalog value; there is no
mutable global renderer registry. Contributors must not add string-based renderer
names, `init()` registration calls, or mutable lookup registries.

Use this contract for a new service:

1. Add or extend the typed service config with only declarative fields.
2. Define the service's explicit descriptor when it has non-standard files,
   conditions, dependencies, or ownership boundaries.
3. Implement any service-specific rendering functions with the repository's
   renderer signatures.
4. Add those functions directly to the appropriate `RenderSpec` fields.
5. Let the descriptor and render plan report the generator-owned output files.
   Put hand-authored manifests and values in the service overlay's user-owned
   `custom/` directory; missing staged defaults may be seeded, but existing
   custom files are never overwritten or pruned.

### Ownership boundaries for new services

Descriptor outputs must stay inside the active service or managed-service scope. Promotion records repository-relative SHA-256 and mode records in the current version-2 cluster ledger at `.opencenter/ownership/clusters/<cluster>.json`; only the small code-defined exact-file allowlist belongs in `.opencenter/ownership/global.json`. The active cluster boundary is `applications/overlays/<cluster>/`, `infrastructure/clusters/<cluster>/`, and `clusters/<cluster>/`; `clusters/<cluster>/flux-system/` is Flux bootstrap-owned, while the generated bridge files beside it remain in scope. Do not rely on a sibling cluster's files, a recursive repository-wide scope, or an existing file to establish authority. `custom/` content is user-owned after any missing default is seeded, and secret-sync-owned artifacts must remain excluded from generator ownership.

The full, applications-only, and single-service promoters share these boundaries. Full-tree promotion may also update the exact global allowlist; scoped runs do not load or rewrite `global.json`, and they prune only the active scope, so a service change must not remove another service or cluster's records. The old root or current overlay `.opencenter-generated.json` manifest is a pre-v1 legacy format and causes fail-fast refusal before mutation; it is never migrated. Use dry-run to inspect the same ownership preflight without writing; `--adopt-generated` is an explicit opt-in only for an untracked planned collision and creates a backup during apply, while modified tracked files and unknown files still fail. `--force` does not override these ownership checks.

A catalog entry uses the actual `RenderSpec` field names. Include only the
fields needed by the service; unused renderer fields remain nil or empty:

```go
RenderSpec{
    ServiceName:            "my-service",
    DefaultNamespace:       "my-service",
    SourceName:             "opencenter-my-service",
    SourceGroup:            "my-service",
    EmitSource:             true,
    BasePath:               "applications/base/services/my-service",
    HasOverrideValues:     true,
    OverrideValuesRenderer: myServiceOverrideValuesRenderer,
    OverlayFilesRenderer:   myServiceOverlayFilesRenderer,
}
```

For descriptor-owned behavior that must create files from typed configuration,
associate a direct function reference with the descriptor name in the catalog's
dynamic planner list:

```go
type dynamicActionPlanner func(v2.Config) ([]clusterAppAction, error)

type catalogDynamicPlanner struct {
    descriptorName string
    planner        dynamicActionPlanner
}

// Part of the RenderCatalog value returned by newBuiltInRenderCatalog.
dynamicPlanners: []catalogDynamicPlanner{
    {descriptorName: "service-my-service", planner: planMyServiceDynamicActions},
}
```

The planner returns generator-owned render actions and is validated against its
explicit descriptor. The association is by descriptor name, but the planner
itself is a direct Go function reference; it is not a string-based renderer
lookup. The public YAML contains typed service fields only. If an operator needs
a value that is not represented by a typed field, document and add that field as
part of the service contract, or direct the operator to `custom/`; never expose
the catalog, descriptor topology, renderer selection, or raw override values as
cluster configuration.

### Examples

**Standard service:**

```go
"my-service": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{
    Enabled:   true,
    Namespace: "my-service",
}},
```

**Service with typed settings:**

```go
type MyServiceConfig struct {
    BaseConfig `yaml:",inline"`
    BucketName string `yaml:"bucket_name,omitempty" json:"bucket_name,omitempty"`
}
```

## Complex Services: Explicit Descriptors

Services that need custom rendering logic (multi-component, conditional files,
templated override-values, or service-specific planning) use explicit descriptors
and, when needed, direct function references in the immutable render catalog.

### When to Use Explicit Descriptors

* Multi-component services (keycloak: 4 sub-stages)
* Services with conditional file rendering (keycloak backup cronjob, region-specific patches)
* Services with templated override-values (loki, tempo, openstack-ccm)
* Services with custom renderers (cert-manager multi-credential DNS)

### Step 1: Create Descriptor

Create `internal/services/descriptors/data/service-<name>.yaml`:

```yaml
name: service-my-complex-service
layer: services
service: my-complex-service
aggregate_targets:
  - services-fluxcd-aggregate
  - services-sources-aggregate
roots:
  - path: services/my-complex-service
files:
  - template: services/sources/opencenter-my-complex-service.yaml.tpl
  - template: services/fluxcd/my-complex-service.yaml.tpl
  - template: services/my-complex-service/conditional-file.yaml.tpl
    when:
      field: opencenter.services.my-complex-service.some_field
      operator: true
```

### Step 2: Create Templates

Create the `.tpl` files referenced by the descriptor under `internal/gitops/templates/cluster-apps-base/`.

### Step 3: Add to Aggregate Lists

Add the service to the hardcoded lists in:

* `services/sources/kustomization.yaml.tpl`
* `services/fluxcd/kustomization.yaml.tpl`

### Step 4: (Optional) Service-Specific Renderer

For services like cert-manager that generate dynamic files based on typed config,
implement the renderer function and reference it directly from the service's
immutable render-catalog entry. The function must report the generator-owned
files it plans or emits. Do not register it by string name or mutate a global
renderer registry.

## Verification

After adding a service:

```bash
# Build
go build ./...

# Run tests
go test ./internal/gitops/ ./internal/config/... ./internal/services/...

# Regenerate schema
mise run schema-v2

# Test rendering (dry-run)
./bin/opencenter cluster generate <org>/<cluster> --dry-run
```

## Evidence

* Immutable render catalog: `internal/gitops/render_catalog.go`
* Explicit descriptors: `internal/services/descriptors/data/`
* Descriptor renderer: `internal/gitops/descriptor_renderer.go`
* Rendering contract: [Rendering Contract](rendering-contract.md)

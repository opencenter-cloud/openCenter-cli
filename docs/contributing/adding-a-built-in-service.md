---
last_updated: 2026-09-25
id: adding-a-built-in-service
title: "Adding a Built-in Service"
sidebar_label: Adding a Built-in Service
description: End-to-end contributor workflow for adding a built-in platform service.
doc_type: how-to
audience: "developers, maintainers"
tags: [services, configuration, rendering, secrets, gitops]
---
# Adding a built-in service

**Purpose:** For developers, maintainers, shows how to End-to-end contributor workflow for adding a built-in platform service.

This is the short path from a service name to a typed, validated, rendered, and
documented built-in service. For broader descriptor examples, see [Adding New
Platform Services](adding-services.md).

## 1. Add the configuration contract

First decide whether the service has public fields beyond `BaseConfig` in
`internal/config/services/base.go`:

| Need | Change |
|---|---|
| Only `enabled`, `namespace`, and other common fields | Add the service name to the `defaults` slice in `internal/config/services/default_services.go`. This registers `DefaultServiceConfig`. |
| Service-specific YAML fields | Add `internal/config/services/<service>.go`, embed `BaseConfig`, and register the type with `registry.RegisterServiceConfig` from `internal/config/registry`. |

Registration and default materialization are separate. Add the public default
shape to `NewDefaultServiceConfig` and, when it belongs in generated cluster
configuration, to `defaultServiceMap` in `internal/config/v2/defaults.go`.
An opt-in service may intentionally have no `defaultServiceMap` entry, as with
the managed `alert-proxy` documented in `docs/reference/services/alert-proxy.md`.
Keep renderer topology, generated paths, and raw Helm values out of the typed
configuration.

If the service varies by infrastructure provider, add its capability/default
mapping in `internal/config/services/provider_registry.go` and validation in
`internal/config/services/provider_validator.go`. Add service dependencies to
`internal/config/services/dependency_validator.go`. Add cross-field deployment
readiness rules to `internal/config/v2/readiness.go`; the generation gate is
assembled by `internal/config/v2/generation_validation.go`.

For credentials, declare conditional requirements in
`internal/config/services/secrets_validator.go` and cover the enabled,
provider/backend, and missing/partial-secret cases. Readiness checks for
service secrets and object-storage fields live alongside `ValidateReadiness`.
Disabled services must not require their secrets.

## 2. Regenerate the schema

The Go types are authoritative. Regenerate the checked-in editor schema after
changing a service type or its public fields:

```bash
mise run schema-v2
git diff -- schema/opencenter-v2.schema.json
```

The task writes `schema/opencenter-v2.schema.json` through the temporary
`TestRegenSchema` test in `internal/config/v2schema`. Do not hand-edit the JSON.

## 3. Choose rendering ownership

Use the immutable built-in `RenderCatalog` in
`internal/gitops/render_catalog.go` when the service follows the standard
catalog planner. Add a `RenderSpec` entry with direct Go function references
for non-static override or overlay behavior. Do not add renderer selectors to
configuration, string renderer names, or a mutable registry.

Use an explicit descriptor when the service has non-standard files,
conditions, dependencies, or ownership. Create
`internal/services/descriptors/data/service-<name>.yaml`, then add the
referenced `.tpl` files below
`internal/gitops/templates/cluster-apps-base/services/`. Update the relevant
aggregate templates, normally
`services/sources/kustomization.yaml.tpl` and
`services/fluxcd/kustomization.yaml.tpl`.

For Helm values, use the catalog's `OverrideValues`,
`OverrideValuesRenderer`, or `OverlayFilesRenderer` fields as appropriate.
Verify the rendered `services/<name>/helm-values/override-values.yaml`, source
and Flux Kustomization files, aggregate files, and generated resource files.
Put contributor/customer-authored files under the service overlay's
`custom/` directory; existing custom files are not generator-owned.

## 4. Wire secrets and ownership

Trace a secret from logical input to its physical artifact before adding
templates:

1. `internal/secretartifacts/planner.go` maps logical owners to a target service
   and `secret.yaml` path, merges owners deterministically, and validates the
   target.
2. `internal/secrets` materializes encrypted manifests and records ownership
   hashes/state. It is separate from SOPS overlay encryption.
3. `internal/sops/overlay_files.go` supplies the shared ordered list of files
   encrypted by `internal/sops/manager.go` and `internal/sops/git.go`. Add the
   service's override-values path there when rendered Helm values can contain
   credentials.
4. GitOps promotion records generated files in
   `.opencenter/ownership/clusters/<cluster>.json`. Secret-sync-owned artifacts
   and existing `custom/` content must remain outside generator ownership;
   review `internal/gitops/ownership.go` and
   `docs/CODEMAPS/rendering-ownership-and-secret-artifacts.md` when the service
   crosses that boundary.

## 5. Add object-storage support only when needed

For a service that provisions Swift or S3-compatible storage, keep the
configuration, backend defaults, required secrets, and rendered values in
agreement. The one-service lifecycle is implemented in
`internal/cluster/storage/openstack/service.go` and exposed by
`cmd/cluster_service_storage.go`:

```bash
opencenter cluster service storage plan <service> --cluster <cluster> --backend swift
opencenter cluster service storage apply <service> --cluster <cluster> --backend swift
opencenter cluster service storage apply <service> --cluster <cluster> --backend s3 --rotate-credentials
```

If the new service needs this CLI path, add its supported backend mapping,
typed config/secret wiring, and persistence cases in
`internal/cluster/storage/openstack/service.go`, then add focused tests in
`internal/cluster/storage/openstack/service_test.go` and command coverage in
`cmd/`. The lifecycle must preserve the existing boundaries: plan performs
preflight; apply ensures the container/bucket, reuses or rotates credentials,
writes a backup, persists typed configuration atomically, revokes replaced
credentials, and retains recovery state for failures after remote creation.
`--dry-run` must not perform remote mutations. Storage provisioning does not
itself run SOPS or secret-manifest synchronization; those remain separate
flows.

If the service does not need the supported storage command, document its
object-storage fields and use the normal config, secret-artifact, and SOPS
paths instead. See [OpenStack provider and storage operations](../CODEMAPS/openstack-provider-storage-operations.md).

## 6. Test and document the service

Add tests beside the changed package. Cover the typed config/defaults,
provider/dependency/readiness/secret rules, catalog or descriptor ownership,
rendered contract, secret artifacts, and storage lifecycle when applicable.
Useful existing surfaces include `internal/gitops/render_catalog_test.go`,
`internal/services/descriptors/loader_test.go`,
`internal/config/v2schema/`, and the service tests under
`internal/config/services/`.

Run the relevant checks:

```bash
go test ./internal/config/services/... ./internal/config/v2/... ./internal/config/v2schema/... -count=1
go test ./internal/gitops/... ./internal/services/descriptors/... ./internal/secretartifacts/... ./internal/secrets/... ./internal/sops/... -count=1
go test ./internal/cluster/storage/openstack ./cmd/... -count=1
mise run schema-v2
go test ./cmd -run TestDocsDoNotUseRemovedGACommands -count=1
mise run test-docs
git diff --check
```

Add or update `docs/reference/services/<service>.md` using the existing service
frontmatter and sections for configuration, validation, secrets, dependencies,
rendering, and CLI commands. Add the page to the appropriate category in
`docs/reference/services/index.md` and the complete service table in
`docs/README.md`. If the built-in command tree changed, regenerate its pages
with `mise run docs-gen` and check `docs/reference/opencenter/`.

## Checklist

- [ ] Chose `DefaultServiceConfig` registration or a typed config in `internal/config/services/`.
- [ ] Added or intentionally omitted generated defaults in `internal/config/v2/defaults.go`.
- [ ] Added provider, dependency, readiness, and conditional secret validation where required.
- [ ] Regenerated `schema/opencenter-v2.schema.json` with `mise run schema-v2`.
- [ ] Added either a `RenderCatalog` entry or an explicit descriptor, templates, aggregates, and overrides.
- [ ] Verified secret artifacts, SOPS file selection, and generator/secret ownership boundaries.
- [ ] Added object-storage plan/apply support only if the service requires it, including rotation and recovery tests.
- [ ] Added package and rendered-contract tests; ran schema and docs-drift checks.
- [ ] Added the service reference and updated both service indexes.

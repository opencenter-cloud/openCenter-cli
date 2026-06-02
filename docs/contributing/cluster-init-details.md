---
id: cluster-init-details
title: "Cluster Init Details"
sidebar_label: Cluster Init Details
description: Explains how openCenter-cli builds a v2 cluster configuration during `cluster init` and how that differs from `cluster generate`.
doc_type: explanation
audience: "developers"
tags: [contributing]
---
# Cluster Init Details

**Purpose:** For developers, explains how the Go code creates a native v2 cluster configuration during `opencenter cluster init`.

This note describes how the current Go code creates a native v2 cluster
configuration. It intentionally ignores the user-facing docs because several of
them are stale.

## Main distinction

`opencenter cluster init` creates the v2 cluster config YAML.

`opencenter cluster generate` does not create the config. It loads an existing
v2 config and renders GitOps, application, infrastructure, and OpenTofu output
from it.

## Config creation path

The main entrypoint is `cmd/cluster_init.go`.

1. `newClusterInitCmd` defines the command and flags.
2. `runClusterInit` builds the DI app, parses flags with `parseInitOptions`,
checks provider availability, and calls `InitService.Initialize`.
3. `parseInitOptions` handles:
   * positional cluster name
   * `organization/cluster` identifiers
   * `--org`
   * `--type`
   * `--config-file`
   * `--strict`
   * `--force`
   * `--no-keygen`
   * `--no-sops-keygen`
   * `--regenerate-keys`
   * `--full-schema`
   * `--kind-disable-default-cni`
   * unknown dotted override keys, for example
   `opencenter.infrastructure.compute.worker_count=5`
4. `parseInitOptions` sets `SchemaVersion` to `2.0`.

The business logic lives in `internal/cluster/init_service.go`.

`InitService.Initialize` performs the flow:

1. Validate cluster name and organization.
2. Default empty organization to `opencenter`.
3. Resolve the org-based path layout.
4. Check whether the cluster already exists, unless `--force` is set.
5. Load an explicit config file or create a new default v2 config.
6. Apply explicit overrides and selected CLI config defaults.
7. Replace placeholder paths with resolved cluster-owned paths.
8. Optionally validate in strict mode.
9. Create cluster directories.
10. Generate SOPS and SSH keys unless disabled.
11. Save the config YAML.
12. Initialize a git repo unless disabled.

## Default v2 object construction

The actual native config object is built in
`internal/config/v2/defaults.go`.

`v2.NewV2Default(name, provider)` is the primary constructor. It creates a
`*v2.Config` with:

* `schema_version: "2.0"`
* system metadata timestamps and creator
* `opencenter.meta`
* `opencenter.cluster`
* `opencenter.infrastructure`
* `opencenter.gitops`
* `opencenter.services`
* `opencenter.managed_services`
* `deployment`
* `opentofu`
* `secrets`

It then applies provider-specific defaults:

* `applyProviderCloudDefaults` populates provider cloud blocks, currently
OpenStack and VMware.
* `applyProviderBehaviorDefaults` mutates behavior for providers such as Kind.
* `applyGitOpsAuthDefaults` chooses SSH or token Git auth defaults.

`v2.NewV2FullTemplate(name, provider)` wraps `NewV2Default` and adds a broader
set of explicit template fields for `--full-schema`.

## CLI defaults

`v2.NewV2Default` calls `loadCLIDefaults`, which reads:

```text
$OPENCENTER_CONFIG_DIR/config.yaml
```

or, when `OPENCENTER_CONFIG_DIR` is unset:

```text
~/.config/opencenter/config.yaml
```

Currently, the defaults that affect initial construction are:

* provider
* region
* environment
* `gitops_auth_method`
* `ssh_authorized_keys`

The struct also reads `base_domain`, `admin_email`, `kubernetes_version`,
`cni`, and `ssh_user`, but those values are not wired into
`NewV2Default` today.

`InitService.applyOverrides` also reads the loaded CLI config through
`ConfigManager` and can replace default region/environment values after the
base config is created.

## Path layout

Path resolution is org-based and lives in `internal/core/paths/strategies.go`.

For a cluster named `my-cluster` in org `my-org`, the resolved layout is:

```text
<clustersDir>/my-org/.my-cluster-config.yaml
<clustersDir>/my-org/infrastructure/clusters/my-cluster/
<clustersDir>/my-org/applications/overlays/my-cluster/
<clustersDir>/my-org/secrets/
<clustersDir>/my-org/secrets/age/keys/my-cluster-key.txt
<clustersDir>/my-org/secrets/ssh/my-cluster
<clustersDir>/my-org/.sops.yaml
```

The clusters root comes from `config.ResolveClustersDir`:

1. `OPENCENTER_CLUSTERS_DIR`
2. `paths.clustersDir` in the CLI config
3. `$OPENCENTER_CONFIG_DIR/clusters`
4. the default config dir plus `/clusters`

`InitService.updateConfigPaths` writes these resolved paths back into the v2
config, including:

* `opencenter.gitops.repository.local_dir`
* `opencenter.infrastructure.ssh.key_path`
* `secrets.ssh_key.private`
* `secrets.ssh_key.public`
* `secrets.sops_age_key_file`
* `secrets.sops.age_key_file`

It also rewrites Git auth defaults based on the effective `gitops_auth_method`.

## Overrides

`cluster init` accepts normal Cobra flags and also accepts dotted override
flags because the command has `UnknownFlags: true`.

Unknown flags without dots are rejected. Unknown dotted flags are collected and
passed to `configflags.NewCLIIntegration().ProcessFlags`.

Example:

```bash
opencenter cluster init my-cluster \
  --org my-org \
  --type openstack \
  opencenter.infrastructure.compute.worker_count=5
```

The override logic updates both the typed `v2.Config` and a `map[string]any`
copy used to track whether a value was explicit. That explicit-value tracking
prevents path and auth defaults from overwriting user-supplied values.

## Saving and validation

Saving uses `internal/config/v2/loader.go`.

`ConfigLoader.SaveToFile` marshals the `v2.Config` to YAML, ensures the document
starts with `---`, and writes it atomically with `0600` permissions.

Strict init validation calls `InitService.validateConfig`, which marshals the
typed config and runs it through the v2 loader pipeline:

```text
load YAML
normalize
resolve references
apply defaults
validate
freeze
```

The v2 loader uses `KnownFields(true)`, so unknown YAML fields fail parsing.

## Guided configure path

`opencenter cluster configure --guided` can create or update the same v2 config.

The command lives in `cmd/cluster_configure.go`; the flow lives in
`internal/cluster/configure_service.go`.

When the config does not exist, `ConfigureService.loadOrCreateConfig` reuses the
same init internals:

1. `initService.createDefaultConfig`
2. `initService.applyOverrides`
3. `initService.updateConfigPaths`

Then the provider orchestrator and capability handlers discover values, prompt
the user, build patches, apply those patches to the typed config, review the
changes, generate keys, validate, and save.

For OpenStack, the provider-specific guided logic is in
`internal/cluster/openstack_configure_orchestrator.go`.

## Generate path

`opencenter cluster generate` consumes an existing config.

The command lives in `cmd/cluster_generate.go`; the main service flow lives in
`internal/cluster/setup_service.go`.

`SetupService.Setup`:

1. Resolves cluster paths.
2. Loads the v2 config through `ConfigurationManager`.
3. Confirms `schema_version` is `2.0`.
4. Checks that `opencenter.gitops.repository.local_dir` is set.
5. Optionally validates setup config.
6. Renders outputs.
7. Validates generated manifests.
8. Commits changes unless this is a dry run.

The render step calls:

```go
gitops.CopyBase(cfg, true)
gitops.RenderClusterApps(cfg)
gitops.RenderInfrastructureCluster(cfg)
tofu.Provision(cfg) // skipped for kind in SetupService
```

`cluster generate --render-only` follows a similar render-only path in
`cmd/cluster_render.go`.

## Mental model

```text
cluster init / cluster configure --guided
  -> v2.NewV2Default or v2.NewV2FullTemplate
  -> apply overrides
  -> resolve cluster-owned paths
  -> generate keys
  -> write .<cluster>-config.yaml

cluster generate
  -> load .<cluster>-config.yaml
  -> render GitOps base
  -> render cluster apps
  -> render infrastructure cluster
  -> render/provision OpenTofu
```
---
id: migrate-clusters
title: Migrate the legacy layout
sidebar_label: Migrate the legacy layout
description: Preview and apply the filesystem layout migration for a legacy openCenter configuration.
doc_type: how-to
audience: openCenter operators
tags: [migration, configuration]
last_updated: 2026-10-22
---
# Migrate the legacy layout

The supported migration command moves a legacy mixed organization directory
into separate GitOps, state, and secrets locations. It is a filesystem layout
migration, not a provider-to-provider cluster migration.

## Preview

```bash
opencenter cluster migrate-layout --org ORG --dry-run
```

The command requires a legacy Git repository and legacy layout markers. It
prints planned `MOVE` operations and config path rewrites without changing
files.

## Apply

```bash
opencenter cluster migrate-layout --org ORG
```

Use `--force` to overwrite existing destinations. Review collisions before
using it.

## Move hand-authored overlay files

For one cluster, custom overlay migration is a dry run unless `--apply` is
given:

```bash
opencenter cluster migrate-layout --custom --org ORG --cluster CLUSTER
opencenter cluster migrate-layout --custom --org ORG --cluster CLUSTER --apply
```

The implementation maps eligible hand-authored service files into service
`custom/` directories and updates custom kustomizations. It reports tracked,
modified, unknown, refused, and symlinked files rather than claiming that all
files can be moved automatically.

## Kubeconfig is not relocated

The migration moves the cluster config, SOPS/SSH keys, and the GitOps tree. It
does **not** move a kubeconfig into the cluster state zone. A `kubeconfig.yaml`
left in the legacy `infrastructure/clusters/<cluster>/` directory is carried
into the new GitOps tree, which the path resolver deliberately ignores:
kubeconfigs are only ever read from
`state/<organization>/<cluster>/kubeconfig.yaml` (see
[File Locations](../reference/file-locations.md)).

After migrating, regenerate the kubeconfig in the state zone rather than
copying the old file:

```bash
opencenter cluster deploy ORG/CLUSTER   # or re-run the provider bootstrap
eval "$(opencenter cluster env ORG/CLUSTER)"   # exports KUBECONFIG from the state zone
```

`cluster status`, `cluster describe`, and `cluster use` all resolve the
state-zone path, so a kubeconfig still sitting in the GitOps tree is invisible
to them.

## Evidence

- Command contract and flags: `cmd/cluster_migrate_layout.go`
- Migration tests: `cmd/cluster_migrate_layout_test.go`
- Secure path model: `internal/core/paths/`

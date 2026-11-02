---
id: manage-secrets
title: Manage secrets
sidebar_label: Manage secrets
description: Generate keys and manage encrypted secret files with openCenter.
doc_type: how-to
audience: openCenter operators
tags: [secrets, security]
last_updated: 2026-10-27
---
# Manage secrets

The CLI exposes SOPS/Age key and file operations. Secret values should not be
placed in documentation or committed unencrypted.

## Generate and validate a key

```bash
opencenter secrets keys generate
opencenter secrets keys generate --key-file ./keys/cluster.txt
opencenter secrets keys validate
opencenter secrets keys validate --key-file ./keys/cluster.txt
```

`generate` updates SOPS configuration by default; use
`--update-sops-config=false` to disable that update. `validate` also accepts
`--config-file` for the SOPS configuration path.

## Encrypt, decrypt, and inspect files

```bash
opencenter secrets encrypt --path PATH
opencenter secrets decrypt --path PATH
opencenter secrets status --path PATH
```

The path is passed to the SOPS file operation; the command does not establish a
particular repository layout. `--no-backup` is available for encrypt/decrypt.

For cluster-generated manifests, synchronize configured values first:

```bash
opencenter secrets sync ORG/CLUSTER
opencenter secrets validate ORG/CLUSTER
```

`sync` supports `--services`, `--dry-run`, `--force`, and `--all`; `validate
--fix` can invoke synchronization for detected drift.

## Sync secrets before deploying

Run `secrets sync` **before** `cluster deploy`, not after. The canonical order is:

```bash
opencenter cluster generate ORG/CLUSTER
opencenter secrets sync ORG/CLUSTER
opencenter cluster deploy ORG/CLUSTER
```

`cluster generate` renders the service overlays but does not create the
per-service secret manifests (for example `opencenter-mimir-secret`). Those are
materialized by `secrets sync` and wired into each service's kustomization.
Deploying before syncing leaves secret-consuming services (such as Mimir)
failing with `secret "opencenter-<service>-secret" not found`.

To prevent that, `cluster deploy` runs a preflight that verifies every required
secret manifest is created and wired before provisioning any infrastructure. If
one is missing it stops immediately with:

```
required secret manifests are not ready for deploy:
  services/<service>/secret.yaml (not created)
Run 'opencenter secrets sync ORG/CLUSTER' before deploying ...
```

Re-run `secrets sync` (and let its manifest refresh complete) to resolve it.

One exception: when `secrets.grafana.admin_password` is set, the Grafana
credential is rendered by `cluster generate` as the dedicated
`services/kube-prometheus-stack/grafana-admin-password.yaml` manifest instead
of the generic `secret.yaml` (both share the `observability/grafana-admin-password`
identity, and Kustomize rejects the duplicate). The preflight accepts the
dedicated manifest in place of `secret.yaml` for that service.


## Rotate and back up keys

```bash
opencenter secrets keys rotate --cluster ORG/CLUSTER --type age
opencenter secrets keys rotate --cluster ORG/CLUSTER --type ssh
opencenter secrets keys backup --backup-dir ./key-backups
opencenter secrets keys reconcile --cluster ORG/CLUSTER
```

Rotation accepts `--dry-run`; Age rotation also accepts `--complete`. Reconcile
is a preview unless `--apply` is supplied.

## Evidence

- Key commands: `cmd/secrets_keys.go`
- File operations: `cmd/secrets_sops.go`
- Cluster synchronization: `cmd/secrets_sync.go`, `cmd/secrets_validate.go`
- Configuration and key paths: `internal/sops/`, `internal/config/v2/`

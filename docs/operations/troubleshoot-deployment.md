---
id: troubleshoot-deployment
title: Troubleshoot a command or deployment
sidebar_label: Troubleshoot a command or deployment
description: Use local checks and command output to investigate openCenter commands and deployments.
doc_type: how-to
audience: openCenter operators
tags: [troubleshooting, deployment]
last_updated: 2026-09-25
---
# Troubleshoot a command or deployment

**Purpose:** For openCenter operators, shows how to use local checks and command output to investigate openCenter commands and deployments.

Start with read-only local checks and the command output. Avoid assuming that
validation proves provider access or that deploy completion proves service
reconciliation.

## Check local prerequisites

```bash
opencenter cluster doctor
opencenter cluster doctor --output json
```

Doctor checks the fixed executable catalog: git, kubectl, helm, flux, sops,
tofu or terraform, kind, podman or docker, ssh, ssh-keyscan, and ssh-keygen.
It does not check the OpenStack CLI or external `age` executable and does not
contact a provider.

## Inspect configuration and state

```bash
opencenter cluster describe ORG/CLUSTER --validate
opencenter cluster status ORG/CLUSTER --paths
opencenter cluster validate ORG/CLUSTER --verbose
```

Use `status --refresh` only when live Kubernetes/provider inspection is wanted;
`status --sync` writes live service status into configuration. `--sync-timeout`
controls that command's live-sync timeout.

## Inspect deploy progress

Deploy reports its bootstrap log and resume state paths. Retry after fixing the
reported issue:

```bash
opencenter cluster deploy ORG/CLUSTER --from-step STEP_ID
opencenter cluster deploy ORG/CLUSTER --restart
```

`--step` runs one step; `--step` and `--from-step` cannot be combined. An
existing operation lock can be handled with `--break-lock`.

## Inspect drift

Drift commands contact the configured provider and therefore are not offline
validation:

```bash
opencenter cluster drift detect ORG/CLUSTER --severity warning
opencenter cluster drift reconcile ORG/CLUSTER --dry-run
opencenter cluster drift reconcile ORG/CLUSTER --confirm
```

Only reconcilable items are automatically applied. `--to-config` is a separate
live-to-config promotion path and requires review.

## Evidence

- Doctor: `cmd/cluster_doctor.go`
- Status/paths: `cmd/cluster_status.go`, `cmd/cluster_describe.go`
- Deploy state: `cmd/cluster_deploy.go`
- Drift: `cmd/cluster_drift.go`

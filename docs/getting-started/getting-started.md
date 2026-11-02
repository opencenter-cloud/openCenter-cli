---
id: getting-started
title: Getting Started with openCenter
sidebar_label: Getting Started
description: Follow the documented openCenter workflow for initializing, inspecting, validating, generating, and deploying a cluster configuration.
doc_type: tutorial
audience: openCenter users
tags: [getting-started, configuration]
last_updated: 2026-11-02
---
# Getting Started with openCenter

This tutorial follows the repository's supported configuration flow: initialize a
v2 cluster file, select it, validate it, generate its GitOps tree, and deploy it.
It does not assume that infrastructure, Kubernetes, or Git remotes are available.

Before starting, install the CLI — see
[Install the CLI](../reference/installation.md) — and confirm it runs with
`opencenter version`.

## 1. Create a configuration

`cluster init` accepts an organization and one of the provider types exposed by
the command (`openstack`, `baremetal`, `kind`, `vmware`, or `magnum`).

```bash
opencenter cluster init demo --org my-org --type kind
```

The organization-aware configuration is written under the configured directory
using the layout `clusters/blueprints/<org>/<cluster>/<cluster>-config.yaml`.
For this example, the path is `clusters/blueprints/my-org/demo/demo-config.yaml`.
Use the CLI rather than assuming a path when a non-default config directory is
used:

```bash
opencenter cluster describe my-org/demo
```

`init` may generate SSH and SOPS Age keys. `--no-keygen` and
`--no-sops-keygen` disable those actions; `--force` permits overwriting an
existing configuration.

## 2. Select the cluster

```bash
opencenter cluster use my-org/demo
```

The selected identifier is used by commands whose cluster argument is optional.
The workflow test verifies that the active marker contains `my-org/demo`.

## 3. Edit and validate

```bash
opencenter cluster edit my-org/demo
opencenter cluster validate my-org/demo
```

Validation accepts only `schema_version: "2.0"`. The default validation mode is
`offline`; it checks local configuration and does not contact providers, Git
remotes, Kubernetes APIs, or other external services. Use online checks only
when those services are available:

```bash
opencenter cluster validate my-org/demo --validation online
```

To validate a standalone file, use `--config-file`:

```bash
opencenter cluster validate --config-file ./demo.yaml
```

The workflow fixture demonstrates a cross-field error: with Octavia disabled
and VRRP enabled, `vrrp_ip` must be set.

## 4. Generate the GitOps tree

```bash
opencenter cluster generate my-org/demo
```

Generation writes the configured GitOps directory and rendered manifests. Use
`--dry-run` globally to preview a mutating operation, `--render-only` to render
without the repository setup flow, or `--skip-validation` only when the
configuration has been validated separately.

The workflow test proves that generation creates the configured repository and
an `applications` directory. Inspect the path reported by the command or by
`cluster describe`; do not infer a fixed home-directory path.

## 5. Deploy when the target is available

```bash
opencenter cluster deploy my-org/demo
```

Deployment uses the provider selected in the configuration. It records state,
acquires an operation lock, and can be resumed after a failed step. Useful
options are `--dry-run`, `--restart`, `--step`, `--from-step`, `--kubeconfig`,
`--log`, and `--break-lock`.

The repository workflow marks deployment as work in progress because its test
harness has no infrastructure. A successful local validation or generation is
not evidence that a real provider deployment will succeed.

## Evidence

- Command definitions: `cmd/cluster_init.go`, `cmd/cluster_use.go`,
  `cmd/cluster_validate.go`, `cmd/cluster_generate.go`, `cmd/cluster_deploy.go`
- Workflow paths and validation behavior: `tests/features/workflow.feature`
- Hand-authored v2 fixture: `tests/features/validation.feature`

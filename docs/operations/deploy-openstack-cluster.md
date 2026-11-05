---
id: deploy-openstack-cluster
title: Deploy from a cluster file
sidebar_label: Deploy from a cluster file
description: Validate, generate, and deploy assets from an openCenter cluster file.
doc_type: how-to
audience: openCenter operators
tags: [deployment, gitops]
last_updated: 2026-09-25
---
# Deploy from a cluster file

**Purpose:** For openCenter operators, shows how to validate, generate, and deploy assets from an openCenter cluster file.

The deploy workflow is provider-independent at the CLI boundary. Configure and
validate first, then generate the GitOps assets before invoking deploy.

```bash
opencenter cluster init demo --org my-org --type openstack
opencenter cluster edit my-org/demo
opencenter cluster validate my-org/demo
opencenter cluster generate my-org/demo
opencenter cluster deploy my-org/demo
```

Use `cluster use my-org/demo` to make the final argument optional. Use
`cluster describe` to find the configured GitOps and state paths.

## Preview and resume

```bash
opencenter --dry-run cluster deploy my-org/demo
opencenter cluster deploy my-org/demo --step STEP_ID
opencenter cluster deploy my-org/demo --from-step STEP_ID
opencenter cluster deploy my-org/demo --restart
```

`--step` and `--from-step` are mutually exclusive. The operation is resumable;
the command reports a bootstrap log and saved state path when applicable.

## GitOps working tree behavior

Before a non-dry-run deploy, the command checks the configured local GitOps
working tree and, when a remote URL is configured, verifies the `origin` URL.
Resolve a warning or mismatch in the checkout before retrying.

## Evidence boundary

The repository's end-to-end workflow is marked `@wip` because infrastructure
is not available in its test harness. Therefore this page documents command
ordering and flags, not successful cloud provisioning or service readiness.

- Workflow: `tests/features/workflow.feature`
- Command definitions: `cmd/cluster_init.go`, `cmd/cluster_use.go`,
  `cmd/cluster_validate.go`, `cmd/cluster_generate.go`, `cmd/cluster_deploy.go`

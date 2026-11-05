---
id: multi-cluster-setup
title: Work with multiple clusters
sidebar_label: Work with multiple clusters
description: Set up and select multiple openCenter cluster configurations within an organization.
doc_type: tutorial
audience: openCenter users
tags: [clusters, configuration]
last_updated: 2026-09-25
---
# Work with multiple clusters

**Purpose:** For openCenter users, shows how to set up and select multiple openCenter cluster configurations within an organization.

Organizations are part of the cluster identifier and filesystem layout. The
repository workflow proves this form:

```bash
opencenter cluster init demo --org my-org --type kind
opencenter cluster use my-org/demo
```

Create another cluster under the same organization with a distinct name:

```bash
opencenter cluster init test --org my-org --type kind
```

List and inspect configurations:

```bash
opencenter cluster list
opencenter cluster describe my-org/demo
opencenter cluster describe my-org/test
```

Select the cluster for commands that use the active selection:

```bash
opencenter cluster use my-org/test
opencenter cluster active
```

Each cluster is validated and generated independently:

```bash
opencenter cluster validate my-org/demo
opencenter cluster generate my-org/demo
opencenter cluster validate my-org/test
opencenter cluster generate my-org/test
```

Use an explicit `org/cluster` identifier in automation to avoid depending on
the active marker. The global `--config-dir` option changes the root used by
the CLI; the workflow fixture uses it to isolate tests.

## Evidence and boundary

- Organization selection and active-marker paths: `tests/features/workflow.feature`
- Cluster commands: `cmd/cluster_list.go`, `cmd/cluster_use.go`,
  `cmd/cluster_describe.go`
- The checkout does not establish organization-wide defaults, shared secrets,
  automatic promotion, or a multi-cluster deployment outcome; those are not
  described here.

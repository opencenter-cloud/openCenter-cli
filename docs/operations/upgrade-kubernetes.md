---
id: upgrade-kubernetes
title: Kubernetes version changes
sidebar_label: Kubernetes version changes
description: Update and validate the Kubernetes version value in an openCenter configuration.
doc_type: how-to
audience: openCenter operators
tags: [kubernetes, configuration]
last_updated: 2026-09-25
---
# Kubernetes version changes

**Purpose:** For openCenter operators, shows how to update and validate the Kubernetes version value in an openCenter configuration.

This checkout does not define a built-in Kubernetes upgrade subcommand.
Kubernetes version is a configuration value, so the evidenced openCenter portion of a
version change is:

```bash
opencenter cluster edit ORG/CLUSTER
opencenter cluster validate ORG/CLUSTER
opencenter cluster generate ORG/CLUSTER
```

Edit the existing `opencenter.cluster.kubernetes.version` field in the v2 file.
The repository's fixtures use semantic-version values, but do not establish a
universally deployable version or an upgrade strategy for every provider.

Do not infer that changing the value upgrades a live cluster. Applying the
generated assets and any provider/Kubernetes node procedure must be verified
for the selected lifecycle outside this CLI command surface.

## Evidence

- Configuration validation: `cmd/cluster_validate.go`
- Generation: `cmd/cluster_generate.go`
- Version field in fixture: `tests/features/validation.feature`

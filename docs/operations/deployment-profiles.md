---
id: deployment-profiles
title: Service enablement profiles
sidebar_label: Service enablement profiles
description: Inspect and change service selection represented in an openCenter configuration.
doc_type: how-to
audience: openCenter operators
tags: [services, configuration]
last_updated: 2026-09-25
---
# Service enablement profiles

**Purpose:** For openCenter operators, shows how to inspect and change service selection represented in an openCenter configuration.

There is no discrete profile flag in the CLI. Service selection is represented
by each service's `enabled` field in the v2 configuration.

Inspect the available service state for a cluster:

```bash
opencenter cluster service status --cluster ORG/CLUSTER
opencenter cluster service options SERVICE --cluster ORG/CLUSTER
```

Enable or disable a service through the command surface:

```bash
opencenter cluster service enable SERVICE --cluster ORG/CLUSTER
opencenter cluster service disable SERVICE --cluster ORG/CLUSTER
```

The commands support service parameters and secrets through their documented
flags. They edit the cluster file; apply the change separately:

```bash
opencenter cluster validate ORG/CLUSTER
opencenter cluster generate ORG/CLUSTER
```

For a generated configuration, keep service YAML structural and use only typed
fields accepted by the schema. Do not add renderer metadata or assume that an
enabled service implies a live deployment.

## Evidence

- Service command surface: `cmd/cluster_service.go`
- Service defaults and typed fields: `internal/config/v2/services.go`,
  `internal/config/v2/defaults.go`
- Validation: `internal/config/v2/readiness.go`

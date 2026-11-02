---
id: providers
title: Infrastructure providers
sidebar_label: Infrastructure providers
description: Reference the infrastructure provider types and common lifecycle workflow exposed by openCenter.
doc_type: reference
audience: openCenter users
tags: [providers, reference]
last_updated: 2026-11-02
---
# Infrastructure providers

`cluster init --type` exposes these provider names:

| Name | Configuration boundary evidenced in this checkout |
| --- | --- |
| `openstack` | OpenStack cloud fields and provider lifecycle are implemented. |
| `magnum` | OpenStack Magnum configuration and validation are implemented. |
| `vmware` | Static VMware node inventory and VMware cloud selectors are validated. |
| `baremetal` | Static master and worker node inventory is validated. |
| `kind` | Local Kind configuration and container-runtime deployment are implemented. |

The configuration model also contains the `aws`, `gcp`, and `azure` provider
types. The CLI availability gate rejects exactly those three for cluster
lifecycle commands; `magnum` is not gated by it. Schema presence is not
deployment support.

## Common workflow

```bash
opencenter cluster init CLUSTER --org ORG --type PROVIDER
opencenter cluster describe ORG/CLUSTER
opencenter cluster validate ORG/CLUSTER
opencenter cluster generate ORG/CLUSTER
opencenter cluster deploy ORG/CLUSTER
```

Use the guided configuration workflow where supported. Use
`cluster validate --validation online` only when provider and Git remote checks
are intended. `cluster generate` and `cluster deploy` reject providers that the
availability gate marks unavailable.

## Provider-specific pages

- [VMware](vmware.md)
- [OpenStack configuration tutorial](../getting-started/openstack-first-cluster.md)
- [Kind configuration tutorial](../getting-started/kind-local-development.md)

## Evidence

- Provider availability: `cmd/provider_availability.go`
- Provider initialization: `cmd/cluster_init.go`
- Provider validation: `internal/config/v2/readiness.go`,
  `internal/config/v2/validator_magnum_test.go`

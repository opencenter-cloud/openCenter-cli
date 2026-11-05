---
id: validate-configuration
title: Validate configuration
sidebar_label: Validate configuration
description: Validate an openCenter v2 cluster configuration in offline or online mode.
doc_type: how-to
audience: openCenter operators
tags: [validation, configuration]
last_updated: 2026-10-21
---
# Validate configuration

**Purpose:** For openCenter operators, shows how to validate an openCenter v2 cluster configuration in offline or online mode.

Validation operates on v2 configurations (`schema_version: "2.0"`). It can
use the active cluster, an explicit identifier, or a file.

```bash
opencenter cluster validate
opencenter cluster validate ORG/CLUSTER
opencenter cluster validate --config-file ./cluster.yaml
```

## Validation modes

Offline is the default. It validates schema, required and cross-field rules,
local GitOps configuration, and local secret/configuration checks without
contacting providers, Git remotes, or Kubernetes APIs.

```bash
opencenter cluster validate ORG/CLUSTER --validation offline
opencenter cluster validate ORG/CLUSTER --validation online
```

Online adds provider discovery/connectivity and Git remote checks. The mode can
also be configured through the CLI behavior setting:

```bash
opencenter settings get behavior.validation
opencenter settings set behavior.validation online
```

## Useful output options

```bash
opencenter cluster validate ORG/CLUSTER --verbose
opencenter cluster validate ORG/CLUSTER --output json
opencenter cluster validate ORG/CLUSTER --generate-debug-config
opencenter cluster validate ORG/CLUSTER --generate-debug-config --output-dir ./artifacts
```

The debug command reports the saved path. `--manifests` validates generated
GitOps manifests rather than only the configuration file.

## Interpreting failures

Read the reported field path and correct the configuration, then run the same
command again. The repository's workflow fixture demonstrates one rule:
`vrrp_ip` is required when `use_octavia` is false and VRRP is enabled. The v2
readiness validator also checks provider-specific required fields and rejects
placeholder values.

### Load balancer ownership

For OpenStack clusters, OpenStack CCM owns classless `LoadBalancer` Services.
MetalLB must not be enabled at the same time: both controllers would try to
reconcile the same Services, which can remove the address published by CCM
and leave Gateway routes without a reachable load-balancer address. The
validator reports this configuration conflict before generation or deployment.

## Evidence

- Command and flags: `cmd/cluster_validate.go`
- Offline/online behavior: `internal/config/v2/readiness.go`
- Executable validation scenarios: `tests/features/validation.feature`
- Cross-field workflow scenario: `tests/features/workflow.feature`

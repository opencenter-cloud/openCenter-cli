---
last_updated: 2026-09-24
id: drift-detection
title: "Infrastructure Drift Detection"
sidebar_label: Infrastructure Drift Detection
description: How openCenter detects and remediates differences between desired and actual infrastructure state.
doc_type: explanation
audience: "platform operators, architects"
tags: [drift, infrastructure, reconciliation]
---
# Infrastructure Drift Detection

**Purpose:** Explain what `opencenter cluster drift` checks today and where the support boundary stops.

## What the Command Compares

The drift workflow builds a desired infrastructure model from the cluster configuration and compares it with the provider’s live API state.

Today that means:

* **OpenStack**: servers, networks, security groups, load balancers, volumes, and floating IPs.
* **VMware**: configured VM nodes, attached networks, and datastore-backed storage expectations.

## Supported Providers

`internal/cloud.CloudProviderFactory` only has two `CloudProvider` implementations registered for drift work — OpenStack and VMware (`cmd/cluster_drift.go`). Kind and bare metal have no entry in that factory at all: asking for drift on those providers is a configuration/usage error, not a partial feature.

| Provider | Status | Reconciliation |
| --- | --- | --- |
| OpenStack | Supported | Limited safe reconciliation for mutable items such as tags and security-group rules |
| VMware | Supported | Detection only; remediation is manual |
| Kind | Not applicable | No entry in the drift provider factory |
| Baremetal | Not applicable | No entry in the drift provider factory |
| Magnum | Not applicable | No entry in the drift provider factory |
| AWS/GCP/Azure | Not applicable | Not a supported infrastructure provider at all |

## Typical Flow

```bash
# Detect drift
opencenter cluster drift detect prod-cluster

# Filter output by severity (critical, warning, info)
opencenter cluster drift detect prod-cluster --severity=critical

# Preview reconciliation
opencenter cluster drift reconcile prod-cluster --dry-run

# Promote approved live state back into the config file instead of
# reconciling infrastructure to match the config
opencenter cluster drift reconcile prod-cluster --to-config --confirm

# Schedule periodic drift checks
opencenter cluster drift schedule prod-cluster --interval=24h
```

Drift severities are `critical` (for example control-plane or network configuration changes), `warning` (worker nodes, tags, labels), and `info` (metadata). `cluster drift` (`cmd/cluster_drift.go`) calls the provider's `DetectDrift`, and both registered providers delegate to `CompareInfrastructureState` in `internal/cloud/compare.go`, which assigns every emitted severity. The parallel severity model in `internal/operations/drift_detector.go` is not wired into any command; only its package tests construct it.

## VMware-Specific Behavior

VMware drift detection uses the configured vCenter metadata plus `secrets.vsphere_csi` credentials to inspect:

* the configured datacenter
* the named VM nodes
* the networks attached to those VMs
* the datastores backing those VMs

The command reports differences, but it does not mutate vSphere resources for you.

## Why Kind and Baremetal Are Different

Kind is a local lifecycle provider, not a cloud-resource backend. Baremetal relies on pre-provisioned hosts that openCenter does not create or own. In both cases, the infrastructure drift contract would be misleading, so those providers are intentionally excluded.
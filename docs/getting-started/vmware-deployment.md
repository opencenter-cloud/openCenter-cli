---
id: vmware-deployment
title: Configure VMware nodes
sidebar_label: Configure VMware nodes
description: Configure pre-provisioned VMware nodes in an openCenter cluster file.
doc_type: tutorial
audience: openCenter users
tags: [vmware, configuration]
last_updated: 2026-09-26
---
# Configure VMware nodes

**Purpose:** For openCenter users, shows how to configure pre-provisioned VMware nodes in an openCenter cluster file.

The VMware provider consumes pre-provisioned node entries. This page documents
the configuration shape enforced by the provider validator; it does not claim
to provision or resize VMs.

## Initialize

```bash
opencenter cluster init vm-demo --org my-org --type vmware
opencenter cluster describe my-org/vm-demo
```

## Add provider and node values

Edit the generated configuration:

```bash
opencenter cluster edit my-org/vm-demo
```

The VMware provider requires the structural fields below. Replace example
values with values from the target environment.

```yaml
opencenter:
  infrastructure:
    provider: vmware
    cloud:
      vmware:
        vcenter_server: "vcenter.example.test"
        datacenter: "Datacenter"
        datastore: "datastore"
        network: "VM Network"
        template: "template"
    compute:
      master_nodes:
        - name: master-1
          access_ip_v4: 192.0.2.10
      worker_nodes:
        - name: worker-1
          access_ip_v4: 192.0.2.20
```

Node entries are static. The validator checks names and access addresses; VM
creation remains outside this CLI path.

## Validate, render, deploy

```bash
opencenter cluster validate my-org/vm-demo
opencenter cluster generate my-org/vm-demo
opencenter cluster deploy my-org/vm-demo
```

Use `--validation online` only when the provider/Git checks are available.
Deployment accepts `--kubeconfig`, `--log`, `--step`, `--from-step`, and
`--restart`.

## Evidence

- VMware readiness rules: `internal/config/v2/readiness.go`
- Provider model: `internal/config/v2/provider.go`
- Provider lifecycle templates: `internal/cluster/`
- CLI definitions: `cmd/cluster_init.go`, `cmd/cluster_deploy.go`

---
id: vmware
title: VMware provider
sidebar_label: VMware provider
description: Reference the VMware provider configuration shape and validation requirements.
doc_type: reference
audience: openCenter operators
tags: [vmware, providers]
last_updated: 2026-11-02
---
# VMware provider

The VMware provider uses pre-provisioned nodes. Its configuration contains a
`cloud.vmware` selector block and static `compute.master_nodes` and
`compute.worker_nodes` entries. VM lifecycle is outside the provider path
documented in this checkout.

## Required configuration shape

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

The provider validator requires the five VMware cloud selectors above. Static
node checks — a required `name` and a required `access_ip_v4` per entry — run
only for the `baremetal` provider, not for VMware. A VMware configuration can
also enable the `vsphere-csi` service and provide its credentials under the
typed `secrets.vsphere_csi` section; the exact service fields should be taken
from the generated v2 configuration and schema.

The readiness report warns when neither `opencenter.cluster.kubernetes.storage_plugin.vsphere_csi.enabled`
nor the `vsphere-csi` service is enabled. The knob the warning names is
`opencenter.cluster.kubernetes.storage_plugin.vsphere_csi`; set its `enabled`
to `true`, or configure an alternative CSI driver.

## Lifecycle commands

```bash
opencenter cluster init vm-demo --org my-org --type vmware
opencenter cluster edit my-org/vm-demo
opencenter cluster validate my-org/vm-demo
opencenter cluster generate my-org/vm-demo
opencenter cluster deploy my-org/vm-demo
```

`generate` renders the VMware infrastructure inputs and node inventory. It does
not prove that VMs, vCenter objects, SSH connectivity, or storage are present.
`deploy` is resumable and supports `--step`, `--from-step`, `--restart`,
`--kubeconfig`, and `--log`.

## Evidence

- VMware validation: `internal/config/v2/readiness.go`
- Static node validation runs only for `baremetal`: `internal/config/v2/readiness.go`
- Configuration types: `internal/config/v2/provider.go`,
  `internal/config/v2/infrastructure.go`
- CLI behavior: `cmd/cluster_init.go`, `cmd/cluster_generate.go`,
  `cmd/cluster_deploy.go`

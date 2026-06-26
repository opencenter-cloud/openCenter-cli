---
id: service-vsphere-csi
title: "vSphere CSI Driver"
sidebar_label: vSphere CSI
description: VMware vSphere Container Storage Interface driver for dynamic volume provisioning.
doc_type: reference
audience: "platform engineers, operators"
tags: [storage, vsphere, vmware, csi, persistent-volumes]
---

> **Purpose:** For platform engineers and operators, documents the vSphere CSI driver configuration, covering storage classes, secrets, and verification for VMware environments.

## Overview

The vSphere CSI driver enables dynamic provisioning of persistent volumes backed by VMware vSphere datastores. It supports volume snapshots, online volume expansion, storage policies, and topology-aware scheduling. Available only for clusters using the VMware/vSphere provider.

## Configuration

```yaml
opencenter:
  services:
    vsphere-csi:
      enabled: false                         # default: false
      storage_classes:
        - name: vsphere-default
          datastore_url: "ds:///vmfs/volumes/datastore1/"
          reclaim_policy: Delete             # Retain or Delete
          volume_binding_mode: WaitForFirstConsumer  # Immediate or WaitForFirstConsumer
          allow_expansion: true
        - name: vsphere-retain
          datastore_url: "ds:///vmfs/volumes/datastore1/"
          reclaim_policy: Retain
          volume_binding_mode: Immediate
          allow_expansion: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable vSphere CSI driver |
| `storage_classes` | list | `[]` | Storage class definitions |
| `storage_classes[].name` | string | — | StorageClass name |
| `storage_classes[].datastore_url` | string | — | vSphere datastore URL |
| `storage_classes[].reclaim_policy` | string | — | `Retain` or `Delete` |
| `storage_classes[].volume_binding_mode` | string | — | `Immediate` or `WaitForFirstConsumer` |
| `storage_classes[].allow_expansion` | bool | `true` | Allow online volume expansion |

## Secrets

Configured under `secrets.vsphere_csi`:

```yaml
secrets:
  vsphere_csi:
    vcenter_host: "vcenter.example.com"
    username: "administrator@vsphere.local"
    password: "ENC[AES256_GCM,data:...,type:str]"
    datacenters: "DC1"
    insecure_flag: false                    # default: false
    port: 443                               # default: 443
    datastoreurl: "ds:///vmfs/volumes/datastore1/"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `vcenter_host` | string | — | vCenter Server hostname or IP |
| `username` | string | — | vCenter authentication username |
| `password` | string | — | vCenter authentication password (SOPS encrypted) |
| `datacenters` | string | — | Comma-separated datacenter names |
| `insecure_flag` | bool | `false` | Skip TLS certificate verification |
| `port` | int | `443` | vCenter API port |
| `datastoreurl` | string | — | Default datastore URL |

## Dependencies

None. Requires the VMware/vSphere infrastructure provider.

## Verification

```bash
# Check CSI controller pod
kubectl get pods -n vmware-system-csi

# Verify CSI driver registration
kubectl get csidrivers csi.vsphere.vmware.com

# List storage classes
kubectl get storageclasses

# Check CSI node status
kubectl get csinodes

# Test volume provisioning
kubectl get pvc --all-namespaces
```

## CLI Commands

```bash
# Enable vSphere CSI
opencenter cluster service enable vsphere-csi

# Disable vSphere CSI
opencenter cluster service disable vsphere-csi

# View configuration options
opencenter cluster service options vsphere-csi

# Check service status
opencenter cluster service status
```

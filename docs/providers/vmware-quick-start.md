---
id: vmware-quick-start
title: "VMware Provider Quick Start"
sidebar_label: VMware Quick Start
description: Quick reference for deploying openCenter clusters on VMware vSphere.
doc_type: how-to
audience: "platform engineers, operators"
tags: [vmware, vsphere, quick-start, deployment]
---

# VMware Provider Quick Start

Quick reference for deploying openCenter clusters on VMware vSphere.

## Prerequisites

- Pre-provisioned Ubuntu 24.04 VMs
- Static IP addresses assigned
- SSH access from bastion host
- vCenter credentials (for CSI driver)

## Quick Setup

### 1. Generate Template

```bash
opencenter cluster template --out vmware.yaml
```

### 2. Edit Configuration

Minimal required configuration:

```yaml
opencenter:
  infrastructure:
    provider: vmware
    cloud:
      vmware:
        vcenter_server: vcenter.example.com
        datacenter: Datacenter1
        datastore: datastore1
        nodes:
          - {name: master-1, ip: 192.168.1.10, role: master}
          - {name: master-2, ip: 192.168.1.11, role: master}
          - {name: master-3, ip: 192.168.1.12, role: master}
          - {name: worker-1, ip: 192.168.1.20, role: worker}
          - {name: worker-2, ip: 192.168.1.21, role: worker}
```

### 3. Initialize Cluster

```bash
opencenter cluster init my-cluster --type vmware
```

### 4. Deploy

```bash
opencenter cluster generate my-cluster
opencenter cluster deploy my-cluster
```

## Node Configuration

Each node requires:
- `name`: Hostname or FQDN
- `ip`: Static IPv4 address
- `role`: `master` or `worker`

Optional fields:
- `uuid`: VM UUID from vCenter
- `mac_address`: Primary NIC MAC address

## vSphere CSI Driver

Enable persistent storage:

```yaml
opencenter:
  services:
    vsphere-csi:
      enabled: true

secrets:
  vsphere_csi:
    vcenter_host: vcenter.example.com
    username: administrator@vsphere.local
    password: "encrypted-with-sops"
    datacenters: Datacenter1
```

## Common Issues

### SSH Connection Failed
```bash
# Test connectivity
ssh ubuntu@192.168.1.10 hostname
```

### Node Not Ready
```bash
# Check kubelet
ssh ubuntu@192.168.1.10 "systemctl status kubelet"
```

### Storage Not Working
```bash
# Verify CSI driver
kubectl get pods -n kube-system | grep vsphere-csi
```

## Key Differences from OpenStack

| Feature | OpenStack | VMware |
|---------|-----------|--------|
| VM Provisioning | Automatic | Manual |
| Terraform | Yes | No |
| Node Scaling | Dynamic | Manual |
| Storage | Cinder CSI | vSphere CSI |
| Load Balancer | Octavia | MetalLB |

## Next Steps

- [Full VMware Guide](./vmware.md)
- [vSphere CSI Configuration](../services/vsphere-csi.md)
- [Kubespray Deployment](../deployment/kubespray.md)

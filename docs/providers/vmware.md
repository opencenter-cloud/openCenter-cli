---
id: vmware-provider-guide
title: "VMware Provider Guide"
sidebar_label: VMware Provider Guide
description: Complete guide for deploying openCenter clusters on VMware vSphere with pre-provisioned VMs.
doc_type: how-to
audience: "platform engineers, operators"
tags: [vmware, vsphere, deployment, csi]
---
# VMware Provider Guide

**Purpose:** For platform engineers, operators, shows how to deploying openCenter clusters on VMware vSphere with pre-provisioned VMs.

Guide for deploying openCenter clusters on VMware vSphere infrastructure with pre-provisioned VMs.

## Table of Contents

* [Overview](#overview)
* [Prerequisites](#prerequisites)
* [Architecture](#architecture)
* [Configuration](#configuration)
  * [Basic Configuration](#basic-configuration)
  * [Node Configuration](#node-configuration)
  * [vSphere Integration](#vsphere-integration)
* [Deployment](#deployment)
* [Storage](#storage)
* [Networking](#networking)
* [Limitations](#limitations)
* [Troubleshooting](#troubleshooting)

## Overview

The VMware provider enables openCenter cluster deployment on VMware vSphere infrastructure. VMs must be pre-provisioned - the provider treats VMware as baremetal, using Kubespray/Ansible to configure existing VMs rather than provisioning new ones.

Key characteristics:

* Requires pre-provisioned VMs with Ubuntu 24.04
* Uses Kubespray deployment method (Ansible-based)
* Supports vSphere CSI driver for persistent storage
* No automatic VM lifecycle management

## Prerequisites

### Infrastructure Requirements

* VMware vSphere 7.0 or later
* Pre-provisioned Ubuntu 24.04 VMs (minimum 3 control plane + 2 worker nodes)
* VMs must have network connectivity to each other
* SSH access to all VMs from bastion/deployment host
* vCenter credentials (for CSI driver integration)

### VM Specifications

Control plane nodes (minimum):

* 4 vCPUs
* 8 GB RAM
* 40 GB disk

Worker nodes (minimum):

* 4 vCPUs
* 16 GB RAM
* 40 GB disk

### Network Requirements

* Static IP addresses for all nodes
* DNS resolution for all node hostnames
* Bastion host with SSH access to all nodes
* Firewall rules allowing Kubernetes traffic (6443, 2379-2380, 10250-10252)

## Architecture

```
┌─────────────────────────────────────────────────────┐
│ vCenter Server                                      │
│  - Manages VMs                                      │
│  - Provides CSI driver integration                  │
└─────────────────────────────────────────────────────┘
                      │
                      │ API
                      ▼
┌─────────────────────────────────────────────────────┐
│ VMware Datacenter                                   │
│  ┌───────────────────────────────────────────────┐ │
│  │ Compute Cluster                               │ │
│  │  ┌─────────────────────────────────────────┐ │ │
│  │  │ Pre-provisioned VMs                     │ │ │
│  │  │  - master-1 (192.168.1.10)             │ │ │
│  │  │  - master-2 (192.168.1.11)             │ │ │
│  │  │  - master-3 (192.168.1.12)             │ │ │
│  │  │  - worker-1 (192.168.1.20)             │ │ │
│  │  │  - worker-2 (192.168.1.21)             │ │ │
│  │  └─────────────────────────────────────────┘ │ │
│  └───────────────────────────────────────────────┘ │
│  ┌───────────────────────────────────────────────┐ │
│  │ Datastore (Persistent Volumes)                │ │
│  └───────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

## Configuration

### Basic Configuration

Create a VMware cluster configuration, then open it for provider-specific values:

```bash
opencenter cluster init my-vmware-cluster --type vmware --org myorg
opencenter cluster configure myorg/my-vmware-cluster
```

Minimal configuration:

```yaml
schema_version: 2.0
opencenter:
  meta:
    name: my-vmware-cluster
    organization: myorg
  infrastructure:
    provider: vmware
    ssh_user: ubuntu
    os_version: "24"
    bastion:
      address: bastion.example.com
    cloud:
      vmware:
        vcenter_server: vcenter.example.com
        datacenter: Datacenter1
        datastore: datastore1
        cluster: Cluster1
        network: VM Network
        nodes:
          - name: master-1.example.com
            ip: 192.168.1.10
            role: master
          - name: master-2.example.com
            ip: 192.168.1.11
            role: master
          - name: master-3.example.com
            ip: 192.168.1.12
            role: master
          - name: worker-1.example.com
            ip: 192.168.1.20
            role: worker
          - name: worker-2.example.com
            ip: 192.168.1.21
            role: worker
  cluster:
    cluster_name: my-vmware-cluster
    kubernetes:
      version: 1.33.5
      master_count: 3
      worker_count: 2
  gitops:
    git_dir: ./gitops-repo
opentofu:
  enabled: false
secrets:
  vsphere_csi:
    vcenter_host: vcenter.example.com
    username: administrator@vsphere.local
    password: ""  # Encrypted with SOPS
    datacenters: Datacenter1
    insecure_flag: "false"
    port: "443"
```

### Node Configuration

Each node requires:

```yaml
nodes:
  - name: master-1.example.com    # FQDN or hostname
    ip: 192.168.1.10              # Static IP address
    role: master                  # master or worker
    uuid: ""                      # Optional: VM UUID
    mac_address: 00:50:56:12:34:56  # Optional: Primary NIC MAC
```

Node roles:

* `master`: Control plane node (runs etcd, API server, scheduler, controller)
* `worker`: Worker node (runs application workloads)

### vSphere Integration

vSphere CSI driver configuration:

```yaml
opencenter:
  services:
    vsphere-csi:
      enabled: true
      image_repository: registry.k8s.io/csi-vsphere
      image_tag: v3.3.0

secrets:
  vsphere_csi:
    vcenter_host: vcenter.example.com
    username: administrator@vsphere.local
    password: "your-vcenter-password"  # Encrypt with SOPS
    datacenters: Datacenter1
    insecure_flag: "false"
    port: "443"
```

## Deployment

### Step 1: Initialize Cluster Configuration

```bash
opencenter cluster init my-vmware-cluster \
  --type vmware \
  --org myorg

# Expected output:
# ✓ Created cluster configuration
# ✓ Generated SSH keys
# ✓ Generated Age encryption keys
```

### Step 2: Configure Nodes

Edit the configuration file to add your pre-provisioned VMs:

```bash
# Configuration stored at:
# ~/.config/opencenter/clusters/myorg/.my-vmware-cluster-config.yaml

# Edit the vmware.nodes section with your VM details
```

### Step 3: Validate Configuration

```bash
opencenter cluster validate my-vmware-cluster

# Expected output:
# ✓ Schema validation passed
# ✓ Provider configuration valid
# ✓ Node configuration valid
# ✓ Network configuration valid
```

### Step 4: Setup GitOps Repository

```bash
opencenter cluster generate my-vmware-cluster

# Expected output:
# ✓ Created GitOps repository structure
# ✓ Generated Kubernetes manifests
# ✓ Generated Ansible inventory
# ✓ Encrypted secrets with SOPS
```

### Step 5: Bootstrap Cluster

```bash
opencenter cluster deploy my-vmware-cluster

# This will:
# 1. Configure SSH access to all nodes
# 2. Install Kubernetes via Kubespray
# 3. Deploy FluxCD
# 4. Apply GitOps manifests
```

## Storage

### vSphere CSI Driver

The vSphere CSI driver provides dynamic persistent volume provisioning:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: vsphere-csi-sc
```

Storage classes:

```yaml
# Default storage class (created by vsphere-csi service)
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: vsphere-csi-sc
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: csi.vsphere.vmware.com
parameters:
  datastoreurl: "ds:///vmfs/volumes/datastore1/"
```

### Datastore Configuration

Specify datastore URL in service configuration:

```yaml
opencenter:
  services:
    vsphere-csi:
      enabled: true
      storage_class:
        default:
          datastore_url: "ds:///vmfs/volumes/1375553-datastore1/"
```

## Networking

### Network Plugin

VMware provider supports Calico CNI (default):

```yaml
opencenter:
  cluster:
    kubernetes:
      network_plugin:
        calico:
          enabled: true
          cni_iface: ens192  # Adjust to match your VM network interface
          encapsulation_type: VXLAN
```

### Load Balancer

For LoadBalancer services, use MetalLB:

```yaml
opencenter:
  services:
    metallb:
      enabled: true
      ip_address_pools:
        - name: default
          addresses:
            - 192.168.1.100-192.168.1.110
```

### Ingress

Gateway API with Istio or ingress-nginx:

```yaml
opencenter:
  services:
    gateway-api:
      enabled: true
    gateway:
      enabled: true
      hostname: "*.example.com"
```

## Limitations

### No Automatic Provisioning

* VMs must be pre-provisioned manually
* No automatic scaling (MachineDeployments not supported)
* Node lifecycle managed outside openCenter

### Deployment Method

* Only Kubespray deployment method supported
* Talos deployment not supported (requires cloud-init integration)
* Kamaji supported but requires manual worker node provisioning

### Infrastructure Management

* No Terraform/OpenTofu integration (opentofu.enabled: false)
* VM configuration changes require manual intervention
* No automated backup/restore of VMs

## Troubleshooting

### SSH Connection Issues

```bash
# Test SSH connectivity to all nodes
for node in master-1 master-2 master-3 worker-1 worker-2; do
  ssh ubuntu@${node}.example.com "hostname"
done

# Verify SSH key is configured
cat ~/.config/opencenter/clusters/myorg/secrets/ssh/my-vmware-cluster
```

### Node Not Joining Cluster

Check Kubespray logs:

```bash
# View Ansible playbook output
tail -f /var/log/opencenter/bootstrap.log

# Check node status
kubectl get nodes

# Verify kubelet is running on node
ssh ubuntu@worker-1.example.com "systemctl status kubelet"
```

### vSphere CSI Driver Issues

```bash
# Check CSI driver pods
kubectl get pods -n kube-system | grep vsphere-csi

# View CSI driver logs
kubectl logs -n kube-system deploy/vsphere-csi-controller

# Verify vCenter credentials
kubectl get secret vsphere-config-secret -n kube-system -o yaml
```

### Storage Provisioning Failures

```bash
# Check PVC status
kubectl get pvc

# View events
kubectl describe pvc my-pvc

# Verify datastore URL
kubectl get storageclass vsphere-csi-sc -o yaml
```

### Network Connectivity

```bash
# Test pod-to-pod connectivity
kubectl run test-pod --image=busybox --rm -it -- ping <pod-ip>

# Check Calico status
kubectl get pods -n calico-system

# Verify network interface
ssh ubuntu@worker-1.example.com "ip addr show ens192"
```

## Related Documentation

* [Kubespray Deployment Method](../getting-started/getting-started.md)
* [vSphere CSI Driver Configuration](../reference/platform-services.md)
* [Baremetal Provider](README.md) (similar architecture)
* [Storage Configuration](../reference/platform-services.md)
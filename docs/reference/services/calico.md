---
id: service-calico
title: "Calico CNI"
sidebar_label: Calico
description: CNI plugin providing pod networking, network policies, and BGP routing.
doc_type: reference
audience: "platform engineers, operators"
tags: [networking, cni, calico, network-policy]
---

> **Purpose:** For platform engineers and operators, documents the Calico CNI service configuration, covering networking modes, installation methods, and verification.

## Overview

Calico provides pod-to-pod networking and network policy enforcement for Kubernetes clusters. It supports VXLAN and IPIP encapsulation, BGP routing, IPv4/IPv6 dual-stack, and Kubernetes NetworkPolicy resources. Installation uses Helm (default) or kustomize-helm.

## Configuration

```yaml
opencenter:
  services:
    calico:
      enabled: true                    # default: true
      kube_api_server: ""              # Kubernetes API server address
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable Calico CNI |
| `kube_api_server` | string | `""` | Kubernetes API server address for Calico to connect to |

## Dependencies

None.

## Verification

```bash
# Check Calico pods are running
kubectl get pods -n calico-system

# Verify Calico node status
kubectl get pods -n calico-system -l k8s-app=calico-node

# Check network policies are enforced
kubectl get networkpolicies --all-namespaces

# Verify BGP peering (if using BGP mode)
kubectl exec -n calico-system -l k8s-app=calico-node -- calicoctl node status
```

## CLI Commands

```bash
# Enable Calico
opencenter cluster service enable calico

# Disable Calico
opencenter cluster service disable calico

# View Calico configuration options
opencenter cluster service options calico

# Check service status
opencenter cluster service status
```

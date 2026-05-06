---
id: providers-reference
title: "Infrastructure Providers Reference"
sidebar_label: Providers
description: Complete reference of supported infrastructure providers, requirements, and configuration.
doc_type: reference
audience: "platform engineers, operators"
tags: [providers, openstack, vmware, kind, baremetal]
---

# Infrastructure Providers Reference

**Purpose:** Complete reference of the GA infrastructure provider surface and its support boundaries.

## Provider Matrix

| Provider | GA Status | Provisioning Model | Deployment Support | Drift Detection | Notes |
|----------|-----------|--------------------|--------------------|-----------------|-------|
| OpenStack | GA | Automated | Kubespray, Kamaji | Detect + limited reconcile | Most complete automation path |
| VMware | GA | Pre-provisioned VMs | Kubespray, Kamaji | Detect only | Canonical name is `vmware`; `vsphere` is an alias |
| Kind | GA for local/dev | Built-in local runtime | Kind bootstrap flow | Not applicable | Use for development and CI only |
| Baremetal | GA | Pre-provisioned hosts | Kubespray | Not applicable | Manual provisioning and host lifecycle |
| AWS | Non-GA infrastructure provider | Not supported for GA cluster provisioning | N/A | Removed from drift registry | AWS service integrations remain supported where used by platform services |

## Drift Detection Support

`opencenter cluster drift` currently supports:

- `openstack`
- `vmware`

`kind` and `baremetal` do not register infrastructure drift backends because they do not own cloud-resource reconciliation. AWS is intentionally excluded from the GA drift registry.

## Canonical Naming

- Use `vmware` in configuration, examples, and documentation.
- Existing `vsphere` configuration values continue to load and validate as a compatibility alias.

## Windows Support

Windows worker guidance remains historical and is not part of the GA support boundary. The supported GA platform path is Linux control plane plus Linux workers.

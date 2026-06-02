---
id: provider-comparison
title: "Provider Comparison"
sidebar_label: Provider Comparison
description: How to choose the right infrastructure provider based on requirements and trade-offs.
doc_type: explanation
audience: "architects, decision makers"
tags: [providers, openstack, vmware, kind, baremetal]
---
# Provider Comparison

**Purpose:** Explain the GA provider choices and the trade-offs between them.

## Quick Recommendation

| If you need... | Choose... |
| --- | --- |
| Fully automated private-cloud provisioning | OpenStack |
| Existing vSphere investment and pre-provisioned VMs | VMware |
| Local development or CI clusters | Kind |
| Physical hosts you already manage | Baremetal |

## Capability Summary

| Provider | Provisioning | Operational Model | Drift Support | Best Fit |
| --- | --- | --- | --- | --- |
| OpenStack | Automated | Cloud-owned infrastructure plus GitOps-managed platform services | Detect + limited reconcile | Production private cloud |
| VMware | Pre-provisioned | Infrastructure team owns VM lifecycle; openCenter owns cluster/service lifecycle | Detect only | Existing enterprise virtualization |
| Kind | Built-in local runtime | Disposable developer cluster | Not applicable | Workstations and CI |
| Baremetal | Pre-provisioned | Hardware lifecycle outside openCenter | Not applicable | Edge sites and physical estates |

## Non-GA Infrastructure Providers

AWS is no longer part of the GA infrastructure-provider story. Keep using AWS-backed service integrations where platform services need them, but do not treat AWS as a supported cluster provisioning target.

## Naming and Compatibility

Documentation uses `vmware`. Existing `vsphere` values continue to load so older configs do not break, but new material should not introduce that spelling.

## Support Boundary

GA support assumes Linux control plane and Linux workers. Windows worker-node content remains informational only and should not be treated as a supported deployment target.
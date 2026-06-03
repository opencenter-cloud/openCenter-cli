---
id: providers-overview
title: "Infrastructure Providers"
sidebar_label: Infrastructure Providers
description: Overview of supported infrastructure providers and their capabilities.
doc_type: reference
audience: "platform engineers, operators"
tags: [providers, openstack, vmware, kind, baremetal]
---
# Infrastructure Providers

**Purpose:** For platform engineers, operators, documents overview of supported infrastructure providers and their capabilities.

openCenter’s GA infrastructure surface is intentionally narrow:

* **OpenStack**: full infrastructure automation and the most complete production path.
* **VMware**: pre-provisioned vSphere VMs with Kubernetes and service deployment managed by openCenter.
* **Kind**: local development and CI clusters.
* **Baremetal**: pre-provisioned Linux hosts managed through the same cluster configuration model.

## Support Boundary

| Provider | Status | Recommended Use |
| --- | --- | --- |
| OpenStack | GA | Production private cloud |
| VMware | GA | Production vSphere environments |
| Kind | GA for local/dev | Developer workstations and CI |
| Baremetal | GA | Pre-provisioned physical infrastructure |
| AWS | Planned / non-GA infrastructure provider | Not recommended for GA cluster provisioning |

## Important Notes

* Use `vmware` as the canonical provider name in new configs and docs.
* `vsphere` is still accepted when loading existing configs.
* AWS-backed service integrations stay in scope for GA features that use them, but AWS is no longer advertised as a supported cluster infrastructure provider.
* Windows worker-node content is not part of the GA support commitment.
---
id: providers-reference
title: "Infrastructure Providers Reference"
sidebar_label: Infrastructure Providers Reference
description: Complete reference of supported infrastructure providers, requirements, and configuration.
doc_type: reference
audience: "platform engineers, operators"
tags: [providers, openstack, magnum, vmware, kind, baremetal]
---
# Infrastructure Providers Reference

**Purpose:** Complete reference of the GA infrastructure provider surface and its support boundaries.

## Provider Matrix

| Provider | GA Status | Provisioning Model | Deployment Support | Drift Detection | Notes |
| --- | --- | --- | --- | --- | --- |
| OpenStack | GA | Automated | Kubespray, Kamaji | Detect + limited reconcile | Most complete automation path |
| Magnum | Supported; GA status not stated | Managed OpenStack Kubernetes provider | Magnum cluster create/poll and kubeconfig retrieval | Not currently supported | Backed by OpenStack Magnum, not OpenTofu; image, network, and COE choices come from the Magnum cluster template |
| VMware | GA | Pre-provisioned VMs | Kubespray, Kamaji | Detect only | Canonical name is `vmware`; `vsphere` is an alias |
| Kind | GA for local/dev | Built-in local runtime | Kind bootstrap flow | Not applicable | Use for development and CI only |
| Baremetal | GA | Pre-provisioned hosts | Kubespray | Not applicable | Manual provisioning and host lifecycle |
| AWS | Non-GA infrastructure provider | Not supported for GA cluster provisioning | N/A | Removed from drift registry | AWS service integrations remain supported where used by platform services |

## Magnum

Magnum is a managed OpenStack Kubernetes provider backed by the OpenStack Magnum service, not by OpenTofu. The provider supports the following lifecycle operations:

* `cluster configure` configures the Magnum provider settings.
* `cluster deploy` creates a Magnum cluster from the configured existing cluster template, polls Magnum until the cluster is ready, and securely writes the resulting kubeconfig.
* `cluster destroy` deletes the Magnum cluster.

Configuration is stored under `opencenter.infrastructure.cloud.magnum` and requires a Keystone auth URL, region, project ID, application credential ID and secret, and a cluster template. Image, network, and COE choices are owned by the Magnum cluster template rather than duplicated in the provider configuration.

## Drift Detection Support

`opencenter cluster drift` currently supports:

* `openstack`
* `vmware`

Magnum drift detection is not currently supported.

`kind` and `baremetal` do not register infrastructure drift backends because they do not own cloud-resource reconciliation. AWS is intentionally excluded from the GA drift registry.

## Canonical Naming

* Use `vmware` in configuration, examples, and documentation.
* Existing `vsphere` configuration values continue to load and validate as a compatibility alias.

## Windows Support

Windows worker guidance remains historical and is not part of the GA support boundary. The supported GA platform path is Linux control plane plus Linux workers.

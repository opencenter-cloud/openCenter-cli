---
last_updated: 2026-09-26
id: providers-reference
title: "Infrastructure Providers Reference"
sidebar_label: Infrastructure Providers Reference
description: Complete reference of supported infrastructure providers, requirements, and configuration.
doc_type: reference
audience: "platform engineers, operators"
tags: [providers, openstack, magnum, vmware, kind, baremetal]
---
# Infrastructure Providers Reference

**Purpose:** Complete reference of the available infrastructure-provider lifecycle surface, the separate drift registry, and the planned-provider boundary.

## Provider Matrix

| Provider | CLI lifecycle status | Lifecycle implementation | Deployment-method validation | Drift registry |
| --- | --- | --- | --- | --- |
| OpenStack | Lifecycle implemented | Shared OpenTofu bootstrap; Kubespray is invoked by the generated infrastructure module | `kubespray` and `kamaji` are accepted when compatible | Detect plus limited reconciliation |
| Magnum | Lifecycle implemented | Direct Magnum API create, kubeconfig export, and delete | The deployment validator does not list Magnum as Kamaji-compatible | None |
| VMware | Lifecycle implemented | Shared OpenTofu bootstrap with VMware-specific inputs | `kubespray` and `kamaji` are accepted when compatible | Detect only |
| Kind | Lifecycle implemented for local use | Kind and kubectl lifecycle provider | `kubespray` is accepted; the live lifecycle is the dedicated Kind path | None |
| Baremetal | Lifecycle implemented | Shared infrastructure bootstrap using pre-provisioned nodes | `kubespray` only; Kamaji is rejected | None |
| AWS, GCP, Azure | Schema/config support only | CLI availability gate rejects these as planned providers | Deployment validators contain compatibility rules, but that does not make the provider available | None |

## Magnum

Magnum is a managed OpenStack Kubernetes provider backed by the OpenStack Magnum service, not by OpenTofu. The provider supports the following lifecycle operations:

* `cluster configure` configures the Magnum provider settings.
* `cluster deploy` creates a Magnum cluster from the configured existing cluster template, obtains the resulting kubeconfig, and writes it to the cluster state path.
* `cluster destroy` deletes the Magnum cluster.

Configuration is stored under `opencenter.infrastructure.cloud.magnum` and requires a Keystone auth URL, region, project ID, application credential ID and secret, and a cluster template. Image, network, and COE choices are owned by the Magnum cluster template rather than duplicated in the provider configuration.

## Drift Detection Support

`opencenter cluster drift` uses the separate `internal/cloud.CloudProvider` registry. It currently supports:

* `openstack`
* `vmware`

Magnum drift detection is not currently supported.

`kind`, `baremetal`, and `magnum` do not register infrastructure drift backends. Lifecycle support and drift support are separate boundaries; a provider can deploy without implementing `CloudProvider`.

## Canonical Naming

* Use `vmware` in configuration, examples, and documentation.
* Existing `vsphere` configuration values continue to load and validate as a compatibility alias.

## Windows workers

Windows worker fields remain part of the configuration model. Provider-specific
requirements and whether a deployment path uses them are enforced by the
provider and readiness validation code; this reference does not make a broader
platform-support claim.

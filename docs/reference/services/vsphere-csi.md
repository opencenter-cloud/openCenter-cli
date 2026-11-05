---
last_updated: 2026-09-25
id: service-vsphere-csi
title: "vSphere CSI Driver"
sidebar_label: vSphere CSI
description: vSphere CSI service configuration, storage classes, and catalog rendering.
doc_type: reference
audience: "platform engineers, operators"
tags: [storage, vsphere, csi, services]
---
# vSphere CSI Driver

**Purpose:** For platform engineers, operators, documents vSphere CSI service configuration, storage classes, and catalog rendering.

> **Evidence:** `internal/config/services/vsphere_csi.go`, `internal/config/v2/defaults.go`, `internal/config/v2/config.go`, and `internal/gitops/render_catalog.go`.

## Configuration

The generated default is disabled in `vmware-system-csi`. `VSphereCSIConfig` adds `storage_classes` to `BaseConfig`.

```yaml
opencenter:
  services:
    vsphere-csi:
      enabled: false
      namespace: vmware-system-csi
      storage_classes:
        - name:
          datastore_url:
          reclaim_policy: Retain
          volume_binding_mode: Immediate
          allow_expansion: true
      adoption_mode: managed
      address_pool:
```

| Field | Default | Evidence |
|-------|---------|----------|
| `enabled` | `false` | `NewDefaultServiceConfig` |
| `namespace` | `vmware-system-csi` | `NewDefaultServiceConfig` |
| `storage_classes[].name`, `datastore_url` | required fields | `VSphereStorageClass` |
| `reclaim_policy` | `Retain` in schema tag | `VSphereStorageClass` |
| `volume_binding_mode` | `Immediate` in schema tag | `VSphereStorageClass` |
| `allow_expansion` | true in schema tag | `VSphereStorageClass` |
| `secrets.vsphere_csi.*` | fields declared | `VSphereCsiSecrets` |

## Rendering

The catalog entry uses namespace `vmware-system-csi`, base path `applications/base/services/vsphere-csi`, and an override-values renderer. No explicit service descriptor is present.

## Commands

```bash
opencenter cluster service enable vsphere-csi
opencenter cluster service disable vsphere-csi
opencenter cluster service options vsphere-csi
```

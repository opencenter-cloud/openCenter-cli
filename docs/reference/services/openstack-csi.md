---
id: service-openstack-csi
title: "OpenStack Cinder CSI"
sidebar_label: OpenStack CSI
description: CSI driver for OpenStack Cinder block storage volumes.
doc_type: reference
audience: "platform engineers, storage administrators"
tags: [openstack, storage, csi]
---

> **Purpose:** For platform engineers deploying on OpenStack, documents the Cinder CSI driver for dynamic volume provisioning and management.

## Overview

The OpenStack Cinder CSI driver enables dynamic provisioning of persistent volumes backed by OpenStack Cinder block storage. It supports volume snapshots, online volume expansion, and multi-attach for shared storage workloads. The driver uses infrastructure-level OpenStack credentials and is available only for clusters using the OpenStack provider.

## Configuration

```yaml
opencenter:
  services:
    openstack-csi:
      enabled: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable OpenStack Cinder CSI (OpenStack provider only) |

### Secrets

None (uses infrastructure-level OpenStack credentials).

## Dependencies

| Service | Required | Notes |
|---------|----------|-------|
| openstack-ccm | Yes | Provides cloud provider integration required by Cinder CSI |

## Provider Availability

| Provider | Available |
|----------|-----------|
| OpenStack | ✓ |
| VMware | ✗ |
| Baremetal | ✗ |
| Kind | ✗ |

## CLI Commands

```bash
opencenter cluster service enable openstack-csi
opencenter cluster service disable openstack-csi
opencenter cluster service status openstack-csi
opencenter cluster service options openstack-csi
```

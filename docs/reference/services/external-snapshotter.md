---
id: service-external-snapshotter
title: "External Snapshotter"
sidebar_label: External Snapshotter
description: CSI volume snapshot controller and CRDs for Kubernetes.
doc_type: reference
audience: "platform engineers, storage administrators"
tags: [storage, csi, snapshots]
---

> **Purpose:** For platform engineers, documents the external snapshotter that provides VolumeSnapshot CRDs and the snapshot controller.

## Overview

The external snapshotter installs the Kubernetes VolumeSnapshot CRDs, the snapshot controller, and a validating webhook. It enables CSI drivers to create and manage volume snapshots through the standard Kubernetes VolumeSnapshot API. This service is a prerequisite for any CSI driver that supports snapshot operations.

## Configuration

```yaml
opencenter:
  services:
    external-snapshotter:
      enabled: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable external snapshotter |

### Secrets

None.

## Dependencies

None.

## CLI Commands

```bash
opencenter cluster service enable external-snapshotter
opencenter cluster service disable external-snapshotter
opencenter cluster service status external-snapshotter
opencenter cluster service options external-snapshotter
```

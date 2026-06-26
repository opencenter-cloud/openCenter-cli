---
id: service-openstack-ccm
title: "OpenStack Cloud Controller Manager"
sidebar_label: OpenStack CCM
description: Cloud Controller Manager for OpenStack-provisioned clusters.
doc_type: reference
audience: "platform engineers, operators"
tags: [openstack, cloud-controller, networking]
---

> **Purpose:** For platform engineers deploying on OpenStack, documents the Cloud Controller Manager that integrates Kubernetes with OpenStack infrastructure.

## Overview

The OpenStack Cloud Controller Manager (CCM) integrates Kubernetes with the OpenStack infrastructure layer, enabling automatic load balancer provisioning via Octavia, floating IP management, and node metadata enrichment. It uses the same OpenStack credentials configured in the infrastructure section—no additional service-specific secrets are required. This service is available only for clusters using the OpenStack provider.

## Configuration

```yaml
opencenter:
  services:
    openstack-ccm:
      enabled: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable OpenStack CCM (OpenStack provider only) |

The CCM uses credentials from `opencenter.infrastructure.cloud.openstack.*` (auth_url, region, application credentials).

### Secrets

None (uses infrastructure-level OpenStack credentials).

## Dependencies

None.

## Provider Availability

| Provider | Available |
|----------|-----------|
| OpenStack | ✓ |
| VMware | ✗ |
| Baremetal | ✗ |
| Kind | ✗ |

## CLI Commands

```bash
opencenter cluster service enable openstack-ccm
opencenter cluster service disable openstack-ccm
opencenter cluster service status openstack-ccm
opencenter cluster service options openstack-ccm
```

---
id: service-alert-proxy
title: "Alert Proxy"
sidebar_label: Alert Proxy
description: Alert forwarding proxy for routing Alertmanager notifications to external systems.
doc_type: reference
audience: "platform engineers, operators"
tags: [alerting, monitoring, proxy]
---

> **Purpose:** For platform engineers, documents the alert proxy service for forwarding Alertmanager alerts to external managed services.

## Overview

The alert proxy provides a forwarding layer between the in-cluster Alertmanager and external alert management systems. It translates Alertmanager webhook payloads into the format required by the destination service, adding authentication and routing metadata. This is a managed service component typically used in environments with centralized alert aggregation.

## Configuration

```yaml
opencenter:
  services:
    alert-proxy:
      enabled: true
      alert_manager_base_url: http://alertmanager.monitoring.svc:9093
      http_route_fqdn: alerts.example.com
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable alert proxy |
| `alert_manager_base_url` | string | — | Alertmanager service URL |
| `http_route_fqdn` | string | — | External FQDN for the alert proxy HTTP route |

### Secrets

```yaml
secrets:
  alert_proxy:
    core_device_id: <device-id>
    account_service_token: <token>
    core_account_number: <account-number>
```

| Secret | Description |
|--------|-------------|
| `secrets.alert_proxy.core_device_id` | Device identifier for the external alert system |
| `secrets.alert_proxy.account_service_token` | Authentication token for the alert service |
| `secrets.alert_proxy.core_account_number` | Account number for alert routing |

## Dependencies

None.

## CLI Commands

```bash
opencenter cluster service enable alert-proxy
opencenter cluster service disable alert-proxy
opencenter cluster service status alert-proxy
opencenter cluster service options alert-proxy
```

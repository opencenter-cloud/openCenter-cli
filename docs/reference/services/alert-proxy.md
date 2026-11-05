---
last_updated: 2026-09-25
id: service-alert-proxy
title: "Alert Proxy"
sidebar_label: Alert Proxy
description: Managed alert-proxy configuration, secrets model, and descriptor conditions.
doc_type: reference
audience: "platform engineers, operators"
tags: [alerting, monitoring, managed-service, services]
---
# Alert Proxy

**Purpose:** For platform engineers, operators, documents managed alert-proxy configuration, secrets model, and descriptor conditions.

> **Evidence:** `internal/config/services/alert_proxy.go`, `internal/config/v2/defaults.go`, `internal/services/plugins/registry.go`, `internal/services/plugins/default_services.go`, `internal/config/v2/config.go`, and `internal/services/descriptors/data/service-alert-proxy.yaml`.

## Configuration

The descriptor names this entry `managed_service: alert-proxy`. The generated managed-service map includes it disabled, with a generated `http_route_fqdn` of `alerts.<cluster FQDN>`.

```yaml
opencenter:
  managed_services:
    alert-proxy:
      enabled: false
      namespace:
      alert_manager_base_url:
      http_route_fqdn:
      adoption_mode: managed
      address_pool:
```

`AlertProxyConfig` adds `alert_manager_base_url` and `http_route_fqdn` to `BaseConfig`. `AlertProxySecrets` declares `core_device_id`, `account_service_token`, and `core_account_number`.

## Runtime and descriptor facts

The plugin registry records `kube-prometheus-stack` as an Alert Proxy dependency. The plugin status reports the two config URLs; its validator and renderer are no-op implementations after type dispatch.

`service-alert-proxy.yaml` owns root `managed-services/alert-proxy`, lists source and Flux templates, and aggregates into `managed-services-fluxcd-aggregate` and `managed-services-sources-aggregate`. The repository does not declare conditional files for this descriptor.

## Commands

```bash
opencenter cluster service enable alert-proxy --managed
opencenter cluster service disable alert-proxy --managed
opencenter cluster service options alert-proxy --managed
```

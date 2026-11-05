---
last_updated: 2026-09-25
id: service-kube-prometheus-stack
title: "kube-prometheus-stack"
sidebar_label: Prometheus Stack
description: Prometheus stack service configuration and observability catalog behavior.
doc_type: reference
audience: "platform engineers, operators"
tags: [prometheus, grafana, alertmanager, observability, services]
---
# kube-prometheus-stack

**Purpose:** For platform engineers, operators, documents Prometheus stack service configuration and observability catalog behavior.

> **Evidence:** `internal/config/services/prometheus_stack.go`, `internal/config/v2/defaults.go`, `internal/services/plugins/prometheus_stack.go`, and `internal/gitops/render_catalog.go`.

## Configuration

The generated default is enabled in `observability`. `PrometheusStackConfig` embeds `BaseConfig`.

| Field group | Fields |
|-------------|--------|
| Release and endpoints | `release_name`, `hostname`, `grafana_hostname`, `prometheus_hostname`, `alertmanager_hostname` |
| Persistent storage | `grafana_volume_size`, `grafana_storage_class`, `prometheus_volume_size`, `prometheus_storage_class`, `alertmanager_volume_size`, `alertmanager_storage_class` |
| Alerting | `webhook_url` |

`release_name` defaults to `kube-prometheus-stack`; an empty namespace resolves to `observability`. The generated default explicitly sets both values. `hostname` is retained as the Grafana hostname field. Volume sizes are validated as non-negative by the plugin. Release names are limited by the runtime validator to 26 characters and lowercase Kubernetes/Helm naming rules; the namespace is validated as a DNS-1123 label.

`secrets.grafana.admin_user` and `secrets.grafana.admin_password` are declared by `GrafanaSecrets`.

## Rendering

The catalog uses source `opencenter-observability`, adds `observability-namespace` and `kube-prometheus-stack-override` stages, and records override dependencies on `sources` and `envoy-gateway-api-base`. It names generated HTTPRoute files for Prometheus, Alertmanager, and Grafana. No explicit service descriptor is present.

## Commands

```bash
opencenter cluster service enable kube-prometheus-stack
opencenter cluster service disable kube-prometheus-stack
opencenter cluster service options kube-prometheus-stack
```

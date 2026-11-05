---
last_updated: 2026-09-25
id: service-opentelemetry-kube-stack
title: "OpenTelemetry Kube Stack"
sidebar_label: OpenTelemetry
description: OpenTelemetry service configuration and observability catalog ownership.
doc_type: reference
audience: "platform engineers, operators"
tags: [observability, opentelemetry, services]
---
# OpenTelemetry Kube Stack

**Purpose:** For platform engineers, operators, documents OpenTelemetry service configuration and observability catalog ownership.

> **Evidence:** `internal/config/services/opentelemetry.go`, `internal/config/v2/defaults.go`, and `internal/gitops/render_catalog.go`.

## Configuration

The generated default is disabled in `observability`. `OpenTelemetryConfig` embeds `BaseConfig` and adds `collector_mode`, `collector_replicas`, `exporters`, and `processors`.

| Field | Type-level default/evidence |
|-------|-----------------------------|
| `collector_mode` | `deployment` in schema tag |
| `collector_replicas` | `1` in schema tag |
| `exporters[].name`, `type`, `endpoint` | required fields; types include `otlp`, `prometheus`, `jaeger` |
| `exporters[].headers` | optional string map |
| `processors` | string list |

`defaults.go` materializes only `enabled: false` and `namespace: observability`. Common `BaseConfig` fields remain available.

## Rendering

The catalog uses observability base path `applications/base/services/observability/opentelemetry-kube-stack` and static override values. No explicit service descriptor is present.

## Commands

```bash
opencenter cluster service enable opentelemetry-kube-stack
opencenter cluster service disable opentelemetry-kube-stack
opencenter cluster service options opentelemetry-kube-stack
```

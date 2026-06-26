---
id: service-opentelemetry-kube-stack
title: "OpenTelemetry Kube Stack"
sidebar_label: OpenTelemetry
description: OpenTelemetry collectors for trace, metric, and log pipelines.
doc_type: reference
audience: "platform engineers, operators"
tags: [observability, opentelemetry, tracing]
---

> **Purpose:** For platform engineers, documents the OpenTelemetry collector stack configuration including deployment modes, exporters, and processors.

## Overview

The OpenTelemetry Kube Stack deploys OpenTelemetry collectors that receive, process, and export telemetry data (traces, metrics, logs) from cluster workloads. Collectors can run as Deployments, DaemonSets, or StatefulSets depending on the collection pattern required. Multiple exporters can be configured to send data to different backends simultaneously.

## Configuration

```yaml
opencenter:
  services:
    opentelemetry-kube-stack:
      enabled: true
      collector_mode: deployment
      collector_replicas: 1
      exporters:
        - name: tempo
          type: otlp
          endpoint: tempo.observability.svc:4317
          headers: {}
        - name: prometheus
          type: prometheus
          endpoint: http://prometheus.monitoring.svc:9090/api/v1/write
      processors:
        - batch
        - memory_limiter
        - resource
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable OpenTelemetry collectors |
| `collector_mode` | string | `deployment` | Mode: `deployment`, `daemonset`, or `statefulset` |
| `collector_replicas` | int | `1` | Number of collector replicas |
| `exporters` | list | — | List of telemetry export destinations |
| `exporters[].name` | string | — | Exporter identifier |
| `exporters[].type` | string | — | Export protocol: `otlp`, `prometheus`, or `jaeger` |
| `exporters[].endpoint` | string | — | Destination endpoint URL |
| `exporters[].headers` | map | — | Additional HTTP headers |
| `processors` | list of strings | — | Processing pipeline stages |

### Secrets

None.

## Dependencies

| Service | Required | Notes |
|---------|----------|-------|
| tempo | Yes | Trace backend for OTLP export |

## CLI Commands

```bash
opencenter cluster service enable opentelemetry-kube-stack
opencenter cluster service disable opentelemetry-kube-stack
opencenter cluster service status opentelemetry-kube-stack
opencenter cluster service options opentelemetry-kube-stack
```

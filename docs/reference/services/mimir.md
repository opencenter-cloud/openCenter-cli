---
id: service-mimir
title: "Grafana Mimir"
sidebar_label: Mimir
description: Long-term metrics storage with S3 backend and Kafka-based ingestion.
doc_type: reference
audience: "platform engineers, operators"
tags: [monitoring, metrics, mimir, s3, kafka, long-term-storage]
---

> **Purpose:** For platform engineers and operators, documents the Grafana Mimir service configuration, covering S3 storage, Kafka ingestion, retention, and distributed architecture.

## Overview

Grafana Mimir provides horizontally scalable long-term metrics storage. It uses S3 for blocks storage, Kafka for ingest buffering, and a distributed architecture comprising distributor, ingester, store-gateway, compactor, querier, and query-frontend components. Default retention is 14 days with ingestion rate limits of 100k samples/s and 2M active series per user.

## Configuration

```yaml
opencenter:
  services:
    mimir:
      enabled: false                         # default: false
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable Grafana Mimir |

S3 credentials are sourced from global AWS application credentials:

```yaml
secrets:
  global:
    aws:
      application:
        access_key: "ENC[AES256_GCM,data:...,type:str]"
        secret_access_key: "ENC[AES256_GCM,data:...,type:str]"
```

## Dependencies

| Service | Reason |
|---------|--------|
| `kafka-cluster` | Provides ingest_storage for buffering ingested metrics |

## Architecture

| Component | Role |
|-----------|------|
| Distributor | Receives and validates incoming samples, distributes to ingesters |
| Ingester | Writes samples to long-term storage, serves recent data |
| Store-gateway | Serves historical blocks from S3 |
| Compactor | Merges and deduplicates blocks in S3 |
| Querier | Executes PromQL queries across ingesters and store-gateway |
| Query-frontend | Splits, caches, and parallelizes queries |

## Verification

```bash
# Check Mimir pods
kubectl get pods -n mimir

# Verify all components are running
kubectl get pods -n mimir -l app.kubernetes.io/name=mimir

# Check distributor readiness
kubectl get pods -n mimir -l app.kubernetes.io/component=distributor

# Verify ingester ring
kubectl port-forward -n mimir svc/mimir-distributor 8080:8080
curl http://localhost:8080/distributor/ring

# Check S3 connectivity (via compactor logs)
kubectl logs -n mimir -l app.kubernetes.io/component=compactor --tail=20

# Verify Kafka ingest_storage connection
kubectl logs -n mimir -l app.kubernetes.io/component=ingester --tail=20 | grep kafka
```

## CLI Commands

```bash
# Enable Mimir
opencenter cluster service enable mimir

# Disable Mimir
opencenter cluster service disable mimir

# View configuration options
opencenter cluster service options mimir

# Check service status
opencenter cluster service status
```

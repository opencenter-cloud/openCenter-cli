---
id: service-kafka-cluster
title: "Kafka Cluster"
sidebar_label: Kafka Cluster
description: Apache Kafka via Strimzi operator for event streaming.
doc_type: reference
audience: "platform engineers, operators"
tags: [kafka, streaming, strimzi]
---

> **Purpose:** For platform engineers, documents the Kafka cluster service deployed via the Strimzi operator for event streaming workloads.

## Overview

The Kafka cluster service deploys Apache Kafka using the Strimzi operator, providing managed Kafka brokers with ZooKeeper or KRaft consensus, topic management, and user authentication. It is required by Mimir for ingest storage when using the Kafka-based write path. The Strimzi operator is installed via OLM.

## Configuration

```yaml
opencenter:
  services:
    kafka-cluster:
      enabled: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable Kafka cluster via Strimzi |

### Secrets

None.

## Dependencies

| Service | Required | Notes |
|---------|----------|-------|
| olm | Yes | Installs the Strimzi operator |

## CLI Commands

```bash
opencenter cluster service enable kafka-cluster
opencenter cluster service disable kafka-cluster
opencenter cluster service status kafka-cluster
opencenter cluster service options kafka-cluster
```

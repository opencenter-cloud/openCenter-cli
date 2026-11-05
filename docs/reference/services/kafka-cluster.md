---
last_updated: 2026-09-25
id: service-kafka-cluster
title: "Kafka Cluster"
sidebar_label: Kafka Cluster
description: Kafka cluster service configuration and explicit descriptor templates.
doc_type: reference
audience: "platform engineers, operators"
tags: [kafka, strimzi, services]
---
# Kafka Cluster

**Purpose:** For platform engineers, operators, documents kafka cluster service configuration and explicit descriptor templates.

> **Evidence:** `internal/config/services/default_services.go`, `internal/config/v2/defaults.go`, and `internal/services/descriptors/data/service-kafka-cluster.yaml`.

## Configuration

Kafka Cluster uses `DefaultServiceConfig`; the generated default is disabled in `kafka-system`.

```yaml
opencenter:
  services:
    kafka-cluster:
      enabled: false
      namespace: kafka-system
      adoption_mode: managed
      address_pool:
```

`defaults.go` comments that the Kafka Kustomization and Flux templates hard-code `kafka-system`; the configured namespace therefore matches the deployment location in the current templates.

## Descriptor rendering

`service-kafka-cluster.yaml` aggregates into `services-fluxcd-aggregate` and `services-sources-aggregate`. It lists the persistent Kafka resource, Kustomization, Strimzi operator source, and Flux templates. No conditional file is declared.

## Commands

```bash
opencenter cluster service enable kafka-cluster
opencenter cluster service disable kafka-cluster
opencenter cluster service options kafka-cluster
```

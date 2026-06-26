---
id: service-tempo
title: "Tempo"
sidebar_label: Tempo
description: Distributed tracing backend with S3 and Swift storage backends.
doc_type: reference
audience: "operators, platform engineers"
tags: [tempo, tracing, observability, s3, swift]
---

# Tempo

> **Purpose:** For operators and platform engineers, documents Tempo configuration fields, storage backends, and secrets.

## Overview

Tempo is the distributed tracing backend for openCenter clusters. It stores and queries trace data from instrumented applications. Uses the same dual-backend storage model as Loki.

## Configuration

```yaml
opencenter:
  services:
    tempo:
      enabled: true
      storage_type: s3             # "s3" or "swift" (default: swift on OpenStack, s3 on AWS)
      bucket_name: ""              # defaults to "<cluster_name>-tempo"
      volume_size: 50              # persistent volume size in GB
      storage_class: ""            # PVC storage class

      # S3 backend fields
      s3_endpoint: ""              # S3-compatible endpoint URL
      s3_region: ""                # defaults to cluster region
      s3_force_path_style: false
      s3_insecure: false

      # Swift backend fields
      swift_auth_url: ""           # Keystone V3 URL
      swift_region: ""
      swift_auth_version: 3
      swift_application_credential_id: ""
      swift_container_name: ""     # defaults to bucket_name
      swift_user_domain_name: ""
      swift_domain_name: ""
```

## Secrets

### Swift Backend (`storage_type: swift`)

```yaml
secrets:
  tempo:
    swift_application_credential_secret: ""  # required
```

### S3 Backend (`storage_type: s3`)

```yaml
secrets:
  tempo:
    access_key: ""       # required (unless global fallback set)
    secret_key: ""       # required (unless global fallback set)
```

### Credential Fallback

S3 credentials resolve in order:

1. `secrets.tempo.access_key` / `secret_key`
2. `secrets.global.aws.application.access_key` / `secret_access_key`
3. `secrets.global.aws.infrastructure.access_key` / `secret_access_key`

## Dependencies

None. Integrates with:
- **kube-prometheus-stack** — Grafana Tempo datasource
- **opentelemetry-kube-stack** — trace export target

## Storage Backends

Identical pattern to Loki. See [Loki storage backends](loki.md#storage-backends) for details on Swift vs S3 configuration.

## Verification

```bash
# Check Tempo pods
kubectl get pods -n tempo-system

# Verify traces in Grafana
# Navigate to Explore > Tempo datasource > Search by trace ID
```

## CLI Commands

```bash
opencenter cluster service enable tempo
opencenter cluster service disable tempo
opencenter cluster service options tempo
opencenter cluster set my-cluster opencenter.services.tempo.storage_type=s3
```

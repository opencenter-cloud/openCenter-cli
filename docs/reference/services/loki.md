---
id: service-loki
title: "Loki"
sidebar_label: Loki
description: Log aggregation service with S3 and Swift storage backends.
doc_type: reference
audience: "operators, platform engineers"
tags: [loki, logging, observability, s3, swift]
---

# Loki

> **Purpose:** For operators and platform engineers, documents Loki configuration fields, storage backends, secrets, and credential fallback behavior.

## Overview

Loki is the log aggregation backend for openCenter clusters. It collects, indexes, and queries logs from all cluster workloads. Deployed as a scalable microservices architecture (write/read/backend) with Memcached caching.

## Configuration

```yaml
opencenter:
  services:
    loki:
      enabled: true
      storage_type: swift          # "s3" or "swift" (default: swift on OpenStack, s3 on AWS)
      bucket_name: ""              # defaults to "<cluster_name>-loki"
      volume_size: 20              # persistent volume size in GB
      storage_class: ""            # PVC storage class (defaults to cluster default)

      # Swift backend fields
      swift_auth_url: ""           # Keystone V3 URL (must end in /v3)
      swift_region: ""             # defaults to cluster region
      swift_auth_version: 3        # authentication version
      swift_application_credential_id: ""  # app credential UUID
      swift_container_name: ""     # defaults to bucket_name
      swift_user_domain_name: ""   # Swift user domain
      swift_domain_name: ""        # Swift domain

      # S3 backend fields
      s3_endpoint: ""              # S3-compatible endpoint URL
      s3_region: ""                # defaults to cluster region
      s3_force_path_style: false   # force path-style addressing
      s3_insecure: false           # allow HTTP (not HTTPS)
```

## Secrets

Secrets depend on the chosen `storage_type`.

### Swift Backend (`storage_type: swift`)

```yaml
secrets:
  loki:
    swift_application_credential_secret: ""  # required
```

The `swift_application_credential_id` is a non-secret config field (UUID) stored under `opencenter.services.loki`. The guided configuration flow copies it from `opencenter.infrastructure.cloud.openstack.application_credential_id`.

### S3 Backend (`storage_type: s3`)

```yaml
secrets:
  loki:
    s3_access_key_id: ""       # required (unless global fallback set)
    s3_secret_access_key: ""   # required (unless global fallback set)
```

### Credential Fallback

S3 credentials resolve in order:

1. `secrets.loki.s3_access_key_id` / `s3_secret_access_key`
2. `secrets.global.aws.application.access_key` / `secret_access_key`
3. `secrets.global.aws.infrastructure.access_key` / `secret_access_key`

Swift credential fallback:

1. `secrets.loki.swift_application_credential_secret`
2. `secrets.service_secrets.loki.swift_application_credential_secret`
3. `secrets.service_secrets.loki.swift_password` (legacy)

## Dependencies

None. Loki operates independently but integrates with:
- **kube-prometheus-stack** — ServiceMonitor for Loki metrics
- **Grafana** — log query datasource

## Storage Backends

### Swift (OpenStack)

Default on OpenStack infrastructure. Uses the same application credential as infrastructure provisioning but stored separately for SOPS encryption isolation.

The rendered Helm values configure Loki's `storage.swift` block with auth_url, region, application_credential_id, application_credential_secret, and container_name.

### S3 (AWS/RadosGW)

Default on AWS infrastructure. Compatible with any S3-compliant endpoint including OpenStack RadosGW.

The rendered Helm values configure Loki's `storage.s3` block with endpoint, region, accessKeyId, secretAccessKey, and path style settings.

## Architecture

Loki deploys in microservices mode:
- **write** — 3 replicas, 100Gi PVC
- **read** — 3 replicas, 50Gi PVC
- **backend** — 3 replicas, 50Gi PVC
- **gateway** — 2 replicas (no PVC)
- **chunksCache** — Memcached
- **resultsCache** — Memcached

Multi-tenancy is enabled (`auth_enabled: true`). Schema uses TSDB with v13 from 2025-01-01.

## Verification

```bash
# Check Loki pods
kubectl get pods -n loki-system

# Check Loki is receiving logs
kubectl logs -n loki-system -l app.kubernetes.io/component=write --tail=10

# Query via Grafana
# Navigate to Explore > Loki datasource > Run query: {namespace="default"}
```

## CLI Commands

```bash
# Enable Loki
opencenter cluster service enable loki

# Disable Loki
opencenter cluster service disable loki

# View configuration options
opencenter cluster service options loki

# Set storage type
opencenter cluster set my-cluster opencenter.services.loki.storage_type=s3
```

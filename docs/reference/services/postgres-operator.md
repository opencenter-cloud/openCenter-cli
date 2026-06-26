---
id: service-postgres-operator
title: "PostgreSQL Operator"
sidebar_label: Postgres Operator
description: Zalando PostgreSQL operator for automated database cluster management.
doc_type: reference
audience: "platform engineers, database administrators"
tags: [database, postgresql, operator]
---

> **Purpose:** For platform engineers, documents the Zalando PostgreSQL operator for provisioning and managing PostgreSQL clusters on Kubernetes.

## Overview

The Zalando PostgreSQL operator automates the deployment and management of highly available PostgreSQL clusters on Kubernetes. It handles automated failover, connection pooling via PgBouncer, continuous backups, and rolling upgrades. The operator creates PostgreSQL clusters from `postgresql` custom resources and is required by services that need managed PostgreSQL databases (e.g., Keycloak).

## Configuration

```yaml
opencenter:
  services:
    postgres-operator:
      enabled: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable Zalando PostgreSQL operator |

### Secrets

None.

## Dependencies

None.

## Required By

| Service | Notes |
|---------|-------|
| keycloak | Uses operator-managed PostgreSQL for identity data |

## CLI Commands

```bash
opencenter cluster service enable postgres-operator
opencenter cluster service disable postgres-operator
opencenter cluster service status postgres-operator
opencenter cluster service options postgres-operator
```

---
last_updated: 2026-09-25
id: service-postgres-operator
title: "PostgreSQL Operator"
sidebar_label: Postgres Operator
description: PostgreSQL operator service configuration and catalog ownership.
doc_type: reference
audience: "platform engineers, database administrators"
tags: [database, postgresql, operator, services]
---
# PostgreSQL Operator

**Purpose:** For platform engineers, database administrators, documents PostgreSQL operator service configuration and catalog ownership.

> **Evidence:** `internal/config/services/default_services.go`, `internal/config/v2/defaults.go`, `internal/services/plugins/registry.go`, and `internal/gitops/render_catalog.go`.

## Configuration

Postgres Operator uses `DefaultServiceConfig`; the generated default is enabled in `postgres-operator`.

```yaml
opencenter:
  services:
    postgres-operator:
      enabled: true
      namespace: postgres-operator
      adoption_mode: managed
      address_pool:
```

## Runtime and rendering

The plugin registry records no dependency for Postgres Operator. The catalog sets base path `applications/base/services/postgres-operator`, enables override values containing `configGeneral.workers: 2`, and emits a source. No explicit service descriptor is present.

## Commands

```bash
opencenter cluster service enable postgres-operator
opencenter cluster service disable postgres-operator
opencenter cluster service options postgres-operator
```

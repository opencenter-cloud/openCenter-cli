---
last_updated: 2026-11-02
id: service-keycloak
title: "Keycloak"
sidebar_label: Keycloak
description: Keycloak service configuration, runtime validation, dependencies, and descriptor conditions.
doc_type: reference
audience: "platform engineers, operators"
tags: [keycloak, identity, services]
---
# Keycloak

**Purpose:** For platform engineers, operators, documents keycloak service configuration, runtime validation, dependencies, and descriptor conditions.

> **Evidence:** `internal/config/services/keycloak.go`, `internal/config/v2/defaults.go`, `internal/services/plugins/keycloak.go`, `internal/config/services/dependency_validator.go`, `internal/config/services/deprecations.go`, `internal/services/plugins/registry.go`, and `internal/services/descriptors/data/service-keycloak.yaml`.

## Configuration

The generated default is enabled in `keycloak`; its generated hostname is `auth.<cluster FQDN>`. `KeycloakConfig` embeds `BaseConfig` and contains access, issuer, realm, theming, runtime, resource, scaling, database, observability, TLS, backup, and SMTP fields.

```yaml
opencenter:
  services:
    keycloak:
      enabled: true
      namespace: keycloak
      hostname:
      issuer_ref:
      dns_credential:
      theme_enabled: true
      tls_secret_name:
      internal_tls_enabled: false
```

| Group | Fields |
|-------|--------|
| Access | `hostname`, `frontend_url` |
| Realm | `realm`, `client_id`, `realm_import_enabled`, `realm_groups`, `realm_admin_email` |
| Runtime | `start_optimized`, `cache_enabled`, `cache_stack` |
| Resources | `resource_requests_cpu`, `resource_requests_memory`, `resource_limits_cpu`, `resource_limits_memory` |
| Scaling | `instances`, `min_replicas`, `max_replicas` |
| Database | `database_host`, `database_port`, `database_name`, `database_user`, `db_pool_min_size`, `db_pool_initial_size`, `db_pool_max_size` |
| Observability | `metrics_enabled`, `event_metrics_enabled`, `health_enabled`, `log_level`, `log_format` |
| TLS | `tls_secret_name`, `internal_tls_enabled`, `issuer_ref`, `dns_credential` |
| Theming | `theme_enabled` |
| Backup | `backup_enabled`, `backup_schedule` |
| SMTP | `smtp_host`, `smtp_port`, `smtp_from`, `smtp_starttls` |

The config type's documented defaults include `client_id: opencenter`, `realm_import_enabled: true`, `theme_enabled: true`, `start_optimized: false`, `cache_enabled: true`, `cache_stack: kubernetes`, resource values, scaling values, `database_port: 5432`, pool values, observability booleans, log values, `tls_secret_name: keycloak-tls-secret`, backup values, and `smtp_port: 587`/`smtp_starttls: true`. `internal_tls_enabled` has no documented default and is false unless configured. `defaults.go` additionally materializes `resource_requests_cpu: 500m`, `resource_limits_cpu: 2`, and `instances: 3` in the generated default. Other fields are empty unless configured.

`internal_tls_enabled` puts TLS on the Keycloak pods themselves; `tls_secret_name` is only consumed when it is set, because both the `Certificate` and the Keycloak CR `tlsSecret` are gated on it. TLS at the edge is the Gateway's default-cluster certificate, so the field stays false in a Gateway-terminated cluster. The former `tls_enabled` knob was never read by any renderer and is registered as deprecated in `deprecations.go`; the public serializer strips it from a saved configuration, so use `internal_tls_enabled` instead.

`issuer_ref` names the ClusterIssuer that mints this service's certificate; empty means the issuer the Gateway listener in front of Keycloak uses, which keeps the listener and the leaf certificate in agreement. `dns_credential` pins the DNS credential used when that certificate needs a DNS-01 challenge, overriding `services.cert-manager.default_credential`. `theme_enabled` controls the OpenCenter login theme and defaults to true when omitted; the config type is a `*bool` so an explicit `false` is distinguishable from unset.

## Runtime validation and dependencies

The service plugin validates HTTP(S) `frontend_url`, optimized startup with at least two instances, `min_replicas <= max_replicas`, and `db_pool_min_size <= db_pool_max_size`. The registered validation engine also validates log level and format and allows cache stacks `kubernetes` and `ispn`.

The config dependency graph records `olm` and `postgres-operator` for Keycloak. The plugin registry separately registers `cert-manager` as a plugin dependency. These are distinct repository registries; this page does not merge their enforcement paths.

`KeycloakSecrets` declares `client_secret` and `admin_password`, but `secrets_validator.go` explicitly has no Keycloak mapping. Treat those declarations as a secrets model fact, not as a documented required-secret rule.

## Descriptor rendering

`service-keycloak.yaml` owns `services/keycloak` and aggregates into the services Flux and sources aggregates. The backup CronJob template is included when `opencenter.services.keycloak.backup_enabled` is true; the HPA template is included when `max_replicas` exists. The descriptor also lists source/config and Flux templates.

## Commands

```bash
opencenter cluster service enable keycloak
opencenter cluster service disable keycloak
opencenter cluster service options keycloak
```

---
id: service-keycloak
title: "Keycloak"
sidebar_label: Keycloak
description: Identity and access management service providing OIDC authentication, realm management, and user federation.
doc_type: reference
audience: "platform engineers, operators"
tags: [keycloak, identity, oidc, authentication, services]
---

> **Purpose:** For platform engineers and operators, documents the complete configuration surface, secrets, dependencies, and verification steps for the Keycloak service.

## Overview

Keycloak provides identity and access management for the cluster, handling OIDC authentication, realm management, user federation, and session caching. It runs as a highly-available deployment backed by PostgreSQL with Kubernetes-native distributed caching and optional SMTP integration for email flows.

## Configuration

```yaml
opencenter:
  services:
    keycloak:
      enabled: true
      hostname:                          # Required. FQDN for Keycloak (e.g., auth.example.com)
      frontend_url:                      # Required. Public-facing URL (e.g., https://auth.example.com)
      realm: opencenter                  # Realm name (default: opencenter)
      client_id: opencenter              # OIDC client identifier (default: opencenter)
      realm_import_enabled: true         # Import realm configuration on startup (default: true)
      realm_groups: []                   # List of realm groups to create
      realm_admin_email:                 # Admin user email address
      start_optimized: true              # Use optimized startup mode (default: true)
      cache_enabled: true                # Enable distributed caching (default: true)
      cache_stack: kubernetes            # Cache discovery mechanism (default: kubernetes)
      resource_requests_cpu: 2           # CPU request in cores (default: 2)
      resource_requests_memory: 1250M    # Memory request (default: 1250M)
      resource_limits_cpu: 6             # CPU limit in cores (default: 6)
      resource_limits_memory: 2250M      # Memory limit (default: 2250M)
      instances: 3                       # Number of replicas (default: 3)
      min_replicas: 3                    # HPA minimum replicas (default: 3)
      max_replicas: 10                   # HPA maximum replicas (default: 10)
      database_host:                     # PostgreSQL host (provided by postgres-operator)
      database_port: 5432                # PostgreSQL port (default: 5432)
      database_name:                     # PostgreSQL database name
      database_user:                     # PostgreSQL user
      db_pool_min_size: 30               # Minimum connection pool size (default: 30)
      db_pool_initial_size: 30           # Initial connection pool size (default: 30)
      db_pool_max_size: 30               # Maximum connection pool size (default: 30)
      metrics_enabled: true              # Expose Prometheus metrics (default: true)
      event_metrics_enabled: true        # Expose event-based metrics (default: true)
      health_enabled: true               # Enable health endpoints (default: true)
      log_level: INFO                    # Log level: INFO | DEBUG | WARN | ERROR (default: INFO)
      log_format: json                   # Log format: default | json (default: json)
      tls_secret_name: keycloak-tls-secret  # TLS certificate secret name (default: keycloak-tls-secret)
      tls_enabled: true                  # Enable TLS termination (default: true)
      backup_enabled: true               # Enable scheduled database backups (default: true)
      backup_schedule: "0 2 * * *"       # Backup cron schedule (default: 0 2 * * *)
      smtp_host:                         # SMTP server hostname
      smtp_port: 587                     # SMTP port (default: 587)
      smtp_from:                         # SMTP sender address
      smtp_starttls: true                # Enable STARTTLS (default: true)
```

### Validation Rules

- `start_optimized: true` requires `instances >= 2` (optimized mode needs clustering).
- `min_replicas` cannot exceed `max_replicas`.
- `db_pool_min_size` cannot exceed `db_pool_max_size`.

## Secrets

| Path | Description | Required |
|------|-------------|----------|
| `secrets.keycloak.admin_password` | Keycloak admin console password | Always |
| `secrets.keycloak.client_secret` | OIDC client secret | Conditional |

When `identity.oidc.source=internal`, the `client_secret` is auto-generated during bootstrap and stored in the encrypted secrets file. When `identity.oidc.source=external`, the operator must provide it explicitly.

## Dependencies

| Service | Reason |
|---------|--------|
| cert-manager | Issues TLS certificates for Keycloak ingress |
| gateway-api | Provides HTTP routing to Keycloak endpoints |
| postgres-operator | Provisions and manages the backing PostgreSQL database |

## Verification

```bash
# Check Keycloak pods are running
kubectl get pods -n keycloak -l app.kubernetes.io/name=keycloak

# Verify all replicas are ready
kubectl rollout status statefulset/keycloak -n keycloak

# Check health endpoint
kubectl exec -n keycloak keycloak-0 -- curl -s http://localhost:8080/health/ready

# Verify TLS certificate
kubectl get certificate -n keycloak

# Check HPA status
kubectl get hpa -n keycloak

# Verify database connectivity
kubectl logs -n keycloak -l app.kubernetes.io/name=keycloak --tail=20 | grep "Database"
```

## CLI Commands

```bash
# Enable Keycloak
opencenter cluster service enable keycloak

# Disable Keycloak
opencenter cluster service disable keycloak

# View service status
opencenter cluster service status keycloak

# Show configuration options
opencenter cluster service options keycloak

# Set a configuration value
opencenter cluster set <cluster> services.keycloak.instances=5
```

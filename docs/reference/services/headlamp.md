---
id: service-headlamp
title: "Headlamp Dashboard"
sidebar_label: Headlamp
description: Kubernetes dashboard with OIDC authentication and Flux GitOps plugin.
doc_type: reference
audience: "platform engineers, operators"
tags: [dashboard, ui, oidc, headlamp, gitops]
---

> **Purpose:** For platform engineers and operators, documents the Headlamp dashboard service configuration, covering OIDC integration, Flux plugin, and verification.

## Overview

Headlamp provides a web-based Kubernetes dashboard with OIDC authentication via Keycloak. It includes the headlamp-plugin-flux plugin for GitOps visibility into FluxCD resources. When `identity.oidc.source=internal`, the OIDC client secret placeholder is acceptable because the bootstrap process generates it automatically.

## Configuration

```yaml
opencenter:
  services:
    headlamp:
      enabled: true                          # default: true
      hostname: "headlamp.example.com"
      oidc_issuer_url: "https://keycloak.example.com/realms/master"
      oidc_client_id: "headlamp"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable Headlamp dashboard |
| `hostname` | string | — | Public hostname for the dashboard |
| `oidc_issuer_url` | string | — | OIDC issuer URL (Keycloak realm) |
| `oidc_client_id` | string | — | OIDC client ID registered in Keycloak |

## Secrets

Configured under `secrets.headlamp`:

```yaml
secrets:
  headlamp:
    oidc_client_secret: "ENC[AES256_GCM,data:...,type:str]"
```

| Field | Type | Description |
|-------|------|-------------|
| `oidc_client_secret` | string | OIDC client secret (SOPS encrypted). When `identity.oidc.source=internal`, a placeholder value is acceptable — bootstrap generates the real secret. |

## Dependencies

| Service | Reason |
|---------|--------|
| `keycloak` | Provides OIDC authentication |
| `gateway-api` | Required for ingress routing |

## Verification

```bash
# Check Headlamp pods
kubectl get pods -n headlamp

# Verify Headlamp service
kubectl get svc -n headlamp

# Check HTTPRoute for ingress
kubectl get httproutes -n headlamp

# Verify OIDC configuration
kubectl get secret -n headlamp headlamp-oidc -o jsonpath='{.data}'

# Access the dashboard
curl -sI https://headlamp.example.com
```

## CLI Commands

```bash
# Enable Headlamp
opencenter cluster service enable headlamp

# Disable Headlamp
opencenter cluster service disable headlamp

# View configuration options
opencenter cluster service options headlamp

# Check service status
opencenter cluster service status
```

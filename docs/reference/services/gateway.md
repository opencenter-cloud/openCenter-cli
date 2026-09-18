---
id: service-gateway
title: "Gateway (Envoy)"
sidebar_label: Gateway
description: Envoy-based Gateway API implementation for HTTP/HTTPS routing and TLS termination.
doc_type: reference
audience: "platform engineers, operators"
tags: [networking, gateway, envoy, routing, tls]
---

> **Purpose:** For platform engineers and operators, documents the Envoy Gateway service configuration, covering listeners, TLS termination, and routing.

## Overview

Gateway deploys an Envoy-based implementation of the Kubernetes Gateway API. It provides HTTP/HTTPS routing, TLS termination, load balancing, and support for multiple listeners. Each listener can bind to a specific hostname and port with optional TLS configuration.

## Configuration

```yaml
opencenter:
  services:
    gateway:
      enabled: true                          # default: true
      gateway_name: rmpk-gateway             # default: rmpk-gateway
      gateway_namespace: rackspace-system     # default: rackspace-system
      gateway_class: eg                       # default: eg
      default_issuer: ""                      # cert-manager ClusterIssuer name
      listeners:
        - name: https
          port: 443
          protocol: HTTPS                    # HTTP or HTTPS
          hostname: "*.example.com"
          tls_secret_name: wildcard-tls
        - name: http
          port: 80
          protocol: HTTP
          hostname: "*.example.com"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable Envoy Gateway |
| `gateway_name` | string | `rmpk-gateway` | Name of the Gateway resource |
| `gateway_namespace` | string | `rackspace-system` | Namespace for the Gateway |
| `gateway_class` | string | `eg` | GatewayClass to use |
| `default_issuer` | string | `""` | Default cert-manager ClusterIssuer for TLS |
| `listeners` | list | `[]` | Listener definitions |
| `listeners[].name` | string | — | Listener identifier |
| `listeners[].port` | int | — | Port number |
| `listeners[].protocol` | string | — | `HTTP` or `HTTPS` |
| `listeners[].hostname` | string | — | Hostname pattern |
| `listeners[].tls_secret_name` | string | — | TLS secret name (HTTPS only) |

## Bring-your-own TLS

By default the generated platform Gateway carries a `cert-manager.io/cluster-issuer`
annotation and every HTTPS listener references a cert-manager-managed leaf Secret
(`keycloak-tls`, `harbor-tls`, `longhorn-tls`, …). Operators who manage TLS
themselves — a wildcard certificate, or pre-existing per-listener Secrets — can
override this from cluster config instead of hand-editing the Gateway on-cluster
(which drifts back on the next reconcile).

Setting **any** bring-your-own option removes the `cert-manager.io/cluster-issuer`
annotation from the Gateway so cert-manager no longer manages those leaves.

```yaml
opencenter:
  services:
    gateway:
      enabled: true
      tls:
        # A single pre-existing Secret (e.g. a wildcard cert) for every HTTPS listener.
        wildcard_secret_name: star-rax-io-tls
        # Optional namespace of the referenced Secret(s).
        secret_namespace: rackspace-system
        # Override specific listeners by logical name; wins over the wildcard.
        per_listener_secrets:
          harbor: harbor-byo-tls
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `tls.wildcard_secret_name` | string | — | One pre-existing TLS Secret used by every HTTPS listener; drops the cert-manager annotation |
| `tls.secret_namespace` | string | — | Namespace of the pre-existing Secret(s); empty means same-namespace lookup |
| `tls.per_listener_secrets` | map | — | Override the TLS Secret for specific listeners by logical name (`keycloak`, `gitops`, `headlamp`, `prometheus`, `alertmanager`, `grafana`, `harbor`, `longhorn`) |

Precedence for each HTTPS listener: `per_listener_secrets[<listener>]`, then
`wildcard_secret_name`, then the built-in `<service>-tls` default. When no `tls`
block is configured the output is unchanged.

## Dependencies

| Service | Reason |
|---------|--------|
| `gateway-api` | Provides the Gateway API CRDs consumed by Envoy Gateway |

## Verification

```bash
# Check Envoy Gateway controller
kubectl get pods -n envoy-gateway-system

# Verify Gateway resource
kubectl get gateway -n rackspace-system rmpk-gateway

# Check Gateway status and listeners
kubectl describe gateway -n rackspace-system rmpk-gateway

# List HTTPRoutes
kubectl get httproutes --all-namespaces

# Verify GatewayClass
kubectl get gatewayclass eg
```

## CLI Commands

```bash
# Enable Gateway
opencenter cluster service enable gateway

# Disable Gateway
opencenter cluster service disable gateway

# View configuration options
opencenter cluster service options gateway

# Check service status
opencenter cluster service status
```

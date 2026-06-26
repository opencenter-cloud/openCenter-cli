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

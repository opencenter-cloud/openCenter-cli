---
id: service-kube-prometheus-stack
title: "kube-prometheus-stack"
sidebar_label: Prometheus Stack
description: Monitoring stack deploying Prometheus, Grafana, Alertmanager, node-exporter, and kube-state-metrics.
doc_type: reference
audience: "platform engineers, operators"
tags: [prometheus, grafana, alertmanager, monitoring, observability, services]
---

> **Purpose:** For platform engineers and operators, documents the complete configuration surface, secrets, dependencies, and verification steps for the kube-prometheus-stack service.

## Overview

kube-prometheus-stack deploys a complete monitoring pipeline: Prometheus for metrics collection and alerting rules, Grafana for dashboards, Alertmanager for notification routing, node-exporter for host metrics, and kube-state-metrics for Kubernetes object state. All components use persistent storage with configurable volume sizes and storage classes.

## Configuration

```yaml
opencenter:
  services:
    kube-prometheus-stack:
      enabled: true
      hostname:                          # Required. Grafana FQDN (e.g., grafana.example.com)
      grafana_volume_size: 10            # Grafana PVC size in GB (default: 10)
      grafana_storage_class: csi-cinder-sc-delete  # Grafana storage class (default: csi-cinder-sc-delete)
      prometheus_volume_size: 50         # Prometheus PVC size in GB (default: 50)
      prometheus_storage_class: csi-cinder-sc-delete  # Prometheus storage class (default: csi-cinder-sc-delete)
      alertmanager_volume_size: 10       # Alertmanager PVC size in GB (default: 10)
      alertmanager_storage_class: csi-cinder-sc-delete  # Alertmanager storage class (default: csi-cinder-sc-delete)
      webhook_url:                       # Alertmanager webhook receiver URL
```

## Secrets

| Path | Description | Required |
|------|-------------|----------|
| `secrets.grafana.admin_password` | Grafana admin user password | Always |

## Dependencies

None. kube-prometheus-stack has no service-level dependencies.

## Verification

```bash
# Check all monitoring pods
kubectl get pods -n monitoring

# Verify Prometheus is scraping targets
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090 &
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets | length'

# Check Grafana is running
kubectl rollout status deployment/kube-prometheus-stack-grafana -n monitoring

# Verify Alertmanager cluster
kubectl get pods -n monitoring -l app.kubernetes.io/name=alertmanager

# Check PVC binding
kubectl get pvc -n monitoring

# Verify node-exporter is running on all nodes
kubectl get daemonset -n monitoring -l app.kubernetes.io/name=node-exporter

# Check kube-state-metrics
kubectl get deployment -n monitoring -l app.kubernetes.io/name=kube-state-metrics
```

## CLI Commands

```bash
# Enable the monitoring stack
opencenter cluster service enable kube-prometheus-stack

# Disable the monitoring stack
opencenter cluster service disable kube-prometheus-stack

# View service status
opencenter cluster service status kube-prometheus-stack

# Show configuration options
opencenter cluster service options kube-prometheus-stack

# Set Prometheus volume size
opencenter cluster set <cluster> services.kube-prometheus-stack.prometheus_volume_size=100

# Set Grafana hostname
opencenter cluster set <cluster> services.kube-prometheus-stack.hostname=grafana.example.com
```

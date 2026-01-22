# Monitoring and Observability


## Table of Contents

- [Who This Is For](#who-this-is-for)
- [Prerequisites](#prerequisites)
- [Monitoring Stack Overview](#monitoring-stack-overview)
- [Enable Monitoring Services](#enable-monitoring-services)
- [Access Monitoring Interfaces](#access-monitoring-interfaces)
- [Configure Dashboards](#configure-dashboards)
- [Configure Alerting](#configure-alerting)
- [Log Aggregation with Loki](#log-aggregation-with-loki)
- [Monitoring Best Practices](#monitoring-best-practices)
- [Troubleshooting](#troubleshooting)
- [Integration with External Systems](#integration-with-external-systems)
- [Capacity Planning](#capacity-planning)
- [Related Documentation](#related-documentation)
- [Next Steps](#next-steps)
**doc_type: how-to**

Set up comprehensive monitoring and observability for openCenter-managed Kubernetes clusters using Prometheus, Grafana, Loki, and integrated alerting.

## Who This Is For

Operations teams and SREs responsible for monitoring cluster health, investigating incidents, and maintaining observability infrastructure. Use this guide to deploy and configure the monitoring stack, set up dashboards, and establish alerting rules.

## Prerequisites

- Running openCenter cluster with GitOps configured
- Access to cluster configuration file
- `kubectl` access to the cluster
- Understanding of Prometheus metrics and PromQL

## Monitoring Stack Overview

openCenter integrates a complete observability stack:

- **Prometheus** - Metrics collection and storage
- **Grafana** - Visualization and dashboards
- **Alertmanager** - Alert routing and notification
- **Loki** - Log aggregation and querying
- **Node Exporter** - Node-level metrics
- **kube-state-metrics** - Kubernetes object metrics

## Enable Monitoring Services

### Enable Prometheus Stack

Edit your cluster configuration to enable the kube-prometheus-stack service:

```yaml
opencenter:
  services:
    kube-prometheus-stack:
      enabled: true
      prometheus_volume_size: 50  # GB
      prometheus_storage_class: csi-cinder-sc-delete
      grafana_volume_size: 10  # GB
      grafana_storage_class: csi-cinder-sc-delete
      alertmanager_volume_size: 10  # GB
      alertmanager_storage_class: csi-cinder-sc-delete
      webhook_url: ""  # Optional: webhook for external alerting

secrets:
  grafana:
    admin_password: "your-secure-password"  # Required
```

Storage sizing recommendations:
- **Development**: Prometheus 20GB, Grafana 5GB, Alertmanager 5GB
- **Production**: Prometheus 100GB+, Grafana 20GB, Alertmanager 10GB
- **High-volume**: Prometheus 500GB+, consider remote storage

### Enable Log Aggregation

Enable Loki for centralized log collection:

```yaml
opencenter:
  services:
    loki:
      enabled: true
      volume_size: 20  # GB
      storage_class: csi-cinder-sc-delete
      bucket_name: my-cluster-loki
      swift_auth_url: https://keystone.api.region.rackspacecloud.com/v3/
      swift_region: REGION
      swift_domain_name: Default

secrets:
  loki:
    swift_password: "your-swift-password"  # For Swift backend
    # OR for S3 backend:
    # s3_access_key_id: "your-access-key"
    # s3_secret_access_key: "your-secret-key"
```

### Apply Configuration

Deploy the monitoring stack:

```bash
# Validate configuration
openCenter cluster validate my-cluster

# Apply changes through GitOps
cd ~/.config/openCenter/clusters/myorg/my-cluster
git add .
git commit -m "Enable monitoring stack"
git push

# Flux will reconcile within 15 minutes, or force sync:
flux reconcile kustomization flux-system
```

## Access Monitoring Interfaces

### Access Grafana Dashboard

Grafana provides visualization and dashboards for metrics and logs.

**Port-forward method** (development):

```bash
kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80
```

Access at `http://localhost:3000`
- Username: `admin`
- Password: Value from `secrets.grafana.admin_password`

**Ingress method** (production):

Configure HTTPRoute in your cluster configuration:

```yaml
opencenter:
  services:
    kube-prometheus-stack:
      enabled: true
      hostname: grafana.my-cluster.region.k8s.opencenter.cloud
```

Access at `https://grafana.my-cluster.region.k8s.opencenter.cloud`

### Access Prometheus UI

Prometheus provides raw metrics query interface.

```bash
kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090
```

Access at `http://localhost:9090`

### Access Alertmanager UI

Alertmanager manages alert routing and silencing.

```bash
kubectl port-forward -n monitoring svc/kube-prometheus-stack-alertmanager 9093:9093
```

Access at `http://localhost:9093`

## Configure Dashboards

### Pre-installed Dashboards

The kube-prometheus-stack includes dashboards for:

- **Kubernetes / Compute Resources / Cluster** - Overall cluster resource usage
- **Kubernetes / Compute Resources / Namespace** - Per-namespace resource usage
- **Kubernetes / Compute Resources / Node** - Per-node resource usage
- **Kubernetes / Compute Resources / Pod** - Per-pod resource usage
- **Node Exporter / Nodes** - Node-level system metrics
- **etcd** - etcd cluster health and performance
- **CoreDNS** - DNS query metrics

### Import Custom Dashboards

Import dashboards from Grafana.com:

1. Navigate to Grafana UI
2. Click **+** → **Import**
3. Enter dashboard ID or paste JSON
4. Select Prometheus data source
5. Click **Import**

Recommended dashboard IDs:
- **15757** - Kubernetes / Views / Global
- **15758** - Kubernetes / Views / Namespaces
- **15759** - Kubernetes / Views / Pods
- **15760** - Kubernetes / Views / Nodes
- **13770** - Kubernetes Cluster Monitoring (via Prometheus)

### Create Custom Dashboards

Create dashboards for application-specific metrics:

1. Click **+** → **Dashboard**
2. Add panel with PromQL query
3. Configure visualization type
4. Set thresholds and alerts
5. Save dashboard

Example PromQL queries:

```promql
# Pod CPU usage
sum(rate(container_cpu_usage_seconds_total{namespace="default"}[5m])) by (pod)

# Pod memory usage
sum(container_memory_working_set_bytes{namespace="default"}) by (pod)

# API server request rate
sum(rate(apiserver_request_total[5m])) by (verb, code)

# etcd leader changes
rate(etcd_server_leader_changes_seen_total[5m])
```

## Configure Alerting

### Configure Alert Receivers

Edit Alertmanager configuration to route alerts:

```yaml
# Create alertmanager-config.yaml
apiVersion: v1
kind: Secret
metadata:
  name: alertmanager-kube-prometheus-stack-alertmanager
  namespace: monitoring
stringData:
  alertmanager.yaml: |
    global:
      resolve_timeout: 5m
    
    route:
      group_by: ['alertname', 'cluster', 'service']
      group_wait: 10s
      group_interval: 10s
      repeat_interval: 12h
      receiver: 'default'
      routes:
        - match:
            severity: critical
          receiver: 'critical-alerts'
        - match:
            severity: warning
          receiver: 'warning-alerts'
    
    receivers:
      - name: 'default'
        webhook_configs:
          - url: 'http://alertmanager-webhook:8080/alerts'
      
      - name: 'critical-alerts'
        slack_configs:
          - api_url: 'https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK'
            channel: '#critical-alerts'
            title: 'Critical Alert: {{ .GroupLabels.alertname }}'
            text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'
        pagerduty_configs:
          - service_key: 'YOUR_PAGERDUTY_KEY'
      
      - name: 'warning-alerts'
        slack_configs:
          - api_url: 'https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK'
            channel: '#warning-alerts'
```

Apply the configuration:

```bash
kubectl apply -f alertmanager-config.yaml
```

### Define Custom Alert Rules

Create custom PrometheusRule resources:

```yaml
# custom-alerts.yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: custom-alerts
  namespace: monitoring
spec:
  groups:
    - name: custom.rules
      interval: 30s
      rules:
        - alert: HighPodMemory
          expr: |
            sum(container_memory_working_set_bytes{namespace="production"}) by (pod) 
            / sum(container_spec_memory_limit_bytes{namespace="production"}) by (pod) > 0.9
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "Pod {{ $labels.pod }} high memory usage"
            description: "Pod {{ $labels.pod }} is using {{ $value | humanizePercentage }} of memory limit"
        
        - alert: PodCrashLooping
          expr: rate(kube_pod_container_status_restarts_total[15m]) > 0
          for: 5m
          labels:
            severity: critical
          annotations:
            summary: "Pod {{ $labels.pod }} is crash looping"
            description: "Pod {{ $labels.pod }} has restarted {{ $value }} times in the last 15 minutes"
        
        - alert: NodeDiskPressure
          expr: kube_node_status_condition{condition="DiskPressure",status="true"} == 1
          for: 5m
          labels:
            severity: critical
          annotations:
            summary: "Node {{ $labels.node }} has disk pressure"
            description: "Node {{ $labels.node }} is experiencing disk pressure"
```

Apply custom alerts:

```bash
kubectl apply -f custom-alerts.yaml
```

### Test Alerting

Trigger a test alert:

```bash
# Create a pod that will crash
kubectl run test-alert --image=busybox --restart=Never -- /bin/sh -c "exit 1"

# Check alert in Prometheus
# Navigate to http://localhost:9090/alerts

# Check alert in Alertmanager
# Navigate to http://localhost:9093
```

## Log Aggregation with Loki

### Query Logs in Grafana

1. Navigate to Grafana → Explore
2. Select Loki data source
3. Use LogQL to query logs

Example LogQL queries:

```logql
# All logs from namespace
{namespace="default"}

# Logs containing error
{namespace="default"} |= "error"

# Logs from specific pod
{namespace="default", pod="my-pod-123"}

# Rate of error logs
rate({namespace="default"} |= "error" [5m])

# Parse JSON logs
{namespace="default"} | json | level="error"
```

### Configure Log Retention

Adjust Loki retention in configuration:

```yaml
# loki-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: loki-config
  namespace: monitoring
data:
  loki.yaml: |
    limits_config:
      retention_period: 744h  # 31 days
    
    compactor:
      working_directory: /data/compactor
      shared_store: filesystem
      compaction_interval: 10m
      retention_enabled: true
      retention_delete_delay: 2h
      retention_delete_worker_count: 150
```

## Monitoring Best Practices

### Metric Collection

- **Use labels wisely** - High cardinality labels (like user IDs) cause performance issues
- **Set appropriate scrape intervals** - Default 30s is suitable for most cases
- **Monitor metric cardinality** - Track number of unique time series
- **Use recording rules** - Pre-compute expensive queries

### Dashboard Design

- **Group related metrics** - Organize dashboards by service or component
- **Use consistent time ranges** - Align panels for correlation
- **Set meaningful thresholds** - Yellow/red zones for quick assessment
- **Include context** - Add descriptions and links to runbooks

### Alert Configuration

- **Avoid alert fatigue** - Only alert on actionable conditions
- **Use appropriate severity** - Critical requires immediate action
- **Include runbook links** - Help responders resolve issues quickly
- **Test alert routing** - Verify alerts reach the right people

### Performance Optimization

- **Limit query time range** - Shorter ranges query faster
- **Use recording rules** - Pre-aggregate expensive queries
- **Adjust retention** - Balance storage cost vs. historical data needs
- **Monitor Prometheus itself** - Track scrape duration and memory usage

## Troubleshooting

### Prometheus Not Scraping Targets

Check ServiceMonitor configuration:

```bash
# List ServiceMonitors
kubectl get servicemonitors -A

# Check Prometheus targets
kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090
# Navigate to http://localhost:9090/targets
```

Common issues:
- ServiceMonitor label selector doesn't match service
- Network policy blocking scrape traffic
- Service port name doesn't match ServiceMonitor

### Grafana Dashboard Not Loading Data

Verify data source configuration:

```bash
# Check Prometheus is running
kubectl get pods -n monitoring | grep prometheus

# Test Prometheus query
curl -G http://localhost:9090/api/v1/query --data-urlencode 'query=up'
```

Common issues:
- Data source URL incorrect
- Prometheus not collecting metrics
- Time range outside retention period

### Loki Logs Not Appearing

Check Promtail (log shipper) status:

```bash
# Check Promtail pods
kubectl get pods -n monitoring | grep promtail

# Check Promtail logs
kubectl logs -n monitoring -l app.kubernetes.io/name=promtail
```

Common issues:
- Promtail not deployed (check if Loki is enabled)
- Insufficient permissions to read pod logs
- Loki storage backend misconfigured

### High Memory Usage

Prometheus memory usage grows with metric cardinality:

```bash
# Check current memory usage
kubectl top pod -n monitoring | grep prometheus

# Check metric cardinality
curl http://localhost:9090/api/v1/status/tsdb
```

Solutions:
- Reduce scrape interval for high-cardinality targets
- Drop unnecessary metrics using relabel_configs
- Increase Prometheus memory limits
- Use remote storage for long-term retention

## Integration with External Systems

### Slack Integration

Configure Slack webhook in Alertmanager:

```yaml
receivers:
  - name: 'slack-notifications'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK'
        channel: '#alerts'
        title: '{{ .GroupLabels.alertname }}'
        text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'
        send_resolved: true
```

### PagerDuty Integration

Configure PagerDuty in Alertmanager:

```yaml
receivers:
  - name: 'pagerduty-critical'
    pagerduty_configs:
      - service_key: 'YOUR_PAGERDUTY_INTEGRATION_KEY'
        description: '{{ .GroupLabels.alertname }}'
```

### Email Integration

Configure email notifications:

```yaml
global:
  smtp_smarthost: 'smtp.example.com:587'
  smtp_from: 'alertmanager@example.com'
  smtp_auth_username: 'alertmanager@example.com'
  smtp_auth_password: 'password'

receivers:
  - name: 'email-notifications'
    email_configs:
      - to: 'ops-team@example.com'
        headers:
          Subject: 'Alert: {{ .GroupLabels.alertname }}'
```

## Capacity Planning

Monitor these metrics for capacity planning:

```promql
# Node CPU usage trend
avg(rate(node_cpu_seconds_total{mode!="idle"}[5m])) by (instance)

# Node memory usage trend
(node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes) / node_memory_MemTotal_bytes

# Persistent volume usage
kubelet_volume_stats_used_bytes / kubelet_volume_stats_capacity_bytes

# Pod count per node
count(kube_pod_info) by (node)
```

Set alerts for capacity thresholds:
- CPU usage > 80% for 15 minutes
- Memory usage > 85% for 10 minutes
- Disk usage > 80%
- Pod count approaching node limit

## Related Documentation

- **[Disaster Recovery](disaster-recovery.md)** - Backup monitoring data
- **[Security Operations](security.md)** - Security monitoring and audit logs
- **[Troubleshooting](../how-to/troubleshooting.md)** - General troubleshooting procedures
- **[Configuration Reference](../reference/configuration.md)** - Service configuration options

## Next Steps

- Set up custom dashboards for your applications
- Configure alert routing to your team's communication channels
- Establish SLOs and SLIs for critical services
- Create runbooks for common alerts
- Schedule regular review of alert rules and thresholds

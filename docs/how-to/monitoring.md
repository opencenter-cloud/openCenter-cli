# Setting Up Monitoring


## Table of Contents

- [What you'll set up](#what-youll-set-up)
- [Prerequisites](#prerequisites)
- [Steps](#steps)
- [Configuration options](#configuration-options)
- [Accessing Prometheus](#accessing-prometheus)
- [Accessing Alertmanager](#accessing-alertmanager)
- [Pre-configured dashboards](#pre-configured-dashboards)
- [Custom dashboards](#custom-dashboards)
- [Alert configuration](#alert-configuration)
- [Troubleshooting](#troubleshooting)
- [Integration with other services](#integration-with-other-services)
- [Best practices](#best-practices)
- [Related documentation](#related-documentation)
- [Next steps](#next-steps)
**doc_type:** how-to

This guide shows you how to configure the kube-prometheus-stack monitoring service in openCenter clusters.

## What you'll set up

The kube-prometheus-stack provides:
- Prometheus for metrics collection
- Grafana for visualization
- Alertmanager for alert routing
- Pre-configured dashboards for Kubernetes

## Prerequisites

- Cluster initialized with `openCenter cluster init`
- Grafana admin password (required secret)
- Persistent storage available (Cinder CSI or equivalent)

## Steps

### 1. Configure monitoring in cluster config

Edit your cluster configuration:

```bash
vim ~/.config/openCenter/clusters/<organization>/.<cluster>-config.yaml
```

Enable kube-prometheus-stack:

```yaml
opencenter:
  services:
    kube-prometheus-stack:
      enabled: true
      prometheus_volume_size: 50
      prometheus_storage_class: csi-cinder-sc-delete
      grafana_volume_size: 10
      grafana_storage_class: csi-cinder-sc-delete
      alertmanager_volume_size: 10
      alertmanager_storage_class: csi-cinder-sc-delete
```

### 2. Set Grafana admin password

Add the required secret:

```yaml
secrets:
  grafana:
    admin_password: ${GRAFANA_ADMIN_PASSWORD}
```

Or set it in your environment:

```bash
export GRAFANA_ADMIN_PASSWORD="your-secure-password"
```

### 3. Validate and deploy

Run the deployment workflow:

```bash
# Validate configuration
openCenter cluster validate my-cluster

# Regenerate GitOps repository
openCenter cluster setup my-cluster --force

# Commit and push
cd ~/.config/openCenter/clusters/myorg/gitops
git add .
git commit -m "Enable kube-prometheus-stack monitoring"
git push origin main
```

### 4. Verify deployment

Wait for FluxCD to reconcile (typically 1-5 minutes):

```bash
# Check HelmRelease status
kubectl get helmrelease kube-prometheus-stack -n monitoring

# Check pod status
kubectl get pods -n monitoring

# Wait for all pods to be ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=prometheus -n monitoring --timeout=300s
```

### 5. Access Grafana

Get the Grafana service endpoint:

```bash
kubectl get svc -n monitoring | grep grafana
```

For LoadBalancer service:

```bash
GRAFANA_IP=$(kubectl get svc kube-prometheus-stack-grafana -n monitoring -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
echo "Grafana URL: http://${GRAFANA_IP}"
```

For port-forward access:

```bash
kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80
```

Open http://localhost:3000 and log in:
- Username: `admin`
- Password: Your configured `GRAFANA_ADMIN_PASSWORD`

## Configuration options

### Storage configuration

Adjust volume sizes based on your retention needs:

```yaml
opencenter:
  services:
    kube-prometheus-stack:
      # Prometheus stores metrics
      prometheus_volume_size: 100  # GB, increase for longer retention
      
      # Grafana stores dashboards and settings
      grafana_volume_size: 10      # GB, usually sufficient
      
      # Alertmanager stores alert state
      alertmanager_volume_size: 10 # GB, usually sufficient
```

### Storage class selection

Use different storage classes for performance tiers:

```yaml
opencenter:
  services:
    kube-prometheus-stack:
      # High-performance storage for Prometheus
      prometheus_storage_class: csi-cinder-sc-retain-ssd
      
      # Standard storage for Grafana
      grafana_storage_class: csi-cinder-sc-delete
      
      # Standard storage for Alertmanager
      alertmanager_storage_class: csi-cinder-sc-delete
```

Available storage classes depend on your provider:
- OpenStack: `csi-cinder-sc-delete`, `csi-cinder-sc-retain`
- AWS: `gp3`, `gp2`, `io1`
- vSphere: `vsphere-csi-sc`

### Retention configuration

Control how long metrics are stored by customizing Helm values.

Create an override file in your GitOps repository:

```bash
vim ~/.config/openCenter/clusters/myorg/gitops/applications/overlays/my-cluster/services/kube-prometheus-stack/values.yaml
```

Add retention settings:

```yaml
prometheus:
  prometheusSpec:
    retention: 30d
    retentionSize: 45GB
```

## Accessing Prometheus

Prometheus UI is available for direct metric queries:

```bash
kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090
```

Open http://localhost:9090 to access the Prometheus UI.

## Accessing Alertmanager

Alertmanager handles alert routing and silencing:

```bash
kubectl port-forward -n monitoring svc/kube-prometheus-stack-alertmanager 9093:9093
```

Open http://localhost:9093 to access the Alertmanager UI.

## Pre-configured dashboards

Grafana includes dashboards for:
- Kubernetes cluster overview
- Node metrics (CPU, memory, disk, network)
- Pod metrics and resource usage
- Persistent volume usage
- etcd performance
- API server metrics

Access dashboards:
1. Log in to Grafana
2. Navigate to Dashboards → Browse
3. Select a dashboard from the list

## Custom dashboards

Import additional dashboards from Grafana.com:

1. Go to Dashboards → Import
2. Enter dashboard ID (e.g., 315 for Kubernetes cluster monitoring)
3. Select Prometheus data source
4. Click Import

## Alert configuration

Configure Alertmanager to send notifications.

Create an Alertmanager config override:

```bash
vim ~/.config/openCenter/clusters/myorg/gitops/applications/overlays/my-cluster/services/kube-prometheus-stack/alertmanager-config.yaml
```

Example Slack integration:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: alertmanager-kube-prometheus-stack-alertmanager
  namespace: monitoring
type: Opaque
stringData:
  alertmanager.yaml: |
    global:
      resolve_timeout: 5m
    route:
      group_by: ['alertname', 'cluster']
      group_wait: 10s
      group_interval: 10s
      repeat_interval: 12h
      receiver: 'slack'
    receivers:
    - name: 'slack'
      slack_configs:
      - api_url: 'https://hooks.slack.com/services/YOUR/WEBHOOK/URL'
        channel: '#alerts'
        title: 'Cluster Alert'
        text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'
```

Encrypt the secret with SOPS:

```bash
sops --encrypt --in-place alertmanager-config.yaml
```

## Troubleshooting

### Prometheus not scraping metrics

Check ServiceMonitor resources:

```bash
kubectl get servicemonitors -n monitoring
kubectl describe servicemonitor <name> -n monitoring
```

Verify Prometheus targets:
1. Port-forward to Prometheus: `kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090`
2. Open http://localhost:9090/targets
3. Check for targets in "down" state

### Grafana login fails

Reset the admin password:

```bash
# Update the secret
kubectl create secret generic grafana-admin -n monitoring \
  --from-literal=admin-password='new-password' \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart Grafana
kubectl rollout restart deployment kube-prometheus-stack-grafana -n monitoring
```

### Persistent volumes not binding

Check PVC status:

```bash
kubectl get pvc -n monitoring
kubectl describe pvc <pvc-name> -n monitoring
```

Verify storage class exists:

```bash
kubectl get storageclass
```

If the storage class is missing, check your CSI driver installation:

```bash
kubectl get pods -n kube-system | grep csi
```

### High memory usage

Prometheus memory usage grows with the number of metrics and retention period.

Reduce retention:

```yaml
prometheus:
  prometheusSpec:
    retention: 15d
    retentionSize: 20GB
```

Or increase memory limits:

```yaml
prometheus:
  prometheusSpec:
    resources:
      limits:
        memory: 8Gi
      requests:
        memory: 4Gi
```

### Dashboards not loading

Check Grafana logs:

```bash
kubectl logs -n monitoring deployment/kube-prometheus-stack-grafana -f
```

Verify Prometheus data source:
1. Log in to Grafana
2. Go to Configuration → Data Sources
3. Test the Prometheus connection

## Integration with other services

### Loki for logs

Enable Loki alongside Prometheus:

```yaml
opencenter:
  services:
    loki:
      enabled: true
      volume_size: 20
      storage_class: csi-cinder-sc-delete
```

Loki automatically integrates with Grafana for log visualization.

### Alert-proxy for external routing

Route alerts to external systems:

```yaml
opencenter:
  managed_service:
    alert-proxy:
      enabled: true
      alert_manager_base_url: http://kube-prometheus-stack-alertmanager.monitoring:9093
```

## Best practices

- **Size volumes appropriately**: Calculate based on metric cardinality and retention
- **Use retain storage classes**: For production, use `retain` to prevent data loss
- **Configure alerts**: Set up Alertmanager routing for critical alerts
- **Monitor the monitors**: Set alerts for Prometheus and Grafana health
- **Regular backups**: Back up Grafana dashboards and Prometheus data
- **Secure access**: Use authentication and network policies
- **Resource limits**: Set appropriate CPU and memory limits

## Related documentation

- [Deploying Changes](deploying-changes.md) - Apply configuration updates
- [Adding Services](adding-services.md) - Enable additional services
- [Troubleshooting](troubleshooting.md) - Debug common issues

## Next steps

- Configure custom alerts for your applications
- Import additional dashboards from Grafana.com
- Set up long-term metric storage with Thanos
- Integrate with external monitoring systems

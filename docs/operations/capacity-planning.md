# Capacity Planning


## Table of Contents

- [Who This Is For](#who-this-is-for)
- [Prerequisites](#prerequisites)
- [Resource Planning Overview](#resource-planning-overview)
- [Analyze Current Usage](#analyze-current-usage)
- [Forecast Resource Needs](#forecast-resource-needs)
- [Scaling Strategies](#scaling-strategies)
- [Resource Optimization](#resource-optimization)
- [Storage Capacity Planning](#storage-capacity-planning)
- [Network Capacity Planning](#network-capacity-planning)
- [Control Plane Scaling](#control-plane-scaling)
- [Cost Optimization](#cost-optimization)
- [Capacity Planning Checklist](#capacity-planning-checklist)
- [Capacity Alerts](#capacity-alerts)
- [Related Documentation](#related-documentation)
- [Next Steps](#next-steps)
**doc_type: how-to**

Plan and forecast resource requirements for openCenter-managed Kubernetes clusters. This guide covers resource analysis, growth forecasting, scaling strategies, and cost optimization.

## Who This Is For

Platform engineers, SREs, and operations teams responsible for cluster capacity management and cost optimization. Use this guide to analyze current usage, forecast future needs, and plan scaling activities.

## Prerequisites

- Running openCenter cluster with monitoring enabled
- Access to Prometheus metrics
- Understanding of Kubernetes resource management
- Historical usage data (recommended: 30+ days)

## Resource Planning Overview

Capacity planning ensures clusters have sufficient resources while avoiding waste:

- **Compute**: CPU and memory for nodes and pods
- **Storage**: Persistent volumes and etcd capacity
- **Network**: Bandwidth and connection limits
- **Control Plane**: API server and etcd scaling

## Analyze Current Usage

### Node Resource Utilization

Check current node resource consumption:

```bash
# View node resource usage
kubectl top nodes

# Get detailed node capacity and allocation
kubectl describe nodes | grep -A 5 "Allocated resources"

# Calculate node utilization percentage
kubectl get nodes -o json | jq -r '.items[] | 
  {
    name: .metadata.name,
    cpu_capacity: .status.capacity.cpu,
    cpu_allocatable: .status.allocatable.cpu,
    memory_capacity: .status.capacity.memory,
    memory_allocatable: .status.allocatable.memory
  }'
```

**Expected output**:
```
NAME          CPU(cores)   CPU%   MEMORY(bytes)   MEMORY%
control-1     1200m        30%    4096Mi          51%
control-2     1150m        28%    3890Mi          48%
control-3     1180m        29%    4012Mi          50%
worker-1      3200m        80%    12288Mi         76%
worker-2      3100m        77%    11890Mi         74%
```

### Pod Resource Requests

Analyze pod resource requests and limits:

```bash
# List pods with resource requests
kubectl get pods -A -o json | jq -r '.items[] | 
  {
    namespace: .metadata.namespace,
    name: .metadata.name,
    cpu_request: .spec.containers[].resources.requests.cpu,
    memory_request: .spec.containers[].resources.requests.memory,
    cpu_limit: .spec.containers[].resources.limits.cpu,
    memory_limit: .spec.containers[].resources.limits.memory
  }'

# Sum resource requests by namespace
kubectl get pods -A -o json | jq -r '
  .items | group_by(.metadata.namespace) | 
  map({
    namespace: .[0].metadata.namespace,
    pod_count: length,
    total_cpu_request: ([.[].spec.containers[].resources.requests.cpu // "0"] | add),
    total_memory_request: ([.[].spec.containers[].resources.requests.memory // "0"] | add)
  })'
```

### Storage Utilization

Check persistent volume usage:

```bash
# List PVC usage
kubectl get pvc -A -o json | jq -r '.items[] | 
  {
    namespace: .metadata.namespace,
    name: .metadata.name,
    capacity: .status.capacity.storage,
    storage_class: .spec.storageClassName
  }'

# Check storage class capacity
kubectl get storageclass

# Monitor volume usage (requires metrics-server)
kubectl get --raw /apis/metrics.k8s.io/v1beta1/pods | jq -r '
  .items[] | 
  select(.containers[].usage.ephemeral_storage != null) |
  {
    namespace: .metadata.namespace,
    name: .metadata.name,
    ephemeral_storage: .containers[].usage.ephemeral_storage
  }'
```

### etcd Capacity

Monitor etcd database size and performance:

```bash
# Check etcd database size
kubectl exec -n kube-system etcd-control-1 -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  endpoint status --write-out=table

# Check etcd metrics
kubectl exec -n kube-system etcd-control-1 -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  endpoint status -w json | jq '.[] | {
    endpoint: .Endpoint,
    db_size: .Status.dbSize,
    db_size_in_use: .Status.dbSizeInUse
  }'
```

**Healthy etcd metrics**:
- Database size: < 2 GB (warning at 4 GB, critical at 6 GB)
- Database size in use: > 50% of total size
- Latency: < 10ms for 99th percentile

## Forecast Resource Needs

### Historical Trend Analysis

Use Prometheus to analyze growth trends:

```promql
# CPU usage trend (30-day average)
avg_over_time(
  sum(rate(container_cpu_usage_seconds_total{container!=""}[5m]))
  [30d:1h]
)

# Memory usage trend (30-day average)
avg_over_time(
  sum(container_memory_working_set_bytes{container!=""})
  [30d:1h]
)

# Pod count growth
avg_over_time(
  count(kube_pod_info)
  [30d:1h]
)

# PVC storage growth
avg_over_time(
  sum(kubelet_volume_stats_used_bytes)
  [30d:1h]
)
```

### Calculate Growth Rate

Determine monthly growth rate:

```bash
# Export metrics to CSV for analysis
# CPU growth rate
curl -G 'http://prometheus:9090/api/v1/query_range' \
  --data-urlencode 'query=sum(rate(container_cpu_usage_seconds_total{container!=""}[5m]))' \
  --data-urlencode 'start=2026-01-01T00:00:00Z' \
  --data-urlencode 'end=2026-01-31T23:59:59Z' \
  --data-urlencode 'step=1h' | \
  jq -r '.data.result[0].values[] | @csv' > cpu_usage.csv

# Calculate growth rate
# Growth Rate = ((Current - Previous) / Previous) * 100
```

**Example calculation**:
```
Month 1 CPU: 10 cores
Month 2 CPU: 12 cores
Growth Rate: ((12 - 10) / 10) * 100 = 20% per month
```

### Forecast Future Capacity

Project resource needs based on growth rate:

```
Future Capacity = Current Capacity * (1 + Growth Rate) ^ Months

Example:
Current: 20 cores
Growth: 15% per month
6-month forecast: 20 * (1 + 0.15) ^ 6 = 46.2 cores
```

**Capacity planning spreadsheet**:

| Resource | Current | Growth Rate | 3-Month | 6-Month | 12-Month |
|----------|---------|-------------|---------|---------|----------|
| CPU cores | 20 | 15%/month | 30.4 | 46.2 | 106.7 |
| Memory GB | 80 | 12%/month | 112.5 | 158.2 | 310.6 |
| Storage TB | 5 | 20%/month | 8.6 | 14.9 | 44.6 |
| Pods | 500 | 10%/month | 665 | 885 | 1570 |

## Scaling Strategies

### Vertical Scaling (Resize Nodes)

Increase node size for resource-intensive workloads:

```yaml
# Update cluster configuration
opencenter:
  cluster:
    kubernetes:
      flavor_worker: gp.0.8.32  # Upgrade from gp.0.4.16
```

**When to use**:
- Workloads require more resources per pod
- Node count is already high
- Simplify management with fewer, larger nodes

**Considerations**:
- Larger blast radius if node fails
- Higher cost per node
- May require workload migration

### Horizontal Scaling (Add Nodes)

Add more nodes to distribute workload:

```yaml
# Update cluster configuration
opencenter:
  cluster:
    kubernetes:
      worker_count: 5  # Increase from 2
```

**When to use**:
- Workloads can be distributed across nodes
- Need better fault tolerance
- Cost-effective for many small workloads

**Considerations**:
- More nodes to manage
- Network overhead increases
- Control plane load increases

### Auto-Scaling

Configure cluster autoscaler for dynamic scaling:

```yaml
# cluster-autoscaler configuration
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cluster-autoscaler
  namespace: kube-system
spec:
  template:
    spec:
      containers:
        - name: cluster-autoscaler
          image: registry.k8s.io/autoscaling/cluster-autoscaler:v1.28.0
          command:
            - ./cluster-autoscaler
            - --cloud-provider=openstack
            - --nodes=2:10:worker-pool
            - --scale-down-enabled=true
            - --scale-down-delay-after-add=10m
            - --scale-down-unneeded-time=10m
```

**Auto-scaling parameters**:
- `--nodes=min:max:pool`: Node count range
- `--scale-down-delay-after-add`: Wait time before scale-down
- `--scale-down-unneeded-time`: Idle time before removal

## Resource Optimization

### Right-Size Pod Requests

Adjust pod resource requests based on actual usage:

```bash
# Analyze pod resource usage vs requests
kubectl get pods -A -o json | jq -r '.items[] | 
  {
    namespace: .metadata.namespace,
    name: .metadata.name,
    cpu_request: .spec.containers[0].resources.requests.cpu,
    cpu_usage: "check metrics",
    memory_request: .spec.containers[0].resources.requests.memory,
    memory_usage: "check metrics"
  }'

# Use Vertical Pod Autoscaler recommendations
kubectl get vpa -A
kubectl describe vpa <vpa-name>
```

**Optimization guidelines**:
- CPU request: Set to 90th percentile usage
- Memory request: Set to 95th percentile usage
- CPU limit: 2-3x request (for burstable workloads)
- Memory limit: 1.5-2x request (prevent OOM)

### Consolidate Underutilized Nodes

Identify and consolidate workloads from underutilized nodes:

```bash
# Find nodes with low utilization
kubectl top nodes | awk '$3 < 30 || $5 < 30 {print $1}'

# Drain underutilized node
kubectl drain <node-name> \
  --ignore-daemonsets \
  --delete-emptydir-data \
  --force

# Remove node from cluster
kubectl delete node <node-name>

# Update cluster configuration to reduce worker count
```

### Implement Pod Disruption Budgets

Ensure availability during scaling operations:

```yaml
# pod-disruption-budget.yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: app-pdb
  namespace: production
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: my-app
```

## Storage Capacity Planning

### Analyze Storage Growth

Monitor persistent volume usage trends:

```promql
# Storage usage by PVC
sum(kubelet_volume_stats_used_bytes) by (persistentvolumeclaim, namespace)

# Storage growth rate
rate(kubelet_volume_stats_used_bytes[30d])

# Storage capacity remaining
sum(kubelet_volume_stats_capacity_bytes - kubelet_volume_stats_used_bytes) 
  by (persistentvolumeclaim, namespace)
```

### Expand Storage Capacity

Resize persistent volumes:

```bash
# Check if storage class supports volume expansion
kubectl get storageclass csi-cinder-sc-delete -o yaml | grep allowVolumeExpansion

# Expand PVC
kubectl patch pvc my-pvc -p '{"spec":{"resources":{"requests":{"storage":"200Gi"}}}}'

# Verify expansion
kubectl get pvc my-pvc -w
```

### Storage Cleanup

Remove unused volumes:

```bash
# Find unbound PVCs
kubectl get pvc -A --field-selector=status.phase=Pending

# Find PVCs not attached to pods
kubectl get pvc -A -o json | jq -r '.items[] | 
  select(.status.phase == "Bound") | 
  select(.metadata.name as $pvc | 
    [.metadata.namespace as $ns | 
      (kubectl get pods -n $ns -o json | 
        .items[].spec.volumes[]?.persistentVolumeClaim.claimName) 
    ] | index($pvc) | not) | 
  "\(.metadata.namespace)/\(.metadata.name)"'

# Delete unused PVC
kubectl delete pvc -n <namespace> <pvc-name>
```

## Network Capacity Planning

### Monitor Network Usage

Track network bandwidth and connections:

```promql
# Network receive bandwidth
sum(rate(container_network_receive_bytes_total[5m])) by (pod, namespace)

# Network transmit bandwidth
sum(rate(container_network_transmit_bytes_total[5m])) by (pod, namespace)

# Connection count
sum(node_netstat_Tcp_CurrEstab) by (instance)
```

### Network Scaling Considerations

Plan for network capacity:

- **Pod network CIDR**: Ensure sufficient IP addresses
  - Default: 10.42.0.0/16 (65,536 IPs)
  - Recommended: /16 for < 1000 nodes, /15 for larger clusters

- **Service network CIDR**: Plan for service growth
  - Default: 10.43.0.0/16 (65,536 IPs)
  - Recommended: 1 IP per 10 pods

- **Node network**: Verify subnet capacity
  - Each node requires 1 IP
  - Add buffer for scaling (20-30%)

## Control Plane Scaling

### Monitor Control Plane Load

Check API server and etcd performance:

```promql
# API server request rate
sum(rate(apiserver_request_total[5m])) by (verb, resource)

# API server latency
histogram_quantile(0.99, 
  sum(rate(apiserver_request_duration_seconds_bucket[5m])) by (verb, le)
)

# etcd request rate
sum(rate(etcd_server_proposals_committed_total[5m]))

# etcd disk sync duration
histogram_quantile(0.99, 
  sum(rate(etcd_disk_wal_fsync_duration_seconds_bucket[5m])) by (le)
)
```

### Scale Control Plane

Add control plane nodes for high-load clusters:

```yaml
# Update cluster configuration
opencenter:
  cluster:
    kubernetes:
      master_count: 5  # Increase from 3
```

**Control plane scaling guidelines**:
- 3 nodes: Up to 100 worker nodes
- 5 nodes: Up to 500 worker nodes
- 7 nodes: Up to 1000 worker nodes

## Cost Optimization

### Analyze Cost by Resource

Calculate cost per resource type:

```bash
# Node cost analysis
# Assuming $0.10/core/hour and $0.02/GB/hour

# Calculate monthly node cost
kubectl get nodes -o json | jq -r '.items[] | 
  {
    name: .metadata.name,
    cpu: .status.capacity.cpu,
    memory: (.status.capacity.memory | rtrimstr("Ki") | tonumber / 1024 / 1024),
    monthly_cost: (
      (.status.capacity.cpu | tonumber) * 0.10 * 730 +
      ((.status.capacity.memory | rtrimstr("Ki") | tonumber / 1024 / 1024) * 0.02 * 730)
    )
  }'
```

### Cost Optimization Strategies

**Reduce waste**:
- Right-size pod requests (avoid over-provisioning)
- Use spot/preemptible instances for non-critical workloads
- Implement auto-scaling to match demand
- Schedule batch jobs during off-peak hours

**Storage optimization**:
- Use appropriate storage classes (standard vs. SSD)
- Implement data lifecycle policies
- Compress logs and backups
- Delete unused volumes

**Network optimization**:
- Use internal load balancers when possible
- Minimize cross-region traffic
- Implement caching to reduce bandwidth
- Use CDN for static content

## Capacity Planning Checklist

### Monthly Review

- [ ] Analyze resource utilization trends
- [ ] Review pod resource requests vs actual usage
- [ ] Check storage growth and cleanup unused volumes
- [ ] Monitor etcd database size
- [ ] Review cost reports and identify optimization opportunities

### Quarterly Planning

- [ ] Forecast resource needs for next 6-12 months
- [ ] Plan node scaling activities
- [ ] Review and update resource quotas
- [ ] Evaluate new instance types or storage classes
- [ ] Update capacity planning documentation

### Annual Planning

- [ ] Review multi-year growth projections
- [ ] Plan major infrastructure changes
- [ ] Evaluate alternative cloud providers or regions
- [ ] Update disaster recovery capacity requirements
- [ ] Budget planning for next fiscal year

## Capacity Alerts

Set up alerts for capacity thresholds:

```yaml
# capacity-alerts.yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: capacity-alerts
  namespace: monitoring
spec:
  groups:
    - name: capacity
      interval: 5m
      rules:
        - alert: NodeCPUPressure
          expr: |
            (sum(rate(container_cpu_usage_seconds_total{container!=""}[5m])) by (node) /
             sum(kube_node_status_allocatable{resource="cpu"}) by (node)) > 0.80
          for: 15m
          labels:
            severity: warning
          annotations:
            summary: "Node {{ $labels.node }} CPU usage above 80%"
            description: "Consider adding more nodes or scaling up"
        
        - alert: NodeMemoryPressure
          expr: |
            (sum(container_memory_working_set_bytes{container!=""}) by (node) /
             sum(kube_node_status_allocatable{resource="memory"}) by (node)) > 0.85
          for: 10m
          labels:
            severity: warning
          annotations:
            summary: "Node {{ $labels.node }} memory usage above 85%"
        
        - alert: StorageNearCapacity
          expr: |
            (kubelet_volume_stats_used_bytes / kubelet_volume_stats_capacity_bytes) > 0.80
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "PVC {{ $labels.persistentvolumeclaim }} is 80% full"
        
        - alert: EtcdDatabaseSizeLarge
          expr: etcd_mvcc_db_total_size_in_bytes > 4e9
          for: 10m
          labels:
            severity: warning
          annotations:
            summary: "etcd database size exceeds 4GB"
            description: "Consider compacting etcd or scaling control plane"
```

## Related Documentation

- **[Monitoring](monitoring.md)** - Metrics collection and dashboards
- **[Disaster Recovery](disaster-recovery.md)** - Backup capacity planning
- **[Configuration Reference](../reference/configuration.md)** - Node sizing options
- **[Cluster Upgrade Runbook](runbooks/cluster-upgrade.md)** - Scaling during upgrades

## Next Steps

- Set up Prometheus queries for capacity metrics
- Create capacity planning dashboard in Grafana
- Schedule monthly capacity review meetings
- Document baseline resource requirements
- Establish capacity alert thresholds

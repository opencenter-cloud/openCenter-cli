# Cluster Upgrade Runbook


## Table of Contents

- [Who This Is For](#who-this-is-for)
- [Prerequisites](#prerequisites)
- [Upgrade Overview](#upgrade-overview)
- [Pre-Upgrade Checklist](#pre-upgrade-checklist)
- [Upgrade Procedure](#upgrade-procedure)
- [Post-Upgrade Validation](#post-upgrade-validation)
- [Rollback Procedure](#rollback-procedure)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)
- [Related Documentation](#related-documentation)
- [Upgrade Schedule](#upgrade-schedule)
- [Post-Upgrade Tasks](#post-upgrade-tasks)
**doc_type: how-to**

Step-by-step procedures for upgrading Kubernetes clusters managed by openCenter, including pre-upgrade validation, upgrade execution, and rollback procedures.

## Who This Is For

Operations teams and SREs responsible for maintaining cluster versions and performing upgrades. Use this runbook when planning and executing Kubernetes version upgrades.

## Prerequisites

- Running openCenter cluster
- Access to cluster configuration file
- `kubectl` access with cluster-admin permissions
- Backup of cluster state (see [Disaster Recovery](../disaster-recovery.md))
- Maintenance window scheduled
- Rollback plan documented

## Upgrade Overview

openCenter clusters use Kubespray for Kubernetes deployment, which supports in-place upgrades of control plane and worker nodes.

**Upgrade Path**:
1. Pre-upgrade validation and backup
2. Control plane upgrade (one node at a time)
3. Worker node upgrade (rolling update)
4. Post-upgrade validation
5. Rollback if issues detected

**Supported Upgrade Paths**:
- Minor version upgrades (e.g., 1.30.x → 1.31.x)
- Patch version upgrades (e.g., 1.31.4 → 1.31.5)
- Maximum one minor version at a time (no skipping versions)

**Upgrade Duration**:
- Small cluster (3 control plane, 2 workers): 30-45 minutes
- Medium cluster (3 control plane, 10 workers): 1-2 hours
- Large cluster (5 control plane, 50+ workers): 3-4 hours

## Pre-Upgrade Checklist

### 1. Review Release Notes

Check Kubernetes release notes for breaking changes:

```bash
# Check current version
kubectl version --short

# Review release notes for target version
# Visit: https://kubernetes.io/docs/setup/release/notes/
```

Key areas to review:
- API deprecations and removals
- Feature gate changes
- Component version requirements
- Known issues and workarounds

### 2. Verify Cluster Health

Ensure cluster is healthy before upgrade:

```bash
# Check node status
kubectl get nodes
# All nodes should be Ready

# Check pod status
kubectl get pods -A
# No pods should be in CrashLoopBackOff or Error state

# Check component health
kubectl get componentstatuses
# All components should be Healthy

# Check etcd health
kubectl exec -n kube-system etcd-<control-plane-node> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/ssl/etcd/ca.pem \
  --cert=/etc/kubernetes/ssl/etcd/node-<node>.pem \
  --key=/etc/kubernetes/ssl/etcd/node-<node>-key.pem \
  endpoint health

# Check for pending PVCs
kubectl get pvc -A | grep Pending
# Should return no results

# Check for failed jobs
kubectl get jobs -A --field-selector status.successful=0
```

### 3. Backup Cluster State

Create comprehensive backup before upgrade:

```bash
# Backup cluster configuration
openCenter cluster backup create my-cluster --encrypt

# Backup etcd (if not using automated backups)
kubectl exec -n kube-system etcd-<control-plane-node> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/ssl/etcd/ca.pem \
  --cert=/etc/kubernetes/ssl/etcd/node-<node>.pem \
  --key=/etc/kubernetes/ssl/etcd/node-<node>-key.pem \
  snapshot save /var/lib/etcd/backup-$(date +%Y%m%d-%H%M%S).db

# Backup persistent volumes (if using Velero)
velero backup create pre-upgrade-backup --wait

# Export all resources
kubectl get all -A -o yaml > all-resources-backup-$(date +%Y%m%d).yaml
```

### 4. Check API Deprecations

Identify deprecated APIs in use:

```bash
# Install kubectl-deprecations plugin
kubectl krew install deprecations

# Check for deprecated APIs
kubectl deprecations

# Or use pluto
brew install pluto  # macOS
pluto detect-files -d .

# Check specific API versions
kubectl get deployments -A -o json | \
  jq -r '.items[] | select(.apiVersion=="apps/v1beta1") | .metadata.name'
```

### 5. Review Addon Compatibility

Verify addon compatibility with target Kubernetes version:

| Addon | Current Version | Target K8s | Compatible | Notes |
|-------|----------------|------------|------------|-------|
| Calico | 3.27.x | 1.31.x | ✓ | Check Calico docs |
| cert-manager | 1.14.x | 1.31.x | ✓ | No changes needed |
| Prometheus | 2.50.x | 1.31.x | ✓ | Update CRDs first |
| Loki | 2.9.x | 1.31.x | ✓ | No changes needed |
| Velero | 1.13.x | 1.31.x | ✓ | Check plugin versions |

### 6. Notify Stakeholders

Communicate upgrade plan:

```markdown
**Subject**: Kubernetes Cluster Upgrade - [Cluster Name]

**Maintenance Window**: [Date] [Start Time] - [End Time] [Timezone]

**Impact**: 
- Brief API server unavailability during control plane upgrade (~5 min per node)
- Workload disruption during worker node upgrades (rolling update)
- Services with PodDisruptionBudgets will be respected

**Upgrade Path**: Kubernetes [Current Version] → [Target Version]

**Rollback Plan**: Automated rollback if upgrade fails validation

**Contact**: [On-call Engineer] [Phone/Slack]
```

## Upgrade Procedure

### Step 1: Update Cluster Configuration

Update Kubernetes version in cluster configuration:

```bash
# Edit cluster configuration
sops ~/.config/openCenter/clusters/myorg/.my-cluster-config.yaml

# Update Kubernetes version
# Change:
#   kubernetes:
#     version: "1.30.5"
# To:
#     version: "1.31.4"

# Validate configuration
openCenter cluster validate my-cluster

# Commit changes
cd ~/.config/openCenter/clusters/myorg/my-cluster
git add .
git commit -m "upgrade: Kubernetes 1.30.5 → 1.31.4"
git push
```

### Step 2: Prepare Upgrade

Generate updated Ansible inventory:

```bash
# Regenerate cluster manifests
openCenter cluster setup my-cluster --force

# Review changes
cd ~/.config/openCenter/clusters/myorg/my-cluster
git diff

# Verify Kubespray version compatibility
cat infrastructure/clusters/my-cluster/kubespray-version.txt
```

### Step 3: Upgrade Control Plane

Upgrade control plane nodes one at a time:

```bash
# Set maintenance mode (optional - prevents new workloads)
kubectl cordon <control-plane-node-1>

# Drain node (moves workloads to other nodes)
kubectl drain <control-plane-node-1> \
  --ignore-daemonsets \
  --delete-emptydir-data \
  --force \
  --timeout=300s

# Run Kubespray upgrade for first control plane node
cd ~/.config/openCenter/clusters/myorg/my-cluster/infrastructure/clusters/my-cluster
ansible-playbook -i inventory/hosts.yaml \
  --limit=<control-plane-node-1> \
  upgrade-cluster.yml

# Verify node upgrade
kubectl get nodes
# Node should show new version

# Verify etcd health
kubectl exec -n kube-system etcd-<control-plane-node-1> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/ssl/etcd/ca.pem \
  --cert=/etc/kubernetes/ssl/etcd/node-<node>.pem \
  --key=/etc/kubernetes/ssl/etcd/node-<node>-key.pem \
  endpoint health

# Uncordon node
kubectl uncordon <control-plane-node-1>

# Wait for node to be Ready
kubectl wait --for=condition=Ready node/<control-plane-node-1> --timeout=300s

# Repeat for remaining control plane nodes
# IMPORTANT: Upgrade one control plane node at a time
```

**Control Plane Upgrade Validation**:

After each control plane node upgrade:

```bash
# Check API server version
kubectl version --short

# Check etcd cluster health
kubectl exec -n kube-system etcd-<control-plane-node> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/ssl/etcd/ca.pem \
  --cert=/etc/kubernetes/ssl/etcd/node-<node>.pem \
  --key=/etc/kubernetes/ssl/etcd/node-<node>-key.pem \
  member list

# Check control plane pods
kubectl get pods -n kube-system | grep -E "kube-apiserver|kube-controller|kube-scheduler"

# Test API server
kubectl get nodes
kubectl get pods -A
```

### Step 4: Upgrade Worker Nodes

Upgrade worker nodes in batches:

```bash
# Determine batch size based on PodDisruptionBudgets
# Recommended: 20-30% of workers at a time

# Batch 1: First set of workers
WORKERS="worker-1 worker-2"

for worker in $WORKERS; do
  echo "Upgrading $worker..."
  
  # Cordon node
  kubectl cordon $worker
  
  # Drain node
  kubectl drain $worker \
    --ignore-daemonsets \
    --delete-emptydir-data \
    --force \
    --timeout=300s
  
  # Run upgrade
  ansible-playbook -i inventory/hosts.yaml \
    --limit=$worker \
    upgrade-cluster.yml
  
  # Uncordon node
  kubectl uncordon $worker
  
  # Wait for node to be Ready
  kubectl wait --for=condition=Ready node/$worker --timeout=300s
  
  echo "$worker upgrade complete"
done

# Verify batch upgrade
kubectl get nodes
kubectl get pods -A -o wide | grep $WORKERS

# Wait for workloads to stabilize before next batch
sleep 300

# Repeat for remaining worker batches
```

**Worker Node Upgrade Validation**:

After each batch:

```bash
# Check node versions
kubectl get nodes -o wide

# Check pod distribution
kubectl get pods -A -o wide | awk '{print $8}' | sort | uniq -c

# Check for pending pods
kubectl get pods -A --field-selector=status.phase=Pending

# Check for failed pods
kubectl get pods -A --field-selector=status.phase=Failed

# Verify application health
kubectl get deployments -A
kubectl get statefulsets -A
```

### Step 5: Update Cluster Addons

Upgrade cluster addons to compatible versions:

```bash
# Update Calico (if needed)
kubectl apply -f https://raw.githubusercontent.com/projectcalico/calico/v3.27.0/manifests/calico.yaml

# Update cert-manager CRDs
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.0/cert-manager.crds.yaml

# Update Prometheus Operator CRDs
kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.71.0/example/prometheus-operator-crd/monitoring.coreos.com_alertmanagerconfigs.yaml
kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.71.0/example/prometheus-operator-crd/monitoring.coreos.com_alertmanagers.yaml
kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.71.0/example/prometheus-operator-crd/monitoring.coreos.com_podmonitors.yaml
kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.71.0/example/prometheus-operator-crd/monitoring.coreos.com_probes.yaml
kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.71.0/example/prometheus-operator-crd/monitoring.coreos.com_prometheusagents.yaml
kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.71.0/example/prometheus-operator-crd/monitoring.coreos.com_prometheuses.yaml
kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.71.0/example/prometheus-operator-crd/monitoring.coreos.com_prometheusrules.yaml
kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.71.0/example/prometheus-operator-crd/monitoring.coreos.com_scrapeconfigs.yaml
kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.71.0/example/prometheus-operator-crd/monitoring.coreos.com_servicemonitors.yaml
kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.71.0/example/prometheus-operator-crd/monitoring.coreos.com_thanosrulers.yaml

# Restart addon pods to pick up changes
kubectl rollout restart deployment -n cert-manager cert-manager
kubectl rollout restart deployment -n monitoring prometheus-operator
```

## Post-Upgrade Validation

### Comprehensive Health Check

Run full cluster validation:

```bash
# Check all nodes
kubectl get nodes -o wide
# All nodes should show new version and be Ready

# Check system pods
kubectl get pods -n kube-system
# All pods should be Running

# Check addon pods
kubectl get pods -n cert-manager
kubectl get pods -n monitoring
kubectl get pods -n kube-system | grep calico

# Check workload pods
kubectl get pods -A | grep -v "Running\|Completed"
# Should return no results (except for jobs)

# Run cluster validation
kubectl cluster-info
kubectl get componentstatuses

# Check API server metrics
kubectl top nodes
kubectl top pods -A

# Verify DNS resolution
kubectl run -it --rm debug --image=busybox --restart=Never -- nslookup kubernetes.default
```

### Application Validation

Test critical applications:

```bash
# Check application deployments
kubectl get deployments -n production
kubectl rollout status deployment -n production <app-name>

# Test application endpoints
curl https://app.example.com/health

# Check application logs
kubectl logs -n production deployment/<app-name> --tail=50

# Verify database connectivity
kubectl exec -n production <app-pod> -- nc -zv postgres.example.com 5432
```

### Performance Validation

Monitor cluster performance:

```bash
# Check API server latency
kubectl get --raw /metrics | grep apiserver_request_duration_seconds

# Check etcd performance
kubectl exec -n kube-system etcd-<control-plane-node> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/ssl/etcd/ca.pem \
  --cert=/etc/kubernetes/ssl/etcd/node-<node>.pem \
  --key=/etc/kubernetes/ssl/etcd/node-<node>-key.pem \
  check perf

# Monitor resource usage
watch kubectl top nodes
watch kubectl top pods -A
```

## Rollback Procedure

If upgrade fails validation, rollback to previous version:

### Automated Rollback

```bash
# Restore cluster configuration
openCenter cluster backup restore my-cluster-<timestamp>

# Revert Kubernetes version in configuration
sops ~/.config/openCenter/clusters/myorg/.my-cluster-config.yaml
# Change version back to previous version

# Regenerate manifests
openCenter cluster setup my-cluster --force

# Run Kubespray rollback
cd ~/.config/openCenter/clusters/myorg/my-cluster/infrastructure/clusters/my-cluster
ansible-playbook -i inventory/hosts.yaml rollback-cluster.yml

# Verify rollback
kubectl get nodes -o wide
# Nodes should show previous version
```

### Manual Rollback

If automated rollback fails:

```bash
# Restore etcd from backup
kubectl exec -n kube-system etcd-<control-plane-node> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/ssl/etcd/ca.pem \
  --cert=/etc/kubernetes/ssl/etcd/node-<node>.pem \
  --key=/etc/kubernetes/ssl/etcd/node-<node>-key.pem \
  snapshot restore /var/lib/etcd/backup-<timestamp>.db

# Restart control plane components
kubectl delete pod -n kube-system kube-apiserver-<control-plane-node>
kubectl delete pod -n kube-system kube-controller-manager-<control-plane-node>
kubectl delete pod -n kube-system kube-scheduler-<control-plane-node>

# Verify cluster health
kubectl get nodes
kubectl get pods -A
```

## Troubleshooting

### Control Plane Node Won't Upgrade

**Symptoms**: Ansible playbook fails during control plane upgrade

**Diagnosis**:

```bash
# Check Ansible logs
tail -f /var/log/ansible.log

# Check kubelet status on node
ssh ubuntu@<control-plane-node>
sudo systemctl status kubelet
sudo journalctl -u kubelet -n 100

# Check API server logs
kubectl logs -n kube-system kube-apiserver-<control-plane-node>
```

**Resolution**:

```bash
# Restart kubelet
ssh ubuntu@<control-plane-node>
sudo systemctl restart kubelet

# Verify kubelet configuration
sudo cat /var/lib/kubelet/config.yaml

# Re-run upgrade for specific node
ansible-playbook -i inventory/hosts.yaml \
  --limit=<control-plane-node> \
  upgrade-cluster.yml
```

### Worker Node Stuck in NotReady

**Symptoms**: Worker node shows NotReady after upgrade

**Diagnosis**:

```bash
# Check node conditions
kubectl describe node <worker-node>

# Check kubelet logs
ssh ubuntu@<worker-node>
sudo journalctl -u kubelet -n 100

# Check CNI plugin
kubectl get pods -n kube-system | grep calico
```

**Resolution**:

```bash
# Restart kubelet
ssh ubuntu@<worker-node>
sudo systemctl restart kubelet

# Restart CNI pods on node
kubectl delete pod -n kube-system -l k8s-app=calico-node --field-selector spec.nodeName=<worker-node>

# Verify node becomes Ready
kubectl wait --for=condition=Ready node/<worker-node> --timeout=300s
```

### Pods Stuck in Pending

**Symptoms**: Pods remain in Pending state after upgrade

**Diagnosis**:

```bash
# Check pod events
kubectl describe pod -n <namespace> <pod-name>

# Check node resources
kubectl describe nodes | grep -A 5 "Allocated resources"

# Check PVC status
kubectl get pvc -A
```

**Resolution**:

```bash
# If resource constraints, scale down non-critical workloads
kubectl scale deployment -n <namespace> <deployment> --replicas=0

# If PVC issues, check storage class
kubectl get storageclass

# If scheduling issues, check taints and tolerations
kubectl describe node <node> | grep Taints
```

### API Server Latency Increased

**Symptoms**: Slow kubectl responses after upgrade

**Diagnosis**:

```bash
# Check API server metrics
kubectl get --raw /metrics | grep apiserver_request_duration_seconds

# Check etcd latency
kubectl exec -n kube-system etcd-<control-plane-node> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/ssl/etcd/ca.pem \
  --cert=/etc/kubernetes/ssl/etcd/node-<node>.pem \
  --key=/etc/kubernetes/ssl/etcd/node-<node>-key.pem \
  check perf

# Check control plane resource usage
kubectl top pods -n kube-system
```

**Resolution**:

```bash
# Restart API server pods
kubectl delete pod -n kube-system kube-apiserver-<control-plane-node>

# Compact etcd database
kubectl exec -n kube-system etcd-<control-plane-node> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/ssl/etcd/ca.pem \
  --cert=/etc/kubernetes/ssl/etcd/node-<node>.pem \
  --key=/etc/kubernetes/ssl/etcd/node-<node>-key.pem \
  defrag

# Increase API server resources if needed
# Edit kube-apiserver manifest on control plane nodes
```

## Best Practices

### Planning
- Schedule upgrades during low-traffic periods
- Test upgrade in non-production environment first
- Review release notes thoroughly
- Document custom configurations that may be affected
- Coordinate with application teams

### Execution
- Upgrade one control plane node at a time
- Wait for etcd to stabilize between control plane upgrades
- Upgrade workers in small batches
- Monitor cluster health continuously during upgrade
- Keep rollback plan ready

### Validation
- Run comprehensive health checks after each phase
- Test critical applications after upgrade
- Monitor performance metrics for 24 hours post-upgrade
- Document any issues encountered
- Update runbook with lessons learned

### Communication
- Notify stakeholders before, during, and after upgrade
- Provide status updates at key milestones
- Document upgrade completion and any issues
- Schedule post-upgrade review meeting

## Related Documentation

- **[Disaster Recovery](../disaster-recovery.md)** - Backup and restore procedures
- **[Monitoring](../monitoring.md)** - Cluster health monitoring
- **[Troubleshooting](../../how-to/troubleshooting.md)** - General troubleshooting guide
- **[Configuration Reference](../../reference/configuration.md)** - Kubernetes version configuration

## Upgrade Schedule

Recommended upgrade cadence:

- **Patch versions**: Within 30 days of release
- **Minor versions**: Within 90 days of release
- **Security patches**: Within 7 days of release (critical)
- **End-of-life versions**: Upgrade before EOL date

Check Kubernetes version support:
- https://kubernetes.io/releases/
- https://endoflife.date/kubernetes

## Post-Upgrade Tasks

After successful upgrade:

1. Update documentation with new version
2. Archive upgrade logs and metrics
3. Conduct post-upgrade review meeting
4. Update monitoring dashboards if needed
5. Schedule next upgrade based on release calendar
6. Remove old backups after retention period
7. Update disaster recovery procedures if needed

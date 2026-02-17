# Upgrade Kubernetes

**Purpose:** For operators, shows how to safely upgrade Kubernetes version with minimal downtime and rollback capability.

This guide covers planning, testing, and executing Kubernetes version upgrades across control plane and worker nodes.

## Prerequisites

- Existing openCenter cluster
- Cluster backup (see [Backup and Restore](backup-and-restore.md))
- Access to cluster configuration
- Understanding of Kubernetes version skew policy

## Task Summary

Upgrade Kubernetes from current version to target version following Kubernetes version skew policy (max 1 minor version at a time) with control plane upgrade first, then worker nodes.

## Kubernetes Version Skew Policy

**Rules:**
- Control plane components: N to N+1 minor version
- Kubelet: N-2 to N minor version
- kubectl: N-1 to N+1 minor version

**Examples:**

```
Current: 1.32.0
Valid upgrades: 1.33.x
Invalid upgrades: 1.34.x (skip 1.33)

Current: 1.33.5
Valid upgrades: 1.33.6, 1.34.0
Invalid upgrades: 1.35.0 (skip 1.34)
```

**Evidence:** Kubernetes version skew policy documentation

## Pre-Upgrade Checklist

Before upgrading, complete these steps:

### 1. Review Release Notes

```bash
# Check Kubernetes release notes
open https://kubernetes.io/docs/setup/release/notes/

# Check for:
# - Breaking changes
# - Deprecated APIs
# - Required actions
# - Known issues
```

### 2. Check Current Version

```bash
# Check control plane version
kubectl version --short

# Check node versions
kubectl get nodes -o wide

# Check component versions
kubectl get pods -n kube-system -o wide
```

### 3. Backup Cluster

```bash
# Create etcd backup
kubectl create job --from=cronjob/etcd-backup etcd-backup-pre-upgrade -n kube-system

# Create Velero backup
velero backup create pre-upgrade-backup-$(date +%Y%m%d) --wait

# Verify backups
velero backup get
aws s3 ls s3://my-cluster-etcd-backups/ | grep pre-upgrade
```

### 4. Check Deprecated APIs

```bash
# Install kubectl-deprecations plugin
kubectl krew install deprecations

# Check for deprecated APIs
kubectl deprecations

# Or use pluto
pluto detect-all-in-cluster

# Fix deprecated APIs before upgrading
```

### 5. Verify Cluster Health

```bash
# Check node status
kubectl get nodes

# Check pod status
kubectl get pods -A | grep -v Running

# Check component health
kubectl get componentstatuses

# Check etcd health
kubectl exec -n kube-system etcd-<node> -- etcdctl endpoint health
```

## Upgrade Steps

### 1. Update Cluster Configuration

Edit cluster configuration with new Kubernetes version:

```bash
opencenter cluster edit my-cluster
```

```yaml
opencenter:
  cluster:
    kubernetes:
      version: "1.34.0"  # Updated from 1.33.5
    
    # Kubespray version (must support target Kubernetes version)
    kubespray_version: "v2.30.0"  # Updated if needed
```

**Version compatibility:**

Check Kubespray compatibility matrix:
```
Kubespray v2.29.1: Kubernetes 1.32.x - 1.33.x
Kubespray v2.30.0: Kubernetes 1.33.x - 1.34.x
```

**Evidence:** `internal/config/defaults.go:197-198` kubernetes.version

### 2. Validate Configuration

```bash
opencenter cluster validate my-cluster
```

**Expected output:**

```
✓ Schema validation passed
✓ Business rules validation passed
✓ Provider validation passed
✓ Kubernetes version upgrade valid: 1.33.5 → 1.34.0

Configuration is valid and ready for deployment.
```

### 3. Render Updated Configuration

```bash
opencenter cluster setup my-cluster --render
```

**What's updated:**

- Kubespray inventory (new Kubernetes version)
- Ansible variables (version-specific settings)
- Component configurations

### 4. Test in Dev/Staging First

**Critical:** Always test upgrades in non-production first.

```bash
# Upgrade dev cluster
opencenter cluster upgrade dev

# Verify dev cluster
kubectl get nodes --context dev
kubectl get pods -A --context dev

# Run application tests
./run-tests.sh --context dev

# If successful, proceed to staging
opencenter cluster upgrade staging

# Verify staging cluster
kubectl get nodes --context staging

# Run full test suite
./run-tests.sh --context staging

# If successful, proceed to production
```

### 5. Upgrade Control Plane

Upgrade control plane nodes first:

```bash
cd ~/my-cluster-gitops/infrastructure/clusters/my-cluster/inventory

# Upgrade control plane only
ansible-playbook -i inventory.yaml upgrade-cluster.yml \
  --limit=kube_control_plane \
  --extra-vars="upgrade_cluster_setup=true"
```

**What happens:**

```
Phase 1: Pre-upgrade checks (5 minutes)
  ✓ Verify cluster health
  ✓ Backup etcd
  ✓ Drain control plane nodes (one at a time)

Phase 2: Upgrade control plane (15-20 minutes per node)
  ✓ Upgrade kubeadm
  ✓ Upgrade control plane components
  ✓ Upgrade kubelet and kubectl
  ✓ Uncordon node

Phase 3: Post-upgrade verification (5 minutes)
  ✓ Verify API server
  ✓ Verify etcd cluster
  ✓ Verify control plane pods
```

**Monitor progress:**

```bash
# In another terminal, watch nodes
watch -n 5 'kubectl get nodes'

# Watch control plane pods
watch -n 5 'kubectl get pods -n kube-system'

# Check upgrade logs
tail -f ~/my-cluster-gitops/infrastructure/clusters/my-cluster/upgrade.log
```

### 6. Verify Control Plane Upgrade

```bash
# Check control plane version
kubectl version --short

# Expected output:
# Server Version: v1.34.0

# Check control plane nodes
kubectl get nodes -l node-role.kubernetes.io/control-plane

# Expected output: All control plane nodes on v1.34.0

# Check control plane pods
kubectl get pods -n kube-system -o wide

# Verify API server
kubectl get --raw /healthz

# Verify etcd
kubectl exec -n kube-system etcd-<node> -- etcdctl endpoint health
```

### 7. Upgrade Worker Nodes

Upgrade worker nodes in batches:

```bash
# Upgrade workers in batches (2 at a time for minimal disruption)
ansible-playbook -i inventory.yaml upgrade-cluster.yml \
  --limit=kube_node[0:1] \
  --extra-vars="upgrade_cluster_setup=true"

# Wait for batch to complete, then upgrade next batch
ansible-playbook -i inventory.yaml upgrade-cluster.yml \
  --limit=kube_node[2:3] \
  --extra-vars="upgrade_cluster_setup=true"

# Continue until all workers upgraded
```

**What happens per worker:**

```
1. Drain node (evict pods)
2. Upgrade kubelet and kubectl
3. Restart kubelet
4. Uncordon node
5. Wait for node Ready
```

**Duration:** 5-10 minutes per worker

### 8. Verify Worker Upgrade

```bash
# Check all nodes
kubectl get nodes -o wide

# Expected output: All nodes on v1.34.0

# Check pod distribution
kubectl get pods -A -o wide | grep -v Running

# Verify workloads
kubectl get deployments -A
kubectl get statefulsets -A
kubectl get daemonsets -A
```

### 9. Upgrade Add-ons

Upgrade cluster add-ons if needed:

```bash
# Check add-on versions
kubectl get pods -n kube-system -o yaml | grep image:

# Update add-ons via GitOps
cd ~/my-cluster-gitops
vim applications/overlays/my-cluster/services/<service>/override-values.yaml

# Commit and push
git add .
git commit -m "Upgrade add-ons for Kubernetes 1.34.0"
git push

# FluxCD will reconcile
flux reconcile kustomization <service>
```

### 10. Post-Upgrade Verification

```bash
# Run full cluster verification
kubectl get nodes
kubectl get pods -A
kubectl get services -A
kubectl get ingresses -A

# Check cluster health
kubectl get componentstatuses

# Run application smoke tests
./run-smoke-tests.sh

# Monitor for issues
kubectl get events -A --sort-by='.lastTimestamp' | tail -20
```

## Verification

Complete post-upgrade verification:

```bash
# 1. All nodes on new version
kubectl get nodes -o wide

# 2. All pods Running
kubectl get pods -A | grep -v Running | grep -v Completed

# 3. All services accessible
kubectl get services -A

# 4. Applications responding
curl https://my-app.example.com/health

# 5. Monitoring working
# Check Grafana dashboards

# 6. Logging working
# Check Loki logs

# 7. No errors in events
kubectl get events -A --field-selector type=Warning
```

## Rollback Procedure

If upgrade fails, rollback to previous version:

### Option 1: Rollback via Configuration

```bash
# 1. Revert configuration
opencenter cluster edit my-cluster

# Change back to previous version
opencenter:
  cluster:
    kubernetes:
      version: "1.33.5"  # Previous version

# 2. Render configuration
opencenter cluster setup my-cluster --render

# 3. Run downgrade playbook
cd ~/my-cluster-gitops/infrastructure/clusters/my-cluster/inventory
ansible-playbook -i inventory.yaml upgrade-cluster.yml \
  --extra-vars="upgrade_cluster_setup=true"
```

### Option 2: Restore from Backup

```bash
# 1. Restore etcd backup
# (See Backup and Restore guide)

# 2. Restore Velero backup
velero restore create rollback-restore \
  --from-backup pre-upgrade-backup-20260217

# 3. Verify cluster
kubectl get nodes
kubectl get pods -A
```

## Troubleshooting

### Control Plane Upgrade Fails

**Symptom:** Control plane node fails to upgrade

**Diagnosis:**

```bash
# Check upgrade logs
tail -100 ~/my-cluster-gitops/infrastructure/clusters/my-cluster/upgrade.log

# Check kubelet logs
ssh ubuntu@<control-plane-node>
sudo journalctl -u kubelet -n 100

# Check API server logs
kubectl logs -n kube-system kube-apiserver-<node>
```

**Common causes:**

1. **Incompatible version:** Skipped minor version
2. **Deprecated APIs:** Resources using deprecated APIs
3. **Insufficient resources:** Node out of disk/memory

**Solution:**

```bash
# Fix deprecated APIs
kubectl deprecations --output=json | jq -r '.[] | .name'
kubectl delete <resource> <name>

# Free up resources
kubectl delete pods --field-selector status.phase=Failed -A

# Retry upgrade
ansible-playbook -i inventory.yaml upgrade-cluster.yml --limit=<node>
```

### Worker Node Fails to Drain

**Symptom:** Worker node drain times out

**Diagnosis:**

```bash
# Check drain status
kubectl drain <node> --dry-run=client

# Check pods preventing drain
kubectl get pods -A --field-selector spec.nodeName=<node>
```

**Common causes:**

1. **PodDisruptionBudget:** PDB prevents eviction
2. **Local storage:** Pods with local volumes
3. **DaemonSets:** DaemonSet pods not ignored

**Solution:**

```bash
# Force drain (use with caution)
kubectl drain <node> \
  --ignore-daemonsets \
  --delete-emptydir-data \
  --force \
  --grace-period=300

# Or update PDB
kubectl edit pdb <pdb-name>
```

### Pods Not Starting After Upgrade

**Symptom:** Pods stuck in Pending or CrashLoopBackOff

**Diagnosis:**

```bash
# Check pod events
kubectl describe pod <pod-name> -n <namespace>

# Check pod logs
kubectl logs <pod-name> -n <namespace>
```

**Common causes:**

1. **API changes:** Deprecated APIs removed
2. **RBAC changes:** Permissions changed
3. **Resource constraints:** Insufficient resources

**Solution:**

```bash
# Update manifests for new API version
kubectl apply -f updated-manifest.yaml

# Update RBAC
kubectl apply -f updated-rbac.yaml

# Scale down temporarily
kubectl scale deployment <name> --replicas=0
kubectl scale deployment <name> --replicas=3
```

## Upgrade Timeline

**Small cluster (3 masters, 3 workers):**
- Pre-upgrade checks: 30 minutes
- Control plane upgrade: 45-60 minutes
- Worker upgrade: 30-45 minutes
- Post-upgrade verification: 30 minutes
- **Total: 2.5-3 hours**

**Large cluster (3 masters, 20 workers):**
- Pre-upgrade checks: 30 minutes
- Control plane upgrade: 45-60 minutes
- Worker upgrade: 2-3 hours (batches)
- Post-upgrade verification: 30 minutes
- **Total: 4-5 hours**

## Best Practices

1. **Test in dev/staging first:** Never upgrade production first
2. **Backup before upgrade:** Always create backup
3. **Check release notes:** Review breaking changes
4. **Fix deprecated APIs:** Before upgrading
5. **Upgrade one minor version:** Don't skip versions
6. **Upgrade during maintenance window:** Minimize user impact
7. **Monitor during upgrade:** Watch for issues
8. **Verify after upgrade:** Run full test suite
9. **Document upgrade:** Record issues and solutions
10. **Plan rollback:** Have rollback procedure ready

## Related Topics

- [Backup and Restore](backup-and-restore.md) - Backup before upgrade
- [Troubleshoot Deployment](troubleshoot-deployment.md) - Debug upgrade issues
- [Configuration Lifecycle](../explanation/configuration-lifecycle.md) - Configuration management
- [Platform Services](../reference/platform-services.md) - Service compatibility

---

## Evidence

This guide is based on:

- Kubernetes version: `internal/config/defaults.go:197-198`
- Kubespray upgrade: Kubespray upgrade-cluster.yml playbook
- Version skew policy: Kubernetes documentation
- Upgrade procedures: Kubespray documentation

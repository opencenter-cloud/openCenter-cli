# Node Replacement Runbook


## Table of Contents

- [Who This Is For](#who-this-is-for)
- [Prerequisites](#prerequisites)
- [Node Replacement Overview](#node-replacement-overview)
- [Pre-Flight Checks](#pre-flight-checks)
- [Worker Node Replacement](#worker-node-replacement)
- [Control Plane Node Replacement](#control-plane-node-replacement)
- [Troubleshooting](#troubleshooting)
- [Post-Replacement Verification](#post-replacement-verification)
- [Best Practices](#best-practices)
- [Related Documentation](#related-documentation)
- [Emergency Node Replacement](#emergency-node-replacement)
- [Node Replacement Checklist](#node-replacement-checklist)
**doc_type: how-to**

Step-by-step procedures for replacing failed or degraded nodes in openCenter-managed Kubernetes clusters, including both control plane and worker nodes.

## Who This Is For

Operations teams and SREs responsible for cluster maintenance and node lifecycle management. Use this runbook when a node fails, becomes degraded, or requires replacement for maintenance.

## Prerequisites

- Running openCenter cluster
- Access to cluster configuration and cloud provider credentials
- `kubectl` access with cluster-admin permissions
- SSH access to cluster nodes
- Backup of cluster state (see [Disaster Recovery](../disaster-recovery.md))
- Understanding of cluster topology

## Node Replacement Overview

Node replacement involves:

1. **Assessment** - Determine if replacement is necessary
2. **Preparation** - Backup data and prepare new node
3. **Workload Migration** - Move pods to healthy nodes
4. **Decommission** - Remove failed node from cluster
5. **Provision** - Create and configure new node
6. **Join** - Add new node to cluster
7. **Verification** - Confirm cluster health

**Estimated Time**:
- Worker node: 30-45 minutes
- Control plane node: 45-60 minutes per node

## Pre-Flight Checks

### Assess Node Health

Determine if node replacement is necessary:

```bash
# Check node status
kubectl get nodes

# Check node conditions
kubectl describe node <node-name> | grep -A 10 Conditions

# Check node resource usage
kubectl top node <node-name>

# Check node events
kubectl get events --field-selector involvedObject.name=<node-name> --sort-by='.lastTimestamp'

# Check kubelet status (if SSH accessible)
ssh ubuntu@<node-ip>
sudo systemctl status kubelet
sudo journalctl -u kubelet -n 100
```

**Replace node if**:
- Node status is NotReady for > 15 minutes
- Kubelet repeatedly crashing
- Hardware failure detected
- Disk full and cannot be cleaned
- Network connectivity issues persist
- Node performance severely degraded

### Identify Node Type

Determine if node is control plane or worker:

```bash
# Check node labels
kubectl get node <node-name> --show-labels | grep node-role

# Control plane nodes have label:
# node-role.kubernetes.io/control-plane=

# Worker nodes typically have label:
# node-role.kubernetes.io/worker=
```

### Check Cluster Capacity

Ensure cluster has capacity to handle workload migration:

```bash
# Check available capacity on other nodes
kubectl describe nodes | grep -A 5 "Allocated resources"

# Check pod distribution
kubectl get pods -A -o wide | awk '{print $8}' | sort | uniq -c

# Verify PodDisruptionBudgets
kubectl get pdb -A
```

**Do not proceed if**:
- Remaining nodes lack capacity for workloads
- PodDisruptionBudgets would be violated
- Only one control plane node remains (for control plane replacement)

## Worker Node Replacement

### Step 1: Cordon Node

Prevent new pods from scheduling on the node:

```bash
# Mark node as unschedulable
kubectl cordon <node-name>

# Verify node is cordoned
kubectl get nodes
# Node should show SchedulingDisabled
```

### Step 2: Drain Node

Migrate workloads to other nodes:

```bash
# Drain node gracefully
kubectl drain <node-name> \
  --ignore-daemonsets \
  --delete-emptydir-data \
  --force \
  --timeout=300s

# Monitor pod migration
watch kubectl get pods -A -o wide | grep <node-name>
```

**Expected behavior**:
- Pods with PodDisruptionBudgets respect constraints
- DaemonSet pods remain (ignored)
- Pods with emptyDir volumes are deleted
- Pods are rescheduled on healthy nodes

**If drain hangs**:

```bash
# Check which pods are blocking
kubectl get pods -A --field-selector spec.nodeName=<node-name>

# Force delete stuck pods (last resort)
kubectl delete pod -n <namespace> <pod-name> --force --grace-period=0
```

### Step 3: Delete Node from Cluster

Remove node from Kubernetes:

```bash
# Delete node object
kubectl delete node <node-name>

# Verify node is removed
kubectl get nodes
```

### Step 4: Decommission Infrastructure

Remove node from cloud provider:

**OpenStack**:

```bash
# List instances
openstack server list | grep <node-name>

# Delete instance
openstack server delete <node-name>

# Verify deletion
openstack server show <node-name>
# Should return: No server with a name or ID of '<node-name>' exists
```

**AWS**:

```bash
# List instances
aws ec2 describe-instances --filters "Name=tag:Name,Values=<node-name>"

# Terminate instance
aws ec2 terminate-instances --instance-ids <instance-id>

# Verify termination
aws ec2 describe-instances --instance-ids <instance-id>
```

### Step 5: Update Cluster Configuration

Update worker count in configuration:

```yaml
# Edit cluster configuration
opencenter:
  cluster:
    kubernetes:
      worker_count: 2  # Keep same count, will provision replacement
```

**Note**: Keep the same worker count. The replacement node will be provisioned automatically.

### Step 6: Provision New Node

Create new worker node:

```bash
# Regenerate cluster manifests
openCenter cluster setup my-cluster --force

# Apply infrastructure changes
cd ~/.config/openCenter/clusters/myorg/my-cluster
openCenter cluster apply my-cluster

# Monitor node provisioning
# OpenStack:
openstack server list | grep worker

# AWS:
aws ec2 describe-instances --filters "Name=tag:Cluster,Values=my-cluster"
```

**Expected timeline**:
- Instance creation: 2-5 minutes
- OS boot and initialization: 2-3 minutes
- Kubelet registration: 1-2 minutes
- Total: 5-10 minutes

### Step 7: Verify New Node

Confirm new node joined cluster:

```bash
# Wait for node to appear
kubectl get nodes -w

# Check node status
kubectl get node <new-node-name>
# Should show Ready

# Check node labels
kubectl get node <new-node-name> --show-labels

# Verify node capacity
kubectl describe node <new-node-name> | grep -A 5 "Allocated resources"

# Check kubelet version
kubectl get node <new-node-name> -o jsonpath='{.status.nodeInfo.kubeletVersion}'
```

### Step 8: Verify Workload Distribution

Ensure pods are properly distributed:

```bash
# Check pod distribution across nodes
kubectl get pods -A -o wide | awk '{print $8}' | sort | uniq -c

# Verify critical workloads are running
kubectl get pods -n production
kubectl get pods -n kube-system

# Check for pending pods
kubectl get pods -A --field-selector=status.phase=Pending
# Should return no results
```

## Control Plane Node Replacement

**WARNING**: Control plane node replacement is more complex and risky. Always maintain at least 2 healthy control plane nodes during replacement.

### Prerequisites for Control Plane Replacement

- Minimum 3 control plane nodes in cluster
- At least 2 control plane nodes must be healthy
- etcd cluster must be healthy
- Recent etcd backup available

### Step 1: Verify etcd Health

Ensure etcd cluster is healthy before proceeding:

```bash
# Check etcd member list
kubectl exec -n kube-system etcd-<healthy-control-plane> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  member list -w table

# Check etcd health
kubectl exec -n kube-system etcd-<healthy-control-plane> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  endpoint health -w table

# All members should show healthy
```

### Step 2: Backup etcd

Create etcd backup before proceeding:

```bash
# SSH to healthy control plane node
ssh ubuntu@<healthy-control-plane>

# Create etcd snapshot
sudo ETCDCTL_API=3 etcdctl snapshot save /tmp/etcd-backup-$(date +%Y%m%d-%H%M%S).db \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key

# Verify backup
sudo ETCDCTL_API=3 etcdctl snapshot status /tmp/etcd-backup-*.db -w table

# Copy backup to safe location
exit
scp ubuntu@<healthy-control-plane>:/tmp/etcd-backup-*.db ~/backups/
```

### Step 3: Remove Failed Control Plane Node

Remove node from etcd cluster:

```bash
# Get etcd member ID of failed node
kubectl exec -n kube-system etcd-<healthy-control-plane> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  member list

# Remove failed member from etcd
kubectl exec -n kube-system etcd-<healthy-control-plane> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  member remove <member-id>

# Verify member removed
kubectl exec -n kube-system etcd-<healthy-control-plane> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  member list
```

### Step 4: Delete Node from Kubernetes

Remove node from cluster:

```bash
# Delete node object
kubectl delete node <control-plane-node-name>

# Verify node removed
kubectl get nodes
```

### Step 5: Decommission Infrastructure

Remove control plane node from cloud provider:

```bash
# OpenStack
openstack server delete <control-plane-node-name>

# AWS
aws ec2 terminate-instances --instance-ids <instance-id>
```

### Step 6: Provision New Control Plane Node

Create new control plane node:

```bash
# Update cluster configuration (keep same master_count)
opencenter:
  cluster:
    kubernetes:
      master_count: 3  # Keep same count

# Regenerate manifests
openCenter cluster setup my-cluster --force

# Apply infrastructure changes
openCenter cluster apply my-cluster

# Monitor provisioning
# OpenStack:
openstack server list | grep control-plane

# AWS:
aws ec2 describe-instances --filters "Name=tag:Role,Values=control-plane"
```

### Step 7: Join New Control Plane Node

Add new node to etcd cluster and Kubernetes:

```bash
# SSH to new control plane node
ssh ubuntu@<new-control-plane-ip>

# Join node to cluster (Kubespray handles this automatically)
# Verify kubelet is running
sudo systemctl status kubelet

# Check node joined cluster
kubectl get nodes
# New control plane node should appear

# Verify etcd member added
kubectl exec -n kube-system etcd-<healthy-control-plane> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  member list -w table
```

### Step 8: Verify Control Plane Health

Confirm control plane is fully operational:

```bash
# Check all control plane nodes
kubectl get nodes -l node-role.kubernetes.io/control-plane

# Check control plane pods
kubectl get pods -n kube-system | grep -E "apiserver|controller|scheduler|etcd"

# Verify etcd cluster health
kubectl exec -n kube-system etcd-<new-control-plane> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  endpoint health -w table

# Test API server
kubectl cluster-info
kubectl get nodes
kubectl get pods -A

# Check etcd performance
kubectl exec -n kube-system etcd-<new-control-plane> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  check perf
```

## Troubleshooting

### Node Won't Drain

**Symptoms**: Drain command hangs or times out

**Diagnosis**:

```bash
# Check which pods are blocking
kubectl get pods -A --field-selector spec.nodeName=<node-name>

# Check PodDisruptionBudgets
kubectl get pdb -A

# Check pod status
kubectl describe pod -n <namespace> <pod-name>
```

**Resolution**:

```bash
# Increase drain timeout
kubectl drain <node-name> \
  --ignore-daemonsets \
  --delete-emptydir-data \
  --force \
  --timeout=600s

# If still stuck, force delete pods
kubectl delete pod -n <namespace> <pod-name> --force --grace-period=0

# Or skip drain if node is completely unresponsive
kubectl delete node <node-name> --force
```

### New Node Won't Join Cluster

**Symptoms**: New node doesn't appear in `kubectl get nodes`

**Diagnosis**:

```bash
# SSH to new node
ssh ubuntu@<new-node-ip>

# Check kubelet status
sudo systemctl status kubelet

# Check kubelet logs
sudo journalctl -u kubelet -n 100

# Check network connectivity to API server
curl -k https://api.cluster.example.com:6443

# Check node can resolve cluster DNS
nslookup kubernetes.default.svc.cluster.local
```

**Resolution**:

```bash
# Restart kubelet
sudo systemctl restart kubelet

# Check kubelet configuration
sudo cat /var/lib/kubelet/config.yaml

# Verify bootstrap token (if using)
sudo cat /etc/kubernetes/bootstrap-kubelet.conf

# Re-run Kubespray join playbook
cd ~/.config/openCenter/clusters/myorg/my-cluster/infrastructure/clusters/my-cluster
ansible-playbook -i inventory/hosts.yaml \
  --limit=<new-node-name> \
  cluster.yml
```

### etcd Member Won't Join

**Symptoms**: New control plane node's etcd member not healthy

**Diagnosis**:

```bash
# Check etcd member list
kubectl exec -n kube-system etcd-<healthy-control-plane> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  member list -w table

# SSH to new control plane node
ssh ubuntu@<new-control-plane>

# Check etcd logs
sudo journalctl -u etcd -n 100

# Check etcd configuration
sudo cat /etc/etcd/etcd.conf
```

**Resolution**:

```bash
# Remove and re-add etcd member
kubectl exec -n kube-system etcd-<healthy-control-plane> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  member remove <member-id>

# Re-run Kubespray to rejoin
cd ~/.config/openCenter/clusters/myorg/my-cluster/infrastructure/clusters/my-cluster
ansible-playbook -i inventory/hosts.yaml \
  --limit=<new-control-plane> \
  cluster.yml
```

### Workloads Not Rescheduling

**Symptoms**: Pods remain pending after node drain

**Diagnosis**:

```bash
# Check pending pods
kubectl get pods -A --field-selector=status.phase=Pending

# Check pod events
kubectl describe pod -n <namespace> <pod-name>

# Check node capacity
kubectl describe nodes | grep -A 5 "Allocated resources"

# Check resource requests
kubectl get pod -n <namespace> <pod-name> -o json | \
  jq '.spec.containers[].resources'
```

**Resolution**:

```bash
# If insufficient capacity, add more nodes
# Update cluster configuration
opencenter:
  cluster:
    kubernetes:
      worker_count: 3  # Increase count

# If resource requests too high, adjust
kubectl set resources deployment -n <namespace> <deployment> \
  --requests=cpu=500m,memory=512Mi

# If node selector/affinity preventing scheduling, update
kubectl patch deployment -n <namespace> <deployment> \
  --type=json -p='[{"op": "remove", "path": "/spec/template/spec/nodeSelector"}]'
```

## Post-Replacement Verification

### Comprehensive Health Check

Verify cluster is fully operational:

```bash
# Check all nodes
kubectl get nodes -o wide
# All nodes should be Ready

# Check system pods
kubectl get pods -n kube-system
# All pods should be Running

# Check workload pods
kubectl get pods -A | grep -v "Running\|Completed"
# Should return no results (except jobs)

# Verify etcd health (for control plane replacement)
kubectl exec -n kube-system etcd-<control-plane> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  endpoint health -w table

# Check cluster info
kubectl cluster-info

# Test API server
kubectl get nodes
kubectl get pods -A

# Check resource usage
kubectl top nodes
kubectl top pods -A
```

### Application Validation

Test critical applications:

```bash
# Check application deployments
kubectl get deployments -A

# Verify application endpoints
curl -I https://app.example.com/health

# Check application logs
kubectl logs -n production deployment/app --tail=50

# Run application health checks
kubectl exec -n production <app-pod> -- /health-check.sh
```

### Update Documentation

Document node replacement:

```markdown
# Node Replacement Log

**Date**: 2026-01-19
**Operator**: John Doe
**Node Replaced**: worker-2
**Reason**: Disk failure
**Downtime**: None (rolling replacement)

**Timeline**:
- 14:00: Node failure detected
- 14:05: Node cordoned and drained
- 14:15: Node removed from cluster
- 14:20: New node provisioned
- 14:30: New node joined cluster
- 14:35: Workloads redistributed
- 14:40: Verification complete

**Issues Encountered**: None

**Lessons Learned**: 
- Drain completed smoothly with PodDisruptionBudgets
- New node provisioning took 10 minutes as expected
- No user-facing impact
```

## Best Practices

### Planning
- Always maintain minimum node count for redundancy
- Schedule node replacements during maintenance windows
- Verify cluster capacity before draining nodes
- Create backups before control plane node replacement
- Test node replacement in non-production first

### Execution
- Drain nodes gracefully with appropriate timeouts
- Respect PodDisruptionBudgets
- Replace control plane nodes one at a time
- Verify etcd health between control plane replacements
- Monitor workload distribution during replacement

### Verification
- Confirm new node is healthy before proceeding
- Verify workloads are properly distributed
- Test application functionality after replacement
- Monitor cluster performance for 24 hours
- Document replacement procedure and issues

### Automation
- Use infrastructure-as-code for node provisioning
- Implement automated health checks
- Configure node auto-repair where available
- Set up alerts for node failures
- Maintain runbooks for common scenarios

## Related Documentation

- **[Disaster Recovery](../disaster-recovery.md)** - Backup and restore procedures
- **[Monitoring](../monitoring.md)** - Node health monitoring
- **[Capacity Planning](../capacity-planning.md)** - Node sizing and scaling
- **[Incident Response](../incident-response.md)** - Node failure response
- **[Cluster Upgrade](cluster-upgrade.md)** - Node upgrade procedures

## Emergency Node Replacement

If multiple nodes fail simultaneously:

1. **Assess impact** - Determine if cluster is still operational
2. **Prioritize control plane** - Replace control plane nodes first
3. **Restore from backup** - If cluster is non-functional, restore from backup
4. **Escalate** - Follow incident response procedures (SEV1)
5. **Document** - Record all actions for post-incident review

See [Incident Response](../incident-response.md) for emergency procedures.

## Node Replacement Checklist

### Pre-Replacement
- [ ] Verify node health and determine replacement is necessary
- [ ] Check cluster has capacity for workload migration
- [ ] Create backup of cluster state
- [ ] For control plane: Verify etcd health and create etcd backup
- [ ] Notify stakeholders of planned maintenance

### During Replacement
- [ ] Cordon node to prevent new workloads
- [ ] Drain node gracefully
- [ ] Delete node from Kubernetes
- [ ] Decommission infrastructure
- [ ] Provision new node
- [ ] Verify new node joins cluster
- [ ] For control plane: Verify etcd member joins and is healthy

### Post-Replacement
- [ ] Verify all nodes are Ready
- [ ] Check workload distribution
- [ ] Test application functionality
- [ ] Monitor cluster performance
- [ ] Update documentation
- [ ] Conduct post-replacement review

# Add Worker Pools

**Purpose:** For operators, shows how to add additional worker node pools with custom configurations to scale cluster capacity.

This guide covers adding worker pools with different instance types, taints, labels, and configurations to support diverse workload requirements.

## Prerequisites

- Existing openCenter cluster
- Cluster configuration file
- Infrastructure provider access (OpenStack, VMware, etc.)
- Basic understanding of Kubernetes nodes

## Task Summary

Add a new worker pool to your cluster with custom configuration (instance type, count, labels, taints) to support specific workload requirements like GPU workloads, high-memory applications, or dedicated tenant isolation.

## Steps

### 1. Edit Cluster Configuration

Open your cluster configuration file:

```bash
opencenter cluster edit my-cluster
```

### 2. Add Worker Pool Configuration

Add a new worker pool to the configuration:

```yaml
opencenter:
  cluster:
    # Existing default worker pool
    worker_count: 3
    worker_flavor: "gp.0.4.16"
    
    # Additional worker pools
    additional_server_pools:
      # High-memory pool for data processing
      - name: highmem-pool
        count: 2
        flavor: "gp.0.8.64"  # 8 vCPU, 64 GB RAM
        labels:
          workload-type: data-processing
          pool: highmem
        taints:
          - key: workload-type
            value: data-processing
            effect: NoSchedule
        volume_size: 100  # GB
        volume_type: "HA-Standard"
      
      # GPU pool for ML workloads
      - name: gpu-pool
        count: 2
        flavor: "gpu.0.4.16"  # GPU-enabled instance
        labels:
          workload-type: ml
          pool: gpu
          nvidia.com/gpu: "true"
        taints:
          - key: nvidia.com/gpu
            value: "true"
            effect: NoSchedule
        volume_size: 200  # GB for ML datasets
        volume_type: "HA-Standard"
      
      # Dedicated pool for tenant isolation
      - name: tenant-a-pool
        count: 3
        flavor: "gp.0.4.16"
        labels:
          tenant: tenant-a
          pool: dedicated
        taints:
          - key: tenant
            value: tenant-a
            effect: NoExecute
        volume_size: 40  # GB
        volume_type: "HA-Standard"
```

**Configuration options:**

- `name`: Pool identifier (must be unique)
- `count`: Number of nodes in pool
- `flavor`: Instance type/flavor
- `labels`: Node labels for pod scheduling
- `taints`: Node taints for pod toleration
- `volume_size`: Root volume size in GB
- `volume_type`: Storage type (provider-specific)

**Evidence:** `schema/cluster.schema.json` additional_server_pools section

### 3. Validate Configuration

Validate the updated configuration:

```bash
opencenter cluster validate my-cluster
```

**Expected output:**

```
✓ Schema validation passed
✓ Business rules validation passed
✓ Provider validation passed
  - Worker pool 'highmem-pool' configuration valid
  - Worker pool 'gpu-pool' configuration valid
  - Worker pool 'tenant-a-pool' configuration valid
  - Total worker nodes: 10 (3 default + 2 highmem + 2 gpu + 3 tenant-a)

Configuration is valid and ready for deployment.
```

### 4. Render Updated Configuration

Generate updated infrastructure configuration:

```bash
opencenter cluster setup my-cluster --render
```

**What's updated:**

- Terraform/OpenTofu configuration (new VM resources)
- Kubespray inventory (new node entries)
- Node labels and taints configuration

### 5. Apply Infrastructure Changes

Apply the infrastructure changes:

**For OpenStack/AWS (Terraform):**

```bash
cd ~/my-cluster-gitops/infrastructure/clusters/my-cluster
terraform plan  # Review changes
terraform apply  # Provision new nodes
```

**For VMware (Manual):**

1. Provision new VMs according to pool specifications
2. Update configuration with VM IPs
3. Re-run setup to update inventory

**For Kind (Not Supported):**

Kind doesn't support additional worker pools. Use single worker pool only.

### 6. Join Nodes to Cluster

Run Kubespray to join new nodes:

```bash
cd ~/my-cluster-gitops/infrastructure/clusters/my-cluster/inventory
ansible-playbook -i inventory.yaml scale.yml
```

**What happens:**

- New nodes are configured with Kubernetes components
- Nodes join the cluster
- Labels and taints are applied
- Nodes become Ready

**Duration:** 5-10 minutes per node

### 7. Verify Worker Pools

Verify new nodes are added:

```bash
# Check all nodes
kubectl get nodes

# Expected output:
# NAME                        STATUS   ROLES           AGE   VERSION
# my-cluster-master-1         Ready    control-plane   30d   v1.33.5
# my-cluster-master-2         Ready    control-plane   30d   v1.33.5
# my-cluster-master-3         Ready    control-plane   30d   v1.33.5
# my-cluster-worker-1         Ready    <none>          30d   v1.33.5
# my-cluster-worker-2         Ready    <none>          30d   v1.33.5
# my-cluster-worker-3         Ready    <none>          30d   v1.33.5
# my-cluster-highmem-pool-1   Ready    <none>          5m    v1.33.5
# my-cluster-highmem-pool-2   Ready    <none>          5m    v1.33.5
# my-cluster-gpu-pool-1       Ready    <none>          5m    v1.33.5
# my-cluster-gpu-pool-2       Ready    <none>          5m    v1.33.5
# my-cluster-tenant-a-pool-1  Ready    <none>          5m    v1.33.5
# my-cluster-tenant-a-pool-2  Ready    <none>          5m    v1.33.5
# my-cluster-tenant-a-pool-3  Ready    <none>          5m    v1.33.5

# Check node labels
kubectl get nodes --show-labels | grep highmem-pool

# Expected output: workload-type=data-processing,pool=highmem

# Check node taints
kubectl describe node my-cluster-highmem-pool-1 | grep Taints

# Expected output: workload-type=data-processing:NoSchedule
```

### 8. Deploy Workloads to Specific Pools

Deploy workloads to specific pools using node selectors and tolerations:

**Example: Deploy to high-memory pool**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: data-processor
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: data-processor
  template:
    metadata:
      labels:
        app: data-processor
    spec:
      # Node selector targets high-memory pool
      nodeSelector:
        pool: highmem
      
      # Toleration allows scheduling on tainted nodes
      tolerations:
      - key: workload-type
        operator: Equal
        value: data-processing
        effect: NoSchedule
      
      containers:
      - name: processor
        image: my-data-processor:latest
        resources:
          requests:
            memory: "32Gi"  # Requires high-memory node
            cpu: "4"
```

**Example: Deploy to GPU pool**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ml-training
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ml-training
  template:
    metadata:
      labels:
        app: ml-training
    spec:
      nodeSelector:
        pool: gpu
      
      tolerations:
      - key: nvidia.com/gpu
        operator: Equal
        value: "true"
        effect: NoSchedule
      
      containers:
      - name: trainer
        image: my-ml-trainer:latest
        resources:
          requests:
            nvidia.com/gpu: 1  # Request GPU
            memory: "16Gi"
            cpu: "4"
```

**Example: Deploy to tenant-dedicated pool**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tenant-a-app
  namespace: tenant-a
spec:
  replicas: 3
  selector:
    matchLabels:
      app: tenant-a-app
  template:
    metadata:
      labels:
        app: tenant-a-app
    spec:
      nodeSelector:
        tenant: tenant-a
      
      tolerations:
      - key: tenant
        operator: Equal
        value: tenant-a
        effect: NoExecute
      
      containers:
      - name: app
        image: tenant-a-app:latest
```

## Verification

Verify worker pools are functioning:

```bash
# 1. Check all nodes are Ready
kubectl get nodes

# 2. Check node labels are applied
kubectl get nodes -L pool,workload-type,tenant

# 3. Check node taints are applied
kubectl get nodes -o json | jq '.items[] | {name: .metadata.name, taints: .spec.taints}'

# 4. Deploy test workload to specific pool
kubectl apply -f test-deployment.yaml

# 5. Verify pods are scheduled on correct nodes
kubectl get pods -o wide

# 6. Check node resource usage
kubectl top nodes
```

**Success criteria:**

- All new nodes are Ready
- Labels are applied correctly
- Taints are applied correctly
- Pods schedule to correct pools
- Pods respect node selectors and tolerations

## Troubleshooting

### Nodes Not Joining Cluster

**Symptom:** New nodes don't appear in `kubectl get nodes`

**Diagnosis:**

```bash
# Check Ansible logs
tail -100 ~/my-cluster-gitops/infrastructure/clusters/my-cluster/ansible.log

# SSH to node and check kubelet
ssh ubuntu@<node-ip>
sudo systemctl status kubelet
sudo journalctl -u kubelet -n 100
```

**Common causes:**

1. **Network connectivity:** Node can't reach control plane
2. **Firewall rules:** Required ports blocked
3. **Kubespray inventory:** Node not in inventory

**Solution:**

```bash
# Verify network connectivity
ssh ubuntu@<node-ip>
ping <control-plane-ip>

# Verify firewall rules
sudo iptables -L

# Re-run Kubespray
cd ~/my-cluster-gitops/infrastructure/clusters/my-cluster/inventory
ansible-playbook -i inventory.yaml scale.yml
```

### Labels Not Applied

**Symptom:** Node labels missing

**Diagnosis:**

```bash
# Check node labels
kubectl get nodes --show-labels

# Check Kubespray inventory
cat ~/my-cluster-gitops/infrastructure/clusters/my-cluster/inventory/inventory.yaml
```

**Solution:**

```bash
# Apply labels manually
kubectl label node my-cluster-highmem-pool-1 workload-type=data-processing pool=highmem

# Or re-run Kubespray
ansible-playbook -i inventory.yaml scale.yml
```

### Taints Not Applied

**Symptom:** Node taints missing

**Diagnosis:**

```bash
# Check node taints
kubectl describe node my-cluster-highmem-pool-1 | grep Taints
```

**Solution:**

```bash
# Apply taints manually
kubectl taint node my-cluster-highmem-pool-1 workload-type=data-processing:NoSchedule

# Or re-run Kubespray
ansible-playbook -i inventory.yaml scale.yml
```

### Pods Not Scheduling to Pool

**Symptom:** Pods remain Pending or schedule to wrong nodes

**Diagnosis:**

```bash
# Check pod events
kubectl describe pod <pod-name>

# Common errors:
# - "0/10 nodes are available: 3 node(s) had untolerated taint"
# - "0/10 nodes are available: 7 node(s) didn't match Pod's node affinity/selector"
```

**Solution:**

```bash
# Verify node selector matches node labels
kubectl get nodes -L pool,workload-type

# Verify tolerations match node taints
kubectl describe node <node-name> | grep Taints

# Update deployment with correct node selector and tolerations
kubectl edit deployment <deployment-name>
```

### Insufficient Resources

**Symptom:** Terraform fails with quota exceeded

**Diagnosis:**

```bash
# Check provider quotas
# OpenStack:
openstack quota show

# VMware:
# Check vCenter resource pools
```

**Solution:**

- Request quota increase from provider
- Reduce worker pool size
- Use smaller instance flavors

## Common Use Cases

### Use Case 1: High-Memory Workloads

**Scenario:** Data processing applications require 64 GB RAM

**Configuration:**

```yaml
additional_server_pools:
  - name: highmem-pool
    count: 2
    flavor: "gp.0.8.64"  # 8 vCPU, 64 GB RAM
    labels:
      pool: highmem
    taints:
      - key: pool
        value: highmem
        effect: NoSchedule
```

**Deployment:**

```yaml
nodeSelector:
  pool: highmem
tolerations:
- key: pool
  value: highmem
  effect: NoSchedule
resources:
  requests:
    memory: "48Gi"
```

### Use Case 2: GPU Workloads

**Scenario:** ML training requires GPU acceleration

**Configuration:**

```yaml
additional_server_pools:
  - name: gpu-pool
    count: 2
    flavor: "gpu.0.4.16"  # GPU-enabled
    labels:
      pool: gpu
      nvidia.com/gpu: "true"
    taints:
      - key: nvidia.com/gpu
        value: "true"
        effect: NoSchedule
```

**Deployment:**

```yaml
nodeSelector:
  pool: gpu
tolerations:
- key: nvidia.com/gpu
  value: "true"
  effect: NoSchedule
resources:
  requests:
    nvidia.com/gpu: 1
```

### Use Case 3: Tenant Isolation

**Scenario:** Dedicated nodes for specific tenant

**Configuration:**

```yaml
additional_server_pools:
  - name: tenant-a-pool
    count: 3
    flavor: "gp.0.4.16"
    labels:
      tenant: tenant-a
    taints:
      - key: tenant
        value: tenant-a
        effect: NoExecute  # Evict non-tenant pods
```

**Deployment:**

```yaml
nodeSelector:
  tenant: tenant-a
tolerations:
- key: tenant
  value: tenant-a
  effect: NoExecute
```

### Use Case 4: Spot/Preemptible Instances

**Scenario:** Cost optimization with spot instances

**Configuration:**

```yaml
additional_server_pools:
  - name: spot-pool
    count: 5
    flavor: "gp.0.4.16"
    spot: true  # Provider-specific
    labels:
      pool: spot
      workload-type: batch
    taints:
      - key: spot
        value: "true"
        effect: NoSchedule
```

**Deployment:**

```yaml
nodeSelector:
  pool: spot
tolerations:
- key: spot
  value: "true"
  effect: NoSchedule
# Use PodDisruptionBudget for resilience
```

## Best Practices

1. **Use descriptive pool names:** `highmem-pool`, `gpu-pool`, not `pool-1`, `pool-2`
2. **Apply consistent labels:** Use standard label keys across all pools
3. **Use taints for dedicated pools:** Prevent accidental scheduling
4. **Document pool purpose:** Add comments in configuration file
5. **Monitor pool utilization:** Track resource usage per pool
6. **Plan for growth:** Size pools based on projected workload
7. **Test before production:** Validate pool configuration in dev/staging

## Related Topics

- [Configure Networking](configure-networking.md) - Network configuration for worker pools
- [Upgrade Kubernetes](upgrade-kubernetes.md) - Upgrade nodes in worker pools
- [Troubleshoot Deployment](troubleshoot-deployment.md) - Debug node issues
- [Configuration Schema](../reference/configuration-schema.md) - Complete configuration reference

---

## Evidence

This guide is based on:

- Worker pool configuration: `schema/cluster.schema.json` additional_server_pools
- Kubespray scaling: Kubespray scale.yml playbook
- Node labels and taints: Kubernetes node management
- Provider configuration: `internal/config/defaults.go`

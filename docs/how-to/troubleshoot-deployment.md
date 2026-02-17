# Troubleshoot Deployment

**Purpose:** For all users, shows how to diagnose and fix common deployment issues, covering validation errors through service failures.

Deployment issues can occur at any stage. This guide helps you diagnose problems and find solutions quickly.

## Prerequisites

- openCenter CLI installed
- Cluster configuration created
- Basic understanding of Kubernetes (helpful)

## Diagnostic Commands

### Check Cluster Status

```bash
# Validate configuration
opencenter cluster validate --verbose

# Check node status
kubectl get nodes

# Check pod status across all namespaces
kubectl get pods -A

# Check FluxCD status
kubectl get kustomizations -n flux-system

# Check service deployments
kubectl get helmreleases -A
```

### Generate Debug Configuration

Create complete configuration for debugging:

```bash
opencenter cluster validate --generate-debug-config --output-dir /tmp
```

This creates a file with all configuration values, including defaults and computed fields.

## Validation Errors

### Missing Required Fields

**Error:**
```
ERROR: Required field missing: opencenter.infrastructure.cloud.openstack.application_credential_id
```

**Diagnosis:** Configuration is incomplete.

**Solution:** Add required field:

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        application_credential_id: "your-app-cred-id"

secrets:
  global:
    openstack:
      application_credential_id: "your-app-cred-id"
```

### Schema Version Mismatch

**Error:**
```
ERROR: Unsupported schema version: 1.0
Only v2 configurations (schema_version: "2.0") are supported
```

**Diagnosis:** Configuration uses old schema format.

**Solution:** Migrate configuration:

```bash
opencenter cluster migrate my-cluster
```

Or manually update:

```yaml
schema_version: "2.0"  # Change from "1.0"
```

### Invalid CIDR Ranges

**Error:**
```
ERROR: Subnet overlap detected
Field: opencenter.cluster.kubernetes.subnet_services
Overlaps with: opencenter.cluster.kubernetes.subnet_pods
```

**Diagnosis:** Network ranges overlap.

**Solution:** Use non-overlapping ranges:

```yaml
opencenter:
  cluster:
    kubernetes:
      subnet_pods: "10.42.0.0/16"
      subnet_services: "10.43.0.0/16"  # Different range
```

## Infrastructure Provisioning Errors

### Terraform/OpenTofu Failures

**Error:** Infrastructure provisioning fails during bootstrap.

**Diagnosis:** Check Terraform logs:

```bash
cd <git_dir>/infrastructure/clusters/<cluster>
cat terraform.log
```

**Common Causes:**

1. **Insufficient Quotas:**
   ```
   Error: Error creating OpenStack server: Quota exceeded for instances
   ```
   
   Solution: Increase OpenStack quotas or reduce node count:
   ```yaml
   opencenter:
     cluster:
       kubernetes:
         master_count: 1  # Reduce from 3
         worker_count: 1  # Reduce from 2
   ```

2. **Invalid Image ID:**
   ```
   Error: Image not found: 799dcf97-3656-4361-8187-13ab1b295e33
   ```
   
   Solution: Use valid image ID:
   ```bash
   openstack image list
   opencenter cluster config set opencenter.infrastructure.cloud.openstack.image_id <valid-id>
   ```

3. **Network Not Found:**
   ```
   Error: Network not found: floating_network_id
   ```
   
   Solution: Set correct network ID:
   ```bash
   openstack network list
   opencenter cluster config set opencenter.infrastructure.cloud.openstack.networking.floating_network_id <network-id>
   ```

### SSH Connection Failures

**Error:** Ansible can't connect to nodes during Kubespray deployment.

**Diagnosis:** Check SSH connectivity:

```bash
ssh -i <ssh-key> ubuntu@<node-ip>
```

**Common Causes:**

1. **Wrong SSH Key:**
   
   Solution: Verify SSH key in configuration:
   ```yaml
   opencenter:
     cluster:
       ssh_authorized_keys:
         - "ssh-ed25519 AAAAC3... your-key"
   ```

2. **Security Group Rules:**
   
   Solution: Ensure port 22 is open:
   ```bash
   openstack security group rule list <security-group>
   ```

3. **Floating IP Not Assigned:**
   
   Solution: Check floating IPs:
   ```bash
   openstack server list
   ```

## Kubernetes Deployment Errors

### Kubespray Failures

**Error:** Kubernetes deployment fails during bootstrap.

**Diagnosis:** Check Ansible logs:

```bash
cd <git_dir>/infrastructure/clusters/<cluster>
cat ansible.log
```

**Common Causes:**

1. **Container Runtime Failure:**
   ```
   TASK [container-engine/containerd : Install containerd] failed
   ```
   
   Solution: Check node connectivity and package repositories:
   ```bash
   ssh ubuntu@<node-ip>
   sudo apt update
   ```

2. **Insufficient Resources:**
   ```
   TASK [kubernetes/control-plane : kubeadm | Initialize first master] failed
   ```
   
   Solution: Increase node resources or use larger flavors:
   ```yaml
   opencenter:
     cluster:
       kubernetes:
         flavor_master: "gp.0.8.16"  # Larger flavor
   ```

3. **Network Plugin Failure:**
   ```
   TASK [network_plugin/calico : Calico | Create calico manifests] failed
   ```
   
   Solution: Check CNI configuration:
   ```yaml
   opencenter:
     cluster:
       kubernetes:
         network_plugin:
           calico:
             enabled: true
             cni_iface: "enp3s0"  # Correct interface
   ```

### Node Not Ready

**Error:** Nodes show NotReady status.

**Diagnosis:**

```bash
kubectl get nodes
kubectl describe node <node-name>
```

**Common Causes:**

1. **CNI Not Running:**
   ```
   Conditions:
     Ready   False   ... network plugin not ready
   ```
   
   Solution: Check CNI pods:
   ```bash
   kubectl get pods -n kube-system | grep calico
   kubectl logs -n kube-system <calico-pod>
   ```

2. **Disk Pressure:**
   ```
   Conditions:
     DiskPressure   True   ... disk pressure
   ```
   
   Solution: Increase disk size or clean up:
   ```bash
   ssh ubuntu@<node-ip>
   df -h
   sudo docker system prune -a
   ```

3. **Memory Pressure:**
   ```
   Conditions:
     MemoryPressure   True   ... memory pressure
   ```
   
   Solution: Use larger node flavors or reduce workloads.

## FluxCD Issues

### GitRepository Not Syncing

**Error:** FluxCD can't access Git repository.

**Diagnosis:**

```bash
kubectl get gitrepositories -n flux-system
kubectl describe gitrepository <name> -n flux-system
```

**Common Causes:**

1. **SSH Key Not Found:**
   ```
   Status:
     Conditions:
       Message: authentication required
   ```
   
   Solution: Create SSH key secret:
   ```bash
   kubectl create secret generic opencenter-base \
     --from-file=identity=<ssh-private-key> \
     --from-file=known_hosts=<known-hosts> \
     -n flux-system
   ```

2. **Invalid Git URL:**
   ```
   Status:
     Conditions:
       Message: repository not found
   ```
   
   Solution: Verify Git URL:
   ```yaml
   opencenter:
     gitops:
       git_url: "ssh://git@github.com/org/repo.git"  # Correct URL
   ```

3. **Branch Not Found:**
   ```
   Status:
     Conditions:
       Message: reference not found
   ```
   
   Solution: Verify branch exists:
   ```bash
   git ls-remote <git-url>
   ```

### Kustomization Failures

**Error:** Kustomization shows reconciliation errors.

**Diagnosis:**

```bash
kubectl get kustomizations -n flux-system
kubectl describe kustomization <name> -n flux-system
```

**Common Causes:**

1. **Path Not Found:**
   ```
   Status:
     Conditions:
       Message: path not found: ./applications/overlays/my-cluster
   ```
   
   Solution: Verify path exists in Git repository:
   ```bash
   cd <git_dir>
   ls -la applications/overlays/my-cluster
   ```

2. **SOPS Decryption Failed:**
   ```
   Status:
     Conditions:
       Message: decryption failed: no key could decrypt
   ```
   
   Solution: Create SOPS Age key secret:
   ```bash
   kubectl create secret generic sops-age \
     --from-file=age.agekey=$SOPS_AGE_KEY_FILE \
     -n flux-system
   ```

3. **Invalid Manifest:**
   ```
   Status:
     Conditions:
       Message: validation failed: invalid YAML
   ```
   
   Solution: Validate manifests:
   ```bash
   cd <git_dir>
   kubectl apply --dry-run=client -f applications/overlays/my-cluster/
   ```

## Service Deployment Issues

### HelmRelease Failures

**Error:** Service fails to deploy via Helm.

**Diagnosis:**

```bash
kubectl get helmreleases -A
kubectl describe helmrelease <name> -n <namespace>
```

**Common Causes:**

1. **Chart Not Found:**
   ```
   Status:
     Conditions:
       Message: chart not found
   ```
   
   Solution: Verify HelmRepository:
   ```bash
   kubectl get helmrepositories -A
   kubectl describe helmrepository <name> -n <namespace>
   ```

2. **Values Error:**
   ```
   Status:
     Conditions:
       Message: template error: invalid value
   ```
   
   Solution: Check Helm values:
   ```bash
   kubectl get helmrelease <name> -n <namespace> -o yaml
   ```

3. **Dependency Not Ready:**
   ```
   Status:
     Conditions:
       Message: dependency not ready: cert-manager
   ```
   
   Solution: Wait for dependencies or check their status:
   ```bash
   kubectl get helmrelease cert-manager -n cert-manager
   ```

### Pod CrashLoopBackOff

**Error:** Service pods repeatedly crash.

**Diagnosis:**

```bash
kubectl get pods -n <namespace>
kubectl logs <pod-name> -n <namespace>
kubectl describe pod <pod-name> -n <namespace>
```

**Common Causes:**

1. **Missing Secret:**
   ```
   Error: secret "keycloak-secret" not found
   ```
   
   Solution: Create secret:
   ```yaml
   # In configuration
   secrets:
     keycloak:
       client_secret: "your-secret"
       admin_password: "your-password"
   ```
   
   Encrypt and regenerate:
   ```bash
   opencenter sops secrets-encrypt --cluster my-cluster
   opencenter cluster setup --render
   ```

2. **Insufficient Resources:**
   ```
   Status:
     Reason: OOMKilled
   ```
   
   Solution: Increase resource limits:
   ```yaml
   opencenter:
     services:
       kube-prometheus-stack:
         prometheus_volume_size: 100  # Increase
   ```

3. **Configuration Error:**
   ```
   Error: invalid configuration: missing required field
   ```
   
   Solution: Check service configuration:
   ```yaml
   opencenter:
     services:
       keycloak:
         hostname: "auth.example.com"  # Required
   ```

## Network Issues

### Pod Network Connectivity

**Error:** Pods can't communicate with each other.

**Diagnosis:**

```bash
# Test pod-to-pod connectivity
kubectl run test-pod --image=busybox --rm -it -- sh
# Inside pod:
ping <other-pod-ip>
nslookup kubernetes.default
```

**Solution:** Check CNI plugin:

```bash
kubectl get pods -n kube-system | grep calico
kubectl logs -n kube-system <calico-node-pod>
```

### Service Not Accessible

**Error:** Service not reachable via ClusterIP or LoadBalancer.

**Diagnosis:**

```bash
kubectl get svc -A
kubectl describe svc <service-name> -n <namespace>
```

**Common Causes:**

1. **No Endpoints:**
   ```
   Endpoints: <none>
   ```
   
   Solution: Check pod selector:
   ```bash
   kubectl get pods -n <namespace> --show-labels
   ```

2. **LoadBalancer Pending:**
   ```
   EXTERNAL-IP: <pending>
   ```
   
   Solution: Check load balancer provider:
   ```bash
   # For Octavia
   kubectl logs -n kube-system <openstack-cloud-controller-manager-pod>
   
   # For MetalLB
   kubectl logs -n metallb-system <metallb-controller-pod>
   ```

### DNS Resolution Fails

**Error:** Pods can't resolve DNS names.

**Diagnosis:**

```bash
kubectl run test-pod --image=busybox --rm -it -- nslookup kubernetes.default
```

**Solution:** Check CoreDNS:

```bash
kubectl get pods -n kube-system | grep coredns
kubectl logs -n kube-system <coredns-pod>
```

## Performance Issues

### Slow Reconciliation

**Error:** FluxCD takes too long to reconcile changes.

**Diagnosis:** Check reconciliation intervals:

```bash
kubectl get kustomizations -n flux-system -o yaml | grep interval
```

**Solution:** Reduce intervals for faster reconciliation:

```yaml
opencenter:
  gitops:
    flux:
      interval: "5m"  # Reduce from 15m
```

### High Resource Usage

**Error:** Cluster nodes running out of resources.

**Diagnosis:**

```bash
kubectl top nodes
kubectl top pods -A
```

**Solution:** Scale cluster or optimize services:

```yaml
opencenter:
  cluster:
    kubernetes:
      worker_count: 4  # Increase workers
```

Or disable unnecessary services:

```yaml
opencenter:
  services:
    tempo:
      enabled: false  # Disable if not needed
```

## Getting Help

### Collect Diagnostic Information

Before asking for help, collect:

1. **Configuration:**
   ```bash
   opencenter cluster validate --generate-debug-config
   ```

2. **Cluster state:**
   ```bash
   kubectl get nodes -o wide > nodes.txt
   kubectl get pods -A -o wide > pods.txt
   kubectl get kustomizations -n flux-system > flux.txt
   ```

3. **Logs:**
   ```bash
   kubectl logs -n flux-system <flux-pod> > flux-logs.txt
   kubectl logs -n kube-system <calico-pod> > calico-logs.txt
   ```

### Report Issues

When reporting issues, include:
- openCenter version (`opencenter version`)
- Provider (OpenStack, VMware, Kind, AWS)
- Error messages (exact text)
- Diagnostic information (above)
- Steps to reproduce

GitHub Issues: https://github.com/rackerlabs/openCenter-cli/issues

## Next Steps

- [Validate Configuration](validate-configuration.md) - Prevent issues before deployment
- [Manage Secrets](manage-secrets.md) - Fix SOPS-related issues
- [Configure Networking](configure-networking.md) - Fix network problems
- [Getting Started Tutorial](../tutorials/getting-started.md) - Review deployment steps

---

## Evidence

This how-to guide is based on:

- Validation command: `cmd/cluster_validate.go:1-108`
- Workflow validation: `tests/features/workflow.feature:38-73`
- SOPS manager errors: `internal/sops/manager.go:100-250`
- Session 1 troubleshooting: A6, A7, A8
- Session 2 facts inventory: B0 sections 8, 13
- Common deployment patterns: Ecosystem.md

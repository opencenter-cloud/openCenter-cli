# Configure Networking

**Purpose:** For network engineers, shows how to configure CNI plugins, load balancers, and network policies, covering network topology through security.

Kubernetes networking in openCenter is highly configurable. This guide shows you how to configure CNI plugins, load balancers, subnets, and network security.

## Prerequisites

- openCenter CLI installed
- Cluster configuration created
- Understanding of Kubernetes networking concepts (helpful)

## Network Topology

### Default Network Configuration

openCenter uses these default CIDR ranges:

```yaml
opencenter:
  cluster:
    kubernetes:
      subnet_pods: "10.42.0.0/16"        # Pod network (65,536 IPs)
      subnet_services: "10.43.0.0/16"    # Service network (65,536 IPs)
    networking:
      subnet_nodes: "10.2.128.0/22"      # Node network (1,024 IPs)
```

### Customize Network Ranges

Change CIDR ranges to avoid conflicts:

```yaml
opencenter:
  cluster:
    kubernetes:
      subnet_pods: "10.244.0.0/16"
      subnet_services: "10.245.0.0/16"
    networking:
      subnet_nodes: "192.168.1.0/24"
```

**Important:** Ensure ranges don't overlap with:
- Existing network infrastructure
- VPN networks
- Other Kubernetes clusters

## CNI Plugin Configuration

### Calico (Default)

Calico is enabled by default with VXLAN encapsulation:

```yaml
opencenter:
  cluster:
    kubernetes:
      network_plugin:
        calico:
          enabled: true
          cni_iface: "enp3s0"
          calico_interface_autodetect: "interface"
          autodetect_cidr: ""
          encapsulation_type: "VXLAN"
          nat_outgoing: true
```

#### Calico Interface Detection

**By Interface Name:**
```yaml
calico:
  calico_interface_autodetect: "interface"
  cni_iface: "enp3s0"  # Specific interface
```

**By CIDR:**
```yaml
calico:
  calico_interface_autodetect: "cidr"
  autodetect_cidr: "10.0.0.0/8"
```

**By Can-Reach:**
```yaml
calico:
  calico_interface_autodetect: "can-reach"
  autodetect_cidr: "8.8.8.8"  # Google DNS
```

**Skip Interface:**
```yaml
calico:
  calico_interface_autodetect: "skip-interface"
  cni_iface: "docker0"  # Skip this interface
```

#### Calico Encapsulation

**VXLAN (Default):**
```yaml
calico:
  encapsulation_type: "VXLAN"
```

**IPIP:**
```yaml
calico:
  encapsulation_type: "IPIP"
```

**No Encapsulation (BGP):**
```yaml
calico:
  encapsulation_type: "None"
```

### Cilium

Enable Cilium with eBPF and kube-proxy replacement:

```yaml
opencenter:
  cluster:
    kubernetes:
      network_plugin:
        calico:
          enabled: false
        cilium:
          enabled: true
          operator_enabled: true
          kube_proxy_replacement: true
```

**Note:** Only one CNI plugin can be enabled at a time.

### Kube-OVN

Enable Kube-OVN with optional Cilium integration:

```yaml
opencenter:
  cluster:
    kubernetes:
      network_plugin:
        calico:
          enabled: false
        kube-ovn:
          enabled: true
          cilium_integration: true
```

## Load Balancer Configuration

### OVN Load Balancer (Default)

Use OVN for load balancing (no external dependency):

```yaml
opencenter:
  cluster:
    kubernetes:
      loadbalancer_provider: "ovn"
    networking:
      use_octavia: false
      loadbalancer_provider: "ovn"
```

### Octavia Load Balancer

Use OpenStack Octavia for production load balancing:

```yaml
opencenter:
  cluster:
    kubernetes:
      loadbalancer_provider: "octavia"
    networking:
      use_octavia: true
      loadbalancer_provider: "octavia"
```

**Requirements:**
- OpenStack cloud with Octavia service
- Sufficient Octavia quota

### MetalLB

Use MetalLB for bare metal or VMware:

```yaml
opencenter:
  cluster:
    kubernetes:
      loadbalancer_provider: "metallb"
```

Configure IP address pool:

```yaml
# In GitOps repository after setup
# applications/overlays/<cluster>/services/metallb/ipaddresspool.yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: default
  namespace: metallb-system
spec:
  addresses:
  - 192.168.1.100-192.168.1.200
```

### No Load Balancer

Disable load balancer (NodePort only):

```yaml
opencenter:
  cluster:
    kubernetes:
      loadbalancer_provider: "none"
```

## VRRP Configuration

### Enable VRRP

For high availability without Octavia:

```yaml
opencenter:
  cluster:
    networking:
      vrrp_enabled: true
      vrrp_ip: "10.0.0.10"
      use_octavia: false
```

**Important:** `vrrp_ip` is required when `use_octavia=false` and `vrrp_enabled=true`.

### Disable VRRP

When using Octavia:

```yaml
opencenter:
  cluster:
    networking:
      vrrp_enabled: false
      use_octavia: true
```

## DNS Configuration

### Cluster DNS

Configure DNS nameservers for nodes:

```yaml
opencenter:
  cluster:
    networking:
      dns_nameservers:
        - "8.8.8.8"
        - "8.8.4.4"
```

Use internal DNS:

```yaml
opencenter:
  cluster:
    networking:
      dns_nameservers:
        - "10.0.0.53"
        - "10.0.0.54"
```

### OpenStack Designate

Enable DNS integration with OpenStack Designate:

```yaml
opencenter:
  cluster:
    networking:
      use_designate: true
      dns_zone_name: "k8s.example.com"
    kubernetes:
      dns_zone_name: "k8s.example.com"
```

Configure Designate in infrastructure:

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        networking:
          designate:
            dns_zone_name: "k8s.example.com"
```

## NTP Configuration

Configure time synchronization:

```yaml
opencenter:
  cluster:
    networking:
      ntp_servers:
        - "time.sjc3.rackspace.com"
        - "time2.sjc3.rackspace.com"
```

Use public NTP:

```yaml
opencenter:
  cluster:
    networking:
      ntp_servers:
        - "0.pool.ntp.org"
        - "1.pool.ntp.org"
```

## VLAN Configuration

### OpenStack VLAN

Configure VLAN for OpenStack networking:

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        networking:
          vlan:
            id: "100"
            mtu: 1500
            provider: "physnet1"
```

### Cluster VLAN

Configure VLAN at cluster level:

```yaml
opencenter:
  cluster:
    networking:
      vlan:
        id: "100"
        mtu: 1500
        provider: "physnet1"
```

## Network Allocation

### Node Network Allocation

Configure IP allocation pool for nodes:

```yaml
opencenter:
  cluster:
    networking:
      subnet_nodes: "10.2.128.0/22"
      allocation_pool_start: "10.2.128.10"
      allocation_pool_end: "10.2.131.250"
```

This reserves:
- `10.2.128.1-9` for infrastructure (gateway, DNS, etc.)
- `10.2.128.10-10.2.131.250` for nodes
- `10.2.131.251-254` for future use

## Kubernetes API Access

### API Port

Configure Kubernetes API server port:

```yaml
opencenter:
  cluster:
    kubernetes:
      api_port: 443  # Default HTTPS port
```

Use custom port:

```yaml
opencenter:
  cluster:
    kubernetes:
      api_port: 6443  # Traditional Kubernetes port
```

### API Access Control

Configure allowed CIDR blocks for API access:

```yaml
opencenter:
  cluster:
    k8s_api_port_acl:
      - "10.0.0.0/8"      # Internal network
      - "192.168.1.0/24"  # Office network
```

Allow from anywhere (not recommended for production):

```yaml
opencenter:
  cluster:
    k8s_api_port_acl:
      - "0.0.0.0/0"
```

## Network Security

### OS Hardening

Enable operating system network hardening:

```yaml
opencenter:
  cluster:
    networking:
      security:
        os_hardening: true
```

This configures:
- Firewall rules
- Kernel parameters (IP forwarding, etc.)
- Network security modules

### CA Certificates

Add custom CA certificates:

```yaml
opencenter:
  cluster:
    networking:
      security:
        ca_certificates: |
          -----BEGIN CERTIFICATE-----
          MIIDXTCCAkWgAwIBAgIJAKZ...
          -----END CERTIFICATE-----
```

## Gateway API Configuration

### Enable Gateway API

Gateway API is enabled by default:

```yaml
opencenter:
  services:
    gateway-api:
      enabled: true
    gateway:
      enabled: true
```

### HTTPRoute Hostname Format

Services use this hostname pattern:

```
<service>.<org>.<cluster>.<region>.k8s.opencenter.cloud
```

Example:
```
auth.my-org.my-cluster.sjc3.k8s.opencenter.cloud
```

Configure base domain:

```yaml
opencenter:
  cluster:
    base_domain: "k8s.opencenter.cloud"
    cluster_fqdn: "my-cluster.sjc3.k8s.opencenter.cloud"
```

## Network Validation

Validate network configuration:

```bash
opencenter cluster validate
```

This checks:
- CIDR ranges don't overlap
- Required fields are set (VRRP IP when needed)
- Network topology is valid
- DNS configuration is correct

## Apply Network Changes

After changing network configuration:

1. **Validate:**
   ```bash
   opencenter cluster validate
   ```

2. **Regenerate manifests:**
   ```bash
   opencenter cluster setup --render
   ```

3. **Review changes:**
   ```bash
   cd <git_dir>
   git diff
   ```

4. **Commit and push:**
   ```bash
   git add .
   git commit -m "Update network configuration"
   git push
   ```

**Warning:** Changing CNI plugin or network ranges on existing clusters requires cluster rebuild.

## Troubleshooting

### Pod Network Issues

**Problem:** Pods can't communicate

**Solution:** Check CNI plugin status:

```bash
kubectl get pods -n kube-system | grep calico
kubectl logs -n kube-system <calico-pod>
```

Verify interface configuration:

```bash
kubectl exec -n kube-system <calico-pod> -- ip addr
```

### Load Balancer Not Working

**Problem:** LoadBalancer services stuck in Pending

**Solution:** Check load balancer provider:

```bash
# For Octavia
kubectl logs -n kube-system <openstack-cloud-controller-manager-pod>

# For MetalLB
kubectl get ipaddresspool -n metallb-system
kubectl logs -n metallb-system <metallb-controller-pod>
```

### DNS Resolution Fails

**Problem:** Pods can't resolve DNS names

**Solution:** Check CoreDNS:

```bash
kubectl get pods -n kube-system | grep coredns
kubectl logs -n kube-system <coredns-pod>
```

Verify DNS configuration:

```bash
kubectl get configmap coredns -n kube-system -o yaml
```

### VRRP IP Conflict

**Problem:** VRRP IP already in use

**Solution:** Choose different IP:

```yaml
opencenter:
  cluster:
    networking:
      vrrp_ip: "10.0.0.11"  # Different IP
```

Verify IP is not in use:

```bash
ping 10.0.0.11  # Should timeout
```

## Next Steps

- [Customize Services](customize-services.md) - Configure network-related services
- [Add Worker Pools](add-worker-pools.md) - Scale network capacity
- [Troubleshoot Deployment](troubleshoot-deployment.md) - Fix network issues

---

## Evidence

This how-to guide is based on:

- Network defaults: `internal/config/defaults.go:177-179,204-205`
- CNI configuration: `internal/config/defaults.go:214-237`
- Schema network plugin: `schema/cluster.schema.json:300-400`
- Load balancer config: `internal/config/defaults.go:206`
- VRRP validation: `tests/features/workflow.feature:38-50`
- Session 1 networking review: A7
- Session 2 facts inventory: B0 section 5

---
title: OpenStack Networking Configuration
doc_type: explanation
provider: openstack
category: networking
last_updated: 2025-01-XX
---

# OpenStack Networking Configuration


## Table of Contents

- [Overview](#overview)
- [Network Architecture Components](#network-architecture-components)
- [Kubernetes Network Configuration](#kubernetes-network-configuration)
- [Advanced Networking Patterns](#advanced-networking-patterns)
- [Network Performance Tuning](#network-performance-tuning)
- [Troubleshooting Network Issues](#troubleshooting-network-issues)
- [Best Practices](#best-practices)
- [Related Documentation](#related-documentation)
- [Additional Resources](#additional-resources)
This guide explains OpenStack networking concepts and configuration options in openCenter, helping you understand how to design and configure network topology for your Kubernetes clusters.

## Overview

OpenStack networking (Neutron) provides flexible network infrastructure for Kubernetes clusters. openCenter supports multiple networking patterns including floating IPs, load balancers, VLAN configurations, and DNS integration through Designate.

## Network Architecture Components

### 1. Network Topology

OpenStack clusters deployed with openCenter use a multi-tier network architecture:

```
┌─────────────────────────────────────────────────────────┐
│                    External Network                      │
│              (Router External Network)                   │
└────────────────────┬────────────────────────────────────┘
                     │
                     │ Floating IPs
                     │
┌────────────────────▼────────────────────────────────────┐
│                  Neutron Router                          │
└────────────────────┬────────────────────────────────────┘
                     │
                     │
┌────────────────────▼────────────────────────────────────┐
│              Cluster Private Network                     │
│                                                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │ Master 1 │  │ Master 2 │  │ Master 3 │              │
│  └──────────┘  └──────────┘  └──────────┘              │
│                                                           │
│  ┌──────────┐  ┌──────────┐                             │
│  │ Worker 1 │  │ Worker 2 │  ...                        │
│  └──────────┘  └──────────┘                             │
└───────────────────────────────────────────────────────────┘
```

### 2. Floating IP Configuration

Floating IPs provide external access to cluster resources. Configure in your cluster YAML:

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        networking:
          # Floating IP pool name (e.g., "PUBLICNET")
          floating_ip_pool: "PUBLICNET"
          
          # Floating network UUID (alternative to pool name)
          floating_network_id: "723f8fa2-dbf7-4cec-8d5f-017e62c12f79"
```

**When to use each option:**

- **`floating_ip_pool`**: Use the pool name when you know the public network name (simpler, more readable)
- **`floating_network_id`**: Use the UUID when you need explicit network targeting or have multiple networks with similar names

### 3. Network and Subnet Configuration

Define the cluster's private network infrastructure:

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        networking:
          # Existing network UUID to use
          network_id: "a1b2c3d4-1234-5678-90ab-cdef12345678"
          
          # Existing subnet UUID to use
          subnet_id: "e5f6g7h8-1234-5678-90ab-cdef12345678"
          
          # External router network for internet access
          router_external_network_id: "723f8fa2-dbf7-4cec-8d5f-017e62c12f79"
```

**Network creation behavior:**

- If `network_id` is **not specified**: openCenter creates a new network for the cluster
- If `network_id` **is specified**: openCenter uses the existing network
- If `subnet_id` is **not specified**: openCenter creates a new subnet within the network
- If `subnet_id` **is specified**: openCenter uses the existing subnet

### 4. VLAN Configuration

For environments requiring VLAN isolation or specific MTU settings:

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        networking:
          vlan:
            # VLAN ID (leave empty for automatic assignment)
            id: "100"
            
            # Maximum Transmission Unit (0 for default)
            mtu: 1500
            
            # Physical network provider
            provider: "physnet1"
```

**VLAN use cases:**

- **Multi-tenant environments**: Isolate cluster traffic using VLANs
- **Compliance requirements**: Separate networks for regulatory compliance
- **Performance tuning**: Adjust MTU for jumbo frames (9000) in high-throughput scenarios
- **Provider networks**: Connect directly to physical network infrastructure

### 5. Load Balancer Configuration

openCenter supports multiple load balancer providers for Kubernetes services:

```yaml
opencenter:
  cluster:
    kubernetes:
      # Load balancer provider: ovn, octavia, metallb, none
      loadbalancer_provider: "ovn"
      
      networking:
        # Enable OpenStack Octavia (LBaaS v2)
        use_octavia: true
```

**Load balancer provider comparison:**

| Provider | Description | Use Case | Requirements |
|----------|-------------|----------|--------------|
| **ovn** | OVN-based load balancing (default) | Modern OpenStack deployments with OVN | OVN networking enabled |
| **octavia** | OpenStack Octavia (LBaaS v2) | Production environments requiring advanced LB features | Octavia service enabled |
| **metallb** | MetalLB for bare-metal style LB | Environments without cloud LB support | IP address pool available |
| **none** | No automatic load balancer | Manual LB configuration or NodePort only | N/A |

**When to use Octavia:**

- Production workloads requiring high availability
- Advanced load balancing features (SSL termination, health checks)
- Integration with OpenStack monitoring and billing
- Multi-protocol support (TCP, UDP, HTTP, HTTPS)

### 6. DNS Integration with Designate

OpenStack Designate provides DNS-as-a-Service for automatic DNS record management:

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        networking:
          designate:
            # DNS zone name for automatic record creation
            dns_zone_name: "k8s.example.com"

networking:
  # Enable Designate integration
  use_designate: true
  
  # DNS zone name (must match Designate configuration)
  dns_zone_name: "k8s.example.com"
```

**Designate benefits:**

- **Automatic DNS records**: Cluster nodes and services get DNS entries automatically
- **Dynamic updates**: DNS records update when nodes are added/removed
- **Integration**: Works with external DNS controllers in Kubernetes
- **Multi-region**: Supports DNS across multiple OpenStack regions

**DNS record patterns:**

```
# Master nodes
cp-1.k8s.example.com -> 10.0.1.10
cp-2.k8s.example.com -> 10.0.1.11
cp-3.k8s.example.com -> 10.0.1.12

# Worker nodes
wn-1.k8s.example.com -> 10.0.1.20
wn-2.k8s.example.com -> 10.0.1.21

# API endpoint
api.cluster-name.region.k8s.example.com -> <floating-ip>
```

## Kubernetes Network Configuration

### Pod and Service Networks

Configure IP address ranges for Kubernetes internal networking:

```yaml
opencenter:
  cluster:
    kubernetes:
      # Pod network CIDR (Calico/Cilium/Kube-OVN)
      subnet_pods: "10.42.0.0/16"
      
      # Service network CIDR
      subnet_services: "10.43.0.0/16"
```

**Network sizing guidelines:**

| Cluster Size | Nodes | Pods per Node | Recommended Pod CIDR | Recommended Service CIDR |
|--------------|-------|---------------|----------------------|--------------------------|
| Small | 3-10 | 110 | 10.42.0.0/16 (65k IPs) | 10.43.0.0/16 (65k IPs) |
| Medium | 10-50 | 110 | 10.42.0.0/14 (262k IPs) | 10.43.0.0/16 (65k IPs) |
| Large | 50-250 | 110 | 10.42.0.0/12 (1M IPs) | 10.43.0.0/16 (65k IPs) |

**Important considerations:**

- Pod and service CIDRs **must not overlap** with node network
- Pod and service CIDRs **must not overlap** with each other
- Ensure CIDRs don't conflict with corporate networks if using VPN
- Plan for growth: oversizing is better than running out of IPs

### CNI Plugin Selection

openCenter supports three CNI plugins. **Only one can be enabled at a time:**

#### Calico (Default)

```yaml
opencenter:
  cluster:
    kubernetes:
      network_plugin:
        calico:
          enabled: true
          
          # Network interface for Calico (e.g., "enp3s0", "eth0")
          cni_iface: "enp3s0"
          
          # Interface detection method: "interface", "cidr", "can-reach"
          calico_interface_autodetect: "interface"
          
          # CIDR for interface autodetection (if using "cidr" method)
          autodetect_cidr: ""
          
          # Encapsulation: "VXLAN", "IPIP", "None"
          encapsulation_type: "VXLAN"
          
          # Enable NAT for outgoing traffic
          nat_outgoing: true
```

**Calico encapsulation types:**

- **VXLAN**: Best compatibility, works across most networks, slight overhead
- **IPIP**: Lower overhead than VXLAN, requires IP-in-IP support
- **None**: No encapsulation (best performance), requires BGP routing

#### Cilium

```yaml
opencenter:
  cluster:
    kubernetes:
      network_plugin:
        cilium:
          enabled: true
          
          # Enable Cilium operator
          operator_enabled: true
          
          # Replace kube-proxy with Cilium
          kubeProxyReplacement: true
```

**Cilium advantages:**

- eBPF-based networking (high performance)
- Advanced network policies (L7, DNS-aware)
- Service mesh capabilities
- Hubble observability integration

#### Kube-OVN

```yaml
opencenter:
  cluster:
    kubernetes:
      network_plugin:
        kube-ovn:
          enabled: true
          
          # Enable Cilium integration for advanced features
          cilium_integration: true
```

**Kube-OVN features:**

- OVN-based networking (integrates with OpenStack OVN)
- Subnet isolation and multi-tenancy
- QoS and traffic shaping
- Dual-stack IPv4/IPv6 support

### Network Security

#### API Server Access Control

Restrict access to the Kubernetes API server using CIDR-based ACLs:

```yaml
opencenter:
  cluster:
    networking:
      # List of CIDRs allowed to access Kubernetes API
      k8s_api_port_acl:
        - "0.0.0.0/0"        # Allow all (not recommended for production)
        # - "10.0.0.0/8"     # Corporate network
        # - "192.168.1.0/24" # VPN network
        # - "203.0.113.0/24" # Office network
```

**Security best practices:**

- **Never use `0.0.0.0/0` in production** unless behind additional security layers
- Restrict to known IP ranges (corporate networks, VPN, bastion hosts)
- Use multiple CIDRs for different access patterns
- Combine with OpenStack security groups for defense in depth

#### DNS and NTP Configuration

```yaml
opencenter:
  cluster:
    networking:
      # DNS servers for cluster nodes
      dns_nameservers:
        - "8.8.8.8"
        - "8.8.4.4"
      
      # NTP servers for time synchronization
      ntp_servers:
        - "time.sjc3.rackspace.com"
        - "time2.sjc3.rackspace.com"
```

**Configuration recommendations:**

- Use internal DNS servers for better performance and security
- Configure region-specific NTP servers for accurate time sync
- Ensure DNS servers are reachable from cluster network
- Consider DNS caching for large clusters

#### OS Hardening

```yaml
opencenter:
  cluster:
    networking:
      security:
        # Enable OS-level security hardening
        os_hardening: true
        
        # Custom CA certificates (PEM format)
        ca_certificates: ""
```

**OS hardening includes:**

- Firewall rules (iptables/nftables)
- SSH hardening (key-only auth, disabled root login)
- Kernel parameter tuning
- Audit logging configuration
- SELinux/AppArmor policies

## Advanced Networking Patterns

### VRRP for High Availability

Virtual Router Redundancy Protocol (VRRP) provides HA for the API endpoint without Octavia:

```yaml
networking:
  # Enable VRRP for API HA
  vrrp_enabled: true
  
  # Virtual IP for VRRP (must be in node subnet)
  vrrp_ip: "10.0.1.100"

opencenter:
  cluster:
    kubernetes:
      # Disable Octavia when using VRRP
      networking:
        use_octavia: false
```

**VRRP vs Octavia:**

| Feature | VRRP | Octavia |
|---------|------|---------|
| **Cost** | No additional resources | Requires LB instances |
| **Complexity** | Simple, node-based | Managed service |
| **Scalability** | Limited to master nodes | Highly scalable |
| **Features** | Basic failover | Advanced LB features |
| **Use case** | Small clusters, cost-sensitive | Production, large clusters |

**VRRP requirements:**

- `vrrp_ip` must be in the same subnet as master nodes
- `use_octavia` must be set to `false`
- At least 2 master nodes for redundancy
- Network must allow VRRP protocol (IP protocol 112)

### Multi-Network Configurations

For clusters requiring multiple networks (management, storage, data):

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        networking:
          # Primary network for cluster communication
          network_id: "mgmt-network-uuid"
          subnet_id: "mgmt-subnet-uuid"

  cluster:
    kubernetes:
      # Additional server pools can use different subnets
      additional_server_pools_worker:
        - name: "storage-pool"
          worker_count: 3
          flavor_worker: "gp.0.8.32"
          node_worker: "storage"
          # Optional: specific subnet for this pool
          subnet_id: "storage-subnet-uuid"
```

**Multi-network use cases:**

- **Storage networks**: Dedicated network for Ceph/storage traffic
- **Management networks**: Separate control plane traffic
- **Data networks**: High-bandwidth networks for application data
- **Security zones**: Isolate sensitive workloads

### Network Allocation Pools

Control IP address allocation within subnets:

```yaml
networking:
  # Node network CIDR
  subnet_nodes: "10.0.1.0/24"
  
  # IP allocation pool start
  allocation_pool_start: "10.0.1.100"
  
  # IP allocation pool end
  allocation_pool_end: "10.0.1.200"
```

**Allocation pool benefits:**

- Reserve IPs for static assignments (1-99 in example above)
- Prevent conflicts with existing infrastructure
- Organize IP space by function (masters, workers, services)
- Simplify network documentation and troubleshooting

## Network Performance Tuning

### MTU Configuration

Maximum Transmission Unit affects network performance:

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        networking:
          vlan:
            # Standard Ethernet
            mtu: 1500
            
            # Jumbo frames (requires network support)
            # mtu: 9000
```

**MTU recommendations:**

- **1500**: Standard, works everywhere (default)
- **1450**: For VXLAN/VLAN overhead (encapsulation)
- **9000**: Jumbo frames for high-throughput workloads (storage, databases)

**Important**: Ensure MTU is consistent across:
- OpenStack network configuration
- Physical network infrastructure
- CNI plugin configuration
- Application requirements

### CNI Performance Tuning

#### Calico Performance

```yaml
opencenter:
  cluster:
    kubernetes:
      network_plugin:
        calico:
          # Use VXLAN for best compatibility
          encapsulation_type: "VXLAN"
          
          # Or disable encapsulation for best performance (requires routing)
          # encapsulation_type: "None"
          
          # Enable NAT for internet access
          nat_outgoing: true
```

#### Cilium Performance

```yaml
opencenter:
  cluster:
    kubernetes:
      network_plugin:
        cilium:
          # Replace kube-proxy for better performance
          kubeProxyReplacement: true
```

## Troubleshooting Network Issues

### Connectivity Testing

```bash
# Test node-to-node connectivity
kubectl run -it --rm debug --image=nicolaka/netshoot --restart=Never -- ping <node-ip>

# Test pod-to-pod connectivity
kubectl run -it --rm debug --image=nicolaka/netshoot --restart=Never -- ping <pod-ip>

# Test service connectivity
kubectl run -it --rm debug --image=nicolaka/netshoot --restart=Never -- curl <service-name>

# Test external connectivity
kubectl run -it --rm debug --image=nicolaka/netshoot --restart=Never -- curl https://google.com
```

### Network Policy Debugging

```bash
# Check Calico network policies
kubectl get networkpolicies --all-namespaces

# View Calico node status
kubectl get nodes -o wide
kubectl describe node <node-name>

# Check CNI plugin logs
kubectl logs -n kube-system -l k8s-app=calico-node
```

### OpenStack Network Verification

```bash
# List networks
openstack network list

# Show network details
openstack network show <network-id>

# List subnets
openstack subnet list

# Show subnet details
openstack subnet show <subnet-id>

# List floating IPs
openstack floating ip list

# List load balancers (if using Octavia)
openstack loadbalancer list
```

## Best Practices

### Network Design

1. **Plan IP address space carefully**: Avoid overlapping CIDRs
2. **Use private networks**: Don't expose cluster nodes directly to internet
3. **Implement network segmentation**: Separate management, data, and storage traffic
4. **Enable network policies**: Use Kubernetes NetworkPolicies for pod-to-pod security
5. **Monitor network performance**: Track latency, throughput, and packet loss

### Security

1. **Restrict API access**: Use `k8s_api_port_acl` to limit API server access
2. **Enable OS hardening**: Set `os_hardening: true` for production clusters
3. **Use security groups**: Configure OpenStack security groups for defense in depth
4. **Encrypt traffic**: Enable CNI encryption features (Calico WireGuard, Cilium encryption)
5. **Regular audits**: Review network policies and security group rules regularly

### Performance

1. **Choose appropriate MTU**: Match network infrastructure capabilities
2. **Select right CNI**: Calico for compatibility, Cilium for performance, Kube-OVN for OVN integration
3. **Optimize encapsulation**: Use VXLAN for compatibility, disable for performance
4. **Size networks appropriately**: Ensure sufficient IP space for growth
5. **Monitor and tune**: Use network monitoring tools to identify bottlenecks

## Related Documentation

- [OpenStack Troubleshooting Guide](./troubleshooting.md)
- [OpenStack Getting Started](./getting-started.md)
- [Security Configuration](../../reference/security.md)
- [Network Policies](../../reference/network-policies.md)

## Additional Resources

- [OpenStack Neutron Documentation](https://docs.openstack.org/neutron/latest/)
- [Calico Documentation](https://docs.tigera.io/calico/latest/about/)
- [Cilium Documentation](https://docs.cilium.io/)
- [Kube-OVN Documentation](https://kubeovn.github.io/docs/)
- [OpenStack Octavia Documentation](https://docs.openstack.org/octavia/latest/)
- [OpenStack Designate Documentation](https://docs.openstack.org/designate/latest/)

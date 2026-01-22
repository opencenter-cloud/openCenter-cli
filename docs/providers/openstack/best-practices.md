# OpenStack Best Practices


## Table of Contents

- [Purpose](#purpose)
- [Prerequisites](#prerequisites)
- [Production Configuration](#production-configuration)
- [Operational Practices](#operational-practices)
- [Configuration Patterns](#configuration-patterns)
- [Troubleshooting Common Issues](#troubleshooting-common-issues)
- [Performance Optimization](#performance-optimization)
- [Security Considerations](#security-considerations)
- [Cost Optimization](#cost-optimization)
- [Related Documentation](#related-documentation)
- [External Resources](#external-resources)
**doc_type: how-to**

This guide provides production recommendations for deploying and operating Kubernetes clusters on OpenStack using openCenter. It covers configuration choices, operational patterns, and lessons learned from production deployments.

## Purpose

Use this guide when preparing to deploy production Kubernetes clusters on OpenStack. It assumes you have completed the [OpenStack Setup Guide](setup.md) and are familiar with the [OpenStack Provider Overview](README.md).

## Prerequisites

- OpenStack environment configured and accessible
- Application credentials or user credentials with appropriate permissions
- Understanding of Kubernetes concepts and operations
- Familiarity with openCenter configuration format

## Production Configuration

### High Availability Setup

Deploy three master nodes for production clusters. This provides quorum for etcd and redundancy for control plane components.

```yaml
opencenter:
  cluster:
    kubernetes:
      master_count: 3
      flavor_master: "gp.0.4.8"  # 4 vCPU, 8 GB RAM minimum
```

**Why three masters:**
- etcd requires odd number of nodes for quorum
- Tolerates one master failure without service disruption
- Distributes API server load across multiple endpoints

**Master sizing:**
- Small clusters (< 50 nodes): 4 vCPU, 8 GB RAM
- Medium clusters (50-200 nodes): 8 vCPU, 16 GB RAM
- Large clusters (> 200 nodes): 16 vCPU, 32 GB RAM

### Load Balancer Selection

Choose between Octavia and VRRP based on your OpenStack capabilities and requirements.

**Use Octavia when:**
- Your OpenStack deployment includes Octavia service
- You need health checks and automatic failover
- You want OpenStack-native load balancing
- You require detailed load balancer metrics

```yaml
opencenter:
  cluster:
    kubernetes:
      networking:
        use_octavia: true
        loadbalancer_provider: "octavia"
```

**Use VRRP when:**
- Octavia is not available in your OpenStack
- You want lower resource overhead
- You need simpler configuration
- You can tolerate 10-30 second failover time

```yaml
opencenter:
  cluster:
    kubernetes:
      networking:
        use_octavia: false
        vrrp_enabled: true
        vrrp_ip: "10.0.4.10"  # Must be in node subnet, not allocated
```

**Avoid single master:**
Never run production clusters with `master_count: 1`. Single master configurations have no redundancy and will cause cluster downtime during master maintenance or failure.

### Worker Node Sizing

Size worker nodes based on workload requirements, not arbitrary defaults.

**General-purpose workloads:**
```yaml
opencenter:
  cluster:
    kubernetes:
      worker_count: 3  # Start small, scale as needed
      flavor_worker: "gp.0.4.16"  # 4 vCPU, 16 GB RAM
```

**Memory-intensive workloads:**
```yaml
opencenter:
  cluster:
    kubernetes:
      flavor_worker: "gp.0.8.32"  # 8 vCPU, 32 GB RAM
```

**Compute-intensive workloads:**
```yaml
opencenter:
  cluster:
    kubernetes:
      flavor_worker: "gp.0.16.16"  # 16 vCPU, 16 GB RAM
```

**Sizing guidelines:**
- Leave 20-30% headroom for system overhead and pod scheduling
- Plan for node failures: N+1 or N+2 capacity
- Use multiple smaller nodes rather than few large nodes
- Consider anti-affinity for workload distribution

### Storage Configuration

Configure Cinder volumes with appropriate types and sizes for your workload.

```yaml
opencenter:
  storage:
    default_storage_class: "csi-cinder-sc-delete"
    worker_volume_size: 100  # Increase from default 40 GB
    worker_volume_type: "HA-Standard"
    worker_volume_destination_type: "volume"
```

**Volume sizing:**
- Base OS: 20 GB minimum
- Container images: 20-40 GB (depends on image count)
- Local volumes: 20-40 GB (depends on usage)
- Logs and temp: 10-20 GB
- **Recommended minimum: 100 GB for production workers**

**Volume types:**
- `HA-Standard`: Replicated storage, good performance
- `HA-Performance`: SSD-backed, higher IOPS
- Check with your OpenStack administrator for available types

**Storage classes:**
- Use `csi-cinder-sc-delete` for ephemeral data
- Use `csi-cinder-sc-retain` for persistent data that survives PVC deletion
- Create custom storage classes for specific volume types

### Network Configuration

Plan network CIDRs carefully to avoid conflicts and allow for growth.

```yaml
opencenter:
  cluster:
    kubernetes:
      subnet_pods: "10.42.0.0/16"      # 65,536 IPs
      subnet_services: "10.43.0.0/16"  # 65,536 IPs
    networking:
      subnet_cidr: "10.0.4.0/22"       # 1,024 IPs for nodes
```

**CIDR planning:**
- Node network: /22 (1,024 IPs) for small clusters, /20 (4,096 IPs) for large
- Pod network: /16 (65,536 IPs) allows ~250 pods per node on 250 nodes
- Service network: /16 (65,536 IPs) allows 65,536 services
- Ensure no overlap with existing networks

**DNS configuration:**
```yaml
opencenter:
  cluster:
    networking:
      dns_nameservers:
        - "8.8.8.8"
        - "8.8.4.4"
```

Use internal DNS servers if available. Public DNS (8.8.8.8) works but adds external dependency.

**NTP configuration:**
```yaml
opencenter:
  cluster:
    networking:
      ntp_servers:
        - "time.example.com"
        - "time2.example.com"
```

Use internal NTP servers for consistent time synchronization. Time skew causes certificate validation failures and etcd issues.

### Security Hardening

Enable security hardening for production clusters.

```yaml
opencenter:
  cluster:
    kubernetes:
      security:
        k8s_hardening: true
        os_hardening: true
      kubelet_rotate_server_certificates: true
```

**Kubernetes hardening:**
- Enables CIS Kubernetes Benchmark compliance
- Enforces Pod Security Standards
- Enables audit logging
- Disables anonymous authentication
- Restricts API server access

**OS hardening:**
- Applies CIS OS Benchmark settings
- Configures firewall rules
- Hardens SSH configuration
- Sets secure kernel parameters
- Restricts file permissions

**Certificate rotation:**
Set `kubelet_rotate_server_certificates: true` to enable automatic certificate rotation. This prevents certificate expiration issues that cause cluster outages.

### API Access Control

Restrict API server access to known networks.

```yaml
opencenter:
  cluster:
    kubernetes:
      networking:
        k8s_api_port_acl:
          - "10.0.0.0/8"      # Internal network
          - "192.168.1.0/24"  # Office network
```

**Never use `0.0.0.0/0` in production.** This exposes the API server to the entire internet. Restrict to:
- Internal networks
- VPN networks
- Bastion host networks
- CI/CD system networks

### Authentication Configuration

Configure OIDC for centralized authentication.

```yaml
opencenter:
  cluster:
    kubernetes:
      oidc:
        enabled: true
        kube_oidc_url: "https://auth.example.com"
        kube_oidc_client_id: "kubernetes"
        kube_oidc_username_claim: "email"
        kube_oidc_groups_claim: "groups"
```

**OIDC benefits:**
- Centralized user management
- Group-based authorization
- Audit trail of user actions
- No shared credentials

**RBAC configuration:**
After enabling OIDC, configure RBAC roles and bindings to map OIDC groups to Kubernetes permissions.

## Operational Practices

### Validation Before Deployment

Always validate configuration before deploying.

```bash
mise run cluster-validate my-cluster
```

The validator checks:
- Schema compliance
- OpenStack connectivity
- Quota availability
- Network configuration
- Credential validity

Fix all errors before proceeding. Warnings should be reviewed but may not block deployment.

### Quota Management

Check quotas before deploying large clusters.

**Required quotas for 3 master, 5 worker cluster:**
- Instances: 9 (3 masters + 5 workers + 1 bastion)
- vCPUs: 40 (3×4 + 5×4 + 1×2 + overhead)
- RAM: 104 GB (3×8 + 5×16 + 1×2 + overhead)
- Volumes: 8 (one per node)
- Volume Storage: 800 GB (8 × 100 GB)
- Floating IPs: 2-4 (bastion + masters or load balancer)
- Security Groups: 5-10
- Security Group Rules: 50-100

Request quota increases before deployment, not during. Quota exhaustion during deployment leaves the cluster in a partial state.

### Deployment Timing

Plan deployments during maintenance windows.

**Typical deployment timeline:**
- Infrastructure provisioning: 10-15 minutes
- Kubernetes installation: 15-20 minutes
- Service deployment: 5-10 minutes
- **Total: 30-45 minutes**

Add buffer time for:
- OpenStack API slowness
- Image download time
- Network configuration
- Unexpected issues

### Monitoring Setup

Configure monitoring before deploying workloads.

**Essential metrics:**
- Node CPU, memory, disk usage
- Pod resource consumption
- API server latency and errors
- etcd performance
- Network throughput
- Storage IOPS and latency

**Alerting thresholds:**
- Node disk > 80% full
- Node memory > 85% used
- API server error rate > 1%
- etcd leader changes
- Certificate expiration < 30 days

See [Monitoring Guide](../../how-to/monitoring.md) for detailed setup instructions.

### Backup Strategy

Implement backups before running production workloads.

**What to back up:**
- etcd snapshots (daily)
- Cluster configuration YAML
- GitOps repository
- Persistent volume data
- Secrets and certificates

**Backup frequency:**
- etcd: Daily, retain 7 days
- Configuration: On every change
- Persistent volumes: Based on data criticality

**Test restores regularly.** Untested backups are not backups.

See [Backup and Recovery Guide](../../how-to/backup-recovery.md) for procedures.

### Upgrade Planning

Plan upgrades carefully and test in non-production first.

**Upgrade order:**
1. Test in development environment
2. Upgrade staging environment
3. Validate staging for 24-48 hours
4. Upgrade production during maintenance window
5. Monitor closely for 24 hours

**Kubernetes version policy:**
- Stay within N-2 of latest release
- Upgrade one minor version at a time
- Never skip minor versions
- Test workload compatibility before upgrading

See [Upgrading Clusters Guide](../../how-to/upgrading-clusters.md) for detailed procedures.

## Configuration Patterns

### Multi-Environment Setup

Use consistent naming and organization for multiple environments.

```yaml
# Production cluster
opencenter:
  meta:
    name: prod-cluster
    env: production
    region: RegionOne
    organization: myorg

# Staging cluster
opencenter:
  meta:
    name: staging-cluster
    env: staging
    region: RegionOne
    organization: myorg
```

**Environment separation:**
- Use separate OpenStack projects for prod/staging/dev
- Use different network CIDRs to prevent conflicts
- Apply stricter security in production
- Use smaller flavors in non-production

### Secrets Management

Store secrets in SOPS-encrypted files, never in plain text.

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        application_credential_id: "${OS_APPLICATION_CREDENTIAL_ID}"
        application_credential_secret: "${OS_APPLICATION_CREDENTIAL_SECRET}"
```

**Secrets best practices:**
- Use environment variables for credentials
- Encrypt secrets with SOPS before committing to Git
- Rotate credentials regularly
- Use application credentials instead of user passwords
- Limit credential scope to minimum required permissions

See [Secrets Management Guide](../../how-to/secrets-management.md) for detailed procedures.

### GitOps Repository Structure

Organize GitOps repositories for maintainability.

```
gitops-repo/
├── infrastructure/
│   └── clusters/
│       ├── prod-cluster/
│       ├── staging-cluster/
│       └── dev-cluster/
├── applications/
│   ├── base/
│   └── overlays/
│       ├── prod-cluster/
│       ├── staging-cluster/
│       └── dev-cluster/
└── flux-system/
```

**Repository practices:**
- One repository per organization or team
- Separate infrastructure and application manifests
- Use Kustomize overlays for environment-specific config
- Keep secrets encrypted with SOPS
- Use meaningful commit messages

### CNI Selection

Choose CNI based on requirements, not defaults.

**Use Calico when:**
- You need full-featured network policies
- You want BGP routing capabilities
- You need Windows node support
- You prefer mature, well-tested CNI

```yaml
opencenter:
  cluster:
    kubernetes:
      network_plugin:
        calico:
          enabled: true
          encapsulation_type: "VXLAN"
          nat_outgoing: true
```

**Use Cilium when:**
- You want eBPF-based networking
- You need advanced observability
- You want service mesh features
- You prioritize performance

```yaml
opencenter:
  cluster:
    kubernetes:
      network_plugin:
        calico:
          enabled: false
        cilium:
          enabled: true
          kubeProxyReplacement: true
```

**Use KubeOVN when:**
- You need deep OpenStack Neutron integration
- You want advanced network features
- You have OVN expertise

**Avoid changing CNI after deployment.** CNI changes require cluster rebuild.

## Troubleshooting Common Issues

### Quota Exhaustion

**Symptom:** Terraform fails with "Quota exceeded" error.

**Solution:**
1. Check current quota usage: `openstack quota show`
2. Request quota increase from OpenStack administrator
3. Clean up unused resources
4. Reduce cluster size if quota increase not possible

### Floating IP Allocation Failure

**Symptom:** Cannot allocate floating IP during deployment.

**Solution:**
1. Check floating IP quota: `openstack floating ip list`
2. Release unused floating IPs
3. Request quota increase
4. Verify `floating_ip_pool` configuration matches available pool

### VRRP Failover Delay

**Symptom:** API unavailable for 10-30 seconds during master failure.

**Solution:**
- This is expected VRRP behavior
- Use Octavia load balancer for faster failover
- Configure monitoring to detect and alert on failover events
- Plan maintenance to minimize impact

### Certificate Expiration

**Symptom:** API server authentication fails, kubectl commands fail.

**Solution:**
1. Enable certificate rotation: `kubelet_rotate_server_certificates: true`
2. Manually renew certificates if already expired
3. Set up monitoring for certificate expiration
4. Plan certificate renewal during maintenance windows

### Calico Pods CrashLoopBackOff

**Symptom:** Calico node pods fail to start, networking broken.

**Solution:**
1. Check interface detection: Set `cni_iface` explicitly
2. Verify MTU settings match network
3. Check for IP conflicts
4. Review Calico logs: `kubectl logs -n kube-system -l k8s-app=calico-node`

### Slow Image Pulls

**Symptom:** Pods take long time to start, image pull timeouts.

**Solution:**
1. Increase worker volume size for image cache
2. Use local container registry
3. Pre-pull common images
4. Check network bandwidth to image registries

## Performance Optimization

### Node Placement

Use server groups with anti-affinity to distribute nodes across compute hosts.

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        server_group_affinity: "soft-anti-affinity"
```

**Affinity options:**
- `anti-affinity`: Hard requirement, may fail if insufficient hosts
- `soft-anti-affinity`: Best effort, allows deployment if insufficient hosts
- `affinity`: Place on same host (rarely used)

**Recommendation:** Use `soft-anti-affinity` for production. It provides distribution without blocking deployment.

### Network Performance

Configure MTU correctly for your network.

```yaml
opencenter:
  cluster:
    kubernetes:
      networking:
        vlan:
          mtu: 1500  # Adjust based on network
```

**MTU considerations:**
- Standard Ethernet: 1500
- Jumbo frames: 9000
- VXLAN overhead: Reduce by 50 bytes
- Verify with network team

### Storage Performance

Choose appropriate volume types for workload requirements.

**IOPS requirements:**
- Database workloads: Use `HA-Performance` (SSD)
- Log aggregation: Use `HA-Performance` (SSD)
- General workloads: Use `HA-Standard` (HDD)
- Ephemeral data: Use local storage

**Volume placement:**
- Spread volumes across availability zones
- Monitor volume IOPS and latency
- Scale horizontally rather than vertically

## Security Considerations

### Network Segmentation

Isolate cluster networks from other infrastructure.

**Security groups:**
- Restrict ingress to required ports only
- Use separate security groups for masters, workers, bastion
- Block inter-node traffic from external networks
- Allow only necessary egress traffic

**Firewall rules:**
- Masters: 6443 (API), 2379-2380 (etcd), 10250 (kubelet)
- Workers: 10250 (kubelet), 30000-32767 (NodePort)
- Bastion: 22 (SSH)

### Credential Management

Protect OpenStack credentials carefully.

**Application credentials:**
- Create per-cluster credentials
- Limit scope to required operations
- Rotate credentials regularly
- Revoke credentials when cluster is destroyed

**SSH keys:**
- Generate unique keys per cluster
- Store private keys securely
- Use SSH agent forwarding through bastion
- Rotate keys periodically

### Audit Logging

Enable audit logging for compliance and security monitoring.

```yaml
opencenter:
  cluster:
    kubernetes:
      security:
        k8s_hardening: true  # Enables audit logging
```

**Audit log retention:**
- Retain logs for compliance period (typically 90 days)
- Ship logs to central logging system
- Monitor for suspicious activity
- Review logs during security incidents

### Compliance

Configure clusters to meet compliance requirements.

**CIS Benchmarks:**
- Enable `k8s_hardening` for CIS Kubernetes Benchmark
- Enable `os_hardening` for CIS OS Benchmark
- Run compliance scans regularly
- Remediate findings promptly

**Pod Security:**
```yaml
opencenter:
  cluster:
    kubernetes:
      security:
        pod_security_exemptions:
          - "kube-system"
          - "monitoring"
```

Minimize exemptions. Each exemption increases security risk.

## Cost Optimization

### Right-Sizing

Start small and scale based on actual usage.

**Initial deployment:**
- 3 masters: `gp.0.4.8` (4 vCPU, 8 GB)
- 2 workers: `gp.0.4.16` (4 vCPU, 16 GB)
- Monitor resource usage for 1-2 weeks
- Scale up or down based on metrics

**Avoid over-provisioning:**
- Don't allocate resources "just in case"
- Use horizontal pod autoscaling
- Use cluster autoscaling (when available)
- Review resource requests and limits

### Storage Costs

Optimize storage usage to reduce costs.

**Volume management:**
- Delete unused volumes
- Use appropriate volume types (HDD vs SSD)
- Implement volume lifecycle policies
- Monitor volume usage

**Storage classes:**
- Use `delete` reclaim policy for ephemeral data
- Use `retain` only for critical data
- Clean up retained volumes manually

### Network Costs

Minimize data transfer costs.

**Floating IPs:**
- Use minimum required floating IPs
- Share floating IPs where possible
- Release unused floating IPs

**Data transfer:**
- Use internal networks for cluster communication
- Cache container images locally
- Minimize external API calls

## Related Documentation

### Setup and Configuration
- [OpenStack Provider Overview](README.md) - Architecture and features
- [OpenStack Setup Guide](setup.md) - Initial setup instructions
- [Network Configuration Guide](networking.md) - Network topology and options

### Operations
- [Troubleshooting Guide](troubleshooting.md) - OpenStack-specific issues
- [Upgrading Clusters](../../how-to/upgrading-clusters.md) - Upgrade procedures
- [Backup and Recovery](../../how-to/backup-recovery.md) - Disaster recovery
- [Monitoring Guide](../../how-to/monitoring.md) - Monitoring setup

### Reference
- [Configuration Reference](../../reference/configuration.md) - Complete configuration options
- [Error Codes Reference](../../reference/error-codes.md) - Error code meanings

### Other Providers
- [Provider Comparison](../README.md) - Compare providers
- [Talos Provider](../talos/README.md) - Alternative provider

## External Resources

- [Kubernetes Production Best Practices](https://kubernetes.io/docs/setup/best-practices/)
- [OpenStack Operations Guide](https://docs.openstack.org/operations-guide/)
- [Kubespray Best Practices](https://kubespray.io/#/docs/operations)
- [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes)
- [Calico Best Practices](https://docs.tigera.io/calico/latest/operations/best-practices)

---

**Last Updated**: January 2026  
**Maintained By**: openCenter Team

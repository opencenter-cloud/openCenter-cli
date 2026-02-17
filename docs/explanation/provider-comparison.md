# Provider Comparison

**Purpose:** For decision makers, explains how to choose the right infrastructure provider, covering provider characteristics through migration considerations.

Choosing the right infrastructure provider affects cost, operational complexity, and feature availability. This explanation helps you make informed provider decisions based on your requirements.

## Provider Overview

openCenter supports six infrastructure providers with different maturity levels:

| Provider | Status | Use Case | Automation | Cost |
|----------|--------|----------|------------|------|
| OpenStack | Production | Private cloud, multi-tenant | Full | Medium |
| VMware | Production | Enterprise, existing vSphere | Partial | High |
| Kind | Development | Local testing, CI/CD | Full | Free |
| AWS | Experimental | Public cloud, managed services | Partial | High |
| Baremetal | Planned | Physical servers, edge | Manual | Low |
| Talos | Planned | Immutable infrastructure | Full | Medium |

**Evidence:** `docs/providers/README.md:1-20`, `internal/config/defaults.go:27-31`, Session 2 B0 section 4

## Provider Characteristics

### OpenStack

**Maturity:** Production-ready, most mature provider

**Automation:**
- Full Terraform/OpenTofu automation
- Automatic network creation
- Automatic load balancer provisioning
- Automatic volume management

**Features:**
- Multi-tenancy (projects, quotas)
- Cinder CSI (block storage)
- Octavia load balancers (optional)
- VRRP load balancers (alternative)
- Barbican secrets backend

**Requirements:**
- OpenStack cloud (public or private)
- API credentials (username, password, project)
- Network quota (1 network, 1 subnet, 1 router)
- Compute quota (N instances, M vCPUs, X GB RAM)
- Storage quota (Y GB volumes)

**Default Configuration:**

```yaml
opencenter:
  infrastructure:
    provider: openstack
    openstack:
      region: sjc3
      availability_zone: az1
      project_domain: rackspace_cloud_domain
      user_domain: rackspace_cloud_domain
      image_id: "799dcf97-3656-4361-8187-13ab1b295e33"  # Ubuntu 24.04
      floating_ip_pool: PUBLICNET
```

**Why choose OpenStack:**
- Need private cloud (data residency, compliance)
- Multi-tenant requirements (multiple teams/projects)
- Full automation (infrastructure as code)
- Cost optimization (no cloud provider markup)

**Trade-offs:** Requires OpenStack expertise. More complex than managed services. But provides full control and flexibility.

**Evidence:** `internal/config/defaults.go:68-157`, `docs/providers/README.md:7`, Session 2 B0 section 5

### VMware vSphere

**Maturity:** Production-ready, pre-provisioned VMs

**Automation:**
- Manual VM provisioning (vSphere UI or Terraform)
- Automatic Kubernetes deployment (Kubespray)
- Automatic service deployment (FluxCD)

**Features:**
- vSphere CSI (block storage, file storage)
- Enterprise integration (vCenter, NSX)
- High availability (vMotion, DRS)
- Existing infrastructure (no new cloud)

**Requirements:**
- Pre-provisioned VMs (Ubuntu 24.04)
- SSH access to VMs
- vSphere credentials (for CSI driver)
- Network connectivity (VMs can reach each other)

**Configuration:**

```yaml
opencenter:
  infrastructure:
    provider: vmware
    vmware:
      vcenter_server: vcenter.example.com
      datacenter: DC1
      datastore: datastore1
      network: VM Network
      folder: /DC1/vm/kubernetes
```

**Why choose VMware:**
- Existing vSphere infrastructure
- Enterprise requirements (support, SLAs)
- No public cloud (air-gapped, compliance)
- Familiar tooling (vCenter, vSphere)

**Trade-offs:** Manual VM provisioning. Higher licensing costs. But leverages existing infrastructure and expertise.

**Evidence:** `docs/providers/README.md:8`, Session 2 B0 section 4

### Kind

**Maturity:** Development-ready, not for production

**Automation:**
- Full automation (Docker containers)
- Fast cluster creation (< 5 minutes)
- Easy cleanup (delete containers)

**Features:**
- Local development (no cloud required)
- CI/CD testing (GitHub Actions, GitLab CI)
- Multi-node clusters (control plane + workers)
- Load balancer simulation (MetalLB)

**Requirements:**
- Docker installed
- 8 GB RAM minimum
- 20 GB disk space

**Configuration:**

```yaml
opencenter:
  infrastructure:
    provider: kind
    kind:
      cluster_name: dev-cluster
      control_plane_nodes: 1
      worker_nodes: 2
```

**Why choose Kind:**
- Local development (no cloud costs)
- Fast iteration (quick cluster creation)
- CI/CD testing (automated testing)
- Learning (experiment without cost)

**Trade-offs:** Not production-ready. Limited resources. No persistent storage. But perfect for development and testing.

**Evidence:** `docs/providers/README.md:12`, Session 2 B0 section 4

### AWS

**Maturity:** Experimental, in development

**Automation:**
- Partial Terraform automation
- EC2 instance provisioning
- VPC and subnet creation
- EBS volume management

**Features:**
- Public cloud (global availability)
- Managed services (RDS, S3, etc.)
- Elastic scaling (auto-scaling groups)
- Pay-as-you-go pricing

**Requirements:**
- AWS account
- IAM credentials (access key, secret key)
- VPC quota
- EC2 quota

**Configuration:**

```yaml
opencenter:
  infrastructure:
    provider: aws
    aws:
      region: us-east-1
      availability_zones: [us-east-1a, us-east-1b]
      instance_type: t3.medium
      ami_id: ami-0c55b159cbfafe1f0  # Ubuntu 24.04
```

**Why choose AWS:**
- Public cloud (no infrastructure management)
- Global availability (multiple regions)
- Managed services (reduce operational overhead)
- Elastic scaling (handle variable load)

**Trade-offs:** Higher costs. Vendor lock-in. Experimental status (not production-ready yet).

**Evidence:** `docs/providers/README.md:16`, Session 2 B0 section 4

### Baremetal

**Maturity:** Planned, not implemented

**Automation:**
- Manual server provisioning
- Automatic Kubernetes deployment (Kubespray)
- Automatic service deployment (FluxCD)

**Features:**
- Physical servers (maximum performance)
- No virtualization overhead
- Edge deployments (remote locations)
- Cost optimization (no cloud markup)

**Requirements:**
- Physical servers (Ubuntu 24.04)
- SSH access
- Network connectivity
- IPMI/BMC access (optional)

**Why choose Baremetal:**
- Maximum performance (no virtualization)
- Edge deployments (remote locations)
- Cost optimization (own hardware)
- Compliance (data residency)

**Trade-offs:** Manual provisioning. Hardware management. Longer deployment time. But provides maximum control and performance.

**Evidence:** `docs/providers/README.md:17`, Session 2 B0 section 4

### Talos

**Maturity:** Planned, not implemented

**Automation:**
- Full Pulumi automation
- Immutable infrastructure
- Declarative configuration

**Features:**
- Talos Linux (minimal, immutable)
- API-driven (no SSH)
- Secure by default (no shell, no package manager)
- Fast updates (image-based)

**Requirements:**
- Talos-compatible infrastructure
- Pulumi installed
- Talos CLI (talosctl)

**Why choose Talos:**
- Immutable infrastructure (security, consistency)
- API-driven (no SSH access)
- Fast updates (image-based)
- Minimal attack surface (no shell)

**Trade-offs:** Different operational model (no SSH). Requires Talos expertise. But provides strong security and consistency.

**Evidence:** `docs/providers/README.md:18`, Session 2 B0 section 4

## Decision Framework

### By Use Case

**Development and Testing:**
- **Best:** Kind (fast, free, local)
- **Alternative:** OpenStack (if cloud available)

**Production (Private Cloud):**
- **Best:** OpenStack (full automation, multi-tenant)
- **Alternative:** VMware (if vSphere available)

**Production (Public Cloud):**
- **Best:** AWS (when mature)
- **Alternative:** OpenStack (private cloud provider)

**Enterprise (Existing Infrastructure):**
- **Best:** VMware (leverage existing vSphere)
- **Alternative:** Baremetal (physical servers)

**Edge Deployments:**
- **Best:** Baremetal (physical servers at edge)
- **Alternative:** Talos (immutable, secure)

**High Security:**
- **Best:** Talos (immutable, minimal)
- **Alternative:** Baremetal (air-gapped)

### By Automation Level

**Full Automation (Infrastructure + Kubernetes):**
- OpenStack
- Kind
- Talos (planned)

**Partial Automation (Kubernetes only):**
- VMware (manual VM provisioning)
- AWS (experimental)
- Baremetal (planned)

### By Cost

**Free:**
- Kind (local development)

**Low Cost:**
- Baremetal (own hardware)
- OpenStack (private cloud, no markup)

**Medium Cost:**
- OpenStack (public cloud provider)
- Talos (infrastructure costs only)

**High Cost:**
- VMware (licensing costs)
- AWS (cloud provider markup)

### By Operational Complexity

**Low Complexity:**
- Kind (Docker only)
- AWS (managed services)

**Medium Complexity:**
- OpenStack (standard cloud)
- Talos (API-driven)

**High Complexity:**
- VMware (vSphere expertise)
- Baremetal (hardware management)

## Feature Comparison

### Storage

| Provider | Block Storage | File Storage | Object Storage |
|----------|---------------|--------------|----------------|
| OpenStack | Cinder CSI | Manila CSI (optional) | Swift (optional) |
| VMware | vSphere CSI | vSphere CSI | External (S3) |
| Kind | Local volumes | Local volumes | External (S3) |
| AWS | EBS CSI | EFS CSI | S3 |
| Baremetal | Longhorn | Longhorn | External (S3) |
| Talos | Longhorn | Longhorn | External (S3) |

### Networking

| Provider | CNI | Load Balancer | Ingress |
|----------|-----|---------------|---------|
| OpenStack | Calico, Cilium, Kube-OVN | Octavia, VRRP | Gateway API, Ingress NGINX |
| VMware | Calico, Cilium | MetalLB | Gateway API, Ingress NGINX |
| Kind | Calico | MetalLB | Gateway API, Ingress NGINX |
| AWS | Calico, AWS VPC CNI | ELB, NLB | Gateway API, ALB |
| Baremetal | Calico, Cilium | MetalLB | Gateway API, Ingress NGINX |
| Talos | Calico, Cilium | MetalLB | Gateway API, Ingress NGINX |

### High Availability

| Provider | Control Plane HA | Worker HA | Storage HA |
|----------|------------------|-----------|------------|
| OpenStack | Yes (3+ masters) | Yes (N workers) | Yes (Cinder replication) |
| VMware | Yes (3+ masters) | Yes (N workers) | Yes (vSphere HA) |
| Kind | No (single node) | No (local only) | No (local volumes) |
| AWS | Yes (3+ masters) | Yes (N workers) | Yes (EBS replication) |
| Baremetal | Yes (3+ masters) | Yes (N workers) | Yes (Longhorn replication) |
| Talos | Yes (3+ masters) | Yes (N workers) | Yes (Longhorn replication) |

### Security

| Provider | Encryption at Rest | Network Isolation | Secrets Backend |
|----------|-------------------|-------------------|-----------------|
| OpenStack | Yes (Cinder encryption) | Yes (security groups) | Barbican |
| VMware | Yes (vSphere encryption) | Yes (NSX, security groups) | External (Vault) |
| Kind | No (local volumes) | No (single host) | External (Vault) |
| AWS | Yes (EBS encryption) | Yes (security groups) | AWS Secrets Manager |
| Baremetal | Yes (LUKS encryption) | Yes (NetworkPolicies) | External (Vault) |
| Talos | Yes (LUKS encryption) | Yes (NetworkPolicies) | External (Vault) |

## Migration Considerations

### Switching Providers

**Scenario:** Migrate from OpenStack to VMware

**Process:**

1. **Backup:** Backup cluster data (Velero)
2. **Export:** Export application manifests
3. **Create:** Create new cluster with VMware provider
4. **Restore:** Restore data to new cluster
5. **Validate:** Validate applications
6. **Cutover:** Switch traffic to new cluster
7. **Decommission:** Delete old cluster

**Challenges:**
- Storage migration (different CSI drivers)
- Network configuration (different load balancers)
- Service configuration (provider-specific settings)

**Mitigation:** Test migration in dev environment first. Use Velero for data migration. Update DNS gradually (canary cutover).

### Provider-Specific Configuration

**Challenge:** Configuration is provider-specific (can't directly migrate).

**Example:**

OpenStack configuration:
```yaml
opencenter:
  infrastructure:
    provider: openstack
    openstack:
      region: sjc3
      image_id: "799dcf97-3656-4361-8187-13ab1b295e33"
```

VMware configuration:
```yaml
opencenter:
  infrastructure:
    provider: vmware
    vmware:
      vcenter_server: vcenter.example.com
      datacenter: DC1
```

**Solution:** Create new configuration file for target provider. Copy application-level settings (services, networking). Update provider-specific settings.

### Data Migration

**Challenge:** Storage is provider-specific (Cinder vs vSphere).

**Solution:** Use Velero for backup/restore:

```bash
# Backup from OpenStack cluster
velero backup create migration-backup --include-namespaces app1,app2

# Restore to VMware cluster
velero restore create --from-backup migration-backup
```

**Limitations:** Persistent volumes must be recreated (different storage classes). Application data is preserved.

## Common Misconceptions

### "Provider choice is permanent"

**Reality:** Providers can be changed, but it requires cluster rebuild and data migration. Not a simple configuration change.

### "All providers have the same features"

**Reality:** Features vary by provider. OpenStack has Barbican secrets backend. VMware has vSphere HA. Kind has no persistent storage.

### "Kind is production-ready"

**Reality:** Kind is for development and testing only. Not suitable for production (single host, no HA, no persistent storage).

### "VMware requires vSphere CSI"

**Reality:** vSphere CSI is optional. Can use Longhorn or other storage solutions. But vSphere CSI provides better integration.

### "OpenStack is only for private cloud"

**Reality:** OpenStack is available as public cloud (Rackspace, OVH, etc.) and private cloud (on-premises).

## Best Practices

### 1. Match Provider to Use Case

**Practice:** Choose provider based on requirements, not familiarity.

**Example:** Use Kind for development, OpenStack for production.

**Rationale:** Each provider has strengths and weaknesses. Match to use case for best results.

### 2. Test in Dev First

**Practice:** Test provider in dev environment before production.

**Workflow:**

```
1. Create dev cluster with provider
2. Deploy applications
3. Test functionality
4. Validate performance
5. Deploy to production
```

**Rationale:** Catch provider-specific issues in dev, not prod.

### 3. Document Provider-Specific Decisions

**Practice:** Document why provider was chosen and any limitations.

**Example:**

```markdown
# Provider Decision

**Chosen:** OpenStack

**Rationale:**
- Private cloud requirement (data residency)
- Multi-tenant support (multiple teams)
- Full automation (infrastructure as code)

**Limitations:**
- Requires OpenStack expertise
- Higher operational complexity than managed services
```

**Rationale:** Context for future maintainers. Audit trail for decisions.

### 4. Plan for Migration

**Practice:** Design applications for portability.

**Guidelines:**
- Use standard Kubernetes APIs (no provider-specific CRDs)
- Use standard storage classes (no provider-specific features)
- Use standard networking (Gateway API, not provider-specific)

**Rationale:** Easier migration if provider needs to change.

### 5. Monitor Provider Costs

**Practice:** Track infrastructure costs by provider.

**Tools:**
- OpenStack: Cloudkitty
- VMware: vRealize Operations
- AWS: Cost Explorer

**Rationale:** Optimize costs. Identify cost trends. Justify provider choice.

## Further Reading

- [Architecture](architecture.md) - System design and components
- [Configuration Lifecycle](configuration-lifecycle.md) - Configuration management
- [Providers Reference](../reference/providers.md) - Provider details and configuration
- [OpenStack First Cluster](../tutorials/openstack-first-cluster.md) - OpenStack tutorial
- [VMware Deployment](../tutorials/vmware-deployment.md) - VMware tutorial
- [Kind Local Development](../tutorials/kind-local-development.md) - Kind tutorial

---

## Evidence

This explanation is based on:

- Provider overview: `docs/providers/README.md:1-20`, Session 2 B0 section 4
- Provider defaults: `internal/config/defaults.go:27-157`
- OpenStack configuration: `internal/config/defaults.go:68-157`
- Provider comparison: Ecosystem.md provider comparison
- Feature matrix: Session 2 B0 sections 4-6
- Migration considerations: Session 1 A5

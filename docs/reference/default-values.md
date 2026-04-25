---
id: default-values
title: "Default Values Reference"
sidebar_label: Default Values
description: Complete reference of default configuration values used when initializing clusters.
doc_type: reference
audience: "all users"
tags: [defaults, configuration, values, reference]
---

# Default Values Reference

**Purpose:** Complete reference of default configuration values by provider for quick lookup.

This reference documents all default values used when initializing cluster configurations.

## Schema Version

**Default:** `"2.0"`

All new configurations use schema version 2.0.

## Cluster Metadata

| Field | Default | Description |
|-------|---------|-------------|
| `opencenter.meta.organization` | `"opencenter"` | Organization name |
| `opencenter.meta.env` | `""` | Environment (empty by default) |
| `opencenter.meta.region` | `"sjc3"` | Cloud region |
| `opencenter.meta.status` | `""` | Cluster status |

## Infrastructure

| Field | Default | Description |
|-------|---------|-------------|
| `opencenter.infrastructure.provider` | `"openstack"` | Infrastructure provider |
| `opencenter.infrastructure.ssh_user` | `"ubuntu"` | SSH user for nodes |
| `opencenter.infrastructure.os_version` | `"24"` | Ubuntu version |
| `opencenter.infrastructure.server_group_affinity` | `["anti-affinity"]` | Server group policy |
| `opencenter.infrastructure.node_naming.worker` | `"wn"` | Worker node prefix |
| `opencenter.infrastructure.node_naming.master` | `"cp"` | Control plane prefix |
| `opencenter.infrastructure.node_naming.worker_windows` | `"win"` | Windows worker prefix |
| `opencenter.infrastructure.bastion.address` | `"localhost"` | Bastion host address |

## OpenStack Provider Defaults

| Field | Default | Description |
|-------|---------|-------------|
| `opencenter.infrastructure.cloud.openstack.region` | `"sjc3"` | OpenStack region |
| `opencenter.infrastructure.cloud.openstack.insecure` | `false` | Skip TLS verification |
| `opencenter.infrastructure.cloud.openstack.availability_zone` | `"az1"` | Availability zone |
| `opencenter.infrastructure.cloud.openstack.project_domain_name` | `"rackspace_cloud_domain"` | Project domain |
| `opencenter.infrastructure.cloud.openstack.user_domain_name` | `"rackspace_cloud_domain"` | User domain |
| `opencenter.infrastructure.cloud.openstack.image_id` | `"799dcf97-3656-4361-8187-13ab1b295e33"` | Ubuntu 24.04 image |
| `opencenter.infrastructure.cloud.openstack.image_id_windows` | `"a2083759-f341-445b-b717-dafb5e31fa6b"` | Windows Server image |
| `opencenter.infrastructure.cloud.openstack.networking.floating_ip_pool` | `"PUBLICNET"` | Floating IP pool |
| `opencenter.infrastructure.cloud.openstack.networking.router_external_network_id` | `"723f8fa2-dbf7-4cec-8d5f-017e62c12f79"` | External network |
| `opencenter.infrastructure.cloud.openstack.networking.k8s_api_port_acl` | `["0.0.0.0/0"]` | API access CIDR |
| `opencenter.infrastructure.cloud.openstack.networking.vlan.provider` | `"physnet1"` | VLAN provider |

## Cluster Configuration

| Field | Default | Description |
|-------|---------|-------------|
| `opencenter.cluster.base_domain` | `"k8s.opencenter.cloud"` | Base domain |
| `opencenter.cluster.cluster_fqdn` | `"<name>.<region>.k8s.opencenter.cloud"` | Cluster FQDN |
| `opencenter.cluster.admin_email` | `""` | Administrator email |
| `opencenter.cluster.k8s_api_port_acl` | `["0.0.0.0/0"]` | API access CIDR |

## Network Configuration

| Field | Default | Description |
|-------|---------|-------------|
| `opencenter.cluster.networking.ntp_servers` | `["time.<region>.rackspace.com", "time2.<region>.rackspace.com"]` | NTP servers |
| `opencenter.cluster.networking.dns_nameservers` | `["8.8.8.8", "8.8.4.4"]` | DNS servers |
| `opencenter.cluster.networking.security.os_hardening` | `true` | OS security hardening |
| `opencenter.cluster.networking.subnet_nodes` | `"10.2.128.0/22"` | Node network CIDR |
| `opencenter.cluster.networking.vrrp_enabled` | `true` | Enable VRRP |
| `opencenter.cluster.networking.use_octavia` | `false` | Use Octavia LB |
| `opencenter.cluster.networking.loadbalancer_provider` | `"ovn"` | Load balancer provider |
| `opencenter.cluster.networking.use_designate` | `false` | Use Designate DNS |
| `opencenter.cluster.networking.vlan.provider` | `"physnet1"` | VLAN provider |

## Kubernetes Configuration

| Field | Default | Description |
|-------|---------|-------------|
| `opencenter.cluster.kubernetes.version` | `"1.33.5"` | Kubernetes version |
| `opencenter.cluster.kubernetes.kubespray_version` | `"v2.29.1"` | Kubespray version |
| `opencenter.cluster.kubernetes.api_port` | `443` | API server port |
| `opencenter.cluster.kubernetes.kube_vip_enabled` | `true` | Enable Kube-VIP |
| `opencenter.cluster.kubernetes.kubelet_rotate_server_certs` | `false` | Rotate kubelet certs |
| `opencenter.cluster.kubernetes.flavor_bastion` | `"gp.0.2.2"` | Bastion flavor |
| `opencenter.cluster.kubernetes.flavor_master` | `"gp.0.4.8"` | Control plane flavor |
| `opencenter.cluster.kubernetes.flavor_worker` | `"gp.0.4.16"` | Worker flavor |
| `opencenter.cluster.kubernetes.flavor_worker_windows` | `"gp.5.4.16"` | Windows worker flavor |
| `opencenter.cluster.kubernetes.subnet_pods` | `"10.42.0.0/16"` | Pod network CIDR |
| `opencenter.cluster.kubernetes.subnet_services` | `"10.43.0.0/16"` | Service network CIDR |
| `opencenter.cluster.kubernetes.loadbalancer_provider` | `"ovn"` | Load balancer provider |
| `opencenter.cluster.kubernetes.master_count` | `3` | Control plane nodes |
| `opencenter.cluster.kubernetes.worker_count` | `2` | Worker nodes |
| `opencenter.cluster.kubernetes.worker_count_windows` | `0` | Windows workers |
| `opencenter.cluster.kubernetes.security.k8s_hardening` | `true` | Kubernetes hardening |
| `opencenter.cluster.kubernetes.security.pod_security_exemptions` | `["trivy-temp", "tigera-operator", "kube-system"]` | PSS exemptions |

## CNI Plugin Defaults

### Calico (Default)

| Field | Default | Description |
|-------|---------|-------------|
| `opencenter.cluster.kubernetes.network_plugin.calico.enabled` | `true` | Enable Calico |
| `opencenter.cluster.kubernetes.network_plugin.calico.cni_iface` | `"enp3s0"` | Network interface |
| `opencenter.cluster.kubernetes.network_plugin.calico.calico_interface_autodetect` | `"interface"` | Interface detection |
| `opencenter.cluster.kubernetes.network_plugin.calico.encapsulation_type` | `"VXLAN"` | Encapsulation type |
| `opencenter.cluster.kubernetes.network_plugin.calico.nat_outgoing` | `true` | NAT outgoing traffic |

### Cilium

| Field | Default | Description |
|-------|---------|-------------|
| `opencenter.cluster.kubernetes.network_plugin.cilium.enabled` | `false` | Enable Cilium |
| `opencenter.cluster.kubernetes.network_plugin.cilium.operator_enabled` | `true` | Enable operator |
| `opencenter.cluster.kubernetes.network_plugin.cilium.kube_proxy_replacement` | `true` | Replace kube-proxy |

### Kube-OVN

| Field | Default | Description |
|-------|---------|-------------|
| `opencenter.cluster.kubernetes.network_plugin.kube-ovn.enabled` | `false` | Enable Kube-OVN |
| `opencenter.cluster.kubernetes.network_plugin.kube-ovn.cilium_integration` | `true` | Cilium integration |

## OIDC Configuration

| Field | Default | Description |
|-------|---------|-------------|
| `opencenter.cluster.kubernetes.oidc.enabled` | `false` | Enable OIDC |
| `opencenter.cluster.kubernetes.oidc.kube_oidc_client_id` | `"kubernetes"` | OIDC client ID |
| `opencenter.cluster.kubernetes.oidc.kube_oidc_username_claim` | `"sub"` | Username claim |
| `opencenter.cluster.kubernetes.oidc.kube_oidc_username_prefix` | `"oidc:"` | Username prefix |
| `opencenter.cluster.kubernetes.oidc.kube_oidc_groups_claim` | `"groups"` | Groups claim |
| `opencenter.cluster.kubernetes.oidc.kube_oidc_groups_prefix` | `"oidc:"` | Groups prefix |

## Windows Workers

| Field | Default | Description |
|-------|---------|-------------|
| `opencenter.cluster.kubernetes.windows_workers.enabled` | `false` | Enable Windows workers |
| `opencenter.cluster.kubernetes.windows_workers.windows_user` | `"Administrator"` | Windows user |

## GitOps Configuration

| Field | Default | Description |
|-------|---------|-------------|
| `opencenter.gitops.git_dir` | `"./testdata/test-git-repo-<name>"` | Git directory |
| `opencenter.gitops.git_branch` | `"main"` | Git branch |
| `opencenter.gitops.gitops_base_repo` | `"ssh://git@github.com/opencenter-cloud/opencenter-gitops-base.git"` | Base repo |
| `opencenter.gitops.gitops_branch` | `"main"` | Base repo branch |
| `opencenter.gitops.flux.interval` | `"15m"` | Flux reconciliation |
| `opencenter.gitops.flux.prune` | `true` | Prune resources |

## Storage Configuration

| Field | Default | Description |
|-------|---------|-------------|
| `opencenter.storage.default_storage_class` | `"csi-cinder-sc-delete"` | Default storage class |
| `opencenter.storage.worker_volume_size` | `40` | Worker volume size (GB) |
| `opencenter.storage.worker_volume_destination_type` | `"volume"` | Volume destination |
| `opencenter.storage.worker_volume_source_type` | `"image"` | Volume source |
| `opencenter.storage.worker_volume_type` | `"HA-Standard"` | Volume type |

## Platform Services Defaults

### Enabled by Default (OpenStack)

| Service | Enabled | Description |
|---------|---------|-------------|
| `calico` | `true` | CNI networking |
| `cert-manager` | `true` | TLS certificate management |
| `etcd-backup` | `true` | Etcd backup to S3 |
| `external-snapshotter` | `true` | Volume snapshots |
| `fluxcd` | `true` | GitOps controller |
| `gateway` | `true` | Gateway implementation |
| `gateway-api` | `true` | Gateway API CRDs |
| `headlamp` | `true` | Kubernetes dashboard |
| `keycloak` | `true` | Identity management |
| `kube-prometheus-stack` | `true` | Monitoring |
| `kyverno` | `true` | Policy engine |
| `loki` | `true` | Log aggregation |
| `olm` | `true` | Operator Lifecycle Manager |
| `openstack-ccm` | `true` | OpenStack cloud controller |
| `openstack-csi` | `true` | OpenStack CSI driver |
| `postgres-operator` | `true` | PostgreSQL operator |
| `rbac-manager` | `true` | RBAC management |
| `sources` | `true` | FluxCD sources |
| `tempo` | `true` | Distributed tracing |
| `velero` | `true` | Backup and DR |

### Disabled by Default

| Service | Enabled | Description |
|---------|---------|-------------|
| `alert-proxy` | `false` | Alert forwarding (requires config) |
| `vsphere-csi` | `false` | VMware CSI driver |
| `weave-gitops` | `false` | Weave GitOps UI |

## Service-Specific Defaults

### cert-manager

| Field | Default |
|-------|---------|
| `email` | `"mpk-support@rackspace.com"` |
| `region` | `"us-east-1"` |
| `letsencrypt_server` | `"https://acme-v02.api.letsencrypt.org/directory"` |

### etcd-backup

| Field | Default |
|-------|---------|
| `s3_host` | `"https://swift.api.dfw3.rackspacecloud.com"` |
| `s3_region` | `"DFW3"` |

### headlamp

| Field | Default |
|-------|---------|
| `hostname` | `"dashboard.<org>.<cluster>.<region>.k8s.opencenter.cloud"` |
| `oidc_issuer_url` | `"https://auth.<org>.<cluster>.<region>.k8s.opencenter.cloud/realms/opencenter"` |
| `oidc_client_id` | `"kubernetes"` |

### keycloak

| Field | Default |
|-------|---------|
| `hostname` | `"auth.<org>.<cluster>.<region>.k8s.opencenter.cloud"` |
| `realm` | `"opencenter"` |
| `client_id` | `"kubernetes"` |
| `frontend_url` | `"https://auth.<org>.<cluster>.<region>.k8s.opencenter.cloud"` |

### kube-prometheus-stack

| Field | Default |
|-------|---------|
| `prometheus_volume_size` | `50` (GB) |
| `prometheus_storage_class` | `"csi-cinder-sc-delete"` |
| `grafana_volume_size` | `10` (GB) |
| `grafana_storage_class` | `"csi-cinder-sc-delete"` |
| `alertmanager_volume_size` | `10` (GB) |
| `alertmanager_storage_class` | `"csi-cinder-sc-delete"` |

### loki

| Field | Default |
|-------|---------|
| `volume_size` | `20` (GB) |
| `storage_class` | `"csi-cinder-sc-delete"` |
| `bucket_name` | `"<cluster>-loki"` |
| `swift_auth_url` | `"https://keystone.api.<region>.rackspacecloud.com/v3/"` |
| `swift_region` | `<region>` (uppercase) |
| `swift_domain_name` | `"Default"` |

### tempo

| Field | Default |
|-------|---------|
| `storage_type` | `"s3"` |
| `bucket_name` | `"<cluster>-tempo"` |
| `volume_size` | `10` (GB) |
| `storage_class` | `"csi-cinder-sc-delete"` |
| `s3_endpoint` | `"https://swift.api.<region>.rackspacecloud.com"` |
| `s3_region` | `<region>` (uppercase) |
| `s3_force_path_style` | `false` |
| `s3_insecure` | `false` |

### velero

| Field | Default |
|-------|---------|
| `backup_bucket` | `"<cluster>-backups"` |
| `region` | `"us-east-1"` |

### vsphere-csi

| Field | Default |
|-------|---------|
| `enabled` | `false` |
| `image_repository` | `"registry.k8s.io/csi-vsphere"` |
| `image_tag` | `"v3.3.0"` |

## OpenTofu Configuration

| Field | Default | Description |
|-------|---------|-------------|
| `opentofu.enabled` | `true` | Enable OpenTofu |
| `opentofu.path` | `"opentofu"` | OpenTofu binary path |
| `opentofu.backend.type` | `"local"` | Backend type |
| `opentofu.backend.local.path` | `".opentofu-local-<name>/terraform.tfstate"` | State file path |

## Deployment Configuration

| Field | Default | Description |
|-------|---------|-------------|
| `deployment.auto_deploy` | `true` | Auto-deploy on setup |

## Secrets Configuration

| Field | Default | Description |
|-------|---------|-------------|
| `secrets.ssh_key.cypher` | `"ed25519"` | SSH key algorithm |

## Provider-Specific Defaults

### OpenStack

- Default provider for new clusters
- Includes OpenStack CCM and CSI drivers
- Uses Cinder for persistent storage
- Supports Octavia load balancers
- Integrates with Designate DNS (optional)

### VMware

- Requires pre-provisioned VMs
- Uses vSphere CSI driver
- MetalLB for load balancing
- No cloud controller manager

### AWS

- Experimental support
- Uses AWS EBS CSI driver
- AWS ELB for load balancing
- AWS cloud controller manager

### Kind

- Local development only
- Single node or multi-node
- No persistent storage by default
- No load balancer

## CLI Configuration Defaults

| Field | Default | Description |
|-------|---------|-------------|
| `defaults.provider` | `""` | Default provider |
| `defaults.region` | `""` | Default region |
| `defaults.environment` | `""` | Default environment |
| `defaults.ssh_authorized_keys` | `[]` | Default SSH keys |

## Configuration Precedence

When multiple sources provide values:

1. Command-line the set override mechanisms (highest priority)
2. Configuration file values
3. CLI defaults (`~/.config/opencenter/config.yaml`)
4. Built-in defaults (lowest priority)

## Overriding Defaults

### Via Configuration File

```yaml
opencenter:
  cluster:
    kubernetes:
      version: "1.34.0"  # Override default 1.33.5
```

### Via Command Line

```bash
opencenter cluster init my-cluster \
  opencenter.cluster.kubernetes.version=1.34.0
```

### Via CLI Defaults

```bash
opencenter config set defaults.provider vmware
opencenter config set defaults.region us-west-2
```

---

## Evidence

This reference is based on:

- Configuration defaults: `internal/config/defaults.go:48-451`
- Service defaults: `internal/config/defaults.go:293-388`
- Network defaults: `internal/config/defaults.go:177-179`
- Kubernetes defaults: `internal/config/defaults.go:197-212`
- Session 2 facts inventory: B0 section 5

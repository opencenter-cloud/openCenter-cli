---
title: Cluster Configuration Architecture
doc_type: reference
weight: 50
---

# Cluster Configuration Architecture


## Table of Contents

- [Who this is for](#who-this-is-for)
- [Configuration Structure Overview](#configuration-structure-overview)
- [Core Configuration Types](#core-configuration-types)
- [Complete Configuration to Terraform Mapping](#complete-configuration-to-terraform-mapping)
- [Template Rendering Flow](#template-rendering-flow)
- [Adding New Configuration Fields](#adding-new-configuration-fields)
- [Service Configuration Architecture](#service-configuration-architecture)
- [Debugging Configuration Issues](#debugging-configuration-issues)
- [Configuration File Locations](#configuration-file-locations)
- [Configuration Precedence and Merging](#configuration-precedence-and-merging)
- [Related Documentation](#related-documentation)
- [Key Files Reference](#key-files-reference)
- [Testing Configuration Changes](#testing-configuration-changes)
- [See Also](#see-also)
This document provides a comprehensive reference for the openCenter cluster configuration structure, including how configuration values map to Terraform/OpenTofu templates and the underlying infrastructure.

## Who this is for

Developers working on openCenter who need to understand:
- Configuration structure and organization
- How YAML config maps to Terraform variables
- Template rendering and variable substitution
- Adding new configuration fields
- Debugging configuration issues

## Configuration Structure Overview

The cluster configuration follows a hierarchical structure defined in `internal/config/`:

```
Config (root)
├── SchemaVersion
├── Metadata (tracking)
├── OpenCenter (main config)
│   ├── Meta (cluster identity)
│   ├── Secrets (backend config)
│   ├── Infrastructure (provider)
│   ├── Cluster (k8s config)
│   ├── GitOps (flux config)
│   ├── Storage (volumes)
│   ├── Talos (optional)
│   ├── Services (service map)
│   └── ManagedService (service map)
├── OpenTofu (IaC backend)
├── Secrets (credentials)
├── Networking (proxy, etc)
└── Deployment (auto-deploy)
```

## Core Configuration Types

### Config Root (`internal/config/config.go`)

```go
type Config struct {
    SchemaVersion string
    OpenCenter    SimplifiedOpenCenter
    OpenTofu      SimplifiedOpenTofu
    Secrets       Secrets
    Networking    Networking
    Deployment    Deployment
    Overrides     map[string]any
    Metadata      ConfigMetadata
}
```

**Purpose**: Top-level configuration container
**File Location**: `~/.config/openCenter/clusters/<org>/<cluster>/.config.yaml`


### OpenCenter Section

The main configuration section containing all cluster-specific settings.

#### Meta (`ClusterMeta`)

```go
type ClusterMeta struct {
    Name         string  // Cluster name
    Organization string  // Organization (multi-tenancy)
    Env          string  // Environment (dev/stage/prod)
    Region       string  // Deployment region
    Status       string  // Cluster status
}
```

**Terraform Mapping**:
```hcl
locals {
  cluster_name = "{{ .OpenCenter.Cluster.ClusterName }}"
  naming_prefix = "${local.cluster_name}-"
}
```

#### Infrastructure (`Infrastructure`)

Provider and infrastructure settings.

```go
type Infrastructure struct {
    Provider            string              // openstack, aws, kind, baremetal
    SSHUser             string              // SSH username
    OSVersion           string              // OS version (e.g., "24")
    ServerGroupAffinity []string            // anti-affinity, affinity
    NodeNaming          NodeNaming          // Node naming conventions
    Bastion             BastionConfig       // Bastion host config
    K8sAPIIP            string              // Kubernetes API IP
    Cloud               CloudConfig         // Cloud provider configs
}
```

**Terraform Mapping**:
```hcl
locals {
  ssh_user = "{{ .OpenCenter.Infrastructure.SSHUser | default "ubuntu" }}"
  ub_version = "{{ .OpenCenter.Infrastructure.OSVersion | default "24" }}"
  node_worker = "{{ .OpenCenter.Infrastructure.NodeNaming.Worker | default "wn" }}"
  node_master = "{{ .OpenCenter.Infrastructure.NodeNaming.Master | default "cp" }}"
  wn_server_group_affinity = [{{ range .OpenCenter.Infrastructure.ServerGroupAffinity }}"{{ . }}",{{ end }}]
}
```


#### Cloud Configuration (`CloudConfig`)

Provider-specific cloud settings.

**OpenStack** (`SimplifiedOpenStackCloud`):

```go
type SimplifiedOpenStackCloud struct {
    AuthURL                     string
    Insecure                    bool
    Region                      string
    ApplicationCredentialID     string
    ApplicationCredentialSecret string
    Domain                      string
    TenantName                  string
    AvailabilityZone            string
    ProjectDomainName           string
    UserDomainName              string
    CA                          string
    ImageID                     string
    ImageIDWindows              string
    Networking                  OpenStackNetworkingConfig
}
```

**Terraform Mapping**:
```hcl
locals {
  openstack_auth_url = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL }}"
  openstack_region = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.Region }}"
  application_credential_id = var.os_application_credential_id
  application_credential_secret = var.os_application_credential_secret
  openstack_project_domain_name = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.ProjectDomainName }}"
  openstack_user_domain_name = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.UserDomainName }}"
  availability_zone = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.AvailabilityZone }}"
  image_id = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.ImageID }}"
  floatingip_pool = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.Networking.FloatingIPPool }}"
}
```

**AWS** (`SimplifiedAWSCloud`):

```go
type SimplifiedAWSCloud struct {
    Profile        string
    Region         string
    VPCID          string
    PrivateSubnets []string
    PublicSubnets  []string
}
```


#### Cluster Configuration (`ClusterConfig`)

Kubernetes cluster settings.

```go
type ClusterConfig struct {
    ClusterName        string
    AWSAccessKey       string
    AWSSecretAccessKey string
    SSHAuthorizedKeys  []string
    BaseDomain         string
    ClusterFQDN        string
    AdminEmail         string
    Kubernetes         KubernetesConfig
    Networking         ClusterNetworkingConfig
}
```

**Terraform Mapping**:
```hcl
locals {
  cluster_name = "{{ .OpenCenter.Cluster.ClusterName }}"
  ssh_authorized_keys = [{{ range .OpenCenter.Cluster.SSHAuthorizedKeys }}"{{ . }}",{{ end }}]
  k8s_api_port_acl = [{{ range .OpenCenter.Cluster.Networking.K8sAPIPortACL }}"{{ . }}",{{ end }}]
  dns_nameservers = [{{ range .OpenCenter.Cluster.Kubernetes.Networking.DNSNameservers }}"{{ . }}",{{ end }}]
  ntp_servers = [{{ range .OpenCenter.Cluster.Kubernetes.Networking.NTPServers }}"{{ . }}",{{ end }}]
}
```

#### Kubernetes Configuration (`KubernetesConfig`)

```go
type KubernetesConfig struct {
    Version                  string
    KubesprayVersion         string
    APIPort                  int
    KubeVIPEnabled           bool
    KubeletRotateServerCerts bool
    FlavorBastion            string
    FlavorMaster             string
    FlavorWorker             string
    FlavorWorkerWindows      string
    SubnetPods               string
    SubnetServices           string
    LoadbalancerProvider     string
    MasterCount              int
    WorkerCount              int
    WorkerCountWindows       int
    Security                 KubernetesSecurityConfig
    NetworkPlugin            NetworkPlugin
    OIDC                     OIDCConfig
    WindowsWorkers           WindowsWorkers
    AdditionalServerPoolsWorker        []AdditionalWorkerPool
    AdditionalServerPoolsWorkerWindows []AdditionalWindowsWorkerPool
    MasterNodes              []NodeConfig  // Baremetal
    WorkerNodes              []NodeConfig  // Baremetal
}
```


**Terraform Mapping**:
```hcl
locals {
  kubernetes_version = "{{ .OpenCenter.Cluster.Kubernetes.Version }}"
  kubespray_version = "{{ .OpenCenter.Cluster.Kubernetes.KubesprayVersion }}"
  k8s_api_port = {{ .OpenCenter.Cluster.Kubernetes.APIPort }}
  kube_vip_enabled = {{ .OpenCenter.Cluster.Kubernetes.KubeVIPEnabled }}
  master_count = {{ .OpenCenter.Cluster.Kubernetes.MasterCount }}
  worker_count = {{ .OpenCenter.Cluster.Kubernetes.WorkerCount }}
  worker_count_windows = {{ .OpenCenter.Cluster.Kubernetes.WorkerCountWindows }}
  flavor_bastion = "{{ .OpenCenter.Cluster.Kubernetes.FlavorBastion }}"
  flavor_master = "{{ .OpenCenter.Cluster.Kubernetes.FlavorMaster }}"
  flavor_worker = "{{ .OpenCenter.Cluster.Kubernetes.FlavorWorker }}"
  subnet_pods = "{{ .OpenCenter.Cluster.Kubernetes.SubnetPods }}"
  subnet_services = "{{ .OpenCenter.Cluster.Kubernetes.SubnetServices }}"
  loadbalancer_provider = "{{ .OpenCenter.Cluster.Kubernetes.LoadbalancerProvider }}"
}
```

#### Network Plugin Configuration

```go
type NetworkPlugin struct {
    Calico  CalicoConfig
    Cilium  CiliumConfig
    KubeOVN KubeOVNConfig
}
```

**Terraform Mapping**:
```hcl
locals {
  # Determines which CNI to use
  network_plugin = "{{ if .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled }}calico{{ else if .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled }}cilium{{ else if .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled }}kube-ovn{{ else }}calico{{ end }}"
  
  # Calico-specific
  cni_iface = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.CNIIface }}"
  calico_interface_autodetect = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.CalicoInterfaceAutodetect }}"
  calico_encapsulation_type = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.EncapsulationType }}"
  calico_nat_outgoing = {{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.NATOutgoing }}
}
```


#### Storage Configuration (`StorageConfig`)

```go
type StorageConfig struct {
    DefaultStorageClass         string
    WorkerVolumeSize            int
    WorkerVolumeDestinationType string
    WorkerVolumeSourceType      string
    WorkerVolumeType            string
    AdditionalBlockDevices      []map[string]any
}
```

**Terraform Mapping**:
```hcl
locals {
  worker_node_bfv_volume_size = {{ .OpenCenter.Storage.WorkerVolumeSize }}
  worker_node_bfv_destination_type = "{{ .OpenCenter.Storage.WorkerVolumeDestinationType }}"
  worker_node_bfv_source_type = "{{ .OpenCenter.Storage.WorkerVolumeSourceType }}"
  worker_node_bfv_volume_type = "{{ .OpenCenter.Storage.WorkerVolumeType }}"
  additional_block_devices_worker = [{{ range .OpenCenter.Storage.AdditionalBlockDevices }}{{ . }},{{ end }}]
}
```

#### Additional Worker Pools

```go
type AdditionalWorkerPool struct {
    Name                          string
    WorkerCount                   int
    FlavorWorker                  string
    NodeWorker                    string
    ServerGroupAffinity           string
    ImageID                       string
    ImageName                     string
    WorkerNodeBFVVolumeSize       int
    WorkerNodeBFVDestinationType  string
    WorkerNodeBFVSourceType       string
    WorkerNodeBFVVolumeType       string
    WorkerNodeBFVDeleteOnTermination bool
    PF9Onboard                    bool
    SubnetID                      string
    AdditionalBlockDevicesWorker  []map[string]any
}
```

**Terraform Mapping**:
```hcl
locals {
  additional_server_pools_worker = [
    {{- range .OpenCenter.Cluster.Kubernetes.AdditionalServerPoolsWorker }}
    {
      name = "{{ .Name }}"
      worker_count = {{ .WorkerCount }}
      flavor_worker = "{{ .FlavorWorker }}"
      node_worker = "{{ .NodeWorker }}"
      server_group_affinity = "{{ .ServerGroupAffinity }}"
      image_id = "{{ .ImageID }}"
      worker_node_bfv_volume_size = {{ .WorkerNodeBFVVolumeSize }}
      # ... additional fields
    },
    {{- end }}
  ]
}
```


## Complete Configuration to Terraform Mapping

This section provides a comprehensive mapping table showing how each configuration field maps to Terraform template variables.

### Infrastructure Mapping

| YAML Path | Terraform Local | Template File | Default Value |
|-----------|----------------|---------------|---------------|
| `opencenter.infrastructure.provider` | N/A (conditional logic) | `main.tf.tpl` | `openstack` |
| `opencenter.infrastructure.ssh_user` | `local.ssh_user` | `main.tf.tpl` | `ubuntu` |
| `opencenter.infrastructure.os_version` | `local.ub_version` | `main.tf.tpl` | `24` |
| `opencenter.infrastructure.server_group_affinity` | `local.wn_server_group_affinity` | `main.tf.tpl` | `["anti-affinity"]` |
| `opencenter.infrastructure.node_naming.worker` | `local.node_worker` | `main.tf.tpl` | `wn` |
| `opencenter.infrastructure.node_naming.master` | `local.node_master` | `main.tf.tpl` | `cp` |
| `opencenter.infrastructure.node_naming.worker_windows` | `local.node_worker_windows` | `main.tf.tpl` | `win` |
| `opencenter.infrastructure.bastion.address` | `local.address_bastion` | `main.tf.tpl` | `localhost` |
| `opencenter.infrastructure.k8s_api_ip` | `local.k8s_api_ip` | `main.tf.tpl` | (computed) |

### OpenStack Cloud Mapping

| YAML Path | Terraform Local | Template File | Default Value |
|-----------|----------------|---------------|---------------|
| `opencenter.infrastructure.cloud.openstack.auth_url` | `local.openstack_auth_url` | `main.tf.tpl` | Region-specific |
| `opencenter.infrastructure.cloud.openstack.insecure` | `local.openstack_insecure` | `main.tf.tpl` | `false` |
| `opencenter.infrastructure.cloud.openstack.region` | `local.openstack_region` | `main.tf.tpl` | `sjc3` |
| `opencenter.infrastructure.cloud.openstack.application_credential_id` | `local.application_credential_id` | `main.tf.tpl` | (from var) |
| `opencenter.infrastructure.cloud.openstack.application_credential_secret` | `local.application_credential_secret` | `main.tf.tpl` | (from var) |
| `opencenter.infrastructure.cloud.openstack.domain` | N/A | `main.tf.tpl` | `Default` |
| `opencenter.infrastructure.cloud.openstack.tenant_name` | `local.openstack_tenant_name` | `main.tf.tpl` | (empty) |
| `opencenter.infrastructure.cloud.openstack.availability_zone` | `local.availability_zone` | `main.tf.tpl` | `az1` |
| `opencenter.infrastructure.cloud.openstack.project_domain_name` | `local.openstack_project_domain_name` | `main.tf.tpl` | `rackspace_cloud_domain` |
| `opencenter.infrastructure.cloud.openstack.user_domain_name` | `local.openstack_user_domain_name` | `main.tf.tpl` | `rackspace_cloud_domain` |
| `opencenter.infrastructure.cloud.openstack.image_id` | `local.image_id` | `main.tf.tpl` | Ubuntu 24.04 UUID |
| `opencenter.infrastructure.cloud.openstack.image_id_windows` | `local.image_id_windows` | `main.tf.tpl` | Windows Server UUID |
| `opencenter.infrastructure.cloud.openstack.networking.floating_ip_pool` | `local.floatingip_pool` | `main.tf.tpl` | `PUBLICNET` |
| `opencenter.infrastructure.cloud.openstack.networking.router_external_network_id` | `local.router_external_network_id` | `main.tf.tpl` | (empty) |
| `opencenter.infrastructure.cloud.openstack.networking.network_id` | Passed to module | `main.tf.tpl` | (empty) |


### Cluster and Kubernetes Mapping

| YAML Path | Terraform Local | Template File | Default Value |
|-----------|----------------|---------------|---------------|
| `opencenter.cluster.cluster_name` | `local.cluster_name` | `main.tf.tpl` | (required) |
| `opencenter.cluster.ssh_authorized_keys` | `local.ssh_authorized_keys` | `main.tf.tpl` | (required) |
| `opencenter.cluster.networking.k8s_api_port_acl` | `local.k8s_api_port_acl` | `main.tf.tpl` | `["0.0.0.0/0"]` |
| `opencenter.cluster.networking.dns_nameservers` | `local.dns_nameservers` | `main.tf.tpl` | `["8.8.8.8", "8.8.4.4"]` |
| `opencenter.cluster.networking.ntp_servers` | `local.ntp_servers` | `main.tf.tpl` | Region-specific |
| `opencenter.cluster.networking.security.ca_certificates` | `local.ca_certificates` | `main.tf.tpl` | (empty) |
| `opencenter.cluster.networking.security.os_hardening` | `local.os_hardening_enabled` | `main.tf.tpl` | `true` |
| `opencenter.cluster.kubernetes.version` | `local.kubernetes_version` | `main.tf.tpl` | `1.33.5` |
| `opencenter.cluster.kubernetes.kubespray_version` | `local.kubespray_version` | `main.tf.tpl` | `v2.29.1` |
| `opencenter.cluster.kubernetes.api_port` | `local.k8s_api_port` | `main.tf.tpl` | `443` |
| `opencenter.cluster.kubernetes.kube_vip_enabled` | `local.kube_vip_enabled` | `main.tf.tpl` | `true` |
| `opencenter.cluster.kubernetes.kubelet_rotate_server_certs` | `local.kubelet_rotate_server_certificates` | `main.tf.tpl` | `false` |
| `opencenter.cluster.kubernetes.master_count` | `local.master_count` | `main.tf.tpl` | `3` |
| `opencenter.cluster.kubernetes.worker_count` | `local.worker_count` | `main.tf.tpl` | `2` |
| `opencenter.cluster.kubernetes.worker_count_windows` | `local.worker_count_windows` | `main.tf.tpl` | `0` |
| `opencenter.cluster.kubernetes.flavor_bastion` | `local.flavor_bastion` | `main.tf.tpl` | `gp.0.2.2` |
| `opencenter.cluster.kubernetes.flavor_master` | `local.flavor_master` | `main.tf.tpl` | `gp.0.4.8` |
| `opencenter.cluster.kubernetes.flavor_worker` | `local.flavor_worker` | `main.tf.tpl` | `gp.0.4.16` |
| `opencenter.cluster.kubernetes.flavor_worker_windows` | `local.flavor_worker_windows` | `main.tf.tpl` | `gp.5.4.16` |
| `opencenter.cluster.kubernetes.subnet_pods` | `local.subnet_pods` | `main.tf.tpl` | `10.42.0.0/16` |
| `opencenter.cluster.kubernetes.subnet_services` | `local.subnet_services` | `main.tf.tpl` | `10.43.0.0/16` |
| `opencenter.cluster.kubernetes.loadbalancer_provider` | `local.loadbalancer_provider` | `main.tf.tpl` | `ovn` |
| `opencenter.cluster.kubernetes.security.k8s_hardening` | `local.k8s_hardening_enabled` | `main.tf.tpl` | `true` |
| `opencenter.cluster.kubernetes.security.pod_security_exemptions` | `local.kube_pod_security_exemptions_namespaces` | `main.tf.tpl` | `["trivy-temp"]` |

### Network Plugin Mapping

| YAML Path | Terraform Local | Template File | Default Value |
|-----------|----------------|---------------|---------------|
| `opencenter.cluster.kubernetes.network_plugin.calico.enabled` | `local.network_plugin` (conditional) | `main.tf.tpl` | `true` |
| `opencenter.cluster.kubernetes.network_plugin.calico.cni_iface` | `local.cni_iface` | `main.tf.tpl` | `enp3s0` |
| `opencenter.cluster.kubernetes.network_plugin.calico.calico_interface_autodetect` | `local.calico_interface_autodetect` | `main.tf.tpl` | `interface` |
| `opencenter.cluster.kubernetes.network_plugin.calico.autodetect_cidr` | `local.calico_interface_autodetect_cidr` | `main.tf.tpl` | (empty) |
| `opencenter.cluster.kubernetes.network_plugin.calico.encapsulation_type` | `local.calico_encapsulation_type` | `main.tf.tpl` | `VXLAN` |
| `opencenter.cluster.kubernetes.network_plugin.calico.nat_outgoing` | `local.calico_nat_outgoing` | `main.tf.tpl` | `true` |
| `opencenter.cluster.kubernetes.network_plugin.cilium.enabled` | `local.network_plugin` (conditional) | `main.tf.tpl` | `false` |
| `opencenter.cluster.kubernetes.network_plugin.cilium.operator_enabled` | `module.cilium.cilium_operator_enabled` | `main.tf.tpl` | `true` |
| `opencenter.cluster.kubernetes.network_plugin.cilium.kube_proxy_replacement` | `module.cilium.cilium_kube_proxy_replacement` | `main.tf.tpl` | `true` |
| `opencenter.cluster.kubernetes.network_plugin.kube-ovn.enabled` | `local.network_plugin` (conditional) | `main.tf.tpl` | `false` |
| `opencenter.cluster.kubernetes.network_plugin.kube-ovn.cilium_integration` | `module.kube-ovn.kube_ovn_cilium_integration` | `main.tf.tpl` | `true` |


### OIDC Configuration Mapping

| YAML Path | Terraform Local | Template File | Default Value |
|-----------|----------------|---------------|---------------|
| `opencenter.cluster.kubernetes.oidc.enabled` | `local.kube_oidc_auth_enabled` | `main.tf.tpl` | `false` |
| `opencenter.cluster.kubernetes.oidc.kube_oidc_url` | `local.kube_oidc_url` | `main.tf.tpl` | (empty) |
| `opencenter.cluster.kubernetes.oidc.kube_oidc_client_id` | `local.kube_oidc_client_id` | `main.tf.tpl` | `kubernetes` |
| `opencenter.cluster.kubernetes.oidc.kube_oidc_ca_file` | `local.kube_oidc_ca_file` | `main.tf.tpl` | (empty) |
| `opencenter.cluster.kubernetes.oidc.kube_oidc_username_claim` | `local.kube_oidc_username_claim` | `main.tf.tpl` | `sub` |
| `opencenter.cluster.kubernetes.oidc.kube_oidc_username_prefix` | `local.kube_oidc_username_prefix` | `main.tf.tpl` | `oidc:` |
| `opencenter.cluster.kubernetes.oidc.kube_oidc_groups_claim` | `local.kube_oidc_groups_claim` | `main.tf.tpl` | `groups` |
| `opencenter.cluster.kubernetes.oidc.kube_oidc_groups_prefix` | `local.kube_oidc_groups_prefix` | `main.tf.tpl` | `oidc:` |

### Windows Workers Mapping

| YAML Path | Terraform Local | Template File | Default Value |
|-----------|----------------|---------------|---------------|
| `opencenter.cluster.kubernetes.windows_workers.enabled` | (conditional logic) | `main.tf.tpl` | `false` |
| `opencenter.cluster.kubernetes.windows_workers.windows_user` | `local.windows_user` | `main.tf.tpl` | `Administrator` |
| `opencenter.cluster.kubernetes.windows_workers.windows_admin_password` | `local.windows_admin_password` | `main.tf.tpl` | (empty) |
| `opencenter.cluster.kubernetes.windows_workers.worker_node_bfv_size_windows` | `local.worker_node_bfv_size_windows` | `main.tf.tpl` | `100` |
| `opencenter.cluster.kubernetes.windows_workers.worker_node_bfv_type_windows` | `local.worker_node_bfv_type_windows` | `main.tf.tpl` | `volume` |

### Storage Mapping

| YAML Path | Terraform Local | Template File | Default Value |
|-----------|----------------|---------------|---------------|
| `opencenter.storage.default_storage_class` | N/A (used in services) | N/A | `csi-cinder-sc-delete` |
| `opencenter.storage.worker_volume_size` | `local.worker_node_bfv_volume_size` | `main.tf.tpl` | `40` |
| `opencenter.storage.worker_volume_destination_type` | `local.worker_node_bfv_destination_type` | `main.tf.tpl` | `volume` |
| `opencenter.storage.worker_volume_source_type` | `local.worker_node_bfv_source_type` | `main.tf.tpl` | `image` |
| `opencenter.storage.worker_volume_type` | `local.worker_node_bfv_volume_type` | `main.tf.tpl` | `HA-Standard` |
| `opencenter.storage.additional_block_devices` | `local.additional_block_devices_worker` | `main.tf.tpl` | `[]` |

### OpenTofu Backend Mapping

| YAML Path | Terraform Backend | Template File | Default Value |
|-----------|------------------|---------------|---------------|
| `opentofu.enabled` | N/A | `opentofu_main.tf.tmpl` | `true` |
| `opentofu.backend.type` | `backend "<type>"` | `opentofu_main.tf.tmpl` | `local` |
| `opentofu.backend.local.path` | `path` | `opentofu_main.tf.tmpl` | `terraform.tfstate` |
| `opentofu.backend.s3.bucket` | `bucket` | `opentofu_main.tf.tmpl` | (empty) |
| `opentofu.backend.s3.key` | `key` | `opentofu_main.tf.tmpl` | (empty) |
| `opentofu.backend.s3.region` | `region` | `opentofu_main.tf.tmpl` | (empty) |
| `opentofu.backend.s3.endpoint` | `endpoint` | `opentofu_main.tf.tmpl` | (empty) |
| `opentofu.backend.s3.encrypt` | `encrypt` | `opentofu_main.tf.tmpl` | `true` |
| `opencenter.cluster.aws_access_key` | `access_key` | `opentofu_main.tf.tmpl` | (from secrets) |
| `opencenter.cluster.aws_secret_access_key` | `secret_key` | `opentofu_main.tf.tmpl` | (from secrets) |


## Template Rendering Flow

### 1. Configuration Loading

```
User YAML Config
    ↓
config.Load(name)
    ↓
YAML Unmarshal → Config struct
    ↓
Validation (schema + business rules)
    ↓
In-memory Config object
```

### 2. Template Rendering

```
Config object
    ↓
template.Execute(templateFile, config)
    ↓
Go text/template engine
    ↓
Variable substitution ({{ .Path.To.Field }})
    ↓
Conditional logic ({{- if }})
    ↓
Range loops ({{- range }})
    ↓
Rendered Terraform file
```

### 3. Template Files

**Main Infrastructure Template**:
- **File**: `internal/gitops/templates/infrastructure-cluster-template/main.tf.tpl`
- **Purpose**: Generates Terraform configuration for cluster infrastructure
- **Modules Used**:
  - `openstack-nova`: OpenStack compute resources
  - `kubespray-cluster`: Kubernetes cluster deployment
  - `calico`/`cilium`/`kube-ovn`: CNI plugins

**OpenTofu Backend Template**:
- **File**: `internal/provision/templates/opentofu_main.tf.tmpl`
- **Purpose**: Generates Terraform backend configuration
- **Backends Supported**: local, s3, azurerm, gcs

### 4. Template Syntax Examples

**Simple Variable Substitution**:
```hcl
cluster_name = "{{ .OpenCenter.Cluster.ClusterName }}"
```

**With Default Value**:
```hcl
ssh_user = "{{ .OpenCenter.Infrastructure.SSHUser | default "ubuntu" }}"
```

**Conditional Rendering**:
```hcl
{{- if .OpenCenter.Cluster.Kubernetes.OIDC.Enabled }}
kube_oidc_auth_enabled = true
kube_oidc_url = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCURL }}"
{{- end }}
```

**Array Rendering**:
```hcl
ssh_authorized_keys = [
  {{- range $i, $key := .OpenCenter.Cluster.SSHAuthorizedKeys }}
  {{- if $i }}, {{ end }}"{{ $key }}"
  {{- end }}
]
```

**Complex Object Rendering**:
```hcl
additional_server_pools_worker = [
  {{- range .OpenCenter.Cluster.Kubernetes.AdditionalServerPoolsWorker }}
  {
    name = "{{ .Name }}"
    worker_count = {{ .WorkerCount }}
    flavor_worker = "{{ .FlavorWorker }}"
  },
  {{- end }}
]
```


## Adding New Configuration Fields

Follow this checklist when adding new configuration fields:

### 1. Define the Go Struct

**File**: `internal/config/types_*.go` (choose appropriate file)

```go
type YourConfig struct {
    NewField string `yaml:"new_field" json:"new_field" jsonschema:"description=Your field description"`
}
```

**YAML Tag Rules**:
- Use snake_case for YAML field names
- Omit `omitempty` if you want the field to always render
- Add `omitempty` for optional fields

**JSON Schema Tags**:
- Add `jsonschema:"description=..."` for documentation
- Add `jsonschema:"default=..."` for default values
- Add `jsonschema:"enum=..."` for restricted values
- Add `jsonschema:"pattern=..."` for validation

### 2. Add to Default Configuration

**File**: `internal/config/config.go`

```go
func defaultConfig(name string) Config {
    cfg := Config{
        // ... existing config
        YourSection: YourConfig{
            NewField: "default-value",
        },
    }
    return cfg
}
```

### 3. Update Template

**File**: `internal/gitops/templates/infrastructure-cluster-template/main.tf.tpl`

```hcl
locals {
  your_new_field = "{{ .OpenCenter.YourSection.NewField | default "fallback-value" }}"
}

module "your-module" {
  source = "..."
  your_field = local.your_new_field
}
```

### 4. Update Schema

**Automatic**: Schema is generated from Go struct tags

```bash
mise run build
./bin/openCenter cluster schema --out schema/cluster.schema.json --pretty
```

### 5. Test the Field

```bash
# Generate template with new field
mise run build
./bin/openCenter cluster template --out test-config.yaml

# Verify field is present
grep "new_field" test-config.yaml

# Test with cluster init
./bin/openCenter cluster init test-cluster --no-keygen
grep "new_field" ~/.config/openCenter/clusters/opencenter/.test-cluster-config.yaml

# Validate configuration
./bin/openCenter cluster validate test-cluster
```

### 6. Update Documentation

- Add field to this document's mapping tables
- Update user-facing documentation in `docs/reference/configuration.md`
- Add examples to `docs/tutorials/` if significant feature


## Service Configuration Architecture

Services use a polymorphic configuration system with a registry pattern. See [Service Registry Patterns](./service-registry-patterns.md) for detailed documentation.

### Service Configuration Structure

```go
// Base configuration (all services)
type BaseConfig struct {
    Enabled   bool
    Status    string
    Namespace string
    Hostname  string
    
    // Image configuration
    ImageRepository string
    ImageTag        string
    
    // Version control fields (for GitOps managed services)
    Release string
    Branch  string
    Uri     string
    
    // GitOps source fields (for managed services)
    GitOpsSourceRepo    string
    GitOpsSourceRelease string
    GitOpsSourceBranch  string
}

// Service-specific configuration
type LokiConfig struct {
    BaseConfig      `yaml:",inline"`
    
    // Storage configuration
    StorageType     string  // "s3" or "swift"
    BucketName      string
    VolumeSize      int
    StorageClass    string
    
    // Swift storage fields
    SwiftAuthURL                 string
    SwiftRegion                  string
    SwiftAuthVersion             int
    SwiftApplicationCredentialID string
    SwiftContainerName           string
    SwiftUserDomainName          string
    SwiftDomainName              string
    
    // S3 storage fields
    S3Endpoint       string
    S3Region         string
    S3ForcePathStyle bool
    S3Insecure       bool
}
```

### Service Registration

**Critical**: Every service MUST register itself in its `init()` function:

```go
// internal/config/services/loki.go
package services

import "github.com/rackerlabs/openCenter-cli/internal/config/registry"

// LokiConfig extends BaseConfig with Loki-specific configuration
type LokiConfig struct {
    BaseConfig `yaml:",inline"`
    
    StorageType  string `yaml:"loki_storage_type" json:"loki_storage_type,omitempty"`
    BucketName   string `yaml:"loki_bucket_name" json:"loki_bucket_name,omitempty"`
    // ... other fields
}

func init() {
    registry.RegisterServiceConfig("loki", LokiConfig{})
}
```

### Service Map

Services are stored in a `ServiceMap` which is a `map[string]any`:

```go
type ServiceMap map[string]any

// Custom unmarshaler looks up registered types
func (sm *ServiceMap) UnmarshalYAML(node *yaml.Node) error {
    // Initialize map if nil, but don't replace existing services
    if *sm == nil {
        *sm = make(ServiceMap)
    }
    
    // Iterate over keys and values from YAML
    for i := 0; i < len(node.Content); i += 2 {
        serviceName := node.Content[i]
        
        // Look up registered type
        configType := registry.GetServiceConfigType(serviceName)
        if configType == nil {
            // Falls back to BaseConfig if not registered
            configType = reflect.TypeOf(services.BaseConfig{})
        }
        
        // Create instance and unmarshal
        configPtr := reflect.New(configType).Interface()
        // ... unmarshal logic
    }
}
```

### Adding a New Service

1. **Create service config struct** in `internal/config/services/`:

```go
// internal/config/services/myservice.go
package services

import "github.com/rackerlabs/openCenter-cli/internal/config/registry"

type MyServiceConfig struct {
    BaseConfig `yaml:",inline"`
    
    // Service-specific fields
    MyField string `yaml:"my_field" json:"my_field"`
}

func init() {
    registry.RegisterServiceConfig("myservice", MyServiceConfig{})
}
```

2. **Add to default config** in `internal/config/config.go`:

```go
// In defaultConfig() function
Services: ServiceMap{
    "myservice": &services.MyServiceConfig{
        BaseConfig: services.BaseConfig{
            Enabled:             false,
            GitOpsSourceRepo:    "ssh://git@github.com/rackerlabs/openCenter-gitops-base.git",
            GitOpsSourceRelease: "v0.1.0",
            GitOpsSourceBranch:  "main",
        },
        MyField: "default-value",
    },
}
```

3. **Test registration**:

```bash
mise run build
./bin/openCenter cluster init test-service --no-keygen
grep -A 10 "myservice:" ~/.config/openCenter/clusters/opencenter/.test-service-config.yaml
```

Verify all fields render (not just BaseConfig fields).


## Debugging Configuration Issues

### Common Issues and Solutions

#### Issue: Field Not Rendering in YAML

**Symptoms**:
- Field defined in struct but missing from generated YAML
- `cluster init` doesn't show the field

**Causes**:
1. `omitempty` tag on YAML field
2. Zero value for the field type
3. Field not added to `defaultConfig()`

**Solution**:
```go
// Remove omitempty from yaml tag
MyField string `yaml:"my_field" json:"my_field,omitempty"`

// Add default value in defaultConfig()
MyField: "default-value",
```

#### Issue: Service Shows Only BaseConfig Fields

**Symptoms**:
- Service configuration only has ~13 BaseConfig fields (enabled, status, namespace, hostname, image_repository, image_tag, release, branch, uri, gitops_source_*)
- Service-specific fields are missing
- No error messages during init or marshaling

**Cause**: Service not registered in the service registry

**Solution**:
```go
// Add init() function to service file (e.g., internal/config/services/myservice.go)
package services

import "github.com/rackerlabs/openCenter-cli/internal/config/registry"

type MyServiceConfig struct {
    BaseConfig `yaml:",inline"`
    MyField    string `yaml:"my_field" json:"my_field"`
}

func init() {
    registry.RegisterServiceConfig("myservice", MyServiceConfig{})
}
```

**Verification**:
```bash
# After adding registration, rebuild and test
mise run build
./bin/openCenter cluster init test-service --no-keygen

# Count fields - should match struct definition
grep -A 30 "myservice:" ~/.config/openCenter/clusters/opencenter/.test-service-config.yaml | wc -l

# Should see all fields, not just BaseConfig fields
grep -A 30 "myservice:" ~/.config/openCenter/clusters/opencenter/.test-service-config.yaml
```

See [Service Registry Patterns](./service-registry-patterns.md) for detailed debugging registered in registry

**Solution**:
```go
// Add init() function to service file
func init() {
    registry.RegisterServiceConfig("myservice", MyServiceConfig{})
}
```

See [Service Registry Patterns](../service-registry-patterns.md) for detailed debugging.

#### Issue: Template Rendering Error

**Symptoms**:
- Error during `cluster setup` or `cluster render`
- Template execution fails

**Causes**:
1. Accessing nil pointer in template
2. Incorrect template syntax
3. Missing field in config struct

**Solution**:
```hcl
# Add nil checks in template
{{- if .OpenCenter.YourSection }}
{{- if .OpenCenter.YourSection.YourField }}
your_field = "{{ .OpenCenter.YourSection.YourField }}"
{{- end }}
{{- end }}

# Or use default filter
your_field = "{{ .OpenCenter.YourSection.YourField | default "fallback" }}"
```

#### Issue: Validation Fails

**Symptoms**:
- `cluster validate` reports errors
- Schema validation fails

**Causes**:
1. Missing required field
2. Invalid value format
3. Schema out of sync with struct

**Solution**:
```bash
# Regenerate schema
mise run build
./bin/openCenter cluster schema --out schema/cluster.schema.json --pretty

# Check validation rules
./bin/openCenter cluster validate test-cluster --verbose
```


### Debugging Tools

#### 1. Generate Complete Template

```bash
# See all available fields
./bin/openCenter cluster template --out complete.yaml

# Check specific provider
./bin/openCenter cluster template --provider openstack --out openstack.yaml
```

#### 2. Inspect Configuration

```bash
# View loaded configuration
./bin/openCenter cluster info test-cluster

# View as JSON
./bin/openCenter cluster info test-cluster --output json | jq .
```

#### 3. Validate Configuration

```bash
# Run validation
./bin/openCenter cluster validate test-cluster

# Verbose output
./bin/openCenter cluster validate test-cluster --verbose

# Check specific sections
./bin/openCenter cluster validate test-cluster --check infrastructure
```

#### 4. Render Templates

```bash
# Render without applying
./bin/openCenter cluster render test-cluster --dry-run

# See generated Terraform
cat ~/.config/openCenter/clusters/opencenter/test-cluster/infrastructure/clusters/test-cluster/main.tf
```

#### 5. Check Schema

```bash
# Generate current schema
./bin/openCenter cluster schema --pretty

# Compare with committed schema
diff <(./bin/openCenter cluster schema) schema/cluster.schema.json
```

## Configuration File Locations

### User Configuration

```
~/.config/openCenter/
├── clusters/
│   └── <organization>/
│       ├── .<cluster>-config.yaml          # Main config
│       ├── secrets/
│       │   ├── age/
│       │   │   └── <cluster>-key.txt       # SOPS Age key
│       │   └── ssh/
│       │       └── <cluster>                # SSH keys
│       └── gitops/                          # GitOps repo
│           ├── applications/
│           └── infrastructure/
│               └── clusters/<cluster>/
│                   └── main.tf              # Generated Terraform
└── config.yaml                              # CLI config
```

### Embedded Templates

```
internal/
├── gitops/
│   └── templates/
│       ├── infrastructure-cluster-template/
│       │   └── main.tf.tpl                  # Main Terraform template
│       └── cluster-apps-base/               # Application templates
└── provision/
    └── templates/
        └── opentofu_main.tf.tmpl            # Backend template
```


## Configuration Precedence and Merging

### Configuration Loading Order

1. **Embedded defaults** - `defaultConfig()` in `internal/config/config.go`
2. **CLI defaults** - `~/.config/openCenter/config.yaml` (user-level defaults)
3. **Cluster config file** - `~/.config/openCenter/clusters/<org>/.<cluster>-config.yaml`
4. **Environment variables** - `OPENCENTER_*` variables
5. **Command-line flags** - Highest precedence

### Service Configuration Merging

Services use a merge strategy where:
- Services defined in YAML override defaults
- Services not in YAML are preserved from defaults
- This allows partial configuration updates

```go
// UnmarshalYAML merges services from YAML with existing services
func (sm *ServiceMap) UnmarshalYAML(node *yaml.Node) error {
    // Initialize map if nil, but don't replace existing services
    if *sm == nil {
        *sm = make(ServiceMap)
    }
    
    // Iterate and merge - YAML overrides defaults
    // Services not in YAML are preserved
}
```

## Related Documentation

- [Service Registry Patterns](./service-registry-patterns.md) - Service configuration architecture and debugging
- [Developer Commands](./developer-commands.md) - Hidden commands for development
- [Architecture](./architecture.md) - Overall system architecture
- [Configuration Reference](../reference/configuration.md) - User-facing configuration docs
- [CLI Commands](../reference/cli-commands.md) - Command reference
- [Templates Reference](../reference/templates.md) - Template engine documentation

## Key Files Reference

### Configuration Types

| File | Purpose | Key Types |
|------|---------|-----------|
| `internal/config/config.go` | Root configuration | `Config`, `defaultConfig()` |
| `internal/config/types_opencenter.go` | OpenCenter section | `SimplifiedOpenCenter`, `ClusterMeta`, `TalosConfig` |
| `internal/config/types_cluster.go` | Cluster config | `ClusterConfig`, `ClusterNetworkingConfig` |
| `internal/config/types_infrastructure.go` | Infrastructure | `Infrastructure`, `CloudConfig`, `SimplifiedOpenStackCloud` |
| `internal/config/types_kubernetes.go` | Kubernetes | `KubernetesConfig`, `NetworkPlugin`, `AdditionalServerPool` |
| `internal/config/types_storage.go` | Storage | `StorageConfig` |
| `internal/config/types_gitops.go` | GitOps | `GitOpsConfig`, `GitOpsFlux` |
| `internal/config/types_opentofu.go` | OpenTofu | `SimplifiedOpenTofu`, `SimplifiedTofuBackend` |
| `internal/config/types_secrets.go` | Secrets | `Secrets`, `GlobalSecrets`, service-specific secrets |
| `internal/config/types_deployment.go` | Deployment | `Deployment` |
| `internal/config/types_networking.go` | Networking | `Networking` |
| `internal/config/types_security.go` | Security | `ClusterSecurityConfig`, `KubernetesSecurityConfig` |

### Service Configuration

| File | Purpose |
|------|---------|
| `internal/config/service_map.go` | ServiceMap implementation |
| `internal/config/registry/registry.go` | Service registry |
| `internal/config/services/*.go` | Individual service configs |

### Templates

| File | Purpose |
|------|---------|
| `internal/gitops/templates/infrastructure-cluster-template/main.tf.tpl` | Main Terraform template |
| `internal/provision/templates/opentofu_main.tf.tmpl` | OpenTofu backend template |
| `internal/gitops/generator.go` | Template rendering logic |

### Commands

| File | Purpose |
|------|---------|
| `cmd/cluster_init.go` | Initialize cluster config |
| `cmd/cluster_template.go` | Generate complete template |
| `cmd/cluster_validate.go` | Validate configuration |
| `cmd/cluster_render.go` | Render templates |
| `cmd/cluster_schema.go` | Generate JSON schema |

## Testing Configuration Changes

### Unit Tests

```bash
# Test configuration loading
mise run test -- -run TestLoad

# Test validation
mise run test -- -run TestValidate

# Test schema generation
mise run test -- -run TestSchema
```

### Integration Tests

```bash
# Test full workflow
mise run godog

# Test specific feature
mise run godog -- --tags @cluster-init
```

### Manual Testing

```bash
# Build and test
mise run build

# Initialize test cluster
./bin/openCenter cluster init test-cluster --no-keygen

# Validate
./bin/openCenter cluster validate test-cluster

# Render templates
./bin/openCenter cluster render test-cluster --dry-run

# Check generated files
ls -la ~/.config/openCenter/clusters/opencenter/test-cluster/
```

## See Also

- [Contributing Guide](./contributing.md) - How to contribute
- [Testing Guide](./testing/README.md) - Testing strategies
- [Coding Standards](./coding-standards.md) - Code style guide
- [Release Process](./release-process.md) - How releases work

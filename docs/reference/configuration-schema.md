---
id: configuration-schema
title: "Configuration Schema Reference"
sidebar_label: Configuration Schema Reference
description: Complete reference of cluster configuration file structure, fields, and validation rules.
doc_type: reference
audience: "all users"
tags: [schema, configuration, yaml, fields]
---
# Configuration Schema Reference

**Purpose:** Complete reference of cluster configuration file structure, fields, and validation rules for quick lookup.

This reference documents the structure of the cluster configuration YAML file with all available fields and their constraints.

## Schema Version

Current schema version: `2.0`

```yaml
schema_version: "2.0"
```

**📌 NOTE**\
Only `schema_version: "2.0"` is supported. Any other schema version is invalid.

## Top-Level Structure

```yaml
schema_version: "2.0"
opencenter:      # Main configuration section
opentofu:        # Infrastructure provisioning
deployment:      # Deployment automation
metadata:        # Configuration lifecycle tracking
secrets:         # Encrypted secrets
```

## opencenter Section

Main configuration for cluster and services.

### opencenter.meta

Cluster metadata and identification.

```yaml
opencenter:
  meta:
    name: "my-cluster"           # Cluster name (required)
    env: "production"            # Environment (dev, staging, production)
    region: "sjc3"               # Cloud region
    status: ""                   # Cluster status
    organization: "my-org"       # Organization name
```

**Validation:**

* `name`: 3-63 characters, lowercase alphanumeric and hyphens, must start/end with alphanumeric
* `organization`: Same rules as name
* `region`: Provider-specific region code

### opencenter.secrets

Secrets backend configuration.

```yaml
opencenter:
  secrets:
    backend: "barbican"          # Secrets backend (barbican)
    barbican:
      auth_url: ""               # Barbican auth URL
      project_id: ""             # OpenStack project ID
      region: ""                 # Barbican region
      user_domain_name: ""       # User domain
      project_domain_name: ""    # Project domain
      ca_cert: ""                # CA certificate
```

### opencenter.infrastructure

Infrastructure provider configuration.

```yaml
opencenter:
  infrastructure:
    provider: "openstack"        # Canonical GA providers: openstack, vmware, kind, baremetal
    ssh_user: "ubuntu"           # SSH user for nodes
    os_version: "24"             # OS version (Ubuntu)
    server_group_affinity:       # Server group affinity
      - "anti-affinity"
    node_naming:
      worker: "wn"               # Worker node prefix
      master: "cp"               # Control plane prefix
      worker_windows: "win"      # Windows worker prefix
    bastion:
      address: "localhost"       # Bastion host address
    k8s_api_ip: ""               # Kubernetes API IP
    cloud:                       # Provider-specific config
      openstack: {}              # OpenStack configuration
      vmware: {}                 # VMware configuration
```

### opencenter.infrastructure.cloud.openstack

OpenStack provider configuration.

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        auth_url: "https://identity.api.rackspacecloud.com/v3"
        insecure: false
        region: "sjc3"
        application_credential_id: ""      # Required
        application_credential_secret: ""  # Required
        domain: "Default"
        tenant_name: ""
        availability_zone: "az1"
        project_domain_name: "rackspace_cloud_domain"
        user_domain_name: "rackspace_cloud_domain"
        ca: ""
        image_id: "799dcf97-3656-4361-8187-13ab1b295e33"
        image_id_windows: "a2083759-f341-445b-b717-dafb5e31fa6b"
        networking:
          floating_ip_pool: "PUBLICNET"
          floating_network_id: ""          # Required
          network_id: ""
          router_external_network_id: "723f8fa2-dbf7-4cec-8d5f-017e62c12f79"
          subnet_id: ""
          k8s_api_port_acl:
            - "0.0.0.0/0"
          designate:
            dns_zone_name: ""
          vlan:
            id: ""
            mtu: 0
            provider: "physnet1"
        modules:
          openstack_nova:
            source: "github.com/opencenter-cloud/opencenter-gitops-base.git//iac/cloud/openstack/openstack-nova?ref=main"
```

**Required Fields:**

* `application_credential_id`
* `application_credential_secret`
* `floating_network_id`

### opencenter.infrastructure.cloud.aws

AWS provider configuration.

```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        profile: ""              # AWS profile name
        region: ""               # AWS region (e.g., us-east-1)
        vpc_id: ""               # VPC ID
        private_subnets: []      # Private subnet IDs
        public_subnets: []       # Public subnet IDs
```

### opencenter.cluster

Kubernetes cluster configuration.

```yaml
opencenter:
  cluster:
    cluster_name: "my-cluster"
    aws_access_key: ""
    aws_secret_access_key: ""
    ssh_authorized_keys:
      - "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExamplePublicKeyDataHere user@example.com"
    base_domain: "k8s.opencenter.cloud"
    cluster_fqdn: "my-cluster.sjc3.k8s.opencenter.cloud"
    admin_email: "admin@example.com"
    k8s_api_port_acl:
      - "0.0.0.0/0"
    networking: {}               # Network configuration
    kubernetes: {}               # Kubernetes settings
```

### opencenter.cluster.networking

Network configuration.

```yaml
opencenter:
  cluster:
    networking:
      ntp_servers:
        - "time.sjc3.rackspace.com"
        - "time2.sjc3.rackspace.com"
      dns_nameservers:
        - "8.8.8.8"
        - "8.8.4.4"
      security:
        ca_certificates: ""
        os_hardening: true
      subnet_nodes: "10.2.128.0/22"
      allocation_pool_start: ""
      allocation_pool_end: ""
      vrrp_ip: "10.2.128.5"      # Required when use_octavia=false and vrrp_enabled=true
      vrrp_enabled: true
      use_octavia: false
      loadbalancer_provider: "ovn"
      use_designate: false
      dns_zone_name: ""
      vlan:
        id: ""
        mtu: 0
        provider: "physnet1"
```

**Validation:**

* `vrrp_ip` required when `use_octavia=false` and `vrrp_enabled=true`
* `subnet_nodes` must be valid CIDR notation
* `dns_nameservers` must be valid IP addresses

### opencenter.cluster.kubernetes

Kubernetes cluster settings.

```yaml
opencenter:
  cluster:
    kubernetes:
      version: "1.33.5"          # Kubernetes version (required)
      kubespray_version: "v2.31.0"
      api_port: 443
      kube_vip_enabled: true
      kubelet_rotate_server_certs: false
      flavor_bastion: "gp.0.2.2"
      flavor_master: "gp.0.4.8"
      flavor_worker: "gp.0.4.16"
      flavor_worker_windows: "gp.5.4.16"
      subnet_pods: "10.42.0.0/16"
      subnet_services: "10.43.0.0/16"
      loadbalancer_provider: "ovn"
      master_count: 3            # 1-9
      worker_count: 2            # 0-100
      worker_count_windows: 0
      dns_zone_name: ""
      security:
        k8s_hardening: true
        pod_security_exemptions:
          - "trivy-temp"
          - "tigera-operator"
          - "kube-system"
      network_plugin: {}         # CNI configuration
      oidc: {}                   # OIDC configuration
      windows_workers: {}        # Windows configuration
      master_nodes: []           # Pre-configured nodes
      additional_server_pools_worker: []
      additional_server_pools_worker_windows: []
```

**Validation:**

* `version`: Semantic version format (e.g., "1.33.5")
* `master_count`: 1-9
* `worker_count`: 0-100
* `subnet_pods` and `subnet_services` must not overlap

### opencenter.cluster.kubernetes.network_plugin

CNI plugin configuration. Only one plugin can be enabled.

```yaml
opencenter:
  cluster:
    kubernetes:
      network_plugin:
        calico:
          enabled: true
          version: "3.32.0"
          install_method: helm
          network_policy: true
        cilium:
          enabled: false
          install_method: helm
          operator_enabled: true
          kube_proxy_replacement: true
          modules:
            cilium:
              source: "github.com/opencenter-cloud/opencenter-gitops-base.git//iac/cni/cilium?ref=main"
        kube-ovn:
          enabled: false
          install_method: helm
          cilium_integration: true
          modules:
            kube_ovn:
              source: "github.com/opencenter-cloud/opencenter-gitops-base.git//iac/cni/kube-ovn?ref=main"
```

**Validation:**

* Only one CNI plugin can have `enabled: true`
* For OpenStack, supported CNI `install_method` values are `helm` and `kustomize-helm`; `kubespray` is rejected with migration guidance.
* For OpenStack, Calico uses bundled `v3.32.0` native `projectcalico.org/v3` CRDs and eBPF custom resources. Other Calico versions are rejected unless matching assets are added to the CLI.

### opencenter.cluster.kubernetes.oidc

OIDC authentication configuration.

```yaml
opencenter:
  cluster:
    kubernetes:
      oidc:
        enabled: false
        kube_oidc_url: ""
        kube_oidc_client_id: "kubernetes"
        kube_oidc_ca_file: ""
        kube_oidc_username_claim: "sub"
        kube_oidc_username_prefix: "oidc:"
        kube_oidc_groups_claim: "groups"
        kube_oidc_groups_prefix: "oidc:"
```

### opencenter.cluster.kubernetes.windows_workers

Windows worker node configuration.

```yaml
opencenter:
  cluster:
    kubernetes:
      windows_workers:
        enabled: false
        windows_user: "Administrator"
        windows_admin_password: ""
        worker_node_bfv_size_windows: 0
        worker_node_bfv_type_windows: ""
```

### opencenter.identity

Identity provider configuration for services that consume OIDC.

```yaml
opencenter:
  identity:
    oidc:
      enabled: true
      source: internal      # internal or external
      provider: keycloak    # keycloak, entra, or generic
```

When `source: internal` and `provider: keycloak`, OIDC client secrets are created by the OpenCenter Keycloak bootstrap flow and are not required before that bootstrap completes. `secrets.keycloak.admin_password` is still user-provided for now.

### opencenter.gitops

GitOps repository configuration.

```yaml
opencenter:
  gitops:
    git_dir: "./my-cluster-gitops"
    git_url: "ssh://git@github.com/org/repo.git"
    git_ssh_key: ""
    git_ssh_pub: ""
    git_branch: "main"
    gitops_base_repo: "ssh://git@github.com/opencenter-cloud/opencenter-gitops-base.git"
    gitops_base_release: "v0.1.0"
    gitops_branch: "main"
    flux:
      interval: "15m"
      prune: true
```

### opencenter.storage

Storage configuration.

```yaml
opencenter:
  storage:
    default_storage_class: "csi-cinder-sc-delete"
    worker_volume_size: 40
    worker_volume_destination_type: "volume"
    worker_volume_source_type: "image"
    worker_volume_type: "HA-Standard"
    additional_block_devices: []
```

### opencenter.services

Platform services configuration.

```yaml
opencenter:
  services:
    calico:
      enabled: true
      kube_api_server: "https://api.my-cluster.sjc3.k8s.opencenter.cloud:6443"
    cert-manager:
      enabled: true
      email: "mpk-support@rackspace.com"
      region: "us-east-1"
      letsencrypt_server: "https://acme-v02.api.letsencrypt.org/directory"
    etcd-backup:
      enabled: true
      s3_host: "https://swift.api.dfw3.rackspacecloud.com"
      s3_region: "DFW3"
    keycloak:
      enabled: true
      hostname: "auth.my-org.my-cluster.sjc3.k8s.opencenter.cloud"
      realm: "opencenter"
      client_id: "kubernetes"
      frontend_url: "https://auth.my-org.my-cluster.sjc3.k8s.opencenter.cloud"
    kube-prometheus-stack:
      enabled: true
      prometheus_volume_size: 50
      prometheus_storage_class: "csi-cinder-sc-delete"
      grafana_volume_size: 10
      grafana_storage_class: "csi-cinder-sc-delete"
      alertmanager_volume_size: 10
      alertmanager_storage_class: "csi-cinder-sc-delete"
    loki:
      enabled: true
      volume_size: 20
      storage_class: "csi-cinder-sc-delete"
      bucket_name: "my-cluster-loki"
      swift_auth_url: "https://keystone.api.sjc3.rackspacecloud.com/v3/"
      swift_region: "SJC3"
      swift_domain_name: "Default"
    # ... (20+ services total)
```

**Service Base Fields:**

All services support these fields:

* `enabled` (bool): Enable/disable service
* `namespace` (string): Kubernetes namespace
* `hostname` (string): HTTPRoute hostname
* `image_repository` (string): Container image repository
* `image_tag` (string): Container image tag
* `gitops_source_repo` (string): GitOps source repository
* `gitops_source_release` (string): GitOps source release tag
* `gitops_source_branch` (string): GitOps source branch

## opentofu Section

Infrastructure provisioning configuration.

```yaml
opentofu:
  enabled: true
  path: "opentofu"
  backend:
    type: "local"              # Backend type (local, s3)
    local:
      path: ".opentofu-local-my-cluster/terraform.tfstate"
    s3:
      bucket: ""
      key: ""
      region: ""
```

## deployment Section

Deployment automation settings.

```yaml
deployment:
  auto_deploy: true
  method: kubespray
```

## metadata Section

Configuration lifecycle tracking.

```yaml
metadata:
  created_at: "2026-02-17T10:30:00Z"
  created_by: "user@example.com"
  updated_at: "2026-02-17T11:00:00Z"
  tags:
    environment: "production"
    team: "platform"
  annotations:
    description: "Production cluster"
```

## secrets Section

Encrypted secrets configuration.

```yaml
secrets:
  sops_age_key_file: "~/.config/opencenter/clusters/my-org/secrets/age/my-cluster-key.txt"
  ssh_key:
    private: "./secrets/ssh/my-cluster"
    public: "./secrets/ssh/my-cluster.pub"
    cypher: "ed25519"
  global:
    aws:
      infrastructure:
        access_key: ""
        secret_access_key: ""
        region: "us-east-1"
      application:
        access_key: ""
        secret_access_key: ""
        region: ""
    openstack:
      application_credential_id: ""
      application_credential_secret: ""
  cert_manager:
    aws_access_key: ""
    aws_secret_access_key: ""
  loki:
    swift_password: ""
  keycloak:
    client_secret: ""      # internal OIDC bootstrap may create this later
    admin_password: ""     # user-provided for now
  headlamp:
    oidc_client_secret: "" # internal OIDC bootstrap may create this later
  weave_gitops:
    password: ""
    password_hash: ""
  grafana:
    admin_password: ""
  tempo:
    access_key: ""
    secret_key: ""
  alert_proxy:
    core_device_id: ""
    account_service_token: ""
    core_account_number: ""
  vsphere_csi:
    vcenter_host: ""
    username: ""
    password: ""
    datacenters: ""
    insecure_flag: "false"
    port: "443"
```

## Validation Rules

### Cross-Field Dependencies

* `vrrp_ip` required when `use_octavia=false` and `vrrp_enabled=true`
* Only one CNI plugin can be enabled
* `subnet_pods` and `subnet_services` must not overlap
* `subnet_nodes` must not overlap with pod or service subnets

### Format Validation

* Email addresses: RFC 5322 format
* Hostnames: RFC 1123 format
* CIDR ranges: Valid IPv4 CIDR notation
* UUIDs: RFC 4122 format (for OpenStack IDs)
* Semantic versions: `major.minor.patch` format

### Range Validation

* `master_count`: 1-9
* `worker_count`: 0-100
* `worker_count_windows`: 0-50
* `api_port`: 1-65535
* Volume sizes: 10-1000 GB

## Configuration Precedence

1. Command-line flags (the set override mechanism)
2. Configuration file
3. CLI defaults (`~/.config/opencenter/config.yaml`)
4. Built-in defaults

## Example Complete Configuration

See [Getting Started Tutorial](../tutorials/getting-started.md#step-3-configure-your-cluster) for complete configuration examples.

---

## Evidence

This reference is based on:

* Schema definition: `schema/cluster.schema.json:1-2382`
* Configuration defaults: `internal/config/defaults.go:48-451`
* Configuration types: `internal/config/types.go`
* Validation rules: `internal/config/validator.go`
* Session 2 facts inventory: B0 sections 3, 5
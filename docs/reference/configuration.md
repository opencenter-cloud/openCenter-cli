# Configuration Reference


## Table of Contents

- [Configuration File Structure](#configuration-file-structure)
- [Configuration Sections](#configuration-sections)
- [Configuration File Locations](#configuration-file-locations)
- [Environment Variables](#environment-variables)
- [Configuration Validation](#configuration-validation)
- [Configuration Examples](#configuration-examples)
- [See Also](#see-also)
Complete reference for openCenter cluster configuration.

## Configuration File Structure

openCenter uses a single YAML file to define the entire cluster configuration. The file follows a hierarchical structure with four main sections:

```yaml
opencenter:     # Main cluster configuration
opentofu:       # Infrastructure-as-Code settings
secrets:        # Secrets management configuration
overrides:      # Custom overrides (optional)
```

## Configuration Sections

### opencenter

The main configuration section containing all cluster-specific settings.

#### opencenter.meta

Cluster metadata and organizational information.

```yaml
opencenter:
  meta:
    name: my-cluster                    # Cluster name (required)
    env: prod                           # Environment (dev, stage, prod, test)
    region: us-east-1                   # Deployment region
    status: active                      # Cluster status
    organization: myorg                 # Organization name (default: opencenter)
```

**Fields:**
- `name` (string, required): Unique cluster identifier. Must be 3-63 characters, start and end with alphanumeric, contain only lowercase letters, numbers, and hyphens.
- `env` (string): Environment designation. Valid values: `dev`, `stage`, `prod`, `test`, or empty.
- `region` (string): Geographic region for deployment.
- `status` (string): Current cluster status. Valid values: `active`, `inactive`, `maintenance`, or empty.
- `organization` (string, required): Organization name for multi-tenant deployments. Default: `opencenter`.

#### opencenter.infrastructure

Infrastructure provider configuration.

```yaml
opencenter:
  infrastructure:
    provider: openstack                 # Cloud provider (required)
    cloud:
      openstack:                        # OpenStack-specific settings
        auth_url: https://keystone.example.com/v3/
        insecure: false
        region: RegionOne
        application_credential_id: ""
        application_credential_secret: ""
        domain: Default
        tenant_name: my-project
        floating_network_id: ""
        subnet_id: ""
      aws:                              # AWS-specific settings
        profile: default
        region: us-east-1
        vpc_id: vpc-12345678
        private_subnets:
          - subnet-11111111
          - subnet-22222222
        public_subnets:
          - subnet-33333333
          - subnet-44444444
```

**Provider Types:**
- `openstack`: OpenStack cloud platform
- `aws`: Amazon Web Services
- `vmware`: VMware vSphere (partial support)
- `kind`: Kubernetes in Docker (local development)
- `baremetal`: Bare metal servers (planned)

**OpenStack Fields:**
- `auth_url` (string, required): Keystone authentication URL
- `insecure` (boolean): Skip TLS verification (not recommended for production)
- `region` (string, required): OpenStack region name
- `application_credential_id` (string): Application credential ID for authentication
- `application_credential_secret` (string): Application credential secret
- `domain` (string): OpenStack domain name (default: "Default")
- `tenant_name` (string, required): Project/tenant name or ID
- `floating_network_id` (string): External network ID for floating IPs
- `subnet_id` (string): Subnet ID for cluster network

**AWS Fields:**
- `profile` (string): AWS CLI profile name
- `region` (string, required): AWS region (e.g., us-east-1)
- `vpc_id` (string, required): VPC ID for cluster deployment
- `private_subnets` (array): List of private subnet IDs
- `public_subnets` (array): List of public subnet IDs

#### opencenter.cluster

Kubernetes cluster configuration.

```yaml
opencenter:
  cluster:
    cluster_name: my-cluster            # Must match meta.name
    aws_access_key: ""                  # AWS credentials for cluster resources
    aws_secret_access_key: ""
    k8s_api_port_acl:                   # API server access control
      - 0.0.0.0/0
    ssh_authorized_keys:                # SSH keys for node access
      - ssh-rsa AAAAB3...
    kubernetes:
      version: 1.31.4                   # Kubernetes version
      flavor_bastion: gp.0.2.2          # Instance size for bastion
      flavor_master: gp.0.4.4           # Instance size for control plane
      flavor_worker: gp.0.4.8           # Instance size for workers
      subnet_pods: 10.42.0.0/16         # Pod network CIDR
      subnet_services: 10.43.0.0/16     # Service network CIDR
      loadbalancer_provider: ovn        # Load balancer provider
      dns_zone_name: cluster.local      # DNS zone for services
      master_count: 3                   # Number of control plane nodes
      worker_count: 2                   # Number of worker nodes
      worker_count_windows: 0           # Number of Windows workers
      network_plugin:                   # CNI configuration
        calico:
          enabled: true
          cni_iface: enp3s0
          calico_interface_autodetect: interface
        cilium:
          enabled: false
          operator_enabled: true
          kubeProxyReplacement: true
        kube-ovn:
          enabled: false
          cilium_integration: true
      oidc:                             # OIDC authentication
        enabled: false
        kube_oidc_url: ""
        kube_oidc_client_id: kubernetes
        kube_oidc_ca_file: ""
        kube_oidc_username_claim: sub
        kube_oidc_username_prefix: "oidc:"
        kube_oidc_groups_claim: groups
        kube_oidc_groups_prefix: "oidc:"
      windows_workers:                  # Windows node configuration
        enabled: false
        windows_user: Administrator
        windows_admin_password: ""
        worker_node_bfv_size_windows: 0
        worker_node_bfv_type_windows: local
```

**Cluster Fields:**
- `cluster_name` (string, required): Must match `opencenter.meta.name`
- `aws_access_key` (string): AWS access key for cluster resources
- `aws_secret_access_key` (string): AWS secret key for cluster resources
- `k8s_api_port_acl` (array): CIDR blocks allowed to access API server
- `ssh_authorized_keys` (array, required): SSH public keys for node access

**Kubernetes Fields:**
- `version` (string, required): Kubernetes version (e.g., 1.31.4)
- `flavor_bastion` (string): Instance flavor for bastion host
- `flavor_master` (string, required): Instance flavor for control plane nodes
- `flavor_worker` (string, required): Instance flavor for worker nodes
- `subnet_pods` (string): Pod network CIDR (default: 10.42.0.0/16)
- `subnet_services` (string): Service network CIDR (default: 10.43.0.0/16)
- `loadbalancer_provider` (string): Load balancer provider (ovn, octavia, metallb, none)
- `dns_zone_name` (string): DNS zone name for cluster services
- `master_count` (integer, required): Number of control plane nodes (1-9)
- `worker_count` (integer, required): Number of worker nodes (0-100)
- `worker_count_windows` (integer): Number of Windows worker nodes (0-50)

**Network Plugin Configuration:**

Only one network plugin can be enabled at a time.

**Calico:**
- `enabled` (boolean): Enable Calico CNI
- `cni_iface` (string): Network interface name (e.g., enp3s0, eth0)
- `calico_interface_autodetect` (string): Interface detection method (interface, can-reach, skip-interface, cidr)

**Cilium:**
- `enabled` (boolean): Enable Cilium CNI
- `operator_enabled` (boolean): Enable Cilium operator
- `kubeProxyReplacement` (boolean): Replace kube-proxy with eBPF

**Kube-OVN:**
- `enabled` (boolean): Enable Kube-OVN CNI
- `cilium_integration` (boolean): Enable Cilium integration

**OIDC Configuration:**
- `enabled` (boolean): Enable OIDC authentication
- `kube_oidc_url` (string): OIDC provider URL
- `kube_oidc_client_id` (string): OIDC client ID
- `kube_oidc_ca_file` (string): Path to OIDC CA certificate
- `kube_oidc_username_claim` (string): JWT claim for username
- `kube_oidc_username_prefix` (string): Prefix for usernames
- `kube_oidc_groups_claim` (string): JWT claim for groups
- `kube_oidc_groups_prefix` (string): Prefix for groups

**Windows Workers:**
- `enabled` (boolean): Enable Windows worker nodes
- `windows_user` (string): Windows administrator username
- `windows_admin_password` (string): Windows administrator password
- `worker_node_bfv_size_windows` (integer): Boot volume size for Windows nodes
- `worker_node_bfv_type_windows` (string): Boot volume type

#### opencenter.gitops

GitOps repository configuration.

```yaml
opencenter:
  gitops:
    git_dir: ./gitops-repo              # Local repository path (required)
    git_url: git@github.com:org/repo.git  # Remote repository URL
    git_ssh_key: ~/.ssh/id_ed25519-flux   # SSH private key path
    git_ssh_pub: ~/.ssh/id_ed25519-flux.pub  # SSH public key path
    git_branch: main                    # Git branch (default: main)
    release: 1.0.0                     # GitOps base release version
    flux:
      interval: 15m                     # Reconciliation interval
      prune: true                       # Enable resource pruning
```

**Fields:**
- `git_dir` (string, required): Local directory for GitOps repository
- `git_url` (string): Remote Git repository URL (SSH or HTTPS)
- `git_ssh_key` (string): Path to SSH private key for authentication
- `git_ssh_pub` (string): Path to SSH public key
- `git_branch` (string): Git branch name (default: "main")
- `release` (string): GitOps base release version
- `flux.interval` (string): FluxCD reconciliation interval (e.g., 15m, 1h)
- `flux.prune` (boolean): Enable automatic pruning of resources not in Git

#### opencenter.managed-service

Managed service configurations (Rackspace-specific).

```yaml
opencenter:
  managed-service:
    alert-proxy:
      enabled: true
      core_device_id: ""
      account_service_token: ""
      alert_manager_base_url: ""
      core_account_number: ""
```

**Fields:**
- `enabled` (boolean): Enable the managed service
- Service-specific fields vary by service

#### opencenter.services

Cluster service toggles and configurations.

```yaml
opencenter:
  services:
    calico:
      enabled: true
      release: v3.27.0
    cert-manager:
      enabled: true
      email: admin@example.com
      region: us-east-1
      aws_access_key: ""
      aws_secret_access_key: ""
      release: v1.14.0
    etcd-backup:
      enabled: true
      s3_host: https://s3.amazonaws.com
      s3_region: us-east-1
      aws_access_key: ""
      aws_secret_access_key: ""
      release: 1.0.0
    external-snapshotter:
      enabled: true
    fluxcd:
      enabled: true
    gateway:
      enabled: true
    gateway-api:
      enabled: true
    headlamp:
      enabled: true
    keycloak:
      enabled: true
    kube-prometheus-stack:
      enabled: true
    kyverno:
      enabled: true
    olm:
      enabled: true
    openstack-ccm:
      enabled: true
    openstack-csi:
      enabled: true
    postgres-operator:
      enabled: true
    rbac-manager:
      enabled: true
    sources:
      enabled: true
    velero:
      enabled: true
    weave-gitops:
      enabled: true
```

**Common Service Fields:**
- `enabled` (boolean): Enable/disable the service
- `release` (string): Service release version

**Service-Specific Fields:**

**cert-manager:**
- `email` (string): Email for Let's Encrypt notifications
- `region` (string): AWS region for Route53 DNS validation
- `aws_access_key` (string): AWS access key for DNS validation
- `aws_secret_access_key` (string): AWS secret key for DNS validation

**etcd-backup:**
- `s3_host` (string): S3-compatible storage endpoint
- `s3_region` (string): S3 region
- `aws_access_key` (string): AWS access key for S3
- `aws_secret_access_key` (string): AWS secret key for S3

### opentofu

OpenTofu/Terraform infrastructure-as-code configuration.

```yaml
opentofu:
  enabled: true                         # Enable OpenTofu provisioning
  path: opentofu                        # Path to OpenTofu binary or directory
  backend:
    type: local                         # Backend type (local, s3)
    local:
      path: terraform.tfstate           # Local state file path
    s3:
      bucket: my-terraform-state        # S3 bucket name
      key: cluster/terraform.tfstate    # S3 object key
      region: us-east-1                 # AWS region
      endpoint: ""                      # Custom S3 endpoint (optional)
      profile: ""                       # AWS CLI profile (optional)
      encrypt: true                     # Enable server-side encryption
```

**Fields:**
- `enabled` (boolean): Enable OpenTofu for infrastructure provisioning
- `path` (string): Path to OpenTofu binary or working directory
- `backend.type` (string): State backend type (local, s3, azurerm, gcs)
- `backend.local.path` (string): Path to local state file
- `backend.s3.bucket` (string, required for S3): S3 bucket name
- `backend.s3.key` (string, required for S3): S3 object key
- `backend.s3.region` (string, required for S3): AWS region
- `backend.s3.endpoint` (string): Custom S3 endpoint URL
- `backend.s3.profile` (string): AWS CLI profile name
- `backend.s3.encrypt` (boolean): Enable server-side encryption

**Note:** When using S3 backend, AWS credentials must be provided via `opencenter.cluster.aws_access_key` and `opencenter.cluster.aws_secret_access_key`.

### secrets

Secrets management configuration.

```yaml
secrets:
  sops_age_key_file: /path/to/age/key.txt  # Path to SOPS Age key
```

**Fields:**
- `sops_age_key_file` (string): Path to SOPS Age encryption key file

### overrides

Custom configuration overrides (optional).

```yaml
overrides:
  custom_field: value
  nested:
    field: value
```

The `overrides` section allows arbitrary key-value pairs for custom extensions and provider-specific configurations.

## Configuration File Locations

### Organization-Based Structure (Recommended)

```
~/.config/openCenter/
└── clusters/
    └── <organization>/
        └── infrastructure/
            └── clusters/
                └── <cluster-name>/
                    ├── .<cluster-name>-config.yaml
                    ├── secrets/
                    │   └── age/
                    │       └── keys/
                    │           └── <cluster-name>-key.txt
                    └── gitops/
```

### Legacy Structure (Backward Compatibility)

```
~/.config/openCenter/
├── clusters/
│   └── <cluster-name>/
│       └── .<cluster-name>-config.yaml
└── <cluster-name>.yaml
```

## Environment Variables

Configuration behavior can be modified using environment variables:

- `OPENCENTER_CONFIG_DIR`: Override default configuration directory
- `OPENCENTER_DEBUG`: Enable debug mode and generate complete configuration files

## Configuration Validation

openCenter performs comprehensive validation:

1. **Schema Validation**: Configuration must conform to JSON schema
2. **Required Fields**: All required fields must be present
3. **Cross-Field Validation**: Related fields must be consistent
4. **Provider Validation**: Provider-specific requirements must be met
5. **Network Validation**: Network CIDRs must not conflict
6. **Plugin Validation**: Only one network plugin can be enabled

## Configuration Examples

### Minimal OpenStack Cluster

```yaml
opencenter:
  meta:
    name: minimal-cluster
    organization: myorg
  infrastructure:
    provider: openstack
    cloud:
      openstack:
        auth_url: https://keystone.example.com/v3/
        region: RegionOne
        application_credential_id: ""
        application_credential_secret: ""
        tenant_name: my-project
        floating_network_id: "network-id"
  cluster:
    cluster_name: minimal-cluster
    ssh_authorized_keys:
      - ssh-rsa AAAAB3...
    kubernetes:
      version: 1.31.4
      master_count: 1
      worker_count: 1
      network_plugin:
        calico:
          enabled: true
  gitops:
    git_dir: ./gitops-minimal

opentofu:
  enabled: true
  path: opentofu
  backend:
    type: local
    local:
      path: terraform.tfstate

secrets:
  sops_age_key_file: ""
```

### Production AWS Cluster

```yaml
opencenter:
  meta:
    name: prod-cluster
    env: prod
    region: us-east-1
    organization: production
  infrastructure:
    provider: aws
    cloud:
      aws:
        profile: production
        region: us-east-1
        vpc_id: vpc-12345678
        private_subnets:
          - subnet-11111111
          - subnet-22222222
        public_subnets:
          - subnet-33333333
          - subnet-44444444
  cluster:
    cluster_name: prod-cluster
    aws_access_key: "AKIAIOSFODNN7EXAMPLE"
    aws_secret_access_key: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
    k8s_api_port_acl:
      - 10.0.0.0/8
    ssh_authorized_keys:
      - ssh-rsa AAAAB3...
    kubernetes:
      version: 1.31.4
      master_count: 3
      worker_count: 5
      network_plugin:
        cilium:
          enabled: true
          operator_enabled: true
          kubeProxyReplacement: true
      oidc:
        enabled: true
        kube_oidc_url: https://oidc.example.com
        kube_oidc_client_id: kubernetes
  gitops:
    git_dir: ./gitops-prod
    git_url: git@github.com:org/prod-gitops.git
    git_ssh_key: ~/.ssh/id_ed25519-flux
    git_branch: main
    flux:
      interval: 10m
      prune: true
  services:
    cert-manager:
      enabled: true
      email: ops@example.com
      region: us-east-1
    kube-prometheus-stack:
      enabled: true
    velero:
      enabled: true

opentofu:
  enabled: true
  path: opentofu
  backend:
    type: s3
    s3:
      bucket: prod-terraform-state
      key: prod-cluster/terraform.tfstate
      region: us-east-1
      encrypt: true

secrets:
  sops_age_key_file: ~/.config/openCenter/clusters/production/infrastructure/clusters/prod-cluster/secrets/age/keys/prod-cluster-key.txt
```

## See Also

- [CLI Commands Reference](cli-commands.md)
- [Schema Reference](schema.md)
- [How-To: Configure a Cluster](../how-to/configure-cluster.md)
- [Tutorial: Quickstart](../tutorials/quickstart.md)

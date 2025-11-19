# Cluster Configuration File Reference

## Overview

The `.<cluster-name>-config.yaml` file is the primary configuration file for an openCenter Kubernetes cluster. It defines all aspects of cluster infrastructure, services, and secrets management. This file is created during `openCenter cluster init` and serves as the single source of truth for cluster lifecycle operations.

## File Location

The configuration file follows an organization-based directory structure:

```
~/.config/openCenter/clusters/<organization>/.<cluster-name>-config.yaml
```

### Examples

```bash
# Production cluster in "acme-corp" organization
~/.config/openCenter/clusters/acme-corp/.prod-cluster-config.yaml

# Development cluster using cluster name as organization
~/.config/openCenter/clusters/dev-cluster/.dev-cluster-config.yaml
```

### Directory Structure

```
~/.config/openCenter/clusters/
└── <organization>/
    ├── .sops.yaml                           # SOPS encryption config
    ├── .<cluster-name>-config.yaml          # Cluster configuration
    ├── .gitignore                           # Git ignore rules
    ├── secrets/
    │   ├── age/
    │   │   └── keys/
    │   │       ├── <cluster>-key.txt        # SOPS private key
    │   │       └── <cluster>-key.pub        # SOPS public key
    │   └── ssh/
    │       ├── <cluster>-<env>-<region>     # SSH private key
    │       └── <cluster>-<env>-<region>.pub # SSH public key
    └── gitops/
        ├── applications/
        │   └── overlays/<cluster>/
        └── infrastructure/
            └── clusters/<cluster>/
```

## File Structure

The configuration file is a YAML document with the following top-level sections:

### 1. opencenter

The main configuration section containing all cluster-specific settings.

#### opencenter.meta

Cluster metadata and organizational information.

```yaml
opencenter:
  meta:
    name: my-cluster                    # Cluster identifier
    env: prod                           # Environment (dev, staging, prod)
    region: us-east-1                   # Cloud region
    status: active                      # Cluster status
    organization: acme-corp             # Organization name
```

#### opencenter.infrastructure

Cloud provider and infrastructure configuration.

```yaml
opencenter:
  infrastructure:
    provider: openstack                 # Provider: openstack, aws, vmware, kind, baremetal
    cloud:
      openstack:
        auth_url: https://keystone.example.com/v3/
        region: RegionOne
        application_credential_id: <credential-id>
        application_credential_secret: <credential-secret>
        domain: Default
        tenant_name: my-project
        floating_network_id: <network-uuid>
        subnet_id: <subnet-uuid>
        insecure: false               # Skip TLS verification (not recommended)
      aws:
        profile: default              # AWS CLI profile
        region: us-east-1
        vpc_id: vpc-xxxxx
        private_subnets:
          - subnet-xxxxx
          - subnet-yyyyy
        public_subnets:
          - subnet-zzzzz
```

**Supported Providers:**
- `openstack` - OpenStack clouds (Rackspace, etc.)
- `aws` - Amazon Web Services
- `vmware` - VMware vSphere
- `kind` - Kubernetes in Docker (local development)
- `baremetal` - Bare metal servers

#### opencenter.cluster

Core Kubernetes cluster configuration.

```yaml
opencenter:
  cluster:
    cluster_name: my-cluster
    base_domain: k8s.example.com
    cluster_fqdn: my-cluster.us-east-1.k8s.example.com
    admin_email: admin@example.com
    
    # API server access control
    k8s_api_port_acl:
      - 0.0.0.0/0                     # CIDR blocks allowed to access API
    
    # SSH access to nodes
    ssh_authorized_keys:
      - ssh-rsa AAAAB3NzaC1yc2E... user@host
      - ssh-ed25519 AAAAC3NzaC1lZDI1... admin@host
    
    kubernetes:
      version: 1.31.4                 # Kubernetes version
      
      # Node configuration
      master_count: 3                 # Control plane nodes (1, 3, 5, 7, 9)
      worker_count: 2                 # Worker nodes (0-100)
      worker_count_windows: 0         # Windows worker nodes (0-50)
      
      # Instance flavors/sizes
      flavor_bastion: gp.0.2.2
      flavor_master: gp.0.4.4
      flavor_worker: gp.0.4.8
      
      # Network configuration
      subnet_pods: 10.42.0.0/16       # Pod network CIDR
      subnet_services: 10.43.0.0/16   # Service network CIDR
      loadbalancer_provider: ovn      # ovn, octavia, metallb, none
      dns_zone_name: my-cluster.us-east-1.k8s.example.com
      
      # CNI plugin (only one should be enabled)
      network_plugin:
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
      
      # OIDC authentication
      oidc:
        enabled: false
        kube_oidc_url: https://auth.example.com
        kube_oidc_client_id: kubernetes
        kube_oidc_ca_file: /path/to/ca.crt
        kube_oidc_username_claim: sub
        kube_oidc_username_prefix: 'oidc:'
        kube_oidc_groups_claim: groups
        kube_oidc_groups_prefix: 'oidc:'
      
      # Windows worker configuration
      windows_workers:
        enabled: false
        windows_user: Administrator
        windows_admin_password: <encrypted>
        worker_node_bfv_size_windows: 100
        worker_node_bfv_type_windows: local
      
      # Baremetal node configuration
      master_nodes:
        - id: node-1
          name: master-1
          access_ip_v4: 192.168.1.10
      worker_nodes:
        - id: node-2
          name: worker-1
          access_ip_v4: 192.168.1.20
```

#### opencenter.gitops

GitOps repository configuration for cluster manifests.

```yaml
opencenter:
  gitops:
    # Local GitOps repository
    git_dir: /path/to/gitops/repo
    
    # Remote repository (optional)
    git_url: git@github.com:org/cluster-gitops.git
    git_branch: main
    
    # SSH authentication
    git_ssh_key: /path/to/ssh/key
    git_ssh_pub: /path/to/ssh/key.pub
    
    # GitOps base repository
    gitops_base_repo: ssh://git@github.com/rackerlabs/openCenter-gitops-base.git
    gitops_base_release: v0.1.0       # Release tag
    gitops_branch: main               # Or use branch instead of release
    
    # FluxCD reconciliation settings
    flux:
      interval: 15m                   # Reconciliation interval
      prune: true                     # Remove resources not in Git
```

**GitOps Base Repository:**
The `gitops_base_repo` contains reusable Kustomize bases and Helm chart configurations for common services. You can specify either:
- `gitops_base_release`: Use a specific release tag (recommended for production)
- `branch`: Use a branch for development/testing

#### opencenter.storage

Storage class configuration.

```yaml
opencenter:
  storage:
    default_storage_class: csi-cinder-sc-delete
```

#### opencenter.managed-service

Alert proxy and monitoring integration.

```yaml
opencenter:
  managed-service:
    alert-proxy:
      enabled: true
      email: alerts@example.com
      alert_manager_base_url: http://alertmanager.svc:9093/api/v2/alerts
      http_route_fqdn: https://alerts.my-cluster.example.com
      image_repository: ghcr.io/rackerlabs/alert-proxy
      image_tag: latest
```

#### opencenter.services

Service catalog with enable/disable flags and service-specific configuration.

```yaml
opencenter:
  services:
    # CNI Plugins
    calico:
      enabled: true
      calico_kube_api_server: https://api.my-cluster.example.com:6443
    
    # Certificate Management
    cert-manager:
      enabled: true
      email: certs@example.com
      letsencrypt_server: https://acme-v02.api.letsencrypt.org/directory
      region: us-east-1
    
    # GitOps
    fluxcd:
      enabled: true
    
    # Gateway API
    gateway-api:
      enabled: true
    gateway:
      enabled: true
    
    # Monitoring Stack
    kube-prometheus-stack:
      enabled: true
      grafana_volume_size: 10         # GB
      grafana_storage_class: csi-cinder-sc-delete
      prometheus_volume_size: 50      # GB
      prometheus_storage_class: csi-cinder-sc-delete
      alertmanager_volume_size: 10    # GB
      alertmanager_storage_class: csi-cinder-sc-delete
    
    # Logging
    loki:
      enabled: false
      swift_auth_url: https://keystone.example.com/v3/
      swift_region: RegionOne
      swift_domain_name: Default
      loki_bucket_name: my-cluster-loki
      loki_volume_size: 20            # GB
      loki_storage_class: csi-cinder-sc-delete
    
    # Authentication
    keycloak:
      enabled: true
      hostname: auth.my-cluster.example.com
      keycloak_realm: opencenter
      keycloak_frontend_url: https://auth.my-cluster.example.com
      keycloak_client_id: kubernetes
    
    # Dashboard
    headlamp:
      enabled: true
      hostname: dashboard.my-cluster.example.com
      headlamp_oidc_issuer_url: https://auth.my-cluster.example.com/realms/opencenter
      headlamp_oidc_client_id: kubernetes
    
    # Backup
    velero:
      enabled: true
      velero_backup_bucket: my-cluster-backups
      velero_region: us-east-1
    
    etcd-backup:
      enabled: true
      s3_host: https://swift.example.com
      s3_region: RegionOne
    
    # Operators
    olm:
      enabled: true
    postgres-operator:
      enabled: true
    kyverno:
      enabled: true
    rbac-manager:
      enabled: true
    
    # Cloud Integrations
    openstack-ccm:
      enabled: true
    openstack-csi:
      enabled: true
    vsphere-csi:
      enabled: false
      namespace: vmware-system-csi
      image_repository: registry.k8s.io/csi-vsphere
      image_tag: v3.3.0
    
    # Storage
    external-snapshotter:
      enabled: true
    
    # GitOps UI
    weave-gitops:
      enabled: true
      hostname: gitops.my-cluster.example.com
    
    # Service Sources
    sources:
      enabled: true
```

**Common Service Fields:**
- `enabled`: Enable/disable the service
- `hostname`: Service hostname for HTTPRoute
- `http_route_fqdn`: Full URL for the service
- `release`: Specific release version
- `branch`: Git branch (mutually exclusive with release)
- `uri`: Custom Git repository URI
- `gitops_source_repo`: Override GitOps base repository
- `gitops_source_release`: Override GitOps base release
- `namespace`: Kubernetes namespace
- `image_repository`: Container image repository
- `image_tag`: Container image tag

### 2. opentofu

Infrastructure as Code configuration using OpenTofu/Terraform.

```yaml
opentofu:
  enabled: true
  path: opentofu                      # Path to Terraform files
  backend:
    type: local                       # local or s3
    local:
      path: terraform.tfstate
    s3:
      bucket: my-cluster-tfstate
      key: my-cluster/tfstate/terraform.tfstate
      region: us-west-2
```

### 3. secrets

References to secret files and sensitive configuration.

```yaml
secrets:
  # SOPS Age encryption key
  sops_age_key_file: /path/to/secrets/age/keys/my-cluster-key.txt
  
  # SSH key pair for node access
  ssh_key:
    private: /path/to/secrets/ssh/my-cluster-prod-us-east-1
    public: /path/to/secrets/ssh/my-cluster-prod-us-east-1.pub
    cypher: ed25519                   # ed25519, rsa, ecdsa
  
  # Service-specific secrets
  cert_manager:
    aws_access_key: <encrypted>
    aws_secret_access_key: <encrypted>
  
  loki:
    swift_password: <encrypted>
  
  keycloak:
    client_secret: <encrypted>
    admin_password: <encrypted>
  
  headlamp:
    oidc_client_secret: <encrypted>
  
  weave_gitops:
    password: <encrypted>
    password_hash: <encrypted>
  
  grafana:
    admin_password: <encrypted>
  
  alert_proxy:
    core_device_id: <encrypted>
    account_service_token: <encrypted>
    core_account_number: <encrypted>
  
  vsphere_csi:
    vcenter_host: <encrypted>
    username: <encrypted>
    password: <encrypted>
    datacenters: <encrypted>
    insecure_flag: "false"
    port: "443"
```

**Secret Management:**
- Secrets are encrypted using SOPS with Age encryption
- The `sops_age_key_file` points to the private key for decryption
- Use `openCenter sops secrets-encrypt` to encrypt secret values
- Use `openCenter sops secrets-decrypt` to decrypt for viewing

## File Permissions

The configuration file is created with `0600` permissions (read/write for owner only) to protect sensitive path references.

```bash
-rw------- 1 user user 12345 Nov 19 10:30 .my-cluster-config.yaml
```

## Creating a Configuration File

### Using cluster init

The recommended way to create a configuration file:

```bash
# Basic initialization
openCenter cluster init my-cluster

# With organization
openCenter cluster init my-cluster --org acme-corp

# With custom values
openCenter cluster init my-cluster \
  --org production \
  --opencenter.meta.env=prod \
  --opencenter.cluster.kubernetes.version=1.31.4 \
  --opencenter.infrastructure.provider=aws

# Baremetal cluster
openCenter cluster init my-cluster --type baremetal

# Without auto-generating keys
openCenter cluster init my-cluster --no-keygen

# Force overwrite existing configuration
openCenter cluster init my-cluster --force
```

### Dot Notation Overrides

You can override any configuration value using dot notation:

```bash
openCenter cluster init my-cluster \
  --opencenter.cluster.kubernetes.version=1.31.4 \
  --opencenter.cluster.kubernetes.master_count=5 \
  --opencenter.cluster.kubernetes.worker_count=10 \
  --opencenter.infrastructure.provider=aws \
  --opencenter.gitops.flux.interval=10m
```

## Editing Configuration

### Using cluster edit

```bash
# Edit with default editor
openCenter cluster edit my-cluster

# Edit with specific editor
EDITOR=vim openCenter cluster edit my-cluster
```

### Manual Editing

You can edit the file directly:

```bash
vim ~/.config/openCenter/clusters/acme-corp/.my-cluster-config.yaml
```

After manual edits, validate the configuration:

```bash
openCenter cluster validate my-cluster
```

## Validation

### Schema Validation

The configuration file is validated against `schema/cluster.schema.json`:

```bash
# Validate configuration
openCenter cluster validate my-cluster

# Validate during init
openCenter cluster init my-cluster --strict
```

### Common Validation Rules

- `cluster_name`: 3-63 characters, lowercase alphanumeric with hyphens
- `kubernetes.version`: Must match pattern `X.Y.Z` (e.g., `1.31.4`)
- `kubernetes.master_count`: 1-9 (odd numbers recommended for HA)
- `kubernetes.worker_count`: 0-100
- `ssh_authorized_keys`: Must be valid SSH public keys
- `k8s_api_port_acl`: Must be valid CIDR blocks
- CNI plugins: Only one can be enabled at a time

## Configuration Updates

### Using config-update command

```bash
# Update configuration with new values
openCenter cluster config-update my-cluster \
  --opencenter.cluster.kubernetes.worker_count=5

# Force update without validation
openCenter cluster config-update my-cluster \
  --opencenter.cluster.kubernetes.version=1.32.0 \
  --force
```

The `config-update` command:
- Creates a timestamped backup before updating
- Validates the new configuration
- Preserves existing values not being updated

### Backup Files

Backups are created with timestamp suffix:

```
.my-cluster-config.yaml.20251119-103000.backup
```

Restore a backup:

```bash
cp ~/.config/openCenter/clusters/acme-corp/.my-cluster-config.yaml.20251119-103000.backup \
   ~/.config/openCenter/clusters/acme-corp/.my-cluster-config.yaml
```

## Migration

### Legacy to Organization Structure

If you have clusters in the legacy structure, migrate them:

```bash
# Migrate a specific cluster
openCenter cluster migrate my-cluster --org acme-corp

# Migrate all clusters
openCenter cluster migrate --all
```

The migration:
- Moves configuration to organization-based structure
- Updates all path references
- Preserves SOPS keys and SSH keys
- Updates `.sops.yaml` configuration

## Integration with Other Tools

### Programmatic Access

The configuration file can be parsed by external tools:

```python
import yaml

# Load configuration
with open('.my-cluster-config.yaml') as f:
    config = yaml.safe_load(f)

# Access values
cluster_name = config['opencenter']['cluster']['cluster_name']
k8s_version = config['opencenter']['cluster']['kubernetes']['version']
enabled_services = [
    name for name, svc in config['opencenter']['services'].items()
    if svc.get('enabled', False)
]
```

```go
package main

import (
    "gopkg.in/yaml.v3"
    "os"
)

type Config struct {
    OpenCenter struct {
        Cluster struct {
            ClusterName string `yaml:"cluster_name"`
            Kubernetes struct {
                Version string `yaml:"version"`
            } `yaml:"kubernetes"`
        } `yaml:"cluster"`
    } `yaml:"opencenter"`
}

func main() {
    data, _ := os.ReadFile(".my-cluster-config.yaml")
    var config Config
    yaml.Unmarshal(data, &config)
}
```

### JSON Schema

The JSON schema is available for validation and IDE integration:

```bash
# Export schema
openCenter cluster schema --out cluster.schema.json

# Validate with ajv-cli
npm install -g ajv-cli
ajv validate -s cluster.schema.json -d .my-cluster-config.yaml
```

### IDE Integration

Configure your IDE to use the schema for autocomplete and validation:

**VS Code** (`.vscode/settings.json`):
```json
{
  "yaml.schemas": {
    "./schema/cluster.schema.json": [
      "**/clusters/**/*.yaml",
      "**/clusters/**/*-config.yaml",
      "**/.opencenter.yaml"
    ]
  }
}
```

**Neovim** (with yaml-language-server):
```lua
require('lspconfig').yamlls.setup {
  settings = {
    yaml = {
      schemas = {
        ["./schema/cluster.schema.json"] = {
          "**/clusters/**/*.yaml",
          "**/clusters/**/*-config.yaml",
          "**/.opencenter.yaml"
        }
      }
    }
  }
}
```

## Environment Variables

Override configuration directory:

```bash
# Use custom config directory
export OPENCENTER_CONFIG_DIR=/custom/path
openCenter cluster init my-cluster

# Use testdata for development
export OPENCENTER_CONFIG_DIR=./testdata/config
openCenter cluster init test-cluster
```

## Best Practices

### Security

1. **Never commit unencrypted secrets** to version control
2. **Use SOPS encryption** for all sensitive values
3. **Restrict file permissions** to `0600` for config files
4. **Use separate organizations** for different security domains
5. **Rotate SSH keys** and SOPS keys periodically

### Organization Structure

1. **Use meaningful organization names** (e.g., `production`, `staging`, `dev`)
2. **Group related clusters** in the same organization
3. **Share SOPS keys** within an organization for operational efficiency
4. **Use separate organizations** for different teams or security boundaries

### Configuration Management

1. **Version control your GitOps repository** but not the config file itself
2. **Use `--strict` validation** in CI/CD pipelines
3. **Create backups** before major changes
4. **Document custom configurations** in your organization's runbook
5. **Use consistent naming conventions** across clusters

### Service Configuration

1. **Enable only required services** to reduce resource usage
2. **Configure persistent storage** for stateful services
3. **Set appropriate resource limits** for monitoring services
4. **Use OIDC authentication** for production clusters
5. **Enable backup services** (Velero, etcd-backup) for production

## Troubleshooting

### Configuration Not Found

```bash
# Check if cluster exists
openCenter cluster list

# Check file location
ls -la ~/.config/openCenter/clusters/*/.*-config.yaml

# Verify organization
openCenter cluster info my-cluster
```

### Validation Errors

```bash
# Validate configuration
openCenter cluster validate my-cluster

# Check schema version
openCenter cluster schema --version

# Regenerate from schema
openCenter cluster init my-cluster --force --strict
```

### Permission Errors

```bash
# Fix file permissions
chmod 600 ~/.config/openCenter/clusters/acme-corp/.my-cluster-config.yaml

# Fix directory permissions
chmod 700 ~/.config/openCenter/clusters/acme-corp
```

### SOPS Decryption Errors

```bash
# Verify SOPS key exists
ls -la ~/.config/openCenter/clusters/acme-corp/secrets/age/keys/

# Check SOPS configuration
cat ~/.config/openCenter/clusters/acme-corp/.sops.yaml

# Test decryption
openCenter sops secrets-decrypt my-cluster
```

## Related Documentation

- [Cluster Commands Reference](./reference/cluster/readme.md)
- [Configuration Reference](./reference/configuration.md)
- [SOPS Secrets Management](./reference/secrets.md)
- [GitOps Integration](./reference/gitops.md)
- [Schema Validation](./reference/cluster/schema.md)
- [Migration Guide](./reference/cluster/migrate.md)

## API Reference for Tool Developers

### Configuration Structure

The configuration follows a hierarchical structure that can be accessed programmatically:

```
opencenter
├── meta (ClusterMetadata)
├── infrastructure (InfrastructureConfig)
│   ├── provider (string)
│   └── cloud (CloudConfig)
├── cluster (ClusterConfig)
│   ├── cluster_name (string)
│   ├── kubernetes (KubernetesConfig)
│   └── ssh_authorized_keys ([]string)
├── gitops (GitOpsConfig)
├── storage (StorageConfig)
├── managed-service (map[string]ServiceConfig)
└── services (map[string]ServiceConfig)
opentofu (OpenTofuConfig)
secrets (SecretsConfig)
```

### Path Resolution

The CLI uses a path resolver to locate configuration files:

```go
// Resolve cluster paths
pathResolver := config.NewPathResolver(configManager)
paths := pathResolver.ResolveClusterPaths(clusterName, organization)

// Access paths
configPath := paths.ConfigPath
gitopsDir := paths.GitOpsDir
sopsKeyPath := paths.SOPSKeyPath
```

### Configuration Loading

```go
// Load configuration
cfg, err := config.LoadClusterConfig(clusterName)
if err != nil {
    return err
}

// Access configuration
version := cfg.OpenCenter.Cluster.Kubernetes.Version
provider := cfg.OpenCenter.Infrastructure.Provider
```

### Schema Validation

```go
// Validate configuration
errs := config.Validate(cfg)
if len(errs) > 0 {
    for _, err := range errs {
        fmt.Println(err)
    }
}
```

### Service Discovery

```go
// Get enabled services
enabledServices := []string{}
for name, svc := range cfg.OpenCenter.Services {
    if svc.Enabled {
        enabledServices = append(enabledServices, name)
    }
}
```

## Version History

- **v0.1.0**: Initial organization-based structure
- **v0.2.0**: Added managed-service section
- **v0.3.0**: Added SSH key configuration in secrets
- **v0.4.0**: Added Talos provider support (planned)

## Contributing

When adding new configuration options:

1. Update `internal/config/config.go` with new fields
2. Update `schema/cluster.schema.json` with validation rules
3. Update this documentation
4. Add test cases in `testdata/config/`
5. Run `mise run schema-verify` to validate changes

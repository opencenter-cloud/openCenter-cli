# Configuration File Reference

The `openCenter` cluster configuration is a single YAML file that serves as the declarative source of truth for a cluster's layout, from its GitOps repository to its cloud provider details.

This document provides a detailed reference for every available field in the configuration file.

For a better editing experience, we highly recommend setting up schema validation in your IDE. See our guide on [How to Configure Your IDE](./../how-to/configure-ide.md).

## Top-Level Fields

These are the main keys at the root of the configuration file.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `cluster_name` | `string` | Yes | The unique name for the cluster. This is used as the filename. |
| `template` | `string` | No | Cluster template type: `openstack` (default), `kind`, `vmware`, `baremetal`, or `talos`. Determines the default infrastructure configuration. |
| `naming_prefix`| `string` | No | An optional prefix for all named resources. |
| `cluster` | `object` | No | High-level metadata: `name`, `env`, `region`, `status`. |
| `gitops` | `object` | Yes | Configuration for the GitOps repository. |
| `opencenter` | `object` | No | Global openCenter settings, including AWS creds for OpenTofu S3 backend. |
| `opentofu` | `object` | No | Settings for OpenTofu scaffolding and state backend. |
| `ansible` | `object` | No | Settings for Ansible integration. |
| `iac` | `object` | Yes | Infrastructure-as-code and cluster layout settings. |
| `cloud` | `object` | Yes | All cloud provider-specific settings. |
| `secrets` | `object` | No | Secret management settings. |

---

## `template`

The `template` field determines which infrastructure-specific configuration defaults are applied during cluster initialization. Each template includes optimized settings for its target platform.

### Available Templates

| Template | Description | Use Case |
| --- | --- | --- |
| `openstack` | Full OpenStack deployment with Nova, networking, and Kubespray | Production OpenStack environments |
| `kind` | Local Kubernetes cluster using Kind | Local development and testing |
| `vmware` | VMware vSphere virtual machine deployment | Enterprise VMware environments |
| `baremetal` | Physical server deployment | On-premises hardware |
| `talos` | Talos Linux Kubernetes deployment | Secure, minimal, and immutable Kubernetes |

### Template-Specific Configurations

**OpenStack Template:**
- Configures OpenStack Nova module for VM provisioning
- Includes Calico networking and Kubespray cluster setup
- Sets up VRRP, floating IPs, and security groups
- Optimized for production workloads

**Kind Template:**
- Configures local Kind cluster with registry support
- Includes ingress controller setup
- Suitable for development workflows
- Minimal resource requirements

**VMware Template:**
- Configures vSphere VM provisioning
- Includes resource pool and datastore management
- Enterprise-grade networking and storage
- Supports HA configurations

**Bare Metal Template:**
- Configures inventory-based deployment
- Includes Ansible-based provisioning
- Direct hardware management
- High-performance computing workloads

**Talos Template:**
- Configures Talos Linux minimal OS
- Immutable infrastructure with API-driven configuration
- Built-in security hardening and encryption
- Optimized for Kubernetes with automatic updates
- Declarative machine and cluster configuration

---

## `gitops`

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `git_dir` | `string` | `""` | **Required.** The absolute path on your local machine where the GitOps repository will be generated. |
| `git_url` | `string` | `""` | **Required.** The SSH URL of the remote Git repository where the configuration will be pushed. |
| `git_ssh_key`| `string` | `""` | Optional. Path to a specific SSH private key to use for pushing to the remote repository. |
| `git_branch`| `string` | `""` | Optional. Branch to push to (defaults to `main` if unset in bootstrap). |
| `flux.interval`| `string` | `""` | Optional. Reconciliation interval (e.g., `1m`). |
| `flux.prune`| `boolean` | `false` | Optional. Enable pruning in Flux. |

---

## `opencenter`

Holds global settings used by multiple subsystems.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `aws_access_key` | `string` | `""` | AWS access key ID used by the OpenTofu S3 backend. |
| `aws_secret_access_key` | `string` | `""` | AWS secret access key used by the OpenTofu S3 backend. |

Notes
- These are only required when `opentofu.backend.type` is `s3`.
- Consider managing the config file with SOPS if storing secrets here.

---

## `opentofu`

Configures the OpenTofu module scaffolding in your GitOps repo and its state backend.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `enabled` | `boolean` | `true` | Whether to scaffold OpenTofu files during `cluster setup`. |
| `path` | `string` | `opentofu` | Subdirectory within `gitops.git_dir` for OpenTofu files. |
| `backend.type` | `string` | `local` | State backend type: `local` or `s3`. |
| `backend.local.path` | `string` | `terraform.tfstate` | Path to the local state file. |
| `backend.s3.bucket` | `string` | `""` | S3 bucket name for remote state. |
| `backend.s3.key` | `string` | `""` | Object key for the state file. |
| `backend.s3.region` | `string` | `""` | AWS region. |
| `backend.s3.endpoint` | `string` | `""` | Optional S3-compatible endpoint. |
| `backend.s3.profile` | `string` | `""` | Optional AWS profile to use. |
| `backend.s3.encrypt` | `boolean` | `false` | Enable SSE encryption flag. |

When `backend.type` is `s3`, the generated `provider.tf` will include `access_key` and `secret_key` from `opencenter` if provided.

Example (S3 backend):
```yaml
opencenter:
  aws_access_key: "AKIA..."
  aws_secret_access_key: "..."
opentofu:
  enabled: true
  path: opentofu
  backend:
    type: s3
    s3:
      bucket: my-terraform-state
      key: clusters/demo/terraform.tfstate
      region: us-east-1
```

---

## `ansible`

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `enabled` | `boolean` | `true` | Whether to use Ansible for provisioning. |
| `path` | `string` | `ansible` | The subdirectory within the `git_dir` for Ansible files. |
| `inventory` | `string` | `""` | Optional. Path or filename for inventory. |
| `playbooks` | `array` | `[]` | Optional. List of playbooks to include. |

---

## `iac`

Infrastructure-as-code inputs used to render Terraform for the cluster. This model is intentionally close to Terraform:

- `iac.main`: object representing the `locals { ... }` block in `main.tf`. Keys become local variable names; values can be strings, numbers, booleans, lists, or maps. When rendering, string values that look like expressions (contain `local.`, `var.`, `module.`, start with `${...}`, or function-like calls) are emitted as expressions; other strings are quoted.
- `iac.modules`: object mapping module name to an object of attributes that become a `module "<name>" { ... }` block. Each module typically includes a `source` string and any required inputs. Values follow the same rendering rules as above.

Notes
- `openCenter cluster init` omits entries whose values contain `local.` by default to keep the YAML concise. Pass `--full-schema` to include all defaults, including `local.*` references.
- During `cluster setup`, `main.tf` is always generated from `iac.main` and `iac.modules`. The legacy `iac.main_tf` string is deprecated and no longer used.
---

<!-- Legacy per-field iac.* subsections (storage, networking, etc.) have been removed in favor of the Terraform-aligned shape described above. -->

---

## `cloud` and `cloud.openstack`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `provider` | `string` | Yes | The cloud provider to use. Default is `openstack`. |
| `openstack.auth_url` | `string` | Yes | The Keystone authentication URL. |
| `openstack.insecure` | `boolean`| No | If true, allows insecure TLS connections. Default `false`. |
| `openstack.region` | `string` | Yes | The OpenStack region. |
| `openstack.user_name` | `string` | Yes | The OpenStack username. |
| `openstack.user_password` | `string` | Yes | The OpenStack user password. |
| `openstack.project_domain_name`| `string` | Yes | The OpenStack project domain name. |
| `openstack.user_domain_name` | `string` | Yes | The OpenStack user domain name. |
| `openstack.tenant_name` | `string` | Yes | The OpenStack tenant/project name. |
| `openstack.availability_zone`| `string` | Yes | The availability zone for resources. |
| `openstack.floatingip_pool` | `string` | Yes | The floating IP pool to use. |
| `openstack.router_external_network_id` | `string` | Yes | The ID of the external network for the router. |
| `openstack.disable_bastion` | `boolean`| No | If true, do not create a bastion host. Default `false`. |
| `openstack.ca` | `string` | No | Path to a custom CA certificate for the OpenStack endpoint. |
| `openstack.external_network` | `string` | No | Name of the external network. |
| `openstack.use_octavia` | `boolean` | No | Convenience flag for Octavia usage. |
| `openstack.vrrp_ip` | `string` | No | Convenience VRRP IP when not using Octavia. |

## `cloud.aws`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `aws.profile` | `string` | No | AWS profile name. |
| `aws.region` | `string` | No | AWS region. |
| `aws.vpc_id` | `string` | No | Target VPC ID. |
| `aws.private_subnets` | `array` | No | Private subnet IDs. |
| `aws.public_subnets` | `array` | No | Public subnet IDs. |

---

## Validation Rules

The `openCenter cluster validate` command enforces these rules:

- `cluster_name` must be set.
- `gitops.git_dir` must be set.
- If `opentofu.enabled` is true, then:
  - `opentofu.path` must be set.
  - `opentofu.backend.type` must be `local` or `s3`.
  - For `local` backend, `opentofu.backend.local.path` must be set.
  - For `s3` backend, `opentofu.backend.s3.bucket`, `key`, and `region` must be set; and `opencenter.aws_access_key` and `opencenter.aws_secret_access_key` must be provided.

## Minimal Example

```yaml
cluster_name: demo
gitops:
  git_dir: /tmp/opencenter-demo
  git_url: git@github.com:example/demo.git
  git_branch: main
  flux:
    interval: 1m
    prune: true
opentofu:
  enabled: true
  backend:
    type: local
    local:
      path: terraform.tfstate
iac:
  main:
    cluster_name: demo
    subnet_nodes: 10.2.188.0/22
    k8s_api_port: 443
  modules:
    openstack-nova:
      source: github.com/rackerlabs/openCenter.git//install/iac/infra/openstack-nova?ref=main
      subnet_nodes: local.subnet_nodes
      k8s_api_port: local.k8s_api_port
    kubespray-cluster:
      source: github.com/rackerlabs/openCenter.git//install/iac/kubespray?ref=main
      cluster_name: local.cluster_name
cloud:
  provider: openstack
  openstack:
    auth_url: https://keystone.example.com/v3
    region: RegionOne
    user_name: my-user
    user_password: my-password
    project_domain_name: Default
    user_domain_name: Default
    tenant_name: my-project
secrets:
  sops_age_key_file: ~/.config/openCenter/sops/age/keys/demo-key.txt
```

## `secrets`

Defines paths and settings related to secret management.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `sops_age_key_file` | `string` | `""` | Path to the SOPS age secret key file used for encryption/decryption. |

Notes
- If `sops_age_key_file` is not set at init time, `openCenter` automatically generates a key at `~/.config/openCenter/sops/age/keys/<cluster-name>-key.txt` and updates the saved config accordingly.
- The generated file is written with permissions `0600` and contains a key string starting with `AGE-SECRET-KEY-1`.
 - To disable auto-generation during init, pass `--no-sops-keygen` to `openCenter cluster init`.

### Sources

*   `internal/config/config.go`
*   `internal/config/schema.go`
*   `README.md`

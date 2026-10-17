---
last_updated: 2026-09-25
id: configuration-schema
title: "Configuration Schema Reference"
sidebar_label: Configuration Schema
description: Structural reference for the v2 cluster configuration schema -- every top-level section, its key fields, types, and where the exhaustive machine-readable schema lives.
doc_type: reference
audience: "operators, developers"
tags: [configuration, schema, v2, reference]
---
# Configuration Schema Reference

**Purpose:** For operators and developers, documents the structure of a v2 cluster configuration file, verified against `internal/config/v2/*.go`. Only `schema_version: "2.0"` is supported by the current CLI.

The exhaustive, always-current, machine-readable schema is generated straight from these Go types:

```bash
mise run schema-v2                               # regenerate schema/opencenter-v2.schema.json from the Go types
opencenter settings ide                          # generate the schema plus editor (YAML Language Server) setup
```

...or read the checked-in copy at `schema/opencenter-v2.schema.json` (regenerate with `mise run schema-v2`; see [Mise Tasks Reference](mise-tasks.md)). The separate `mise run schema` task is a legacy definition and is not the task used for this v2 file. This page is a structural map to navigate the schema, not a byte-for-byte transcription of it -- always trust the generated schema over this page for an exact enum list or a field you don't see below.

## Top level (`Config`)

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `schema_version` | string | yes | Must equal `"2.0"`. |
| `metadata` | object | no | System-managed (`created_at`, `updated_at`, `created_by`, `version`, `labels`, `annotations`). |
| `opencenter` | object | yes | Everything else -- see below. |
| `deployment` | object | no | Deployment method (see below). |
| `opentofu` | object | yes | OpenTofu/Terraform backend config. |
| `secrets` | object | yes | Secret values and secret-adjacent config (SOPS, overlay-unit secrets). See [GitOps Configuration Reference](gitops-configuration.md) for the overlay-unit half of this. |

## `opencenter` (`OpenCenterConfig`)

| Field | Required | Notes |
| --- | --- | --- |
| `meta` | yes | Cluster identity: `name` (DNS-1123), `organization`, `env` (`dev`\|`staging`\|`production`), `region`, `stage`, `status`, `locked`, `lock_reason`. |
| `cluster` | yes | Kubernetes-specific config, independent of infrastructure -- see below. |
| `infrastructure` | yes | Provider selection and provider-specific config -- see below. |
| `secrets` | no | `backend` selection plus `barbican` (OpenStack Key Manager client config: `auth_url`, `project_id`, `region`, `user_domain_name`, `project_domain_name`, `ca_cert`). |
| `identity` | no | `oidc.enabled`, `oidc.source` (`internal`\|`external`), `oidc.provider` (`keycloak`\|`entra`\|`generic`). |
| `services` | no | The canonical platform-services map (see below). |
| `managed_services` | no | The canonical managed/customer-application-services map (see below). |
| `managed-service` | no | **Legacy alias** for `managed_services` (Go field `LegacyManaged`) -- kept for backward compatibility with older config files, not the field new configs should use. |
| `talos` | no | Legacy field (`LegacyTalos`, `map[string]any`), excluded from the generated JSON schema (`json:"-"`) -- present only for round-tripping old files. |
| `gitops` | yes | See [GitOps Configuration Reference](gitops-configuration.md) for the complete field list. |

### `opencenter.cluster` (`ClusterConfig`)

`cluster_name` (DNS-1123), `base_domain` (FQDN), `cluster_fqdn` (FQDN), `admin_email`, and `kubernetes`:

| `kubernetes.*` field | Notes |
| --- | --- |
| `version` | Required, semver. |
| `api_port` | Required, 1-65535. |
| `kube_vip_enabled`, `kubelet_rotate_server_certs` | Booleans. |
| `subnet_pods`, `subnet_services` | Required, IPv4 CIDR. |
| `network_plugin` | For non-Kind providers, exactly one of `calico`, `cilium`, `kube-ovn` must be enabled. Kind uses its built-in `kindnet` when `infrastructure.kind.disable_default_cni` is false; exactly one managed plugin is required only when that flag is true. |
| `storage_plugin`, `security`, `oidc` | Optional sub-blocks. |

### `opencenter.infrastructure` (`InfrastructureConfig`)

| Field | Notes |
| --- | --- |
| `provider` | Required. One of `openstack`, `aws`, `gcp`, `azure`, `baremetal`, `vsphere`, `vmware`, `kind`, `magnum`. (`aws`/`gcp`/`azure` are accepted by the schema but currently rejected at the CLI level by `checkProviderAvailability` as "planned, not yet available".) |
| `ssh` | Required. `authorized_keys` (min 1), `username`/`user`, `key_path`. |
| `os_version` | Required string. |
| `k8s_api_ip`, `node_naming`, `bastion` | Optional. `bastion.flavor`/`bastion.image` required only if `bastion.enabled`. |
| `networking` | Required -- node subnet/allocation pool/gateway, VRRP (`vrrp_ip` required if `vrrp_enabled`), load-balancer provider (`ovn`\|`octavia`\|`metallb`\|`cloud-native`), Designate/DNS, NTP servers (min 1), VLAN. |
| `compute` | Required -- flavors, `master_count`/`worker_count`/`worker_count_windows`, static node lists (baremetal/vmware), `additional_server_pools_worker[_windows]` for extra worker pools. `worker_group_affinity` and `cp_server_group_affinity` are optional `[]string` fields that render to `locals.wn_server_group_affinity` / `locals.cp_server_group_affinity` in the OpenStack `main.tf`; both default to `["soft-anti-affinity"]`. |
| `storage` | Required -- `default_storage_class`, worker/master volume size/type/source/destination, `additional_block_devices`. |
| `cloud` | Required -- exactly one of the per-provider blocks below should be populated for the selected provider; every other provider's block must be empty (enforced by readiness validation). |
| `kind` | Optional, Kind-specific settings that don't yet have a provider-agnostic home (`cluster_name`, `kubernetes_version`, `node_image`, node counts, API server address/port, pod/service subnet, `disable_default_cni`, ingress, runtime, registry, `extra_port_mappings`, `extra_mounts`). |

#### `infrastructure.cloud` (`CloudConfig`) -- one populated block per provider

| Sub-block | Verified required fields |
| --- | --- |
| `openstack` | `auth_url` (URL), `region`, `project_id`; optional `project_name`, application credential pair, `insecure`, `domain`/`domain_name`, plus a nested `OpenStackNetworkingConfig` (`network_id`, `subnet_id`, `floating_ip_pool`/`floating_network_id`, `router_external_network_id`, `k8s_api_port_acl`, `designate`). |
| `magnum` | `auth_url` (URL), `region`, `project_id`, `cluster_template`; `application_credential_id`/`application_credential_secret` required together (`validate:"required_with=..."` both ways); optional `insecure`, `domain`, `ca`, `labels`, `keypair`, `master_flavor_id`, `node_flavor_id`, `create_timeout`, `master_lb_enabled`. |
| `aws`, `gcp`, `azure`, `vmware` | Exist as their own typed blocks; see [Providers](providers.md) for field-level detail (owned separately from this page). |

Every provider's config-level `ValidateConfig` explicitly rejects a populated `cloud.magnum` block unless `provider: magnum` (and analogous checks exist for the other blocks) -- these blocks are mutually exclusive by construction, not just convention.

### `opencenter.services` / `opencenter.managed_services` (`ServiceMap`)

`ServiceMap` is `map[string]any` with **custom YAML unmarshaling**: the service registry (`internal/config/registry`) resolves each key to its registered typed Go struct at unmarshal time, so `services.cert-manager` in a config file becomes a real `*services.CertManagerConfig` in memory, not a loose map. Every service's typed config embeds `services.BaseConfig` (`enabled`, `namespace`, plus status fields). This makes the shape of any individual service field-by-field a per-service question -- see [Platform Services](platform-services.md) (owned separately) for the enabled service catalog, and [Adding New Platform Services](../contributing/adding-services.md) for the contract new service configs must follow (declarative fields only -- no rendering topology, no raw Helm override content).

Stability note (from `internal/config/v2/config.go`): the overlay-unit types referenced by `gitops.overlay_units` and `secrets.overlay_units` are stable as of schema version 2.0; `ServiceMap` itself keeps its `map[string]any` polymorphic shape by design.

## `opentofu` (`OpenTofuConfig`)

`enabled` (bool), `path`, `backend` (required): `backend.type` is one of `s3`\|`local`\|`remote`, with a matching `local`/`s3` sub-block.

## `deployment` (`DeploymentConfig`)

`auto_deploy` (bool), `method` (required, schema enum `kubespray`\|`kamaji`\|`eks`\|`gke`\|`aks`\|`cluster-api`), plus a matching optional sub-block (`kubespray`, `kamaji`, or `cluster_api`). The current deployment validator implements only `kubespray` and `kamaji`; `eks`, `gke`, `aks`, and `cluster-api` are accepted by the struct tag/schema but rejected when `ValidateDeployment` resolves a method. `kubespray.version` is required (semver) when the `kubespray` block is present; it also carries a `modules` map and a `kubespray_cluster` module config for enabling/version-pinning individual Kubespray roles. `kubespray.cloud_init_timeout` controls the CLI-owned Kubespray lifecycle wait; rendered Terraform does not pass it unconditionally to legacy/custom modules, which retain their own defaults unless they independently support an equivalent input.

## `secrets` (`SecretsConfig`)

Plaintext secret *values* referenced by services and infrastructure -- this is the section SOPS is expected to encrypt at rest. Notable fields: `sops_age_key_file`, `ssh_key` (`private`/`public`/`cypher`), `global` (cross-cutting AWS/OpenStack credentials), per-service secret blocks (`cert_manager`, `loki`, `mimir`, `keycloak`, `headlamp`, `weave_gitops`, `grafana`, `harbor`, `tempo`, `etcd_backup`, `velero`, `alert_proxy`, `vsphere_csi`), a catch-all `service_secrets` map, `sops` (SOPS encryption toggle + `age_key_file` + `encrypted_regex`), and `overlay_units` (the `secrets.overlay_units.customer_managed` block documented in [GitOps Configuration Reference](gitops-configuration.md)).

`cert_manager` secrets support **named, multi-credential** configuration -- `aws` and `cloudflare` are each `map[string]<credential>`, so a cluster can hold several named AWS Route53 or Cloudflare credentials scoped to different DNS zones, in addition to legacy flat fields kept only for migration compatibility.

## Cross-references

* [GitOps Configuration Reference](gitops-configuration.md) -- full `opencenter.gitops.*` and `secrets.overlay_units.*` field list.
* [Default Values](default-values.md) -- what `opencenter cluster init` actually populates before you touch the file.
* [Configuration Precedence](configuration-precedence.md) -- how file, template, and CLI-flag values combine.
* [Validation Rules](validation-rules.md) -- which of the fields above are actually enforced, and how.
* [Providers](providers.md), [Platform Services](platform-services.md) -- per-provider and per-service field detail (owned separately).

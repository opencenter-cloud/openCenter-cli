# Platform Services Reference

Quick-reference directory for all openCenter platform services. Each file documents configuration fields, required secrets, dependencies, and verification steps.

## Services by Category

### Networking

| Service | Default | Secrets | Key Config |
|---------|---------|---------|------------|
| [calico](calico.md) | ✅ Enabled | None | `kube_api_server` |
| [gateway-api](gateway-api.md) | ✅ Enabled | None | CRDs only |
| [gateway](gateway.md) | ✅ Enabled | None | `gateway_name`, `gateway_class`, `listeners[]` |
| [metallb](metallb.md) | ✅ Enabled | None | `ip_address_pools[].addresses` |

### Security

| Service | Default | Secrets | Key Config |
|---------|---------|---------|------------|
| [cert-manager](cert-manager.md) | ✅ Enabled | AWS/Cloudflare creds per `dns_provider` | `dns_provider`, `email`, `issuers[]` |
| [keycloak](keycloak.md) | ✅ Enabled | `admin_password` (always), `client_secret` (external OIDC) | `hostname`, `realm`, `instances` |
| [kyverno](kyverno.md) | ✅ Enabled | None | 17 default policies |
| [rbac-manager](rbac-manager.md) | ✅ Enabled | None | RBACDefinition CRD |
| [sealed-secrets](sealed-secrets.md) | ✅ Enabled | None | — |

### Storage

| Service | Default | Secrets | Key Config |
|---------|---------|---------|------------|
| [openstack-ccm](openstack-ccm.md) | ✅ Enabled | Uses infra OpenStack creds | OpenStack only |
| [openstack-csi](openstack-csi.md) | ✅ Enabled | Uses infra OpenStack creds | OpenStack only |
| [vsphere-csi](vsphere-csi.md) | ❌ Disabled | `vcenter_host`, `username`, `password` | `storage_classes[]`, VMware only |
| [longhorn](longhorn.md) | ❌ Disabled | None | `default_replica_count`, `backup_target` |
| [external-snapshotter](external-snapshotter.md) | ✅ Enabled | None | — |

### Observability

| Service | Default | Secrets | Key Config |
|---------|---------|---------|------------|
| [kube-prometheus-stack](kube-prometheus-stack.md) | ✅ Enabled | `grafana.admin_password` | `*_volume_size`, `hostname` |
| [loki](loki.md) | ✅ Enabled | Swift **or** S3 creds per `storage_type` | `storage_type`, `bucket_name`, `s3_endpoint` |
| [tempo](tempo.md) | ✅ Enabled | Swift **or** S3 creds per `storage_type` | `storage_type`, `bucket_name`, `s3_endpoint` |
| [mimir](mimir.md) | ❌ Disabled | Global AWS app creds | S3 + Kafka ingest |
| [opentelemetry-kube-stack](opentelemetry-kube-stack.md) | ❌ Disabled | None | `collector_mode`, `exporters[]` |
| [alert-proxy](alert-proxy.md) | ❌ Disabled | `core_device_id`, `account_service_token` | `http_route_fqdn` |

### GitOps

| Service | Default | Secrets | Key Config |
|---------|---------|---------|------------|
| [fluxcd](fluxcd.md) | ✅ Enabled | None | Core service |
| [weave-gitops](weave-gitops.md) | ❌ Disabled | `password` or `password_hash` | — |

### Backup

| Service | Default | Secrets | Key Config |
|---------|---------|---------|------------|
| [velero](velero.md) | ✅ Enabled | Swift/S3/GCS/Azure creds per `storage_type` | `backup_bucket`, `storage_type` |
| [etcd-backup](etcd-backup.md) | ✅ Enabled | Global AWS creds | `s3_host`, `s3_region` |

### Management

| Service | Default | Secrets | Key Config |
|---------|---------|---------|------------|
| [headlamp](headlamp.md) | ✅ Enabled | `oidc_client_secret` | `hostname`, `oidc_issuer_url` |
| [olm](olm.md) | ✅ Enabled | None | — |
| [postgres-operator](postgres-operator.md) | ✅ Enabled | None | Required by keycloak |
| [harbor](harbor.md) | ❌ Disabled | None | `hostname`, `storage_type`, `registry_volume_size` |
| [kafka-cluster](kafka-cluster.md) | ❌ Disabled | None | Required by mimir |

## Storage Backend Quick Reference

Services with object storage (`loki`, `tempo`, `velero`) auto-detect the backend from the infrastructure provider:

| Provider | Default `storage_type` | Credential Source |
|----------|----------------------|-------------------|
| OpenStack | `swift` | `secrets.<service>.swift_application_credential_secret` |
| AWS | `s3` | `secrets.<service>.s3_*` → global AWS app creds fallback |
| GCP | `gcs` | `secrets.service_secrets.<service>.gcp_service_account_key` |
| Azure | `azure` | `secrets.service_secrets.<service>.azure_storage_account_key` |

## Common Operations

```bash
# List all service states
opencenter cluster service status

# Enable/disable
opencenter cluster service enable <service>
opencenter cluster service disable <service>

# View available config fields
opencenter cluster service options <service>

# Set a config value
opencenter cluster set my-cluster opencenter.services.<service>.<field>=<value>
```
